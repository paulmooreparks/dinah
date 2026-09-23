package main

import (
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestNoSequenceOfCommandsTakesANonOperatorPastAFailedCriterion is
// dinah-472/criteria/20, /25, /47 and /52, and it is the case the whole family
// of guards exists for.
//
// Three review rounds each found a different door, and each was found because
// the sweep before it asked too narrow a question. The question this case asks
// is the widest one the card produced: what can stop this item holding the
// card. Every act that reaches that outcome is tried, with and without a
// standing grant, and the move is tried again after each one, so an act that
// half worked is caught by the move rather than by the act's own answer.
func TestNoSequenceOfCommandsTakesANonOperatorPastAFailedCriterion(t *testing.T) {
	for _, granted := range []bool{false, true} {
		name := "with no grant"
		if granted {
			name = "under a standing grant"
		}
		t.Run(name, func(t *testing.T) {
			root := statesFixture(t, "a card the finding holds")
			mustRun(t, root, "file", "--column", "done", "fx-1", "acceptance_criterion", "something to check")
			mustRun(t, root, "fail", "fx-1/criteria/1", "--text", "the check did not hold")
			if granted {
				mustRun(t, root, "grant", "fx-1", "criterion-retirement")
			}

			held := mustRefuse(t, root, "move", "fx-1", "done", "--actor", "sam")
			assertRefusal(t, held, contract.UnresolvedItem, "the move this case is about")

			for _, attempt := range [][]string{
				{"reopen", "fx-1/criteria/1", "the finding was wrong"},
				{"withdraw", "fx-1/criteria/1", "--text", "away"},
				{"waive", "fx-1/criteria/1", "--text", "let it through"},
				{"set", "fx-1/criteria/1", bench.ItemColumnField, "review"},
				{"set", "fx-1/criteria/1", bench.ItemColumnField},
				{"set", "fx-1", "route", "short"},
				{"archive", "fx-1/criteria/1"},
				{"delete", "fx-1/criteria/1", "--yes"},
				{"move", "fx-1", "done", "--override"},
			} {
				got := mustRefuse(t, root, append(append([]string{}, attempt...), "--actor", "sam")...)
				if name := refusalNameOf(got.errw); name == "" {
					t.Errorf("%v was refused with no name at all: %s", attempt, got.errw)
				}
				again := mustRefuse(t, root, "move", "fx-1", "done", "--actor", "sam")
				assertRefusal(t, again, contract.UnresolvedItem, "the move after "+strings.Join(attempt, " "))
			}

			// And the criterion is still standing at failed, still naming the
			// column, and still on the card, which is what makes the refusals
			// above refusals rather than silent successes.
			if state := soleItemState(t, root, "fx-1"); state != bench.ItemFailed {
				t.Errorf("the criterion stands at %q after every attempt, wanted %q", state, bench.ItemFailed)
			}
			if column := itemKeyOf(t, root, "fx-1/criteria/1", bench.ItemColumnField); column == "" {
				t.Error("the criterion names no column after every attempt, so one of the clears landed")
			}
		})
	}
}

// TestTheColumnKeyOfAProtectedItemIsTheOperatorsToWriteAndToClear is
// dinah-472/criteria/25, /26 and /27, with the accepting half beside each
// refusal.
func TestTheColumnKeyOfAProtectedItemIsTheOperatorsToWriteAndToClear(t *testing.T) {
	root := statesFixture(t, "a card carrying three items")
	mustRun(t, root, "file", "--column", "done", "fx-1", "acceptance_criterion", "a criterion")
	mustRun(t, root, "file", "--column", "done", "fx-1", "open_question", "his question", "--owner", "operator")
	mustRun(t, root, "file", "--column", "done", "fx-1", "open_question", "the holder's question", "--owner", "holder")

	// A value the card's route carries is refused all the same, so the guard
	// is not one the route check happens to satisfy.
	valued := mustRefuse(t, root, "set", "fx-1/criteria/1", bench.ItemColumnField, "review", "--actor", "sam")
	assertRefusal(t, valued, contract.NotOperator, "a write of a criterion's column by somebody else")
	cleared := mustRefuse(t, root, "set", "fx-1/criteria/1", bench.ItemColumnField, "--actor", "sam")
	assertRefusal(t, cleared, contract.NotOperator, "a clear of a criterion's column by somebody else")
	if column := itemKeyOf(t, root, "fx-1/criteria/1", bench.ItemColumnField); column == "" {
		t.Error("the criterion names no column after the refused clear")
	}
	mustRun(t, root, "set", "fx-1/criteria/1", bench.ItemColumnField, "review")
	mustRun(t, root, "set", "fx-1/criteria/1", bench.ItemColumnField)

	// The owner clause, whatever the kind, and the item that carries neither
	// shape, which anybody may still repair.
	owned := mustRefuse(t, root, "set", "fx-1/questions/1", bench.ItemColumnField, "--actor", "sam")
	assertRefusal(t, owned, contract.NotOperator, "a clear of an operator-owned question's column by somebody else")
	mustRun(t, root, "set", "fx-1/questions/2", bench.ItemColumnField, "review", "--actor", "sam")
	mustRun(t, root, "set", "fx-1/questions/2", bench.ItemColumnField, "--actor", "sam")
}

// TestARouteWriteIsRefusedWhereItWouldStrandAHoldingItem is
// dinah-472/criteria/28 and /29. The route readers ask whether an item's state
// releases the hold rather than whether it is pending, which is what lets a
// failed criterion be stranded and stop firing.
func TestARouteWriteIsRefusedWhereItWouldStrandAHoldingItem(t *testing.T) {
	for _, state := range []string{bench.ItemFailed, bench.ItemWaived, bench.ItemWithdrawn} {
		t.Run(state, func(t *testing.T) {
			root := newBenchFromDefinition(t, routedStatesDefinition)
			mustRun(t, root, "add", "a card whose route may drop a station")
			mustRun(t, root, "file", "--column", "review", "fx-1", "acceptance_criterion", "something to check")
			mustRun(t, root, "fail", "fx-1/criteria/1", "--text", "the check did not hold")
			if state != bench.ItemFailed {
				settleTo(t, root, "fx-1/criteria/1", state)
			}

			written := runCLI(t, root, "set", "fx-1", "route", "short")
			checked := runCLI(t, root, "check")
			if state == bench.ItemFailed {
				if written.code == 0 {
					t.Fatalf("the route write stranded a failed criterion:\n%s", written.out)
				}
				assertRefusal(t, written, contract.RouteStrandsItem, "a route write stranding a failed criterion")
				if !strings.Contains(written.errw, "review") {
					t.Errorf("the refusal names no column: %s", written.errw)
				}
				return
			}
			if written.code != 0 {
				t.Fatalf("the route write was refused over an item standing at %s, which strands nothing: %s", state, written.errw)
			}
			if strings.Contains(checked.out, "names a column the card's route does not carry") {
				t.Errorf("check reports %s over an item standing at %s:\n%s", bench.FindingItemOffRoute, state, checked.out)
			}
		})
	}

	// And the finding's own half, over the state that holds.
	root := newBenchFromDefinition(t, routedStatesDefinition)
	mustRun(t, root, "add", "a card standing off its road")
	mustRun(t, root, "file", "--column", "review", "fx-1", "acceptance_criterion", "something to check")
	mustRun(t, root, "fail", "fx-1/criteria/1", "--text", "the check did not hold")
	setCardRouteByHand(t, root, "fx-1", "short")
	checked := runCLI(t, root, "check")
	// The finding is read as the sentence a person sees rather than as its
	// key, because the key is how the code names it and the report is what
	// the reader meets.
	if !strings.Contains(checked.out, "names a column the card's route does not carry") {
		t.Errorf("check does not report %s over a failed criterion stranded off the road:\n%s", bench.FindingItemOffRoute, checked.out)
	}
}

// routedStatesDefinition declares a short route that drops the review station,
// so a route write can strand an item naming it.
const routedStatesDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Routes and states",
  "routes": { "short": ["c00000000001", "c00000000002", "c00000000004"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Doing", "kind": "work" },
    { "id": "c00000000003", "title": "Review", "kind": "work", "gate_items": true },
    { "id": "c00000000004", "title": "Done", "kind": "done" }
  ]
}`

// setCardRouteByHand writes a card's route straight onto its anchor, which is
// how a case reaches a card standing off its own road: the write path refuses
// exactly that, and the state it produces is one a reshape leaves behind.
func setCardRouteByHand(t *testing.T, root, ref, route string) {
	t.Helper()
	opened, err := bench.OpenAwaitingResolution(soleBenchDir(t, root))
	if err != nil {
		t.Fatalf("open the workbench: %v", err)
	}
	entity, err := opened.ResolveEntity(ref)
	if err != nil {
		t.Fatalf("resolve %s: %v", ref, err)
	}
	card, err := bench.LoadCard(opened.CardsRoot(), entity.Card.ID)
	if err != nil {
		t.Fatalf("load %s: %v", ref, err)
	}
	card.Route = route
	if err := card.Save(); err != nil {
		t.Fatalf("save %s: %v", ref, err)
	}
}

// TestAnAnswerOfRecordCannotBeErased is dinah-472/criteria/30 and /61. A
// settled item asserts that somebody decided something, and an erasable record
// of who and why is not a record.
func TestAnAnswerOfRecordCannotBeErased(t *testing.T) {
	for _, state := range []string{bench.ItemResolved, bench.ItemVerified, bench.ItemFailed, bench.ItemWaived, bench.ItemWithdrawn} {
		t.Run(state, func(t *testing.T) {
			root := statesFixture(t, "a card carrying an item standing at "+state)
			standAt(t, root, state)
			ref := itemRefFor(state)

			cleared := mustRefuse(t, root, "set", ref, bench.ItemResolutionField)
			assertRefusal(t, cleared, contract.DesignationRequired, "a clear of the answer of an item standing at "+state)
			if !strings.Contains(cleared.errw, state) {
				t.Errorf("the refusal does not carry the state as its detail: %s", cleared.errw)
			}
			if got := itemKeyOf(t, root, ref, bench.ItemResolutionField); got == "" {
				t.Error("the answer went all the same")
			}

			// A rewrite to another comment of the same item stays open,
			// because that is a correction rather than an erasure.
			mustRun(t, root, "comment", ref, "a second comment of the same item")
			mustRun(t, root, "set", ref, bench.ItemResolutionField, ref+"/comments/2")

			// And the one legitimate erasure, which clears the state with it.
			mustRun(t, root, "reopen", ref, "the answer stopped standing")
			if got := itemKeyOf(t, root, ref, bench.ItemResolutionField); got != "" {
				t.Errorf("the reopen left the answer %q standing", got)
			}
			mustRun(t, root, "set", ref+"/comments/2", "body", "a second comment, rewritten")
		})
	}
}

// TestADesignatedCommentsBodyCannotBeRewrittenWhileItIsTheAnswer is
// dinah-472/criteria/60. Identity keying fixes which comment the answer names
// and does nothing about what that comment says.
func TestADesignatedCommentsBodyCannotBeRewrittenWhileItIsTheAnswer(t *testing.T) {
	root := statesFixture(t, "a card carrying an answered question")
	mustRun(t, root, "file", "fx-1", "open_question", "something to answer")
	mustRun(t, root, "comment", "fx-1/questions/1", "the answer, as the operator wrote it")
	mustRun(t, root, "comment", "fx-1/questions/1", "a note nobody designated")
	mustRun(t, root, "resolve", "fx-1/questions/1", "fx-1/questions/1/comments/1")

	// By somebody else, and by the author who wrote it, because a record
	// anybody can rewrite is no record and its own author is anybody.
	for _, actor := range []string{"sam", "alka"} {
		refused := mustRefuse(t, root, "set", "fx-1/questions/1/comments/1", "body", "something else entirely", "--actor", actor)
		assertRefusal(t, refused, contract.NotDesignatable, "a rewrite of the designated comment by "+actor)
		if !strings.Contains(refused.errw, "fx-1/questions/1") {
			t.Errorf("the refusal does not name the designating item: %s", refused.errw)
		}
	}
	shown := mustRun(t, root, "show", "fx-1/questions/1/comments/1")
	if !strings.Contains(shown.out, "the answer, as the operator wrote it") {
		t.Errorf("the designated comment was rewritten after all:\n%s", shown.out)
	}

	// A comment of the same item that nothing designates is writable, which
	// is what makes the refusal above about the designation.
	mustRun(t, root, "set", "fx-1/questions/1/comments/2", "body", "a note, rewritten")

	// And the route the refusal's next step names reaches the correction.
	mustRun(t, root, "reopen", "fx-1/questions/1", "the answer has to be corrected")
	mustRun(t, root, "set", "fx-1/questions/1/comments/1", "body", "the answer, corrected")
}

// TestRemovingAProtectedItemIsTheOperatorsAlone is dinah-472/criteria/48,
// /49, /50 and /51.
func TestRemovingAProtectedItemIsTheOperatorsAlone(t *testing.T) {
	// The three protected shapes, each refused both acts and each then
	// removed by the operator, so the reservation is shown to be about the
	// actor rather than about the act.
	for _, plant := range []struct {
		name string
		ref  string
		file []string
		then []string
	}{
		{"an acceptance criterion", "fx-1/criteria/1",
			[]string{"file", "fx-1", "acceptance_criterion", "something to check"}, nil},
		{"an operator-owned question", "fx-1/questions/1",
			[]string{"file", "fx-1", "open_question", "his to answer", "--owner", "operator"}, nil},
		{"a waived question", "fx-1/questions/1",
			[]string{"file", "fx-1", "open_question", "something nobody answered"},
			[]string{"waive", "fx-1/questions/1", "--text", "the operator decided"}},
	} {
		t.Run(plant.name, func(t *testing.T) {
			for _, act := range []string{"archive", "delete"} {
				root := statesFixture(t, "a card carrying "+plant.name)
				mustRun(t, root, plant.file...)
				if plant.then != nil {
					mustRun(t, root, plant.then...)
				}
				argv := []string{act, plant.ref}
				if act == "delete" {
					argv = append(argv, "--yes")
				}
				refused := mustRefuse(t, root, append(append([]string{}, argv...), "--actor", "sam")...)
				assertRefusal(t, refused, contract.NotOperator, act+" of "+plant.name+" by somebody else")
				mustRun(t, root, argv...)
			}
		})
	}

	// Every other item stays removable by anybody, which keeps the ordinary
	// tidying of a question or a decision where it was.
	for _, plant := range [][]string{
		{"file", "fx-1", "open_question", "nobody's in particular"},
		{"file", "fx-1", "decision", "a decision to be resolved"},
	} {
		for _, act := range []string{"archive", "delete"} {
			root := statesFixture(t, "a card carrying an unprotected item")
			mustRun(t, root, plant...)
			ref := "fx-1/questions/1"
			if plant[2] == "decision" {
				ref = "fx-1/decisions/1"
				mustRun(t, root, "resolve", ref, "--text", "somebody took it")
			}
			argv := []string{act, ref, "--actor", "sam"}
			if act == "delete" {
				argv = append(argv, "--yes")
			}
			mustRun(t, root, argv...)
		}
	}
}

// TestRestoreIsNotReserved is dinah-472/criteria/51. Restoring an item puts it
// back into the live set and can only re-impose a hold, which is the reasoning
// reopen was built on and which is sound here for the reason it stopped being
// sound there: nothing composes with a restore to reach a removal.
func TestRestoreIsNotReserved(t *testing.T) {
	root := statesFixture(t, "a card whose criterion comes back")
	mustRun(t, root, "file", "--column", "done", "fx-1", "acceptance_criterion", "something to check")
	mustRun(t, root, "file", "fx-1", "open_question", "his to answer", "--owner", "operator")
	mustRun(t, root, "archive", "fx-1/criteria/1")
	mustRun(t, root, "archive", "fx-1/questions/1")

	mustRun(t, root, "restore", "fx-1/criteria/1", "--actor", "sam")
	mustRun(t, root, "restore", "fx-1/questions/1", "--actor", "sam")

	// The restored criterion names the gated column again, so the hold it was
	// imposing is back.
	held := mustRefuse(t, root, "move", "fx-1", "done")
	assertRefusal(t, held, contract.UnresolvedItem, "a move into the gated column after the restore")
}

// TestTheOperatorsQueueSurvivesAnAttemptToEmptyIt is dinah-472/criteria/50.
// The stamp section 5.3 rests on has to survive the item being removed as well
// as rewritten, which is the hole the fourth review reproduced twice over.
func TestTheOperatorsQueueSurvivesAnAttemptToEmptyIt(t *testing.T) {
	root := statesFixture(t, "a card carrying his question")
	mustRun(t, root, "file", "fx-1", "open_question", "his to answer", "--owner", "operator")
	before := mustRun(t, root, "prime")
	if !strings.Contains(before.out, "his to answer") {
		t.Fatalf("prime does not count the question before the attempts:\n%s", before.out)
	}
	for _, argv := range [][]string{
		{"archive", "fx-1/questions/1", "--actor", "sam"},
		{"delete", "fx-1/questions/1", "--yes", "--actor", "sam"},
	} {
		refused := mustRefuse(t, root, argv...)
		assertRefusal(t, refused, contract.NotOperator, strings.Join(argv[:1], " ")+" of his question")
	}
	after := mustRun(t, root, "prime")
	if !strings.Contains(after.out, "his to answer") {
		t.Errorf("prime no longer counts the question:\n%s", after.out)
	}
}

// TestDeletingAnItemNamesWhatItRequired is dinah-472/criteria/53. A deletion
// is the one removal that destroys what it removed, so the line has to say
// what went away rather than only that something with an identifier did.
func TestDeletingAnItemNamesWhatItRequired(t *testing.T) {
	root := statesFixture(t, "a card whose criterion is deleted")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "the endpoint answers 404 for an unknown id")
	mustRun(t, root, "delete", "fx-1/criteria/1", "--yes")

	found := false
	for _, event := range cardJournal(t, root, cardID(t, root, "fx-1")) {
		if event.Event != contract.EventDeleted {
			continue
		}
		found = true
		if event.Title != "the endpoint answers 404 for an unknown id" {
			t.Errorf("the deleted line carries the title %q, wanted the item's own text", event.Title)
		}
		if event.Kind != "acceptance_criterion" {
			t.Errorf("the deleted line carries the kind %q, wanted the item's own kind", event.Kind)
		}
	}
	if !found {
		t.Error("the deletion wrote no deleted line at all")
	}
}

// TestReopenIsNarrowedToThreeCasesAndStaysOpenInEveryOther is
// dinah-472/criteria/41, /42 and /43.
func TestReopenIsNarrowedToThreeCasesAndStaysOpenInEveryOther(t *testing.T) {
	// The two states, each refused to somebody else and each admitted to the
	// operator.
	for _, state := range []string{bench.ItemFailed, bench.ItemWaived} {
		root := statesFixture(t, "a card carrying a criterion at "+state)
		standAt(t, root, state)
		refused := mustRefuse(t, root, "reopen", "fx-1/criteria/1", "it was wrong", "--actor", "sam")
		assertRefusal(t, refused, contract.NotOperator, "a reopen of an item standing at "+state)
		if got := soleItemState(t, root, "fx-1"); got != state {
			t.Errorf("the item moved to %q under the refusal", got)
		}
		mustRun(t, root, "reopen", "fx-1/criteria/1", "it was wrong")
		if got := soleItemState(t, root, "fx-1"); got != bench.ItemPending {
			t.Errorf("the operator's reopen left the item at %q", got)
		}
	}

	// The owner case, whatever the state, with the unstamped item beside it.
	owned := statesFixture(t, "a card carrying two answered questions")
	mustRun(t, owned, "file", "fx-1", "open_question", "his to answer", "--owner", "operator")
	mustRun(t, owned, "file", "fx-1", "open_question", "the holder's to answer", "--owner", "holder")
	mustRun(t, owned, "resolve", "fx-1/questions/1", "--text", "the operator ruled")
	mustRun(t, owned, "resolve", "fx-1/questions/2", "--text", "the holder answered")
	refused := mustRefuse(t, owned, "reopen", "fx-1/questions/1", "it was wrong", "--actor", "sam")
	assertRefusal(t, refused, contract.NotOperator, "a reopen of an operator-owned question")
	mustRun(t, owned, "reopen", "fx-1/questions/2", "it was wrong", "--actor", "sam")

	// And the three states the narrowing does not reach, which is the
	// ordinary case and the common one.
	for _, state := range []string{bench.ItemResolved, bench.ItemVerified, bench.ItemWithdrawn} {
		root := statesFixture(t, "a card carrying an item at "+state)
		standAt(t, root, state)
		mustRun(t, root, "reopen", itemRefFor(state), "a reviewer found it was closed wrongly", "--actor", "sam")
	}
}

// TestAGrantIsGivenAndTakenBackByTheOperatorAlone is dinah-472/criteria/31,
// /38, /39 and /40.
func TestAGrantIsGivenAndTakenBackByTheOperatorAlone(t *testing.T) {
	root := statesFixture(t, "a card the operator narrows")
	mustRun(t, root, "move", "fx-1", "doing")

	refused := mustRefuse(t, root, "grant", "fx-1", "criterion-retirement", "--actor", "sam")
	assertRefusal(t, refused, contract.NotOperator, "a grant by somebody who is not the operator")
	missing := mustRefuse(t, root, "revoke", "fx-1", "criterion-retirement")
	assertRefusal(t, missing, contract.NoGrant, "a revoke over a card carrying no grant")
	if !strings.Contains(missing.errw, "fx-1") {
		t.Errorf("the refusal does not name the card: %s", missing.errw)
	}

	mustRun(t, root, "grant", "fx-1", "criterion-retirement")
	shown := mustRun(t, root, "show", "fx-1", "--fields", "card")
	if !strings.Contains(shown.out, "doing") || !strings.Contains(shown.out, "Doing") {
		t.Errorf("the card does not report the grant with the column it is bound to:\n%s", shown.out)
	}

	// Granting again succeeds and rebinds, which is the intended flow after
	// a move and which refusing would put a turnstile in front of.
	mustRun(t, root, "grant", "fx-1", "criterion-retirement")

	refusedRevoke := mustRefuse(t, root, "revoke", "fx-1", "criterion-retirement", "--actor", "sam")
	assertRefusal(t, refusedRevoke, contract.NotOperator, "a revoke by somebody who is not the operator")
	mustRun(t, root, "revoke", "fx-1", "criterion-retirement")
	gone := mustRun(t, root, "show", "fx-1", "--fields", "card")
	if strings.Contains(gone.out, "retirement") {
		t.Errorf("the card still reports a grant after the revoke:\n%s", gone.out)
	}

	granted, revoked := 0, 0
	for _, event := range cardJournal(t, root, cardID(t, root, "fx-1")) {
		switch event.Event {
		case contract.EventRetirementGranted:
			granted++
			if event.To == "" {
				t.Error("a retirement_granted line names no column at all")
			}
		case contract.EventRetirementRevoked:
			revoked++
		}
	}
	if granted != 2 || revoked != 1 {
		t.Errorf("the journal carries %d granted lines and %d revoked, wanted two and one", granted, revoked)
	}
}

// TestAGrantAdmitsAWithdrawalAndReachesNothingElse is
// dinah-472/criteria/32, /33, /34, /35, /36, /44 and /45.
func TestAGrantAdmitsAWithdrawalAndReachesNothingElse(t *testing.T) {
	// What it admits: a criterion nobody has found anything wrong with, in
	// each of the three states the exclusion leaves open.
	for _, state := range []string{bench.ItemPending, bench.ItemResolved, bench.ItemVerified} {
		root := statesFixture(t, "a card the operator narrowed")
		kind := "acceptance_criterion"
		mustRun(t, root, "file", "fx-1", kind, "something that stopped applying")
		switch state {
		case bench.ItemVerified:
			mustRun(t, root, "verify", "fx-1/criteria/1", "--text", "the check was run")
		case bench.ItemResolved:
			// A criterion cannot be resolved, so the resolved case is
			// reached through the field write the state routes.
			setItemStateByHand(t, root, "fx-1/criteria/1", bench.ItemResolved)
			mustRun(t, root, "comment", "fx-1/criteria/1", "an answer of record")
			setItemResolutionByHand(t, root, "fx-1/criteria/1")
		}
		before := mustRefuse(t, root, "withdraw", "fx-1/criteria/1", "--text", "away", "--actor", "sam")
		assertRefusal(t, before, contract.NotOperator, "a withdrawal of a criterion at "+state+" with no grant")
		mustRun(t, root, "grant", "fx-1", "criterion-retirement")
		mustRun(t, root, "withdraw", "fx-1/criteria/1", "--text", "the card was narrowed", "--actor", "sam")
	}

	// What it excludes: a finding somebody recorded, in the two states a
	// non-operator can neither reach nor leave.
	for _, state := range []string{bench.ItemFailed, bench.ItemWaived} {
		root := statesFixture(t, "a card carrying a finding")
		standAt(t, root, state)
		mustRun(t, root, "grant", "fx-1", "criterion-retirement")
		refused := mustRefuse(t, root, "withdraw", "fx-1/criteria/1", "--text", "away", "--actor", "sam")
		assertRefusal(t, refused, contract.GrantExcludesFinding, "a withdrawal of a criterion at "+state+" under a grant")
		if !strings.Contains(refused.errw, state) {
			t.Errorf("the refusal does not carry the state as its detail: %s", refused.errw)
		}
		// And the reopen that would reach around it is refused too, which is
		// what makes the state key durable rather than composable.
		reopened := mustRefuse(t, root, "reopen", "fx-1/criteria/1", "back to pending", "--actor", "sam")
		assertRefusal(t, reopened, contract.NotOperator, "a reopen of a criterion at "+state+" under a grant")
		if got := soleItemState(t, root, "fx-1"); got != state {
			t.Errorf("the criterion moved to %q under the two refusals", got)
		}
		if got := soleItemResolution(t, root, "fx-1"); got == "" {
			t.Error("the criterion lost its answer of record under the two refusals")
		}
	}

	// A criterion that failed, was reopened by the operator and then passed
	// is withdrawable under a grant, so an earlier failure does not lock an
	// item out for the life of the card.
	healed := statesFixture(t, "a card whose criterion was fixed")
	standAt(t, healed, bench.ItemFailed)
	mustRun(t, healed, "reopen", "fx-1/criteria/1", "the fix landed")
	mustRun(t, healed, "verify", "fx-1/criteria/1", "--text", "the check was run again and it held")
	mustRun(t, healed, "grant", "fx-1", "criterion-retirement")
	mustRun(t, healed, "withdraw", "fx-1/criteria/1", "--text", "the card was narrowed", "--actor", "sam")

	// What it does not reach: the owner guard, waive, the column key, and
	// the two removals.
	narrow := statesFixture(t, "a card carrying items a grant does not reach")
	mustRun(t, narrow, "file", "--column", "done", "fx-1", "acceptance_criterion", "his own criterion", "--owner", "operator")
	mustRun(t, narrow, "file", "--column", "done", "fx-1", "acceptance_criterion", "an ordinary criterion")
	mustRun(t, narrow, "grant", "fx-1", "criterion-retirement")
	for _, argv := range [][]string{
		{"withdraw", "fx-1/criteria/1", "--text", "away"},
		{"waive", "fx-1/criteria/2", "--text", "let it through"},
		{"set", "fx-1/criteria/2", bench.ItemColumnField, "review"},
		{"set", "fx-1/criteria/2", bench.ItemColumnField},
		{"archive", "fx-1/criteria/2"},
		{"delete", "fx-1/criteria/2", "--yes"},
	} {
		got := mustRefuse(t, narrow, append(append([]string{}, argv...), "--actor", "sam")...)
		assertRefusal(t, got, contract.NotOperator, strings.Join(argv, " ")+" under a standing grant")
	}

	// And a grant on one card admits nothing on another.
	elsewhere := statesFixture(t, "the card carrying the grant", "the card that carries none")
	mustRun(t, elsewhere, "file", "fx-2", "acceptance_criterion", "something that stopped applying")
	mustRun(t, elsewhere, "grant", "fx-1", "criterion-retirement")
	other := mustRefuse(t, elsewhere, "withdraw", "fx-2/criteria/1", "--text", "away", "--actor", "sam")
	assertRefusal(t, other, contract.NotOperator, "a withdrawal on a card carrying no grant of its own")
}

// setItemResolutionByHand writes an item's answer straight onto its anchor,
// which is how a case reaches a criterion standing at resolved: no verb lands
// that pair, and what the case is about is the state the grant reads rather
// than the route by which an item got there.
func setItemResolutionByHand(t *testing.T, root, ref string) {
	t.Helper()
	dir := itemDirOf(t, root, ref)
	comments, err := bench.Comments(dir)
	if err != nil || len(comments) == 0 {
		t.Fatalf("read the comments of %s: %v", ref, err)
	}
	fm, body, err := bench.ReadItemAnchor(dir)
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	fm.Set(bench.ItemResolutionField, comments[0].ID)
	if err := bench.WriteItemAnchor(dir, fm, body); err != nil {
		t.Fatalf("write the anchor of %s: %v", ref, err)
	}
}

// TestAGrantIsSpentByTheCardsNextMove is dinah-472/criteria/37. The narrowing
// and the tidying are one episode at one station, so the departure that ends
// the episode ends the permission.
func TestAGrantIsSpentByTheCardsNextMove(t *testing.T) {
	root := statesFixture(t, "a card the operator narrowed")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "one that stopped applying")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "another that stopped applying")
	mustRun(t, root, "grant", "fx-1", "criterion-retirement")
	mustRun(t, root, "withdraw", "fx-1/criteria/1", "--text", "the card was narrowed", "--actor", "sam")

	mustRun(t, root, "move", "fx-1", "doing")
	refused := mustRefuse(t, root, "withdraw", "fx-1/criteria/2", "--text", "this one too", "--actor", "sam")
	assertRefusal(t, refused, contract.NotOperator, "a withdrawal after the move that spent the grant")
	if got := itemKeyOf(t, root, "fx-1/criteria/1", bench.ItemStateField); got != bench.ItemWithdrawn {
		t.Errorf("the first criterion stands at %q, so the move undid the withdrawal as well", got)
	}
	shown := mustRun(t, root, "show", "fx-1", "--fields", "card")
	if strings.Contains(shown.out, "retirement") {
		t.Errorf("the card still carries a grant after the move that spent it:\n%s", shown.out)
	}
}

// TestAWithdrawalUnderAGrantIsMarkedAndTheOperatorsIsNot is
// dinah-472/criteria/39's second half. A reader can tell work done under a
// grant from work the operator did himself.
func TestAWithdrawalUnderAGrantIsMarkedAndTheOperatorsIsNot(t *testing.T) {
	root := statesFixture(t, "a card the operator narrowed")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "one that stopped applying")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "another that stopped applying")
	mustRun(t, root, "grant", "fx-1", "criterion-retirement")
	mustRun(t, root, "withdraw", "fx-1/criteria/1", "--text", "the card was narrowed", "--actor", "sam")
	mustRun(t, root, "withdraw", "fx-1/criteria/2", "--text", "and this one as well")

	marked, plain := 0, 0
	for _, event := range cardJournal(t, root, cardID(t, root, "fx-1")) {
		if event.Event != contract.EventItemWithdrawn {
			continue
		}
		if event.Grant {
			marked++
			continue
		}
		plain++
	}
	if marked != 1 || plain != 1 {
		t.Errorf("the journal carries %d marked withdrawals and %d plain, wanted one of each", marked, plain)
	}
}
