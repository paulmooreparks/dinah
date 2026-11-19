package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"studio/internal/appletcontext"
	"studio/internal/judge"
)

// A suggested reply (docs/plans/reply-suggestions.md). When a turn ends in a
// chat that has a window, the page asks for what the person might say next,
// and shows the answer over the empty prompt box, where Tab takes it. The
// call is made here, after the turn has ended and never on the path of
// sending or of typing. The rule proposes nothing, so with no judge, or a
// late or wrong one, the box is as it always is.
//
// The kind is on wherever a judge is set up (Paul, 19 November 2026). A call
// is made for every turn that ends in a chat with a window, which costs
// nothing on a local model and a great deal on a hosted agent program
// (docs/plans/judgement-agents.md has the measurements); the weekly budget in
// Settings bounds it, and the kind's own switch turns it off.

// replySuggestionIn is everything a judge is shown to propose a reply.
type replySuggestionIn struct {
	You   string `json:"you" doc:"what the person last wrote in the chat, with anything the program added cut off, at most 600 characters from its start"`
	Agent string `json:"agent" doc:"what the agent answered after that, at most 2,000 characters from its end"`
}

// replySuggestionOut is the judge's answer. An empty reply proposes nothing.
type replySuggestionOut struct {
	Reply string `json:"reply"`
}

const replySuggestionKindName = "reply-suggestion"

// The longest suggestion, in characters and in lines. A reply longer than
// this is the person's to write.
const (
	replySuggestionLimit = 400
	replySuggestionLines = 6
	replyYouLimit        = 600
	replyAgentLimit      = 2000
)

var replySuggestionKind = judge.Declare(judge.Kind[replySuggestionIn, replySuggestionOut]{
	Name:     replySuggestionKindName,
	Describe: "Proposes no reply, so the prompt box shows its ordinary words. With a judge, one call is made each time a turn ends in a chat that has a window.",
	System: "You propose what a person would most likely write next in a chat with a software agent. You are shown the person's last message and the agent's answer to it. " +
		"Write the reply the person would send: short, in the person's own voice and language, and answering what the agent asked. " +
		"Where the agent asked numbered questions, answer each by its number on a line of its own. " +
		"Where the agent asked nothing and no next step is plain from what it wrote, propose nothing. " +
		"Never put in a fact, a name, a number or a decision that is not in what you are shown.",
	Form: judge.JSONForm[replySuggestionOut](`an object with the one field "reply": the reply as plain text, or an empty string to propose nothing`),
	Check: func(in replySuggestionIn, out replySuggestionOut) error {
		reply := strings.TrimSpace(out.Reply)
		switch {
		case len([]rune(reply)) > replySuggestionLimit:
			return errors.New("the reply is longer than a suggestion may be")
		case strings.Count(reply, "\n") >= replySuggestionLines:
			return errors.New("the reply has more lines than a suggestion may have")
		case strings.Contains(reply, "[Dinah ") || appletcontext.Contains(reply):
			return errors.New("the reply holds text that was added to a prompt")
		}
		return nil
	},
	// Nobody waits for a suggestion, so it may take a while: a hosted agent
	// program as the judge took four to seven seconds on 19 November 2026 and
	// was late at ten.
	Deadline: 20 * time.Second,
	Rule:     func(replySuggestionIn) replySuggestionOut { return replySuggestionOut{} },
	InputCap: 8000,
})

func init() {
	judge.DeclareCase(replySuggestionKind, "a yes-or-no question",
		replySuggestionIn{You: "Rename the launcher's entry for the default shell to Terminal.", Agent: "Done. The entry is called Terminal and stands in alphabetical order with the shells.\n\nShall I rename the pinned one too?"},
		"a short reply that says yes or no to renaming the pinned one",
		func(out replySuggestionOut) bool {
			r := strings.ToLower(out.Reply)
			return r != "" && len([]rune(r)) <= 120 && (strings.Contains(r, "yes") || strings.Contains(r, "no") || strings.Contains(r, "rename") || strings.Contains(r, "leave"))
		})
	judge.DeclareCase(replySuggestionKind, "two numbered questions",
		replySuggestionIn{You: "Where do the two libraries stand?", Agent: "Neither has been asked for anything yet.\n\n1. May I send the two requests now?\n2. Do you want the old chat deleted or renamed?"},
		"a reply that answers both questions by their numbers",
		func(out replySuggestionOut) bool {
			return strings.Contains(out.Reply, "1") && strings.Contains(out.Reply, "2") && strings.Contains(out.Reply, "\n")
		})
	judge.DeclareCase(replySuggestionKind, "nothing to answer",
		replySuggestionIn{You: "Thanks.", Agent: "You are welcome."},
		"an empty reply, or one of a few words",
		func(out replySuggestionOut) bool { return len([]rune(strings.TrimSpace(out.Reply))) <= 40 })
}

// ReplySuggestion is a suggestion as the page takes it: the words, and the
// record of the call, by which the page says what became of it.
type ReplySuggestion struct {
	Text string `json:"text"`
	Call int64  `json:"call"`
}

// Outcomes of a suggestion (judge.Store.SetOutcome).
const (
	replyTaken      = "taken"
	replyDropped    = "dropped"
	replyPassedOver = "passed-over"
)

// replySuggesting keeps the calls under way, one for a chat at a time, so the
// page's two askings for one chat (a turn that ended, a window that opened)
// are one call.
type replySuggesting struct {
	mu      sync.Mutex
	running map[string]*replyCall
}

type replyCall struct {
	done chan struct{}
	out  ReplySuggestion
}

// judgeKindOn says a kind would be answered by a judge now. A call for a
// kind that is off is recorded as one, which for a kind asked at every turn's
// end would fill the record with nothing.
func (a *App) judgeKindOn(kind string) bool {
	set, err := a.judges()
	if err != nil {
		return false
	}
	a.judging.mu.Lock()
	closing := a.judging.closing
	a.judging.mu.Unlock()
	return !closing && set.Answers(kind)
}

// replyInput is what a judge is shown for a chat as its saved conversation
// stands: the person's last message and what the agent answered after it.
// ok is false where there is nothing to reply to.
func replyInput(messages []ConversationMessage) (in replySuggestionIn, ok bool) {
	asked := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "you" {
			asked = i
			break
		}
	}
	if asked < 0 {
		return in, false
	}
	var answer []string
	for _, m := range messages[asked+1:] {
		if m.Role == "agent" && strings.TrimSpace(m.Text) != "" {
			answer = append(answer, strings.TrimSpace(m.Text))
		}
	}
	if len(answer) == 0 {
		return in, false
	}
	in.You = head(personWords(messages[asked].Text), replyYouLimit)
	in.Agent = tail(strings.Join(answer, "\n\n"), replyAgentLimit)
	return in, in.Agent != ""
}

// personWords is the person's own words in a prompt: its blocks without
// those the program added.
func personWords(text string) string {
	var kept []string
	for _, block := range strings.Split(text, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "[Dinah ") || appletcontext.Contains(block) {
			continue
		}
		kept = append(kept, block)
	}
	return strings.Join(kept, "\n\n")
}

func head(text string, limit int) string {
	if r := []rune(text); len(r) > limit {
		return string(r[:limit])
	}
	return text
}

func tail(text string, limit int) string {
	if r := []rune(text); len(r) > limit {
		return string(r[len(r)-limit:])
	}
	return text
}

// SuggestReply proposes the person's next message in a chat, or nothing. The
// page asks when a turn ends in a chat that has a window, and when a chat's
// window opens. One call is made for a turn: a suggestion already made for
// the conversation as it stands is given again from the record, and one the
// person has taken, dropped or passed over is not made again.
func (a *App) SuggestReply(id string) (ReplySuggestion, error) {
	if !a.judgeKindOn(replySuggestionKindName) {
		return ReplySuggestion{}, nil
	}
	s := a.session(id)
	if s == nil || s.State().Working {
		return ReplySuggestion{}, nil
	}
	tr, err := readTranscript(id)
	if err != nil {
		return ReplySuggestion{}, nil
	}
	in, ok := replyInput(tr.Messages)
	if !ok {
		return ReplySuggestion{}, nil
	}
	j := &a.replies
	j.mu.Lock()
	if call := j.running[id]; call != nil {
		j.mu.Unlock()
		<-call.done
		return call.out, nil
	}
	a.judging.mu.Lock()
	closing := a.judging.closing
	if !closing {
		a.judging.work.Add(1)
	}
	a.judging.mu.Unlock()
	if closing {
		j.mu.Unlock()
		return ReplySuggestion{}, nil
	}
	defer a.judging.work.Done()
	call := &replyCall{done: make(chan struct{})}
	if j.running == nil {
		j.running = map[string]*replyCall{}
	}
	j.running[id] = call
	j.mu.Unlock()
	defer func() {
		j.mu.Lock()
		delete(j.running, id)
		j.mu.Unlock()
		close(call.done)
	}()
	call.out = a.suggestReply(id, in)
	// A turn that started while the judge worked makes the answer out of date.
	if s := a.session(id); s == nil || s.State().Working {
		call.out = ReplySuggestion{}
	}
	return call.out, nil
}

// suggestReply gives the suggestion the record already has for this input,
// or asks the judge for one.
func (a *App) suggestReply(id string, in replySuggestionIn) ReplySuggestion {
	set, err := a.judgesForCall(replySuggestionKindName)
	if err != nil {
		return ReplySuggestion{}
	}
	rendered, _ := json.Marshal(in)
	if recs, _, err := set.Store().List(judge.Filter{Kind: replySuggestionKindName, EffectKind: "chat", EffectID: id, Limit: 1}); err == nil && len(recs) == 1 &&
		recs[0].Input == appletcontext.Redact(string(rendered)) && recs[0].Used == judge.UsedJudge {
		var out replySuggestionOut
		if outcome, err := set.Store().Outcome(recs[0].ID); err != nil || outcome != "" || json.Unmarshal([]byte(recs[0].Out), &out) != nil {
			return ReplySuggestion{}
		}
		return ReplySuggestion{Text: strings.TrimSpace(out.Reply), Call: recs[0].ID}
	}
	r := judge.Call(context.Background(), set, replySuggestionKind, in, judge.Effect{Kind: "chat", ID: id})
	if r.Used != judge.UsedJudge || r.ID == 0 {
		return ReplySuggestion{}
	}
	a.emitJudgementsChanged()
	return ReplySuggestion{Text: strings.TrimSpace(r.Out.Reply), Call: r.ID}
}

// ReplySuggestionOutcome records that the person took a suggestion (Tab) or
// dropped it (Esc). The record is how the share of suggestions that are
// taken is measured.
func (a *App) ReplySuggestionOutcome(id string, call int64, outcome string) error {
	if outcome != replyTaken && outcome != replyDropped {
		return errors.New("a suggestion is taken or dropped")
	}
	return a.setReplyOutcome(id, call, outcome)
}

func (a *App) setReplyOutcome(id string, call int64, outcome string) error {
	set, err := a.judges()
	if err != nil {
		return err
	}
	rec, err := set.Store().Get(call)
	if err != nil || rec.Kind != replySuggestionKindName || rec.EffectID != id {
		return errors.New("there is no such suggestion for this chat")
	}
	return set.Store().SetOutcome(call, outcome)
}

// noteReplyPassedOver is called when a message starts a turn in a chat. A
// suggestion that was made for the chat and neither taken nor dropped was
// passed over for what the person wrote instead.
func (a *App) noteReplyPassedOver(id string) {
	if !a.judgeKindOn(replySuggestionKindName) {
		return
	}
	a.judging.mu.Lock()
	closing := a.judging.closing
	if !closing {
		a.judging.work.Add(1)
	}
	a.judging.mu.Unlock()
	if closing {
		return
	}
	go func() {
		defer a.judging.work.Done()
		set, err := a.judges()
		if err != nil {
			return
		}
		recs, _, err := set.Store().List(judge.Filter{Kind: replySuggestionKindName, EffectKind: "chat", EffectID: id, Limit: 1})
		if err != nil || len(recs) != 1 || recs[0].Used != judge.UsedJudge {
			return
		}
		var out replySuggestionOut
		if json.Unmarshal([]byte(recs[0].Out), &out) != nil || strings.TrimSpace(out.Reply) == "" {
			return
		}
		_ = set.Store().SetOutcome(recs[0].ID, replyPassedOver)
	}()
}
