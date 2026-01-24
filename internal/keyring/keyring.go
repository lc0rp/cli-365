// Package keyring provides secure storage for credentials.
package keyring

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lc0rp/cli-365/internal/paths"
)

const (
	// StorageOS uses the OS keyring (macOS Keychain, Windows Credential Manager, etc.)
	StorageOS = "os"
	// StorageEncrypted uses an encrypted file
	StorageEncrypted = "encrypted-file"
	// StoragePlain uses plain file storage (not recommended)
	StoragePlain = "plain"
)

const (
	serviceName = "cli-365"
	accountName = "tokens"
)

// Store handles credential storage.
type Store struct {
	storageType string
	encKey      []byte // For encrypted-file storage
}

// NewStore creates a new credential store.
func NewStore(storageType string) (*Store, error) {
	if storageType == "" {
		storageType = StorageOS
	}

	s := &Store{storageType: storageType}

	if storageType == StorageEncrypted {
		key, err := getOrCreateEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("failed to get encryption key: %w", err)
		}
		s.encKey = key
	}

	return s, nil
}

// Set stores a value.
func (s *Store) Set(key string, value []byte) error {
	switch s.storageType {
	case StorageOS:
		return setOS(key, value)
	case StorageEncrypted:
		return s.setEncrypted(key, value)
	case StoragePlain:
		return setPlain(key, value)
	default:
		return fmt.Errorf("unknown storage type: %s", s.storageType)
	}
}

// Get retrieves a value.
func (s *Store) Get(key string) ([]byte, error) {
	switch s.storageType {
	case StorageOS:
		return getOS(key)
	case StorageEncrypted:
		return s.getEncrypted(key)
	case StoragePlain:
		return getPlain(key)
	default:
		return nil, fmt.Errorf("unknown storage type: %s", s.storageType)
	}
}

// Delete removes a value.
func (s *Store) Delete(key string) error {
	switch s.storageType {
	case StorageOS:
		return deleteOS(key)
	case StorageEncrypted:
		return s.deleteEncrypted(key)
	case StoragePlain:
		return deletePlain(key)
	default:
		return fmt.Errorf("unknown storage type: %s", s.storageType)
	}
}

// --- Plain file storage (fallback) ---

func plainPath(key string) string {
	return filepath.Join(paths.StateDir(), "cli-365", "creds", key+".json")
}

func setPlain(key string, value []byte) error {
	path := plainPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, value, 0o600)
}

func getPlain(key string) ([]byte, error) {
	return os.ReadFile(plainPath(key))
}

func deletePlain(key string) error {
	err := os.Remove(plainPath(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// --- Encrypted file storage ---

func encryptedPath(key string) string {
	return filepath.Join(paths.StateDir(), "cli-365", "creds", key+".enc")
}

func keyFilePath() string {
	return filepath.Join(paths.StateDir(), "cli-365", "creds", ".key")
}

func getOrCreateEncryptionKey() ([]byte, error) {
	keyPath := keyFilePath()

	// Try to read existing key
	if data, err := os.ReadFile(keyPath); err == nil {
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}

	// Generate new key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	// Save key
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(keyPath, []byte(encoded), 0o600); err != nil {
		return nil, err
	}

	return key, nil
}

func (s *Store) setEncrypted(key string, value []byte) error {
	encrypted, err := encrypt(value, s.encKey)
	if err != nil {
		return err
	}

	path := encryptedPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, encrypted, 0o600)
}

func (s *Store) getEncrypted(key string) ([]byte, error) {
	encrypted, err := os.ReadFile(encryptedPath(key))
	if err != nil {
		return nil, err
	}
	return decrypt(encrypted, s.encKey)
}

func (s *Store) deleteEncrypted(key string) error {
	err := os.Remove(encryptedPath(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func encrypt(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

func decrypt(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// --- OS Keyring ---
// Platform-specific implementations

func setOS(key string, value []byte) error {
	// Try OS keyring first, fall back to encrypted file
	if err := setOSNative(key, value); err != nil {
		// Fall back to encrypted file
		store, err := NewStore(StorageEncrypted)
		if err != nil {
			return err
		}
		return store.Set(key, value)
	}
	return nil
}

func getOS(key string) ([]byte, error) {
	data, err := getOSNative(key)
	if err != nil {
		// Try encrypted file fallback
		store, storeErr := NewStore(StorageEncrypted)
		if storeErr != nil {
			return nil, err // Return original error
		}
		return store.Get(key)
	}
	return data, nil
}

func deleteOS(key string) error {
	err := deleteOSNative(key)
	if err != nil {
		// Try encrypted file fallback
		store, storeErr := NewStore(StorageEncrypted)
		if storeErr != nil {
			return err
		}
		return store.Delete(key)
	}
	return nil
}

// MachineID returns a stable machine identifier for key derivation.
func MachineID() string {
	// Simple machine ID based on hostname and OS
	hostname, _ := os.Hostname()
	hash := sha256.Sum256([]byte(hostname + runtime.GOOS + runtime.GOARCH))
	return base64.StdEncoding.EncodeToString(hash[:16])
}

// TokenStorage provides a higher-level interface for storing tokens.
type TokenStorage struct {
	store *Store
}

// NewTokenStorage creates a new token storage.
func NewTokenStorage(storageType string) (*TokenStorage, error) {
	store, err := NewStore(storageType)
	if err != nil {
		return nil, err
	}
	return &TokenStorage{store: store}, nil
}

// SaveTokens saves tokens to secure storage.
func (t *TokenStorage) SaveTokens(tokens interface{}) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return err
	}
	return t.store.Set(accountName, data)
}

// LoadTokens loads tokens from secure storage.
func (t *TokenStorage) LoadTokens(tokens interface{}) error {
	data, err := t.store.Get(accountName)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, tokens)
}

// ClearTokens removes tokens from secure storage.
func (t *TokenStorage) ClearTokens() error {
	return t.store.Delete(accountName)
}
