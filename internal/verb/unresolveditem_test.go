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

// TestAnUnresolvedItemRefusesTheClaim drives CORE-CLAIM-9, the seventh row of
// the claim's own list. A card carrying a question nobody has answered is a
// card whose work has not been cleared to begin, so the claim is refused
// wherever the card stands and whoever asks.
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
// half of a pull runs CORE-CLAIM-9 and the rest of a pull does not. A plain
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

	detail, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: carrying})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if detail.Card.BlockingItems != 2 {
		t.Errorf("wanted the two questions counted and the criterion not, got %d", detail.Card.BlockingItems)
	}

	quiet, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: clean})
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
