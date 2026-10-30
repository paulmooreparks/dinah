package ollamaagent

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The test binary is its own applet server: with DINAH_OLLAMA_CHILD set, it
// runs the mode named by its first argument instead of the tests.
func TestMain(m *testing.M) {
	if os.Getenv("DINAH_OLLAMA_CHILD") == "1" {
		runChild(os.Args[1])
		os.Exit(0)
	}
	os.Exit(m.Run())
}

const childSchema = `{"properties":{"x":{"type":"string"}},"type":"object"}`

const childBig = 17 << 20

func runChild(mode string) {
	if path := os.Getenv("DINAH_OLLAMA_STARTS"); path != "" {
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		f.WriteString("start\n")
		f.Close()
	}
	switch mode {
	case "exit":
		os.Exit(3)
	case "silent":
		// Never answer; exit when stdin closes.
		bufio.NewReader(os.Stdin).WriteTo(nopWriter{})
		return
	case "nolist":
		childNoList()
		return
	case "hold":
		// A grandchild that keeps the inherited stdout open until the test
		// removes its file.
		os.WriteFile(os.Getenv("DINAH_OLLAMA_HOLD")+".started", nil, 0o600)
		for end := time.Now().Add(30 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
			if _, err := os.Stat(os.Getenv("DINAH_OLLAMA_HOLD")); err != nil {
				return
			}
		}
		return
	case "grandchild":
		hold := exec.Command(os.Args[0], "hold")
		hold.Env = os.Environ()
		hold.Stdout = os.Stdout
		if err := hold.Start(); err != nil {
			os.Exit(4)
		}
	case "stubborn":
		signal.Ignore(syscall.SIGTERM)
	}
	childServe()
	if mode == "linger" || mode == "stubborn" {
		// A bare select would be reported as a deadlock and exit.
		time.Sleep(time.Hour)
	}
}

type nopWriter struct{}

func (nopWriter) Write(b []byte) (int, error) { return len(b), nil }

func childServe() {
	s := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "1"}, nil)
	add := func(name string, readOnly bool, h mcp.ToolHandler) {
		s.AddTool(&mcp.Tool{Name: name, Description: name + " tool", InputSchema: json.RawMessage(childSchema), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly}}, h)
	}
	text := func(t string, failed bool) *mcp.CallToolResult {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: t}}, IsError: failed}
	}
	add("read", true, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return text("read ok", false), nil
	})
	add("write", false, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return text("write ok", false), nil
	})
	add("fail", true, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return text("it failed", true), nil
	})
	add("big", true, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return text(strings.Repeat("a", childBig), false), nil
	})
	add("mixed", true, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "a"}, &mcp.ImageContent{Data: []byte{1}, MIMEType: "image/png"}, &mcp.TextContent{Text: "b\n"}}}, nil
	})
	add("slow", true, func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	add("quit", true, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		os.Exit(0)
		return nil, nil
	})
	_ = s.Run(context.Background(), &mcp.StdioTransport{MaxLineLength: -1})
}

// childNoList initializes and then fails tools/list.
func childNoList() {
	in := bufio.NewScanner(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	for in.Scan() {
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				ProtocolVersion string `json:"protocolVersion"`
			} `json:"params"`
		}
		if json.Unmarshal(in.Bytes(), &m) != nil || m.ID == nil {
			continue
		}
		switch m.Method {
		case "initialize":
			out.Encode(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": map[string]any{"protocolVersion": m.Params.ProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "nolist", "version": "1"}}})
		default:
			out.Encode(map[string]any{"jsonrpc": "2.0", "id": m.ID, "error": map[string]any{"code": -32603, "message": "no tools here"}})
		}
	}
}

func childServer(t *testing.T, mode string) server {
	t.Helper()
	return server{Name: "fx", Command: os.Args[0], Args: []string{mode}, Env: []string{"DINAH_OLLAMA_CHILD=1"}}
}

func testPhases(d time.Duration) [3]time.Duration { return [3]time.Duration{d, d, d} }

func startChild(t *testing.T, mode string, phases [3]time.Duration) *appletProcess {
	t.Helper()
	p, err := startAppletProcess(childServer(t, mode), phases)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.close() })
	return p
}

func exited(t *testing.T, p *appletProcess) {
	t.Helper()
	select {
	case <-p.exited:
	case <-time.After(10 * time.Second):
		t.Fatal("the process was not collected")
	}
}

func TestAppletToolsAndCalls(t *testing.T) {
	s := childServer(t, "serve")
	c := &Conn{applets: map[string]*appletServer{}, sessions: map[string]*session{"one": {servers: []server{s}, approvals: ApprovalsNone}}}
	t.Cleanup(func() {
		for _, a := range c.applets {
			a.close()
		}
	})
	tools, err := c.appletTools(context.Background(), []server{s})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, v := range tools {
		f := v.(map[string]any)["function"].(map[string]any)
		names[f["name"].(string)] = true
		var got, want any
		json.Unmarshal(f["parameters"].(json.RawMessage), &got)
		json.Unmarshal([]byte(childSchema), &want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s schema = %s", f["name"], f["parameters"])
		}
		if f["description"] != strings.TrimPrefix(f["name"].(string), "fx__")+" tool" {
			t.Errorf("%s description = %v", f["name"], f["description"])
		}
	}
	for _, name := range []string{"fx__read", "fx__write", "fx__fail", "fx__big", "fx__mixed", "fx__slow", "fx__quit"} {
		if !names[name] {
			t.Errorf("missing %s", name)
		}
	}
	a := c.applets[s.key()]
	if !a.readOnly["read"] || a.readOnly["write"] {
		t.Errorf("read-only annotations = %v", a.readOnly)
	}
	tt := turnTools{c: c, session: "one", commands: ApprovalsNone}
	for _, tc := range []struct {
		tool, want string
		ok         bool
	}{
		{"read", "read ok", true},
		{"write", "write ok", true},
		{"fail", "it failed", false},
		{"mixed", "a\nb", true},
	} {
		if got, ok := c.callApplet(context.Background(), tt, "id", "fx", tc.tool, map[string]any{}); got != tc.want || ok != tc.ok {
			t.Errorf("%s = %q, %v", tc.tool, got, ok)
		}
	}
	if got, ok := c.callApplet(context.Background(), tt, "id", "fx", "big", map[string]any{}); len(got) != childBig || !ok {
		t.Errorf("big result = %d bytes, %v", len(got), ok)
	}
	if got, _ := c.callApplet(context.Background(), tt, "id", "fx", "none", nil); got != "The applet server fx has no tool named none." {
		t.Errorf("unknown tool = %q", got)
	}
	if got, _ := c.callApplet(context.Background(), tt, "id", "other", "read", nil); got != "There is no applet server named other." {
		t.Errorf("unknown server = %q", got)
	}
}

func TestAppletServerStartsOnce(t *testing.T) {
	starts := filepath.Join(t.TempDir(), "starts")
	s := childServer(t, "serve")
	s.Env = append(s.Env, "DINAH_OLLAMA_STARTS="+starts)
	c := &Conn{applets: map[string]*appletServer{}}
	t.Cleanup(func() { c.applets[s.key()].close() })
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := c.running(context.Background(), s); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	data, _ := os.ReadFile(starts)
	if n := strings.Count(string(data), "start"); n != 1 {
		t.Fatalf("started %d processes", n)
	}
}

func TestAppletStartFailures(t *testing.T) {
	for _, tc := range []struct {
		mode, prefix string
		within       time.Duration
	}{
		{"exit", "the applet server fx did not start: ", time.Minute},
		{"nolist", "the applet server fx did not list its tools: ", time.Minute},
		{"silent", "the applet server fx did not start: ", 300 * time.Millisecond},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			p := startChild(t, tc.mode, closePhases)
			begun := time.Now()
			if _, err := p.connect(context.Background(), "fx", tc.within); err == nil || !strings.HasPrefix(err.Error(), tc.prefix) {
				t.Fatalf("error = %v", err)
			}
			if tc.mode == "silent" && time.Since(begun) > 5*time.Second {
				t.Fatalf("a silent server held the start for %v", time.Since(begun))
			}
			exited(t, p)
		})
	}
	c := &Conn{applets: map[string]*appletServer{}}
	s := childServer(t, "exit")
	if _, err := c.running(context.Background(), s); err == nil {
		t.Fatal("a failed server started")
	}
	if a := c.applets[s.key()]; a.process != nil {
		t.Fatal("a failed server was published")
	}
	s.Command = filepath.Join(t.TempDir(), "missing")
	if _, err := c.running(context.Background(), s); err == nil || !strings.HasPrefix(err.Error(), "the applet server fx could not start: ") {
		t.Fatalf("missing command error = %v", err)
	}
}

func TestAppletCallCancellation(t *testing.T) {
	p := startChild(t, "serve", closePhases)
	if _, err := p.connect(context.Background(), "fx", time.Minute); err != nil {
		t.Fatal(err)
	}
	one, cancelOne := context.WithCancel(context.Background())
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { _, err := p.call(one, "slow", nil); first <- err }()
	go func() { _, err := p.call(context.Background(), "slow", nil); second <- err }()
	time.Sleep(100 * time.Millisecond)
	cancelOne()
	select {
	case err := <-first:
		if err == nil {
			t.Fatal("a cancelled call succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelling a call did not end it")
	}
	if res, err := p.call(context.Background(), "read", nil); err != nil || res.IsError {
		t.Fatalf("the server stopped answering after a cancellation: %v", err)
	}
	select {
	case err := <-second:
		t.Fatalf("the other call ended: %v", err)
	default:
	}
}

// Close releases a pending call at once, before any shutdown phase, even
// when the process ignores everything short of Kill.
func TestAppletCloseReleasesPendingCalls(t *testing.T) {
	p := startChild(t, "stubborn", testPhases(time.Second))
	if _, err := p.connect(context.Background(), "fx", time.Minute); err != nil {
		t.Fatal(err)
	}
	pending := make(chan error, 1)
	go func() { _, err := p.call(context.Background(), "slow", nil); pending <- err }()
	time.Sleep(100 * time.Millisecond)
	closed := make(chan struct{})
	begun := time.Now()
	go func() { p.close(); close(closed) }()
	select {
	case err := <-pending:
		if err == nil {
			t.Fatal("a pending call succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("close did not release a pending call")
	}
	<-closed
	if d := time.Since(begun); d < 2*time.Second {
		t.Fatalf("a stubborn process closed in %v, before Kill", d)
	}
	exited(t, p)
	if _, err := p.call(context.Background(), "read", nil); err == nil {
		t.Fatal("a call succeeded after close")
	}
}

func TestAppletClosePhases(t *testing.T) {
	const phase = 2 * time.Second
	for _, tc := range []struct {
		mode     string
		min, max time.Duration
	}{
		// Exits when stdin closes: inside the first phase.
		{"serve", 0, phase},
		// Ignores stdin closing. SIGTERM ends it on Unix in the second
		// phase; Windows sends no signal, so Kill ends it in the third.
		{"linger", lingerMin(phase), lingerMin(phase) + phase},
		// Ignores SIGTERM as well: Kill ends it in the third phase.
		{"stubborn", 2 * phase, 3 * phase},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Parallel()
			p := startChild(t, tc.mode, testPhases(phase))
			if _, err := p.connect(context.Background(), "fx", time.Minute); err != nil {
				t.Fatal(err)
			}
			begun := time.Now()
			p.close()
			if d := time.Since(begun); d < tc.min || d > tc.max {
				t.Fatalf("closed in %v, want %v to %v", d, tc.min, tc.max)
			}
			exited(t, p)
		})
	}
}

func lingerMin(phase time.Duration) time.Duration {
	if os.PathSeparator == '\\' {
		return 2 * phase
	}
	return phase
}

// Close called many times at once, racing the child's own exit, does its
// work once and gives every caller the same result.
func TestAppletCloseIsIdempotent(t *testing.T) {
	p := startChild(t, "serve", closePhases)
	if _, err := p.connect(context.Background(), "fx", time.Minute); err != nil {
		t.Fatal(err)
	}
	quit := make(chan struct{})
	go func() { p.call(context.Background(), "quit", nil); close(quit) }()
	results := make(chan error, 8)
	for range 8 {
		go func() { results <- p.close() }()
	}
	first := <-results
	for range 7 {
		if err := <-results; err != first {
			t.Fatalf("close results differ: %v and %v", first, err)
		}
	}
	<-quit
	exited(t, p)
}

// A grandchild that inherits the child's stdout keeps the pipe open after
// the child exits. Close waits for nothing but the process, so it returns
// within the three phases whatever the session does with its files.
func TestAppletCloseWithGrandchild(t *testing.T) {
	hold := filepath.Join(t.TempDir(), "hold")
	os.WriteFile(hold, nil, 0o600)
	t.Cleanup(func() { os.Remove(hold) })
	s := childServer(t, "grandchild")
	s.Env = append(s.Env, "DINAH_OLLAMA_HOLD="+hold)
	p, err := startAppletProcess(s, testPhases(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.connect(context.Background(), "fx", time.Minute); err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(hold + ".started"); err == nil {
			break
		}
		if time.Now().After(end) {
			t.Fatal("the grandchild did not start")
		}
	}
	// Allow scheduling slack on top of the three one-second phases.
	limit := 3*time.Second + 500*time.Millisecond
	done := make(chan struct{})
	go func() { p.close(); close(done) }()
	select {
	case <-done:
	case <-time.After(limit):
		t.Fatalf("close took longer than its phases (%v)", limit)
	}
	exited(t, p)
}
