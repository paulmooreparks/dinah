package verb

import (
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The identifiers and the handles of the routed fixture's six columns. They
// are fixed rather than minted so that a route declaration can name them and a
// failure can name them back.
const (
	routedIntake   = "b00000000001"
	routedQueue    = "b00000000002"
	routedAlpha    = "b00000000003"
	routedBeta     = "b00000000004"
	routedGamma    = "b00000000005"
	routedFinished = "b00000000006"
)

// routedDefinition is a flow long enough for a route to drop a station in the
// middle of it and still have a station on either side, which is the shape
// every interesting question about routes needs.
//
// Three routes are declared. skipbeta drops the middle station and skipalpha
// drops the one before it, so a card standing at beta's own flow upstream can be
// carrying somewhere else while a card standing further back is carrying into
// beta, which is the position a pull that filters only the further sources
// never reaches. direct drops the buffer, so a card standing there on that road
// has no landing at all. longskip drops two stations together, so a card
// standing at the first of them rejoins its road somewhere other than the
// flow's own next column.
//
// Gamma rejects to beta, which skipbeta does not carry, so a push-back on that
// road lands off it.
const routedDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Routed",
  "routes": {
    "skipbeta": ["b00000000001", "b00000000002", "b00000000003", "b00000000005", "b00000000006"],
    "skipalpha": ["b00000000001", "b00000000002", "b00000000004", "b00000000005", "b00000000006"],
    "direct": ["b00000000001", "b00000000003", "b00000000004", "b00000000005", "b00000000006"],
    "longskip": ["b00000000001", "b00000000002", "b00000000005", "b00000000006"]
  },
  "columns": [
    { "id": "b00000000001", "title": "Intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Queue", "kind": "dinah.buffer" },
    { "id": "b00000000003", "title": "Alpha", "kind": "work" },
    { "id": "b00000000004", "title": "Beta", "kind": "work" },
    { "id": "b00000000005", "title": "Gamma", "kind": "work", "reject_to": "beta" },
    { "id": "b00000000006", "title": "Finished", "kind": "done" }
  ]
}`

// routedHarness builds the routed fixture and opens it.
func routedHarness(t *testing.T) *harness {
	t.Helper()
	return harnessFromDefinition(t, "rt", routedDefinition)
}

// onRoute puts a card on a route by the ordinary field write, and fails the
// test where the write was refused, so a fixture step cannot pass for the
// behaviour under test.
func (h *harness) onRoute(ref, route string) {
	h.t.Helper()
	response := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref, Field: bench.RouteField, Value: route,
	})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("put %s on the route %s: %s %s", ref, route, response.Outcome, response.Refusal)
	}
	h.reopen()
}

// pulled runs a pull into one column and answers the reference it took, empty
// where it took nothing, together with the whole response so a caller can read
// the refusal.
func (h *harness) pulled(actor, column string) (string, *Response) {
	h.t.Helper()
	response := h.library.Pull(&Request{Verb: Pull, Actor: actor, Column: column})
	h.reopen()
	if response.Card == nil {
		return "", response
	}
	return response.Card.Ref, response
}

// TestAPullFollowsTheCardsOwnRoute is dinah-542/criteria/1 and
// dinah-542/criteria/2. Two cards stand in one buffer on two roads, and each is
// taken by the pull its own road carries it into and by no other.
//
// The buffer is what makes the two halves one test. A pull looks through a
// buffer rather than stopping at it, so the card on the short road is carried
// past the station its road drops and into the one beyond, and the card on the
// full list is carried into the station the short road dropped.
func TestAPullFollowsTheCardsOwnRoute(t *testing.T) {
	t.Run("the routed card is taken by the station its road reaches", func(t *testing.T) {
		h := routedHarness(t)
		routed := h.add("walking the short road")
		h.at(routed, routedQueue)
		h.onRoute(routed, "skipalpha")

		// Alpha is the station skipalpha drops, and the pull naming it finds
		// nothing, because this card's road carries it past.
		if took, response := h.pulled("alka", "alpha"); took != "" {
			t.Fatalf("the pull into the dropped station took %s: %s", took, response.Message)
		}
		took, response := h.pulled("alka", "beta")
		if took != routed {
			t.Fatalf("the pull into beta took %q, wanted %s: %s %s", took, routed, response.Outcome, response.Refusal)
		}
	})

	t.Run("the card on the full list is taken by the station the road dropped", func(t *testing.T) {
		h := routedHarness(t)
		plain := h.add("walking the whole flow")
		h.at(plain, routedQueue)

		if took, _ := h.pulled("alka", "beta"); took != "" {
			t.Fatalf("the pull into beta took %s, and this card carries into alpha", took)
		}
		took, response := h.pulled("alka", "alpha")
		if took != plain {
			t.Fatalf("the pull into alpha took %q, wanted %s: %s %s", took, plain, response.Outcome, response.Refusal)
		}
	})

	t.Run("a card standing after the column its road drops is taken", func(t *testing.T) {
		h := routedHarness(t)
		// direct drops the buffer, so alpha stands at index one of that road
		// and at position two of the flow. A road indexed by the flow's
		// position would read gamma as alpha's successor and carry this card
		// past beta, which is the commonest way an implementation of routes
		// goes wrong.
		routed := h.add("standing after the dropped buffer")
		h.at(routed, routedAlpha)
		h.onRoute(routed, "direct")
		took, response := h.pulled("alka", "beta")
		if took != routed {
			t.Fatalf("the pull into beta took %q, wanted %s: %s %s", took, routed, response.Outcome, response.Refusal)
		}
	})

	t.Run("neither card starves or hides the other", func(t *testing.T) {
		h := routedHarness(t)
		routed := h.add("walking the short road")
		h.at(routed, routedQueue)
		h.onRoute(routed, "skipalpha")
		plain := h.add("walking the whole flow")
		h.at(plain, routedQueue)

		if took, _ := h.pulled("alka", "beta"); took != routed {
			t.Errorf("the pull into beta took %q, wanted the routed card %s", took, routed)
		}
		if took, _ := h.pulled("alka", "alpha"); took != plain {
			t.Errorf("the pull into alpha took %q, wanted the card on the full list %s", took, plain)
		}
	})
}

// TestTheFirstStepOfANamedPullAppliesTheRouteFilter is
// dinah-542/criteria/28. The destination's immediate flow upstream holds a
// ready card whose own road carries it somewhere else, and a column further
// back holds one whose road carries it into the destination. The pull takes the
// card from further back and leaves the nearer one standing.
//
// This is the position a pull that filtered only its further sources never
// reaches: trunk tries the immediate upstream first and on its own terms, so
// the nearer card would be taken and carried into a station its road does not
// name.
func TestTheFirstStepOfANamedPullAppliesTheRouteFilter(t *testing.T) {
	h := routedHarness(t)
	// Alpha is beta's immediate upstream in the flow. On skipbeta a card
	// standing there carries into gamma, which is not the destination.
	nearer := h.add("standing at the immediate upstream")
	h.at(nearer, routedAlpha)
	h.onRoute(nearer, "skipbeta")
	// The queue is further back. On skipalpha a card standing there carries
	// into beta, because the road drops the station between them.
	further := h.add("standing further back")
	h.at(further, routedQueue)
	h.onRoute(further, "skipalpha")

	took, response := h.pulled("alka", "beta")
	if took != further {
		t.Fatalf("the pull into beta took %q, wanted %s from further back: %s %s",
			took, further, response.Outcome, response.Refusal)
	}
	if card := h.card(nearer); card.Column != routedAlpha || card.State != contract.StateReady {
		t.Errorf("the nearer card moved: column %q state %q", card.Column, card.State)
	}
}

// TestArrivalOrderHoldsAmongTheCardsBoundForOneStation is
// dinah-542/criteria/17. Three ready cards stand in one buffer: one bound for
// one station arriving first, then two bound for another. Each pull takes the
// earliest card bound for the station it names, and the listing goes on
// reporting all three in arrival order.
func TestArrivalOrderHoldsAmongTheCardsBoundForOneStation(t *testing.T) {
	h := routedHarness(t)
	first := h.add("bound for beta, arriving first")
	h.at(first, routedQueue)
	h.onRoute(first, "skipalpha")
	second := h.add("bound for alpha, arriving second")
	h.at(second, routedQueue)
	third := h.add("bound for alpha, arriving third")
	h.at(third, routedQueue)

	// The listing is read before anything is taken, because CORE-QUEUE-3 fixes
	// the order of a column's cards and this card changes none of it.
	listing, err := h.library.List(&Request{Verb: "list", Actor: "alka", Column: routedQueue})
	if err != nil {
		t.Fatalf("list the buffer: %v", err)
	}
	if len(listing.Cards) != 3 {
		t.Fatalf("the buffer lists %d cards, wanted the three that were planted", len(listing.Cards))
	}
	wanted := []string{first, second, third}
	for at, card := range listing.Cards {
		if card.Ref != wanted[at] {
			t.Fatalf("the buffer lists %v, wanted arrival order %v", refsOfCards(listing.Cards), wanted)
		}
	}

	if took, _ := h.pulled("alka", "alpha"); took != second {
		t.Errorf("the first pull into alpha took %q, wanted the earlier of the two bound there, %s", took, second)
	}
	if took, _ := h.pulled("alka", "alpha"); took != third {
		t.Errorf("the second pull into alpha took %q, wanted %s", took, third)
	}
	if took, _ := h.pulled("alka", "beta"); took != first {
		t.Errorf("the pull into beta took %q, wanted %s, which neither starved nor lost its turn", took, first)
	}
}

// refsOfCards names a run of card views for a failure message.
func refsOfCards(cards []CardView) []string {
	refs := make([]string, 0, len(cards))
	for _, card := range cards {
		refs = append(refs, card.Ref)
	}
	return refs
}

// TestNextComposesItsOfferPerCard is dinah-542/criteria/18 and
// dinah-542/criteria/27. The offer a queue composes is built from the cards
// standing in it rather than from the column, so two cards on two roads give
// two landings, a card whose road carries it nowhere is passed over, and a
// queue whose ready cards all have nowhere to go reports no taker.
func TestNextComposesItsOfferPerCard(t *testing.T) {
	t.Run("the earlier card is named and its landing reported", func(t *testing.T) {
		h := routedHarness(t)
		earlier := h.add("bound for beta")
		h.at(earlier, routedQueue)
		h.onRoute(earlier, "skipalpha")
		later := h.add("bound for alpha")
		h.at(later, routedQueue)

		offer := routeOfferAt(t, h, routedQueue)
		if offer.Card == nil || offer.Card.Ref != earlier {
			t.Fatalf("the offer names %+v, wanted the earlier card %s", offer.Card, earlier)
		}
		if !offer.TakenByPull {
			t.Error("the offer at a buffer should be taken by pull")
		}
		if offer.Landing != "beta" {
			t.Errorf("the offer reports the landing %q, wanted beta, which is where this card's road carries it", offer.Landing)
		}
	})

	t.Run("a card whose road carries it nowhere is passed over", func(t *testing.T) {
		h := routedHarness(t)
		// direct does not carry the buffer, so a card standing there on that
		// road has no column beyond it on its road and no landing at all.
		earlier := h.add("standing where its road does not reach")
		h.at(earlier, routedQueue)
		h.onRoute(earlier, "direct")
		later := h.add("bound for alpha")
		h.at(later, routedQueue)

		offer := routeOfferAt(t, h, routedQueue)
		if offer.Card == nil || offer.Card.Ref != later {
			t.Fatalf("the offer names %+v, wanted the later card %s, since the earlier one has nowhere to go", offer.Card, later)
		}
		if offer.AboveTier {
			t.Error("the earlier card was reported as work above the caller, and no tier floor withheld it")
		}
		if offer.Landing != "alpha" {
			t.Errorf("the offer reports the landing %q, wanted alpha", offer.Landing)
		}
	})

	t.Run("no taker follows the cards rather than the column", func(t *testing.T) {
		h := routedHarness(t)
		stranded := h.add("standing where its road does not reach")
		h.at(stranded, routedQueue)
		h.onRoute(stranded, "direct")

		offer := routeOfferAt(t, h, routedQueue)
		if !offer.NoTaker {
			t.Fatalf("a buffer whose only ready card has no landing answered %+v, wanted no taker", offer)
		}
		if offer.Card != nil || offer.AboveTier {
			t.Errorf("the buffer answered %+v beside no taker", offer)
		}

		arriving := h.add("bound for alpha")
		h.at(arriving, routedQueue)
		offer = routeOfferAt(t, h, routedQueue)
		if offer.NoTaker {
			t.Error("the buffer went on reporting no taker after a card with a landing arrived")
		}
		if offer.Card == nil || offer.Card.Ref != arriving {
			t.Errorf("the buffer offers %+v, wanted %s", offer.Card, arriving)
		}
	})

	t.Run("an empty buffer answers as it does without routes", func(t *testing.T) {
		h := routedHarness(t)
		offer := routeOfferAt(t, h, routedQueue)
		if offer.Card != nil || offer.NoTaker || offer.AboveTier {
			t.Errorf("an empty buffer answered %+v, wanted the plain empty offer read against the full column list", offer)
		}
	})
}

// routeOfferAt reads the offer one column composes, named apart from the
// offerAt beside it, which picks one offer out of a list a caller already
// holds.
func routeOfferAt(t *testing.T, h *harness, column string) Offer {
	t.Helper()
	offers, err := h.library.Next(&Request{Verb: "next", Actor: "alka", Column: column})
	if err != nil {
		t.Fatalf("next at %s: %v", column, err)
	}
	if len(offers) != 1 {
		t.Fatalf("next named one column and answered %d offers", len(offers))
	}
	return offers[0]
}

// TestACardStandingOffItsRouteRejoinsIt is dinah-542/criteria/3 and
// dinah-542/criteria/7. A card carried to a column its own road does not carry
// keeps its column as its only position, and the forward move its road offers
// is the first road column standing after that column in the flow's own order.
func TestACardStandingOffItsRouteRejoinsIt(t *testing.T) {
	h := routedHarness(t)
	ref := h.add("pushed back off its own road")
	h.onRoute(ref, "skipbeta")
	h.at(ref, routedGamma)

	// Gamma rejects to beta, which skipbeta does not carry. The rejection is
	// an ordinary move to the row marked reject, and it lands the card at a
	// column its own road does not carry.
	var target string
	for _, move := range h.library.legalMoves(h.card(ref)) {
		if move.Reject {
			target = move.Column
		}
	}
	if target != routedBeta {
		t.Fatalf("gamma's reject row names %q, wanted beta", target)
	}
	h.at(ref, target)

	card := h.card(ref)
	if card.Column != routedBeta {
		t.Fatalf("the card stands at %q, and its column is the whole of where it stands", card.Column)
	}
	if card.Route != "skipbeta" {
		t.Errorf("the card reads back the route %q", card.Route)
	}
	moves := h.library.legalMoves(card)
	var marked []string
	for _, move := range moves {
		if move.OnRoute {
			marked = append(marked, move.Ref)
		}
	}
	if len(marked) != 1 || marked[0] != "gamma" {
		t.Fatalf("the moves mark %v as on route, wanted gamma alone, which is where this card rejoins its road", marked)
	}
	// Every declared column but the card's own is still a row and each carries
	// its direction against the flow, which is what keeps CORE-STATE-7
	// satisfied by the row that satisfies it without routes.
	if len(moves) != len(h.library.Bench.Columns)-1 {
		t.Errorf("the card carries %d legal moves and the workbench declares %d columns", len(moves), len(h.library.Bench.Columns))
	}
	for _, move := range moves {
		if move.Ref == "finished" && move.Direction != Forward {
			t.Errorf("a column ahead of the card reads %s", move.Direction)
		}
	}

	// check reports the card while it stands there and stops once it has
	// moved back onto its road.
	if !hasFinding(t, h, bench.FindingCardOffRoute) {
		t.Error("check does not report the card standing off its road")
	}
	h.at(ref, routedGamma)
	if hasFinding(t, h, bench.FindingCardOffRoute) {
		t.Error("check goes on reporting the card after it moved back onto its road")
	}
}

// TestACardPastTheEndOfItsRouteIsOfferedNoRouteForwardRow is the last clause of
// dinah-542/criteria/3. A card standing after every column its road carries is
// offered no forward move on its road rather than an arbitrary one.
func TestACardPastTheEndOfItsRouteIsOfferedNoRouteForwardRow(t *testing.T) {
	h := harnessFromDefinition(t, "pe", pastTheEndDefinition)
	ref := h.add("run past its own road")
	h.onRoute(ref, "early")
	h.at(ref, "f00000000003")
	for _, move := range h.library.legalMoves(h.card(ref)) {
		if move.OnRoute {
			t.Errorf("a card past the end of its road is offered %s as its road's forward move", move.Ref)
		}
	}
}

// pastTheEndDefinition declares a road ending before the flow does, so a card
// can stand after every column the road carries.
const pastTheEndDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Past the end",
  "routes": { "early": ["f00000000001", "f00000000002"] },
  "columns": [
    { "id": "f00000000001", "title": "Intake", "kind": "intake" },
    { "id": "f00000000002", "title": "Doing", "kind": "work" },
    { "id": "f00000000003", "title": "Later", "kind": "work" },
    { "id": "f00000000004", "title": "Finished", "kind": "done" }
  ]
}`

// hasFinding reports whether dinah check reports one finding key over the
// harness's workbench.
func hasFinding(t *testing.T, h *harness, key string) bool {
	t.Helper()
	findings, err := h.library.Bench.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		if finding.Key == key {
			return true
		}
	}
	return false
}

// TestARoutedCardIsMarkedAtItsRoutesNextColumn is dinah-542/criteria/10 on a
// card that carries a route. The one row carrying the marker is the next
// column of the card's own road, and the flow's own next column is still a row,
// still reads forward, and does not carry the marker.
func TestARoutedCardIsMarkedAtItsRoutesNextColumn(t *testing.T) {
	h := routedHarness(t)
	ref := h.add("on a road that drops alpha")
	h.at(ref, routedQueue)
	h.onRoute(ref, "skipalpha")
	moves := h.library.legalMoves(h.card(ref))
	var marked []string
	for _, move := range moves {
		if move.OnRoute {
			marked = append(marked, move.Ref)
		}
		if move.Ref == "alpha" {
			if move.Direction != Forward {
				t.Errorf("the flow's next column reads %s", move.Direction)
			}
			if move.OnRoute {
				t.Error("the flow's next column carries the marker, and this card's road does not carry it")
			}
		}
	}
	if len(marked) != 1 || marked[0] != "beta" {
		t.Errorf("the moves mark %v, wanted beta alone, which is the next column of this card's road", marked)
	}
}

// TestACardOffItsRouteRejoinsPastTwoDroppedColumns is the position of
// dinah-542/criteria/3 where the rejoin column and the flow's own next column
// differ, because the road drops two stations together.
func TestACardOffItsRouteRejoinsPastTwoDroppedColumns(t *testing.T) {
	h := routedHarness(t)
	ref := h.add("off a road that drops alpha and beta")
	h.onRoute(ref, "longskip")
	h.at(ref, routedAlpha)
	var marked []string
	for _, move := range h.library.legalMoves(h.card(ref)) {
		if move.OnRoute {
			marked = append(marked, move.Ref)
		}
	}
	if len(marked) != 1 || marked[0] != "gamma" {
		t.Errorf("the moves mark %v, wanted gamma alone: beta is the flow's next column and longskip drops it", marked)
	}
}

// TestACardOnTheFullListIsMarkedAtTheFlowsNextColumn is the first half of
// dinah-542/criteria/10. A card carrying no route walks the whole flow, so the
// one row carrying the marker is the flow's own next column.
func TestACardOnTheFullListIsMarkedAtTheFlowsNextColumn(t *testing.T) {
	h := routedHarness(t)
	ref := h.add("on the full column list")
	h.at(ref, routedAlpha)
	var marked []string
	for _, move := range h.library.legalMoves(h.card(ref)) {
		if move.OnRoute {
			marked = append(marked, move.Ref)
		}
	}
	if len(marked) != 1 || marked[0] != "beta" {
		t.Errorf("the moves mark %v, wanted beta alone, which is the flow's own next column", marked)
	}
}

// TestACardAtATerminalColumnCarriesNoRouteMarker is the second half of
// dinah-542/criteria/10. A done column has no forward move at all, so it has no
// forward move along any road either.
//
// The flow ends in two done columns and the road carries both, so the road
// does name a column after the one the card stands at. That is the position
// where a marker could appear at all, and the test is about it rather than
// about a card standing at the last column of everything.
func TestACardAtATerminalColumnCarriesNoRouteMarker(t *testing.T) {
	h := harnessFromDefinition(t, "tm", twoTerminalsDefinition)
	ref := h.add("finished")
	h.onRoute(ref, "both")
	h.at(ref, "a10000000003")
	moves := h.library.legalMoves(h.card(ref))
	if len(moves) == 0 {
		t.Fatal("a card at a done column carries no legal moves at all, so this asserts nothing")
	}
	for _, move := range moves {
		if move.Direction == Forward {
			t.Errorf("a card at a done column is offered the forward move to %s", move.Ref)
		}
		if move.OnRoute {
			t.Errorf("a card at a done column is offered %s as its route's forward move", move.Ref)
		}
	}
}

// twoTerminalsDefinition ends in two done columns, and its one road carries
// both of them.
const twoTerminalsDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Two terminals",
  "routes": { "both": ["a10000000001", "a10000000002", "a10000000003", "a10000000004"] },
  "columns": [
    { "id": "a10000000001", "title": "Intake", "kind": "intake" },
    { "id": "a10000000002", "title": "Doing", "kind": "work" },
    { "id": "a10000000003", "title": "Finished", "kind": "done" },
    { "id": "a10000000004", "title": "Closed", "kind": "done" }
  ]
}`

// TestARouteWriteRefusesWhatItWouldStrand is dinah-542/criteria/4 and
// dinah-542/criteria/23. A route that drops a column a pending item names is
// refused by name, and each accepting case passes beside it.
func TestARouteWriteRefusesWhatItWouldStrand(t *testing.T) {
	write := func(h *harness, ref, route string) *Response {
		response := h.library.SetField(&Request{
			Verb: "set", Actor: "brin", Ref: ref, Field: bench.RouteField, Value: route,
		})
		h.reopen()
		return response
	}

	t.Run("a pending item naming a dropped column refuses", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("carrying a question for beta")
		fileItem(t, h, ref, "open_question", "who answers at beta?")
		h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: ref + "/questions/1",
			Field: bench.ItemColumnField, Value: "beta",
		})
		h.reopen()
		response := write(h, ref, "skipbeta")
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.RouteStrandsItem {
			t.Fatalf("wanted %s, got %s %s", contract.RouteStrandsItem, response.Outcome, response.Refusal)
		}
		if !strings.Contains(response.Context["item"], "questions/1") {
			t.Errorf("the refusal names the item %q", response.Context["item"])
		}
		if response.Context["column"] != "beta" {
			t.Errorf("the refusal names the column %q, wanted beta", response.Context["column"])
		}
		if card := h.card(ref); card.Route != "" {
			t.Errorf("the refused write stored the route %q", card.Route)
		}
	})

	t.Run("the same item settled takes the write", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("carrying a settled question for beta")
		fileItem(t, h, ref, "open_question", "who answered at beta?")
		h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: ref + "/questions/1",
			Field: bench.ItemColumnField, Value: "beta",
		})
		h.reopen()
		if resolved := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: ref + "/questions/1", Text: "the operator did"}); resolved.Outcome != contract.OutcomeOK {
			t.Fatalf("resolve the item: %s %s", resolved.Outcome, resolved.Refusal)
		}
		h.reopen()
		response := write(h, ref, "skipbeta")
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("a settled item stranded the write: %s %s", response.Outcome, response.Refusal)
		}
	})

	t.Run("a route that carries the item's column takes the write", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("carrying a question for beta")
		fileItem(t, h, ref, "open_question", "who answers at beta?")
		h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: ref + "/questions/1",
			Field: bench.ItemColumnField, Value: "beta",
		})
		h.reopen()
		if response := write(h, ref, "skipalpha"); response.Outcome != contract.OutcomeOK {
			t.Fatalf("a road carrying the item's column was refused: %s %s", response.Outcome, response.Refusal)
		}
	})

	t.Run("the write needs no claim and journals both names", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("nobody holds this")
		before := len(h.events(ref))
		if response := write(h, ref, "skipbeta"); response.Outcome != contract.OutcomeOK {
			t.Fatalf("a route write on an unheld card was refused: %s %s", response.Outcome, response.Refusal)
		}
		events := h.events(ref)
		if len(events) != before+1 {
			t.Fatalf("the write appended %d journal lines, wanted one", len(events)-before)
		}
		line := events[len(events)-1]
		if line.Event != contract.EventCardUpdated || line.Field != bench.RouteField {
			t.Errorf("the write recorded %s on the field %q", line.Event, line.Field)
		}
		if line.From != "" || line.To != "skipbeta" {
			t.Errorf("the line carries from %q and to %q, wanted the empty previous name and skipbeta", line.From, line.To)
		}
		if line.Actor.Name != "brin" {
			t.Errorf("the line carries the actor %q, and brin asked", line.Actor.Name)
		}

		// A clear appends the same event with the new name absent.
		cleared := h.library.SetField(&Request{
			Verb: "set", Actor: "brin", Ref: ref, Field: bench.RouteField, Value: "",
		})
		h.reopen()
		if cleared.Outcome != contract.OutcomeOK {
			t.Fatalf("clearing the route was refused: %s %s", cleared.Outcome, cleared.Refusal)
		}
		events = h.events(ref)
		line = events[len(events)-1]
		if line.From != "skipbeta" || line.To != "" {
			t.Errorf("the clear carries from %q and to %q", line.From, line.To)
		}
	})
}

// TestARouteWriteRefusesSkippingAnOperatorColumn is dinah-542/criteria/22,
// under the answer the contract is written to. A route omitting an
// operator-owned column the card has not yet passed is refused, and the three
// accepting cases pass beside it.
func TestARouteWriteRefusesSkippingAnOperatorColumn(t *testing.T) {
	// The routed fixture declares no operator-owned column, so this one is
	// declared here: beta becomes the operator's, and skipbeta is the road
	// that drops it.
	h := routedHarness(t)
	h.declare(routedBeta, "operator_owned", "true")

	write := func(ref, route string) *Response {
		response := h.library.SetField(&Request{
			Verb: "set", Actor: "brin", Ref: ref, Field: bench.RouteField, Value: route,
		})
		h.reopen()
		return response
	}

	ahead := h.add("standing before the reserved station")
	response := write(ahead, "skipbeta")
	if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.RouteSkipsOperatorColumn {
		t.Fatalf("wanted %s, got %s %s", contract.RouteSkipsOperatorColumn, response.Outcome, response.Refusal)
	}
	if response.Context["column"] != "beta" {
		t.Errorf("the refusal names the column %q, wanted beta", response.Context["column"])
	}

	// Past the station, the same road is written without complaint.
	past := h.add("standing past the reserved station")
	h.at(past, routedGamma)
	if response := write(past, "skipbeta"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("a card already past the station was refused: %s %s", response.Outcome, response.Refusal)
	}

	// A road carrying every operator-owned column ahead of the card is written.
	carrying := h.add("taking a road that keeps the station")
	if response := write(carrying, "skipalpha"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("a road carrying the reserved station was refused: %s %s", response.Outcome, response.Refusal)
	}

	// On a workbench declaring no operator-owned column, any road is written.
	plain := routedHarness(t)
	elsewhere := plain.add("on a workbench reserving nothing")
	unreserved := plain.library.SetField(&Request{
		Verb: "set", Actor: "brin", Ref: elsewhere, Field: bench.RouteField, Value: "skipbeta",
	})
	plain.reopen()
	if unreserved.Outcome != contract.OutcomeOK {
		t.Fatalf("a workbench declaring no reserved column refused the write: %s %s", unreserved.Outcome, unreserved.Refusal)
	}
}

// TestFilingAnItemOffTheRouteIsRefused is dinah-542/criteria/5. An item naming
// a column the card's own road does not carry is a hold that would never fire,
// and it is refused under Dinah's own name, with every accepting case beside
// it.
func TestFilingAnItemOffTheRouteIsRefused(t *testing.T) {
	file := func(h *harness, ref, column string) *Response {
		response := h.library.File(&Request{
			Verb: "file", Actor: "alka", Card: ref,
			Kind: "open_question", Text: "who answers this?", Column: column,
		})
		h.reopen()
		return response
	}

	t.Run("a column the road drops refuses", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("on the short road")
		h.onRoute(ref, "skipbeta")
		response := file(h, ref, "beta")
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.ItemOffRoute {
			t.Fatalf("wanted %s, got %s %s", contract.ItemOffRoute, response.Outcome, response.Refusal)
		}
		if response.Detail != "beta" {
			t.Errorf("the refusal names %q, wanted beta", response.Detail)
		}
		if response.Context["route"] != "skipbeta" {
			t.Errorf("the refusal names the route %q", response.Context["route"])
		}
	})

	t.Run("a card carrying no route takes the same item", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("on the full column list")
		if response := file(h, ref, "beta"); response.Outcome != contract.OutcomeOK {
			t.Fatalf("a card on the full list was refused: %s %s", response.Outcome, response.Refusal)
		}
	})

	t.Run("a road carrying the column takes the same item", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("on a road that keeps beta")
		h.onRoute(ref, "skipalpha")
		if response := file(h, ref, "beta"); response.Outcome != contract.OutcomeOK {
			t.Fatalf("a road carrying the column was refused: %s %s", response.Outcome, response.Refusal)
		}
	})

	t.Run("an item naming no column at all is filed", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("on the short road")
		h.onRoute(ref, "skipbeta")
		if response := file(h, ref, ""); response.Outcome != contract.OutcomeOK {
			t.Fatalf("an item naming no column was refused: %s %s", response.Outcome, response.Refusal)
		}
	})

	t.Run("moving an item onto a dropped column refuses", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("on the short road")
		h.onRoute(ref, "skipbeta")
		fileItem(t, h, ref, "open_question", "who answers this?")
		response := h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: ref + "/questions/1",
			Field: bench.ItemColumnField, Value: "beta",
		})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.ItemOffRoute {
			t.Fatalf("wanted %s, got %s %s", contract.ItemOffRoute, response.Outcome, response.Refusal)
		}
	})
}

// TestAddRefusesAColumnTheRouteDrops is dinah-542/criteria/21. One refusal
// covers both halves: a named column the road drops, and a bare filing into a
// first column the road drops.
func TestAddRefusesAColumnTheRouteDrops(t *testing.T) {
	added := func(h *harness, route, column string) *Response {
		response := h.library.Add(&Request{
			Verb: "add", Actor: "alka", Title: "a filing", Route: route, Column: column,
		})
		h.reopen()
		return response
	}

	t.Run("a named column the road drops refuses", func(t *testing.T) {
		h := routedHarness(t)
		response := added(h, "skipbeta", "beta")
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.RouteOffColumn {
			t.Fatalf("wanted %s, got %s %s", contract.RouteOffColumn, response.Outcome, response.Refusal)
		}
		if response.Detail != "beta" || response.Context["route"] != "skipbeta" {
			t.Errorf("the refusal names %q on the route %q", response.Detail, response.Context["route"])
		}
	})

	t.Run("a road that drops the first column refuses a bare filing", func(t *testing.T) {
		h := harnessFromDefinition(t, "nd", noDoorDefinition)
		response := added(h, "nodoor", "")
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.RouteOffColumn {
			t.Fatalf("wanted %s, got %s %s", contract.RouteOffColumn, response.Outcome, response.Refusal)
		}
	})

	t.Run("the three accepting cases file", func(t *testing.T) {
		h := routedHarness(t)
		if response := added(h, "skipbeta", ""); response.Outcome != contract.OutcomeOK {
			t.Errorf("--route alone was refused: %s %s", response.Outcome, response.Refusal)
		}
		if response := added(h, "skipbeta", "gamma"); response.Outcome != contract.OutcomeOK {
			t.Errorf("--route with a column the road carries was refused: %s %s", response.Outcome, response.Refusal)
		}
		if response := added(h, "", "beta"); response.Outcome != contract.OutcomeOK {
			t.Errorf("--column alone was refused: %s %s", response.Outcome, response.Refusal)
		}
	})

	t.Run("an unknown route is still refused by its own name", func(t *testing.T) {
		h := routedHarness(t)
		response := added(h, "nonesuch", "")
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.UnknownRoute {
			t.Fatalf("wanted %s, got %s %s", contract.UnknownRoute, response.Outcome, response.Refusal)
		}
	})
}

// noDoorDefinition declares a route that does not carry the workbench's first
// column, which is the configuration check.route-missing-first-column reports
// and which a bare filing on that road meets as a refusal.
const noDoorDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "No door",
  "routes": { "nodoor": ["e00000000002", "e00000000003"] },
  "columns": [
    { "id": "e00000000001", "title": "Intake", "kind": "intake" },
    { "id": "e00000000002", "title": "Doing", "kind": "work" },
    { "id": "e00000000003", "title": "Finished", "kind": "done" }
  ]
}`

// TestTheLoopLimitDoesNotReadTheRoute is dinah-542/criteria/6. A card parked at
// a column its own road does not carry goes on counting its regressive
// departures, and the limit refuses at its declared number exactly as it does
// for a card on the full column list.
//
// The comparison stays on the flow order at all four sites, and that is what
// this pins: a route gives no answer at all for a column it does not carry, so
// an implementation that read the route here would stop counting and the
// published gate would silently switch off.
func TestTheLoopLimitDoesNotReadTheRoute(t *testing.T) {
	for _, route := range []string{"", "skipbeta"} {
		name := "on the full column list"
		if route != "" {
			name = "standing off the road " + route
		}
		t.Run(name, func(t *testing.T) {
			h := routedHarness(t)
			h.declare(routedBeta, "loop_limit", "2")
			ref := h.add("going round")
			if route != "" {
				h.onRoute(ref, route)
			}
			// Beta is the column skipbeta drops, so the routed card is parked
			// off its own road for every departure below.
			h.at(ref, routedBeta)
			for at := 0; at < 2; at++ {
				h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: "alpha"})
				h.at(ref, routedBeta)
			}
			refused := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: "alpha"})
			if refused.Outcome != contract.OutcomeRefused || refused.Refusal != contract.AtLoopLimit {
				t.Fatalf("the third regressive departure answered %s %s, wanted %s",
					refused.Outcome, refused.Refusal, contract.AtLoopLimit)
			}
		})
	}
}

// TestABarePullSkipsAnOperatorOwnedSource is dinah-542/criteria/26, and
// TestANamedPullDoesNotSkipAnOperatorOwnedSource beside it is
// dinah-542/criteria/25. The two forms hold opposite policies about a source
// column the workbench reserves to its operator, and each test fails against an
// implementation applying the other form's rule to both.
func TestABarePullSkipsAnOperatorOwnedSource(t *testing.T) {
	h := routedHarness(t)
	h.declare(routedAlpha, "operator_owned", "true")
	ref := h.add("standing where the operator takes it up")
	h.at(ref, routedAlpha)

	response := h.library.Pull(&Request{Verb: Pull, Actor: "brin"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK || response.Card != nil {
		t.Fatalf("the bare pull answered %s with card %+v, wanted the empty answer", response.Outcome, response.Card)
	}
	if response.Message != "answer.pull.empty.bare" {
		t.Errorf("the bare pull answered %q, wanted the empty message rather than the above-tier one", response.Message)
	}

	// The operator nominates the station and takes the card, which is what
	// stops this passing against a build that answers empty for everybody.
	taken := h.library.Pull(&Request{Verb: Pull, Actor: "alka"})
	h.reopen()
	if taken.Outcome != contract.OutcomeOK || taken.Card == nil || taken.Card.Ref != ref {
		t.Fatalf("the operator's bare pull answered %s with %+v, wanted %s", taken.Outcome, taken.Card, ref)
	}
}

// TestANamedPullDoesNotSkipAnOperatorOwnedSource is dinah-542/criteria/25. The
// named form selects the card standing in a source the workbench reserves,
// reading the caller's identity not at all, so the lock answers not-operator
// and names the column rather than leaving the caller with the empty answer for
// a card the board is showing them.
func TestANamedPullDoesNotSkipAnOperatorOwnedSource(t *testing.T) {
	h := routedHarness(t)
	h.declare(routedAlpha, "operator_owned", "true")
	ref := h.add("standing where the operator takes it up")
	h.at(ref, routedAlpha)

	took, response := h.pulled("brin", "beta")
	if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotOperator {
		t.Fatalf("the named pull answered %s %s, wanted %s", response.Outcome, response.Refusal, contract.NotOperator)
	}
	if took != "" && took != ref {
		t.Errorf("the refusal carries the card %q", took)
	}

	taken, answer := h.pulled("alka", "beta")
	if taken != ref {
		t.Fatalf("the operator's named pull took %q, wanted %s: %s %s", taken, ref, answer.Outcome, answer.Refusal)
	}
}

// TestANamedPullDoesNotSkipAnOperatorOwnedSourceFurtherBack is
// dinah-542/criteria/25 at the position the further walk reaches. The reserved
// column stands behind the destination's immediate upstream, so the card is
// found by the walk over the further sources rather than by the first step, and
// that walk is where a predicate carrying the bare form's policy would skip it.
func TestANamedPullDoesNotSkipAnOperatorOwnedSourceFurtherBack(t *testing.T) {
	h := routedHarness(t)
	h.declare(routedQueue, "operator_owned", "true")
	ref := h.add("waiting in a buffer the operator reserves")
	h.at(ref, routedQueue)
	h.onRoute(ref, "skipalpha")

	_, response := h.pulled("brin", "beta")
	if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotOperator {
		t.Fatalf("the named pull answered %s %s %q, wanted %s", response.Outcome, response.Refusal, response.Message, contract.NotOperator)
	}
	// The refusal's detail is the caller, which is what not-operator names
	// everywhere it is raised, and the card it carries names the reserved
	// column the card stands in, which is how the caller learns where.
	if response.Card == nil || response.Card.Column != routedQueue {
		t.Errorf("the refusal carries the card %+v, wanted the card standing in the reserved queue", response.Card)
	}

	taken, answer := h.pulled("alka", "beta")
	if taken != ref {
		t.Fatalf("the operator's named pull took %q, wanted %s: %s %s", taken, ref, answer.Outcome, answer.Refusal)
	}
}

// TestColumnNewJudgesDisruptionPerCard is dinah-542/criteria/13. The guard
// reads each live card's own road, so a placement that would change where a
// pull carries any live card is refused and one that changes no live card's
// destination is admitted even where it changes the flow-derived answer for a
// column holding nothing.
func TestColumnNewJudgesDisruptionPerCard(t *testing.T) {
	place := func(h *harness) *Response {
		response := h.library.NewColumn(&Request{
			Verb: "column", Actor: "alka", Column: "Wedged", Kind: contract.KindWork, Before: "alpha",
		})
		h.reopen()
		return response
	}

	t.Run("a placement changing a live card's destination refuses", func(t *testing.T) {
		h := routedHarness(t)
		ref := h.add("standing in the buffer on the full list")
		h.at(ref, routedQueue)
		response := place(h)
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.ColumnRoutingDisrupted {
			t.Fatalf("wanted %s, got %s %s", contract.ColumnRoutingDisrupted, response.Outcome, response.Refusal)
		}
	})

	t.Run("a placement changing no live card's destination is admitted", func(t *testing.T) {
		h := routedHarness(t)
		// The buffer's answer on the full column list is alpha, and the new
		// station would change it. The only card standing there walks
		// skipalpha, which carries into beta whether or not the new station
		// exists, because no route names a column that does not exist yet.
		// So the flow-derived answer for the column changes and no live card's
		// does, which is the position that tells a guard reading cards from a
		// guard reading occupied columns.
		ref := h.add("standing in the buffer on a short road")
		h.at(ref, routedQueue)
		h.onRoute(ref, "skipalpha")
		response := place(h)
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("a placement disturbing no live card was refused: %s %s", response.Outcome, response.Refusal)
		}
	})
}

// TestTheRouteReachesTheCardView is dinah-542/criteria/15. Two cards standing
// in one column on two roads report two destinations, and the column view goes
// on reporting the destination the full column list gives.
func TestTheRouteReachesTheCardView(t *testing.T) {
	h := routedHarness(t)
	routed := h.add("on the short road")
	h.at(routed, routedQueue)
	h.onRoute(routed, "skipalpha")
	plain := h.add("on the full column list")
	h.at(plain, routedQueue)

	routedView := h.view(routed)
	if routedView.Route != "skipalpha" {
		t.Errorf("the routed card reports the route %q", routedView.Route)
	}
	if routedView.PullDestination != "beta" {
		t.Errorf("the routed card reports the destination %q, wanted beta", routedView.PullDestination)
	}
	plainView := h.view(plain)
	if plainView.Route != "" {
		t.Errorf("the card on the full list reports the route %q", plainView.Route)
	}
	if plainView.PullDestination != "alpha" {
		t.Errorf("the card on the full list reports the destination %q, wanted alpha", plainView.PullDestination)
	}

	// A card standing after the column its road drops reports the next
	// station of its road, which a road indexed by the flow's position reads
	// wrongly.
	after := h.add("standing after the dropped buffer")
	h.at(after, routedAlpha)
	h.onRoute(after, "direct")
	if destination := h.view(after).PullDestination; destination != "beta" {
		t.Errorf("the card after the dropped buffer reports the destination %q, wanted beta", destination)
	}

	columns, err := h.library.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	for _, column := range columns {
		if column.ID != routedQueue {
			continue
		}
		if column.PullDestination != "alpha" {
			t.Errorf("the column reports the destination %q, wanted alpha, which is the full list's answer", column.PullDestination)
		}
	}
}

// view reads one card's published view.
func (h *harness) view(ref string) *CardView {
	h.t.Helper()
	view, err := h.library.view(h.card(ref))
	if err != nil {
		h.t.Fatalf("view %s: %v", ref, err)
	}
	return view
}

// TestTheRoutesListingNamesWhatEachRoadSkips is dinah-542/criteria/14. The
// listing prints each declared route with the live columns it omits and marks
// the ones the workbench reserves to its operator.
func TestTheRoutesListingNamesWhatEachRoadSkips(t *testing.T) {
	h := routedHarness(t)
	h.declare(routedBeta, "operator_owned", "true")
	listing := h.library.Routes()
	if len(listing.Routes) != 4 {
		t.Fatalf("the listing carries %d routes and the workbench declares four", len(listing.Routes))
	}
	if listing.Routes[0].Name != "skipbeta" {
		t.Errorf("the listing leads with %q, and the declaration leads with skipbeta", listing.Routes[0].Name)
	}
	first := listing.Routes[0]
	if first.Columns != 5 {
		t.Errorf("skipbeta carries %d columns, wanted five of the six", first.Columns)
	}
	if len(first.Skips) != 1 || first.Skips[0] != "beta" {
		t.Errorf("skipbeta skips %v, wanted beta alone", first.Skips)
	}
	if len(first.OperatorOwnedSkips) != 1 || first.OperatorOwnedSkips[0] != "beta" {
		t.Errorf("skipbeta reports the reserved skips as %v", first.OperatorOwnedSkips)
	}
	// The second road drops a station the workbench does not reserve, so it
	// names the skip and marks none of it, which is what tells the two columns
	// of the row apart.
	second := listing.Routes[1]
	if len(second.Skips) != 1 || second.Skips[0] != "alpha" {
		t.Errorf("skipalpha skips %v, wanted alpha alone", second.Skips)
	}
	if len(second.OperatorOwnedSkips) != 0 {
		t.Errorf("skipalpha reports reserved skips %v and the workbench reserves none of them", second.OperatorOwnedSkips)
	}
}

// TestTheRouteReachesTheQueryAndTheTree is the rest of dinah-542/criteria/14.
// A query selects the cards on a road and the cards on none, and the tree nests
// along the route axis.
func TestTheRouteReachesTheQueryAndTheTree(t *testing.T) {
	h := routedHarness(t)
	routed := h.add("on the short road")
	h.onRoute(routed, "skipbeta")
	plain := h.add("on the full column list")

	carrying, err := h.library.Query(&Request{Verb: "query", Actor: "alka", Query: "route:skipbeta"})
	if err != nil {
		t.Fatalf("query the road: %v", err)
	}
	if len(carrying.Cards) != 1 || carrying.Cards[0].Ref != routed {
		t.Errorf("route:skipbeta selected %v, wanted %s alone", refsOfCards(carrying.Cards), routed)
	}
	absent, err := h.library.Query(&Request{Verb: "query", Actor: "alka", Query: `route:""`})
	if err != nil {
		t.Fatalf("query the absence: %v", err)
	}
	if len(absent.Cards) != 1 || absent.Cards[0].Ref != plain {
		t.Errorf(`route:"" selected %v, wanted %s alone`, refsOfCards(absent.Cards), plain)
	}

	grouped := false
	for _, axis := range GroupAxes() {
		if axis == FieldRoute {
			grouped = true
		}
	}
	if !grouped {
		t.Error("the tree's disposition table does not admit route as an axis")
	}
}

// TestABarePullReportsWorkStandingAboveTheCaller is dinah-542/criteria/19. The
// above-tier answer survives the selection being keyed on the card: the only
// ready card stands on a short road and carries into the destination, and the
// caller's declared tier is admitted for none of it.
func TestABarePullReportsWorkStandingAboveTheCaller(t *testing.T) {
	h := tieredRouteHarness(t)
	ref := h.add("wanted by a more senior caller")
	h.at(ref, "c00000000002")
	h.onRoute(ref, "short")
	h.library.SetField(&Request{Verb: "set", Actor: "alka", Ref: ref, Field: bench.TierField, Value: "frontier"})
	h.reopen()

	withheld := h.library.Pull(&Request{
		Verb: Pull, Actor: "brin", Provider: "acme", Model: "workhorse",
	})
	h.reopen()
	if withheld.Card != nil {
		t.Fatalf("the bare pull took %s, and the caller's tier is admitted for none of it", withheld.Card.Ref)
	}
	if withheld.Message != "answer.pull.above-tier.bare" {
		t.Fatalf("the bare pull answered %q, wanted the above-tier message", withheld.Message)
	}

	taken := h.library.Pull(&Request{
		Verb: Pull, Actor: "brin", Provider: "acme", Model: "frontier",
	})
	h.reopen()
	if taken.Card == nil || taken.Card.Ref != ref {
		t.Fatalf("the admitted caller answered %s %s %q with %+v, wanted %s", taken.Outcome, taken.Refusal, taken.Message, taken.Card, ref)
	}
}

// tieredRouteDefinition declares a short road over a buffer, and
// tieredRouteHarness adds the tier table the way tieredHarness does, so the
// above-tier answer can be reached over a card the road is carrying.
const tieredRouteDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Tiered",
  "routes": { "short": ["c00000000001", "c00000000002", "c00000000004", "c00000000005"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Queue", "kind": "dinah.buffer" },
    { "id": "c00000000003", "title": "Skipped", "kind": "work" },
    { "id": "c00000000004", "title": "Station", "kind": "work" },
    { "id": "c00000000005", "title": "Finished", "kind": "done" }
  ]
}`

// tieredRouteHarness builds the tiered road and declares two rungs on it, each
// listing one model, written into the anchor in the shape tieredHarness writes
// them in.
func tieredRouteHarness(t *testing.T) *harness {
	t.Helper()
	h := harnessFromDefinition(t, "tr", tieredRouteDefinition)
	anchor := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(anchor)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw("levels", []string{"levels:", "  tier: [workhorse, frontier]"})
	fm.SetRaw("tiers", []string{
		"tiers:",
		"  workhorse:",
		"    meaning: scoped implementation against a contract",
		"    models:",
		"      - {provider: acme, model: workhorse}",
		"  frontier:",
		"    meaning: novel design judgement",
		"    models:",
		"      - {provider: acme, model: frontier}",
	})
	if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()
	return h
}

// TestTheRouteFilterRunsBeforeTheAboveTierObservation is the second clause of
// dinah-542/criteria/26. A tier-gated card standing in a column the bare form's
// source predicate excluded does not report work standing above the caller,
// because the predicate runs first.
func TestTheRouteFilterRunsBeforeTheAboveTierObservation(t *testing.T) {
	h := tieredRouteHarness(t)
	h.declare("c00000000002", "operator_owned", "true")
	ref := h.add("gated and standing where the operator takes it up")
	h.at(ref, "c00000000002")
	h.library.SetField(&Request{Verb: "set", Actor: "alka", Ref: ref, Field: bench.TierField, Value: "frontier"})
	h.reopen()

	response := h.library.Pull(&Request{
		Verb: Pull, Actor: "brin", Provider: "acme", Model: "workhorse",
	})
	h.reopen()
	if response.Message != "answer.pull.empty.bare" {
		t.Fatalf("the bare pull answered %q, wanted the empty message: the excluded column's gated card must not be reported as work above the caller", response.Message)
	}
}
