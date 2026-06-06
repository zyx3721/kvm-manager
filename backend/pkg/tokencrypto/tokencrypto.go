package tokencrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const prefix = "v1:"

func Seal(secret string, token string) (string, error) {
	if strings.TrimSpace(token) == "" {
		return "", nil
	}
	gcm, err := cipherForSecret(secret)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("create token nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(token), nil)
	return prefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func Open(secret string, ciphertext string) (string, error) {
	ciphertext = strings.TrimSpace(ciphertext)
	if ciphertext == "" {
		return "", nil
	}
	if !strings.HasPrefix(ciphertext, prefix) {
		return "", fmt.Errorf("unsupported token ciphertext format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, prefix))
	if err != nil {
		return "", fmt.Errorf("decode token ciphertext: %w", err)
	}
	gcm, err := cipherForSecret(secret)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("token ciphertext is too short")
	}
	nonce := payload[:gcm.NonceSize()]
	body := payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("open token ciphertext: %w", err)
	}
	return string(plain), nil
}

func cipherForSecret(secret string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create token cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create token gcm: %w", err)
	}
	return gcm, nil
}
