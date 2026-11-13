package mcpserver

import (
	"context"
	"path/filepath"
	"strings"
	"studio/internal/bus"
	"testing"
)

func TestAskPersonToolDurableReceiptAndGuidance(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	me, err := b.RegisterSession(ctx, "window-asking", t.TempDir(), "Asker")
	if err != nil {
		t.Fatal(err)
	}
	client := connect(t, b, me.ID)
	tools, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name == "ask_person" {
			found = true
			for _, words := range []string{"only the person can make", "recommendation", "Never use this for progress reports"} {
				if !strings.Contains(tool.Description, words) {
					t.Fatalf("missing usage guidance %q", words)
				}
			}
		}
	}
	if !found {
		t.Fatal("ask_person not listed")
	}
	input := map[string]any{"operationId": "agent-choice-1", "questions": []map[string]any{{"number": 1, "text": "May I change the policy?", "recommendation": "Keep the current policy.", "choices": []string{"Keep", "Change"}}}}
	first, bad := call(t, client, "ask_person", input)
	if bad || !strings.Contains(first, `"id":"request:`) {
		t.Fatalf("first receipt %s", first)
	}
	second, bad := call(t, client, "ask_person", input)
	if bad || second != first {
		t.Fatalf("retry %s", second)
	}
	page, err := b.Decisions(ctx, 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].AskerSession != me.ID {
		t.Fatalf("durable request %+v %v", page, err)
	}
	input["operationId"] = "invalid"
	input["questions"] = []map[string]any{{"number": 1, "text": "Missing recommendation"}}
	_, bad = call(t, client, "ask_person", input)
	if !bad {
		t.Fatal("tool accepted missing recommendation")
	}
}
