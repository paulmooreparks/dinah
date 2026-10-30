package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"studio/internal/judge"
)

// The title is made from the person's own words: the first block that Dinah did
// not add, cleaned as a title is.
func TestTheFirstLineOfAPromptIsThePersonsOwn(t *testing.T) {
	secret := "dinah-applet-v1:5QAHWLZE3I6FEG3ZDG634FNDCD.53G4C65FBRVPZX77KYPVNNDMAH"
	for _, c := range []struct{ text, want string }{
		{"Please tag the parser release\nand tell Docs.", "Please tag the parser release"},
		{"[Dinah Applet Context]\nuse " + secret + "\n\nWrite the changelog.\n\nThanks.", "Write the changelog."},
		{"\n\n  Spaced   out  \nnext", "Spaced out"},
		{"[Dinah chat: x]", ""},
		{"", ""},
	} {
		if got := firstLineOfPrompt(c.text); got != c.want {
			t.Errorf("firstLineOfPrompt(%q) = %q, want %q", c.text, got, c.want)
		}
	}
	if !isCutOf("Please tag the parser…", "Please tag the parser release now") || !isCutOf("Please tag", "Please tag the parser") ||
		isCutOf("Release notes for Docs", "Please tag the parser release") || isCutOf("", "x") || isCutOf("…", "x") {
		t.Error("isCutOf is wrong")
	}
}

// titled opens a chat and sends its first message, and waits for the turn.
func titled(t *testing.T, a *App, turns chan map[string]any, text string) string {
	t.Helper()
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Send(info.ID, text, nil, nil); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	return info.ID
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(testWait)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func titleApp(t *testing.T) (*App, chan map[string]any) {
	t.Helper()
	a, events, _ := judgementApp(t)
	turns := make(chan map[string]any, 16)
	inner := a.sink
	a.sink = func(name string, data any) {
		inner(name, data)
		if name == "agent:turn" {
			turns <- data.(map[string]any)
		}
	}
	_ = events
	return a, turns
}

// With a model chosen, the first message of a chat is judged once, off the
// sending path, and the chat is shown the judged title wherever its titles are
// read; with the judge failing, nothing changes.
func TestTheFirstMessageOfAChatIsTitledByTheJudge(t *testing.T) {
	a, turns := titleApp(t)
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	id := titled(t, a, turns, "Please tag the parser release\nand tell Docs.")
	s := a.session(id)
	waitFor(t, "the judged title", func() bool { return s.Title() == "Judged line" })
	rows, _ := a.JudgementRecords(JudgementFilter{EffectKind: "chat", EffectID: id})
	if len(rows.Records) != 1 || rows.Records[0].Kind != "chat-title" || rows.Records[0].Used != "judge" || rows.Records[0].Input != `{"line":"Please tag the parser release"}` ||
		rows.Records[0].RuleAnswer != `"Please tag the parser release"` || rows.Records[0].Out != `"Judged line"` {
		t.Fatalf("records %+v", rows.Records)
	}
	jt, ok := a.judgedTitle(id)
	if !ok || jt.Title != "Judged line" || jt.Line != "Please tag the parser release" || jt.Call != rows.Records[0].ID {
		t.Fatalf("kept %+v %v", jt, ok)
	}
	// The titles are read through one path, which shows the judged title for the
	// provider's first-line title and leaves a title the provider wrote alone.
	if got := a.sessionTitle(id, "Please tag the parser release"); got != "Judged line" {
		t.Fatalf("the list shows %q", got)
	}
	if got := a.sessionTitle(id, "Please tag the parser…"); got != "Judged line" {
		t.Fatalf("the cut first line shows %q", got)
	}
	if got := a.sessionTitle(id, "Parser release checklist"); got != "Parser release checklist" {
		t.Fatalf("the provider's own title shows %q", got)
	}
	// A second message is not judged again.
	if err := a.Send(id, "And one more thing.", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	if rows, _ := a.JudgementRecords(JudgementFilter{Kind: "chat-title"}); len(rows.Records) != 1 {
		t.Fatalf("%d title calls after a second message", len(rows.Records))
	}
	// A restart keeps it: a new window reads the judged title with the settings.
	second := NewApp()
	second.sink = func(string, any) {}
	if got := second.sessionTitle(id, "Please tag the parser release"); got != "Judged line" {
		t.Fatalf("after a restart the list shows %q", got)
	}
}

// A late or wrong judge, or none, leaves the title as it is, and the call is
// recorded with the rule's answer.
func TestWithoutAUsableJudgeTheTitleIsTheRule(t *testing.T) {
	a, turns := titleApp(t)
	id := titled(t, a, turns, "Please tag the parser release")
	waitFor(t, "the record", func() bool {
		rows, _ := a.JudgementRecords(JudgementFilter{EffectID: id})
		return len(rows.Records) == 1
	})
	rows, _ := a.JudgementRecords(JudgementFilter{EffectID: id})
	if rows.Records[0].Used != "rule" || rows.Records[0].Why != "no-judge" || rows.Records[0].Out != `"Please tag the parser release"` {
		t.Fatalf("record %+v", rows.Records[0])
	}
	if _, ok := a.judgedTitle(id); ok || a.session(id).Title() != "" {
		t.Fatal("a title was made with no judge")
	}
	if got := a.sessionTitle(id, "Please tag the parser release"); got != "Please tag the parser release" {
		t.Fatalf("the list shows %q", got)
	}
}

// A chat the person has named is not judged, and neither is a first message of
// files only.
func TestANamedChatAndAChatOfFilesAreNotJudged(t *testing.T) {
	a, turns := titleApp(t)
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	named, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.RenameSession(named.ID, "My own name"); err != nil {
		t.Fatal(err)
	}
	if err := a.Send(named.ID, "Please tag the parser release", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	files, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Send(files.ID, "[Dinah chat: window-x]", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitTurn(t, turns)
	time.Sleep(200 * time.Millisecond)
	if rows, _ := a.JudgementRecords(JudgementFilter{Kind: "chat-title"}); len(rows.Records) != 0 {
		t.Fatalf("%d title calls for a named chat and a chat of files", len(rows.Records))
	}
	if a.shownTitle(a.session(named.ID)) != "My own name" {
		t.Fatal("the person's name was replaced")
	}
}

// Correcting a judged title renames the chat to the person's words, so their name
// wins everywhere, and the record keeps what they wrote; a correction that cannot
// be applied is not recorded.
func TestCorrectingAJudgedTitleRenamesTheChat(t *testing.T) {
	a, turns := titleApp(t)
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	id := titled(t, a, turns, "Please tag the parser release")
	s := a.session(id)
	waitFor(t, "the judged title", func() bool { return s.Title() == "Judged line" })
	rows, _ := a.JudgementRecords(JudgementFilter{EffectID: id})
	rec, err := a.CorrectJudgement(rows.Records[0].ID, "Parser release")
	if err != nil || rec.Correction != "Parser release" || rec.CorrectedAt == nil {
		t.Fatalf("%+v %v", rec, err)
	}
	if got := a.shownTitle(s); got != "Parser release" || !strings.Contains(a.chatTitle(s), "Parser release") {
		t.Fatalf("the chat is shown as %q", got)
	}
	// A decision that was about no chat cannot be applied, and is not recorded.
	set, _ := a.judges()
	other := judgeRecordWithoutAChat(t, a, set)
	if _, err := a.CorrectJudgement(other, "x"); err == nil {
		t.Fatal("a correction that could not be applied was accepted")
	}
	if r, _ := set.Store().Get(other); r.Correction != "" {
		t.Fatalf("it was recorded: %+v", r)
	}
}

// judgeRecordWithoutAChat makes a chat-title decision that names no chat.
func judgeRecordWithoutAChat(t *testing.T, a *App, set *judge.Judges) int64 {
	t.Helper()
	r := judge.Call(context.Background(), set, chatTitleKind, chatTitleIn{Line: "A line"}, judge.Effect{})
	if r.ID == 0 {
		t.Fatal("no record")
	}
	return r.ID
}
