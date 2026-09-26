package httphead

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/resident"
	"dinah/internal/resident/residenttest"
	"dinah/internal/verb"
)

// residentHandle is what a resident fixture keeps beside the head: the
// resident, the notifier the test drives, and the publishes it has seen.
type residentHandle struct {
	w         *resident.Workbench
	manual    *residenttest.Manual
	published chan resident.Published
	passed    *pathLog
}

// pathLog collects the paths a hook reports.
type pathLog struct {
	mu    sync.Mutex
	paths []string
}

func (p *pathLog) add(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paths = append(p.paths, path)
}

func (p *pathLog) take() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	taken := p.paths
	p.paths = nil
	return taken
}

// withResident is an option that populates the workbench through a library
// of its own, then opens a resident over it with a hand-driven notifier,
// waits for the first snapshot and serves it.
func withResident(t *testing.T, handle *residentHandle, populate func(l *verb.Library)) func(*Config) {
	return func(cfg *Config) {
		t.Helper()
		if populate != nil {
			opened, err := bench.Open(cfg.Root)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			populate(verb.New(opened, cfg.Home))
		}
		handle.manual = residenttest.NewManual()
		handle.published = make(chan resident.Published, 64)
		handle.passed = &pathLog{}
		hooks := &resident.Hooks{
			AfterPublish: func(p resident.Published) {
				select {
				case handle.published <- p:
				default:
				}
			},
			PassThrough: handle.passed.add,
		}
		w, err := resident.Open(cfg.Root, resident.Options{Notifier: handle.manual, Hooks: hooks})
		if err != nil {
			t.Fatalf("open the resident: %v", err)
		}
		t.Cleanup(func() { w.Close() })
		select {
		case <-w.Ready():
		case <-time.After(30 * time.Second):
			t.Fatal("the resident's first build did not finish within 30 seconds")
		}
		handle.w = w
		cfg.Resident = w
	}
}

// commandLogger records the commands TimeRead saw, in order.
type commandLogger struct {
	mu       sync.Mutex
	commands []string
}

func (c *commandLogger) add(command string, _ time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commands = append(c.commands, command)
}

func (c *commandLogger) take() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	taken := c.commands
	c.commands = nil
	return taken
}

// sourceLogger records what observeSource reported.
type sourceLogger struct {
	mu      sync.Mutex
	choices []bool
}

func (s *sourceLogger) add(fromResident bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.choices = append(s.choices, fromResident)
}

func (s *sourceLogger) take() []bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	taken := s.choices
	s.choices = nil
	return taken
}

// TestTimeReadSeesEveryLibraryCall is part of dinah-619/criteria/14. For the
// HTML card page with no window open, TimeRead sees the library acquisition
// and every library call the head makes, in order, on the resident and on the
// disk path. The disk path opens the workbench twice, once for the page's own
// reads and once for the route's, as it always has, and each open is timed;
// the resident path acquires its one library once. The page makes no show of
// its own when no window is open, so the route's show is the only one. The
// 10ms criterion is only as good as this test, because a TimeRead that
// skipped a call would meet the number for free.
//
// Arming: removing the timing of open from open and contextLibrary drops the
// opens from both sequences, and removing it from readFor drops changes,
// status, tree and instructions.
func TestTimeReadSeesEveryLibraryCall(t *testing.T) {
	cases := []struct {
		name     string
		resident bool
		want     []string
	}{
		{"resident", true, []string{"open", "changes", "status", "tree", "show", "instructions"}},
		{"disk", false, []string{"open", "changes", "status", "tree", "open", "show", "instructions"}},
	}
	for _, c := range cases {
		logger := &commandLogger{}
		sources := &sourceLogger{}
		handle := &residentHandle{}
		options := []func(*Config){func(cfg *Config) {
			cfg.TimeRead = logger.add
			cfg.observeSource = sources.add
		}}
		var card string
		populate := func(l *verb.Library) {
			response := l.Add(&verb.Request{Verb: "add", Actor: "alka", Title: "A card", Column: "build"})
			if response.Card == nil {
				t.Fatalf("add: %+v", response)
			}
			card = response.Card.Ref
		}
		if c.resident {
			options = append(options, withResident(t, handle, populate))
		}
		f := newFixture(t, options...)
		if !c.resident {
			card = f.add("A card", "build")
		}
		logger.take()
		sources.take()
		answered := f.page("/cards/" + card)
		if answered.status != 200 {
			t.Fatalf("%s: GET the card page answered %d", c.name, answered.status)
		}
		if got := logger.take(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: TimeRead saw %v, wanted %v", c.name, got, c.want)
		}
		if got := sources.take(); len(got) != 1 || got[0] != c.resident {
			t.Errorf("%s: observeSource reported %v, wanted one choice of %v", c.name, got, c.resident)
		}
	}
}
