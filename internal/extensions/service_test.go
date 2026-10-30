package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	extension "github.com/paulmooreparks/dinah-extension-go"
)

func TestPreparedExtensionCommandsAreBoundedAndExpire(t *testing.T) {
	a := NewService(t.TempDir(), Hooks{}, func(string, any) {})
	tickets := []string{}
	a.emit = func(name string, value any) {
		if name == "extension:terminal" {
			tickets = append(tickets, value.(map[string]any)["ticket"].(string))
		}
	}
	for i := 0; i < 65; i++ {
		a.prepareExtensionTerminal(extension.TerminalInput{Program: "git", Args: []string{"status"}})
	}
	if len(a.preparedExtensionTerminals) != 64 {
		t.Fatal("unbounded prepared terminal tickets")
	}
	evicted := ""
	for _, ticket := range tickets {
		if _, ok := a.preparedExtensionTerminals[ticket]; !ok {
			evicted = ticket
			break
		}
	}
	if evicted == "" {
		t.Fatal("no ticket was evicted")
	}
	if _, err := a.TakeTerminal(evicted); err == nil {
		t.Fatal("evicted ticket launched")
	}
	a.preparedExtensionTerminals["expired"] = preparedExtensionTerminal{Created: time.Now().Add(-16 * time.Minute)}
	if _, err := a.TakeTerminal("expired"); err == nil {
		t.Fatal("expired ticket launched")
	}
}

// A page that missed extension:review events reads the reviews still waiting,
// each as its event carried it and oldest first; one the person has answered
// is no longer pending, and neither is one whose action ended.
func TestPendingReviewsAreWhatTheEventCarried(t *testing.T) {
	s := NewService(t.TempDir(), Hooks{}, func(string, any) {})
	var mu sync.Mutex
	var events []map[string]any
	emitted := make(chan struct{}, 2)
	s.emit = func(name string, value any) {
		if name == "extension:review" {
			mu.Lock()
			events = append(events, value.(map[string]any))
			mu.Unlock()
			emitted <- struct{}{}
		}
	}
	if got := s.PendingReviews(); len(got) != 0 {
		t.Fatalf("pending before any review: %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for _, program := range []string{"git", "go"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Hooks.Review(ctx, "ext-"+program, extension.RunInput{Program: program, Args: []string{"status"}})
		}()
		<-emitted
	}
	// The event is a map and the pending list a struct, so their key order
	// differs; what the page reads is the same.
	var got, want any
	pending, _ := json.Marshal(s.PendingReviews())
	sent, _ := json.Marshal(events)
	json.Unmarshal(pending, &got)
	json.Unmarshal(sent, &want)
	if !reflect.DeepEqual(got, want) || len(events) != 2 {
		t.Fatalf("pending %s differs from the events %s", pending, sent)
	}
	first := events[0]["key"].(string)
	if err := s.AnswerCommand(first, "cancel", "[]"); err != nil {
		t.Fatal(err)
	}
	// Answered, and gone from the list at once, whether or not the action
	// has finished yet.
	left := s.PendingReviews()
	if len(left) != 1 || left[0].Key != events[1]["key"] || left[0].Extension != "ext-go" {
		t.Fatalf("after answering the first: %+v", left)
	}
	cancel()
	wg.Wait()
	if got := s.PendingReviews(); len(got) != 0 {
		t.Fatalf("pending after every review ended: %+v", got)
	}
}

func TestReviewPreservesBytesAndCallbackOrder(t *testing.T) {
	for _, decision := range []string{"run", "terminal", "cancel-context"} {
		t.Run(decision, func(t *testing.T) {
			ready := make(chan string, 1)
			events := []string{}
			s := NewService(t.TempDir(), Hooks{}, func(string, any) {})
			s.emit = func(name string, value any) {
				if !s.mu.TryLock() {
					t.Error("callback held pending-state lock")
				} else {
					s.mu.Unlock()
				}
				events = append(events, name)
				if name == "extension:review" {
					ready <- value.(map[string]any)["key"].(string)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			original := string([]byte{'a', 0xff, 'b'})
			type result struct {
				args []string
				err  error
			}
			done := make(chan result, 1)
			go func() {
				args, err := s.Hooks.Review(ctx, "test", extension.RunInput{Program: "git", Args: []string{original}})
				done <- result{args, err}
			}()
			var key string
			select {
			case key = <-ready:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if decision == "cancel-context" {
				cancel()
			} else {
				arguments, _ := json.Marshal([]string{original})
				if err := s.AnswerCommand(key, decision, string(arguments)); err != nil {
					t.Fatal(err)
				}
			}
			var got result
			select {
			case got = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("review did not finish")
			}
			if decision == "run" && (got.err != nil || len(got.args) != 1 || got.args[0] != original) {
				t.Fatalf("byte preservation: %+v", got)
			}
			if decision == "cancel-context" && !errors.Is(got.err, context.Canceled) {
				t.Fatalf("cancellation: %v", got.err)
			}
			want := "extension:review,extension:review-ended"
			if decision == "terminal" {
				want = "extension:review,extension:terminal,extension:review-ended"
				if got.err == nil {
					t.Fatal("terminal diversion ran the action")
				}
				for key := range s.preparedExtensionTerminals {
					input, err := s.TakeTerminal(key)
					if err != nil || input.Args[0] != original {
						t.Fatalf("terminal arguments: %+v %v", input, err)
					}
				}
			}
			if strings.Join(events, ",") != want {
				t.Fatalf("events: %v", events)
			}
			if len(s.extensionReviews) != 0 {
				t.Fatal("review remained pending")
			}
		})
	}
}

func TestPreparedTerminalConsumedOnlyOnce(t *testing.T) {
	s := NewService(t.TempDir(), Hooks{}, func(string, any) {})
	s.preparedExtensionTerminals = map[string]preparedExtensionTerminal{"ticket": {Created: time.Now()}}
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.TakeTerminal("ticket"); results <- err }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("ticket consumed %d times", successes)
	}
}

func TestReviewSettingDoesNotInvalidateCatalog(t *testing.T) {
	changed, emitted := 0, 0
	s := NewService(t.TempDir(), Hooks{Changed: func() { changed++ }}, func(name string, _ any) {
		if name == "extensions:changed" {
			emitted++
		}
	})
	signed := installFixture(t, s.Manager, helperManifest())
	if err := s.SetCommandReview(signed.ID, true); err != nil {
		t.Fatal(err)
	}
	if changed != 0 || emitted != 1 {
		t.Fatalf("review invalidation: %d, events: %d", changed, emitted)
	}
	if err := s.Enable(signed.ID, false); err != nil {
		t.Fatal(err)
	}
	// Enable invalidates explicitly and again when its existing pause lease resumes.
	if changed != 2 || emitted != 1 {
		t.Fatal("full catalog change did not use its distinct callback")
	}
}
