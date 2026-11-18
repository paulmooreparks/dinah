package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"studio/internal/acp"
	"studio/internal/layout"
	"studio/internal/sessionhost"
)

// listedChat retains provider metadata separately from the person's title.
// It cannot change a removal reservation or establish a switch relationship.
type listedChat struct {
	Provider         string                  `json:"provider"`
	Workspace        string                  `json:"workspace"`
	Title            string                  `json:"title,omitempty"`
	LocalTitle       string                  `json:"localTitle,omitempty"`
	LocalObservation bool                    `json:"localObservation,omitempty"`
	Repair           string                  `json:"repair,omitempty"`
	UpdatedAt        string                  `json:"updatedAt,omitempty"`
	ObservedAt       string                  `json:"observedAt"`
	Freshness        string                  `json:"freshness,omitempty"`
	Sequence         uint64                  `json:"sequence,omitempty"`
	Conflicts        []listedChatObservation `json:"conflicts,omitempty"`
}

// Conflicting observations are evidence only and never grant ownership.
type listedChatObservation struct {
	Provider   string `json:"provider"`
	Workspace  string `json:"workspace"`
	Title      string `json:"title,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
	ObservedAt string `json:"observedAt,omitempty"`
	Sequence   uint64 `json:"sequence,omitempty"`
}

func retainListingConflict(prior, observed listedChat, reason string) listedChat {
	prior.Repair = reason
	evidence := listedChatObservation{observed.Provider, observed.Workspace, observed.Title, observed.UpdatedAt, observed.ObservedAt, observed.Sequence}
	for _, existing := range prior.Conflicts {
		if existing == evidence {
			return prior
		}
	}
	prior.Conflicts = append(prior.Conflicts, evidence)
	return prior
}

type listingScope struct {
	Provider  string `json:"provider"`
	Workspace string `json:"workspace"`
	Next      uint64 `json:"next"`
	Accepted  uint64 `json:"accepted"`
	Complete  bool   `json:"complete"`
}

func listingScopeKey(provider, workspace string) string {
	workspace = filepath.Clean(workspace)
	if runtime.GOOS == "windows" {
		workspace = strings.ToLower(workspace)
	}
	data, _ := json.Marshal([]string{provider, workspace})
	return string(data)
}

func (a *App) beginListing(provider, workspace string) (uint64, error) {
	if provider == "" || !filepath.IsAbs(workspace) {
		return 0, errors.New("listing needs a program and absolute workspace")
	}
	var sequence uint64
	err := a.updateSettings(func(s *Settings) error {
		if s.ListingScopes == nil {
			s.ListingScopes = map[string]listingScope{}
		}
		key := listingScopeKey(provider, workspace)
		scope := s.ListingScopes[key]
		if scope.Next == ^uint64(0) {
			return errors.New("listing sequence exhausted")
		}
		scope.Provider, scope.Workspace = provider, workspace
		scope.Next++
		sequence = scope.Next
		s.ListingScopes[key] = scope
		return nil
	})
	return sequence, err
}

func (a *App) recordListedSessions(provider, workspace string, rows []acp.SessionInfo) error {
	sequence, err := a.beginListing(provider, workspace)
	if err != nil {
		return err
	}
	return a.finishListing(provider, workspace, sequence, rows, false)
}

func (a *App) finishListing(provider, workspace string, sequence uint64, rows []acp.SessionInfo, complete bool) error {
	safe := make(map[string]listedChat)
	for _, row := range rows {
		if row.SessionID == "" || !layout.SamePath(row.Cwd, workspace) {
			return errors.New("native listing contains an invalid identity or workspace scope")
		}
		if sessionhost.NotChat(row.SessionID) {
			continue
		}
		title := row.Title
		if fromAppletContext(title) {
			title = ""
		}
		safe[row.SessionID] = listedChat{Provider: provider, Workspace: row.Cwd, Title: title, UpdatedAt: row.UpdatedAt, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Freshness: "lastKnown", Sequence: sequence}
	}
	var conflictErr error
	err := a.updateSettings(func(s *Settings) error {
		key := listingScopeKey(provider, workspace)
		scope, exists := s.ListingScopes[key]
		if !exists || sequence == 0 || sequence > scope.Next {
			return errors.New("unrecognized listing refresh")
		}
		if sequence <= scope.Accepted {
			return nil
		}
		if s.ListedChats == nil {
			s.ListedChats = map[string]listedChat{}
		}
		for id, row := range safe {
			if _, held := s.RemovedChats[id]; held {
				continue
			}
			if prior := s.ListedChats[id]; prior.Provider != "" && (prior.Provider != provider || !layout.SamePath(prior.Workspace, row.Workspace)) {
				problem := fmt.Errorf("listed chat %q conflicts with its recorded program or workspace", id)
				s.ListedChats[id] = retainListingConflict(prior, row, problem.Error())
				conflictErr = errors.Join(conflictErr, problem)
				continue
			}
			if saved := s.SessionProviders[id]; saved != "" && saved != provider {
				problem := fmt.Errorf("listed chat %q conflicts with its saved program", id)
				prior, exists := s.ListedChats[id]
				if !exists {
					prior = row
				}
				s.ListedChats[id] = retainListingConflict(prior, row, problem.Error())
				conflictErr = errors.Join(conflictErr, problem)
			}
		}
		// Commit the hold before reporting failure. A contradictory refresh cannot
		// establish completeness or replace any other observation.
		if conflictErr != nil {
			scope.Accepted, scope.Complete = sequence, false
			s.ListingScopes[key] = scope
			return nil
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
			row.LocalTitle = s.ListedChats[id].LocalTitle
			row.LocalObservation = s.ListedChats[id].LocalObservation
			row.Repair = s.ListedChats[id].Repair
			row.Conflicts = s.ListedChats[id].Conflicts
			s.ListedChats[id] = row
		}
		if complete {
			for id, row := range s.ListedChats {
				if row.Provider != provider || !layout.SamePath(row.Workspace, workspace) {
					continue
				}
				if _, held := s.RemovedChats[id]; held {
					continue
				}
				if _, seen := safe[id]; !seen {
					row.Freshness, row.Sequence = "missing", sequence
					s.ListedChats[id] = row
				}
			}
		}
		scope.Accepted, scope.Complete = sequence, complete
		s.ListingScopes[key] = scope
		return nil
	})
	return errors.Join(err, conflictErr)
}

// Runtime titles have separate provenance because an opened conversation may
// already carry an effective title. They never overwrite a native listing title.
func (a *App) captureRuntimeInventory() error {
	a.mu.Lock()
	if a.startupErr != nil {
		err := a.startupErr
		a.mu.Unlock()
		return err
	}
	rows := map[string]SessionInfo{}
	for _, chat := range a.runtime.ConversationsLocked() {
		info := chat.Info()
		cached, present := a.settings.ListedChats[info.ID]
		if _, held := a.settings.RemovedChats[info.ID]; held || sessionhost.NotChat(info.ID) {
			continue
		}
		if cached.Repair == "" && (!present || (info.Title != "" && !fromAppletContext(info.Title) && cached.LocalTitle != info.Title)) {
			rows[info.ID] = info
		}
	}
	a.mu.Unlock()
	if len(rows) == 0 {
		return nil
	}
	return a.updateSettings(func(s *Settings) error {
		if s.ListedChats == nil {
			s.ListedChats = map[string]listedChat{}
		}
		for id, info := range rows {
			if _, held := s.RemovedChats[id]; held {
				continue
			}
			if info.Provider == "" || !filepath.IsAbs(info.Cwd) {
				continue
			}
			row, present := s.ListedChats[id]
			if present && (row.Provider != info.Provider || !layout.SamePath(row.Workspace, info.Cwd)) {
				continue
			}
			if !present {
				row = listedChat{Provider: info.Provider, Workspace: info.Cwd, Freshness: "lastKnown", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			}
			row.LocalObservation = true
			if info.Title != "" && !fromAppletContext(info.Title) {
				row.LocalTitle = info.Title
			}
			row.UpdatedAt = sessionhost.LatestActivity(row.UpdatedAt, info.UpdatedAt)
			s.ListedChats[id] = row
		}
		return nil
	})
}

// The independent projection survives an older build rewriting settings
// without the cache fields. It contains no user overrides or execution grants.
type listedChatProjection struct {
	Version int                     `json:"version"`
	Chats   map[string]listedChat   `json:"chats"`
	Scopes  map[string]listingScope `json:"scopes"`
}

func saveListedChatProjection(s Settings) error {
	path := filepath.Join(configDir(), "chat-inventory.json")
	if len(s.ListedChats) == 0 && len(s.ListingScopes) == 0 {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
	}
	data, err := json.Marshal(listedChatProjection{Version: 1, Chats: s.ListedChats, Scopes: s.ListingScopes})
	if err != nil {
		return err
	}
	return writeStateFile(path, data)
}

func reconcileListedChatProjection(s *Settings) error {
	data, err := readStateFile(filepath.Join(configDir(), "chat-inventory.json"))
	if errors.Is(err, os.ErrNotExist) {
		return saveListedChatProjection(*s)
	}
	if err != nil {
		return err
	}
	return mergeListedChatProjection(s, data)
}

// mergeListedChatProjection merges the saved projection, data, into the
// settings, as loading the settings does. It writes nothing, so a reader of
// the window's state that must not change it, such as the command line's
// workspace export (workspace_export_option.go), can use it.
func mergeListedChatProjection(s *Settings, data []byte) error {
	var projection listedChatProjection
	if err := json.Unmarshal(data, &projection); err != nil {
		return fmt.Errorf("read chat inventory: %w", err)
	}
	if projection.Version != 1 {
		return errors.New("unsupported chat inventory version")
	}
	if s.ListedChats == nil {
		s.ListedChats = map[string]listedChat{}
	}
	for id, row := range projection.Chats {
		if r, held := s.RemovedChats[id]; held && (r.Stage == "deleted" || r.Stage == "removed") {
			continue
		}
		if id == "" || row.Provider == "" || !filepath.IsAbs(row.Workspace) {
			return errors.New("chat inventory contains invalid identity metadata")
		}
		if existing, ok := s.ListedChats[id]; ok && (existing.Provider != row.Provider || !layout.SamePath(existing.Workspace, row.Workspace)) {
			existing = retainListingConflict(existing, row, "Recorded chat program or workspace conflicts with its durable inventory. Repair ownership before using this chat.")
			s.ListedChats[id] = existing
			continue
		}
		if existing, ok := s.ListedChats[id]; ok {
			if existing.Repair != "" {
				// An older projection or later listing cannot release a repair hold.
				continue
			}
			if existing.Sequence == 0 && row.Sequence != 0 && (existing.Title != row.Title || existing.UpdatedAt != row.UpdatedAt || existing.ObservedAt != row.ObservedAt) {
				s.ListedChats[id] = retainListingConflict(existing, row, "A legacy listing changed recorded metadata without an orderable revision. Review both observations before using this chat.")
				continue
			}
		}
		if existing := s.ListedChats[id]; existing.Sequence > row.Sequence {
			continue
		}
		s.ListedChats[id] = row
	}
	if s.ListingScopes == nil {
		s.ListingScopes = map[string]listingScope{}
	}
	for key, scope := range projection.Scopes {
		if key != listingScopeKey(scope.Provider, scope.Workspace) || scope.Accepted > scope.Next {
			return errors.New("chat inventory contains invalid refresh scope")
		}
		if existing := s.ListingScopes[key]; existing.Next > scope.Next {
			continue
		}
		s.ListingScopes[key] = scope
	}
	return nil
}
