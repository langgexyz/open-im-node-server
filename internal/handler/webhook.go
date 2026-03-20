package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/langgexyz/open-im-node-server/internal/hub"
	"github.com/langgexyz/open-im-node-server/internal/openim"
	"github.com/langgexyz/open-im-node-server/internal/store"
)

type WebhookHandler struct {
	accounts  *store.Accounts
	hubCli    *hub.Client
	openimCli *openim.Client
	groupID   string // 频道群组 ID（= app_id）
}

func NewWebhookHandler(accounts *store.Accounts, hubCli *hub.Client, openimCli *openim.Client, groupID string) *WebhookHandler {
	return &WebhookHandler{accounts: accounts, hubCli: hubCli, openimCli: openimCli, groupID: groupID}
}

func (h *WebhookHandler) AfterGroupMsg(c *gin.Context) {
	var body struct {
		GroupID string `json:"groupID"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{})

	// 使用独立 context，不依赖已结束的 HTTP 请求 context
	go h.dispatchPush(context.Background(), body.GroupID, body.Content)
}

func (h *WebhookHandler) dispatchPush(ctx context.Context, groupID, content string) {
	allNodeUIDs, err := h.openimCli.GetGroupMemberNodeUIDs(ctx, groupID)
	if err != nil {
		log.Printf("webhook: get members error: %v", err)
		return
	}

	offlineNodeUIDs, err := h.openimCli.GetOfflineNodeUIDs(ctx, allNodeUIDs)
	if err != nil {
		log.Printf("webhook: get offline error: %v", err)
		return
	}
	if len(offlineNodeUIDs) == 0 {
		return
	}

	appUIDMap, err := h.accounts.GetAppUIDs(offlineNodeUIDs)
	if err != nil {
		log.Printf("webhook: accounts lookup error: %v", err)
		return
	}

	appUIDs := make([]string, 0, len(appUIDMap))
	for _, v := range appUIDMap {
		appUIDs = append(appUIDs, v)
	}
	if len(appUIDs) == 0 {
		return
	}

	if err := h.hubCli.PushNotify(ctx, appUIDs, "新内容", content, `{"group_id":"`+groupID+`"}`); err != nil {
		log.Printf("webhook: push error: %v", err)
	}
}
