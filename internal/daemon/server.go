package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lc0rp/cli-365/internal/security"
)

type Server struct {
	opts   Options
	execFn ExecFunc

	mu       sync.Mutex
	cancel   context.CancelFunc
	listener net.Listener
	lockFile *os.File
	execQ    chan queuedExec
	stopCh   chan struct{}
	stopOnce sync.Once

	pauseMu sync.RWMutex
	paused  bool
	pauseCh chan struct{}

	coalesceMu sync.Mutex
	coalesced  map[string][]chan Response

	writeMu            sync.Mutex
	writeSeen          map[string]time.Time
	writeEvents        []time.Time
	recipientWriteSeen map[string][]time.Time

	authMu            sync.RWMutex
	authState         AuthState
	authProbe         authProbeFunc
	secureInputRunner secureInputRunnerFunc
	notifyAuth        notifyAuthFunc

	randMu sync.Mutex
	rng    *rand.Rand

	logMu     sync.Mutex
	logWriter io.Writer
}

type queuedExec struct {
	req         Request
	argv        []string
	timeout     time.Duration
	enqueuedAt  time.Time
	respCh      chan Response
	coalesceKey string
}

func NewServer(opts Options, execFn ExecFunc) *Server {
	opts = opts.withDefaults()
	if execFn == nil {
		execFn = defaultExecFunc
	}
	srv := &Server{
		opts:               opts,
		execFn:             execFn,
		execQ:              make(chan queuedExec, opts.MaxQueueSize),
		stopCh:             make(chan struct{}),
		pauseCh:            make(chan struct{}),
		coalesced:          make(map[string][]chan Response),
		writeSeen:          make(map[string]time.Time),
		recipientWriteSeen: make(map[string][]time.Time),
		authState:          AuthStateReady,
	}
	srv.authProbe = defaultAuthProbe(execFn)
	srv.secureInputRunner = defaultSecureInputRunner
	srv.notifyAuth = defaultNotifyAuth(opts.NotifyProvider, opts.NotifyOpenClawCmd, opts.NotifyChannel, opts.NotifyTarget)
	srv.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	srv.logWriter = io.Discard
	return srv
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
	s.logEvent("info", "daemon_start", map[string]interface{}{
		"pid":         os.Getpid(),
		"socket_path": s.opts.SocketPath,
		"lock_path":   s.opts.LockPath,
	})

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
		s.logEvent("info", "daemon_stop", map[string]interface{}{
			"pid":         os.Getpid(),
			"socket_path": s.opts.SocketPath,
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
		if err := s.validateCommandPolicy(req.CommandPath, argv); err != nil {
			response.OK = false
			response.ExitCode = 1
			response.ErrorCode = ErrorCodeInvalidRequest
			response.Stderr = err.Error()
			break
		}
		if code, msg := s.checkDuplicateWrite(req.CommandPath, argv); code != "" {
			response.OK = false
			response.ExitCode = 1
			response.ErrorCode = code
			response.Stderr = msg
			break
		}
		if code, msg := s.checkWriteRateLimits(req.CommandPath, argv); code != "" {
			response.OK = false
			response.ExitCode = 1
			response.ErrorCode = code
			response.Stderr = msg
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
			req:         req,
			argv:        argv,
			timeout:     timeout,
			enqueuedAt:  time.Now().UTC(),
			respCh:      make(chan Response, 1),
			coalesceKey: s.coalesceKeyForRequest(req.CommandPath, argv, req.CDPPort),
		}
		if code := s.tryQueueOrCoalesce(task); code != "" {
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
		s.requestGracefulStop()
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
	s.setPaused(false)
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *Server) requestGracefulStop() {
	s.mu.Lock()
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}
	s.mu.Unlock()
	s.setPaused(false)
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
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

func (s *Server) tryQueueOrCoalesce(task queuedExec) string {
	if task.coalesceKey == "" {
		return s.tryEnqueue(task)
	}

	s.coalesceMu.Lock()
	if waiters, ok := s.coalesced[task.coalesceKey]; ok {
		s.coalesced[task.coalesceKey] = append(waiters, task.respCh)
		s.coalesceMu.Unlock()
		return ""
	}

	s.coalesced[task.coalesceKey] = []chan Response{task.respCh}
	s.coalesceMu.Unlock()

	if code := s.tryEnqueue(task); code != "" {
		s.coalesceMu.Lock()
		delete(s.coalesced, task.coalesceKey)
		s.coalesceMu.Unlock()
		return code
	}
	return ""
}

func (s *Server) completeTask(task queuedExec, resp Response) {
	if task.coalesceKey == "" {
		task.respCh <- resp
		return
	}

	s.coalesceMu.Lock()
	waiters := s.coalesced[task.coalesceKey]
	delete(s.coalesced, task.coalesceKey)
	s.coalesceMu.Unlock()

	if len(waiters) == 0 {
		task.respCh <- resp
		return
	}

	for _, waiter := range waiters {
		waiter <- resp
	}
}

func (s *Server) runWorker(ctx context.Context) {
	stopCode := ErrorCodeDaemonUnavailable
	stopMsg := "daemon unavailable"

	for {
		if s.stopRequested() {
			s.drainPending(stopCode, stopMsg)
			return
		}

		if !s.waitUntilUnpaused(ctx) {
			s.drainPending(stopCode, stopMsg)
			return
		}
		if s.stopRequested() {
			s.drainPending(stopCode, stopMsg)
			return
		}

		select {
		case <-s.stopCh:
			s.drainPending(stopCode, stopMsg)
			return
		case <-ctx.Done():
			s.drainPending(stopCode, stopMsg)
			return
		case task := <-s.execQ:
			if s.stopRequested() {
				s.completeTask(task, daemonFailureResponse(task.req.RequestID, stopCode, stopMsg))
				s.drainPending(stopCode, stopMsg)
				return
			}
			if !s.waitUntilUnpaused(ctx) {
				s.completeTask(task, daemonFailureResponse(task.req.RequestID, stopCode, stopMsg))
				s.drainPending(stopCode, stopMsg)
				return
			}
			if s.stopRequested() {
				s.completeTask(task, daemonFailureResponse(task.req.RequestID, stopCode, stopMsg))
				s.drainPending(stopCode, stopMsg)
				return
			}
			resp := s.executeTaskSafely(ctx, task)
			s.completeTask(task, resp)
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
	deadline := start.Add(task.timeout)
	resp := Response{
		RequestID:   task.req.RequestID,
		StartedAt:   start,
		QueueWaitMS: start.Sub(task.enqueuedAt).Milliseconds(),
	}

	var result ExecResult
	retriedAfterAuthRecovery := false
	readRetries := 0

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			end := time.Now().UTC()
			resp.FinishedAt = end
			resp.ExecMS = end.Sub(start).Milliseconds()
			return daemonFailureResponse(task.req.RequestID, ErrorCodeRequestTimeout, "request timeout exceeded")
		}

		execCtx, cancel := context.WithTimeout(parent, remaining)
		result = s.execFn(execCtx, task.argv, remaining)
		cancel()

		if !shouldAttemptAuthRecovery(task.req.CommandPath, task.argv, result) {
			if shouldRetryReadCommand(task.req.CommandPath, task.argv, result) && readRetries < maxReadRetries {
				delay := s.nextReadRetryDelay(readRetries)
				readRetries++

				remaining = time.Until(deadline)
				if remaining <= 0 {
					break
				}
				if delay > remaining {
					delay = remaining
				}
				if !sleepWithContext(parent, delay) {
					break
				}
				continue
			}
			break
		}

		if retriedAfterAuthRecovery {
			end := time.Now().UTC()
			resp.FinishedAt = end
			resp.ExecMS = end.Sub(start).Milliseconds()
			return authRecoveryFailureResponse(task.req.RequestID, "daemon auth recovery failed")
		}

		if ok := s.runAuthRecovery(parent); !ok {
			end := time.Now().UTC()
			resp.FinishedAt = end
			resp.ExecMS = end.Sub(start).Milliseconds()
			return authRecoveryFailureResponse(task.req.RequestID, "daemon auth recovery timed out")
		}

		retriedAfterAuthRecovery = true
	}

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

	resp.Stdout = limitResponseOutput(resp.Stdout, s.opts.MaxResponseBytes)
	resp.Stderr = limitResponseOutput(resp.Stderr, s.opts.MaxResponseBytes)
	s.logEvent("info", "request_complete", map[string]interface{}{
		"request_id":    task.req.RequestID,
		"command_path":  task.req.CommandPath,
		"exit_code":     resp.ExitCode,
		"error_code":    resp.ErrorCode,
		"queue_wait_ms": resp.QueueWaitMS,
		"exec_ms":       resp.ExecMS,
		"ok":            resp.OK,
		"stderr":        resp.Stderr,
	})

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
		case <-s.stopCh:
			return false
		case <-ch:
		}
	}
}

func (s *Server) stopRequested() bool {
	select {
	case <-s.stopCh:
		return true
	default:
		return false
	}
}

func (s *Server) drainPending(code, msg string) {
	for {
		select {
		case task := <-s.execQ:
			s.completeTask(task, daemonFailureResponse(task.req.RequestID, code, msg))
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

func limitResponseOutput(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	return value[:maxBytes]
}

func (s *Server) validateCommandPolicy(commandPath string, argv []string) error {
	path := strings.TrimSpace(commandPath)
	if path == "" {
		path = inferCommandPath(argv)
	}
	normalized := strings.ToLower(strings.TrimSpace(path))
	if normalized == "" || normalized == "help" || strings.HasPrefix(normalized, "help ") {
		return nil
	}
	if path == "" {
		return errors.New("command path is required")
	}

	policy := security.Policy{
		Readonly:  s.opts.Readonly || hasReadonlyFlag(argv),
		Allowlist: append([]string{}, s.opts.Allowlist...),
	}
	return policy.Check(path)
}

func inferCommandPath(argv []string) string {
	parts := make([]string, 0, 3)
	for _, arg := range argv {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		parts = append(parts, arg)
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, " ")
}

func hasReadonlyFlag(argv []string) bool {
	for i := 0; i < len(argv); i++ {
		arg := strings.TrimSpace(strings.ToLower(argv[i]))
		switch arg {
		case "--readonly", "-readonly":
			if i+1 < len(argv) {
				next := strings.TrimSpace(strings.ToLower(argv[i+1]))
				if next == "false" || next == "0" {
					return false
				}
			}
			return true
		case "--readonly=true", "--readonly=1":
			return true
		case "--readonly=false", "--readonly=0":
			return false
		}
	}
	return false
}

func (s *Server) checkDuplicateWrite(commandPath string, argv []string) (string, string) {
	path := strings.ToLower(strings.TrimSpace(commandPath))
	if path == "" {
		path = strings.ToLower(strings.TrimSpace(inferCommandPath(argv)))
	}
	writePath, window := s.writeSuppressionPathAndWindow(path)
	if writePath == "" || window <= 0 {
		return "", ""
	}
	if hasAllowDuplicateWriteFlag(argv) {
		return "", ""
	}

	now := time.Now().UTC()
	args := sanitizeWriteArgs(argv)
	sum := sha256.Sum256([]byte(writePath + "\x1f" + strings.Join(args, "\x1f")))
	fingerprint := fmt.Sprintf("%x", sum[:])
	key := writePath + ":" + fingerprint

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	expireBefore := now.Add(-window)
	for existingKey, seenAt := range s.writeSeen {
		if seenAt.Before(expireBefore) {
			delete(s.writeSeen, existingKey)
		}
	}

	if seenAt, ok := s.writeSeen[key]; ok {
		if now.Sub(seenAt) <= window {
			return ErrorCodeDuplicateWriteSuspected, "duplicate write suspected within suppression window"
		}
	}

	s.writeSeen[key] = now
	return "", ""
}

func (s *Server) writeSuppressionPathAndWindow(path string) (string, time.Duration) {
	switch {
	case path == "mail send" || strings.HasPrefix(path, "mail send "):
		return "mail send", s.opts.DuplicateWriteWindowMail
	case path == "mail reply" || strings.HasPrefix(path, "mail reply "):
		return "mail reply", s.opts.DuplicateWriteWindowMail
	case path == "calendar create" || strings.HasPrefix(path, "calendar create "):
		return "calendar create", s.opts.DuplicateWriteWindowCalendar
	default:
		return "", 0
	}
}

func (s *Server) checkWriteRateLimits(commandPath string, argv []string) (string, string) {
	path := strings.ToLower(strings.TrimSpace(commandPath))
	if path == "" {
		path = strings.ToLower(strings.TrimSpace(inferCommandPath(argv)))
	}
	writePath := canonicalWritePath(path)
	if writePath == "" {
		return "", ""
	}

	now := time.Now().UTC()
	windowStart := now.Add(-time.Minute)

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.writeEvents = pruneTimesSince(s.writeEvents, windowStart)
	if s.opts.WriteRateLimitPerMinute > 0 && len(s.writeEvents) >= s.opts.WriteRateLimitPerMinute {
		return ErrorCodeWriteThrottled, "global write rate limit exceeded"
	}

	if s.opts.RecipientWriteRateLimitPerMinute > 0 {
		recipients := extractWriteRecipients(writePath, argv)
		for _, recipient := range recipients {
			events := pruneTimesSince(s.recipientWriteSeen[recipient], windowStart)
			if len(events) >= s.opts.RecipientWriteRateLimitPerMinute {
				s.recipientWriteSeen[recipient] = events
				return ErrorCodeWriteThrottled, "recipient write rate limit exceeded"
			}
			s.recipientWriteSeen[recipient] = events
		}
	}

	s.writeEvents = append(s.writeEvents, now)
	if s.opts.RecipientWriteRateLimitPerMinute > 0 {
		recipients := extractWriteRecipients(writePath, argv)
		for _, recipient := range recipients {
			s.recipientWriteSeen[recipient] = append(s.recipientWriteSeen[recipient], now)
		}
	}

	return "", ""
}

func canonicalWritePath(path string) string {
	best := ""
	for candidate := range security.WriteCommands {
		if path == candidate || strings.HasPrefix(path, candidate+" ") {
			if len(candidate) > len(best) {
				best = candidate
			}
		}
	}
	return best
}

func pruneTimesSince(values []time.Time, since time.Time) []time.Time {
	if len(values) == 0 {
		return values
	}
	out := values[:0]
	for _, v := range values {
		if !v.Before(since) {
			out = append(out, v)
		}
	}
	return out
}

func extractWriteRecipients(writePath string, argv []string) []string {
	switch writePath {
	case "mail send", "mail reply":
	default:
		return nil
	}

	values := make(map[string]struct{})
	for i := 0; i < len(argv); i++ {
		arg := strings.TrimSpace(strings.ToLower(argv[i]))
		switch {
		case arg == "--to" || arg == "--cc" || arg == "--bcc":
			if i+1 < len(argv) {
				addRecipientValues(values, argv[i+1])
				i++
			}
		case strings.HasPrefix(arg, "--to="):
			addRecipientValues(values, argv[i][len("--to="):])
		case strings.HasPrefix(arg, "--cc="):
			addRecipientValues(values, argv[i][len("--cc="):])
		case strings.HasPrefix(arg, "--bcc="):
			addRecipientValues(values, argv[i][len("--bcc="):])
		}
	}

	out := make([]string, 0, len(values))
	for v := range values {
		out = append(out, v)
	}
	return out
}

func addRecipientValues(dst map[string]struct{}, raw string) {
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	}) {
		recipient := strings.TrimSpace(strings.ToLower(part))
		if recipient == "" {
			continue
		}
		dst[recipient] = struct{}{}
	}
}

func hasAllowDuplicateWriteFlag(argv []string) bool {
	for _, arg := range argv {
		val := strings.TrimSpace(strings.ToLower(arg))
		switch val {
		case "--allow-duplicate-write", "--allow-duplicate-write=true", "--allow-duplicate-write=1":
			return true
		}
	}
	return false
}

func sanitizeWriteArgs(argv []string) []string {
	out := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		arg := strings.TrimSpace(argv[i])
		if arg == "" {
			continue
		}
		lower := strings.ToLower(arg)
		if lower == "--allow-duplicate-write" {
			if i+1 < len(argv) {
				next := strings.TrimSpace(strings.ToLower(argv[i+1]))
				if next == "true" || next == "false" || next == "1" || next == "0" {
					i++
				}
			}
			continue
		}
		if strings.HasPrefix(lower, "--allow-duplicate-write=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func (s *Server) coalesceKeyForRequest(commandPath string, argv []string, cdpPort int) string {
	if !s.opts.CoalesceIdenticalReads {
		return ""
	}

	path := strings.ToLower(strings.TrimSpace(commandPath))
	if path == "" {
		path = strings.ToLower(strings.TrimSpace(inferCommandPath(argv)))
	}
	path = canonicalCoalescePath(path)
	if !isCoalescableReadPath(path) {
		return ""
	}

	parts := make([]string, 0, len(argv))
	for _, arg := range argv {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		parts = append(parts, arg)
	}

	return path + "|cdp=" + strconv.Itoa(cdpPort) + "|argv=" + strings.Join(parts, "\x1f")
}

func isCoalescableReadPath(path string) bool {
	switch path {
	case "mail search",
		"mail view",
		"mail thread get",
		"mail attachments list",
		"calendar list",
		"calendar get",
		"auth status",
		"browser status",
		"daemon status":
		return true
	default:
		return false
	}
}

func canonicalCoalescePath(path string) string {
	candidates := []string{
		"mail search",
		"mail view",
		"mail thread get",
		"mail attachments list",
		"calendar list",
		"calendar get",
		"auth status",
		"browser status",
		"daemon status",
	}
	for _, candidate := range candidates {
		if path == candidate || strings.HasPrefix(path, candidate+" ") {
			return candidate
		}
	}
	return path
}
