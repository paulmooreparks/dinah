package main

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/msg"
	"dinah/internal/verb"
)

// kindsSentinel stands in for {kinds} while the help.reference-kinds template is
// rendered backwards. It carries no letter of any language and no separator, so
// it cannot collide with a label, a summary or the template's own punctuation.
const kindsSentinel = "\x01KINDS\x01"

// foldWhitespace recovers a clause from a wrapped help page. cmd/dinah/row.go's
// packTokens lays out whole tokens taken from strings.Fields and joins them with
// one space, and it never hyphenates and never breaks inside a word, so folding
// every run of whitespace to a single space puts a wrapped clause back together
// exactly as it was composed.
func foldWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// catalogueTags enumerates the catalogue directory rather than naming the
// languages, so a ninth language fails these checks instead of being missed.
func catalogueTags(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repositoryRoot, "internal", "msg", "locales", "*.json"))
	if err != nil {
		t.Fatalf("glob the catalogues: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("the tree carries no message catalogue, so this check would read nothing")
	}
	tags := make([]string, 0, len(files))
	for _, file := range files {
		tags = append(tags, strings.TrimSuffix(filepath.Base(file), ".json"))
	}
	sort.Strings(tags)
	return tags
}

// referenceParamOf is the one parameter of a command that takes a reference.
// TestNoReferenceTakingParameterAlsoDeclaresAVocabulary holds the roster to one
// each, so finding none here is a defect rather than a shape this file handles.
func referenceParamOf(t *testing.T, command string) verb.Param {
	t.Helper()
	for _, param := range verb.Params(command) {
		if param.Guide == "references" {
			return param
		}
	}
	t.Fatalf("%s is on the references roster and declares no parameter naming that guide", command)
	return verb.Param{}
}

// TestEveryHelpPageNamesTheReferenceKindsItDeclares is the check that makes the
// references guide's promise true. The guide tells a reader that a command's own
// help page carries the same answer about what that command accepts, and this
// holds every page in every language to the declaration in internal/verb.
//
// Its expectation is built from the catalogue entries and the declaration, and it
// never calls verb.ArgumentMeaning, the template reversal included. A guard that
// recomputed its expected value from the function under test would pass against
// any renderer at all.
//
// It isolates the kinds region before it splits anything. The rendered row is a
// summary inside a template, and two of the summaries this card ships carry
// verb.ReferenceKindSeparator themselves, so splitting the whole row would read a
// summary's own semicolon as a kind boundary and fire against correct code. The
// template is rendered with a sentinel standing in for {kinds}, which yields the
// literal prefix and suffix it wraps the kinds in, and only the region between
// them is split.
func TestEveryHelpPageNamesTheReferenceKindsItDeclares(t *testing.T) {
	roster := commandsTakingAReference()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	tags := catalogueTags(t)
	dir := t.TempDir()

	pages := 0
	sawAttach := false
	sawAttachments := false
	for _, tag := range tags {
		catalog := msg.For(tag)
		for _, command := range roster {
			param := referenceParamOf(t, command)
			kinds, declared := verb.ReferenceKindsFor(command)
			if !declared || len(kinds) == 0 {
				t.Errorf("%s takes a reference and internal/verb declares no kinds for it", command)
				continue
			}

			summary := catalog.T(param.SummaryKey(command))
			rendered := catalog.T("help.reference-kinds", "summary", summary, "kinds", kindsSentinel)
			// The rendered template is folded with the sentinel still in it and
			// cut afterwards. Folding the two halves separately would strip the
			// space the template puts before {kinds}, because that space sits at
			// the end of the prefix where strings.Fields drops it, and the region
			// would then open with a space that is not part of any label.
			foldedPrefix, foldedSuffix, split := strings.Cut(foldWhitespace(rendered), kindsSentinel)
			if !split {
				t.Fatalf("%s's help.reference-kinds entry does not substitute {kinds}, so the clause cannot be reversed", tag)
			}
			wantPrefix := foldedPrefix
			wantSuffix := foldedSuffix

			var folded [2]string
			for at, columns := range [2]string{"40", "200"} {
				t.Setenv("COLUMNS", columns)
				t.Setenv("DINAH_LANG", tag)
				got := runCLI(t, dir, "help", command)
				if got.code != 0 {
					t.Fatalf("dinah help %s under %s: %d %s", command, tag, got.code, got.errw)
				}
				pages++
				page := foldWhitespace(got.out)

				at1 := strings.Index(page, wantPrefix)
				if at1 < 0 {
					t.Errorf("`dinah help %s` under %s carries no row opening %q, so its argument names no kinds at all", command, tag, wantPrefix)
					break
				}
				rest := page[at1+len(wantPrefix):]
				at2 := strings.Index(rest, wantSuffix)
				if at2 < 0 {
					t.Errorf("`dinah help %s` under %s opens the kinds clause and never closes it with %q", command, tag, wantSuffix)
					break
				}
				folded[at] = rest[:at2]
			}
			if folded[0] == "" && folded[1] == "" {
				continue
			}
			// Rendering the page at two widths and requiring one clause is what
			// proves the check reads the clause rather than an accident of one
			// window, and it is what would catch a wrapper that began breaking
			// inside a word.
			if folded[0] != folded[1] {
				t.Errorf("`dinah help %s` under %s draws the kinds clause as %q at COLUMNS=40 and %q at COLUMNS=200", command, tag, folded[0], folded[1])
				continue
			}

			printed := map[string]bool{}
			for _, label := range strings.Split(folded[0], verb.ReferenceKindSeparator) {
				printed[label] = true
			}
			for _, kind := range kinds {
				label := catalog.T(kind.MessageKey())
				if !printed[label] {
					t.Errorf("%s declares that %s's reference may name %s and `dinah help %s` under %s does not print the label %q", "internal/verb", command, kind, command, tag, label)
				}
				delete(printed, label)
			}
			for label := range printed {
				t.Errorf("`dinah help %s` under %s prints the label %q and internal/verb declares no kind of %s carrying it", command, tag, label, command)
			}
			switch command {
			case "attach":
				sawAttach = true
			case "attachments":
				sawAttachments = true
			}
		}
	}
	if !sawAttach || !sawAttachments {
		t.Errorf("the sweep did not reach attach (%t) and attachments (%t), whose summaries carry the separator themselves and are what a naive split would fire against", sawAttach, sawAttachments)
	}
	t.Logf("%d catalogues enumerated from internal/msg/locales/, %d help pages read, attach and attachments among them", len(tags), pages/2)
	if want := len(tags) * len(roster); pages/2 != want {
		t.Fatalf("the sweep read %d help pages and %d commands across %d catalogues is %d", pages/2, len(roster), len(tags), want)
	}
}

// TestNoReferenceKindLabelCarriesTheClauseSeparator is what lets the region split
// above be exact. A label carrying the separator would be read as two labels and
// the check would fire against correct code, so the catalogue that introduced it
// is refused here instead, in whichever language, and the remedy is one
// punctuation change rather than an exemption entry.
//
// Its subject is the six reference.kind.* labels and nothing else. A summary
// carrying the pair is legal and five of them ship carrying it today, because the
// template reversal removes the summary before anything is split.
func TestNoReferenceKindLabelCarriesTheClauseSeparator(t *testing.T) {
	tags := catalogueTags(t)
	kinds := verb.ReferenceKindOrder()
	if len(kinds) == 0 {
		t.Fatal("internal/verb declares no reference kinds, so this check read nothing")
	}
	read := 0
	for _, tag := range tags {
		for _, kind := range kinds {
			entry, held := msg.CatalogEntry(tag, kind.MessageKey())
			if !held {
				t.Errorf("%s carries no entry for %s", tag, kind.MessageKey())
				continue
			}
			read++
			if strings.Contains(entry.Text, verb.ReferenceKindSeparator) {
				t.Errorf("%s's %s reads %q, which carries the clause separator %q, so the guard over the rendered clause would read it as two labels", tag, kind.MessageKey(), entry.Text, verb.ReferenceKindSeparator)
			}
		}
	}
	t.Logf("%d labels read", read)
	if want := len(tags) * len(kinds); read != want {
		t.Fatalf("the sweep read %d labels and %d kinds across %d catalogues is %d", read, len(kinds), len(tags), want)
	}
}
