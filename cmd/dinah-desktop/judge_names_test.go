package main

import (
	"context"
	"testing"

	"studio/internal/drafts"
	"studio/internal/judge"
)

// A suggested name must be one a file can have.
func TestAFileNameIsCheckedBeforeItIsOffered(t *testing.T) {
	for _, ok := range []string{"Parser release notes", "Notes 2026-10-30", "a.b", "Über Cafe"} {
		if err := checkFileName(ok); err != nil {
			t.Errorf("%q was refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "  ", "a/b", `a\b`, "a:b", "what?", "a*b", `say "hi"`, "<a>", "a|b", "ends.", "ends ", " begins", "con", "NUL", "com1.txt", "Lpt9", "tab\there"} {
		if err := checkFileName(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func newDraftWith(t *testing.T, a *App, text string) string {
	t.Helper()
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		t.Fatal(err)
	}
	d, err := o.Drafts.Store.Create(drafts.Origin{Kind: drafts.OriginNew}, drafts.Baseline{}, "Untitled", &text)
	if err != nil {
		t.Fatal(err)
	}
	return d.ID
}

// The Save As dialog is offered a name made from the first non-empty line of a new
// document, with the call recorded against the draft; with no judge, an empty
// document or an answer no file can have, the name is Untitled, as it is today.
func TestANewDocumentIsOfferedAJudgedName(t *testing.T) {
	a, _, _ := judgementApp(t)
	id := newDraftWith(t, a, "\n\n  # Parser release notes  \nSecond line.\n")
	// No model chosen: the rule.
	got, err := a.SuggestDocumentName(id)
	if err != nil || got.Name != "Untitled" || got.Used != "rule" || got.Call == 0 {
		t.Fatalf("with no judge %+v %v", got, err)
	}
	choose(t, a, func(s *JudgementSettings) { s.Model = "tiny:4b" })
	got, err = a.SuggestDocumentName(id)
	if err != nil || got.Name != "Judged line" || got.Used != "judge" || got.Call == 0 {
		t.Fatalf("with a judge %+v %v", got, err)
	}
	rows, _ := a.JudgementRecords(JudgementFilter{Kind: "document-name", EffectKind: "draft", EffectID: id})
	if len(rows.Records) != 2 || rows.Records[0].Input != `{"line":"# Parser release notes"}` || rows.Records[0].RuleAnswer != `"Untitled"` || rows.Records[0].Out != `"Judged line"` {
		t.Fatalf("records %+v", rows.Records)
	}
	// An empty document has nothing to name it from, and no call is made.
	empty := newDraftWith(t, a, " \n\n")
	before := len(rows.Records)
	if got, _ := a.SuggestDocumentName(empty); got.Name != "Untitled" || got.Used != "rule" || got.Call != 0 {
		t.Fatalf("empty %+v", got)
	}
	if after, _ := a.JudgementRecords(JudgementFilter{Kind: "document-name"}); len(after.Records) != before {
		t.Fatal("a call was made for an empty document")
	}
	if _, err := a.SuggestDocumentName("no-such-draft"); err == nil {
		t.Fatal("a draft that does not exist was named")
	}
}

// answering is a judge whose answer a test sets.
type answering struct{ text string }

func (j *answering) Ask(_ context.Context, _ judge.Request) (judge.Response, error) {
	return judge.Response{Text: j.text, Model: "m"}, nil
}

// A name that no file can have is not offered, whatever the judge says, and a
// document that is not new keeps its own name and costs no call.
func TestAnUnusableJudgedNameFallsBackToUntitled(t *testing.T) {
	a, _, _ := judgementApp(t)
	set, _ := a.judges()
	id := newDraftWith(t, a, "Parser release notes")
	for _, bad := range []string{"../secret", "CON", "What now?", "ends."} {
		set.SetJudge(&answering{bad}, "test", "tokens")
		got, err := a.SuggestDocumentName(id)
		if err != nil || got.Name != "Untitled" || got.Used != "rule" || got.Call == 0 {
			t.Fatalf("%q was offered as %+v %v", bad, got, err)
		}
		if r, _ := set.Store().Get(got.Call); r.Why != "check" || r.Answer != bad {
			t.Fatalf("%q: record %+v", bad, r)
		}
	}
	set.SetJudge(&answering{"Parser release notes"}, "test", "tokens")
	if got, _ := a.SuggestDocumentName(id); got.Name != "Parser release notes" || got.Used != "judge" {
		t.Fatalf("a good name %+v", got)
	}
	o, _ := a.documentOps(a.bindingContext(), "")
	text := "Notes"
	old, err := o.Drafts.Store.Create(drafts.Origin{Kind: "local-folder"}, drafts.Baseline{}, "report.md", &text)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := a.JudgementRecords(JudgementFilter{Kind: "document-name"})
	if got, err := a.SuggestDocumentName(old.ID); err != nil || got.Name != "report.md" || got.Used != "rule" || got.Call != 0 {
		t.Fatalf("a document that is not new %+v %v", got, err)
	}
	if after, _ := a.JudgementRecords(JudgementFilter{Kind: "document-name"}); len(after.Records) != len(before.Records) {
		t.Fatal("a call was made for a document that is not new")
	}
}
