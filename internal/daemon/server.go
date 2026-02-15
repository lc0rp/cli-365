package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Server struct {
	opts   Options
	execFn ExecFunc

	mu       sync.Mutex
	cancel   context.CancelFunc
	listener net.Listener
	lockFile *os.File
	execQ    chan queuedExec

	pauseMu sync.RWMutex
	paused  bool
	pauseCh chan struct{}
}

type queuedExec struct {
	req        Request
	argv       []string
	timeout    time.Duration
	enqueuedAt time.Time
	respCh     chan Response
}

func NewServer(opts Options, execFn ExecFunc) *Server {
	opts = opts.withDefaults()
	if execFn == nil {
		execFn = defaultExecFunc
	}
	return &Server{
		opts:    opts,
		execFn:  execFn,
		execQ:   make(chan queuedExec, opts.MaxQueueSize),
		pauseCh: make(chan struct{}),
	}
}

func (s *Server) Run(ctx context.Context) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return ErrUnsupportedPlatform
	}

	if err := os.MkdirAll(s.opts.StateDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.opts.StateDir, 0o700); err != nil {
		return err
	}

	lockFile, err := acquireFileLock(s.opts.LockPath)
	if err != nil {
		return err
	}
	s.lockFile = lockFile

	if err := os.RemoveAll(s.opts.SocketPath); err != nil {
		_ = releaseFileLock(s.opts.LockPath, s.lockFile)
		return err
	}

	ln, err := net.Listen("unix", s.opts.SocketPath)
	if err != nil {
		_ = releaseFileLock(s.opts.LockPath, s.lockFile)
		return err
	}
	s.listener = ln
	if err := os.Chmod(s.opts.SocketPath, 0o600); err != nil {
		_ = ln.Close()
		_ = releaseFileLock(s.opts.LockPath, s.lockFile)
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	started := time.Now().UTC()
	if err := s.writeStatus(Status{
		Running:    true,
		PID:        os.Getpid(),
		SocketPath: s.opts.SocketPath,
		LockPath:   s.opts.LockPath,
		StartedAt:  started,
	}); err != nil {
		_ = s.listener.Close()
		_ = releaseFileLock(s.opts.LockPath, s.lockFile)
		return err
	}

	defer func() {
		s.requestStop()
		_ = os.Remove(s.opts.SocketPath)
		_ = releaseFileLock(s.opts.LockPath, s.lockFile)
		_ = s.writeStatus(Status{
			Running:    false,
			PID:        os.Getpid(),
			SocketPath: s.opts.SocketPath,
			LockPath:   s.opts.LockPath,
			StartedAt:  started,
			StoppedAt:  time.Now().UTC(),
		})
	}()

	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		s.runWorker(runCtx)
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if runCtx.Err() != nil || errors.Is(err, net.ErrClosed) {
				<-workerDone
				return nil
			}
			continue
		}
		go s.handleConn(runCtx, conn)
	}
}

func (s *Server) handleConn(parent context.Context, conn net.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(io.LimitReader(conn, int64(s.opts.MaxRequestBytes)))
	enc := json.NewEncoder(conn)

	var req Request
	if err := dec.Decode(&req); err != nil {
		_ = enc.Encode(Response{
			OK:        false,
			ExitCode:  1,
			ErrorCode: ErrorCodeInvalidRequest,
			Stderr:    err.Error(),
		})
		return
	}

	timeout := s.opts.DefaultCommandTimeout
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}

	response := Response{
		RequestID: req.RequestID,
		StartedAt: time.Now().UTC(),
	}

	switch req.Command {
	case CommandPing:
		response.OK = true
		response.ExitCode = 0
		response.Stdout = "pong\n"
	case CommandStop:
		response.OK = true
		response.ExitCode = 0
		response.Stdout = "stopping\n"
	default:
		if req.Command != "" && req.Command != CommandExec {
			response.OK = false
			response.ExitCode = 1
			response.ErrorCode = ErrorCodeInvalidRequest
			response.Stderr = fmt.Sprintf("unsupported daemon command: %s", req.Command)
			break
		}

		argv := stripDaemonFlags(req.Argv)
		if len(argv) == 0 {
			response.OK = false
			response.ExitCode = 1
			response.ErrorCode = ErrorCodeInvalidRequest
			response.Stderr = "argv is required"
			break
		}

		if msg, mismatch := s.cdpPortMismatch(req.CDPPort); mismatch {
			response.OK = false
			response.ExitCode = 1
			response.ErrorCode = ErrorCodeCDPPortMismatch
			response.Stderr = msg
			break
		}

		task := queuedExec{
			req:        req,
			argv:       argv,
			timeout:    timeout,
			enqueuedAt: time.Now().UTC(),
			respCh:     make(chan Response, 1),
		}
		if code := s.tryEnqueue(task); code != "" {
			response.OK = false
			response.ExitCode = 1
			response.ErrorCode = code
			if code == ErrorCodeAuthPaused {
				response.Stderr = "daemon auth recovery in progress"
			} else {
				response.Stderr = "daemon queue full"
			}
			break
		}

		select {
		case response = <-task.respCh:
		case <-parent.Done():
			response.OK = false
			response.ExitCode = 1
			response.ErrorCode = ErrorCodeDaemonUnavailable
			response.Stderr = "daemon unavailable"
		}
	}

	response.FinishedAt = time.Now().UTC()
	_ = enc.Encode(response)

	if req.Command == CommandStop {
		s.requestStop()
	}
}

func (s *Server) requestStop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}
}

func (s *Server) tryEnqueue(task queuedExec) string {
	if s.opts.RejectNewWhilePaused && s.isPaused() {
		return ErrorCodeAuthPaused
	}
	select {
	case s.execQ <- task:
		return ""
	default:
		return ErrorCodeQueueFull
	}
}

func (s *Server) runWorker(ctx context.Context) {
	stopCode := ErrorCodeDaemonUnavailable
	stopMsg := "daemon unavailable"

	for {
		if !s.waitUntilUnpaused(ctx) {
			s.drainPending(stopCode, stopMsg)
			return
		}

		select {
		case <-ctx.Done():
			s.drainPending(stopCode, stopMsg)
			return
		case task := <-s.execQ:
			if !s.waitUntilUnpaused(ctx) {
				task.respCh <- daemonFailureResponse(task.req.RequestID, stopCode, stopMsg)
				s.drainPending(stopCode, stopMsg)
				return
			}
			resp := s.executeTaskSafely(ctx, task)
			task.respCh <- resp
		}
	}
}

func (s *Server) executeTaskSafely(parent context.Context, task queuedExec) (resp Response) {
	defer func() {
		if r := recover(); r != nil {
			resp = daemonFailureResponse(task.req.RequestID, ErrorCodeExecFailed, fmt.Sprintf("panic during execution: %v", r))
		}
	}()
	return s.executeTask(parent, task)
}

func (s *Server) executeTask(parent context.Context, task queuedExec) Response {
	start := time.Now().UTC()
	resp := Response{
		RequestID:   task.req.RequestID,
		StartedAt:   start,
		QueueWaitMS: start.Sub(task.enqueuedAt).Milliseconds(),
	}

	execCtx, cancel := context.WithTimeout(parent, task.timeout)
	result := s.execFn(execCtx, task.argv, task.timeout)
	cancel()

	end := time.Now().UTC()
	resp.FinishedAt = end
	resp.ExecMS = end.Sub(start).Milliseconds()
	resp.Stdout = result.Stdout
	resp.Stderr = result.Stderr
	resp.ExitCode = result.ExitCode
	resp.OK = result.ExitCode == 0 && result.Err == nil

	if result.Err != nil {
		if resp.Stderr == "" {
			resp.Stderr = result.Err.Error()
		}
		if resp.ExitCode == 0 {
			resp.ExitCode = 1
		}
		if errors.Is(result.Err, context.DeadlineExceeded) {
			resp.ErrorCode = ErrorCodeRequestTimeout
		} else {
			resp.ErrorCode = ErrorCodeExecFailed
		}
	}

	return resp
}

func (s *Server) writeStatus(status Status) error {
	if err := os.MkdirAll(filepath.Dir(s.opts.StatusPath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.opts.StatusPath, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(s.opts.StatusPath, 0o600)
}

func stripDaemonFlags(args []string) []string {
	out := make([]string, 0, len(args))
	skipNext := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		if arg == "--daemon" {
			if i+1 < len(args) {
				next := strings.ToLower(strings.TrimSpace(args[i+1]))
				if next == "true" || next == "false" || next == "1" || next == "0" {
					skipNext = true
				}
			}
			continue
		}
		if strings.HasPrefix(arg, "--daemon=") {
			continue
		}

		out = append(out, arg)
	}
	return out
}

func (s *Server) cdpPortMismatch(requested int) (string, bool) {
	if requested <= 0 {
		return "", false
	}
	if requested == s.opts.CDPPort {
		return "", false
	}
	return fmt.Sprintf("daemon running with cdp-port=%d; requested=%d", s.opts.CDPPort, requested), true
}

func (s *Server) isPaused() bool {
	s.pauseMu.RLock()
	defer s.pauseMu.RUnlock()
	return s.paused
}

func (s *Server) setPaused(paused bool) {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()

	if s.paused == paused {
		return
	}

	s.paused = paused
	close(s.pauseCh)
	s.pauseCh = make(chan struct{})
}

func (s *Server) waitUntilUnpaused(ctx context.Context) bool {
	for {
		s.pauseMu.RLock()
		paused := s.paused
		ch := s.pauseCh
		s.pauseMu.RUnlock()

		if !paused {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-ch:
		}
	}
}

func (s *Server) drainPending(code, msg string) {
	for {
		select {
		case task := <-s.execQ:
			task.respCh <- daemonFailureResponse(task.req.RequestID, code, msg)
		default:
			return
		}
	}
}

func daemonFailureResponse(requestID, code, msg string) Response {
	now := time.Now().UTC()
	return Response{
		RequestID:  requestID,
		OK:         false,
		ExitCode:   1,
		ErrorCode:  code,
		Stderr:     msg,
		StartedAt:  now,
		FinishedAt: now,
	}
}
