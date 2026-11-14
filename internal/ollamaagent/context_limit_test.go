package ollamaagent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"studio/internal/acp"
)

func TestLocalContextCeilingsAndMissingMetadata(t *testing.T) {
	for _, tc := range []struct{ chosen, model, cap, want int }{
		{0, 262144, 65536, 65536}, {0, 8192, 65536, 8192},
		{16384, 262144, 65536, 16384}, {131072, 262144, 65536, 65536},
		{16384, 8192, 65536, 8192}, {0, 0, 65536, 4096},
		{0, 0, 2048, 2048}, {8192, 0, 65536, 8192},
	} {
		if got := localContext(tc.chosen, tc.model, tc.cap); got != tc.want {
			t.Errorf("%+v got %d", tc, got)
		}
	}
	for _, n := range []float64{-1, 0, 1.5, 2147483648} {
		s := shown{ModelInfo: map[string]any{"general.architecture": "test", "test.context_length": n}}
		if s.contextLength() != 0 {
			t.Errorf("invalid context metadata %v was accepted", n)
		}
	}
}

func TestAutomaticContextIsSentAndSettingsChangesReachOpenChats(t *testing.T) {
	srv, count, seen := countingOllama(t)
	c, err := Start(srv.URL, t.TempDir(), &host{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var opened acp.NewSessionResult
	if err := c.Call(context.Background(), "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	sendTurn(t, c, opened.SessionID, count, 100)
	if got := (*seen)[0].Options["num_ctx"]; got != float64(65536) {
		t.Fatalf("automatic request = %v", got)
	}
	cap := 32768
	c.SetContextCap(func() int { return cap })
	sendTurn(t, c, opened.SessionID, count, 200)
	if got := (*seen)[1].Options["num_ctx"]; got != float64(32768) {
		t.Fatalf("changed request = %v", got)
	}
	if st := windowState(t, c, opened.SessionID); !st.Known || st.Size != 32768 {
		t.Fatalf("meter = %+v", st)
	}
	cap = 262144
	sendTurn(t, c, opened.SessionID, count, 300)
	if got := (*seen)[2].Options["num_ctx"]; got != float64(131072) {
		t.Fatalf("model ceiling = %v", got)
	}
	if rec, _ := c.load(opened.SessionID); rec.Context != 0 {
		t.Fatalf("automatic choice was frozen: %d", rec.Context)
	}
}

func TestAFullResponseEndsTheTurnWithAnExplicitWarning(t *testing.T) {
	c, h, count, _, id := windowChat(t)
	count.Store(2048)
	var result acp.PromptResult
	if err := c.Call(context.Background(), "session/prompt", acp.PromptParams{SessionID: id, Prompt: []acp.ContentBlock{{Type: "text", Text: "Continue the task"}}}, &result); err != nil {
		t.Fatal(err)
	}
	if result.StopReason != "max_tokens" || !strings.Contains(h.said(), "The turn ended with its context window full") {
		t.Fatalf("result = %+v, updates = %s", result, h.said())
	}
	if contextFullNotice(2047, 2048) != "" || contextFullNotice(2048, 0) != "" {
		t.Fatal("an incomplete window was reported full")
	}
}

func TestLengthStopBelowTheWindowDoesNotClaimContextExhaustion(t *testing.T) {
	base, _, _ := countingOllama(t)
	upstream, _ := url.Parse(base.URL)
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			io.WriteString(w, `{"message":{"role":"assistant","content":"partial"},"done":true,"done_reason":"length","prompt_eval_count":100,"eval_count":10}`+"\n")
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer srv.Close()
	h := &host{}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var opened acp.NewSessionResult
	if err := c.Call(context.Background(), "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	var result acp.PromptResult
	if err := c.Call(context.Background(), "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Continue"}}}, &result); err != nil {
		t.Fatal(err)
	}
	if result.StopReason != "max_tokens" || !strings.Contains(h.said(), "Ollama reported that this response ended with reason length") || strings.Contains(h.said(), "context window full") {
		t.Fatalf("result = %+v, updates = %s", result, h.said())
	}
}

func TestContextFullToolCallsReceiveResultsBeforeStopping(t *testing.T) {
	srv, seen := fakeOllama(t, "read_file")
	h := &host{answer: "once"}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var opened acp.NewSessionResult
	if err := c.Call(context.Background(), "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	// The fixture's first response reports 110 tokens and asks to read a file.
	c.SetContextCap(func() int { return 110 })
	var result acp.PromptResult
	if err := c.Call(context.Background(), "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Read notes.md"}}}, &result); err != nil {
		t.Fatal(err)
	}
	rec, err := c.load(opened.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if result.StopReason != "max_tokens" || len(*seen) != 1 || rec.Messages[len(rec.Messages)-1].Role != "tool" || !strings.Contains(h.said(), "context window full") {
		t.Fatalf("result=%+v, requests=%d, messages=%+v", result, len(*seen), rec.Messages)
	}
}
