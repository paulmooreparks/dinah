package codexagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"studio/internal/acp"
)

type smokeApprovalHandler struct {
	requests chan acp.RequestPermissionParams
	choices  chan string
}

func (*smokeApprovalHandler) Notify(string, json.RawMessage) {}
func (h *smokeApprovalHandler) Request(ctx context.Context, method string, data json.RawMessage) (any, *acp.Error) {
	if method != "session/request_permission" {
		return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: method}
	}
	var request acp.RequestPermissionParams
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, &acp.Error{Code: acp.CodeInternal, Message: err.Error()}
	}
	select {
	case h.requests <- request:
	case <-ctx.Done():
		return acp.Cancelled(), nil
	}
	select {
	case choice := <-h.choices:
		return acp.Selected(choice), nil
	case <-ctx.Done():
		return acp.Cancelled(), nil
	}
}

// The documented client tool-call API exercises real MCP transport and
// elicitation responses without starting a model turn or using account data.
func TestInstalledMCPApprovalSmoke(t *testing.T) {
	if os.Getenv("DINAH_CODEX_SMOKE") != "1" {
		t.Skip("set DINAH_CODEX_SMOKE=1 for the installed binary check")
	}
	path, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CODEX_API_KEY", "")
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "approval-fixture", Version: "1"}, nil)
	approvals := make(chan string, 4)
	mcpServer.AddTool(&mcp.Tool{Name: "applet_invoke", InputSchema: json.RawMessage(`{"type":"object"}`)}, func(ctx context.Context, r *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := r.Session.Elicit(ctx, &mcp.ElicitParams{Mode: "form", Message: "Approve this synthetic applet invocation?", RequestedSchema: map[string]any{"type": "object", "properties": map[string]any{}}})
		if err != nil {
			return nil, err
		}
		approvals <- result.Action
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: result.Action}}}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return mcpServer }, nil))
	defer httpServer.Close()
	h := &smokeApprovalHandler{requests: make(chan acp.RequestPermissionParams, 1), choices: make(chan string, 1)}
	c, err := Start(path, []string{"app-server", "-c", "cli_auth_credentials_store=\"file\""}, t.TempDir(), h, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, nil); err != nil {
		t.Fatal(err)
	}
	var session acp.NewSessionResult
	if err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: t.TempDir(), McpServers: []any{map[string]string{"name": "dinah-applets", "type": "http", "url": httpServer.URL}}}, &session); err != nil {
		t.Fatal(err)
	}
	id, err := native(session.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, choice := range []string{"accept", "decline", "cancel"} {
		var response json.RawMessage
		done := make(chan error, 1)
		go func() {
			done <- c.Conn.Call(ctx, "mcpServer/tool/call", map[string]any{"threadId": id, "server": "dinah-applets", "tool": "applet_invoke", "arguments": map[string]any{}}, &response)
		}()
		select {
		case permission := <-h.requests:
			if permission.SessionID != session.SessionID {
				t.Fatal("approval was routed to a different chat")
			}
		case err := <-done:
			t.Fatalf("tool completed before permission was requested: %v %s", err, response)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		select {
		case err := <-done:
			t.Fatalf("tool completed while permission was unanswered: %v %s", err, response)
		default:
		}
		h.choices <- choice
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if !strings.Contains(string(response), choice) || strings.Contains(string(response), `"isError":true`) {
			t.Fatalf("tool did not receive selected answer %q: %s", choice, response)
		}
		select {
		case action := <-approvals:
			if action != choice {
				t.Fatalf("MCP server received %q instead of %q", action, choice)
			}
		default:
			t.Fatal("tool did not receive its approval response")
		}
	}
}
