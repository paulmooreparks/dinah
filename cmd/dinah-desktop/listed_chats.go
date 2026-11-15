package main

import (
	"fmt"
	"time"

	"studio/internal/acp"
	"studio/internal/layout"
	"studio/internal/sessionhost"
)

// listedChat retains provider metadata separately from the person's title.
// It cannot change a removal reservation or establish a switch relationship.
type listedChat struct {
	Provider   string `json:"provider"`
	Workspace  string `json:"workspace"`
	Title      string `json:"title,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
	ObservedAt string `json:"observedAt"`
}

func (a *App) recordListedSessions(provider, workspace string, rows []acp.SessionInfo) error {
	safe := make(map[string]listedChat)
	for _, row := range rows {
		if row.SessionID == "" || !layout.SamePath(row.Cwd, workspace) || sessionhost.NotChat(row.SessionID) {
			continue
		}
		title := row.Title
		if fromAppletContext(title) {
			title = ""
		}
		safe[row.SessionID] = listedChat{Provider: provider, Workspace: row.Cwd, Title: title, UpdatedAt: row.UpdatedAt, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	if len(safe) == 0 {
		return nil
	}
	return a.updateSettings(func(s *Settings) error {
		for id, row := range safe {
			if _, held := s.RemovedChats[id]; held {
				continue
			}
			if prior := s.ListedChats[id]; prior.Provider != "" && (prior.Provider != provider || !layout.SamePath(prior.Workspace, row.Workspace)) {
				return fmt.Errorf("listed chat %q conflicts with its recorded program or workspace", id)
			}
			if saved := s.SessionProviders[id]; saved != "" && saved != provider {
				return fmt.Errorf("listed chat %q conflicts with its saved program", id)
			}
		}
		if s.ListedChats == nil {
			s.ListedChats = map[string]listedChat{}
		}
		for id, row := range safe {
			if _, held := s.RemovedChats[id]; held {
				continue
			}
			if row.Title == "" {
				row.Title = s.ListedChats[id].Title
			}
			s.ListedChats[id] = row
		}
		return nil
	})
}
