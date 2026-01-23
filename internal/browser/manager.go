package browser

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod/lib/launcher"

	"github.com/lc0rp/outlook-browser-cli/internal/config"
	"github.com/lc0rp/outlook-browser-cli/internal/paths"
)

type RuntimeInfo struct {
	WSEndpoint string    `json:"ws_endpoint"`
	PID        int       `json:"pid"`
	Managed    bool      `json:"managed"`
	StartedAt  time.Time `json:"started_at"`
}

func Start(_ context.Context, cfg config.Config) (*RuntimeInfo, error) {
	if cfg.Browser.CDPEndpoint != "" {
		rt := &RuntimeInfo{
			WSEndpoint: cfg.Browser.CDPEndpoint,
			PID:        0,
			Managed:    false,
			StartedAt:  time.Now(),
		}
		if err := SaveRuntime(rt); err != nil {
			return nil, err
		}
		return rt, nil
	}

	profileDir := cfg.ProfileDir
	if profileDir == "" {
		profileDir = paths.ProfileDir()
	}
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return nil, err
	}

	l := launcher.New().Headless(cfg.Browser.Headless).UserDataDir(profileDir)
	url, err := l.Launch()
	if err != nil {
		return nil, err
	}

	pid := l.PID()
	rt := &RuntimeInfo{
		WSEndpoint: url,
		PID:        pid,
		Managed:    true,
		StartedAt:  time.Now(),
	}
	if err := SaveRuntime(rt); err != nil {
		return nil, err
	}
	return rt, nil
}

func Status() (*RuntimeInfo, error) {
	return LoadRuntime()
}

func Stop() error {
	rt, err := LoadRuntime()
	if err != nil {
		return err
	}
	if !rt.Managed || rt.PID == 0 {
		return errors.New("no managed browser to stop")
	}
	proc, err := os.FindProcess(rt.PID)
	if err != nil {
		return err
	}
	if err := proc.Kill(); err != nil {
		return err
	}
	return os.Remove(runtimePath())
}

func runtimePath() string {
	return paths.RuntimePath()
}

func LoadRuntime() (*RuntimeInfo, error) {
	path := runtimePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rt RuntimeInfo
	if err := json.Unmarshal(data, &rt); err != nil {
		return nil, err
	}
	return &rt, nil
}

func SaveRuntime(rt *RuntimeInfo) error {
	path := runtimePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rt, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
