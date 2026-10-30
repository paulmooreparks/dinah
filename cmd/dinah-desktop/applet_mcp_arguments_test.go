package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func appletArgumentCases() map[string]any {
	return map[string]any{
		"missing":              nil,
		"empty":                map[string]any{},
		"array":                []string{"unexpected"},
		"string":               "unexpected",
		"timeout-string":       map[string]any{"timeoutSeconds": "30"},
		"timeout-float-string": map[string]any{"timeoutSeconds": "30.0"},
		"timeout-wrong-type":   map[string]any{"timeoutSeconds": true},
		"timeout-low":          map[string]any{"timeoutSeconds": 0},
		"timeout-high":         map[string]any{"timeoutSeconds": 86401},
		"timeout-fraction":     map[string]any{"timeoutSeconds": 1.5},
		"timeout-last":         map[string]any{"timeoutSeconds": 86400},
		"exact":                map[string]any{"inputJSON": `{"large":9007199254740993}`, "timeoutSeconds": 30, "requestId": "same", "action": "fixture/read@1", "digest": "digest"},
	}
}

func TestAppletMCPArgumentCompatibility(t *testing.T) {
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q appletBridgeRequest
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		json.NewEncoder(w).Encode(q)
	}))
	defer bridge.Close()
	client, err := newAppletBridgeClient(bridge.URL+"/rpc", "test")
	if err != nil {
		t.Fatal(err)
	}
	p := newAppletWire(t, func(string) (*appletBridgeClient, error) { return client, nil })
	p.initialize()
	data, err := os.ReadFile("testdata/applet-mcp-arguments.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]any
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	check := func(name string) {
		msg := p.read()
		data, _ := json.Marshal(msg)
		var got any
		json.Unmarshal(data, &got)
		if !reflect.DeepEqual(got, expected[name]) {
			t.Errorf("%s changed: %s; want %v", name, data, expected[name])
		}
	}
	for name, args := range appletArgumentCases() {
		p.send(name, "tools/call", map[string]any{"name": "applet_invoke", "arguments": args})
		check(name)
	}
	// The SDK writes the unknown-tool error itself, so its wording may differ
	// from the old text kept in the fixture; the code and ID must not.
	p.send("unknown", "tools/call", map[string]any{"name": "not-a-tool", "arguments": map[string]any{}})
	msg := p.read()
	var unknown struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(msg["error"], &unknown); err != nil || unknown.Code != -32602 || string(msg["id"]) != `"unknown"` {
		t.Errorf("unknown tool changed: %s", msg)
	}
}

func TestAppletMCPAuthorizationPrecedesTimeout(t *testing.T) {
	p := newAppletWire(t, func(string) (*appletBridgeClient, error) { return nil, io.EOF })
	p.initialize()
	p.send(1, "tools/call", map[string]any{"name": "applet_invoke", "arguments": map[string]any{"timeoutSeconds": 0}})
	msg := p.read()
	var r struct {
		Content []struct{ Text string }
		IsError bool
	}
	if err := json.Unmarshal(msg["result"], &r); err != nil {
		t.Fatal(err)
	}
	if !r.IsError || len(r.Content) != 1 || r.Content[0].Text != "EOF" {
		t.Fatalf("authorization precedence changed: %s", msg)
	}
}

// A call that lacks a required field, or carries one the tool does not have, is refused
// with the field's name, and is not forwarded as a call with an empty value: the case is
// applet_status given invocationId in place of id.
func TestAppletToolsRefuseMissingAndUnknownFields(t *testing.T) {
	forwarded := 0
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded++
		var q appletBridgeRequest
		_ = json.NewDecoder(r.Body).Decode(&q)
		json.NewEncoder(w).Encode(q)
	}))
	defer bridge.Close()
	client, err := newAppletBridgeClient(bridge.URL+"/rpc", "test")
	if err != nil {
		t.Fatal(err)
	}
	p := newAppletWire(t, func(string) (*appletBridgeClient, error) { return client, nil })
	p.initialize()
	for name, c := range map[string]struct {
		tool string
		args map[string]any
		want []string
	}{
		"unknown":   {"applet_status", map[string]any{"invocationId": "abc"}, []string{"id is required", "invocationId is not a parameter of applet_status", "the parameters of applet_status are context, id"}},
		"wrongtype": {"applet_status", map[string]any{"id": 7}, []string{"id must be a string"}},
		"extra":     {"applet_discover", map[string]any{"name": "x"}, []string{"name is not a parameter of applet_discover"}},
		"cancel":    {"applet_cancel", map[string]any{}, []string{"id is required"}},
	} {
		p.send(name, "tools/call", map[string]any{"name": c.tool, "arguments": c.args})
		msg := p.read()
		data, _ := json.Marshal(msg)
		for _, want := range c.want {
			if !strings.Contains(string(data), want) {
				t.Errorf("%s: the refusal does not say %q: %s", name, want, data)
			}
		}
		if !strings.Contains(string(data), `"isError":true`) {
			t.Errorf("%s: not an error: %s", name, data)
		}
	}
	if forwarded != 0 {
		t.Fatalf("%d refused calls reached the bridge", forwarded)
	}
	// A correct call still goes through.
	p.send("good", "tools/call", map[string]any{"name": "applet_status", "arguments": map[string]any{"id": "abc123"}})
	if msg := p.read(); !strings.Contains(string(msg["result"]), "abc123") || forwarded != 1 {
		t.Fatalf("a correct call: %s (%d forwarded)", msg["result"], forwarded)
	}
}
