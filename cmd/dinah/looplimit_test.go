package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// declareLoopLimit writes loop_limit into one column's own anchor, found by
// the title the init flow gives it. Nothing in the tool sets the field, so a
// test reaching the rendered surface writes the declaration the way a person
// editing a column.md would, which is what declareRejectTo already does one
// file over for the other column-level declaration.
func declareLoopLimit(t *testing.T, root, title, limit string) {
	t.Helper()
	columns := filepath.Join(soleBenchDir(t, root), bench.ColumnsDir)
	for _, id := range bench.ListIDs(columns) {
		path := filepath.Join(columns, id, bench.ColumnAnchor)
		text, err := bench.ReadText(path)
		if err != nil {
			t.Fatalf("read a column: %v", err)
		}
		fm, body := bench.ParseAnchor(text)
		if fm.Value("title") != title {
			continue
		}
		fm.Set("loop_limit", limit)
		if err := os.WriteFile(path, []byte(fm.Render(body)), 0o644); err != nil {
			t.Fatalf("write a column: %v", err)
		}
		return
	}
	t.Fatalf("the fixture flow carries no column titled %s", title)
}

// loopedCard files a card, carries it to doing, sends it back to intake once
// and returns it, which is one regressive departure from doing. The card is
// standing at doing when this returns.
func loopedCard(t *testing.T, root string) {
	t.Helper()
	if got := runCLI(t, root, "add", "Write the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")
	if got := runCLI(t, root, "move", "fx-1", "intake"); got.code != 0 {
		t.Fatalf("move back: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")
}

// TestInstructionsPrintTheLoopStanding is dinah-364 AC-3 at the terminal. The
// undeclared half is the guard that matters: a renderer printing the line
// unconditionally would pass the first half alone and would tell every agent
// on every ordinary board that it stands in a bounded loop.
func TestInstructionsPrintTheLoopStanding(t *testing.T) {
	t.Run("a declaring column prints the line", func(t *testing.T) {
		root := newBench(t)
		declareLoopLimit(t, root, "Doing", "2")
		loopedCard(t, root)

		served := runCLI(t, root, "instructions", "fx-1")
		if served.code != 0 {
			t.Fatalf("instructions: %d %s", served.code, served.errw)
		}
		wanted := msg.For("en").T("instructions.loop", "count", "1", "limit", "2", "column", "doing")
		if !strings.Contains(served.out, wanted) {
			t.Errorf("wanted the line %q in:\n%s", wanted, served.out)
		}
	})

	t.Run("a column declaring nothing prints no line", func(t *testing.T) {
		root := newBench(t)
		loopedCard(t, root)

		served := runCLI(t, root, "instructions", "fx-1")
		if served.code != 0 {
			t.Fatalf("instructions: %d %s", served.code, served.errw)
		}
		// The label is asserted against rather than the whole rendered
		// sentence, because a renderer that filled the slots with zeroes
		// would still carry the label and nothing else here would notice.
		for _, count := range []string{"0", "1"} {
			line := msg.For("en").T("instructions.loop", "count", count, "limit", "0", "column", "doing")
			if strings.Contains(served.out, line) {
				t.Errorf("a column declaring nothing printed %q:\n%s", line, served.out)
			}
		}
		if strings.Contains(served.out, "regressive departures") {
			t.Errorf("a column declaring nothing printed a loop line:\n%s", served.out)
		}
	})
}

// TestTheJSONInstructionsCarryTheLoopMember is dinah-364 AC-3 on the machine
// surface, and it is what pins the member's spelling on the wire rather than
// the Go field's name.
func TestTheJSONInstructionsCarryTheLoopMember(t *testing.T) {
	root := newBench(t)
	declareLoopLimit(t, root, "Doing", "2")
	loopedCard(t, root)

	served := runCLI(t, root, "instructions", "fx-1", "--json")
	if served.code != 0 {
		t.Fatalf("instructions: %d %s", served.code, served.errw)
	}
	if !strings.Contains(served.out, `"loop"`) {
		t.Fatalf("the member is not spelled loop on the wire:\n%s", served.out)
	}
	var decoded verb.Served
	if err := json.Unmarshal([]byte(served.out), &decoded); err != nil {
		t.Fatalf("decode: %v\n%s", err, served.out)
	}
	if decoded.Loop == nil {
		t.Fatal("wanted a loop member, got none")
	}
	if decoded.Loop.Column != "doing" || decoded.Loop.Limit != 2 || decoded.Loop.Count != 1 || decoded.Loop.AtLimit {
		t.Errorf("wanted doing, 2, 1 and not at the limit, got %+v", decoded.Loop)
	}

	plain := newBench(t)
	loopedCard(t, plain)
	bare := runCLI(t, plain, "instructions", "fx-1", "--json")
	if bare.code != 0 {
		t.Fatalf("instructions: %d %s", bare.code, bare.errw)
	}
	if strings.Contains(bare.out, `"loop"`) {
		t.Errorf("a column declaring nothing carried the member:\n%s", bare.out)
	}
}

// TestHelpMoveListsTheLoopLimitRow is dinah-364 AC-6 at the surface a person
// reads. The row is Dinah's own ninth, appended after the profile's eight, and
// the generated per-command help is where the tool says the refusal exists at
// all.
func TestHelpMoveListsTheLoopLimitRow(t *testing.T) {
	container := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(container, "home"))
	t.Setenv("DINAH_ACTOR", "alka")

	got := runCLI(t, container, "help", "move")
	if got.code != 0 {
		t.Fatalf("help move: %d %s", got.code, got.errw)
	}
	entry, ok := msg.BaseEntry("check.move.9")
	if !ok || entry.Text == "" {
		t.Fatal("the catalog carries no check.move.9")
	}
	if !strings.Contains(got.out, entry.Text) {
		t.Errorf("help move carries no check.move.9 row:\n%s", got.out)
	}
	if !strings.Contains(got.out, contract.AtLoopLimit) {
		t.Errorf("help move names no %s refusal:\n%s", contract.AtLoopLimit, got.out)
	}
}

// TestCheckExitsFiveOnACardAtItsLoopLimit is dinah-364 OQ-2. The finding rides
// the findings path every other check finding rides, so it earns exit 5 rather
// than the 2 a refused invocation earns, and this board has shipped that
// collision once already: before dinah-346 and dinah-358 both answers were 2
// and a caller reading the exit code could not tell a bad --workbench from a
// workbench carrying defects. The assertion is against ExitCodeForRead and
// against ExitCode(OutcomeRefused) separately, so a build that collapsed them
// again fails here rather than passing on a number that happens to match.
func TestCheckExitsFiveOnACardAtItsLoopLimit(t *testing.T) {
	root := newBench(t)
	declareLoopLimit(t, root, "Doing", "1")
	loopedCard(t, root)

	checked := runCLI(t, root, "check", "--json")
	wanted := contract.ExitCodeForRead(contract.ReadFindings)
	if checked.code != wanted {
		t.Fatalf("wanted exit %d, got %d\n%s%s", wanted, checked.code, checked.out, checked.errw)
	}
	if checked.code == contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("a workbench carrying findings answered the refusal's own code %d", checked.code)
	}

	var report verb.CheckReport
	if err := json.Unmarshal([]byte(checked.out), &report); err != nil {
		t.Fatalf("decode: %v\n%s", err, checked.out)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("wanted the loop finding alone, got %d: %+v", len(report.Findings), report.Findings)
	}
	if report.Findings[0].Key != bench.FindingAtLoopLimit {
		t.Errorf("wanted %s, got %s", bench.FindingAtLoopLimit, report.Findings[0].Key)
	}
	if report.Findings[0].Detail != "doing" {
		t.Errorf("wanted the declaring column named, got %q", report.Findings[0].Detail)
	}
}

// TestARegressiveMovePastTheLimitIsRefusedAtTheTerminal is dinah-364 AC-4 at
// the head, which is where the operator meets the override and reads the
// sentence the catalog composes.
func TestARegressiveMovePastTheLimitIsRefusedAtTheTerminal(t *testing.T) {
	root := newBench(t)
	declareLoopLimit(t, root, "Doing", "1")
	loopedCard(t, root)

	refused := runCLI(t, root, "move", "fx-1", "intake")
	if refused.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("wanted the refusal's exit code, got %d\n%s", refused.code, refused.errw)
	}
	if !strings.Contains(refused.errw, contract.AtLoopLimit) {
		t.Errorf("the refusal names no %s:\n%s", contract.AtLoopLimit, refused.errw)
	}
	sentence := msg.For("en").T("refusal.dinah.at-loop-limit", "detail", "doing")
	if !strings.Contains(refused.errw, sentence) {
		t.Errorf("wanted the sentence %q in:\n%s", sentence, refused.errw)
	}

	overridden := runCLI(t, root, "move", "fx-1", "intake", "--override")
	if overridden.code != 0 {
		t.Fatalf("the override was refused: %d %s", overridden.code, overridden.errw)
	}
}
