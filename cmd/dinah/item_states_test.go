package main

import (
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// itemStatesDefinition is a four-column flow whose working station holds a
// card on the way out and whose done column holds one on the way in, so one
// fixture reaches both directions of the gate the two new states release.
const itemStatesDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Item states",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work", "gate_items": "out" },
    { "id": "a00000000003", "title": "Review", "kind": "work" },
    { "id": "a00000000004", "title": "Done", "kind": "done", "gate_items": true }
  ]
}`

// statesFixture builds that flow with one card standing in it, and files
// whatever items the case needs afterwards.
func statesFixture(t *testing.T, titles ...string) string {
	t.Helper()
	root := newBenchFromDefinition(t, itemStatesDefinition)
	for _, title := range titles {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	return root
}

// mustRefuse runs one invocation, fails the case where it succeeded, and
// answers the refusal name it carried.
func mustRefuse(t *testing.T, root string, argv ...string) invocation {
	t.Helper()
	got := runCLI(t, root, argv...)
	if got.code == 0 {
		t.Fatalf("%v was admitted and this case is about its refusal:\n%s", argv, got.out)
	}
	return got
}

// assertRefusal fails a case whose refusal carried the wrong name.
//
// An invocation carrying nothing on its error stream is reported rather than
// read for a name, because the name reader takes the first word and a first
// word of nothing is a panic. A run that exited non-zero and said nothing is
// exactly what a broken guard produces, so this is the arming path rather than
// an unlikely one.
func assertRefusal(t *testing.T, got invocation, wanted, what string) {
	t.Helper()
	if strings.TrimSpace(got.errw) == "" {
		t.Errorf("%s exited %d and carried no refusal at all, wanted %s:\n%s", what, got.code, wanted, got.out)
		return
	}
	if name := refusalNameOf(got.errw); name != wanted {
		t.Errorf("%s answered %s, wanted %s: %s", what, name, wanted, got.errw)
	}
}

// TestWaiveIsTheOperatorsAloneAndOnlyFromAHoldingState is
// dinah-472/criteria/1, /2 and /3. Each refusal is pinned beside the call
// that succeeds, because a refusal every call satisfies would pass a build
// that refused all of them.
func TestWaiveIsTheOperatorsAloneAndOnlyFromAHoldingState(t *testing.T) {
	root := statesFixture(t, "a card carrying a criterion")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "something nobody checked")

	// The owner key is read by neither guard: the item carries none at all
	// and the refusal is the actor's.
	refused := mustRefuse(t, root, "waive", "fx-1/criteria/1", "--text", "let it through", "--actor", "sam")
	assertRefusal(t, refused, contract.NotOperator, "a waiver by somebody who is not the operator")
	if !strings.Contains(refused.errw, "sam") {
		t.Errorf("the refusal does not name the actor: %s", refused.errw)
	}
	mustRun(t, root, "waive", "fx-1/criteria/1", "--text", "the operator decided the card may go on")
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemWaived {
		t.Errorf("the item stands at %q after the operator's waiver, wanted %q", state, bench.ItemWaived)
	}

	// A waiver lifts a hold, so an item holding nothing cannot be waived. The
	// four refused states are walked on one item, and each refusal carries
	// the state it met.
	for _, state := range []string{bench.ItemWaived, bench.ItemResolved, bench.ItemVerified, bench.ItemWithdrawn} {
		root := statesFixture(t, "a card carrying an item to stand at "+state)
		standAt(t, root, state)
		got := mustRefuse(t, root, "waive", itemRefFor(state), "--text", "let it through")
		assertRefusal(t, got, contract.NotWaivable, "a waiver of an item standing at "+state)
		if !strings.Contains(got.errw, state) {
			t.Errorf("the refusal over %s does not carry the state as its detail: %s", state, got.errw)
		}
	}

	// And the two states it is legal from, which is the accepting half.
	for _, state := range []string{bench.ItemPending, bench.ItemFailed} {
		root := statesFixture(t, "a card carrying an item to stand at "+state)
		standAt(t, root, state)
		mustRun(t, root, "waive", itemRefFor(state), "--text", "the operator decided")
	}
}

// itemRefFor names the item standAt files for one state, which differs by
// kind because fail and verify close an acceptance criterion and resolve
// closes an open question.
func itemRefFor(state string) string {
	switch state {
	case bench.ItemResolved, bench.ItemWithdrawn:
		return "fx-1/questions/1"
	}
	return "fx-1/criteria/1"
}

// standAt files one item on fx-1 and leaves it standing at a named state,
// through the verbs rather than by writing the anchor, so the item the case
// meets is one the tool could have produced.
func standAt(t *testing.T, root, state string) {
	t.Helper()
	ref := itemRefFor(state)
	kind := "acceptance_criterion"
	if ref == "fx-1/questions/1" {
		kind = "open_question"
	}
	mustRun(t, root, "file", "fx-1", kind, "something to settle")
	switch state {
	case bench.ItemPending:
	case bench.ItemResolved:
		mustRun(t, root, "resolve", ref, "--text", "somebody answered it")
	case bench.ItemVerified:
		mustRun(t, root, "verify", ref, "--text", "the check was run")
	case bench.ItemFailed:
		mustRun(t, root, "fail", ref, "--text", "the check did not hold")
	case bench.ItemWaived:
		mustRun(t, root, "waive", ref, "--text", "the operator decided")
	case bench.ItemWithdrawn:
		mustRun(t, root, "withdraw", ref, "--text", "the question stopped applying")
	default:
		t.Fatalf("standAt has no route to %q", state)
	}
}

// TestWithdrawIsWideAndItsGuardsAreTheKindAndTheOwner is
// dinah-472/criteria/4, /5, /6, /19 and /23.
func TestWithdrawIsWideAndItsGuardsAreTheKindAndTheOwner(t *testing.T) {
	// A criterion standing at verified is withdrawn by the operator, and the
	// comment it designated before stays where it is.
	root := statesFixture(t, "a card carrying a verified criterion")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "something checked")
	mustRun(t, root, "verify", "fx-1/criteria/1", "--text", "the check was run and it held")
	before := soleItemResolution(t, root, "fx-1")
	mustRun(t, root, "withdraw", "fx-1/criteria/1", "--text", "the card was narrowed and this went with it")
	if state := soleItemState(t, root, "fx-1"); state != bench.ItemWithdrawn {
		t.Errorf("the criterion stands at %q, wanted %q", state, bench.ItemWithdrawn)
	}
	after := soleItemResolution(t, root, "fx-1")
	if after == before {
		t.Errorf("the withdrawal left the earlier answer standing as the item's own, and it records its own reason")
	}
	shown := mustRun(t, root, "show", "fx-1/criteria/1/comments/1")
	if !strings.Contains(shown.out, "the check was run and it held") {
		t.Errorf("the comment the earlier settling designated is gone:\n%s", shown.out)
	}

	// Already withdrawn is the one source state the verb refuses.
	got := mustRefuse(t, root, "withdraw", "fx-1/criteria/1", "--text", "again")
	assertRefusal(t, got, contract.AlreadyWithdrawn, "a second withdrawal")
	if !strings.Contains(got.errw, bench.ItemWithdrawn) {
		t.Errorf("the refusal does not carry the state as its detail: %s", got.errw)
	}

	// A waived question is withdrawable, which is the widest edge the verb
	// carries.
	waived := statesFixture(t, "a card carrying a waived question")
	mustRun(t, waived, "file", "fx-1", "open_question", "something nobody answered")
	mustRun(t, waived, "waive", "fx-1/questions/1", "--text", "the operator decided")
	mustRun(t, waived, "withdraw", "fx-1/questions/1", "--text", "and then it stopped applying")

	// The owner guard, with the accepting case beside it.
	owned := statesFixture(t, "a card carrying two questions")
	mustRun(t, owned, "file", "fx-1", "open_question", "his to answer", "--owner", "operator")
	mustRun(t, owned, "file", "fx-1", "open_question", "the holder's to answer", "--owner", "holder")
	refused := mustRefuse(t, owned, "withdraw", "fx-1/questions/1", "--text", "away", "--actor", "sam")
	assertRefusal(t, refused, contract.NotOperator, "a withdrawal of an operator-owned question")
	mustRun(t, owned, "withdraw", "fx-1/questions/2", "--text", "the card was narrowed", "--actor", "sam")

	// An unstamped question is withdrawable by anybody, which is the exposure
	// this card neither widens nor narrows and pins so that it is deliberate.
	unstamped := statesFixture(t, "a card carrying an unstamped question")
	mustRun(t, unstamped, "file", "fx-1", "open_question", "nobody's in particular")
	mustRun(t, unstamped, "withdraw", "fx-1/questions/1", "--text", "the card was narrowed", "--actor", "sam")

	// The kind guard refuses a criterion to a non-operator from every state.
	for _, state := range []string{bench.ItemPending, bench.ItemFailed, bench.ItemVerified} {
		kinded := statesFixture(t, "a card carrying a criterion at "+state)
		standAt(t, kinded, state)
		got := mustRefuse(t, kinded, "withdraw", "fx-1/criteria/1", "--text", "away", "--actor", "sam")
		assertRefusal(t, got, contract.NotOperator, "a withdrawal of a criterion standing at "+state)
		mustRun(t, kinded, "withdraw", "fx-1/criteria/1", "--text", "the operator decided")
	}
	resolved := statesFixture(t, "a card carrying a resolved question")
	standAt(t, resolved, bench.ItemResolved)
	mustRun(t, resolved, "withdraw", "fx-1/questions/1", "--text", "the card was narrowed", "--actor", "sam")
}

// TestBothNewVerbsDemandAnAnswerOfRecord is dinah-472/criteria/7. A waiver
// with no recorded reason is a silent way past a check, and a withdrawal with
// none says only that somebody made a question go away.
func TestBothNewVerbsDemandAnAnswerOfRecord(t *testing.T) {
	for _, verb := range []string{"waive", "withdraw"} {
		root := statesFixture(t, "a card carrying an item for "+verb)
		mustRun(t, root, "file", "fx-1", "acceptance_criterion", "something to settle")
		mustRun(t, root, "comment", "fx-1/criteria/1", "the reason written out first")

		bare := mustRefuse(t, root, verb, "fx-1/criteria/1")
		assertRefusal(t, bare, contract.Malformed, verb+" carrying neither a comment nor --text")
		if !strings.Contains(bare.errw, bench.ItemResolutionField) {
			t.Errorf("%s does not name the missing answer: %s", verb, bare.errw)
		}
		both := mustRefuse(t, root, verb, "fx-1/criteria/1", "fx-1/criteria/1/comments/1", "--text", "and also this")
		assertRefusal(t, both, contract.Usage, verb+" carrying both a comment and --text")

		// The comment form, and then the text form on a second item, since
		// the first is settled by the time the comment form has run.
		mustRun(t, root, verb, "fx-1/criteria/1", "fx-1/criteria/1/comments/1")
		mustRun(t, root, "file", "fx-1", "acceptance_criterion", "something else to settle")
		mustRun(t, root, verb, "fx-1/criteria/2", "--text", "the reason, written in the same act")
	}
}

// TestNeitherNewVerbInheritsTheKindCheckOrTheCitationObligation is
// dinah-472/criteria/1's second half and /6's own claim, read over every kind.
//
// A gate reads no kind, so an item of any kind can hold a card and an item of
// any kind can need waiving. And a waiver is precisely the case where no check
// was run, so demanding evidence of one would make the state unreachable
// exactly where it is most needed.
func TestNeitherNewVerbInheritsTheKindCheckOrTheCitationObligation(t *testing.T) {
	root := newBenchFromDefinition(t, evidenceStatesDefinition)
	if got := runCLI(t, root, "add", "a card carrying one of each kind"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for _, kind := range bench.ItemKinds {
		mustRun(t, root, "file", "fx-1", kind, "one "+kind+" to settle")
	}
	// The workbench declares an evidence scheme, so a criterion leaving
	// pending through fail is refused with no citation, which is what makes
	// the two waivers below an assertion rather than a coincidence.
	uncited := mustRefuse(t, root, "fail", "fx-1/criteria/1", "--text", "the check did not hold")
	assertRefusal(t, uncited, contract.Uncited, "a failure carrying no citation")

	mustRun(t, root, "waive", "fx-1/criteria/1", "--text", "the operator decided the check need not be run")
	mustRun(t, root, "withdraw", "fx-1/questions/1", "--text", "the question stopped applying")
	mustRun(t, root, "withdraw", "fx-1/decisions/1", "--text", "the decision stopped applying")
}

// evidenceStatesDefinition declares an evidence scheme, so the citation
// obligation is in force and the two verbs that do not inherit it can be shown
// not to.
const evidenceStatesDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Evidence",
  "evidence": { "url": {} },
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work" },
    { "id": "a00000000003", "title": "Done", "kind": "done", "gate_items": true }
  ]
}`

// TestBothNewStatesReleaseEveryHoldAFailedItemKeeps is
// dinah-472/criteria/8, /9 and /10, driven over the three readers that ask
// whether an item is settled: the entry gate, the exit gate and the claim.
func TestBothNewStatesReleaseEveryHoldAFailedItemKeeps(t *testing.T) {
	for _, state := range []string{bench.ItemWaived, bench.ItemWithdrawn} {
		t.Run(state, func(t *testing.T) {
			// The entry gate. The card carries an item naming the done
			// column, and the move is made with no override at all.
			entry := statesFixture(t, "a card held at the gate")
			mustRun(t, entry, "file", "--column", "done", "fx-1", "acceptance_criterion", "something to check")
			mustRun(t, entry, "fail", "fx-1/criteria/1", "--text", "the check did not hold")
			held := mustRefuse(t, entry, "move", "fx-1", "done")
			assertRefusal(t, held, contract.UnresolvedItem, "a move into the gated column against a failed criterion")
			settleTo(t, entry, "fx-1/criteria/1", state)
			moved := mustRun(t, entry, "move", "fx-1", "done")
			_ = moved
			for _, event := range cardJournal(t, entry, cardID(t, entry, "fx-1")) {
				if event.Event == contract.EventMoved && event.Override {
					t.Errorf("the move that %s admitted carries an override marker, and nothing was overridden", state)
				}
			}

			// The exit gate, which reads the same predicate in the other
			// direction.
			exit := statesFixture(t, "a card held on the way out")
			mustRun(t, exit, "move", "fx-1", "doing")
			mustRun(t, exit, "file", "--column", "doing", "fx-1", "acceptance_criterion", "something to check")
			leaving := mustRefuse(t, exit, "move", "fx-1", "review")
			assertRefusal(t, leaving, contract.UnresolvedItemExit, "a forward departure against a pending criterion")
			settleTo(t, exit, "fx-1/criteria/1", state)
			mustRun(t, exit, "move", "fx-1", "review")

			// The claim refusal, which an item naming no column the workbench
			// declares is what raises.
			claimed := statesFixture(t, "a card carrying a misfiled question")
			mustRun(t, claimed, "move", "fx-1", "doing")
			mustRun(t, claimed, "file", "fx-1", "open_question", "filed against nothing")
			setItemColumnByHand(t, claimed, "fx-1/questions/1", "b99999999999")
			refused := mustRefuse(t, claimed, "claim", "fx-1")
			assertRefusal(t, refused, contract.UnresolvedItem, "a claim against an item naming no declared column")
			settleTo(t, claimed, "fx-1/questions/1", state)
			mustRun(t, claimed, "claim", "fx-1")
		})
	}
}

// settleTo lands one item at a named state through the verb that lands it,
// reopening first where the item is already settled.
func settleTo(t *testing.T, root, ref, state string) {
	t.Helper()
	if soleItemStateOf(t, root, ref) != bench.ItemPending {
		mustRun(t, root, "reopen", ref, "the case needs it back at pending")
	}
	switch state {
	case bench.ItemWaived:
		mustRun(t, root, "waive", ref, "--text", "the operator decided")
	case bench.ItemWithdrawn:
		mustRun(t, root, "withdraw", ref, "--text", "the question stopped applying")
	default:
		t.Fatalf("settleTo has no route to %q", state)
	}
}

// setItemColumnByHand writes an item's column key straight onto its anchor,
// which is how a case reaches an item naming a column the workbench does not
// declare: the write path refuses one, and the state it produces is one a
// reshape or a hand edit leaves behind.
func setItemColumnByHand(t *testing.T, root, ref, column string) {
	t.Helper()
	dir := itemDirOf(t, root, ref)
	fm, body, err := bench.ReadItemAnchor(dir)
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	fm.Set(bench.ItemColumnField, column)
	if err := bench.WriteItemAnchor(dir, fm, body); err != nil {
		t.Fatalf("write the anchor of %s: %v", ref, err)
	}
}

// soleItemStateOf is one named item's state, where soleItemState beside it
// answers for the one item a card carries.
func soleItemStateOf(t *testing.T, root, ref string) string {
	t.Helper()
	dir := itemDirOf(t, root, ref)
	item, err := bench.LoadItem(dir)
	if err != nil {
		t.Fatalf("load %s: %v", ref, err)
	}
	return item.State
}

// soleItemResolution is the answer the one item of a card records.
func soleItemResolution(t *testing.T, root, card string) string {
	t.Helper()
	return itemKeyOf(t, root, card+"/criteria/1", bench.ItemResolutionField)
}

// TestReopenComesBackFromBothStatesAndRestoresTheHoldWhereTheCardStillStands
// is dinah-472/criteria/11. Reopening clears the answer and returns the item
// to pending, and what it recovers depends on where the card is standing.
func TestReopenComesBackFromBothStatesAndRestoresTheHoldWhereTheCardStillStands(t *testing.T) {
	for _, state := range []string{bench.ItemWaived, bench.ItemWithdrawn} {
		t.Run(state, func(t *testing.T) {
			root := statesFixture(t, "a card standing before the gate")
			mustRun(t, root, "file", "--column", "done", "fx-1", "acceptance_criterion", "something to check")
			settleTo(t, root, "fx-1/criteria/1", state)
			if got := soleItemResolution(t, root, "fx-1"); got == "" {
				t.Fatalf("the item records no answer after the %s, so the clear below would assert nothing", state)
			}

			mustRun(t, root, "reopen", "fx-1/criteria/1", "the finding stands after all")
			if got := soleItemState(t, root, "fx-1"); got != bench.ItemPending {
				t.Errorf("the item stands at %q after the reopen, wanted %q", got, bench.ItemPending)
			}
			if got := soleItemResolution(t, root, "fx-1"); got != "" {
				t.Errorf("the item still records the answer %q after the reopen", got)
			}
			// The card stands before the column the item names, so the hold
			// is restored.
			held := mustRefuse(t, root, "move", "fx-1", "done")
			assertRefusal(t, held, contract.UnresolvedItem, "a move into the gated column after the reopen")

			// And past it, where the reopen restores nothing: the card is
			// carried in, the item is reopened there, and no refusal follows,
			// because the entry hold fires on arrival and the card has
			// arrived.
			settleTo(t, root, "fx-1/criteria/1", state)
			mustRun(t, root, "move", "fx-1", "done")
			mustRun(t, root, "reopen", "fx-1/criteria/1", "the finding stands after all")
			if got := soleItemState(t, root, "fx-1"); got != bench.ItemPending {
				t.Errorf("the item stands at %q after the second reopen, wanted %q", got, bench.ItemPending)
			}
		})
	}
}

// TestTheStateFieldAndSettleReachBothVerbsWithTheirOwnGuards is
// dinah-472/criteria/12 and /13.
func TestTheStateFieldAndSettleReachBothVerbsWithTheirOwnGuards(t *testing.T) {
	root := statesFixture(t, "a card carrying two items")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "one to waive")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "one to withdraw")
	mustRun(t, root, "comment", "fx-1/criteria/1", "the operator's reason")
	mustRun(t, root, "comment", "fx-1/criteria/2", "the reason it stopped applying")

	mustRun(t, root, "set", "fx-1/criteria/1", bench.ItemStateField, bench.ItemWaived, "--note", "fx-1/criteria/1/comments/1")
	if got := soleItemStateOf(t, root, "fx-1/criteria/1"); got != bench.ItemWaived {
		t.Errorf("the state write landed the item at %q, wanted %q", got, bench.ItemWaived)
	}
	mustRun(t, root, "set", "fx-1/criteria/2", bench.ItemStateField, bench.ItemWithdrawn, "--note", "fx-1/criteria/2/comments/1")
	if got := soleItemStateOf(t, root, "fx-1/criteria/2"); got != bench.ItemWithdrawn {
		t.Errorf("the state write landed the item at %q, wanted %q", got, bench.ItemWithdrawn)
	}

	// The write runs the verb's own guards rather than a second copy of them,
	// which the operator guard is what shows: the same write by somebody else
	// is refused by name.
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "one nobody may waive")
	mustRun(t, root, "comment", "fx-1/criteria/3", "a reason")
	refused := mustRefuse(t, root, "set", "fx-1/criteria/3", bench.ItemStateField, bench.ItemWaived,
		"--note", "fx-1/criteria/3/comments/1", "--actor", "sam")
	assertRefusal(t, refused, contract.NotOperator, "a state write landing waived, run by somebody else")

	// A value outside the six is refused, and the sentence lists all six.
	unknown := mustRefuse(t, root, "set", "fx-1/criteria/3", bench.ItemStateField, "abandoned")
	assertRefusal(t, unknown, contract.UnknownValue, "a state write naming a value outside the set")
	for _, state := range bench.ItemStates {
		if !strings.Contains(unknown.errw, state) {
			t.Errorf("the refusal does not list %s: %s", state, unknown.errw)
		}
	}
}

// TestTheUnresolvedFiltersAndThePrimeQueueDropBothStates is
// dinah-472/criteria/14 and /15.
func TestTheUnresolvedFiltersAndThePrimeQueueDropBothStates(t *testing.T) {
	root := statesFixture(t, "a card carrying one item of every state")
	mustRun(t, root, "file", "fx-1", "open_question", "the waived question", "--owner", "operator")
	mustRun(t, root, "file", "fx-1", "open_question", "the withdrawn question", "--owner", "operator")
	mustRun(t, root, "file", "fx-1", "open_question", "the pending question", "--owner", "operator")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "the failed criterion")
	mustRun(t, root, "waive", "fx-1/questions/1", "--text", "the operator decided")
	mustRun(t, root, "withdraw", "fx-1/questions/2", "--text", "it stopped applying")
	mustRun(t, root, "fail", "fx-1/criteria/1", "--text", "the check did not hold")

	for _, argv := range [][]string{
		{"show", "fx-1", "--fields", "checklist", "--unresolved"},
		{"list", "fx-1/checklist", "--unresolved"},
	} {
		got := mustRun(t, root, argv...)
		for _, gone := range []string{"the waived question", "the withdrawn question"} {
			if strings.Contains(got.out, gone) {
				t.Errorf("%v still serves %q:\n%s", argv, gone, got.out)
			}
		}
		for _, kept := range []string{"the pending question", "the failed criterion"} {
			if !strings.Contains(got.out, kept) {
				t.Errorf("%v no longer serves %q:\n%s", argv, kept, got.out)
			}
		}
	}

	primed := mustRun(t, root, "prime")
	for _, gone := range []string{"the waived question", "the withdrawn question"} {
		if strings.Contains(primed.out, gone) {
			t.Errorf("prime still counts %q:\n%s", gone, primed.out)
		}
	}
	if !strings.Contains(primed.out, "the pending question") {
		t.Errorf("prime no longer counts the pending question:\n%s", primed.out)
	}
}

// TestAnUnrecognisedStateGoesOnHoldingAndGoesOnRefusing is
// dinah-472/criteria/16. Reading a damaged file as settled lets through the
// thing a hold exists to catch, so a state outside the six releases nothing.
func TestAnUnrecognisedStateGoesOnHoldingAndGoesOnRefusing(t *testing.T) {
	root := statesFixture(t, "a card carrying an item in no state at all")
	mustRun(t, root, "move", "fx-1", "doing")
	mustRun(t, root, "file", "--column", "done", "fx-1", "acceptance_criterion", "something to check")
	setItemStateByHand(t, root, "fx-1/criteria/1", "abandoned")

	held := mustRefuse(t, root, "move", "fx-1", "done")
	assertRefusal(t, held, contract.UnresolvedItem, "a move into the gated column against an unrecognised state")

	// And the claim refusal, which reads the other predicate. The item is an
	// open question naming a column the workbench does not declare, because
	// the claim refusal exempts an acceptance criterion outright and exempts
	// an item whose column resolves.
	mustRun(t, root, "file", "fx-1", "open_question", "filed against nothing")
	setItemColumnByHand(t, root, "fx-1/questions/1", "b99999999999")
	setItemStateByHand(t, root, "fx-1/questions/1", "abandoned")
	refused := mustRefuse(t, root, "claim", "fx-1")
	assertRefusal(t, refused, contract.UnresolvedItem, "a claim against an unrecognised state")
}

// setItemStateByHand writes a state key straight onto an item's anchor, which
// is the one way to reach a token the verbs refuse to write.
func setItemStateByHand(t *testing.T, root, ref, state string) {
	t.Helper()
	dir := itemDirOf(t, root, ref)
	fm, body, err := bench.ReadItemAnchor(dir)
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	fm.Set(bench.ItemStateField, state)
	if err := bench.WriteItemAnchor(dir, fm, body); err != nil {
		t.Fatalf("write the anchor of %s: %v", ref, err)
	}
}

// TestEachNewVerbWritesItsOwnJournalEvent is dinah-472/criteria/21 and /22.
// A journal saying an item was resolved when somebody waived it says the
// question was answered and the work held, and a reader of the card's history
// has no other source for what happened.
func TestEachNewVerbWritesItsOwnJournalEvent(t *testing.T) {
	root := statesFixture(t, "a card carrying two items")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "one to waive")
	mustRun(t, root, "file", "fx-1", "open_question", "one to withdraw")
	mustRun(t, root, "fail", "fx-1/criteria/1", "--text", "the check did not hold")
	mustRun(t, root, "waive", "fx-1/criteria/1", "--text", "the operator decided")
	mustRun(t, root, "withdraw", "fx-1/questions/1", "--text", "it stopped applying")

	waived, withdrawn := 0, 0
	for _, event := range cardJournal(t, root, cardID(t, root, "fx-1")) {
		switch event.Event {
		case contract.EventItemWaived:
			waived++
			if event.From != bench.ItemFailed || event.To != bench.ItemWaived {
				t.Errorf("the waived line reads from %q to %q", event.From, event.To)
			}
			if event.Actor.Name != "alka" {
				t.Errorf("the waived line names the actor %q", event.Actor.Name)
			}
		case contract.EventItemWithdrawn:
			withdrawn++
			if event.From != bench.ItemPending || event.To != bench.ItemWithdrawn {
				t.Errorf("the withdrawn line reads from %q to %q", event.From, event.To)
			}
		case contract.EventItemResolved, contract.EventItemVerified:
			t.Errorf("the journal carries a %s line, and neither new verb writes one", event.Event)
		}
	}
	if waived != 1 || withdrawn != 1 {
		t.Errorf("the journal carries %d waived lines and %d withdrawn, wanted one of each", waived, withdrawn)
	}

	// Both names are members of the event roster, so a query over them
	// resolves and selects the card rather than being refused.
	for _, event := range []string{contract.EventItemWaived, contract.EventItemWithdrawn} {
		got := mustRun(t, root, "query", "event:"+event)
		if !strings.Contains(got.out, "fx-1") {
			t.Errorf("a query for %s selects no card:\n%s", event, got.out)
		}
	}
}
