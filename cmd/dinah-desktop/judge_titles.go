package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"studio/internal/appletcontext"
	"studio/internal/bus"
	"studio/internal/judge"
	"studio/internal/sessionhost"
)

// Chat titles are the first kind of judgement call (docs/plans/
// judgement-agents.md, step 2). When the person's first message in a chat starts
// its turn, a judge writes a short title from the first line of it, on its own
// goroutine and never on the sending path. The rule is the cleaned first line,
// which is the title the chat has today, so with no judge, or a late or wrong
// one, nothing changes.
//
// A judged title is kept beside the first line it came from. The title shown for
// a chat is the name the person gave it, else the judged title while the
// provider's own title is still a cut of that first line, else the provider's
// title, so a provider that writes its own title later is untouched.

// chatTitleIn is everything a judge is shown to title a chat.
type chatTitleIn struct {
	Line string `json:"line" doc:"the first line of what the person first wrote in the chat, with anything the program added cut off, at most 120 characters"`
}

const chatTitleKindName = "chat-title"

// chatTitleLimit is the longest title a judge may give.
const chatTitleLimit = 60

var chatTitleKind = judge.Declare(judge.Kind[chatTitleIn, string]{
	Name:        chatTitleKindName,
	Describe:    "Uses the first line of the person's first message, cut short.",
	Correctable: true,
	System:      "You write a short title for a chat from the first line of the person's first message. Answer with the title only, in the person's language, without quotation marks or a full stop.",
	Form:        judge.TextForm(chatTitleLimit),
	Check: func(in chatTitleIn, out string) error {
		if strings.Contains(out, "[Dinah ") || appletcontext.Contains(out) {
			return errors.New("the title holds text that was added to a prompt")
		}
		return nil
	},
	Deadline: 3 * time.Second,
	Rule:     func(in chatTitleIn) string { return bus.CleanTitle(in.Line) },
})

// JudgedTitle is a title a judge wrote, with the first line it was written from
// and the record of the call.
type JudgedTitle struct {
	Line  string `json:"line"`
	Title string `json:"title"`
	Call  int64  `json:"call"`
}

// firstLineOfPrompt is the first line of the person's words in a prompt: the first
// block of text that is not something Dinah added, cleaned as a title is.
func firstLineOfPrompt(text string) string {
	for _, block := range strings.Split(text, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "[Dinah ") || appletcontext.Contains(block) {
			continue
		}
		if line := bus.CleanTitle(block); line != "" {
			return line
		}
	}
	return ""
}

// isCutOf says a title is a cut of a line: the line, or its start with an
// ellipsis, which is how a title made from a message is shortened.
func isCutOf(title, line string) bool {
	t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(title), "…"))
	return t != "" && strings.HasPrefix(line, t)
}

// judgedTitle is the judged title kept for a chat.
func (a *App) judgedTitle(id string) (JudgedTitle, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	jt, ok := a.settings.JudgedTitles[id]
	return jt, ok
}

// withJudgedTitle is the title to show for a chat whose provider's title is
// title: the judged title while the provider's is still a cut of the first line
// it came from, or not yet there, and the provider's otherwise.
func (a *App) withJudgedTitle(id, title string) string {
	if jt, ok := a.judgedTitle(id); ok && (title == "" || isCutOf(title, jt.Line)) {
		return jt.Title
	}
	return title
}

// noteFirstPrompt is called before the person's prompt is recorded. It starts the
// judgement of the chat's title when this is the person's first message in a
// chat the person has not named, and does nothing otherwise, which includes a
// first message of files only and a chat that was titled before this run.
func (a *App) noteFirstPrompt(s *Session, p *prompt) {
	if len(p.Delivered()) > 0 || p.Label() != "" {
		return
	}
	id := s.ID()
	a.mu.Lock()
	named := a.settings.SessionTitles[id] != ""
	_, judged := a.settings.JudgedTitles[id]
	a.mu.Unlock()
	if named || judged {
		return
	}
	j := &a.judging
	j.mu.Lock()
	if j.seen == nil {
		j.seen = map[string]bool{}
	}
	already := j.seen[id]
	j.seen[id] = true
	j.mu.Unlock()
	if already || a.hasEarlierPerson(s) {
		return
	}
	line := firstLineOfPrompt(p.Text())
	if line == "" {
		return
	}
	go a.judgeChatTitle(id, line)
}

// hasEarlierPerson says the chat's saved conversation already holds a message
// the person wrote, so the one being sent is not the first.
func (a *App) hasEarlierPerson(s *Session) bool {
	a.cmu.Lock()
	tr := s.HistorySavedLocked()
	a.cmu.Unlock()
	if tr == nil {
		return false
	}
	for _, m := range tr.Messages {
		if m.Role == "you" {
			return true
		}
	}
	return false
}

// judgeChatTitle asks the judge for the title and, when the judge's answer was
// used, keeps it and shows it.
func (a *App) judgeChatTitle(id, line string) {
	set, err := a.judges()
	if err != nil {
		return
	}
	r := judge.Call(context.Background(), set, chatTitleKind, chatTitleIn{Line: line}, judge.Effect{Kind: "chat", ID: id})
	if r.Used != judge.UsedJudge || r.ID == 0 {
		return
	}
	err = a.updateSettings(func(s *Settings) error {
		if s.SessionTitles[id] != "" {
			return nil
		}
		if s.JudgedTitles == nil {
			s.JudgedTitles = map[string]JudgedTitle{}
		}
		s.JudgedTitles[id] = JudgedTitle{Line: line, Title: r.Out, Call: r.ID}
		return nil
	})
	if err != nil {
		logf("judge", "warning", "the judged title of chat %s could not be kept: %v", id, err)
		return
	}
	if s := a.session(id); s != nil {
		title := a.withJudgedTitle(id, s.Title())
		s.UpdateDisplay(&title, nil)
		sessionhost.EmitSession(a.emit, s)
		a.busCoordinator().PublishBusTitle(s)
	}
}

// correctors apply a person's correction to what a kind of decision was about.
// The record keeps the correction whether or not there is one.
var correctors = map[string]func(a *App, rec judge.Record, value string) error{
	chatTitleKindName: func(a *App, rec judge.Record, value string) error {
		if rec.EffectKind != "chat" || rec.EffectID == "" {
			return errors.New("this decision was not about a chat")
		}
		_, err := a.RenameSession(rec.EffectID, value)
		return err
	},
}
