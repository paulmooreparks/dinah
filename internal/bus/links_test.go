package bus

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// A message far along a chain of handoffs wakes nobody without a trusted
// link, wakes the other side within the link's own limit, and always wakes
// it when the link has no limit.
func TestATrustedLinkHasItsOwnHopLimit(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	alpha, _ := b.RegisterSession(ctx, "s-a", workspace(t, root, "alpha"), "")
	beta, _ := b.RegisterSession(ctx, "s-b", workspace(t, root, "beta"), "")
	if _, err := b.db.Exec(`UPDATE sessions SET hops = 5 WHERE id = ?`, alpha.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{Session: &alpha}, "beta", "Carry on with the migration.", nil, 0); err != nil {
		t.Fatal(err)
	}
	ring := func() int {
		n, _, err := b.Doorbell(ctx, beta.ID)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if ring() != 0 {
		t.Fatal("a message past three hops woke a session with no trusted link")
	}
	if err := b.Trust(ctx, "alpha", "beta", 10); err != nil {
		t.Fatal(err)
	}
	if ring() != 1 {
		t.Fatal("a trusted link with a limit of ten did not wake the session")
	}
	if err := b.Trust(ctx, "beta", "alpha", 5); err != nil {
		t.Fatal(err)
	}
	if ring() != 0 {
		t.Fatal("a link's lower limit was not kept, or naming it the other way round made a second link")
	}
	if err := b.Trust(ctx, "alpha", "beta", NoHopLimit); err != nil {
		t.Fatal(err)
	}
	if ring() != 1 {
		t.Fatal("a link with no limit did not wake the session")
	}
	links, err := b.Links(ctx)
	if err != nil || len(links) != 1 || links[0].A != "alpha" || links[0].B != "beta" || links[0].HopLimit != NoHopLimit {
		t.Fatalf("the links are wrong: %+v %v", links, err)
	}
	msgs, _, err := b.Deliver(ctx, beta.ID, DoorbellTurn)
	if err != nil || len(msgs) != 1 || !msgs[0].Trusted || !strings.Contains(FormatForAgent(msgs, PersonFallback), "marked the link between your workspace and alpha as trusted") {
		t.Fatalf("the message was not delivered as trusted: %+v %v", msgs, err)
	}
	if err := b.Untrust(ctx, "beta", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{Session: &alpha}, "beta", "And the docs.", nil, 0); err != nil {
		t.Fatal(err)
	}
	msgs, _, _ = b.Deliver(ctx, beta.ID, "UserPromptSubmit")
	if len(msgs) != 1 || msgs[0].Trusted {
		t.Fatalf("a message after the link was removed was still trusted: %+v", msgs)
	}
}

// A workspace may trust itself, so that its own chats keep handing work to
// one another past the default limit, as a group of chats in one workspace
// working together needs.
func TestAWorkspaceMayTrustItself(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	ws := workspace(t, root, "alpha")
	one, _ := b.RegisterSession(ctx, "s-1", ws, "")
	two, _ := b.RegisterSession(ctx, "s-2", ws, "")
	if _, err := b.db.Exec(`UPDATE sessions SET hops = 5 WHERE id = ?`, one.ID); err != nil {
		t.Fatal(err)
	}
	b.SetTitle(ctx, two.ID, "Two")
	if _, err := b.Send(ctx, Sender{Session: &one}, "Two", "Your turn.", nil, 0); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := b.Doorbell(ctx, two.ID); n != 0 {
		t.Fatal("a message past three hops woke a chat of the same workspace with no link")
	}
	if err := b.Trust(ctx, "alpha", "alpha", NoHopLimit); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := b.Doorbell(ctx, two.ID); n != 1 {
		t.Fatal("a workspace's link to itself did not wake its own chat")
	}
	links, err := b.Links(ctx)
	if err != nil || len(links) != 1 || links[0].A != "alpha" || links[0].B != "alpha" || links[0].HopLimit != NoHopLimit {
		t.Fatalf("the links are wrong: %+v %v", links, err)
	}
	if err := b.Untrust(ctx, "alpha", "alpha"); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := b.Doorbell(ctx, two.ID); n != 0 {
		t.Fatal("the removed link still woke the chat")
	}
}

// A bus file made when a link had to join two different workspaces is
// rebuilt on opening so that a workspace may trust itself, and its links
// are kept.
func TestAnOlderBusFileLetsAWorkspaceTrustItself(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "bus.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE links (workspace_a TEXT NOT NULL, workspace_b TEXT NOT NULL, hop_limit INTEGER NOT NULL, created_at TEXT NOT NULL,
			PRIMARY KEY (workspace_a, workspace_b), CHECK (workspace_a < workspace_b))`,
		`INSERT INTO links VALUES ('a', 'b', 10, '2026-10-29T00:00:00Z')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	b, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var ddl string
	if err := b.db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'links'`).Scan(&ddl); err != nil || !strings.Contains(ddl, "workspace_a <= workspace_b") {
		t.Fatalf("the links table was not rebuilt: %q %v", ddl, err)
	}
	var n int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM links WHERE workspace_a = 'a' AND workspace_b = 'b' AND hop_limit = 10`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the old link was lost: %d %v", n, err)
	}
	ws := workspace(t, dir, "alpha")
	b.RegisterSession(ctx, "s-1", ws, "")
	if err := b.Trust(ctx, "alpha", "alpha", NoHopLimit); err != nil {
		t.Fatal(err)
	}
}

func TestALinkJoinsTwoWorkspacesOnTheBus(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	b.RegisterSession(ctx, "s-a", workspace(t, root, "alpha"), "")
	if err := b.Trust(ctx, "alpha", "nowhere", 10); err == nil {
		t.Fatal("a link was made to a workspace that is not on the bus")
	}
	b.RegisterSession(ctx, "s-b", workspace(t, root, "beta"), "")
	if err := b.Trust(ctx, "alpha", "beta", 0); err == nil {
		t.Fatal("a link allowed no handoffs at all")
	}
}
