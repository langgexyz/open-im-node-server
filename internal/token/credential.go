package token

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	bridgecrypto "github.com/langgexyz/open-im-node-server/internal/crypto"
)

var (
	ErrTokenExpired  = errors.New("token expired")
	ErrInvalidSigner = errors.New("invalid token signer")
	ErrMalformed     = errors.New("malformed token")
)

type CredentialPayload struct {
	AppUID string `json:"app_uid"`
	Exp    int64  `json:"exp"`
}

func IssueCredential(appUID string, privKey *ecdsa.PrivateKey, expiry time.Time) (string, error) {
	return signPayload(CredentialPayload{AppUID: appUID, Exp: expiry.Unix()}, privKey)
}

func VerifyCredential(tokenStr, expectedSigner string) (*CredentialPayload, error) {
	payloadB64, _, err := splitToken(tokenStr)
	if err != nil {
		return nil, err
	}
	var payload CredentialPayload
	if err := decodePayload(payloadB64, &payload); err != nil {
		return nil, err
	}
	if time.Now().Unix() > payload.Exp {
		return nil, ErrTokenExpired
	}
	return &payload, verifySig(payloadB64, tokenStr, expectedSigner)
}

// --- 内部工具 ---

func signPayload(payload any, privKey *ecdsa.PrivateKey) (string, error) {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	b64 := base64.RawURLEncoding.EncodeToString(jsonBytes)
	sig, err := bridgecrypto.Sign([]byte(b64), privKey)
	if err != nil {
		return "", err
	}
	return b64 + "." + hex.EncodeToString(sig), nil
}

func splitToken(s string) (string, string, error) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return "", "", ErrMalformed
	}
	return parts[0], parts[1], nil
}

func decodePayload(b64 string, dst any) error {
	data, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	return json.Unmarshal(data, dst)
}

func verifySig(payloadB64, tokenStr, expectedSigner string) error {
	_, sigHex, _ := splitToken(tokenStr)
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("%w: invalid sig hex", ErrMalformed)
	}
	recovered, err := bridgecrypto.Ecrecover([]byte(payloadB64), sig)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSigner, err)
	}
	if !strings.EqualFold(recovered, expectedSigner) {
		return ErrInvalidSigner
	}
	return nil
}
