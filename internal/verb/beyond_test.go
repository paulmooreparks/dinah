package verb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/msg"
)

// TestAddFilesACard asserts the creation rules: the first column, state
// ready, a journal opened with the created event, a title-less request
// refused malformed (CORE-CARD-3), a column the bench does not declare refused
// unknown-column (CORE-CARD-4), and the asymmetric capacity treatment of the
// first column against a named one.
func TestAddFilesACard(t *testing.T) {
	h := newHarness(t)
	ref := h.add("first card")
	card := h.card(ref)
	if card.Column != intake {
		t.Errorf("wanted the first column %s, got %s", intake, card.Column)
	}
	if card.State != contract.StateReady {
		t.Errorf("wanted state ready, got %s", card.State)
	}
	events := h.events(ref)
	if len(events) != 1 || events[0].Event != contract.EventCreated {
		t.Fatalf("wanted a journal opened with the created event, got %+v", events)
	}
	if events[0].Title != "first card" {
		t.Errorf("created event: wanted the title, got %q", events[0].Title)
	}

	if response := h.library.Add(&Request{Verb: "add", Actor: "alka"}); response.Refusal != contract.Malformed {
		t.Errorf("a title-less request: wanted malformed, got %s %s", response.Outcome, response.Refusal)
	}
	if response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "x", Column: "nowhere"}); response.Refusal != contract.UnknownColumn {
		t.Errorf("an unknown column: wanted unknown-column, got %s %s", response.Outcome, response.Refusal)
	}

	// Doing is limited to one card. A filing into it is refused once it is
	// full, while a filing into the first column is admitted whatever.
	full := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "occupant", Column: doing})
	if full.Outcome != contract.OutcomeOK {
		t.Fatalf("first filing into Doing: %s %s", full.Outcome, full.Refusal)
	}
	h.reopen()
	if response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "second", Column: doing}); response.Refusal != contract.AtCapacity {
		t.Errorf("a filing into a full column: wanted at-capacity, got %s %s", response.Outcome, response.Refusal)
	} else if response.Detail != "doing" {
		// Named by the slug a caller could type back ("doing"), not by the
		// raw identifier behind it: dinah-29 cycle 2 fixed this for the
		// legal-moves listing and missed this refusal path.
		t.Errorf("at-capacity refusal: wanted the slug %q, got %q", "doing", response.Detail)
	}
	h.reopen()
	for i := 0; i < 3; i++ {
		if response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "intake filing"}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("a filing into the first column should never be refused, got %s", response.Refusal)
		}
		h.reopen()
	}
}

// TestAddRefusesRatherThanPanicsWithNoLiveColumns asserts AC-7 and AC-8: Add
// against a workbench whose live columns list is empty raises
// contract.AddNeedsAColumn rather than indexing Bench.Columns[0], which is the
// crash the guard exists to stop, and creates no card directory.
// Bench.Open tolerates this shape so check can diagnose it, so the harness
// reaches it by clearing the in-memory slice after opening rather than by
// writing a new fixture: no dinah check --migrate-columns run is involved.
func TestAddRefusesRatherThanPanicsWithNoLiveColumns(t *testing.T) {
	h := newHarness(t)
	h.library.Bench.Columns = nil
	response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "stranded filing"})
	if response.Refusal != contract.AddNeedsAColumn {
		t.Fatalf("wanted contract.AddNeedsAColumn, got %s %s", response.Outcome, response.Refusal)
	}
	want := filepath.Join(h.library.Bench.Root, bench.WorkbenchAnchor)
	if response.Detail != want {
		t.Errorf("wanted the workbench.md path %q as detail, got %q", want, response.Detail)
	}
	entries, err := os.ReadDir(h.library.Bench.CardsRoot())
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read cards root: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("wanted no card directory created, got %v", entries)
	}
}

// TestIdentifiersAreUniqueAcrossBothHalves asserts CORE-CARD-1: an identifier
// in use in the archived half is not reused, which is the uniqueness scope the
// format's archive section fixes.
func TestIdentifiersAreUniqueAcrossBothHalves(t *testing.T) {
	h := newHarness(t)
	ref := h.add("archivable")
	id := h.card(ref).ID
	h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
	h.reopen()
	if !h.library.Bench.HasIdentifier(id) {
		t.Error("an archived identifier should still be in use")
	}
	if h.library.Bench.HasIdentifier("ffffffffffff") {
		t.Error("an identifier nothing carries should be free")
	}
}

// TestCommentAndAttach asserts the comment and attachment entities: the
// anchors they write, the ordering by creation ordinal, and the journal events
// each lifecycle act records.
//
// The clock runs backwards between the two comments, so the write order and
// the timestamp order disagree and the assertion below can only pass on the
// ordinal.
func TestCommentAndAttach(t *testing.T) {
	h := newHarness(t)
	ref := h.add("annotated")

	h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: ref, Text: "the first thought"})
	h.reopen()
	h.advance(-time.Hour)
	h.library.Comment(&Request{Verb: "comment", Actor: "bob", Card: ref, Text: "the second thought"})
	h.reopen()

	comments, err := bench.Comments(h.card(ref).Dir)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("wanted two comments, got %d", len(comments))
	}
	if !strings.Contains(comments[0].Body, "first thought") {
		t.Errorf("comments should order by creation ordinal, got %q first", comments[0].Body)
	}
	if comments[0].Author != "alka" {
		t.Errorf("author: wanted alka, got %q", comments[0].Author)
	}
	if comments[0].Ordinal != 1 || comments[1].Ordinal != 2 {
		t.Errorf("wanted ordinals 1 and 2, got %d and %d", comments[0].Ordinal, comments[1].Ordinal)
	}

	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("source: %v", err)
	}
	attached := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: ref, File: source, Description: "notes"})
	if attached.Outcome != contract.OutcomeOK {
		t.Fatalf("attach: %s %s", attached.Outcome, attached.Refusal)
	}
	h.reopen()
	payload := filepath.Join(h.card(ref).Dir, bench.AttachmentsDir, attached.Detail, bench.PayloadDir, "notes.txt")
	if !bench.Exists(payload) {
		t.Errorf("wanted the payload under its original name at %s", payload)
	}

	// Each collection instance counts from one on its own, so the card's
	// first attachment is ordinal 1 even though the card already carries two
	// comments, and the comment's own first attachment is ordinal 1 too.
	cardAttachment, err := bench.LoadAttachment(filepath.Join(h.card(ref).Dir, bench.AttachmentsDir, attached.Detail))
	if err != nil {
		t.Fatalf("load the attachment: %v", err)
	}
	if cardAttachment.Ordinal != 1 {
		t.Errorf("the card's first attachment carries ordinal %d, wanted 1", cardAttachment.Ordinal)
	}
	below := h.library.Attach(&Request{
		Verb:  "attach",
		Actor: "alka",
		Ref:   ref + "/" + bench.CommentsDir + "/1",
		File:  source,
	})
	if below.Outcome != contract.OutcomeOK {
		t.Fatalf("attach to a comment: %s %s", below.Outcome, below.Refusal)
	}
	h.reopen()
	onComment, err := bench.LoadAttachment(filepath.Join(comments[0].Dir, bench.AttachmentsDir, below.Detail))
	if err != nil {
		t.Fatalf("load the comment's attachment: %v", err)
	}
	if onComment.Ordinal != 1 {
		t.Errorf("the comment's first attachment carries ordinal %d, wanted 1", onComment.Ordinal)
	}

	replacement := filepath.Join(t.TempDir(), "revised.txt")
	if err := os.WriteFile(replacement, []byte("other bytes"), 0o644); err != nil {
		t.Fatalf("replacement: %v", err)
	}
	target := ref + "/attachments/" + attached.Detail
	if response := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: target, File: replacement, Replace: true}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("replace: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: target, Confirm: true}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("delete attachment: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	wanted := map[string]bool{
		contract.EventAttached:           false,
		contract.EventAttachmentReplaced: false,
		contract.EventAttachmentRemoved:  false,
	}
	for _, ev := range h.events(ref) {
		if _, ok := wanted[ev.Event]; !ok {
			continue
		}
		wanted[ev.Event] = true
		if ev.Attachment == "" || ev.Filename == "" {
			t.Errorf("%s: wanted the attachment id and its filename as of the event, got %q %q", ev.Event, ev.Attachment, ev.Filename)
		}
	}
	for name, seen := range wanted {
		if !seen {
			t.Errorf("journal: wanted a %s event, got none", name)
		}
	}
}

// TestArchiveAndDelete asserts that archiving takes a card out of the live
// set while its identifier still resolves, that a delete needs its
// confirmation, that a column cards occupy is refused, and that deleting a
// card another card's link names succeeds and leaves check the dangler.
func TestArchiveAndDelete(t *testing.T) {
	h := newHarness(t)
	occupant := h.add("occupant")
	linked := h.add("linked")

	// A card carrying a link to the card that is about to be deleted.
	card := h.card(occupant)
	card.FM.SetRaw("links", []string{"links:", "  - kind: relates", "    to: " + h.card(linked).ID})
	if err := card.Save(); err != nil {
		t.Fatalf("write link: %v", err)
	}
	h.reopen()

	if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: linked}); response.Refusal != contract.Unconfirmed {
		t.Fatalf("an unconfirmed delete: wanted %s, got %s %s", contract.Unconfirmed, response.Outcome, response.Refusal)
	}
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: intake}); response.Refusal != contract.Occupied {
		t.Fatalf("archiving an occupied column: wanted %s, got %s %s", contract.Occupied, response.Outcome, response.Refusal)
	}
	if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: intake, Confirm: true}); response.Refusal != contract.Occupied {
		t.Fatalf("deleting an occupied column: wanted %s, got %s %s", contract.Occupied, response.Outcome, response.Refusal)
	}

	// The archived card leaves the listings, the offers and the count.
	archivable := h.add("archivable")
	h.mustDo(&Request{Verb: Move, Card: archivable, Actor: "alka", Column: doing})
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: archivable}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	listing, err := h.library.List(&Request{Verb: "ls", Column: doing})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listing.Cards) != 0 {
		t.Errorf("an archived card should leave the listing, got %d cards", len(listing.Cards))
	}
	// Doing is limited to one card, so an admitted move proves the archived
	// card left the count of CORE-MOVE-5 as well.
	if response := h.do(&Request{Verb: Move, Card: occupant, Actor: "alka", Column: doing}); response.Outcome != contract.OutcomeOK {
		t.Errorf("an archived card should leave the capacity count, got %s", response.Refusal)
	}

	// Deleting a card another card's link names succeeds, and check reports
	// the dangling reference afterwards.
	deleted := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: linked, Confirm: true})
	if deleted.Outcome != contract.OutcomeOK {
		t.Fatalf("delete: %s %s", deleted.Outcome, deleted.Refusal)
	}
	h.reopen()
	report, err := h.library.Check(&Request{Verb: "check", Actor: "alka"})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	findings := report.Findings
	found := false
	for _, finding := range findings {
		if finding.Key == bench.FindingDanglingLink {
			found = true
		}
	}
	if !found {
		t.Errorf("wanted a dangling-link finding after the delete, got %+v", findings)
	}
}

// TestArchiveRemovesTheColumnFromTheDefinition asserts AC-1: archiving an
// unoccupied column that is not the sole remaining one drops the identifier
// from workbench.md's columns list in the same run, and every command that
// opens the bench afterwards succeeds and no longer lists it.
func TestArchiveRemovesTheColumnFromTheDefinition(t *testing.T) {
	h := newHarness(t)
	before := len(h.library.Bench.Columns)
	response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: aftercare})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if got := len(h.library.Bench.Columns); got != before-1 {
		t.Fatalf("wanted %d columns after the archive, got %d", before-1, got)
	}
	if h.library.Bench.Column(aftercare) != nil {
		t.Error("the archived column is still declared")
	}
	if findings := h.check(); len(findings) != 0 {
		t.Errorf("a workbench with no dangling entry should check clean, got %+v", findings)
	}
}

// TestDeleteRemovesTheColumnFromTheDefinition asserts AC-2: deleting an
// unoccupied column behaves identically to archiving it for the purposes of
// AC-1.
func TestDeleteRemovesTheColumnFromTheDefinition(t *testing.T) {
	h := newHarness(t)
	before := len(h.library.Bench.Columns)
	response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: review, Confirm: true})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("delete: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if got := len(h.library.Bench.Columns); got != before-1 {
		t.Fatalf("wanted %d columns after the delete, got %d", before-1, got)
	}
	if h.library.Bench.Column(review) != nil {
		t.Error("the deleted column is still declared")
	}
	if findings := h.check(); len(findings) != 0 {
		t.Errorf("a workbench with no dangling entry should check clean, got %+v", findings)
	}
}

// TestRetiringTheLastColumnIsRefused asserts AC-3: archiving or deleting the
// sole remaining column is refused with dinah.last-column, both as the third
// precondition row of archive and the fourth of delete, and the workbench
// keeps opening and working normally afterwards.
func TestRetiringTheLastColumnIsRefused(t *testing.T) {
	if got := Checks("archive"); len(got) != 3 || got[2].Refusal != contract.LastColumn {
		t.Fatalf("archive's preconditions: wanted dinah.last-column third, got %+v", got)
	}
	if got := Checks("delete"); len(got) != 4 || got[3].Refusal != contract.LastColumn {
		t.Fatalf("delete's preconditions: wanted dinah.last-column fourth, got %+v", got)
	}

	h := newHarness(t)
	for _, id := range []string{doing, review, finished, aftercare, closed} {
		response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: id})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("archive %s: %s %s", id, response.Outcome, response.Refusal)
		}
		h.reopen()
	}
	if len(h.library.Bench.Columns) != 1 {
		t.Fatalf("wanted one column left, got %d", len(h.library.Bench.Columns))
	}
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: intake}); response.Refusal != contract.LastColumn {
		t.Fatalf("archiving the last column: wanted %s, got %s %s", contract.LastColumn, response.Outcome, response.Refusal)
	} else if response.Detail != "intake" {
		// Named by its slug, not by its raw identifier: a caller who typed
		// "intake" is told about "intake", never about a12300000001.
		t.Fatalf("last-column refusal: wanted the slug %q, got %q", "intake", response.Detail)
	}
	h.reopen()
	if h.library.Bench.Column(intake) == nil {
		t.Fatal("the refused archive removed the last column anyway")
	}
	if findings := h.check(); len(findings) != 0 {
		t.Errorf("a workbench holding its last column should check clean, got %+v", findings)
	}
	if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: intake, Confirm: true}); response.Refusal != contract.LastColumn {
		t.Fatalf("deleting the last column: wanted %s, got %s %s", contract.LastColumn, response.Outcome, response.Refusal)
	} else if response.Detail != "intake" {
		t.Fatalf("last-column refusal: wanted the slug %q, got %q", "intake", response.Detail)
	}
}

// TestInterchangeRoundTrip asserts CORE-JSON-1, CORE-JSON-2, CORE-JSON-3,
// CORE-JSON-4, CORE-JSON-5, CORE-JSON-7 and CORE-CARD-9:
// an object carrying an unknown member survives export, import and export,
// and so does an unrecognised field on a card.
func TestInterchangeRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.library.Bench.FM.Set("acme.department", `"catering"`)
	h.library.Bench.Standing = "Review every card before claiming it."
	if err := h.library.Bench.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	h.reopen()

	first, err := h.library.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	object := map[string]any{}
	if err := json.Unmarshal(first, &object); err != nil {
		t.Fatalf("the interchange form should parse as one object: %v", err)
	}
	for _, member := range []string{"profile", "title", "columns"} {
		if _, ok := object[member]; !ok {
			t.Errorf("CORE-JSON-3: the written object carries no %s", member)
		}
	}
	if object["acme.department"] != "catering" {
		t.Errorf("CORE-JSON-7: the unrecognised member did not survive the export, got %v", object["acme.department"])
	}
	if object["instructions"] != h.library.Bench.Standing {
		t.Errorf("the exported object should carry the standing instruction, got %v", object["instructions"])
	}

	definition, err := bench.ReadDefinition(first)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(definition.Columns) != len(h.library.Bench.Columns) {
		t.Fatalf("wanted %d columns, got %d", len(h.library.Bench.Columns), len(definition.Columns))
	}
	for i, element := range definition.Columns {
		var id string
		if err := json.Unmarshal(element["id"], &id); err != nil {
			t.Fatalf("column id: %v", err)
		}
		if id != h.library.Bench.Columns[i].ID {
			t.Errorf("CORE-JSON-4: position %d wanted %s, got %s", i, h.library.Bench.Columns[i].ID, id)
		}
	}

	second := containedPath(filepath.Join(t.TempDir(), "again"))
	if err := bench.Instantiate(second, "again", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	reopened, err := bench.Open(second)
	if err != nil {
		t.Fatalf("open the instantiated workbench: %v", err)
	}
	third, err := reopened.Export()
	if err != nil {
		t.Fatalf("export again: %v", err)
	}
	if string(third) != string(first) {
		t.Errorf("a read and a write back changed the interchange form:\nfirst:\n%s\nthird:\n%s", first, third)
	}

	// A field the profile does not define survives on a card too.
	ref := h.ready("extended")
	card := h.card(ref)
	card.FM.Set("acme.table", "seven")
	if err := card.Save(); err != nil {
		t.Fatalf("card save: %v", err)
	}
	h.reopen()
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	if got := h.card(ref).FM.Value("acme.table"); got != "seven" {
		t.Errorf("CORE-CARD-9: the unrecognised field did not survive a write, got %q", got)
	}
}

// TestExtractReproducesTheDefinition asserts that extract copies the bench's
// definition out with its identifiers kept and leaves the work behind, and
// that a bench instantiated from the result has the same definition.
//
// The comparison is over the parsed definition rather than over bytes,
// because canonical frontmatter key order is not settled yet. The byte-level
// form of the property waits on the card that settles it.
func TestExtractReproducesTheDefinition(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("work that stays behind")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})

	target := filepath.Join(t.TempDir(), "template")
	if err := h.library.Extract(target); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if bench.Exists(filepath.Join(target, bench.CardsDir)) {
		t.Error("extract carried the cards out with the definition")
	}
	if bench.Exists(filepath.Join(target, bench.JournalName)) {
		t.Error("extract carried a journal out with the definition")
	}

	extracted, err := bench.OpenUncontained(target)
	if err != nil {
		t.Fatalf("open the extracted definition: %v", err)
	}
	if len(extracted.Columns) != len(h.library.Bench.Columns) {
		t.Fatalf("wanted %d columns, got %d", len(h.library.Bench.Columns), len(extracted.Columns))
	}
	for i, column := range extracted.Columns {
		original := h.library.Bench.Columns[i]
		if column.ID != original.ID || column.Title != original.Title || column.Kind != original.Kind {
			t.Errorf("column %d: wanted %s/%s/%s, got %s/%s/%s", i,
				original.ID, original.Title, original.Kind, column.ID, column.Title, column.Kind)
		}
		if column.Capacity != original.Capacity || column.OperatorOwned != original.OperatorOwned {
			t.Errorf("column %d: capacity or operator flag differs", i)
		}
	}
}

// TestInitFromATemplateCarriesTheStandingInstruction asserts the literal
// path a person hits with a template: extracting a workbench that carries a
// standing instruction, then instantiating a new workbench from that
// directory with Init, produces a workbench whose own standing instruction
// equals the source's.
func TestInitFromATemplateCarriesTheStandingInstruction(t *testing.T) {
	h := newHarness(t)
	h.library.Bench.Standing = "Claim before you start, and never leave a card idle."
	if err := h.library.Bench.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	h.reopen()

	template := filepath.Join(t.TempDir(), "template")
	if err := h.library.Extract(template); err != nil {
		t.Fatalf("extract: %v", err)
	}

	created := filepath.Join(t.TempDir(), "new-workbench")
	written, err := Init(created, "again", "alka", template, "", "")
	if err != nil {
		t.Fatalf("init --from: %v", err)
	}
	opened, err := bench.Open(written)
	if err != nil {
		t.Fatalf("open the new workbench: %v", err)
	}
	if opened.Standing != h.library.Bench.Standing {
		t.Errorf("wanted the standing instruction %q, got %q", h.library.Bench.Standing, opened.Standing)
	}
}

// TestActorLadderAndConfig asserts the actor ladder, the config surface and
// the preservation of a key the tool does not know.
func TestActorLadderAndConfig(t *testing.T) {
	h := newHarness(t)
	cfg := bench.LoadConfig(h.home)
	if err := cfg.Set("actor", "from-config"); err != nil {
		t.Fatalf("set actor: %v", err)
	}
	if err := cfg.Set("lang", "hi"); err != nil {
		t.Fatalf("set lang: %v", err)
	}
	if err := cfg.Set("colour", "green"); err == nil {
		t.Error("an unknown key should be refused")
	} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnknownKey {
		t.Errorf("wanted %s, got %v", contract.UnknownKey, err)
	}

	// A key the tool does not recognise, already in the file, survives.
	path := filepath.Join(h.home, bench.UserBaseName, bench.ConfigName)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(text, "---\n", "---\nacme.theme: dark\n", 1)), 0o644); err != nil {
		t.Fatalf("hand edit: %v", err)
	}
	reloaded := bench.LoadConfig(h.home)
	if err := reloaded.Set("lang", "en"); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	after := bench.LoadConfig(h.home)
	if after.Get("acme.theme") != "dark" {
		t.Errorf("an unrecognised setting did not survive a write, got %q", after.Get("acme.theme"))
	}
	if after.Get("actor") != "from-config" {
		t.Errorf("actor: wanted from-config, got %q", after.Get("actor"))
	}

	// The flag wins over every layer below it.
	resolved, err := bench.ResolveActor("from-flag", after)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != "from-flag" {
		t.Errorf("wanted the flag to win, got %q", resolved)
	}
	// With no layer carrying one, the ladder refuses rather than inventing.
	empty := bench.LoadConfig(filepath.Join(t.TempDir(), "nothing"))
	t.Setenv("DINAH_ACTOR", "")
	if _, err := bench.ResolveActor("", empty); err == nil {
		t.Error("an unresolvable actor should be refused")
	}
}

// TestVersionCarriesTheConformanceClaim asserts CORE-VER-1 and CORE-VER-2:
// the claim carries a major and a minor number and no maturity channel, and
// the coverage report names every shipped catalog. It also asserts that the
// release number a release build stamps into the binary reaches the report.
func TestVersionCarriesTheConformanceClaim(t *testing.T) {
	release := Version(true)
	if release.Profile != bench.ProfileVersion {
		t.Errorf("conformance claim: wanted %s, got %s", bench.ProfileVersion, release.Profile)
	}
	for _, channel := range []string{"dev", "beta", "stable"} {
		if strings.Contains(release.Profile, channel) {
			t.Errorf("CORE-VER-2: the claim names the channel %s", channel)
		}
	}
	if release.Tool == release.Profile {
		t.Error("the tool's release number and the conformance claim must not be conflated")
	}
	if release.Format != bench.StorageFormat {
		t.Errorf("storage format: wanted %d, got %d", bench.StorageFormat, release.Format)
	}
	// The roster of which catalogs ship complete lives once, as msg.Complete
	// and msg.Skeleton, so this test reads the same declaration
	// TestEveryDeclaredLanguageShips in internal/msg reads rather than
	// carrying its own copy.
	isComplete := map[string]bool{}
	for _, tag := range msg.Complete {
		isComplete[tag] = true
	}
	// The two rosters are read separately rather than as one another's
	// negation. A catalog can be on neither: dinah-287 took Hindi and German
	// off Complete while both go on carrying hundreds of real translations,
	// so "not complete" stopped meaning "generated skeleton". Each roster's
	// claim is asserted against the tags that roster actually names, which is
	// the shape TestEveryDeclaredLanguageShips already reads them in.
	isSkeleton := map[string]bool{}
	for _, tag := range msg.Skeleton {
		isSkeleton[tag] = true
	}
	wanted := map[string]bool{}
	for _, tag := range append(append([]string{}, msg.Complete...), msg.Skeleton...) {
		wanted[tag] = true
	}
	for _, coverage := range release.Catalogs {
		delete(wanted, coverage.Tag)
		if coverage.Present != coverage.Total {
			t.Errorf("%s: wanted every key present, got %d of %d", coverage.Tag, coverage.Present, coverage.Total)
		}
		complete := isComplete[coverage.Tag]
		if complete && coverage.Translated != coverage.Total {
			t.Errorf("%s ships complete, got %d of %d translated", coverage.Tag, coverage.Translated, coverage.Total)
		}
		if isSkeleton[coverage.Tag] && coverage.Translated != 0 {
			t.Errorf("%s ships as a skeleton, got %d translated", coverage.Tag, coverage.Translated)
		}
	}
	if len(wanted) > 0 {
		t.Errorf("catalogs missing from the report: %v", wanted)
	}
	// A release build overwrites the release number with the tag it was built
	// from, through -ldflags -X, which reaches a variable and not a constant.
	// The assignment below is the assertion: it does not compile if anyone
	// turns ToolRelease back into a constant, and the comparison catches a
	// report that stops reading it.
	original := ToolRelease
	t.Cleanup(func() { ToolRelease = original })
	ToolRelease = "v0.1.0-dev.42"
	stamped := Version(false)
	if stamped.Tool != "v0.1.0-dev.42" {
		t.Errorf("a stamped release number should reach the report, got %s", stamped.Tool)
	}
}

// TestUnsupportedVersionsAreRefusedLoudly asserts CORE-BENCH-5 and the
// storage format's own refusal, each naming the version it wanted, together
// with CORE-OUT-5, which supplies malformed where no more particular refusal
// name applies.
func TestUnsupportedVersionsAreRefusedLoudly(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		value   string
		refusal string
	}{
		{name: "a profile major this binary does not implement", key: "profile", value: "dinah-core/9.0", refusal: contract.UnsupportedVer},
		{name: "a storage format newer than the binary knows", key: "format", value: "99", refusal: contract.UnsupportedVer},
		{name: "no profile declared at all", key: "profile", value: "", refusal: contract.Malformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			if c.value == "" {
				h.library.Bench.FM.Delete(c.key)
			} else {
				h.library.Bench.FM.Set(c.key, c.value)
			}
			if err := bench.WriteText(filepath.Join(h.root, bench.WorkbenchAnchor), h.library.Bench.FM.Render(h.library.Bench.Standing)); err != nil {
				t.Fatalf("write: %v", err)
			}
			_, err := bench.Open(h.root)
			refusal, ok := err.(*contract.Refusal)
			if !ok {
				t.Fatalf("wanted a refusal, got %v", err)
			}
			if refusal.Name != c.refusal {
				t.Errorf("wanted %s, got %s", c.refusal, refusal.Name)
			}
			if c.refusal == contract.UnsupportedVer && !strings.Contains(refusal.Detail, strings.TrimPrefix(c.value, "dinah-core/")) {
				t.Errorf("the refusal should name the version wanted, got %q", refusal.Detail)
			}
		})
	}
}

// TestWhoamiAnswersTheOperatorQuestion asserts CORE-OWNER-2 through the read
// surface, and that a bench designating no operator refuses.
func TestWhoamiAnswersTheOperatorQuestion(t *testing.T) {
	h := newHarness(t)
	identity, err := h.library.Whoami(&Request{Verb: "whoami", Actor: "alka"})
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if !identity.IsOperator {
		t.Error("alka is the operator of the fixture workbench")
	}
	other, err := h.library.Whoami(&Request{Verb: "whoami", Actor: "bob"})
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if other.IsOperator {
		t.Error("bob is not the operator of the fixture workbench")
	}
	if _, err := h.library.Whoami(&Request{Verb: "whoami"}); err == nil {
		t.Error("with no actor resolvable, whoami should refuse rather than invent one")
	}
	status, err := h.library.Status(&Request{Verb: "status", Actor: "alka"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.IsOperator || status.Operator != "alka" {
		t.Error("status carries the same two facts as whoami")
	}
}

// TestGuidesAreServedAndNeverSeeded asserts that the embedded guides list and
// print, that an unknown topic is refused, and that no guide text is written
// into a bench by init or by any verb.
func TestGuidesAreServedAndNeverSeeded(t *testing.T) {
	topics := guide.Topics()
	if len(topics) == 0 {
		t.Fatal("wanted at least one embedded guide")
	}
	first, err := guide.Text(topics[0])
	if err != nil {
		t.Fatalf("guide: %v", err)
	}
	if first == "" {
		t.Error("an embedded guide should carry text")
	}
	if _, err := guide.Text("no-such-topic"); err == nil {
		t.Error("an unknown topic should be refused")
	} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnknownGuide {
		t.Errorf("wanted %s, got %v", contract.UnknownGuide, err)
	}

	h := newHarness(t)
	ref := h.ready("ordinary work")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: doing})
	needle := strings.SplitN(strings.TrimPrefix(first, "# "), "\n", 2)[0]
	err = filepath.WalkDir(h.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		text, readErr := bench.ReadText(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(text, needle) {
			t.Errorf("guide text was seeded into the workbench at %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// TestEveryRefusalNameIsLegal asserts CORE-OUT-3 and DOC-LAYER-1: every
// refusal name this binary can emit is one the profile declares or one
// carrying the dinah layer's prefix, and the two sets do not overlap.
func TestEveryRefusalNameIsLegal(t *testing.T) {
	declared := map[string]bool{}
	for _, name := range contract.Declared {
		declared[name] = true
		if strings.Contains(name, ".") {
			t.Errorf("DOC-LAYER-1: the profile's own name %s carries a full stop", name)
		}
	}
	if len(contract.Declared) != 16 {
		t.Errorf("wanted the profile's sixteen declared names, got %d", len(contract.Declared))
	}
	for _, name := range contract.Introduced {
		if declared[name] {
			t.Errorf("%s is both declared by the profile and introduced by this layer", name)
		}
		if !strings.HasPrefix(name, contract.LayerPrefix) {
			t.Errorf("%s is not one of the profile's sixteen and carries no layer prefix", name)
		}
	}
	// Every name reachable from the code is one of the two sets, which is
	// what the check enumerating them is for.
	reachable := append(append([]string{}, contract.Declared...), contract.Introduced...)
	for _, name := range reachable {
		if !contract.NameIsLegal(name) {
			t.Errorf("CORE-OUT-3 admits neither form of %s", name)
		}
	}
	for _, checks := range [][]Check{
		Checks(Claim), Checks(Move), Checks(Release), Checks(Block), Checks(Unblock),
		Checks("add"), Checks("comment"), Checks("attach"), Checks("archive"),
		Checks("delete"), Checks("guide"), Checks("config"), Checks("init"),
		Checks("extract"), Checks("edit"), Checks("path"), Checks("show"),
		Checks("log"), Checks("ls"), Checks("next"), Checks("instructions"),
		Checks("whoami"),
	} {
		for _, check := range checks {
			if !contract.NameIsLegal(check.Refusal) {
				t.Errorf("a declared check reports the illegal refusal name %s", check.Refusal)
			}
		}
	}
}

// TestOutcomesAndExitCodes asserts CORE-OUT-1 and the exit code each outcome
// carries.
func TestOutcomesAndExitCodes(t *testing.T) {
	wanted := map[string]int{
		contract.OutcomeOK:          0,
		contract.OutcomeRefused:     2,
		contract.OutcomeStale:       3,
		contract.OutcomeUnreachable: 4,
	}
	for outcome, code := range wanted {
		if got := contract.ExitCode(outcome); got != code {
			t.Errorf("%s: wanted exit code %d, got %d", outcome, code, got)
		}
	}
	h := newHarness(t)
	ref := h.add("outcomes")
	for _, req := range []*Request{
		{Verb: Claim, Card: ref, Actor: "alka"},
		{Verb: Claim, Card: ref, Actor: "bob"},
		{Verb: Claim, Card: "fx-99", Actor: "alka"},
	} {
		response := h.do(req)
		if _, ok := wanted[response.Outcome]; !ok {
			t.Errorf("CORE-OUT-1: %q is not one of the four outcome tokens", response.Outcome)
		}
		if response.Outcome == contract.OutcomeRefused && response.Refusal == "" {
			t.Error("CORE-OUT-2: a refusal carries exactly one refusal name")
		}
		if response.Outcome == contract.OutcomeOK && response.Refusal != "" {
			t.Errorf("an ok outcome carries the refusal name %s", response.Refusal)
		}
	}
}

// TestStalePrefixWarnsRatherThanRefuses asserts that within a bench a card
// reference resolves on its number, and a prefix naming no current slug is
// accepted with a warning.
func TestStalePrefixWarnsRatherThanRefuses(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("renamed workbench")
	number := strings.TrimPrefix(ref, "fx-")
	response := h.do(&Request{Verb: Claim, Card: "yokoten-" + number, Actor: "alka"})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("wanted the number to resolve the card, got %s %s", response.Outcome, response.Refusal)
	}
	if response.Warning == "" || response.WarningDetail != "yokoten" {
		t.Errorf("wanted a warning naming the stale prefix, got %q %q", response.Warning, response.WarningDetail)
	}
}

// journalLength counts the lines a journal carries, which is what proves a
// refused write left no event behind.
func journalLength(t *testing.T, path string) int {
	t.Helper()
	events, _, err := bench.ReadJournal(path)
	if err != nil {
		t.Fatalf("journal %s: %v", path, err)
	}
	return len(events)
}

// TestAttachTakesTheEnclosingEntitysLock asserts that attach acquires the lock
// of the nearest enclosing journal-bearing entity for every reference kind it
// accepts, which is the card's own lock below a card and the bench's lock
// everywhere else, and that a held lock leaves nothing written: no attachment
// entity in the target's collection and no line on the journal the event would
// have gone to.
//
// Arming: removing the acquisition from Attach admits every row, and each one
// then writes both the entity and the event while another process holds the
// lock that was supposed to stop it.
func TestAttachTakesTheEnclosingEntitysLock(t *testing.T) {
	h := newHarness(t)
	ref := h.add("annotated")
	if response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: ref, Text: "a thought"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("source: %v", err)
	}
	if response := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: ref, File: source}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("attach: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	card := h.card(ref)
	comments := bench.ListIDs(filepath.Join(card.Dir, bench.CommentsDir))
	attachments := bench.ListIDs(filepath.Join(card.Dir, bench.AttachmentsDir))
	if len(comments) != 1 || len(attachments) != 1 {
		t.Fatalf("wanted one comment and one attachment to aim at, got %d and %d", len(comments), len(attachments))
	}
	benchJournal := h.library.Bench.JournalPath()
	cases := []struct {
		kind    string
		ref     string
		lockDir string
		owner   string
		journal string
	}{
		{kind: "workbench", ref: "", lockDir: h.root, owner: h.root, journal: benchJournal},
		{
			kind:    "column",
			ref:     intake,
			lockDir: h.root,
			owner:   filepath.Join(h.root, bench.ColumnsDir, intake),
			journal: benchJournal,
		},
		{kind: "card", ref: ref, lockDir: card.Dir, owner: card.Dir, journal: card.JournalPath()},
		{
			kind:    "comment",
			ref:     ref + "/comments/1",
			lockDir: card.Dir,
			owner:   filepath.Join(card.Dir, bench.CommentsDir, comments[0]),
			journal: card.JournalPath(),
		},
		{
			kind:    "attachment",
			ref:     ref + "/attachments/1",
			lockDir: card.Dir,
			owner:   filepath.Join(card.Dir, bench.AttachmentsDir, attachments[0]),
			journal: card.JournalPath(),
		},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			collection := filepath.Join(c.owner, bench.AttachmentsDir)
			entities := len(bench.ListIDs(collection))
			lines := journalLength(t, c.journal)
			held := h.hold(c.lockDir, "someone")
			response := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: c.ref, File: source})
			held.Release()
			if response.Refusal != contract.Locked {
				t.Fatalf("wanted %s, got %s %s", contract.Locked, response.Outcome, response.Refusal)
			}
			if response.Detail != "someone" {
				t.Errorf("the refusal should name the holder, got %q", response.Detail)
			}
			if got := len(bench.ListIDs(collection)); got != entities {
				t.Errorf("a refused attach left %d entities behind, wanted %d", got, entities)
			}
			if got := journalLength(t, c.journal); got != lines {
				t.Errorf("a refused attach appended to the journal: %d lines, wanted %d", got, lines)
			}
		})
	}

	// The references above are the only spellings that reach a card and the
	// entities below one, so the pairing they assert is the whole of it. A
	// second spelling composed down from the workbench carries no card, and
	// an attach through one wrote the card's own event to the bench journal
	// and took the bench lock, which left two spellings of one comment
	// excluding neither each other nor a concurrent writer.
	t.Run("a card is not reached down from the workbench", func(t *testing.T) {
		benchLines := journalLength(t, benchJournal)
		cardLines := journalLength(t, card.JournalPath())
		for _, spelling := range []string{
			h.library.Bench.Slug + "/cards/1",
			"workbench/cards/1/comments/1",
		} {
			response := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: spelling, File: source})
			if response.Outcome == contract.OutcomeOK {
				t.Errorf("attach %s succeeded, and no walk draws that reference", spelling)
			}
		}
		if got := journalLength(t, benchJournal); got != benchLines {
			t.Errorf("the workbench journal grew to %d lines over a card's own event, wanted %d", got, benchLines)
		}
		if got := journalLength(t, card.JournalPath()); got != cardLines {
			t.Errorf("the card journal grew to %d lines over a refused attach, wanted %d", got, cardLines)
		}
	})
}

// TestAStructuralActIsRefusedByAnyOfItsThreeLocks asserts that archiving and
// deleting a card are each refused when any one of the bench's lock, the
// card's sibling and the card's own lock is already held, that the refusal
// names the holder recorded in whichever lock was found, and that the unwind
// gives back every lock the act did take.
//
// The last of those is what the assertion on the bench's locks proves: after
// the refusal the only lock standing anywhere is the one the test planted. The
// card is left intact with no archived line on its journal and no deleted line
// on the bench's, which is what puts the refusal before the record.
//
// Arming: leaving the bench lock unreleased on the sibling refusal leaves two
// locks standing and the row fails.
func TestAStructuralActIsRefusedByAnyOfItsThreeLocks(t *testing.T) {
	acts := []struct {
		name string
		run  func(*harness, string) *Response
	}{
		{name: "archive", run: func(h *harness, ref string) *Response {
			return h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
		}},
		{name: "delete", run: func(h *harness, ref string) *Response {
			return h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: ref, Confirm: true})
		}},
	}
	locks := []struct {
		name string
		path func(root, dir string) string
	}{
		{name: "the workbench lock", path: func(root, dir string) string {
			return filepath.Join(root, bench.LockName)
		}},
		{name: "the sibling", path: func(root, dir string) string {
			return bench.SiblingPath(dir)
		}},
		{name: "the card's own lock", path: func(root, dir string) string {
			return filepath.Join(dir, bench.LockName)
		}},
	}
	for _, act := range acts {
		for _, lock := range locks {
			t.Run(act.name+" against "+lock.name, func(t *testing.T) {
				h := newHarness(t)
				ref := h.add("contested")
				card := h.card(ref)
				planted := lock.path(h.root, card.Dir)
				h.plant(planted, bench.LockRecord{Actor: "someone", PID: 4242, TS: bench.Stamp(h.clock)})

				response := act.run(h, ref)
				if response.Refusal != contract.Locked {
					t.Fatalf("wanted %s, got %s %s", contract.Locked, response.Outcome, response.Refusal)
				}
				if response.Detail != "someone" {
					t.Errorf("the refusal should name the holder, got %q", response.Detail)
				}
				relative, err := filepath.Rel(h.root, planted)
				if err != nil {
					t.Fatalf("relative: %v", err)
				}
				wanted := filepath.ToSlash(relative)
				if got := strings.Join(h.locks(), ", "); got != wanted {
					t.Errorf("locks standing after the refusal: %q, wanted only %q", got, wanted)
				}
				h.reopen()
				if !bench.Exists(filepath.Join(card.Dir, bench.CardAnchor)) {
					t.Error("the refused act moved or removed the card anyway")
				}
				for _, ev := range h.events(ref) {
					if ev.Event == contract.EventArchived {
						t.Error("the refusal happened after the card's journal was written")
					}
				}
				for _, ev := range h.benchEvents() {
					if ev.Event == contract.EventDeleted {
						t.Error("the refusal happened after the workbench journal was written")
					}
				}
			})
		}
	}
}

// TestASiblingLockIsInvisibleToEveryReadPath asserts that a sibling standing
// beside a live card, and a second one naming an identifier no card holds,
// change no listing, no offer, no capacity count, no export and no identifier
// answer, because every read walks a collection's identifiers and a
// seventeen-character file is not one of them.
//
// The second planted file is the one that catches a read built on a glob
// rather than on ListIDs. The comparison sets the interrupted-act findings
// aside, since reporting a standing sibling is what the checker gained here,
// and the bench is otherwise clean so its card walk has nothing else to say.
func TestASiblingLockIsInvisibleToEveryReadPath(t *testing.T) {
	h := newHarness(t)
	ref := h.add("visible")
	h.add("second")
	card := h.card(ref)

	read := func() string {
		t.Helper()
		h.reopen()
		listing, err := h.library.List(&Request{Verb: "ls"})
		if err != nil {
			t.Fatalf("ls: %v", err)
		}
		offers, err := h.library.Next(&Request{Verb: "next", Column: intake})
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		status, err := h.library.Status(&Request{Verb: "status", Actor: "alka"})
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		exported, err := h.library.Export()
		if err != nil {
			t.Fatalf("export: %v", err)
		}
		var clean []bench.Finding
		for _, f := range h.check() {
			if f.Key == bench.FindingInterruptedAct || f.Key == bench.FindingEntityAtBothPaths {
				continue
			}
			clean = append(clean, f)
		}
		identifiers := []bool{
			h.library.Bench.HasIdentifier(card.ID),
			h.library.Bench.HasIdentifier("ffffffffffff"),
		}
		reading := []any{listing, offers, status.Columns, string(exported), clean, identifiers}
		encoded, err := json.Marshal(reading)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(encoded)
	}

	before := read()
	record := bench.LockRecord{Actor: "someone", PID: 4242, TS: bench.Stamp(h.clock), Op: bench.OpArchive}
	h.plant(bench.SiblingPath(card.Dir), record)
	h.plant(filepath.Join(h.library.Bench.CardsRoot(), "ffffffffffff"+bench.SiblingSuffix), record)
	if after := read(); after != before {
		t.Errorf("a sibling changed what a read reports:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestDeletingACardRecordsItOnTheBenchJournal asserts that deleting a card
// leaves exactly one new line on the bench's own journal, that the line is a
// deleted event carrying the identifier and the title as of the event, and
// that both names the closed event set gained here render for a reader in
// English and Hindi, two of the language ruling's now three complete
// catalogs.
//
// The bench journal is where the record has to go, since the deletion destroys
// the journal inside the card.
func TestDeletingACardRecordsItOnTheBenchJournal(t *testing.T) {
	h := newHarness(t)
	ref := h.add("the card with a name")
	id := h.card(ref).ID
	before := len(h.benchEvents())

	response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: ref, Confirm: true})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("delete: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	events := h.benchEvents()
	if len(events) != before+1 {
		t.Fatalf("wanted one new line on the workbench journal, got %d", len(events)-before)
	}
	ev := events[len(events)-1]
	if ev.Event != contract.EventDeleted {
		t.Errorf("event: wanted %s, got %s", contract.EventDeleted, ev.Event)
	}
	if ev.Note != id {
		t.Errorf("the event should carry the identifier, got %q", ev.Note)
	}
	if ev.Title != "the card with a name" {
		t.Errorf("the event should carry the title as of the event, got %q", ev.Title)
	}
	for _, tag := range []string{"en", "hi"} {
		renderer := msg.For(tag)
		for _, name := range []string{contract.EventDeleted, contract.EventRestored} {
			if !renderer.Has("token." + name) {
				t.Errorf("%s renders no token for the event %s", tag, name)
			}
		}
	}
}

// TestAnOrdinaryLockLineIsUnchangedAndNoLockTravels asserts that the lock a
// contract verb takes carries the three members it has always carried and
// neither of the two a sibling adds, so a hand-written lock line stays valid,
// and that no lock of either kind reaches interchange or a template.
func TestAnOrdinaryLockLineIsUnchangedAndNoLockTravels(t *testing.T) {
	h := newHarness(t)
	ref := h.add("locked")
	card := h.card(ref)

	held := h.hold(card.Dir, "alka")
	text, err := bench.ReadText(filepath.Join(card.Dir, bench.LockName))
	if err != nil {
		t.Fatalf("read the lock: %v", err)
	}
	held.Release()
	members := map[string]any{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &members); err != nil {
		t.Fatalf("the lock line should be one JSON object: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("an ordinary lock line carries actor, pid and ts alone, got %v", members)
	}
	for _, member := range []string{"actor", "pid", "ts"} {
		if _, ok := members[member]; !ok {
			t.Errorf("the lock line carries no %s", member)
		}
	}

	record := bench.LockRecord{Actor: "someone", PID: 4242, TS: bench.Stamp(h.clock), Op: bench.OpArchive}
	h.plant(bench.SiblingPath(card.Dir), record)
	h.plant(filepath.Join(card.Dir, bench.LockName), record)
	exported, err := h.library.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if strings.Contains(string(exported), "someone") || strings.Contains(string(exported), "\"pid\"") {
		t.Errorf("the interchange form carries coordination-plane state:\n%s", exported)
	}
	target := filepath.Join(t.TempDir(), "template")
	if err := h.library.Extract(target); err != nil {
		t.Fatalf("extract: %v", err)
	}
	err = filepath.WalkDir(target, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name := entry.Name()
		if name == bench.LockName || strings.HasSuffix(name, bench.SiblingSuffix) {
			t.Errorf("a lock travelled into the template at %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// hashDirectory sums a directory's whole contents by path and by bytes, which
// is how a test says a rolled-back act left a card byte-identical to what it
// was rather than merely present.
func hashDirectory(t *testing.T, dir string) string {
	t.Helper()
	sum := sha256.New()
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		sum.Write([]byte(filepath.ToSlash(relative)))
		sum.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("hash %s: %v", dir, err)
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// archivedDir is where a card's directory sits once the archive has landed.
func (h *harness) archivedDir(id string) string {
	return filepath.Join(h.library.Bench.ArchivedCardsRoot(), id)
}

// TestAnAttachAndAnArchiveOnOneCardSerialize asserts that an attach and an
// archive of the same card cannot both proceed, from either order, and that
// whichever loses is refused with nothing half-written.
//
// The second order is the one that matters most, because an attach admitted
// inside the window would create its entity directory inside a card that then
// moves out from under it, leaving an attachment in one half of the collection
// and its card in the other.
func TestAnAttachAndAnArchiveOnOneCardSerialize(t *testing.T) {
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("source: %v", err)
	}

	t.Run("the attach got there first", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("contested")
		other := h.second()
		var blocked *Response
		h.library.Interleave = func() {
			blocked = other.Archive(&Request{Verb: "archive", Actor: "bob", Ref: ref})
		}
		attached := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: ref, File: source})
		h.library.Interleave = nil
		if attached.Outcome != contract.OutcomeOK {
			t.Fatalf("the attach: wanted ok, got %s %s", attached.Outcome, attached.Refusal)
		}
		if blocked == nil {
			t.Fatal("the interleaved archive never ran, so this test proves nothing")
		}
		if blocked.Refusal != contract.Locked {
			t.Errorf("the interleaved archive: wanted %s, got %s %s", contract.Locked, blocked.Outcome, blocked.Refusal)
		}
		h.reopen()
		card := h.card(ref)
		payload := filepath.Join(card.Dir, bench.AttachmentsDir, attached.Detail, bench.PayloadDir, "notes.txt")
		if !bench.Exists(payload) {
			t.Errorf("the attachment that won should be complete, no payload at %s", payload)
		}
		attachedEvents := 0
		for _, ev := range h.events(ref) {
			if ev.Event == contract.EventAttached {
				attachedEvents++
			}
		}
		if attachedEvents != 1 {
			t.Errorf("wanted one attached event, got %d", attachedEvents)
		}
	})

	t.Run("the archive got there first", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("contested")
		id := h.card(ref).ID
		other := h.second()
		var blocked *Response
		h.inWindow(func() {
			blocked = other.Attach(&Request{Verb: "attach", Actor: "bob", Ref: ref, File: source})
		})
		archived := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
		h.library.Bench.Hooks = nil
		if archived.Outcome != contract.OutcomeOK {
			t.Fatalf("the archive: wanted ok, got %s %s", archived.Outcome, archived.Refusal)
		}
		if blocked == nil {
			t.Fatal("the interleaved attach never ran, so this test proves nothing")
		}
		if blocked.Refusal != contract.Locked {
			t.Errorf("the interleaved attach: wanted %s, got %s %s", contract.Locked, blocked.Outcome, blocked.Refusal)
		}
		h.reopen()
		live := filepath.Join(h.library.Bench.CardsRoot(), id, bench.AttachmentsDir)
		archivedHalf := filepath.Join(h.archivedDir(id), bench.AttachmentsDir)
		for _, collection := range []string{live, archivedHalf} {
			if got := len(bench.ListIDs(collection)); got != 0 {
				t.Errorf("a refused attach left %d entities at %s", got, collection)
			}
		}
	})
}

// TestStructuralActsOnACommentAndAnAttachment asserts the easy half of the
// design, and with it that the protocol is one pattern rather than a card-only
// special case: archiving a comment and deleting an attachment take the
// enclosing card's lock, write a sibling in the entity's own collection, land
// the entity in the mirror at its own level, and record the event on the
// card's journal. An interrupted one is finished the same way a card's is.
func TestStructuralActsOnACommentAndAnAttachment(t *testing.T) {
	h := newHarness(t)
	ref := h.add("annotated")
	if response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: ref, Text: "a thought"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("the bytes"), 0o644); err != nil {
		t.Fatalf("source: %v", err)
	}
	attached := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: ref, File: source})
	if attached.Outcome != contract.OutcomeOK {
		t.Fatalf("attach: %s %s", attached.Outcome, attached.Refusal)
	}
	h.reopen()
	card := h.card(ref)
	commentID := bench.ListIDs(filepath.Join(card.Dir, bench.CommentsDir))[0]
	commentRef := ref + "/comments/1"
	attachmentRef := ref + "/attachments/1"

	// The card's own lock stops both acts, because it sits above the
	// directory that moves and outlives it.
	held := h.hold(card.Dir, "someone")
	for _, response := range []*Response{
		h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: commentRef}),
		h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: attachmentRef, Confirm: true}),
	} {
		if response.Refusal != contract.Locked {
			t.Errorf("with the card's lock held: wanted %s, got %s %s", contract.Locked, response.Outcome, response.Refusal)
		}
	}
	held.Release()

	// The sibling stands in the comment's own collection while the act runs,
	// and is gone once it has finished.
	sibling := bench.SiblingPath(filepath.Join(card.Dir, bench.CommentsDir, commentID))
	stood := false
	h.inWindow(func() { stood = bench.Exists(sibling) })
	response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: commentRef})
	h.library.Bench.Hooks = nil
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive the comment: %s %s", response.Outcome, response.Refusal)
	}
	if !stood {
		t.Error("no sibling stood beside the comment while its archive ran")
	}
	if bench.Exists(sibling) {
		t.Error("the sibling outlived the act")
	}
	mirror := filepath.Join(card.Dir, bench.ArchiveDir, bench.CommentsDir, commentID)
	if !bench.Exists(filepath.Join(mirror, bench.CommentAnchor)) {
		t.Errorf("the comment did not land in the mirror at its own level, wanted %s", mirror)
	}
	h.reopen()
	if !recorded(h.events(ref), contract.EventArchived, commentID) {
		t.Error("the card's journal carries no archived event for the comment")
	}

	// An interrupted act below a card finishes exactly as a card's does.
	h.abortAt(5)
	h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: attachmentRef, Confirm: true})
	h.clearBenchLock()
	h.reopen()
	if _, ok := finding(h.check(), bench.FindingInterruptedAct); !ok {
		t.Fatalf("wanted an interrupted-act finding, got %+v", h.check())
	}
	if _, err := h.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if got := len(bench.ListIDs(filepath.Join(card.Dir, bench.AttachmentsDir))); got != 0 {
		t.Errorf("the finish left %d attachments behind", got)
	}
	if findings := h.check(); len(findings) != 0 {
		t.Errorf("the finish left findings behind: %+v", findings)
	}
}

// recorded reports whether a journal carries one act's event about an entity.
func recorded(events []bench.Event, name, id string) bool {
	for _, ev := range events {
		if ev.Event == name && ev.Note == id {
			return true
		}
	}
	return false
}

// TestAnInterruptedArchiveIsRolledBackBeforeItsRecord asserts the crash rows
// before the point of record. An act aborted once it holds the entity's lock
// has written no event, so check reports the interruption and repairs nothing,
// and the finish rolls the sibling away, removes the lock the dead act left
// inside the directory, and leaves the card byte-identical to what it was.
//
// The step hook fires after the numbered step completes, so aborting at step
// three is a crash between the third acquisition and the journal append.
func TestAnInterruptedArchiveIsRolledBackBeforeItsRecord(t *testing.T) {
	h := newHarness(t)
	ref := h.add("interrupted")
	card := h.card(ref)
	id := card.ID
	before := hashDirectory(t, card.Dir)

	h.abortAt(3)
	h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
	h.clearBenchLock()
	h.reopen()

	if !bench.Exists(filepath.Join(card.Dir, bench.LockName)) {
		t.Error("a crash between the third acquisition and the append leaves the entity's lock behind")
	}
	findings := h.check()
	detail, ok := finding(findings, bench.FindingInterruptedAct)
	if !ok {
		t.Fatalf("wanted an interrupted-act finding, got %+v", findings)
	}
	if detail != id+" "+bench.OpArchive+" "+bench.DirectionRollback {
		t.Errorf("the finding should name the id, the op and the direction, got %q", detail)
	}
	if !bench.Exists(bench.SiblingPath(card.Dir)) {
		t.Error("a bare check repaired the sibling away")
	}

	if _, err := h.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if bench.Exists(bench.SiblingPath(card.Dir)) {
		t.Error("the finish left the sibling standing")
	}
	if !bench.Exists(filepath.Join(card.Dir, bench.CardAnchor)) {
		t.Fatal("the rollback lost the card")
	}
	if after := hashDirectory(t, card.Dir); after != before {
		t.Error("the rolled-back card is not byte-identical to what it was")
	}
	for _, ev := range h.events(ref) {
		if ev.Event == contract.EventArchived {
			t.Error("the rolled-back card carries an archived event")
		}
	}
}

// TestAnInterruptedArchiveIsFinishedForwardAfterItsRecord asserts the crash
// rows past the point of record. An act aborted between the release of the
// entity's lock and the move has its event on the record, so check reports the
// interruption reading forward and the finish completes the move, leaving the
// card in the archive with every line of its history and no lock inside it.
//
// A second finish over the same bench reports nothing and changes nothing,
// which is what makes the repair safe to run twice.
func TestAnInterruptedArchiveIsFinishedForwardAfterItsRecord(t *testing.T) {
	h := newHarness(t)
	ref := h.add("interrupted")
	card := h.card(ref)
	id := card.ID
	lines := journalLength(t, card.JournalPath())

	h.abortAt(5)
	h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
	h.clearBenchLock()
	h.reopen()

	if !bench.Exists(filepath.Join(card.Dir, bench.CardAnchor)) {
		t.Fatal("the abort came after the move rather than before it")
	}
	detail, ok := finding(h.check(), bench.FindingInterruptedAct)
	if !ok {
		t.Fatal("wanted an interrupted-act finding")
	}
	if detail != id+" "+bench.OpArchive+" "+bench.DirectionForward {
		t.Errorf("the finding should read forward, got %q", detail)
	}

	if _, err := h.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	archived := h.archivedDir(id)
	if !bench.Exists(filepath.Join(archived, bench.CardAnchor)) {
		t.Fatalf("the finish did not complete the move, wanted the card at %s", archived)
	}
	if bench.Exists(card.Dir) {
		t.Error("the finish left the card in the live half as well")
	}
	if bench.Exists(bench.SiblingPath(card.Dir)) {
		t.Error("the finish left the sibling standing")
	}
	if bench.Exists(filepath.Join(archived, bench.LockName)) {
		t.Error("a lock travelled into the archive")
	}
	if got := journalLength(t, filepath.Join(archived, bench.JournalName)); got != lines+1 {
		t.Errorf("the archived journal carries %d lines, wanted %d", got, lines+1)
	}

	column := hashDirectory(t, h.root)
	second, err := h.finish()
	if err != nil {
		t.Fatalf("a second finish: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("a second finish reported %+v", second)
	}
	if after := hashDirectory(t, h.root); after != column {
		t.Error("a second finish changed the workbench")
	}
}

// TestAnInterruptedDeletionIsFinishedFromTheBenchJournal asserts the deletion
// rows, including the one a partial removal leaves. A card whose anchor is
// gone and whose journal is not would otherwise read as the quarantine case,
// and the sibling is what tells the two apart.
func TestAnInterruptedDeletionIsFinishedFromTheBenchJournal(t *testing.T) {
	t.Run("the directory is whole", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("doomed")
		id := h.card(ref).ID
		h.abortAt(5)
		h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: ref, Confirm: true})
		h.clearBenchLock()
		h.reopen()
		if !recorded(h.benchEvents(), contract.EventDeleted, id) {
			t.Fatal("the workbench journal carries no deleted event to finish from")
		}
		if _, err := h.finish(); err != nil {
			t.Fatalf("finish: %v", err)
		}
		if bench.Exists(filepath.Join(h.library.Bench.CardsRoot(), id)) {
			t.Error("the finish left the card behind")
		}
		if findings := h.check(); len(findings) != 0 {
			t.Errorf("the finish left findings behind: %+v", findings)
		}
	})

	t.Run("the removal got part way", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("doomed")
		card := h.card(ref)
		id := card.ID
		h.abortAt(5)
		h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: ref, Confirm: true})
		h.clearBenchLock()
		h.reopen()
		if err := os.Remove(filepath.Join(card.Dir, bench.CardAnchor)); err != nil {
			t.Fatalf("simulate a partial removal: %v", err)
		}

		findings := h.check()
		if _, ok := finding(findings, bench.FindingMissingAnchor); ok {
			t.Error("a half-removed card read as the quarantine case rather than as an interrupted act")
		}
		if _, ok := finding(findings, bench.FindingInterruptedAct); !ok {
			t.Fatalf("wanted an interrupted-act finding, got %+v", findings)
		}
		if _, err := h.finish(); err != nil {
			t.Fatalf("finish: %v", err)
		}
		if bench.Exists(filepath.Join(h.library.Bench.CardsRoot(), id)) {
			t.Error("the finish left the remains behind")
		}
	})
}

// TestTheTwoUnresolvableCrashStatesAreReportedAndNotRepaired asserts the row
// that keeps directory-rename atomicity out of the correctness argument: a
// directory found at both paths and one found at neither are each reported,
// and the finish leaves the bench exactly as it found it.
func TestTheTwoUnresolvableCrashStatesAreReportedAndNotRepaired(t *testing.T) {
	cases := []struct {
		name  string
		plant func(*harness, string)
		key   string
	}{
		{
			name: "at both paths",
			plant: func(h *harness, id string) {
				live := filepath.Join(h.library.Bench.CardsRoot(), id)
				if err := bench.WriteText(filepath.Join(h.archivedDir(id), bench.CardAnchor), "---\ntitle: copy\n---\n"); err != nil {
					h.t.Fatalf("copy: %v", err)
				}
				_ = live
			},
			key: bench.FindingEntityAtBothPaths,
		},
		{
			name: "at neither path",
			plant: func(h *harness, id string) {
				if err := os.RemoveAll(filepath.Join(h.library.Bench.CardsRoot(), id)); err != nil {
					h.t.Fatalf("remove: %v", err)
				}
			},
			key: bench.FindingInterruptedAct,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			ref := h.add("ambiguous")
			card := h.card(ref)
			id := card.ID
			record := bench.LockRecord{
				Actor: "alka",
				PID:   4242,
				TS:    bench.Stamp(h.clock),
				Op:    bench.OpArchive,
				To:    bench.ArchiveTarget(card.Dir),
			}
			h.plant(bench.SiblingPath(card.Dir), record)
			c.plant(h, id)
			h.reopen()

			detail, ok := finding(h.check(), c.key)
			if !ok {
				t.Fatalf("wanted a %s finding, got %+v", c.key, h.check())
			}
			if c.key == bench.FindingInterruptedAct && detail != id+" "+bench.OpArchive+" "+bench.DirectionMissing {
				t.Errorf("the finding should name the missing directory, got %q", detail)
			}

			before := hashDirectory(t, h.root)
			findings, err := h.finish()
			if err != nil {
				t.Fatalf("finish: %v", err)
			}
			if _, ok := finding(findings, c.key); !ok {
				t.Errorf("the finish should report what it will not resolve, got %+v", findings)
			}
			if after := hashDirectory(t, h.root); after != before {
				t.Error("the finish changed a workbench it had refused to resolve")
			}
		})
	}
}

// TestALiveFailureAfterTheRecordReportsAnInterruption asserts the row a crash
// cannot reach: the process survives and its own unwind code runs. A failure
// injected after the journal append leaves the sibling standing, releases the
// bench lock so the repair is not deadlocked against its predecessor, and
// reports the act as interrupted rather than as a plain failure. The finish
// then completes it with nothing else changed.
//
// Arming: making the unwind unconditional takes the sibling off, the archived
// event stands beside a live card, and nothing on disk says an act was in
// flight, which is the recordless state lock-then-move was rejected for.
func TestALiveFailureAfterTheRecordReportsAnInterruption(t *testing.T) {
	h := newHarness(t)
	ref := h.add("interrupted")
	card := h.card(ref)
	id := card.ID

	h.library.Bench.Hooks = &bench.Hooks{
		AfterStep: func(n int) error {
			if n == 5 {
				return errors.New("the rename was refused")
			}
			return nil
		},
	}
	response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
	h.reopen()

	if response.Refusal != contract.Interrupted {
		t.Fatalf("wanted %s, got %s %s", contract.Interrupted, response.Outcome, response.Refusal)
	}
	if response.Detail != id {
		t.Errorf("the refusal should name the entity, got %q", response.Detail)
	}
	if !bench.Exists(bench.SiblingPath(card.Dir)) {
		t.Error("the unwind took the sibling off after the point of record")
	}
	if bench.Exists(filepath.Join(h.root, bench.LockName)) {
		t.Error("the workbench lock was left held, so the finish would deadlock against it")
	}
	if bench.Exists(filepath.Join(card.Dir, bench.LockName)) {
		t.Error("the entity's own lock was left inside the directory")
	}

	if _, err := h.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if !bench.Exists(filepath.Join(h.archivedDir(id), bench.CardAnchor)) {
		t.Error("the finish did not complete the interrupted archive")
	}
}

// TestTheFinishClearsOnlyTheLockItsOwnActLeft asserts the three encounters a
// repair can have with a lock. A lock the interrupted act itself left inside
// the directory is deleted before the move, so nothing travels into the
// archive. A lock naming anyone else belongs to a live process, so the finish
// reports it and stops with the directory untouched. And a bench lock a dead
// predecessor left behind refuses the finish until a human clears it, which is
// the crash-then-clear-then-finish sequence a person actually walks.
func TestTheFinishClearsOnlyTheLockItsOwnActLeft(t *testing.T) {
	t.Run("its own lock is cleared before the move", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("interrupted")
		card := h.card(ref)
		id := card.ID
		h.abortAt(4)
		h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
		h.clearBenchLock()
		h.reopen()
		if !bench.Exists(filepath.Join(card.Dir, bench.LockName)) {
			t.Fatal("the abort left no entity lock to clear")
		}
		if _, err := h.finish(); err != nil {
			t.Fatalf("finish: %v", err)
		}
		archived := h.archivedDir(id)
		if !bench.Exists(filepath.Join(archived, bench.CardAnchor)) {
			t.Fatal("the finish did not complete the move")
		}
		if bench.Exists(filepath.Join(archived, bench.LockName)) {
			t.Error("a lock travelled into the archive")
		}
	})

	t.Run("a lock naming a stranger stops it", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("interrupted")
		card := h.card(ref)
		h.abortAt(4)
		h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
		h.clearBenchLock()
		h.reopen()
		h.plant(filepath.Join(card.Dir, bench.LockName), bench.LockRecord{
			Actor: "somebody else",
			PID:   9999,
			TS:    bench.Stamp(h.clock),
		})
		before := hashDirectory(t, card.Dir)

		findings, err := h.finish()
		if err != nil {
			t.Fatalf("finish: %v", err)
		}
		detail, ok := finding(findings, bench.FindingInterruptedAct)
		if !ok {
			t.Fatalf("the finish should report the lock it would not break, got %+v", findings)
		}
		if !strings.HasSuffix(detail, bench.DirectionLocked) {
			t.Errorf("the finding should say a lock stopped it, got %q", detail)
		}
		if after := hashDirectory(t, card.Dir); after != before {
			t.Error("the finish touched a directory a live process holds")
		}
	})

	t.Run("a stale workbench lock refuses it", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("interrupted")
		card := h.card(ref)
		h.abortAt(5)
		h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
		h.reopen()
		h.plant(filepath.Join(h.root, bench.LockName), bench.LockRecord{
			Actor: "the dead predecessor",
			PID:   9999,
			TS:    bench.Stamp(h.clock),
		})

		_, err := h.finish()
		refusal, ok := err.(*contract.Refusal)
		if !ok {
			t.Fatalf("wanted a refusal, got %v", err)
		}
		if refusal.Name != contract.Locked || refusal.Detail != "the dead predecessor" {
			t.Errorf("wanted %s naming the holder, got %s %q", contract.Locked, refusal.Name, refusal.Detail)
		}
		if !bench.Exists(filepath.Join(card.Dir, bench.CardAnchor)) {
			t.Error("the refused finish moved the card anyway")
		}
	})
}

// guideCommand finds every `dinah <word>` a guide's prose names, which is what
// a reader will type after reading it.
var guideCommand = regexp.MustCompile(`\bdinah ([a-z][a-z-]*)`)

// TestEveryCommandTheGuidesNameIsOneTheToolHas holds the embedded guides
// against the command definitions rather than against a reader's memory. A
// guide that teaches a command the tool dropped, or spells one wrongly, fails
// the build here instead of misleading somebody at a terminal.
//
// It is one half of what dinah-144 asks for. The other half, checking that a
// guide's example produces the output the guide shows, is that card's work and
// this test does not attempt it.
func TestEveryCommandTheGuidesNameIsOneTheToolHas(t *testing.T) {
	known := map[string]bool{}
	for _, name := range Commands() {
		known[name] = true
	}
	checked := 0
	for _, topic := range guide.Topics() {
		text, err := guide.Text(topic)
		if err != nil {
			t.Fatalf("guide %s: %v", topic, err)
		}
		for _, found := range guideCommand.FindAllStringSubmatch(text, -1) {
			checked++
			if !known[found[1]] {
				t.Errorf("the %s guide tells a reader to run `dinah %s`, and no command is declared under that name", topic, found[1])
			}
		}
	}
	if checked == 0 {
		t.Error("no guide named a command, so this test read the wrong thing and asserts nothing")
	}
}

// TestTheQueryGuideTeachesTheFieldsTheLanguageHas asserts that the query
// guide's field list is the language's own, in both directions: every field the
// parser admits is taught, and the guide's list is exactly ten items long, so a
// field added to the vocabulary and left out of the guide fails the build.
func TestTheQueryGuideTeachesTheFieldsTheLanguageHas(t *testing.T) {
	text, err := guide.Text("query")
	if err != nil {
		t.Fatalf("the query guide: %v", err)
	}
	for _, field := range QueryFields {
		if !strings.Contains(text, "- `"+field+"` is ") {
			t.Errorf("the query guide does not teach the field %s", field)
		}
	}
	taught := strings.Count(text, "- `")
	if taught != len(QueryFields) {
		t.Errorf("the query guide lists %d fields and the language has %d", taught, len(QueryFields))
	}
}

// specSection6EscapeHatch is the two-line escape hatch of the dinah-135 spec's
// section 6, transcribed once so that the guide is held against one string
// rather than against a second typing of the same SQL. The unnest is the part
// that matters: a reader who writes the query as though the cards were the
// document's top level binds against a column that is not there.
const specSection6EscapeHatch = "dinah query --json > cards.json\n" +
	`duckdb -c "select card.column_title, count(*) from (select unnest(cards) as card from read_json_auto('cards.json')) group by 1 order by 1"`

// TestTheQueryGuideCarriesTheEscapeHatchTheSpecFixed asserts that the guide
// hands a reader the escape hatch the spec settled on, character for
// character, since a downstream reader that binds against the wrong shape fails
// with a column error rather than with an empty result.
func TestTheQueryGuideCarriesTheEscapeHatchTheSpecFixed(t *testing.T) {
	text, err := guide.Text("query")
	if err != nil {
		t.Fatalf("the query guide: %v", err)
	}
	flat := strings.Join(strings.Fields(strings.ReplaceAll(text, "\n", " ")), " ")
	want := strings.Join(strings.Fields(strings.ReplaceAll(specSection6EscapeHatch, "\n", " ")), " ")
	if !strings.Contains(flat, want) {
		t.Errorf("the query guide does not carry the escape hatch the spec fixed:\n%s", specSection6EscapeHatch)
	}
	if !strings.Contains(text, "unnest(cards)") {
		t.Error("the query guide's escape hatch does not unnest the cards member")
	}
}

// TestWorkbenchReadsTheThreeFieldsAndRefusesAnyOther asserts that the read
// answers with the workbench's own three fields, that it is open to a caller
// naming no actor at all, and that a get naming a field outside the set
// refuses with the name config already raises for the same mistake.
func TestWorkbenchReadsTheThreeFieldsAndRefusesAnyOther(t *testing.T) {
	h := newHarness(t)
	view, err := h.library.Workbench(&Request{Verb: "workbench"})
	if err != nil {
		t.Fatalf("the read refused a caller naming no actor: %v", err)
	}
	if view.Title != "Fixture" || view.Slug != "fx" || view.Operator != "alka" {
		t.Errorf("the read answered %+v, wanted the fixture's own three fields", view)
	}
	for name, wanted := range map[string]string{"title": "Fixture", "slug": "fx", "operator": "alka"} {
		if got := view.Field(name); got != wanted {
			t.Errorf("the %s field read back as %q, wanted %q", name, got, wanted)
		}
	}
	if got := view.Field("profile"); got != "" {
		t.Errorf("a name outside the set read back as %q, wanted nothing", got)
	}
	_, err = h.library.Workbench(&Request{Verb: "workbench", Action: "get", Field: "profile"})
	refusal := &contract.Refusal{}
	if !errors.As(err, &refusal) || refusal.Name != contract.UnknownKey {
		t.Fatalf("a get of an unknown field: wanted %s, got %v", contract.UnknownKey, err)
	}
	if refusal.Detail != "profile" {
		t.Errorf("the refusal names %q, wanted the field the caller typed", refusal.Detail)
	}
}

// TestSetWorkbenchEvaluatesItsChecksInOrder asserts the ladder the spec fixes.
// Each case satisfies every rung above the one it is aimed at, so a rung that
// stopped running would show up as the rung below it answering in its place,
// and each case leaves the anchor byte-identical.
func TestSetWorkbenchEvaluatesItsChecksInOrder(t *testing.T) {
	cases := []struct {
		name    string
		request *Request
		wanted  string
		detail  string
	}{
		{
			name:    "an unknown field, before anything about the value",
			request: &Request{Verb: "workbench", Actor: "alka", Field: "profile", Value: ""},
			wanted:  contract.UnknownKey,
			detail:  "profile",
		},
		{
			name:    "an empty value, before the owner is asked for",
			request: &Request{Verb: "workbench", Field: "title", Value: "   "},
			wanted:  contract.Malformed,
			detail:  "title",
		},
		{
			name:    "a slug outside the grammar, on the same rung",
			request: &Request{Verb: "workbench", Actor: "alka", Field: "slug", Value: "sprint-2", Confirm: true},
			wanted:  contract.Malformed,
			detail:  "slug",
		},
		{
			name:    "no owner, before the operator is compared",
			request: &Request{Verb: "workbench", Field: "title", Value: "Renamed"},
			wanted:  contract.NoOwner,
		},
		{
			name:    "an owner who is not the operator",
			request: &Request{Verb: "workbench", Actor: "bob", Field: "title", Value: "Renamed"},
			wanted:  contract.NotOperator,
			detail:  "bob",
		},
		{
			name:    "a rename carrying no confirmation, last of the six",
			request: &Request{Verb: "workbench", Actor: "alka", Field: "slug", Value: "fx-dev"},
			wanted:  contract.Unconfirmed,
			detail:  "fx-dev",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			before := h.anchorBytes()
			response := h.library.SetWorkbench(c.request)
			if response.Outcome != contract.OutcomeRefused || response.Refusal != c.wanted {
				t.Fatalf("wanted %s, got %s %s", c.wanted, response.Outcome, response.Refusal)
			}
			if response.Detail != c.detail {
				t.Errorf("the refusal names %q, wanted %q", response.Detail, c.detail)
			}
			if after := h.anchorBytes(); after != before {
				t.Error("the refusal wrote to the anchor")
			}
		})
	}

	t.Run("a workbench designating no operator, ahead of all five", func(t *testing.T) {
		h := newHarness(t)
		h.library.Bench.Operator = ""
		response := h.library.SetWorkbench(&Request{Verb: "workbench", Actor: "alka", Field: "profile", Value: ""})
		if response.Refusal != contract.NoOperator {
			t.Errorf("wanted %s ahead of the unknown field, got %s", contract.NoOperator, response.Refusal)
		}
	})

	t.Run("a title and an operator need no confirmation", func(t *testing.T) {
		h := newHarness(t)
		for _, field := range []string{"title", "operator"} {
			response := h.library.SetWorkbench(&Request{Verb: "workbench", Actor: "alka", Field: field, Value: "alka"})
			if response.Outcome != contract.OutcomeOK {
				t.Errorf("set %s without the flag: %s %s", field, response.Outcome, response.Refusal)
			}
			h.reopen()
		}
	})
}

// TestSetWorkbenchWritesUnderOneLockAndJournalsWhatChanged asserts the write
// itself: each of the three fields round-trips, the keys the tool does not set
// survive, one workbench_updated event lands per write carrying what it
// rewrote, and a second library driven into the middle of the transaction sees
// an anchor and a journal that have not moved yet and cannot take the lock.
func TestSetWorkbenchWritesUnderOneLockAndJournalsWhatChanged(t *testing.T) {
	h := newHarness(t)
	writes := []struct {
		field string
		value string
		was   string
	}{
		{field: "title", value: "The renamed fixture", was: "Fixture"},
		{field: "operator", value: "alka", was: "alka"},
		{field: "slug", value: "fx-dev", was: "fx"},
	}
	for _, w := range writes {
		req := &Request{Verb: "workbench", Actor: "alka", Field: w.field, Value: w.value, Confirm: true}
		if response := h.library.SetWorkbench(req); response.Outcome != contract.OutcomeOK {
			t.Fatalf("set %s: %s %s", w.field, response.Outcome, response.Refusal)
		}
		h.reopen()
		if got := h.library.Bench.WorkbenchField(w.field); got != w.value {
			t.Errorf("%s read back as %q, wanted %q", w.field, got, w.value)
		}
	}
	// The structural keys and the standing text the command never names.
	if h.library.Bench.Profile == "" || h.library.Bench.Format == 0 || len(h.library.Bench.Columns) != 6 {
		t.Errorf("a write disturbed the structural keys: profile %q, format %d, %d columns",
			h.library.Bench.Profile, h.library.Bench.Format, len(h.library.Bench.Columns))
	}
	if !strings.Contains(h.library.Bench.Standing, "The standing text of this workbench.") {
		t.Errorf("a write disturbed the standing text: %q", h.library.Bench.Standing)
	}

	var journalled []bench.Event
	for _, ev := range h.benchEvents() {
		if ev.Event == contract.EventWorkbenchUpdated {
			journalled = append(journalled, ev)
		}
	}
	if len(journalled) != len(writes) {
		t.Fatalf("wanted one %s event per write, got %d", contract.EventWorkbenchUpdated, len(journalled))
	}
	for i, ev := range journalled {
		if ev.TS == "" || ev.Actor != "alka" {
			t.Errorf("event %d carries ts %q and actor %q", i, ev.TS, ev.Actor)
		}
		if ev.Field != writes[i].field || ev.From != writes[i].was || ev.To != writes[i].value {
			t.Errorf("event %d reads field %q from %q to %q, wanted %q, %q and %q",
				i, ev.Field, ev.From, ev.To, writes[i].field, writes[i].was, writes[i].value)
		}
	}

	// The whole write sits inside one acquisition of the workbench root, so a
	// second view reaching the middle of it finds nothing written yet and
	// cannot take the lock for itself.
	h.reopen()
	other := h.second()
	var midAnchor string
	var midEvents int
	var blocked *Response
	h.library.Interleave = func() {
		midAnchor = h.anchorBytes()
		midEvents = len(h.benchEvents())
		blocked = other.SetWorkbench(&Request{Verb: "workbench", Actor: "alka", Field: "title", Value: "A third name"})
	}
	before := h.anchorBytes()
	beforeEvents := len(h.benchEvents())
	response := h.library.SetWorkbench(&Request{Verb: "workbench", Actor: "alka", Field: "title", Value: "Renamed again"})
	h.library.Interleave = nil
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("the write the hook ran inside: %s %s", response.Outcome, response.Refusal)
	}
	if midAnchor == "" || blocked == nil {
		t.Fatal("the interleave hook never ran, so this test proves nothing")
	}
	if midAnchor != before {
		t.Error("the anchor had already moved when the hook fired, so the write is not inside the lock")
	}
	if midEvents != beforeEvents {
		t.Error("the journal had already grown when the hook fired, so the event is not inside the lock")
	}
	if blocked.Refusal != contract.Locked {
		t.Errorf("the interleaved write: wanted %s, got %s %s", contract.Locked, blocked.Outcome, blocked.Refusal)
	}
}

// anchorBytes reads the workbench anchor as it stands, which is what a test
// asserting that a refusal wrote nothing compares.
func (h *harness) anchorBytes() string {
	h.t.Helper()
	text, err := bench.ReadText(filepath.Join(h.root, bench.WorkbenchAnchor))
	if err != nil {
		h.t.Fatalf("read the anchor: %v", err)
	}
	return text
}

// newWorkstream files a workstream through the library and fails the test
// unless it was created, returning what the response carried.
func (h *harness) newWorkstream(title string) *WorkstreamView {
	h.t.Helper()
	response := h.library.NewWorkstream(&Request{Verb: "workstream", Action: "new", Actor: "alka", Workstream: title})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("workstream new %q: %s %s", title, response.Outcome, response.Refusal)
	}
	h.reopen()
	return response.Workstream
}

// workstreamEvents reads one workstream's journal, which is where its own arc
// is recorded.
func (h *harness) workstreamEvents(id string) []bench.Event {
	h.t.Helper()
	workstream := h.library.Bench.Workstream(id)
	if workstream == nil {
		h.t.Fatalf("the workbench carries no workstream %s", id)
	}
	events, torn, err := bench.ReadJournal(workstream.JournalPath())
	if err != nil {
		h.t.Fatalf("read the journal of workstream %s: %v", id, err)
	}
	if torn {
		h.t.Fatalf("the journal of workstream %s is torn", id)
	}
	return events
}

// TestAWorkstreamIsBornWithASlugAStatusAnOrdinalAndAJournal asserts what
// `workstream new` writes: the four frontmatter fields, the slug derived from
// the whole title, and a journal opening with a created event naming the
// caller.
func TestAWorkstreamIsBornWithASlugAStatusAnOrdinalAndAJournal(t *testing.T) {
	h := newHarness(t)
	view := h.newWorkstream("Portfolio work")
	if view.Slug != "portfolio-work" {
		t.Errorf("the slug is %q, and the whole title through SlugifyDashed is portfolio-work", view.Slug)
	}
	if view.Title != "Portfolio work" || view.Status != bench.StatusActive || view.Cards != 0 {
		t.Errorf("the workstream reads %+v, wanted the title, an active status and no cards", view)
	}
	stored := h.library.Bench.Workstream(view.ID)
	if stored == nil {
		t.Fatalf("the workbench carries no workstream %s", view.ID)
	}
	if stored.Ordinal != 1 {
		t.Errorf("the creation ordinal is %d, wanted 1", stored.Ordinal)
	}
	events := h.workstreamEvents(view.ID)
	if len(events) != 1 || events[0].Event != contract.EventCreated || events[0].Actor != "alka" {
		t.Errorf("the journal opens with %+v, wanted one created event naming alka", events)
	}
	listing, err := h.library.Workstreams()
	if err != nil {
		t.Fatalf("list the workstreams: %v", err)
	}
	if len(listing.Workstreams) != 1 || listing.Workstreams[0].Cards != 0 {
		t.Errorf("the listing reads %+v, wanted the one workstream with no cards", listing.Workstreams)
	}
}

// TestASecondWorkstreamOfTheSameTitleTakesTheCountingSuffix asserts the
// collision resolution the column slugs already use, and that the suffix the
// resolver writes is a slug the workstream grammar admits.
func TestASecondWorkstreamOfTheSameTitleTakesTheCountingSuffix(t *testing.T) {
	h := newHarness(t)
	first := h.newWorkstream("Sprint 2")
	second := h.newWorkstream("Sprint 2")
	if first.Slug != "sprint-2" {
		t.Errorf("the first slug is %q, and SlugifyDashed derives sprint-2 where Slugify would derive sprint2", first.Slug)
	}
	if second.Slug != "sprint-2-2" {
		t.Errorf("the second slug is %q, wanted the counting suffix sprint-2-2", second.Slug)
	}
	for _, slug := range []string{first.Slug, second.Slug} {
		if !bench.ValidColumnSlug(slug) {
			t.Errorf("%q is not a slug the grammar admits, so the resolver wrote something nobody can type", slug)
		}
	}
}

// TestRenamingAWorkstreamOntoATakenSlugIsAcceptedAndReported pins the column a
// write can reach and creation cannot. The write is accepted, the shared slug
// still resolves to the earlier workstream, and check raises exactly one
// duplicate finding, naming the workstream whose slug is now shadowed.
//
// Four assertions carry four different regressions. A later card that decides
// to refuse the write turns the test red at the outcome. One that changes which
// workstream a reference reaches turns it red at the resolution. One that makes
// check report both workstreams turns it red at the count. And one that puts
// checkWorkstreams back on an unordered walk turns it red at the identity,
// since the finding would then name whichever of the pair sorted later by a
// random identifier.
//
// The workbench holds a third workstream carrying its own slug, and the count
// is what that one is here for. On a workbench holding the colliding pair
// alone, a check that raised a duplicate over a workstream sharing its slug
// with nobody has no such workstream to raise it over, so the count agrees with
// a checker that cannot tell a collision from an ordinary slug. The bystander
// is the wrong answer that assertion had none of.
func TestRenamingAWorkstreamOntoATakenSlugIsAcceptedAndReported(t *testing.T) {
	h := newHarness(t)
	first := h.newWorkstream("Portfolio work")
	second := h.newWorkstream("Console redesign")
	bystander := h.newWorkstream("Documentation sweep")

	req := &Request{Verb: "workstream", Action: "set", Actor: "alka", Workstream: "console-redesign", Field: bench.SlugField, Value: first.Slug, Confirm: true}
	got := h.library.SetWorkstream(req)
	h.reopen()
	if got.Outcome != contract.OutcomeOK {
		t.Fatalf("renaming onto a taken slug: %s %s, wanted it accepted", got.Outcome, got.Refusal)
	}
	if stored := h.library.Bench.Workstream(second.ID); stored.Slug != first.Slug {
		t.Errorf("the second workstream stored the slug %q, wanted the duplicate %q it was told to take", stored.Slug, first.Slug)
	}
	if found := h.library.Bench.WorkstreamByRef(first.Slug); found == nil || found.ID != first.ID {
		t.Errorf("the shared slug resolved to %v, wanted the earlier workstream %s", found, first.ID)
	}
	var duplicates []string
	for _, finding := range h.check() {
		if finding.Key == bench.FindingWorkstreamSlugDuplicate {
			duplicates = append(duplicates, finding.Detail)
		}
	}
	if len(duplicates) != 1 {
		t.Fatalf("check raised %d %s findings %v, wanted exactly one over the pair, so the duplicate this write minted is either undetected or reported twice", len(duplicates), bench.FindingWorkstreamSlugDuplicate, duplicates)
	}
	if duplicates[0] != second.ID {
		t.Errorf("the duplicate finding names %s, wanted the shadowed workstream %s; the earlier workstream is %s and the bystander is %s", duplicates[0], second.ID, first.ID, bystander.ID)
	}
}

// TestWritingAWorkstreamFieldRecordsOneUpdateOnItsOwnJournal asserts the field
// write, the one event it appends, and the confirmation a slug change needs.
func TestWritingAWorkstreamFieldRecordsOneUpdateOnItsOwnJournal(t *testing.T) {
	h := newHarness(t)
	view := h.newWorkstream("Portfolio work")

	set := func(field, value string, confirm bool) *Response {
		h.t.Helper()
		req := &Request{Verb: "workstream", Action: "set", Actor: "alka", Workstream: "portfolio-work", Field: field, Value: value, Confirm: confirm}
		response := h.library.SetWorkstream(req)
		h.reopen()
		return response
	}

	if got := set("status", "paused", false); got.Outcome != contract.OutcomeOK {
		t.Fatalf("set status: %s %s", got.Outcome, got.Refusal)
	}
	stored := h.library.Bench.Workstream(view.ID)
	if stored.Status != "paused" {
		t.Errorf("the stored status is %q, wanted paused", stored.Status)
	}
	events := h.workstreamEvents(view.ID)
	if len(events) != 2 {
		t.Fatalf("the journal carries %d events, wanted the created event and one update", len(events))
	}
	update := events[1]
	if update.Event != contract.EventWorkstreamUpdated || update.Field != "status" || update.From != bench.StatusActive || update.To != "paused" {
		t.Errorf("the update reads %+v, wanted field=status from=active to=paused", update)
	}

	before, err := bench.ReadText(stored.AnchorPath())
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	refused := set(bench.SlugField, "folio", false)
	if refused.Outcome != contract.OutcomeRefused || refused.Refusal != contract.Unconfirmed {
		t.Fatalf("a slug change without --yes: %s %s, wanted a refusal of %s", refused.Outcome, refused.Refusal, contract.Unconfirmed)
	}
	after, err := bench.ReadText(stored.AnchorPath())
	if err != nil {
		t.Fatalf("read the anchor again: %v", err)
	}
	if after != before {
		t.Errorf("the refused slug change rewrote the anchor:\n%q\n%q", before, after)
	}
	if got := set(bench.SlugField, "folio", true); got.Outcome != contract.OutcomeOK {
		t.Fatalf("a slug change with the flag: %s %s", got.Outcome, got.Refusal)
	}
	if h.library.Bench.WorkstreamByRef("portfolio-work") != nil {
		t.Error("the old slug still resolves after the rename")
	}
	if h.library.Bench.WorkstreamByRef("folio") == nil {
		t.Error("the new slug does not resolve after the rename")
	}
}

// TestJoiningAndLeavingWriteOnlyTheMembershipNamed asserts the pair's whole
// contract: the identifier is appended and removed, the events land on the
// card's own journal, a repeat of either writes nothing at all, and a
// workstream no reference names refuses.
func TestJoiningAndLeavingWriteOnlyTheMembershipNamed(t *testing.T) {
	h := newHarness(t)
	first := h.newWorkstream("Portfolio work")
	second := h.newWorkstream("Console redesign")
	third := h.newWorkstream("Translation")
	ref := h.add("a card to belong")

	for _, slug := range []string{"portfolio-work", "console-redesign", "translation"} {
		h.mustDo(&Request{Verb: Join, Actor: "alka", Card: ref, Workstream: slug})
	}
	card := h.card(ref)
	wanted := []string{first.ID, second.ID, third.ID}
	if strings.Join(card.Workstreams, ",") != strings.Join(wanted, ",") {
		t.Fatalf("the card lists %v, wanted %v in the order they were joined", card.Workstreams, wanted)
	}

	anchor, err := bench.ReadText(card.AnchorPath())
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	h.mustDo(&Request{Verb: Join, Actor: "alka", Card: ref, Workstream: "portfolio-work"})
	again, err := bench.ReadText(h.card(ref).AnchorPath())
	if err != nil {
		t.Fatalf("read the anchor again: %v", err)
	}
	if again != anchor {
		t.Errorf("joining a workstream the card already belongs to rewrote the anchor:\n%q\n%q", anchor, again)
	}

	h.mustDo(&Request{Verb: Leave, Actor: "alka", Card: ref, Workstream: "console-redesign"})
	card = h.card(ref)
	wanted = []string{first.ID, third.ID}
	if strings.Join(card.Workstreams, ",") != strings.Join(wanted, ",") {
		t.Errorf("after leaving the second, the card lists %v, wanted %v in their original order", card.Workstreams, wanted)
	}
	events, _, err := bench.ReadJournal(card.JournalPath())
	if err != nil {
		t.Fatalf("read the card's journal: %v", err)
	}
	joined, left := 0, 0
	for _, ev := range events {
		switch ev.Event {
		case contract.EventWorkstreamJoined:
			joined++
		case contract.EventWorkstreamLeft:
			left++
			if ev.Workstream != second.ID {
				t.Errorf("the left event names workstream %s, wanted %s", ev.Workstream, second.ID)
			}
		}
	}
	if joined != 3 || left != 1 {
		t.Errorf("the card's journal carries %d joins and %d departures, wanted 3 and 1", joined, left)
	}

	anchor, err = bench.ReadText(card.AnchorPath())
	if err != nil {
		t.Fatalf("read the anchor a third time: %v", err)
	}
	h.mustDo(&Request{Verb: Leave, Actor: "alka", Card: ref, Workstream: "console-redesign"})
	again, err = bench.ReadText(h.card(ref).AnchorPath())
	if err != nil {
		t.Fatalf("read the anchor a fourth time: %v", err)
	}
	if again != anchor {
		t.Errorf("leaving a workstream the card never joined rewrote the anchor:\n%q\n%q", anchor, again)
	}

	refused := h.do(&Request{Verb: Join, Actor: "alka", Card: ref, Workstream: "nosuch"})
	if refused.Outcome != contract.OutcomeRefused || refused.Refusal != contract.UnknownWorkstream || refused.Detail != "nosuch" {
		t.Errorf("joining an unknown workstream: %s %s %q, wanted a refusal of %s naming nosuch", refused.Outcome, refused.Refusal, refused.Detail, contract.UnknownWorkstream)
	}
	if got := h.card(ref); strings.Join(got.Workstreams, ",") != strings.Join(wanted, ",") {
		t.Errorf("the refused join rewrote the membership list: %v", got.Workstreams)
	}
}

// TestJoinAndLeaveHonourTheBasis asserts that the pair inherits Do's
// transaction rather than opening one of their own: a basis naming a revision
// the card no longer has comes back stale, carrying the revision on disk, and
// writes nothing.
func TestJoinAndLeaveHonourTheBasis(t *testing.T) {
	h := newHarness(t)
	h.newWorkstream("Portfolio work")
	ref := h.add("a card to belong")
	current := h.card(ref).Revision

	for _, name := range []string{Join, Leave} {
		response := h.do(&Request{Verb: name, Actor: "alka", Card: ref, Workstream: "portfolio-work", Basis: "sha256:0000"})
		if response.Outcome != contract.OutcomeStale {
			t.Errorf("%s on a stale basis: %s, wanted %s", name, response.Outcome, contract.OutcomeStale)
			continue
		}
		if response.Card == nil || response.Card.Revision != current {
			t.Errorf("%s on a stale basis carried back %+v, wanted the revision on disk %s", name, response.Card, current)
		}
		if got := h.card(ref); len(got.Workstreams) != 0 || got.Revision != current {
			t.Errorf("%s on a stale basis wrote to the card: %v %s", name, got.Workstreams, got.Revision)
		}
	}
}

// TestDeletingAWorkstreamLiveCardsBelongToIsRefusedAndArchivingIsNot asserts
// the pair the format's Workstreams section fixes, and the reading this card
// makes of which cards count: the live half alone, so a workstream only
// archived cards list is deleted without refusal and those cards keep the
// membership they were archived with.
func TestDeletingAWorkstreamLiveCardsBelongToIsRefusedAndArchivingIsNot(t *testing.T) {
	h := newHarness(t)
	view := h.newWorkstream("Portfolio work")
	live := h.add("a card that stays")
	going := h.add("a card that is archived")
	h.mustDo(&Request{Verb: Join, Actor: "alka", Card: live, Workstream: "portfolio-work"})
	h.mustDo(&Request{Verb: Join, Actor: "alka", Card: going, Workstream: "portfolio-work"})

	refused := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: "workstream/portfolio-work", Confirm: true})
	h.reopen()
	if refused.Outcome != contract.OutcomeRefused || refused.Refusal != contract.Referenced {
		t.Fatalf("deleting a workstream live cards belong to: %s %s, wanted a refusal of %s", refused.Outcome, refused.Refusal, contract.Referenced)
	}

	archivedCard := h.card(going)
	membership := strings.Join(archivedCard.Workstreams, ",")
	archived := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: going})
	h.reopen()
	if archived.Outcome != contract.OutcomeOK {
		t.Fatalf("archiving the card: %s %s", archived.Outcome, archived.Refusal)
	}
	h.mustDo(&Request{Verb: Leave, Actor: "alka", Card: live, Workstream: "portfolio-work"})

	deleted := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: "workstream/portfolio-work", Confirm: true})
	h.reopen()
	if deleted.Outcome != contract.OutcomeOK {
		t.Fatalf("deleting a workstream only an archived card lists: %s %s", deleted.Outcome, deleted.Refusal)
	}
	if h.library.Bench.Workstream(view.ID) != nil {
		t.Error("the workstream directory survived the deletion")
	}
	stayed, err := bench.LoadCard(h.library.Bench.ArchivedCardsRoot(), archivedCard.ID)
	if err != nil {
		t.Fatalf("read the archived card: %v", err)
	}
	if strings.Join(stayed.Workstreams, ",") != membership {
		t.Errorf("the archived card's membership reads %v, wanted the %v it was archived with", stayed.Workstreams, membership)
	}
	findings, err := h.library.Bench.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		if finding.Key == bench.FindingDanglingWorkstream {
			t.Errorf("check reported %+v about a card in the archived half, which its walk never reaches", finding)
		}
	}
}

// TestArchivingAWorkstreamLeavesItsMembersResolvable asserts that an archived
// workstream still resolves, so a card belonging to one is not a dangler and
// check stays quiet about it.
func TestArchivingAWorkstreamLeavesItsMembersResolvable(t *testing.T) {
	h := newHarness(t)
	view := h.newWorkstream("Portfolio work")
	ref := h.add("a card that belongs")
	h.mustDo(&Request{Verb: Join, Actor: "alka", Card: ref, Workstream: "portfolio-work"})

	archived := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: "workstream/portfolio-work"})
	h.reopen()
	if archived.Outcome != contract.OutcomeOK {
		t.Fatalf("archiving a workstream cards belong to: %s %s", archived.Outcome, archived.Refusal)
	}
	if got := h.library.Bench.Workstreams(); len(got) != 0 {
		t.Errorf("the live listing carries %d workstreams after the archiving, wanted none", len(got))
	}
	if !h.library.Bench.HasWorkstream(view.ID) {
		t.Error("the archived workstream no longer resolves, so its members became danglers")
	}
	findings, err := h.library.Bench.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		if finding.Key == bench.FindingDanglingWorkstream {
			t.Errorf("check reported %+v about a membership that resolves in the archived half", finding)
		}
	}
}

// TestACheckReportSaysWhetherItFoundAnything is dinah-346 AC-2. The three
// cases are the three a client has to tell apart, and the third is the one
// that constrains where the outcome is computed: a migration branch appends
// its own findings before the checker runs, so a report stamped any earlier
// than the end of Check would call that run clean.
func TestACheckReportSaysWhetherItFoundAnything(t *testing.T) {
	h := newHarness(t)
	clean, err := h.library.Check(&Request{Verb: "check", Actor: "alka"})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if clean.Outcome != contract.ReadOK {
		t.Errorf("a clean workbench reports outcome %q, wanted %q, over findings %+v", clean.Outcome, contract.ReadOK, clean.Findings)
	}

	// A directory under the cards root carrying no anchor file, which the
	// checker reports and no migration in this request touches.
	stray := filepath.Join(h.library.Bench.CardsRoot(), "f00000000001")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	h.reopen()
	dirty, err := h.library.Check(&Request{Verb: "check", Actor: "alka"})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if dirty.Outcome != contract.ReadFindings {
		t.Errorf("a workbench carrying %d findings reports outcome %q, wanted %q", len(dirty.Findings), dirty.Outcome, contract.ReadFindings)
	}
	if err := os.Remove(stray); err != nil {
		t.Fatalf("remove the stray directory: %v", err)
	}

	// A comment written by hand, carrying no ordinal and named by no journal
	// entry. The ordinal migration stamps it and reports the guess it had to
	// make, and the checker that runs afterwards finds nothing at all, so
	// the whole report's findings came from the migration branch.
	card := h.add("A card the migration reaches")
	byHand := filepath.Join(filepath.Dir(h.card(card).AnchorPath()), bench.CommentsDir, "e00000000001")
	if err := os.MkdirAll(byHand, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "---\nts: 2026-08-17T09:00:00Z\nauthor: alka\n---\nBy hand.\n"
	if err := os.WriteFile(filepath.Join(byHand, bench.CommentAnchor), []byte(body), 0o644); err != nil {
		t.Fatalf("write the comment: %v", err)
	}
	h.reopen()
	migrated, err := h.library.Check(&Request{Verb: "check", Actor: "alka", MigrateOrdinals: true})
	if err != nil {
		t.Fatalf("check --migrate-ordinals: %v", err)
	}
	if migrated.Outcome != contract.ReadFindings {
		t.Errorf("a migration that reported %+v carries outcome %q, wanted %q", migrated.Findings, migrated.Outcome, contract.ReadFindings)
	}
	guessedOnly := len(migrated.Findings) > 0
	for _, finding := range migrated.Findings {
		if finding.Key != bench.FindingOrdinalGuessed {
			guessedOnly = false
		}
	}
	if !guessedOnly {
		t.Errorf("wanted the migration branch to be the only source of findings, got %+v", migrated.Findings)
	}
}

// newWorkstreamNamingItsSlug files a workstream through the library with the
// slug the caller names, and fails the test unless it was created. It sits
// beside newWorkstream rather than replacing it, because the title-only helper
// is what the two title-derived-path tests drive and this card leaves that path
// exactly where it found it.
func (h *harness) newWorkstreamNamingItsSlug(title, slug string) *WorkstreamView {
	h.t.Helper()
	response := h.library.NewWorkstream(&Request{Verb: "workstream", Action: "new", Actor: "alka", Workstream: title, Slug: slug})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("workstream new %q with the slug %q: %s %s", title, slug, response.Outcome, response.Refusal)
	}
	h.reopen()
	return response.Workstream
}

// workstreamDirs is the collection's own directory listing, which is what
// answers whether a refused creation wrote anything at all. It reads the
// filesystem rather than the opened workbench, because a directory a refusal
// left behind without an anchor is one Workstreams would not carry, and that is
// the leftover these assertions have to be able to see.
func (h *harness) workstreamDirs() []string {
	h.t.Helper()
	entries, err := os.ReadDir(h.library.Bench.WorkstreamsRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		h.t.Fatalf("read the workstreams collection: %v", err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// TestAWorkstreamCreatedWithASlugStoresThatSlug asserts the row this card adds
// to creation: the slug the caller named is what the anchor carries, rather
// than the one SlugifyDashed would have derived from the title.
//
// The stored anchor is read back through bench.LoadWorkstream rather than off
// the response, because a parameter accepted, echoed and never written would
// satisfy a response-only assertion.
func TestAWorkstreamCreatedWithASlugStoresThatSlug(t *testing.T) {
	h := newHarness(t)
	if derived := bench.SlugifyDashed("Autumn release"); derived == "autumn" {
		t.Fatalf("the title derives %q on its own, so this test cannot tell a written slug from a derived one", derived)
	}
	view := h.newWorkstreamNamingItsSlug("Autumn release", "autumn")
	if view.Slug != "autumn" {
		t.Errorf("the response carries the slug %q, wanted the autumn the caller named", view.Slug)
	}
	stored, err := bench.LoadWorkstream(h.library.Bench.WorkstreamsRoot(), view.ID)
	if err != nil {
		t.Fatalf("load the stored workstream: %v", err)
	}
	if stored.Slug != "autumn" {
		t.Errorf("the anchor carries the slug %q, wanted autumn", stored.Slug)
	}
	if stored.Title != "Autumn release" || stored.Status != bench.StatusActive {
		t.Errorf("the anchor reads title %q and status %q, wanted the title and the active status the slug row did not disturb", stored.Title, stored.Status)
	}
	if found := h.library.Bench.WorkstreamByRef("autumn"); found == nil || found.ID != view.ID {
		t.Errorf("the slug autumn resolves to %v, wanted the workstream it was written on", found)
	}
}

// TestAMalformedSlugAtCreationIsRefusedBeforeAnythingIsWritten asserts the
// grammar row, the argument its detail names, and that the refusal leaves the
// collection as empty as it found it.
//
// The detail is the field name rather than the value the caller typed, which is
// the convention SetWorkstream's own malformed rows already keep: the field is
// what a caller can act on.
func TestAMalformedSlugAtCreationIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	for _, slug := range []string{"Autumn", "autumn release", "-autumn", "autumn--release", "autumn/release"} {
		t.Run(slug, func(t *testing.T) {
			if bench.ValidColumnSlug(slug) {
				t.Fatalf("%q is a slug the grammar admits, so this case asserts nothing about a malformed one", slug)
			}
			h := newHarness(t)
			before := h.workstreamDirs()
			response := h.library.NewWorkstream(&Request{Verb: "workstream", Action: "new", Actor: "alka", Workstream: "Autumn release", Slug: slug})
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.Malformed {
				t.Fatalf("creation with the slug %q: %s %s, wanted a refusal of %s", slug, response.Outcome, response.Refusal, contract.Malformed)
			}
			if response.Detail != bench.SlugField {
				t.Errorf("the refusal names %q, wanted the argument %q", response.Detail, bench.SlugField)
			}
			h.reopen()
			if after := h.workstreamDirs(); len(after) != len(before) {
				t.Errorf("the refused creation left the collection reading %v, wanted the %v it found", after, before)
			}
			if got := h.library.Bench.Workstreams(); len(got) != 0 {
				t.Errorf("the workbench carries %d workstreams, wanted none", len(got))
			}
		})
	}
}

// TestASecondCreationOnATakenSlugIsRefusedAndWritesNothing asserts the column
// the explicit path does not share with the derived one. A title-derived slug
// that collides counts upward, because nobody typed it; an explicit slug that
// collides is refused, because somebody did.
//
// Three things are asserted not to have happened, and each is a different way
// the refusal could still have cost something: no second directory in the
// collection, no second event on the surviving journal, and no creation ordinal
// consumed, which the next creation's own ordinal is what reads.
func TestASecondCreationOnATakenSlugIsRefusedAndWritesNothing(t *testing.T) {
	h := newHarness(t)
	first := h.newWorkstreamNamingItsSlug("Autumn release", "autumn")
	before := h.workstreamDirs()

	response := h.library.NewWorkstream(&Request{Verb: "workstream", Action: "new", Actor: "alka", Workstream: "Autumn release", Slug: "autumn"})
	if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.WorkstreamSlugTaken {
		t.Fatalf("the repeated creation: %s %s, wanted a refusal of %s rather than a counting suffix", response.Outcome, response.Refusal, contract.WorkstreamSlugTaken)
	}
	if response.Detail != "autumn" {
		t.Errorf("the refusal names %q, wanted the slug the caller supplied", response.Detail)
	}
	h.reopen()
	if after := h.workstreamDirs(); len(after) != len(before) {
		t.Errorf("the refused creation left the collection reading %v, wanted the %v it found", after, before)
	}
	live := h.library.Bench.Workstreams()
	if len(live) != 1 || live[0].ID != first.ID || live[0].Slug != "autumn" {
		t.Fatalf("the workbench carries %d workstreams, wanted the one born under autumn alone", len(live))
	}
	if events := h.workstreamEvents(first.ID); len(events) != 1 {
		t.Errorf("the surviving workstream's journal carries %d events, wanted the one created event the refusal did not add to", len(events))
	}
	next := h.newWorkstreamNamingItsSlug("Winter release", "winter")
	stored := h.library.Bench.Workstream(next.ID)
	if stored.Ordinal != 2 {
		t.Errorf("the next workstream created carries the ordinal %d, wanted 2, so the refused creation consumed one", stored.Ordinal)
	}
}

// TestCreatingAWorkstreamDoesNotLetTheCreatorWriteItsFields asserts the
// boundary this card leaves exactly where it found it. Field writes on a
// workstream are the operator's, and having created one moments earlier buys
// the creator nothing, on any of the three fields the entity carries.
//
// The creator is the interesting actor rather than an arbitrary stranger. The
// gap this card was filed over is that a non-operator can create a workstream
// and then cannot finish setting it up, and the fix widens creation's inputs
// rather than widening who may write, so a change that closed the gap the other
// way turns this test red.
func TestCreatingAWorkstreamDoesNotLetTheCreatorWriteItsFields(t *testing.T) {
	h := newHarness(t)
	if h.library.Bench.Operator != "alka" {
		t.Fatalf("the harness operator is %q, and bob below is a non-operator only while it is alka", h.library.Bench.Operator)
	}
	created := h.library.NewWorkstream(&Request{Verb: "workstream", Action: "new", Actor: "bob", Workstream: "Autumn release", Slug: "autumn"})
	if created.Outcome != contract.OutcomeOK {
		t.Fatalf("bob creating a workstream: %s %s, wanted it accepted, since creation asks for an owner rather than for the operator", created.Outcome, created.Refusal)
	}
	h.reopen()
	stored := h.library.Bench.Workstream(created.Workstream.ID)
	before, err := bench.ReadText(stored.AnchorPath())
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	for _, c := range []struct{ field, value string }{
		{"title", "Autumn 2025 release"},
		{bench.SlugField, "autumn-2025"},
		{"status", "finished"},
	} {
		t.Run(c.field, func(t *testing.T) {
			response := h.library.SetWorkstream(&Request{Verb: "workstream", Action: "set", Actor: "bob", Workstream: "autumn", Field: c.field, Value: c.value, Confirm: true})
			h.reopen()
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotOperator {
				t.Fatalf("bob writing %s on the workstream he created: %s %s, wanted a refusal of %s", c.field, response.Outcome, response.Refusal, contract.NotOperator)
			}
			if response.Detail != "bob" {
				t.Errorf("the refusal names %q, wanted the actor it turned away", response.Detail)
			}
			after, err := bench.ReadText(stored.AnchorPath())
			if err != nil {
				t.Fatalf("read the anchor again: %v", err)
			}
			if after != before {
				t.Errorf("the refused write on %s rewrote the anchor:\n%q\n%q", c.field, before, after)
			}
		})
	}
}
