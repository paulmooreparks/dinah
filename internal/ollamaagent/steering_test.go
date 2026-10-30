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
	"time"

	"studio/internal/acp"
)

func TestSteeringIsInsertedAtTheNextModelExchange(t *testing.T) {
	for _, scenario := range []string{"final response", "tool result", "save failure", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			tool := scenario == "tool result"
			started, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			var requests []chatRequest
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/version":
					io.WriteString(w, `{"version":"0.34.1"}`)
				case "/api/tags":
					io.WriteString(w, `{"models":[{"name":"test","model":"test"}]}`)
				case "/api/show":
					io.WriteString(w, `{"capabilities":["completion","tools"]}`)
				case "/api/chat":
					var req chatRequest
					json.NewDecoder(r.Body).Decode(&req)
					requests = append(requests, req)
					if len(requests) == 1 {
						close(started)
						select {
						case <-release:
						case <-r.Context().Done():
							return
						}
						if tool {
							io.WriteString(w, `{"message":{"role":"assistant","tool_calls":[{"function":{"name":"read_file","arguments":{"path":"notes.md"}}}]},"done":true}`+"\n")
							return
						}
						io.WriteString(w, `{"message":{"role":"assistant","content":"First answer."},"done":true}`+"\n")
						return
					}
					io.WriteString(w, `{"message":{"role":"assistant","content":"Updated answer."},"done":true}`+"\n")
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			ws := t.TempDir()
			os.WriteFile(filepath.Join(ws, "notes.md"), []byte("retained tool result"), 0o600)
			c, err := Start(srv.URL, t.TempDir(), &host{})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var opened acp.NewSessionResult
			if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: ws}}, &opened); err != nil {
				t.Fatal(err)
			}
			finished := make(chan error, 1)
			go func() {
				finished <- c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Start."}}}, nil)
			}()
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			delivered := make(chan struct{}, 1)
			if err := c.Steer(ctx, acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Use the revised scope."}}}, func() { delivered <- struct{}{} }); err != nil {
				t.Fatal(err)
			}
			select {
			case <-delivered:
				t.Fatal("steering interrupted a model exchange")
			default:
			}
			if scenario == "save failure" {
				blocked := filepath.Join(t.TempDir(), "not-a-directory")
				if err := os.WriteFile(blocked, []byte("block writes"), 0600); err != nil {
					t.Fatal(err)
				}
				c.dir = blocked
			}
			if scenario == "cancel" {
				if err := c.cancelTurn(acp.CancelParams{SessionID: opened.SessionID}); err != nil {
					t.Fatal(err)
				}
				if err := c.Steer(ctx, acp.PromptParams{SessionID: opened.SessionID}, nil); !errors.Is(err, acp.ErrNoActiveTurn) {
					t.Fatalf("steering after cancel: %v", err)
				}
			}
			unblock()
			select {
			case err := <-finished:
				if (err != nil) != (scenario == "save failure") {
					t.Fatalf("turn error: %v", err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if scenario == "cancel" || scenario == "save failure" {
				select {
				case <-delivered:
					t.Fatal("undelivered input acknowledged")
				default:
				}
				if len(requests) != 1 {
					t.Fatalf("unexpected model requests: %d", len(requests))
				}
				c.mu.Lock()
				pending := len(c.sessions[opened.SessionID].steers)
				c.mu.Unlock()
				if pending != 0 {
					t.Fatal("adapter retained input owned by host")
				}
				return
			}
			select {
			case <-delivered:
			default:
				t.Fatal("no delivery receipt")
			}
			if len(requests) != 2 {
				t.Fatalf("%d model exchanges", len(requests))
			}
			messages := requests[1].Messages
			if last := messages[len(messages)-1]; last.Role != "user" || last.Content != "Use the revised scope." {
				t.Fatalf("steer missing: %+v", messages)
			}
			if tool && (messages[len(messages)-2].Role != "tool" || !strings.Contains(messages[len(messages)-2].Content, "retained tool result")) {
				t.Fatalf("tool result sequence broken: %+v", messages)
			}
			rec, err := c.load(opened.SessionID)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, m := range rec.Messages {
				if m.Content == "Use the revised scope." {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("persisted steer %d times", count)
			}
			if err := c.Steer(ctx, acp.PromptParams{SessionID: opened.SessionID}, nil); !errors.Is(err, acp.ErrNoActiveTurn) {
				t.Fatalf("idle steering: %v", err)
			}
		})
	}
}

// cancellingHost cancels the turn when the reply's last words reach it,
// after the reply has come back whole but before the turn has used it.
type cancellingHost struct {
	host
	cancel func()
}

func (h *cancellingHost) Notify(method string, params json.RawMessage) {
	h.host.Notify(method, params)
	if strings.Contains(string(params), "Last words.") && h.cancel != nil {
		h.cancel()
	}
}

// A turn cancelled while its reply was arriving, whose reply still came back
// whole, does not save the steering message queued for it or report it as
// delivered, since the model never read it.
func TestACancelDuringTheReplyLeavesQueuedSteeringUndelivered(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			io.WriteString(w, `{"version":"0.34.1"}`)
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"test","model":"test"}]}`)
		case "/api/show":
			io.WriteString(w, `{"capabilities":["completion","tools"]}`)
		case "/api/chat":
			requests++
			if requests == 1 {
				close(started)
				<-release
			}
			// The words and the end of the reply arrive together.
			io.WriteString(w, `{"message":{"role":"assistant","content":"Last words."},"done":false}`+"\n"+`{"message":{"role":"assistant","content":""},"done":true}`+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	h := &cancellingHost{}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var opened acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	h.cancel = func() { _ = c.cancelTurn(acp.CancelParams{SessionID: opened.SessionID}) }
	finished := make(chan error, 1)
	go func() {
		finished <- c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Start."}}}, nil)
	}()
	<-started
	delivered := make(chan struct{}, 1)
	if err := c.Steer(ctx, acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Use the revised scope."}}}, func() { delivered <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatalf("turn error: %v", err)
	}
	select {
	case <-delivered:
		t.Fatal("a steering message the model never read was reported as delivered")
	default:
	}
	rec, err := c.load(opened.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range rec.Messages {
		if m.Content == "Use the revised scope." {
			t.Fatal("a steering message the model never read was saved to the conversation")
		}
	}
	if requests != 1 {
		t.Fatalf("the cancelled turn asked the model again (%d requests)", requests)
	}
}

func TestSteeringContentChecksModelVision(t *testing.T) {
	srv, _ := fakeOllama(t, "read_file")
	c, err := Start(srv.URL, t.TempDir(), &host{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var opened acp.NewSessionResult
	if err := c.Call(context.Background(), "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	if !c.CanSteerContent(opened.SessionID, []string{"text"}) || c.CanSteerContent(opened.SessionID, []string{"image"}) {
		t.Fatal("text-only model capability mismatch")
	}
}

type steeringApprovalHost struct {
	host
	waiting, release chan struct{}
}

func (h *steeringApprovalHost) Request(ctx context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	if method == "session/request_permission" {
		close(h.waiting)
		select {
		case <-h.release:
		case <-ctx.Done():
			return nil, &acp.Error{Code: acp.CodeInternal, Message: ctx.Err().Error()}
		}
	}
	return h.host.Request(ctx, method, params)
}
func TestSteeringDoesNotAnswerAnApproval(t *testing.T) {
	srv, _ := fakeOllama(t, "run_command")
	h := &steeringApprovalHost{host: host{answer: "reject"}, waiting: make(chan struct{}), release: make(chan struct{})}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var opened acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Work"}}}, nil)
	}()
	select {
	case <-h.waiting:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	delivered := make(chan struct{}, 1)
	if err := c.Steer(ctx, acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Keep files"}}}, func() { delivered <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-delivered:
		t.Fatal("steering bypassed approval")
	default:
	}
	h.mu.Lock()
	answered := len(h.asked)
	h.mu.Unlock()
	if answered != 0 {
		t.Fatal("steering answered the approval")
	}
	close(h.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-delivered:
	default:
		t.Fatal("guidance was not delivered after the tool result")
	}
	rec, err := c.load(opened.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range rec.Messages {
		if m.Content == "Keep files" && (i == 0 || rec.Messages[i-1].Role != "tool") {
			t.Fatal("steering broke tool-result ordering")
		}
	}
}

func TestHistoryFailureDoesNotAcknowledgeUnconfirmedInsertion(t *testing.T) {
	var rec record
	saves, receipts := 0, 0
	pending := []steeringInput{
		{message: message{Role: "user", Content: "saved"}, delivered: func() {
			if saves != 1 {
				t.Error("receipt preceded persistence")
			}
			receipts++
		}},
		{message: message{Role: "user", Content: "uncertain"}, delivered: func() { receipts++ }},
	}
	err := persistSteeringMessages(&rec, pending, func() error {
		saves++
		if saves == 2 {
			return errors.New("transaction completion failed")
		}
		return nil
	})
	if !errors.Is(err, acp.ErrSteeringUnconfirmed) || receipts != 1 {
		t.Fatalf("err=%v receipts=%d", err, receipts)
	}
}
