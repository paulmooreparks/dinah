package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/bus"
)

// busChatSource is continuationSource for a chat that already uses the bus
// under the identity "window-kept", with a message waiting for it.
func busChatSource(t *testing.T, a *App) (*Session, string) {
	t.Helper()
	const id = "window-kept"
	s := continuationSource(t, a)
	err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.RegisterSession(context.Background(), id, s.Cwd(), "Original Chat")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	a.busCoordinator().JoinBus(s, id)
	return s, id
}

func busSessionEnded(t *testing.T, a *App, id string) bool {
	t.Helper()
	var ended bool
	err := a.busHost().With(func(b *bus.Bus) error {
		row, err := b.Session(context.Background(), id)
		ended = row.Ended
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return ended
}

func savedBusSessions(a *App) map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := map[string]string{}
	for k, v := range a.settings.BusSessions {
		out[k] = v
	}
	return out
}

// Changing a chat's provider moves the chat's place on the bus to the new
// chat: the saved mapping, the live identity, and the identity the new agent
// is given when its session starts.
func TestProviderSwitchKeepsTheChatsBusIdentity(t *testing.T) {
	a, c := providerTestApp(t)
	// Claude is the provider whose agent runs the steering hooks, so its
	// prompts carry the marker that the hooks use to find the chat.
	a.runtime.PutProviderLocked("claude", a.runtime.ProviderLocked("codex"))
	s, id := busChatSource(t, a)
	preview, err := a.PreviewSessionContinuation(s.ID(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.ContinueSessionWithProvider(s.ID(), "claude", preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	next := a.session(result.Session.ID)
	if next == nil || next.BusID() != id || s.BusID() != "" {
		t.Fatalf("the identity after the switch: new %q, old %q, want %q and none", next.BusID(), s.BusID(), id)
	}
	if got := savedBusSessions(a); got[next.ID()] != id || got[s.ID()] != "" {
		t.Fatalf("the saved mapping: %v", got)
	}
	if busSessionEnded(t, a, id) {
		t.Fatal("the switch ended the bus session")
	}
	// The new agent's bus helper was started under the kept identity.
	found := false
	for _, call := range c.calls {
		if call.method == "session/new" && strings.Contains(fmt.Sprintf("%+v", call.params), "--session "+id) {
			found = true
		}
	}
	if !found {
		t.Fatalf("the new session was not started under %s: %+v", id, c.calls)
	}
	// Closing the old chat afterwards leaves the identity alone.
	a.busCoordinator().LeaveBus(s)
	if busSessionEnded(t, a, id) {
		t.Fatal("closing the old chat ended the new chat's bus session")
	}
	// The new agent's hooks resolve to the kept identity: the turn's prompt
	// carries a steering marker for it.
	a.turnIO().Run(next, promptPointer("Continue", []acp.ContentBlock{{Type: "text", Text: "Continue"}}))
	var sent acp.PromptParams
	for _, call := range c.calls {
		if call.method == "session/prompt" {
			sent = call.params.(acp.PromptParams)
		}
	}
	prompt := fmt.Sprintf("%+v", sent.Prompt)
	if !strings.Contains(prompt, "[Dinah chat: "+id+"]") {
		t.Fatalf("the turn's prompt does not carry the steering marker for %s: %s", id, prompt)
	}
}

// A switch that fails after the identity may have been touched leaves it with
// the old chat, in memory and in the settings, and leaves the bus session
// alive.
func TestFailedProviderSwitchLeavesTheBusIdentityWithTheOldChat(t *testing.T) {
	a, _ := providerTestApp(t)
	s, id := busChatSource(t, a)
	preview, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	before := savedBusSessions(a)
	// A leftover transaction file makes every settings write fail.
	stateTestWrite(t, filepath.Join(configDir(), "settings.json.pending"), "invalid transaction")
	if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", preview.Revision); err == nil {
		t.Fatal("the switch succeeded though the settings could not be written")
	}
	if s.BusID() != id {
		t.Fatalf("the old chat's identity after the failed switch is %q, want %q", s.BusID(), id)
	}
	if got := savedBusSessions(a); fmt.Sprint(got) != fmt.Sprint(before) {
		t.Fatalf("the saved mapping changed: %v, was %v", got, before)
	}
	if busSessionEnded(t, a, id) {
		t.Fatal("the failed switch ended the bus session")
	}
	for _, conv := range a.runtime.ConversationsLocked() {
		if conv != s && conv.BusID() == id {
			t.Fatalf("chat %s holds the identity after the failed switch", conv.ID())
		}
	}
}

// A chat that has not used the bus has no identity to move, and the switch
// gives the new chat a fresh one as it always did.
func TestProviderSwitchOfAChatWithoutBusIdentityTakesANewOne(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	preview, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.ContinueSessionWithProvider(s.ID(), "codex", preview.Revision)
	if err != nil {
		t.Fatal(err)
	}
	next := a.session(result.Session.ID)
	if got := next.BusID(); got == "" || savedBusSessions(a)[next.ID()] != got {
		t.Fatalf("the new chat's identity %q, saved %v", got, savedBusSessions(a))
	}
}
