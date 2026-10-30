package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/bus"
)

// groupOfClosedChats makes closed chats with ids "codex:g0", "codex:g1" and
// so on, puts them in one group with a message from the person in it, and
// returns their ids. None has a message waiting for it, so nothing but the
// start-up pass would open them.
func groupOfClosedChats(t *testing.T, a *App, name string, n int) []string {
	t.Helper()
	var refs []BusChatRef
	var ids []string
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("codex:%s%d", name, i)
		ids = append(ids, id)
		refs = append(refs, BusChatRef{ID: id, Cwd: t.TempDir(), Title: fmt.Sprintf("Chat %s%d", name, i)})
	}
	g, err := a.BusStartGroup("Group "+name, refs)
	if err != nil {
		t.Fatal(err)
	}
	// A member says hello, so the group has been used recently. The person's
	// message would wake every member, which is not what the callers test.
	err = a.busHost().With(func(b *bus.Bus) error {
		first, err := b.Session(context.Background(), a.busSessionID(ids[0]))
		if err != nil {
			return err
		}
		_, err = b.Send(context.Background(), bus.Sender{Session: &first}, "#"+g.Name, "Hello all", nil, 0)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

// The chats of a group are opened in the background when the window starts,
// two at a time, without a message waiting for any of them.
func TestGroupChatsAreOpenedInTheBackgroundAtStart(t *testing.T) {
	a, _ := providerTestApp(t)
	r := watchConnects(a)
	ids := groupOfClosedChats(t, a, "g", 5)
	wakeRound(t, a)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a group chat was opened before the page said which chats have windows: %v", askedIDs(got))
	}
	a.ChatWindows(nil)
	wakeRound(t, a)
	asked := r.take()
	if got := askedIDs(asked); strings.Join(got, ",") != strings.Join(ids[:2], ",") {
		t.Fatalf("the first two group chats were not asked for: %v", got)
	}
	for _, m := range asked {
		if m["background"] != true {
			t.Fatalf("a group chat was not asked for in the background: %v", m)
		}
	}
	wakeRound(t, a)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a third group chat was asked for while two were connecting: %v", askedIDs(got))
	}
	openAsAsked(t, a, asked)
	for _, id := range ids[:2] {
		if info := a.session(id).Info(); !info.InBackground || !info.Open {
			t.Fatalf("%s is not open in the background: %+v", id, info)
		}
	}
	wakeRound(t, a)
	asked = r.take()
	if got := askedIDs(asked); strings.Join(got, ",") != strings.Join(ids[2:4], ",") {
		t.Fatalf("the next two were not asked for when the first two connected: %v", got)
	}
	openAsAsked(t, a, asked)
	wakeRound(t, a)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a chat past the four was asked for: %v", askedIDs(got))
	}
}

// The pass is made once, so a chat that closes is not opened again by it, and
// a connect that failed is not tried again.
func TestTheStartPassIsMadeOnceAndAFailureIsNotRetried(t *testing.T) {
	a, _ := providerTestApp(t)
	r := watchConnects(a)
	ids := groupOfClosedChats(t, a, "o", 2)
	a.ChatWindows(nil)
	wakeRound(t, a)
	asked := r.take()
	if len(asked) != 2 {
		t.Fatalf("asked for %v", askedIDs(asked))
	}
	a.busCoordinator().Connected(ids[0], context.DeadlineExceeded)
	openAsAsked(t, a, asked[1:])
	for i := 0; i < 3; i++ {
		wakeRound(t, a)
	}
	if got := r.take(); len(got) != 0 {
		t.Fatalf("the start pass ran again: %v", askedIDs(got))
	}
}

// Only the first four chats are opened, in the order of their groups'
// activity, and a group nobody has used for a week is left alone.
func TestTheStartPassIsBounded(t *testing.T) {
	a, _ := providerTestApp(t)
	r := watchConnects(a)
	ids := groupOfClosedChats(t, a, "b", 10)
	a.ChatWindows(nil)
	var opened []string
	for i := 0; i < 10; i++ {
		wakeRound(t, a)
		asked := r.take()
		opened = append(opened, askedIDs(asked)...)
		openAsAsked(t, a, asked)
	}
	if len(opened) != 4 || strings.Join(opened, ",") != strings.Join(ids[:4], ",") {
		t.Fatalf("opened %v", opened)
	}

	b, _ := providerTestApp(t)
	r = watchConnects(b)
	groupOfClosedChats(t, b, "old", 2)
	b.wakeNow = func() time.Time { return time.Now().Add(8 * 24 * time.Hour) }
	b.ChatWindows(nil)
	wakeRound(t, b)
	if got := r.take(); len(got) != 0 {
		t.Fatalf("a group unused for eight days was opened: %v", askedIDs(got))
	}
}

// A hidden chat and a chat already open are not opened, and a chat with a
// window that is not open is, since the layout restores the window with the
// saved copy only.
func TestTheStartPassSkipsHiddenAndOpenChatsButNotWindowedOnes(t *testing.T) {
	a, _ := providerTestApp(t)
	r := watchConnects(a)
	ids := groupOfClosedChats(t, a, "s", 4)
	if err := a.updateSettings(func(s *Settings) error { s.Hidden = append(s.Hidden, ids[0]); return nil }); err != nil {
		t.Fatal(err)
	}
	a.ChatWindows([]string{ids[1]})
	wakeRound(t, a)
	got := askedIDs(r.take())
	for _, id := range got {
		if id == ids[0] {
			t.Fatalf("the hidden chat %s was asked for", id)
		}
	}
	if len(got) != 2 || got[0] != ids[1] || got[1] != ids[2] {
		t.Fatalf("asked for %v", got)
	}
}

// Chats opened at start do not use the three background slots, so a message
// that wakes another chat is not made to wait for one of them to close.
func TestWarmChatsLeaveTheBackgroundSlotsFree(t *testing.T) {
	a, _ := providerTestApp(t)
	r := watchConnects(a)
	ids := groupOfClosedChats(t, a, "w", 4)
	a.ChatWindows(nil)
	for i := 0; i < 3; i++ {
		wakeRound(t, a)
		openAsAsked(t, a, r.take())
	}
	for _, id := range ids {
		if s := a.session(id); s == nil || !s.Info().InBackground || s.UsesBackgroundSlot() {
			t.Fatalf("%s is not an open background chat outside the slots", id)
		}
	}
	for _, id := range []string{"codex:one", "codex:two", "codex:three"} {
		closedChat(t, a, id, "Chat "+id)
	}
	wakeRound(t, a)
	if got := askedIDs(r.take()); strings.Join(got, ",") != "codex:one,codex:two,codex:three" {
		t.Fatalf("the chats woken by messages did not get their three slots: %v", got)
	}
}

// A message waiting for a chat that is still replaying is announced once, as
// arriving, with who it is from and the start of its text; the chat event
// log keeps it for a window opened meanwhile, and the delivery that follows
// clears it.
func TestAMessageWaitingForAReplayIsAnnouncedOnceAsArriving(t *testing.T) {
	a, c, s, me, turns := busChat(t)
	var mu sync.Mutex
	var arriving []map[string]any
	inner := a.sink
	a.sink = func(name string, data any) {
		if name == "agent:arriving" {
			mu.Lock()
			arriving = append(arriving, data.(map[string]any))
			mu.Unlock()
		}
		inner(name, data)
	}
	long := strings.Repeat("x", 600)
	s.SetReplaying(true)
	sendTo(t, a, me, "Is the release ready? "+long)
	ring(t, a)
	ring(t, a)
	mu.Lock()
	got := append([]map[string]any(nil), arriving...)
	mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("%d arriving events for one waiting message", len(got))
	}
	msgs := got[0]["messages"].([]map[string]any)
	text := msgs[0]["text"].(string)
	if len(msgs) != 1 || !strings.HasPrefix(text, "Is the release ready?") || len([]rune(text)) != 401 || !strings.HasSuffix(text, "…") {
		t.Fatalf("the announcement: %+v", got[0])
	}
	if msgs[0]["id"].(int64) == 0 || msgs[0]["from"] == "" || got[0]["more"].(int) != 0 || got[0]["n"] == nil {
		t.Fatalf("the announcement lacks its fields or its number: %+v", got[0])
	}
	if len(prompts(c)) != 0 || waiting(t, a, s.BusID()) != 1 {
		t.Fatal("announcing took the message")
	}
	gen, _ := s.StreamPosition()
	view := s.ChatEvents(gen, 0)
	found := false
	for _, e := range view.Events {
		if e.Name == "agent:arriving" {
			var data map[string]any
			_ = json.Unmarshal(e.Data, &data)
			found = data["sessionId"] == s.ID()
		}
	}
	if !found {
		t.Fatalf("the chat event log does not have the arriving event: %+v", view)
	}
	// A second message is announced by itself; five more are cut at five.
	sendTo(t, a, me, "And a second one")
	ring(t, a)
	mu.Lock()
	n := len(arriving)
	second := arriving[n-1]["messages"].([]map[string]any)
	mu.Unlock()
	if n != 2 || len(second) != 1 || second[0]["text"] != "And a second one" {
		t.Fatalf("the second announcement: %d events, %+v", n, second)
	}
	// Once the replay ends the messages are delivered together, once.
	s.SetReplaying(false)
	ring(t, a)
	waitTurn(t, turns)
	if p := prompts(c); len(p) != 1 || !strings.Contains(p[0], "And a second one") || !strings.Contains(p[0], "Is the release ready?") {
		t.Fatalf("the delivery: %q", p)
	}
	if s.Announce([]int64{msgs[0]["id"].(int64)}) == nil {
		t.Fatal("a delivered message is still remembered as announced")
	}
}

// More than five waiting messages are announced as five and a count of the
// rest, and the rest are announced as the earlier ones are delivered.
func TestMoreThanFiveArrivingMessagesAreCounted(t *testing.T) {
	a, _, s, me, _ := busChat(t)
	var mu sync.Mutex
	var arriving []map[string]any
	inner := a.sink
	a.sink = func(name string, data any) {
		if name == "agent:arriving" {
			mu.Lock()
			arriving = append(arriving, data.(map[string]any))
			mu.Unlock()
		}
		inner(name, data)
	}
	s.SetRestoring(true)
	for i := 0; i < 7; i++ {
		sendTo(t, a, me, fmt.Sprintf("message %d", i))
	}
	ring(t, a)
	mu.Lock()
	defer mu.Unlock()
	if len(arriving) != 1 || len(arriving[0]["messages"].([]map[string]any)) != 5 || arriving[0]["more"].(int) != 2 {
		t.Fatalf("the announcement: %+v", arriving)
	}
}

// Arriving reads the bus without taking anything, and agrees with the
// doorbell about what is waiting.
func TestArrivingChangesNothing(t *testing.T) {
	a, _, s, me, _ := busChat(t)
	sendTo(t, a, me, "first")
	sendTo(t, a, me, "second")
	for i := 0; i < 2; i++ {
		var msgs []bus.Message
		err := a.busHost().With(func(b *bus.Bus) error {
			var err error
			msgs, _, err = b.Arriving(context.Background(), s.BusID(), 5)
			return err
		})
		if err != nil || len(msgs) != 2 || msgs[0].Text != "first" || waiting(t, a, s.BusID()) != 2 {
			t.Fatalf("pass %d: %+v %v", i, msgs, err)
		}
	}
}
