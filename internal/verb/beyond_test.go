package verb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/msg"
)

// TestAddFilesACard asserts the creation rules: the first state, substate
// ready, a journal opened with the created event, a title-less request
// refused malformed (CORE-CARD-3), a state the bench does not declare refused
// unknown-state (CORE-CARD-4), and the asymmetric capacity treatment of the
// first state against a named one.
func TestAddFilesACard(t *testing.T) {
	h := newHarness(t)
	ref := h.add("first card")
	card := h.card(ref)
	if card.State != intake {
		t.Errorf("wanted the first state %s, got %s", intake, card.State)
	}
	if card.Substate != contract.SubstateReady {
		t.Errorf("wanted substate ready, got %s", card.Substate)
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
	if response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "x", State: "nowhere"}); response.Refusal != contract.UnknownState {
		t.Errorf("an unknown state: wanted unknown-state, got %s %s", response.Outcome, response.Refusal)
	}

	// Doing is limited to one card. A filing into it is refused once it is
	// full, while a filing into the first state is admitted whatever.
	full := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "occupant", State: doing})
	if full.Outcome != contract.OutcomeOK {
		t.Fatalf("first filing into Doing: %s %s", full.Outcome, full.Refusal)
	}
	h.reopen()
	if response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "second", State: doing}); response.Refusal != contract.AtCapacity {
		t.Errorf("a filing into a full state: wanted at-capacity, got %s %s", response.Outcome, response.Refusal)
	} else if response.Detail != "doing" {
		// Named by the slug a caller could type back ("doing"), not by the
		// raw identifier behind it: dinah-29 cycle 2 fixed this for the
		// legal-moves listing and missed this refusal path.
		t.Errorf("at-capacity refusal: wanted the slug %q, got %q", "doing", response.Detail)
	}
	h.reopen()
	for i := 0; i < 3; i++ {
		if response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "intake filing"}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("a filing into the first state should never be refused, got %s", response.Refusal)
		}
		h.reopen()
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
// confirmation, that a state cards occupy is refused, and that deleting a
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
		t.Fatalf("archiving an occupied state: wanted %s, got %s %s", contract.Occupied, response.Outcome, response.Refusal)
	}
	if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: intake, Confirm: true}); response.Refusal != contract.Occupied {
		t.Fatalf("deleting an occupied state: wanted %s, got %s %s", contract.Occupied, response.Outcome, response.Refusal)
	}

	// The archived card leaves the listings, the offers and the count.
	archivable := h.add("archivable")
	h.mustDo(&Request{Verb: Move, Card: archivable, Actor: "alka", State: doing})
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: archivable}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	listing, err := h.library.List(&Request{Verb: "ls", State: doing})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listing.Cards) != 0 {
		t.Errorf("an archived card should leave the listing, got %d cards", len(listing.Cards))
	}
	// Doing is limited to one card, so an admitted move proves the archived
	// card left the count of CORE-MOVE-5 as well.
	if response := h.do(&Request{Verb: Move, Card: occupant, Actor: "alka", State: doing}); response.Outcome != contract.OutcomeOK {
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

// TestArchiveRemovesTheStateFromTheDefinition asserts AC-1: archiving an
// unoccupied state that is not the sole remaining one drops the identifier
// from workbench.md's states list in the same run, and every command that
// opens the bench afterwards succeeds and no longer lists it.
func TestArchiveRemovesTheStateFromTheDefinition(t *testing.T) {
	h := newHarness(t)
	before := len(h.library.Bench.States)
	response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: aftercare})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if got := len(h.library.Bench.States); got != before-1 {
		t.Fatalf("wanted %d states after the archive, got %d", before-1, got)
	}
	if h.library.Bench.State(aftercare) != nil {
		t.Error("the archived state is still declared")
	}
	if findings := h.check(); len(findings) != 0 {
		t.Errorf("a workbench with no dangling entry should check clean, got %+v", findings)
	}
}

// TestDeleteRemovesTheStateFromTheDefinition asserts AC-2: deleting an
// unoccupied state behaves identically to archiving it for the purposes of
// AC-1.
func TestDeleteRemovesTheStateFromTheDefinition(t *testing.T) {
	h := newHarness(t)
	before := len(h.library.Bench.States)
	response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: review, Confirm: true})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("delete: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if got := len(h.library.Bench.States); got != before-1 {
		t.Fatalf("wanted %d states after the delete, got %d", before-1, got)
	}
	if h.library.Bench.State(review) != nil {
		t.Error("the deleted state is still declared")
	}
	if findings := h.check(); len(findings) != 0 {
		t.Errorf("a workbench with no dangling entry should check clean, got %+v", findings)
	}
}

// TestRetiringTheLastStateIsRefused asserts AC-3: archiving or deleting the
// sole remaining state is refused with dinah.last-state, both as the third
// precondition row of archive and the fourth of delete, and the workbench
// keeps opening and working normally afterwards.
func TestRetiringTheLastStateIsRefused(t *testing.T) {
	if got := Checks("archive"); len(got) != 3 || got[2].Refusal != contract.LastState {
		t.Fatalf("archive's preconditions: wanted dinah.last-state third, got %+v", got)
	}
	if got := Checks("delete"); len(got) != 4 || got[3].Refusal != contract.LastState {
		t.Fatalf("delete's preconditions: wanted dinah.last-state fourth, got %+v", got)
	}

	h := newHarness(t)
	for _, id := range []string{doing, review, finished, aftercare} {
		response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: id})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("archive %s: %s %s", id, response.Outcome, response.Refusal)
		}
		h.reopen()
	}
	if len(h.library.Bench.States) != 1 {
		t.Fatalf("wanted one state left, got %d", len(h.library.Bench.States))
	}
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: intake}); response.Refusal != contract.LastState {
		t.Fatalf("archiving the last state: wanted %s, got %s %s", contract.LastState, response.Outcome, response.Refusal)
	} else if response.Detail != "intake" {
		// Named by its slug, not by its raw identifier: a caller who typed
		// "intake" is told about "intake", never about a12300000001.
		t.Fatalf("last-state refusal: wanted the slug %q, got %q", "intake", response.Detail)
	}
	h.reopen()
	if h.library.Bench.State(intake) == nil {
		t.Fatal("the refused archive removed the last state anyway")
	}
	if findings := h.check(); len(findings) != 0 {
		t.Errorf("a workbench holding its last state should check clean, got %+v", findings)
	}
	if response := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: intake, Confirm: true}); response.Refusal != contract.LastState {
		t.Fatalf("deleting the last state: wanted %s, got %s %s", contract.LastState, response.Outcome, response.Refusal)
	} else if response.Detail != "intake" {
		t.Fatalf("last-state refusal: wanted the slug %q, got %q", "intake", response.Detail)
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
	for _, member := range []string{"profile", "title", "states"} {
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
	if len(definition.States) != len(h.library.Bench.States) {
		t.Fatalf("wanted %d states, got %d", len(h.library.Bench.States), len(definition.States))
	}
	for i, element := range definition.States {
		var id string
		if err := json.Unmarshal(element["id"], &id); err != nil {
			t.Fatalf("state id: %v", err)
		}
		if id != h.library.Bench.States[i].ID {
			t.Errorf("CORE-JSON-4: position %d wanted %s, got %s", i, h.library.Bench.States[i].ID, id)
		}
	}

	second := filepath.Join(t.TempDir(), "again")
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
	ref := h.add("extended")
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
	ref := h.add("work that stays behind")
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

	extracted, err := bench.Open(target)
	if err != nil {
		t.Fatalf("open the extracted definition: %v", err)
	}
	if len(extracted.States) != len(h.library.Bench.States) {
		t.Fatalf("wanted %d states, got %d", len(h.library.Bench.States), len(extracted.States))
	}
	for i, state := range extracted.States {
		original := h.library.Bench.States[i]
		if state.ID != original.ID || state.Title != original.Title || state.Kind != original.Kind {
			t.Errorf("state %d: wanted %s/%s/%s, got %s/%s/%s", i,
				original.ID, original.Title, original.Kind, state.ID, state.Title, state.Kind)
		}
		if state.Capacity != original.Capacity || state.OperatorOwned != original.OperatorOwned {
			t.Errorf("state %d: capacity or operator flag differs", i)
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
// the coverage report names every shipped catalog.
func TestVersionCarriesTheConformanceClaim(t *testing.T) {
	release := Version(true)
	if release.Profile != "dinah-core/1.0" {
		t.Errorf("conformance claim: wanted dinah-core/1.0, got %s", release.Profile)
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
	wanted := map[string]bool{"en": true, "hi": true, "de": true, "cs": true, "id": true, "es": true, "fil": true, "af": true}
	for _, coverage := range release.Catalogs {
		delete(wanted, coverage.Tag)
		if coverage.Present != coverage.Total {
			t.Errorf("%s: wanted every key present, got %d of %d", coverage.Tag, coverage.Present, coverage.Total)
		}
		complete := coverage.Tag == "en" || coverage.Tag == "hi"
		if complete && coverage.Translated != coverage.Total {
			t.Errorf("%s ships complete, got %d of %d translated", coverage.Tag, coverage.Translated, coverage.Total)
		}
		if !complete && coverage.Translated != 0 {
			t.Errorf("%s ships as a skeleton, got %d translated", coverage.Tag, coverage.Translated)
		}
	}
	if len(wanted) > 0 {
		t.Errorf("catalogs missing from the report: %v", wanted)
	}
}

// TestUnsupportedVersionsAreRefusedLoudly asserts CORE-BENCH-4 and the
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
	ref := h.add("ordinary work")
	h.mustDo(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", State: doing})
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
	ref := h.add("renamed workbench")
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
			kind:    "state",
			ref:     intake,
			lockDir: h.root,
			owner:   filepath.Join(h.root, bench.StatesDir, intake),
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
		offers, err := h.library.Next(&Request{Verb: "next", State: intake})
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
		reading := []any{listing, offers, status.States, string(exported), clean, identifiers}
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
// that both names the closed event set gained here render for a reader in each
// of the two complete catalogs.
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

	state := hashDirectory(t, h.root)
	second, err := h.finish()
	if err != nil {
		t.Fatalf("a second finish: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("a second finish reported %+v", second)
	}
	if after := hashDirectory(t, h.root); after != state {
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
