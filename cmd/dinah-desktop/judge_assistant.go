package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"studio/internal/judge"
)

// The window assistant's reading of a request
// (docs/plans/four-requests-2026-11-20.md, docs/mockups/window-assistant).
//
// The assistant is a small place in the tray where the person says what they
// want of the window in their own words: "find all the windows for the
// pricing work", "put the documents side by side", "close what is
// finished". What they said, and the list of the windows that are open, go
// to a judge, which answers with one thing to do from a short list and the
// windows to do it to. The page carries that out with the window layer's
// own operations, each of which the person could do by hand, and can undo
// it. No window's content is sent but its title, its kind, its workspace and
// its state.
//
// With no judge, or a late or wrong one, the rule answers: it reads the
// request by its words and finds windows by the words in their titles. So
// the assistant works with judgements off, and better with them on, where
// "the pricing work" also finds a window called "Discount Flow".

// AssistantWindow is one open window as the assistant is told of it.
type AssistantWindow struct {
	ID        string `json:"id" doc:"the window's key, which an answer names it by"`
	Kind      string `json:"kind" doc:"what the window is: chat, document, diagram, slides, terminal, files and so on"`
	Title     string `json:"title" doc:"the window's title"`
	Workspace string `json:"workspace,omitempty" doc:"the name of the workspace the window belongs to, when it has one"`
	// State is "working" for a chat in a turn, "waiting" for a chat that
	// waits for the person, "idle" for a chat whose last turn has ended,
	// "unsaved" for a document with changes not saved, and empty otherwise.
	// Away says the window is put away (minimized).
	State string `json:"state,omitempty" doc:"working for a chat in a turn, waiting for a chat that waits for the person, idle for a chat whose turn has ended, unsaved for a document with changes not saved"`
	Away  bool   `json:"away,omitempty" doc:"true when the window is put away, which is minimized"`
}

type assistantIn struct {
	Ask     string            `json:"ask" doc:"what the person asked of the window, in their own words"`
	Windows []AssistantWindow `json:"windows" doc:"every open window, at most 60"`
}

// AssistantPlan is what to do. Do is one of the assistantDo words, Windows
// the keys of the windows it is done to, in order, and Say one line for the
// person that says what was done or answers their question.
type AssistantPlan struct {
	Do      string   `json:"do"`
	Windows []string `json:"windows"`
	Say     string   `json:"say"`
	// Under is the key of the window that goes under a row of windows put
	// side by side, when one does.
	Under string `json:"under,omitempty"`
	// Judged says a judge read the request, and not the rule.
	Judged bool `json:"judged"`
}

// What the assistant can do. Each is carried out by the page with the window
// layer's own operations (assistant.js).
const (
	assistantGather  = "gather"  // show these windows, tiled, and put the rest away
	assistantTile    = "tile"    // tile the windows that are showing
	assistantWorking = "working" // show only the windows with work in them
	assistantTidy    = "tidy"    // close the chats that are idle and the documents with nothing unsaved
	assistantAll     = "all"     // bring every window back
	assistantSide    = "side"    // these windows side by side, and the one in Under below them
	assistantFront   = "front"   // bring these windows to the front
	assistantSay     = "say"     // do nothing to the windows; Say answers the question
)

var assistantDo = []string{assistantGather, assistantTile, assistantWorking, assistantTidy, assistantAll, assistantSide, assistantFront, assistantSay}

const (
	assistantWindowLimit = 60
	assistantSayLimit    = 400
	assistantAskLimit    = 600
)

const assistantKindName = "window-assistant"

var assistantKind = judge.Declare(judge.Kind[assistantIn, AssistantPlan]{
	Name:     assistantKindName,
	Describe: "Reads a request by its words and finds windows by the words in their titles. With a judge, one call is made for each request typed to the window assistant or run from its tasks.",
	System: "You are the assistant of a desktop workbench. The person asks you to find, gather, arrange or tidy its open windows, or asks a question about them. " +
		"You are given the request and every open window with its id, kind, title, workspace and state. Answer with one thing to do. " +
		"\"gather\": the request names a subject, a project, a ticket or a piece of work. Give the ids of every window that belongs to it, judging by meaning and not only by matching words; the rest are put away, not closed. " +
		"When no window belongs to it, use \"say\" and say that none of the open windows is about it. Never refuse to look. " +
		"\"tile\": arrange the windows that are showing. \"working\": show only windows with work in them. \"tidy\": close what is finished. \"all\": bring every window back. " +
		"\"side\": put windows side by side; give the ids to put in a row, and when one is to go under the others give its id as \"under\" and not in the row. " +
		"\"front\": bring the windows named to the front. \"say\": change nothing and answer the person's question about the windows from what you are given. " +
		"Always give \"say\" one short sentence for the person: what you did and to which windows, by their titles, or the answer to their question. " +
		"Use only ids from the list. Never invent a window.",
	Form: judge.JSONForm[AssistantPlan](`an object with "do" (one of gather, tile, working, tidy, all, side, front, say), "windows" (an array of window ids from the list, empty where the action needs none), "say" (one short sentence for the person) and, only for side, an optional "under" (the id of the window to put below the row)`),
	Check: func(in assistantIn, out AssistantPlan) error {
		known := false
		for _, do := range assistantDo {
			known = known || out.Do == do
		}
		if !known {
			return fmt.Errorf("%q is not something the assistant can do", out.Do)
		}
		have := map[string]bool{}
		for _, w := range in.Windows {
			have[w.ID] = true
		}
		for _, id := range out.Windows {
			if !have[id] {
				return fmt.Errorf("the answer names a window that is not open: %q", id)
			}
		}
		if out.Under != "" && (!have[out.Under] || out.Do != assistantSide) {
			return fmt.Errorf("the answer puts a window under the others that it cannot: %q", out.Under)
		}
		if (out.Do == assistantGather || out.Do == assistantSide || out.Do == assistantFront) && len(out.Windows) == 0 {
			return errors.New("the answer names no window to do it to")
		}
		if strings.TrimSpace(out.Say) == "" || len([]rune(out.Say)) > assistantSayLimit {
			return errors.New("the answer has no line for the person, or one that is too long")
		}
		return nil
	},
	// The person is waiting, so the judge has a few seconds and the rule
	// answers after that.
	Deadline: 12 * time.Second,
	Rule:     assistantRule,
	InputCap: 12000,
})

var assistantWord = regexp.MustCompile(`[a-z0-9]+`)

// assistantSkip are the words of a request that say what to do and not what
// it is about.
var assistantSkip = map[string]bool{"find": true, "all": true, "the": true, "windows": true, "window": true, "for": true, "about": true, "work": true,
	"show": true, "everything": true, "related": true, "to": true, "which": true, "are": true, "is": true, "gather": true, "me": true, "of": true, "on": true,
	"and": true, "a": true, "an": true, "my": true, "with": true, "that": true, "open": true, "in": true, "please": true, "only": true, "just": true, "this": true}

func titlesOf(list []AssistantWindow) string {
	names := make([]string, 0, len(list))
	for _, w := range list {
		names = append(names, w.Title)
	}
	return strings.Join(names, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// assistantRule reads a request without a judge: by the words that say what
// to do, and otherwise by finding the request's other words in the windows'
// titles and workspaces.
func assistantRule(in assistantIn) AssistantPlan {
	ask := strings.ToLower(in.Ask)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(ask, w) {
				return true
			}
		}
		return false
	}
	showing := func(keep func(AssistantWindow) bool) []AssistantWindow {
		var out []AssistantWindow
		for _, w := range in.Windows {
			if !w.Away && (keep == nil || keep(w)) {
				out = append(out, w)
			}
		}
		return out
	}
	ids := func(list []AssistantWindow) []string {
		out := make([]string, 0, len(list))
		for _, w := range list {
			out = append(out, w.ID)
		}
		return out
	}
	switch {
	case has("side by side", "next to each other"):
		docs := showing(func(w AssistantWindow) bool { return w.Kind != "chat" })
		chats := showing(func(w AssistantWindow) bool { return w.Kind == "chat" })
		if len(docs) == 0 {
			return AssistantPlan{Do: assistantSay, Windows: []string{}, Say: "No window that is showing can be put side by side with another."}
		}
		if len(docs) > 3 {
			docs = docs[:3]
		}
		plan := AssistantPlan{Do: assistantSide, Windows: ids(docs), Say: "Put " + titlesOf(docs) + " side by side. The rest are put away, not closed."}
		if has("under", "below") && len(chats) > 0 {
			plan.Under = chats[0].ID
			plan.Say = "Put " + titlesOf(docs) + " side by side with " + chats[0].Title + " under them. The rest are put away, not closed."
		}
		return plan
	case has("every window", "bring everything", "show all", "bring back", "restore"):
		return AssistantPlan{Do: assistantAll, Windows: []string{}, Say: "All " + plural(len(in.Windows), "window is", "windows are") + " showing again."}
	case has("waiting for me", "needs me", "need me"):
		var need []string
		for _, w := range in.Windows {
			switch w.State {
			case "unsaved":
				need = append(need, w.Title+" (not saved)")
			case "waiting":
				need = append(need, w.Title+" (waiting for you)")
			case "idle":
				need = append(need, w.Title+" (finished its turn)")
			}
		}
		if len(need) == 0 {
			return AssistantPlan{Do: assistantSay, Windows: []string{}, Say: "Nothing is waiting for you. No chat is waiting or has finished a turn, and no document has changes not saved."}
		}
		return AssistantPlan{Do: assistantSay, Windows: []string{}, Say: plural(len(need), "window", "windows") + " may need you: " + strings.Join(need, ", ") + "."}
	case has("close what", "close the idle", "close idle", "tidy", "clean up"):
		return AssistantPlan{Do: assistantTidy, Windows: []string{}, Say: "Closed the chats that are idle and the documents with nothing unsaved."}
	case has("with work", "working", "active") && !has("about", "for the"):
		return AssistantPlan{Do: assistantWorking, Windows: []string{}, Say: "Showing only the windows with work in them. The others are put away, not closed."}
	case has("tile", "arrange", "grid"):
		return AssistantPlan{Do: assistantTile, Windows: []string{}, Say: "Tiled the windows that are showing."}
	}
	var topic []string
	for _, w := range assistantWord.FindAllString(ask, -1) {
		if !assistantSkip[w] {
			topic = append(topic, w)
		}
	}
	if len(topic) == 0 {
		return AssistantPlan{Do: assistantSay, Windows: []string{}, Say: "Tell me what to find, gather, arrange or tidy."}
	}
	// A window belongs to the subject when it has every word of it, and with
	// no such window, when it has any.
	match := func(all bool) []AssistantWindow {
		var out []AssistantWindow
		for _, w := range in.Windows {
			hay := strings.ToLower(w.Title + " " + w.Workspace)
			n := 0
			for _, word := range topic {
				if strings.Contains(hay, word) {
					n++
				}
			}
			if (all && n == len(topic)) || (!all && n > 0) {
				out = append(out, w)
			}
		}
		return out
	}
	found := match(true)
	if len(found) == 0 {
		found = match(false)
	}
	name := strings.Join(topic, " ")
	if len(found) == 0 {
		return AssistantPlan{Do: assistantSay, Windows: []string{}, Say: "No open window has " + name + " in its title. I looked at all " + plural(len(in.Windows), "window", "windows") + "."}
	}
	say := "Found " + plural(len(found), "window", "windows") + " about " + name + ": " + titlesOf(found) + ". The rest are put away, not closed."
	if len([]rune(say)) > assistantSayLimit {
		say = "Found " + plural(len(found), "window", "windows") + " about " + name + ". The rest are put away, not closed."
	}
	return AssistantPlan{Do: assistantGather, Windows: ids(found), Say: say}
}

func init() {
	windows := []AssistantWindow{
		{ID: "w1", Kind: "chat", Title: "Pricing Rules for Bundles", State: "working"},
		{ID: "w2", Kind: "document", Title: "Pricing Design.md"},
		{ID: "w3", Kind: "chat", Title: "Release Notes Draft", State: "idle"},
		{ID: "w4", Kind: "diagram", Title: "Refund Flow.d2"},
		{ID: "w5", Kind: "document", Title: "Meeting Notes.md", State: "unsaved"},
	}
	has := func(list []string, id string) bool {
		for _, x := range list {
			if x == id {
				return true
			}
		}
		return false
	}
	judge.DeclareCase(assistantKind, "a subject to find", assistantIn{Ask: "Find all the windows for the pricing work", Windows: windows},
		"gather, with both pricing windows and neither the release notes nor the meeting notes",
		func(out AssistantPlan) bool {
			return out.Do == assistantGather && has(out.Windows, "w1") && has(out.Windows, "w2") && !has(out.Windows, "w3") && !has(out.Windows, "w5")
		})
	judge.DeclareCase(assistantKind, "a subject no window has", assistantIn{Ask: "Find all the windows for the payroll migration", Windows: windows},
		"say, with no windows, and a line that says none is about it",
		func(out AssistantPlan) bool { return out.Do == assistantSay && len(out.Windows) == 0 && out.Say != "" })
	judge.DeclareCase(assistantKind, "an arrangement", assistantIn{Ask: "tile the windows that are showing", Windows: windows},
		"tile",
		func(out AssistantPlan) bool { return out.Do == assistantTile })
	judge.DeclareCase(assistantKind, "tidying", assistantIn{Ask: "close what is finished", Windows: windows},
		"tidy",
		func(out AssistantPlan) bool { return out.Do == assistantTidy })
}

// AskAssistant reads what the person asked of the window and says what to
// do. The page gives the open windows, since it is the page that has them,
// and carries the answer out.
func (a *App) AskAssistant(ask string, windows []AssistantWindow) (AssistantPlan, error) {
	ask = strings.TrimSpace(ask)
	if ask == "" {
		return AssistantPlan{}, errors.New("say what you want of the windows")
	}
	if r := []rune(ask); len(r) > assistantAskLimit {
		ask = string(r[:assistantAskLimit])
	}
	if len(windows) > assistantWindowLimit {
		windows = windows[:assistantWindowLimit]
	}
	if windows == nil {
		windows = []AssistantWindow{}
	}
	in := assistantIn{Ask: ask, Windows: windows}
	set, err := a.judgesForCall(assistantKindName)
	if err != nil {
		return assistantRule(in), nil
	}
	r := judge.Call(context.Background(), set, assistantKind, in, judge.Effect{Kind: "assistant", ID: "window"})
	plan := r.Out
	plan.Judged = r.Used == judge.UsedJudge
	if plan.Windows == nil {
		plan.Windows = []string{}
	}
	return plan, nil
}

// AssistantTask is one of the person's tasks in the assistant's list: a name
// and the words the assistant is asked, the same words a person could type.
type AssistantTask struct {
	Name string `json:"name"`
	Ask  string `json:"ask"`
}

// assistantStarterTasks are the tasks the list begins with. The person can
// change, reorder and delete them, and have them back.
func assistantStarterTasks() []AssistantTask {
	return []AssistantTask{
		{Name: "Tile the Windows That Are Showing", Ask: "tile the windows that are showing"},
		{Name: "Show Only Windows with Work in Them", Ask: "show only the windows with work in them"},
		{Name: "Tidy Up", Ask: "close what is finished"},
		{Name: "Documents Side by Side, Chat Under", Ask: "put the documents side by side and the chat under them"},
		{Name: "Bring Everything Back", Ask: "show every window again"},
		{Name: "What Needs Me?", Ask: "which windows are waiting for me?"},
	}
}

const (
	assistantTaskLimit     = 100
	assistantTaskNameLimit = 80
)

// AssistantTasks are the person's tasks, or the starter set while they have
// changed none.
func (a *App) AssistantTasks() []AssistantTask {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.settings.AssistantTasksSet {
		return assistantStarterTasks()
	}
	return append([]AssistantTask{}, a.settings.AssistantTasks...)
}

// SetAssistantTasks keeps the person's tasks, in their order. An empty list
// is kept as one: the person deleted them all.
func (a *App) SetAssistantTasks(tasks []AssistantTask) ([]AssistantTask, error) {
	if len(tasks) > assistantTaskLimit {
		return nil, fmt.Errorf("the assistant keeps at most %d tasks", assistantTaskLimit)
	}
	kept := make([]AssistantTask, 0, len(tasks))
	for _, t := range tasks {
		t.Name, t.Ask = strings.TrimSpace(t.Name), strings.TrimSpace(t.Ask)
		switch {
		case t.Name == "" || t.Ask == "":
			return nil, errors.New("a task needs a name and what to ask")
		case len([]rune(t.Name)) > assistantTaskNameLimit:
			return nil, fmt.Errorf("a task's name has at most %d characters", assistantTaskNameLimit)
		case len([]rune(t.Ask)) > assistantAskLimit:
			return nil, fmt.Errorf("what a task asks has at most %d characters", assistantAskLimit)
		}
		kept = append(kept, t)
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.AssistantTasks, s.AssistantTasksSet = kept, true
		return nil
	}); err != nil {
		return nil, err
	}
	return a.AssistantTasks(), nil
}

// RestoreAssistantTasks puts the starter set back in place of the person's
// tasks.
func (a *App) RestoreAssistantTasks() ([]AssistantTask, error) {
	if err := a.updateSettings(func(s *Settings) error {
		s.AssistantTasks, s.AssistantTasksSet = nil, false
		return nil
	}); err != nil {
		return nil, err
	}
	return a.AssistantTasks(), nil
}
