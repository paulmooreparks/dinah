//go:build live

package main

import (
	"os"
	"strings"
	"testing"
)

// This opt-in test runs Antigravity through its installed command line on
// the person's own sign-in: a new chat answers, and the chat answers again
// with what it was told after its agent was stopped and the chat reopened.
// It sends two short turns and keeps every file in throwaway folders.
func TestLiveAntigravityChatAndReopen(t *testing.T) {
	if os.Getenv("DINAH_LIVE_ANTIGRAVITY") != "1" {
		t.Skip("set DINAH_LIVE_ANTIGRAVITY=1 to send live Antigravity turns")
	}
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	a := NewApp()
	a.settings.DefaultProvider = antigravityID
	ws := t.TempDir()
	a.settings.Workspace = ws
	record(a)
	// The bus and the logs are closed last, so the test's folder can be removed.
	t.Cleanup(func() { a.busHost().CloseHandle(); a.closeAgentLogs(); a.closeJudges(); a.queueStore.Close() })
	t.Cleanup(func() { a.lifecycle().StopAll(); a.closeAppletBridge() })
	if info := a.AgentCheck(antigravityID); info.Status != "connected" {
		t.Fatalf("Antigravity did not connect: %s %s", info.Status, info.Detail)
	}
	info, err := a.NewSessionWithProviderAt(ws, antigravityID)
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	if !strings.HasPrefix(s.ID(), "antigravity:") {
		t.Fatalf("the session's ID %q is not an Antigravity one", s.ID())
	}
	t.Logf("options: %d, model %q", len(s.Options()), info.Model)
	words, end := liveAsk(t, a, s, "First", "Use no tools. Remember the word heliotrope, and reply with only the word: noted.")
	if end.err != nil || end.stopReason != "end_turn" || !strings.Contains(strings.ToLower(words), "noted") {
		t.Fatalf("the first turn: %q %s %v", words, end.stopReason, end.err)
	}
	a.AgentStop(antigravityID)
	if _, err := a.OpenSessionAt(ws, s.ID()); err != nil {
		t.Fatalf("reopening the chat: %v", err)
	}
	words, end = liveAsk(t, a, a.session(s.ID()), "Second", "Use no tools. Reply with only the word I asked you to remember.")
	if end.err != nil || !strings.Contains(strings.ToLower(words), "heliotrope") {
		t.Fatalf("the reopened chat did not keep the conversation: %q %v", words, end.err)
	}
	t.Log("Antigravity answered, and a reopened chat kept its conversation")
}
