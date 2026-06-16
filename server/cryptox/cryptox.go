// Package cryptox provides authenticated symmetric encryption for secret
// settings (such as the AI API key) so they are not stored in plaintext.
package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const prefix = "enc::"

func keyFrom(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Encrypt returns a base64 AES-256-GCM ciphertext tagged with a prefix so we
// can detect already-encrypted values. Empty input yields empty output.
func Encrypt(secret, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(keyFrom(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt. Values without the prefix are returned unchanged
// (treated as legacy plaintext) so the system stays robust during migration.
func Decrypt(secret, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(keyFrom(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// IsEncrypted reports whether the value carries the encryption prefix.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, prefix)
}
