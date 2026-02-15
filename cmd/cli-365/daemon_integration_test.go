package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
