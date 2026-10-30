package ollamaagent

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"studio/internal/acp"
)

// chatOf is a record whose turns cost what costs says, each a person's message and
// an answer.
func chatOf(used int, costs ...int) record {
	rec := record{Used: used}
	for i, c := range costs {
		rec.Messages = append(rec.Messages,
			message{Role: "user", Content: fmt.Sprintf("Question %d is asked here", i+1), Turn: true, Cost: c},
			message{Role: "assistant", Content: fmt.Sprintf("Answer %d is given here", i+1)})
	}
	return rec
}

// The turns offered are those that may be set aside: not the latest, not already
// set aside, and with a recorded cost. Each has the first line of what was said and
// of the answer, cut short and without anything added to a prompt, and the tokens to
// free are what gets the conversation under the warning line.
func TestTheTurnsOfferedAreThoseThatMayBeSetAside(t *testing.T) {
	rec := chatOf(1900, 500, 0, 300, 250, 400)
	long := strings.Repeat("w", 300)
	rec.Messages[0].Content = "[Dinah Applet Context]\nuse dinah-applet-v1:5QAHWLZE3I6FEG3ZDG634FNDCD.53G4C65FBRVPZX77KYPVNNDMAH\n\n" + long + "\nsecond line"
	rec.Messages[1].Content = "A  short\nanswer\nover three lines"
	// Turn 4 is set aside already.
	for i := 6; i < 8; i++ {
		rec.Messages[i].Aside = true
	}
	got := asideTurnsOf(&rec, 2048)
	// 1900 of 2048 is over the 80% line of 1638.4, and 1900-262 = 1638 is under it, so 262 must be freed.
	if got.Needed != 262 || len(got.Turns) != 2 || got.Turns[0].N != 1 || got.Turns[0].Cost != 500 || got.Turns[1].N != 3 || got.Turns[1].Cost != 300 {
		t.Fatalf("offered %+v", got)
	}
	if said := got.Turns[0].Said; !strings.HasPrefix(said, "www") || len([]rune(said)) != 121 || strings.Contains(said, "5QAHWLZE") || strings.Contains(said, "second line") || !strings.HasSuffix(said, "…") {
		t.Fatalf("the first turn's line %q", said)
	}
	if got.Turns[0].Answered != "A short" || got.Turns[1].Said != "Question 3 is asked here" || got.Turns[1].Answered != "Answer 3 is given here" {
		t.Fatalf("lines %+v", got.Turns)
	}
	// Under the line nothing needs freeing, and the list is empty and not null.
	under := asideTurnsOf(&rec, 100000)
	if under.Needed != 0 || under.Turns == nil || len(under.Turns) != 0 {
		t.Fatalf("under the line %+v", under)
	}
	if neededToFree(1639, 2048) != 1 || neededToFree(1638, 2048) != 0 {
		t.Fatalf("the fewest tokens to free: %d and %d", neededToFree(1639, 2048), neededToFree(1638, 2048))
	}
}

// Turns chosen by a caller are set aside whole, wherever they are, and any list the
// oldest-first rule could not have made is refused with nothing changed.
func TestChosenTurnsAreSetAsideWholeOrNotAtAll(t *testing.T) {
	fresh := func() record { return chatOf(1900, 500, 0, 300, 250, 400) }
	rec := fresh()
	n, err := setAsideChosen(&rec, 2048, []int{3, 1})
	if err != nil || n != 2 || rec.Used != 1100 {
		t.Fatalf("set aside %d, used %d: %v", n, rec.Used, err)
	}
	var aside []int
	for _, turn := range recordTurns(&rec) {
		if turn.aside {
			aside = append(aside, turn.n)
		}
		for i := turn.first; i < turn.next; i++ {
			if rec.Messages[i].Aside != (turn.n == 1 || turn.n == 3) {
				t.Fatalf("message %d of turn %d is flagged %v", i, turn.n, rec.Messages[i].Aside)
			}
		}
	}
	if !slices.Equal(aside, []int{1, 3}) {
		t.Fatalf("turns set aside %v", aside)
	}
	// A single turn that frees enough is allowed.
	one := fresh()
	if n, err := setAsideChosen(&one, 2048, []int{1}); err != nil || n != 1 || one.Used != 1400 {
		t.Fatalf("one turn: %d %d %v", n, one.Used, err)
	}
	for name, list := range map[string][]int{
		"the latest turn":       {5},
		"a turn with no cost":   {2},
		"a turn twice":          {1, 1},
		"a turn that is not":    {9},
		"a turn numbered zero":  {0},
		"too little":            {4},
		"a negative turn":       {-1},
		"the latest among good": {1, 5},
	} {
		rec := fresh()
		before := fresh()
		if n, err := setAsideChosen(&rec, 2048, list); err == nil || n != 0 || !reflect.DeepEqual(rec, before) {
			t.Errorf("%s: set aside %d, error %v, changed %v", name, n, err, !reflect.DeepEqual(rec, before))
		}
	}
	// A turn already set aside, and a conversation under the line, are refused.
	done := fresh()
	setAsideChosen(&done, 2048, []int{1})
	done.Used = 1900
	if _, err := setAsideChosen(&done, 2048, []int{1}); err == nil {
		t.Fatal("a turn already set aside was set aside again")
	}
	under := chatOf(1000, 500, 300, 200)
	if _, err := setAsideChosen(&under, 2048, []int{1}); err == nil {
		t.Fatal("turns were set aside under the line")
	}
}

// Through the connection: the turns are listed, a set a caller chose is set aside
// and the next request leaves out exactly those turns, the transcript says which,
// and a list that is not allowed is refused and changes nothing.
func TestAConnectionSetsAsideTheTurnsACallerChose(t *testing.T) {
	c, h, count, seen, id := windowChat(t)
	for _, used := range []int64{1000, 1500, 1900} {
		sendTurn(t, c, id, count, used)
	}
	ctx := context.Background()
	var offered AsideTurns
	if err := c.Call(ctx, TurnsMethod, contextParams{SessionID: id}, &offered); err != nil {
		t.Fatal(err)
	}
	// Turn 1 cost 1000 and turn 2 cost 500; turn 3, the latest, is not offered. 1900 of
	// 2048 needs 262 freed to get under the line.
	if offered.Needed != 262 || len(offered.Turns) != 2 || offered.Turns[0].N != 1 || offered.Turns[0].Cost != 1000 || offered.Turns[1].N != 2 || offered.Turns[1].Cost != 500 {
		t.Fatalf("offered %+v", offered)
	}
	var res SetAsideResult
	if err := c.Call(ctx, SetAsideMethod, contextParams{SessionID: id, Turns: []int{3}}, &res); err == nil {
		t.Fatal("the latest turn was set aside")
	}
	if rec, _ := c.load(id); rec.Used != 1900 || slices.ContainsFunc(rec.Messages, func(m message) bool { return m.Aside }) {
		t.Fatalf("a refused request changed the record: used %d", rec.Used)
	}
	if err := c.Call(ctx, SetAsideMethod, contextParams{SessionID: id, Turns: []int{2}}, &res); err != nil || res.Turns != 1 {
		t.Fatalf("set aside %d: %v", res.Turns, err)
	}
	if !strings.Contains(h.said(), "Turn 2 is set aside to make room, and stays in the saved chat.") {
		t.Fatalf("the transcript did not say which turn: %s", h.said())
	}
	stored, _ := c.load(id)
	if len(stored.Messages) != 6 || stored.Messages[0].Aside || !stored.Messages[2].Aside || !stored.Messages[3].Aside || stored.Messages[4].Aside || stored.Used != 1400 {
		t.Fatalf("the record after: used %d, %+v", stored.Used, stored.Messages)
	}
	sendTurn(t, c, id, count, 900)
	last := (*seen)[len(*seen)-1]
	// The system message, turn 1 and turn 3 (two messages each) and the new message:
	// turn 2, which is not the earliest, is left out.
	if len(last.Messages) != 1+4+1 {
		t.Fatalf("the request has %d messages", len(last.Messages))
	}
	var _ acp.PromptResult
}
