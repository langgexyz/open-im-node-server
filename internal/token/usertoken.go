package token

import (
	"crypto/ecdsa"
	"time"
)

// UserTokenPayload 节点签发的用户 Token
// node_uid 为 uint64（accounts 表自增 id，同时是 OpenIM userID）
// session_sig 由 Hub Server 动态签发，绑定 node_public_key + app_uid，App 端验证
type UserTokenPayload struct {
	AppUID     string `json:"app_uid"`
	AppID      string `json:"app_id"`
	NodeUID    uint64 `json:"node_uid"`
	SessionSig string `json:"session_sig"`
	Exp        int64  `json:"exp"`
}

func IssueUserToken(appUID, appID string, nodeUID uint64, sessionSig string, privKey *ecdsa.PrivateKey, expiry time.Time) (string, error) {
	return signPayload(UserTokenPayload{
		AppUID:     appUID,
		AppID:      appID,
		NodeUID:    nodeUID,
		SessionSig: sessionSig,
		Exp:        expiry.Unix(),
	}, privKey)
}

func VerifyUserToken(tokenStr, expectedSigner string) (*UserTokenPayload, error) {
	payloadB64, _, err := splitToken(tokenStr)
	if err != nil {
		return nil, err
	}
	var payload UserTokenPayload
	if err := decodePayload(payloadB64, &payload); err != nil {
		return nil, err
	}
	if time.Now().Unix() > payload.Exp {
		return nil, ErrTokenExpired
	}
	return &payload, verifySig(payloadB64, tokenStr, expectedSigner)
}
