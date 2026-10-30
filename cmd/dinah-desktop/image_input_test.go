package main

import (
	"testing"

	"studio/internal/acp"
)

func withImageCapability(a *App, provider string, image bool) {
	initial := a.runtime.ProviderLocked(provider).Initialization()
	initial.AgentCapabilities.PromptCapabilities.Image = image
	setTestInitialization(a, provider, initial)
}

// A chat's image input is "yes", "no" or "unknown", from documented sources
// only; the page warns for "no" alone.
func TestChatImageInputIsYesNoOrUnknown(t *testing.T) {
	a, c := providerTestApp(t)
	open := func(id, provider string) {
		s := newSessionState(&sessionSeed{ID: id, Provider: provider, Cwd: t.TempDir()}, sessionInitial{Open: true})
		a.runtime.PutConversationLocked(id, s)
	}
	// An agent that says it takes images.
	open("codex:yes", "codex")
	withImageCapability(a, "codex", true)
	if got := a.ChatImageInput("codex:yes"); got != "yes" {
		t.Errorf("an agent that takes images: %q", got)
	}
	// An agent that does not say so: false is the protocol's default.
	withImageCapability(a, "codex", false)
	if got := a.ChatImageInput("codex:yes"); got != "no" {
		t.Errorf("an agent that does not take images: %q", got)
	}
	// Unknown: no such chat, Devin's per-model support, a provider that has not
	// answered.
	if got := a.ChatImageInput("codex:nobody"); got != "unknown" {
		t.Errorf("a chat that is not open: %q", got)
	}
	// Devin says it takes images, but its support is per model, so it stays
	// unknown.
	a.runtime.PutProviderLocked("devin", newTestProvider(c, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{PromptCapabilities: acp.PromptCapabilities{Image: true}}}))
	open("devin-chat", "devin")
	if got := a.ChatImageInput("devin-chat"); got != "unknown" {
		t.Errorf("a Devin chat: %q", got)
	}
	open("claude:silent", "claude")
	if got := a.ChatImageInput("claude:silent"); got != "unknown" {
		t.Errorf("a provider that has not answered: %q", got)
	}
	// Ollama says it per model, through its own request: its answer is passed
	// on, and when it cannot answer the chat is unknown.
	a.runtime.PutProviderLocked("ollama", newTestProvider(c, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{LoadSession: true, PromptCapabilities: acp.PromptCapabilities{Image: true}}}))
	open("ollama:chat", "ollama")
	for _, answer := range []string{"yes", "no"} {
		c.imageInput = answer
		if got := a.ChatImageInput("ollama:chat"); got != answer {
			t.Errorf("Ollama says %q, the chat is %q", answer, got)
		}
	}
	c.imageInput = ""
	if got := a.ChatImageInput("ollama:chat"); got != "unknown" {
		t.Errorf("Ollama could not say: %q", got)
	}
}
