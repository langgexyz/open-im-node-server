package server

import (
	"database/sql"
	"fmt"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/langgexyz/open-im-node-server/internal/config"
	bridgecrypto "github.com/langgexyz/open-im-node-server/internal/crypto"
	"github.com/langgexyz/open-im-node-server/internal/handler"
	"github.com/langgexyz/open-im-node-server/internal/hub"
	"github.com/langgexyz/open-im-node-server/internal/openim"
	"github.com/langgexyz/open-im-node-server/internal/store"
)

type Server struct {
	Engine *gin.Engine
	HubCli *hub.Client
	Cfg    *config.NodeConfig
}

func New(cfg *config.NodeConfig) (*Server, error) {
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
	hubCli, err := hub.NewClient(cfg.HubGRPCAddr, cfg.NodePublicKey, privKey)
	if err != nil {
		return nil, fmt.Errorf("init hub grpc client: %w", err)
	}

	authH, err := handler.NewAuthHandler(cfg, accounts, openimCli, hubCli)
	if err != nil {
		return nil, fmt.Errorf("init auth handler: %w", err)
	}
	// channel_group_id 固定等于 app_id（激活时创建）
	webhookH := handler.NewWebhookHandler(accounts, hubCli, openimCli, cfg.AppID)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/node/info", handler.NodeInfo(cfg))
	r.POST("/auth/token", authH.PostToken)
	r.POST("/auth/exchange", authH.PostExchange)
	r.POST("/internal/after-group-msg", webhookH.AfterGroupMsg)

	return &Server{Engine: r, HubCli: hubCli, Cfg: cfg}, nil
}
