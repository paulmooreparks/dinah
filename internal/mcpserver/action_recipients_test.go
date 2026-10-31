package mcpserver

import (
	"context"
	"path/filepath"
	"testing"

	"studio/internal/bus"
)

func TestActionRecipientsContract(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	dir := t.TempDir()
	me, _ := b.RegisterSession(ctx, "sender", dir, "Sender")
	target, _ := b.RegisterSession(ctx, "target", dir, "Target")
	g, err := b.CreateGroup(ctx, bus.Sender{}, "Work", []string{me.ID, target.ID})
	if err != nil {
		t.Fatal(err)
	}
	cs := connect(t, b, me.ID)
	for _, args := range []map[string]any{
		{"to": "#" + g.Name, "text": "passive", "actionRecipients": []string{}},
		{"to": "#" + g.Name, "text": "also passive"},
	} {
		if out, failed := call(t, cs, "send", args); failed {
			t.Fatal(out)
		}
	}
	if n, _, _ := b.Doorbell(ctx, target.ID); n != 0 {
		t.Fatalf("passive posts rang %d times", n)
	}
	for _, field := range []string{"actionRecipients", "mentions"} {
		if out, failed := call(t, cs, "send", map[string]any{"to": "#" + g.Name, "text": "act", field: []string{"Target"}}); failed {
			t.Fatal(out)
		}
	}
	if n, _, _ := b.Doorbell(ctx, target.ID); n != 2 {
		t.Fatalf("requests rang %d times", n)
	}
	msgs, _, err := b.Deliver(ctx, target.ID, bus.DoorbellTurn)
	if err != nil || len(msgs) != 4 || !msgs[0].Notice || !msgs[1].Notice || !msgs[2].Asked || !msgs[3].Asked {
		t.Fatalf("delivery: %+v %v", msgs, err)
	}
	root := msgs[2].ID
	for _, tool := range []string{"send", "reply"} {
		args := map[string]any{"text": "conflict", "actionRecipients": []string{}, "mentions": []string{}}
		if tool == "send" {
			args["to"] = "#" + g.Name
		} else {
			args["message_id"] = root
		}
		if out, failed := call(t, cs, tool, args); !failed {
			t.Fatalf("accepted conflicting routing: %s", out)
		}
	}
	if out, failed := call(t, cs, "reply", map[string]any{"message_id": root, "text": "reply action", "actionRecipients": []string{"Target"}}); failed {
		t.Fatal(out)
	}
	if n, _, _ := b.Doorbell(ctx, target.ID); n != 1 {
		t.Fatalf("group reply rang %d times", n)
	}
	if out, failed := call(t, cs, "send", map[string]any{"to": "Target", "text": "invalid", "actionRecipients": []string{}}); !failed {
		t.Fatalf("direct routing accepted: %s", out)
	}
	if out, failed := call(t, cs, "send", map[string]any{"to": "Target", "text": "direct request"}); failed {
		t.Fatal(out)
	}
}
