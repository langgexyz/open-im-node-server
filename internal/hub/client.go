package hub

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	hubv1 "github.com/langgexyz/open-im-hub-proto/hub/v1"
	bridgecrypto "github.com/langgexyz/open-im-node-server/internal/crypto"
)

// Client 是 Node 向 Hub Server 发起 gRPC 调用的客户端。
// 每次调用自动附加节点签名 metadata：
//   x-node-public-key  节点以太坊地址
//   x-node-timestamp   Unix 秒级时间戳
//   x-node-body-hash   keccak256(proto bytes) 的 hex（客户端预计算并传递）
//   x-node-sig         Sign(keccak256(full_method || 0x00 || body_hash || 0x00 || timestamp), node_private_key) 的 hex
//
// 签名有效期：Hub Server 校验 |now - timestamp| ≤ 60s（分钟级别）。
type Client struct {
	nodePublicKey string
	conn          *grpc.ClientConn
	svc           hubv1.HubServiceClient
}

// NewClient 建立到 Hub Server 的 gRPC 连接。
// hubGRPCAddr 格式：host:port（如 hub.example.com:50051）。
func NewClient(hubGRPCAddr, nodePublicKey string, nodePrivKey *ecdsa.PrivateKey) (*Client, error) {
	conn, err := grpc.NewClient(hubGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(NewNodeSignInterceptor(nodePublicKey, nodePrivKey)),
	)
	if err != nil {
		return nil, fmt.Errorf("hub grpc dial: %w", err)
	}
	return &Client{
		nodePublicKey: nodePublicKey,
		conn:          conn,
		svc:           hubv1.NewHubServiceClient(conn),
	}, nil
}

// Close 关闭 gRPC 连接
func (c *Client) Close() error { return c.conn.Close() }

// Activate 节点注册（一次性）。
// activationCode 通过 gRPC metadata x-activation-code 传递，Hub Server 对此方法跳过 node-sig 验证。
func (c *Client) Activate(ctx context.Context, activationCode, nodeWSAddr string) (appID, hubPublicKey string, err error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "x-activation-code", activationCode)
	resp, err := c.svc.Activate(ctx, &hubv1.ActivateRequest{
		NodePublicKey: c.nodePublicKey,
		NodeWsAddr:    nodeWSAddr,
	})
	if err != nil {
		return "", "", fmt.Errorf("activate: %w", err)
	}
	return resp.AppId, resp.HubPublicKey, nil
}

// Heartbeat 发送节点心跳
func (c *Client) Heartbeat(ctx context.Context, wsAddr string) error {
	_, err := c.svc.Heartbeat(ctx, &hubv1.HeartbeatRequest{
		NodePublicKey: c.nodePublicKey,
		WsAddr:        wsAddr,
	})
	return err
}

// SignSession 向 Hub Server 请求为用户签发 session_sig。
// userCredential 是 App 发来的凭证原始字符串（Bearer ...），Hub Server 验证并提取 app_uid。
// 返回 (session_sig, app_uid, error)
func (c *Client) SignSession(ctx context.Context, userCredential string, expiry int64) (string, string, error) {
	resp, err := c.svc.SignSession(ctx, &hubv1.SignSessionRequest{
		UserCredential: userCredential,
		Expiry:         expiry,
	})
	if err != nil {
		return "", "", fmt.Errorf("sign-session: %w", err)
	}
	return resp.SessionSig, resp.AppUid, nil
}

// PushNotify 分批向 Hub Server 转发离线推送
func (c *Client) PushNotify(ctx context.Context, appUIDs []string, title, body, dataJSON string) error {
	const batchSize = 1000
	for i := 0; i < len(appUIDs); i += batchSize {
		end := i + batchSize
		if end > len(appUIDs) {
			end = len(appUIDs)
		}
		_, err := c.svc.PushNotify(ctx, &hubv1.PushNotifyRequest{
			AppUids:  appUIDs[i:end],
			Title:    title,
			Body:     body,
			DataJson: dataJSON,
		})
		if err != nil {
			return fmt.Errorf("push notify: %w", err)
		}
	}
	return nil
}

// NewNodeSignInterceptor 返回一个 gRPC UnaryClientInterceptor，为所有出站调用附加节点签名 metadata。
// 签名消息：keccak256(full_method || 0x00 || body_hash || 0x00 || timestamp)
// 其中 body_hash = keccak256(proto.Marshal(req))
// 导出以便测试直接调用。
func NewNodeSignInterceptor(nodePublicKey string, nodePrivKey *ecdsa.PrivateKey) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		protoMsg, ok := req.(proto.Message)
		if !ok {
			return fmt.Errorf("request is not proto.Message")
		}
		rawBytes, err := proto.Marshal(protoMsg)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyHash := bridgecrypto.Keccak256(rawBytes)
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)

		// 签名消息：full_method || 0x00 || body_hash || 0x00 || timestamp
		sigMsg := BuildMsg([]byte(method), bodyHash, []byte(timestamp))
		sig, err := bridgecrypto.Sign(sigMsg, nodePrivKey)
		if err != nil {
			return fmt.Errorf("sign grpc request: %w", err)
		}

		ctx = metadata.AppendToOutgoingContext(ctx,
			"x-node-public-key", nodePublicKey,
			"x-node-timestamp", timestamp,
			"x-node-body-hash", hex.EncodeToString(bodyHash),
			"x-node-sig", hex.EncodeToString(sig),
		)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// BuildMsg 拼接带 0x00 分隔符的消息，防哈希碰撞。导出以便测试验证签名内容。
func BuildMsg(parts ...[]byte) []byte {
	var msg []byte
	for i, p := range parts {
		msg = append(msg, p...)
		if i < len(parts)-1 {
			msg = append(msg, 0x00)
		}
	}
	return msg
}
