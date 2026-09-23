package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// designationDefinition is a three-column flow whose done column holds a card
// against the items that name it, so one fixture reaches both the conversion
// and the gate the conversion must not disturb.
const designationDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Designations",
  "columns": [
    { "id": "d00000000001", "title": "Intake", "kind": "intake" },
    { "id": "d00000000002", "title": "Doing", "kind": "work" },
    { "id": "d00000000003", "title": "Done", "kind": "done", "gate_items": true }
  ]
}`

// windBack puts one item's stored answer back into the shape a build of the
// previous vintage left behind, which is the designated comment's position
// among that item's live comments.
//
// It is called at the moment the settling happened rather than at the end of
// the fixture, because a position is only what the older build wrote if it is
// read before anything moved. Winding back after a deletion would store the
// position the comment holds now, which is the one thing a store awaiting the
// conversion never carries.
//
// The store is built forward with this build's own verbs and then wound back
// rather than planted by hand, because what the conversion reads is each
// card's journal, and a hand-built journal would be a second author's idea of
// what the verbs write. Winding back touches the stored answers alone and
// leaves every journal exactly as the verbs wrote it.
func windBack(t *testing.T, root, ref string) {
	t.Helper()
	dir := itemDirOf(t, root, ref)
	item, err := bench.LoadItem(dir)
	if err != nil {
		t.Fatalf("load %s: %v", ref, err)
	}
	if item.Resolution == "" {
		t.Fatalf("%s designates nothing, so there is no answer to wind back", ref)
	}
	comments, err := bench.Comments(dir)
	if err != nil {
		t.Fatalf("read the comments of %s: %v", ref, err)
	}
	position := 0
	for at, comment := range comments {
		if comment.ID == item.Resolution {
			position = at + 1
		}
	}
	if position == 0 {
		t.Fatalf("%s designates %q and no live comment of it carries that identifier", ref, item.Resolution)
	}
	fm, body, err := bench.ReadItemAnchor(dir)
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	fm.Set(bench.ItemResolutionField, ref+"/comments/"+strconv.Itoa(position))
	if err := bench.WriteItemAnchor(dir, fm, body); err != nil {
		t.Fatalf("write the anchor of %s: %v", ref, err)
	}
}

// awaitingConversion stamps the workbench at the format a store carrying
// positional answers declares, which is the last thing a fixture does: every
// ordinary read is refused from that moment until the conversion runs.
func awaitingConversion(t *testing.T, root string) {
	t.Helper()
	stampFormat(t, soleBenchDir(t, root), bench.ResolutionFormat)
}

// stampFormat writes a format number onto a workbench anchor, which is how a
// fixture declares the vintage it belongs to.
func stampFormat(t *testing.T, dir string, format int) {
	t.Helper()
	anchor := filepath.Join(dir, bench.WorkbenchAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set("format", strconv.Itoa(format))
	if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// designationFixture builds the four items the conversion decides four
// different ways, and winds the store back to the vintage that stored a
// position.
//
// Each item is named for the route it takes, because the report groups by
// route and a case reading the report has to know which item it is looking at
// without counting.
func designationFixture(t *testing.T) string {
	t.Helper()
	root := designationCards(t)
	awaitingConversion(t, root)
	return root
}

// designationHeldFixture is designationFixture with one card claimed, which
// is the state the conversion's own refusal is about.
//
// The claim is taken before the store declares the older format, because a
// claim is an ordinary write and an ordinary write of such a store is refused
// by name. What the refusal under test reads is the claim on disk rather than
// the act that put it there.
func designationHeldFixture(t *testing.T) string {
	t.Helper()
	root := designationCards(t)
	if got := runCLI(t, root, "move", "fx-1", "doing"); got.code != 0 {
		t.Fatalf("move fx-1 to doing: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "claim", "fx-1", "--actor", "sam"); got.code != 0 {
		t.Fatalf("claim: %d %s", got.code, got.errw)
	}
	awaitingConversion(t, root)
	return root
}

// designationCards builds the items and winds each answer back, and stops
// before the store declares the older format, so a caller can make one more
// ordinary write first.
func designationCards(t *testing.T) string {
	t.Helper()
	root := newBenchFromDefinition(t, designationDefinition)
	if got := runCLI(t, root, "add", "a card carrying four answers"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}

	// The journal route. A stray comment stands first, the settling mints its
	// own comment and journals its identifier, and the stray is then deleted,
	// so the stored position now reaches nothing and only the journal can say
	// what the answer was.
	if got := runCLI(t, root, "file", "fx-1", "acceptance_criterion", "the journal route"); got.code != 0 {
		t.Fatalf("file the journal criterion: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "comment", "fx-1/criteria/1", "a stray note written before anybody answered"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "verify", "fx-1/criteria/1", "--text", "the check was run and it held"); got.code != 0 {
		t.Fatalf("verify: %d %s", got.code, got.errw)
	}
	windBack(t, root, "fx-1/criteria/1")
	if got := runCLI(t, root, "delete", "fx-1/criteria/1/comments/1", "--yes"); got.code != 0 {
		t.Fatalf("delete the stray note: %d %s", got.code, got.errw)
	}

	// The undisturbed route, on a card of its own. An existing comment is
	// named as the answer, so the journal carries no identifier for the
	// settling, and nothing is removed from that card afterwards, so no
	// position can have shifted.
	//
	// The second card is what makes it undisturbed rather than merely
	// untouched. A comment deleted outright is gone from every collection, so
	// the journal cannot say whose it was, and the conversion reads a removal
	// it cannot attribute as a disturbance of every item on that card. That
	// is the safe direction and it is why this item stands where the other
	// card's deletions cannot reach it.
	if got := runCLI(t, root, "add", "a card whose answer nothing disturbed"); got.code != 0 {
		t.Fatalf("add the second card: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "fx-2", "decision", "the undisturbed route"); got.code != 0 {
		t.Fatalf("file the undisturbed decision: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "comment", "fx-2/decisions/1", "the reasoning somebody wrote out first"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "cite", "fx-2/decisions/1", "url", "https://example.invalid/reasoning"); got.code != 0 {
		t.Fatalf("cite: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "resolve", "fx-2/decisions/1", "fx-2/decisions/1/comments/1"); got.code != 0 {
		t.Fatalf("resolve by naming a comment: %d %s", got.code, got.errw)
	}
	windBack(t, root, "fx-2/decisions/1")

	// The unrecoverable case. An existing comment is named, so no identifier
	// was journalled for the settling, and a later deletion moves the
	// positions underneath the stored reference, so the reference now reaches
	// somebody else's words.
	//
	// The citation between the comment and the settling is what makes the
	// case the one it is about. The journal route reads the line immediately
	// before the settling, so a settling that follows a comment of its own
	// item directly is the minted form whatever else is true; putting
	// another act of that item in between is how an answer recorded by
	// naming an existing comment is told apart from one the settling minted.
	if got := runCLI(t, root, "file", "fx-1", "open_question", "the unrecoverable route"); got.code != 0 {
		t.Fatalf("file the unrecoverable question: %d %s", got.code, got.errw)
	}
	for _, body := range []string{"the answer somebody meant", "an opinion written afterwards"} {
		if got := runCLI(t, root, "comment", "fx-1/questions/1", body); got.code != 0 {
			t.Fatalf("comment: %d %s", got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "cite", "fx-1/questions/1", "url", "https://example.invalid/evidence"); got.code != 0 {
		t.Fatalf("cite: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "resolve", "fx-1/questions/1", "fx-1/questions/1/comments/1"); got.code != 0 {
		t.Fatalf("resolve by naming a comment: %d %s", got.code, got.errw)
	}
	windBack(t, root, "fx-1/questions/1")
	if got := runCLI(t, root, "comment", "fx-1/questions/1", "a third note nobody designated"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "delete", "fx-1/questions/1/comments/3", "--yes"); got.code != 0 {
		t.Fatalf("delete the third note: %d %s", got.code, got.errw)
	}

	// An item whose comments hold an archived member, which is the population
	// whose positions may already have drifted before anybody ran this.
	if got := runCLI(t, root, "file", "fx-1", "acceptance_criterion", "the archived route"); got.code != 0 {
		t.Fatalf("file the archived criterion: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "comment", "fx-1/criteria/2", "a note archived afterwards"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "verify", "fx-1/criteria/2", "--text", "this one was checked too"); got.code != 0 {
		t.Fatalf("verify: %d %s", got.code, got.errw)
	}
	windBack(t, root, "fx-1/criteria/2")
	if got := runCLI(t, root, "archive", "fx-1/criteria/2/comments/1"); got.code != 0 {
		t.Fatalf("archive the note: %d %s", got.code, got.errw)
	}
	return root
}

// itemResolutionOf reads one item's stored answer straight off its anchor,
// which is what the conversion writes and what a reference composed for a
// reader is composed from.
func itemResolutionOf(t *testing.T, root, ref string) string {
	t.Helper()
	return itemKeyOf(t, root, ref, bench.ItemResolutionField)
}

// itemStateOf is the state one item's own anchor records.
func itemStateOf(t *testing.T, root, ref string) string {
	t.Helper()
	return itemKeyOf(t, root, ref, bench.ItemStateField)
}

// itemKeyOf reads one key off an item's own anchor.
//
// It reads the file rather than running dinah get, because half the cases here
// read an item on a store the conversion has not reached yet, and an ordinary
// read of such a store is refused by name. What these cases are about is what
// is on disk before and after the run, so the file is the honest source.
func itemKeyOf(t *testing.T, root, ref, key string) string {
	t.Helper()
	fm, _, err := bench.ReadItemAnchor(itemDirOf(t, root, ref))
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	return fm.Value(key)
}

// itemDirOf resolves one item's own directory through the diagnostic opener,
// which is the opener that reads a store the conversion has not reached.
func itemDirOf(t *testing.T, root, ref string) string {
	t.Helper()
	opened, err := bench.OpenAwaitingResolution(soleBenchDir(t, root))
	if err != nil {
		t.Fatalf("open the workbench: %v", err)
	}
	entity, err := opened.ResolveEntity(ref)
	if err != nil {
		t.Fatalf("resolve %s: %v", ref, err)
	}
	return entity.Dir
}

// TestTheConversionDecidesFromTheJournalAndLeavesTheRestUnanswered is
// dinah-472/criteria/46, /63, /64, /67, /68 and /69 driven as one run, because
// the four routes are four groups of one report and reading them apart would
// need four conversions of four fixtures.
//
// What it pins is that the conversion reads the card's history rather than the
// item's live comments. The journal-route item's stored position reaches
// nothing at all after the deletion, and the unrecoverable item's stored
// position reaches a comment nobody designated, so a build resolving the
// stored reference would write one answer it could not have found and one that
// is somebody else's words.
func TestTheConversionDecidesFromTheJournalAndLeavesTheRestUnanswered(t *testing.T) {
	root := designationFixture(t)

	// What the stored reference reaches before the run, asserted so that the
	// case cannot pass against a build that resolved positions: the journal
	// route's stored position is past the end of its collection, and the
	// unrecoverable one reaches the opinion rather than the answer.
	if got := itemResolutionOf(t, root, "fx-1/criteria/1"); got != "fx-1/criteria/1/comments/2" {
		t.Fatalf("the journal-route criterion stores %q, so the fixture is not in the shape this case is about", got)
	}

	converted := runCLI(t, root, "check", "--migrate-designations", "--actor", "alka")
	// The exit code is check's own, and this run leaves an item with no
	// answer of record, which check reports as a finding. What the case is
	// about is the report and what the run wrote, so the code is read as
	// check's rather than as the conversion's verdict.
	assertConverted(t, converted)

	// The journal route recovered the comment the settling minted, which is
	// the one carrying the answer's own words.
	journalAnswer := itemResolutionOf(t, root, "fx-1/criteria/1")
	if !bench.IsID(journalAnswer) {
		t.Errorf("the journal-route criterion stores %q, wanted a bare identifier", journalAnswer)
	}
	if body := designatedBodyOf(t, root, "fx-1/criteria/1"); body != "the check was run and it held" {
		t.Errorf("the journal-route criterion now cites %q, wanted the comment the settling minted", body)
	}

	// The undisturbed route converted the comment the stored position still
	// reached, because nothing had moved since the settling.
	if body := designatedBodyOf(t, root, "fx-2/decisions/1"); body != "the reasoning somebody wrote out first" {
		t.Errorf("the undisturbed decision now cites %q", body)
	}

	// The unrecoverable item lost its answer and kept its state, which is the
	// operator's own ruling: the conversion does not guess.
	if got := itemResolutionOf(t, root, "fx-1/questions/1"); got != "" {
		t.Errorf("the unrecoverable question stores %q, wanted nothing at all", got)
	}
	if state := itemStateOf(t, root, "fx-1/questions/1"); state != bench.ItemResolved {
		t.Errorf("the unrecoverable question stands at %q, wanted the %q it stood at before the run", state, bench.ItemResolved)
	}

	// The report names each group and the item in it, and the unanswered
	// count is repeated on the last line because it is the number the
	// operator acts on.
	for _, wanted := range []string{
		"Converted 2 items from the history.",
		"Converted 1 item as undisturbed.",
		"1 converted item carries an archived comment.",
		"Left 1 item unanswered.",
		"Each item above keeps the state it stands at and has lost its answer of record.",
		"Stamped format 7 on the workbench anchor.",
		"1 item is waiting for somebody to answer it again.",
	} {
		if !strings.Contains(converted.out, wanted) {
			t.Errorf("the report does not carry %q:\n%s", wanted, converted.out)
		}
	}
	// The unanswered line names the reference that was stored and the author
	// the reference reaches today, so the operator sees what a guessing
	// conversion would have written down.
	if !strings.Contains(converted.out, "keeps resolved") || !strings.Contains(converted.out, "alka") {
		t.Errorf("the unanswered line does not say what the item keeps and whose comment its stored reference reaches:\n%s", converted.out)
	}

	// The store declares the new format and an ordinary read is admitted,
	// which is the whole point of running it.
	if got := runCLI(t, root, "show", "fx-1", "--fields", "card"); got.code != 0 {
		t.Fatalf("the converted store still refuses a read: %d %s", got.code, got.errw)
	}

	// dinah check names the item the conversion left unanswered, so the set
	// stays visible once the report has scrolled away.
	checked := runCLI(t, root, "check")
	if !strings.Contains(checked.out, "carries no answer of record") {
		t.Errorf("check does not name the unanswered item:\n%s", checked.out)
	}
}

// TestTheEvidencedRouteWinsOverTheInferredOneInsideOneSecond is Agent Code
// Review's own reproduction, and it is the case the route order exists for.
//
// Three commands, all inside one second: a comment carrying the answer, a
// second comment carrying an aside, and a settling naming the first. Nothing
// is ever archived, restored or deleted, so the stored position is still
// correct and the conversion has evidence rather than a hint. The journal
// cannot tell the two invocations apart, because its stamp carries seconds and
// all three lines carry the same one, so a conversion that consulted the
// journal first matches the aside and writes it down as the answer of record,
// silently, on a store where the value on disk was right all along.
//
// The case asserts which comment the item ends up citing rather than which
// route the report named, because what the operator loses is the answer and
// not the label. It asserts the rehearsal as well, since rehearsing first is
// the one protection he was offered and it prints whatever the converting run
// would print.
func TestTheEvidencedRouteWinsOverTheInferredOneInsideOneSecond(t *testing.T) {
	root := oneSecondFixture(t)

	rehearsed := runCLI(t, root, "check", "--migrate-designations", "--rehearse", "--actor", "alka")
	if strings.TrimSpace(rehearsed.errw) != "" {
		t.Fatalf("the rehearsal was refused: %s", rehearsed.errw)
	}
	converted := runCLI(t, root, "check", "--migrate-designations", "--actor", "alka")
	assertConverted(t, converted)

	if body := designatedBodyOf(t, root, "fx-1/questions/1"); body != "THE REAL ANSWER: go left" {
		t.Errorf("the item now cites %q, and the answer somebody settled it with is the one it stored", body)
	}
	for _, report := range []struct {
		what string
		out  string
	}{{"the rehearsal", rehearsed.out}, {"the conversion", converted.out}} {
		if !strings.Contains(report.out, "Converted 1 item as undisturbed.") {
			t.Errorf("%s did not read the stored position as evidence:\n%s", report.what, report.out)
		}
		if !strings.Contains(report.out, "Converted 0 items from the history.") {
			t.Errorf("%s consulted the history over a store nothing had disturbed:\n%s", report.what, report.out)
		}
	}
}

// oneSecondFixture builds the three acts the case above is about and hands
// back a workbench awaiting the conversion, retrying until the three land
// inside one second.
//
// The retry is what keeps the case from asserting nothing on a slow machine. A
// run whose acts straddled a second boundary would exercise a different
// population and pass without touching this one, and skipping there would be
// the same silence wearing a label, so the fixture is rebuilt instead and the
// case fails outright if it never lands.
func oneSecondFixture(t *testing.T) string {
	t.Helper()
	for attempt := 0; attempt < 8; attempt++ {
		root := newBenchFromDefinition(t, designationDefinition)
		mustRun(t, root, "add", "a card whose answer and aside share one second")
		mustRun(t, root, "file", "fx-1", "open_question", "which way do we go?")
		mustRun(t, root, "comment", "fx-1/questions/1", "THE REAL ANSWER: go left")
		mustRun(t, root, "comment", "fx-1/questions/1", "an aside written just now")
		mustRun(t, root, "resolve", "fx-1/questions/1", "fx-1/questions/1/comments/1")
		if !sharesOneSecond(t, root, "fx-1") {
			continue
		}
		// The stored position still reaches the real answer, which is what
		// makes this the undisturbed population rather than a recovery.
		windBack(t, root, "fx-1/questions/1")
		if stored := itemResolutionOf(t, root, "fx-1/questions/1"); stored != "fx-1/questions/1/comments/1" {
			t.Fatalf("the item stores %q, so this case is not the one it is about", stored)
		}
		awaitingConversion(t, root)
		return root
	}
	t.Fatal("eight attempts and the comment, the aside and the settling never landed inside one second, so this case asserted nothing")
	return ""
}

// sharesOneSecond reports whether every comment and settling line one card's
// journal carries names the same instant.
func sharesOneSecond(t *testing.T, root, card string) bool {
	t.Helper()
	stamps := map[string]bool{}
	for _, event := range cardJournal(t, root, cardID(t, root, card)) {
		switch event.Event {
		case contract.EventCommented, contract.EventItemResolved:
			stamps[event.TS] = true
		}
	}
	if len(stamps) == 0 {
		t.Fatalf("%s carries no comment or settling line, so the fixture is broken rather than slow", card)
	}
	return len(stamps) == 1
}

// TestTheUnansweredLineNamesEveryAbsenceItCanCarry is Agent Code Review's
// third finding, which is the one group of the report the operator is told to
// act on rendering a sentence with a hole in it.
//
// Two slots of that line can be empty and both are reachable by design. An
// item whose card journal records no settling at all is unrecoverable by
// section 5.13.2's own reading, and it left the line reading "settled by ,
// keeps failed". A stored reference reaching no comment of the item is the
// ordinary shape of a removal since the settling, and it left the line
// trailing off into "reaches  today".
//
// The case drives both on one card and reads the rendered sentences, because
// the defect was invisible to every assertion that read a count or a key.
func TestTheUnansweredLineNamesEveryAbsenceItCanCarry(t *testing.T) {
	root := newBenchFromDefinition(t, designationDefinition)
	mustRun(t, root, "add", "a card whose history says nothing")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "something nobody can speak for")
	mustRun(t, root, "comment", "fx-1/criteria/1", "the answer of record")
	mustRun(t, root, "fail", "fx-1/criteria/1", "fx-1/criteria/1/comments/1")
	windBack(t, root, "fx-1/criteria/1")

	// The journal is emptied, which is the state a card whose history is gone
	// arrives in and which puts the item squarely in the unrecoverable
	// population with no settling to name.
	if err := os.WriteFile(cardJournalPath(t, root, "fx-1"), nil, 0o644); err != nil {
		t.Fatalf("empty the card's journal: %v", err)
	}
	// And a second card whose stored reference reaches nothing, because the
	// comment it named was deleted outright.
	mustRun(t, root, "add", "a card whose answer was deleted")
	mustRun(t, root, "file", "fx-2", "acceptance_criterion", "something whose answer went")
	mustRun(t, root, "comment", "fx-2/criteria/1", "the answer that will go")
	mustRun(t, root, "fail", "fx-2/criteria/1", "fx-2/criteria/1/comments/1")
	windBack(t, root, "fx-2/criteria/1")
	mustRun(t, root, "delete", "fx-2/criteria/1/comments/1", "--yes", "--force")
	awaitingConversion(t, root)

	converted := runCLI(t, root, "check", "--migrate-designations", "--actor", "alka")
	assertConverted(t, converted)
	if !strings.Contains(converted.out, "Left 2 items unanswered.") {
		t.Fatalf("the run did not leave both items unanswered, so this case is not the one it is about:\n%s", converted.out)
	}
	if strings.Contains(converted.out, "settled by ,") {
		t.Errorf("the unanswered line leaves the settling slot blank:\n%s", converted.out)
	}
	if strings.Contains(converted.out, "reaches  today") {
		t.Errorf("the unanswered line leaves the comment it reaches unnamed:\n%s", converted.out)
	}
	if !strings.Contains(converted.out, "its history records no settling") {
		t.Errorf("the unanswered line does not say that the history records no settling:\n%s", converted.out)
	}
	if !strings.Contains(converted.out, "no comment of that item") {
		t.Errorf("the unanswered line does not say that the stored reference reaches no comment:\n%s", converted.out)
	}
}

// cardJournalPath is one card's own journal file, which a case emptying a
// history needs by path rather than by reference.
func cardJournalPath(t *testing.T, root, ref string) string {
	t.Helper()
	opened, err := bench.OpenAwaitingResolution(soleBenchDir(t, root))
	if err != nil {
		t.Fatalf("open the workbench: %v", err)
	}
	entity, err := opened.ResolveEntity(ref)
	if err != nil {
		t.Fatalf("resolve %s: %v", ref, err)
	}
	return entity.Card.JournalPath()
}

// TestTheResidualCaseIsNamedWhicheverWayTheRemovalHappened is Agent Code
// Review's second-cycle finding, and it is about disclosure rather than about
// the answer the conversion writes.
//
// The residue this card cannot close is an item whose positions have moved and
// whose settling followed an unrelated comment of its own inside one second.
// The conversion falls to the journal route there, the stamp carries seconds
// and cannot separate the two invocations, and the item can be given the wrong
// comment. Both halves of that are driven here and the case asserts what the
// report says rather than pretending the answer is right.
//
// The two halves differ only in how the earlier comment was removed, and that
// difference is the finding. An archived comment leaves something in the
// archive for a narrow signal to find; a deleted one leaves nothing anywhere,
// which is the blind spot this card already paid for once, when an earlier
// draft detected drift by looking for archived comments and the design review
// defeated it by deleting instead. A caution that inherited that blind spot
// would miss the one removal the card knows it cannot see, so the caution
// stands over the journal group, which is exactly the inferred population.
func TestTheResidualCaseIsNamedWhicheverWayTheRemovalHappened(t *testing.T) {
	for _, removal := range []string{"archive", "delete"} {
		t.Run(removal, func(t *testing.T) {
			root := residualFixture(t, removal)
			converted := runCLI(t, root, "check", "--migrate-designations", "--actor", "alka")
			assertConverted(t, converted)

			// The misfire itself, asserted rather than assumed, because a
			// build that had closed it would make the caution below a
			// caution about nothing and this case would go on passing.
			if !strings.Contains(converted.out, "Converted 1 item from the history.") {
				t.Fatalf("the run did not fall to the journal route, so this case is not the one it is about:\n%s", converted.out)
			}

			// The caution, which is what the finding asked for and which has
			// to be there whichever way the removal happened.
			if !strings.Contains(converted.out, "recovered from the history rather than from what was stored") {
				t.Errorf("the report does not say that the items above were inferred:\n%s", converted.out)
			}
			if !strings.Contains(converted.out, "may not be the one it meant") {
				t.Errorf("the report does not say that the comment it named may be the wrong one:\n%s", converted.out)
			}

			// And the archived listing is shown to be the narrower signal
			// the caution replaces: it names the item on one half and not on
			// the other, which is why it cannot carry this on its own.
			named := strings.Contains(converted.out, "1 converted item carries an archived comment.")
			if removal == "archive" && !named {
				t.Errorf("the archived listing does not name the item on the half it can see:\n%s", converted.out)
			}
			if removal == "delete" && named {
				t.Errorf("the archived listing claims to see a deleted comment, so this case is not showing what it is about:\n%s", converted.out)
			}
		})
	}
}

// residualFixture builds an item whose positions have moved and whose settling
// shares a second with an unrelated comment of its own, removing the earlier
// comment the way the caller names.
//
// The four acts have to land inside one second for the journal route's guard
// to match at all, so the fixture is rebuilt until they do and fails outright
// rather than skipping, on the terms oneSecondFixture keeps.
func residualFixture(t *testing.T, removal string) string {
	t.Helper()
	for attempt := 0; attempt < 8; attempt++ {
		root := newBenchFromDefinition(t, designationDefinition)
		mustRun(t, root, "add", "a card whose positions moved inside one second")
		mustRun(t, root, "file", "fx-1", "open_question", "which way do we go?")
		mustRun(t, root, "comment", "fx-1/questions/1", "a stray note written first")
		mustRun(t, root, "comment", "fx-1/questions/1", "THE REAL ANSWER: go left")
		mustRun(t, root, "comment", "fx-1/questions/1", "an aside written just now")
		mustRun(t, root, "resolve", "fx-1/questions/1", "fx-1/questions/1/comments/2")
		if !sharesOneSecond(t, root, "fx-1") {
			continue
		}
		windBack(t, root, "fx-1/questions/1")
		// The removal is what moves the positions, and it is the whole of
		// the difference between the two halves.
		argv := []string{removal, "fx-1/questions/1/comments/1"}
		if removal == "delete" {
			argv = append(argv, "--yes")
		}
		mustRun(t, root, argv...)
		awaitingConversion(t, root)
		return root
	}
	t.Fatal("eight attempts and the three comments and the settling never landed inside one second, so this case asserted nothing")
	return ""
}

// TestASecondConversionWritesNothing is dinah-472/criteria/74. A converted
// store carries no positional answer, so the run finds nothing, reports
// nothing converted and leaves every anchor and every journal where it was.
func TestASecondConversionWritesNothing(t *testing.T) {
	root := designationFixture(t)
	assertConverted(t, runCLI(t, root, "check", "--migrate-designations", "--actor", "alka"))
	before := treeDigest(t, soleBenchDir(t, root))

	second := runCLI(t, root, "check", "--migrate-designations", "--actor", "alka")
	if strings.TrimSpace(second.errw) != "" {
		t.Fatalf("the second conversion was refused: %s", second.errw)
	}
	for _, wanted := range []string{
		"Converted 0 items from the history.",
		"Converted 0 items as undisturbed.",
		"Left 0 items unanswered.",
	} {
		if !strings.Contains(second.out, wanted) {
			t.Errorf("the second run does not report %q:\n%s", wanted, second.out)
		}
	}
	if after := treeDigest(t, soleBenchDir(t, root)); after != before {
		t.Error("the second conversion changed the store, and a run finding nothing to convert writes nothing")
	}
}

// TestTheRehearsalDecidesEverythingAndWritesNothing is
// dinah-472/criteria/73. The rehearsal is the form an agent may run: it is
// refused to nobody, it is admitted on a workbench carrying a claimed card,
// and the store it leaves behind is the store it found.
func TestTheRehearsalDecidesEverythingAndWritesNothing(t *testing.T) {
	root := designationHeldFixture(t)
	before := treeDigest(t, soleBenchDir(t, root))

	rehearsed := runCLI(t, root, "check", "--migrate-designations", "--rehearse", "--actor", "sam")
	if strings.TrimSpace(rehearsed.errw) != "" {
		t.Fatalf("the rehearsal was refused: %s", rehearsed.errw)
	}
	for _, wanted := range []string{
		"Converted 2 items from the history.",
		"Converted 1 item as undisturbed.",
		"Left 1 item unanswered.",
		"Nothing was written. Run it again without --rehearse to convert and stamp the workbench.",
	} {
		if !strings.Contains(rehearsed.out, wanted) {
			t.Errorf("the rehearsal does not report %q:\n%s", wanted, rehearsed.out)
		}
	}
	if strings.Contains(rehearsed.out, "Stamped format") {
		t.Errorf("the rehearsal claims to have stamped the workbench:\n%s", rehearsed.out)
	}
	if after := treeDigest(t, soleBenchDir(t, root)); after != before {
		t.Error("the rehearsal changed the store, and a rehearsal writes nothing at all")
	}
}

// TestTheConversionIsTheOperatorsAndWaitsForAnIdleWorkbench is
// dinah-472/criteria/72, with the accepting case beside each refusal so that
// nothing here passes against a build that refuses everybody.
func TestTheConversionIsTheOperatorsAndWaitsForAnIdleWorkbench(t *testing.T) {
	root := designationHeldFixture(t)

	refused := runCLI(t, root, "check", "--migrate-designations", "--actor", "sam")
	assertRefusal(t, refused, contract.NotOperator, "the conversion run by somebody who is not the operator")

	held := runCLI(t, root, "check", "--migrate-designations", "--actor", "alka")
	assertRefusal(t, held, contract.WorkbenchInUse, "the conversion over a held card")
	if !strings.Contains(held.errw, "fx-1") || !strings.Contains(held.errw, "sam") {
		t.Errorf("the refusal names neither the card nor its holder: %s", held.errw)
	}

	// The force is the operator's alone, and it names in the report and in
	// the workbench's own history every claim it passed.
	forcedByAnother := runCLI(t, root, "check", "--migrate-designations", "--force-claims", "--actor", "sam")
	assertRefusal(t, forcedByAnother, contract.NotOperator, "the force run by somebody who is not the operator")

	forced := runCLI(t, root, "check", "--migrate-designations", "--force-claims", "--actor", "alka")
	assertConverted(t, forced)
	if !strings.Contains(forced.out, "Passed 1 card somebody still holds.") || !strings.Contains(forced.out, "held by sam") {
		t.Errorf("the forced run does not name the claim it passed:\n%s", forced.out)
	}
	events, _, err := bench.ReadJournal(filepath.Join(soleBenchDir(t, root), bench.JournalName))
	if err != nil {
		t.Fatalf("read the workbench journal: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Event != contract.EventDesignationsMigrated {
			continue
		}
		found = true
		if len(event.Cards) != 1 || event.Cards[0] != "fx-1" {
			t.Errorf("the forced run's own line names %v, wanted the one card it passed", event.Cards)
		}
	}
	if !found {
		t.Errorf("the forced run wrote no %s line, so the judgement it recorded is nameable nowhere", contract.EventDesignationsMigrated)
	}
}

// TestAnUnansweredItemHoldsExactlyWhatItHeldBefore is
// dinah-472/criteria/70. Every hold reads an item's state and none reads its
// answer, so clearing an answer moves no card: a waived criterion goes on
// admitting the gated move with no override and a failed one goes on refusing
// it by name.
func TestAnUnansweredItemHoldsExactlyWhatItHeldBefore(t *testing.T) {
	root := newBenchFromDefinition(t, designationDefinition)
	for _, title := range []string{"a card the waiver lets through", "a card the finding holds"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	// Both criteria name the gated column, and both are settled with an
	// answer the conversion will not be able to recover: the answer names an
	// existing comment, so nothing is journalled, and a later deletion moves
	// the positions underneath it.
	for at, ref := range []string{"fx-1", "fx-2"} {
		if got := runCLI(t, root, "file", "--column", "done", ref, "acceptance_criterion", "something to check"); got.code != 0 {
			t.Fatalf("file on %s: %d %s", ref, got.code, got.errw)
		}
		for _, body := range []string{"what the check showed", "a note written afterwards"} {
			if got := runCLI(t, root, "comment", ref+"/criteria/1", body); got.code != 0 {
				t.Fatalf("comment on %s: %d %s", ref, got.code, got.errw)
			}
		}
		// The citation between the last comment and the settling is what
		// makes the answer a named one rather than a minted one, which is
		// the shape the conversion cannot recover and this case needs.
		if got := runCLI(t, root, "cite", ref+"/criteria/1", "url", "https://example.invalid/evidence"); got.code != 0 {
			t.Fatalf("cite on %s: %d %s", ref, got.code, got.errw)
		}
		verb := "waive"
		if at == 1 {
			verb = "fail"
		}
		if got := runCLI(t, root, verb, ref+"/criteria/1", ref+"/criteria/1/comments/1"); got.code != 0 {
			t.Fatalf("%s on %s: %d %s", verb, ref, got.code, got.errw)
		}
		windBack(t, root, ref+"/criteria/1")
		if got := runCLI(t, root, "comment", ref+"/criteria/1", "a third note nobody designated"); got.code != 0 {
			t.Fatalf("comment on %s: %d %s", ref, got.code, got.errw)
		}
		if got := runCLI(t, root, "delete", ref+"/criteria/1/comments/3", "--yes"); got.code != 0 {
			t.Fatalf("delete on %s: %d %s", ref, got.code, got.errw)
		}
	}
	awaitingConversion(t, root)

	assertConverted(t, runCLI(t, root, "check", "--migrate-designations", "--actor", "alka"))
	for _, ref := range []string{"fx-1", "fx-2"} {
		if answer := itemResolutionOf(t, root, ref+"/criteria/1"); answer != "" {
			t.Fatalf("%s's criterion still stores %q, so this case is not exercising an unanswered item", ref, answer)
		}
	}

	// The waived criterion still releases the gate, with no override, and the
	// failed one still refuses the same move by name.
	if got := runCLI(t, root, "move", "fx-1", "done"); got.code != 0 {
		t.Errorf("the waived criterion stopped releasing the gate after the conversion: %d %s", got.code, got.errw)
	}
	held := runCLI(t, root, "move", "fx-2", "done")
	if held.code == 0 {
		t.Fatalf("the failed criterion admitted the gated move after the conversion:\n%s", held.out)
	}
	if name := refusalNameOf(held.errw); name != contract.UnresolvedItem {
		t.Errorf("the failed criterion answered %s after the conversion, wanted %s: %s", name, contract.UnresolvedItem, held.errw)
	}
}

// assertConverted fails a run the conversion refused, and passes one check
// merely found something to report.
//
// The two are told apart by the refusal rather than by the code, because a
// conversion that leaves an item unanswered is a conversion that worked and a
// store carrying an unanswered item is a store check has a finding for.
func assertConverted(t *testing.T, got invocation) {
	t.Helper()
	if strings.TrimSpace(got.errw) != "" {
		t.Fatalf("the conversion was refused: %s", got.errw)
	}
	if !strings.Contains(got.out, "Stamped format") {
		t.Fatalf("the conversion stamped no format, so it did not run: %d\n%s\n%s", got.code, got.out, got.errw)
	}
}

// carryToDoingAt moves a card to a named station, which is carryToDoing over a
// flow that spells its working column differently.
func carryToDoingAt(t *testing.T, root, card, column string) {
	t.Helper()
	if got := runCLI(t, root, "move", card, column); got.code != 0 {
		t.Fatalf("move %s to %s: %d %s", card, column, got.code, got.errw)
	}
}

// itemIDOf is one item's own identifier, read off the reference a person
// types.
func itemIDOf(t *testing.T, root, ref string) string {
	t.Helper()
	return filepath.Base(itemDirOf(t, root, ref))
}

// designatedBodyOf is the body of the comment one item cites as its answer,
// read through the reading surface rather than off the anchor, so the
// composition the read performs is exercised as well as the value stored.
func designatedBodyOf(t *testing.T, root, ref string) string {
	t.Helper()
	answer := itemResolutionOf(t, root, ref)
	if answer == "" {
		return ""
	}
	shown := runCLI(t, root, "show", ref+"/comments/"+answer, "--actor", "alka")
	if shown.code != 0 {
		t.Fatalf("show the designated comment of %s: %d %s", ref, shown.code, shown.errw)
	}
	return lastParagraphOf(shown.out)
}

// lastParagraphOf is the body a comment read prints, which is everything after
// the anchor's own closing fence.
//
// It splits on the fence rather than on what a header line looks like. An
// earlier form skipped every line carrying a colon and a space, which reads a
// body as a header wherever somebody wrote one: the reproduction this file
// carries answers "THE REAL ANSWER: go left", and the helper reported that the
// item cited nothing at all.
func lastParagraphOf(printed string) string {
	lines := strings.Split(strings.ReplaceAll(printed, "\r\n", "\n"), "\n")
	fences := 0
	for at, line := range lines {
		if strings.TrimSpace(line) != "---" {
			continue
		}
		fences++
		if fences == 2 {
			return strings.TrimSpace(strings.Join(lines[at+1:], "\n"))
		}
	}
	return strings.TrimSpace(printed)
}

// treeDigest is a stable summary of every file under a directory, which is how
// a case asserts that a run wrote nothing at all rather than that it wrote
// nothing it reported.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		lines = append(lines, filepath.ToSlash(rel)+" "+strconv.Itoa(len(body))+" "+bench.CommentDigest(string(body)))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(lines) == 0 {
		t.Fatalf("walking %s read no file, so a comparison of two digests would compare nothing", root)
	}
	return strings.Join(lines, "\n")
}
