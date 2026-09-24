package verb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// standingBlock is the wedding column's declaration from the dinah-593
// contract, planted on whichever column a case names.
const standingBlock = `standing_items:
  deposit-paid:
    kind: acceptance_criterion
    text: The vendor's deposit has been paid and the receipt is on the card.
    evidence: receipt
  contract-countersigned:
    kind: acceptance_criterion
    text: Both parties have signed the vendor contract.
    owner: operator
  date-confirmed:
    kind: open_question
    text: Has the vendor confirmed the wedding date in writing?
`

// standingKeys are the block's entry keys in declaration order.
var standingKeys = []string{"deposit-paid", "contract-countersigned", "date-confirmed"}

// declareStanding writes a standing_items block into one column's own anchor
// and reopens the bench. No verb writes the block: it is frontmatter a person
// edits, on the terms declareEvidence states for the workbench's block.
func (h *harness) declareStanding(id, block string) {
	h.t.Helper()
	path := filepath.Join(h.root, bench.ColumnsDir, id, bench.ColumnAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		h.t.Fatalf("read the anchor of %s: %v", id, err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.StandingItemsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		h.t.Fatalf("write the anchor of %s: %v", id, err)
	}
	h.reopen()
}

// items reads a card's live checklist in creation order.
func (h *harness) items(ref string) []*bench.Item {
	h.t.Helper()
	items, err := bench.Items(h.card(ref).Dir)
	if err != nil {
		h.t.Fatalf("items of %s: %v", ref, err)
	}
	return items
}

// instancesOf answers the live items of a card that a standing declaration
// minted for one column, keyed by entry key, and fails the test if a key is
// carried twice, which is the duplication the identity rule exists to prevent.
func (h *harness) instancesOf(ref, column string) map[string]*bench.Item {
	h.t.Helper()
	found := map[string]*bench.Item{}
	for _, item := range h.items(ref) {
		if item.Column != column || item.Standing == "" {
			continue
		}
		if _, twice := found[item.Standing]; twice {
			h.t.Fatalf("%s carries %s for %s twice", ref, item.Standing, column)
		}
		found[item.Standing] = item
	}
	return found
}

// assertMinted holds a card's checklist to exactly one pending instance per
// entry of the block, in declaration order, each carrying the declared
// column, key, kind, text, owner and evidence, and answers the instances.
func (h *harness) assertMinted(ref, column string) []*bench.Item {
	h.t.Helper()
	items := h.items(ref)
	if len(items) != len(standingKeys) {
		h.t.Fatalf("%s carries %d items, wanted %d", ref, len(items), len(standingKeys))
	}
	want := []*bench.Item{
		{Kind: "acceptance_criterion", Standing: "deposit-paid", Evidence: "receipt", Text: "The vendor's deposit has been paid and the receipt is on the card."},
		{Kind: "acceptance_criterion", Standing: "contract-countersigned", Owner: "operator", Text: "Both parties have signed the vendor contract."},
		{Kind: "open_question", Standing: "date-confirmed", Text: "Has the vendor confirmed the wedding date in writing?"},
	}
	for at, item := range items {
		expected := want[at]
		if item.State != bench.ItemPending || item.Column != column || item.Standing != expected.Standing ||
			item.Kind != expected.Kind || item.Text != expected.Text || item.Owner != expected.Owner ||
			item.Evidence != expected.Evidence || item.Ordinal != at+1 {
			h.t.Errorf("item %d of %s reads %+v, wanted %+v pending at %s", at+1, ref, item, expected, column)
		}
	}
	return items
}

// settle runs one checklist verb against an item by its reference, as the
// operator, and fails the test unless it succeeded. The item verbs are not
// among the seven Library.Do dispatches, so a test reaches them by name.
func (h *harness) settle(verb, ref, text string) {
	h.t.Helper()
	req := &Request{Verb: verb, Actor: "alka", Ref: ref, Text: text}
	var response *Response
	switch verb {
	case "resolve":
		response = h.library.Resolve(req)
	case "verify":
		response = h.library.Verify(req)
	case "fail":
		response = h.library.Fail(req)
	case "waive":
		response = h.library.Waive(req)
	case "withdraw":
		response = h.library.Withdraw(req)
	default:
		h.t.Fatalf("settle knows no verb %s", verb)
	}
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("%s %s: %s %s %q", verb, ref, response.Outcome, response.Refusal, response.Detail)
	}
	h.reopen()
}

// attempt runs one checklist verb against an item and answers the response,
// without deciding whether it is the one the case wanted.
func (h *harness) attempt(verb, actor, ref, text string) *Response {
	h.t.Helper()
	req := &Request{Verb: verb, Actor: actor, Ref: ref, Text: text}
	var response *Response
	switch verb {
	case "resolve":
		response = h.library.Resolve(req)
	case "verify":
		response = h.library.Verify(req)
	case "fail":
		response = h.library.Fail(req)
	case "waive":
		response = h.library.Waive(req)
	case "withdraw":
		response = h.library.Withdraw(req)
	default:
		h.t.Fatalf("attempt knows no verb %s", verb)
	}
	h.reopen()
	return response
}

// cite writes one citation on an item as the operator.
func (h *harness) cite(ref, scheme, target string) {
	h.t.Helper()
	response := h.library.Cite(&Request{Verb: "cite", Actor: "alka", Ref: ref, Scheme: scheme, CiteTarget: target})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("cite %s %s: %s %s", ref, scheme, response.Outcome, response.Refusal)
	}
	h.reopen()
}

// instanceIDs answers the identifiers of a card's live items as a set, which
// is what a refusal naming the first holding item in identifier order is held
// against.
func (h *harness) instanceIDs(ref string) map[string]bool {
	h.t.Helper()
	ids := map[string]bool{}
	for _, item := range h.items(ref) {
		ids[item.ID] = true
	}
	return ids
}

// filedLines answers the item_filed lines of a card's journal, in order.
func (h *harness) filedLines(ref string) []bench.Event {
	h.t.Helper()
	var filed []bench.Event
	for _, ev := range h.events(ref) {
		if ev.Event == contract.EventItemFiled {
			filed = append(filed, ev)
		}
	}
	return filed
}

// TestAnArrivalMintsTheColumnsStandingItems drives criterion 1, which is
// CORE-GATE-5 as Dinah reads it, across the four arrivals: a move, a pull, a filing straight into the column and a
// reshape carrying a card there. Each mints exactly one pending instance per
// entry, asserted by count, with every member the entry declares.
func TestAnArrivalMintsTheColumnsStandingItems(t *testing.T) {
	t.Run("by move", func(t *testing.T) {
		h := newHarness(t)
		h.declareStanding(aftercare, standingBlock)
		ref := h.add("moved in")
		if items := h.items(ref); len(items) != 0 {
			t.Fatalf("a card filed at intake carries %d items before it reaches the declaring column", len(items))
		}
		h.at(ref, aftercare)
		h.assertMinted(ref, aftercare)
	})
	t.Run("by pull", func(t *testing.T) {
		h := newHarness(t)
		h.declareStanding(aftercare, standingBlock)
		ref := h.readyAt("pulled in", review)
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: aftercareSlug})
		h.reopen()
		if response.Outcome != contract.OutcomeOK || response.Card.Ref != ref {
			t.Fatalf("pull: %s %s %+v", response.Outcome, response.Refusal, response.Card)
		}
		h.assertMinted(ref, aftercare)
	})
	t.Run("by add", func(t *testing.T) {
		h := newHarness(t)
		h.declareStanding(aftercare, standingBlock)
		response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "filed in", Column: aftercareSlug})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("add: %s %s", response.Outcome, response.Refusal)
		}
		h.assertMinted(response.Card.Ref, aftercare)
		// The lines land after the created line, at the same instant, and
		// the response counts the items it minted.
		events := h.events(response.Card.Ref)
		if events[0].Event != contract.EventCreated || len(events) != 1+len(standingKeys) {
			t.Fatalf("wanted the created line and three filings, got %+v", events)
		}
		if response.Card.ChecklistCount != len(standingKeys) {
			t.Errorf("the response counts %d items, wanted %d", response.Card.ChecklistCount, len(standingKeys))
		}
	})
	t.Run("by reshape", func(t *testing.T) {
		h := newHarness(t)
		h.declareStanding(review, standingBlock)
		ref := h.readyAt("carried in", aftercare)
		if _, err := h.reshape(h.source(dropsAftercare), true, aftercare+"="+review); err != nil {
			t.Fatalf("reshape: %v", err)
		}
		if column := h.card(ref).Column; column != review {
			t.Fatalf("the card stands at %s, wanted review", column)
		}
		h.assertMinted(ref, review)
		filed := h.filedLines(ref)
		if len(filed) != len(standingKeys) || filed[0].Actor.Name != "alka" || filed[0].ColumnTitle != "Review" {
			t.Errorf("the carry's filings read %+v", filed)
		}
	})
}

// TestAReEntryMintsNothingTwice drives criterion 2, the second half of
// CORE-GATE-5: a card that leaves the
// declaring column by a regressive move and comes back keeps one instance per
// key, whatever state the instance stands in, asserted by count, and a card
// whose instance was archived receives a fresh one.
func TestAReEntryMintsNothingTwice(t *testing.T) {
	settle := map[string]func(h *harness, ref string){
		bench.ItemPending: func(h *harness, ref string) {},
		bench.ItemResolved: func(h *harness, ref string) {
			h.settle("resolve", ref+"/questions/1", "Yes, in writing.")
		},
		bench.ItemVerified: func(h *harness, ref string) {
			h.settle("verify", ref+"/criteria/2", "Both signatures are on file.")
		},
		bench.ItemFailed: func(h *harness, ref string) {
			h.settle("fail", ref+"/criteria/2", "One signature is missing.")
		},
		bench.ItemWaived: func(h *harness, ref string) {
			h.settle("waive", ref+"/criteria/2", "Proceed regardless.")
		},
		bench.ItemWithdrawn: func(h *harness, ref string) {
			h.settle("withdraw", ref+"/criteria/2", "No contract this time.")
		},
	}
	for _, state := range bench.ItemStates {
		t.Run("returning with an instance "+state, func(t *testing.T) {
			h := newHarness(t)
			h.declareStanding(aftercare, standingBlock)
			ref := h.readyAt("looping", aftercare)
			settle[state](h, ref)
			h.at(ref, doing)
			h.at(ref, aftercare)
			items := h.items(ref)
			if len(items) != len(standingKeys) {
				t.Fatalf("after the loop the card carries %d items, wanted %d: %+v", len(items), len(standingKeys), items)
			}
			if len(h.instancesOf(ref, aftercare)) != len(standingKeys) {
				t.Errorf("the instances are not one per key: %+v", items)
			}
			if len(h.filedLines(ref)) != len(standingKeys) {
				t.Errorf("the journal carries %d filings, wanted %d", len(h.filedLines(ref)), len(standingKeys))
			}
		})
	}
	t.Run("returning after an instance was archived", func(t *testing.T) {
		h := newHarness(t)
		h.declareStanding(aftercare, standingBlock)
		ref := h.readyAt("looping", aftercare)
		archived := h.instancesOf(ref, aftercare)["date-confirmed"].ID
		h.archive(ref + "/questions/1")
		h.at(ref, doing)
		h.at(ref, aftercare)
		items := h.items(ref)
		if len(items) != len(standingKeys) {
			t.Fatalf("after the archive and the loop the card carries %d live items, wanted %d", len(items), len(standingKeys))
		}
		if fresh := h.instancesOf(ref, aftercare)["date-confirmed"]; fresh == nil || fresh.ID == archived {
			t.Errorf("wanted a fresh instance of the archived key, got %+v", fresh)
		}
	})
}

// TestTheFilingLineSaysWhichDeclarationFiledIt drives criterion 3: each
// minted instance writes one item_filed line directly after the moved line,
// at the same instant, with the arriving act's actor and the three members a
// hand filing never carries.
func TestTheFilingLineSaysWhichDeclarationFiledIt(t *testing.T) {
	h := newHarness(t)
	h.declareStanding(aftercare, standingBlock)
	ref := h.add("journalled")
	h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: aftercare, Harness: "claude-code"})
	h.file(ref, "decision", "A hand-filed decision.")
	events := h.events(ref)
	var moved int
	for at, ev := range events {
		if ev.Event == contract.EventMoved && ev.To == aftercare {
			moved = at
		}
	}
	instances := h.instancesOf(ref, aftercare)
	for at, key := range standingKeys {
		line := events[moved+1+at]
		instance := instances[key]
		if line.Event != contract.EventItemFiled || line.TS != events[moved].TS || line.Actor.Name != "alka" || line.Actor.Harness != "claude-code" {
			t.Errorf("line %d after the move reads %+v, wanted an item_filed line at the move's instant by its actor", at+1, line)
		}
		if line.Item != instance.ID || line.Kind != instance.Kind || line.Column != aftercare || line.ColumnTitle != "Aftercare" || line.Standing != key {
			t.Errorf("line %d after the move reads %+v, wanted item %s kind %s column %s Aftercare standing %s", at+1, line, instance.ID, instance.Kind, aftercare, key)
		}
	}
	hand := events[len(events)-1]
	if hand.Event != contract.EventItemFiled || hand.Column != "" || hand.ColumnTitle != "" || hand.Standing != "" {
		t.Errorf("the hand filing's line reads %+v, wanted none of the three members", hand)
	}
}

// TestTheHoldNowHasSomethingToHoldOn drives criterion 4 on both directions:
// under out the minted item holds the forward departure and passes the
// regressive one, a failed instance still holds and a verified, waived or
// withdrawn one releases; under both the first arrival is admitted and a
// return is refused by the pending instance in either direction of arrival,
// admitted once settled or carried with the operator's override.
func TestTheHoldNowHasSomethingToHoldOn(t *testing.T) {
	t.Run("out", func(t *testing.T) {
		h := newHarness(t)
		h.declare(aftercare, "gate_items", "out")
		h.declareStanding(aftercare, standingBlock)
		ref := h.readyAt("held", aftercare)
		minted := h.instanceIDs(ref)
		refused := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished})
		if refused.Refusal != contract.UnresolvedItemExit || !minted[refused.Detail] {
			t.Fatalf("the forward move: wanted %s naming a minted instance, got %s %s %q", contract.UnresolvedItemExit, refused.Outcome, refused.Refusal, refused.Detail)
		}
		h.at(ref, doing)
		h.at(ref, aftercare)
		h.settle("fail", ref+"/criteria/2", "Unsigned.")
		if refused := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished}); refused.Refusal != contract.UnresolvedItemExit {
			t.Errorf("after a failed instance the forward move was %s %s, wanted it still refused", refused.Outcome, refused.Refusal)
		}
		// The three settled instances lift the hold together: the criterion
		// with the receipt demand needs its citation first.
		h.cite(ref+"/criteria/1", "receipt", "R-2026-0417")
		h.settle("verify", ref+"/criteria/1", "Paid on the 20th.")
		h.settle("waive", ref+"/criteria/2", "Proceed regardless.")
		h.settle("withdraw", ref+"/questions/1", "The date is fixed elsewhere.")
		if admitted := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished}); admitted.Outcome != contract.OutcomeOK {
			t.Errorf("after every instance settled the forward move was %s %s", admitted.Outcome, admitted.Refusal)
		}
	})
	for _, hold := range []string{"true", "both"} {
		t.Run(hold, func(t *testing.T) {
			h := newHarness(t)
			h.declare(aftercare, "gate_items", hold)
			h.declareStanding(aftercare, standingBlock)
			ref := h.add("returning")
			if first := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: aftercare}); first.Outcome != contract.OutcomeOK {
				t.Fatalf("the first arrival was refused by the items it was about to mint: %s %s", first.Outcome, first.Refusal)
			}
			minted := h.instanceIDs(ref)
			h.at(ref, doing)
			forward := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: aftercare})
			if forward.Refusal != contract.UnresolvedItem || !minted[forward.Detail] {
				t.Fatalf("the forward return: wanted %s naming a minted instance, got %s %s %q", contract.UnresolvedItem, forward.Outcome, forward.Refusal, forward.Detail)
			}
			// The regressive arrival is reached by carrying the card past
			// the column with the override and sending it back.
			h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: aftercare, Override: true})
			h.mustDo(&Request{Verb: Move, Card: ref, Actor: "alka", Column: finished, Override: true})
			if backward := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: aftercare}); backward.Refusal != contract.UnresolvedItem {
				t.Errorf("the regressive return: wanted %s, got %s %s", contract.UnresolvedItem, backward.Outcome, backward.Refusal)
			}
			h.cite(ref+"/criteria/1", "receipt", "R-2026-0417")
			h.settle("fail", ref+"/criteria/1", "The receipt is for the wrong amount.")
			h.settle("verify", ref+"/criteria/2", "Signed.")
			h.settle("resolve", ref+"/questions/1", "Confirmed.")
			failed := h.instancesOf(ref, aftercare)["deposit-paid"]
			if still := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: aftercare}); still.Refusal != contract.UnresolvedItem || still.Detail != failed.ID {
				t.Errorf("with the first instance failed the return was %s %s %q, wanted it refused on %s", still.Outcome, still.Refusal, still.Detail, failed.ID)
			}
			h.settle("waive", ref+"/criteria/1", "The receipt is with the planner.")
			if admitted := h.do(&Request{Verb: Move, Card: ref, Actor: "alka", Column: aftercare}); admitted.Outcome != contract.OutcomeOK {
				t.Errorf("with every instance settled the return was %s %s", admitted.Outcome, admitted.Refusal)
			}
			if len(h.instancesOf(ref, aftercare)) != len(standingKeys) || len(h.items(ref)) != len(standingKeys) {
				t.Errorf("the returns minted a second copy: %+v", h.items(ref))
			}
		})
	}
}

// TestAnItemDemandingASchemeIsSettledAgainstItAlone drives criterion 5: with
// the evidence block declared, the citation obligation runs first, a citation
// of another scheme leaves the demand standing, a citation of the named scheme
// lifts it, and waive and withdraw need no citation at all. The demand is
// asserted on a question as well as on a criterion, because the uncited row
// reaches a criterion alone and the demand has to reach every kind.
func TestAnItemDemandingASchemeIsSettledAgainstItAlone(t *testing.T) {
	h := newHarness(t)
	h.declareEvidence("evidence:\n  receipt: The receipt's number.\n  record: The inspection record's number.\n")
	h.declareStanding(aftercare, strings.Replace(standingBlock,
		"    text: Has the vendor confirmed the wedding date in writing?\n",
		"    text: Has the vendor confirmed the wedding date in writing?\n    evidence: record\n", 1))
	ref := h.readyAt("demanding", aftercare)
	criterion, question := ref+"/criteria/1", ref+"/questions/1"

	if got := h.attempt("verify", "alka", criterion, "Paid."); got.Refusal != contract.Uncited {
		t.Fatalf("with no citation at all: wanted %s first, got %s %s", contract.Uncited, got.Outcome, got.Refusal)
	}
	h.cite(criterion, "record", "BC-2026-04417")
	for _, verb := range []string{"resolve", "verify", "fail"} {
		got := h.attempt(verb, "alka", criterion, "Paid.")
		if verb == "resolve" {
			if got.Refusal != contract.WrongItemKind {
				t.Errorf("resolve on a criterion: wanted %s, got %s", contract.WrongItemKind, got.Refusal)
			}
			continue
		}
		if got.Refusal != contract.EvidenceSchemeRequired || got.Detail != "receipt" {
			t.Errorf("%s with a citation of another scheme: wanted %s with detail receipt, got %s %s %q", verb, contract.EvidenceSchemeRequired, got.Outcome, got.Refusal, got.Detail)
		}
		if got.Context["item"] != criterion {
			t.Errorf("%s: the refusal's extras name %q, wanted the item %s", verb, got.Context["item"], criterion)
		}
	}
	if got := h.attempt("resolve", "alka", question, "Yes."); got.Refusal != contract.EvidenceSchemeRequired || got.Detail != "record" {
		t.Errorf("a question demanding a scheme: wanted %s with detail record, got %s %s %q", contract.EvidenceSchemeRequired, got.Outcome, got.Refusal, got.Detail)
	}
	h.cite(criterion, "receipt", "R-2026-0417")
	if got := h.attempt("verify", "alka", criterion, "Paid."); got.Outcome != contract.OutcomeOK {
		t.Errorf("with the named scheme cited: wanted ok, got %s %s", got.Outcome, got.Refusal)
	}
	h.cite(question, "record", "BC-2026-04417")
	if got := h.attempt("resolve", "alka", question, "Yes."); got.Outcome != contract.OutcomeOK {
		t.Errorf("the question with the named scheme cited: wanted ok, got %s %s", got.Outcome, got.Refusal)
	}

	// waive and withdraw are untouched, on a fresh card carrying the same
	// demands and no citation.
	other := h.readyAt("waived", aftercare)
	if got := h.attempt("waive", "alka", other+"/criteria/1", "Proceed."); got.Outcome != contract.OutcomeOK {
		t.Errorf("waive with no citation: wanted ok, got %s %s", got.Outcome, got.Refusal)
	}
	if got := h.attempt("withdraw", "alka", other+"/questions/1", "Moot."); got.Outcome != contract.OutcomeOK {
		t.Errorf("withdraw with no citation: wanted ok, got %s %s", got.Outcome, got.Refusal)
	}
}

// TestTheEvidenceKeyFollowsTheColumnKeysAuthority drives criterion 6 at the
// library: the key is written and cleared through set, refused to anybody but
// the operator on a criterion and on an operator-owned item and open to any
// owner on every other item, the standing key is no field at all, and the
// views carry both.
func TestTheEvidenceKeyFollowsTheColumnKeysAuthority(t *testing.T) {
	h := newHarness(t)
	h.declareStanding(aftercare, standingBlock)
	ref := h.readyAt("fielded", aftercare)
	question := h.file(ref, "decision", "A hand-filed decision.")
	criterion := ref + "/criteria/1"
	owned := ref + "/questions/1"
	if got := h.set(owned, "owner", "operator"); got.Outcome != contract.OutcomeOK {
		t.Fatalf("set owner: %s %s", got.Outcome, got.Refusal)
	}

	if got := h.library.SetField(&Request{Verb: "set", Actor: "bo", Ref: criterion, Field: "evidence", Value: "record"}); got.Refusal != contract.NotOperator {
		t.Errorf("a non-operator writing a criterion's evidence: wanted %s, got %s %s", contract.NotOperator, got.Outcome, got.Refusal)
	}
	if got := h.library.SetField(&Request{Verb: "set", Actor: "bo", Ref: owned, Field: "evidence", Value: "record"}); got.Refusal != contract.NotOperator {
		t.Errorf("a non-operator writing an operator-owned item's evidence: wanted %s, got %s %s", contract.NotOperator, got.Outcome, got.Refusal)
	}
	if got := h.library.SetField(&Request{Verb: "set", Actor: "bo", Ref: question, Field: "evidence", Value: "record"}); got.Outcome != contract.OutcomeOK {
		t.Errorf("any owner writing another item's evidence: wanted ok, got %s %s", got.Outcome, got.Refusal)
	}
	h.reopen()
	if got, err := h.library.GetField(&Request{Verb: "get", Ref: question, Field: "evidence"}); err != nil || got != "record" {
		t.Errorf("get evidence answered %q %v, wanted record", got, err)
	}
	if got := h.set(criterion, "evidence", ""); got.Outcome != contract.OutcomeOK {
		t.Errorf("the operator clearing a criterion's evidence: wanted ok, got %s %s", got.Outcome, got.Refusal)
	}
	if got, err := h.library.GetField(&Request{Verb: "get", Ref: criterion, Field: "evidence"}); err != nil || got != "" {
		t.Errorf("after the clear get evidence answered %q %v, wanted nothing", got, err)
	}
	for _, verb := range []string{"get", "set"} {
		var refusal string
		if verb == "get" {
			_, err := h.library.GetField(&Request{Verb: verb, Ref: criterion, Field: "standing"})
			refusal = refusalNameOfChange(err)
		} else {
			refusal = h.library.SetField(&Request{Verb: verb, Actor: "alka", Ref: criterion, Field: "standing", Value: "other"}).Refusal
		}
		if refusal != contract.UnknownField {
			t.Errorf("%s standing: wanted %s, got %q", verb, contract.UnknownField, refusal)
		}
	}

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref, Fields: "checklist"})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if view := detail.Checklist[1]; view.Standing != "contract-countersigned" || view.Evidence != "" {
		t.Errorf("the second item's view reads standing %q evidence %q", view.Standing, view.Evidence)
	}
	if view := detail.Checklist[3]; view.Standing != "" || view.Evidence != "record" {
		t.Errorf("the hand-filed item's view reads standing %q evidence %q, wanted no standing and record", view.Standing, view.Evidence)
	}
	columns, err := h.library.columnViews(map[string]int{})
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	var declaring *ColumnView
	for at := range columns {
		if columns[at].ID == aftercare {
			declaring = &columns[at]
		}
	}
	want := []StandingItemView{
		{Key: "deposit-paid", Kind: "acceptance_criterion", Text: "The vendor's deposit has been paid and the receipt is on the card.", Evidence: "receipt"},
		{Key: "contract-countersigned", Kind: "acceptance_criterion", Text: "Both parties have signed the vendor contract.", Owner: "operator"},
		{Key: "date-confirmed", Kind: "open_question", Text: "Has the vendor confirmed the wedding date in writing?"},
	}
	if declaring == nil || len(declaring.StandingItems) != len(want) {
		t.Fatalf("the column view carries %+v", declaring)
	}
	for at, entry := range declaring.StandingItems {
		if entry != want[at] {
			t.Errorf("entry %d reads %+v, wanted %+v", at, entry, want[at])
		}
	}
	for at := range columns {
		if columns[at].ID != aftercare && len(columns[at].StandingItems) != 0 {
			t.Errorf("a column declaring nothing carries standing items in its view: %+v", columns[at])
		}
	}
}

// TestTheRepairMintsWhatTheArrivalsMissed drives criterion 8: the preview
// names each missing instance and writes nothing, the confirmed run mints them
// through the arrival's own path with the operator as actor, a following check
// reports nothing, and anybody but the operator is refused.
func TestTheRepairMintsWhatTheArrivalsMissed(t *testing.T) {
	h := newHarness(t)
	before := h.readyAt("filed before the declaration", aftercare)
	elsewhere := h.readyAt("standing elsewhere", doing)
	h.declareStanding(aftercare, standingBlock)
	// A declaration written after the card arrived leaves the gate with
	// nothing to hold on, which check reports at cleanup severity.
	missing := findingsNamed(h.check(), bench.FindingStandingItemMissing)
	if len(missing) != len(standingKeys) {
		t.Fatalf("check reported %d missing instances, wanted %d: %+v", len(missing), len(standingKeys), missing)
	}

	if _, err := h.library.Check(&Request{Verb: "check", Actor: "bo", FileStanding: true, Confirm: true}); refusalNameOfChange(err) != contract.NotOperator {
		t.Errorf("a non-operator running the repair: wanted %s, got %v", contract.NotOperator, err)
	}

	digest := h.digest()
	preview, err := h.library.Check(&Request{Verb: "check", Actor: "alka", FileStanding: true})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if h.digest() != digest {
		t.Fatal("the preview wrote to the workbench")
	}
	if preview.FiledStanding == nil || !preview.FiledStanding.Preview || len(preview.FiledStanding.Filed) != len(standingKeys) {
		t.Fatalf("the preview reads %+v", preview.FiledStanding)
	}
	for at, filing := range preview.FiledStanding.Filed {
		if filing.Card != before || filing.Column != aftercareSlug || filing.Key != standingKeys[at] {
			t.Errorf("preview line %d reads %+v", at, filing)
		}
	}

	applied, err := h.library.Check(&Request{Verb: "check", Actor: "alka", FileStanding: true, Confirm: true})
	h.reopen()
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.FiledStanding == nil || applied.FiledStanding.Preview || len(applied.FiledStanding.Filed) != len(standingKeys) {
		t.Fatalf("the confirmed run reads %+v", applied.FiledStanding)
	}
	h.assertMinted(before, aftercare)
	if len(h.items(elsewhere)) != 0 {
		t.Errorf("the repair filed on a card standing in another column: %+v", h.items(elsewhere))
	}
	for _, line := range h.filedLines(before) {
		if line.Actor.Name != "alka" || line.Column != aftercare || line.ColumnTitle != "Aftercare" || line.Standing == "" {
			t.Errorf("a repair filing reads %+v, wanted the operator's line carrying the three members", line)
		}
	}
	if found := findingsNamed(h.check(), bench.FindingStandingItemMissing); len(found) != 0 {
		t.Errorf("after the repair check still reports %+v", found)
	}
	again, err := h.library.Check(&Request{Verb: "check", Actor: "alka", FileStanding: true, Confirm: true})
	if err != nil || len(again.FiledStanding.Filed) != 0 {
		t.Errorf("a second confirmed run filed %+v %v, wanted nothing", again.FiledStanding, err)
	}
}

// findingsNamed answers the findings carrying one key.
func findingsNamed(findings []bench.Finding, key string) []bench.Finding {
	var kept []bench.Finding
	for _, finding := range findings {
		if finding.Key == key {
			kept = append(kept, finding)
		}
	}
	return kept
}

// TestAReshapeWithdrawsTheStandingItemsOfTheColumnItRetires drives criterion
// 12 in full: the preview counts and writes nothing; the apply withdraws each
// pending instance on every live card in any column, with the designated
// comment naming the retired column's title, or its identifier for an adopted
// entry, and the two journal lines; a settled instance and a hand-filed item
// are untouched; a claim the pending instance refused succeeds afterwards; and
// check reports the hand-filed item and no withdrawn instance.
func TestAReshapeWithdrawsTheStandingItemsOfTheColumnItRetires(t *testing.T) {
	h := newHarness(t)
	block := strings.Replace(standingBlock, "  contract-countersigned:\n    kind: acceptance_criterion\n",
		"  contract-countersigned:\n    kind: decision\n", 1)
	h.declareStanding(aftercare, block)
	standing := h.readyAt("standing in the column", aftercare)
	departed := h.readyAt("passed through", aftercare)
	h.settle("resolve", departed+"/questions/1", "Confirmed.")
	h.at(departed, review)
	// The card already carries one minted criterion, so the hand-filed one
	// is the second of its kind.
	h.file(departed, "acceptance_criterion", "A criterion somebody filed by hand.")
	handFiled := departed + "/criteria/2"
	if got := h.set(handFiled, "column", aftercareSlug); got.Outcome != contract.OutcomeOK {
		t.Fatalf("set column: %s %s", got.Outcome, got.Refusal)
	}
	resolvedInstance := h.instancesOf(departed, aftercare)["date-confirmed"].ID
	handFiledID := filepath.Base(h.itemDir(handFiled))
	// The adopted case: a card stranded at an identifier the workbench does
	// not declare, carrying a pending instance a declaration minted there,
	// which refuses every claim until the reshape withdraws it.
	stranded := h.readyAt("stranded", review)
	h.item(stranded, "b00000000001", "kind: open_question\nstate: pending\ncolumn: "+strandedID+"\nstanding: date-confirmed\n", "Has the vendor confirmed?")
	h.strand(stranded, strandedID)
	if refused := h.do(&Request{Verb: Claim, Card: stranded, Actor: "alka"}); refused.Refusal != contract.UnresolvedItem {
		t.Fatalf("before the withdrawal the claim was %s %s, wanted %s", refused.Outcome, refused.Refusal, contract.UnresolvedItem)
	}

	digest := h.digest()
	preview, err := h.reshape(h.source(dropsAftercare), false, aftercare+"="+review, strandedID+"="+review)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if h.digest() != digest {
		t.Fatal("the preview wrote to the workbench")
	}
	counts := map[string]int{}
	for _, retirement := range preview.Retirements {
		counts[retirement.ID] = retirement.StandingWithdrawn
	}
	if counts[aftercare] != 5 || counts[strandedID] != 1 {
		t.Fatalf("the preview counts %v, wanted 5 for aftercare and 1 for the adopted entry", counts)
	}

	applied, err := h.reshape(h.source(dropsAftercare), true, aftercare+"="+review, strandedID+"="+review)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	counts = map[string]int{}
	for _, retirement := range applied.Retirements {
		counts[retirement.ID] = retirement.StandingWithdrawn
	}
	if counts[aftercare] != 5 || counts[strandedID] != 1 {
		t.Errorf("the apply counts %v, wanted 5 for aftercare and 1 for the adopted entry", counts)
	}
	wrote := map[string]int{}
	for _, step := range applied.Wrote {
		wrote[step.Step] = step.Count
	}
	if wrote[ReshapeWroteWithdrawn] != 6 {
		t.Errorf("the write-phase steps read %v, wanted 6 withdrawn", wrote)
	}

	for _, ref := range []string{standing, departed} {
		for _, item := range h.items(ref) {
			if item.Standing == "" {
				if item.State != bench.ItemPending {
					t.Errorf("the hand-filed item on %s was touched: %+v", ref, item)
				}
				continue
			}
			if ref == departed && item.Standing == "date-confirmed" {
				if item.State != bench.ItemResolved {
					t.Errorf("the resolved instance on %s was touched: %+v", ref, item)
				}
				continue
			}
			h.assertWithdrawnByReshape(ref, item, "Aftercare")
		}
	}
	adopted := h.items(stranded)[0]
	h.assertWithdrawnByReshape(stranded, adopted, strandedID)
	if admitted := h.do(&Request{Verb: Claim, Card: stranded, Actor: "alka"}); admitted.Outcome != contract.OutcomeOK {
		t.Errorf("after the withdrawal the claim was %s %s", admitted.Outcome, admitted.Refusal)
	}
	// The sweep goes on reporting the two items still naming the retired
	// column in a state other than withdrawn: the hand-filed criterion and
	// the instance the card had already resolved. Every withdrawn instance is
	// passed over.
	reported := map[string]bool{}
	for _, finding := range findingsNamed(h.check(), bench.FindingItemColumnUnresolved) {
		reported[filepath.Base(filepath.Dir(finding.Path))] = true
	}
	if len(reported) != 2 || !reported[handFiledID] || !reported[resolvedInstance] {
		t.Errorf("check reports %v under item-column-unresolved, wanted the hand-filed item %s and the resolved instance %s alone", reported, handFiledID, resolvedInstance)
	}
}

// itemDir resolves an item reference to its directory.
func (h *harness) itemDir(ref string) string {
	h.t.Helper()
	entity, err := h.library.Bench.ResolveEntity(ref)
	if err != nil {
		h.t.Fatalf("resolve %s: %v", ref, err)
	}
	return entity.Dir
}

// assertWithdrawnByReshape holds one instance to the shape the reshape's
// withdrawal leaves: state withdrawn, one comment naming the column,
// designated as the resolution, and on the card's journal a commented line
// followed by an item_withdrawn line carrying the reshape marker and the
// operator as actor.
func (h *harness) assertWithdrawnByReshape(ref string, item *bench.Item, named string) {
	h.t.Helper()
	if item.State != bench.ItemWithdrawn {
		h.t.Errorf("%s of %s stands %s, wanted withdrawn", item.Standing, ref, item.State)
		return
	}
	comments, err := bench.Comments(item.Dir)
	if err != nil {
		h.t.Fatalf("comments of %s: %v", item.ID, err)
	}
	if len(comments) != 1 || item.Resolution != comments[0].ID || !strings.Contains(comments[0].Body, named) {
		h.t.Errorf("%s of %s carries %d comments, resolution %q; wanted one designated comment naming %s", item.Standing, ref, len(comments), item.Resolution, named)
	}
	events := h.events(ref)
	withdrawn, commented := 0, 0
	for at, ev := range events {
		if ev.Item != item.ID {
			continue
		}
		switch ev.Event {
		case contract.EventCommented:
			commented++
		case contract.EventItemWithdrawn:
			withdrawn++
			if !ev.Reshape || ev.Actor.Name != "alka" || ev.From != bench.ItemPending || ev.To != bench.ItemWithdrawn {
				h.t.Errorf("the withdrawal line of %s reads %+v", item.Standing, ev)
			}
			if at == 0 || events[at-1].Event != contract.EventCommented || events[at-1].Item != item.ID {
				h.t.Errorf("the withdrawal line of %s is not preceded by its commented line", item.Standing)
			}
		}
	}
	if withdrawn != 1 || commented != 1 {
		h.t.Errorf("%s of %s has %d withdrawal lines and %d commented lines, wanted one of each", item.Standing, ref, withdrawn, commented)
	}
}

// TestAReRunAfterAnInterruptedWithdrawalWritesNoSecondComment drives the last
// clause of criterion 12. The interruption lands after an instance's anchor
// was written and before its journal lines, which is the point the contract
// admits no second comment for: the re-run passes over the withdrawn anchor,
// writes no second comment, and the card's journal ends with exactly one
// item_withdrawn line for the instance.
func TestAReRunAfterAnInterruptedWithdrawalWritesNoSecondComment(t *testing.T) {
	h := newHarness(t)
	h.declareStanding(aftercare, "standing_items:\n  date-confirmed:\n    kind: open_question\n    text: Has the vendor confirmed the wedding date in writing?\n")
	ref := h.readyAt("interrupted", aftercare)
	journal := h.card(ref).JournalPath()
	h.library.Interpose = func(step string) {
		if step != reshapeStepWithdrawn {
			return
		}
		h.library.Interpose = nil
		// The journal is made unwritable, so the first line's append fails
		// exactly where a process dying there would have stopped.
		if err := os.Chmod(journal, 0o444); err != nil {
			t.Fatalf("chmod: %v", err)
		}
	}
	if _, err := h.reshape(h.source(dropsAftercare), true, aftercare+"="+review); err == nil {
		t.Fatal("the interrupted run reported no error")
	}
	if err := os.Chmod(journal, 0o644); err != nil {
		t.Fatalf("chmod back: %v", err)
	}
	instance := h.items(ref)[0]
	if instance.State != bench.ItemWithdrawn {
		t.Fatalf("the interrupted run left the instance %s, wanted withdrawn", instance.State)
	}
	if lines := linesNaming(h.events(ref), instance.ID); lines != 0 {
		t.Fatalf("the interrupted run wrote %d withdrawal lines for the instance, wanted none", lines)
	}

	report, err := h.reshape(h.source(dropsAftercare), true, aftercare+"="+review)
	if err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if !report.Applied {
		t.Errorf("the re-run did not apply")
	}
	instance = h.items(ref)[0]
	comments, err := bench.Comments(instance.Dir)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	if len(comments) != 1 {
		t.Errorf("the instance carries %d comments after the re-run, wanted one", len(comments))
	}
	withdrawn := 0
	for _, ev := range h.events(ref) {
		if ev.Item == instance.ID && ev.Event == contract.EventItemWithdrawn {
			withdrawn++
		}
	}
	if withdrawn != 1 {
		t.Errorf("the journal carries %d item_withdrawn lines for the instance after the re-run, wanted exactly one", withdrawn)
	}
	h.assertWithdrawnByReshape(ref, instance, "Aftercare")
}

// linesNaming counts the journal lines a withdrawal writes for one item, which
// are its commented line and its item_withdrawn line; the filing line the
// minting wrote is not one of them.
func linesNaming(events []bench.Event, item string) int {
	count := 0
	for _, ev := range events {
		if ev.Item == item && (ev.Event == contract.EventCommented || ev.Event == contract.EventItemWithdrawn) {
			count++
		}
	}
	return count
}
