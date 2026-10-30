package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"studio/internal/judge"
)

// Which turns to set aside when a window fills is the fourth kind of judgement call
// (docs/plans/judgement-agents.md, step 6). The adapter sets the oldest whole turns
// aside, by the cost it recorded for each, until the conversation is under the
// warning line. A judge may choose the turns that matter least instead. It is shown
// one line from each turn that may be set aside and what each cost, and what must be
// freed, and nothing else; the latest turn, a turn with no recorded cost and the
// rest of any message are never shown. Its answer is checked against the measured
// costs here, and again by the adapter, which refuses what the oldest-first rule
// could not do, so a judge's answer can never set aside the latest turn, a turn
// whose cost is unknown, or too little.

// setAsideTurn is one turn a judge may choose.
type setAsideTurn struct {
	N        int    `json:"n" doc:"the turn's number, counting from 1"`
	Said     string `json:"said" doc:"the first line of what the person said, at most 120 characters"`
	Answered string `json:"answered" doc:"the first line of how the agent began its answer, at most 80 characters"`
	Cost     int    `json:"cost" doc:"the tokens recorded for the turn"`
}

// setAsideIn is everything a judge is shown to choose the turns.
type setAsideIn struct {
	Turns  []setAsideTurn `json:"turns" doc:"the turns that may be set aside, oldest first"`
	Needed int            `json:"needed" doc:"the tokens that must be freed"`
}

type setAsideOut struct {
	Turns []int `json:"turns"`
}

const (
	// setAsideInputCap is the most bytes of turns a judge is shown, about 3,000 tokens.
	// The oldest turns that fit are shown, since they are the candidates.
	setAsideInputCap = 12_000
	// setAsideDeadline is how long the person waits at the hold or the send.
	setAsideDeadline = 10 * time.Second
)

// checkSetAside refuses an answer that is in form and not allowed: no turn, a turn
// that was not offered, a turn twice, or turns whose recorded costs do not free what
// must be freed.
func checkSetAside(in setAsideIn, out setAsideOut) error {
	if len(out.Turns) == 0 {
		return errors.New("no turn was chosen")
	}
	cost := map[int]int{}
	for _, t := range in.Turns {
		cost[t.N] = t.Cost
	}
	seen := map[int]bool{}
	freed := 0
	for _, n := range out.Turns {
		c, ok := cost[n]
		switch {
		case !ok:
			return fmt.Errorf("turn %d was not offered", n)
		case seen[n]:
			return fmt.Errorf("turn %d is chosen twice", n)
		}
		seen[n] = true
		freed += c
	}
	if freed < in.Needed {
		return fmt.Errorf("the turns chosen free %d tokens, and %d must be freed", freed, in.Needed)
	}
	return nil
}

var setAsideKind = judge.Declare(judge.Kind[setAsideIn, setAsideOut]{
	Name:     "set-aside-choice",
	Describe: "Sets the oldest turns aside until enough is freed.",
	System: "A conversation between a person and an AI agent has grown too long, and some turns must be dropped from what the agent is sent. " +
		"You are given the older turns, oldest first, each with what the person said, how the agent began its answer and how many tokens it takes. " +
		"Choose the turns that can be dropped with the least harm to the work that goes on: keep turns that record decisions, requirements, names or open questions, " +
		"and prefer small talk, thanks and turns that later ones replaced. Choose turns whose tokens add up to at least the number that must be freed.",
	Form:       judge.JSONForm[setAsideOut]("an object with turns, a list of the numbers of the turns to drop"),
	Check:      checkSetAside,
	Deadline:   setAsideDeadline,
	InputCap:   setAsideInputCap,
	DefaultOff: true,
	// The rule is the adapter's own: the oldest turns until enough is freed.
	Rule: func(in setAsideIn) setAsideOut {
		var out setAsideOut
		freed := 0
		for _, t := range in.Turns {
			if freed >= in.Needed {
				break
			}
			out.Turns = append(out.Turns, t.N)
			freed += t.Cost
		}
		return out
	},
})

func init() {
	says := func(n int, said, answered string, cost int) setAsideTurn {
		return setAsideTurn{N: n, Said: said, Answered: answered, Cost: cost}
	}
	judge.DeclareCase(setAsideKind, "an early decision", setAsideIn{
		Needed: 2500,
		Turns: []setAsideTurn{
			says(1, "Hi, can you help me with the exporter?", "Of course. What does it need to do?", 900),
			says(2, "Decision: the public API must stay backwards compatible, so never remove or rename a field.", "Understood, I will only add fields.", 700),
			says(3, "Thanks, that is clear.", "You are welcome.", 800),
			says(4, "What is the weather like in Singapore?", "I cannot check the weather, but it is usually warm.", 600),
			says(5, "Ok, never mind that. Add the CSV writer.", "Done, the CSV writer is in exportd/csv.go.", 900),
			says(6, "Thanks again.", "Any time.", 700),
			says(7, "Now add a flag for the delimiter.", "Added a -delimiter flag.", 800),
		},
	}, "turns that free at least 2500 tokens and do not include turn 2, which records the decision to keep the API backwards compatible",
		func(out setAsideOut) bool { return !slices.Contains(out.Turns, 2) })
}

// judgeSetAside asks the judge which of the turns offered to set aside to free
// needed tokens. It returns the numbers chosen, in the order the judge gave them,
// and whether the judge's answer was used; when it was not, the caller leaves the
// choice to the adapter's rule. The judge is shown as many of the oldest turns as
// fit the cap, and a call whose shown turns cannot free what is needed is not made.
func (a *App) judgeSetAside(chat string, needed int, offered []setAsideTurn) ([]int, bool) {
	if needed <= 0 || len(offered) == 0 {
		return nil, false
	}
	shown, size, freed := 0, len(`{"turns":[],"needed":0000000}`), 0
	for _, t := range offered {
		encoded, _ := json.Marshal(t)
		if size+len(encoded)+1 > setAsideInputCap {
			break
		}
		size += len(encoded) + 1
		freed += t.Cost
		shown++
	}
	if shown == 0 || freed < needed {
		return nil, false
	}
	set, err := a.judges()
	if err != nil {
		return nil, false
	}
	r := judge.Call(context.Background(), set, setAsideKind, setAsideIn{Turns: offered[:shown], Needed: needed}, judge.Effect{Kind: "chat", ID: chat})
	if r.Used != judge.UsedJudge {
		return nil, false
	}
	return r.Out.Turns, true
}
