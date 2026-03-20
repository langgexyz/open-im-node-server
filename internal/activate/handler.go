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

type activatePayload struct {
	NodeID         string `json:"node_id"`
	NodePrivateKey string `json:"node_private_key"`
	NodePublicKey  string `json:"node_public_key"`
	HubPublicKey   string `json:"hub_public_key"`
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
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cfg.NodePrivateKey != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "already activated"})
		return
	}

	code := c.Query("code")
	if code == "" || code != h.code {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid code"})
		return
	}

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

	h.cfg.NodeID = payload.NodeID
	h.cfg.NodePrivateKey = payload.NodePrivateKey
	h.cfg.NodePublicKey = payload.NodePublicKey
	h.cfg.HubPublicKey = payload.HubPublicKey
	if err := config.Save(h.cfg, h.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	h.code = "" // one-time use

	if h.onActivated != nil {
		if err := h.onActivated(payload.NodeID); err != nil {
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
