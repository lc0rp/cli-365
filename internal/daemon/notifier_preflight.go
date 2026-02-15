package daemon

import (
	"os/exec"
	"strings"
)

type lookupPathFunc func(file string) (string, error)

func defaultLookupPath(file string) (string, error) {
	return exec.LookPath(file)
}

func notifierCommandName(provider, openClawCmd string) string {
	if strings.ToLower(strings.TrimSpace(provider)) != "openclaw-cli" {
		return ""
	}
	parts := strings.Fields(strings.TrimSpace(openClawCmd))
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func (s *Server) logNotifierAvailability() {
	if s == nil {
		return
	}

	command := notifierCommandName(s.opts.NotifyProvider, s.opts.NotifyOpenClawCmd)
	if command == "" {
		return
	}

	lookup := s.lookupPath
	if lookup == nil {
		lookup = defaultLookupPath
	}
	if _, err := lookup(command); err != nil {
		s.logEvent("warn", "notifier_unavailable", map[string]interface{}{
			"provider": s.opts.NotifyProvider,
			"command":  command,
			"error":    err.Error(),
		})
	}
}
