package mcpserver

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"studio/internal/bus"
)

// A chat in a window may call the wrong server for its bus tools: the
// command line program's, which the person's own agent settings start. That
// server answers by saying which server has the tool, whether it stood down
// or, where the agent hid the window's mark from it, did not (Paul, 10
// November 2026).
func TestTheWrongServerSaysWhichServerHasTheTool(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	client := func(srv *mcp.Server) *mcp.ClientSession {
		t.Helper()
		st, ct := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(ctx, st, nil); err != nil {
			t.Fatal(err)
		}
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cs.Close() })
		return cs
	}
	where := "tool of the " + bus.WindowServer + " server"

	// Stood down: every bus tool is listed, says where it is, and does nothing.
	idle := client(NewIdle(b).idleServer("Paul"))
	listed, err := idle.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, tool := range listed.Tools {
		names[tool.Name] = tool.Description
	}
	for _, name := range redirectedTools {
		if !strings.Contains(names[name], "the "+name+" "+where) {
			t.Errorf("the stood-down %s is described as %q", name, names[name])
		}
	}
	if _, ok := names["hook_deliver"]; !ok || len(names) != len(redirectedTools)+1 {
		t.Errorf("the stood-down server lists %d tools: %v", len(names), names)
	}
	out, failed := call(t, idle, "reply", map[string]any{"message_id": 7, "text": "Hello", "actionRecipients": []string{"paul"}})
	if !failed || !strings.Contains(out, "Nothing was done") || !strings.Contains(out, "the reply "+where) {
		t.Errorf("the stood-down reply answered %v %q", failed, out)
	}
	if n, _ := b.ChangeCounter(ctx); n != 0 {
		t.Errorf("a stood-down call changed the bus: %d", n)
	}

	// Not stood down and never identified: the same tool refuses, and says
	// the same thing first.
	srv := mcp.NewServer(&mcp.Implementation{Name: "dinah", Version: Version}, nil)
	New(b).register(srv, "Paul")
	out, failed = call(t, client(srv), "reply", map[string]any{"message_id": 7, "text": "Hello"})
	at, hooks := strings.Index(out, "bus tools are on the "+bus.WindowServer+" server"), strings.Index(out, "hook")
	if !failed || at < 0 || hooks < at {
		t.Errorf("the unidentified reply answered %v %q", failed, out)
	}
}

// What a chat in a window is sent with a message says which server its
// reply tool is on, and a session outside a window is told the tool alone.
func TestADeliveredMessageSaysWhichServerAnswers(t *testing.T) {
	msgs := []bus.Message{
		{ID: 8, FromName: "paul", ToLabel: "#Work", Room: "work", RoomTitle: "Work", Text: "Are you there?", Kind: "text", Asked: true},
		{ID: 9, FromName: "Other", ToLabel: "#Work", Room: "work", RoomTitle: "Work", Text: "For somebody else.", Kind: "text", Notice: true},
	}
	window := bus.FormatForWindowChat(msgs, "Paul")
	for _, want := range []string{
		"use the reply tool of the " + bus.WindowServer + " server (message_id).",
		"a reply tool of any other server does not reach it",
		"Read one whole with the read_message tool of the " + bus.WindowServer + " server (message_id)",
	} {
		if !strings.Contains(window, want) {
			t.Errorf("a window chat's delivery lacks %q:\n%s", want, window)
		}
	}
	outside := bus.FormatForAgent(msgs, "Paul")
	if strings.Contains(outside, bus.WindowServer) || !strings.Contains(outside, "use the bus's reply tool (message_id).") || !strings.Contains(outside, "the bus's read_message tool") {
		t.Errorf("a delivery outside a window:\n%s", outside)
	}
}
