package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const prefix = "enc:v1:"

// Cipher encrypts and decrypts strings using AES-256-GCM.
type Cipher struct {
	key []byte
}

// New creates a Cipher from a 32-byte key.
func New(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, errors.New("secret: key must be exactly 32 bytes for AES-256")
	}
	k := make([]byte, 32)
	copy(k, key)
	return &Cipher{key: k}, nil
}

// Encrypt encrypts plaintext and returns a prefixed base64 string.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(c.key)
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
	// Seal appends ciphertext+tag to nonce
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a string produced by Encrypt. If the value is not
// prefixed (legacy plaintext), it is returned as-is.
func (c *Cipher) Decrypt(s string) (string, error) {
	if !strings.HasPrefix(s, prefix) {
		return s, nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, prefix))
	if err != nil {
		return "", errors.New("secret: invalid base64")
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("secret: ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("secret: decryption failed (wrong key?)")
	}
	return string(plaintext), nil
}
