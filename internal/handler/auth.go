package handler

import (
	"crypto/ecdsa"
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

// PostToken POST /auth/token
// Authorization: Bearer <user_credential>
// 流程：
//  1. 将凭证交给 Hub Server 验证，Hub Server 提取 app_uid 并签发 session_sig
//  2. 本地开户（幂等），在 OpenIM 注册用户（幂等）
//  3. 签发 user_token（含 session_sig）
func (h *AuthHandler) PostToken(c *gin.Context) {
	credStr := c.GetHeader("Authorization")

	expiry := time.Now().Add(time.Duration(h.cfg.TokenExpirySecs) * time.Second)

	// 向 Hub Server 请求 session_sig，同时验证凭证并获取 app_uid
	sessionSig, appUID, err := h.hubCli.SignSession(c.Request.Context(), credStr, expiry.Unix())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 开户或查户（幂等）
	nodeUID, err := h.accounts.GetOrCreate(appUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account: " + err.Error()})
		return
	}

	// 在 OpenIM 注册该用户（幂等）
	if err := h.openimCli.RegisterUser(c.Request.Context(), nodeUID, "user"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "register: " + err.Error()})
		return
	}

	userToken, err := token.IssueUserToken(appUID, h.cfg.AppID, nodeUID, sessionSig, h.nodePriv, expiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_token":      userToken,
		"node_public_key": h.cfg.NodePublicKey,
	})
}

// PostExchange POST /auth/exchange
func (h *AuthHandler) PostExchange(c *gin.Context) {
	var body struct {
		UserToken string `json:"user_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payload, err := token.VerifyUserToken(body.UserToken, h.cfg.NodePublicKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	openimToken, err := h.openimCli.GetUserToken(c.Request.Context(), payload.NodeUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"openim_token": openimToken, "node_uid": payload.NodeUID})
}
