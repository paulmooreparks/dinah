package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The peer uses newline JSON over pipes so assertions cover the public wire.
type appletWire struct {
	t         *testing.T
	input     *io.PipeWriter
	output    *io.PipeReader
	writeMu   sync.Mutex
	responses chan map[string]json.RawMessage
}

func newAppletWire(t *testing.T, resolve func(string) (*appletBridgeClient, error)) *appletWire {
	return newAppletWireServer(t, appletMCPServer(resolve))
}

func newAppletWireServer(t *testing.T, server *mcp.Server) *appletWire {
	return newAppletWireRunner(t, func(ctx context.Context, in io.ReadCloser, out io.WriteCloser) error {
		return server.Run(ctx, &mcp.IOTransport{Reader: in, Writer: out, MaxLineLength: -1})
	})
}

func newAppletWireRunner(t *testing.T, run func(context.Context, io.ReadCloser, io.WriteCloser) error) *appletWire {
	t.Helper()
	in, writer := io.Pipe()
	reader, out := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer out.Close()
		_ = run(ctx, in, out)
	}()
	p := &appletWire{t: t, input: writer, output: reader, responses: make(chan map[string]json.RawMessage, 100)}
	go func() {
		defer close(p.responses)
		d := json.NewDecoder(reader)
		for {
			var msg map[string]json.RawMessage
			if d.Decode(&msg) != nil {
				return
			}
			p.responses <- msg
		}
	}()
	t.Cleanup(func() {
		cancel()
		writer.Close()
		reader.Close()
		in.Close()
		out.Close()
		select {
		case <-done:
		case <-time.After(testWait):
			t.Error("helper did not stop")
		}
	})
	return p
}

func (p *appletWire) send(id any, method string, params any) {
	p.t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		msg["id"] = id
	}
	if params != nil {
		msg["params"] = params
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if err := json.NewEncoder(p.input).Encode(msg); err != nil {
		p.t.Fatal(err)
	}
}
func (p *appletWire) read() map[string]json.RawMessage {
	p.t.Helper()
	select {
	case msg, ok := <-p.responses:
		if !ok {
			p.t.Fatal("helper closed without response")
		}
		return msg
	case <-time.After(testWait):
		p.t.Fatal("helper response timed out")
		return nil
	}
}
func (p *appletWire) initialize() map[string]json.RawMessage {
	p.send("init", "initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "wire-test", "version": "1"}})
	result := p.read()
	p.send(nil, "notifications/initialized", map[string]any{})
	return result
}

func TestAppletMCPWireContract(t *testing.T) {
	p := newAppletWire(t, func(string) (*appletBridgeClient, error) { return nil, errors.New("fixture authorization refused") })
	init := p.initialize()
	p.send("list", "tools/list", map[string]any{})
	list := p.read()
	p.send("call", "tools/call", map[string]any{"name": "applet_discover", "arguments": map[string]any{}})
	call := p.read()
	var listResult map[string]json.RawMessage
	if err := json.Unmarshal(list["result"], &listResult); err != nil {
		t.Fatal(err)
	}
	if string(listResult["ttlMs"]) != "0" || string(listResult["cacheScope"]) != `"public"` {
		t.Fatalf("unexpected catalog cache fields: %s", list["result"])
	}
	delete(listResult, "ttlMs")
	delete(listResult, "cacheScope")
	list["result"], _ = json.Marshal(listResult)
	snapshot := map[string]any{"initialize": init, "list": list, "unauthorized": call}
	// Version is a build input, not an SDK migration difference.
	var initResult map[string]any
	if err := json.Unmarshal(init["result"], &initResult); err != nil {
		t.Fatal(err)
	}
	initResult["serverInfo"].(map[string]any)["version"] = "BUILD_VERSION"
	init["result"], _ = json.Marshal(initResult)
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "applet-mcp-wire.json")

	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(expected, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("MCP wire contract changed:\n%s", data)
	}
}

func (p *appletWire) handle(raw json.RawMessage) map[string]json.RawMessage {
	p.t.Helper()
	p.writeMu.Lock()
	_, err := p.input.Write(append(raw, '\n'))
	p.writeMu.Unlock()
	if err != nil {
		p.t.Fatal(err)
	}
	return p.read()
}
