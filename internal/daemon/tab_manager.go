package daemon

import (
	"strings"

	"github.com/go-rod/rod"

	"github.com/lc0rp/cli-365/internal/browser"
)

type tabSnapshot struct {
	ID  string
	URL string
}

type tabBrowserConn struct {
	Endpoint string
	Browser  *rod.Browser
}

type tabPlan struct {
	PrimaryID string
	CloseIDs  []string
}

func shouldMaintainPrimaryTab(commandPath string, argv []string) bool {
	path := strings.ToLower(strings.TrimSpace(commandPath))
	if path == "" {
		path = strings.ToLower(strings.TrimSpace(inferCommandPath(argv)))
	}
	switch {
	case strings.HasPrefix(path, "mail "):
		return true
	case strings.HasPrefix(path, "calendar "):
		return true
	case strings.HasPrefix(path, "auth login"):
		return true
	case strings.HasPrefix(path, "debug "):
		return true
	default:
		return false
	}
}

func planPrimaryTab(existingPrimary string, pages []tabSnapshot) tabPlan {
	isOWA := make(map[string]bool, len(pages))
	orderedOWA := make([]string, 0, len(pages))
	blankIDs := make([]string, 0, len(pages))
	for _, p := range pages {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			continue
		}
		if isOWAURLForDaemon(p.URL) {
			isOWA[id] = true
			orderedOWA = append(orderedOWA, id)
			continue
		}
		if isAboutBlankURL(p.URL) {
			blankIDs = append(blankIDs, id)
		}
	}

	primary := ""
	if existingPrimary != "" && isOWA[existingPrimary] {
		primary = existingPrimary
	} else if len(orderedOWA) > 0 {
		primary = orderedOWA[0]
	}

	if primary == "" {
		return tabPlan{}
	}

	closeSet := make(map[string]struct{}, len(blankIDs)+len(orderedOWA))
	for _, id := range orderedOWA {
		if id != primary {
			closeSet[id] = struct{}{}
		}
	}
	for _, id := range blankIDs {
		if id != primary {
			closeSet[id] = struct{}{}
		}
	}

	closeIDs := make([]string, 0, len(closeSet))
	for _, p := range pages {
		if _, ok := closeSet[p.ID]; ok {
			closeIDs = append(closeIDs, p.ID)
		}
	}

	return tabPlan{
		PrimaryID: primary,
		CloseIDs:  closeIDs,
	}
}

func (s *Server) getPrimaryTabID() string {
	s.tabMu.Lock()
	defer s.tabMu.Unlock()
	return s.primaryTabID
}

func (s *Server) setPrimaryTabID(id string) {
	s.tabMu.Lock()
	s.primaryTabID = strings.TrimSpace(id)
	s.tabMu.Unlock()
}

func (s *Server) getTabBrowser(endpoint string) *rod.Browser {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}

	s.tabMu.Lock()
	defer s.tabMu.Unlock()

	if s.tabConn != nil && s.tabConn.Endpoint == endpoint && s.tabConn.Browser != nil {
		return s.tabConn.Browser
	}

	conn, err := browser.ConnectEndpoint(endpoint)
	if err != nil {
		s.tabConn = nil
		s.primaryTabID = ""
		return nil
	}
	s.tabConn = &tabBrowserConn{
		Endpoint: endpoint,
		Browser:  conn,
	}
	return conn
}

func (s *Server) resetTabBrowser() {
	s.tabMu.Lock()
	s.tabConn = nil
	s.primaryTabID = ""
	s.tabMu.Unlock()
}

func (s *Server) maintainPrimaryOWATab() {
	rt, err := browser.LoadRuntime()
	if err != nil || rt == nil || strings.TrimSpace(rt.WSEndpoint) == "" {
		return
	}

	b := s.getTabBrowser(rt.WSEndpoint)
	if b == nil {
		return
	}

	pages, err := b.Pages()
	if err != nil {
		s.resetTabBrowser()
		return
	}

	snapshots := make([]tabSnapshot, 0, len(pages))
	pageByID := make(map[string]interface{ Close() error }, len(pages))
	for _, p := range pages {
		info, err := p.Info()
		if err != nil || info == nil {
			continue
		}
		id := strings.TrimSpace(string(info.TargetID))
		if id == "" {
			continue
		}
		snapshots = append(snapshots, tabSnapshot{
			ID:  id,
			URL: info.URL,
		})
		pageByID[id] = p
	}

	plan := planPrimaryTab(s.getPrimaryTabID(), snapshots)
	for _, id := range plan.CloseIDs {
		if p, ok := pageByID[id]; ok {
			_ = p.Close()
		}
	}
	s.setPrimaryTabID(plan.PrimaryID)
}

func isAboutBlankURL(raw string) bool {
	return strings.EqualFold(strings.TrimSpace(raw), "about:blank")
}

func isOWAURLForDaemon(raw string) bool {
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
