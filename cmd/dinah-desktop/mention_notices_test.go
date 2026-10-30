package main

import (
	"context"
	"strings"
	"sync"
	"testing"

	"studio/internal/bus"
)

type fakeHostNotifier struct {
	mu      sync.Mutex
	shown   []string
	focused bool
}

func (f *fakeHostNotifier) Notify(title, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = append(f.shown, title+" | "+body)
	return nil
}
func (f *fakeHostNotifier) Attention() error { return nil }
func (f *fakeHostNotifier) Focused() (bool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.focused, true
}
func (f *fakeHostNotifier) Close() error { return nil }
func (f *fakeHostNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.shown)
}

// pass runs the watcher's look for mentions once.
func pass(t *testing.T, a *App) {
	t.Helper()
	if err := a.busHost().With(func(b *bus.Bus) error { a.noticeMentions(context.Background(), b); return nil }); err != nil {
		t.Fatal(err)
	}
}

func mentionCards(t *testing.T, a *App) []Notification {
	t.Helper()
	items, err := a.Notifications()
	if err != nil {
		t.Fatal(err)
	}
	var out []Notification
	for _, n := range items {
		if n.Kind == "mention" {
			out = append(out, n)
		}
	}
	return out
}

// mentionRig is a window with a titled chat and a group of it and a closed
// chat, with the events the window emits and the host notifier recorded.
func mentionRig(t *testing.T) (*App, *Session, bus.Session, string, *fakeHostNotifier, *[]string) {
	t.Helper()
	a, _, s, me, _ := busChat(t)
	if _, err := a.RenameSession(s.ID(), "Lead"); err != nil {
		t.Fatal(err)
	}
	g, err := a.BusStartGroup("Work", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Lead"}, {ID: "closed-chat", Cwd: t.TempDir(), Title: "Timesheet Applet"}})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var events []string
	inner := a.sink
	a.sink = func(name string, data any) {
		if strings.HasPrefix(name, "notifications:") {
			mu.Lock()
			events = append(events, name)
			mu.Unlock()
		}
		if inner != nil {
			inner(name, data)
		}
	}
	notifier := &fakeHostNotifier{}
	a.notices.setNotifier(notifier)
	t.Cleanup(func() { a.notices.setNotifier(nil) })
	a.settings.HostNotificationsOff = false
	return a, s, me, g.Name, notifier, &events
}

func chatSays(t *testing.T, a *App, from bus.Session, group, text string, mentions ...string) {
	t.Helper()
	err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.SendMentioning(context.Background(), bus.Sender{Session: &from}, "#"+group, text, nil, 0, mentions)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A mention of the person makes one card, however many times the window looks,
// with the sender, the group and the message; the operating system is told when
// the window is not the active one; and mentions from before the window first
// looked stay in the inbox without a card.
func TestAMentionMakesOneCardAndTheHostIsTold(t *testing.T) {
	a, _, me, group, notifier, events := mentionRig(t)
	chatSays(t, a, me, group, "an old mention, from before the window looked", bus.Human)
	pass(t, a)
	if cards := mentionCards(t, a); len(cards) != 0 {
		t.Fatalf("a mention from before the baseline made a card: %+v", cards)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		old, err := b.Mentions(context.Background(), 0)
		if len(old) != 1 {
			t.Fatalf("the old mention left the inbox: %v %v", old, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	chatSays(t, a, me, group, "I need a decision on the budget", bus.Human)
	pass(t, a)
	pass(t, a)
	cards := mentionCards(t, a)
	if len(cards) != 1 {
		t.Fatalf("%d cards", len(cards))
	}
	c := cards[0]
	if c.Kind != "mention" || c.Severity != "info" || c.Title != "Lead mentioned you in Work" || c.Body != "I need a decision on the budget" || c.GroupTitle != "Work" || c.Group == "" || c.Message == 0 || c.SessionTitle != "Lead" || c.SessionID == "" {
		t.Fatalf("the card: %+v", c)
	}
	if got := strings.Count(strings.Join(*events, ","), "notifications:added"); got != 1 {
		t.Fatalf("%d added events: %v", got, *events)
	}
	if notifier.count() != 1 || !strings.Contains(notifier.shown[0], "Lead mentioned you in Work") {
		t.Fatalf("the host was told %v", notifier.shown)
	}

	// A chat that mentions only the other chat, or nobody, makes no card.
	chatSays(t, a, me, group, "just for the group")
	pass(t, a)
	if len(mentionCards(t, a)) != 1 {
		t.Fatal("a group message that did not mention the person made a card")
	}

	// The window with the focus, or with the setting off, shows nothing from the host.
	notifier.focused = true
	chatSays(t, a, me, group, "a second mention while focused", bus.Human)
	pass(t, a)
	a.settings.HostNotificationsOff = true
	notifier.focused = false
	chatSays(t, a, me, group, "a third mention with the setting off", bus.Human)
	pass(t, a)
	if len(mentionCards(t, a)) != 3 {
		t.Fatalf("%d cards after three mentions", len(mentionCards(t, a)))
	}
	if notifier.count() != 1 {
		t.Fatalf("the host was told %d times: %v", notifier.count(), notifier.shown)
	}
}

// Reading the message dismisses its card, in the group's window or the inbox,
// and dismissing a card leaves its message unread.
func TestReadingAMentionDismissesItsCardAndNotTheOtherWayRound(t *testing.T) {
	a, _, me, group, _, events := mentionRig(t)
	pass(t, a)
	chatSays(t, a, me, group, "first mention", bus.Human)
	chatSays(t, a, me, group, "second mention", bus.Human)
	pass(t, a)
	cards := mentionCards(t, a)
	if len(cards) != 2 {
		t.Fatalf("%d cards", len(cards))
	}
	// Dismissing a card does not read its message.
	if err := a.DismissNotification(cards[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		unread, err := b.Mentions(context.Background(), 0)
		if err != nil || len(unread) != 2 {
			t.Fatalf("dismissing a card read its message: %d unread, %v", len(unread), err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Reading the group dismisses the card that is left.
	if err := a.BusMarkGroupRead(group); err != nil {
		t.Fatal(err)
	}
	pass(t, a)
	if left := mentionCards(t, a); len(left) != 0 {
		t.Fatalf("a read mention kept its card: %+v", left)
	}
	if !strings.Contains(strings.Join(*events, ","), "notifications:dismissed") {
		t.Fatalf("the page was not told: %v", *events)
	}
}
