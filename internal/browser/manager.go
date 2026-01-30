package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	if cfg.Browser.CDPPort > 0 {
		l = l.Set("remote-debugging-port", strconv.Itoa(cfg.Browser.CDPPort))
	}
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

	return ConnectEndpoint(rt.WSEndpoint)
}

// ConnectEndpoint connects to a specific CDP websocket endpoint.
func ConnectEndpoint(endpoint string) (*rod.Browser, error) {
	if endpoint == "" {
		return nil, errors.New("cdp endpoint is empty")
	}
	browser := rod.New().ControlURL(endpoint)
	if err := browser.Connect(); err != nil {
		return nil, err
	}
	return browser, nil
}

// ConnectPort connects to a CDP instance running on a fixed port.
func ConnectPort(port int) (*rod.Browser, error) {
	endpoint, err := ResolveWSEndpoint(port)
	if err != nil {
		return nil, err
	}
	return ConnectEndpoint(endpoint)
}

// ResolveWSEndpoint resolves the websocket debugger endpoint for a CDP port.
func ResolveWSEndpoint(port int) (string, error) {
	if port <= 0 {
		return "", errors.New("cdp port must be positive")
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("cdp version endpoint returned %s", resp.Status)
	}
	var payload struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.WebSocketDebuggerURL == "" {
		return "", errors.New("cdp websocket endpoint missing")
	}
	return payload.WebSocketDebuggerURL, nil
}

// EnsureBrowser ensures a browser is running, starting one if needed.
func EnsureBrowser(ctx context.Context, cfg config.Config) (*rod.Browser, error) {
	if cfg.Browser.CDPEndpoint != "" {
		rt := &RuntimeInfo{
			WSEndpoint: cfg.Browser.CDPEndpoint,
			PID:        0,
			Managed:    false,
			StartedAt:  time.Now(),
		}
		_ = SaveRuntime(rt)

		browser := rod.New().ControlURL(cfg.Browser.CDPEndpoint)
		if err := browser.Connect(); err != nil {
			return nil, err
		}
		return browser, nil
	}
	if cfg.Browser.CDPPort > 0 {
		if endpoint, err := ResolveWSEndpoint(cfg.Browser.CDPPort); err == nil {
			if browser, err := ConnectEndpoint(endpoint); err == nil {
				_ = SaveRuntime(&RuntimeInfo{
					WSEndpoint: endpoint,
					PID:        0,
					Managed:    false,
					StartedAt:  time.Now(),
				})
				return browser, nil
			}
		}
	}

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
