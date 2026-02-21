package daemon

import (
	"context"
	"testing"
)

func TestShouldAttemptAuthRecoverySignals(t *testing.T) {
	tests := []struct {
		name        string
		commandPath string
		argv        []string
		result      ExecResult
		want        bool
	}{
		{
			name:        "success result",
			commandPath: "calendar list",
			argv:        []string{"calendar", "list"},
			result:      ExecResult{ExitCode: 0},
			want:        false,
		},
		{
			name:        "canary missing",
			commandPath: "calendar list",
			argv:        []string{"calendar", "list"},
			result:      ExecResult{ExitCode: 1, Stderr: "canary token not found - are you logged in?"},
			want:        true,
		},
		{
			name:        "canary extraction failed",
			commandPath: "mail search",
			argv:        []string{"mail", "search", "invoice"},
			result:      ExecResult{ExitCode: 1, Stderr: "failed to extract canary: page eval failed"},
			want:        true,
		},
		{
			name:        "login host startupdata request",
			commandPath: "calendar list",
			argv:        []string{"calendar", "list"},
			result:      ExecResult{ExitCode: 1, Stderr: "fetch https://login.microsoftonline.com/owa/startupdata.ashx?app=Mail&n=0 failed"},
			want:        true,
		},
		{
			name:        "missing bearer token",
			commandPath: "calendar list",
			argv:        []string{"calendar", "list"},
			result:      ExecResult{ExitCode: 1, Stderr: "missing bearer token"},
			want:        true,
		},
		{
			name:        "mailbox info unavailable is not auth recovery",
			commandPath: "calendar add-from-directory",
			argv:        []string{"calendar", "add-from-directory", "Example User"},
			result:      ExecResult{ExitCode: 1, Stderr: "mailbox info unavailable; unable to determine mailbox smtp address"},
			want:        false,
		},
		{
			name:        "auth login timeout triggers recovery",
			commandPath: "auth login",
			argv:        []string{"auth", "login"},
			result:      ExecResult{ExitCode: 1, Stderr: "login timeout"},
			want:        true,
		},
		{
			name:        "deadline exceeded does not trigger",
			commandPath: "calendar list",
			argv:        []string{"calendar", "list"},
			result:      ExecResult{ExitCode: 1, Err: context.DeadlineExceeded, Stderr: "status 401 unauthorized"},
			want:        false,
		},
		{
			name:        "non mail and calendar command ignored",
			commandPath: "daemon status",
			argv:        []string{"daemon", "status"},
			result:      ExecResult{ExitCode: 1, Stderr: "status 401 unauthorized"},
			want:        false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAttemptAuthRecovery(tt.commandPath, tt.argv, tt.result)
			if got != tt.want {
				t.Fatalf("shouldAttemptAuthRecovery() = %v, want %v", got, tt.want)
			}
		})
	}
}
