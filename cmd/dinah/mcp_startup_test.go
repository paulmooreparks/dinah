package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// TestTheStartupRefusalsMCPRaisesLeadWithTheirName asserts AC-25 across all
// three refusals dinah mcp can raise before it serves, in one test, because
// the criterion is about the three agreeing rather than about any one of them.
//
// A client reads the refusal name off this stream by splitting the line on
// whitespace and taking the first field, so a name carrying a trailing colon
// matches nothing the client is looking for. Two of the three lines are built
// by hand and are space-separated; the third was rendered through
// Refusal.Error, which formats as "%s: %s". A test holding one line at a time
// cannot see a disagreement between three, so this one holds all three.
func TestTheStartupRefusalsMCPRaisesLeadWithTheirName(t *testing.T) {
	container := newBench(t)
	workbench := soleBenchDir(t, container)

	elsewhere := t.TempDir()
	empty := filepath.Join(elsewhere, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	missing := filepath.Join(elsewhere, "missing")

	cases := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "a root no directory sits at",
			argv: []string{"mcp", "--root", missing},
			want: contract.UnknownRoot,
		},
		{
			name: "a named workbench holding no anchor",
			argv: []string{"--workbench", empty, "mcp", "--root", empty},
			want: contract.NoWorkbench,
		},
		{
			name: "a named workbench outside the root",
			argv: []string{"--workbench", workbench, "mcp", "--root", empty},
			want: contract.OutsideRoot,
		},
	}
	for _, held := range cases {
		t.Run(held.name, func(t *testing.T) {
			got := runCLI(t, empty, held.argv...)
			if got.code != contract.ExitCode(contract.OutcomeRefused) {
				t.Fatalf("wanted the refused exit code %d, got %d: %s", contract.ExitCode(contract.OutcomeRefused), got.code, got.errw)
			}
			fields := strings.Fields(got.errw)
			if len(fields) == 0 {
				t.Fatalf("the error stream carried nothing")
			}
			if fields[0] != held.want {
				t.Errorf("the first whitespace-delimited token on stderr: wanted %q, got %q from %q", held.want, fields[0], strings.TrimSpace(got.errw))
			}
			if len(fields) < 2 {
				t.Errorf("the line names no detail after the refusal name: %q", strings.TrimSpace(got.errw))
			}
		})
	}
}

// TestMCPServesWithNoRootAndNothingDiscoverable is dinah-307 AC-1. An agent
// that registers one server and names the workbench on each call has nowhere
// to start it from that is guaranteed to sit under a workbench, so a server
// that refuses to exist unless something was discoverable is a server that
// agent cannot use at all.
//
// The stream is closed immediately, which is how the process is asked to serve
// and then reach the end of its input at once. Serving is proved by the exit
// code: before dinah-307 this invocation wrote dinah.no-workbench-found to
// stderr and exited 2 without answering anything.
func TestMCPServesWithNoRootAndNothingDiscoverable(t *testing.T) {
	base := t.TempDir()
	elsewhere := filepath.Join(base, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// The user base is disposable and holds no workbench key, so the ancestor
	// walk and the configured pointer both come back with nothing, which is
	// the whole point of the case.
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("DINAH_MCP_ROOT", "")

	got := runCLI(t, elsewhere, "mcp")
	if got.code != 0 {
		t.Fatalf("mcp with no root and nothing discoverable: wanted exit 0, got %d: %s", got.code, got.errw)
	}
	if strings.Contains(got.errw, contract.NoWorkbenchFound) {
		t.Errorf("the server refused to start rather than serving: %q", strings.TrimSpace(got.errw))
	}
}

// TestMCPKeepsDiscoveredDefaultButStopsNarrowingWithNoRoot is dinah-307 AC-8.
// Starting inside a workbench still says which workbench answers a call that
// names none. It no longer says which workbenches may be named, because a
// default is an answer to an unqualified call rather than a boundary.
//
// Both halves are held here together because they are separable and only one
// of them was ruled away: before dinah-307 the discovered workbench became the
// server's root, so the second workbench below was refused dinah.outside-root
// with the first one's directory named as the root.
func TestMCPKeepsDiscoveredDefaultButStopsNarrowingWithNoRoot(t *testing.T) {
	container := newBench(t)
	discovered := soleBenchDir(t, container)

	// A second workbench, unrelated to the first and outside every directory
	// above it that the first one's discovery walks.
	elsewhere := t.TempDir()
	if got := runCLI(t, elsewhere, "init", "--slug", "sx", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init the second workbench: %d %s", got.code, got.errw)
	}
	second := soleBenchDir(t, elsewhere)

	named := mcpToolPayload(t, runCLIWithInput(t, discovered,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbench","arguments":{"actor":"alka","workbench":"`+filepath.ToSlash(second)+`"}}}`+"\n"),
		"mcp"))
	if named["refusal"] == contract.OutsideRoot {
		t.Fatalf("a server that merely discovered a workbench refused a second one as outside its root: %v", named)
	}
	if fields, ok := named["workbench"].(map[string]any); !ok || fields["slug"] != "sx" {
		t.Errorf("naming the second workbench by absolute path answered %v, wanted the workbench slugged sx", named)
	}

	unnamed := mcpToolPayload(t, runCLIWithInput(t, discovered,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbench","arguments":{"actor":"alka"}}}`+"\n"),
		"mcp"))
	if fields, ok := unnamed["workbench"].(map[string]any); !ok || fields["slug"] != "fx" {
		t.Errorf("a call naming no workbench answered %v, wanted the discovered workbench slugged fx", unnamed)
	}
}

// mcpToolPayload decodes the single tool answer an mcp invocation wrote to
// stdout: the JSON-RPC envelope, then the canonical payload the surface
// carries inside its first content block.
func mcpToolPayload(t *testing.T, got invocation) map[string]any {
	t.Helper()
	if got.code != 0 {
		t.Fatalf("mcp exited %d: %s", got.code, got.errw)
	}
	var envelope struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	line := strings.TrimSpace(got.out)
	if line == "" {
		t.Fatalf("the server answered nothing: stderr %q", strings.TrimSpace(got.errw))
	}
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		t.Fatalf("decode %q: %v", line, err)
	}
	if envelope.Error != nil {
		t.Fatalf("the call failed at the transport: %s", envelope.Error.Message)
	}
	if len(envelope.Result.Content) == 0 {
		t.Fatalf("the answer carried no content: %q", line)
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(envelope.Result.Content[0].Text), &payload); err != nil {
		t.Fatalf("decode the payload %q: %v", envelope.Result.Content[0].Text, err)
	}
	return payload
}
