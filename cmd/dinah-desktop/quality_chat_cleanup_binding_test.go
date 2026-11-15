//go:build quality_chat_cleanup && chatcleanup_acceptance

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"studio/internal/bus"
	"testing"
	"time"
)

type qualityHostCleanup struct {
	t         *testing.T
	a         *App
	ws        string
	now       time.Time
	seeds     map[string]qualityCleanupSeed
	copies    map[string]bool
	events    []qualityCleanupEvent
	providers *chatCleanupAcceptanceProviders
	connects  *connectRequests
	wakes     int
	addresses map[string]string
	groups    map[string]string
	linked    *qualityLinkedState
}

func init() {
	qualityCleanupFactory = func(t *testing.T) qualityCleanupFixture {
		providers := chatCleanupAcceptanceCountProviders(t)
		a := newFileAccessFixtureApp(t)
		providers.Bind(t, a)
		f := &qualityHostCleanup{t: t, a: a, ws: t.TempDir(), now: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), seeds: map[string]qualityCleanupSeed{}, copies: map[string]bool{}}
		f.providers = providers
		f.connects = watchConnects(a)
		f.addresses = map[string]string{}
		f.groups = map[string]string{}
		f.install()
		return f
	}
}
func (f *qualityHostCleanup) install() {
	f.a.chatCleanupAcceptanceInstall(chatCleanupAcceptanceHooks{Now: func() time.Time { return f.now }, UnloadProvider: func(context.Context, string, string) (bool, error) { return false, nil }, DeleteProvider: func(_ context.Context, _, id string) (bool, error) {
		r, ok := f.a.cleanupRecord(id)
		if !ok || r.Group == "" {
			f.t.Fatal("provider delete without retained archive")
		}
		if _, e := os.Stat(filepath.Join(chatCleanupArchiveDir(r.Group), "manifest.json")); e != nil {
			f.t.Fatal("provider delete after archive erasure", e)
		}
		mode := f.seeds[id].ProviderDelete
		if mode == "unsupported" {
			return false, nil
		}
		f.events = append(f.events, qualityCleanupEvent{id, "provider-delete"})
		if mode == "failing" {
			return true, errors.New("invented deletion failure")
		}
		f.copies[id] = false
		return true, nil
	}})
}
func (f *qualityHostCleanup) Seed(s qualityCleanupSeed) {
	f.seeds[s.ID] = s
	f.copies[s.ID] = true
	provider := strings.SplitN(s.ID, ":", 2)[0]
	if s.Loaded {
		provider = "codex"
	}
	recipient := ""
	if s.Queued > 0 || s.Loaded {
		owner := s.ID
		if len(s.Chain) > 0 {
			owner = s.Chain[0]
		}
		recipient = "window-quality-" + strings.ReplaceAll(owner, ":", "-")
	}
	workspace := f.ws
	if s.Relation == "workspace-move" {
		workspace = f.t.TempDir()
	}
	cleanupFixtureChat(f.t, f.a, s.ID, provider, workspace, recipient)
	if s.Loaded {
		other := "codex:quality-shared-provider-control"
		cleanupFixtureChat(f.t, f.a, other, provider, workspace, "")
		for _, id := range []string{other} {
			if _, err := f.a.OpenSessionAt(workspace, id); err != nil {
				f.t.Fatal(err)
			}
		}
		f.a.runtime.PutConversationLocked(s.ID, newSessionState(&sessionSeed{ID: s.ID, Provider: provider, Cwd: workspace}, sessionInitial{Open: true}))
		if f.providers.Count(provider, "") != 1 || f.providers.Count(provider, "session/load") != 1 {
			f.t.Fatal("loaded seed did not exercise counted launch and loads")
		}
	}
	if s.Working {
		f.a.runtime.PutConversationLocked(s.ID, newSessionState(&sessionSeed{ID: s.ID, Provider: provider, Cwd: f.ws}, sessionInitial{Open: true, Working: true}))
	}
	if s.Pinned {
		if err := f.a.updateSettings(func(v *Settings) error {
			record := newWorkspaceRecord(f.ws)
			record.Pins = []WorkspacePin{{Kind: "chat", Session: s.ID}}
			v.WorkspaceRecords = append(v.WorkspaceRecords, record)
			return nil
		}); err != nil {
			f.t.Fatal(err)
		}
	}
	if recipient != "" {
		if e := f.a.busHost().With(func(b *bus.Bus) error {
			session, e := b.Session(context.Background(), recipient)
			if e != nil {
				return e
			}
			f.addresses[s.ID] = session.Address()
			room, err := b.CreateGroup(context.Background(), bus.Sender{}, "Quality group "+s.ID, []string{session.Address()})
			if err != nil {
				return err
			}
			f.groups[s.ID] = "#" + room.Name
			for i := 0; i < s.Queued; i++ {
				if _, e = b.Send(context.Background(), bus.Sender{}, session.Address(), "invented retained input", nil, 0); e != nil {
					return e
				}
			}
			return nil
		}); e != nil {
			f.t.Fatal(e)
		}
	}
	if e := f.a.updateSettings(func(v *Settings) error {
		if v.ChatBindings == nil {
			v.ChatBindings = map[string]string{}
		}
		if len(s.Chain) > 0 {
			for i := 1; i < len(s.Chain); i++ {
				if strings.Contains(s.Chain[i], "history-target") {
					continue
				}
				v.ChatBindings[s.Chain[i-1]] = s.Chain[i]
			}
			if s.HistoryOnly {
				v.ChatTransferHistory = append(v.ChatTransferHistory, chatTransfer{Source: s.Chain[len(s.Chain)-2], Target: s.ID, Stage: "allocated", Created: f.now})
			}
		}
		return nil
	}); e != nil {
		f.t.Fatal(e)
	}
	if s.Parent != "" {
		tr, e := readTranscript(s.ID)
		if e != nil {
			f.t.Fatal(e)
		}
		tr.ParentSessionID = s.Parent
		if e = writeTranscript(tr); e != nil {
			f.t.Fatal(e)
		}
	}
}
func qualityHostResult(r ChatCleanupResult) qualityCleanupResult {
	out := qualityCleanupResult{ID: r.SessionID, OK: r.OK, Status: r.Status, Error: r.Error, ProviderCopyRemains: r.ProviderCopyRemains, ProviderDeletionReason: r.ProviderDeletionReason}
	if r.Row != nil {
		out.Text = r.Row.RuntimeNote
	}
	return out
}
func (f *qualityHostCleanup) Act(action string, ids ...string) []qualityCleanupResult {
	var rs []ChatCleanupResult
	switch action {
	case "remove":
		rs = f.a.RemoveChats(ids)
	case "restore":
		rs = []ChatCleanupResult{f.a.RestoreRemovedChat(ids[0])}
		if e := f.a.chatCleanupAcceptanceDiscoveryRound(); e != nil {
			f.t.Fatal(e)
		}
	case "delete":
		rs = []ChatCleanupResult{f.a.DeleteRemovedChat(ids[0])}
	case "empty":
		rs = f.a.EmptyChatRecycleBin()
	case "expire":
		previous := f.a.sink
		f.a.sink = func(name string, data any) {
			if name == "chat:finalized" {
				payload, ok := data.(map[string]any)
				if !ok {
					f.t.Fatal("unexpected finalized payload")
				}
				result, ok := payload["result"].(ChatCleanupResult)
				if !ok {
					f.t.Fatal("missing actual finalized result")
				}
				rs = append(rs, result)
			}
			if previous != nil {
				previous(name, data)
			}
		}
		defer func() { f.a.sink = previous }()
		if e := f.a.chatCleanupAcceptanceSweep(); e != nil {
			f.t.Fatal(e)
		}
	default:
		f.t.Fatal("unbound action", action)
	}
	for id := range f.seeds {
		r, ok := f.a.cleanupRecord(id)
		if ok && r.Stage == "deleted" && r.Group == "" {
			seen := false
			for _, e := range f.events {
				if e.ID == id && e.Kind == "local-delete" {
					seen = true
				}
			}
			if !seen {
				f.events = append(f.events, qualityCleanupEvent{id, "local-delete"})
			}
		}
	}
	out := []qualityCleanupResult{}
	for _, r := range rs {
		out = append(out, qualityHostResult(r))
	}
	return out
}
func (f *qualityHostCleanup) Send(route, id string) error {
	err := f.a.busHost().With(func(b *bus.Bus) error {
		var e error
		switch route {
		case "direct":
			_, e = b.Send(context.Background(), bus.Sender{}, f.addresses[id], "must not wake", nil, 0)
		case "group":
			_, e = b.SendActionRecipients(context.Background(), bus.Sender{}, f.groups[id], "must not wake", nil, 0, []string{f.addresses[id]})
		case "broadcast":
			_, e = b.SendActionRecipients(context.Background(), bus.Sender{}, f.groups[id], "must not wake", nil, 0, []string{bus.Everyone})
		default:
			f.t.Fatal("unknown route", route)
		}
		return e
	})
	if e := f.a.chatCleanupAcceptanceDiscoveryRound(); e != nil {
		f.t.Fatal(e)
	}
	return err
}
func (f *qualityHostCleanup) Restart() {
	f.a.lifecycle().StopAll()
	f.a.turnIO().Close(3 * time.Second)
	f.a.busHost().CloseHandle()
	f.a.queueStore.Close()
	f.a.closeAppletBridge()
	f.a = NewApp()
	f.providers.Bind(f.t, f.a)
	f.connects = watchConnects(f.a)
	a := f.a
	f.t.Cleanup(func() { a.busHost().CloseHandle(); a.queueStore.Close(); a.closeAppletBridge() })
	f.install()
	if e := f.a.chatCleanupAcceptanceRestartChats(); e != nil {
		f.t.Fatal(e)
	}
}
func (f *qualityHostCleanup) Advance(d time.Duration) { f.now = f.now.Add(d) }
func (f *qualityHostCleanup) Events() []qualityCleanupEvent {
	return append([]qualityCleanupEvent(nil), f.events...)
}
func (f *qualityHostCleanup) Observe(id string) qualityCleanupState {
	obs, e := f.a.chatCleanupAcceptanceObserve(id)
	if e != nil {
		f.t.Fatal(e)
	}
	r, removed := f.a.cleanupRecord(id)
	out := qualityCleanupState{Active: !removed, Removed: obs.Binned, Tombstone: obs.Tombstoned, LocalCopy: obs.TranscriptPresent, ProviderCopy: f.copies[id], RemovedAt: obs.RemovedAt, ExpiresAt: obs.ExpiresAt}
	out.Open = obs.RuntimeLoaded
	out.Loaded = obs.ProviderLoaded
	for p := range providerModules {
		out.Starts += f.providers.Count(p, "")
		out.Loads += f.providers.Count(p, "session/load") + f.providers.Count(p, "session/resume")
		out.Wakes += f.providers.Count(p, "session/prompt")
	}
	f.wakes += len(f.connects.take())
	out.Wakes += f.wakes
	// The saved listing and bus inventory are queried without provider discovery.
	rows, err := f.a.ListChatCleanup()
	if err != nil {
		f.t.Fatal(err)
	}
	for _, row := range rows.Rows {
		for _, member := range row.SessionIDs {
			if member == id {
				out.OrdinaryList = true
			}
		}
	}
	settings := f.a.settings
	recipient := settings.BusSessions[id]
	if r.Group != "" {
		ar, err := readChatCleanupArchive(r.Group)
		if err != nil {
			f.t.Fatal(err)
		}
		recipient = ar.Settings.BusSessions[id]
	}
	out.BusIdentity = recipient
	page, err := f.a.Decisions(0)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, decision := range page.Items {
		if recipient != "" && decision.AskerSession == recipient {
			out.Decisions = true
		}
	}
	if recipient != "" {
		if err := f.a.busHost().With(func(b *bus.Bus) error {
			var err error
			out.Queued, err = b.StoredSessionDeliveries(context.Background(), recipient)
			if err != nil {
				return err
			}
			active, e := b.RecipientAvailable(context.Background(), recipient)
			if e != nil {
				return e
			}
			out.Broadcast = active
			return err
		}); err != nil {
			f.t.Fatal(err)
		}
	}
	data, e := os.ReadFile(transcriptPath(id))
	if r.Group != "" {
		ar, err := readChatCleanupArchive(r.Group)
		if err != nil {
			f.t.Fatal(err)
		}
		out.Settings = ar.Settings.SessionTitles[id]
		for _, file := range ar.Files {
			if filepath.Join(configDir(), file.Path) == transcriptPath(id) {
				data, e = os.ReadFile(filepath.Join(chatCleanupArchiveDir(r.Group), file.Slot))
			}
		}
	} else {
		out.Settings = settings.SessionTitles[id]
	}
	if e == nil {
		out.Content = string(data)
	} else if !errors.Is(e, os.ErrNotExist) {
		f.t.Fatal(e)
	}
	return out
}
func (f *qualityHostCleanup) SaveUIState(values map[string]string) {
	state := UIState{Version: 1, Values: map[string]json.RawMessage{}}
	for k, v := range values {
		b, e := json.Marshal(v)
		if e != nil {
			f.t.Fatal(e)
		}
		state.Values[k] = b
	}
	if e := f.a.SaveUIState(state); e != nil {
		f.t.Fatal(e)
	}
}
func (f *qualityHostCleanup) ReadUIState() map[string]string {
	data, e := os.ReadFile(filepath.Join(configDir(), "ui-state.json"))
	if e != nil {
		f.t.Fatal(e)
	}
	var s UIState
	if e = json.Unmarshal(data, &s); e != nil {
		f.t.Fatal(e)
	}
	out := map[string]string{}
	for k, v := range s.Values {
		var text string
		if e = json.Unmarshal(v, &text); e != nil {
			f.t.Fatal(e)
		}
		out[k] = text
	}
	return out
}
