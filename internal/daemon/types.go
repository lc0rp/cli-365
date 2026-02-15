package daemon

import (
	"context"
	"errors"
	"time"
)

const (
	CommandExec = "exec"
	CommandPing = "__ping"
	CommandStop = "__stop"
)

const (
	ErrorCodeInvalidRequest    = "INVALID_REQUEST"
	ErrorCodeExecFailed        = "EXEC_FAILED"
	ErrorCodeQueueFull         = "QUEUE_FULL"
	ErrorCodeRequestTimeout    = "REQUEST_TIMEOUT"
	ErrorCodeAuthPaused        = "AUTH_PAUSED"
	ErrorCodeAuthTimeout       = "AUTH_TIMEOUT"
	ErrorCodeDaemonUnavailable = "DAEMON_UNAVAILABLE"
	ErrorCodeCDPPortMismatch   = "CDP_PORT_MISMATCH"
)

var (
	ErrAlreadyRunning      = errors.New("daemon already running")
	ErrUnsupportedPlatform = errors.New("daemon mode is supported only on linux and macos")
)

type Request struct {
	RequestID   string    `json:"request_id"`
	SubmittedAt time.Time `json:"submitted_at,omitempty"`
	Command     string    `json:"command"`
	Argv        []string  `json:"argv,omitempty"`
	TimeoutMS   int       `json:"timeout_ms,omitempty"`
	CDPPort     int       `json:"cdp_port,omitempty"`
}

type Response struct {
	RequestID   string    `json:"request_id"`
	OK          bool      `json:"ok"`
	ExitCode    int       `json:"exit_code"`
	Stdout      string    `json:"stdout,omitempty"`
	Stderr      string    `json:"stderr,omitempty"`
	ErrorCode   string    `json:"error_code,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	FinishedAt  time.Time `json:"finished_at,omitempty"`
	QueueWaitMS int64     `json:"queue_wait_ms,omitempty"`
	ExecMS      int64     `json:"exec_ms,omitempty"`
}

type Status struct {
	Running    bool      `json:"running"`
	PID        int       `json:"pid,omitempty"`
	SocketPath string    `json:"socket_path,omitempty"`
	LockPath   string    `json:"lock_path,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	StoppedAt  time.Time `json:"stopped_at,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
}

type Options struct {
	StateDir              string
	SocketPath            string
	LockPath              string
	StatusPath            string
	DefaultCommandTimeout time.Duration
	MaxQueueSize          int
	MaxRequestBytes       int
	CDPPort               int
	RejectNewWhilePaused  bool
}

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

type ExecFunc func(ctx context.Context, argv []string, timeout time.Duration) ExecResult
