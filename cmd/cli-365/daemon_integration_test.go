package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/lc0rp/cli-365/internal/browser"
)

type binaryResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func TestDaemonAutoStartAndReuseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binPath := buildCLIBinary(t)
	stateHome := t.TempDir()
	homeDir := t.TempDir()

	run := func(args ...string) binaryResult {
		return runBinary(t, binPath, stateHome, homeDir, args...)
	}

	// Best-effort cleanup for all test paths.
	defer func() {
		_ = run("daemon", "stop")
	}()

	first := run("--daemon", "help")
	if first.exitCode != 0 {
		t.Fatalf("first --daemon call exit=%d stderr=%q", first.exitCode, first.stderr)
	}

	status1 := run("--json", "daemon", "status")
	if status1.exitCode != 0 {
		t.Fatalf("daemon status exit=%d stderr=%q", status1.exitCode, status1.stderr)
	}

	var s1 struct {
		Running bool `json:"running"`
		PID     int  `json:"pid"`
	}
	if err := json.Unmarshal([]byte(status1.stdout), &s1); err != nil {
		t.Fatalf("parse status1: %v (stdout=%q)", err, status1.stdout)
	}
	if !s1.Running || s1.PID <= 0 {
		t.Fatalf("status1 running=%v pid=%d", s1.Running, s1.PID)
	}

	second := run("--daemon", "help")
	if second.exitCode != 0 {
		t.Fatalf("second --daemon call exit=%d stderr=%q", second.exitCode, second.stderr)
	}

	status2 := run("--json", "daemon", "status")
	if status2.exitCode != 0 {
		t.Fatalf("daemon status (second) exit=%d stderr=%q", status2.exitCode, status2.stderr)
	}

	var s2 struct {
		Running bool `json:"running"`
		PID     int  `json:"pid"`
	}
	if err := json.Unmarshal([]byte(status2.stdout), &s2); err != nil {
		t.Fatalf("parse status2: %v (stdout=%q)", err, status2.stdout)
	}
	if !s2.Running || s2.PID <= 0 {
		t.Fatalf("status2 running=%v pid=%d", s2.Running, s2.PID)
	}
	if s2.PID != s1.PID {
		t.Fatalf("daemon PID changed between calls: first=%d second=%d", s1.PID, s2.PID)
	}
}

func TestDaemonStartCommandIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binPath := buildCLIBinary(t)
	stateHome := t.TempDir()
	homeDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "daemon.log")

	run := func(args ...string) binaryResult {
		return runBinary(t, binPath, stateHome, homeDir, args...)
	}

	defer func() {
		_ = run("daemon", "stop")
	}()

	start := run("daemon", "start", "--log-file", logPath)
	if start.exitCode != 0 {
		t.Fatalf("daemon start exit=%d stderr=%q", start.exitCode, start.stderr)
	}

	status1 := run("--json", "daemon", "status")
	if status1.exitCode != 0 {
		t.Fatalf("daemon status exit=%d stderr=%q", status1.exitCode, status1.stderr)
	}
	var s1 struct {
		Running bool `json:"running"`
		PID     int  `json:"pid"`
	}
	if err := json.Unmarshal([]byte(status1.stdout), &s1); err != nil {
		t.Fatalf("parse status1: %v (stdout=%q)", err, status1.stdout)
	}
	if !s1.Running || s1.PID <= 0 {
		t.Fatalf("status1 running=%v pid=%d", s1.Running, s1.PID)
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("log path missing %q: %v", logPath, err)
	}
	browserStatus := run("--json", "browser", "status")
	if browserStatus.exitCode != 0 {
		t.Fatalf("browser status exit=%d stderr=%q", browserStatus.exitCode, browserStatus.stderr)
	}
	var b1 struct {
		Running bool `json:"running"`
	}
	if err := json.Unmarshal([]byte(browserStatus.stdout), &b1); err != nil {
		t.Fatalf("parse browser status: %v (stdout=%q)", err, browserStatus.stdout)
	}
	if !b1.Running {
		if shouldSkipBrowserIntegration(start.stderr) {
			t.Skipf("daemon browser warmup prerequisites unavailable: %s", strings.TrimSpace(start.stderr))
		}
		t.Fatalf("daemon start should warm browser, running=%v stderr=%q", b1.Running, start.stderr)
	}

	startAgain := run("daemon", "start", "--log-file", logPath)
	if startAgain.exitCode != 0 {
		t.Fatalf("daemon start (again) exit=%d stderr=%q", startAgain.exitCode, startAgain.stderr)
	}

	status2 := run("--json", "daemon", "status")
	if status2.exitCode != 0 {
		t.Fatalf("daemon status (second) exit=%d stderr=%q", status2.exitCode, status2.stderr)
	}
	var s2 struct {
		Running bool `json:"running"`
		PID     int  `json:"pid"`
	}
	if err := json.Unmarshal([]byte(status2.stdout), &s2); err != nil {
		t.Fatalf("parse status2: %v (stdout=%q)", err, status2.stdout)
	}
	if !s2.Running || s2.PID <= 0 {
		t.Fatalf("status2 running=%v pid=%d", s2.Running, s2.PID)
	}
	if s2.PID != s1.PID {
		t.Fatalf("daemon PID changed between start calls: first=%d second=%d", s1.PID, s2.PID)
	}
}

func TestDaemonBrowserStartPrimaryTabReuseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binPath := buildCLIBinary(t)
	stateHome := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("HOME", homeDir)

	run := func(args ...string) binaryResult {
		return runBinary(t, binPath, stateHome, homeDir, args...)
	}

	// Best-effort cleanup for all test paths.
	defer func() {
		_ = run("daemon", "stop")
		_ = run("browser", "stop")
	}()

	first := run("--daemon", "browser", "start")
	if first.exitCode != 0 {
		if shouldSkipBrowserIntegration(first.stderr) {
			t.Skipf("browser integration prerequisites unavailable: %s", strings.TrimSpace(first.stderr))
		}
		t.Fatalf("first --daemon browser start exit=%d stderr=%q", first.exitCode, first.stderr)
	}

	second := run("--daemon", "browser", "start")
	if second.exitCode != 0 {
		if shouldSkipBrowserIntegration(second.stderr) {
			t.Skipf("browser integration prerequisites unavailable: %s", strings.TrimSpace(second.stderr))
		}
		t.Fatalf("second --daemon browser start exit=%d stderr=%q", second.exitCode, second.stderr)
	}

	rt, err := browser.LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime() error: %v", err)
	}
	b, err := browser.ConnectEndpoint(rt.WSEndpoint)
	if err != nil {
		t.Fatalf("ConnectEndpoint() error: %v", err)
	}

	pages, err := b.Pages()
	if err != nil {
		t.Fatalf("Pages() error: %v", err)
	}

	owaCount := 0
	for _, page := range pages {
		info, err := page.Info()
		if err != nil || info == nil {
			continue
		}
		url := strings.ToLower(strings.TrimSpace(info.URL))
		if strings.Contains(url, "outlook.office.com") || strings.Contains(url, "outlook.office365.com") || strings.Contains(url, "outlook.live.com") || strings.Contains(url, "outlook.cloud.microsoft") {
			owaCount++
		}
	}
	if owaCount == 0 {
		t.Fatalf("expected at least one OWA tab after daemon browser start, got %d", owaCount)
	}
	if owaCount > 1 {
		t.Fatalf("expected one primary OWA tab after repeated daemon browser start, got %d", owaCount)
	}

	status := run("--json", "daemon", "status")
	if status.exitCode != 0 {
		t.Fatalf("daemon status exit=%d stderr=%q", status.exitCode, status.stderr)
	}
	var ds struct {
		Running bool `json:"running"`
		PID     int  `json:"pid"`
	}
	if err := json.Unmarshal([]byte(status.stdout), &ds); err != nil {
		t.Fatalf("parse daemon status: %v (stdout=%q)", err, status.stdout)
	}
	if !ds.Running || ds.PID <= 0 {
		t.Fatalf("daemon status running=%v pid=%d", ds.Running, ds.PID)
	}
}

func TestDaemonBrowserCrashRecoveryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binPath := buildCLIBinary(t)
	stateHome := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("HOME", homeDir)

	run := func(args ...string) binaryResult {
		return runBinary(t, binPath, stateHome, homeDir, args...)
	}

	defer func() {
		_ = run("daemon", "stop")
		_ = run("browser", "stop")
	}()

	first := run("--daemon", "browser", "start")
	if first.exitCode != 0 {
		if shouldSkipBrowserIntegration(first.stderr) {
			t.Skipf("browser integration prerequisites unavailable: %s", strings.TrimSpace(first.stderr))
		}
		t.Fatalf("first --daemon browser start exit=%d stderr=%q", first.exitCode, first.stderr)
	}

	status1 := run("--json", "browser", "status")
	if status1.exitCode != 0 {
		t.Fatalf("browser status exit=%d stderr=%q", status1.exitCode, status1.stderr)
	}
	var b1 struct {
		Running    bool   `json:"running"`
		PID        int    `json:"pid"`
		WSEndpoint string `json:"ws_endpoint"`
	}
	if err := json.Unmarshal([]byte(status1.stdout), &b1); err != nil {
		t.Fatalf("parse browser status1: %v (stdout=%q)", err, status1.stdout)
	}
	if !b1.Running {
		t.Fatal("browser status1 running=false, want true")
	}
	if b1.PID <= 0 {
		t.Skipf("browser status1 pid unavailable (pid=%d), skipping crash-recovery integration", b1.PID)
	}

	proc, err := os.FindProcess(b1.PID)
	if err != nil {
		t.Fatalf("FindProcess(%d): %v", b1.PID, err)
	}
	if err := proc.Kill(); err != nil {
		t.Fatalf("kill browser pid %d: %v", b1.PID, err)
	}

	second := run("--daemon", "browser", "start")
	if second.exitCode != 0 {
		t.Fatalf("second --daemon browser start exit=%d stderr=%q", second.exitCode, second.stderr)
	}

	status2 := run("--json", "browser", "status")
	if status2.exitCode != 0 {
		t.Fatalf("browser status2 exit=%d stderr=%q", status2.exitCode, status2.stderr)
	}
	var b2 struct {
		Running    bool   `json:"running"`
		PID        int    `json:"pid"`
		WSEndpoint string `json:"ws_endpoint"`
	}
	if err := json.Unmarshal([]byte(status2.stdout), &b2); err != nil {
		t.Fatalf("parse browser status2: %v (stdout=%q)", err, status2.stdout)
	}
	if !b2.Running {
		t.Fatal("browser status2 running=false, want true")
	}
	if b2.PID <= 0 && b2.WSEndpoint == "" {
		t.Fatalf("browser status2 missing pid/ws endpoint: %+v", b2)
	}
	if b2.PID == b1.PID && b2.WSEndpoint == b1.WSEndpoint {
		t.Fatalf("expected recovered browser identity to change after crash, got same pid=%d ws=%q", b2.PID, b2.WSEndpoint)
	}
}

func TestDaemonClosedPrimaryTabRecoveryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binPath := buildCLIBinary(t)
	stateHome := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("HOME", homeDir)

	run := func(args ...string) binaryResult {
		return runBinary(t, binPath, stateHome, homeDir, args...)
	}

	defer func() {
		_ = run("daemon", "stop")
		_ = run("browser", "stop")
	}()

	first := run("--daemon", "browser", "start")
	if first.exitCode != 0 {
		if shouldSkipBrowserIntegration(first.stderr) {
			t.Skipf("browser integration prerequisites unavailable: %s", strings.TrimSpace(first.stderr))
		}
		t.Fatalf("first --daemon browser start exit=%d stderr=%q", first.exitCode, first.stderr)
	}

	rt, err := browser.LoadRuntime()
	if err != nil {
		t.Fatalf("LoadRuntime() error: %v", err)
	}
	b, err := browser.ConnectEndpoint(rt.WSEndpoint)
	if err != nil {
		t.Fatalf("ConnectEndpoint() error: %v", err)
	}

	if err := closeAllOWATabs(b); err != nil {
		t.Fatalf("closeAllOWATabs() error: %v", err)
	}

	second := run("--daemon", "browser", "start")
	if second.exitCode != 0 {
		t.Fatalf("second --daemon browser start exit=%d stderr=%q", second.exitCode, second.stderr)
	}

	if err := waitForSingleOWATab(b, 3*time.Second); err != nil {
		t.Fatalf("waitForSingleOWATab() error: %v", err)
	}
}

func buildCLIBinary(t *testing.T) string {
	t.Helper()

	root := repoRoot(t)
	binPath := filepath.Join(t.TempDir(), "cli-365")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/cli-365")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(out))
	}
	return binPath
}

func runBinary(t *testing.T, binPath, stateHome, homeDir string, args ...string) binaryResult {
	t.Helper()

	cmd := exec.Command(binPath, args...)
	env := append([]string{}, os.Environ()...)
	env = append(env, "XDG_STATE_HOME="+stateHome)
	env = append(env, "HOME="+homeDir)
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := binaryResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	if err == nil {
		result.exitCode = 0
	} else if exitErr, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitErr.ExitCode()
	} else {
		t.Fatalf("exec %q failed: %v", strings.Join(append([]string{binPath}, args...), " "), err)
	}
	return result
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func shouldSkipBrowserIntegration(stderr string) bool {
	lower := strings.ToLower(stderr)
	for _, needle := range []string{
		"chrome",
		"chromium",
		"executable file not found",
		"no such file or directory",
		"cannot find",
		"sandbox",
		"permission denied",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func closeAllOWATabs(b *rod.Browser) error {
	pages, err := b.Pages()
	if err != nil {
		return err
	}

	closedAny := false
	for _, page := range pages {
		info, err := page.Info()
		if err != nil || info == nil {
			continue
		}
		if !isOWAURL(info.URL) {
			continue
		}
		_ = page.Close()
		closedAny = true
	}
	if !closedAny {
		return nil
	}
	return nil
}

func waitForSingleOWATab(b *rod.Browser, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		count, err := countOWATabs(b)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if count == 1 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	count, err := countOWATabs(b)
	if err != nil {
		return err
	}
	return fmt.Errorf("expected one OWA tab after recovery, got %d", count)
}

func countOWATabs(b *rod.Browser) (int, error) {
	pages, err := b.Pages()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, page := range pages {
		info, err := page.Info()
		if err != nil || info == nil {
			continue
		}
		if isOWAURL(info.URL) {
			count++
		}
	}
	return count, nil
}

func isOWAURL(raw string) bool {
	url := strings.ToLower(strings.TrimSpace(raw))
	if url == "" {
		return false
	}
	if !strings.Contains(url, "outlook.office.com") &&
		!strings.Contains(url, "outlook.office365.com") &&
		!strings.Contains(url, "outlook.live.com") &&
		!strings.Contains(url, "outlook.cloud.microsoft") {
		return false
	}
	return strings.Contains(url, "/mail") ||
		strings.Contains(url, "/owa/") ||
		strings.Contains(url, "/calendar")
}
