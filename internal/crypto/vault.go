package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	ErrEmptyKey      = errors.New("encryption key cannot be empty")
	ErrInvalidCipher = errors.New("invalid ciphertext format")
	ErrDecryptFail   = errors.New("decryption failed")
)

// Vault manages encrypted secrets for the autodev pipeline.
type Vault struct {
	mu      sync.Mutex
	key     []byte
	cache   map[string]string
	aead    cipher.AEAD
}

// New creates a Vault from a password.
func New(password string) (*Vault, error) {
	if password == "" {
		return nil, ErrEmptyKey
	}

	key := sha256.Sum256([]byte(password))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &Vault{
		key:   key[:],
		cache: make(map[string]string),
		aead:  aead,
	}, nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext.
func (v *Vault) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := v.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decodes and decrypts base64-encoded ciphertext.
func (v *Vault) Decrypt(encodedCiphertext string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonceSize := v.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", ErrInvalidCipher
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := v.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// Set stores an encrypted secret in the vault cache.
func (v *Vault) Set(key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	encrypted, err := v.Encrypt(value)
	if err != nil {
		return err
	}

	v.cache[key] = encrypted
	return nil
}

// Get retrieves and decrypts a secret from the vault cache.
func (v *Vault) Get(keyOrAlias string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	encrypted, exists := v.cache[keyOrAlias]
	if !exists {
		return "", fmt.Errorf("secret %q not found", keyOrAlias)
	}

	return v.Decrypt(encrypted)
}

// Delete removes a secret from the vault cache.
func (v *Vault) Delete(key string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.cache, key)
}

// Keys returns all secret aliases in the vault.
func (v *Vault) Keys() []string {
	v.mu.Lock()
	defer v.mu.Unlock()

	keys := make([]string, 0, len(v.cache))
	for k := range v.cache {
		keys = append(keys, k)
	}
	return keys
}

// Exists checks if a secret exists in the vault.
func (v *Vault) Exists(key string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, exists := v.cache[key]
	return exists
}

// RotateSecret re-encrypts and replaces a secret.
func (v *Vault) RotateSecret(key, newValue string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	encrypted, err := v.Encrypt(newValue)
	if err != nil {
		return err
	}

	v.cache[key] = encrypted
	return nil
}

// Clear removes all secrets from the vault cache.
func (v *Vault) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cache = make(map[string]string)
}

// Count returns the number of secrets stored.
func (v *Vault) Count() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.cache)
}

// Export exports all secrets (returns encrypted data, safe for storage).
func (v *Vault) Export() map[string]string {
	v.mu.Lock()
	defer v.mu.Unlock()

	export := make(map[string]string)
	for k, v := range v.cache {
		export[k] = v
	}
	return export
}

// Import imports encrypted secrets into the vault (does not decrypt).
func (v *Vault) Import(secrets map[string]string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for k, encrypted := range secrets {
		v.cache[k] = encrypted
	}
}

// GenerateKey generates a random 256-bit key for use as a master password.
func GenerateKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}
