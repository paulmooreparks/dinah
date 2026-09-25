package main

import (
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestArchivingAnEarlierCommentLeavesTheAnswerWhereItWas is
// dinah-472/criteria/54 and /66, which is the fifth review's reproduction
// closed.
//
// The reproduction was three commands: an item carrying a stray note, the
// operator's answer and a later agent comment; archive the stray note; and the
// item, untouched, cites the agent. What closes it is that the answer names
// the comment's own identifier, which archiving an earlier comment cannot
// move.
func TestArchivingAnEarlierCommentLeavesTheAnswerWhereItWas(t *testing.T) {
	root := statesFixture(t, "a card carrying an answered question")
	mustRun(t, root, "file", "fx-1", "open_question", "which vendor do we cite?")
	mustRun(t, root, "comment", "fx-1/questions/1", "a stray note written first", "--actor", "sam")
	mustRun(t, root, "comment", "fx-1/questions/1", "the operator's own answer")
	mustRun(t, root, "comment", "fx-1/questions/1", "an opinion written afterwards", "--actor", "sam")
	mustRun(t, root, "resolve", "fx-1/questions/1", "fx-1/questions/1/comments/2")

	// Archiving the stray note renumbers the survivors, which is the whole of
	// what the reproduction turned on.
	mustRun(t, root, "archive", "fx-1/questions/1/comments/1", "--actor", "sam")
	assertAnswerReads(t, root, "fx-1/questions/1", "the operator's own answer", "alka")

	// Restoring it puts it back into creation order and shifts the positions
	// of everything after it, which is the same defect read the other way.
	mustRun(t, root, "restore", "fx-1/questions/1/comments/1", "--actor", "sam")
	assertAnswerReads(t, root, "fx-1/questions/1", "the operator's own answer", "alka")

	// And deleting one, which leaves no archived member anywhere to notice.
	mustRun(t, root, "delete", "fx-1/questions/1/comments/1", "--yes", "--actor", "sam")
	assertAnswerReads(t, root, "fx-1/questions/1", "the operator's own answer", "alka")
}

// assertAnswerReads fails a case whose item no longer cites the words it was
// settled with, reading the reference the surface composes as well as the
// comment the read serves.
func assertAnswerReads(t *testing.T, root, ref, body, author string) {
	t.Helper()
	shown := mustRun(t, root, "show", cardOfRef(ref), "--fields", "checklist.full")
	if !strings.Contains(shown.out, body) {
		t.Errorf("the checklist read does not carry the answer %q:\n%s", body, shown.out)
	}
	if !strings.Contains(shown.out, author) {
		t.Errorf("the checklist read does not name the answer's author %q:\n%s", author, shown.out)
	}
	// The reference the read prints is composed at the moment of the read, so
	// what a person copies out of the report reaches the comment they saw.
	printed := printedResolutionOf(shown.out)
	if printed == "" {
		t.Fatalf("the checklist read prints no reference for the answer:\n%s", shown.out)
	}
	comment := mustRun(t, root, "show", printed)
	if !strings.Contains(comment.out, body) {
		t.Errorf("the reference %q the read printed reaches %q rather than the answer:\n%s", printed, comment.out, shown.out)
	}
}

// cardOfRef is the card half of an item's reference.
func cardOfRef(ref string) string {
	if at := strings.Index(ref, "/"); at >= 0 {
		return ref[:at]
	}
	return ref
}

// printedResolutionOf finds the comment reference a checklist read printed,
// which is the one cell of the row carrying a comments segment.
func printedResolutionOf(printed string) string {
	for _, field := range strings.Fields(printed) {
		if strings.Contains(field, "/comments/") {
			return field
		}
	}
	return ""
}

// TestEveryTerminalVerbTakesEitherSpellingAndStoresTheIdentifier is
// dinah-472/criteria/55 and /56.
func TestEveryTerminalVerbTakesEitherSpellingAndStoresTheIdentifier(t *testing.T) {
	for _, settling := range []struct {
		verb string
		kind string
		ref  string
	}{
		{"resolve", "open_question", "fx-1/questions/1"},
		{"verify", "acceptance_criterion", "fx-1/criteria/1"},
		{"fail", "acceptance_criterion", "fx-1/criteria/1"},
		{"waive", "acceptance_criterion", "fx-1/criteria/1"},
		{"withdraw", "open_question", "fx-1/questions/1"},
	} {
		t.Run(settling.verb, func(t *testing.T) {
			// The positional spelling first, then the identifier spelling on
			// a second item, and both store the identifier.
			for _, spelling := range []string{"position", "identifier"} {
				root := statesFixture(t, "a card carrying an item for "+settling.verb)
				mustRun(t, root, "file", "fx-1", settling.kind, "something to settle")
				mustRun(t, root, "comment", settling.ref, "the answer of record")
				named := settling.ref + "/comments/1"
				if spelling == "identifier" {
					named = commentIDOf(t, root, settling.ref+"/comments/1")
				}
				mustRun(t, root, settling.verb, settling.ref, named)
				stored := itemKeyOf(t, root, settling.ref, bench.ItemResolutionField)
				if !bench.IsID(stored) {
					t.Errorf("the %s spelling stored %q, wanted a bare identifier", spelling, stored)
				}
				if strings.Contains(stored, "/") {
					t.Errorf("the stored answer %q carries a slash, so it is a reference rather than an identifier", stored)
				}
			}

			// A reference naming another item's comment is refused, which is
			// the check the identifier spelling must not have loosened.
			root := statesFixture(t, "a card carrying two items")
			mustRun(t, root, "file", "fx-1", settling.kind, "the item being settled")
			mustRun(t, root, "file", "fx-1", "decision", "an item of its own")
			mustRun(t, root, "comment", "fx-1/decisions/1", "somebody else's words")
			refused := mustRefuse(t, root, settling.verb, settling.ref, "fx-1/decisions/1/comments/1")
			assertRefusal(t, refused, contract.NotADesignation, settling.verb+" handed another item's comment")
			card := mustRefuse(t, root, settling.verb, settling.ref, "fx-1/comments/1")
			assertRefusal(t, card, contract.NotADesignation, settling.verb+" handed a card comment")
		})
	}
}

// commentIDOf is one comment's own identifier, resolved through the same
// opener every other case here uses.
func commentIDOf(t *testing.T, root, ref string) string {
	t.Helper()
	opened, err := bench.OpenAwaitingResolution(soleBenchDir(t, root))
	if err != nil {
		t.Fatalf("open the workbench: %v", err)
	}
	entity, err := opened.ResolveEntity(ref)
	if err != nil {
		t.Fatalf("resolve %s: %v", ref, err)
	}
	return entity.ID
}

// TestTheViewsCarryTheIdentifierAndTheComposedReference is
// dinah-472/criteria/57 and /58.
func TestTheViewsCarryTheIdentifierAndTheComposedReference(t *testing.T) {
	root := statesFixture(t, "a card carrying an answered question")
	mustRun(t, root, "file", "fx-1", "open_question", "which vendor do we cite?")
	mustRun(t, root, "comment", "fx-1/questions/1", "a stray note written first")
	mustRun(t, root, "comment", "fx-1/questions/1", "the answer of record")
	mustRun(t, root, "resolve", "fx-1/questions/1", "fx-1/questions/1/comments/2")

	stored := itemKeyOf(t, root, "fx-1/questions/1", bench.ItemResolutionField)
	before := mustRun(t, root, "show", "fx-1", "--fields", "checklist", "--json")
	if !strings.Contains(before.out, "\"resolution_id\": \""+stored+"\"") {
		t.Errorf("the view carries no resolution_id holding the stored identifier:\n%s", before.out)
	}
	if !strings.Contains(before.out, "\"resolution\": \"fx-1/questions/1/comments/2\"") {
		t.Errorf("the view does not compose the answer's position:\n%s", before.out)
	}

	// After the stray note is archived the identifier is the same and the
	// composed position has moved, which is what composing at read time is
	// for.
	mustRun(t, root, "archive", "fx-1/questions/1/comments/1")
	after := mustRun(t, root, "show", "fx-1", "--fields", "checklist", "--json")
	if !strings.Contains(after.out, "\"resolution_id\": \""+stored+"\"") {
		t.Errorf("the stored identifier moved when a comment was archived:\n%s", after.out)
	}
	if !strings.Contains(after.out, "\"resolution\": \"fx-1/questions/1/comments/1\"") {
		t.Errorf("the view did not recompose the answer's position after the archive:\n%s", after.out)
	}
}

// TestADesignatedCommentIsArchivableAndStillRefusesAnUnforcedDeletion is
// dinah-472/criteria/59 and /75.
func TestADesignatedCommentIsArchivableAndStillRefusesAnUnforcedDeletion(t *testing.T) {
	root := statesFixture(t, "a card carrying an answered question")
	mustRun(t, root, "file", "fx-1", "open_question", "which vendor do we cite?")
	mustRun(t, root, "comment", "fx-1/questions/1", "the answer of record")
	mustRun(t, root, "resolve", "fx-1/questions/1", "fx-1/questions/1/comments/1")

	refused := mustRefuse(t, root, "delete", "fx-1/questions/1/comments/1", "--yes")
	assertRefusal(t, refused, contract.NotDesignatable, "an unforced deletion of the designated comment")

	// Archiving it is permitted and needs no guard, which is the point the
	// operator made from the other side: archiving moves a comment rather
	// than destroying it, and with identity keying it can no longer make
	// somebody else's words into the answer.
	mustRun(t, root, "archive", "fx-1/questions/1/comments/1")
	shown := mustRun(t, root, "show", "fx-1", "--fields", "checklist.full")
	if !strings.Contains(shown.out, "the answer of record") {
		t.Errorf("the item stopped citing the comment once it was archived:\n%s", shown.out)
	}

	// And the forced deletion, which reopens the item as part of the same
	// act and records the comment it destroyed.
	mustRun(t, root, "restore", "fx-1/questions/1/comments/1")
	mustRun(t, root, "delete", "fx-1/questions/1/comments/1", "--yes", "--force")
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemPending {
		t.Errorf("the forced deletion left the item at %q, wanted %q", state, bench.ItemPending)
	}
	found := false
	for _, event := range cardJournal(t, root, cardID(t, root, "fx-1")) {
		if event.Event == contract.EventItemReopened && strings.Contains(event.Reason, "fx-1/questions/1/comments/1") {
			found = true
		}
	}
	if !found {
		t.Error("no reopened line names the comment the forced deletion destroyed, so the record it leaves is a bare identifier")
	}
}

// TestAStoreOfEitherVintageIsRefusedByTheOther is dinah-472/criteria/62 and
// /65. Neither build reads the other's store silently.
func TestAStoreOfEitherVintageIsRefusedByTheOther(t *testing.T) {
	if bench.StorageFormat != 11 {
		t.Fatalf("the storage format is %d, and this case is written against the 11 dinah-608 moved it to; the older vintage it plants is still the one dinah-472 left behind", bench.StorageFormat)
	}
	root := statesFixture(t, "a card of the older vintage")
	dir := soleBenchDir(t, root)

	// A store of the previous vintage is refused by name, and the refusal
	// names the conversion.
	stampFormat(t, dir, bench.ResolutionFormat)
	refused := mustRefuse(t, root, "show", "fx-1")
	assertRefusal(t, refused, contract.StoreAwaitingMigration, "an ordinary read of a store of the previous vintage")
	if !strings.Contains(refused.errw, "--migrate-designations") {
		t.Errorf("the refusal names no conversion: %s", refused.errw)
	}
	// check itself opens it, which is what makes the advice followable.
	if got := runCLI(t, root, "check"); strings.Contains(got.errw, contract.StoreAwaitingMigration) {
		t.Errorf("check is refused the store it is the diagnostic for: %s", got.errw)
	}

	// A store of a later vintage is refused too, naming the version this
	// build wanted.
	stampFormat(t, dir, bench.StorageFormat+1)
	newer := mustRefuse(t, root, "show", "fx-1")
	assertRefusal(t, newer, contract.UnsupportedVer, "a read of a store declaring a later format")
}

// TestCheckNamesEverySettledItemCarryingNoAnswer is
// dinah-472/criteria/71. The conversion leaves such items behind, and the set
// has to stay visible once its report has scrolled away.
func TestCheckNamesEverySettledItemCarryingNoAnswer(t *testing.T) {
	root := statesFixture(t, "a card carrying one item of every state")
	for _, state := range []string{bench.ItemResolved, bench.ItemVerified, bench.ItemFailed, bench.ItemWaived, bench.ItemWithdrawn} {
		kind := "acceptance_criterion"
		ref := "fx-1/criteria/1"
		if state == bench.ItemResolved || state == bench.ItemWithdrawn {
			kind = "open_question"
			ref = "fx-1/questions/1"
		}
		settled := statesFixture(t, "a card carrying an item at "+state)
		mustRun(t, settled, "file", "fx-1", kind, "something to settle")
		standAtOn(t, settled, ref, state)
		clearResolutionByHand(t, settled, ref)
		checked := runCLI(t, settled, "check")
		if !strings.Contains(checked.out, "carries no answer of record") {
			t.Errorf("check does not name an item settled at %s carrying no answer:\n%s", state, checked.out)
		}
	}

	// And a pending item carrying none, which asserts nothing and draws no
	// finding.
	mustRun(t, root, "file", "fx-1", "open_question", "nobody has answered it yet")
	checked := runCLI(t, root, "check")
	if strings.Contains(checked.out, "carries no answer of record") {
		t.Errorf("check names a pending item carrying no answer:\n%s", checked.out)
	}
}

// standAtOn lands one named item at a state, which is standAt for a case that
// filed the item itself.
func standAtOn(t *testing.T, root, ref, state string) {
	t.Helper()
	switch state {
	case bench.ItemResolved:
		mustRun(t, root, "resolve", ref, "--text", "somebody answered it")
	case bench.ItemVerified:
		mustRun(t, root, "verify", ref, "--text", "the check was run")
	case bench.ItemFailed:
		mustRun(t, root, "fail", ref, "--text", "the check did not hold")
	case bench.ItemWaived:
		mustRun(t, root, "waive", ref, "--text", "the operator decided")
	case bench.ItemWithdrawn:
		mustRun(t, root, "withdraw", ref, "--text", "the question stopped applying")
	default:
		t.Fatalf("standAtOn has no route to %q", state)
	}
}

// clearResolutionByHand removes an item's answer straight off its anchor,
// which is the state the conversion leaves an unrecoverable item in and which
// no verb produces on its own.
func clearResolutionByHand(t *testing.T, root, ref string) {
	t.Helper()
	dir := itemDirOf(t, root, ref)
	fm, body, err := bench.ReadItemAnchor(dir)
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	fm.Delete(bench.ItemResolutionField)
	if err := bench.WriteItemAnchor(dir, fm, body); err != nil {
		t.Fatalf("write the anchor of %s: %v", ref, err)
	}
}
