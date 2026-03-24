package handler

import (
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/langgexyz/open-im-node-server/internal/config"
	bridgecrypto "github.com/langgexyz/open-im-node-server/internal/crypto"
	"github.com/langgexyz/open-im-node-server/internal/hub"
	"github.com/langgexyz/open-im-node-server/internal/openim"
	"github.com/langgexyz/open-im-node-server/internal/store"
	"github.com/langgexyz/open-im-node-server/internal/token"
)

type AuthHandler struct {
	cfg       *config.NodeConfig
	accounts  *store.Accounts
	openimCli *openim.Client
	hubCli    *hub.Client
	nodePriv  *ecdsa.PrivateKey
}

func NewAuthHandler(cfg *config.NodeConfig, accounts *store.Accounts, openimCli *openim.Client, hubCli *hub.Client) (*AuthHandler, error) {
	priv, err := bridgecrypto.PrivKeyFromHex(cfg.NodePrivateKey)
	if err != nil {
		return nil, err
	}
	return &AuthHandler{cfg: cfg, accounts: accounts, openimCli: openimCli, hubCli: hubCli, nodePriv: priv}, nil
}

// PostToken POST /auth/token { credential }
// 流程：
//  1. 将凭证交给 Hub Server 验证，Hub Server 提取 app_uid 并签发 session_sig
//  2. 本地开户（幂等），在 OpenIM 注册用户（幂等）
//  3. 签发 app_token（含 session_sig）
func (h *AuthHandler) PostToken(c *gin.Context) {
	var body struct {
		Credential string `json:"credential" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	expiry := time.Now().Add(time.Duration(h.cfg.TokenExpirySecs) * time.Second)

	sessionSig, appUID, err := h.hubCli.SignSession(c.Request.Context(), body.Credential, expiry.Unix())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	nodeUID, err := h.accounts.GetOrCreate(appUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account: " + err.Error()})
		return
	}

	if err := h.openimCli.RegisterUser(c.Request.Context(), nodeUID, "user"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "register: " + err.Error()})
		return
	}

	appToken, err := token.IssueUserToken(appUID, h.cfg.AppID, nodeUID, sessionSig, h.nodePriv, expiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"app_token": appToken,
		"app_uid":   appUID,
	})
}

// PostExchange POST /auth/exchange { app_token }
// 返回 OpenIM 连接所需的全部参数（供前端 SDK 初始化）
func (h *AuthHandler) PostExchange(c *gin.Context) {
	var body struct {
		AppToken string `json:"app_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payload, err := token.VerifyUserToken(body.AppToken, h.cfg.NodePublicKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	openimToken, err := h.openimCli.GetUserToken(c.Request.Context(), payload.NodeUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"openim_token":    openimToken,
		"openim_api_addr": h.cfg.OpenIMAPIAddr,
		"openim_ws_addr":  h.cfg.OpenIMWSAddr,
		"user_id":         fmt.Sprintf("%d", payload.NodeUID),
		"group_id":        "0",
	})
}
