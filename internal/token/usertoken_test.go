package token_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/langgexyz/open-im-node-server/internal/token"
)

func TestUserTokenRoundtrip(t *testing.T) {
	priv, addr := genKey(t)
	tok, err := token.IssueUserToken("uid_app", "app_id_123", 10001, "0xsession_sig_hex", priv, time.Now().Add(time.Hour))
	require.NoError(t, err)

	payload, err := token.VerifyUserToken(tok, addr)
	require.NoError(t, err)
	require.Equal(t, "uid_app", payload.AppUID)
	require.Equal(t, "app_id_123", payload.AppID)
	require.Equal(t, uint64(10001), payload.NodeUID)
	require.Equal(t, "0xsession_sig_hex", payload.SessionSig)
}

func TestUserTokenExpired(t *testing.T) {
	priv, addr := genKey(t)
	tok, _ := token.IssueUserToken("u", "a", 1, "0xsig", priv, time.Now().Add(-time.Minute))
	_, err := token.VerifyUserToken(tok, addr)
	require.ErrorIs(t, err, token.ErrTokenExpired)
}
