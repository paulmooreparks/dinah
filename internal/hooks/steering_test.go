package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/bus"
)

func TestDevinCommandHooksDeliverSteeringOnce(t *testing.T) {
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ctx := context.Background()
	chat, err := b.RegisterSession(ctx, "window-devin", t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	run := func(event, prompt string) string {
		t.Helper()
		input, _ := json.Marshal(map[string]string{"session_id": "agent", "prompt_id": "prompt-1", "hook_event_name": event, "prompt": prompt})
		var out bytes.Buffer
		if err := Steering(ctx, b, bytes.NewReader(input), &out); err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out.String())
	}
	run("UserPromptSubmit", "Work.\n"+bus.SteerMarker(chat.ID))
	if id, _ := b.SessionOfAgent(ctx, "devin:agent"); id != chat.ID {
		t.Fatal("Devin hook did not bind the chat")
	}
	if id, _ := b.SessionOfAgent(ctx, "agent"); id != "" {
		t.Fatal("Devin collided with a Claude hook session")
	}
	b.QueueSteer(ctx, chat.ID, "Keep the existing API.")
	out := run("PostToolUse", "")
	if !strings.Contains(out, "additionalContext") || !strings.Contains(out, "Keep the existing API.") {
		t.Fatalf("PostToolUse: %s", out)
	}
	if out := run("PostToolUse", ""); out != "{}" {
		t.Fatalf("delivered twice: %s", out)
	}
	b.QueueSteer(ctx, chat.ID, "Also update the design.")
	var stop map[string]string
	if err := json.Unmarshal([]byte(run("Stop", "")), &stop); err != nil || stop["decision"] != "block" || !strings.Contains(stop["reason"], "Also update the design.") {
		t.Fatalf("Stop: %+v %v", stop, err)
	}
}
