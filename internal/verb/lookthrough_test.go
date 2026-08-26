package verb

import (
	"testing"
	"time"

	"dinah/internal/contract"
)

// TestANamedPullLooksThroughAnEmptyBuffer is dinah-273 AC-33. The immediate
// upstream holds nothing, so the pull reaches behind it to the column that
// carries into this destination, and the card goes straight to the station
// without being written into the buffer on the way.
func TestANamedPullLooksThroughAnEmptyBuffer(t *testing.T) {
	h := newBufferHarness(t)
	ref := h.add("waiting at the intake column")

	response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: "doing"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("pull: %s %s", response.Outcome, response.Refusal)
	}
	if response.Card == nil || response.Card.Ref != ref {
		t.Fatalf("the pull took %+v, and the only ready card is %s", response.Card, ref)
	}
	card := h.card(ref)
	if card.Column != bufferDoing || card.State != contract.StateActive || card.Holder != "bo" {
		t.Fatalf("the card reads column %q state %q holder %q", card.Column, card.State, card.Holder)
	}
	claimed, moved := 0, 0
	var departure string
	for _, event := range h.events(ref) {
		switch event.Event {
		case contract.EventClaimed:
			claimed++
		case contract.EventMoved:
			moved++
			departure = event.From
		}
	}
	if claimed != 1 || moved != 1 {
		t.Errorf("the journal holds %d claimed and %d moved events, and a pull appends one of each", claimed, moved)
	}
	if departure != bufferIntake {
		t.Errorf("the moved event names %q as the departure, and the card left the intake column", departure)
	}
	// Nothing was written into the buffer on the way, which is what makes this
	// one act rather than two.
	listing, err := h.library.List(&Request{Verb: "ls", Column: bufferQueue})
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if len(listing.Cards) != 0 {
		t.Errorf("the buffer holds %d cards and the pull looked through it", len(listing.Cards))
	}
}

// TestAFullBufferDrainsBeforeThePullReachesBehindIt is dinah-273 AC-34. Nearest
// first is flow order reversed, so the card furthest along goes first.
func TestAFullBufferDrainsBeforeThePullReachesBehindIt(t *testing.T) {
	h := newBufferHarness(t)
	behind := h.add("waiting at the intake column")
	h.advance(time.Hour)
	inTheBuffer := h.add("waiting in the buffer")
	h.at(inTheBuffer, bufferQueue)

	response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: "doing"})
	h.reopen()
	if response.Card == nil || response.Card.Ref != inTheBuffer {
		t.Fatalf("the pull took %+v, and the buffer's card is nearest", response.Card)
	}
	if card := h.card(behind); card.Column != bufferIntake || card.State != contract.StateReady {
		t.Errorf("the card behind the buffer moved: column %q state %q", card.Column, card.State)
	}
}

// TestAPullDoesNotReachPastAStationThatHoldsACard is dinah-273 AC-35. A card
// standing where somebody works is taken from where it stands rather than
// carried past a queue, so a pull naming the column beyond the queue finds
// nothing and says so.
func TestAPullDoesNotReachPastAStationThatHoldsACard(t *testing.T) {
	h := harnessFromDefinition(t, "ps", `{
  "profile": "dinah-core/0.5",
  "title": "Station then buffer",
  "columns": [
    { "id": "100000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "100000000002", "title": "First", "slug": "first", "kind": "work" },
    { "id": "100000000003", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" },
    { "id": "100000000004", "title": "Second", "slug": "second", "kind": "work" }
  ]
}`)
	ref := h.add("standing where somebody works")
	h.at(ref, "100000000002")
	before := len(h.events(ref))

	response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: "second"})
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("wanted the empty answer, got %s %s", response.Outcome, response.Refusal)
	}
	if response.Card != nil {
		t.Fatalf("the pull reached past a station and took %+v", response.Card)
	}
	if got := len(h.events(ref)); got != before {
		t.Errorf("the empty answer wrote %d events", got-before)
	}
}

// TestTheLookThroughRemovesNoRefusalTheNamedFormOwes is dinah-273 AC-36. The
// immediate upstream is tried first and on its own terms, so a done upstream
// still answers terminal and one waiting on somebody outside still answers its
// own name rather than going quiet.
func TestTheLookThroughRemovesNoRefusalTheNamedFormOwes(t *testing.T) {
	t.Run("a done upstream still answers terminal", func(t *testing.T) {
		h := harnessFromDefinition(t, "du", `{
  "profile": "dinah-core/0.5",
  "title": "Done upstream",
  "columns": [
    { "id": "200000000001", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "200000000002", "title": "Finished", "slug": "finished", "kind": "done" },
    { "id": "200000000003", "title": "Aftercare", "slug": "aftercare", "kind": "work" }
  ]
}`)
		ref := h.add("resting in the done column")
		h.at(ref, "200000000002")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: "aftercare"})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.Terminal {
			t.Fatalf("wanted %s, got %s %s", contract.Terminal, response.Outcome, response.Refusal)
		}
		if response.Detail != "finished" {
			t.Errorf("the refusal names %q, and the departure is the finished column", response.Detail)
		}
	})

	t.Run("an upstream waiting on somebody outside still answers its own name", func(t *testing.T) {
		h := harnessFromDefinition(t, "ou", `{
  "profile": "dinah-core/0.5",
  "title": "Outside upstream",
  "columns": [
    { "id": "200000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "200000000002", "title": "Outside", "slug": "outside", "kind": "work", "awaiting_outside": true },
    { "id": "200000000003", "title": "Doing", "slug": "doing", "kind": "work" }
  ]
}`)
		ref := h.add("waiting on the customer")
		h.at(ref, "200000000002")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: "doing"})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.AwaitingOutside {
			t.Fatalf("wanted %s, got %s %s", contract.AwaitingOutside, response.Outcome, response.Refusal)
		}
	})
}

// TestAnOperatorOwnedSourceAnswersInWordsRatherThanSilence is dinah-273 AC-39.
// Step three of the named form reads no caller, so the operator's own pull
// takes the card and anybody else's is refused not-operator under the lock,
// which is a better answer than the empty one for a card the board shows them.
func TestAnOperatorOwnedSourceAnswersInWordsRatherThanSilence(t *testing.T) {
	const flow = `{
  "profile": "dinah-core/0.5",
  "title": "Operator-owned buffer",
  "columns": [
    { "id": "300000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "300000000002", "title": "Reserved", "slug": "reserved", "kind": "dinah.buffer", "operator_owned": true },
    { "id": "300000000003", "title": "Waiting", "slug": "waiting", "kind": "dinah.buffer" },
    { "id": "300000000004", "title": "Doing", "slug": "doing", "kind": "work" }
  ]
}`

	t.Run("the operator takes the card", func(t *testing.T) {
		h := harnessFromDefinition(t, "oo", flow)
		ref := h.add("standing in the reserved buffer")
		h.at(ref, "300000000002")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: "doing"})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("the operator's pull: %s %s", response.Outcome, response.Refusal)
		}
		card := h.card(ref)
		if card.Column != "300000000004" || card.State != contract.StateActive || card.Holder != "alka" {
			t.Errorf("the card reads column %q state %q holder %q", card.Column, card.State, card.Holder)
		}
	})

	t.Run("anybody else is refused in words", func(t *testing.T) {
		h := harnessFromDefinition(t, "oo", flow)
		ref := h.add("standing in the reserved buffer")
		h.at(ref, "300000000002")
		response := h.library.Pull(&Request{Verb: Pull, Actor: "bo", Column: "doing"})
		h.reopen()
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotOperator {
			t.Fatalf("wanted %s rather than the empty answer, got %s %s", contract.NotOperator, response.Outcome, response.Refusal)
		}
	})
}
