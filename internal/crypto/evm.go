package crypto

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// GenerateKey 生成 secp256k1 密钥对，返回私钥和以太坊地址（小写，含 0x 前缀）
func GenerateKey() (*ecdsa.PrivateKey, string, error) {
	privKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, "", err
	}
	addr, err := PrivKeyToAddress(privKey)
	if err != nil {
		return nil, "", err
	}
	return privKey, addr, nil
}

// PrivKeyToAddress 从私钥派生以太坊地址（小写，含 0x 前缀）
func PrivKeyToAddress(privKey *ecdsa.PrivateKey) (string, error) {
	pubKey, ok := privKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("invalid public key type")
	}
	return strings.ToLower(crypto.PubkeyToAddress(*pubKey).Hex()), nil
}

// Keccak256 计算多个字节片段拼接的 keccak256
func Keccak256(data ...[]byte) []byte {
	return crypto.Keccak256(data...)
}

// Sign 对消息取 keccak256 后用私钥签名，返回 65 字节（r+s+v）
func Sign(message []byte, privKey *ecdsa.PrivateKey) ([]byte, error) {
	return crypto.Sign(Keccak256(message), privKey)
}

// Ecrecover 从消息和签名恢复签名者地址（小写，含 0x 前缀）
func Ecrecover(message, sig []byte) (string, error) {
	if len(sig) != 65 {
		return "", fmt.Errorf("invalid signature length: %d", len(sig))
	}
	pubKeyBytes, err := crypto.Ecrecover(Keccak256(message), sig)
	if err != nil {
		return "", err
	}
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		return "", err
	}
	return strings.ToLower(crypto.PubkeyToAddress(*pubKey).Hex()), nil
}

// PrivKeyToHex 私钥序列化为十六进制（无 0x）
func PrivKeyToHex(privKey *ecdsa.PrivateKey) string {
	return hex.EncodeToString(crypto.FromECDSA(privKey))
}

// PrivKeyFromHex 从十六进制字符串（可含 0x 前缀）解码私钥
func PrivKeyFromHex(hexKey string) (*ecdsa.PrivateKey, error) {
	return crypto.HexToECDSA(strings.TrimPrefix(hexKey, "0x"))
}
