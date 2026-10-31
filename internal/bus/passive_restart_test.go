package bus

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPassiveNoticeSurvivesRestartAndStaleDoorbell(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bus.db")
	b, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	sender, _ := b.RegisterSession(ctx, "sender", dir, "Sender")
	target, _ := b.RegisterSession(ctx, "target", dir, "Target")
	g, err := b.CreateGroup(ctx, Sender{}, "Work", []string{sender.ID, target.ID})
	if err != nil {
		t.Fatal(err)
	}
	request, err := b.SendActionRecipients(ctx, Sender{Session: &sender}, "#"+g.Name, "Please act", nil, 0, []string{target.ID})
	if err != nil {
		t.Fatal(err)
	}
	if n, _, _ := b.Doorbell(ctx, target.ID); n != 1 {
		t.Fatal("request did not ring")
	}
	// Another consumer wins after the scheduler observed the request.
	if got, _, err := b.Deliver(ctx, target.ID, "PostToolUse"); err != nil || len(got) != 1 || got[0].ID != request.ID {
		t.Fatalf("request: %+v %v", got, err)
	}
	notice, err := b.SendActionRecipients(ctx, Sender{Session: &sender}, "#"+g.Name, "Information", nil, 0, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if got, _, err := b.Deliver(ctx, target.ID, DoorbellTurn); err != nil || len(got) != 0 {
		t.Fatalf("stale doorbell: %+v %v", got, err)
	}
	if err := b.EndSession(ctx, target.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if wakes, err := b.Waking(ctx); err != nil || len(wakes) != 0 {
		t.Fatalf("passive post reopened a chat: %+v %v", wakes, err)
	}
	if _, err := b.RegisterSession(ctx, target.ID, dir, "Target"); err != nil {
		t.Fatal(err)
	}
	if n, _, err := b.Doorbell(ctx, target.ID); err != nil || n != 0 {
		t.Fatalf("notice rang after restart: %d %v", n, err)
	}
	got, _, err := b.Deliver(ctx, target.ID, "UserPromptSubmit")
	if err != nil || len(got) != 1 || got[0].ID != notice.ID || !got[0].Notice {
		t.Fatalf("saved notice: %+v %v", got, err)
	}
	if got, _, err := b.Deliver(ctx, target.ID, "UserPromptSubmit"); err != nil || len(got) != 0 {
		t.Fatalf("notice repeated: %+v %v", got, err)
	}
}
