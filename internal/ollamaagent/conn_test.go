package ollamaagent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
)

// host is a fake Dinah: it reads files, answers permission questions
// with answer, and keeps the updates it is sent.
type host struct {
	mu      sync.Mutex
	updates []string
	asked   []string
	answer  string
}

func (h *host) Notify(method string, params json.RawMessage) {
	h.mu.Lock()
	h.updates = append(h.updates, string(params))
	h.mu.Unlock()
}

func (h *host) Request(ctx context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	switch method {
	case "fs/read_text_file":
		var p acp.ReadTextFileParams
		_ = json.Unmarshal(params, &p)
		b, err := os.ReadFile(p.Path)
		if err != nil {
			return nil, &acp.Error{Code: acp.CodeInternal, Message: err.Error()}
		}
		return acp.ReadTextFileResult{Content: string(b)}, nil
	case "session/request_permission":
		h.mu.Lock()
		h.asked = append(h.asked, string(params))
		h.mu.Unlock()
		return acp.Selected(h.answer), nil
	}
	return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: method}
}

func (h *host) said() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.updates, "\n")
}

// fakeOllama answers as Ollama's API documents: the first answer of a turn
// asks for a tool, and the answer after the tool's result quotes it.
func fakeOllama(t *testing.T, call string) (*httptest.Server, *[]chatRequest) {
	var mu sync.Mutex
	var seen []chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			io.WriteString(w, `{"version":"0.34.1"}`)
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"glm:cloud","model":"glm:cloud","remote_host":"https://ollama.com:443","details":{}},{"name":"nano:4b","model":"nano:4b","details":{"parameter_size":"4.0B"}}]}`)
		case "/api/show":
			io.WriteString(w, `{"capabilities":["completion","tools","thinking"],"model_info":{"general.architecture":"nano","nano.context_length":32768}}`)
		case "/api/chat":
			var req chatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			seen = append(seen, req)
			mu.Unlock()
			last := req.Messages[len(req.Messages)-1]
			if last.Role == "user" {
				io.WriteString(w, `{"message":{"role":"assistant","content":"","thinking":"I should look."},"done":false}`+"\n")
				io.WriteString(w, `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"`+call+`","arguments":{"path":"notes.md","command":"del notes.md"}}}]},"done":false}`+"\n")
				io.WriteString(w, `{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":100,"eval_count":10}`+"\n")
				return
			}
			io.WriteString(w, `{"message":{"role":"assistant","content":"The tool said: "},"done":false}`+"\n")
			b, _ := json.Marshal(last.Content)
			io.WriteString(w, `{"message":{"role":"assistant","content":`+string(b)+`},"done":true,"done_reason":"stop","prompt_eval_count":120,"eval_count":12}`+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

// A turn runs the tools the model asks for and gives it their results; the
// conversation is kept and reopens; models on this computer come first, and
// the context offered stops at the model's own length.
func TestTurnRunsTheToolsAndKeepsTheConversation(t *testing.T) {
	srv, seen := fakeOllama(t, "read_file")
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "notes.md"), []byte("The release ships on Friday."), 0o644)
	h := &host{answer: "once"}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	var opened acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: ws}}, &opened); err != nil {
		t.Fatal(err)
	}
	if !valid(opened.SessionID) || string(opened.ConfigOptions[0].CurrentValue) != `"nano:4b"` || !strings.Contains(string(opened.ConfigOptions[0].Options), "glm:cloud (Ollama cloud)") {
		t.Fatalf("opened %+v", opened)
	}
	var ctxOption acp.ConfigOption
	for _, o := range opened.ConfigOptions {
		if o.ID == "context" {
			ctxOption = o
		}
	}
	// A new conversation leaves the length to Ollama's own setting, and the
	// sizes offered stop at the model's limit.
	if string(ctxOption.CurrentValue) != `"0"` || !strings.Contains(string(ctxOption.Options), "Ollama's setting") || strings.Contains(string(ctxOption.Options), "65536") {
		t.Fatalf("context %+v", ctxOption)
	}
	var res acp.PromptResult
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "When does the release ship?"}}}, &res); err != nil {
		t.Fatal(err)
	}
	said := h.said()
	if res.StopReason != "end_turn" || !strings.Contains(said, "The release ships on Friday.") || !strings.Contains(said, `"tool_call"`) || !strings.Contains(said, "I should look.") || !strings.Contains(said, "usage_update") || !strings.Contains(said, `"sizeKind":"model"`) || !strings.Contains(said, `"size":32768`) {
		t.Fatalf("stop %q, updates:\n%s", res.StopReason, said)
	}
	if len(*seen) != 2 || len((*seen)[0].Tools) == 0 || (*seen)[0].Options["num_ctx"] != nil || (*seen)[1].Messages[len((*seen)[1].Messages)-1].ToolName != "read_file" {
		t.Fatalf("requests %+v", *seen)
	}
	// The conversation is listed for its workspace and reopens with its
	// messages.
	var list acp.ListSessionsResult
	c.Call(ctx, "session/list", acp.ListSessionsParams{Cwd: ws}, &list)
	if len(list.Sessions) != 1 || list.Sessions[0].Title != "When does the release ship?" {
		t.Fatalf("list %+v", list)
	}
	h2 := &host{}
	c2, _ := Start(srv.URL, filepath.Dir(filepath.Dir(c.dir)), h2)
	defer c2.Close()
	if err := c2.Call(ctx, "session/load", SessionParams{LoadSessionParams: acp.LoadSessionParams{SessionID: opened.SessionID, Cwd: ws}}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h2.said(), "When does the release ship?") || !strings.Contains(h2.said(), "The release ships on Friday.") {
		t.Fatalf("replay %s", h2.said())
	}
	if err := c2.Call(ctx, "session/load", SessionParams{LoadSessionParams: acp.LoadSessionParams{SessionID: opened.SessionID, Cwd: t.TempDir()}}, nil); err == nil {
		t.Fatal("a conversation opened in another workspace")
	}
	// A removed conversation is gone.
	if err := c2.Call(ctx, "session/delete", acp.DeleteSessionParams{SessionID: opened.SessionID}, nil); err != nil {
		t.Fatal(err)
	}
	if c2.Call(ctx, "session/list", acp.ListSessionsParams{Cwd: ws}, &list); len(list.Sessions) != 0 {
		t.Fatalf("a removed conversation is listed: %+v", list)
	}
}

// A closed conversation takes no prompt until it is loaded again, and
// loading it replays what it kept.
func TestCloseLetsAConversationGoAndLoadBringsItBack(t *testing.T) {
	srv, _ := fakeOllama(t, "read_file")
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "notes.md"), []byte("The release ships on Friday."), 0o644)
	h := &host{answer: "once"}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	var init acp.InitializeResult
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, &init); err != nil {
		t.Fatal(err)
	}
	if init.AgentCapabilities.SessionCapabilities.Close == nil {
		t.Fatal("session/close is not advertised")
	}
	var opened acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: ws}}, &opened); err != nil {
		t.Fatal(err)
	}
	prompt := acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "When does the release ship?"}}}
	if err := c.Call(ctx, "session/prompt", prompt, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: opened.SessionID}, nil); err != nil {
		t.Fatal(err)
	}
	var notActive *acp.Error
	if err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: opened.SessionID}, nil); !errors.As(err, &notActive) || notActive.Code != acp.CodeResourceNotFound {
		t.Fatalf("closing it again answered %v", err)
	}
	if err := c.Call(ctx, "session/prompt", prompt, nil); err == nil {
		t.Fatal("a closed conversation took a prompt")
	}
	h2 := &host{}
	c.handler = h2
	if err := c.Call(ctx, "session/load", SessionParams{LoadSessionParams: acp.LoadSessionParams{SessionID: opened.SessionID, Cwd: ws}}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h2.said(), "When does the release ship?") {
		t.Fatalf("replay %s", h2.said())
	}
}

// A command runs only with the person's permission; refused, the model is
// told so and nothing runs.
func TestCommandAsksFirst(t *testing.T) {
	srv, seen := fakeOllama(t, "run_command")
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "notes.md"), []byte("kept"), 0o644)
	h := &host{answer: "reject"}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	var opened acp.NewSessionResult
	c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: ws}}, &opened)
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Delete the notes."}}}, nil); err != nil {
		t.Fatal(err)
	}
	if len(h.asked) != 1 || !strings.Contains(h.asked[0], "del notes.md") {
		t.Fatalf("asked %v", h.asked)
	}
	if _, err := os.Stat(filepath.Join(ws, "notes.md")); err != nil {
		t.Fatal("the command ran without permission")
	}
	if got := (*seen)[1].Messages[len((*seen)[1].Messages)-1].Content; !strings.Contains(got, "did not allow") {
		t.Fatalf("the model was told %q", got)
	}
}

// A glob with ** matches at any depth, and a search gives file and line.
func TestFindAndSearch(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(root, "a", "b", "c.md"), []byte("one\nthe lexer\n"), 0o644)
	os.WriteFile(filepath.Join(root, "top.md"), []byte("x"), 0o644)
	if out, ok := find(context.Background(), root, "**/*.md"); !ok || out != "a/b/c.md\ntop.md" {
		t.Fatalf("find %q", out)
	}
	if out, ok := search(context.Background(), root, "lexer", "*.md"); !ok || out != "a/b/c.md:2: the lexer" {
		t.Fatalf("search %q", out)
	}
}
