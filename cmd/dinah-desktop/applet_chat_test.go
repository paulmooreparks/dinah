package main

import (
	"strings"
	"testing"

	"studio/internal/applet"
)

func TestChatAppletWorkspaceIsolationWithoutAgent(t *testing.T) {
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	ws, other := t.TempDir(), t.TempDir()
	a := NewApp()
	a.runtime.PutConversationLocked("own", newSessionState(&sessionSeed{ID: "own", Cwd: ws, Title: "Own Chat"}))
	a.runtime.PutConversationLocked("other", newSessionState(&sessionSeed{ID: "other", Cwd: other, Title: "Other Chat"}))
	a.remember(ws, "", "send", "our prompt", nil)
	a.remember(other, "", "send", "other secret prompt", nil)
	bundle, err := a.chatAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := builtinTestContext(ws)
	result, err := r.Execute(ctx, chatListAction, `{}`)
	if err != nil || !strings.Contains(result.OutputJSON, "Own Chat") || strings.Contains(result.OutputJSON, "Other Chat") || strings.Contains(result.OutputJSON, ws) {
		t.Fatalf("list: %s, %v", result.OutputJSON, err)
	}
	result, err = r.Execute(ctx, chatHistoryAction, `{}`)
	if err != nil || !strings.Contains(result.OutputJSON, "our prompt") || strings.Contains(result.OutputJSON, "other secret prompt") || !strings.Contains(result.OutputJSON, "prompt-only") {
		t.Fatalf("history: %s, %v", result.OutputJSON, err)
	}
	if a.runtime.ProviderLocked("devin") != nil {
		t.Fatal("read actions started the agent")
	}
	if _, err := r.Execute(ctx, chatListAction, `{"limit":101}`); err == nil {
		t.Fatal("accepted excessive limit")
	}
}
