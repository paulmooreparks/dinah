package main

import (
	"reflect"
	"testing"

	"dinah/internal/contract"
)

// testValued is the subset of valuedFlags these tests exercise as
// value-taking, mirroring what run's own valued map holds.
func testValued() map[string]bool {
	valued := map[string]bool{}
	for _, name := range valuedFlags {
		valued[name] = true
	}
	return valued
}

// TestParseArgsHonorsTheEndOfOptionsMarker asserts dinah-92's AC-4: a bare
// "--" is consumed as the end-of-options marker rather than checked against
// known flags, and everything after the first one seen is positional
// regardless of shape, including a second literal "--".
func TestParseArgsHonorsTheEndOfOptionsMarker(t *testing.T) {
	cases := []struct {
		name           string
		argv           []string
		wantPositional []string
	}{
		{
			name:           "bare marker alone",
			argv:           []string{"comment", "fx-1", "--"},
			wantPositional: []string{"comment", "fx-1"},
		},
		{
			name:           "word after marker at start",
			argv:           []string{"comment", "fx-1", "--", "--verbose", "is", "a", "flag"},
			wantPositional: []string{"comment", "fx-1", "--verbose", "is", "a", "flag"},
		},
		{
			name:           "two markers",
			argv:           []string{"comment", "fx-1", "--", "--"},
			wantPositional: []string{"comment", "fx-1", "--"},
		},
		{
			name:           "marker inside text already past a marker",
			argv:           []string{"comment", "fx-1", "--", "please", "mention", "--", "here"},
			wantPositional: []string{"comment", "fx-1", "please", "mention", "--", "here"},
		},
		{
			name:           "single-dash word freed into positional",
			argv:           []string{"comment", "fx-1", "--", "-w"},
			wantPositional: []string{"comment", "fx-1", "-w"},
		},
		{
			name:           "known flag before the marker still parses as a flag",
			argv:           []string{"comment", "fx-1", "--json", "before", "the", "marker"},
			wantPositional: []string{"comment", "fx-1", "before", "the", "marker"},
		},
		{
			name:           "the same flag word after the marker is positional text",
			argv:           []string{"comment", "fx-1", "--", "after", "--json", "here"},
			wantPositional: []string{"comment", "fx-1", "after", "--json", "here"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parsed, err := parseArgs(c.argv, testValued())
			if err != nil {
				t.Fatalf("parseArgs: wanted no error, got %v", err)
			}
			if !reflect.DeepEqual(parsed.positional, c.wantPositional) {
				t.Errorf("positional: wanted %v, got %v", c.wantPositional, parsed.positional)
			}
		})
	}
}

// TestParseArgsMarkerLeavesFlagsBeforeItUnchanged asserts that a known flag
// written before the marker still lands in flags exactly as it does without
// a marker on the line at all.
func TestParseArgsMarkerLeavesFlagsBeforeItUnchanged(t *testing.T) {
	parsed, err := parseArgs([]string{"comment", "fx-1", "--json", "--", "text"}, testValued())
	if err != nil {
		t.Fatalf("parseArgs: wanted no error, got %v", err)
	}
	if !parsed.has("json") {
		t.Errorf("wanted --json, written before the marker, to still be recognized as a flag")
	}
	want := []string{"comment", "fx-1", "text"}
	if !reflect.DeepEqual(parsed.positional, want) {
		t.Errorf("positional: wanted %v, got %v", want, parsed.positional)
	}
}

// TestParseArgsUnknownFlagStillRefusesWithTheDashHint asserts dinah-92's
// AC-6 at the parseArgs level: an unrecognized --word, and a recognized
// valued flag missing its value, both still refuse before any marker is
// seen, and both carry dashHint so the head appends the hint sentence.
func TestParseArgsUnknownFlagStillRefusesWithTheDashHint(t *testing.T) {
	_, err := parseArgs([]string{"comment", "fx-1", "--bogus", "text"}, testValued())
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a *contract.Refusal, got %v", err)
	}
	if refusal.Name != contract.Usage || refusal.Detail != "--bogus" {
		t.Errorf("wanted usage refusing --bogus, got %s %q", refusal.Name, refusal.Detail)
	}
	if refusal.Extra["dashHint"] == "" {
		t.Errorf("wanted the unrecognized-flag refusal to carry dashHint, got %v", refusal.Extra)
	}

	_, err = parseArgs([]string{"comment", "fx-1", "--actor"}, testValued())
	refusal, ok = err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a *contract.Refusal, got %v", err)
	}
	if refusal.Name != contract.Usage || refusal.Detail != "--actor" {
		t.Errorf("wanted usage refusing --actor, got %s %q", refusal.Name, refusal.Detail)
	}
	if refusal.Extra["dashHint"] == "" {
		t.Errorf("wanted the missing-value refusal to carry dashHint, got %v", refusal.Extra)
	}
}

// TestScanLangFlagReadsOnlyALangThatIsAFlag asserts dinah-97's AC-7 and the
// scan half of its AC-8: scanLangFlag reports a --lang only where walkFlags
// places that word as a flag of its own, so a --lang standing in another
// valued flag's value slot is not a language choice, wherever on the line it
// falls. The table is driven against scanLangFlag rather than through the
// CLI because a CLI comparison would turn on which refusal happens to fire
// first rather than on what the scan read.
func TestScanLangFlagReadsOnlyALangThatIsAFlag(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "lang in state's value slot is state's value",
			argv: []string{"move", "card1", "--state", "--lang", "de"},
			want: "",
		},
		{
			name: "an ordinary lang after a filled value slot still reads",
			argv: []string{"move", "card1", "--state", "de", "--lang", "hi"},
			want: "hi",
		},
		{
			name: "lang in actor's value slot is actor's value",
			argv: []string{"comment", "fx-1", "--actor", "--lang", "de"},
			want: "",
		},
		{
			name: "lang in a value slot past the word that fails to parse",
			argv: []string{"--nosuchflag", "--state", "--lang", "de"},
			want: "",
		},
		{
			name: "lang written before the word that fails to parse",
			argv: []string{"--lang", "de", "--nosuchflag"},
			want: "de",
		},
		{
			name: "lang written after the word that fails to parse",
			argv: []string{"--nosuchflag", "--lang", "de"},
			want: "de",
		},
		{
			name: "the last complete lang on the line wins",
			argv: []string{"--lang", "de", "--nosuchflag", "--lang", "hi"},
			want: "hi",
		},
		{
			name: "a lang with no value left is not reported",
			argv: []string{"--nosuchflag", "--lang"},
			want: "",
		},
		{
			name: "a lang past the end-of-options marker is literal text",
			argv: []string{"add", "--", "--lang", "de"},
			want: "",
		},
		{
			name: "an inline spelling reads its own value",
			argv: []string{"--nosuchflag", "--lang=de"},
			want: "de",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scanLangFlag(c.argv); got != c.want {
				t.Errorf("scanLangFlag(%v): wanted %q, got %q", c.argv, c.want, got)
			}
		})
	}
}

// TestScanLangFlagAgreesWithParseArgsOnASuccessfulParse asserts the claim the
// two callers rest on, that scanLangFlag and parseArgs read one occurrence of
// --lang the same way. Where the parse succeeds, parsed.value("lang") is the
// answer scanLangFlag has to reach as well, since a disagreement there would
// render an invocation's refusal in a language its own parse never asked for.
func TestScanLangFlagAgreesWithParseArgsOnASuccessfulParse(t *testing.T) {
	lines := [][]string{
		{"move", "card1", "--state", "--lang", "de"},
		{"move", "card1", "--state", "de", "--lang", "hi"},
		{"comment", "fx-1", "--actor", "--lang", "de"},
		{"add", "--", "--lang", "de"},
		{"ls", "--lang", "de", "--json"},
		{"ls", "--lang=de"},
		{"ls"},
	}
	for _, argv := range lines {
		parsed, err := parseArgs(argv, testValued())
		if err != nil {
			t.Fatalf("parseArgs(%v): wanted no error, got %v", argv, err)
		}
		if got, want := scanLangFlag(argv), parsed.value("lang"); got != want {
			t.Errorf("the two readings of %v disagree: scanLangFlag got %q, parseArgs got %q", argv, got, want)
		}
	}
}

// TestParseArgsRecordsNoDomainCaptureForASessionFlag asserts dinah-97's AC-9,
// and it guards an invariant scanLangFlag depends on rather than one of its
// own lines. resolveOpenTailFlags rewrites parsed.flags only through
// domainCaptures, so a session flag that never enters that list is a flag no
// free-text zone can move after walkFlags has read it. A change folding
// session flags into domainCaptures fails here, on the day it is written,
// instead of quietly handing an open-tail command a --lang it may rewrite.
//
// help and version are in the roster because walkFlags gave them the same
// visit callback every other flag reaches. They used to arrive at a branch of
// their own that carried no capture code, so their exclusion was structural
// and no edit to a table could have undone it. sessionFlagNames is what
// excludes them now, and a table is a thing an edit can change.
func TestParseArgsRecordsNoDomainCaptureForASessionFlag(t *testing.T) {
	valued := testValued()
	for _, name := range []string{"workbench", "json", "quiet", "lang", "actor", "help", "version"} {
		t.Run(name, func(t *testing.T) {
			if !sessionFlagNames[name] {
				t.Fatalf("%s is no longer a session flag, so this case is asserting nothing", name)
			}
			argv := []string{"--" + name}
			if valued[name] {
				argv = append(argv, "de")
			}
			parsed, err := parseArgs(argv, valued)
			if err != nil {
				t.Fatalf("parseArgs(%v): wanted no error, got %v", argv, err)
			}
			if !parsed.has(name) {
				t.Fatalf("parseArgs(%v) did not record --%s at all, so this case is asserting nothing", argv, name)
			}
			if len(parsed.domainCaptures) != 0 {
				t.Errorf("--%s was recorded as a domain capture (%d of them), which puts it within reach of resolveOpenTailFlags", name, len(parsed.domainCaptures))
			}
		})
	}
	// A domain flag does still produce a capture, so the cases above are
	// reading a list that fills rather than one nothing ever reaches. Were
	// parseArgs to stop recording captures at all, every case above would
	// pass while the invariant they assert had gone empty.
	t.Run("a domain flag is still captured", func(t *testing.T) {
		if !valued["state"] {
			t.Fatalf("--state is no longer a valued flag, so this control is asserting nothing")
		}
		argv := []string{"--state", "de"}
		parsed, err := parseArgs(argv, valued)
		if err != nil {
			t.Fatalf("parseArgs(%v): wanted no error, got %v", argv, err)
		}
		if len(parsed.domainCaptures) != 1 {
			t.Fatalf("parseArgs(%v) recorded %d domain captures, wanted 1, so the cases above prove nothing", argv, len(parsed.domainCaptures))
		}
		if got := parsed.domainCaptures[0].tokens; len(got) != 2 || got[0] != "--state" || got[1] != "de" {
			t.Errorf("the capture for --state carries %q, wanted the two words the caller wrote", got)
		}
	})
}

// TestWalkFlagsGivesEveryOccurrenceTheWordsItConsumed asserts that walkFlags
// hands visit the argv word or words behind every flag it recognizes, the
// askedFor spellings included. parseArgs stores those words on a
// domainCapture, and resolveOpenTailFlags splices them back into positional
// text when it rejects that capture, so an occurrence reported with no
// tokens is one the splice would drop off the line without saying so.
// sessionFlagNames is all that keeps the askedFor spellings out of the
// capture list today, and this asserts the walk itself is safe whichever way
// that table is edited.
func TestWalkFlagsGivesEveryOccurrenceTheWordsItConsumed(t *testing.T) {
	valued := testValued()
	known := map[string]bool{}
	for _, name := range append(append([]string{}, valuedFlags...), markerFlags...) {
		known[name] = true
	}
	argv := []string{"-h", "--json", "--version", "--lang", "de", "--lang=hi", "move", "--state"}
	seen := 0
	walkFlags(argv, valued, known,
		func(string) {},
		func(name, value string, complete bool, tokens []string) {
			seen++
			if len(tokens) == 0 {
				t.Errorf("walkFlags reported --%s with no tokens, so a caller splicing tokens back into the line would lose the word the caller wrote", name)
			}
		},
		func(string) bool { return false },
	)
	if seen != 6 {
		t.Fatalf("walkFlags(%v) reported %d flag occurrences, wanted 6, so this is not reading the walk it was written for", argv, seen)
	}
}
