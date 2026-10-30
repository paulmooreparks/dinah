package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type target struct{ got []string }

func (t *target) Nothing()                                  {}
func (t *target) Fails() error                              { return errors.New("it failed") }
func (t *target) False() bool                               { return false }
func (t *target) Add(a, b int) (int, error)                 { return a + b, nil }
func (t *target) Echo(ctx context.Context, s string) string { return s }
func (t *target) Struct(v struct{ N int }) int              { return v.N }
func (t *target) Watch() chan int                           { return nil }
func (t *target) Three() (int, int, error)                  { return 0, 0, nil }
func (t *target) unexported()                               {}

func post(t *testing.T, h http.Handler, method string, args ...any) answer {
	t.Helper()
	raw := make([]json.RawMessage, len(args))
	for i, a := range args {
		raw[i], _ = json.Marshal(a)
	}
	body, _ := json.Marshal(call{Method: method, Args: raw})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/bridge/call", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", method, rec.Code, rec.Body)
	}
	var a answer
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestOnlyBindableMethodsAreOffered(t *testing.T) {
	got := strings.Join(New(&target{}).Methods(), ",")
	if got != "Add,Echo,Fails,False,Nothing,Struct" {
		t.Fatalf("offered %s", got)
	}
}

func TestCallsAnswerAsAWailsBindingWould(t *testing.T) {
	h := New(&target{}).Handler("/bridge")
	if a := post(t, h, "Add", 2, 3); string(a.Result) != "5" || a.Error != "" {
		t.Fatalf("Add: %+v", a)
	}
	// A result the page tests for falsehood must still arrive.
	if a := post(t, h, "False"); string(a.Result) != "false" {
		t.Fatalf("False: %+v", a)
	}
	if a := post(t, h, "Fails"); a.Error != "it failed" {
		t.Fatalf("Fails: %+v", a)
	}
	if a := post(t, h, "Echo", "hi"); string(a.Result) != `"hi"` {
		t.Fatalf("Echo: %+v", a)
	}
	// An argument the page leaves out is the zero value.
	if a := post(t, h, "Add", 4); string(a.Result) != "4" {
		t.Fatalf("Add with one argument: %+v", a)
	}
	if a := post(t, h, "Struct", map[string]int{"N": 7}); string(a.Result) != "7" {
		t.Fatalf("Struct: %+v", a)
	}
	if a := post(t, h, "Add", 1, 2, 3); !strings.Contains(a.Error, "takes 2 arguments") {
		t.Fatalf("too many arguments: %+v", a)
	}
	if a := post(t, h, "Watch"); !strings.Contains(a.Error, "no method") {
		t.Fatalf("Watch: %+v", a)
	}
}

func TestRuntimeFunctions(t *testing.T) {
	b := New(&target{})
	var opened string
	b.Runtime("BrowserOpenURL", func(_ context.Context, args []json.RawMessage) error {
		return json.Unmarshal(args[0], &opened)
	})
	h := b.Handler("/bridge")
	if a := post(t, h, "runtime.BrowserOpenURL", "https://example.com"); a.Error != "" || opened != "https://example.com" {
		t.Fatalf("BrowserOpenURL: %+v, opened %q", a, opened)
	}
	if a := post(t, h, "runtime.Quit"); !strings.Contains(a.Error, "does not offer") {
		t.Fatalf("Quit: %+v", a)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/bridge/app.js", nil))
	s := rec.Body.String()
	if !strings.Contains(s, `"/bridge"`) || !strings.Contains(s, `["Add","Echo","Fails","False","Nothing","Struct"]`) || !strings.Contains(s, `["BrowserOpenURL"]`) {
		t.Fatalf("the script is %s", s)
	}
}

func TestEventsReachAListeningPage(t *testing.T) {
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
	lines.Scan() // ": connected"
	b.Emit("agent:update", map[string]string{"sessionId": "s1"})
	for lines.Scan() {
		if line, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
			if line != `{"name":"agent:update","data":{"sessionId":"s1"}}` {
				t.Fatalf("the event arrived as %s", line)
			}
			return
		}
	}
	t.Fatal("no event arrived")
}

func TestCallObserverFinishesSuccessfulAndFailedCalls(t *testing.T) {
	b := New(&target{})
	active, completed := 0, 0
	b.ObserveCall = func(name string) func() { active++; return func() { active--; completed++ } }
	b.Call(context.Background(), "Nothing", nil)
	b.Call(context.Background(), "Fails", nil)
	if active != 0 || completed != 2 {
		t.Fatal("observer did not finish both calls")
	}
	b.Call(context.Background(), "Add", []json.RawMessage{json.RawMessage(`"invalid"`)})
	if completed != 2 {
		t.Fatal("invalid call was reported as executing")
	}
}

type detailedError struct{}

func (detailedError) Error() string    { return "the choice is not offered" }
func (detailedError) ErrorDetail() any { return map[string][]string{"offered": {"a", "b"}} }

type detailTarget struct{}

func (detailTarget) Refuse() error { return fmt.Errorf("open: %w", detailedError{}) }

// An error with a detail gives the page its detail beside its message, even
// when it is wrapped, and the script rejects with an Error that contains it.
func TestDetailedErrorsGiveThePageTheirDetail(t *testing.T) {
	h := New(detailTarget{}).Handler("/bridge")
	a := post(t, h, "Refuse")
	if a.Error != "open: the choice is not offered" || string(a.Detail) != `{"offered":["a","b"]}` {
		t.Fatalf("Refuse: %+v", a)
	}
	if plain := post(t, New(&target{}).Handler("/bridge"), "Fails"); plain.Detail != nil {
		t.Fatalf("a plain error has a detail: %+v", plain)
	}
	if !strings.Contains(script, "e.detail = a.detail") {
		t.Fatal("the page's script drops the detail")
	}
}

func TestRestrictedBridgeUsesPolicyForBindingsAndCalls(t *testing.T) {
	policy := map[string]bool{"Add": true, "Fails": false}
	b, err := NewRestricted(&target{}, policy)
	if err != nil {
		t.Fatal(err)
	}
	// Later edits to the caller's policy cannot change the constructed bridge.
	policy["Nothing"] = true
	if got := strings.Join(b.Methods(), ","); got != "Add" {
		t.Fatalf("offered %s", got)
	}
	h := b.Handler("/bridge")
	script := httptest.NewRecorder()
	h.ServeHTTP(script, httptest.NewRequest("GET", "/bridge/app.js", nil))
	if !strings.Contains(script.Body.String(), `methods = ["Add"]`) {
		t.Fatal(script.Body.String())
	}
	if a := post(t, h, "Add", 2, 3); a.Error != "" || string(a.Result) != "5" {
		t.Fatalf("allowed call: %+v", a)
	}
	for _, name := range []string{"Fails", "Nothing"} {
		if _, err := b.Call(context.Background(), name, nil); err == nil {
			t.Errorf("direct call to %s succeeded", name)
		}
		if a := post(t, h, name); a.Error == "" {
			t.Errorf("HTTP call to %s succeeded", name)
		}
	}
	empty, err := NewRestricted(&target{}, nil)
	if err != nil || len(empty.Methods()) != 0 {
		t.Fatalf("empty policy: %v, %v", empty, err)
	}
}

func TestRestrictedBridgeRejectsInvalidEntries(t *testing.T) {
	for _, name := range []string{"Missing", "Watch", "Three", "unexported", ""} {
		for _, allowed := range []bool{false, true} {
			if b, err := NewRestricted(&target{}, map[string]bool{name: allowed}); err == nil || b != nil {
				t.Errorf("accepted invalid entry %q (%v): %v", name, allowed, err)
			}
		}
	}
	if _, err := NewRestricted(nil, nil); err == nil {
		t.Fatal("accepted nil target")
	}
}
