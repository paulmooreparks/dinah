package main

import (
	"encoding/json"
	"testing"
	"time"
)

// Exercise the MCP tool, HTTP bridge and durable host together for the two
// actions reported as stalled. Provider approvals are tested in codexagent.
func TestAppletMCPInvokesSettingsAndEAN13(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := withTestApplets(t, NewApp())
	_, client := testAppletBridge(t, a, t.TempDir(), "codex:test-thread")
	s := newAppletWire(t, func(string) (*appletBridgeClient, error) { return client, nil })
	s.initialize()
	call := func(name string, arguments map[string]any, out any) {
		t.Helper()
		request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": arguments}})
		response, _ := json.Marshal(s.handle(request))
		var wire struct {
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal(response, &wire); err != nil || wire.Result.IsError || len(wire.Result.Content) != 1 {
			t.Fatalf("%s failed: %s", name, response)
		}
		if err := json.Unmarshal([]byte(wire.Result.Content[0].Text), out); err != nil {
			t.Fatalf("%s response: %v", name, err)
		}
	}
	var catalog struct {
		Actions []struct {
			Action struct {
				Ref string `json:"ref"`
			} `json:"action"`
			Digest string `json:"digest"`
		} `json:"actions"`
	}
	call("applet_discover", map[string]any{}, &catalog)
	digests := map[string]string{}
	for _, action := range catalog.Actions {
		digests[action.Action.Ref] = action.Digest
	}
	for _, test := range []struct{ ref, input string }{
		{"dinah.settings/read@1", `{}`},
		{testPictureRef, `{"text":"Hello"}`},
	} {
		t.Run(test.ref, func(t *testing.T) {
			if digests[test.ref] == "" {
				t.Fatal("action not discovered")
			}
			var invocation AppletInvocation
			call("applet_invoke", map[string]any{"requestId": test.ref, "action": test.ref, "inputJSON": test.input, "digest": digests[test.ref], "timeoutSeconds": 30}, &invocation)
			if invocation.ID == "" {
				t.Fatal("missing invocation ID")
			}
			deadline := time.Now().Add(testWait)
			for invocation.Status == "running" && time.Now().Before(deadline) {
				call("applet_status", map[string]any{"id": invocation.ID}, &invocation)
				time.Sleep(10 * time.Millisecond)
			}
			if invocation.Status != "done" {
				t.Fatalf("action did not complete: %+v", invocation)
			}
			if test.ref == testPictureRef {
				if len(invocation.Artifacts) != 1 {
					t.Fatal("missing picture SVG")
				}
				var artifact AppletArtifact
				call("applet_artifact", map[string]any{"id": invocation.Artifacts[0].ID}, &artifact)
				if artifact.DataBase64 == "" {
					t.Fatal("missing picture bytes")
				}
			} else if invocation.OutputJSON == "" {
				t.Fatal("missing settings preferences")
			}
		})
	}
}
