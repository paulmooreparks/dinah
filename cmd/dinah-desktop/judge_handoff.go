package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"studio/internal/appletcontext"
	"studio/internal/judge"
)

// The summary of the older messages in a handoff is the third kind of judgement
// call (docs/plans/judgement-agents.md, step 6). When a chat is continued with
// another provider, or compacted with its own, the text the new chat starts from
// keeps the newest messages whole and stands for the older ones with one line
// each. A judge may write that part instead. It is shown the same lines and
// nothing else, the newest messages are never shown to it, and its summary takes
// no more room than the lines would, a quarter of the provider's handoff budget,
// so the handed text stays within the budget exactly as before. The saved summary
// kept beside the chat for later switches stays the rule's, so a switch never
// needs a model.

// handoffIn is everything a judge is shown to summarise the older messages.
type handoffIn struct {
	Lines []string `json:"lines" doc:"one line for each earlier message of the chat that no longer fits, oldest first: who said it and the first 160 characters of what was said"`
	Limit int      `json:"limit" doc:"the most characters the summary may take"`
}

const (
	// handoffInputCap is the most bytes of lines a judge is shown, about 3,000 tokens.
	// It cannot come from the provider's handoff budget, which is far too large for a
	// small judge, and turning characters into tokens would be an estimate. Lines past
	// it are left out of what the judge sees, and the text says how many.
	handoffInputCap = 12_000
	// handoffDeadline is how long the person waits for the summary: they asked for the
	// continuation, and a late answer costs them this much and gives them the rule's.
	handoffDeadline = 10 * time.Second
)

var handoffKind = judge.Declare(judge.Kind[handoffIn, string]{
	Name:     "handoff-summary",
	Describe: "Writes one line for each earlier message, as many as fit.",
	System: "You summarise the start of a conversation between a person and an AI agent, so that a new agent can carry on the work. " +
		"You are given one line for each earlier message, oldest first. Write a short plain-text summary of what was asked, what was decided and what was done, " +
		"keeping every decision, name, file path and open question, in no more characters than the limit. Add nothing that is not in the lines.",
	Form: judge.ParagraphsForm(),
	Check: func(in handoffIn, out string) error {
		if len(out) > in.Limit {
			return errors.New("the summary is longer than the room it may take")
		}
		if strings.Contains(out, "[Dinah ") || appletcontext.Contains(out) {
			return errors.New("the summary holds text that was added to a prompt")
		}
		return nil
	},
	Deadline:   handoffDeadline,
	InputCap:   handoffInputCap,
	DefaultOff: true,
	Rule: func(in handoffIn) string {
		var b strings.Builder
		for _, line := range in.Lines {
			row := "- " + line + "\n"
			if b.Len()+len(row) > in.Limit {
				break
			}
			b.WriteString(row)
		}
		return strings.TrimSpace(b.String())
	},
})

func init() {
	judge.DeclareCase(handoffKind, "an early decision", handoffIn{
		Limit: 700,
		Lines: []string{
			"[Person] Let's build the exporter as a separate module called exportd, and not inside the server.",
			"[Agent] Understood. I will make exportd its own module with its own go.mod and keep the server unchanged.",
			"[Person] It should write CSV first. JSON can wait.",
			"[Agent] I created exportd/main.go and the CSV writer, and ran the tests, which pass.",
			"[Person] Add a flag for the delimiter.",
			"[Agent] Added a -delimiter flag, defaulting to a comma, with a test for tabs.",
			"[Person] Good. Now document it in the README.",
			"[Agent] I added a short README section for exportd with two examples.",
		},
	}, "a summary that keeps the early decision to build the exporter as its own module called exportd", func(out string) bool {
		return strings.Contains(strings.ToLower(out), "exportd")
	})
}

// handoffCacheLimit is how many judged summaries are kept, so that what a person is
// shown for a continuation is what is handed over when they approve it, even if the
// judge could answer differently the second time. They are kept for the run.
const handoffCacheLimit = 16

type handoffEntry struct {
	summary string
	shown   int
	ok      bool
}

// forgetHandoffs drops the kept summaries, since a change in the settings, such as
// turning the kind on or choosing another model, can change what the judge says.
func (a *App) forgetHandoffs() {
	j := &a.judging
	j.mu.Lock()
	j.handoffs = nil
	j.mu.Unlock()
}

// judgeHandoffDigest is the judge's summary of the older lines of a chat being
// continued, for a handoff. The judge is shown as many of the lines as fit
// handoffInputCap and no more, and shown says how many. It is never called with
// a lock held. A result, including that there is none, is kept for the same lines
// and limit, so that a preview and the continuation it leads to agree.
func (a *App) judgeHandoffDigest(chat string, lines []string, limit int) (string, int, bool) {
	if limit <= 0 || len(lines) == 0 {
		return "", 0, false
	}
	shown := 0
	size := len(`{"lines":[],"limit":000000}`)
	for _, line := range lines {
		encoded, _ := json.Marshal(line)
		if size+len(encoded)+1 > handoffInputCap {
			break
		}
		size += len(encoded) + 1
		shown++
	}
	if shown == 0 {
		return "", 0, false
	}
	sum := sha256.New()
	for _, line := range lines[:shown] {
		sum.Write([]byte(line))
		sum.Write([]byte{0})
	}
	sum.Write([]byte(chat))
	key := hex.EncodeToString(sum.Sum(nil)) + "/" + strconv.Itoa(limit)
	j := &a.judging
	j.mu.Lock()
	cached, hit := j.handoffs[key]
	j.mu.Unlock()
	if hit {
		return cached.summary, cached.shown, cached.ok
	}
	set, err := a.judges()
	if err != nil {
		return "", 0, false
	}
	r := judge.Call(a.bindingContext(), set, handoffKind, handoffIn{Lines: lines[:shown], Limit: limit}, judge.Effect{Kind: "chat", ID: chat})
	entry := handoffEntry{summary: r.Out, shown: shown, ok: r.Used == judge.UsedJudge}
	j.mu.Lock()
	if j.handoffs == nil || len(j.handoffs) >= handoffCacheLimit {
		j.handoffs = map[string]handoffEntry{}
	}
	j.handoffs[key] = entry
	j.mu.Unlock()
	return entry.summary, entry.shown, entry.ok
}
