package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
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
			if !a.turnIO().Close(3 * time.Second) {
				t.Fatal("recovery's initial continuation did not finish")
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
				if call.method == "session/load" || call.method == "session/resume" {
					t.Fatal("recovery contacted a missing native session")
				}
				if call.method == "session/prompt" && call.params.(acp.PromptParams).SessionID != result.Session.ID {
					t.Fatal("continuation prompt went to a different native session")
				}
			}
			got := prompts(c)
			if len(got) != 1 || !strings.Contains(got[0], "A question for Original owner") || strings.Contains(got[0], "A question for Separate owner") {
				t.Fatalf("initial continuation did not carry exactly the original owner's waiting message: %q", got)
			}
			tr, err := readTranscript(result.Session.ID)
			if err != nil {
				t.Fatal(err)
			}
			deliveries := 0
			for _, entry := range tr.Messages {
				if entry.Delivered == nil {
					continue
				}
				deliveries++
				if entry.Delivered.ID == 0 || !strings.Contains(entry.Delivered.Prompt, "A question for Original owner") {
					t.Fatal("saved delivery lost its stable message identity or submitted prompt")
				}
				if err := a.busHost().With(func(b *bus.Bus) error {
					message, err := b.Message(context.Background(), entry.Delivered.ID)
					if err == nil && message.Text != "A question for Original owner" {
						t.Fatal("saved delivery points at another message")
					}
					return err
				}); err != nil {
					t.Fatal(err)
				}
			}
			if deliveries != 1 {
				t.Fatalf("saved %d deliveries, want exactly the original message", deliveries)
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
				// Recovery now submits its first prompt immediately. The original
				// queue was included in that prompt, while the separate fork stays waiting.
				for id, want := range map[string]int{owner: 0, otherOwner: 1} {
					n, _, err := b.Doorbell(context.Background(), id)
					if err != nil {
						return err
					}
					if n != want {
						t.Fatalf("after recovery %s=%d", id, n)
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
	if !a.turnIO().Close(3 * time.Second) {
		t.Fatal("recovery prompt did not finish")
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
	selectedBeforePrompt := false
	for _, call := range c.calls {
		if call.method == "session/set_config_option" && call.params.(acp.SetConfigOptionParams).ConfigID == "model" {
			selectedBeforePrompt = true
		}
		if call.method == "session/prompt" && !selectedBeforePrompt {
			t.Fatal("delivery preceded model selection")
		}
	}
	if !found || !selectedBeforePrompt || countCalls(c, "session/prompt") != 1 {
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

func TestRejectedAllocationSettlementIsTransactionalAndStageChecked(t *testing.T) {
	a, _ := providerTestApp(t)
	source, _, owner, _ := recoveryConflict(t, a)
	if err := a.transferChat(source, "", "codex", owner, "reviewed", "allocating", true); err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(configDir(), "settings.json.pending")
	stateTestWrite(t, pending, "invalid transaction")
	if err := a.transferChat(source, "", "codex", owner, "reviewed", "rejected", true); err == nil {
		t.Fatal("rejection settled through a failed settings write")
	}
	if a.settings.ChatTransfers[source].Stage != "allocating" || len(a.settings.ChatTransferHistory) != 0 || !a.transferHeld(source, owner) {
		t.Fatal("failed settlement changed the allocation hold")
	}
	if err := os.Remove(pending); err != nil {
		t.Fatal(err)
	}
	if err := a.transferChat(source, "codex:candidate", "codex", owner, "reviewed", "allocated", true); err != nil {
		t.Fatal(err)
	}
	if err := a.transferChat(source, "codex:candidate", "codex", owner, "reviewed", "rejected", true); err == nil {
		t.Fatal("an existing candidate was treated as never allocated")
	}
	if a.settings.ChatTransfers[source].Stage != "allocated" || !a.transferHeld(source, owner) {
		t.Fatal("invalid settlement released a known allocation")
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
