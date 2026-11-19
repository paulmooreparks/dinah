package bus

import (
	"context"
	"strings"
	"testing"
	"time"
)

func liveIDs(t *testing.T, b *Bus) string {
	t.Helper()
	ss, err := b.Sessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range ss {
		ids = append(ids, s.ID)
	}
	return strings.Join(ids, ",")
}

// A chat started outside the window that has not been seen for a week is
// ended when another chat starts. A chat of the window's is the window's to
// end, and a chat seen within the week stays.
func TestAChatOutsideTheWindowUnseenForAWeekIsEnded(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	now := time.Date(2026, 11, 3, 14, 40, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	ws := workspace(t, root, "alpha")
	for _, id := range []string{"old-outside", WindowSessionPrefix + "OLD"} {
		if _, err := b.RegisterSession(ctx, id, ws, "Development"); err != nil {
			t.Fatal(err)
		}
	}
	now = now.Add(24 * time.Hour)
	if _, err := b.RegisterSession(ctx, "recent-outside", ws, "Recent"); err != nil {
		t.Fatal(err)
	}
	// Six days and a bit after the first two were seen: nothing is stale yet.
	now = time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)
	if _, err := b.RegisterSession(ctx, "new-1", ws, "New One"); err != nil {
		t.Fatal(err)
	}
	if got := liveIDs(t, b); !strings.Contains(got, "old-outside") {
		t.Fatalf("a chat seen under a week ago was ended: %s", got)
	}
	// Past the week for the first two, and not for the third.
	now = time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	if _, err := b.RegisterSession(ctx, "new-2", ws, "New Two"); err != nil {
		t.Fatal(err)
	}
	got := liveIDs(t, b)
	if strings.Contains(got, "old-outside") {
		t.Fatalf("the chat outside the window, unseen for a week, is still open: %s", got)
	}
	for _, id := range []string{WindowSessionPrefix + "OLD", "recent-outside", "new-1", "new-2"} {
		if !strings.Contains(got, id) {
			t.Fatalf("%s was ended, and should not have been: %s", id, got)
		}
	}
}

// A chat that was ended as unseen is open again as soon as it is seen, as
// any ended chat is.
func TestAStaleChatIsOpenAgainWhenItIsSeen(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	ws := workspace(t, root, "alpha")
	if _, err := b.RegisterSession(ctx, "sleeper", ws, "Sleeper"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(StaleAfter + time.Hour)
	if n, err := b.EndStaleSessions(ctx); err != nil || n != 1 {
		t.Fatalf("ended %d chats (%v), want 1", n, err)
	}
	if got := liveIDs(t, b); got != "" {
		t.Fatalf("still open: %s", got)
	}
	if _, _, err := b.Deliver(ctx, "sleeper", "UserPromptSubmit"); err != nil {
		t.Fatal(err)
	}
	if got := liveIDs(t, b); got != "sleeper" {
		t.Fatalf("a chat that took a delivery is not open again: %q", got)
	}
}

// The program serving a chat's tools keeps the chat seen while it runs, so
// a chat that sits idle is not taken as ended. It does not open a chat that
// has ended.
func TestTheHeartbeatKeepsAnIdleChatOpen(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	ws := workspace(t, root, "alpha")
	for _, id := range []string{"idle", "gone"} {
		if _, err := b.RegisterSession(ctx, id, ws, strings.ToUpper(id[:1])+id[1:]); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.EndSession(ctx, "gone"); err != nil {
		t.Fatal(err)
	}
	for range 10 {
		now = now.Add(24 * time.Hour)
		for _, id := range []string{"idle", "gone"} {
			if err := b.TouchSession(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := b.EndStaleSessions(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := liveIDs(t, b); got != "idle" {
		t.Fatalf("open chats are %q, want the idle one alone", got)
	}
}

// Two chats of one workspace with one title cannot be told apart by the
// workspace, so the refusal gives each chat's ID on the bus, and a message
// sent to that ID arrives.
func TestChatsOfOneWorkspaceThatShareATitleAreSentToByTheirIDs(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	alpha := workspace(t, root, "alpha")
	beta := workspace(t, root, "beta")
	sender, err := b.RegisterSession(ctx, "sender", beta, "Sender")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"twin-1", "twin-2"} {
		if _, err := b.RegisterSession(ctx, id, alpha, "Development"); err != nil {
			t.Fatal(err)
		}
	}
	for _, to := range []string{"Development", "Development (alpha)"} {
		_, err := b.Send(ctx, Sender{Session: &sender}, to, "hello", nil, 0)
		if err == nil {
			t.Fatalf("a message to %q, which two chats are called, was sent", to)
		}
		for _, want := range []string{"address twin-1", "address twin-2", "last seen", "send to the chat's address"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("the refusal for %q does not say %q: %v", to, want, err)
			}
		}
		if strings.Contains(err.Error(), "Add the workspace") {
			t.Fatalf("the refusal for %q advises the workspace, which does not tell the chats apart: %v", to, err)
		}
	}
	m, err := b.Send(ctx, Sender{Session: &sender}, "twin-2", "hello", nil, 0)
	if err != nil {
		t.Fatalf("a message to a chat's ID was refused: %v", err)
	}
	var to string
	if err := b.db.QueryRow(`SELECT to_key FROM messages WHERE id = ?`, m.ID).Scan(&to); err != nil || to != "twin-2" {
		t.Fatalf("the message went to %q (%v), want twin-2", to, err)
	}
	// Chats of two workspaces are still told apart by the workspace.
	if _, err := b.RegisterSession(ctx, "elsewhere", beta, "Shared"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RegisterSession(ctx, "here", alpha, "Shared"); err != nil {
		t.Fatal(err)
	}
	_, err = b.Send(ctx, Sender{Session: &sender}, "Shared", "hello", nil, 0)
	if err == nil || !strings.Contains(err.Error(), "Add the workspace in parentheses") || strings.Contains(err.Error(), "address ") {
		t.Fatalf("chats of two workspaces should be told apart by the workspace alone: %v", err)
	}
	if _, err := b.Send(ctx, Sender{Session: &sender}, "Shared (alpha)", "hello", nil, 0); err != nil {
		t.Fatal(err)
	}
}

// A window that starts ends the chats of the window that an earlier one
// left open, and leaves the chats it has opened itself, the chats started
// outside the window, and a chat that was ended and opens again.
func TestAWindowThatStartsEndsTheChatsAnEarlierOneLeftOpen(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	ws := workspace(t, root, "alpha")
	left, outside := WindowSessionPrefix+"LEFT", "outside"
	for _, id := range []string{left, outside} {
		if _, err := b.RegisterSession(ctx, id, ws, "Development"); err != nil {
			t.Fatal(err)
		}
	}
	// The next window starts a day later and has already opened one chat.
	started := now.Add(24 * time.Hour)
	now = started.Add(time.Second)
	opened := WindowSessionPrefix + "OPENED"
	if _, err := b.RegisterSession(ctx, opened, ws, "Opened"); err != nil {
		t.Fatal(err)
	}
	if n, err := b.EndWindowSessionsSeenBefore(ctx, started); err != nil || n != 1 {
		t.Fatalf("ended %d chats (%v), want the one an earlier window left open", n, err)
	}
	got := liveIDs(t, b)
	if strings.Contains(got, left) || !strings.Contains(got, opened) || !strings.Contains(got, outside) {
		t.Fatalf("open chats are %q", got)
	}
	// The chat it ended is still known, so a message to it waits, and it is
	// open again when its chat opens.
	if _, err := b.RegisterSession(ctx, left, ws, ""); err != nil {
		t.Fatal(err)
	}
	if got := liveIDs(t, b); !strings.Contains(got, left) {
		t.Fatalf("the chat is not open again after it opened: %q", got)
	}
}
