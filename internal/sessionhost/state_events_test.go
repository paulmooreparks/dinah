package sessionhost

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func stateConversation(id string) *Conversation {
	c := NewConversation(Identity{ID: id, Provider: "codex", Cwd: "."}, InitialActivity{Open: true}, false, false)
	c.StartWork()
	return c
}

// A state event's number orders the snapshots the way the owner's lock orders
// them, so the value an event carries is the value that number describes.
// Goroutines each add to a conversation's queue and emit it, and the emits are
// let through in reverse order of the changes; the events, put in order of
// their numbers, show the queue only growing. A number taken outside the
// lock, before the value is read, breaks this.
func TestAQueueEventsNumberDescribesItsValue(t *testing.T) {
	c := stateConversation("queue")
	const writers = 40
	type event struct {
		ver    uint64
		queued int
	}
	var mu sync.Mutex
	var events []event
	emit := func(name string, v any) {
		m := v.(map[string]any)
		mu.Lock()
		events = append(events, event{m["stateVer"].(uint64), len(m["queued"].([]string))})
		mu.Unlock()
	}
	// Each writer waits for the next one's emit before emitting its own, so
	// the emits come out in the opposite order to the changes.
	gates := make([]chan struct{}, writers+1)
	for i := range gates {
		gates[i] = make(chan struct{})
	}
	close(gates[writers])
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			c.QueueActive(Prompt{})
			// The emit's snapshot is taken now; the delivery is held back.
			var snapshot map[string]any
			EmitQueue(func(_ string, v any) { snapshot = v.(map[string]any) }, c)
			<-gates[i+1]
			emit(EventQueue, snapshot)
			close(gates[i])
		})
	}
	wg.Wait()
	sort.Slice(events, func(i, j int) bool { return events[i].ver < events[j].ver })
	for i := 1; i < len(events); i++ {
		if events[i].ver == events[i-1].ver {
			t.Fatalf("two events carry number %d", events[i].ver)
		}
		if events[i].queued < events[i-1].queued {
			t.Fatalf("number %d describes a queue of %d, and the earlier number %d one of %d", events[i].ver, events[i].queued, events[i-1].ver, events[i-1].queued)
		}
	}
	if len(events) != writers || events[len(events)-1].queued != writers {
		t.Fatalf("%d events, the last describing %d queued", len(events), events[len(events)-1].queued)
	}
}

// The number must come from the same critical section as the value. With the
// conversation's lock held, an emit started now has to wait for it; a
// snapshot taken meanwhile under the lock gets the next number and the value
// as it is then, and the emit that waited then gets a later number with the
// later value. Numbered before it waited, it would carry an older number with
// a newer value, and the page would drop the newer one.
func TestAnEmitWaitingForTheLockIsNumberedAfterTheSnapshotsTakenMeanwhile(t *testing.T) {
	c := stateConversation("wait")
	c.mu.Lock()
	got := make(chan map[string]any, 1)
	go EmitQueue(func(_ string, v any) { got <- v.(map[string]any) }, c)
	time.Sleep(200 * time.Millisecond)
	_, meanwhile := c.clock.Next()
	before := len(c.activity.QueueTextsLocked())
	c.activity.QueueActiveLocked(Prompt{})
	c.mu.Unlock()
	late := <-got
	lateVer, lateQueued := late["stateVer"].(uint64), len(late["queued"].([]string))
	if before != 0 || lateQueued != 1 {
		t.Fatalf("the test did not arrange its values: %d then %d", before, lateQueued)
	}
	if lateVer <= meanwhile {
		t.Fatalf("the emit that waited for the lock has number %d, not after the snapshot taken meanwhile (%d) that holds an older value", lateVer, meanwhile)
	}
}

// The same for a provider's state, which is stored and numbered under its
// state lock.
func TestAProviderStateIsNumberedInTheSameSectionThatStoresIt(t *testing.T) {
	l, _ := lifecycleFixture()
	got := make(chan map[string]any, 1)
	l.host.Emit = func(_ string, v any) { got <- v.(map[string]any) }
	l.stateMu.Lock()
	go l.state("codex", "a", "")
	time.Sleep(200 * time.Millisecond)
	l.states["codex"] = ProviderState{Provider: "codex", State: "b"}
	meanwhile := l.snapshotLocked("codex")
	l.stateMu.Unlock()
	late := <-got
	if late["state"] != "a" || late["stateVer"].(uint64) <= meanwhile.StateVer {
		t.Fatalf("the state stored last has number %v, not after %d, the number of the older state %q", late["stateVer"], meanwhile.StateVer, meanwhile.State)
	}
}

// The same for the information of a session, which the page is also given by
// calls: a later read never has a lower number, and a reread after an event
// has a higher one.
func TestSessionInformationNumbersOnlyGoUp(t *testing.T) {
	c := stateConversation("info")
	var told []Info
	EmitSession(func(_ string, v any) { told = append(told, v.(Info)) }, c)
	c.QueueActive(Prompt{})
	reread := c.InfoVersioned()
	if told[0].StateGen == "" || told[0].StateVer == 0 || reread.StateGen != told[0].StateGen || reread.StateVer <= told[0].StateVer {
		t.Fatalf("event %s/%d, reread %s/%d", told[0].StateGen, told[0].StateVer, reread.StateGen, reread.StateVer)
	}
	if len(reread.Queued) != 1 || len(told[0].Queued) != 0 {
		t.Fatalf("the snapshots do not describe their states: %v then %v", told[0].Queued, reread.Queued)
	}
	// Info, which code that compares information uses, carries no number.
	if plain := c.Info(); plain.StateGen != "" || plain.StateVer != 0 {
		t.Fatalf("Info is numbered: %+v", plain)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[uint64]bool{}
	for range 50 {
		wg.Go(func() {
			v := c.InfoVersioned().StateVer
			mu.Lock()
			defer mu.Unlock()
			if seen[v] {
				t.Errorf("number %d was given twice", v)
			}
			seen[v] = true
		})
	}
	wg.Wait()
}

// A conversation opened again has a generation of its own, which the page
// treats as a new owner whatever its number.
func TestAConversationOpenedAgainHasANewGeneration(t *testing.T) {
	first, second := stateConversation("same"), stateConversation("same")
	a, b := first.InfoVersioned(), second.InfoVersioned()
	if a.StateGen == "" || a.StateGen == b.StateGen {
		t.Fatalf("generations %q and %q", a.StateGen, b.StateGen)
	}
	if a.StateGen != a.StreamGen {
		t.Fatalf("the state generation %q is not the chat log's %q", a.StateGen, a.StreamGen)
	}
}

// Two state changes of one provider are stored in one order and numbered in
// that order, and the event with the higher number carries the state that is
// stored, whichever event is emitted last.
func TestProviderStateEventsAreNumberedWithTheStoredState(t *testing.T) {
	l, _ := lifecycleFixture()
	var mu sync.Mutex
	var told []map[string]any
	release := make(chan struct{})
	first := make(chan struct{})
	l.host.Emit = func(_ string, v any) {
		m := v.(map[string]any)
		if m["state"] == "a" {
			// The first change's event is held until the second has been told.
			close(first)
			<-release
		}
		mu.Lock()
		told = append(told, m)
		mu.Unlock()
	}
	done := make(chan struct{})
	go func() { l.state("codex", "a", ""); close(done) }()
	<-first
	l.state("codex", "b", "")
	close(release)
	<-done
	if len(told) != 2 || told[0]["state"] != "b" || told[1]["state"] != "a" {
		t.Fatalf("the events were not inverted as the test arranged: %v", told)
	}
	byVer := map[uint64]string{}
	for _, m := range told {
		byVer[m["stateVer"].(uint64)] = m["state"].(string)
	}
	if len(byVer) != 2 {
		t.Fatalf("the events share a number: %v", told)
	}
	highest := uint64(0)
	for v := range byVer {
		highest = max(highest, v)
	}
	stored := l.States()
	if len(stored) != 1 || byVer[highest] != stored[0].State {
		t.Fatalf("the event with the highest number says %q, the stored state is %q", byVer[highest], stored[0].State)
	}
	if stored[0].StateVer <= highest || stored[0].StateGen != told[0]["stateGen"] {
		t.Fatalf("a reread %s/%d is not newer than the events %v", stored[0].StateGen, stored[0].StateVer, highest)
	}
}

// The events that carry whole state are emitted only through the functions
// that number them. This fails when any other Go source names one of them or
// emits one by its constant.
func TestStateEventsHaveOneEmitPath(t *testing.T) {
	root := filepath.Join("..", "..")
	names := regexp.MustCompile(`"(agent:(session|queue|options|state|account)|settings:changed|update:state|judgements:changed)"`)
	constants := regexp.MustCompile(`(?i)emit\(\s*(sessionhost\.)?Event(Session|Queue|Options|State)\b`)
	allowed := map[string]bool{
		filepath.Join("internal", "sessionhost", "state_events.go"):      true,
		filepath.Join("cmd", "dinah-desktop", "provider_account.go"):     true,
		filepath.Join("cmd", "dinah-desktop", "applet_settings.go"):      true,
		filepath.Join("cmd", "dinah-desktop", "judgements.go"):           true,
		filepath.Join("cmd", "dinah-desktop", "update.go"):               true,
		filepath.Join("internal", "sessionhost", "state_events_test.go"): true,
	}
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
			if allowed[rel] {
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
		t.Fatalf("state events emitted outside the functions that number them:\n%s", strings.Join(found, "\n"))
	}
	if !names.MatchString(`a.emit("agent:session", x)`) || !constants.MatchString(`t.host.Emit(EventQueue, x)`) {
		t.Fatal("the scan's patterns do not match an emit")
	}
}
