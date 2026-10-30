package hooks

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
)

type liveSteeringHost struct {
	mu   sync.Mutex
	text strings.Builder
}

func (h *liveSteeringHost) Notify(_ string, data json.RawMessage) {
	var p struct {
		Update struct {
			Kind    string `json:"sessionUpdate"`
			Content struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"update"`
	}
	json.Unmarshal(data, &p)
	if p.Update.Kind == "agent_message_chunk" {
		h.mu.Lock()
		h.text.WriteString(p.Update.Content.Text)
		h.mu.Unlock()
	}
}

func (h *liveSteeringHost) Request(_ context.Context, method string, data json.RawMessage) (any, *acp.Error) {
	if method == "session/request_permission" {
		var p acp.RequestPermissionParams
		json.Unmarshal(data, &p)
		for _, option := range p.Options {
			if option.Kind == "allow_once" {
				return acp.Selected(option.OptionID), nil
			}
		}
	}
	return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: method}
}

// This opt-in check spends one real Devin turn in an isolated workspace.
// DINAH_STEERING_TEST_CLI points to the just-built dinah executable.
func TestLiveDevinACPHooksSteerTheSameTurn(t *testing.T) {
	helper := os.Getenv("DINAH_STEERING_TEST_CLI")
	if helper == "" {
		t.Skip("live Devin steering check is opt-in")
	}
	devin, err := exec.LookPath("devin")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	state, workspace := t.TempDir(), t.TempDir()
	b, err := bus.OpenAt(filepath.Join(state, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	chat, err := b.RegisterSession(ctx, "live-steering-window", workspace, "")
	if err != nil {
		t.Fatal(err)
	}
	const hostTurn = "live-host-turn"
	if err := b.BeginHookTurn(ctx, chat.ID, hostTurn); err != nil {
		t.Fatal(err)
	}
	defer b.EndHookTurn(context.Background(), chat.ID, hostTurn)
	key, err := b.QueueSteer(ctx, chat.ID, "In your final reply include the exact token STEER_HOOK_REACHED_74193. This is guidance for your existing task.")
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]any{}
	for _, event := range []string{"UserPromptSubmit", "PostToolUse", "Stop"} {
		groups[event] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "dinah steering-hook", "timeout": 10}}}}
	}
	config := filepath.Join(workspace, ".devin")
	os.MkdirAll(config, 0o700)
	hooksJSON, _ := json.Marshal(groups)
	if err := os.WriteFile(filepath.Join(config, "hooks.v1.json"), hooksJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	h := &liveSteeringHost{}
	env := append(os.Environ(), "DINAH_HOME="+state, bus.HostedVar+"=1", "PATH="+filepath.Dir(helper)+string(os.PathListSeparator)+os.Getenv("PATH"))
	c, err := acp.Start(devin, []string{"acp"}, workspace, h, acp.Options{Env: env})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var init acp.InitializeResult
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1, ClientCapabilities: acp.ClientCaps()}, &init); err != nil {
		t.Fatal(err)
	}
	c.Notify("initialized", map[string]any{})
	var opened acp.NewSessionResult
	if err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: workspace, McpServers: []any{}}, &opened); err != nil {
		t.Fatal(err)
	}
	var result acp.PromptResult
	text := "Run the harmless shell command echo STEERING_COMMAND_PROBE once. Then finish with one brief sentence. Do not write files.\n" + bus.SteerTurnMarker(chat.ID, hostTurn)
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: text}}}, &result); err != nil {
		t.Fatal(err)
	}
	delivered, err := b.SteerDelivered(ctx, key)
	if err != nil || !delivered {
		t.Fatalf("hook did not deliver the steer: %v", err)
	}
	h.mu.Lock()
	output := h.text.String()
	h.mu.Unlock()
	if !strings.Contains(output, "STEER_HOOK_REACHED_74193") {
		t.Fatalf("steer was not used in the same prompt response: %s", output)
	}
	t.Logf("Devin ACP consumed hook steering in one prompt; stop reason %s", result.StopReason)
}
