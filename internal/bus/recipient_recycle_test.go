package bus

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecycleRecipientRetainsQueueWithoutWakeAndRestoresDormant(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	cwd := workspace(t, root, "work")
	s, err := b.RegisterSession(ctx, WindowSessionPrefix+"recycle-owner", cwd, "Retained")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Send(ctx, Sender{}, s.Address(), "Retain these bytes", nil, 0); err != nil {
		t.Fatal(err)
	}
	if err = b.BinRecipient(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	checkDormant := func(b *Bus) {
		t.Helper()
		if n, _, err := b.Doorbell(ctx, s.ID); err != nil || n != 0 {
			t.Fatalf("doorbell %d %v", n, err)
		}
		if msgs, _, err := b.Deliver(ctx, s.ID, "UserPromptSubmit"); err != nil || len(msgs) != 0 {
			t.Fatalf("delivery %+v %v", msgs, err)
		}
		wakes, err := b.Waking(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range wakes {
			if w.Session == s.ID {
				t.Fatal("retained recipient was woken")
			}
		}
		var pending int
		if err = b.db.QueryRow(`SELECT COUNT(*) FROM deliveries WHERE recipient=? AND delivered_at IS NULL AND cancelled_at IS NULL`, s.ID).Scan(&pending); err != nil || pending != 1 {
			t.Fatalf("retained queue %d %v", pending, err)
		}
	}
	checkDormant(b)
	if n, err := b.PendingSessionDeliveries(ctx, s.ID); err != nil || n != 0 {
		t.Fatalf("binned count %d %v", n, err)
	}
	if _, err = b.Send(ctx, Sender{}, s.Address(), "Must refuse", nil, 0); err == nil || !strings.Contains(err.Error(), "removed") {
		t.Fatalf("send %v", err)
	}
	if _, err = b.RegisterSession(ctx, s.ID, cwd, "Old process"); err == nil {
		t.Fatal("binned recipient registered")
	}
	restarted, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	checkDormant(restarted)
	if err = restarted.RestoreRecipient(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	checkDormant(restarted)
	// A second removal after restoration must also retain the inactive marker.
	if err = restarted.BinRecipient(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	checkDormant(restarted)
	if err = restarted.RestoreRecipient(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.Send(ctx, Sender{}, s.Address(), "New accepted input", nil, 0); err != nil {
		t.Fatal(err)
	}
	msgs, _, err := restarted.Deliver(ctx, s.ID, "UserPromptSubmit")
	if err != nil || len(msgs) != 2 || msgs[0].Text != "Retain these bytes" {
		t.Fatalf("restored delivery %+v %v", msgs, err)
	}
	if err = restarted.BinRecipient(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if err = restarted.ForgetBinnedRecipient(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = restarted.db.QueryRow(`SELECT COUNT(*) FROM deliveries WHERE recipient=?`, s.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("final queue %d %v", n, err)
	}
	if err = restarted.RestoreRecipient(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.RegisterSession(ctx, s.ID, cwd, "Must stay deleted"); err == nil {
		t.Fatal("final tombstone was restored")
	}
}

func TestRecycleRecipientGroupAndDecisionsStayOffOrdinaryLists(t *testing.T) {
	ctx := context.Background()
	b, s, _ := personRequestFixture(t)
	other, err := b.RegisterSession(ctx, "other-chat", s.WorkspacePath, "Other")
	if err != nil {
		t.Fatal(err)
	}
	g, err := b.CreateGroup(ctx, Sender{Session: &other}, "Cleanup group", []string{s.Address()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.SetLead(ctx, s.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	if err = b.BinRecipient(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if members, err := b.TeamMembers(ctx, other.ID); err != nil || len(members) != 0 {
		t.Fatalf("team lists binned chat %+v %v", members, err)
	}
	if page, err := b.Decisions(ctx, 0); err != nil || len(page.Items) != 0 {
		t.Fatalf("decisions %+v %v", page, err)
	}
	if _, err = b.SendMentioning(ctx, Sender{Session: &other}, "#"+g.Name, "Other members only", nil, 0, []string{Everyone}); err != nil {
		t.Fatal(err)
	}
	groups, err := b.GroupChats(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		if g.Session == s.ID {
			t.Fatal("startup group list reopens binned chat")
		}
	}
	members, err := b.RoomMembers(ctx, g.Name)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.Key == s.ID {
			t.Fatal("group lists binned chat")
		}
	}
	if err = b.RestoreRecipient(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if members, err := b.TeamMembers(ctx, other.ID); err != nil || len(members) != 1 {
		t.Fatalf("team lost restored chat %+v %v", members, err)
	}
	if page, err := b.Decisions(ctx, 0); err != nil || len(page.Items) != 1 {
		t.Fatalf("restored decisions %+v %v", page, err)
	}
	if _, err = b.RegisterSession(ctx, s.ID, s.WorkspacePath, "Explicitly opened"); err != nil {
		t.Fatal(err)
	}
}
