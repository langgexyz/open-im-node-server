package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	bridgecrypto "github.com/langgexyz/open-im-node-server/internal/crypto"
)

func TestSignAndRecover(t *testing.T) {
	privKey, addr, err := bridgecrypto.GenerateKey()
	require.NoError(t, err)
	require.NotEmpty(t, addr)

	msg := []byte("hello bridge")
	sig, err := bridgecrypto.Sign(msg, privKey)
	require.NoError(t, err)
	require.Len(t, sig, 65)

	recovered, err := bridgecrypto.Ecrecover(msg, sig)
	require.NoError(t, err)
	require.Equal(t, addr, recovered)
}

func TestKeccak256Separator(t *testing.T) {
	h1 := bridgecrypto.Keccak256([]byte("ab"))
	h2 := bridgecrypto.Keccak256([]byte("a"), []byte{0x00}, []byte("b"))
	require.NotEqual(t, h1, h2)
}

func TestPrivKeyHexRoundtrip(t *testing.T) {
	privKey, addr, _ := bridgecrypto.GenerateKey()
	hexKey := bridgecrypto.PrivKeyToHex(privKey)

	restored, err := bridgecrypto.PrivKeyFromHex(hexKey)
	require.NoError(t, err)

	restoredAddr, err := bridgecrypto.PrivKeyToAddress(restored)
	require.NoError(t, err)
	require.Equal(t, addr, restoredAddr)
}
