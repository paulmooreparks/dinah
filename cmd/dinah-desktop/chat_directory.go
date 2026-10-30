package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"studio/internal/applet"
	"studio/internal/bus"
	"studio/internal/layout"
)

//go:embed chat_directory.go
var chatDirectorySource []byte

const chatDirectoryAction = "dinah.chat/list@2"
const chatFindAction = "dinah.chat/find@1"

// The states a chat in the directory can be in: open in a window or in this
// run, open in the background without a window for a message that woke it
// (bus_wake.go), or closed. A closed chat can still be asked, since asking
// it opens it in the background (chat_ask.go).
const (
	chatStateOpen       = "open"
	chatStateBackground = "open-in-background"
	chatStateClosed     = "closed"
)

type chatDirectoryEntry struct {
	SessionID   string `json:"sessionId"`
	Title       string `json:"title"`
	Workspace   string `json:"workspace"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	Provider    string `json:"provider"`
	// State and Working are the old vocabulary, kept until the page reads ChatState.
	State     string `json:"state"`
	Working   bool   `json:"working"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	// ChatState is the chat's life, window and execution state (chat_state.go).
	ChatState
	// cwd is the chat's folder, and named the title the chat has of its
	// own, empty for an untitled chat, whose Title is "Untitled Chat in"
	// its workspace as the bus shows it. Neither is part of the output.
	cwd   string
	named string
}

func (a *App) chatWorkspaceName(cwd string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, w := range a.settings.WorkspaceRecords {
		if layout.SamePath(w.ExecutionPath, cwd) {
			return w.Name
		}
	}
	return filepath.Base(cwd)
}

// chatDirectory lists the chats this application knows across its
// workspaces: those open in this run, and the closed chats of the window's
// workspaces that Dinah keeps a saved copy of (continuation.go), less the
// hidden and deleted ones and those continued as another chat. A closed
// chat's title is the person's name for it, else the title the bus last had
// for it; an untitled chat is shown as the bus shows it, "Untitled Chat in"
// its workspace. Open chats come first, then closed ones, each newest first.
// It neither connects an agent nor reads a conversation.
func (a *App) chatDirectory() []chatDirectoryEntry {
	links, records := conversationLinks()
	out := []chatDirectoryEntry{}
	seen := map[string]bool{}
	a.mu.Lock()
	for _, s := range a.runtime.ConversationsLocked() {
		seen[s.ID()] = true
		if a.hidden(s.ID()) {
			continue
		}
		state := s.State()
		if state.Superseded {
			continue
		}
		e := chatDirectoryEntry{SessionID: s.ID(), named: state.Title, Provider: normalizedProvider(s.Provider()), Working: state.Working, UpdatedAt: state.UpdatedAt, cwd: s.Cwd(), State: chatStateOpen}
		switch {
		case !state.Open || state.Parked:
			e.State, e.Working = chatStateClosed, false
		case state.InBackground:
			e.State = chatStateBackground
		}

		if title := a.settings.SessionTitles[s.ID()]; title != "" {
			e.named = title
		}
		out = append(out, e)
	}
	busIDs := map[string]string{}
	for id, tr := range records {
		if seen[id] || tr.Deleted || links[id] != "" || a.hidden(id) || tr.Workspace == "" || !a.windowWorkspaceLocked(tr.Workspace) {
			continue
		}
		out = append(out, chatDirectoryEntry{SessionID: id, named: a.settings.SessionTitles[id], Provider: normalizedProvider(tr.Provider), cwd: tr.Workspace, State: chatStateClosed})
		if b := a.settings.BusSessions[id]; b != "" {
			busIDs[id] = b
		}
	}
	a.mu.Unlock()
	// A closed chat takes its title and when it was last seen from the bus,
	// where the window wrote them while it was open (publishBusTitle).
	if len(busIDs) > 0 {
		err := a.busHost().With(func(b *bus.Bus) error {
			for i := range out {
				id := busIDs[out[i].SessionID]
				if id == "" {
					continue
				}
				s, err := b.Session(context.Background(), id)
				if errors.Is(err, bus.ErrNotFound) {
					continue
				}
				if err != nil {
					return err
				}
				if out[i].named == "" {
					out[i].named = s.Title
				}
				out[i].UpdatedAt = s.LastSeen
			}
			return nil
		})
		if err != nil {
			logf("bus", "error", "reading closed chats' titles from the bus: %v", err)
		}
	}
	a.mu.Lock()
	for i := range out {
		e := &out[i]
		e.Workspace = filepath.Base(e.cwd)
		for _, w := range a.settings.WorkspaceRecords {
			if layout.SamePath(w.ExecutionPath, e.cwd) {
				e.Workspace, e.WorkspaceID = w.Name, w.ID
				break
			}
		}
		e.Title = bus.DisplayTitle(e.named, e.Workspace)
	}
	a.mu.Unlock()
	fill := func(b *bus.Bus) {
		ctx := context.Background()
		for i := range out {
			id := out[i].SessionID
			busID := a.knownBusID(id)
			retired := false
			if busID != "" && b != nil {
				_, retired, _ = b.LeadOf(ctx, busID)
			}
			out[i].ChatState = a.chatState(ctx, b, busID, id, retired, false)
		}
	}
	if err := a.busHost().With(func(b *bus.Bus) error { fill(b); return nil }); err != nil {
		logf("bus", "error", "reading the chats' states from the bus: %v", err)
		fill(nil)
	}
	sort.Slice(out, func(i, j int) bool {
		if ci, cj := out[i].State == chatStateClosed, out[j].State == chatStateClosed; ci != cj {
			return cj
		}
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].SessionID < out[j].SessionID
	})
	return out
}

// windowWorkspaceLocked reports whether cwd is the folder of one of the
// window's workspaces. The caller holds a.mu.
func (a *App) windowWorkspaceLocked(cwd string) bool {
	for _, w := range a.settings.WorkspaceRecords {
		if layout.SamePath(w.ExecutionPath, cwd) {
			return true
		}
	}
	return false
}

// directoryChat returns the chat id as the directory lists it, or false.
func (a *App) directoryChat(id string) (chatDirectoryEntry, bool) {
	for _, e := range a.chatDirectory() {
		if e.SessionID == id {
			return e, true
		}
	}
	return chatDirectoryEntry{}, false
}

func (a *App) chatDirectoryHandler(find bool) applet.Handler {
	return func(ctx context.Context, input string) (applet.Result, error) {
		if _, err := builtinWorkspace(ctx); err != nil {
			return applet.Result{}, err
		}
		var in struct {
			Name      string `json:"name"`
			Workspace string `json:"workspace"`
			Limit     int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return applet.Result{}, err
		}
		query := strings.ToLower(strings.TrimSpace(in.Name))
		if find && query == "" {
			return applet.Result{}, errors.New("give the chat's name to find it")
		}
		matches, exact := []chatDirectoryEntry{}, []chatDirectoryEntry{}
		for _, entry := range a.chatDirectory() {
			if in.Workspace != "" && !strings.EqualFold(in.Workspace, entry.Workspace) && in.Workspace != entry.WorkspaceID {
				continue
			}
			name := strings.ToLower(entry.Title)
			if strings.Contains(name, query) || entry.SessionID == in.Name {
				matches = append(matches, entry)
				if name == query || entry.SessionID == in.Name {
					exact = append(exact, entry)
				}
			}
		}
		if find && len(exact) > 0 {
			matches = exact
		}
		status := "not-found"
		if len(matches) == 1 {
			status = "unique"
		} else if len(matches) > 1 {
			status = "ambiguous"
		}
		limit := chatAppletLimit(input)
		truncated := len(matches) > limit
		if truncated {
			matches = matches[:limit]
		}
		return builtinJSON(map[string]any{"chats": matches, "truncated": truncated, "coverage": "open-application", "match": status})
	}
}
