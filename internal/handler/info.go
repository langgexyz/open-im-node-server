package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/langgexyz/open-im-node-server/internal/config"
)

func NodeInfo(cfg *config.NodeConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"app_id":          cfg.AppID,
			"node_public_key": cfg.NodePublicKey,
			"node_ws_addr":    cfg.NodeWSAddr,
		})
	}
}
