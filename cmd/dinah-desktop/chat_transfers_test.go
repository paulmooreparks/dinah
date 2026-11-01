package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func recoveryConflict(t *testing.T, a *App) (string, string, string, string) {
	t.Helper()
	source, other := "codex:original-owner", "claude:separate-owner"
	owner, cwd := closedChat(t, a, source, "Original owner")
	otherOwner, _ := closedChat(t, a, other, "Separate owner")
	parent := &sessionhost.Transcript{SessionID: source, Provider: "codex", Workspace: cwd, Complete: true, Messages: []sessionhost.ConversationMessage{{Role: "you", Text: "Keep the original work."}}}
	child := &sessionhost.Transcript{SessionID: other, Provider: "claude", Workspace: cwd, Complete: true, ParentSessionID: source, ParentHistory: parent.Messages, PendingHandoff: true}
	for _, tr := range []*sessionhost.Transcript{parent, child} {
		if err := sessionhost.WriteTranscript(configDir(), tr); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := a.busCoordinator().CurrentDeliveryAttempt(source); err {
		t.Fatal("unexpected opening")
	}
	a.busCoordinator().AskToConnect(source, cwd, true)
	return source, other, owner, otherOwner
}

func TestRecoveryConflictKeepsLogicalRecipientAndDeliversOnlyItsQueue(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			a, c := providerTestApp(t)
			c.newIDs = []string{provider + ":recovered"}
			a.runtime.PutProviderLocked("claude", a.runtime.ProviderLocked("codex"))
			source, other, owner, otherOwner := recoveryConflict(t, a)
			beforeHold, err := readWaitingChats()
			if err != nil {
				t.Fatal(err)
			}
			original, _ := os.ReadFile(transcriptPath(source))
			separate, _ := os.ReadFile(transcriptPath(other))
			preview, err := a.PreviewSessionRecovery(source, provider)
			if err != nil {
				t.Fatal(err)
			}
			result, err := a.RecoverSession(source, provider, preview.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if result.Session.ID == source || result.Session.ID == other || a.settings.BusSessions[result.Session.ID] != owner || a.settings.BusSessions[other] != otherOwner {
				t.Fatal("recovery merged recipients or reused a native identity")
			}
			if head, err := a.ResolveSessionContinuation(source); err != nil || head != result.Session.ID {
				t.Fatal("committed binding lost to the older fork", head, err)
			}
			if head, err := a.ResolveSessionContinuation(other); err != nil || head != other {
				t.Fatal("separate recipient was redirected", head, err)
			}
			if route, err := a.waitingRoute(source, result.Session.Cwd); err != nil || route.Recipient != owner || route.Target != result.Session.ID || route.TargetRecipient != owner {
				t.Fatal("historical source lost its committed recipient", route, err)
			}
			for _, call := range c.calls {
				if call.method == "session/prompt" || call.method == "session/load" || call.method == "session/resume" {
					t.Fatal("recovery submitted a turn or contacted a missing native session")
				}
			}
			after, _ := os.ReadFile(transcriptPath(source))
			afterOther, _ := os.ReadFile(transcriptPath(other))
			if !bytes.Equal(original, after) || !bytes.Equal(separate, afterOther) {
				t.Fatal("recovery rewrote source histories")
			}
			// Read commit authority from disk and recreate the durable hold owner,
			// as after a process restart before waiting-state cleanup.
			if err := writeWaitingChats(beforeHold); err != nil {
				t.Fatal(err)
			}
			saved, err := loadSettings()
			if err != nil {
				t.Fatal(err)
			}
			a.mu.Lock()
			a.settings = saved
			a.mu.Unlock()
			records, err := a.readReconciledWaitingChats()
			if err != nil || len(records) != 0 {
				t.Fatal(records, err)
			}
			err = a.busHost().With(func(b *bus.Bus) error {
				for id, want := range map[string]int{owner: 1, otherOwner: 1} {
					n, _, err := b.Doorbell(context.Background(), id)
					if err != nil {
						return err
					}
					if n != want {
						t.Fatalf("before delivery %s=%d", id, n)
					}
				}
				a.busCoordinator().Ring(context.Background(), b, a.session(result.Session.ID))
				for id, want := range map[string]int{owner: 0, otherOwner: 1} {
					n, _, err := b.Doorbell(context.Background(), id)
					if err != nil {
						return err
					}
					if n != want {
						t.Fatalf("after delivery %s=%d", id, n)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRecoveryModelChoiceIsConfirmedBeforeRecipientCommit(t *testing.T) {
	a, c := providerTestApp(t)
	source, other, owner, otherOwner := recoveryConflict(t, a)
	p, err := a.PreviewSessionRecoveryModel(source, "codex", "changed")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.RecoverSessionModel(source, "codex", p.Revision, "initial"); err == nil {
		t.Fatal("model changed after confirmation")
	}
	if countCalls(c, "session/new") != 0 {
		t.Fatal("stale choice allocated a provider session")
	}
	r, err := a.RecoverSessionModel(source, "codex", p.Revision, "changed")
	if err != nil {
		t.Fatal(err)
	}
	if a.settings.ChatTransfers[source].Model != "changed" || a.settings.BusSessions[r.Session.ID] != owner || a.settings.BusSessions[other] != otherOwner {
		t.Fatal("model or ownership was not retained")
	}
	found := false
	for _, o := range a.SessionOptions(r.Session.ID) {
		if o.Category == "model" && string(o.CurrentValue) == `"changed"` {
			found = true
		}
	}
	if !found || countCalls(c, "session/prompt") != 0 {
		t.Fatal("model was not selected before any delivery")
	}
}

func TestRecoveryAmbiguousBranchesRequireExplicitDestination(t *testing.T) {
	a, c := providerTestApp(t)
	source, _, _, _ := recoveryConflict(t, a)
	third := "codex:another-branch"
	_, _ = closedChat(t, a, third, "Another recipient")
	parent, err := readTranscript(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTranscript(&conversationTranscript{SessionID: third, Provider: "codex", Workspace: parent.Workspace, ParentSessionID: source, ParentHistory: parent.Messages, Complete: true, PendingHandoff: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResolveSessionContinuation(source); err == nil {
		t.Fatal("ambiguous ancestry selected a destination")
	}
	route, err := a.waitingRoute(source, parent.Workspace)
	if err != nil || route.RoutingError == "" {
		t.Fatal("ambiguous route has no durable failure reason", route, err)
	}
	a.busCoordinator().AskToConnect(source, parent.Workspace, true)
	if countCalls(c, "session/load")+countCalls(c, "session/resume")+countCalls(c, "session/new") != 0 {
		t.Fatal("ambiguous ancestry contacted a provider")
	}
	p, err := a.PreviewSessionRecovery(source, "codex")
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.RecoverSession(source, "codex", p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if head, err := a.ResolveSessionContinuation(source); err != nil || head != r.Session.ID {
		t.Fatal("explicit recovery did not settle the binding", head, err)
	}
	if head, err := a.ResolveSessionContinuation(third); err != nil || head != third {
		t.Fatal("separate branch was merged", head, err)
	}
}

func TestRecoveryAllocationFailureAndRestartRetainOwnership(t *testing.T) {
	a, c := providerTestApp(t)
	source, other, owner, otherOwner := recoveryConflict(t, a)
	p, err := a.PreviewSessionRecovery(source, "codex")
	if err != nil {
		t.Fatal(err)
	}
	c.newErr = errors.New("allocation connection lost")
	if _, err := a.RecoverSession(source, "codex", p.Revision); err == nil {
		t.Fatal("allocation failure ignored")
	}
	saved, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	restarted := &App{settings: saved}
	if !restarted.transferHeld(source, owner) || restarted.transferHeld(other, otherOwner) || restarted.transferWarning(source) == "" {
		t.Fatal("restart lost or widened the transfer hold")
	}
	if saved.ChatTransfers[source].Stage != "allocating" || saved.BusSessions[source] != owner || saved.BusSessions[other] != otherOwner {
		t.Fatal("allocation failure published ownership")
	}
	if len(restarted.transferNotifications()) != 1 {
		t.Fatal("unfinished allocation has no actionable status")
	}
	// A new explicit recovery retains the earlier unconfirmed allocation.
	c.newErr = nil
	p, err = a.PreviewSessionRecovery(source, "codex")
	if err != nil || len(p.Warnings) == 0 {
		t.Fatal(p, err)
	}
	if _, err = a.RecoverSession(source, "codex", p.Revision); err != nil {
		t.Fatal(err)
	}
	if len(a.settings.ChatTransferHistory) != 1 {
		t.Fatal("earlier allocation record was discarded")
	}
}

func TestRecoveryTransferCommitBoundariesKeepQueueHeld(t *testing.T) {
	for _, stage := range []string{"allocating", "allocated", "prepared"} {
		t.Run(stage, func(t *testing.T) {
			a, _ := providerTestApp(t)
			source, _, owner, _ := recoveryConflict(t, a)
			if err := a.transferChat(source, "", "codex", owner, "reviewed", "allocating", true); err != nil {
				t.Fatal(err)
			}
			if stage != "allocating" {
				if err := a.transferChat(source, "codex:candidate", "codex", owner, "reviewed", "allocated", true); err != nil {
					t.Fatal(err)
				}
			}
			if stage == "prepared" {
				if err := a.transferChat(source, "codex:candidate", "codex", owner, "reviewed", "prepared", true); err != nil {
					t.Fatal(err)
				}
				stateTestWrite(t, filepath.Join(configDir(), "settings.json.pending"), "invalid transaction")
				if err := a.moveBusSession(source, "codex:candidate", owner); err == nil {
					t.Fatal("binding committed through a failed settings transaction")
				}
				if a.settings.BusSessions[source] != owner || a.settings.ChatBindings[source] != "" || !a.transferHeld(source, owner) {
					t.Fatal("failed commit released the source")
				}
				if err := os.Remove(filepath.Join(configDir(), "settings.json.pending")); err != nil {
					t.Fatal(err)
				}
			}
			saved, err := loadSettings()
			if err != nil {
				t.Fatal(err)
			}
			restarted := &App{settings: saved}
			if !restarted.transferHeld(source, owner) || saved.ChatBindings[source] != "" || saved.BusSessions[source] != owner {
				t.Fatal("interrupted transfer released ownership")
			}
			if got := restarted.chatBindings()[source]; got != "" {
				t.Fatal("uncommitted candidate became a route")
			}
		})
	}
}
