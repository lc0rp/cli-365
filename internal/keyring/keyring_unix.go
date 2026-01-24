//go:build !windows

package keyring

import (
	"errors"
	"os/exec"
	"strings"
)

// setOSNative tries to use the native OS keyring.
// On macOS, uses security command. On Linux, uses secret-tool if available.
func setOSNative(key string, value []byte) error {
	if isMacOS() {
		return setMacOSKeychain(key, value)
	}
	if hasSecretTool() {
		return setSecretTool(key, value)
	}
	return errors.New("no native keyring available")
}

func getOSNative(key string) ([]byte, error) {
	if isMacOS() {
		return getMacOSKeychain(key)
	}
	if hasSecretTool() {
		return getSecretTool(key)
	}
	return nil, errors.New("no native keyring available")
}

func deleteOSNative(key string) error {
	if isMacOS() {
		return deleteMacOSKeychain(key)
	}
	if hasSecretTool() {
		return deleteSecretTool(key)
	}
	return errors.New("no native keyring available")
}

func isMacOS() bool {
	_, err := exec.LookPath("security")
	return err == nil
}

func hasSecretTool() bool {
	_, err := exec.LookPath("secret-tool")
	return err == nil
}

// --- macOS Keychain ---

func setMacOSKeychain(key string, value []byte) error {
	// Delete existing entry first (ignore error)
	_ = deleteMacOSKeychain(key)

	cmd := exec.Command("security", "add-generic-password",
		"-a", accountName,
		"-s", serviceName+"-"+key,
		"-w", string(value),
		"-U", // Update if exists
	)
	return cmd.Run()
}

func getMacOSKeychain(key string) ([]byte, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-a", accountName,
		"-s", serviceName+"-"+key,
		"-w", // Output password only
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(string(output))), nil
}

func deleteMacOSKeychain(key string) error {
	cmd := exec.Command("security", "delete-generic-password",
		"-a", accountName,
		"-s", serviceName+"-"+key,
	)
	return cmd.Run()
}

// --- Linux secret-tool (GNOME Keyring / KDE Wallet) ---

func setSecretTool(key string, value []byte) error {
	cmd := exec.Command("secret-tool", "store",
		"--label", serviceName+" "+key,
		"service", serviceName,
		"account", key,
	)
	cmd.Stdin = strings.NewReader(string(value))
	return cmd.Run()
}

func getSecretTool(key string) ([]byte, error) {
	cmd := exec.Command("secret-tool", "lookup",
		"service", serviceName,
		"account", key,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return output, nil
}

func deleteSecretTool(key string) error {
	cmd := exec.Command("secret-tool", "clear",
		"service", serviceName,
		"account", key,
	)
	return cmd.Run()
}
