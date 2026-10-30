package main

import (
	"errors"
	"strings"
	"time"

	"studio/internal/bus"
	"studio/internal/drafts"
	"studio/internal/judge"
)

// Naming an untitled document is the second kind of judgement call
// (docs/plans/judgement-agents.md, step 2). A new document is called Untitled
// until Save As, and by the rule that a judge never interrupts, the suggestion
// goes only where the person is already choosing: the name box of the Save As
// dialog, which asks for it once the dialog is open and fills it in if the
// person has not typed. The rule is the name Untitled, so with no model, or a
// late or wrong answer, the box is as it is today.

// documentNameIn is everything a judge is shown to name a document.
type documentNameIn struct {
	Line string `json:"line" doc:"the first non-empty line of the document, at most 120 characters"`
}

// documentNameLimit is the longest name a judge may suggest.
const documentNameLimit = 60

// unnamedDocument is what a new document is called until it is saved.
const unnamedDocument = "Untitled"

var documentNameKind = judge.Declare(judge.Kind[documentNameIn, string]{
	Name:     "document-name",
	Describe: "Names the document Untitled.",
	System:   "You suggest a short file name for a document from its first line. Answer with the name only, without a file extension, quotation marks or a full stop.",
	Form:     judge.TextForm(documentNameLimit),
	Check: func(_ documentNameIn, out string) error {
		return checkFileName(out)
	},
	Deadline: 3 * time.Second,
	Rule:     func(documentNameIn) string { return unnamedDocument },
})

// reservedFileNames are the names Windows keeps for devices, which no file can
// have, with or without an extension.
var reservedFileNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// checkFileName refuses a name that no file can have: an empty one, one with a
// character a file name cannot hold, one that ends in a space or a dot, or a
// name Windows keeps for a device.
func checkFileName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("the name is empty")
	}
	if strings.ContainsAny(name, "\\/:*?\"<>|") || strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 }) {
		return errors.New("the name has a character a file name cannot hold")
	}
	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") || strings.HasPrefix(name, " ") {
		return errors.New("the name begins with a space or ends with a space or a dot")
	}
	stem, _, _ := strings.Cut(name, ".")
	if reservedFileNames[strings.ToLower(stem)] {
		return errors.New("the name is one the system keeps for a device")
	}
	return nil
}

// DocumentNameSuggestion is the name offered to the Save As dialog. Used says
// whether it is the judge's or the rule's, and Call is the record of the call
// that made it, or 0 when there was none.
type DocumentNameSuggestion struct {
	Name string `json:"name"`
	Used string `json:"used"`
	Call int64  `json:"call"`
}

// firstTextLine is the first line of a text that has any, cleaned as a title is.
func firstTextLine(text string) string {
	if len(text) > 4096 {
		text = text[:4096]
	}
	return bus.CleanTitle(text)
}

// SuggestDocumentName suggests a name for a new document, from the first
// non-empty line of its text, for the Save As dialog to offer in its name box. It
// waits no longer than the call's deadline and never fails because of the judge:
// the answer is then Untitled, which is what the box held before. A document that
// is not new has its own name, which is returned, and an empty one is Untitled
// with no call.
func (a *App) SuggestDocumentName(draftID string) (DocumentNameSuggestion, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return DocumentNameSuggestion{}, err
	}
	d, err := o.Draft(draftID)
	if err != nil {
		return DocumentNameSuggestion{}, err
	}
	if d.Origin.Kind != drafts.OriginNew {
		return DocumentNameSuggestion{Name: d.Title, Used: judge.UsedRule}, nil
	}
	line := firstTextLine(d.Text)
	set, err := a.judges()
	if line == "" || err != nil {
		return DocumentNameSuggestion{Name: unnamedDocument, Used: judge.UsedRule}, nil
	}
	r := judge.Call(a.bindingContext(), set, documentNameKind, documentNameIn{Line: line}, judge.Effect{Kind: "draft", ID: draftID})
	return DocumentNameSuggestion{Name: r.Out, Used: r.Used, Call: r.ID}, nil
}
