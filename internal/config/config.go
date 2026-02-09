package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/lc0rp/cli-365/internal/paths"
)

type Config struct {
	ProfileDir string         `yaml:"profile_dir"`
	Browser    BrowserConfig  `yaml:"browser"`
	Auth       AuthConfig     `yaml:"auth"`
	Security   SecurityConfig `yaml:"security"`
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
	Scopes      []string `yaml:"scopes"`
}

type SecurityConfig struct {
	Allowlist []string `yaml:"allowlist"`
	Keyring   string   `yaml:"keyring"`
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
			Scopes:      []string{"mail.readwrite", "mail.send"},
		},
		Security: SecurityConfig{
			Allowlist: []string{"mail", "calendar", "auth", "browser"},
			Keyring:   "os",
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
