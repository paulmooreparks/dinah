package codexagent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"studio/internal/acp"
)

// A bare thread is started with instructions of its own, in memory only, and
// with the documented keys that leave out Codex's tools and texts, each
// installed plugin among them by its own name. An ordinary thread has none
// of that.
func TestABareThreadLeavesOutWhatCodexGivesEveryThread(t *testing.T) {
	var started []map[string]any
	c := fake(t, &testHandler{}, func(m message, send func(message)) {
		switch m.Method {
		case "initialize":
			send(answer(m, map[string]any{}))
		case "config/read":
			send(answer(m, map[string]any{"config": map[string]any{"plugins": map[string]any{"documents@bundled": map[string]any{"enabled": true}}}}))
		case "thread/inject_items", "thread/unsubscribe":
			send(answer(m, map[string]any{}))
		case "model/list":
			send(answer(m, catalogResult()))
		case "thread/start":
			var p map[string]any
			json.Unmarshal(m.Params, &p)
			started = append(started, p)
			send(answer(m, map[string]any{"thread": map[string]string{"id": "thread"}, "model": "test", "reasoningEffort": "medium"}))
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, nil); err != nil {
		t.Fatal(err)
	}
	var s acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: "C:/test", McpServers: []any{}}, Bare: true}, &s); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "session/new", acp.NewSessionParams{Cwd: "C:/test", McpServers: []any{}}, &s); err != nil {
		t.Fatal(err)
	}
	if len(started) != 2 {
		t.Fatalf("%d threads were started", len(started))
	}
	bare, plain := started[0], started[1]
	config := bare["config"].(map[string]any)
	features, _ := config["features"].(map[string]any)
	plugins, _ := config["plugins"].(map[string]any)
	skills, _ := config["skills"].(map[string]any)
	off := func(m map[string]any, key string) bool {
		inner, _ := m[key].(map[string]any)
		return inner != nil && inner["enabled"] == false
	}
	if bare["ephemeral"] != true || bare["baseInstructions"] != bareInstructions || config["web_search"] != "disabled" || config["project_doc_max_bytes"] != float64(0) ||
		skills["include_instructions"] != false || !off(skills, "bundled") ||
		features["shell_tool"] != false || features["view_image"] != false || features["apps"] != false || features["multi_agent"] != false || features["memories"] != false ||
		!off(plugins, "_default") || !off(plugins, "documents@bundled") {
		t.Fatalf("the bare thread was started with %#v", bare)
	}
	plainConfig := plain["config"].(map[string]any)
	if plain["ephemeral"] != false || plain["baseInstructions"] != nil || plainConfig["features"] != nil || plainConfig["plugins"] != nil || plainConfig["web_search"] != nil {
		t.Fatalf("an ordinary thread was started with %#v", plain)
	}
}
