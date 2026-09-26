package mcp

import "testing"

// TestTheTerminalHeadHasNoTool is the MCP half of dinah-603/criteria/1: the
// tui command is held out of the tool list on the ground that it starts a
// head, and no tool is served for it, whose command is tui, under any name.
func TestTheTerminalHeadHasNoTool(t *testing.T) {
	held, ok := toolExemptions["tui"]
	if !ok || held.ground != GroundTheHeadItself {
		t.Fatalf("toolExemptions holds tui as %+v, wanted the ground %s", held, GroundTheHeadItself)
	}
	checked := 0
	for _, served := range tools {
		checked++
		if served.command == "tui" || served.name == "tui" {
			t.Errorf("the tool %s serves the tui command", served.name)
		}
	}
	if checked == 0 {
		t.Fatal("the tool list is empty, so this proves nothing")
	}
}
