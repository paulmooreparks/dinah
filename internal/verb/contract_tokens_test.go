package verb

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// contractTokens is the closed set of machine vocabulary this project's own
// commands, substates, level axes and flags are spelled with, composed from
// each vocabulary's own declaration so a name added there is protected
// without a second list to keep in step. "stdout" is written by hand: it
// names a Unix stream, not anything this project declares a constant for,
// and en.json's cmd.export.summary is the one place it appears.
func contractTokens() []string {
	set := map[string]bool{
		contract.SubstateReady:   true,
		contract.SubstateActive:  true,
		contract.SubstateBlocked: true,
		bench.WorkbenchAnchor:    true,
		"stdout":                 true,
	}
	for _, axis := range bench.LevelAxes {
		set[axis] = true
	}
	for _, name := range Commands() {
		set[name] = true
		for _, p := range params[name] {
			if !p.Flag {
				continue
			}
			shown := p.Name
			if p.Display != "" {
				shown = p.Display
			}
			set["--"+shown] = true
		}
	}
	tokens := make([]string, 0, len(set))
	for token := range set {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

// backtickSpan matches one backtick-quoted span of an English entry, which is
// how the catalog marks text a reader is meant to copy rather than read.
var backtickSpan = regexp.MustCompile("`([^`]+)`")

// literalTokens returns the contract tokens text exposes as a literal a
// reader is meant to copy, which is every backtick-quoted span that is
// exactly one token. Ordinary prose using one of these words as a plain verb
// or noun, such as cmd.archive.summary's "Move a card...", is not one, and is
// deliberately not checked: word-boundary matching over every English
// sentence would fail that sentence's correct German ("Eine Karte ...
// nehmen") for translating an ordinary verb.
//
// A token.* entry is not one either, and dinah-252's spec was wrong to say it
// was. That namespace holds the reading a person gets for a canonical token,
// which every catalog is supposed to translate: token.active's own context
// says "How the canonical token active is rendered where a person reads it.
// The machine surface always carries the canonical spelling." Selecting on it
// fails de/token.active for saying "aktiv", which is the entry doing its job.
// The two entries of that namespace whose rendering really is the identifier,
// token.dinah-editor and token.visual, declare that in their context and are
// guarded by internal/msg's TestEveryUntranslatableIdentifierSurvivesTranslation.
func literalTokens(text string, tokens map[string]bool) []string {
	var found []string
	for _, m := range backtickSpan.FindAllStringSubmatch(text, -1) {
		if tokens[m[1]] {
			found = append(found, m[1])
		}
	}
	return found
}

// TestContractTokensSurviveInBackticks asserts that a name this project
// declares travels into every translation byte-identical wherever the English
// marks it as the literal thing itself by quoting it in backticks. dinah-245
// coined three different German names for one flag across four strings, and
// this is the guard that would have refused them.
func TestContractTokensSurviveInBackticks(t *testing.T) {
	tokens := map[string]bool{}
	for _, token := range contractTokens() {
		tokens[token] = true
	}
	checked := 0
	for _, key := range msg.Keys() {
		base, ok := msg.BaseEntry(key)
		if !ok {
			continue
		}
		wanted := literalTokens(base.Text, tokens)
		if len(wanted) == 0 {
			continue
		}
		for _, tag := range msg.Tags() {
			if tag == msg.Base {
				continue
			}
			entry, carried := msg.CatalogEntry(tag, key)
			if !carried || entry.Skeleton {
				continue
			}
			checked++
			for _, token := range wanted {
				if strings.Contains(entry.Text, token) {
					continue
				}
				t.Errorf("%s/%s: wanted the contract token %q untranslated, got %q", tag, key, token, entry.Text)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no entry carried a contract token in a backtick span, so this guard is asserting nothing")
	}
}
