package token_test

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	bridgecrypto "github.com/langgexyz/open-im-node-server/internal/crypto"
	"github.com/langgexyz/open-im-node-server/internal/token"
)

func genKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	priv, addr, err := bridgecrypto.GenerateKey()
	require.NoError(t, err)
	return priv, addr
}

func TestCredentialRoundtrip(t *testing.T) {
	priv, addr := genKey(t)
	cred, err := token.IssueCredential("user_abc", priv, time.Now().Add(time.Hour))
	require.NoError(t, err)

	payload, err := token.VerifyCredential(cred, addr)
	require.NoError(t, err)
	require.Equal(t, "user_abc", payload.AppUID)
}

func TestCredentialExpired(t *testing.T) {
	priv, addr := genKey(t)
	cred, _ := token.IssueCredential("user_abc", priv, time.Now().Add(-time.Hour))
	_, err := token.VerifyCredential(cred, addr)
	require.ErrorIs(t, err, token.ErrTokenExpired)
}

func TestCredentialWrongSigner(t *testing.T) {
	priv, _ := genKey(t)
	_, wrongAddr := genKey(t)
	cred, _ := token.IssueCredential("user_abc", priv, time.Now().Add(time.Hour))
	_, err := token.VerifyCredential(cred, wrongAddr)
	require.ErrorIs(t, err, token.ErrInvalidSigner)
}
