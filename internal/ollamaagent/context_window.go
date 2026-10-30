package ollamaagent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"studio/internal/acp"
	"studio/internal/appletcontext"
)

// What the adapter does when a conversation's known window fills
// (docs/plans/ollama-context-window.md). Every figure is Ollama's own count
// of what it processed, prompt_eval_count and eval_count after a request, and
// what each turn cost is recorded as it happens, so nothing here estimates what
// the next request will be.

// The requests Dinah makes of the adapter about a conversation's window.
const (
	// ContextMethod says where the conversation's window stands.
	ContextMethod = "_dinah/context"
	// SetAsideMethod sets turns aside: the oldest, or the turns the request lists.
	SetAsideMethod = "_dinah/setAside"
	// TurnsMethod lists the turns that may be set aside, with what freeing room
	// needs, for a caller that chooses which.
	TurnsMethod = "_dinah/asideTurns"
	// ChoiceMethod keeps, or with "" forgets, the course the person chose for a
	// full window.
	ChoiceMethod = "_dinah/contextChoice"
)

// The courses the person may choose when the window fills.
const (
	ChoiceSetAside = "set-aside"
	ChoiceCompact  = "compact"
)

// Where the measured count stands against a known window: warning is the line
// at which the person is told, and full the line at which the next send is held.
const (
	warnPercent = 80
	fullPercent = 90
)

// ContextState is a conversation's window as the adapter measures it.
type ContextState struct {
	// Choice is the course the person chose for this conversation, or "".
	Choice string `json:"choice"`
	// Used is the tokens in the conversation as Ollama last counted them, less
	// what the turns set aside since cost, and Size and Known the window.
	Used  int  `json:"used"`
	Size  int  `json:"size"`
	Known bool `json:"known"`
	// Full says Known, and Used has reached the line at which the next send is
	// held.
	Full bool `json:"full"`
	// CanSetAside says setting the oldest turns aside would free at least one, which
	// it cannot when the oldest turn's cost was not recorded. A caller offers the
	// course only when this is true.
	CanSetAside bool `json:"canSetAside"`
}

type contextParams struct {
	SessionID string `json:"sessionId"`
	Choice    string `json:"choice,omitempty"`
	// Turns are the turns to set aside, by the numbers TurnsMethod gave them. Without
	// them the oldest turns are set aside.
	Turns []int `json:"turns,omitempty"`
}

// AsideTurn is a turn that may be set aside. N is its number, counting from 1 in
// the order the turns were made; Said and Answered are the first line of what the
// person said and of the agent's first answer, cut short; Cost is the tokens
// recorded for the turn when it was sent.
type AsideTurn struct {
	N        int    `json:"n"`
	Said     string `json:"said"`
	Answered string `json:"answered"`
	Cost     int    `json:"cost"`
}

// AsideTurns are the turns that may be set aside, oldest first, and Needed, the
// tokens that must be freed to get under the warning line; 0 when it is not
// crossed. The latest turn, a turn already set aside and a turn whose cost was not
// recorded are never offered.
type AsideTurns struct {
	Needed int         `json:"needed"`
	Turns  []AsideTurn `json:"turns"`
}

// SetAsideResult says how many turns were set aside.
type SetAsideResult struct {
	Turns int `json:"turns"`
}

// window is a conversation's window: the size the person chose, which is known,
// and otherwise the model's own context_length as the model's limit, which is
// known for a cloud model, because Ollama documents that a cloud model is set to
// its maximum context length by default, and not for a local one, because Ollama
// documents no way to read the length its server is set to.
func (c *Conn) window(ctx context.Context, rec record) (size int, kind string, known bool) {
	if rec.Context > 0 {
		return rec.Context, "chosen", true
	}
	if s, err := c.details(ctx, rec.Model); err == nil {
		size = s.contextLength()
	}
	return size, "model", isCloudModel(rec.Model)
}

// isCloudModel says a model runs in Ollama's cloud, by its name ending in :cloud
// or -cloud. This rests on Paul's decision of 30 October 2026, and not on Ollama's
// documentation: its pages show cloud models by example (gemma4:cloud,
// gpt-oss:120b-cloud) and state no rule that makes a name a way to recognise
// one. Paul's reason is that every cloud model he has seen is named so and no
// local model is. If Ollama documents a field or a rule, this is where it
// replaces the convention.
func isCloudModel(name string) bool { return IsCloudModel(name) }

// IsCloudModel is the naming convention above, for the window.
func IsCloudModel(name string) bool {
	return strings.HasSuffix(name, ":cloud") || strings.HasSuffix(name, "-cloud")
}

// contextState reads where a conversation's window stands.
func (c *Conn) contextState(ctx context.Context, id string) (ContextState, error) {
	c.mu.Lock()
	s := c.sessions[id]
	if s == nil {
		c.mu.Unlock()
		return ContextState{}, errors.New("the Ollama conversation is not open")
	}
	rec := s.rec
	// The turns are copied so that asking what could be set aside changes nothing.
	trial := rec
	trial.Messages = slices.Clone(rec.Messages)
	c.mu.Unlock()
	size, _, known := c.window(ctx, rec)
	st := ContextState{Choice: rec.Choice, Used: rec.Used, Size: size, Known: known}
	st.Full = known && size > 0 && rec.Used*100 >= size*fullPercent
	st.CanSetAside = st.Full && setAside(&trial, size) > 0
	return st, nil
}

// visibleMessages are the messages the request is built from: those not set
// aside, which the saved record keeps.
func visibleMessages(messages []message) []message {
	out := make([]message, 0, len(messages))
	for _, m := range messages {
		if !m.Aside {
			out = append(out, m)
		}
	}
	return out
}

// setAside sets the oldest whole turns aside until what is left, by the costs
// recorded when they were sent, is below the warning line of the window. The
// latest turn is never set aside, and a turn whose cost was not recorded stops
// the walk, since its size would be a guess. It returns how many turns it set
// aside, changing rec, and the turns stay in the record.
func setAside(rec *record, size int) int {
	var starts []int
	for i, m := range rec.Messages {
		if m.Turn {
			starts = append(starts, i)
		}
	}
	remaining, n := rec.Used, 0
	for k := 0; k+1 < len(starts) && remaining*100 >= size*warnPercent; k++ {
		first, next := starts[k], starts[k+1]
		if rec.Messages[first].Aside {
			continue
		}
		cost := rec.Messages[first].Cost
		if cost <= 0 {
			break
		}
		for i := first; i < next; i++ {
			rec.Messages[i].Aside = true
		}
		remaining -= cost
		n++
	}
	if n > 0 {
		rec.Used = max(remaining, 0)
	}
	return n
}

// turnInfo is a turn of a record: where its messages begin and end, its number
// from 1, and the cost recorded for it.
type turnInfo struct {
	n, first, next, cost int
	aside                bool
}

// recordTurns lists a record's turns in order.
func recordTurns(rec *record) []turnInfo {
	var starts []int
	for i, m := range rec.Messages {
		if m.Turn {
			starts = append(starts, i)
		}
	}
	out := make([]turnInfo, len(starts))
	for k, first := range starts {
		next := len(rec.Messages)
		if k+1 < len(starts) {
			next = starts[k+1]
		}
		out[k] = turnInfo{n: k + 1, first: first, next: next, cost: rec.Messages[first].Cost, aside: rec.Messages[first].Aside}
	}
	return out
}

// neededToFree is the fewest tokens that must be freed from used to get under the
// warning line of a window of size; 0 when it is not crossed.
func neededToFree(used, size int) int {
	if used*100 < size*warnPercent {
		return 0
	}
	return (used*100-size*warnPercent)/100 + 1
}

// asideCandidates are the turns that may be set aside: not the latest, not already
// set aside, and with a cost that was recorded.
func asideCandidates(rec *record) []turnInfo {
	turns := recordTurns(rec)
	var out []turnInfo
	for k := 0; k+1 < len(turns); k++ {
		if turns[k].aside || turns[k].cost <= 0 {
			continue
		}
		out = append(out, turns[k])
	}
	return out
}

// lineOf is the first line of what a message says that is the speaker's own, on one
// line and cut at limit characters, without anything that was added to a prompt.
func lineOf(content string, limit int) string {
	for _, block := range strings.Split(content, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "[Dinah ") || appletcontext.Contains(block) {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			line = strings.Join(strings.Fields(line), " ")
			if line == "" {
				continue
			}
			if r := []rune(line); len(r) > limit {
				line = string(r[:limit]) + "…"
			}
			return line
		}
	}
	return ""
}

// asideTurnsOf lists what a caller needs to choose which turns to set aside.
func asideTurnsOf(rec *record, size int) AsideTurns {
	out := AsideTurns{Needed: neededToFree(rec.Used, size), Turns: []AsideTurn{}}
	if out.Needed == 0 {
		return out
	}
	for _, t := range asideCandidates(rec) {
		answered := ""
		for i := t.first + 1; i < t.next; i++ {
			if rec.Messages[i].Role == "assistant" && strings.TrimSpace(rec.Messages[i].Content) != "" {
				answered = lineOf(rec.Messages[i].Content, 80)
				break
			}
		}
		out.Turns = append(out.Turns, AsideTurn{N: t.n, Said: lineOf(rec.Messages[t.first].Content, 120), Answered: answered, Cost: t.cost})
	}
	return out
}

// setAsideChosen sets aside the turns listed, whole, and says how many. It refuses,
// changing nothing, a list that includes the latest turn, a turn already set aside
// or whose cost was not recorded, a turn twice or one that does not exist, or that
// frees too little to get under the warning line, so a caller that chooses can
// never do what the oldest-first rule could not.
func setAsideChosen(rec *record, size int, chosen []int) (int, error) {
	needed := neededToFree(rec.Used, size)
	if needed == 0 {
		return 0, errors.New("the conversation is under the warning line, so no turn needs to be set aside")
	}
	allowed := map[int]turnInfo{}
	for _, t := range asideCandidates(rec) {
		allowed[t.n] = t
	}
	seen := map[int]bool{}
	freed := 0
	for _, n := range chosen {
		t, ok := allowed[n]
		switch {
		case seen[n]:
			return 0, fmt.Errorf("turn %d is listed twice", n)
		case !ok:
			return 0, fmt.Errorf("turn %d cannot be set aside: it is the latest, is already set aside, has no recorded cost, or does not exist", n)
		}
		seen[n] = true
		freed += t.cost
	}
	if freed < needed {
		return 0, fmt.Errorf("the turns listed free %d tokens, and %d must be freed", freed, needed)
	}
	for n := range seen {
		t := allowed[n]
		for i := t.first; i < t.next; i++ {
			rec.Messages[i].Aside = true
		}
	}
	rec.Used = max(rec.Used-freed, 0)
	return len(seen), nil
}

// setAsideNow sets turns aside in an open conversation that has no turn
// running, saves the record and says so in the transcript. With chosen it sets
// those turns aside, and otherwise the oldest.
func (c *Conn) setAsideNow(ctx context.Context, id string, chosen []int) (int, error) {
	c.mu.Lock()
	s := c.sessions[id]
	if s == nil {
		c.mu.Unlock()
		return 0, errors.New("the Ollama conversation is not open")
	}
	if s.busy {
		c.mu.Unlock()
		return 0, errors.New("the Ollama conversation has a turn going")
	}
	rec := s.rec
	rec.Messages = append([]message(nil), rec.Messages...)
	c.mu.Unlock()
	size, _, known := c.window(ctx, rec)
	if !known || size <= 0 {
		return 0, errors.New("turns are set aside only when the window is known; choose a context size first")
	}
	var n int
	note := ""
	if len(chosen) > 0 {
		var err error
		if n, err = setAsideChosen(&rec, size, chosen); err != nil {
			return 0, err
		}
		slices.Sort(chosen)
		note = setAsideChosenNote(chosen)
	} else {
		n = setAside(&rec, size)
		note = setAsideNote(n)
	}
	if err := c.save(rec); err != nil {
		return 0, fmt.Errorf("save conversation: %w", err)
	}
	c.mu.Lock()
	s.rec = rec
	s.warned = false
	c.mu.Unlock()
	c.update(id, map[string]any{"sessionUpdate": "dinah_status", "text": note})
	return n, nil
}

// setAsideChosenNote says which turns were set aside, since they may not be the
// earliest.
func setAsideChosenNote(turns []int) string {
	names := make([]string, len(turns))
	for i, n := range turns {
		names[i] = strconv.Itoa(n)
	}
	list := strings.Join(names, ", ")
	if len(names) == 1 {
		return "Turn " + list + " is set aside to make room, and stays in the saved chat."
	}
	return "Turns " + list + " are set aside to make room, and stay in the saved chat."
}

// setAsideNote says what was done, and never that the window was not crossed.
func setAsideNote(n int) string {
	switch n {
	case 0:
		return "No turn could be set aside to make room, because none had its size recorded. The saved chat is whole."
	case 1:
		return "The earliest turn is set aside to make room, and stays in the saved chat."
	}
	return fmt.Sprintf("The earliest %d turns are set aside to make room, and stay in the saved chat.", n)
}

// noteWindow tells the person, once each time the line is crossed, that the
// window is nearly full, from the count Ollama reported for the last request.
func (c *Conn) noteWindow(s *session, id string, used, size int, known bool) {
	if !known || size <= 0 {
		return
	}
	c.mu.Lock()
	over := used*100 >= size*warnPercent
	said := s.warned
	s.warned = over
	c.mu.Unlock()
	if over && !said {
		c.update(id, map[string]any{"sessionUpdate": "dinah_status", "kind": "warning",
			"text": fmt.Sprintf("The conversation is using %d of the %d tokens of its context window.", used, size)})
	}
}

// contextRequest answers the requests about a window.
func (c *Conn) contextRequest(ctx context.Context, method string, params, out any) error {
	var p contextParams
	if err := acp.Decode(params, &p); err != nil {
		return err
	}
	switch method {
	case ContextMethod:
		st, err := c.contextState(ctx, p.SessionID)
		if err != nil {
			return err
		}
		return acp.Result(st, out)
	case TurnsMethod:
		c.mu.Lock()
		s := c.sessions[p.SessionID]
		if s == nil {
			c.mu.Unlock()
			return errors.New("the Ollama conversation is not open")
		}
		rec := s.rec
		rec.Messages = append([]message(nil), rec.Messages...)
		c.mu.Unlock()
		size, _, known := c.window(ctx, rec)
		if !known || size <= 0 {
			return errors.New("turns are set aside only when the window is known; choose a context size first")
		}
		return acp.Result(asideTurnsOf(&rec, size), out)
	case SetAsideMethod:
		n, err := c.setAsideNow(ctx, p.SessionID, p.Turns)
		if err != nil {
			return err
		}
		return acp.Result(SetAsideResult{Turns: n}, out)
	case ChoiceMethod:
		if p.Choice != "" && p.Choice != ChoiceSetAside && p.Choice != ChoiceCompact {
			return errors.New("that is not a course for a full window")
		}
		c.mu.Lock()
		s := c.sessions[p.SessionID]
		if s == nil {
			c.mu.Unlock()
			return errors.New("the Ollama conversation is not open")
		}
		rec := s.rec
		rec.Choice = p.Choice
		c.mu.Unlock()
		if err := c.save(rec); err != nil {
			return fmt.Errorf("save conversation: %w", err)
		}
		c.mu.Lock()
		s.rec.Choice = p.Choice
		c.mu.Unlock()
		return nil
	}
	return errors.New("unknown request")
}
