package verb

import (
	"sort"
	"strings"
	"testing"

	"dinah/internal/guide"
	"dinah/internal/msg"
)

// TestTheSyntaxLineIsComposedOfTheSameTokensTheTableReads asserts that the
// syntax line is the command's own name followed by its tokens and nothing
// else. An edit that makes Usage spell any argument differently from Token
// fails here, whatever route through Usage produced the spelling.
//
// The test does not catch a later edit that decorates an argument inside
// Usage again. Usage decorated inline before this card and composed the same
// bytes Token composes now, so restoring that body leaves this equality true
// and this test green. What holds Token itself to its own rule is the
// ratified help block, which pins every command's syntax line byte for byte,
// and the arguments-table assertion in cmd/dinah, which reads the drawn
// column and reddens when the cell is spelled any other way.
//
// The equality holds for a command with no arguments too, which is what the
// obvious spelling of it gets wrong: TrimPrefix(Usage(name), name+" ") returns
// the bare command word for export, mcp, columns, status, whoami and
// workbenches, while the joined tokens return the empty string.
func TestTheSyntaxLineIsComposedOfTheSameTokensTheTableReads(t *testing.T) {
	bare := 0
	for _, name := range Commands() {
		tokens := Tokens(name)
		want := strings.TrimSpace(name + " " + strings.Join(tokens, " "))
		if got := Usage(name); got != want {
			t.Errorf("%s: the syntax line reads %q and its tokens compose %q", name, got, want)
		}
		if len(tokens) == 0 {
			bare++
		}
	}
	if bare == 0 {
		t.Error("no command takes no argument, so the case this assertion was corrected for is not exercised")
	}
}

// TestEveryTokenCarriesItsBracketsByTheOneRule asserts the typography rule on
// the four shapes a token takes, so a change to Token that loses a bracket
// fails here as well as in the ratified block.
func TestEveryTokenCarriesItsBracketsByTheOneRule(t *testing.T) {
	cases := []struct {
		label string
		param Param
		want  string
	}{
		{label: "a required positional", param: Param{Name: "card", Required: true}, want: "<card>"},
		{label: "an optional positional", param: Param{Name: "column"}, want: "[column]"},
		{label: "an optional marker", param: Param{Name: "ready", Flag: true, Marker: true}, want: "[--ready]"},
		{label: "a required marker", param: Param{Name: "yes", Flag: true, Marker: true, Required: true}, want: "--yes"},
		{label: "an optional valued flag", param: Param{Name: "kind", Flag: true, Value: "kind"}, want: "[--kind <kind>]"},
		{label: "a display standing in for the name", param: Param{Name: "card", Display: "ref", Required: true}, want: "<ref>"},
	}
	for _, c := range cases {
		if got := c.param.Token(); got != c.want {
			t.Errorf("%s: wanted %q, got %q", c.label, c.want, got)
		}
	}
}

// TestEveryArgumentNamesOneSentenceAndTheCatalogCarriesNoOther asserts both
// halves of the shared-sentence rule. Every (command, argument) pair resolves a
// non-empty summary at exactly one of its two forms, and the catalog carries
// neither a per-command key for a parameter declaring Shared nor a shared key
// no parameter declares, so it cannot grow a dead string a later reader
// mistakes for a live one.
//
// The catalog is read through the base entries rather than through a renderer,
// since a renderer answers for a key nobody wrote by echoing the key.
func TestEveryArgumentNamesOneSentenceAndTheCatalogCarriesNoOther(t *testing.T) {
	shared := map[string]bool{}
	perCommand := map[string]bool{}
	pairs := 0
	for _, name := range Commands() {
		for _, param := range Params(name) {
			pairs++
			own := "param." + name + "." + param.Name + ".summary"
			key := param.SummaryKey(name)
			if key == own {
				perCommand[own] = true
			} else {
				shared[key] = true
			}
			if text := baseText(t, key); text == "" {
				t.Errorf("%s %s: the catalog carries no sentence at %s", name, param.Name, key)
			}
			if key == own {
				continue
			}
			if text := baseText(t, own); text != "" {
				t.Errorf("%s %s takes the shared sentence at %s and the catalog also carries %s, so one of the two is dead", name, param.Name, key, own)
			}
		}
	}
	if pairs == 0 {
		t.Fatal("no command declares an argument, so this test proves nothing")
	}
	if len(shared) == 0 {
		t.Fatal("no parameter declares Shared, so the half of this test about shared keys proves nothing")
	}
	for _, key := range catalogKeysUnder(t, "param.") {
		if shared[key] || perCommand[key] {
			continue
		}
		t.Errorf("the catalog carries %s and no parameter names it, so it is a dead string", key)
	}
	// Every param.<command>.<argument>.summary in the base catalog carries
	// English text. The base catalog is the fallback every other catalog
	// degrades through, so a non-ASCII string here is a translation that
	// slipped into the wrong file and would render to an English reader as
	// a foreign script. The test reads the text directly so a translation
	// mistake fails the build rather than passing silently.
	for _, key := range catalogKeysUnder(t, "param.") {
		entry, ok := msg.BaseEntry(key)
		if !ok {
			continue
		}
		if !isASCIIText(entry.Text) {
			t.Errorf("%s in the base catalog carries non-ASCII text %q, which means a translation slipped into en", key, entry.Text)
		}
	}
}

// isASCIIText reports whether s carries only printable ASCII, the character
// set the base catalog's English text has to occupy. It rejects umlauts and
// every other non-ASCII byte, which is what catches a translation slipping
// into the wrong locale file.
func isASCIIText(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

// TestEveryDeclaredVocabularyResolves asserts that an argument naming a closed
// set names one that exists, and that a set is either carried here or named as
// a source a head can answer.
func TestEveryDeclaredVocabularyResolves(t *testing.T) {
	declared := 0
	for _, name := range Commands() {
		for _, param := range Params(name) {
			if param.Vocabulary == "" {
				continue
			}
			declared++
			set, ok := VocabularyFor(name, param.Name)
			if !ok {
				t.Errorf("%s %s names the vocabulary %q, which no set answers to", name, param.Name, param.Vocabulary)
				continue
			}
			if len(set.Values) == 0 && set.Source == "" {
				t.Errorf("%s %s names a vocabulary that carries no members and no source", name, param.Name)
			}
			if len(set.Values) > 0 && set.Source != "" {
				t.Errorf("%s %s names a vocabulary carrying both members and a source, so which one answers is undecided", name, param.Name)
			}
		}
	}
	if declared == 0 {
		t.Fatal("no argument declares a vocabulary, so this test proves nothing")
	}
	if _, ok := VocabularyFor("ls", "ready"); ok {
		t.Error("an argument declaring no vocabulary reported one")
	}
	if _, ok := VocabularyFor("ls", "no-such-argument"); ok {
		t.Error("an argument no command declares reported a vocabulary")
	}
}

// TestEveryGuideTopicDeclaredIsOneTheBinaryCarries asserts that a command or a
// parameter pointing a reader at a guide points at a guide that ships, so a
// help page never sends somebody to `dinah guide` with a topic it refuses.
func TestEveryGuideTopicDeclaredIsOneTheBinaryCarries(t *testing.T) {
	shipped := map[string]bool{}
	for _, topic := range guide.Topics() {
		shipped[topic] = true
	}
	if len(shipped) == 0 {
		t.Fatal("the binary carries no guide, so this test proves nothing")
	}
	pointed := 0
	for _, name := range Commands() {
		for _, topic := range Guides(name) {
			pointed++
			if !shipped[topic] {
				t.Errorf("%s points at the guide %q, which the binary does not carry", name, topic)
			}
		}
	}
	if pointed == 0 {
		t.Fatal("no command points at a guide, so this test proves nothing")
	}
	for _, name := range []string{"path", "edit", "show", "instructions", "attach", "archive", "delete"} {
		if got := Guides(name); len(got) != 1 || got[0] != "references" {
			t.Errorf("%s points at %v rather than at the references guide alone", name, got)
		}
	}
	if got := Guides("claim"); len(got) != 0 {
		t.Errorf("claim takes a card rather than a reference and points at %v", got)
	}
}

// baseText reads one key out of the English catalog, or the empty string where
// the catalog carries no such key. It reads the entry rather than rendering
// the key, since a renderer answers for a key nobody wrote by echoing it.
func baseText(t *testing.T, key string) string {
	t.Helper()
	entry, ok := msg.BaseEntry(key)
	if !ok {
		return ""
	}
	return entry.Text
}

// catalogKeysUnder lists the English catalog's keys carrying a prefix, sorted.
func catalogKeysUnder(t *testing.T, prefix string) []string {
	t.Helper()
	var keys []string
	for _, key := range msg.Keys() {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
