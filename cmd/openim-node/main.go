package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/langgexyz/open-im-node-server/internal/config"
	"github.com/langgexyz/open-im-node-server/internal/handler"
	"github.com/langgexyz/open-im-node-server/internal/server"
)

func main() {
	activateCode := flag.String("activate", "", "一次性激活码")
	configPath := flag.String("config", config.DefaultConfigPath, "配置文件路径")
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	flag.Parse()

	if *activateCode != "" {
		cfg, err := handler.Activate(handler.ActivateParams{
			ActivationCode:   *activateCode,
			HubGRPCAddr:      requireEnv("HUB_GRPC_ADDR"),
			OpenIMAPIAddr:    requireEnv("OPENIM_API_ADDR"),
			OpenIMAdminToken: requireEnv("OPENIM_ADMIN_TOKEN"),
			NodeWSAddr:       requireEnv("NODE_WS_ADDR"),
			MySQLDSN:         requireEnv("MYSQL_DSN"),
			ConfigPath:       *configPath,
		})
		if err != nil {
			log.Fatalf("激活失败: %v", err)
		}
		fmt.Printf("激活成功！AppID: %s\n", cfg.AppID)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		// Config file may not exist before first activation; start with empty config.
		log.Printf("配置文件未找到或读取失败 (%v)，以未激活模式启动", err)
		cfg = &config.NodeConfig{}
	}

	srv, err := server.New(cfg, *configPath)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 启动心跳：立即发送一次，之后每 30s 一次（仅激活后）
	if cfg.Activated() {
		go runHeartbeat(srv)
	}

	log.Printf("Bridge 启动 %s，AppID: %s", *addr, cfg.AppID)
	if err := srv.Engine.Run(*addr); err != nil {
		log.Fatalf("服务器错误: %v", err)
	}
}

func runHeartbeat(srv *server.Server) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := srv.HubCli.Heartbeat(ctx, srv.Cfg.NodeWSAddr); err != nil {
			log.Printf("heartbeat failed: %v", err)
		}
		cancel()
		time.Sleep(30 * time.Second)
	}
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("环境变量 %s 未设置", key)
	}
	return v
}
