package hub_test

import (
	"context"
	"encoding/hex"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	hubv1 "github.com/langgexyz/open-im-hub-proto/hub/v1"
	bridgecrypto "github.com/langgexyz/open-im-node-server/internal/crypto"
	"github.com/langgexyz/open-im-node-server/internal/hub"
)

// metaCaptureServer 是一个最简 gRPC 服务，捕获每次调用的 incoming metadata。
type metaCaptureServer struct {
	hubv1.UnimplementedHubServiceServer
	captured metadata.MD
}

func (s *metaCaptureServer) Heartbeat(ctx context.Context, req *hubv1.HeartbeatRequest) (*hubv1.HeartbeatResponse, error) {
	s.captured, _ = metadata.FromIncomingContext(ctx)
	return &hubv1.HeartbeatResponse{}, nil
}

// startCaptureServer 在随机端口启动 gRPC，返回地址和服务实例。
func startCaptureServer(t *testing.T) (addr string, svc *metaCaptureServer) {
	t.Helper()
	svc = &metaCaptureServer{}
	srv := grpc.NewServer()
	hubv1.RegisterHubServiceServer(srv, svc)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { srv.Stop() })
	go srv.Serve(lis) //nolint:errcheck
	return lis.Addr().String(), svc
}

// TestNodeSignInterceptorAttachesMetadata 验证拦截器在每次 RPC 调用中附加了所有必要 metadata，
// 且签名可通过 ecrecover 还原出正确的节点公钥地址。
func TestNodeSignInterceptorAttachesMetadata(t *testing.T) {
	privKey, pubKey, err := bridgecrypto.GenerateKey()
	require.NoError(t, err)

	addr, svc := startCaptureServer(t)

	cli, err := hub.NewClient(addr, pubKey, privKey)
	require.NoError(t, err)
	defer cli.Close()

	err = cli.Heartbeat(context.Background(), "ws://test:8080")
	require.NoError(t, err)

	md := svc.captured
	require.NotEmpty(t, md.Get("x-node-public-key"), "x-node-public-key missing")
	require.NotEmpty(t, md.Get("x-node-timestamp"), "x-node-timestamp missing")
	require.NotEmpty(t, md.Get("x-node-body-hash"), "x-node-body-hash missing")
	require.NotEmpty(t, md.Get("x-node-sig"), "x-node-sig missing")

	require.Equal(t, pubKey, md.Get("x-node-public-key")[0])

	// 验证签名可还原出正确公钥
	// Sign(sigMsg) 内部做 keccak256(sigMsg)，Ecrecover 也内部做 keccak256，故直接传 sigMsg
	tsStr := md.Get("x-node-timestamp")[0]
	bodyHashHex := md.Get("x-node-body-hash")[0]
	sigHex := md.Get("x-node-sig")[0]

	bodyHash, err := hex.DecodeString(bodyHashHex)
	require.NoError(t, err)
	sig, err := hex.DecodeString(sigHex)
	require.NoError(t, err)

	const method = "/hub.v1.HubService/Heartbeat"
	sigMsg := hub.BuildMsg([]byte(method), bodyHash, []byte(tsStr))

	// bridgecrypto.Ecrecover 内部对 sigMsg 做 keccak256 后恢复公钥
	recovered, err := bridgecrypto.Ecrecover(sigMsg, sig)
	require.NoError(t, err)
	require.Equal(t, pubKey, recovered)
}

// TestNodeSignInterceptorTimestamp 验证 timestamp 在调用发生的 1 秒内。
func TestNodeSignInterceptorTimestamp(t *testing.T) {
	privKey, pubKey, err := bridgecrypto.GenerateKey()
	require.NoError(t, err)

	addr, svc := startCaptureServer(t)

	cli, err := hub.NewClient(addr, pubKey, privKey)
	require.NoError(t, err)
	defer cli.Close()

	before := time.Now().Unix()
	err = cli.Heartbeat(context.Background(), "ws://test:8080")
	require.NoError(t, err)
	after := time.Now().Unix()

	tsStr := svc.captured.Get("x-node-timestamp")[0]
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	require.NoError(t, err)

	require.GreaterOrEqual(t, ts, before)
	require.LessOrEqual(t, ts, after+1)
}
