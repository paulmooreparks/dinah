package main

import (
	"context"
	"testing"

	"studio/internal/bus"
)

func savedTitle(a *App, id string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.SessionTitles[id]
}

// A provider switch gives the new chat the name the person gave the old one.
func TestASwitchCopiesThePersonsName(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	if _, err := a.RenameSession(s.ID(), "Dinah Architecture"); err != nil {
		t.Fatal(err)
	}
	preview, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.ContinueSessionWithProvider(s.ID(), "codex", preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if got := savedTitle(a, result.Session.ID); got != "Dinah Architecture" {
		t.Fatalf("the continuation's name is %q", got)
	}
}

// A continuation saved without a name takes the name of its nearest named
// ancestor, on the bus as well, and a continuation that has a name keeps it.
func TestContinuationsWithoutANameTakeTheirNearestAncestorsAtStart(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	chain := []string{"codex:root", "codex:middle", "claude:leaf", "claude:named-leaf", "claude:orphan"}
	parents := []string{"", "codex:root", "codex:middle", "codex:middle", ""}
	for i, id := range chain {
		tr := &conversationTranscript{SessionID: id, Provider: "claude", Workspace: ws, ParentSessionID: parents[i], Complete: true}
		if err := writeTranscript(tr); err != nil {
			t.Fatal(err)
		}
	}
	err := a.updateSettings(func(s *Settings) error {
		s.SessionTitles = map[string]string{"codex:root": "Dinah Architecture", "claude:named-leaf": "Mine"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// The leaf is on the bus under the agent's own title, as it was.
	busID := bus.WindowSessionPrefix + "leaf"
	a.saveBusSession("claude:leaf", busID)
	if err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.RegisterSession(context.Background(), busID, ws, "Dinah cleanup phase 1 step 7")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	a.nameContinuations()
	for id, want := range map[string]string{"codex:root": "Dinah Architecture", "codex:middle": "Dinah Architecture", "claude:leaf": "Dinah Architecture", "claude:named-leaf": "Mine", "claude:orphan": ""} {
		if got := savedTitle(a, id); got != want {
			t.Errorf("%s is named %q, not %q", id, got, want)
		}
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		s, err := b.Session(context.Background(), busID)
		if err != nil {
			return err
		}
		if s.Title != "Dinah Architecture" {
			t.Errorf("the bus knows the leaf as %q", s.Title)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A second run changes nothing.
	a.nameContinuations()
	if got := savedTitle(a, "claude:named-leaf"); got != "Mine" {
		t.Fatalf("a named continuation was renamed: %q", got)
	}
}
