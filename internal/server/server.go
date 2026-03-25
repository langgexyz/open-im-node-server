package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/langgexyz/open-im-node-server/internal/activate"
	"github.com/langgexyz/open-im-node-server/internal/config"
	bridgecrypto "github.com/langgexyz/open-im-node-server/internal/crypto"
	"github.com/langgexyz/open-im-node-server/internal/gateway"
	"github.com/langgexyz/open-im-node-server/internal/handler"
	"github.com/langgexyz/open-im-node-server/internal/hub"
	"github.com/langgexyz/open-im-node-server/internal/openim"
	"github.com/langgexyz/open-im-node-server/internal/registry"
	"github.com/langgexyz/open-im-node-server/internal/store"
	"github.com/langgexyz/open-im-node-server/internal/token"
)

// generateCode returns a 64-character hex activation code backed by 32 random bytes.
func generateCode() string {
	b := make([]byte, 32)
	rand.Read(b) //nolint:errcheck // crypto/rand never returns an error on supported platforms
	return fmt.Sprintf("%x", b)
}

// Server holds the HTTP engine and any long-lived clients.
type Server struct {
	Engine *gin.Engine
	HubCli *hub.Client
	Cfg    *config.NodeConfig
}

// New builds the Gin engine, always exposing:
//
//	POST /node/activate  — one-time activation endpoint
//	GET  /node/info      — probe/health (activated bool)
//
// When cfg.Activated() is true the server additionally wires:
//
//	etcd registry watch (goroutine)
//	GET|POST|… /biz/*path — AuthMiddleware + ProxyHandler
//	  (falls back to 503 handler when etcd is unavailable)
//
// A fresh 64-char hex activation code is generated and printed to the log every
// time the server starts while the node is not yet activated.
func New(cfg *config.NodeConfig, configPath string) (*Server, error) {
	r := gin.New()
	r.Use(gin.Recovery())

	// ── /node/info (always exposed) ──────────────────────────────────────────
	r.GET("/node/info", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"activated": cfg.Activated(),
			"hub_web_origin": cfg.HubWebOrigin,
			"app_id":    cfg.AppID,
		})
	})

	// ── /node/activate (always exposed) ─────────────────────────────────────
	// onActivated: 激活完成后注册运营者 OpenIM 账号，并创建订阅群 group_id="0"
	// 注意：cfg.OpenIMAPIAddr / cfg.OpenIMAdminToken 由运营者在激活前写入 config.json
	onActivated := func(nodeID string) error {
		if cfg.OpenIMAPIAddr == "" || cfg.OpenIMAdminToken == "" {
			log.Printf("warn: openim not configured, skipping post-activation init")
			return nil
		}
		cli := openim.NewClient(cfg.OpenIMAPIAddr, cfg.OpenIMAdminToken)
		ctx := context.Background()
		// 注册运营者账号（uid=0，幂等）
		if err := cli.RegisterUser(ctx, 0, "operator"); err != nil {
			log.Printf("warn: register operator user: %v", err)
		}
		// 创建订阅群 group_id="0"，owner="0"（幂等）
		if err := cli.CreateGroup(ctx, "0", "0"); err != nil {
			return fmt.Errorf("create subscription group: %w", err)
		}
		log.Printf("post-activation init done: node_id=%s", nodeID)
		return nil
	}

	activateH := activate.NewHandler(cfg, configPath, onActivated)
	if !cfg.Activated() {
		code := generateCode()
		activateH.SetCode(code)
		log.Printf("activation code: %s", code)
	}
	r.POST("/node/activate", activateH.Activate)

	// ── activated-only routes ────────────────────────────────────────────────
	var hubCli *hub.Client
	if cfg.Activated() {
		var err error

		privKey, err := bridgecrypto.PrivKeyFromHex(cfg.NodePrivateKey)
		if err != nil {
			return nil, fmt.Errorf("load node private key: %w", err)
		}

		db, err := sql.Open("mysql", cfg.MySQLDSN)
		if err != nil {
			return nil, fmt.Errorf("open mysql: %w", err)
		}
		accounts, err := store.NewAccounts(db)
		if err != nil {
			return nil, fmt.Errorf("init accounts: %w", err)
		}

		openimCli := openim.NewClient(cfg.OpenIMAPIAddr, cfg.OpenIMAdminToken)
		hubCli, err = hub.NewClient(cfg.HubGRPCAddr, cfg.NodePublicKey, privKey)
		if err != nil {
			return nil, fmt.Errorf("init hub grpc client: %w", err)
		}

		authH, err := handler.NewAuthHandler(cfg, accounts, openimCli, hubCli)
		if err != nil {
			return nil, fmt.Errorf("init auth handler: %w", err)
		}
		webhookH := handler.NewWebhookHandler(accounts, hubCli, openimCli, "0")

		r.POST("/auth/token", authH.PostToken)
		r.POST("/auth/exchange", authH.PostExchange)
		r.POST("/internal/after-group-msg", webhookH.AfterGroupMsg)

		// ── etcd registry + /biz/* gateway ──────────────────────────────────
		reg := registry.New()
		bizGroup := r.Group("/biz")

		verifier := token.NewVerifier(cfg.NodePublicKey)

		etcdClient, etcdErr := registry.NewEtcdClient(cfg.ETCDAddr)
		if etcdErr != nil {
			log.Printf("warn: etcd unavailable (%v); /biz/* will return 503", etcdErr)
			bizGroup.Any("/*path", func(c *gin.Context) {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service discovery unavailable"})
			})
		} else {
			// Start background watch; context is tied to process lifetime.
			go reg.Watch(context.Background(), etcdClient)

			bizGroup.Use(gateway.AuthMiddleware(verifier))
			bizGroup.Any("/*path", gateway.ProxyHandler(reg))
		}
	}

	return &Server{Engine: r, HubCli: hubCli, Cfg: cfg}, nil
}
