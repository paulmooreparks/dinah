package codexagent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"studio/internal/acp"
)

// The app-server stand-in launches this child from the documented stdio
// configuration. It proves the environment survives both protocol translation
// and process creation, without an account-backed provider.
func TestDiagramManagedChild(t *testing.T) {
	if os.Getenv("DRAWING_CODEX_CHILD") != "1" {
		return
	}
	values := map[string]string{}
	for _, key := range []string{"DIAGRAM_PICTURE_LEASE", "DIAGRAM_PICTURE_ENDPOINT", "DIAGRAM_PICTURE_WORKSPACE", "DIAGRAM_PICTURE_ROOT"} {
		values[key] = os.Getenv(key)
	}
	json.NewEncoder(os.Stdout).Encode(values)
	os.Exit(0)
}

func TestDiagramSessionEnvironmentDelivery(t *testing.T) {
	want := map[string]string{"DIAGRAM_PICTURE_LEASE": "invented-codex-lease", "DIAGRAM_PICTURE_ENDPOINT": "http://127.0.0.1:12345/diagram-pictures", "DIAGRAM_PICTURE_WORKSPACE": "fixture-workspace", "DIAGRAM_PICTURE_ROOT": t.TempDir()}
	values := []map[string]string{{"name": "DRAWING_CODEX_CHILD", "value": "1"}}
	for key, value := range want {
		values = append(values, map[string]string{"name": key, "value": value})
	}
	custom := json.RawMessage(`{"command":"person-owned","args":["keep"],"env":{"KEEP":"yes"}}`)
	for _, conflict := range []bool{false, true} {
		c := fake(t, &testHandler{}, func(m message, send func(message)) {
			switch m.Method {
			case "config/read":
				entries := map[string]json.RawMessage{"fixture-tools": custom}
				if conflict {
					entries["fixture-drawing-random"] = custom
				}
				send(answer(m, map[string]any{"config": map[string]any{"mcp_servers": entries}}))
			case "thread/inject_items":
				send(answer(m, map[string]any{}))
			case "model/list":
				send(answer(m, catalogResult()))
			case "thread/start":
				if conflict {
					t.Error("conflict reached thread/start")
				}
				var p struct {
					Config struct {
						Servers map[string]struct {
							Command string
							Args    []string
							Env     map[string]string
						} `json:"mcp_servers"`
					}
				}
				if err := json.Unmarshal(m.Params, &p); err != nil {
					t.Error(err)
				}
				if _, exists := p.Config.Servers["fixture-tools"]; exists {
					t.Error("custom entry was overridden")
				}
				managed := p.Config.Servers["fixture-drawing-random"]
				cmd := exec.Command(managed.Command, managed.Args...)
				acp.HideWindow(cmd)
				cmd.Env = os.Environ()
				for key, value := range managed.Env {
					cmd.Env = append(cmd.Env, key+"="+value)
				}
				b, err := cmd.Output()
				if err != nil {
					t.Error(err)
				}
				var got map[string]string
				if json.Unmarshal(b, &got) != nil || !reflect.DeepEqual(got, want) {
					t.Error("managed child environment differs")
				}
				send(answer(m, map[string]any{"thread": map[string]string{"id": "fixture"}, "model": "test"}))
			}
		})
		c.RegisterTraceSecret(want["DIAGRAM_PICTURE_LEASE"])
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: want["DIAGRAM_PICTURE_ROOT"], McpServers: []any{map[string]any{"name": "fixture-drawing-random", "command": os.Args[0], "args": []string{"-test.run=^TestDiagramManagedChild$"}, "env": values}}}, nil)
		cancel()
		if (err != nil) != conflict {
			t.Fatalf("conflict=%v: %v", conflict, err)
		}
	}
}
