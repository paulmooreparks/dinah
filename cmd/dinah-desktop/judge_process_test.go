package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/judge"
)

// judgeProcessConn is the agent of a process for judging: it answers a call, and
// starts a tool in one when a test makes it.
type judgeProcessConn struct {
	handler acp.Handler
	done    chan struct{}
	once    sync.Once

	mu       sync.Mutex
	init     acp.InitializeParams
	sessions int
	model    string
	// tool makes the agent start a tool in the prompt and wait to be stopped.
	tool bool
}

func (c *judgeProcessConn) update(session string, update map[string]any) {
	params, _ := json.Marshal(map[string]any{"sessionId": session, "update": update})
	c.handler.Notify("session/update", params)
}

func (c *judgeProcessConn) Call(ctx context.Context, method string, p, out any) error {
	switch method {
	case "initialize":
		c.mu.Lock()
		c.init = p.(acp.InitializeParams)
		c.mu.Unlock()
		*out.(*acp.InitializeResult) = acp.InitializeResult{ProtocolVersion: 1}
	case "session/new":
		c.mu.Lock()
		c.sessions++
		id := fmt.Sprintf("judge-%d", c.sessions)
		c.mu.Unlock()
		*out.(*acp.NewSessionResult) = acp.NewSessionResult{SessionID: id, ConfigOptions: []acp.ConfigOption{{ID: "model", Category: "model", Type: "select",
			CurrentValue: json.RawMessage(`"initial"`), Options: json.RawMessage(`[{"value":"initial","name":"Initial"},{"value":"small","name":"Small"}]`)}}}
	case "session/set_config_option":
		c.mu.Lock()
		if raw, ok := p.(acp.SetConfigOptionParams).Value.(json.RawMessage); ok {
			_ = json.Unmarshal(raw, &c.model)
		}
		c.mu.Unlock()
	case "session/prompt":
		id := p.(acp.PromptParams).SessionID
		c.mu.Lock()
		tool := c.tool
		c.mu.Unlock()
		if tool {
			c.update(id, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t1", "title": "Run echo", "kind": "execute", "status": "pending"})
			<-ctx.Done()
			return ctx.Err()
		}
		c.update(id, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": `{"line":"Judged"}`}})
		c.update(id, map[string]any{"sessionUpdate": "usage_update", "used": 15700, "size": 200000})
		*out.(*acp.PromptResult) = acp.PromptResult{StopReason: "end_turn"}
	}
	return nil
}
func (c *judgeProcessConn) Notify(string, any) error                  { return nil }
func (c *judgeProcessConn) Done() <-chan struct{}                     { return c.done }
func (c *judgeProcessConn) Err() error                                { return nil }
func (c *judgeProcessConn) Close()                                    { c.once.Do(func() { close(c.done) }) }
func (c *judgeProcessConn) RegisterTraceSecret(string)                {}
func (c *judgeProcessConn) RegisterTraceRedactor(func(string) string) {}

func (c *judgeProcessConn) ended() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// judgeProcessFixture is a judge module whose process is a judgeProcessConn, with
// the connections it started and the folders they were started in.
type judgeProcessFixture struct {
	mu      sync.Mutex
	started []*judgeProcessConn
	dirs    []string
	empty   []bool
	tool    bool
}

func (f *judgeProcessFixture) start(_ *App, dir string, handler acp.Handler) (agentConnection, error) {
	entries, err := os.ReadDir(dir)
	f.mu.Lock()
	defer f.mu.Unlock()
	c := &judgeProcessConn{handler: handler, done: make(chan struct{}), tool: f.tool}
	f.started = append(f.started, c)
	f.dirs = append(f.dirs, dir)
	f.empty = append(f.empty, err == nil && len(entries) == 0)
	return c, nil
}

func (f *judgeProcessFixture) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.started)
}

func judgeProcessJudge(t *testing.T, f *judgeProcessFixture) (*App, judge.Judge) {
	t.Helper()
	a, _ := providerTestApp(t)
	module := processJudgeModule(f.start, "note", "not calibrated")
	return a, module.Build(a, "devin", "small")
}

// The process starts with the first call, in an empty folder, with a client that
// offers the agent no files and no terminal; later calls use the same process,
// each in a session of its own; the model asked for is set; and the cost is
// recorded as unknown, since the usage reported is not in the budget's unit.
func TestAJudgeProcessStartsOnceAndItsCostIsUnknown(t *testing.T) {
	f := &judgeProcessFixture{}
	a, j := judgeProcessJudge(t, f)
	for i := 0; i < 2; i++ {
		resp, err := j.Ask(context.Background(), judge.Request{System: "Name the chat.", Input: `{"x":1}`, JSON: true})
		if err != nil || resp.Text != `{"line":"Judged"}` || resp.Model != "small" || resp.Provider != "devin" {
			t.Fatalf("call %d: %+v %v", i, resp, err)
		}
		if resp.Cost != (judge.Cost{Unknown: true}) {
			t.Fatalf("call %d: cost %+v", i, resp.Cost)
		}
	}
	if f.count() != 1 || !f.empty[0] {
		t.Fatalf("started %d times, in an empty folder %v", f.count(), f.empty)
	}
	c := f.started[0]
	if c.sessions != 2 || c.model != "small" {
		t.Fatalf("sessions %d, model %q", c.sessions, c.model)
	}
	if caps := c.init.ClientCapabilities; caps.FS.ReadTextFile || caps.FS.WriteTextFile || caps.Terminal {
		t.Fatalf("the client offered %+v", caps)
	}
	// The connection the chats use was not used.
	if a.judging.sessions != nil {
		t.Fatal("the call was registered among the shared connection's judge sessions")
	}
	// Closing the judges ends the process and removes its folder, and no call
	// starts another.
	a.closeJudges()
	if !c.ended() {
		t.Fatal("the process outlived the judges")
	}
	if _, err := os.Stat(f.dirs[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the folder of the process is still there: %v", err)
	}
	if _, err := j.Ask(context.Background(), judge.Request{System: "Name the chat.", Input: "{}"}); err == nil || f.count() != 1 {
		t.Fatalf("a call after closing: %v, started %d", err, f.count())
	}
}

// An agent whose own rules refuse a tool asks the client nothing, so the start of
// a tool is what ends the call, with no answer.
func TestAJudgeProcessCallEndsWhenTheAgentStartsATool(t *testing.T) {
	f := &judgeProcessFixture{tool: true}
	_, j := judgeProcessJudge(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := j.Ask(ctx, judge.Request{System: "Name the chat.", Input: "{}"})
	if err == nil || !strings.Contains(err.Error(), "tried to use a tool (Run echo)") || resp.Text != "" {
		t.Fatalf("response %+v, error %v", resp, err)
	}
	if ctx.Err() != nil {
		t.Fatal("the call was not ended until its time ran out")
	}
}

// The process ends when no call has used it for a while, and the next call starts
// another.
func TestAJudgeProcessEndsWhenIdleAndStartsAgain(t *testing.T) {
	old := judgeProcessIdle
	judgeProcessIdle = 20 * time.Millisecond
	t.Cleanup(func() { judgeProcessIdle = old })
	f := &judgeProcessFixture{}
	_, j := judgeProcessJudge(t, f)
	if _, err := j.Ask(context.Background(), judge.Request{System: "Name the chat.", Input: "{}"}); err != nil {
		t.Fatal(err)
	}
	first := f.started[0]
	for deadline := time.Now().Add(5 * time.Second); !first.ended(); {
		if time.Now().After(deadline) {
			t.Fatal("the idle process did not end")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := j.Ask(context.Background(), judge.Request{System: "Name the chat.", Input: "{}"}); err != nil || f.count() != 2 {
		t.Fatalf("the call after the idle end: %v, started %d", err, f.count())
	}
}

// A process that stopped by itself is replaced by the next call.
func TestAJudgeProcessThatStoppedIsStartedAgain(t *testing.T) {
	f := &judgeProcessFixture{}
	_, j := judgeProcessJudge(t, f)
	if _, err := j.Ask(context.Background(), judge.Request{System: "Name the chat.", Input: "{}"}); err != nil {
		t.Fatal(err)
	}
	f.started[0].Close()
	if _, err := j.Ask(context.Background(), judge.Request{System: "Name the chat.", Input: "{}"}); err != nil || f.count() != 2 {
		t.Fatalf("the call after the stop: %v, started %d", err, f.count())
	}
}

// The judge file for Devin denies every documented tool and pattern and allows
// nothing, and it is written whole, so an allowance saved in it does not survive a
// start.
func TestDevinJudgeRulesDenyEveryToolAndAllowNothing(t *testing.T) {
	providerTestApp(t)
	path := filepath.Join(configDir(), "devin-judge.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"permissions":{"allow":["exec","Write(**)"],"deny":[]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := devinJudgeConfig()
	if err != nil || got != path {
		t.Fatalf("path %q %v", got, err)
	}
	b, _ := os.ReadFile(path)
	var file struct {
		Permissions permissions `json:"permissions"`
	}
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Permissions.Allow) != 0 || len(file.Permissions.Ask) != 0 {
		t.Fatalf("the judge file allows or asks: %s", b)
	}
	for _, rule := range []string{"read", "edit", "grep", "glob", "exec", "Read(/**)", "Read(**)", "Write(/**)", "Write(**)", "Fetch(https://*)", "Fetch(http://*)", "mcp__*"} {
		found := false
		for _, d := range file.Permissions.Deny {
			found = found || d == rule
		}
		if !found {
			t.Errorf("%s is not denied", rule)
		}
	}
}

// Devin can judge, its calls are said to use the plan, and its models are not
// calibrated, by a run the person starts or by the automatic one.
func TestDevinJudgesAndIsNotCalibrated(t *testing.T) {
	a, _, _ := judgementApp(t)
	module := providerModules["devin"].Judge
	if module == nil || module.Note == "" || module.NoCalibration == "" {
		t.Fatalf("the judge module of Devin: %+v", module)
	}
	choose(t, a, func(s *JudgementSettings) { s.Provider = "devin"; s.Model = "" })
	if err := a.RunJudgementCalibration(); err == nil || err.Error() != module.NoCalibration {
		t.Fatalf("a calibration of Devin: %v", err)
	}
	if !a.beginCalibration() {
		t.Fatal("the refused run was left marked as running")
	}
	a.runCalibration(true)
	if view, err := a.JudgementCalibration(); err != nil || view.Running || view.State != "idle" {
		t.Fatalf("after an automatic run: %+v %v", view, err)
	}
}
