package guide

import (
	"strings"
	"testing"
)

// TestTheReadingOrderPlacesEveryEmbeddedGuide asserts that the declared
// reading order and the embedded guides directory name the same set of topics.
//
// The check runs in both directions, because each direction loses something
// different. A topic embedded and left unplaced is served by no surface at
// all, since Topics reads the order rather than the directory. A topic placed
// and never embedded is offered in a listing that cannot answer it. Neither
// failure is visible in a passing suite that reads one direction only.
func TestTheReadingOrderPlacesEveryEmbeddedGuide(t *testing.T) {
	entries, err := guides.ReadDir("guides")
	if err != nil {
		t.Fatalf("read the embedded guides: %v", err)
	}
	embedded := map[string]bool{}
	for _, entry := range entries {
		embedded[strings.TrimSuffix(entry.Name(), ".md")] = true
	}
	if len(embedded) == 0 {
		t.Fatal("the binary embeds no guide, so this test reads nothing")
	}
	placed := map[string]bool{}
	for _, topic := range reading {
		placed[topic] = true
	}
	for topic := range embedded {
		if placed[topic] {
			continue
		}
		t.Errorf("internal/guide/guides/%s.md is embedded and the reading order does not place it, so no surface offers or serves it", topic)
	}
	for topic := range placed {
		if embedded[topic] {
			continue
		}
		t.Errorf("the reading order places %s and no internal/guide/guides/%s.md is embedded, so a listing offers a topic nothing can answer", topic, topic)
	}
}

// TestTopicsAreOfferedInTheDeclaredReadingOrder asserts that Topics hands its
// caller the reading order itself rather than any other arrangement of the
// same names, which is what makes one list govern every surface that offers
// the guides.
//
// This test holds Topics against the declared order and cannot hold the
// declared order itself, since both sides read the same slice: reordering
// `reading` moves the expectation with the code. What pins the approved order
// is the replayed `dinah guide` block in docs/quick-start.md, which the quick
// start's replay drives byte for byte, so a reordering that nobody approved
// fails there. Read the two together.
func TestTopicsAreOfferedInTheDeclaredReadingOrder(t *testing.T) {
	got := Topics()
	if len(got) != len(reading) {
		t.Fatalf("Topics offered %d topics and the reading order places %d: %v", len(got), len(reading), got)
	}
	for at, topic := range reading {
		if got[at] == topic {
			continue
		}
		t.Errorf("Topics offered %q at position %d and the reading order places %q there", got[at], at, topic)
	}
}

// TestMCPGuideNoLongerPromisesTheNarrowingThatWasRemoved is dinah-307 AC-11.
// The guide told agents that a server started without a directory to serve
// narrows to the one workbench it discovered and searches nowhere else. That
// was true when it was written and is false as of dinah-307, and a guide the
// tool disagrees with is worse than no guide, because an agent acts on it.
//
// The text is compared with its line breaks folded away, since a sentence in
// the source is wrapped across lines and a reader reads the sentence.
func TestMCPGuideNoLongerPromisesTheNarrowingThatWasRemoved(t *testing.T) {
	text, err := Text("mcp")
	if err != nil {
		t.Fatalf("read the mcp guide: %v", err)
	}
	folded := strings.Join(strings.Fields(text), " ")

	gone := "so `workbenches` answers with an empty list while that workbench goes on answering every other call"
	if strings.Contains(folded, gone) {
		t.Errorf("the mcp guide still promises the narrowing dinah-307 removed: %q", gone)
	}
	want := "any workbench you can name by its absolute path is reachable"
	if !strings.Contains(folded, want) {
		t.Errorf("the mcp guide does not tell an agent what an unbounded server reaches: wanted %q", want)
	}
}

// TestMCPGuideNoLongerPromisesTheOldWorkbenchesRefusal is dinah-301's guide
// correction, covering two stale claims in one paragraph pair: dinah-312
// already made the root-skip claim false, and this card's own fix makes the
// unconditional-refusal claim false. The assertions name marker phrases rather
// than whole sentences, so a later rewording of the surrounding prose does not
// fail this test over wording the claim itself does not depend on.
func TestMCPGuideNoLongerPromisesTheOldWorkbenchesRefusal(t *testing.T) {
	text, err := Text("mcp")
	if err != nil {
		t.Fatalf("read the mcp guide: %v", err)
	}
	folded := strings.Join(strings.Fields(text), " ")

	for _, gone := range []string{
		"skips the top directory itself",
		"rather than listing anything, even where the server does carry a default",
	} {
		if strings.Contains(folded, gone) {
			t.Errorf("the mcp guide still carries a claim dinah-301 falsified: %q", gone)
		}
	}
	if !strings.Contains(folded, "`unbounded`") {
		t.Errorf("the mcp guide's workbenches paragraph does not name the unbounded marker: %q", folded)
	}
}

// TestMCPGuideNamesTheExclusionsTheWalkApplies holds the workbenches paragraph
// to what walkableDir in internal/bench actually does. The walk skips a
// dot-prefixed directory and a symlinked one, so a sentence promising every
// directory below the root at any depth is false, and an agent reading it has
// no way to tell an omitted workbench from an absent one. The assertions name
// marker phrases rather than whole sentences, and the banned phrase is the
// universal this card first shipped and code review caught.
func TestMCPGuideNamesTheExclusionsTheWalkApplies(t *testing.T) {
	text, err := Text("mcp")
	if err != nil {
		t.Fatalf("read the mcp guide: %v", err)
	}
	folded := strings.Join(strings.Fields(text), " ")

	for _, want := range []string{
		"name begins with a dot",
		"symbolic link",
	} {
		if !strings.Contains(folded, want) {
			t.Errorf("the mcp guide's workbenches paragraph does not name an exclusion the walk applies: wanted %q", want)
		}
	}
	if gone := "at or below that one, at any depth"; strings.Contains(folded, gone) {
		t.Errorf("the mcp guide claims a reach the walk does not have: %q", gone)
	}
}
