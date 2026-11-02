package bus

import (
	"context"
	"path/filepath"
	"testing"
)

func TestHumanInboxSelectionPersistenceAndBoundedBulk(t *testing.T) {
	b, root := open(t)
	ctx := context.Background()
	sender, err := b.RegisterSession(ctx, "sender", workspace(t, root, "studio"), "Sender")
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for i := 0; i < 205; i++ {
		m, err := b.Send(ctx, Sender{Session: &sender}, Human, "Inbox message", nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, m.ID)
	}
	page, err := b.HumanInboxPage(ctx, "unread", 0, 200)
	if err != nil || page.Unread != 205 || len(page.Items) != 200 || page.NextBefore == 0 {
		t.Fatalf("capped count or missing cursor: %+v %v", page, err)
	}
	older, err := b.HumanInboxPage(ctx, "unread", page.NextBefore, 200)
	if err != nil || len(older.Items) != 5 || older.Items[4].ID != ids[0] || older.NextBefore != 0 {
		t.Fatal("older page", older, err)
	}
	if _, err := b.HumanInboxMessage(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	if unchanged, _ := b.HumanInboxPage(ctx, "unread", 0, 1); unchanged.Unread != 205 {
		t.Fatal("lookup marked an item read")
	}
	if err := b.ChangeHumanInbox(ctx, ids[0], false, false); err != nil {
		t.Fatal(err)
	}
	if err := b.ChangeHumanInbox(ctx, ids[1], false, true); err != nil {
		t.Fatal(err)
	}
	dismissed, err := b.HumanInboxMessage(ctx, ids[1])
	if err != nil || !dismissed.Unread || !dismissed.Dismissed {
		t.Fatal("dismissal changed read state", dismissed, err)
	}
	read, err := b.HumanInboxPage(ctx, "read", 0, 200)
	if err != nil || len(read.Items) != 1 || read.Items[0].ID != ids[0] || read.Unread != 203 {
		t.Fatal("selected-item read", read, err)
	}
	late, err := b.Send(ctx, Sender{Session: &sender}, Human, "Arrived after the snapshot", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.ChangeHumanInbox(ctx, page.Newest, true, false); err != nil {
		t.Fatal(err)
	}
	if err := b.ChangeHumanInbox(ctx, page.Newest, true, true); err != nil {
		t.Fatal(err)
	}
	if got, err := b.HumanInboxPage(ctx, "unread", 0, 200); err != nil || got.Unread != 1 || len(got.Items) != 1 || got.Items[0].ID != late.ID {
		t.Fatal("bulk swallowed new arrival", got, err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if got, err := restarted.HumanInboxPage(ctx, "dismissed", 0, 200); err != nil || len(got.Items) != 200 || got.NextBefore == 0 || got.Unread != 1 {
		t.Fatal("dismissal lost on restart", got, err)
	}
	if err := restarted.ChangeHumanInbox(ctx, late.ID, false, false); err != nil {
		t.Fatal(err)
	}
	if got, err := restarted.HumanInboxPage(ctx, "unread", 0, 200); err != nil || got.Unread != 0 || len(got.Items) != 0 {
		t.Fatal("zero unread", got, err)
	}
	if _, err := restarted.HumanInboxMessage(ctx, late.ID+100); err == nil {
		t.Fatal("missing target accepted")
	}
}

func TestInboxGroupTargetOutsideLatestPage(t *testing.T) {
	b, root := open(t)
	ctx := context.Background()
	sender, err := b.RegisterSession(ctx, "sender", workspace(t, root, "studio"), "Sender")
	if err != nil {
		t.Fatal(err)
	}
	g, err := b.CreateGroup(ctx, Sender{}, "Target group", []string{sender.Address()})
	if err != nil {
		t.Fatal(err)
	}
	var target Message
	for i := 0; i < 305; i++ {
		m, err := b.Send(ctx, Sender{Session: &sender}, "#"+g.Name, "Group message", nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			target = m
		}
	}
	latest, err := b.RoomMessages(ctx, g.Name, 300)
	if err != nil || len(latest) != 300 || latest[0].ID <= target.ID {
		t.Fatal("fixture target was not outside latest page", err)
	}
	context, err := b.RoomMessagesFrom(ctx, g.Name, target.ID, 2)
	if err != nil || len(context) != 305 || context[2].ID != target.ID || context[len(context)-1].ID != latest[len(latest)-1].ID {
		t.Fatal("exact target and context", len(context), err)
	}
	if _, err := b.RoomMessagesFrom(ctx, "different-group", target.ID, 20); err == nil {
		t.Fatal("target from another group accepted")
	}
}
