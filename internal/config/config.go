package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lc0rp/cli-365/internal/paths"
)

type Config struct {
	ProfileDir string         `yaml:"profile_dir"`
	Browser    BrowserConfig  `yaml:"browser"`
	Auth       AuthConfig     `yaml:"auth"`
	Security   SecurityConfig `yaml:"security"`
	Daemon     DaemonConfig   `yaml:"daemon"`
}

type BrowserConfig struct {
	Headless    bool   `yaml:"headless"`
	NoSandbox   bool   `yaml:"no_sandbox"`
	CDPEndpoint string `yaml:"cdp_endpoint"`
	CDPPort     int    `yaml:"cdp_port"`
}

type AuthConfig struct {
	Tenant      string   `yaml:"tenant"`
	AccountHint string   `yaml:"account_hint"`
	Readonly    bool     `yaml:"readonly"`
	SecureInput string   `yaml:"secure_input"`
	Scopes      []string `yaml:"scopes"`
}

type SecurityConfig struct {
	Allowlist []string `yaml:"allowlist"`
	Keyring   string   `yaml:"keyring"`
}

type DaemonConfig struct {
	Enabled                          bool               `yaml:"enabled"`
	SocketPath                       string             `yaml:"socket_path"`
	LockPath                         string             `yaml:"lock_path"`
	StatusPath                       string             `yaml:"status_path"`
	MaxQueueSize                     int                `yaml:"max_queue_size"`
	MaxRequestBytes                  int                `yaml:"max_request_bytes"`
	MaxResponseBytes                 int                `yaml:"max_response_bytes"`
	DefaultCommandTimeout            time.Duration      `yaml:"default_command_timeout"`
	AuthRecoveryTimeout              time.Duration      `yaml:"auth_recovery_timeout"`
	RejectNewWhileAuthPaused         bool               `yaml:"reject_new_while_auth_paused"`
	Display                          string             `yaml:"display"`
	CoalesceIdenticalReads           bool               `yaml:"coalesce_identical_reads"`
	DuplicateWriteWindowMail         time.Duration      `yaml:"duplicate_write_window_mail"`
	DuplicateWriteWindowCalendar     time.Duration      `yaml:"duplicate_write_window_calendar"`
	WriteRateLimitPerMinute          int                `yaml:"write_rate_limit_per_minute"`
	RecipientWriteRateLimitPerMinute int                `yaml:"recipient_write_rate_limit_per_minute"`
	Notify                           DaemonNotifyConfig `yaml:"notify"`
}

type DaemonNotifyConfig struct {
	Provider    string `yaml:"provider"`
	OpenClawCmd string `yaml:"openclaw_cmd"`
	Channel     string `yaml:"channel"`
	Target      string `yaml:"target"`
}

func Default() Config {
	return Config{
		ProfileDir: paths.ProfileDir(),
		Browser: BrowserConfig{
			Headless:    true,
			NoSandbox:   false,
			CDPEndpoint: "",
			CDPPort:     0,
		},
		Auth: AuthConfig{
			Tenant:      "common",
			AccountHint: "",
			Readonly:    false,
			SecureInput: "secure-targeted-input",
			Scopes:      []string{"mail.readwrite", "mail.send"},
		},
		Security: SecurityConfig{
			Allowlist: []string{"mail", "calendar", "auth", "browser", "daemon"},
			Keyring:   "os",
		},
		Daemon: DaemonConfig{
			Enabled:                          false,
			SocketPath:                       "",
			LockPath:                         "",
			StatusPath:                       "",
			MaxQueueSize:                     64,
			MaxRequestBytes:                  1024 * 1024,
			MaxResponseBytes:                 1024 * 1024,
			DefaultCommandTimeout:            2 * time.Minute,
			AuthRecoveryTimeout:              5 * time.Minute,
			RejectNewWhileAuthPaused:         true,
			Display:                          ":1",
			CoalesceIdenticalReads:           true,
			DuplicateWriteWindowMail:         12 * time.Hour,
			DuplicateWriteWindowCalendar:     1 * time.Hour,
			WriteRateLimitPerMinute:          20,
			RecipientWriteRateLimitPerMinute: 6,
			Notify: DaemonNotifyConfig{
				Provider:    "openclaw-cli",
				OpenClawCmd: "openclaw",
				Channel:     "discord",
				Target:      "",
			},
		},
	}
}

func ResolvePath(path string) string {
	if path == "" {
		return paths.ConfigPath()
	}
	return paths.ExpandUser(path)
}

func Load(path string) (Config, error) {
	cfg := Default()
	resolved := ResolvePath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.ProfileDir == "" {
		cfg.ProfileDir = paths.ProfileDir()
	} else {
		cfg.ProfileDir = paths.ExpandUser(cfg.ProfileDir)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	resolved := ResolvePath(path)
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(resolved, data, 0o600)
}
