package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"studio/internal/bus"
)

// The receipt of an agent's group post that names nobody says whom it woke,
// and a message to a chat outside Dinah's window says that chat cannot be
// woken and when it was last active (Paul, 10 November 2026).
func TestTheReceiptSaysWhomAPostWokeAndWhenAChatCannotBeWoken(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	dir := t.TempDir()
	lead, _ := b.RegisterSession(ctx, "window-lead", dir, "Lead")
	docs, _ := b.RegisterSession(ctx, "window-docs", dir, "Docs Pass")
	outside, _ := b.RegisterSession(ctx, "outside-session", dir, "Old Port")
	g, err := b.CreateGroup(ctx, bus.Sender{}, "Review", []string{"Lead", "Docs Pass"})
	if err != nil {
		t.Fatal(err)
	}
	s := New(b)
	receipt := func(m bus.Message) string {
		return fmt.Sprint(s.sentReceipt(ctx, m).Content[0].(*mcp.TextContent).Text)
	}
	// The person asks Docs Pass, which makes it engaged in the group.
	if _, err := b.SendMentioning(ctx, bus.Sender{}, "#"+g.Name, "Docs, please look", nil, 0, []string{docs.ID}); err != nil {
		t.Fatal(err)
	}
	m, err := b.Send(ctx, bus.Sender{Session: &lead}, "#"+g.Name, "The build is ready.", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := receipt(m); !strings.Contains(got, "woke the members active in the group lately: Docs Pass") {
		t.Fatalf("an engaged member's receipt: %s", got)
	}
	m, err = b.SendMentioning(ctx, bus.Sender{Session: &lead}, "#"+g.Name, "For Docs", nil, 0, []string{docs.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := receipt(m); strings.Contains(got, "named nobody") {
		t.Fatalf("an addressed post's receipt: %s", got)
	}
	m, err = b.Send(ctx, bus.Sender{Session: &lead}, "Old Port", "Are you there?", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := receipt(m); !strings.Contains(got, "runs outside Dinah's window, so Dinah cannot start a turn for it") || !strings.Contains(got, "last active at") {
		t.Fatalf("an outside chat's receipt: %s", got)
	}
	m, err = b.Send(ctx, bus.Sender{Session: &outside}, "Docs Pass", "Hello from outside", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := receipt(m); strings.Contains(got, "outside") && strings.Contains(got, "cannot start a turn") {
		t.Fatalf("a window chat's receipt: %s", got)
	}
}
