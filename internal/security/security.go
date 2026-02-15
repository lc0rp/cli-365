// Package security provides access control and policy enforcement.
package security

import (
	"fmt"
	"strings"
)

// Policy represents the security policy for CLI operations.
type Policy struct {
	Readonly  bool
	Allowlist []string
}

// ReadonlyCommands are commands that are safe in readonly mode.
var ReadonlyCommands = map[string]bool{
	// Auth commands (all safe)
	"auth":        true,
	"auth login":  true,
	"auth logout": true,
	"auth status": true,

	// Browser commands (all safe)
	"browser":        true,
	"browser start":  true,
	"browser status": true,
	"browser stop":   true,

	// Daemon commands (all safe control-plane)
	"daemon":        true,
	"daemon run":    true,
	"daemon status": true,
	"daemon stop":   true,
	"daemon ping":   true,

	// Mail read operations
	"mail":               true,
	"mail search":        true,
	"mail view":          true,
	"mail thread":        true,
	"mail thread get":    true,
	"mail attachments":   true,
	"mail attachments list":     true,
	"mail attachments download": true,

	// Debug commands (read-only discovery)
	"debug":          true,
	"debug discover": true,
}

// WriteCommands are commands that modify data.
var WriteCommands = map[string]bool{
	"mail draft":        true,
	"mail draft create": true,
	"mail draft update": true,
	"mail draft delete": true,
	"mail draft send":   true,
	"mail send":         true,
}

// CheckReadonly returns an error if the command is not allowed in readonly mode.
func (p *Policy) CheckReadonly(commandPath string) error {
	if !p.Readonly {
		return nil
	}

	// Normalize command path
	cmd := strings.TrimSpace(strings.ToLower(commandPath))

	// Check if it's a write command
	if WriteCommands[cmd] {
		return fmt.Errorf("command %q is not allowed in readonly mode", commandPath)
	}

	// Also check if any prefix is a write command
	parts := strings.Fields(cmd)
	for i := len(parts); i > 0; i-- {
		prefix := strings.Join(parts[:i], " ")
		if WriteCommands[prefix] {
			return fmt.Errorf("command %q is not allowed in readonly mode", commandPath)
		}
	}

	return nil
}

// CheckAllowlist returns an error if the command is not in the allowlist.
func (p *Policy) CheckAllowlist(commandPath string) error {
	if len(p.Allowlist) == 0 {
		return nil // Empty allowlist means allow all
	}

	cmd := strings.TrimSpace(strings.ToLower(commandPath))
	parts := strings.Fields(cmd)

	// Check if command or any prefix is in allowlist
	for i := len(parts); i > 0; i-- {
		prefix := strings.Join(parts[:i], " ")
		for _, allowed := range p.Allowlist {
			allowed = strings.TrimSpace(strings.ToLower(allowed))
			if prefix == allowed || strings.HasPrefix(prefix, allowed+" ") {
				return nil
			}
		}
	}

	// Also check if first word (top-level command) is allowed
	if len(parts) > 0 {
		for _, allowed := range p.Allowlist {
			allowed = strings.TrimSpace(strings.ToLower(allowed))
			if parts[0] == allowed {
				return nil
			}
		}
	}

	return fmt.Errorf("command %q is not in allowlist", commandPath)
}

// Check validates both readonly and allowlist policies.
func (p *Policy) Check(commandPath string) error {
	if err := p.CheckAllowlist(commandPath); err != nil {
		return err
	}
	return p.CheckReadonly(commandPath)
}

// CommandPath builds a command path from parts.
func CommandPath(parts ...string) string {
	return strings.Join(parts, " ")
}
