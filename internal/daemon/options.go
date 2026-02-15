package daemon

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/lc0rp/cli-365/internal/config"
	"github.com/lc0rp/cli-365/internal/paths"
)

func ResolveOptions(cfg config.Config) Options {
	stateDir := paths.DaemonStateDir()

	socketPath := strings.TrimSpace(cfg.Daemon.SocketPath)
	if socketPath == "" {
		socketPath = paths.DaemonSocketPath()
	}

	lockPath := strings.TrimSpace(cfg.Daemon.LockPath)
	if lockPath == "" {
		lockPath = paths.DaemonLockPath()
	}

	statusPath := strings.TrimSpace(cfg.Daemon.StatusPath)
	if statusPath == "" {
		statusPath = paths.DaemonStatusPath()
	}

	timeout := cfg.Daemon.DefaultCommandTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	maxQueueSize := cfg.Daemon.MaxQueueSize
	if maxQueueSize <= 0 {
		maxQueueSize = 64
	}
	maxRequestBytes := cfg.Daemon.MaxRequestBytes
	if maxRequestBytes <= 0 {
		maxRequestBytes = 1024 * 1024
	}

	return Options{
		StateDir:              stateDir,
		SocketPath:            socketPath,
		LockPath:              lockPath,
		StatusPath:            statusPath,
		DefaultCommandTimeout: timeout,
		MaxQueueSize:          maxQueueSize,
		MaxRequestBytes:       maxRequestBytes,
		CDPPort:               cfg.Browser.CDPPort,
		RejectNewWhilePaused:  cfg.Daemon.RejectNewWhileAuthPaused,
	}
}

func (o Options) withDefaults() Options {
	stateDir := o.StateDir
	if stateDir == "" {
		stateDir = paths.DaemonStateDir()
	}

	socketPath := o.SocketPath
	if socketPath == "" {
		socketPath = filepath.Join(stateDir, "daemon.sock")
	}

	lockPath := o.LockPath
	if lockPath == "" {
		lockPath = filepath.Join(stateDir, "daemon.lock")
	}

	statusPath := o.StatusPath
	if statusPath == "" {
		statusPath = filepath.Join(stateDir, "daemon.json")
	}

	timeout := o.DefaultCommandTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	maxQueueSize := o.MaxQueueSize
	if maxQueueSize <= 0 {
		maxQueueSize = 64
	}
	maxRequestBytes := o.MaxRequestBytes
	if maxRequestBytes <= 0 {
		maxRequestBytes = 1024 * 1024
	}

	return Options{
		StateDir:              stateDir,
		SocketPath:            socketPath,
		LockPath:              lockPath,
		StatusPath:            statusPath,
		DefaultCommandTimeout: timeout,
		MaxQueueSize:          maxQueueSize,
		MaxRequestBytes:       maxRequestBytes,
		CDPPort:               o.CDPPort,
		RejectNewWhilePaused:  o.RejectNewWhilePaused,
	}
}
