package main

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/lc0rp/cli-365/internal/daemon"
)

var stdioCaptureMu sync.Mutex

func daemonExecFunc() daemon.ExecFunc {
	return func(ctx context.Context, argv []string, timeout time.Duration) daemon.ExecResult {
		return runCLIInProcess(ctx, argv, timeout)
	}
}

func runCLIInProcess(parent context.Context, argv []string, timeout time.Duration) daemon.ExecResult {
	if len(argv) == 0 {
		return daemon.ExecResult{
			ExitCode: 1,
			Err:      context.Canceled,
		}
	}

	runCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	args := append([]string{"cli-365"}, append([]string{}, argv...)...)
	exitCode := 0

	stdout, stderr, captureErr := captureProcessStdio(func() {
		exitCode = runCLI(runCtx, args, cliAppOptions{
			DisableDaemonForwarding: true,
		})
	})

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

func captureProcessStdio(run func()) (string, string, error) {
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
		data, readErr := io.ReadAll(file)
		_ = file.Close()
		ch <- readResult{data: string(data), err: readErr}
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

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
