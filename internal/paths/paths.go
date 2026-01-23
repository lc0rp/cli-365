package paths

import (
	"os"
	"path/filepath"
	"strings"
)

const appName = "outlook-browser-cli"

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
	home, _ := os.UserHomeDir()
	return home
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
