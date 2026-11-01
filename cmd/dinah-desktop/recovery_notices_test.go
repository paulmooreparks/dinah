package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func TestRecoveryNoticeUpsertDismissAndClearPersistWithoutQueueChanges(t *testing.T) {
	a, _ := providerTestApp(t)
	source, _, recipient, _ := recoveryConflict(t, a)
	if err := a.transferChat(source, "", "codex", recipient, "reviewed", "allocating", true); err != nil {
		t.Fatal(err)
	}
	if err := a.transferChat(source, "codex:candidate", "codex", recipient, "reviewed", "allocated", true); err != nil {
		t.Fatal(err)
	}
	a.transferRunning.Delete(source)
	cards, err := a.Notifications()
	if err != nil || len(cards) != 1 {
		t.Fatal("one source generated multiple notices", cards, err)
	}
	first := cards[0]
	if first.OriginProvider != "codex" || first.OriginStarted == "" || !strings.Contains(first.Body, "1 message is waiting") {
		t.Fatal("source identity or queue state missing", first)
	}
	if err := a.DismissNotification(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.ReloadWaitingChatState(); err != nil {
		t.Fatal(err)
	}
	restarted := NewApp()
	defer restarted.busHost().CloseHandle()
	defer restarted.queueStore.Close()
	if cards, err := restarted.Notifications(); err != nil || len(cards) != 0 {
		t.Fatal("dismissal did not survive restart", cards, err)
	}
	if restarted.removedChat(source) {
		t.Fatal("dismissal removed the chat")
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		n, err := b.PendingSessionDeliveries(context.Background(), recipient)
		if n != 1 {
			t.Fatal("dismissal changed queue", n)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// A different original recipient with the same displayed title remains a
	// separate incident, even though it resolves to the same target chat.
	second := "codex:another-original"
	other, _ := closedChat(t, a, second, "Original owner")
	records, err := readWaitingChats()
	if err != nil {
		t.Fatal(err)
	}
	record := records[source]
	record.Route.Origin = second
	record.Route.Recipient = other
	record.Attempt = "another-attempt"
	records[second] = record
	if err := writeWaitingChats(records); err != nil {
		t.Fatal(err)
	}
	if err := restarted.ReloadWaitingChatState(); err != nil {
		t.Fatal(err)
	}
	cards, err = restarted.Notifications()
	if err != nil || len(cards) != 1 || cards[0].Lead != second || cards[0].ID == first.ID {
		t.Fatal("different original recipient was hidden", cards, err)
	}
	if err := restarted.ClearNotifications(); err != nil {
		t.Fatal(err)
	}
	if cards, err := restarted.Notifications(); err != nil || len(cards) != 0 {
		t.Fatal(cards, err)
	}
	if records, err := readWaitingChats(); err != nil || len(records) != 2 {
		t.Fatal("clear changed recovery authority", records, err)
	}
}

func TestRemoveFailedChatRetiresOnlyOriginalAcrossRestart(t *testing.T) {
	a, c := providerTestApp(t)
	source, other, recipient, otherRecipient := recoveryConflict(t, a)
	before, _ := os.ReadFile(transcriptPath(other))
	if err := a.RemoveChat(source); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveChat(source); err != nil {
		t.Fatal("removal was not idempotent", err)
	}
	if countCalls(c, "session/new")+countCalls(c, "session/load")+countCalls(c, "session/resume")+countCalls(c, "session/delete") != 0 {
		t.Fatal("local removal contacted a provider")
	}
	after, _ := os.ReadFile(transcriptPath(other))
	if !bytes.Equal(before, after) {
		t.Fatal("separate chat history changed")
	}
	if err := a.UnhideSessions(); err != nil {
		t.Fatal(err)
	}
	if !a.hidden(source) {
		t.Fatal("show hidden undid permanent removal")
	}
	restarted := NewApp()
	defer restarted.busHost().CloseHandle()
	defer restarted.queueStore.Close()
	if _, err := restarted.ResolveSessionContinuation(source); err == nil {
		t.Fatal("removed source reopened its successor")
	}
	restarted.busCoordinator().AskToConnect(source, "C:/unused", true)
	if cards, err := restarted.Notifications(); err != nil || len(cards) != 0 {
		t.Fatal("removed chat's incident returned", cards, err)
	}
	if err := restarted.busHost().With(func(b *bus.Bus) error {
		for id, want := range map[string]int{recipient: 0, otherRecipient: 1} {
			n, err := b.PendingSessionDeliveries(context.Background(), id)
			if err != nil {
				return err
			}
			if n != want {
				t.Fatal("recipient queue", id, n)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestInterruptedRemovalFinishesAfterRestartAndFailedIntentChangesNothing(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(map[bool]string{false: "failed-intent", true: "restart"}[interrupted], func(t *testing.T) {
			a, _ := providerTestApp(t)
			source, _, recipient, _ := recoveryConflict(t, a)
			if !interrupted {
				stateTestWrite(t, filepath.Join(configDir(), "settings.json.pending"), "invalid transaction")
				if err := a.RemoveChat(source); err == nil {
					t.Fatal("removal succeeded without durable intent")
				}
				if a.removedChat(source) {
					t.Fatal("failed intent changed removal state")
				}
				return
			}
			if err := a.updateSettings(func(s *Settings) error {
				s.RemovedChats = map[string]chatRemoval{source: {Recipient: recipient, Stage: "removing", Created: time.Now().UTC()}}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			restarted := NewApp()
			defer restarted.busHost().CloseHandle()
			defer restarted.queueStore.Close()
			if err := restarted.reconcileChatRemovals(); err != nil {
				t.Fatal(err)
			}
			if restarted.settings.RemovedChats[source].Stage != "removed" {
				t.Fatal("unfinished removal was not settled")
			}
			if records, err := readWaitingChats(); err != nil || len(records) != 0 {
				t.Fatal(records, err)
			}
			tr, err := sessionhost.ReadTranscript(configDir(), source)
			if err != nil || !tr.Deleted {
				t.Fatal("removed history was not tombstoned", tr, err)
			}
		})
	}
}
