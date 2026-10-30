package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/applet"
)

const quickRef = "test.quick/echo@1"

// quickRegistry has one pure action that returns its input at once, like the
// editor's-flags check, which only parses text.
func quickRegistry(tb testing.TB) *applet.Registry {
	tb.Helper()
	schema := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"value":{"type":"integer"}}}`)
	r, err := applet.New(applet.Bundle{
		Manifest: applet.Manifest{ID: "test.quick", Version: "1.0.0", APIVersion: 1, Title: "Quick", Actions: []applet.Action{
			{Ref: quickRef, Title: "Echo", Replay: "pure", Capabilities: []string{}, InputSchema: schema, OutputSchema: schema},
		}},
		Handlers: map[string]applet.Handler{quickRef: func(ctx context.Context, input string) (applet.Result, error) {
			return applet.Result{OutputJSON: input}, nil
		}},
		Sources: map[string][]byte{"test": []byte("test")},
	})
	if err != nil {
		tb.Fatal(err)
	}
	return r
}

// An invocation that holds the broker while it tells of a change, as it
// holds it while it reads and writes its record, does not keep another
// invocation from starting and finishing. The first invocation's change
// handler runs a second invocation and waits for it, which cannot finish
// if one lock covers every invocation's record work.
func TestInvocationsDoNotWaitOnEachOther(t *testing.T) {
	state, ws := t.TempDir(), t.TempDir()
	b := &Broker{Registry: quickRegistry(t), State: state}
	nested := make(chan error, 1)
	var once sync.Once
	b.Emit = func(event string, data any) {
		v, ok := data.(Invocation)
		if event != "applet:invocation" || !ok || v.ID != "first" {
			return
		}
		once.Do(func() {
			done := make(chan error, 1)
			go func() {
				_, err := b.Run(context.Background(), ws, "chat", "other-run", "second", quickRef, `{"value":2}`, "", time.Now().Add(time.Minute))
				done <- err
			}()
			select {
			case err := <-done:
				nested <- err
			case <-time.After(5 * time.Second):
				nested <- fmt.Errorf("the second invocation waited on the first one's record work")
			}
		})
	}
	out, err := b.Run(context.Background(), ws, "chat", "run", "first", quickRef, `{"value":1}`, "", time.Now().Add(time.Minute))
	if err != nil || out != `{"value":1}` {
		t.Fatalf("first: %q %v", out, err)
	}
	select {
	case err := <-nested:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the second invocation never finished")
	}
}

// seed saves finished invocations of about the size seen in a real state
// folder, in a workspace of their own, for Attention to read.
func seed(tb testing.TB, b *Broker, count int) {
	tb.Helper()
	ws := tb.TempDir()
	digest, _ := b.Registry.Digest(quickRef)
	padding, _ := json.Marshal(map[string]string{"text": strings.Repeat("x", 6000)})
	for i := range count {
		v := Invocation{ID: fmt.Sprintf("seed-%d", i), Workspace: ws, OwnerKind: "chat", OwnerID: "old-run", Action: quickRef, Digest: digest, InputJSON: `{"value":1}`, OutputJSON: string(padding), Status: "done", Deadline: time.Now().Add(time.Hour), Created: time.Now()}
		if err := b.Save(&v); err != nil {
			tb.Fatal(err)
		}
	}
}

// BenchmarkAttention reads the attention list of a state folder holding as
// many finished invocations as a real one did.
func BenchmarkAttention(b *testing.B) {
	br := &Broker{Registry: quickRegistry(b), State: b.TempDir()}
	seed(b, br, 600)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := br.Attention(func(Summary) bool { return true }); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConcurrentRuns runs a quick pure action from several flow runs at
// once and reports how long each invocation takes. With "scan", a stand-in
// for the app's Notifications reads both attention lists after every change,
// as the front end does.
func BenchmarkConcurrentRuns(b *testing.B) {
	for _, scan := range []bool{false, true} {
		for _, n := range []int{1, 4, 8} {
			b.Run(fmt.Sprintf("scan=%v/runs=%d", scan, n), func(b *testing.B) {
				br := &Broker{Registry: quickRegistry(b), State: b.TempDir()}
				ws := b.TempDir()
				stop := make(chan struct{})
				var scanner sync.WaitGroup
				if scan {
					seed(b, br, 600)
					changed := make(chan struct{}, 1)
					br.Emit = func(event string, _ any) {
						select {
						case changed <- struct{}{}:
						default:
						}
					}
					scanner.Add(1)
					go func() {
						defer scanner.Done()
						for {
							select {
							case <-stop:
								return
							case <-changed:
							}
							var both sync.WaitGroup
							for range 2 {
								both.Add(1)
								go func() { defer both.Done(); _, _ = br.Attention(func(Summary) bool { return true }) }()
							}
							both.Wait()
						}
					}()
				}
				var mu sync.Mutex
				var took []time.Duration
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					var wg sync.WaitGroup
					for r := range n {
						wg.Add(1)
						go func() {
							defer wg.Done()
							start := time.Now()
							_, err := br.Run(context.Background(), ws, "chat", fmt.Sprintf("run-%d", r), fmt.Sprintf("i-%d-%d", i, r), quickRef, `{"value":1}`, "", time.Now().Add(time.Minute))
							if err != nil {
								b.Error(err)
							}
							mu.Lock()
							took = append(took, time.Since(start))
							mu.Unlock()
						}()
					}
					wg.Wait()
				}
				b.StopTimer()
				close(stop)
				scanner.Wait()
				sort.Slice(took, func(i, j int) bool { return took[i] < took[j] })
				var sum time.Duration
				for _, d := range took {
					sum += d
				}
				b.ReportMetric(float64(sum.Microseconds())/float64(len(took))/1000, "ms/invocation")
				b.ReportMetric(float64(took[len(took)*95/100].Microseconds())/1000, "p95-ms")
				b.ReportMetric(float64(took[len(took)-1].Microseconds())/1000, "max-ms")
			})
		}
	}
}
