package bus

import (
	"context"
	"errors"
	"testing"
)

func TestResolveRoomNavigation(t *testing.T) {
	ctx := context.Background()
	b, g, lead, _, _ := noticeGroup(t)
	old := "Original navigation title"
	if _, err := b.RenameGroup(ctx, Sender{}, g.Name, old); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RenameGroup(ctx, Sender{}, g.Name, "Current navigation title"); err != nil {
		t.Fatal(err)
	}
	check := func(address, wantState, wantKey string) {
		t.Helper()
		before, err := b.ChangeCounter(ctx)
		if err != nil {
			t.Fatal(err)
		}
		state, r, err := b.ResolveRoom(ctx, address)
		if err != nil || state != wantState || (r == nil) != (wantKey == "") {
			t.Fatalf("%q: %s %+v %v", address, state, r, err)
		}
		if r != nil && r.Name != wantKey {
			t.Fatalf("%q: room %s, want %s", address, r.Name, wantKey)
		}
		after, err := b.ChangeCounter(ctx)
		if err != nil || before != after {
			t.Fatalf("lookup changed the bus: %d -> %d, %v", before, after, err)
		}
	}
	for _, address := range []string{g.Name, "Current navigation title", "#" + old} {
		check(address, "resolved", g.Name)
	}
	if _, err := b.Send(ctx, Sender{Session: &lead}, "#"+old, "stale", nil, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale send: %v", err)
	}
	check("unknown navigation room", "not-found", "")
	other, err := b.CreateGroup(ctx, Sender{}, old, []string{"Lead", "Docs Pass"})
	if err != nil {
		t.Fatal(err)
	}
	check(old, "resolved", other.Name)
	if _, err := b.db.Exec(`UPDATE rooms SET title = ? WHERE name = ?`, g.Name, other.Name); err != nil {
		t.Fatal(err)
	}
	check(g.Name, "resolved", g.Name)
	if _, err := b.db.Exec(`UPDATE rooms SET title = ? WHERE name = ?`, old, other.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RenameGroup(ctx, Sender{}, other.Name, "Other navigation title"); err != nil {
		t.Fatal(err)
	}
	check(old, "ambiguous", "")
	// A window that retained its canonical key survives further renames.
	if _, err := b.RenameGroup(ctx, Sender{}, g.Name, "Latest navigation title"); err != nil {
		t.Fatal(err)
	}
	check(g.Name, "resolved", g.Name)
	_, r, err := b.ResolveRoom(ctx, g.Name)
	if err != nil || r.Title != "Latest navigation title" {
		t.Fatalf("latest title: %+v %v", r, err)
	}
	if err := b.DeleteRoom(ctx, g.Name); err != nil {
		t.Fatal(err)
	}
	check(g.Name, "not-found", "")
	check("Latest navigation title", "not-found", "")
	check(old, "resolved", other.Name)
}

func TestResolveRoomOperationalFailure(t *testing.T) {
	b, _, _, _, _ := noticeGroup(t)
	b.db.Close()
	_, room, err := b.ResolveRoom(context.Background(), "anything")
	if err == nil || room != nil {
		t.Fatalf("closed database: %+v %v", room, err)
	}
}

func TestResolveRoomRenameDuringResolution(t *testing.T) {
	ctx := context.Background()
	b, g, _, _, _ := noticeGroup(t)
	if _, err := b.RenameGroup(ctx, Sender{}, g.Name, "Selected navigation title"); err != nil {
		t.Fatal(err)
	}
	_, key, err := b.resolveRoomKey(ctx, "Selected navigation title")
	if err != nil || key != g.Name {
		t.Fatalf("identity selection: %s %v", key, err)
	}
	if _, err := b.RenameGroup(ctx, Sender{}, g.Name, "Renamed during lookup"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.CreateGroup(ctx, Sender{}, "Selected navigation title", []string{"Lead", "Docs Pass"}); err != nil {
		t.Fatal(err)
	}
	state, r, err := b.resolvedRoom(ctx, key)
	if err != nil || state != "resolved" || r == nil || r.Name != g.Name || r.Title != "Renamed during lookup" {
		t.Fatalf("after title reuse: %s %+v %v", state, r, err)
	}
	if err := b.DeleteRoom(ctx, g.Name); err != nil {
		t.Fatal(err)
	}
	state, r, err = b.resolvedRoom(ctx, key)
	if err != nil || state != "not-found" || r != nil {
		t.Fatalf("after deletion: %s %+v %v", state, r, err)
	}
}
