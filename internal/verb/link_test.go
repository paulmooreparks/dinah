package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// anchorText reads a card's own card.md back as bytes, which is how the tests
// below assert what landed on disk rather than what the response said. A
// criterion that reads the response is a criterion the response can satisfy on
// its own, and what these verbs are for is the file.
func anchorText(t *testing.T, h *harness, ref string) string {
	t.Helper()
	body, err := os.ReadFile(h.card(ref).AnchorPath())
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	return string(body)
}

// journalEvents reads the event names a card's own journal carries, in order.
func journalEvents(t *testing.T, h *harness, ref string) []string {
	t.Helper()
	body, err := os.ReadFile(h.card(ref).JournalPath())
	if err != nil {
		t.Fatalf("read the journal of %s: %v", ref, err)
	}
	var names []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event struct {
			Event string `json:"event"`
			Kind  string `json:"kind"`
			To    string `json:"to"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("the journal of %s carries an unreadable line %q: %v", ref, line, err)
		}
		names = append(names, event.Event)
	}
	return names
}

// countEvents answers how many lines of one event name a card's journal
// carries.
func countEvents(t *testing.T, h *harness, ref, want string) int {
	t.Helper()
	found := 0
	for _, name := range journalEvents(t, h, ref) {
		if name == want {
			found++
		}
	}
	return found
}

// mustLink writes one link and fails the test unless it was written.
func mustLink(t *testing.T, h *harness, card, kind, to string) *Response {
	t.Helper()
	response := h.library.Link(&Request{Verb: "link", Actor: "alka", Card: card, Kind: kind, LinkTo: to})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("link %s %s %s: %s %s", card, kind, to, response.Outcome, response.Refusal)
	}
	h.reopen()
	return response
}

// TestLinkWritesOneEntryToTheCardsOwnAnchor drives CORE-LINK-1, which permits
// a card to carry links, and CORE-LINK-2, which requires every link to carry a
// kind and the identifier of the card it names.
//
// The assertion is on the raw frontmatter rather than on Card.Links, because
// the reader would answer a well-formed slice out of a block whose stored to:
// held the reference the caller typed, and storing the reference is the defect
// this test exists to catch.
func TestLinkWritesOneEntryToTheCardsOwnAnchor(t *testing.T) {
	h := newHarness(t)
	source := h.add("the source")
	target := h.add("the target")
	targetID := h.card(target).ID

	mustLink(t, h, source, "duplicates", target)

	anchor := anchorText(t, h, source)
	want := "links:\n  - kind: duplicates\n    to: " + targetID + "\n"
	if !strings.Contains(anchor, want) {
		t.Errorf("the anchor of %s carries no links block of the shape\n%s\ngot:\n%s", source, want, anchor)
	}
	// The reference the caller typed is not what is stored, so a to: holding
	// it is the failure this half names. The reference is a slug and a number
	// and cannot be a 12-hex identifier, so finding it in the block is
	// unambiguous.
	if strings.Contains(anchor, "to: "+target) {
		t.Errorf("the anchor of %s stored the reference %q under to: rather than the identifier %q", source, target, targetID)
	}
	links := h.card(source).Links
	if len(links) != 1 || links[0].Kind != "duplicates" || links[0].To != targetID {
		t.Errorf("wanted one duplicates link to %s, got %+v", targetID, links)
	}
	if got := countEvents(t, h, source, contract.EventLinked); got != 1 {
		t.Errorf("wanted one linked event on the card's own journal, got %d", got)
	}
}

// TestLinkResolvesATargetInEitherHalfOfTheCollection asserts that a link may
// name an archived card, which is the one write-time reference in this package
// allowed to resolve outside the live half, and that a bare identifier
// resolves the same way a reference does.
//
// It drives CORE-LINK-2, whose requirement is that what a link carries is the
// identifier of the card it names, and the identifier space the format fixes
// spans both halves of the collection.
func TestLinkResolvesATargetInEitherHalfOfTheCollection(t *testing.T) {
	h := newHarness(t)
	source := h.add("the source")
	archived := h.add("the archived target")
	archivedID := h.card(archived).ID
	byID := h.add("named by identifier")
	byIDIdentifier := h.card(byID).ID

	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: archived}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	// The archived card resolves by its human reference, which the live half
	// no longer answers, so this is the archive route rather than the live one.
	if _, err := h.library.Bench.ResolveCard(archived); err == nil {
		t.Fatalf("the live half still resolves %s, so this case is not testing the archive route", archived)
	}
	mustLink(t, h, source, "supersedes", archived)
	mustLink(t, h, source, "relates", byIDIdentifier)

	anchor := anchorText(t, h, source)
	for _, want := range []string{"to: " + archivedID, "to: " + byIDIdentifier} {
		if !strings.Contains(anchor, want) {
			t.Errorf("the anchor of %s carries no %q, got:\n%s", source, want, anchor)
		}
	}
}

// TestLinkRefusesATargetTheWorkbenchDoesNotCarry drives CORE-LINK-3, which
// requires a tool to refuse a link naming a card its workbench does not carry
// and to report unknown-card. Both spellings a caller may type are driven,
// since a bare identifier is checked for presence and a reference is resolved,
// and the two take different routes through the resolver.
func TestLinkRefusesATargetTheWorkbenchDoesNotCarry(t *testing.T) {
	h := newHarness(t)
	source := h.add("the source")
	before := anchorText(t, h, source)

	for _, absent := range []string{"fx-99", "000000000000"} {
		response := h.library.Link(&Request{Verb: "link", Actor: "alka", Card: source, Kind: "blocks", LinkTo: absent})
		if response.Refusal != contract.UnknownCard {
			t.Errorf("link to %q: wanted %s, got %s %s", absent, contract.UnknownCard, response.Outcome, response.Refusal)
		}
		if response.Detail != absent {
			t.Errorf("link to %q: the refusal names %q rather than what was typed", absent, response.Detail)
		}
		h.reopen()
		if after := anchorText(t, h, source); after != before {
			t.Errorf("link to %q rewrote the anchor of %s:\n%s", absent, source, after)
		}
	}
}

// TestLinkTakesAnyKindAndCanonicalisesNothing drives CORE-LINK-4, the
// prohibition on restricting a link's kind to a closed set, which is the one
// property of this card most likely to erode: a closed set catches typos and
// makes the help text finite, and every kind this project uses today would fit
// inside one.
//
// The kinds below are chosen to be words no part of this codebase knows. Each
// has to be accepted and each has to read back exactly as it was given, so a
// set that admitted them and a canonicaliser that rewrote them both fail here.
func TestLinkTakesAnyKindAndCanonicalisesNothing(t *testing.T) {
	h := newHarness(t)
	source := h.add("the source")
	target := h.add("the target")
	targetID := h.card(target).ID

	// A renovation's vocabulary, a suggested spelling, and two shapes a
	// canonicaliser would be tempted to fold: mixed case and a space.
	kinds := []string{"tile-order-duplicates", "relates", "Blocks-Drywall", "came out of"}
	for _, kind := range kinds {
		mustLink(t, h, source, kind, target)
	}

	links := h.card(source).Links
	if len(links) != len(kinds) {
		t.Fatalf("wanted %d links, got %d: %+v", len(kinds), len(links), links)
	}
	for i, kind := range kinds {
		if links[i].Kind != kind {
			t.Errorf("link %d stored the kind %q rather than %q, so something rewrote what the caller typed", i, links[i].Kind, kind)
		}
		if links[i].To != targetID {
			t.Errorf("link %d names %q rather than %q", i, links[i].To, targetID)
		}
	}
	// Two of those name the same target under different kinds, and one card
	// naming itself is legal on the same grounds: nothing in the format gives
	// a ground to refuse either.
	sourceID := h.card(source).ID
	mustLink(t, h, source, "relates", source)
	self := h.card(source).Links
	if last := self[len(self)-1]; last.Kind != "relates" || last.To != sourceID {
		t.Errorf("wanted a self-link recorded as relates -> %s, got %+v", sourceID, last)
	}
}

// TestLinkWritesNothingToTheCardItNames drives CORE-LINK-6, the prohibition on
// adding a link to a card as a consequence of a link another card carries.
// The whole of the named card's anchor is compared, so a reverse entry, a
// touched timestamp or any other write shows up.
func TestLinkWritesNothingToTheCardItNames(t *testing.T) {
	h := newHarness(t)
	source := h.add("the source")
	target := h.add("the target")

	before := anchorText(t, h, target)
	beforeJournal := journalEvents(t, h, target)

	mustLink(t, h, source, "blocks", target)

	if after := anchorText(t, h, target); after != before {
		t.Errorf("linking to %s rewrote its anchor:\nbefore:\n%s\nafter:\n%s", target, before, after)
	}
	if after := journalEvents(t, h, target); len(after) != len(beforeJournal) {
		t.Errorf("linking to %s wrote %d journal lines on it, wanted none", target, len(after)-len(beforeJournal))
	}
	if links := h.card(target).Links; len(links) != 0 {
		t.Errorf("the named card carries %+v, and nothing should have written a reverse entry", links)
	}
}

// TestNoVerbRefusesOnAccountOfALink drives CORE-LINK-5, which forbids
// reporting one of the profile's refusal names for a claim, a move, a release,
// a block or an unblock refused on the ground of a link. All five run against
// a card carrying links, including one to itself, and all five are admitted.
//
// The second half is the other side of the same property: neither link nor
// unlink consults the claim system, so both succeed on a card another owner
// holds, exactly as a comment does.
func TestNoVerbRefusesOnAccountOfALink(t *testing.T) {
	h := newHarness(t)
	source := h.ready("the source")
	target := h.add("the target")
	mustLink(t, h, source, "blocks", target)
	mustLink(t, h, source, "relates", source)

	before := h.card(source)
	h.mustDo(&Request{Verb: Claim, Card: source, Actor: "bob"})
	h.mustDo(&Request{Verb: Block, Card: source, Actor: "bob", Reason: "stopped for a moment"})
	h.mustDo(&Request{Verb: Unblock, Card: source, Actor: "alka"})
	h.mustDo(&Request{Verb: Claim, Card: source, Actor: "bob"})
	h.mustDo(&Request{Verb: Release, Card: source, Actor: "bob"})
	h.mustDo(&Request{Verb: Claim, Card: source, Actor: "bob"})

	// The card is held by bob, and alka writes and removes a link on it.
	if holder := h.card(source).Holder; holder != "bob" {
		t.Fatalf("wanted the card held by bob for the unheld-write case, got %q", holder)
	}
	mustLink(t, h, source, "duplicates", target)
	response := h.library.Unlink(&Request{Verb: "unlink", Actor: "alka", Card: source, Kind: "duplicates", LinkTo: target})
	if response.Outcome != contract.OutcomeOK {
		t.Errorf("unlink on a card another owner holds: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if holder := h.card(source).Holder; holder != "bob" {
		t.Errorf("a link write disturbed the claim: wanted bob, got %q", holder)
	}
	after := h.card(source)
	if after.Title != before.Title || after.Severity != before.Severity || after.Priority != before.Priority || after.Tier != before.Tier {
		t.Errorf("a link write moved a level or the title: before %+v, after %+v", before, after)
	}
}

// TestLinkIsIdempotentOnAPairTheCardAlreadyCarries asserts that repeating a
// call writes no second entry and journals no second event, which is
// SetCardTierAt's own precedent for a write that changes nothing.
func TestLinkIsIdempotentOnAPairTheCardAlreadyCarries(t *testing.T) {
	h := newHarness(t)
	source := h.add("the source")
	target := h.add("the target")

	mustLink(t, h, source, "duplicates", target)
	anchor := anchorText(t, h, source)
	// The second call names the target by its identifier rather than by the
	// reference the first used, so idempotency is decided on the resolved
	// pair rather than on the spelling.
	mustLink(t, h, source, "duplicates", h.card(target).ID)

	if links := h.card(source).Links; len(links) != 1 {
		t.Errorf("wanted the one entry to survive a repeat, got %+v", links)
	}
	if again := anchorText(t, h, source); again != anchor {
		t.Errorf("a repeated link rewrote the anchor:\nbefore:\n%s\nafter:\n%s", anchor, again)
	}
	if got := countEvents(t, h, source, contract.EventLinked); got != 1 {
		t.Errorf("wanted one linked event after a repeat, got %d", got)
	}
}

// TestUnlinkRemovesExactlyOneEntry asserts that removal is scoped to the pair
// named, that the rest keep their order, that the target resolves by the same
// rule link's does, and that a pair the card does not carry is refused.
func TestUnlinkRemovesExactlyOneEntry(t *testing.T) {
	h := newHarness(t)
	source := h.add("the source")
	first := h.add("the first target")
	second := h.add("the second target")
	firstID, secondID := h.card(first).ID, h.card(second).ID

	mustLink(t, h, source, "duplicates", first)
	mustLink(t, h, source, "relates", second)
	mustLink(t, h, source, "blocks", first)

	// Removed by the bare identifier though it was written by reference,
	// which is the interchangeability the two verbs share.
	response := h.library.Unlink(&Request{Verb: "unlink", Actor: "alka", Card: source, Kind: "relates", LinkTo: secondID})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("unlink: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	links := h.card(source).Links
	want := []bench.Link{{Kind: "duplicates", To: firstID}, {Kind: "blocks", To: firstID}}
	if len(links) != len(want) {
		t.Fatalf("wanted %d links left, got %+v", len(want), links)
	}
	for i := range want {
		if links[i] != want[i] {
			t.Errorf("entry %d is %+v, wanted %+v, so removal did not keep the order", i, links[i], want[i])
		}
	}
	if got := countEvents(t, h, source, contract.EventUnlinked); got != 1 {
		t.Errorf("wanted one unlinked event, got %d", got)
	}

	// A pair the card does not carry: the target is real and the kind is
	// real, and no entry pairs them.
	before := anchorText(t, h, source)
	refused := h.library.Unlink(&Request{Verb: "unlink", Actor: "alka", Card: source, Kind: "relates", LinkTo: first})
	if refused.Refusal != contract.UnknownLink {
		t.Errorf("unlink of a pair the card does not carry: wanted %s, got %s %s", contract.UnknownLink, refused.Outcome, refused.Refusal)
	}
	if refused.Detail != first {
		t.Errorf("the refusal names %q rather than the target as it was typed, %q", refused.Detail, first)
	}
	if refused.Context["kind"] != "relates" {
		t.Errorf("the refusal carries the kind %q rather than relates", refused.Context["kind"])
	}
	h.reopen()
	if after := anchorText(t, h, source); after != before {
		t.Errorf("a refused unlink rewrote the anchor:\n%s", after)
	}
}

// TestLinkRefusesItsPreconditionsInTheProfilesOrder asserts that both verbs run
// the workbench's operator, the request's owner, and the two arguments in the
// order every other write verb in this package fixes, and that a refusal at
// any of them writes nothing.
func TestLinkRefusesItsPreconditionsInTheProfilesOrder(t *testing.T) {
	h := newHarness(t)
	source := h.add("the source")
	target := h.add("the target")
	mustLink(t, h, source, "duplicates", target)
	before := anchorText(t, h, source)

	// The two entries are closures over the harness rather than method values
	// off h.library, because the loop reopens the bench and a method value
	// would go on calling the library that existed when the map was built.
	verbs := map[string]func(*Request) *Response{
		"link":   func(req *Request) *Response { return h.library.Link(req) },
		"unlink": func(req *Request) *Response { return h.library.Unlink(req) },
	}
	for name, run := range verbs {
		// no-operator precedes everything, including a card that resolves to
		// nothing, so the request below names a card the workbench does carry
		// and would otherwise be admitted.
		operator := h.library.Bench.Operator
		h.library.Bench.Operator = ""
		if response := run(&Request{Verb: name, Actor: "alka", Card: source, Kind: "duplicates", LinkTo: target}); response.Refusal != contract.NoOperator {
			t.Errorf("%s with no operator designated: wanted %s, got %s %s", name, contract.NoOperator, response.Outcome, response.Refusal)
		}
		h.library.Bench.Operator = operator

		// no-owner precedes the two arguments, so the request below carries
		// an empty kind as well and still reports the owner.
		if response := run(&Request{Verb: name, Card: source, Kind: "", LinkTo: target}); response.Refusal != contract.NoOwner {
			t.Errorf("%s with no actor: wanted %s, got %s %s", name, contract.NoOwner, response.Outcome, response.Refusal)
		}
		for field, req := range map[string]*Request{
			"kind": {Verb: name, Actor: "alka", Card: source, Kind: "   ", LinkTo: target},
			"to":   {Verb: name, Actor: "alka", Card: source, Kind: "duplicates", LinkTo: "  "},
		} {
			response := run(req)
			if response.Refusal != contract.Malformed {
				t.Errorf("%s with an empty %s: wanted %s, got %s %s", name, field, contract.Malformed, response.Outcome, response.Refusal)
			}
			if response.Detail != field {
				t.Errorf("%s with an empty %s: the refusal names %q", name, field, response.Detail)
			}
		}
		h.reopen()
		if after := anchorText(t, h, source); after != before {
			t.Errorf("a refused %s rewrote the anchor:\n%s", name, after)
		}
	}
}

// TestSavingACardWithNoLinksLeavesNoEmptyBlock asserts the other half of the
// declared-block clause Save gained: a card carrying no link carries no links
// key at all, rather than a key with nothing under it.
func TestSavingACardWithNoLinksLeavesNoEmptyBlock(t *testing.T) {
	h := newHarness(t)
	source := h.add("the source")
	target := h.add("the target")

	mustLink(t, h, source, "duplicates", target)
	if !strings.Contains(anchorText(t, h, source), "links:") {
		t.Fatalf("the anchor carries no links block to remove")
	}
	response := h.library.Unlink(&Request{Verb: "unlink", Actor: "alka", Card: source, Kind: "duplicates", LinkTo: target})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("unlink: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if anchor := anchorText(t, h, source); strings.Contains(anchor, "links:") {
		t.Errorf("removing the last link left a links key behind:\n%s", anchor)
	}
	if got := filepath.Base(h.card(source).AnchorPath()); got != bench.CardAnchor {
		t.Errorf("the anchor read is %s rather than the card's own %s", got, bench.CardAnchor)
	}
}
