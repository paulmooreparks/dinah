//go:build live

package claudeagent

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
)

// steerWatch keeps every tool call it is told of, as it arrived, and says
// when a command with a given word in it has been seen so many times.
type steerWatch struct {
	mu     sync.Mutex
	calls  []string
	said   []string
	sleeps chan struct{}
	t      *testing.T
}

func (w *steerWatch) Notify(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	if strings.Contains(string(params), `"agent_message_chunk"`) || strings.Contains(string(params), `"user_message_chunk"`) {
		var m struct {
			Update struct {
				Kind    string `json:"sessionUpdate"`
				Content struct {
					Text string `json:"text"`
				} `json:"content"`
				Meta json.RawMessage `json:"_meta"`
			} `json:"update"`
		}
		if json.Unmarshal(params, &m) == nil {
			w.mu.Lock()
			w.said = append(w.said, m.Update.Kind+" "+string(m.Update.Meta)+": "+m.Update.Content.Text)
			w.mu.Unlock()
		}
		return
	}
	if !strings.Contains(string(params), `"tool_call`) {
		return
	}
	w.mu.Lock()
	w.calls = append(w.calls, string(params))
	w.mu.Unlock()
	if strings.Contains(string(params), `"sessionUpdate":"tool_call"`) && strings.Contains(string(params), "sleep 5") {
		select {
		case w.sleeps <- struct{}{}:
		default:
		}
	}
}

// Request allows every tool once, so that the subagent's commands run.
func (w *steerWatch) Request(_ context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	var p struct {
		Options []struct {
			OptionID string `json:"optionId"`
			Kind     string `json:"kind"`
		} `json:"options"`
	}
	_ = json.Unmarshal(params, &p)
	for _, o := range p.Options {
		if o.Kind == "allow_once" {
			return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": o.OptionID}}, nil
		}
	}
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
}

// This opt-in test is a measurement, not a check of a contract: the Agent
// SDK's documentation says when a message pushed into a running turn is read
// and does not say whether the main agent or a subagent reads it. The test
// has Claude start a subagent that runs slow commands, steers the session
// through the adapter's own steering while the subagent works, and asks
// whoever reads the steer to run a marked command. It prints the marked
// command's tool call beside one of the subagent's, so that a person can
// see which agent ran it. It fails only if the turn fails.
func TestLiveWhoReadsASteerWhileASubagentWorks(t *testing.T) {
	if os.Getenv("DINAH_LIVE_CLAUDE") != "1" {
		t.Skip("set DINAH_LIVE_CLAUDE=1 to send live Claude turns")
	}
	state := os.Getenv("DINAH_LIVE_STATE")
	if !InstalledRelease(state, Version) {
		t.Fatalf("the pinned adapter is not installed under %q", state)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	w := &steerWatch{sleeps: make(chan struct{}, 16), t: t}
	c, err := Start(node, state, t.TempDir(), "", w, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, nil); err != nil {
		t.Fatal(err)
	}
	if !c.CanSteer("") {
		t.Fatal("the adapter does not offer its own steering")
	}
	cwd := filepath.Join(t.TempDir(), "w")
	os.MkdirAll(cwd, 0o700)
	var created acp.NewSessionResult
	if err := c.Call(ctx, "session/new", sessionWithMeta{Cwd: cwd, McpServers: []any{}}, &created); err != nil {
		t.Fatal(err)
	}
	prompt := "Start exactly one subagent with the Agent tool, of the general-purpose kind, in the foreground, and wait for it. Its whole task: run the shell command `sleep 5` six times, each as a separate command, one after another, and then report the single word done. Do not run those commands yourself. When the subagent has returned, reply with the single word finished."
	turn := make(chan error, 1)
	go func() {
		turn <- c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: created.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: prompt}}}, nil)
	}()
	// The steer goes in once the subagent's second slow command has started.
	for seen := 0; seen < 2; {
		select {
		case <-w.sleeps:
			seen++
		case err := <-turn:
			w.mu.Lock()
			for _, call := range w.calls {
				if len(call) > 500 {
					call = call[:500]
				}
				t.Log(call)
			}
			t.Log(strings.Join(w.said, " | "))
			w.mu.Unlock()
			t.Fatalf("the turn ended before the subagent ran two commands: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	steer := "A message from the person, sent while you work: whichever agent reads this, run the shell command `echo STEER-SEEN` once, now, as your next command, say in your next message the words STEER READ BY followed by whether you are the main agent or a subagent, and then carry on with what you were doing."
	sent := time.Now()
	if err := c.Steer(ctx, acp.PromptParams{SessionID: created.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: steer}}}, func() {
		t.Logf("the adapter reported the steer delivered after %s", time.Since(sent).Round(time.Millisecond))
	}); err != nil {
		t.Fatalf("the steer: %v", err)
	}
	select {
	case err := <-turn:
		if err != nil {
			t.Fatalf("the turn: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	short := func(s string) string {
		if len(s) > 1400 {
			return s[:1400] + " ..."
		}
		return s
	}
	var sleepCall, marked string
	order := []string{}
	for _, call := range w.calls {
		if !strings.Contains(call, `"sessionUpdate":"tool_call"`) {
			continue
		}
		switch {
		case strings.Contains(call, "STEER-SEEN"):
			order = append(order, "MARKED")
			if marked == "" {
				marked = call
			}
		case strings.Contains(call, "sleep 5"):
			order = append(order, "sleep")
			if sleepCall == "" {
				sleepCall = call
			}
		default:
			order = append(order, "other")
			t.Logf("another tool call:\n%s", short(call))
		}
	}
	t.Logf("tool calls in order: %s", strings.Join(order, ", "))
	t.Logf("what was said, in pieces as they arrived:\n%s", short(strings.Join(w.said, "\n")))
	t.Logf("one of the subagent's commands:\n%s", short(sleepCall))
	if marked == "" {
		t.Log("no agent ran the marked command")
		return
	}
	t.Logf("the marked command:\n%s", short(marked))
}
