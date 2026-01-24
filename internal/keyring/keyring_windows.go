//go:build windows

package keyring

import (
	"errors"
)

// Windows implementation uses encrypted file storage as fallback.
// For production, you might want to use Windows Credential Manager via syscall.

func setOSNative(key string, value []byte) error {
	return errors.New("Windows Credential Manager not yet implemented - using encrypted file")
}

func getOSNative(key string) ([]byte, error) {
	return nil, errors.New("Windows Credential Manager not yet implemented - using encrypted file")
}

func deleteOSNative(key string) error {
	return errors.New("Windows Credential Manager not yet implemented - using encrypted file")
}
