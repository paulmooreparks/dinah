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
