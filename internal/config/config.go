package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DefaultConfigPath = "/data/config.json"

type NodeConfig struct {
	AppID            string `json:"app_id"`
	NodePublicKey    string `json:"node_public_key"`
	NodePrivateKey   string `json:"node_private_key"`   // hex，无 0x 前缀
	HubPublicKey     string `json:"hub_public_key"`     // Hub Server 公钥，激活时自动写入，用于验证 user_credential
	OpenIMAdminToken string `json:"openim_admin_token"`
	OpenIMAPIAddr    string `json:"openim_api_addr"`
	HubGRPCAddr      string `json:"hub_grpc_addr"`      // Hub Server gRPC 地址，如 hub.example.com:50051
	NodeWSAddr       string `json:"node_ws_addr"`
	MySQLDSN         string `json:"mysql_dsn"`
	TokenExpirySecs  int64  `json:"token_expiry_secs"`  // 默认 86400
}

func Load(path string) (*NodeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg NodeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.TokenExpirySecs <= 0 {
		cfg.TokenExpirySecs = 86400
	}
	return &cfg, nil
}

func Save(cfg *NodeConfig, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
