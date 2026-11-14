package mcpserver

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"path/filepath"
	"studio/internal/bus"
	"testing"
)

func TestPersonOnlyMutationsAreAbsentAndRefusedByBusTools(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	me, err := b.RegisterSession(ctx, "window-audit", t.TempDir(), "Agent")
	if err != nil {
		t.Fatal(err)
	}
	client := connect(t, b, me.ID)
	listed, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AnswerDecision", "answer_decision", "DismissDecision", "dismiss_decision", "ConfirmDecisionBroadcast", "broadcast", "PauseAndRestart", "pause_and_restart", "RemoveSetAsidePrograms", "remove_set_aside_programs", "CallService"} {
		for _, tool := range listed.Tools {
			if tool.Name == name {
				t.Fatalf("person-only tool offered: %s", name)
			}
		}
		result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
		if err == nil && (result == nil || !result.IsError) {
			t.Fatalf("person-only tool was callable: %s", name)
		}
	}
}
