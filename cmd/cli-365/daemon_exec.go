package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lc0rp/cli-365/internal/daemon"
)

var stdioCaptureMu sync.Mutex

func daemonExecFunc(maxResponseBytes int, daemonConfigPath string) daemon.ExecFunc {
	daemonConfigPath = strings.TrimSpace(daemonConfigPath)
	return func(ctx context.Context, argv []string, timeout time.Duration) daemon.ExecResult {
		return runCLIInProcess(ctx, argv, timeout, maxResponseBytes, daemonConfigPath)
	}
}

func runCLIInProcess(parent context.Context, argv []string, timeout time.Duration, maxCaptureBytes int, daemonConfigPath string) daemon.ExecResult {
	if len(argv) == 0 {
		return daemon.ExecResult{
			ExitCode: 1,
			Err:      context.Canceled,
		}
	}

	runCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	commandArgv := append([]string{}, argv...)
	if daemonConfigPath != "" && !hasConfigArg(commandArgv) {
		commandArgv = append([]string{"--config", daemonConfigPath}, commandArgv...)
	}

	args := append([]string{"cli-365"}, commandArgv...)
	exitCode := 0

	stdout, stderr, captureErr := captureProcessStdio(func() {
		exitCode = runCLI(runCtx, args, cliAppOptions{
			DisableDaemonForwarding: true,
		})
	}, maxCaptureBytes)

	result := daemon.ExecResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}
	if captureErr != nil {
		result.Err = captureErr
		if result.ExitCode == 0 {
			result.ExitCode = 1
		}
		return result
	}
	if runCtx.Err() != nil {
		result.Err = runCtx.Err()
		if result.ExitCode == 0 {
			result.ExitCode = 1
		}
	}
	return result
}

func hasConfigArg(argv []string) bool {
	for i := 0; i < len(argv); i++ {
		arg := strings.TrimSpace(argv[i])
		switch {
		case arg == "--config", arg == "-c":
			return true
		case strings.HasPrefix(arg, "-c="):
			return true
		case strings.HasPrefix(arg, "--config="):
			return true
		}
	}
	return false
}

func captureProcessStdio(run func(), maxBytes int) (string, string, error) {
	stdioCaptureMu.Lock()
	defer stdioCaptureMu.Unlock()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return "", "", err
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		return "", "", err
	}

	type readResult struct {
		data string
		err  error
	}

	readPipe := func(file *os.File, ch chan<- readResult) {
		defer close(ch)
		data, readErr := readBoundedOutput(file, maxBytes)
		_ = file.Close()
		ch <- readResult{data: data, err: readErr}
	}

	stdoutCh := make(chan readResult, 1)
	stderrCh := make(chan readResult, 1)
	go readPipe(stdoutR, stdoutCh)
	go readPipe(stderrR, stderrCh)

	origStdout := os.Stdout
	origStderr := os.Stderr
	os.Stdout = stdoutW
	os.Stderr = stderrW

	run()

	closeErr := firstError(stdoutW.Close(), stderrW.Close())
	os.Stdout = origStdout
	os.Stderr = origStderr

	stdoutRes := <-stdoutCh
	stderrRes := <-stderrCh

	return stdoutRes.data, stderrRes.data, firstError(closeErr, stdoutRes.err, stderrRes.err)
}

func readBoundedOutput(r io.Reader, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		data, err := io.ReadAll(r)
		return string(data), err
	}

	var out bytes.Buffer
	buf := make([]byte, 32*1024)
	written := 0

	for {
		n, err := r.Read(buf)
		if n > 0 && written < maxBytes {
			keep := n
			if written+keep > maxBytes {
				keep = maxBytes - written
			}
			if keep > 0 {
				_, _ = out.Write(buf[:keep])
				written += keep
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return out.String(), nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
