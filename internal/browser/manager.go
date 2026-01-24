package browser

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"

	"github.com/lc0rp/cli-365/internal/config"
	"github.com/lc0rp/cli-365/internal/paths"
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
	if !cfg.Browser.Headless {
		l = l.Delete("no-startup-window")
		l = l.Leakless(false)
		l = l.Delete("disable-breakpad")
		l = l.Set("disable-gpu")
		l = l.Set("use-gl", "swiftshader")
		l = l.Set("enable-logging")
		logDir := filepath.Join(paths.StateDir(), "cli-365")
		_ = os.MkdirAll(logDir, 0o700)
		l = l.Set("log-file", filepath.Join(logDir, "chromium.log"))
		l = l.Set("v", "1")
		if stderrLog, err := os.OpenFile(filepath.Join(logDir, "chromium.stderr.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			l = l.Logger(stderrLog)
		}
	}
	if cfg.Browser.NoSandbox {
		l = l.NoSandbox(true)
	}
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

// Connect connects to a running browser instance and returns a rod.Browser.
func Connect(ctx context.Context) (*rod.Browser, error) {
	rt, err := LoadRuntime()
	if err != nil {
		return nil, errors.New("no browser running - run 'browser start' first")
	}

	browser := rod.New().ControlURL(rt.WSEndpoint)
	if err := browser.Connect(); err != nil {
		return nil, err
	}

	return browser, nil
}

// EnsureBrowser ensures a browser is running, starting one if needed.
func EnsureBrowser(ctx context.Context, cfg config.Config) (*rod.Browser, error) {
	// Try to connect to existing browser
	rt, err := LoadRuntime()
	if err == nil && rt.WSEndpoint != "" {
		browser := rod.New().ControlURL(rt.WSEndpoint)
		if err := browser.Connect(); err == nil {
			return browser, nil
		}
		// Clean up stale runtime file
		_ = os.Remove(runtimePath())
	}

	// Start a new browser
	_, err = Start(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return Connect(ctx)
}

// IsRunning checks if a browser is currently running.
func IsRunning() bool {
	rt, err := LoadRuntime()
	if err != nil {
		return false
	}
	if rt.PID > 0 {
		proc, err := os.FindProcess(rt.PID)
		if err != nil {
			return false
		}
		// On Unix, FindProcess always succeeds, so we send signal 0 to check
		if err := proc.Signal(nil); err != nil {
			return false
		}
		return true
	}
	return rt.WSEndpoint != ""
}
