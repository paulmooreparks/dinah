package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func TestBackgroundLimitDefaultsPersistsAndAppliesWithoutRestart(t *testing.T) {
	a, _ := providerTestApp(t)
	before := a.BackgroundChatLimit()
	t.Cleanup(func() { _ = a.SetBackgroundChatLimit(before) })
	if backgroundChatLimit(Settings{}) != 8 {
		t.Fatal("missing setting did not default to eight")
	}
	for _, bad := range []int{-1, 0, 65} {
		if a.SetBackgroundChatLimit(bad) == nil {
			t.Fatalf("accepted %d", bad)
		}
	}
	if err := a.SetBackgroundChatLimit(2); err != nil {
		t.Fatal(err)
	}
	stored, err := loadSettings()
	if err != nil || stored.BackgroundChatLimit != 2 || a.State().BackgroundChatLimit != 2 || a.runtime.BackgroundLimitLocked() != 2 {
		t.Fatalf("setting not persisted/applied: %+v %v", stored.BackgroundChatLimit, err)
	}
	if err := a.SetBackgroundChatLimit(8); err != nil {
		t.Fatal(err)
	}
	r := watchConnects(a)
	a.ChatWindows(nil)
	for i := 0; i < 9; i++ {
		closedChat(t, a, fmt.Sprintf("codex:limit-%d", i), fmt.Sprintf("Limit %d", i))
	}
	wakeRound(t, a)
	if got := r.take(); len(got) != 8 {
		t.Fatalf("default did not admit eight: %v", askedIDs(got))
	}
	if err := a.SetBackgroundChatLimit(9); err != nil {
		t.Fatal(err)
	}
	wakeRound(t, a)
	if got := r.take(); len(got) != 1 || got[0]["sessionId"] != "codex:limit-8" {
		t.Fatalf("increase did not admit waiting chat: %v", askedIDs(got))
	}
	if err := a.SetBackgroundChatLimit(1); err != nil {
		t.Fatal(err)
	}
	if a.backgroundUsedLocked() != 9 {
		t.Fatal("lowering cancelled existing admissions")
	}
}

func TestWaitingSenderNotifiedOnceAndYieldDoesNotSendASecondNotice(t *testing.T) {
	a, _ := providerTestApp(t)
	a.settings.BackgroundChatLimit = 1
	r := watchConnects(a)
	a.ChatWindows(nil)
	closedChat(t, a, "codex:holder", "Holder")
	wakeRound(t, a)
	openAsAsked(t, a, r.take())
	now := time.Now()
	a.session("codex:holder").SetIdleSince(now)
	target, waitingCwd := closedChat(t, a, "codex:waiting", "Waiting member")
	var caller bus.Session
	var group string
	err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		caller, err = b.RegisterSession(context.Background(), "sender-of-waiting", waitingCwd, "Sender")
		if err != nil {
			return err
		}
		room, err := b.CreateGroup(context.Background(), bus.Sender{}, "Waiting notice test", []string{caller.ID, target})
		if err != nil {
			return err
		}
		group = "#" + room.Name
		_, err = b.SendMentioning(context.Background(), bus.Sender{Session: &caller}, group, "Please work", nil, 0, []string{target})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	a.wakeNow = func() time.Time { return now.Add(time.Second) }
	wakeRound(t, a)
	wakeRound(t, a)
	if !a.busCoordinator().WaitingForPlace("codex:waiting") {
		t.Fatal("blocked target not waiting")
	}
	check := func(want int) {
		t.Helper()
		if err := a.busHost().With(func(b *bus.Bus) error {
			messages, err := b.Recent(context.Background(), 100)
			if err != nil {
				return err
			}
			n := 0
			for _, m := range messages {
				if m.Kind == bus.ResultKind && strings.Contains(m.Text, "Waiting member") && m.ToLabel == caller.Title {
					n++
				}
			}
			if n != want {
				t.Fatalf("sender notices %d, want %d: %+v", n, want, messages)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	check(1)
	// A later message from the same sender must not repeat the waiting notice.
	if err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.SendMentioning(context.Background(), bus.Sender{Session: &caller}, group, "One more detail", nil, 0, []string{target})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	wakeRound(t, a)
	check(1)
	a.wakeNow = func() time.Time { return now.Add(sessionhost.BackgroundYield + time.Second) }
	wakeRound(t, a)
	if got := askedIDs(r.take()); !strings.Contains(strings.Join(got, ","), "codex:waiting") {
		t.Fatalf("yield did not open target: %v", got)
	}
	check(1)
}
