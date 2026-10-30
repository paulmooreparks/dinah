package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// event is one frame of the stream, read back.
type event struct {
	epoch string
	seq   uint64
	name  string
	data  json.RawMessage
}

func parseFrame(t *testing.T, frame []byte) event {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(string(frame), "\n\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "id: ") || !strings.HasPrefix(lines[1], "data: ") {
		t.Fatalf("a frame is not an id and a data line: %q", frame)
	}
	epoch, seq, ok := parseCursor(strings.TrimPrefix(lines[0], "id: "))
	if !ok {
		t.Fatalf("the frame's id is not a cursor: %q", lines[0])
	}
	var body struct {
		Name string          `json:"name"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "data: ")), &body); err != nil {
		t.Fatal(err)
	}
	return event{epoch, seq, body.Name, body.Data}
}

// queued returns what is waiting in a subscriber's queue, without blocking.
func queued(t *testing.T, ch chan []byte) []event {
	t.Helper()
	var out []event
	for {
		select {
		case frame, open := <-ch:
			if !open {
				return out
			}
			out = append(out, parseFrame(t, frame))
		default:
			return out
		}
	}
}

func (b *Bridge) cursor(seq uint64) string { return fmt.Sprintf("%s.%d", b.epoch, seq) }

func (b *Bridge) drop(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, open := b.subs[ch]; open {
		delete(b.subs, ch)
		close(ch)
	}
}

// Every event has the next sequence number, and the stream says how long to
// wait before reconnecting, once.
func TestEventsAreNumberedAndTheStreamSaysWhenToRetry(t *testing.T) {
	b := New(&target{})
	srv := httptest.NewServer(b.Handler("/bridge"))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/bridge/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	lines := bufio.NewScanner(resp.Body)
	var head []string
	for lines.Scan() && lines.Text() != "" {
		head = append(head, lines.Text())
	}
	if strings.Join(head, "|") != ": connected|retry: 1000" {
		t.Fatalf("the stream opened with %q", head)
	}
	b.Emit("a", 1)
	b.Emit("b", 2)
	var ids []string
	for lines.Scan() && len(ids) < 2 {
		if id, ok := strings.CutPrefix(lines.Text(), "id: "); ok {
			ids = append(ids, id)
		}
		if strings.HasPrefix(lines.Text(), "retry:") {
			t.Fatal("the retry field was sent again")
		}
	}
	if len(ids) != 2 || ids[0] != b.epoch+".1" || ids[1] != b.epoch+".2" {
		t.Fatalf("the ids were %q, want %s.1 and %s.2", ids, b.epoch, b.epoch)
	}
}

// A page that reconnects while events are still being emitted receives each
// event after its cursor exactly once and in order, including when its
// stream is closed for being slow and it reconnects again. The emitter and
// the subscriber race on purpose, so this fails if numbering, storing and
// queuing an event, or registering a subscriber with its replay, are not
// each done under one lock.
func TestReplayHasNoGapAndNoRepeatWhileEmitting(t *testing.T) {
	const total = 3000
	for round := 0; round < 25; round++ {
		b := New(&target{})
		for i := 0; i < 500; i++ {
			b.Emit("n", i)
		}
		start := uint64(100 + round*10)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 500; i < total; i++ {
				b.Emit("n", i)
			}
		}()
		sub := b.subscribe(b.cursor(start))
		next := start + 1
		for next <= total {
			select {
			case frame, open := <-sub:
				if !open {
					// Closed for being slow: come back with the cursor.
					sub = b.subscribe(b.cursor(next - 1))
					continue
				}
				e := parseFrame(t, frame)
				if e.name == ResyncEvent {
					t.Fatalf("round %d: a resync though the ring reaches the cursor", round)
				}
				if e.seq != next || string(e.data) != fmt.Sprint(e.seq-1) {
					t.Fatalf("round %d: expected event %d, got %d with data %s", round, next, e.seq, e.data)
				}
				next++
			case <-time.After(10 * time.Second):
				t.Fatalf("round %d: stuck waiting for event %d", round, next)
			}
		}
		<-done
		b.drop(sub)
	}
}

// With a ring of 5, a cursor at the highest dropped event is replayed from,
// and one before it is told to resync. The resync has a number of its own, and
// a page that reconnects with it continues without another.
func TestACursorOlderThanTheRingGetsAResync(t *testing.T) {
	b := New(&target{})
	b.ReplayEvents = 5
	for i := 1; i <= 20; i++ {
		b.Emit("n", i)
	}
	// The ring holds 16 to 20, and 15 is the highest it dropped.
	got := queued(t, b.subscribe(b.cursor(15)))
	if len(got) != 5 || got[0].seq != 16 || got[4].seq != 20 {
		t.Fatalf("a cursor at the edge: %+v", got)
	}
	got = queued(t, b.subscribe(b.cursor(14)))
	if len(got) != 1 || got[0].name != ResyncEvent || string(got[0].data) != "null" || got[0].seq != 21 {
		t.Fatalf("a cursor before the ring: %+v", got)
	}
	// The resync's number is its own: the next event follows it.
	b.Emit("n", 21)
	if got = queued(t, b.subscribe(b.cursor(21))); len(got) != 1 || got[0].seq != 22 {
		t.Fatalf("after a resync's id: %+v", got)
	}
}

func TestOtherCursors(t *testing.T) {
	b := New(&target{})
	for i := 1; i <= 3; i++ {
		b.Emit("n", i)
	}
	for _, tc := range []struct {
		label, cursor string
		resync        bool
		events        int
	}{
		{"no cursor", "", false, 0},
		{"the latest event", b.cursor(3), false, 0},
		{"the one before", b.cursor(2), false, 1},
		{"another epoch", "other.2", true, 0},
		{"not a cursor", "garbage", true, 0},
		{"no sequence", b.epoch + ".", true, 0},
		{"a sequence that is not a number", b.epoch + ".x", true, 0},
		{"ahead of the numbering", b.cursor(999), true, 0},
	} {
		got := queued(t, b.subscribe(tc.cursor))
		isResync := len(got) > 0 && got[0].name == ResyncEvent
		if isResync != tc.resync {
			t.Errorf("%s: resync = %v, want %v (%+v)", tc.label, isResync, tc.resync, got)
		}
		if !tc.resync && len(got) != tc.events {
			t.Errorf("%s: %d events replayed, want %d", tc.label, len(got), tc.events)
		}
	}
}

// The ring is bounded by count and by size, whichever is reached first, and
// an event bigger than the size bound is not kept.
func TestRingBounds(t *testing.T) {
	b := New(&target{})
	b.ReplayEvents = 3
	for i := 1; i <= 10; i++ {
		b.Emit("n", i)
	}
	if len(b.ring) != 3 || b.ring[0].seq != 8 || b.floor != 7 {
		t.Fatalf("by count: %d kept from %d, floor %d", len(b.ring), b.ring[0].seq, b.floor)
	}

	b = New(&target{})
	b.Emit("n", 1)
	one := len(b.ring[0].frame)
	// Events 1 to 9 have frames of one size, and event 10 is two bytes
	// longer (a digit in the id, a digit in the data), so room for events
	// 9 and 10 and nothing more is 2*one+2.
	b = New(&target{})
	b.ReplayBytes = 2*one + 2
	for i := 1; i <= 10; i++ {
		b.Emit("n", i)
	}
	if len(b.ring) != 2 || b.ring[0].seq != 9 || b.ringBytes > b.ReplayBytes || b.floor != 8 {
		t.Fatalf("by size: %d kept from %d, %d bytes of %d, floor %d", len(b.ring), b.ring[0].seq, b.ringBytes, b.ReplayBytes, b.floor)
	}

	b = New(&target{})
	b.ReplayBytes = 10
	b.Emit("n", "much larger than the bound")
	if len(b.ring) != 0 || b.ringBytes != 0 || b.floor != 1 {
		t.Fatalf("an oversize event was kept: %d kept, %d bytes, floor %d", len(b.ring), b.ringBytes, b.floor)
	}
	if got := queued(t, b.subscribe(b.cursor(0))); len(got) != 1 || got[0].name != ResyncEvent {
		t.Fatalf("a page that missed the dropped event: %+v", got)
	}
}

func TestRingDefaults(t *testing.T) {
	if DefaultReplayEvents != 8192 || DefaultReplayBytes != 16<<20 {
		t.Fatalf("the defaults are %d events and %d bytes", DefaultReplayEvents, DefaultReplayBytes)
	}
	b := New(&target{})
	if events, bytes := b.limits(); events != DefaultReplayEvents || bytes != DefaultReplayBytes {
		t.Fatalf("zero options give %d and %d", events, bytes)
	}
	for i := 0; i < DefaultReplayEvents+100; i++ {
		b.Emit("n", i)
	}
	if len(b.ring) != DefaultReplayEvents {
		t.Fatalf("the default ring holds %d events", len(b.ring))
	}
}

// A subscriber that falls more than its queue behind is closed, and when it
// comes back with the cursor of the last event it got, it is replayed
// exactly the events it missed.
func TestASlowSubscriberIsClosedThenReplayed(t *testing.T) {
	b := New(&target{})
	slow := b.subscribe("")
	for i := 1; i <= 300; i++ {
		b.Emit("n", i)
	}
	got := queued(t, slow)
	if len(got) != subscriberQueue || got[len(got)-1].seq != subscriberQueue {
		t.Fatalf("the slow subscriber got %d events", len(got))
	}
	if _, open := <-slow; open {
		t.Fatal("the slow subscriber's stream was left open")
	}
	b.mu.Lock()
	left := len(b.subs)
	b.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d subscribers are still registered", left)
	}
	again := queued(t, b.subscribe(b.cursor(got[len(got)-1].seq)))
	if len(again) != 300-subscriberQueue || again[0].seq != subscriberQueue+1 || again[len(again)-1].seq != 300 {
		t.Fatalf("the replay after reconnecting: %d events, from %d", len(again), again[0].seq)
	}
	for i, e := range again {
		if e.seq != uint64(subscriberQueue+1+i) {
			t.Fatalf("the replay skipped or repeated an event at %d: %d", i, e.seq)
		}
	}
}

// A replay larger than the usual queue still leaves the new stream room for
// live events.
func TestAReplayDoesNotUseUpTheLiveQueue(t *testing.T) {
	b := New(&target{})
	for i := 1; i <= 1000; i++ {
		b.Emit("n", i)
	}
	sub := b.subscribe(b.cursor(0))
	for i := 1001; i <= 1000+subscriberQueue; i++ {
		b.Emit("n", i)
	}
	got := queued(t, sub)
	if len(got) != 1000+subscriberQueue {
		t.Fatalf("the stream got %d events, and was closed if fewer than %d", len(got), 1000+subscriberQueue)
	}
}

// Over HTTP, the Last-Event-ID header sets the cursor, the after parameter
// does the same for a client that is not a browser, and the header wins.
func TestTheCursorComesFromTheHeaderOrTheAfterParameter(t *testing.T) {
	b := New(&target{})
	for i := 1; i <= 3; i++ {
		b.Emit("n", i)
	}
	srv := httptest.NewServer(b.Handler("/bridge"))
	defer srv.Close()
	read := func(header, query string, want int) []event {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/bridge/events"+query, nil)
		if header != "" {
			req.Header.Set("Last-Event-ID", header)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out []event
		lines := bufio.NewScanner(resp.Body)
		var id string
		for len(out) < want && lines.Scan() {
			text := lines.Text()
			if v, ok := strings.CutPrefix(text, "id: "); ok {
				id = v
			}
			if v, ok := strings.CutPrefix(text, "data: "); ok {
				out = append(out, parseFrame(t, []byte("id: "+id+"\ndata: "+v+"\n\n")))
			}
		}
		return out
	}
	if got := read(b.cursor(1), "", 2); len(got) != 2 || got[0].seq != 2 || got[1].seq != 3 {
		t.Fatalf("by header: %+v", got)
	}
	if got := read("", "?after="+b.cursor(2), 1); len(got) != 1 || got[0].seq != 3 {
		t.Fatalf("by parameter: %+v", got)
	}
	// A parameter the bridge cannot use does not override a header it can.
	if got := read(b.cursor(1), "?after=other.9", 2); len(got) != 2 || got[0].name == ResyncEvent || got[0].seq != 2 {
		t.Fatalf("the header did not win: %+v", got)
	}
	if got := read("other.9", "?after="+b.cursor(2), 1); len(got) != 1 || got[0].name != ResyncEvent {
		t.Fatalf("the header did not win over a good parameter: %+v", got)
	}
	if got := read("", "?after=other.9", 1); len(got) != 1 || got[0].name != ResyncEvent {
		t.Fatalf("another epoch by parameter: %+v", got)
	}
}
