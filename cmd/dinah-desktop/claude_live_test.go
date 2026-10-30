//go:build live

package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"studio/internal/acp"
)

// livePicturePrompt asks an agent to use the test picture applet.
// liveAsk puts text to a chat as a labelled question, as dinah.chat/ask does
// (chat_ask.go), and returns the agent's words and how the turn ended.
func liveAsk(t *testing.T, a *App, s *Session, label, text string) (string, turnEnd) {
	t.Helper()
	p, err := a.promptFrom(s, text, nil)
	if err != nil {
		t.Fatal(err)
	}
	p = p.WithLabel(label)
	return a.askWait(context.Background(), s, p)
}

const livePicturePrompt = "Draw one picture of the text Hello using the test Picture applet. Discover its schema, invoke it, poll completion, and retrieve its SVG artifact through the applet tools. Do not use shell, file editing, external network or unrelated tools. Report the invocation ID."

// This opt-in test runs Claude through the pinned adapter on the person's
// own Claude Code sign-in (docs/proposals/claude-agent-integration.md,
// Acceptance). It installs the adapter with npm into a throwaway settings
// folder, sends two short turns, which use the signed-in plan's allowance,
// and keeps every file in throwaway folders. A bogus ANTHROPIC_API_KEY is
// set in the environment: Claude would refuse it, so a turn that succeeds
// shows the key was kept from the adapter and the sign-in was used.
func TestLiveClaudeAppletAndResume(t *testing.T) {
	if os.Getenv("DINAH_LIVE_CLAUDE") != "1" {
		t.Skip("set DINAH_LIVE_CLAUDE=1 to send live Claude turns")
	}
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-not-a-real-key")
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	packagedAppletHelper(t)
	a := NewApp()
	a.settings.DefaultProvider = "claude"
	ws := t.TempDir()
	a.settings.Workspace = ws
	recorder := record(a)
	sink := a.sink
	a.sink = func(name string, data any) {
		if name != "agent:permission" {
			sink(name, data)
			return
		}
		// Applet tools are allowed once; anything else is refused.
		question := data.(map[string]any)
		var tool struct {
			Title string `json:"title"`
		}
		_ = json.Unmarshal(question["toolCall"].(json.RawMessage), &tool)
		applet := strings.Contains(tool.Title, "applet_") || strings.Contains(tool.Title, "dinah-applets")
		choice := ""
		for _, option := range question["options"].([]acp.PermissionOption) {
			if applet && option.Kind == "allow_once" {
				choice = option.OptionID
				break
			}
			if !applet && option.Kind == "reject_once" {
				choice = option.OptionID
			}
		}
		t.Logf("Claude permission %q: chose %q", tool.Title, choice)
		a.Answer(question["key"].(string), choice)
	}
	t.Cleanup(func() { a.lifecycle().StopAll(); a.closeAppletBridge() })
	if info, err := a.InstallClaudeAdapter(); err != nil || !info.Available {
		t.Fatalf("the adapter was not installed: %v %+v", err, info)
	}
	if info := a.AgentCheck("claude"); info.Status != "connected" {
		t.Fatalf("Claude did not connect: %s %s", info.Status, info.Detail)
	}
	withTestApplets(t, a)
	info, err := a.NewSessionWithProviderAt(ws, "claude")
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	if !strings.HasPrefix(s.ID(), "claude:") {
		t.Fatalf("the session's ID %q is not a Claude one", s.ID())
	}
	if _, end := liveAsk(t, a, s, "Picture", livePicturePrompt); end.err != nil || end.stopReason != "end_turn" {
		t.Fatalf("the turn failed: %s %v", end.stopReason, end.err)
	}
	invocations, err := a.AppletInvocationsAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	pictureID := ""
	for _, v := range invocations {
		if v.OwnerID == s.ID() && v.Action == testPictureRef && v.Status == "done" && len(v.Artifacts) == 1 {
			pictureID = v.ID
		}
	}
	if pictureID == "" {
		recorder.mu.Lock()
		response := recorder.said.String()
		recorder.mu.Unlock()
		t.Fatalf("Claude did not draw a picture through the applet: %s", response)
	}
	if a.runtime.ProviderLocked("devin") != nil {
		t.Fatal("the Claude chat started Devin")
	}
	a.AgentStop("claude")
	if _, err := a.OpenSessionAt(ws, s.ID()); err != nil {
		t.Fatal(err)
	}
	words, end := liveAsk(t, a, a.session(s.ID()), "Resume", "Reply with only the picture invocation ID from this conversation. Do not invoke any tools or draw another picture.")
	if end.err != nil || !strings.Contains(words, pictureID) {
		t.Fatalf("the reopened chat did not keep the picture's result: %q %v", words, end.err)
	}
	t.Log("Claude, the picture applet, and a reopened chat passed on the sign-in, with a stray API key ignored")
}
