package bus

import (
	"context"
	"strings"
	"testing"
)

// A message never keeps a chat's applet context, whoever wrote it: the stored
// text and every delivery have it replaced.
func TestAMessageNeverKeepsAnAppletContext(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio := workspace(t, root, "studio")
	lead, _ := b.RegisterSession(ctx, "window-lead", studio, "Lead")
	docs, _ := b.RegisterSession(ctx, "window-docs", studio, "Docs Pass")
	const live = "dinah-applet-v1:ABCDEFGHIJKLMNOPQRSTUVWXYZ.ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	m, err := b.Send(ctx, Sender{Session: &lead}, "Docs Pass", "Use this context: "+live+" for the call.", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(m.Text, "ABCDEFGH") || !strings.Contains(m.Text, "[private applet context]") {
		t.Fatalf("the stored message: %q", m.Text)
	}
	got, _, err := b.Deliver(ctx, docs.ID, DoorbellTurn)
	if err != nil || len(got) != 1 {
		t.Fatalf("deliveries: %+v %v", got, err)
	}
	if strings.Contains(got[0].Text, "ABCDEFGH") || strings.Contains(FormatOne(got[0], "Paul"), "ABCDEFGH") {
		t.Fatalf("the delivery holds the context: %q", got[0].Text)
	}
	// The person's message, a reply and a group message are covered too: they
	// all go through the same insert.
	g, err := b.CreateGroup(ctx, Sender{}, "Review", []string{"Lead", "Docs Pass"})
	if err != nil {
		t.Fatal(err)
	}
	gm, err := b.SendMentioning(ctx, Sender{}, "#"+g.Name, "Context "+live, nil, 0, nil)
	if err != nil || strings.Contains(gm.Text, "ABCDEFGH") {
		t.Fatalf("a group message: %q %v", gm.Text, err)
	}
}

// Messages stored before the bus stopped keeping contexts are cleaned once, and
// a message without one is left exactly as it was.
func TestStoredMessagesAreCleanedOfAppletContexts(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio := workspace(t, root, "studio")
	lead, _ := b.RegisterSession(ctx, "window-lead", studio, "Lead")
	b.RegisterSession(ctx, "window-docs", studio, "Docs Pass")
	clean, err := b.Send(ctx, Sender{Session: &lead}, "Docs Pass", "A message with no context.", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	dirty, err := b.Send(ctx, Sender{Session: &lead}, "Docs Pass", "placeholder", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	// As an older build stored it.
	const live = "dinah-applet-v1:ABCDEFGHIJKLMNOPQRSTUVWXYZ.ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if _, err := b.db.Exec(`UPDATE messages SET text = ? WHERE id = ?`, "Use "+live+" please", dirty.ID); err != nil {
		t.Fatal(err)
	}
	n, err := b.RedactStoredMessages(ctx)
	if err != nil || n != 1 {
		t.Fatalf("the pass changed %d messages: %v", n, err)
	}
	got, _ := b.Message(ctx, dirty.ID)
	if strings.Contains(got.Text, "ABCDEFGH") || got.Text != "Use [private applet context] please" {
		t.Fatalf("the stored message: %q", got.Text)
	}
	if kept, _ := b.Message(ctx, clean.ID); kept.Text != clean.Text {
		t.Fatalf("a clean message was changed: %q", kept.Text)
	}
	if n, _ := b.RedactStoredMessages(ctx); n != 0 {
		t.Fatalf("a second pass changed %d messages", n)
	}
}
