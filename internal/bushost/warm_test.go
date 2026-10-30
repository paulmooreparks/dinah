package bushost

import (
	"fmt"
	"testing"
	"time"
)

func warmList(n int) []Decision {
	var out []Decision
	for i := 0; i < n; i++ {
		out = append(out, Decision{Chat: fmt.Sprint("chat", i), Path: "folder"})
	}
	return out
}

// At most WarmChats are queued, once, and WarmConcurrency connect at a time.
func TestWarmQueueIsBoundedAndMadeOnce(t *testing.T) {
	var s Scheduler
	now := time.Now()
	s.SetWindows(nil)
	s.Expire(now)
	s.QueueWarm(warmList(20))
	s.QueueWarm(warmList(3))
	never := func(string) bool { return false }
	var started []string
	for {
		d, ok := s.NextWarm(never, now)
		if !ok {
			break
		}
		if !d.Background {
			t.Fatal("a warm chat was not asked for in the background")
		}
		started = append(started, d.Chat)
	}
	if len(started) != WarmConcurrency || s.WarmPending() != WarmConcurrency {
		t.Fatalf("started %v", started)
	}
	total := len(started)
	for len(started) > 0 {
		for _, chat := range started {
			s.Complete(chat, nil, now)
		}
		started = started[:0]
		for {
			d, ok := s.NextWarm(never, now)
			if !ok {
				break
			}
			started = append(started, d.Chat)
			total++
		}
	}
	if total != WarmChats {
		t.Fatalf("%d warm chats were started, want %d", total, WarmChats)
	}
}

// Warm connects do not count against the background slots, so chats woken by
// messages still get all of them.
func TestWarmConnectsLeaveTheSlotsFree(t *testing.T) {
	var s Scheduler
	now := time.Now()
	s.SetWindows(nil)
	s.Expire(now)
	s.QueueWarm(warmList(2))
	for i := 0; i < WarmConcurrency; i++ {
		if _, ok := s.NextWarm(func(string) bool { return false }, now); !ok {
			t.Fatal("no warm connect")
		}
	}
	if s.BackgroundPending() != 0 {
		t.Fatalf("%d slots used by warm connects", s.BackgroundPending())
	}
	paths := map[string]string{}
	for i := 0; i < BackgroundSlots; i++ {
		id := fmt.Sprint("woken", i)
		paths[id] = "folder"
		s.Consider(id, "folder", 1, false, false, now)
		if _, ok := s.Next(paths, 0, now); !ok {
			t.Fatalf("woken chat %d did not get a slot", i)
		}
	}
}

// A chat already connecting or open is not warmed. A chat with a window is, as
// a chat that is not a background one, since the layout restores a window with
// its saved copy and does not open the chat.
func TestWarmQueueSkipsChatsThatNeedNoOpening(t *testing.T) {
	var s Scheduler
	now := time.Now()
	s.SetWindows([]string{"chat0"})
	s.Expire(now)
	s.Consider("chat1", "folder", 1, false, false, now)
	s.QueueWarm(warmList(4))
	d, ok := s.NextWarm(func(chat string) bool { return chat == "chat2" }, now)
	if !ok || d.Chat != "chat0" || d.Background {
		t.Fatalf("the chat with a window was not warmed as a foreground chat: %+v %v", d, ok)
	}
	d, ok = s.NextWarm(func(chat string) bool { return chat == "chat2" }, now)
	if !ok || d.Chat != "chat3" || !d.Background {
		t.Fatalf("got %+v %v", d, ok)
	}
}
