package main

import (
	"strings"
	"testing"

	"studio/internal/acp"
)

func TestCreateNamedChatMakesAFinishedChatAndSendsNoPrompt(t *testing.T) {
	a, c := providerTestApp(t)
	c.newIDs = []string{"codex:named"}
	before := len(c.calls.Snapshot())
	info, err := a.CreateNamedChat(t.TempDir(), "codex", "", "  Fix the Parser ")
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Fix the Parser" || a.session(info.ID) == nil {
		t.Fatalf("the chat: %+v", info)
	}
	for _, call := range c.calls.Snapshot()[before:] {
		if _, ok := call.params.(acp.PromptParams); ok {
			t.Fatalf("a prompt was sent: %+v", call)
		}
	}
}

func TestCreateNamedChatWithoutATitleMakesNothing(t *testing.T) {
	a, c := providerTestApp(t)
	c.newIDs = []string{"codex:named"}
	before := len(c.calls.Snapshot())
	if _, err := a.CreateNamedChat(t.TempDir(), "codex", "", "   "); err == nil {
		t.Fatal("expected a refusal")
	}
	if len(c.calls.Snapshot()) != before {
		t.Fatalf("a chat was started: %d calls", len(c.calls.Snapshot())-before)
	}
}

func TestCreateNamedChatWithAModelTheProviderLacksRemovesTheChat(t *testing.T) {
	a, c := providerTestApp(t)
	// Cleanup stops the final unused provider. Match production ownership so
	// Stop can join the watcher's completion instead of waiting on an orphan.
	p := a.runtime.ProviderLocked("codex")
	go a.lifecycle().Watch("codex", p)
	t.Cleanup(func() { c.Close(); p.Wait() })
	c.newIDs = []string{"codex:named"}
	_, err := a.CreateNamedChat(t.TempDir(), "codex", "no-such-model", "Fix the Parser")
	if err == nil || !strings.Contains(err.Error(), "does not offer the model") {
		t.Fatalf("expected the refusal, got %v", err)
	}
	if a.session("codex:named") != nil {
		t.Fatal("the half-made chat was left open")
	}
}
