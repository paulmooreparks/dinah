package claudeagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
)

// The test binary stands in for the adapter when CLAUDEAGENT_FAKE is set: it
// answers initialize with what its environment holds, gives session/new the
// ID "abc", and answers session/prompt after a session/update and a
// permission request about the session it was asked about.
func TestMain(m *testing.M) {
	if os.Getenv("CLAUDEAGENT_FAKE") == "1" {
		fakeAdapter()
		return
	}
	os.Exit(m.Run())
}

func fakeAdapter() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := json.NewEncoder(os.Stdout)
	var prompt json.RawMessage
	for in.Scan() {
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				SessionID string `json:"sessionId"`
			} `json:"params"`
		}
		if json.Unmarshal(in.Bytes(), &m) != nil {
			continue
		}
		// The answer to the permission request ends the prompt's turn, as
		// an agent waits for the person before going on.
		if m.Method == "" && string(m.ID) == "900" && prompt != nil {
			out.Encode(map[string]any{"jsonrpc": "2.0", "id": prompt, "result": map[string]any{"stopReason": "end_turn"}})
			prompt = nil
			continue
		}
		if m.Method == "" {
			continue
		}
		switch m.Method {
		case "initialize":
			env := []string{}
			for _, v := range append(credentialVariables, "KEEP_ME") {
				if val, ok := os.LookupEnv(v); ok {
					env = append(env, v+"="+val)
				}
			}
			out.Encode(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": map[string]any{"protocolVersion": 1, "agentInfo": map[string]any{"name": strings.Join(env, ";")}}})
		case "session/new":
			out.Encode(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": map[string]any{"sessionId": "abc", "nested": []any{map[string]any{"sessionId": "def", "count": 12345678901234567}}}})
		case "session/prompt":
			out.Encode(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": m.Params.SessionID, "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "the sessionId is " + m.Params.SessionID}}}})
			prompt = m.ID
			out.Encode(map[string]any{"jsonrpc": "2.0", "id": 900, "method": "session/request_permission", "params": map[string]any{"sessionId": m.Params.SessionID}})
		}
	}
}

type recorder struct {
	mu       sync.Mutex
	notified []string
	asked    []string
}

func (r *recorder) Notify(method string, params json.RawMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notified = append(r.notified, method+" "+string(params))
}

func (r *recorder) Request(_ context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asked = append(r.asked, method+" "+string(params))
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
}

// fakeInstall makes the state folder look as if the pinned adapter were
// installed.
func fakeInstall(t *testing.T) string {
	t.Helper()
	state := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(Entry(state)), 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(Entry(state), []byte("// stand-in"), 0o600)
	os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(Entry(state))), "package.json"), []byte(`{"version":"`+Version+`"}`), 0o600)
	return state
}

func start(t *testing.T, apiKey string) (*Conn, *recorder) {
	t.Helper()
	t.Setenv("CLAUDEAGENT_FAKE", "1")
	t.Setenv("KEEP_ME", "yes")
	t.Setenv("ANTHROPIC_API_KEY", "stray-key-from-the-profile")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "stray-token")
	r := &recorder{}
	c, err := Start(os.Args[0], fakeInstall(t), t.TempDir(), apiKey, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c, r
}

func initialize(t *testing.T, c *Conn) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var init acp.InitializeResult
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, &init); err != nil {
		t.Fatal(err)
	}
	return init.AgentInfo.Name
}

// With the person's sign-in chosen, a key or token left in the environment
// never reaches the adapter; with an API key chosen, that key alone does.
func TestTheAdapterGetsOnlyTheChosenCredential(t *testing.T) {
	c, _ := start(t, "")
	if env := initialize(t, c); env != "KEEP_ME=yes" {
		t.Fatalf("with the sign-in chosen the adapter saw %q", env)
	}
	c, _ = start(t, "chosen-key")
	if env := initialize(t, c); env != "ANTHROPIC_API_KEY=chosen-key;KEEP_ME=yes" {
		t.Fatalf("with a key chosen the adapter saw %q", env)
	}
}

// A session's ID has the prefix on Dinah's side and not on the
// adapter's, in answers, notifications and the adapter's own requests.
func TestSessionIDsCarryThePrefixOnOurSideOnly(t *testing.T) {
	c, r := start(t, "")
	initialize(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var created struct {
		SessionID string `json:"sessionId"`
		Nested    []struct {
			SessionID string      `json:"sessionId"`
			Count     json.Number `json:"count"`
		} `json:"nested"`
	}
	if err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: "C:/W"}, &created); err != nil {
		t.Fatal(err)
	}
	if created.SessionID != "claude:abc" || created.Nested[0].SessionID != "claude:def" || created.Nested[0].Count != "12345678901234567" {
		t.Fatalf("the new session came back as %+v", created)
	}
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: created.SessionID}, nil); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.notified) != 1 || !strings.Contains(r.notified[0], `"sessionId":"claude:abc"`) || !strings.Contains(r.notified[0], "the sessionId is abc") {
		t.Fatalf("the update arrived as %v", r.notified)
	}
	if len(r.asked) != 1 || !strings.Contains(r.asked[0], `"sessionId":"claude:abc"`) {
		t.Fatalf("the permission request arrived as %v", r.asked)
	}
}

// A subagent's session ID, which subagent_spawned and subagent_state_update
// give as subagentSessionId, gets the prefix too, so that it matches the
// sessionId of the subagent's own updates on Dinah's side, and loses it on
// the way back, as a session/cancel for the subagent does.
func TestSubagentSessionIDsCarryThePrefix(t *testing.T) {
	in := json.RawMessage(`{"sessionId":"abc","update":{"sessionUpdate":"subagent_spawned","subagentSessionId":"child","name":"Explore"}}`)
	out, err := rewrite(in, add)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"subagentSessionId":"claude:child"`) || !strings.Contains(string(out), `"sessionId":"claude:abc"`) {
		t.Fatalf("the spawn arrived as %s", out)
	}
	out, err = rewrite(acp.CancelParams{SessionID: "claude:child"}, strip)
	if err != nil || string(out) != `{"sessionId":"child"}` {
		t.Fatalf("the cancel went out as %s %v", out, err)
	}
}

func TestEnvDropsCredentialsByName(t *testing.T) {
	got := Env([]string{"PATH=x", "ANTHROPIC_API_KEY=a", "ANTHROPIC_AUTH_TOKEN=b", "CLAUDE_CODE_OAUTH_TOKEN=c", "ANTHROPIC_MODEL=m"}, "")
	if fmt.Sprint(got) != "[PATH=x ANTHROPIC_MODEL=m]" {
		t.Fatalf("the environment was %v", got)
	}
}

func TestInstalledNeedsThePinnedVersion(t *testing.T) {
	state := fakeInstall(t)
	if !Installed(state) {
		t.Fatal("the pinned version was not seen as installed")
	}
	os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(Entry(state))), "package.json"), []byte(`{"version":"0.1.0"}`), 0o600)
	if Installed(state) {
		t.Fatal("another version was taken for the pinned one")
	}
	if _, err := Start(os.Args[0], t.TempDir(), t.TempDir(), "", &recorder{}, nil); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("starting without the adapter said %v", err)
	}
}
