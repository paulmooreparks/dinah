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
	t      *testing.T
	a      *App
	ws     string
	now    time.Time
	seeds  map[string]qualityCleanupSeed
	copies map[string]bool
	events []qualityCleanupEvent
}

func init() {
	qualityCleanupFactory = func(t *testing.T) qualityCleanupFixture {
		if !strings.HasPrefix(t.Name(), "TestQualityCleanupDraftsAndStaleSave") && !strings.HasPrefix(t.Name(), "TestQualityCleanupExpiryBoundary") && !strings.HasPrefix(t.Name(), "TestQualityCleanupProviderDeletion") {
			t.Fatal("this host binding does not yet implement the runtime, bus-routing and ownership observations required by this case")
		}
		if strings.Contains(t.Name(), "TestQualityCleanupProviderDeletion") && strings.HasSuffix(t.Name(), "/expire") {
			t.Fatal("expiry outcome projection is pending; no result is synthesized from expected state")
		}
		a, _ := providerTestApp(t)
		f := &qualityHostCleanup{t: t, a: a, ws: t.TempDir(), now: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), seeds: map[string]qualityCleanupSeed{}, copies: map[string]bool{}}
		f.install()
		return f
	}
}
func (f *qualityHostCleanup) install() {
	f.a.chatCleanupAcceptanceInstall(chatCleanupAcceptanceHooks{Now: func() time.Time { return f.now }, DeleteProvider: func(_ context.Context, _, id string) (bool, error) {
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
	if s.Working || s.Pinned || s.Loaded {
		f.t.Fatal("runtime and bus fixture binding is pending")
	}
	f.seeds[s.ID] = s
	f.copies[s.ID] = true
	provider := strings.SplitN(s.ID, ":", 2)[0]
	recipient := ""
	if s.Queued > 0 {
		owner := s.ID
		if len(s.Chain) > 0 {
			owner = s.Chain[0]
		}
		recipient = "window-quality-" + strings.ReplaceAll(owner, ":", "-")
	}
	cleanupFixtureChat(f.t, f.a, s.ID, provider, f.ws, recipient)
	if recipient != "" {
		if e := f.a.busHost().With(func(b *bus.Bus) error {
			session, e := b.Session(context.Background(), recipient)
			if e != nil {
				return e
			}
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
	case "delete":
		rs = []ChatCleanupResult{f.a.DeleteRemovedChat(ids[0])}
	case "empty":
		rs = f.a.EmptyChatRecycleBin()
	case "expire":
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
func (f *qualityHostCleanup) Send(string, string) error {
	f.t.Fatal("bus delivery binding pending")
	return nil
}
func (f *qualityHostCleanup) Restart() {
	f.a.busHost().CloseHandle()
	f.a.queueStore.Close()
	f.a.closeAppletBridge()
	f.a = NewApp()
	a := f.a
	f.t.Cleanup(func() { a.busHost().CloseHandle(); a.queueStore.Close(); a.closeAppletBridge() })
	f.install()
	if e := f.a.chatCleanupAcceptanceSweep(); e != nil {
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
	if recipient != "" {
		if err := f.a.busHost().With(func(b *bus.Bus) error {
			var err error
			out.Queued, err = b.StoredSessionDeliveries(context.Background(), recipient)
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
