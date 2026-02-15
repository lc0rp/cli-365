package daemon

import (
	"context"
	"time"
)

const defaultSessionMaintenanceInterval = 30 * time.Second

func (s *Server) runPrimaryMaintenance() {
	if s == nil {
		return
	}
	if s.maintainPrimaryFn != nil {
		s.maintainPrimaryFn()
		return
	}
	s.maintainPrimaryOWATab()
}

func (s *Server) runSessionMaintenance(ctx context.Context) {
	if s == nil {
		return
	}
	if !s.shouldCleanupManagedBrowser() {
		return
	}

	interval := s.maintenanceInterval
	if interval <= 0 {
		interval = defaultSessionMaintenanceInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if s.stopRequested() {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.runPrimaryMaintenance()
		}
	}
}
