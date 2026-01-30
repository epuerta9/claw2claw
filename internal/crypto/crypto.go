// Package crypto provides end-to-end encryption for claw2claw using AES-256-GCM
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	KeySize   = 32 // AES-256
	NonceSize = 12 // GCM standard nonce size
)

var (
	ErrInvalidKey       = errors.New("invalid key size")
	ErrDecryptionFailed = errors.New("decryption failed: authentication error")
	ErrInvalidCiphertext = errors.New("ciphertext too short")
)

// DeriveKey derives an AES-256 key from shared PAKE secret using HKDF
func DeriveKey(sharedSecret []byte, salt []byte, info string) ([]byte, error) {
	hash := sha256.New
	hkdfReader := hkdf.New(hash, sharedSecret, salt, []byte(info))

	key := make([]byte, KeySize)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with the provided key
// Returns: nonce || ciphertext || tag
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Seal appends ciphertext + tag to nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM with the provided key
// Expects input format: nonce || ciphertext || tag
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}

	if len(ciphertext) < NonceSize {
		return nil, ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := ciphertext[:NonceSize]
	encryptedData := ciphertext[NonceSize:]

	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// GenerateRandom generates cryptographically secure random bytes
func GenerateRandom(size int) ([]byte, error) {
	bytes := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}
