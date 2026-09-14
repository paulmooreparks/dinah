package verb

import (
	"encoding/json"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// The frontmatter blocks the tests in this file plant. Each is a whole header
// rather than a line assembled from pieces, so that what reaches disk is
// readable in the test and a reader can tell at a glance which key is being
// varied.
const (
	pendingQuestion   = "kind: open_question\nstate: pending\n"
	resolvedQuestion  = "kind: open_question\nstate: resolved\n"
	pendingDecision   = "kind: decision\nstate: pending\n"
	pendingCriterion  = "kind: acceptance_criterion\nstate: pending\n"
	questionNoState   = "kind: open_question\n"
	questionEmpty     = "kind: open_question\nstate:\n"
	questionNonsense  = "kind: open_question\nstate: mulling\n"
	verifiedCriterion = "kind: acceptance_criterion\nstate: verified\n"
)

// TestAnUnresolvedItemRefusesTheClaim drives CORE-CLAIM-10, the seventh row of
// the claim's own list, on the state and the kind alone. Every item it plants
// names no column, which is the shape the refusal still reaches, so the claim
// is refused wherever the card stands and whoever asks. What an item naming a
// declared column does is the subject of the tests below it.
//
// The admitted cases carry the weight here. A tool refusing every claim on a
// card that has any checklist item at all would pass the refused half on its
// own, so the resolved question and the pending criterion are what tell the
// rule apart from a blanket refusal.
func TestAnUnresolvedItemRefusesTheClaim(t *testing.T) {
	refused := []struct {
		name        string
		frontmatter string
	}{
		{"a pending open question", pendingQuestion},
		{"a pending decision", pendingDecision},
	}
	for _, item := range refused {
		t.Run(item.name+" refuses the claim", func(t *testing.T) {
			h := newHarness(t)
			ref := h.ready("carrying a judgement")
			h.item(ref, "b00000000001", item.frontmatter, "Which way round?")

			response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.UnresolvedItem {
				t.Fatalf("wanted %s, got %s %s", contract.UnresolvedItem, response.Outcome, response.Refusal)
			}
			if response.Detail == "" {
				t.Error("the refusal should say what it was about, got nothing")
			}
			if card := h.card(ref); card.State != contract.StateReady || card.Holder != "" {
				t.Errorf("the refused claim wrote to the card: state %q holder %q", card.State, card.Holder)
			}
		})
	}

	admitted := []struct {
		name        string
		frontmatter string
	}{
		{"a resolved open question", resolvedQuestion},
		{"a pending acceptance criterion", pendingCriterion},
		{"a verified acceptance criterion", verifiedCriterion},
	}
	for _, item := range admitted {
		t.Run(item.name+" admits the claim", func(t *testing.T) {
			h := newHarness(t)
			ref := h.ready("carrying a judgement")
			h.item(ref, "b00000000001", item.frontmatter, "Which way round?")

			response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
			if response.Outcome != contract.OutcomeOK {
				t.Fatalf("wanted ok, got %s %s", response.Outcome, response.Refusal)
			}
			if card := h.card(ref); card.State != contract.StateActive || card.Holder != "alka" {
				t.Errorf("the admitted claim: state %q holder %q", card.State, card.Holder)
			}
		})
	}
}

// TestAnItemWhoseStateIsUnreadableRefusesTheClaim asserts the fail-closed half
// of the same rule. A blocking-kind item whose state is absent, empty or
// outside the closed set is read as pending, because reading a damaged file as
// resolved lets through exactly the unanswered question the refusal exists to
// catch, where reading it as pending costs a claim until somebody repairs the
// file.
//
// The interesting way to be wrong is not the missing key but the fall-through:
// an implementation comparing the state against each resolved value in turn
// and admitting whatever matched none would admit all three of these.
func TestAnItemWhoseStateIsUnreadableRefusesTheClaim(t *testing.T) {
	cases := []struct {
		name        string
		frontmatter string
	}{
		{"the state key is absent", questionNoState},
		{"the state key is empty", questionEmpty},
		{"the state is outside the closed set", questionNonsense},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			h := newHarness(t)
			ref := h.ready("carrying a damaged item")
			h.item(ref, "b00000000002", item.frontmatter, "Which way round?")

			response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.UnresolvedItem {
				t.Fatalf("wanted %s, got %s %s", contract.UnresolvedItem, response.Outcome, response.Refusal)
			}
		})
	}
}

// TestAPullThatTakesTheCardUpAnswersTheClaimsLastRow asserts that the claim
// half of a pull runs CORE-CLAIM-10 and the rest of a pull does not. A plain
// pull takes the card up, so it is refused by the item; a pull carrying
// --no-claim leaves the card ready and takes nothing up, so the item has
// nothing to refuse and the card lands.
func TestAPullThatTakesTheCardUpAnswersTheClaimsLastRow(t *testing.T) {
	t.Run("a pull that claims is refused", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("waiting upstream")
		h.item(ref, "b00000000003", pendingQuestion, "Which way round?")

		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: "doing"})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.UnresolvedItem {
			t.Fatalf("wanted %s, got %s %s", contract.UnresolvedItem, response.Outcome, response.Refusal)
		}
		if card := h.card(ref); card.Column != intake || card.Holder != "" {
			t.Errorf("the refused pull wrote to the card: column %q holder %q", card.Column, card.Holder)
		}
	})

	t.Run("a pull carrying --no-claim lands the card", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("waiting upstream")
		h.item(ref, "b00000000003", pendingQuestion, "Which way round?")

		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: "doing", NoClaim: true})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("wanted ok, got %s %s", response.Outcome, response.Refusal)
		}
		card := h.card(ref)
		if card.Column != doing {
			t.Errorf("column: wanted the card landed at doing, got %q", card.Column)
		}
		if card.State != contract.StateReady || card.Holder != "" {
			t.Errorf("the pull took the card up anyway: state %q holder %q", card.State, card.Holder)
		}
	})
}

// TestACardPublishesHowManyItemsWouldRefuseAClaim asserts that a reader sees
// the refusal coming rather than meeting it and being told afterwards. The
// count is of the items that would refuse a claim right now, so the criterion
// standing beside the two questions is not counted, and a card with nothing to
// refuse over omits the field rather than publishing a zero.
func TestACardPublishesHowManyItemsWouldRefuseAClaim(t *testing.T) {
	h := newHarness(t)
	carrying := h.ready("carrying three items")
	h.item(carrying, "b00000000004", pendingQuestion, "Which way round?")
	h.item(carrying, "b00000000005", pendingDecision, "Which way round?")
	h.item(carrying, "b00000000006", pendingCriterion, "It works.")
	clean := h.ready("carrying nothing that refuses")
	h.item(clean, "b00000000007", resolvedQuestion, "Which way round?")

	detail, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: carrying})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if detail.Card.BlockingItems != 2 {
		t.Errorf("wanted the two questions counted and the criterion not, got %d", detail.Card.BlockingItems)
	}

	quiet, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: clean})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if quiet.Card.BlockingItems != 0 {
		t.Errorf("wanted nothing counted, got %d", quiet.Card.BlockingItems)
	}
	encoded, err := json.Marshal(quiet.Card)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "blocking_items") {
		t.Errorf("a card with nothing to refuse over should omit the key, got %s", encoded)
	}
}

// holdingDefinition is a flow carrying a station that holds on the way out, a
// later station that holds on the way in, and a plain station on either side
// of each. It is the smallest flow that can stand a card before, at and past a
// hold in both directions, which is what the position tests below need, and it
// carries a station declaring no hold at all, which is what tells a build
// reading CORE-CLAIM-10 apart from one that read the hold instead.
const holdingDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Holding",
  "columns": [
    { "id": "f00000000001", "title": "Queue", "kind": "intake",
      "instructions": "Queue instructions.\n" },
    { "id": "f00000000002", "title": "Early", "kind": "work",
      "instructions": "Early instructions.\n" },
    { "id": "f00000000003", "title": "Spec", "kind": "work", "gate_items": "out",
      "instructions": "Spec instructions.\n" },
    { "id": "f00000000004", "title": "Build", "kind": "work",
      "instructions": "Build instructions.\n" },
    { "id": "f00000000005", "title": "Merge", "kind": "work", "gate_items": true,
      "instructions": "Merge instructions.\n" },
    { "id": "f00000000006", "title": "Landed", "kind": "work",
      "instructions": "Landed instructions.\n" },
    { "id": "f00000000007", "title": "Closed", "kind": "done",
      "instructions": "Closed instructions.\n" }
  ]
}`

// The identifiers of the holding flow's seven columns, and one more that is
// shaped like an identifier and names no column of it.
const (
	holdQueue  = "f00000000001"
	holdEarly  = "f00000000002"
	holdSpec   = "f00000000003"
	holdBuild  = "f00000000004"
	holdMerge  = "f00000000005"
	holdLanded = "f00000000006"

	// holdUndeclared is well formed and names no column this flow carries,
	// which is the second of the two shapes CORE-CLAIM-10 refuses over and
	// the one a build resolving the field loosely would admit.
	holdUndeclared = "f0000000dead"
)

// newHoldingHarness builds a harness over the holding flow above.
func newHoldingHarness(t *testing.T) *harness {
	t.Helper()
	return harnessFromDefinition(t, "hd", holdingDefinition)
}

// questionNaming is the frontmatter of one pending open question naming a
// column, or of one naming no column when the argument is empty. The ordinal
// is written because the tests below reach the item by its ordinal reference.
func questionNaming(column string) string {
	if column == "" {
		return pendingQuestion + "ordinal: 1\n"
	}
	return pendingQuestion + "column: " + column + "\nordinal: 1\n"
}

// criterionNaming is questionNaming for an acceptance criterion.
func criterionNaming(column string) string {
	if column == "" {
		return pendingCriterion + "ordinal: 1\n"
	}
	return pendingCriterion + "column: " + column + "\nordinal: 1\n"
}

// claimStands claims a card and fails unless the claim landed and the card on
// disk reads active with the claiming owner as holder. The name is printed
// rather than derived, because the tests calling it name six positions that a
// card reference cannot tell apart.
func claimStands(h *harness, name, ref string) {
	h.t.Helper()
	response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	if response.Outcome != contract.OutcomeOK {
		h.t.Errorf("%s: wanted ok, got %s %s", name, response.Outcome, response.Refusal)
		return
	}
	if card := h.card(ref); card.State != contract.StateActive || card.Holder != "alka" {
		h.t.Errorf("%s: the claim landed but the card reads state %q holder %q", name, card.State, card.Holder)
	}
}

// claimIsRefusedOverTheItem claims a card and fails unless the claim was
// refused unresolved-item naming the item, and unless the card on disk is
// still waiting in the open afterwards.
func claimIsRefusedOverTheItem(h *harness, name, ref, item string) {
	h.t.Helper()
	response := h.do(&Request{Verb: Claim, Card: ref, Actor: "alka"})
	if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.UnresolvedItem {
		h.t.Errorf("%s: wanted %s, got %s %s", name, contract.UnresolvedItem, response.Outcome, response.Refusal)
		return
	}
	if !strings.Contains(response.Detail, item) {
		h.t.Errorf("%s: the refusal says %q, which does not name the item %s", name, response.Detail, item)
	}
	if card := h.card(ref); card.State != contract.StateReady || card.Holder != "" {
		h.t.Errorf("%s: the refused claim wrote to the card: state %q holder %q", name, card.State, card.Holder)
	}
}

// TestAnItemNamingADeclaredColumnDoesNotRefuseTheClaim drives CORE-CLAIM-10
// and CORE-ITEM-4. An item names at most one column, and that column is what
// decides where the card stops, so the claim leaves every item naming a
// declared column to the column it names and refuses over the items no
// column's hold can ever reach.
//
// Five fixtures differ in the item's column field and in nothing else. Three
// of the five carry the weight. The column declaring no hold is what tells
// this build apart from one that read a hold instead of a declaration, since
// an item filed against such a column stops nothing anywhere and is admitted
// anyway. The undeclared identifier is what tells it apart from a build
// resolving the field through ColumnByRef, which would match a slug or a
// title, and from one reading a failed lookup as a column it found. And the
// item naming no column is what keeps the refusal from becoming a refusal
// nothing satisfies.
func TestAnItemNamingADeclaredColumnDoesNotRefuseTheClaim(t *testing.T) {
	admitted := []struct {
		name   string
		column string
	}{
		{"a declared column that holds on entry", holdMerge},
		{"a declared column that holds on exit", holdSpec},
		{"a declared column that declares no hold", holdBuild},
	}
	for _, c := range admitted {
		t.Run(c.name+" admits the claim", func(t *testing.T) {
			h := newHoldingHarness(t)
			ref := h.readyAt("carrying a question for a station", holdEarly)
			h.item(ref, "b00000000001", questionNaming(c.column), "Which way round?")
			claimStands(h, c.name, ref)
		})
	}

	refused := []struct {
		name   string
		column string
	}{
		{"no column at all", ""},
		{"a column the workbench does not declare", holdUndeclared},
	}
	for _, c := range refused {
		t.Run(c.name+" refuses the claim", func(t *testing.T) {
			h := newHoldingHarness(t)
			ref := h.readyAt("carrying a question nothing can reach", holdEarly)
			h.item(ref, "b00000000001", questionNaming(c.column), "Which way round?")
			claimIsRefusedOverTheItem(h, c.name, ref, "b00000000001")
		})
	}
}

// TestTheClaimRefusalReadsNoCardPosition is the half of CORE-CLAIM-10 that
// keeps the rule out of the card's own travel. Where the card stands is not
// read, on the entry side and the exit side alike, so a question filed for a
// later station rides the card there without stopping anybody working the
// stations in front of it, and a question filed for the station the card is
// standing at does not deadlock the one station that can answer it.
//
// Each of the six positions is asserted on its own rather than inside a loop,
// because a loop that stopped at the first failure would report one position
// and say nothing about the other five. The seventh card is what keeps the
// six from passing against a build that admits every claim.
func TestTheClaimRefusalReadsNoCardPosition(t *testing.T) {
	h := newHoldingHarness(t)

	exitBefore := h.readyAt("naming the exit hold, standing before it", holdEarly)
	h.item(exitBefore, "b00000000001", questionNaming(holdSpec), "Which way round?")
	exitAt := h.readyAt("naming the exit hold, standing at it", holdSpec)
	h.item(exitAt, "b00000000002", questionNaming(holdSpec), "Which way round?")
	exitPast := h.readyAt("naming the exit hold, standing past it", holdBuild)
	h.item(exitPast, "b00000000003", questionNaming(holdSpec), "Which way round?")

	entryBefore := h.readyAt("naming the entry hold, standing before it", holdBuild)
	h.item(entryBefore, "b00000000004", questionNaming(holdMerge), "Which way round?")
	entryAt := h.readyAt("naming the entry hold, standing at it", holdMerge)
	h.item(entryAt, "b00000000005", questionNaming(holdMerge), "Which way round?")
	entryPast := h.readyAt("naming the entry hold, standing past it", holdLanded)
	h.item(entryPast, "b00000000006", questionNaming(holdMerge), "Which way round?")

	claimStands(h, "the exit-hold column named, the card standing before it", exitBefore)
	claimStands(h, "the exit-hold column named, the card standing at it", exitAt)
	claimStands(h, "the exit-hold column named, the card standing past it", exitPast)
	claimStands(h, "the entry-hold column named, the card standing before it", entryBefore)
	claimStands(h, "the entry-hold column named, the card standing at it", entryAt)
	claimStands(h, "the entry-hold column named, the card standing past it", entryPast)

	unreachable := h.readyAt("naming no column, standing where the second stands", holdSpec)
	h.item(unreachable, "b00000000007", questionNaming(""), "Which way round?")
	claimIsRefusedOverTheItem(h, "a question naming no column, standing at the exit hold", unreachable, "b00000000007")
}

// TestTheExitHoldSurvivesTheNarrowedClaim is the guard on what CORE-CLAIM-10
// did not touch. The claim stopped refusing over an item naming a declared
// column, and the column that item names goes on holding the card exactly as
// it did, so the stop somebody meant to create still exists.
//
// The three acts run in order on one fixture, which is what pins the refusal
// between a claim that worked and a move that worked. A test asserting the
// refusal alone would pass against a build refusing every move out of that
// column, and one asserting the move alone would pass against a build holding
// nothing.
func TestTheExitHoldSurvivesTheNarrowedClaim(t *testing.T) {
	h := newHoldingHarness(t)
	ref := h.readyAt("standing at the station its question names", holdSpec)
	h.item(ref, "b00000000001", questionNaming(holdSpec), "Which way round?")

	claimStands(h, "a card standing at the column its own question names", ref)

	held := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: holdBuild})
	if held.Outcome != contract.OutcomeRefused || held.Refusal != contract.UnresolvedItemExit {
		t.Fatalf("the move out: wanted %s, got %s %s", contract.UnresolvedItemExit, held.Outcome, held.Refusal)
	}
	if !strings.Contains(held.Detail, "b00000000001") {
		t.Errorf("the refusal says %q, which does not name the item holding the card", held.Detail)
	}
	if card := h.card(ref); card.Column != holdSpec {
		t.Errorf("the refused move carried the card to %q", card.Column)
	}

	settled := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: ref + "/questions/1", Note: "The operator ruled."})
	if settled.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", settled.Outcome, settled.Refusal)
	}
	h.reopen()

	admitted := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: holdBuild})
	if admitted.Outcome != contract.OutcomeOK {
		t.Fatalf("the move after the item was settled: %s %s", admitted.Outcome, admitted.Refusal)
	}
	if card := h.card(ref); card.Column != holdBuild {
		t.Errorf("the admitted move left the card at %q", card.Column)
	}
}

// TestAnAcceptanceCriterionStaysOutsideTheClaimsBlockingSet is the ruling
// CORE-CLAIM-10 inherited rather than moved. A criterion is verified after the
// work rather than before it, so refusing the claim over one would refuse the
// card the very work that lets anybody verify it, and that reaches a criterion
// naming no column as well as one naming a column.
//
// The kind clause and the column clause are both live, which is what the
// fourth fixture says. A build that dropped the kind test in favour of the
// column test would admit the first two criteria and start refusing the third,
// and a build that kept only the kind test would admit the question as well.
func TestAnAcceptanceCriterionStaysOutsideTheClaimsBlockingSet(t *testing.T) {
	criteria := []struct {
		name   string
		column string
	}{
		{"a criterion naming a column that holds", holdMerge},
		{"a criterion naming a column that does not hold", holdBuild},
		{"a criterion naming no column", ""},
	}
	for _, c := range criteria {
		t.Run(c.name+" admits the claim", func(t *testing.T) {
			h := newHoldingHarness(t)
			ref := h.readyAt("carrying a criterion", holdEarly)
			h.item(ref, "b00000000001", criterionNaming(c.column), "It works.")
			claimStands(h, c.name, ref)
		})
	}

	t.Run("a question naming no column refuses the same claim", func(t *testing.T) {
		h := newHoldingHarness(t)
		ref := h.readyAt("carrying a question with the same empty column", holdEarly)
		h.item(ref, "b00000000001", questionNaming(""), "Which way round?")
		claimIsRefusedOverTheItem(h, "a pending question naming no column", ref, "b00000000001")
	})
}

// TestAPullAnswersTheNarrowedRowAtTheSameRow asserts that pull's fifteenth row
// narrowed with claim's seventh. A pull that takes the card up is a claim, and
// a build that narrowed the claim path alone would refuse the first half here
// while passing every test above.
//
// The rule reads no card position, so the answer is the same whether it is
// asked against the column the card leaves or the column it lands at, which is
// what makes one reading serve a verb that does both in a single act.
func TestAPullAnswersTheNarrowedRowAtTheSameRow(t *testing.T) {
	t.Run("a question naming a declared column lands and is taken up", func(t *testing.T) {
		h := newHoldingHarness(t)
		ref := h.add("waiting upstream with a question for a later station")
		h.item(ref, "b00000000001", questionNaming(holdMerge), "Which way round?")

		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: holdEarly})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("wanted ok, got %s %s", response.Outcome, response.Refusal)
		}
		card := h.card(ref)
		if card.Column != holdEarly {
			t.Errorf("the card landed at %q, wanted the destination", card.Column)
		}
		if card.State != contract.StateActive || card.Holder != "alka" {
			t.Errorf("the pull left the card at state %q holder %q", card.State, card.Holder)
		}
	})

	t.Run("a question naming no column refuses the pull", func(t *testing.T) {
		h := newHoldingHarness(t)
		ref := h.add("waiting upstream with a question nothing can reach")
		h.item(ref, "b00000000001", questionNaming(""), "Which way round?")

		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: holdEarly})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.UnresolvedItem {
			t.Fatalf("wanted %s, got %s %s", contract.UnresolvedItem, response.Outcome, response.Refusal)
		}
		if card := h.card(ref); card.Column != holdQueue || card.Holder != "" {
			t.Errorf("the refused pull wrote to the card: column %q holder %q", card.Column, card.Holder)
		}
	})

	t.Run("the same card lands under --no-claim", func(t *testing.T) {
		h := newHoldingHarness(t)
		ref := h.add("waiting upstream with a question nothing can reach")
		h.item(ref, "b00000000001", questionNaming(""), "Which way round?")

		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: holdEarly, NoClaim: true})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("wanted ok, got %s %s", response.Outcome, response.Refusal)
		}
		card := h.card(ref)
		if card.Column != holdEarly {
			t.Errorf("the card landed at %q, wanted the destination", card.Column)
		}
		if card.State != contract.StateReady || card.Holder != "" {
			t.Errorf("the pull took the card up anyway: state %q holder %q", card.State, card.Holder)
		}
	})
}
