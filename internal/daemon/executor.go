package daemon

import (
	"context"
	"errors"
	"time"
)

func defaultExecFunc(ctx context.Context, argv []string, timeout time.Duration) ExecResult {
	_ = timeout
	if len(argv) == 0 {
		return ExecResult{
			ExitCode: 1,
			Err:      context.Canceled,
		}
	}
	return ExecResult{
		ExitCode: 1,
		Err:      errors.New("daemon exec function is not configured"),
	}
}
