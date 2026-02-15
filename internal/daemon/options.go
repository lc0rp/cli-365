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
	authRecoveryTimeout := cfg.Daemon.AuthRecoveryTimeout
	if authRecoveryTimeout <= 0 {
		authRecoveryTimeout = 5 * time.Minute
	}
	authProbeInterval := 2 * time.Second
	maxQueueSize := cfg.Daemon.MaxQueueSize
	if maxQueueSize <= 0 {
		maxQueueSize = 64
	}
	maxRequestBytes := cfg.Daemon.MaxRequestBytes
	if maxRequestBytes <= 0 {
		maxRequestBytes = 1024 * 1024
	}
	maxResponseBytes := cfg.Daemon.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = 1024 * 1024
	}
	duplicateWriteWindowMail := cfg.Daemon.DuplicateWriteWindowMail
	if duplicateWriteWindowMail <= 0 {
		duplicateWriteWindowMail = 12 * time.Hour
	}
	duplicateWriteWindowCalendar := cfg.Daemon.DuplicateWriteWindowCalendar
	if duplicateWriteWindowCalendar <= 0 {
		duplicateWriteWindowCalendar = 1 * time.Hour
	}
	writeRateLimitPerMinute := cfg.Daemon.WriteRateLimitPerMinute
	if writeRateLimitPerMinute <= 0 {
		writeRateLimitPerMinute = 20
	}
	recipientWriteRateLimitPerMinute := cfg.Daemon.RecipientWriteRateLimitPerMinute
	if recipientWriteRateLimitPerMinute <= 0 {
		recipientWriteRateLimitPerMinute = 6
	}
	secureInputCommand := strings.TrimSpace(cfg.Auth.SecureInput)
	if secureInputCommand == "" {
		secureInputCommand = "secure-targeted-input"
	}
	notifyProvider := strings.TrimSpace(cfg.Daemon.Notify.Provider)
	if notifyProvider == "" {
		notifyProvider = "openclaw-cli"
	}
	notifyOpenClawCmd := strings.TrimSpace(cfg.Daemon.Notify.OpenClawCmd)
	if notifyOpenClawCmd == "" {
		notifyOpenClawCmd = "openclaw"
	}
	notifyChannel := strings.TrimSpace(cfg.Daemon.Notify.Channel)
	if notifyChannel == "" {
		notifyChannel = "discord"
	}
	loginURL := "https://outlook.office.com/mail/"

	return Options{
		StateDir:                         stateDir,
		SocketPath:                       socketPath,
		LockPath:                         lockPath,
		StatusPath:                       statusPath,
		DefaultCommandTimeout:            timeout,
		AuthRecoveryTimeout:              authRecoveryTimeout,
		AuthProbeInterval:                authProbeInterval,
		MaxQueueSize:                     maxQueueSize,
		MaxRequestBytes:                  maxRequestBytes,
		MaxResponseBytes:                 maxResponseBytes,
		CDPPort:                          cfg.Browser.CDPPort,
		RejectNewWhilePaused:             cfg.Daemon.RejectNewWhileAuthPaused,
		CoalesceIdenticalReads:           cfg.Daemon.CoalesceIdenticalReads,
		DuplicateWriteWindowMail:         duplicateWriteWindowMail,
		DuplicateWriteWindowCalendar:     duplicateWriteWindowCalendar,
		WriteRateLimitPerMinute:          writeRateLimitPerMinute,
		RecipientWriteRateLimitPerMinute: recipientWriteRateLimitPerMinute,
		SecureInputCommand:               secureInputCommand,
		NotifyProvider:                   notifyProvider,
		NotifyOpenClawCmd:                notifyOpenClawCmd,
		NotifyChannel:                    notifyChannel,
		NotifyTarget:                     strings.TrimSpace(cfg.Daemon.Notify.Target),
		LoginURL:                         loginURL,
		Allowlist:                        append([]string{}, cfg.Security.Allowlist...),
		Readonly:                         cfg.Auth.Readonly,
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
	authRecoveryTimeout := o.AuthRecoveryTimeout
	if authRecoveryTimeout <= 0 {
		authRecoveryTimeout = 5 * time.Minute
	}
	authProbeInterval := o.AuthProbeInterval
	if authProbeInterval <= 0 {
		authProbeInterval = 2 * time.Second
	}
	maxQueueSize := o.MaxQueueSize
	if maxQueueSize <= 0 {
		maxQueueSize = 64
	}
	maxRequestBytes := o.MaxRequestBytes
	if maxRequestBytes <= 0 {
		maxRequestBytes = 1024 * 1024
	}
	maxResponseBytes := o.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = 1024 * 1024
	}
	duplicateWriteWindowMail := o.DuplicateWriteWindowMail
	if duplicateWriteWindowMail <= 0 {
		duplicateWriteWindowMail = 12 * time.Hour
	}
	duplicateWriteWindowCalendar := o.DuplicateWriteWindowCalendar
	if duplicateWriteWindowCalendar <= 0 {
		duplicateWriteWindowCalendar = 1 * time.Hour
	}
	writeRateLimitPerMinute := o.WriteRateLimitPerMinute
	if writeRateLimitPerMinute <= 0 {
		writeRateLimitPerMinute = 20
	}
	recipientWriteRateLimitPerMinute := o.RecipientWriteRateLimitPerMinute
	if recipientWriteRateLimitPerMinute <= 0 {
		recipientWriteRateLimitPerMinute = 6
	}
	secureInputCommand := strings.TrimSpace(o.SecureInputCommand)
	if secureInputCommand == "" {
		secureInputCommand = "secure-targeted-input"
	}
	notifyProvider := strings.TrimSpace(o.NotifyProvider)
	if notifyProvider == "" {
		notifyProvider = "openclaw-cli"
	}
	notifyOpenClawCmd := strings.TrimSpace(o.NotifyOpenClawCmd)
	if notifyOpenClawCmd == "" {
		notifyOpenClawCmd = "openclaw"
	}
	notifyChannel := strings.TrimSpace(o.NotifyChannel)
	if notifyChannel == "" {
		notifyChannel = "discord"
	}
	loginURL := strings.TrimSpace(o.LoginURL)
	if loginURL == "" {
		loginURL = "https://outlook.office.com/mail/"
	}

	return Options{
		StateDir:                         stateDir,
		SocketPath:                       socketPath,
		LockPath:                         lockPath,
		StatusPath:                       statusPath,
		DefaultCommandTimeout:            timeout,
		AuthRecoveryTimeout:              authRecoveryTimeout,
		AuthProbeInterval:                authProbeInterval,
		MaxQueueSize:                     maxQueueSize,
		MaxRequestBytes:                  maxRequestBytes,
		MaxResponseBytes:                 maxResponseBytes,
		CDPPort:                          o.CDPPort,
		RejectNewWhilePaused:             o.RejectNewWhilePaused,
		CoalesceIdenticalReads:           o.CoalesceIdenticalReads,
		DuplicateWriteWindowMail:         duplicateWriteWindowMail,
		DuplicateWriteWindowCalendar:     duplicateWriteWindowCalendar,
		WriteRateLimitPerMinute:          writeRateLimitPerMinute,
		RecipientWriteRateLimitPerMinute: recipientWriteRateLimitPerMinute,
		SecureInputCommand:               secureInputCommand,
		NotifyProvider:                   notifyProvider,
		NotifyOpenClawCmd:                notifyOpenClawCmd,
		NotifyChannel:                    notifyChannel,
		NotifyTarget:                     strings.TrimSpace(o.NotifyTarget),
		LoginURL:                         loginURL,
		Allowlist:                        append([]string{}, o.Allowlist...),
		Readonly:                         o.Readonly,
	}
}
