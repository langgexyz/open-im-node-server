package activate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/langgexyz/open-im-node-server/internal/config"
)

// activatePayload mirrors hub-server's activatePayload (AES-encrypted JSON sent by Hub).
type activatePayload struct {
	AppID         string `json:"app_id"`
	AppPrivateKey string `json:"app_private_key"`
	AppPublicKey  string `json:"app_public_key"`
	HubGRPCAddr   string `json:"hub_grpc_addr"`
	HubPublicKey  string `json:"hub_public_key"`
	HubWebOrigin  string `json:"hub_web_origin"`
}

// OnActivatedFunc is called after activation completes.
// It should initialise the MySQL operator account and create the subscription group (group_id="0").
type OnActivatedFunc func(nodeID string) error

// Handler handles the POST /node/activate endpoint.
type Handler struct {
	cfg         *config.Config
	configPath  string
	mu          sync.Mutex
	code        string
	onActivated OnActivatedFunc
}

// NewHandler creates a new activate Handler.
func NewHandler(cfg *config.Config, configPath string, onActivated OnActivatedFunc) *Handler {
	return &Handler{cfg: cfg, configPath: configPath, onActivated: onActivated}
}

// SetCode sets the one-time activation code that must be supplied as a query parameter.
func (h *Handler) SetCode(code string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.code = code
}

// Activate handles POST /node/activate?code=<code> with an AES-GCM encrypted body.
func (h *Handler) Activate(c *gin.Context) {
	// Step 1: Fast-path check for already-activated (read-only shared state).
	h.mu.Lock()
	alreadyActivated := h.cfg.NodePrivateKey != ""
	h.mu.Unlock()
	if alreadyActivated {
		c.JSON(http.StatusConflict, gin.H{"error": "already activated"})
		return
	}

	// Step 2: All I/O and computation outside the lock.
	code := c.Query("code")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty body"})
		return
	}

	aesKey := makeAESKey(code)
	plaintext, err := aesDecrypt(aesKey, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "decryption failed"})
		return
	}

	var payload activatePayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// Step 3: Lock to re-check state, validate code, then update cfg fields.
	h.mu.Lock()
	if h.cfg.NodePrivateKey != "" {
		h.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "already activated"})
		return
	}
	if code == "" || code != h.code {
		h.mu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid code"})
		return
	}
	h.cfg.AppID = payload.AppID
	h.cfg.NodePrivateKey = payload.AppPrivateKey
	h.cfg.NodePublicKey = payload.AppPublicKey
	h.cfg.HubPublicKey = payload.HubPublicKey
	if payload.HubGRPCAddr != "" {
		h.cfg.HubGRPCAddr = payload.HubGRPCAddr
	}
	h.mu.Unlock()

	// Step 3: Save config outside the lock.
	if err := config.Save(h.cfg, h.configPath); err != nil {
		// Rollback cfg fields on failure so the code remains usable for retry.
		h.mu.Lock()
		h.cfg.AppID = ""
		h.cfg.NodePrivateKey = ""
		h.cfg.NodePublicKey = ""
		h.cfg.HubPublicKey = ""
		h.mu.Unlock()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	// Step 4: Clear the one-time code only after a successful save.
	h.mu.Lock()
	h.code = ""
	h.mu.Unlock()

	// Step 5: Run post-activation callback outside the lock.
	if h.onActivated != nil {
		if err := h.onActivated(payload.AppID); err != nil {
			log.Printf("warn: post-activation init failed: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "activated"})
}

// makeAESKey derives a 32-byte AES key from the activation code using SHA-256.
// This matches the derivation used in Hub Server.
func makeAESKey(code string) []byte {
	sum := sha256.Sum256([]byte(code))
	return sum[:]
}

func aesDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
}
