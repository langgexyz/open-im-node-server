package openim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	baseURL    string
	adminToken string
	http       *http.Client
}

func NewClient(baseURL, adminToken string) *Client {
	return &Client{baseURL: baseURL, adminToken: adminToken, http: &http.Client{Timeout: 10 * time.Second}}
}

type apiResp struct {
	ErrCode int             `json:"errCode"`
	ErrMsg  string          `json:"errMsg"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) post(ctx context.Context, path string, body []byte, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", c.adminToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("openim http: %w", err)
	}
	defer resp.Body.Close()
	var result apiResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if dst != nil && len(result.Data) > 0 {
		if err := json.Unmarshal(result.Data, dst); err != nil {
			return fmt.Errorf("decode data: %w", err)
		}
	}
	// errCode 非零由调用方检查
	if result.ErrCode != 0 {
		// 返回结构体让调用方检查
		if code, ok := dst.(*apiResp); ok {
			*code = result
		}
	}
	// Store the full response for callers that need errCode
	if r, ok := dst.(*apiResp); ok {
		*r = result
	}
	return nil
}

func (c *Client) postRaw(ctx context.Context, path string, body []byte) (*apiResp, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", c.adminToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openim http: %w", err)
	}
	defer resp.Body.Close()
	var result apiResp
	return &result, json.NewDecoder(resp.Body).Decode(&result)
}

// RegisterUser 注册用户（幂等，errCode 10002 = 已存在，忽略）
func (c *Client) RegisterUser(ctx context.Context, nodeUID uint64, nickname string) error {
	body, _ := json.Marshal(map[string]any{
		"users": []map[string]any{
			{"userID": strconv.FormatUint(nodeUID, 10), "nickname": nickname},
		},
	})
	resp, err := c.postRaw(ctx, "/user/user_register", body)
	if err != nil {
		return err
	}
	if resp.ErrCode != 0 && resp.ErrCode != 10002 {
		return fmt.Errorf("register user: %s (code %d)", resp.ErrMsg, resp.ErrCode)
	}
	return nil
}

type tokenData struct {
	Token string `json:"token"`
}

// GetUserToken 以 Admin 身份获取指定用户的 OpenIM token
func (c *Client) GetUserToken(ctx context.Context, nodeUID uint64) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"userID":     strconv.FormatUint(nodeUID, 10),
		"platformID": 1,
	})
	resp, err := c.postRaw(ctx, "/auth/get_user_token", body)
	if err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("get user token: %s (code %d)", resp.ErrMsg, resp.ErrCode)
	}
	var data tokenData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}
	return data.Token, nil
}

type memberData struct {
	Members []struct {
		UserID string `json:"userID"`
	} `json:"members"`
}

// GetGroupMemberNodeUIDs 获取群组所有成员的 node_uid（uint64）
func (c *Client) GetGroupMemberNodeUIDs(ctx context.Context, groupID string) ([]uint64, error) {
	body, _ := json.Marshal(map[string]any{
		"groupID":    groupID,
		"pagination": map[string]any{"pageNumber": 1, "showNumber": 10000},
	})
	resp, err := c.postRaw(ctx, "/group/get_group_member_list", body)
	if err != nil {
		return nil, err
	}
	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("get group members: %s (code %d)", resp.ErrMsg, resp.ErrCode)
	}
	var data memberData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("decode members: %w", err)
	}
	ids := make([]uint64, 0, len(data.Members))
	for _, m := range data.Members {
		id, err := strconv.ParseUint(m.UserID, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type onlineData struct {
	StatusList []struct {
		UserID      string `json:"userID"`
		OnlineState int32  `json:"onlineState"`
	} `json:"statusList"`
}

// GetOfflineNodeUIDs 在给定 nodeUID 列表中返回当前离线的 node_uid
func (c *Client) GetOfflineNodeUIDs(ctx context.Context, nodeUIDs []uint64) ([]uint64, error) {
	if len(nodeUIDs) == 0 {
		return nil, nil
	}
	userIDs := make([]string, len(nodeUIDs))
	for i, id := range nodeUIDs {
		userIDs[i] = strconv.FormatUint(id, 10)
	}
	body, _ := json.Marshal(map[string]any{"userIDs": userIDs})
	resp, err := c.postRaw(ctx, "/user/get_users_online_status", body)
	if err != nil {
		return nil, err
	}
	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("get online status: %s (code %d)", resp.ErrMsg, resp.ErrCode)
	}
	var data onlineData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("decode status: %w", err)
	}
	var offline []uint64
	for _, s := range data.StatusList {
		if s.OnlineState == 0 {
			id, _ := strconv.ParseUint(s.UserID, 10, 64)
			offline = append(offline, id)
		}
	}
	return offline, nil
}

// CreateGroup 创建群组（激活时调用，group_id = app_id，owner = 运营者 node_uid）
func (c *Client) CreateGroup(ctx context.Context, groupID, ownerUserID string) error {
	body, _ := json.Marshal(map[string]any{
		"groupInfo": map[string]any{
			"groupID":   groupID,
			"groupName": groupID,
			"groupType": 2,
		},
		"memberUserIDs": []string{},
		"ownerUserID":   ownerUserID,
	})
	resp, err := c.postRaw(ctx, "/group/create_group", body)
	if err != nil {
		return err
	}
	if resp.ErrCode != 0 && resp.ErrCode != 10006 {
		return fmt.Errorf("create group: %s (code %d)", resp.ErrMsg, resp.ErrCode)
	}
	return nil
}
