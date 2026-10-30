package sessionhost

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func eventsOf(t *testing.T, c *Conversation, after uint64) ChatEventsView {
	t.Helper()
	gen, _ := c.StreamPosition()
	return c.ChatEvents(gen, after)
}

// An entry is in the log before its event is emitted, under concurrent
// emitters: inside the emit callback, the entry the event is numbered as is
// already readable. With the append moved after the emit, this fails at the
// first event. The numbers that result are 1 to the count, each once, and the
// log reads back in number order with no gap.
func TestEmitChatStoresBeforeEmitting(t *testing.T) {
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{}, false, false)
	gen, _ := c.StreamPosition()
	const writers, each = 8, 250
	var mu sync.Mutex
	seen := map[uint64]bool{}
	emit := func(name string, v any) {
		data := v.(map[string]any)
		n := data["n"].(uint64)
		if data["gen"] != gen {
			t.Errorf("event %d has generation %v", n, data["gen"])
		}
		view := c.ChatEvents(gen, n-1)
		if len(view.Events) == 0 || view.Events[0].N != n {
			t.Errorf("event %d was emitted before its entry was kept: %+v", n, view.Events)
		}
		mu.Lock()
		if seen[n] {
			t.Errorf("number %d was used twice", n)
		}
		seen[n] = true
		mu.Unlock()
	}
	var wg sync.WaitGroup
	for w := range writers {
		wg.Go(func() {
			for i := range each {
				EmitChat(emit, c, EventUpdate, map[string]any{"sessionId": "chat", "update": fmt.Sprintf("%d-%d", w, i)})
			}
		})
	}
	wg.Wait()
	all := eventsOf(t, c, 0)
	if len(all.Events) != writers*each || all.Head != writers*each || all.Truncated || all.Reset {
		t.Fatalf("the log holds %d events to %d (truncated %v, reset %v)", len(all.Events), all.Head, all.Truncated, all.Reset)
	}
	for i, e := range all.Events {
		if e.N != uint64(i+1) || e.Name != EventUpdate {
			t.Fatalf("entry %d is number %d, %s", i, e.N, e.Name)
		}
	}
}

// The event a page receives is the data given with gen and n added; the
// caller's map is not changed, and the entry keeps the data without them.
func TestEmitChatAddsTheNumberToACopy(t *testing.T) {
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{}, false, false)
	data := map[string]any{"sessionId": "chat", "stopReason": "end_turn"}
	var got map[string]any
	EmitChat(func(_ string, v any) { got = v.(map[string]any) }, c, EventTurn, data)
	if got["n"] != uint64(1) || got["gen"] == "" || got["stopReason"] != "end_turn" {
		t.Fatalf("emitted %v", got)
	}
	if _, changed := data["n"]; changed {
		t.Fatal("the caller's map was changed")
	}
	entry := eventsOf(t, c, 0).Events[0]
	if string(entry.Data) != `{"sessionId":"chat","stopReason":"end_turn"}` || entry.Name != EventTurn {
		t.Fatalf("the entry is %s %s", entry.Name, entry.Data)
	}
	// A session with no open chat has no log, and its event is sent as it is.
	var plain any
	EmitChat(func(_ string, v any) { plain = v }, nil, EventUpdate, data)
	if m := plain.(map[string]any); m["n"] != nil || m["gen"] != nil {
		t.Fatalf("an event with no chat was numbered: %v", m)
	}
}

// Replayed updates take numbers in the order they are queued, which is the
// order the agent sent them, and two goroutines flushing the queue at once
// never send a number twice or leave one out.
func TestReplayIsNumberedInQueueOrder(t *testing.T) {
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{}, true, false)
	const total = 450
	var mu sync.Mutex
	sent := map[uint64]string{}
	emit := func(name string, v any) {
		if name != EventUpdates {
			t.Errorf("a replay was sent as %s", name)
			return
		}
		data := v.(map[string]any)
		ns := data["ns"].([]uint64)
		updates := data["updates"].([]json.RawMessage)
		mu.Lock()
		defer mu.Unlock()
		for i, n := range ns {
			if i > 0 && n <= ns[i-1] {
				t.Errorf("a batch is out of order: %v", ns)
			}
			if _, again := sent[n]; again {
				t.Errorf("number %d was sent twice", n)
			}
			sent[n] = string(updates[i])
		}
	}
	flush := func() { EmitChatReplay(emit, c, c.DrainReplay()) }
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 2 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					flush()
				}
			}
		})
	}
	for i := range total {
		replay, _ := c.QueueIfReplaying(json.RawMessage(fmt.Sprintf(`{"i":%d}`, i)), 200)
		if !replay {
			t.Fatal("an update during the replay was not queued")
		}
	}
	close(stop)
	wg.Wait()
	flush()
	if len(sent) != total {
		t.Fatalf("%d of %d updates were sent", len(sent), total)
	}
	for n := uint64(1); n <= total; n++ {
		if sent[n] != fmt.Sprintf(`{"i":%d}`, n-1) {
			t.Fatalf("number %d is %s, so the numbers do not follow the agent's order", n, sent[n])
		}
	}
	if view := eventsOf(t, c, 0); len(view.Events) != total || view.Events[total-1].N != total {
		t.Fatalf("the log has %d replayed entries", len(view.Events))
	}
}

// Numbering a replayed update copies it, so a caller that reuses its buffer
// does not change what the queue sends.
func TestReplayNumbersCopyTheirUpdates(t *testing.T) {
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{}, true, false)
	raw := json.RawMessage(`{"value":1}`)
	c.QueueReplay([]json.RawMessage{raw}, 10)
	raw[9] = '2'
	if got := c.DrainReplay(); len(got) != 1 || string(got[0].Update) != `{"value":1}` {
		t.Fatalf("the queue shares its update with the caller: %+v", got)
	}
}

func TestChatEventsReadsAfterANumber(t *testing.T) {
	r := &Runtime{}
	r.SetChatLogsLocked(NewChatLogs(ChatLogLimits{Events: 5, Bytes: 1 << 20, TotalBytes: 1 << 30}))
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{}, false, false)
	r.PutConversationLocked("chat", c)
	emit := func(string, any) {}
	for i := 1; i <= 8; i++ {
		EmitChat(emit, c, EventUpdate, map[string]any{"i": i})
	}
	gen, head := c.StreamPosition()
	if head != 8 {
		t.Fatalf("head %d", head)
	}
	// The log keeps 4 to 8; 3 is the highest it dropped.
	if v := c.ChatEvents(gen, 3); v.Truncated || len(v.Events) != 5 || v.Events[0].N != 4 {
		t.Fatalf("after the highest dropped: %+v", v)
	}
	if v := c.ChatEvents(gen, 2); !v.Truncated || len(v.Events) != 5 {
		t.Fatalf("before the highest dropped: %+v", v)
	}
	if v := c.ChatEvents(gen, 8); v.Truncated || v.Reset || v.Events == nil || len(v.Events) != 0 {
		t.Fatalf("at the head: %+v", v)
	}
	if v := c.ChatEvents(gen, 9); !v.Reset {
		t.Fatalf("ahead of the head: %+v", v)
	}
	if v := c.ChatEvents("another", 3); !v.Reset || v.Gen != gen || len(v.Events) != 0 {
		t.Fatalf("another generation: %+v", v)
	}
}

// An entry larger than the per-chat byte bound is numbered but not kept, and
// a page that missed it is told so.
func TestAnOversizeEntryIsNotKept(t *testing.T) {
	r := &Runtime{}
	r.SetChatLogsLocked(NewChatLogs(ChatLogLimits{Events: 100, Bytes: 200, TotalBytes: 1 << 30}))
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{}, false, false)
	r.PutConversationLocked("chat", c)
	var numbers []uint64
	emit := func(_ string, v any) { numbers = append(numbers, v.(map[string]any)["n"].(uint64)) }
	EmitChat(emit, c, EventUpdate, map[string]any{"i": 1})
	EmitChat(emit, c, EventUpdate, map[string]any{"picture": strings.Repeat("x", 500)})
	EmitChat(emit, c, EventUpdate, map[string]any{"i": 3})
	if fmt.Sprint(numbers) != "[1 2 3]" {
		t.Fatalf("numbers %v", numbers)
	}
	gen, _ := c.StreamPosition()
	if v := c.ChatEvents(gen, 1); !v.Truncated || len(v.Events) != 1 || v.Events[0].N != 3 {
		t.Fatalf("a page that missed the oversize entry: %+v", v)
	}
	if v := c.ChatEvents(gen, 2); v.Truncated || len(v.Events) != 1 {
		t.Fatalf("a page that had it: %+v", v)
	}
	if got := r.ChatLogsLocked().Total(); got > 200 {
		t.Fatalf("the oversize entry was counted: total %d", got)
	}
}

// The highest number not kept only ever rises. Entries 1 to 5 are kept, 6 is
// too large to keep, then trimming drops 1: a page at 5 must still be told
// that 6 is gone. If trimming lowered the marker to 1, the page would be told
// nothing was lost.
func TestTheLossMarkerNeverMovesBack(t *testing.T) {
	r := &Runtime{}
	r.SetChatLogsLocked(NewChatLogs(ChatLogLimits{Events: 6, Bytes: 400, TotalBytes: 1 << 30}))
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{}, false, false)
	r.PutConversationLocked("chat", c)
	emit := func(string, any) {}
	for i := 1; i <= 5; i++ {
		EmitChat(emit, c, EventUpdate, map[string]any{"i": i})
	}
	EmitChat(emit, c, EventUpdate, map[string]any{"picture": strings.Repeat("x", 500)})
	gen, _ := c.StreamPosition()
	if v := c.ChatEvents(gen, 5); !v.Truncated {
		t.Fatalf("before the trim, a page at 5 was not told 6 is gone: %+v", v)
	}
	// Two more small entries make seven kept, over the bound of six, so the
	// oldest, 1, is trimmed.
	EmitChat(emit, c, EventUpdate, map[string]any{"i": 7})
	EmitChat(emit, c, EventUpdate, map[string]any{"i": 8})
	if v := c.ChatEvents(gen, 1); v.Events[0].N != 2 {
		t.Fatalf("entry 1 was not trimmed: %+v", v.Events)
	}
	if v := c.ChatEvents(gen, 5); !v.Truncated || len(v.Events) != 2 || v.Events[0].N != 7 {
		t.Fatalf("after the trim, a page at 5 was not told 6 is gone: %+v", v)
	}
	if v := c.ChatEvents(gen, 6); v.Truncated {
		t.Fatalf("a page at 6 was told of a loss: %+v", v)
	}
}

// The logs' total stays within its bound after each append, the largest log
// gives back first, each trim is logged, and a closed chat's bytes leave the
// total.
func TestChatLogsTotalIsBoundedAndTrimsTheLargest(t *testing.T) {
	r := &Runtime{}
	logs := NewChatLogs(ChatLogLimits{Events: 1000, Bytes: 1 << 20, TotalBytes: 3000})
	var trims []string
	logs.Logf = func(level, format string, args ...any) { trims = append(trims, fmt.Sprintf(format, args...)) }
	r.SetChatLogsLocked(logs)
	big := NewConversation(Identity{ID: "big"}, InitialActivity{}, false, false)
	small := NewConversation(Identity{ID: "small"}, InitialActivity{}, false, false)
	r.PutConversationLocked("big", big)
	r.PutConversationLocked("small", small)
	emit := func(string, any) {}
	for i := range 3 {
		EmitChat(emit, small, EventUpdate, map[string]any{"i": i})
	}
	smallBytes := logs.Total()
	for i := range 200 {
		EmitChat(emit, big, EventUpdate, map[string]any{"text": strings.Repeat("y", 40), "i": i})
		if logs.Total() > 3000 {
			t.Fatalf("after append %d the total is %d", i, logs.Total())
		}
	}
	if v := eventsOf(t, small, 0); len(v.Events) != 3 || v.Truncated {
		t.Fatalf("the small log was trimmed before the large one: %+v", v)
	}
	if v := eventsOf(t, big, 0); !v.Truncated {
		t.Fatal("the large log was not trimmed")
	}
	if len(trims) == 0 || !strings.Contains(trims[0], "chat big's event log") {
		t.Fatalf("trims logged: %q", trims)
	}
	r.RemoveConversationLocked("big")
	if logs.Total() != smallBytes {
		t.Fatalf("after closing the large chat the total is %d, want %d", logs.Total(), smallBytes)
	}
	// A chat put again under its ID replaces the old one's bytes.
	again := NewConversation(Identity{ID: "small"}, InitialActivity{}, false, false)
	r.PutConversationLocked("small", again)
	if logs.Total() != 0 {
		t.Fatalf("after replacing the small chat the total is %d", logs.Total())
	}
}

// No two log mutexes are ever held at once: appending, trimming across logs,
// reading, building a chat's info, and registering and removing logs each take
// one at a time. Run from one goroutine, so the count is that goroutine's.
func TestNoTwoChatLogLocksAreHeldTogether(t *testing.T) {
	held, most := 0, 0
	chatLogLocked = func() {
		held++
		most = max(most, held)
	}
	chatLogUnlocked = func() { held-- }
	t.Cleanup(func() { chatLogLocked, chatLogUnlocked = nil, nil })
	r := &Runtime{}
	r.SetChatLogsLocked(NewChatLogs(ChatLogLimits{Events: 1000, Bytes: 1 << 20, TotalBytes: 2000}))
	var chats []*Conversation
	for i := range 6 {
		c := NewConversation(Identity{ID: fmt.Sprint("chat", i)}, InitialActivity{}, i%2 == 0, false)
		r.PutConversationLocked(c.ID(), c)
		chats = append(chats, c)
	}
	emit := func(string, any) {}
	for round := range 40 {
		for i, c := range chats {
			EmitChat(emit, c, EventSent, map[string]any{"text": strings.Repeat("z", 10*(i+1)), "round": round})
			if i%2 == 0 {
				c.QueueReplay([]json.RawMessage{json.RawMessage(`{"r":1}`)}, 1000)
				EmitChatReplay(emit, c, c.DrainReplay())
			}
			_ = c.Info()
			_ = eventsOf(t, c, 0)
		}
	}
	r.RemoveConversationLocked("chat0")
	r.PutConversationLocked("chat1", NewConversation(Identity{ID: "chat1"}, InitialActivity{}, false, false))
	if held != 0 || most != 1 {
		t.Fatalf("at most %d log mutexes were held together, and %d are still held", most, held)
	}
}

// Closing a chat and opening it again gives it a new generation, so a page
// that asks with the old one is told to reopen it. A chat's info says which
// generation and number its log is at.
func TestReopeningAChatChangesItsGeneration(t *testing.T) {
	first := NewConversation(Identity{ID: "chat"}, InitialActivity{}, false, false)
	EmitChat(func(string, any) {}, first, EventTurn, map[string]any{"sessionId": "chat"})
	gen, head := first.StreamPosition()
	info := first.Info()
	if info.StreamGen != gen || info.StreamHead != head || head != 1 {
		t.Fatalf("info %q %d, log %q %d", info.StreamGen, info.StreamHead, gen, head)
	}
	second := NewConversation(Identity{ID: "chat"}, InitialActivity{}, false, false)
	if g, _ := second.StreamPosition(); g == gen {
		t.Fatal("a reopened chat kept its generation")
	}
	if v := second.ChatEvents(gen, 1); !v.Reset {
		t.Fatalf("the old generation was accepted: %+v", v)
	}
}

// agent:update, agent:updates, agent:turn, agent:steer, agent:sent and agent:arriving are
// emitted only through EmitChat and EmitChatReplay, so every one is numbered
// in its chat's log. This fails when any other Go source names one of those
// events or emits one by its constant.
func TestChatEventsHaveOneEmitPath(t *testing.T) {
	root := filepath.Join("..", "..")
	names := regexp.MustCompile(`"agent:(update|updates|turn|steer|sent|arriving)"`)
	constants := regexp.MustCompile(`(?i)emit\(\s*(sessionhost\.)?Event(Update|Updates|Turn|Steer|Sent)\b`)
	own := filepath.Join("internal", "sessionhost", "chat_events.go")
	var found []string
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "frontend") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if rel == own {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(data), "\n") {
				if names.MatchString(line) || constants.MatchString(line) {
					found = append(found, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(found) > 0 {
		t.Fatalf("chat events emitted outside EmitChat:\n%s", strings.Join(found, "\n"))
	}
	// The scan itself finds what it looks for.
	if !names.MatchString(`a.emit("agent:turn", x)`) || !constants.MatchString(`t.host.Emit(EventSteer, x)`) {
		t.Fatal("the scan's patterns do not match an emit")
	}
}
