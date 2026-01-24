package keyring

import (
	"bytes"
	"os"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	tests := []struct {
		name string
		data []byte
	}{
		{"simple text", []byte("Hello, World!")},
		{"empty", []byte{}},
		{"binary", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}},
		{"json", []byte(`{"key": "value", "number": 42}`)},
		{"long text", bytes.Repeat([]byte("A"), 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := encrypt(tt.data, key)
			if err != nil {
				t.Fatalf("encrypt error: %v", err)
			}

			// Encrypted should be different from original (unless empty)
			if len(tt.data) > 0 && bytes.Equal(encrypted, tt.data) {
				t.Error("encrypted data should be different from original")
			}

			decrypted, err := decrypt(encrypted, key)
			if err != nil {
				t.Fatalf("decrypt error: %v", err)
			}

			if !bytes.Equal(decrypted, tt.data) {
				t.Errorf("decrypted = %v, want %v", decrypted, tt.data)
			}
		})
	}
}

func TestEncryptDifferentEachTime(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	data := []byte("test data")

	enc1, err := encrypt(data, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	enc2, err := encrypt(data, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	// Each encryption should produce different output (different nonce)
	if bytes.Equal(enc1, enc2) {
		t.Error("encrypting same data twice should produce different ciphertext")
	}

	// But both should decrypt to same plaintext
	dec1, _ := decrypt(enc1, key)
	dec2, _ := decrypt(enc2, key)
	if !bytes.Equal(dec1, dec2) {
		t.Error("both ciphertexts should decrypt to same plaintext")
	}
}

func TestDecryptInvalidData(t *testing.T) {
	key := make([]byte, 32)

	// Too short
	_, err := decrypt([]byte{1, 2, 3}, key)
	if err == nil {
		t.Error("decrypt should fail on short data")
	}

	// Random garbage
	garbage := make([]byte, 100)
	for i := range garbage {
		garbage[i] = byte(i)
	}
	_, err = decrypt(garbage, key)
	if err == nil {
		t.Error("decrypt should fail on invalid ciphertext")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 1)
	}

	data := []byte("secret data")
	encrypted, err := encrypt(data, key1)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	// Decrypt with wrong key should fail
	_, err = decrypt(encrypted, key2)
	if err == nil {
		t.Error("decrypt with wrong key should fail")
	}
}

func TestPlainStorage(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	store, err := NewStore(StoragePlain)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	key := "test-key"
	value := []byte("test value")

	// Set
	if err := store.Set(key, value); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Get
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get = %v, want %v", got, value)
	}

	// Delete
	if err := store.Delete(key); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// Get after delete should fail
	_, err = store.Get(key)
	if err == nil {
		t.Error("Get after Delete should fail")
	}
}

func TestEncryptedStorage(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	store, err := NewStore(StorageEncrypted)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	key := "secret-key"
	value := []byte("secret value that should be encrypted")

	// Set
	if err := store.Set(key, value); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Get
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get = %v, want %v", got, value)
	}

	// Delete
	if err := store.Delete(key); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
}

func TestEncryptedKeyPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	// Create first store and save data
	store1, err := NewStore(StorageEncrypted)
	if err != nil {
		t.Fatalf("NewStore 1 error: %v", err)
	}

	key := "persist-key"
	value := []byte("persistent secret")
	if err := store1.Set(key, value); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Create second store (should use same encryption key)
	store2, err := NewStore(StorageEncrypted)
	if err != nil {
		t.Fatalf("NewStore 2 error: %v", err)
	}

	// Should be able to read data encrypted by first store
	got, err := store2.Get(key)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Errorf("Get = %v, want %v", got, value)
	}
}

func TestTokenStorage(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	ts, err := NewTokenStorage(StoragePlain)
	if err != nil {
		t.Fatalf("NewTokenStorage error: %v", err)
	}

	type TestTokens struct {
		Canary string `json:"canary"`
		Bearer string `json:"bearer"`
	}

	original := TestTokens{
		Canary: "test-canary-123",
		Bearer: "Bearer test-token",
	}

	// Save
	if err := ts.SaveTokens(original); err != nil {
		t.Fatalf("SaveTokens error: %v", err)
	}

	// Load
	var loaded TestTokens
	if err := ts.LoadTokens(&loaded); err != nil {
		t.Fatalf("LoadTokens error: %v", err)
	}

	if loaded.Canary != original.Canary {
		t.Errorf("Canary = %q, want %q", loaded.Canary, original.Canary)
	}
	if loaded.Bearer != original.Bearer {
		t.Errorf("Bearer = %q, want %q", loaded.Bearer, original.Bearer)
	}

	// Clear
	if err := ts.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens error: %v", err)
	}

	// Load after clear should fail
	if err := ts.LoadTokens(&loaded); err == nil {
		t.Error("LoadTokens after Clear should fail")
	}
}

func TestMachineID(t *testing.T) {
	id1 := MachineID()
	id2 := MachineID()

	// Should be stable
	if id1 != id2 {
		t.Error("MachineID should be stable")
	}

	// Should be non-empty
	if id1 == "" {
		t.Error("MachineID should not be empty")
	}
}

func TestStorageTypes(t *testing.T) {
	// Verify constants are defined
	types := []string{StorageOS, StorageEncrypted, StoragePlain}

	for _, st := range types {
		if st == "" {
			t.Error("storage type should not be empty")
		}
	}

	if StorageOS != "os" {
		t.Errorf("StorageOS = %q, want 'os'", StorageOS)
	}
	if StorageEncrypted != "encrypted-file" {
		t.Errorf("StorageEncrypted = %q, want 'encrypted-file'", StorageEncrypted)
	}
}

func TestDeleteNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	origXDG := os.Getenv("XDG_STATE_HOME")
	os.Setenv("XDG_STATE_HOME", tmpDir)
	defer os.Setenv("XDG_STATE_HOME", origXDG)

	store, err := NewStore(StoragePlain)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// Deleting non-existent key should not error
	if err := store.Delete("non-existent"); err != nil {
		t.Errorf("Delete non-existent should not error: %v", err)
	}
}
