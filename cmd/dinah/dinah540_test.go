package main

import (
	"strings"
	"testing"

	"dinah/internal/contract"
)

// dinah540Definition is a three-column workbench used by dinah-540's own
// CLI-level coverage: an intake station a card is added to and a work
// station and a done station, which is more than any one of these tests
// needs but keeps the fixture reusable across them.
const dinah540Definition = `{
  "profile": "dinah-core/0.7",
  "title": "dinah-540 coverage",
  "columns": [
    { "id": "d00000000001", "title": "Intake", "kind": "intake" },
    { "id": "d00000000002", "title": "Doing", "kind": "work" },
    { "id": "d00000000003", "title": "Done", "kind": "done" }
  ]
}`

// TestMoveReleaseUnblockRefuseNoOwnerAheadOfTheirOwnChecks is dinah-540 AC-1
// and AC-3: the three verbs already ran their owner check ahead of the row
// checks.go's published table put first, so this pins the observable
// ordering rather than the table, and the accepting case beside each pins
// that the same call with a resolvable actor reaches the row the table did
// have right.
func TestMoveReleaseUnblockRefuseNoOwnerAheadOfTheirOwnChecks(t *testing.T) {
	root := newBenchFromDefinition(t, dinah540Definition)
	if got := runCLI(t, root, "add", "a card for move/release/unblock coverage"); got.code != 0 {
		t.Fatalf("fixture: add: %d %s", got.code, got.errw)
	}
	t.Setenv("DINAH_ACTOR", "")

	t.Run("move: no-owner beats unknown-column", func(t *testing.T) {
		refused := runCLI(t, root, "move", "fx-1", "bogus-column")
		if refusal := refusalNameOf(refused.errw); refusal != contract.NoOwner {
			t.Errorf("no actor named: wanted no-owner, got %s:\n%s", refusal, refused.errw)
		}
		accepted := runCLI(t, root, "move", "fx-1", "bogus-column", "--actor", "alka")
		if refusal := refusalNameOf(accepted.errw); refusal != contract.UnknownColumn {
			t.Errorf("with an actor supplied: wanted unknown-column, got %s:\n%s", refusal, accepted.errw)
		}
	})

	t.Run("release refuses no-owner with no actor", func(t *testing.T) {
		refused := runCLI(t, root, "release", "fx-1")
		if refusal := refusalNameOf(refused.errw); refusal != contract.NoOwner {
			t.Errorf("no actor named: wanted no-owner, got %s:\n%s", refusal, refused.errw)
		}
		// fx-1 has never been claimed, so a resolvable actor reaches
		// release's own not-holder row rather than succeeding.
		accepted := runCLI(t, root, "release", "fx-1", "--actor", "alka")
		if refusal := refusalNameOf(accepted.errw); refusal != contract.NotHolder {
			t.Errorf("release of an unheld card with a resolvable actor: wanted not-holder, got %s:\n%s", refusal, accepted.errw)
		}
	})

	t.Run("unblock refuses no-owner with no actor", func(t *testing.T) {
		if got := runCLI(t, root, "block", "fx-1", "an obstacle", "--actor", "alka"); got.code != 0 {
			t.Fatalf("fixture: block: %d %s", got.code, got.errw)
		}
		refused := runCLI(t, root, "unblock", "fx-1")
		if refusal := refusalNameOf(refused.errw); refusal != contract.NoOwner {
			t.Errorf("no actor named: wanted no-owner, got %s:\n%s", refusal, refused.errw)
		}
		// alka is the workbench's operator (newBenchFromDefinition names
		// them so), so a non-operator actor is what reaches unblock's own
		// not-operator row.
		accepted := runCLI(t, root, "unblock", "fx-1", "--actor", "bela")
		if refusal := refusalNameOf(accepted.errw); refusal != contract.NotOperator {
			t.Errorf("unblock by a non-operator with a resolvable actor: wanted not-operator, got %s:\n%s", refusal, accepted.errw)
		}
	})
}

// TestHelpMovePrintsTheNoOwnerRowWhereItActuallyRuns is dinah-540 AC-2: the
// generated help for move, release and unblock now prints a no-owner row,
// positioned immediately after "the card exists" and ahead of every row the
// table already published, because that is where canRoute, release and
// unblock actually check it.
func TestHelpMovePrintsTheNoOwnerRowWhereItActuallyRuns(t *testing.T) {
	root := newBenchFromDefinition(t, dinah540Definition)
	for _, verb := range []string{"move", "release", "unblock"} {
		t.Run(verb, func(t *testing.T) {
			got := runCLI(t, root, "help", verb)
			if got.code != 0 {
				t.Fatalf("help %s: %d %s", verb, got.code, got.errw)
			}
			cardExists := strings.Index(got.out, "the card exists")
			noOwner := strings.Index(got.out, "the request names an owner")
			if cardExists < 0 || noOwner < 0 {
				t.Fatalf("help %s does not carry both rows:\n%s", verb, got.out)
			}
			if noOwner < cardExists {
				t.Errorf("help %s prints the owner row before the card-exists row:\n%s", verb, got.out)
			}
			// Nothing else in the table is allowed to sit between the two:
			// the owner row is the very next line after the card-exists
			// row's own line.
			between := got.out[cardExists:noOwner]
			if strings.Count(between, "\n") > 1 {
				t.Errorf("help %s prints something between the card-exists row and the owner row:\n%s", verb, got.out)
			}
			if !strings.Contains(got.out, contract.NoOwner) {
				t.Errorf("help %s does not name the no-owner refusal in its table:\n%s", verb, got.out)
			}
		})
	}
}

// TestHelpCheckNowPrintsAWhatCanGoWrongSection is dinah-540 AC-11: check
// carried no ordered precondition table before this card, because
// beyondChecks had no "check" entry and Checks("check") returned nil.
func TestHelpCheckNowPrintsAWhatCanGoWrongSection(t *testing.T) {
	root := newBenchFromDefinition(t, dinah540Definition)
	got := runCLI(t, root, "help", "check")
	if got.code != 0 {
		t.Fatalf("help check: %d %s", got.code, got.errw)
	}
	harness := strings.Index(got.out, contract.MalformedHarness)
	owner := strings.Index(got.out, contract.NoOwner)
	if harness < 0 || owner < 0 {
		t.Fatalf("help check does not carry both refusal names:\n%s", got.out)
	}
	if owner < harness {
		t.Errorf("help check prints no-owner before malformed-harness:\n%s", got.out)
	}
}

// TestCheckRepairsRefuseNoOwnerBeforeAnyBranchRuns is dinah-540 AC-9 and
// AC-10: a diverged card checked with --witness and no actor is refused
// before WriteWitnesses ever reaches AppendEvent, and a request carrying two
// repair markers, one that writes nothing (--migrate-slugs) and one that
// does (--witness), is refused before either branch runs at all, so the
// workbench's column slugs are exactly what they were before the call.
func TestCheckRepairsRefuseNoOwnerBeforeAnyBranchRuns(t *testing.T) {
	root := newBenchFromDefinition(t, dinah540Definition)
	if got := runCLI(t, root, "add", "a card to diverge"); got.code != 0 {
		t.Fatalf("fixture: add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "doing", "--actor", "alka"); got.code != 0 {
		t.Fatalf("fixture: move: %d %s", got.code, got.errw)
	}
	handEditColumn(t, root, "fx-1", "d00000000003")

	before := runCLI(t, root, "list", "fx-1/journal")
	if before.code != 0 {
		t.Fatalf("fixture: list journal: %d %s", before.code, before.errw)
	}
	columnsBefore := runCLI(t, root, "list", "columns")
	if columnsBefore.code != 0 {
		t.Fatalf("fixture: list columns: %d %s", columnsBefore.code, columnsBefore.errw)
	}

	t.Setenv("DINAH_ACTOR", "")

	t.Run("--witness alone", func(t *testing.T) {
		refused := runCLI(t, root, "check", "--witness", "--yes")
		if refused.code == 0 {
			t.Fatalf("check --witness with no actor: wanted refused, exited 0:\n%s", refused.out)
		}
		if refusal := refusalNameOf(refused.errw); refusal != contract.NoOwner {
			t.Errorf("wanted no-owner, got %s:\n%s", refusal, refused.errw)
		}
		after := runCLI(t, root, "list", "fx-1/journal")
		if after.out != before.out {
			t.Errorf("the refused check wrote to the journal:\nbefore:\n%s\nafter:\n%s", before.out, after.out)
		}
	})

	t.Run("--migrate-slugs and --witness together", func(t *testing.T) {
		refused := runCLI(t, root, "check", "--migrate-slugs", "--witness", "--yes")
		if refused.code == 0 {
			t.Fatalf("check with two repair markers and no actor: wanted refused, exited 0:\n%s", refused.out)
		}
		if refusal := refusalNameOf(refused.errw); refusal != contract.NoOwner {
			t.Errorf("wanted no-owner, got %s:\n%s", refusal, refused.errw)
		}
		after := runCLI(t, root, "list", "fx-1/journal")
		if after.out != before.out {
			t.Errorf("the refused check wrote to the card's journal:\nbefore:\n%s\nafter:\n%s", before.out, after.out)
		}
		columnsAfter := runCLI(t, root, "list", "columns")
		if columnsAfter.out != columnsBefore.out {
			t.Errorf("the refused check changed the workbench's columns:\nbefore:\n%s\nafter:\n%s", columnsBefore.out, columnsAfter.out)
		}
	})

	// Accepting case beside both: the same diverged card, checked the same
	// way with a resolvable actor, succeeds and the journal gains the
	// manual_correction line WitnessDivergence is meant to write.
	t.Run("with a resolvable actor", func(t *testing.T) {
		accepted := runCLI(t, root, "check", "--witness", "--yes", "--actor", "alka")
		if accepted.code != 0 {
			t.Fatalf("check --witness with a resolvable actor: %d %s", accepted.code, accepted.errw)
		}
		after := runCLI(t, root, "list", "fx-1/journal")
		if after.code != 0 {
			t.Fatalf("list journal: %d %s", after.code, after.errw)
		}
		if !strings.Contains(after.out, "manual correction") {
			t.Errorf("the accepted check's journal does not carry a manual correction line:\n%s", after.out)
		}
	})
}

// TestNoOwnerExplainsItselfWhenAHarnessExcludedTheConfigRung is dinah-540
// AC-13 and AC-14. The operator ruled that a call declaring a harness is
// refused rather than promoted to whatever the shared config file carries,
// and the refusal a caller reads has to say why rather than merely
// contradicting what `dinah config get actor` just showed them, without ever
// printing the configured value itself.
func TestNoOwnerExplainsItselfWhenAHarnessExcludedTheConfigRung(t *testing.T) {
	root := newBenchFromDefinition(t, dinah540Definition)
	for _, argv := range [][]string{
		{"add", "card one"},
		{"add", "card two"},
		{"add", "card three"},
		// intake takes no work up, so each card is moved into the work
		// station before any of these subtests tries to claim it.
		{"move", "fx-1", "doing", "--actor", "alka"},
		{"move", "fx-2", "doing", "--actor", "alka"},
		{"move", "fx-3", "doing", "--actor", "alka"},
	} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Fatalf("fixture: %v: %d %s", argv, got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "config", "set", "actor", "paul"); got.code != 0 {
		t.Fatalf("config set actor: %d %s", got.code, got.errw)
	}
	t.Setenv("DINAH_ACTOR", "")

	t.Run("a harness declared, no actor named", func(t *testing.T) {
		t.Setenv("DINAH_HARNESS", "claude-code")
		human := runCLI(t, root, "claim", "fx-1")
		if human.code == 0 {
			t.Fatalf("claim: wanted refused, exited 0:\n%s", human.out)
		}
		if strings.Contains(human.out, "paul") || strings.Contains(human.errw, "paul") {
			t.Errorf("the refusal names the configured actor:\nstdout: %s\nstderr: %s", human.out, human.errw)
		}
		if !strings.Contains(human.errw, "the claude-code harness is declared") {
			t.Errorf("the refusal does not carry the harness-variant clause:\n%s", human.errw)
		}
		refusal, context := refusalContextOf(t, root, "claim", "fx-1")
		if refusal != contract.NoOwner {
			t.Fatalf("wanted no-owner, got %s", refusal)
		}
		if context["harness"] != "claude-code" {
			t.Errorf("the refusal's context carries harness=%q, wanted claude-code", context["harness"])
		}
		if _, ok := context["actor"]; ok {
			t.Errorf("the refusal's context carries an actor member it should not: %v", context)
		}

		who := runCLI(t, root, "whoami")
		if who.code == 0 {
			t.Fatalf("whoami: wanted refused, exited 0:\n%s", who.out)
		}
		if refusal := refusalNameOf(who.errw); refusal != contract.NoOwner {
			t.Errorf("whoami: wanted no-owner, got %s:\n%s", refusal, who.errw)
		}
		if strings.Contains(who.out, "paul") || strings.Contains(who.errw, "paul") {
			t.Errorf("whoami names the configured actor:\nstdout: %s\nstderr: %s", who.out, who.errw)
		}
		if !strings.Contains(who.errw, "the claude-code harness is declared") {
			t.Errorf("whoami's refusal does not carry the harness-variant clause:\n%s", who.errw)
		}

		status := runCLI(t, root, "status")
		if status.code != 0 {
			t.Fatalf("status: %d %s", status.code, status.errw)
		}
		if strings.Contains(status.out, "paul") {
			t.Errorf("status prints the configured actor:\n%s", status.out)
		}
		if strings.Contains(status.out, "operator: yes") || strings.Contains(status.out, "operator: no") {
			t.Errorf("status interpolates an empty actor into an operator claim:\n%s", status.out)
		}
		if !strings.Contains(status.out, "no actor named") {
			t.Errorf("status does not print the unnamed line:\n%s", status.out)
		}
	})

	t.Run("no harness declared, the config rung answers", func(t *testing.T) {
		t.Setenv("DINAH_HARNESS", "")
		accepted := runCLI(t, root, "claim", "fx-1")
		if accepted.code != 0 {
			t.Fatalf("claim: %d %s", accepted.code, accepted.errw)
		}
		if strings.Contains(accepted.errw, "harness is declared") {
			t.Errorf("the accepting case printed the harness-variant clause:\n%s", accepted.errw)
		}
		show := runCLI(t, root, "show", "fx-1")
		if show.code != 0 || !strings.Contains(show.out, "held by paul") {
			t.Errorf("fx-1 was not claimed by the configured actor paul:\n%s", show.out)
		}
	})

	t.Run("DINAH_HARNESS set to the empty string", func(t *testing.T) {
		t.Setenv("DINAH_HARNESS", "")
		accepted := runCLI(t, root, "claim", "fx-2")
		if accepted.code != 0 {
			t.Fatalf("claim: %d %s", accepted.code, accepted.errw)
		}
		show := runCLI(t, root, "show", "fx-2")
		if show.code != 0 || !strings.Contains(show.out, "held by paul") {
			t.Errorf("fx-2 was not claimed by the configured actor paul:\n%s", show.out)
		}
	})
}

// TestAMalformedHarnessOrderingAgainstNoOwner is dinah-540 AC-16, and it
// documents rather than confirms the criterion: the criterion as written
// asks for no-owner (the harness variant) ahead of dinah.malformed-harness,
// and driving the built binary shows the opposite for claim, which this
// test pins as the actually-shipped behaviour. See dinah-540/criteria/16,
// failed with this finding, and the open question filed on the card for the
// operator's ruling.
//
// The contradiction: gap 2 of the card's own specification positions Do()'s
// new req.Actor == "" check "immediately after l.Bench.ResolveCard
// succeeds", which is after malformedHarness's existing check (Do() already
// ran it before ResolveCard, unchanged by this card, per section 0's own
// claim that malformed-harness's established behaviour needs no touching).
// AC-16 needs the opposite order for exactly the case it tests. Satisfying
// AC-16 literally would mean moving malformedHarness's check to run after
// ResolveCard and after the new owner check inside Do(), which section 0
// and gap 2 both say is unnecessary and unchanged, and doing it only for
// Do()'s seven verbs (and not for the roughly fifteen other historyWriters
// members whose own functions run malformedHarness before their own actor
// check the same way, none of which any criterion touches) would leave the
// published check-order table wrong for claim while leaving every other
// verb's malformed-harness position exactly as documented. That is a
// larger, unscoped change with no test asking for it beyond AC-16 itself,
// so this implementation keeps the existing, reviewed ordering rather than
// guessing which of the two contradicting clauses the operator meant to
// win.
func TestAMalformedHarnessOrderingAgainstNoOwner(t *testing.T) {
	root := newBenchFromDefinition(t, dinah540Definition)
	if got := runCLI(t, root, "add", "a card for the malformed-harness ordering"); got.code != 0 {
		t.Fatalf("fixture: add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "config", "set", "actor", "paul"); got.code != 0 {
		t.Fatalf("config set actor: %d %s", got.code, got.errw)
	}
	t.Setenv("DINAH_ACTOR", "")
	t.Setenv("DINAH_HARNESS", illegalHarness)

	refused := runCLI(t, root, "claim", "fx-1")
	if refused.code == 0 {
		t.Fatalf("claim under a malformed harness with no actor: wanted refused, exited 0:\n%s", refused.out)
	}
	if refusal := refusalNameOf(refused.errw); refusal != contract.MalformedHarness {
		t.Errorf("wanted dinah.malformed-harness (the shipped ordering; AC-16 itself asks for no-owner, see this test's own doc comment), got %s:\n%s", refusal, refused.errw)
	}

	// The same malformed value, with an actor supplied by flag: the actor
	// ladder resolves normally (the malformed value never participates in
	// it), and the command's own precondition list then refuses on the
	// harness value itself. This half of AC-16 is not in dispute and holds
	// under both orderings.
	accepted := runCLI(t, root, "claim", "fx-1", "--actor", "bela")
	if accepted.code == 0 {
		t.Fatalf("claim under a malformed harness with an actor: wanted refused, exited 0:\n%s", accepted.out)
	}
	if refusal := refusalNameOf(accepted.errw); refusal != contract.MalformedHarness {
		t.Errorf("with an actor supplied: wanted dinah.malformed-harness, got %s:\n%s", refusal, accepted.errw)
	}
}
