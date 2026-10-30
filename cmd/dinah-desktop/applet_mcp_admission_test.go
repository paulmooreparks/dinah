package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func waitAppletAdmission(t *testing.T, a *appletMCPAdmission, active, waiting int) {
	t.Helper()
	deadline := time.NewTimer(testWait)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		a.mu.Lock()
		gotActive, gotWaiting := a.active, len(a.queue)
		a.mu.Unlock()
		if gotActive == active && gotWaiting == waiting {
			return
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("admission state = %d active, %d waiting; want %d, %d", gotActive, gotWaiting, active, waiting)
		}
	}
}

func TestAppletMCPAdmissionThroughWire(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var active, maxActive atomic.Int32
	var mu sync.Mutex
	seen := map[string]int{}
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q appletBridgeRequest
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		mu.Lock()
		seen[q.ID]++
		mu.Unlock()
		n := active.Add(1)
		defer active.Add(-1)
		for old := maxActive.Load(); n > old; old = maxActive.Load() {
			if maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		io.WriteString(w, `{"status":"done"}`)
	}))
	defer bridge.Close()
	defer unblock()
	client, err := newAppletBridgeClient(bridge.URL+"/rpc", "test")
	if err != nil {
		t.Fatal(err)
	}
	a := &appletMCPAdmission{}
	p := newAppletWireServer(t, appletMCPServerUsing(func(string) (*appletBridgeClient, error) { return client, nil }, a))
	p.initialize()
	send := func(id string) {
		p.send(id, "tools/call", map[string]any{"name": "applet_status", "arguments": map[string]any{"id": id}})
	}
	for i := 0; i < 4; i++ {
		send(fmt.Sprintf("active-%d", i))
	}
	waitAppletAdmission(t, a, 4, 0)
	for i := 0; i < 32; i++ {
		send(fmt.Sprintf("queued-%d", i))
	}
	waitAppletAdmission(t, a, 4, 32)
	send("excess")
	msg := p.read()
	if string(msg["id"]) != `"excess"` {
		t.Fatalf("wrong overload ID: %s", msg["id"])
	}
	var rpcError struct {
		Code    int
		Message string
	}
	if err := json.Unmarshal(msg["error"], &rpcError); err != nil {
		t.Fatal(err)
	}
	if rpcError.Code != -32000 || rpcError.Message != "applet helper overloaded; try again later" {
		t.Fatalf("wrong overload: %+v", rpcError)
	}
	// Control requests remain serviceable while every tool slot is occupied.
	p.send("ping", "ping", map[string]any{})
	if msg := p.read(); string(msg["id"]) != `"ping"` || msg["error"] != nil {
		t.Fatalf("ping blocked/failed: %s", msg)
	}
	p.send("list", "tools/list", map[string]any{})
	if msg := p.read(); string(msg["id"]) != `"list"` || msg["error"] != nil {
		t.Fatalf("list blocked/failed: %s", msg)
	}
	p.send(nil, "notifications/cancelled", map[string]any{"requestId": "queued-0", "reason": "test cancellation"})
	waitAppletAdmission(t, a, 4, 31)
	send("replacement")
	waitAppletAdmission(t, a, 4, 32)
	unblock()
	// Cancellation may itself produce a response. Every live call must finish once.
	completed := map[string]bool{}
	for len(completed) < 36 {
		msg := p.read()
		var id string
		if err := json.Unmarshal(msg["id"], &id); err != nil {
			t.Fatal(err)
		}
		if id == "queued-0" {
			continue
		}
		if msg["error"] != nil || msg["result"] == nil {
			t.Fatalf("call %s failed: %s", id, msg)
		}
		if completed[id] {
			t.Fatalf("duplicate response: %s", id)
		}
		completed[id] = true
	}
	waitAppletAdmission(t, a, 0, 0)
	mu.Lock()
	defer mu.Unlock()
	if seen["queued-0"] != 0 || seen["excess"] != 0 {
		t.Fatalf("cancelled/overloaded request reached bridge: %v", seen)
	}
	if len(seen) != 36 {
		t.Fatalf("bridge saw %d calls", len(seen))
	}
	for id, count := range seen {
		if count != 1 {
			t.Fatalf("%s executed %d times", id, count)
		}
	}
	if maxActive.Load() > 4 {
		t.Fatalf("bridge concurrency reached %d", maxActive.Load())
	}
}

func TestAppletMCPLargeResponseThroughWire(t *testing.T) {
	payload := `{"data":"` + strings.Repeat("a", 17<<20) + `"}`
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, payload) }))
	defer bridge.Close()
	client, err := newAppletBridgeClient(bridge.URL+"/rpc", "test")
	if err != nil {
		t.Fatal(err)
	}
	p := newAppletWire(t, func(string) (*appletBridgeClient, error) { return client, nil })
	p.initialize()
	p.send("large", "tools/call", map[string]any{"name": "applet_artifact", "arguments": map[string]any{"id": "artifact"}})
	msg := p.read()
	var result struct {
		Content []struct{ Text string }
		IsError bool
	}
	if err := json.Unmarshal(msg["result"], &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) != 1 || result.Content[0].Text != payload {
		t.Fatal("large bridge response did not survive MCP framing")
	}
}

func TestAppletMCPNormalSDKNegotiationAndCancellation(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Consume the body before waiting so the server can observe peer closure.
		io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer bridge.Close()
	client, err := newAppletBridgeClient(bridge.URL+"/rpc", "test")
	if err != nil {
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server, err := appletMCPServer(func(string) (*appletBridgeClient, error) { return client, nil }).Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "normal-client", Version: "1"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil || len(listed.Tools) != 5 {
		t.Fatalf("normal negotiation/list failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "applet_status", Arguments: map[string]any{"id": "one"}})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(testWait):
		t.Fatal("bridge did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled call succeeded")
		}
	case <-time.After(testWait):
		t.Fatal("call did not cancel")
	}
	select {
	case <-cancelled:
	case <-time.After(testWait):
		t.Fatal("bridge request did not cancel")
	}
	if _, err := session.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("cancellation ended session: %v", err)
	}
}
