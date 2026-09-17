package main

import (
	"strings"
	"testing"

	"dinah/internal/guide"
	"dinah/internal/verb"
)

// countWords are the number words the guides spell out, which is how this
// project writes a small count in prose. The guard below composes its
// expectation through this table rather than matching a digit, so the guide
// stays prose and the check stays derived from the code.
var countWords = map[int]string{
	5: "five", 6: "six", 7: "seven", 8: "eight", 9: "nine", 10: "ten", 11: "eleven",
}

// TestTheMCPGuidesFieldsSectionCountsWhatTheCodeDeclares holds the guide to
// the tool rather than to another sentence. The member count comes from
// verb.DetailFields and the count of names a caller may write comes from
// verb.DetailSelectors, so a member or a modifier added to either fails this
// until somebody edits the guide.
//
// Three sentences of this section were false on the trunk before this card:
// one gave the member count as six and listed the six without `checklist`,
// one said a caller naming no fields is served those six, and one said a name
// outside the six is refused. The count had been wrong by one since before
// the card was filed, and this is the guard that stops it going wrong again.
func TestTheMCPGuidesFieldsSectionCountsWhatTheCodeDeclares(t *testing.T) {
	text, err := guide.Text("mcp")
	if err != nil {
		t.Fatalf("guide mcp: %v", err)
	}
	const heading = "## Ask show for the members you want"
	start := strings.Index(text, heading)
	if start < 0 {
		t.Fatalf("the mcp guide carries no %q section; this guard and the guide have drifted apart", heading)
	}
	section := text[start:]
	if next := strings.Index(section[len(heading):], "\n## "); next >= 0 {
		section = section[:len(heading)+next]
	}

	members, selectors := len(verb.DetailFields), len(verb.DetailSelectors)
	memberWord, ok := countWords[members]
	if !ok {
		t.Fatalf("no number word for %d members, so this guard cannot compose its expectation", members)
	}
	selectorWord, ok := countWords[selectors]
	if !ok {
		t.Fatalf("no number word for %d selectors, so this guard cannot compose its expectation", selectors)
	}

	sentences := []struct {
		what string
		want string
	}{
		{what: "the member count", want: "The card has " + memberWord + " members"},
		{what: "the count of names against the count of members",
			want: "there are " + selectorWord + " names you may write over " + memberWord + " members"},
		{what: "what a name outside the set is refused as",
			want: "A name outside the " + selectorWord + " is refused"},
	}
	for _, sentence := range sentences {
		if !strings.Contains(section, sentence.want) {
			t.Errorf("the guide does not state %s as the code declares it; wanted a sentence carrying %q",
				sentence.what, sentence.want)
		}
	}
	t.Logf("%d sentences checked against %d members and %d selectors",
		len(sentences), members, selectors)

	// The names themselves, so a guide carrying the right count and the
	// wrong set fails too.
	for _, name := range verb.DetailSelectors {
		if !strings.Contains(section, "`"+name+"`") {
			t.Errorf("the guide's fields section does not name the selector %s", name)
		}
	}
}
