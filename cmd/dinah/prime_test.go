package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// writePrimeCLIGlobal writes the user-global instruction layer under the
// home newBench pointed DINAH_HOME at, so a prime call sees both Global and
// Standing carrying text at once.
func writePrimeCLIGlobal(t *testing.T, text string) {
	t.Helper()
	home := os.Getenv("DINAH_HOME")
	if home == "" {
		t.Fatal("DINAH_HOME is not set; call this after the fixture is built")
	}
	dir := bench.UserBase(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("the user base: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, bench.InstructionsName), []byte(text), 0o644); err != nil {
		t.Fatalf("the global layer: %v", err)
	}
}

// TestPrimeCLIWithHoldingReadyAndPending drives dinah prime's default,
// populated form: a card held, a card ready, and enough pending items to
// trip the operator's own-queue cap, so the full-pending hint and the
// per-column breakdown both draw.
func TestPrimeCLIWithHoldingReadyAndPending(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "set", "workbench", "instructions", "Standing text for the fixture."); got.code != 0 {
		t.Fatalf("set the standing text: %d %s", got.code, got.errw)
	}
	writePrimeCLIGlobal(t, "Global text for the fixture.")

	held := addCard(t, root, "held card")
	carryToDoing(t, root, held)
	if got := runCLI(t, root, "claim", held); got.code != 0 {
		t.Fatalf("claim: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", held, "acceptance_criterion", "verify it", "--owner", "holder"); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}

	// alka is the operator newBench names, so twenty-two unstamped
	// open_questions on cards it does not hold trip the cap: twenty land in
	// Pending, two are withheld, and both file under the doing column.
	for i := 0; i < 22; i++ {
		card := addCard(t, root, "queued card")
		carryToDoing(t, root, card)
		if got := runCLI(t, root, "file", card, "open_question", "an operator question", "--column", "doing"); got.code != 0 {
			t.Fatalf("file: %d %s", got.code, got.errw)
		}
	}

	addCard(t, root, "ready at intake")

	plain := runCLI(t, root, "prime")
	if plain.code != 0 {
		t.Fatalf("prime: %d %s", plain.code, plain.errw)
	}
	for _, want := range []string{
		held, "ready at intake", "an operator question",
		"showing 21 of 23", "2 withheld",
		"Run `dinah prime --full-pending`",
		"Standing text for the fixture.",
		"Global text for the fixture.",
	} {
		if !strings.Contains(plain.out, want) {
			t.Errorf("dinah prime does not carry %q:\n%s", want, plain.out)
		}
	}
	if strings.Contains(plain.out, "omitted (--brief)") {
		t.Errorf("the plain call carries the brief line: %s", plain.out)
	}

	full := runCLI(t, root, "prime", "--full-pending")
	if full.code != 0 {
		t.Fatalf("prime --full-pending: %d %s", full.code, full.errw)
	}
	if strings.Contains(full.out, "withheld") {
		t.Errorf("--full-pending still names something withheld:\n%s", full.out)
	}

	brief := runCLI(t, root, "prime", "--brief")
	if brief.code != 0 {
		t.Fatalf("prime --brief: %d %s", brief.code, brief.errw)
	}
	if !strings.Contains(brief.out, "omitted (--brief)") {
		t.Errorf("dinah prime --brief does not omit the standing text:\n%s", brief.out)
	}
	if strings.Contains(brief.out, "Standing text for the fixture.") {
		t.Errorf("--brief carried the standing text in full:\n%s", brief.out)
	}
	if !strings.Contains(brief.out, "dinah instructions") {
		t.Errorf("--brief carries no recovery hint:\n%s", brief.out)
	}
}

// TestPrimeCLIWithNothingHeldReadyOrPending drives the three empty-case
// lines together, on a freshly initialized workbench nobody has touched.
func TestPrimeCLIWithNothingHeldReadyOrPending(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "prime")
	if got.code != 0 {
		t.Fatalf("prime: %d %s", got.code, got.errw)
	}
	for _, want := range []string{"Holding: nothing.", "Ready for you: nothing.", "Pending for you: nothing."} {
		if !strings.Contains(got.out, want) {
			t.Errorf("dinah prime does not carry %q:\n%s", want, got.out)
		}
	}

	// --brief with nothing held falls back to the workbench's first column
	// for the recovery example, since there is no held card's own column to
	// name.
	brief := runCLI(t, root, "prime", "--brief")
	if brief.code != 0 {
		t.Fatalf("prime --brief: %d %s", brief.code, brief.errw)
	}
	if !strings.Contains(brief.out, "dinah instructions intake") {
		t.Errorf("--brief with nothing held does not name the first column in its recovery hint:\n%s", brief.out)
	}
}

// TestPrimeCLIReadyAboveTier drives renderPrimeReady's other branch: a
// column holding ready work above the caller's declared tier, which carries
// no card and prints the above-tier sentence instead.
func TestPrimeCLIReadyAboveTier(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	ready := runCLI(t, root, "add", "needs frontier", "--column", "test")
	if ready.code != 0 {
		t.Fatalf("add: %d %s", ready.code, ready.errw)
	}
	// A column's own tier default creates no per-card requirement by
	// itself (topTierDefinition's own AC-7 shape); the card needs its own.
	if got := runCLI(t, root, "set", "fx-1", "tier", "frontier"); got.code != 0 {
		t.Fatalf("set the card's tier: %d %s", got.code, got.errw)
	}

	got := claimingAs(t, "workhorse", root, "prime")
	if got.code != 0 {
		t.Fatalf("prime: %d %s", got.code, got.errw)
	}
	if !strings.Contains(got.out, "above your declared tier") {
		t.Fatalf("dinah prime does not withhold the above-tier card:\n%s", got.out)
	}
}
