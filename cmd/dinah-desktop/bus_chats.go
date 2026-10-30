package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"path/filepath"

	"studio/internal/bus"
	"studio/internal/bushost"
	"studio/internal/layout"
	"studio/internal/mcpserver"
	"studio/internal/sessionhost"
)

// Every open chat is a session on Dinah's message bus. The window registers
// it, gives its agent the bus's tools through a server of its own, and
// delivers its messages as prompts: a chat that is idle when a message
// arrives starts a turn with it, and a message the person writes carries any
// that are waiting. The agent therefore needs no hooks and no channel, and
// every provider is woken the same way.
//
// The bus file is the one in the window's state folder, which is the bus's
// home folder outside tests (settings.go, configDir).

// busChatLabel is how the chat's log marks a prompt the bus delivered.
const busChatLabel = sessionhost.BusChatLabel

func (a *App) busHost() *bushost.Service {
	a.busMu.Lock()
	defer a.busMu.Unlock()
	if a.busService == nil {
		a.busService = bushost.New(bushost.Options{
			State: configDir(), Labels: a.busWorkspaceLabels, Reconcile: func(ctx context.Context, b *bus.Bus, reported map[string]bool) {
				a.busCoordinator().ReconcileResults(ctx, b, reported)
			},
			Deliver: func(ctx context.Context, b *bus.Bus) { a.busCoordinator().DeliverRound(ctx, b) }, Notice: a.noticeMentions, Wake: func(ctx context.Context, b *bus.Bus) { a.busCoordinator().ConnectWaking(ctx, b) }, Emit: a.emit,
			Log: func(level, format string, args ...any) { logf("bus", level, format, args...) },
		})
	}
	return a.busService
}

// busSessionID returns the bus session a chat keeps across reopening, making
// one for a chat that has none. A new chat has no ID until its agent makes
// the session, and its bus server must be named before that, so the bus ID
// is made first and saved under the chat's ID once the chat has one
// (saveBusSession).
func (a *App) busSessionID(chat string) string {
	if chat != "" {
		a.mu.Lock()
		id := a.settings.BusSessions[chat]
		a.mu.Unlock()
		if id != "" {
			return id
		}
	}
	return bus.WindowSessionPrefix + rand.Text()
}

// BusChatRef is a chat the person chose in the picker of a group chat: its
// ID in the window, its folder and its title as the window shows it.
type BusChatRef struct {
	ID    string `json:"id"`
	Cwd   string `json:"cwd"`
	Title string `json:"title"`
}

// busChat returns the bus session of a chat chosen for a group. An open
// chat has its session already. A closed chat keeps the session it had when
// it was last open, or is given one now, and the bus records it as known but
// not open (bus.KnowSession), so a group can have it and messages wait for
// it; when the chat opens, it joins the bus under that session
// (busSessionID). The person's own name for the chat comes before the title
// the page has.
func (a *App) busChat(ref BusChatRef) (string, error) {
	if ref.ID == "" {
		return "", errors.New("say which chat")
	}
	if s := a.session(ref.ID); s != nil {
		id := s.BusID()
		if id != "" {
			return id, nil
		}
	}
	if ref.Cwd == "" || !filepath.IsAbs(ref.Cwd) {
		return "", errors.New("the chat's folder is not known")
	}
	a.mu.Lock()
	id := a.settings.BusSessions[ref.ID]
	if t := a.settings.SessionTitles[ref.ID]; t != "" {
		ref.Title = t
	}
	a.mu.Unlock()
	fresh := id == ""
	if fresh {
		id = a.busSessionID("")
	}
	err := a.busHost().With(func(b *bus.Bus) error {
		for p, label := range a.busWorkspaceLabels() {
			if layout.SamePath(p, ref.Cwd) {
				if _, err := b.LabelWorkspace(context.Background(), p, label); err != nil {
					return err
				}
			}
		}
		_, err := b.KnowSession(context.Background(), id, ref.Cwd, ref.Title)
		return err
	})
	if err != nil {
		return "", err
	}
	if fresh {
		a.saveBusSession(ref.ID, id)
	}
	return id, nil
}

// busChats returns the bus sessions of the chats chosen for a group.
func (a *App) busChats(refs []BusChatRef) ([]string, error) {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		id, err := a.busChat(r)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// chatOfBus returns the window's chat whose bus session id is, or "".
func (a *App) chatOfBus(id string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for chat, bid := range a.settings.BusSessions {
		if bid == id {
			return chat
		}
	}
	return ""
}

// busChatState is a group member's state as the window shows it: working,
// open, background or closed. A chat open in the window is working while a
// turn runs, and background while it is open in the background, without a
// window, whether a turn runs or not (bus_wake.go); one that closed itself
// there is closed. A session outside the window is open while the bus has
// it open.
func (a *App) busChatState(chat string, open bool) string {
	if s := a.session(chat); chat != "" && s != nil {
		state := s.State()
		switch {
		case state.Parked:
			return "closed"
		case state.InBackground:
			return "background"
		case state.Working:
			return "working"
		case state.Open:
			return "open"
		}
		return "closed"
	}
	if open && chat == "" {
		return "open"
	}
	return "closed"
}

func (a *App) saveBusSession(chat, id string) {
	err := a.updateSettings(func(s *Settings) error {
		if s.BusSessions == nil {
			s.BusSessions = map[string]string{}
		}
		s.BusSessions[chat] = id
		return nil
	})
	if err != nil {
		logf("bus", "error", "keeping chat %s's place on the bus: %v", chat, err)
	}
}

// busServer is the bus's MCP server for one chat: this program, started as
// a helper that serves the chat's bus session (runBusMCP). It is named
// dinah-bus because the person's own Claude Code settings may also start a
// server called dinah in the same agent, which stands down there.
func busServer(id string) (map[string]any, error) {
	executable, err := appletHelperExecutable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, err
	}
	return map[string]any{"name": "dinah-bus", "command": executable,
		"args": []string{"--bus-mcp", "--state-dir", configDir(), "--session", id}, "env": []any{}}, nil
}

// withBusServer adds a chat's bus server to the servers its session starts
// with, registering the chat on the bus in its workspace. A chat the bus
// cannot take still opens, without the bus.
func (a *App) withBusServer(id, cwd, title string, servers []any) []any {
	out, err := a.registerBusServer(id, cwd, title, servers)
	if err != nil {
		logf("bus", "error", "putting a chat in %s on the bus: %v", cwd, err)
		return servers
	}
	return out
}

// adoptBusServer is withBusServer for a chat that already has its bus
// identity, which a provider switch carries to the chat's new session. It
// reports a failure instead of opening the chat without the bus, since the
// switch must not leave the chat without its place on it.
func (a *App) adoptBusServer(id, cwd string, servers []any) ([]any, error) {
	return a.registerBusServer(id, cwd, "", servers)
}

func (a *App) registerBusServer(id, cwd, title string, servers []any) ([]any, error) {
	err := a.busHost().With(func(b *bus.Bus) error {
		// The workspace is labelled first, so an untitled chat is shown in
		// its workspace's name from the start (busWorkspaces).
		for p, label := range a.busWorkspaceLabels() {
			if layout.SamePath(p, cwd) {
				if _, err := b.LabelWorkspace(context.Background(), p, label); err != nil {
					return err
				}
			}
		}
		_, err := b.RegisterSession(context.Background(), id, cwd, title)
		return err
	})
	var server map[string]any
	if err == nil {
		server, err = busServer(id)
	}
	if err != nil {
		return nil, err
	}
	return append(append([]any{}, servers...), server), nil
}

// moveBusSession gives a bus identity to one chat and takes it from another,
// in one settings write, so no saved state has both chats or neither.
func (a *App) moveBusSession(from, to, id string) error {
	return a.updateSettings(func(s *Settings) error {
		if s.BusSessions == nil {
			s.BusSessions = map[string]string{}
		}
		delete(s.BusSessions, from)
		s.BusSessions[to] = id
		return nil
	})
}

// busIdentityOf is the bus session a chat answers to: the one it is joined
// to now, or else the one saved for it.
func (a *App) busIdentityOf(s *Session) string {
	if id := s.BusID(); id != "" {
		return id
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.BusSessions[s.ID()]
}

// busWorkspaceLabels returns the name of each of the window's workspaces by
// the folder its agents work in. Two workspaces in one folder share it on
// the bus, which takes the first one's name.
func (a *App) busWorkspaceLabels() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[string]string{}
	for _, r := range a.settings.WorkspaceRecords {
		if _, ok := out[r.ExecutionPath]; r.ExecutionPath != "" && !ok {
			out[r.ExecutionPath] = r.Name
		}
	}
	return out
}

// runBusMCP serves the bus's tools to one chat's agent, as the helper
// busServer starts.
func runBusMCP(args []string) error {
	flags := flag.NewFlagSet("bus-mcp", flag.ContinueOnError)
	stateDir := flags.String("state-dir", "", product.Name+" state directory")
	session := flags.String("session", "", "the chat's session on the bus")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *stateDir == "" || *session == "" || flags.NArg() != 0 {
		return errors.New("the bus helper needs --state-dir and --session")
	}
	b, err := bus.OpenAt(filepath.Join(*stateDir, "bus.db"))
	if err != nil {
		return err
	}
	defer b.Close()
	if _, err := b.Session(context.Background(), *session); err != nil {
		return err
	}
	return mcpserver.NewWindow(b, *session).Run(context.Background())
}
