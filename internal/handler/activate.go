package handler

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/langgexyz/open-im-node-server/internal/config"
	bridgecrypto "github.com/langgexyz/open-im-node-server/internal/crypto"
	"github.com/langgexyz/open-im-node-server/internal/hub"
	"github.com/langgexyz/open-im-node-server/internal/openim"
	"github.com/langgexyz/open-im-node-server/internal/store"
)

type ActivateParams struct {
	ActivationCode   string
	HubGRPCAddr      string // Hub Server gRPC 地址，如 hub.example.com:50051
	OpenIMAPIAddr    string
	OpenIMAdminToken string
	NodeWSAddr       string
	MySQLDSN         string
	ConfigPath       string
}

func Activate(p ActivateParams) (*config.NodeConfig, error) {
	privKey, pubKey, err := bridgecrypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 通过 gRPC 调用 Hub Server Activate（Hub Server 对此方法跳过 node-sig，验证激活码）
	hubCli, err := hub.NewClient(p.HubGRPCAddr, pubKey, privKey)
	if err != nil {
		return nil, fmt.Errorf("hub grpc connect: %w", err)
	}
	defer hubCli.Close()

	appID, hubPublicKey, err := hubCli.Activate(ctx, p.ActivationCode, p.NodeWSAddr)
	if err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}

	// 初始化 MySQL，预插入运营者账号
	db, err := sql.Open("mysql", p.MySQLDSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()

	accounts, err := store.NewAccounts(db)
	if err != nil {
		return nil, fmt.Errorf("init accounts: %w", err)
	}
	ownerUID, err := accounts.InsertOwner()
	if err != nil {
		return nil, fmt.Errorf("insert owner: %w", err)
	}

	// 在 OpenIM 创建频道群组（group_id = app_id）并注册运营者账号
	openimCli := openim.NewClient(p.OpenIMAPIAddr, p.OpenIMAdminToken)
	if err := openimCli.RegisterUser(ctx, ownerUID, "node-owner"); err != nil {
		return nil, fmt.Errorf("register owner in openim: %w", err)
	}
	ownerIDStr := strconv.FormatUint(ownerUID, 10)
	if err := openimCli.CreateGroup(ctx, appID, ownerIDStr); err != nil {
		return nil, fmt.Errorf("create channel group: %w", err)
	}

	cfg := &config.NodeConfig{
		AppID:            appID,
		NodePublicKey:    pubKey,
		NodePrivateKey:   bridgecrypto.PrivKeyToHex(privKey),
		HubPublicKey:     hubPublicKey,
		OpenIMAdminToken: p.OpenIMAdminToken,
		OpenIMAPIAddr:    p.OpenIMAPIAddr,
		HubGRPCAddr:      p.HubGRPCAddr,
		NodeWSAddr:       p.NodeWSAddr,
		MySQLDSN:         p.MySQLDSN,
		TokenExpirySecs:  86400,
	}
	return cfg, config.Save(cfg, p.ConfigPath)
}
