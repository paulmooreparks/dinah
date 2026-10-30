package ollamaagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"studio/internal/acp"
)

// countingOllama answers every chat with one short message and the counts the
// test sets, so a test can walk a conversation up to its window.
func countingOllama(t *testing.T) (*httptest.Server, *atomic.Int64, *[]chatRequest) {
	t.Helper()
	var count atomic.Int64
	var mu sync.Mutex
	var seen []chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			io.WriteString(w, `{"version":"0.34.1"}`)
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"nano:4b","model":"nano:4b","details":{}}]}`)
		case "/api/show":
			io.WriteString(w, `{"capabilities":["completion"],"model_info":{"general.architecture":"nano","nano.context_length":131072}}`)
		case "/api/chat":
			var req chatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			seen = append(seen, req)
			mu.Unlock()
			io.WriteString(w, fmt.Sprintf(`{"message":{"role":"assistant","content":"ok"},"done":true,"done_reason":"stop","prompt_eval_count":%d,"eval_count":0}`+"\n", count.Load()))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &count, &seen
}

// windowChat opens a conversation on a chosen size of 2048 tokens.
func windowChat(t *testing.T) (*Conn, *host, *atomic.Int64, *[]chatRequest, string) {
	t.Helper()
	srv, count, seen := countingOllama(t)
	h := &host{answer: "once"}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	ctx := context.Background()
	var opened acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &opened); err != nil {
		t.Fatal(err)
	}
	var set acp.ConfigOptionsResult
	if err := c.Call(ctx, "session/set_config_option", acp.SetConfigOptionParams{SessionID: opened.SessionID, ConfigID: "context", Value: "2048"}, &set); err != nil {
		t.Fatal(err)
	}
	return c, h, count, seen, opened.SessionID
}

func sendTurn(t *testing.T, c *Conn, id string, count *atomic.Int64, used int64) {
	t.Helper()
	count.Store(used)
	var res acp.PromptResult
	if err := c.Call(context.Background(), "session/prompt", acp.PromptParams{SessionID: id, Prompt: []acp.ContentBlock{{Type: "text", Text: "Next"}}}, &res); err != nil {
		t.Fatal(err)
	}
}

func windowState(t *testing.T, c *Conn, id string) ContextState {
	t.Helper()
	var st ContextState
	if err := c.Call(context.Background(), ContextMethod, contextParams{SessionID: id}, &st); err != nil {
		t.Fatal(err)
	}
	return st
}

// A chat on a chosen size is told once each time it passes 80%, the cost of each
// turn is recorded on its first message, and the state says when the window is
// full at 90%.
func TestTheWindowIsMeasuredNotedAndFullAtNinetyPercent(t *testing.T) {
	c, h, count, _, id := windowChat(t)
	warnings := func() int { return strings.Count(h.said(), `"kind":"warning"`) }
	sendTurn(t, c, id, count, 1000)
	if st := windowState(t, c, id); st.Used != 1000 || st.Size != 2048 || !st.Known || st.Full || warnings() != 0 {
		t.Fatalf("after the first turn: %+v, %d warnings", st, warnings())
	}
	sendTurn(t, c, id, count, 1700) // 83%: noted
	if warnings() != 1 {
		t.Fatalf("%d warnings after passing 80%%", warnings())
	}
	sendTurn(t, c, id, count, 1750) // still over: not noted again
	if warnings() != 1 {
		t.Fatalf("%d warnings while it stayed over 80%%", warnings())
	}
	sendTurn(t, c, id, count, 500) // under again
	sendTurn(t, c, id, count, 1700)
	if warnings() != 2 {
		t.Fatalf("%d warnings after crossing the line a second time", warnings())
	}
	if st := windowState(t, c, id); st.Full {
		t.Fatalf("full at 83%%: %+v", st)
	}
	sendTurn(t, c, id, count, 1900) // 92%
	if st := windowState(t, c, id); !st.Full || st.Used != 1900 {
		t.Fatalf("not full at 92%%: %+v", st)
	}
	rec, err := c.load(id)
	if err != nil {
		t.Fatal(err)
	}
	var costs []int
	for _, m := range rec.Messages {
		if m.Turn {
			costs = append(costs, m.Cost)
		}
	}
	// A turn that shrank the count cost nothing.
	if !slices.Equal(costs, []int{1000, 700, 50, 0, 1200, 200}) {
		t.Fatalf("the turns cost %v", costs)
	}
}

func turnsOf(costs ...int) []message {
	var out []message
	for _, c := range costs {
		out = append(out, message{Role: "user", Content: "q", Turn: true, Cost: c}, message{Role: "assistant", Content: "a"})
	}
	return out
}

// Setting turns aside takes the oldest whole turns, by their recorded costs,
// until the rest is under 80%; the latest turn is kept; and a turn with no
// recorded cost stops the walk.
func TestSettingTurnsAsideTakesTheOldestByTheirRecordedCosts(t *testing.T) {
	rec := record{Used: 1900, Messages: turnsOf(1000, 500, 400)}
	if n := setAside(&rec, 2048); n != 1 || !rec.Messages[0].Aside || !rec.Messages[1].Aside || rec.Messages[2].Aside || rec.Used != 900 {
		t.Fatalf("set aside %d, used %d: %+v", n, rec.Used, rec.Messages)
	}
	rec = record{Used: 1900, Messages: turnsOf(100, 100, 100)}
	if n := setAside(&rec, 2048); n != 2 {
		t.Fatalf("with small turns %d were set aside, and the latest must stay", n)
	}
	if rec.Messages[4].Aside {
		t.Fatal("the latest turn was set aside")
	}
	rec = record{Used: 1900, Messages: turnsOf(0, 500, 500)}
	if n := setAside(&rec, 2048); n != 0 {
		t.Fatalf("a turn with no recorded cost was set aside: %d", n)
	}
}

// Through the connection: the record keeps every message, the next request
// leaves the set-aside ones out, and the transcript says what was done.
func TestSetAsideTurnsStayInTheRecordAndLeaveTheRequest(t *testing.T) {
	c, h, count, seen, id := windowChat(t)
	for _, used := range []int64{1000, 1500, 1900} {
		sendTurn(t, c, id, count, used)
	}
	var res SetAsideResult
	if err := c.Call(context.Background(), SetAsideMethod, contextParams{SessionID: id}, &res); err != nil || res.Turns != 1 {
		t.Fatalf("set aside %d turns: %v", res.Turns, err)
	}
	if !strings.Contains(h.said(), "dinah_status") || !strings.Contains(h.said(), "The earliest turn is set aside") {
		t.Fatalf("the transcript was not told: %s", h.said())
	}
	stored, err := c.load(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Messages) != 6 || !stored.Messages[0].Aside || stored.Messages[2].Aside {
		t.Fatalf("the record did not keep the turns, flagged: %+v", stored.Messages)
	}
	sendTurn(t, c, id, count, 900)
	last := (*seen)[len(*seen)-1]
	// The system message, the two turns that were kept (two messages each), and
	// the new message: the set-aside turn is left out.
	if len(last.Messages) != 1+4+1 {
		for _, m := range last.Messages {
			t.Logf("%s %q", m.Role, m.Content)
		}
		t.Fatalf("the request has %d messages", len(last.Messages))
	}
}

// The person's choice for a full window is kept in the record, can be forgotten,
// and must be one of the courses.
func TestTheChoiceForAFullWindowIsKeptAndForgotten(t *testing.T) {
	c, _, _, _, id := windowChat(t)
	choose := func(choice string) error {
		return c.Call(context.Background(), ChoiceMethod, contextParams{SessionID: id, Choice: choice}, nil)
	}
	if err := choose("shrug"); err == nil {
		t.Fatal("a course that does not exist was kept")
	}
	if err := choose(ChoiceCompact); err != nil {
		t.Fatal(err)
	}
	if st := windowState(t, c, id); st.Choice != ChoiceCompact {
		t.Fatalf("the choice is %q", st.Choice)
	}
	if rec, _ := c.load(id); rec.Choice != ChoiceCompact {
		t.Fatalf("the saved choice is %q", rec.Choice)
	}
	if err := choose(""); err != nil {
		t.Fatal(err)
	}
	if rec, _ := c.load(id); rec.Choice != "" {
		t.Fatalf("the choice was not forgotten: %q", rec.Choice)
	}
}

// The state says whether setting turns aside could free any: not while the oldest
// turn has no recorded cost, since its size would be a guess, and not when only
// the latest turn exists. Asking changes nothing.
func TestTheStateSaysWhetherTurnsCouldBeSetAside(t *testing.T) {
	c, _, count, _, id := windowChat(t)
	sendTurn(t, c, id, count, 1000)
	sendTurn(t, c, id, count, 1900)
	st := windowState(t, c, id)
	if !st.Full || !st.CanSetAside {
		t.Fatalf("full with a costed oldest turn: %+v", st)
	}
	again := windowState(t, c, id)
	if again != st {
		t.Fatalf("asking changed the state: %+v then %+v", st, again)
	}
	rec, err := c.load(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range rec.Messages {
		if m.Aside {
			t.Fatal("asking set a turn aside")
		}
	}
	// The oldest turn's cost was not recorded.
	c.mu.Lock()
	for i, m := range c.sessions[id].rec.Messages {
		if m.Turn {
			c.sessions[id].rec.Messages[i].Cost = 0
			break
		}
	}
	c.mu.Unlock()
	if st := windowState(t, c, id); !st.Full || st.CanSetAside {
		t.Fatalf("full with an uncosted oldest turn: %+v", st)
	}
	// A window that is not full has nothing to free.
	sendTurn(t, c, id, count, 500)
	if st := windowState(t, c, id); st.Full || st.CanSetAside {
		t.Fatalf("not full: %+v", st)
	}
}
