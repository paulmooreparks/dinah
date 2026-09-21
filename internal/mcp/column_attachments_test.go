package mcp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// attachToDoing attaches a file carrying body to the fixture's doing column as
// the operator, straight through the library, which is what a person running
// the terminal against the same workbench does while a head is serving it.
func attachToDoing(t *testing.T, library *verb.Library, name, body, description string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(source, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", source, err)
	}
	response := library.Attach(&verb.Request{Verb: "attach", Actor: "alka", Ref: "doing", File: source, Description: description})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("attach %s: %s %s", name, response.Outcome, response.Refusal)
	}
}

// replaceFirst replaces the bytes of doing's first attachment under the same
// filename, which changes nothing the listing serves.
func replaceFirst(t *testing.T, library *verb.Library, name, body string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(source, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", source, err)
	}
	response := library.Attach(&verb.Request{Verb: "attach", Actor: "alka", Ref: "doing/attachments/1", File: source, Replace: true})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("replace %s: %s %s", name, response.Outcome, response.Refusal)
	}
}

// listingWithheld reports whether a chain names the listing withheld.
func listingWithheld(served verb.Instructions) bool {
	for _, name := range served.Withheld {
		if name == verb.LayerColumnAttachments {
			return true
		}
	}
	return false
}

// wantListing fails unless a chain carries the listing in full with the
// filenames given, in order, each entry's path naming a file that exists, and
// does not name the listing withheld.
func wantListing(t *testing.T, what string, served verb.Instructions, filenames ...string) {
	t.Helper()
	if listingWithheld(served) {
		t.Errorf("%s: the listing was named withheld: %v", what, served.Withheld)
	}
	if len(served.ColumnAttachments) != len(filenames) {
		t.Fatalf("%s: wanted %d entries, got %+v", what, len(filenames), served.ColumnAttachments)
	}
	for position, filename := range filenames {
		entry := served.ColumnAttachments[position]
		if entry.Filename != filename {
			t.Errorf("%s: entry %d is %q, wanted %q", what, position+1, entry.Filename, filename)
		}
		if !bench.Exists(entry.Path) {
			t.Errorf("%s: entry %d names %s, which does not exist", what, position+1, entry.Path)
		}
	}
}

// wantListingWithheld fails unless a chain names the listing withheld and
// carries none of it.
func wantListingWithheld(t *testing.T, what string, served verb.Instructions) {
	t.Helper()
	if !listingWithheld(served) {
		t.Errorf("%s: the listing was not named withheld: %v", what, served.Withheld)
	}
	if served.ColumnAttachments != nil {
		t.Errorf("%s: a withheld listing was carried anyway: %+v", what, served.ColumnAttachments)
	}
}

// TestAColumnsListingIsWithheldAndRecovered asserts dinah-545/criteria/3 and
// the protocol half of dinah-545/criteria/6. The first claim serves the
// listing; a move landing the same owner at the same column names it
// withheld after the three layers, carries none of it, and names the column
// to reread; the column-shaped request serves it in full on the same
// connection; and a different owner is served it in full.
func TestAColumnsListingIsWithheldAndRecovered(t *testing.T) {
	library := newLibrary(t)
	writeGlobal(t, library, "Global text.\n")
	attachToDoing(t, library, "merge.md", "Merge after the checks pass.\n", "how to merge")
	session := newChainSession(t, library, newChainMemory())

	wantListing(t, "the claim", chain(t, session.call("claim", map[string]any{"card": "fx-1", "actor": "alka"})), "merge.md")
	moved := chain(t, session.call("move", map[string]any{"card": "fx-2", "column": "doing", "actor": "alka"}))
	wantWithheld(t, "a second card arriving", moved, verb.LayerGlobal, verb.LayerStanding, verb.LayerColumn, verb.LayerColumnAttachments)
	wantListingWithheld(t, "a second card arriving", moved)
	if moved.Reread != "doing" {
		t.Errorf("wanted the column's ref to read the chain back, got %q", moved.Reread)
	}
	wantListing(t, "the column-shaped request", chain(t, session.call("instructions", map[string]any{"card": moved.Reread, "actor": "alka"})), "merge.md")
	wantListing(t, "a different owner", chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "bo"})), "merge.md")
}

// TestAChangedListingIsServedAgain asserts dinah-545/criteria/4,
// dinah-545/criteria/7 and dinah-545/criteria/25 on one connection whose head
// was started before any of the attachments below existed. A file attached
// through the library while the head runs reaches the next serve, in full,
// while the unchanged column text stays withheld. Replacing a payload under
// its own filename leaves the listing withheld. Deleting the only attachment
// and attaching the same file with the same description gives the same
// reference with a new identifier and path, and that listing is served in
// full rather than withheld on the strength of the old one. The repeats are
// card-shaped instructions requests, which the library answers through the
// same serve a claim and a move call.
func TestAChangedListingIsServedAgain(t *testing.T) {
	library := newLibrary(t)
	session := newChainSession(t, library, newChainMemory())
	serveAtDoing := func() verb.Instructions {
		return chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka"}))
	}

	attachToDoing(t, library, "one.md", "one\n", "the first")
	attachToDoing(t, library, "two.md", "two\n", "the second")
	wantListing(t, "the first serve", chain(t, session.call("claim", map[string]any{"card": "fx-1", "actor": "alka"})), "one.md", "two.md")
	wantListingWithheld(t, "the repeat", serveAtDoing())

	attachToDoing(t, library, "three.md", "three\n", "the third")
	third := serveAtDoing()
	wantListing(t, "after a third file", third, "one.md", "two.md", "three.md")
	if !strings.Contains(strings.Join(third.Withheld, ","), verb.LayerColumn) {
		t.Errorf("after a third file the unchanged column text was not withheld: %v", third.Withheld)
	}

	replaceFirst(t, library, "one.md", "one, replaced\n")
	wantListingWithheld(t, "after a replacement under the same name", serveAtDoing())

	for ordinal := 3; ordinal >= 1; ordinal-- {
		ref := "doing/attachments/" + strconv.Itoa(ordinal)
		if response := library.Delete(&verb.Request{Verb: "delete", Actor: "alka", Ref: ref, Confirm: true}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("delete %s: %s %s", ref, response.Outcome, response.Refusal)
		}
	}
	attachToDoing(t, library, "one.md", "one\n", "the first")
	wantListing(t, "the single attachment", serveAtDoing(), "one.md")
	before := serveAtDoing()
	wantListingWithheld(t, "the single attachment again", before)

	oldPath := filepath.Dir(filepath.Dir(firstEntryPath(t, library)))
	if response := library.Delete(&verb.Request{Verb: "delete", Actor: "alka", Ref: "doing/attachments/1", Confirm: true}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("delete the single attachment: %s %s", response.Outcome, response.Refusal)
	}
	attachToDoing(t, library, "one.md", "one\n", "the first")
	again := serveAtDoing()
	wantListing(t, "the re-attached file", again, "one.md")
	if again.ColumnAttachments[0].Ref != "doing/attachments/1" {
		t.Errorf("the re-attached file answers %q, wanted the same reference", again.ColumnAttachments[0].Ref)
	}
	if strings.HasPrefix(again.ColumnAttachments[0].Path, oldPath) {
		t.Errorf("the re-attached file's path %s lies in the deleted directory %s", again.ColumnAttachments[0].Path, oldPath)
	}
}

// firstEntryPath answers the path doing's first attachment is served under,
// read through a column-shaped request on a connection of its own.
func firstEntryPath(t *testing.T, library *verb.Library) string {
	t.Helper()
	served, err := library.Instructions(&verb.Request{Verb: "instructions", Actor: "alka", Card: "doing"})
	if err != nil || len(served.Instructions.ColumnAttachments) == 0 {
		t.Fatalf("read the listing: %v %+v", err, served)
	}
	return served.Instructions.ColumnAttachments[0].Path
}

// TestTwoWorkbenchesDoNotWithholdEachOthersListing asserts
// dinah-545/criteria/26. Two workbenches instantiated from one definition,
// whose doing columns each carry the same file under the same description,
// serve listings that differ in their paths, so a claim on the second after
// the first has been served carries the second's listing in full, and a
// repeat on the second withholds it.
func TestTwoWorkbenchesDoNotWithholdEachOthersListing(t *testing.T) {
	base := t.TempDir()
	first := newLibraryUnder(t, filepath.Join(base, "one"))
	second := newLibraryUnder(t, filepath.Join(base, "two"))
	attachToDoing(t, first, "merge.md", "Merge after the checks pass.\n", "how to merge")
	attachToDoing(t, second, "merge.md", "Merge after the checks pass.\n", "how to merge")
	session := newChainSessionUnder(t, base, first, newChainMemory())

	served := chain(t, session.call("claim", map[string]any{"card": "fx-1", "actor": "alka"}))
	wantListing(t, "the first workbench", served, "merge.md")
	elsewhere := chain(t, session.call("claim", map[string]any{"card": "fx-1", "actor": "alka", "workbench": second.Bench.Root}))
	wantListing(t, "the second workbench", elsewhere, "merge.md")
	if elsewhere.ColumnAttachments[0].Path == served.ColumnAttachments[0].Path {
		t.Errorf("the two workbenches served one path: %s", served.ColumnAttachments[0].Path)
	}
	if !strings.HasPrefix(elsewhere.ColumnAttachments[0].Path, second.Bench.Root) {
		t.Errorf("the second workbench served %s, which is not its own", elsewhere.ColumnAttachments[0].Path)
	}
	repeat := chain(t, session.call("instructions", map[string]any{"card": "fx-1", "actor": "alka", "workbench": second.Bench.Root}))
	wantListingWithheld(t, "a repeat on the second workbench", repeat)
}

// TestTheWorkingAgreementNamesTheListing asserts the text half of
// dinah-545/criteria/7: the agreement initialize carries names the member the
// listing travels under and says it is read from disk on every serve.
func TestTheWorkingAgreementNamesTheListing(t *testing.T) {
	library := newLibrary(t)
	text := workingAgreement(library.Bench.Root, library)
	for _, want := range []string{
		"instructions.column_attachments",
		"and so is a column's attachment listing",
		"read from disk on every serve",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the working agreement does not say %q:\n%s", want, text)
		}
	}
}
