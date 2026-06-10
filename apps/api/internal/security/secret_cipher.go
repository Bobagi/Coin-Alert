package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// SecretCipher encrypts and decrypts sensitive values (such as Binance API secrets) at rest
// using AES-256-GCM. The encryption key is supplied as a base64-encoded 32-byte key, typically
// via the CREDENTIALS_ENCRYPTION_KEY environment variable.
type SecretCipher struct {
	authenticatedCipher cipher.AEAD
	keyBytes            []byte // kept for keyed fingerprinting (HMAC), separate purpose from encryption
}

// NewSecretCipher builds a SecretCipher from a base64-encoded 32-byte key.
func NewSecretCipher(base64EncodedKey string) (*SecretCipher, error) {
	if base64EncodedKey == "" {
		return nil, errors.New("credentials encryption key is not configured")
	}

	keyBytes, decodeError := base64.StdEncoding.DecodeString(base64EncodedKey)
	if decodeError != nil {
		return nil, fmt.Errorf("credentials encryption key is not valid base64: %w", decodeError)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("credentials encryption key must decode to 32 bytes, got %d", len(keyBytes))
	}

	blockCipher, blockCipherError := aes.NewCipher(keyBytes)
	if blockCipherError != nil {
		return nil, blockCipherError
	}

	galoisCounterMode, galoisCounterModeError := cipher.NewGCM(blockCipher)
	if galoisCounterModeError != nil {
		return nil, galoisCounterModeError
	}

	return &SecretCipher{authenticatedCipher: galoisCounterMode, keyBytes: keyBytes}, nil
}

// EmailFingerprint returns a keyed one-way fingerprint (HMAC-SHA256) of an email address. It lets the
// app correlate or look up an email (e.g. for the deletion audit) WITHOUT storing the address itself:
// the result cannot be reversed to the email without this server key. Returns "" if no key is set.
func (secretCipher *SecretCipher) EmailFingerprint(email string) string {
	if secretCipher == nil || len(secretCipher.keyBytes) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, secretCipher.keyBytes)
	// Domain-separate from any other use of the same key.
	mac.Write([]byte("account-deletion-email-fingerprint:" + strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(mac.Sum(nil))
}

// EncryptString returns a base64-encoded payload of (nonce || ciphertext || auth tag).
func (secretCipher *SecretCipher) EncryptString(plainText string) (string, error) {
	nonce := make([]byte, secretCipher.authenticatedCipher.NonceSize())
	if _, randomReadError := io.ReadFull(rand.Reader, nonce); randomReadError != nil {
		return "", randomReadError
	}

	sealedPayload := secretCipher.authenticatedCipher.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(sealedPayload), nil
}

// DecryptString reverses EncryptString. It fails if the payload was tampered with.
func (secretCipher *SecretCipher) DecryptString(encodedPayload string) (string, error) {
	rawPayload, decodeError := base64.StdEncoding.DecodeString(encodedPayload)
	if decodeError != nil {
		return "", decodeError
	}

	nonceSize := secretCipher.authenticatedCipher.NonceSize()
	if len(rawPayload) < nonceSize {
		return "", errors.New("encrypted payload is too short to contain a nonce")
	}

	nonce := rawPayload[:nonceSize]
	cipherText := rawPayload[nonceSize:]

	plainBytes, openError := secretCipher.authenticatedCipher.Open(nil, nonce, cipherText, nil)
	if openError != nil {
		return "", openError
	}

	return string(plainBytes), nil
}
