package sessionhost

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func chainConfirmationFixture(t *testing.T) (*Continuations, *Conversation, *Transcript, *Transcript) {
	t.Helper()
	c, s, source := confirmationFixtureSize(t, 0)
	parent := &Transcript{
		SessionID: "ancestor", Provider: "earlier", Workspace: t.TempDir(), Complete: true,
		Messages: []ConversationMessage{
			{Role: "agent", Text: "Keep the earlier decision."},
			{Role: "limitation", Text: "Earlier images are not transferred."},
		},
	}
	source.ParentSessionID = parent.SessionID
	source.ParentWorkspace = parent.Workspace
	source.ParentHistory = append([]ConversationMessage{}, parent.Messages...)
	for _, tr := range []*Transcript{parent, source} {
		if err := WriteTranscript(c.host.StateDir(), tr); err != nil {
			t.Fatal(err)
		}
	}
	return c, s, source, parent
}

// Confirmation must disclose every limitation the history assembler will add,
// including its fallback reason, before the person approves the transfer.
func TestConfirmationChainWarningsMatchPreparation(t *testing.T) {
	for _, test := range []struct {
		name, fault     string
		fallback        bool
		refused         bool
		ancestorSummary bool
	}{
		{name: "cross-workspace"},
		{name: "missing-with-summary", fault: "missing", fallback: true},
		{name: "corrupt-with-summary", fault: "corrupt", fallback: true},
		{name: "missing-without-summary", fault: "missing", refused: true},
		{name: "corrupt-without-summary", fault: "corrupt", refused: true},
		{name: "incomplete-with-summary", fault: "incomplete", fallback: true},
		{name: "wrong-workspace-with-summary", fault: "workspace", fallback: true},
		{name: "missing-parent-snapshot-with-summary", fault: "snapshot", fallback: true},
		{name: "missing-grandparent-with-ancestor-summary", fault: "grandparent", fallback: true, ancestorSummary: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, s, source, parent := chainConfirmationFixture(t)
			if test.fallback && !test.ancestorSummary {
				source.Summary = "Saved summary of the earlier decision."
			}
			if test.ancestorSummary {
				parent.Summary = "Saved summary of the earlier decision."
			}
			if test.fault == "snapshot" {
				source.ParentHistory = nil
			}
			if err := WriteTranscript(c.host.StateDir(), source); err != nil {
				t.Fatal(err)
			}
			path := TranscriptPath(c.host.StateDir(), parent.SessionID)
			switch test.fault {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(path, []byte(`{"sessionId":`), 0600); err != nil {
					t.Fatal(err)
				}
			case "incomplete", "workspace":
				if test.fault == "incomplete" {
					parent.Complete = false
				} else {
					parent.Workspace = t.TempDir()
				}
				if err := WriteTranscript(c.host.StateDir(), parent); err != nil {
					t.Fatal(err)
				}
			case "grandparent":
				parent.ParentSessionID = "missing-grandparent"
				parent.ParentHistory = []ConversationMessage{}
				if err := WriteTranscript(c.host.StateDir(), parent); err != nil {
					t.Fatal(err)
				}
			}
			confirmation, confirmErr := c.PreviewSession(s.ID(), "new")
			// Exercise the history assembler independently of confirmation so that
			// shared rejection by prepareFor cannot conceal differing behavior.
			c.mu.Lock()
			prepared, prepareErr := c.previewLocked(s, "new", "", nil)
			c.mu.Unlock()
			if (confirmErr != nil) != test.refused || (prepareErr != nil) != test.refused {
				t.Fatalf("confirmation=%v preparation=%v refused=%t", confirmErr, prepareErr, test.refused)
			}
			if test.refused {
				if confirmErr.Error() != prepareErr.Error() {
					t.Fatalf("rejection differs: confirmation=%v preparation=%v", confirmErr, prepareErr)
				}
				return
			}
			if !slices.Contains(confirmation.Warnings, source.Messages[0].Text) {
				t.Fatal("confirmation lost a current-message limitation")
			}
			for _, warning := range prepared.Warnings {
				if !slices.Contains(confirmation.Warnings, warning) {
					t.Errorf("preparation adds an unconfirmed limitation %q; confirmation=%q", warning, confirmation.Warnings)
				}
			}
			if len(prepared.Warnings) != len(confirmation.Warnings) {
				t.Errorf("warning sets differ: confirmation=%q preparation=%q", confirmation.Warnings, prepared.Warnings)
			}
			if test.fallback {
				if len(prepared.History) == 0 || !strings.Contains(prepared.History[0].Text, "Saved summary of the earlier decision.") {
					t.Fatal("preparation did not use the saved summary")
				}
			} else if !slices.Contains(confirmation.Warnings, parent.Messages[1].Text) {
				t.Fatal("cross-workspace history lost its inherited limitation")
			}
			if _, err := c.prepareFor(s, "new", "", confirmation.Revision); err != nil {
				t.Fatalf("approved preparation: %v", err)
			}
		})
	}
}

func TestConfirmationAncestorChangesInvalidateApproval(t *testing.T) {
	for _, name := range []string{"changed-ancestor", "restored-ancestor", "missing-to-empty-corrupt"} {
		t.Run(name, func(t *testing.T) {
			c, s, source, parent := chainConfirmationFixture(t)
			if name != "changed-ancestor" {
				source.Summary = "Saved summary."
				if err := WriteTranscript(c.host.StateDir(), source); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(TranscriptPath(c.host.StateDir(), parent.SessionID)); err != nil {
					t.Fatal(err)
				}
			}
			confirmation, err := c.PreviewSession(s.ID(), "new")
			if err != nil {
				t.Fatal(err)
			}
			parent.Messages[0].Text = "The ancestor changed after approval."
			if name == "missing-to-empty-corrupt" {
				err = os.WriteFile(TranscriptPath(c.host.StateDir(), parent.SessionID), []byte{}, 0600)
			} else {
				err = WriteTranscript(c.host.StateDir(), parent)
			}
			if err != nil {
				t.Fatal(err)
			}
			c.host.Formatter.Speaker = func(ConversationMessage) string {
				t.Error("stale approval reached handoff formatting")
				return "agent"
			}
			if _, err := c.prepareFor(s, "new", "", confirmation.Revision); err == nil || !strings.Contains(err.Error(), "review") {
				t.Fatalf("ancestor change accepted: %v", err)
			}
		})
	}
}
