package paths

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

const appName = "cli-365"

var (
	userHomeDirFunc = os.UserHomeDir
	currentUserFunc = user.Current
)

func ConfigDir() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	return filepath.Join(HomeDir(), ".config")
}

func StateDir() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	return filepath.Join(HomeDir(), ".local", "state")
}

func HomeDir() string {
	if home, err := userHomeDirFunc(); err == nil {
		if normalized := normalizeAbsolutePath(home); normalized != "" {
			return normalized
		}
	}
	if current, err := currentUserFunc(); err == nil && current != nil {
		if normalized := normalizeAbsolutePath(current.HomeDir); normalized != "" {
			return normalized
		}
	}
	if normalized := normalizeAbsolutePath(os.Getenv("HOME")); normalized != "" {
		return normalized
	}
	return filepath.Join(os.TempDir(), appName+"-home")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), appName, "config.yaml")
}

func ProfileDir() string {
	return filepath.Join(ConfigDir(), appName, "profile")
}

func RuntimePath() string {
	return filepath.Join(StateDir(), appName, "runtime.json")
}

func DaemonStateDir() string {
	return filepath.Join(StateDir(), appName)
}

func DaemonSocketPath() string {
	return filepath.Join(DaemonStateDir(), "daemon.sock")
}

func DaemonLockPath() string {
	return filepath.Join(DaemonStateDir(), "daemon.lock")
}

func DaemonStatusPath() string {
	return filepath.Join(DaemonStateDir(), "daemon.json")
}

func ExpandUser(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		return HomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(HomeDir(), path[2:])
	}
	return path
}

func normalizeAbsolutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return abs
}
