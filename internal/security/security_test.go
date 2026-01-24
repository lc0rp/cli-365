package security

import (
	"testing"
)

func TestPolicyCheckReadonly(t *testing.T) {
	tests := []struct {
		name        string
		readonly    bool
		commandPath string
		wantErr     bool
	}{
		// Readonly mode - read commands allowed
		{"readonly: mail search", true, "mail search", false},
		{"readonly: mail view", true, "mail view", false},
		{"readonly: mail thread get", true, "mail thread get", false},
		{"readonly: mail attachments list", true, "mail attachments list", false},
		{"readonly: auth status", true, "auth status", false},
		{"readonly: browser status", true, "browser status", false},

		// Readonly mode - write commands blocked
		{"readonly: mail send", true, "mail send", true},
		{"readonly: mail draft create", true, "mail draft create", true},
		{"readonly: mail draft update", true, "mail draft update", true},
		{"readonly: mail draft delete", true, "mail draft delete", true},
		{"readonly: mail draft send", true, "mail draft send", true},

		// Non-readonly mode - all allowed
		{"normal: mail send", false, "mail send", false},
		{"normal: mail draft create", false, "mail draft create", false},
		{"normal: mail search", false, "mail search", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Policy{Readonly: tt.readonly}
			err := p.CheckReadonly(tt.commandPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckReadonly() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPolicyCheckAllowlist(t *testing.T) {
	tests := []struct {
		name        string
		allowlist   []string
		commandPath string
		wantErr     bool
	}{
		// Empty allowlist allows all
		{"empty allowlist: mail search", []string{}, "mail search", false},
		{"empty allowlist: mail send", []string{}, "mail send", false},

		// Allowlist with mail - allows all mail subcommands
		{"mail allowed: mail search", []string{"mail"}, "mail search", false},
		{"mail allowed: mail send", []string{"mail"}, "mail send", false},
		{"mail allowed: mail draft create", []string{"mail"}, "mail draft create", false},
		{"mail allowed: browser blocked", []string{"mail"}, "browser start", true},

		// Specific subcommand allowlist
		{"mail search only: mail search", []string{"mail search"}, "mail search", false},
		{"mail search only: mail view blocked", []string{"mail search"}, "mail view", true},

		// Multiple allowlist entries
		{"mail+browser: mail search", []string{"mail", "browser"}, "mail search", false},
		{"mail+browser: browser start", []string{"mail", "browser"}, "browser start", false},
		{"mail+browser: auth blocked", []string{"mail", "browser"}, "auth login", true},

		// Auth in allowlist
		{"auth allowed: auth login", []string{"auth", "mail"}, "auth login", false},
		{"auth allowed: auth status", []string{"auth", "mail"}, "auth status", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Policy{Allowlist: tt.allowlist}
			err := p.CheckAllowlist(tt.commandPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckAllowlist() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPolicyCheck(t *testing.T) {
	tests := []struct {
		name        string
		policy      Policy
		commandPath string
		wantErr     bool
	}{
		// Both checks pass
		{
			name:        "allowed and not readonly",
			policy:      Policy{Readonly: false, Allowlist: []string{"mail"}},
			commandPath: "mail send",
			wantErr:     false,
		},
		// Readonly blocks
		{
			name:        "allowed but readonly blocks",
			policy:      Policy{Readonly: true, Allowlist: []string{"mail"}},
			commandPath: "mail send",
			wantErr:     true,
		},
		// Allowlist blocks
		{
			name:        "not allowed",
			policy:      Policy{Readonly: false, Allowlist: []string{"browser"}},
			commandPath: "mail search",
			wantErr:     true,
		},
		// Both would block (allowlist checked first)
		{
			name:        "not allowed and readonly",
			policy:      Policy{Readonly: true, Allowlist: []string{"browser"}},
			commandPath: "mail send",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Check(tt.commandPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReadonlyCommandsMap(t *testing.T) {
	// Verify key commands are in the map
	readCmds := []string{
		"mail search",
		"mail view",
		"mail thread get",
		"mail attachments list",
		"auth status",
		"browser status",
	}

	for _, cmd := range readCmds {
		if !ReadonlyCommands[cmd] {
			t.Errorf("expected %q to be in ReadonlyCommands", cmd)
		}
	}
}

func TestWriteCommandsMap(t *testing.T) {
	// Verify key commands are in the map
	writeCmds := []string{
		"mail send",
		"mail draft create",
		"mail draft update",
		"mail draft delete",
		"mail draft send",
	}

	for _, cmd := range writeCmds {
		if !WriteCommands[cmd] {
			t.Errorf("expected %q to be in WriteCommands", cmd)
		}
	}
}

func TestCommandPath(t *testing.T) {
	tests := []struct {
		parts []string
		want  string
	}{
		{[]string{"mail"}, "mail"},
		{[]string{"mail", "search"}, "mail search"},
		{[]string{"mail", "draft", "create"}, "mail draft create"},
		{[]string{}, ""},
	}

	for _, tt := range tests {
		got := CommandPath(tt.parts...)
		if got != tt.want {
			t.Errorf("CommandPath(%v) = %q, want %q", tt.parts, got, tt.want)
		}
	}
}

func TestCaseInsensitivity(t *testing.T) {
	p := Policy{Readonly: true, Allowlist: []string{"MAIL"}}

	// Should match case-insensitively
	if err := p.CheckAllowlist("mail search"); err != nil {
		t.Errorf("allowlist should be case-insensitive: %v", err)
	}

	// Write check should also be case-insensitive
	if err := p.CheckReadonly("MAIL SEND"); err == nil {
		t.Error("readonly check should block 'MAIL SEND'")
	}
}
