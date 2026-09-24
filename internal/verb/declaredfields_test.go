package verb

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// fixtureDeclaration is the fields block every case in this file starts from:
// two keys on a card alone, one on the workbench and a column, one on a
// workstream alone, and one on the three default kinds because it names no
// kinds at all.
const fixtureDeclaration = `fields:
  git.branch:
    type: string
    meaning: the branch the card's code lives on
    on: [card]
  git.trunk:
    type: string
    meaning: the branch cards merge into
    on: [workbench, column]
  git.pr:
    type: url
    meaning: the request that lands the card
    on: [card]
  venue.deposit-paid:
    type: date
    meaning: the day the deposit fell due
  trip.destination:
    type: string
    meaning: the city the trip is to
    on: [workstream]
`

// declareFields writes a fields block into the workbench anchor, which is the
// only way one arrives: no verb declares a field, and the block is frontmatter
// a person edits. It is declareEvidence's shape, aimed at the other block.
func (h *harness) declareFields(block string) {
	h.t.Helper()
	path := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		h.t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.FieldsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		h.t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()
}

// declaringHarness is a fixture whose workbench declares the five fields
// above, which is where every case below starts.
func declaringHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.declareFields(fixtureDeclaration)
	return h
}

// set writes one field through the library and answers the response, without
// deciding whether the response is the one the case wanted.
func (h *harness) set(ref, field, value string) *Response {
	h.t.Helper()
	response := h.library.SetField(&Request{Verb: "set", Actor: "alka", Ref: ref, Field: field, Value: value})
	h.reopen()
	return response
}

// cardView answers the view a read of one card composes, which is the surface
// every response carrying a card draws from.
func (h *harness) cardView(ref string) *CardView {
	h.t.Helper()
	view, err := h.library.view(h.card(ref))
	if err != nil {
		h.t.Fatalf("view %s: %v", ref, err)
	}
	return view
}

// cardAnchorText reads a card's anchor as it stands, which is what a case
// asserting where a value landed compares against.
func (h *harness) cardAnchorText(ref string) string {
	h.t.Helper()
	text, err := bench.ReadText(filepath.Join(h.root, bench.CardsDir, h.cardID(ref), bench.CardAnchor))
	if err != nil {
		h.t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	return text
}

// TestAWriteReachesADeclaredKeyAndIsRefusedAnUndeclaredOne drives
// CORE-FIELD-6. A write under a declared key answers ok, the anchor carries
// the key inside the value block, and a read answers the value back; a write
// under a key the workbench does not declare is refused by name and the anchor
// gains no key anywhere.
func TestAWriteReachesADeclaredKeyAndIsRefusedAnUndeclaredOne(t *testing.T) {
	h := declaringHarness(t)
	ref := h.ready("A card")
	if response := h.set(ref, "git.branch", "dinah-498-declared-fields"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the declared write answered %s %s", response.Outcome, response.Refusal)
	}
	anchor := h.cardAnchorText(ref)
	if !strings.Contains(anchor, "field_values:\n  git.branch: dinah-498-declared-fields\n") {
		t.Errorf("the anchor does not carry the value inside the block:\n%s", anchor)
	}
	got, err := h.library.GetField(&Request{Verb: "get", Ref: ref, Field: "git.branch"})
	if err != nil {
		t.Fatalf("read it back: %v", err)
	}
	if got != "dinah-498-declared-fields" {
		t.Errorf("the value reads back as %q", got)
	}

	before := h.cardAnchorText(ref)
	refused := h.set(ref, "git.upstream", "origin")
	if refused.Outcome != contract.OutcomeRefused || refused.Refusal != contract.UndeclaredField {
		t.Fatalf("the undeclared write answered %s %s", refused.Outcome, refused.Refusal)
	}
	if refused.Detail != "git.upstream" {
		t.Errorf("the refusal names %q, wanted the key", refused.Detail)
	}
	if got := refused.Context["declared"]; !strings.Contains(got, "git.branch") || !strings.Contains(got, "git.pr") {
		t.Errorf("the refusal lists %q, wanted the keys declared on a card", got)
	}
	after := h.cardAnchorText(ref)
	if after != before {
		t.Errorf("the refused write changed the anchor:\n%s", after)
	}
	if strings.Contains(after, "git.upstream") {
		t.Errorf("the refused key reached the anchor:\n%s", after)
	}
}

// TestTheOnListRestrictsWhichKindsAKeyReaches drives CORE-FIELD-4 and
// CORE-FIELD-6. A key declared on a card alone reaches a card and is refused
// at a column and at the workbench, each refusal naming the kinds the
// declaration does list; a key declared on the workbench and a column reaches
// both and is refused at a card.
func TestTheOnListRestrictsWhichKindsAKeyReaches(t *testing.T) {
	h := declaringHarness(t)
	ref := h.ready("A card")
	cases := []struct {
		key   string
		on    string
		admit bool
		kinds string
	}{
		{"git.branch", ref, true, ""},
		{"git.branch", aftercareSlug, false, "card"},
		{"git.branch", "workbench", false, "card"},
		{"git.trunk", "workbench", true, ""},
		{"git.trunk", aftercareSlug, true, ""},
		{"git.trunk", ref, false, "workbench, column"},
	}
	admitted := 0
	for _, c := range cases {
		response := h.set(c.on, c.key, "main")
		if c.admit {
			if response.Outcome != contract.OutcomeOK {
				t.Errorf("%s on %s answered %s %s, wanted ok", c.key, c.on, response.Outcome, response.Refusal)
				continue
			}
			admitted++
			continue
		}
		if response.Refusal != contract.UndeclaredField {
			t.Errorf("%s on %s answered %s %s, wanted undeclared-field", c.key, c.on, response.Outcome, response.Refusal)
			continue
		}
		if got := response.Context["kinds"]; got != c.kinds {
			t.Errorf("%s on %s names the kinds %q, wanted %q", c.key, c.on, got, c.kinds)
		}
	}
	if admitted != 3 {
		t.Errorf("%d of the six writes were admitted, wanted three", admitted)
	}
}

// TestADeclaredValueIsRefusedWhenItDoesNotSuitItsType drives CORE-FIELD-5. The
// refusal is malformed and carries the key as its detail, and the accepting
// case beside it is what keeps this from passing against a build that refuses
// every value.
func TestADeclaredValueIsRefusedWhenItDoesNotSuitItsType(t *testing.T) {
	h := declaringHarness(t)
	ref := h.ready("A card")
	if response := h.set(ref, "git.pr", "https://example.com/x"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("a well-formed url answered %s %s", response.Outcome, response.Refusal)
	}
	refused := h.set(ref, "git.pr", "12")
	if refused.Refusal != contract.Malformed || refused.Detail != "git.pr" {
		t.Errorf("a value that is not a url answered %s %s %q", refused.Outcome, refused.Refusal, refused.Detail)
	}
	if got, err := h.library.GetField(&Request{Verb: "get", Ref: ref, Field: "git.pr"}); err != nil || got != "https://example.com/x" {
		t.Errorf("the refused write replaced the value: %q %v", got, err)
	}
}

// TestAnUndeclaredKeyIsPreservedOnReadAndIsNotSurfaced drives CORE-FIELD-7 and
// CORE-FIELD-8. A key nothing declares reads back, survives a write of its
// neighbour byte for byte, and is left out of the view; a name with no full
// stop is still refused, which pins the preserved case beside the refused one.
func TestAnUndeclaredKeyIsPreservedOnReadAndIsNotSurfaced(t *testing.T) {
	h := declaringHarness(t)
	ref := h.ready("A card")
	path := filepath.Join(h.root, bench.CardsDir, h.cardID(ref), bench.CardAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.FieldValuesKey, []string{"field_values:", "  imported.key: kept", "  git.branch: first"})
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the anchor: %v", err)
	}
	h.reopen()

	got, err := h.library.GetField(&Request{Verb: "get", Ref: ref, Field: "imported.key"})
	if err != nil {
		t.Fatalf("a read of an undeclared key refused: %v", err)
	}
	if got != "kept" {
		t.Errorf("the undeclared key reads back as %q", got)
	}
	if response := h.set(ref, "git.branch", "second"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the write of the neighbour answered %s %s", response.Outcome, response.Refusal)
	}
	if after := h.cardAnchorText(ref); !strings.Contains(after, "  imported.key: kept\n") {
		t.Errorf("the undeclared line did not survive:\n%s", after)
	}
	view := h.cardView(ref)
	if _, surfaced := view.Fields["imported.key"]; surfaced {
		t.Errorf("the view carries the undeclared key: %v", view.Fields)
	}
	if view.Fields["git.branch"] != "second" {
		t.Errorf("the view carries %v, wanted the declared value", view.Fields)
	}
	if _, err := h.library.GetField(&Request{Verb: "get", Ref: ref, Field: "nosuchfield"}); err == nil {
		t.Error("a name with no full stop was read rather than refused")
	}
}

// TestAColumnRequiringAFieldHoldsOnEntry drives CORE-FIELD-10 and
// CORE-FIELD-11. A card carrying no value is refused by name on a move into a
// column that requires one, the same card moves in once the value is written,
// the operator carries the unset card in with an override and the moved event
// records it, and a non-operator carrying the same marker is still refused.
func TestAColumnRequiringAFieldHoldsOnEntry(t *testing.T) {
	h := declaringHarness(t)
	h.declare(finished, bench.RequireFieldsKey, "[git.branch]")
	ref := h.ready("A card")

	refused := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
	if refused.Refusal != contract.MissingField || refused.Detail != "git.branch" {
		t.Fatalf("the move answered %s %s %q", refused.Outcome, refused.Refusal, refused.Detail)
	}
	if got := refused.Context[contract.ValueColumn]; got == "" {
		t.Error("the refusal names no column, so a reader cannot tell which station asked")
	}
	h.reopen()
	if got := h.card(ref).Column; got != aftercare {
		t.Errorf("the refused move carried the card to %s", got)
	}

	if response := h.set(ref, "git.branch", "dinah-498-declared-fields"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the write answered %s %s", response.Outcome, response.Refusal)
	}
	moved := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
	if moved.Outcome != contract.OutcomeOK {
		t.Fatalf("the move after the write answered %s %s", moved.Outcome, moved.Refusal)
	}
	h.reopen()

	// The override half runs on a second card, so the accepting case above is
	// not what the override is read against.
	other := h.ready("Another card")
	stranger := h.library.Do(&Request{Verb: Move, Card: other, Actor: "bo", Column: finished, Override: true})
	if stranger.Refusal != contract.NotOperator {
		t.Errorf("a non-operator carrying the marker answered %s %s", stranger.Outcome, stranger.Refusal)
	}
	h.reopen()
	carried := h.library.Do(&Request{Verb: Move, Card: other, Actor: "alka", Column: finished, Override: true})
	if carried.Outcome != contract.OutcomeOK {
		t.Fatalf("the operator's override answered %s %s", carried.Outcome, carried.Refusal)
	}
	h.reopen()
	events := h.events(other)
	last := events[len(events)-1]
	if last.Event != contract.EventMoved || !last.Override {
		t.Errorf("the moved event records override %v", last.Override)
	}
}

// TestARequirementNamingAnUndeclaredKeyHoldsNothing drives CORE-FIELD-10. A
// column requiring a key the workbench does not declare admits a card that
// carries no value for it, because a key nothing can ever be written under
// would make the column unreachable.
func TestARequirementNamingAnUndeclaredKeyHoldsNothing(t *testing.T) {
	h := declaringHarness(t)
	h.declare(finished, bench.RequireFieldsKey, "[git.nothing]")
	ref := h.ready("A card")
	moved := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
	if moved.Outcome != contract.OutcomeOK {
		t.Errorf("a requirement naming an undeclared key held the card: %s %s", moved.Outcome, moved.Refusal)
	}
}

// TestAPullIsRefusedOnTheSameGroundAsAMove drives CORE-FIELD-11 at the other
// verb. canLand serves both, so the row fires on a pull whether or not
// anybody writes it down, and the published pull list names it where canLand
// runs it.
func TestAPullIsRefusedOnTheSameGroundAsAMove(t *testing.T) {
	h := declaringHarness(t)
	// The requirement rides on the review station rather than on a done
	// column, because a pull into a done column is refused before this row is
	// reached and the case would be asserting the wrong refusal.
	h.declare(review, bench.RequireFieldsKey, "[git.branch]")
	ref := h.readyAt("A card", doing)
	refused := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: review})
	if refused.Refusal != contract.MissingField {
		t.Fatalf("the pull answered %s %s", refused.Outcome, refused.Refusal)
	}
	h.reopen()
	if response := h.set(ref, "git.branch", "dinah-498-declared-fields"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the write answered %s %s", response.Outcome, response.Refusal)
	}
	pulled := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: review})
	if pulled.Outcome != contract.OutcomeOK {
		t.Fatalf("the pull after the write answered %s %s", pulled.Outcome, pulled.Refusal)
	}
}

// TestAMoveFailingBothTheHoldAndTheRequirementAnswersTheHold pins the order of
// the two entry rows rather than only their numbering. The hold runs first in
// canLand, so a card failing both is told about the item.
func TestAMoveFailingBothTheHoldAndTheRequirementAnswersTheHold(t *testing.T) {
	h := declaringHarness(t)
	h.declare(finished, bench.RequireFieldsKey, "[git.branch]")
	h.declare(finished, bench.GateItemsKey, "true")
	ref := h.ready("A card")
	item := h.file(ref, "open_question", "Who signs this off?")
	if response := h.set(item, bench.ItemColumnField, finished); response.Outcome != contract.OutcomeOK {
		t.Fatalf("naming the column on the item answered %s %s", response.Outcome, response.Refusal)
	}
	refused := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
	if refused.Refusal != contract.UnresolvedItem {
		t.Errorf("a card failing both rows answered %s, wanted unresolved-item", refused.Refusal)
	}
}

// TestTheThreeUnreachableKindsRefuseEveryKey drives CORE-FIELD-6. A
// declaration reaches a card, a column, the workbench and, since dinah-582, a
// workstream, so a write of a declared key to a comment, a checklist item or
// an attachment is refused whatever the declaration says. The successful write
// beside them is what keeps this from passing against a build that refuses
// every kind.
//
// The workstream left this list on dinah-582 and is exercised by
// TestADeclaredKeyOnAWorkstreamIsAnyOwnersToWrite below, which asserts the
// write that used to be refused here.
func TestTheThreeUnreachableKindsRefuseEveryKey(t *testing.T) {
	h := declaringHarness(t)
	ref := h.ready("A card")
	h.comment(ref, "A comment.")
	h.attach(ref, "note.txt", "some bytes")
	item := h.file(ref, "decision", "A decision.")
	unreachable := []string{ref + "/comments/1", item, ref + "/attachments/1"}
	for _, target := range unreachable {
		for _, key := range []string{"venue.deposit-paid", "git.branch", "trip.destination"} {
			response := h.set(target, key, "2026-09-14")
			if response.Refusal != contract.UndeclaredField {
				t.Errorf("a write of %s to %s answered %s %s, wanted undeclared-field", key, target, response.Outcome, response.Refusal)
			}
		}
	}
	if len(unreachable) != 3 {
		t.Errorf("the case list covers %d kinds, wanted three", len(unreachable))
	}
	if response := h.set(ref, "venue.deposit-paid", "2026-09-14"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the reachable write answered %s %s", response.Outcome, response.Refusal)
	}
}

// TestADeclaredKeyOnAWorkstreamIsAnyOwnersToWrite is dinah-582's own case at
// the library. A key declared on a workstream alone is written there by an
// actor who is not the operator, the value lands in the anchor's field_values
// block, and the workstream's own journal records it.
//
// Four writes run in order and each asserts a different rule. The first
// journals one workstream_updated line carrying the key, an empty from and the
// written value. The second repeats the value, leaves the anchor
// byte-identical and journals nothing. The third carries a different value and
// journals a line whose from is what stood there. The fourth clears the key
// and journals a line whose to is empty. None of them carries a note, because
// a workstream's event lands on its own journal rather than on somebody
// else's.
func TestADeclaredKeyOnAWorkstreamIsAnyOwnersToWrite(t *testing.T) {
	h := declaringHarness(t)
	if h.library.Bench.Operator != "alka" {
		t.Fatalf("the harness operator is %q, and bob below is a non-operator only while it is alka", h.library.Bench.Operator)
	}
	workstream := h.newWorkstream("Berlin trip")
	id := workstream.ID
	anchor := func() string {
		h.t.Helper()
		stored := h.library.Bench.Workstream(id)
		if stored == nil {
			h.t.Fatalf("the workbench carries no workstream %s", id)
		}
		text, err := bench.ReadText(stored.AnchorPath())
		if err != nil {
			h.t.Fatalf("read the workstream anchor: %v", err)
		}
		return text
	}
	write := func(value string) *Response {
		h.t.Helper()
		response := h.library.SetField(&Request{Verb: "set", Actor: "bob", Ref: workstream.Ref, Field: "trip.destination", Value: value})
		h.reopen()
		return response
	}
	before := len(h.workstreamEvents(id))

	if response := write("Berlin"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("bob writing a declared key on a workstream answered %s %s, wanted ok", response.Outcome, response.Refusal)
	}
	if got := anchor(); !strings.Contains(got, "field_values:") || !strings.Contains(got, "  trip.destination: Berlin\n") {
		t.Errorf("the value did not land in the anchor's field_values block:\n%s", got)
	}
	got, readErr := h.library.GetField(&Request{Verb: "get", Actor: "bob", Ref: workstream.Ref, Field: "trip.destination"})
	if readErr != nil {
		t.Fatalf("reading the key back: %v", readErr)
	}
	if got != "Berlin" {
		t.Errorf("a read answers %q, wanted Berlin", got)
	}
	events := h.workstreamEvents(id)
	if len(events) != before+1 {
		t.Fatalf("the workstream's journal carries %d events, wanted %d", len(events), before+1)
	}
	last := events[len(events)-1]
	if last.Event != contract.EventWorkstreamUpdated {
		t.Errorf("the event is %q, wanted %s", last.Event, contract.EventWorkstreamUpdated)
	}
	if last.Field != "trip.destination" || last.From != "" || last.To != "Berlin" {
		t.Errorf("the event carries field %q from %q to %q", last.Field, last.From, last.To)
	}
	if last.Note != "" {
		t.Errorf("the event carries the note %q, and a workstream's event lands on its own journal", last.Note)
	}

	// The same value again is accepted, silent on disk and silent in the
	// journal, which is writeField's standing rule.
	settled := anchor()
	if response := write("Berlin"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the repeated write answered %s %s", response.Outcome, response.Refusal)
	}
	if got := anchor(); got != settled {
		t.Errorf("the repeated write rewrote the anchor:\n%s", got)
	}
	if got := len(h.workstreamEvents(id)); got != before+1 {
		t.Errorf("the repeated write grew the journal to %d events", got)
	}

	// A different value journals the previous one as its from.
	if response := write("Hamburg"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the second value answered %s %s", response.Outcome, response.Refusal)
	}
	events = h.workstreamEvents(id)
	if len(events) != before+2 {
		t.Fatalf("the journal carries %d events, wanted %d", len(events), before+2)
	}
	if last = events[len(events)-1]; last.From != "Berlin" || last.To != "Hamburg" {
		t.Errorf("the second event carries from %q to %q", last.From, last.To)
	}

	// A clear takes the key out and journals an empty to.
	if response := write(""); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the clear answered %s %s", response.Outcome, response.Refusal)
	}
	if got := anchor(); strings.Contains(got, "trip.destination") {
		t.Errorf("the cleared key is still in the anchor:\n%s", got)
	}
	events = h.workstreamEvents(id)
	if len(events) != before+3 {
		t.Fatalf("the journal carries %d events, wanted %d", len(events), before+3)
	}
	if last = events[len(events)-1]; last.From != "Hamburg" || last.To != "" {
		t.Errorf("the clear's event carries from %q to %q", last.From, last.To)
	}
}

// TestADeclarationNamingOneKindRefusesTheOther pins dinah-582's two new
// refusing directions beside the two accepting ones. A key declared on a
// workstream alone is refused on a card and the refusal names the workstream;
// a key declared on a card alone is refused on a workstream and names the
// card. Each refusal stands beside the write the same key accepts, so a build
// refusing everything fails here.
func TestADeclarationNamingOneKindRefusesTheOther(t *testing.T) {
	h := declaringHarness(t)
	ref := h.ready("A card")
	workstream := h.newWorkstream("Berlin trip")

	if response := h.set(ref, "trip.destination", "Berlin"); response.Refusal != contract.UndeclaredField {
		t.Errorf("a workstream key written on a card answered %s %s, wanted undeclared-field", response.Outcome, response.Refusal)
	} else if got := response.Context["kinds"]; got != bench.KindWorkstream {
		t.Errorf("the refusal names the kinds %q, wanted %q", got, bench.KindWorkstream)
	}
	if response := h.set(workstream.Ref, "git.branch", "dinah-582"); response.Refusal != contract.UndeclaredField {
		t.Errorf("a card key written on a workstream answered %s %s, wanted undeclared-field", response.Outcome, response.Refusal)
	} else if got := response.Context["kinds"]; got != bench.KindCard {
		t.Errorf("the refusal names the kinds %q, wanted %q", got, bench.KindCard)
	}
	if response := h.set(workstream.Ref, "trip.destination", "Berlin"); response.Outcome != contract.OutcomeOK {
		t.Errorf("the accepting case on a workstream answered %s %s", response.Outcome, response.Refusal)
	}
	if response := h.set(ref, "git.branch", "dinah-582"); response.Outcome != contract.OutcomeOK {
		t.Errorf("the accepting case on a card answered %s %s", response.Outcome, response.Refusal)
	}
}

// TestAWriteOfTheValueAlreadyStoredChangesNothing asserts writeField's own
// no-op rule over a declared field. The second write answers ok, leaves the
// anchor byte-identical and appends no journal line; the third, carrying a
// different value, does append one, which pins the accepting case beside the
// silent one.
func TestAWriteOfTheValueAlreadyStoredChangesNothing(t *testing.T) {
	h := declaringHarness(t)
	ref := h.ready("A card")
	before := len(h.events(ref))
	if response := h.set(ref, "git.branch", "first"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the first write answered %s %s", response.Outcome, response.Refusal)
	}
	anchor := h.cardAnchorText(ref)
	if response := h.set(ref, "git.branch", "first"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the second write answered %s %s", response.Outcome, response.Refusal)
	}
	if after := h.cardAnchorText(ref); after != anchor {
		t.Errorf("the repeated write rewrote the anchor:\n%s", after)
	}
	if got := len(h.events(ref)); got != before+1 {
		t.Errorf("the journal grew to %d events, wanted %d", got, before+1)
	}
	if response := h.set(ref, "git.branch", "second"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the third write answered %s %s", response.Outcome, response.Refusal)
	}
	if got := len(h.events(ref)); got != before+2 {
		t.Errorf("a write of a different value left the journal at %d events", got)
	}
	last := h.events(ref)[before+1]
	if last.Field != "git.branch" || last.From != "first" || last.To != "second" {
		t.Errorf("the event records %+v", last)
	}
}

// TestTheServeCarriesTheDeclaredValues asserts what a reader sees. A
// successful claim and a successful move each answer with the card view
// carrying the values, a card holding none answers with no member at all, and
// the values are the declared keys and no others.
func TestTheServeCarriesTheDeclaredValues(t *testing.T) {
	h := declaringHarness(t)
	ref := h.ready("A card")
	if response := h.set(ref, "git.branch", "dinah-498-declared-fields"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the write answered %s %s", response.Outcome, response.Refusal)
	}
	claimed := h.library.Do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	if claimed.Outcome != contract.OutcomeOK {
		t.Fatalf("the claim answered %s %s", claimed.Outcome, claimed.Refusal)
	}
	want := map[string]string{"git.branch": "dinah-498-declared-fields"}
	if !reflect.DeepEqual(claimed.Card.Fields, want) {
		t.Errorf("the claim's card view carries %v, wanted %v", claimed.Card.Fields, want)
	}
	h.reopen()
	h.mustDo(&Request{Verb: Release, Card: ref, Actor: "alka"})
	h.reopen()
	moved := h.library.Do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
	if moved.Outcome != contract.OutcomeOK {
		t.Fatalf("the move answered %s %s", moved.Outcome, moved.Refusal)
	}
	if !reflect.DeepEqual(moved.Card.Fields, want) {
		t.Errorf("the move's card view carries %v, wanted %v", moved.Card.Fields, want)
	}
	h.reopen()
	bare := h.ready("A card carrying none")
	if view := h.cardView(bare); view.Fields != nil {
		t.Errorf("a card carrying no value answers with %v, wanted no member", view.Fields)
	}
}

// TestAColumnViewCarriesItsOwnValuesAndItsRequirement asserts the other read
// surface. A column's own declared values and its requirement both reach the
// view, and a column carrying neither answers with neither.
func TestAColumnViewCarriesItsOwnValuesAndItsRequirement(t *testing.T) {
	h := declaringHarness(t)
	h.declare(finished, bench.RequireFieldsKey, "[git.branch]")
	if response := h.set(aftercareSlug, "git.trunk", "main"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the column write answered %s %s", response.Outcome, response.Refusal)
	}
	views, err := h.library.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	seen := 0
	for _, view := range views {
		switch view.ID {
		case aftercare:
			seen++
			if view.Fields["git.trunk"] != "main" {
				t.Errorf("the aftercare column carries %v", view.Fields)
			}
			if len(view.RequireFields) != 0 {
				t.Errorf("a column declaring no requirement carries %v", view.RequireFields)
			}
		case finished:
			seen++
			if !reflect.DeepEqual(view.RequireFields, []string{"git.branch"}) {
				t.Errorf("the finished column requires %v", view.RequireFields)
			}
			if view.Fields != nil {
				t.Errorf("a column carrying no value answers with %v", view.Fields)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("the listing carried %d of the two columns this case reads", seen)
	}
}

// TestAWorkbenchValueNeverLandsInTheLayerNamespace drives CORE-FIELD-9. A
// value written on the workbench goes inside the value block and leaves no key
// of that name at the top level, and a dotted top-level key that is not a
// declared field survives a declared-field write untouched.
func TestAWorkbenchValueNeverLandsInTheLayerNamespace(t *testing.T) {
	h := declaringHarness(t)
	path := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set("acme.layer", "1.0")
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()
	if response := h.set("workbench", "git.trunk", "main"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the workbench write answered %s %s", response.Outcome, response.Refusal)
	}
	anchor := h.anchorBytes()
	if !strings.Contains(anchor, "field_values:\n  git.trunk: main\n") {
		t.Errorf("the value is not inside the block:\n%s", anchor)
	}
	for _, line := range bench.SplitLines(anchor) {
		if strings.HasPrefix(line, "git.trunk:") {
			t.Errorf("a value reached the top level of the anchor:\n%s", anchor)
		}
	}
	if !strings.Contains(anchor, "\nacme.layer: 1.0\n") {
		t.Errorf("the hand-written layer declaration did not survive:\n%s", anchor)
	}
}

// TestARecordCarriesItsDeclaredValuesAfterItsBuiltInRows is dinah-582's read
// side, at both kinds that answer a record. A workstream's record carries its
// four built-in rows and then one row per declared key it holds a value for,
// in declaration order, and the workbench's record does the same after the
// rows the bare listing prints.
//
// The row count is asserted rather than the presence of one row. A record that
// drew every declared key, value or no value, satisfies a membership check and
// fails here, which is the point: a declared key holding nothing is left out
// while a built-in field holding nothing is drawn empty.
func TestARecordCarriesItsDeclaredValuesAfterItsBuiltInRows(t *testing.T) {
	h := declaringHarness(t)
	workstream := h.newWorkstream("Berlin trip")

	builtIn := bench.FieldsOf(bench.KindWorkstream)
	bare := h.record(workstream.Ref)
	if len(bare.Fields) != len(builtIn) {
		t.Fatalf("a workstream holding no declared value draws %d rows, wanted the %d built-in ones: %v", len(bare.Fields), len(builtIn), bare.Fields)
	}

	if response := h.set(workstream.Ref, "trip.destination", "Berlin"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("writing the declared key answered %s %s", response.Outcome, response.Refusal)
	}
	got := h.record(workstream.Ref)
	if len(got.Fields) != len(builtIn)+1 {
		t.Fatalf("the workstream's record draws %d rows, wanted %d: %v", len(got.Fields), len(builtIn)+1, got.Fields)
	}
	for i, name := range builtIn {
		if got.Fields[i].Name != name {
			t.Errorf("row %d is %q, wanted the built-in field %q", i, got.Fields[i].Name, name)
		}
	}
	last := got.Fields[len(got.Fields)-1]
	if last.Name != "trip.destination" || last.Value != "Berlin" {
		t.Errorf("the declared row is %q = %q", last.Name, last.Value)
	}

	// The workbench answers the same shape. git.trunk and venue.deposit-paid
	// both reach it, so writing one and leaving the other empty is what pins
	// the omission rule rather than merely the append.
	listing := bench.WorkbenchListingFields
	if response := h.set("workbench", "git.trunk", "main"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("writing a declared key on the workbench answered %s %s", response.Outcome, response.Refusal)
	}
	bench2 := h.record("workbench")
	if len(bench2.Fields) != len(listing)+1 {
		t.Fatalf("the workbench's record draws %d rows, wanted %d: %v", len(bench2.Fields), len(listing)+1, bench2.Fields)
	}
	last = bench2.Fields[len(bench2.Fields)-1]
	if last.Name != "git.trunk" || last.Value != "main" {
		t.Errorf("the workbench's declared row is %q = %q", last.Name, last.Value)
	}
	for _, row := range bench2.Fields {
		if row.Name == "venue.deposit-paid" {
			t.Errorf("a declared key holding no value was drawn: %q = %q", row.Name, row.Value)
		}
	}
}

// record answers the Record a show of one reference composes, failing the test
// if the read itself could not finish or answered something else.
func (h *harness) record(ref string) *Record {
	h.t.Helper()
	_, record, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		h.t.Fatalf("show %s: %v", ref, err)
	}
	if record == nil {
		h.t.Fatalf("show %s answered no record", ref)
	}
	return record
}

// enumeratedFieldDeclaration is fixtureDeclaration plus one string field
// carrying a `values` list, which is what TestAValuesListRefusesAnOutsideValue
// and TestClearingAnEnumeratedFieldIsUnaffectedByValues drive against.
const enumeratedFieldDeclaration = fixtureDeclaration + `  card.kind:
    type: string
    meaning: what kind of work this card is
    values: [bug, feature, chore]
`

// TestAValuesListRefusesAnOutsideValue drives CORE-FIELD-13 on the CLI write
// path. A value named in the field's `values` list succeeds, and one outside
// it is refused malformed with the legal list under legalValues in the
// refusal's context, the same shape a type mismatch's refusal already keeps.
func TestAValuesListRefusesAnOutsideValue(t *testing.T) {
	h := declaringHarness(t)
	h.declareFields(enumeratedFieldDeclaration)
	ref := h.ready("A card")
	if response := h.set(ref, "card.kind", "bug"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("a listed value answered %s %s", response.Outcome, response.Refusal)
	}
	refused := h.set(ref, "card.kind", "Bug")
	if refused.Refusal != contract.Malformed || refused.Detail != "card.kind" {
		t.Errorf("an unlisted value answered %s %s %q", refused.Outcome, refused.Refusal, refused.Detail)
	}
	if got := refused.Context["legalValues"]; got != "bug, feature, chore" {
		t.Errorf("the refusal's legalValues context is %q, wanted %q", got, "bug, feature, chore")
	}
	if got, err := h.library.GetField(&Request{Verb: "get", Ref: ref, Field: "card.kind"}); err != nil || got != "bug" {
		t.Errorf("the refused write replaced the stored value: %q %v", got, err)
	}
}

// TestClearingAnEnumeratedFieldIsUnaffectedByValues drives the "every declared
// field is clearable" rule against a field carrying a `values` list: setting
// a listed value and then clearing it both succeed, exactly as they do on a
// field declaring no `values` member.
func TestClearingAnEnumeratedFieldIsUnaffectedByValues(t *testing.T) {
	h := declaringHarness(t)
	h.declareFields(enumeratedFieldDeclaration)
	ref := h.ready("A card")
	if response := h.set(ref, "card.kind", "chore"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("setting a listed value answered %s %s", response.Outcome, response.Refusal)
	}
	if response := h.set(ref, "card.kind", ""); response.Outcome != contract.OutcomeOK {
		t.Fatalf("clearing the field answered %s %s", response.Outcome, response.Refusal)
	}
	if got, err := h.library.GetField(&Request{Verb: "get", Ref: ref, Field: "card.kind"}); err != nil || got != "" {
		t.Errorf("the cleared field still reads %q, err %v", got, err)
	}
}
