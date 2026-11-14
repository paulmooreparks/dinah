package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/claudeagent"
	"studio/internal/codexagent"
	"studio/internal/mcpserver"
	"studio/internal/ollamaagent"
)

type qualityPauseWire struct {
	Method       string `json:"method"`
	Work         bool   `json:"work"`
	Pause        bool   `json:"pause"`
	Resume       bool   `json:"resume"`
	ExpectedTurn string `json:"expectedTurnId"`
	IdleBehavior string `json:"idleBehavior"`
	Violation    string `json:"violation"`
}

func qualityAwait(t *testing.T, why string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !f() {
		if time.Now().After(deadline) {
			t.Fatal(why)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func qualityWire(t *testing.T, dir string) []qualityPauseWire {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(dir, "wire.jsonl"))
	var out []qualityPauseWire
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var row qualityPauseWire
		if json.Unmarshal([]byte(line), &row) == nil {
			out = append(out, row)
		}
	}
	return out
}
func qualityCount(rows []qualityPauseWire, kind string) int {
	n := 0
	for _, r := range rows {
		if (kind == "work" && r.Work) || (kind == "pause" && r.Pause) || (kind == "resume" && r.Resume) {
			n++
		}
	}
	return n
}

// These cases were written from the design note before reading the candidate.
// Real adapters run against local wire peers. Recovery creates a fresh App
// over the same durable state; this suite does not launch a desktop window.
func TestQualityPauseProviderContract(t *testing.T) {
	for _, mode := range []string{"codex", "claude", "devin", "plain", "ollama"} {
		t.Run(mode, func(t *testing.T) { qualityPausePath(t, mode) })
	}
}
func qualityPausePath(t *testing.T, mode string) {
	a, _ := providerTestApp(t)
	dir := t.TempDir()
	t.Setenv("QUALITY_PAUSE_DIR", dir)
	t.Setenv("QUALITY_PAUSE_MODE", mode)
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("testdata/quality-pause-provider.cjs")
	if err != nil {
		t.Fatal(err)
	}
	provider := mode
	if mode == "plain" {
		provider = "devin"
	}
	var httpMu sync.Mutex
	var httpRows []qualityPauseWire
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			io.WriteString(w, `{"version":"0.34.1"}`)
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"test","model":"test"}]}`)
		case "/api/show":
			io.WriteString(w, `{"capabilities":["completion","tools"]}`)
		case "/api/chat":
			var req struct {
				Messages []struct{ Role, Content string }
			}
			json.NewDecoder(r.Body).Decode(&req)
			var last string
			pauseOccurrences := 0
			for _, m := range req.Messages {
				if m.Role == "user" {
					last = m.Content
					if strings.Contains(m.Content, "Stop at the next safe point") {
						pauseOccurrences++
					}
				}
			}
			row := qualityPauseWire{Method: "/api/chat", Work: strings.Contains(last, "quality working turn"), Pause: strings.Contains(last, "Stop at the next safe point"), Resume: strings.Contains(last, "has restarted")}
			if pauseOccurrences > 1 {
				row.Violation = "pause duplicated in messages"
			}
			httpMu.Lock()
			httpRows = append(httpRows, row)
			httpMu.Unlock()
			if row.Work {
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
			}
			io.WriteString(w, `{"message":{"role":"assistant","content":"Fixture turn complete."},"done":true}`+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	// Release any blocked HTTP exchange before closing the server after failure.
	defer unblock()
	state := t.TempDir()
	connect := func(app *App) agentConnection {
		var c agentConnection
		var e error
		switch mode {
		case "codex":
			c, e = codexagent.Start(node, []string{script}, dir, app.newAgentHandler(), nil)
		case "claude":
			entry := claudeagent.Entry(state)
			os.MkdirAll(filepath.Dir(entry), 0700)
			data, _ := os.ReadFile(script)
			os.WriteFile(entry, data, 0600)
			os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(entry)), "package.json"), []byte(`{"version":"`+claudeagent.Version+`"}`), 0600)
			c, e = claudeagent.Start(node, state, dir, "", app.newAgentHandler(), nil)
		case "ollama":
			c, e = ollamaagent.Start(srv.URL, state, app.newAgentHandler())
		default:
			c, e = acp.Start(node, []string{script}, dir, app.newAgentHandler(), acp.Options{})
		}
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(c.Close)
		var init acp.InitializeResult
		if e = c.Call(context.Background(), "initialize", acp.InitializeParams{ProtocolVersion: 1}, &init); e != nil {
			t.Fatal(e)
		}
		app.runtime.PutProviderLocked(provider, newTestProvider(c, init))
		return c
	}
	rows := func() []qualityPauseWire {
		if mode == "ollama" {
			httpMu.Lock()
			defer httpMu.Unlock()
			return append([]qualityPauseWire(nil), httpRows...)
		}
		return qualityWire(t, dir)
	}
	connect(a)
	turns := make(chan map[string]any, 16)
	a.sink = func(name string, data any) {
		if name == "agent:turn" {
			turns <- data.(map[string]any)
		}
	}
	info, err := a.NewSessionWithProviderAt(t.TempDir(), provider)
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	// Complete one turn so recovery is eligible under the first-turn safeguard.
	if err = a.Send(info.ID, "quality completed first turn", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	if err = a.Send(info.ID, "quality working turn", nil, nil); err != nil {
		t.Fatal(err)
	}
	qualityAwait(t, "working prompt was not received", func() bool { return qualityCount(rows(), "work") == 1 })
	if mode == "devin" {
		data, err := os.ReadFile(filepath.Join(dir, "prompt"))
		if err != nil {
			t.Fatal(err)
		}
		err = a.busHost().With(func(b *bus.Bus) error {
			mcpserver.HostedTurnHook(context.Background(), b, "devin:fixture", "UserPromptSubmit", string(data), "quality-turn")
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if mode != "plain" {
		qualityAwait(t, "working path did not advertise steering", func() bool { return a.CanSteer(info.ID, nil) })
	}
	if err = a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	// A repeated request must not send the pause again while the turn is busy.
	if err = a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	if st := a.UpdateState(); !st.Pending || !st.Pausing || st.Working != 1 {
		t.Fatalf("pause must wait for the peer: %+v", st)
	}
	if mode == "devin" {
		var first, second string
		err = a.busHost().With(func(b *bus.Bus) error {
			first = mcpserver.HostedTurnHook(context.Background(), b, "devin:fixture", "PostToolUse", "", "quality-turn")
			second = mcpserver.HostedTurnHook(context.Background(), b, "devin:fixture", "PostToolUse", "", "quality-turn")
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		var output struct {
			HookSpecificOutput struct {
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal([]byte(first), &output) != nil || strings.Count(output.HookSpecificOutput.AdditionalContext, "Stop at the next safe point") != 1 || strings.Contains(second, "Stop at the next safe point") {
			t.Fatal("hook must deliver one pause through additionalContext")
		}
	} else if mode != "plain" && mode != "ollama" {
		qualityAwait(t, "pause was not delivered once", func() bool { return qualityCount(rows(), "pause") == 1 })
	}
	if mode == "plain" && qualityCount(rows(), "pause") != 0 {
		t.Fatal("plain ACP received unsupported input")
	}
	if mode == "ollama" && qualityCount(rows(), "pause") != 0 {
		t.Fatal("model request was interrupted")
	}
	os.WriteFile(filepath.Join(dir, "finish"), []byte("finish"), 0600)
	unblock()
	waitTurn(t, turns)
	qualityAwait(t, "restart was not ready after turn completion", func() bool { a.updMu.Lock(); defer a.updMu.Unlock(); return a.update.restarting })
	record := readPaused()
	if len(record.Chats) != 1 || !record.Chats[0].Ended || record.Chats[0].Told != (mode != "plain") {
		t.Fatalf("durable pause record: %+v", record)
	}
	if mode == "ollama" && qualityCount(rows(), "pause") != 1 {
		t.Fatal("host loop did not insert one pause at the next exchange")
	}
	b, _ := restarted(t, a)
	connect(b)
	nextTurns := make(chan map[string]any, 16)
	b.sink = func(name string, data any) {
		if name == "agent:turn" {
			nextTurns <- data.(map[string]any)
		}
	}
	if _, err = b.OpenSessionAt(s.Cwd(), s.ID()); err != nil {
		t.Fatal(err)
	}
	b.resumePaused(true)
	ring(t, b)
	waitTurn(t, nextTurns)
	pausedMu.Lock()
	err = writePaused(record)
	pausedMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	b.resumePaused(true)
	ring(t, b)
	if qualityCount(rows(), "resume") != 1 || waiting(t, b, s.BusID()) != 0 {
		t.Fatalf("carry-on must be delivered once: %+v", rows())
	}
	for _, row := range rows() {
		if row.Violation != "" || row.Method == "session/cancel" || row.Method == "turn/interrupt" {
			t.Fatalf("forbidden protocol operation: %+v", row)
		}
		if row.Pause && mode == "codex" && row.ExpectedTurn != "turn-2" {
			t.Fatalf("wrong target: %+v", row)
		}
		if row.Pause && mode == "claude" && row.IdleBehavior != "promptRequired" {
			t.Fatalf("unsafe idle steering: %+v", row)
		}
	}
	t.Logf("%s: pause, completed turn, fresh-App recovery and deduplicated carry-on verified", mode)
}

func TestQualityPauseLeavesIdleClaudeAlone(t *testing.T) {
	a, c := providerTestApp(t)
	c.newIDs = []string{"claude:idle"}
	a.runtime.PutProviderLocked("claude", newTestProvider(c, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{LoadSession: true}}))
	info, err := a.NewSessionWithProviderAt(t.TempDir(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	done, err := a.admitWrite()
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if err = a.PauseAndRestart(); err != nil {
		t.Fatal(err)
	}
	if len(readPaused().Chats) != 0 || len(prompts(c)) != 0 || a.session(info.ID).State().WorkActivity().Busy() {
		t.Fatal("pause started the idle Claude chat")
	}
	a.CancelRestart()
}
