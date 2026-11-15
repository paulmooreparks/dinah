package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func cleanupFixtureChat(t *testing.T, a *App, id, provider, workspace, recipient string) {
	t.Helper()
	if err := a.updateSettings(func(s *Settings) error {
		if s.SessionProviders == nil {
			s.SessionProviders = map[string]string{}
		}
		if s.SessionTitles == nil {
			s.SessionTitles = map[string]string{}
		}
		if s.BusSessions == nil {
			s.BusSessions = map[string]string{}
		}
		s.SessionProviders[id] = provider
		s.SessionTitles[id] = "Invented " + id
		s.BusSessions[id] = recipient
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeTranscript(&sessionhost.Transcript{SessionID: id, Provider: provider, Workspace: workspace, Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "Keep this invented question"}, {Role: "agent", Text: "Keep this invented answer"}}}); err != nil {
		t.Fatal(err)
	}
	if recipient != "" {
		if err := a.busHost().With(func(b *bus.Bus) error {
			_, err := b.RegisterSession(context.Background(), recipient, workspace, "Invented "+id)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCleanupStagesRestoresAndFinallyDeletesThroughDocumentedHook(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	id := "codex:cleanup"
	recipient := "window-cleanup"
	cleanupFixtureChat(t, a, id, "codex", ws, recipient)
	nativeDeletes := 0
	a.chatCleanupHooks.DeleteProvider = func(_ context.Context, provider, native string) (bool, error) {
		nativeDeletes++
		if provider != "codex" || native != id {
			t.Errorf("native identity changed %q %q", provider, native)
		}
		if _, err := os.Stat(filepath.Join(chatCleanupArchiveDir(id), "manifest.json")); err != nil {
			t.Error("local archive erased before native deletion", err)
		}
		return true, nil
	}
	if err := a.RemoveChat(id); err != nil {
		t.Fatal(err)
	}
	if nativeDeletes != 0 {
		t.Fatal("staging deleted native history")
	}
	if _, err := os.Stat(transcriptPath(id)); !os.IsNotExist(err) {
		t.Fatal("retained transcript remained in startup scan", err)
	}
	listing, err := a.ListChatCleanup()
	if err != nil || len(listing.Rows) != 0 || len(listing.Removed) != 1 || !listing.Removed[0].Restorable {
		t.Fatalf("bin listing %+v %v", listing, err)
	}
	if err := a.RemoveChat(id); err != nil {
		t.Fatal("repeat staging", err)
	}
	restored := a.RestoreRemovedChat(id)
	if !restored.OK || restored.Status != "restored" || a.session(id) != nil {
		t.Fatalf("closed restore %+v", restored)
	}
	tr, err := readTranscript(id)
	if err != nil || len(tr.Messages) != 2 {
		t.Fatal("restored transcript changed", tr, err)
	}
	if err := a.DeleteSession(id); err != nil {
		t.Fatal("compatibility call did not stage", err)
	}
	if nativeDeletes != 0 {
		t.Fatal("compatibility call bypassed retention")
	}
	result := a.DeleteRemovedChat(id)
	if !result.OK || result.ProviderCopyRemains || nativeDeletes != 1 {
		t.Fatalf("final deletion %+v calls %d", result, nativeDeletes)
	}
	record, _ := a.cleanupRecord(id)
	if record.Stage != "deleted" || record.Group != "" || record.Snapshot != nil {
		t.Fatalf("nonminimal tombstone %+v", record)
	}
	a.mu.Lock()
	_, title := a.settings.SessionTitles[id]
	_, provider := a.settings.SessionProviders[id]
	a.mu.Unlock()
	if title || provider {
		t.Fatal("final deletion kept chat settings")
	}
	if _, err := os.Stat(chatCleanupArchiveDir(id)); !os.IsNotExist(err) {
		t.Fatal("archive survived final deletion", err)
	}
	if a.RestoreRemovedChat(id).OK {
		t.Fatal("final tombstone was restorable")
	}
}

func TestCleanupNativeFailuresStillEraseTheWholeChainOldestFirst(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	for i, id := range []string{"old", "middle", "head"} {
		cleanupFixtureChat(t, a, id, []string{"codex", "claude", "devin"}[i], ws, "")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.ChatBindings = map[string]string{"old": "middle", "middle": "head"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	calls := []string{}
	a.chatCleanupHooks.DeleteProvider = func(_ context.Context, provider, id string) (bool, error) {
		calls = append(calls, id)
		switch id {
		case "old":
			return true, nil
		case "middle":
			return false, nil
		default:
			return true, errors.New("invented native refusal")
		}
	}
	if err := a.RemoveChat("head"); err != nil {
		t.Fatal(err)
	}
	result := a.DeleteRemovedChat("head")
	if !result.OK || !result.ProviderCopyRemains || len(result.ProgramCopies) != 3 || len(calls) != 3 {
		t.Fatalf("chain deletion %+v calls %v", result, calls)
	}
	for i, want := range []string{"old", "middle", "head"} {
		if calls[i] != want {
			t.Fatalf("native order %v", calls)
		}
		if !a.removedChat(want) {
			t.Fatal("lost permanent tombstone", want)
		}
	}
}

func TestCleanupExpiryAndLoadedWithoutUnload(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	id := "loaded"
	cleanupFixtureChat(t, a, id, "codex", ws, "")
	loaded := newSessionState(&sessionSeed{ID: id, Provider: "codex", Cwd: ws}, sessionInitial{Open: true})
	other := newSessionState(&sessionSeed{ID: "other", Provider: "codex", Cwd: ws}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(id, loaded)
	a.runtime.PutConversationLocked("other", other)
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	a.chatCleanupHooks.Now = func() time.Time { return now }
	a.chatCleanupHooks.UnloadProvider = func(context.Context, string, string) (bool, error) { return false, nil }
	deletes := 0
	a.chatCleanupHooks.DeleteProvider = func(context.Context, string, string) (bool, error) { deletes++; return true, nil }
	if err := a.RemoveChat(id); err != nil {
		t.Fatal(err)
	}
	listing, err := a.ListChatCleanup()
	if err != nil || len(listing.Removed) != 1 || listing.Removed[0].RuntimeNote == "" {
		t.Fatalf("loaded disclosure %+v %v", listing, err)
	}
	if a.session(id) != nil || a.session("other") == nil {
		t.Fatal("binning retained runtime or stopped another chat")
	}
	now = now.Add(30*24*time.Hour - time.Nanosecond)
	if err := a.reconcileChatRemovals(); err != nil {
		t.Fatal(err)
	}
	if deletes != 0 {
		t.Fatal("expired before thirty days")
	}
	now = now.Add(time.Nanosecond)
	if err := a.reconcileChatRemovals(); err != nil {
		t.Fatal(err)
	}
	if deletes != 1 {
		t.Fatal("did not expire at thirty days", deletes)
	}
}

func TestCleanupResumesEveryDurablePhaseAfterRestart(t *testing.T) {
	for _, phase := range []string{"retaining", "restoring", "restored_cleanup", "finalizing", "erasing"} {
		t.Run(phase, func(t *testing.T) {
			a, _ := providerTestApp(t)
			id := "interrupted-" + phase
			cleanupFixtureChat(t, a, id, "codex", t.TempDir(), "")
			if err := a.RemoveChat(id); err != nil {
				t.Fatal(err)
			}
			archive, err := readChatCleanupArchive(id)
			if err != nil {
				t.Fatal(err)
			}
			if phase == "retaining" || phase == "restored_cleanup" {
				if err := archive.moveFiles(true); err != nil {
					t.Fatal(err)
				}
			}
			if err := a.updateSettings(func(s *Settings) error {
				r := s.RemovedChats[id]
				r.Stage = phase
				s.RemovedChats[id] = r
				if phase == "erasing" {
					cleanupEraseSettings(s, []string{id})
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			restarted := NewApp()
			defer restarted.busHost().CloseHandle()
			defer restarted.queueStore.Close()
			deletes := 0
			restarted.chatCleanupHooks.DeleteProvider = func(context.Context, string, string) (bool, error) { deletes++; return true, nil }
			if err := restarted.reconcileChatRemovals(); err != nil {
				t.Fatal(err)
			}
			record, exists := restarted.cleanupRecord(id)
			switch phase {
			case "retaining":
				if !exists || record.Stage != "retained" {
					t.Fatal("retention was not resumed", record)
				}
			case "restoring", "restored_cleanup":
				if exists {
					t.Fatal("restore reservation remained", record)
				}
				if _, err := readTranscript(id); err != nil {
					t.Fatal(err)
				}
			default:
				if !exists || record.Stage != "deleted" || record.Group != "" || record.Snapshot != nil {
					t.Fatal("finalization did not leave a minimal tombstone", record)
				}
			}
			if deletes != 0 && phase != "finalizing" || phase == "finalizing" && deletes != 1 {
				t.Fatalf("unexpected native calls %d", deletes)
			}
			if restarted.session(id) != nil {
				t.Fatal("recovery opened a chat")
			}
		})
	}
}
