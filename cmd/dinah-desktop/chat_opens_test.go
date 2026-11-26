package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"studio/internal/sessionhost"
)

// sentEvents collects the events the App sends the page.
type sentEvents struct {
	mu   sync.Mutex
	all  []string
	open chan ChatOpenOutcome
}

func watchEvents(a *App) *sentEvents {
	e := &sentEvents{open: make(chan ChatOpenOutcome, 32)}
	a.sink = func(name string, data any) {
		e.mu.Lock()
		e.all = append(e.all, name)
		e.mu.Unlock()
		if name == chatOpenEvent {
			e.open <- data.(ChatOpenOutcome)
		}
	}
	return e
}

func (e *sentEvents) count(name string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, sent := range e.all {
		if sent == name {
			n++
		}
	}
	return n
}

func (e *sentEvents) outcome(t *testing.T) ChatOpenOutcome {
	t.Helper()
	select {
	case o := <-e.open:
		return o
	case <-t.Context().Done():
		t.Fatal("no outcome of the open reached the page")
		return ChatOpenOutcome{}
	}
}

// heldLoads makes the test agent keep every session/load unanswered until
// the returned function is called, which a cleanup also does.
func heldLoads(t *testing.T, c *testProviderConnection) (release func()) {
	t.Helper()
	gate := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(gate) }) }
	c.onLoad = func() { <-gate }
	t.Cleanup(release)
	return release
}

// until waits for a condition that another goroutine brings about.
func until(t *testing.T, what string, met func() bool) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		if met() {
			return
		}
	}
	t.Fatalf("timed out waiting until %s", what)
}

func rawArgs(t *testing.T, values ...any) []json.RawMessage {
	t.Helper()
	out := make([]json.RawMessage, 0, len(values))
	for _, v := range values {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, raw)
	}
	return out
}

// The wake scheduler forgets a connect it asked for after its patience, so a
// load must fail inside it for the failure to be reported as that connect's.
// The requests that restore the chat's settings follow the load and have one
// limit for all of them, so the open's bounded steps are the grace, the load
// and the restore, and their sum has to be inside the patience.
func TestTheReopenLimitFitsTheWakePatience(t *testing.T) {
	if sum := connectGraceDefault + sessionhost.ReopenLoadLimit + sessionhost.RestoreSettingsLimit; sum >= connectPatience {
		t.Fatalf("a grace of %s, a load of %s and a restore of %s make %s, which is not inside the wake patience of %s", connectGraceDefault, sessionhost.ReopenLoadLimit, sessionhost.RestoreSettingsLimit, sum, connectPatience)
	}
	if sessionhost.ReopenLoadLimit <= reopenPatience {
		t.Fatalf("the load limit of %s ends before the reopen watch has logged at %s", sessionhost.ReopenLoadLimit, reopenPatience)
	}
}

func TestAReopenPastItsLimitLeavesTheChatClosedAndTheDeliveryRetriable(t *testing.T) {
	a, c := providerTestApp(t)
	events := watchEvents(a)
	release := heldLoads(t, c)
	a.sessionsLifecycle().LoadLimit = 40 * time.Millisecond
	const id = "codex:slow"
	busID, cwd := closedChat(t, a, id, "Slow Chat")
	a.ChatWindows([]string{id})
	wakeRound(t, a)
	if _, _, active := a.busCoordinator().CurrentDeliveryAttempt(id); !active {
		t.Fatal("no delivery attempt was begun for the chat with a message waiting")
	}

	_, err := a.OpenSessionAt(cwd, id)
	if !sessionhost.IsLoadTimeout(err) {
		t.Fatalf("the open did not fail at the load limit: %v", err)
	}
	if a.session(id) != nil {
		t.Fatal("the chat is open after its load passed the limit")
	}
	if n := waiting(t, a, busID); n != 1 {
		t.Fatalf("%d messages wait after the failed open, and 1 was waiting before it", n)
	}
	if _, _, active := a.busCoordinator().CurrentDeliveryAttempt(id); active {
		t.Fatal("the delivery attempt is still running after its open failed")
	}
	records, err := a.busCoordinator().WaitingFailures()
	if err != nil {
		t.Fatal(err)
	}
	if record, ok := records[id]; !ok || record.Stage != "failed" {
		t.Fatalf("the attempt was not recorded as failed, which is what a retry starts from: %+v", records)
	}
	cards, err := a.waitingChatNotifications()
	if err != nil || len(cards) != 1 || !cards[0].RetryAllowed {
		t.Fatalf("the person is not offered a retry: %+v %v", cards, err)
	}

	// The agent still owes its answer, so the chat is not asked for again,
	// and what the given-up load still replays reaches nobody.
	if _, err := a.RetryWaitingSession(id); !errors.Is(err, sessionhost.ErrLoadOutstanding) {
		t.Fatalf("a retry beside the unanswered load was not refused: %v", err)
	}
	if n := countCalls(c, "session/load"); n != 1 {
		t.Fatalf("the agent was asked to load the chat %d times while the first load was unanswered", n)
	}
	before := events.count("agent:update")
	a.newAgentHandler().Notify("session/update", json.RawMessage(`{"sessionId":"codex:slow","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"late replay"}}}`))
	if events.count("agent:update") != before {
		t.Fatal("the replay of a load that was given up reached the page")
	}

	// The late answer is discarded, and the retry then opens the chat.
	release()
	until(t, "the late answer has been taken", func() bool { return !a.sessionsLifecycle().LoadOutstanding(id) })
	if a.session(id) != nil {
		t.Fatal("the late answer to a load that was given up opened the chat")
	}
	if n := waiting(t, a, busID); n != 1 {
		t.Fatalf("%d messages wait after the late answer, and 1 was waiting before it", n)
	}
	if _, err := a.RetryWaitingSession(id); err != nil {
		t.Fatalf("the retry after the late answer failed: %v", err)
	}
	if s := a.session(id); s == nil || !s.Open() {
		t.Fatal("the retry did not open the chat")
	}
	if n := countCalls(c, "session/load"); n != 2 {
		t.Fatalf("the agent was asked to load the chat %d times, and the open and its retry are 2", n)
	}
}

func TestALateAnswerToAnAbandonedLoadIsIgnored(t *testing.T) {
	a, c := providerTestApp(t)
	initial := a.runtime.ProviderLocked("codex").Initialization()
	initial.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
	setTestInitialization(a, "codex", initial)
	events := watchEvents(a)
	release := heldLoads(t, c)
	a.sessionsLifecycle().LoadLimit = 40 * time.Millisecond
	ws := t.TempDir()
	const id = "codex:late"

	_, err := a.OpenSessionAt(ws, id)
	if !sessionhost.IsLoadTimeout(err) {
		t.Fatalf("the open did not fail at the load limit: %v", err)
	}
	var detailed interface{ ErrorDetail() any }
	if !errors.As(err, &detailed) {
		t.Fatal("the failure has no detail for the page")
	}
	if !a.sessionsLifecycle().LoadOutstanding(id) {
		t.Fatal("the load that was given up is not recorded as unanswered")
	}
	sessions := events.count("agent:session")

	release()
	until(t, "the late answer has been taken", func() bool { return !a.sessionsLifecycle().LoadOutstanding(id) })
	if a.session(id) != nil {
		t.Fatal("the late answer opened the chat")
	}
	if events.count("agent:session") != sessions {
		t.Fatal("the late answer changed what the page is told of the chat")
	}
	// The agent holds the session loaded now, and it advertises
	// session/close, so the session is closed as before any second load.
	if n := countCalls(c, "session/close"); n != 1 {
		t.Fatalf("the session the late answer left loaded was closed %d times", n)
	}
	// The connection was not ended for one slow chat.
	select {
	case <-c.Done():
		t.Fatal("the agent's connection was ended because one chat's load was slow")
	default:
	}
	if _, err := a.OpenSessionAt(ws, id); err != nil {
		t.Fatalf("the chat could not be opened after the late answer: %v", err)
	}
}

func TestAnOpenStartedFromThePageReturnsAtOnceAndReportsByEvent(t *testing.T) {
	a, c := providerTestApp(t)
	events := watchEvents(a)
	release := heldLoads(t, c)
	ws := t.TempDir()
	const id = "codex:paged"
	args := rawArgs(t, ws, id)

	began := time.Now()
	state, err := a.BeginChatOpen("page-1", "OpenSessionAt", args)
	if err != nil || state.State != "opening" || state.Outcome != nil {
		t.Fatalf("the open did not start: %+v %v", state, err)
	}
	if took := time.Since(began); took > 2*time.Second {
		t.Fatalf("starting the open took %s while the agent had not answered", took)
	}
	until(t, "the agent has been asked to load the chat", func() bool { return countCalls(c, "session/load") == 1 })
	// The open counts as unfinished work for a window asked to close, as the
	// call that waited for it did.
	a.mu.Lock()
	unfinished := a.exitCallNames["OpenSessionAt"]
	a.mu.Unlock()
	if unfinished != 1 {
		t.Fatalf("the running open is counted %d times as unfinished work", unfinished)
	}

	// The same token asks again and starts nothing. Another token, as a
	// reloaded page has, joins the load in flight.
	if again, err := a.BeginChatOpen("page-1", "OpenSessionAt", args); err != nil || again.State != "opening" {
		t.Fatalf("asking again gave %+v %v", again, err)
	}
	if other, err := a.BeginChatOpen("page-2", "OpenSessionAt", args); err != nil || other.State != "opening" {
		t.Fatalf("a second open of the chat gave %+v %v", other, err)
	}
	select {
	case o := <-events.open:
		t.Fatalf("an outcome was sent before the agent answered: %+v", o)
	case <-time.After(50 * time.Millisecond):
	}

	release()
	seen := map[string]bool{}
	for range 2 {
		o := events.outcome(t)
		if o.Error != "" || o.Info == nil || o.Info.ID != id || o.Method != "OpenSessionAt" {
			t.Fatalf("the outcome is not the open chat: %+v", o)
		}
		seen[o.Open] = true
	}
	if !seen["page-1"] || !seen["page-2"] {
		t.Fatalf("the outcomes were for %v", seen)
	}
	if n := countCalls(c, "session/load"); n != 1 {
		t.Fatalf("two opens of one chat at once made %d loads", n)
	}
	done, err := a.BeginChatOpen("page-1", "OpenSessionAt", args)
	if err != nil || done.State != "done" || done.Outcome == nil || done.Outcome.Info == nil || done.Outcome.Info.ID != id {
		t.Fatalf("asking again after the end gave %+v %v", done, err)
	}

	if _, err := a.BeginChatOpen("page-3", "RemoveChat", rawArgs(t, id)); err == nil {
		t.Fatal("a method that opens no chat was started as an open")
	}
	if _, err := a.BeginChatOpen("", "OpenSessionAt", args); err == nil {
		t.Fatal("an open with no token was started")
	}
	if pageMethods["BeginChatOpen"] != pageQuery {
		t.Fatal("BeginChatOpen is not a query, so the call would count as unfinished work beside the open it starts")
	}
}

func TestAnOpenStartedFromThePageReportsItsFailureWithTheDetail(t *testing.T) {
	a, c := providerTestApp(t)
	events := watchEvents(a)
	release := heldLoads(t, c)
	a.sessionsLifecycle().LoadLimit = 40 * time.Millisecond
	if _, err := a.BeginChatOpen("page-1", "OpenSessionAt", rawArgs(t, t.TempDir(), "codex:never")); err != nil {
		t.Fatal(err)
	}
	o := events.outcome(t)
	var detail struct {
		Kind string `json:"kind"`
	}
	if o.Info != nil || o.Error == "" || json.Unmarshal(o.Detail, &detail) != nil || detail.Kind != "open-timeout" {
		t.Fatalf("the outcome is not the failure at the load limit: %+v", o)
	}
	if a.session("codex:never") != nil {
		t.Fatal("the chat is open after its load passed the limit")
	}
	// Nothing of this test runs on after it.
	release()
	until(t, "the late answer has been taken", func() bool { return !a.sessionsLifecycle().LoadOutstanding("codex:never") })
}

// A browser runs a limited number of requests to one host at a time. The
// client here is given six connections, which the event stream and the calls
// share, as a page's do. Six chats whose loads are never answered are opened,
// and another call still returns.
func TestOpensThatNeverFinishLeaveThePageItsCalls(t *testing.T) {
	a, c := providerTestApp(t)
	outcomes := watchEvents(a)
	release := heldLoads(t, c)
	b, err := newPageBridge(a, pageMethods)
	if err != nil {
		t.Fatal(err)
	}
	a.events = b
	t.Cleanup(func() { a.events = nil })
	server := httptest.NewServer(b.Handler("/bridge"))
	t.Cleanup(server.Close)
	client := &http.Client{Transport: &http.Transport{MaxConnsPerHost: 6}, Timeout: 10 * time.Second}
	t.Cleanup(client.CloseIdleConnections)

	stream, err := http.NewRequest(http.MethodGet, server.URL+"/bridge/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	streaming := &http.Client{Transport: client.Transport}
	events, err := streaming.Do(stream)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { events.Body.Close() })

	call := func(method string, args ...any) (json.RawMessage, string) {
		t.Helper()
		body, err := json.Marshal(map[string]any{"method": method, "args": args})
		if err != nil {
			t.Fatal(err)
		}
		res, err := client.Post(server.URL+"/bridge/call", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("%s did not return: %v", method, err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var answer struct {
			Result json.RawMessage `json:"result"`
			Error  string          `json:"error"`
		}
		if err := json.Unmarshal(raw, &answer); err != nil {
			t.Fatalf("%s answered %q", method, raw)
		}
		return answer.Result, answer.Error
	}
	ws := t.TempDir()
	for _, n := range []string{"1", "2", "3", "4", "5", "6"} {
		result, failure := call("BeginChatOpen", "page-"+n, "OpenSessionAt", []any{ws, "codex:stuck-" + n})
		var state ChatOpenState
		if failure != "" || json.Unmarshal(result, &state) != nil || state.State != "opening" {
			t.Fatalf("open %s gave %s %s", n, result, failure)
		}
	}
	until(t, "the agent has been asked for all six chats", func() bool { return countCalls(c, "session/load") == 6 })
	if result, failure := call("ChatRemoved", "codex:stuck-1"); failure != "" || string(result) != "false" {
		t.Fatalf("a call beside six unfinished opens gave %s %s", result, failure)
	}
	// Nothing of this test runs on after it.
	release()
	for range 6 {
		outcomes.outcome(t)
	}
}
