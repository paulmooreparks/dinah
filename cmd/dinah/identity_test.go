package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestWhoamiReportsWhatWasDeclaredAndOmitsWhatWasNot drives dinah-496's whoami
// criterion. Every declared fact and the resolved tier are reported when all
// are declared, and each is omitted when the corresponding fact is not declared
// or resolves to nothing.
func TestWhoamiReportsWhatWasDeclaredAndOmitsWhatWasNot(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)

	t.Run("everything declared", func(t *testing.T) {
		t.Setenv("DINAH_HARNESS", "claude-code")
		t.Setenv("DINAH_PROVIDER", tierTableProvider)
		t.Setenv("DINAH_MODEL", "frontier")
		t.Setenv("DINAH_SERVER", "ollama.com")
		got := runCLI(t, root, "whoami")
		if got.code != 0 {
			t.Fatalf("whoami: %d %s", got.code, got.errw)
		}
		for _, clause := range []string{"claude-code", tierTableProvider, "frontier"} {
			if !strings.Contains(got.out, clause) {
				t.Errorf("whoami does not report %q:\n%s", clause, got.out)
			}
		}
		if !strings.Contains(got.out, "ollama.com") {
			t.Errorf("whoami does not report the declared server:\n%s", got.out)
		}
		// The server the table's frontier entry does not carry means the
		// resolution falls to the second pass, which finds the serverless
		// entry, so the tier is still reported.
		if !strings.Contains(got.out, "tier: frontier") {
			t.Errorf("whoami does not report the resolved tier:\n%s", got.out)
		}
	})

	t.Run("nothing declared", func(t *testing.T) {
		for _, name := range []string{"DINAH_HARNESS", "DINAH_PROVIDER", "DINAH_MODEL", "DINAH_SERVER"} {
			t.Setenv(name, "")
		}
		got := runCLI(t, root, "whoami")
		if got.code != 0 {
			t.Fatalf("whoami: %d %s", got.code, got.errw)
		}
		if lines := strings.Count(strings.TrimSpace(got.out), "\n"); lines != 0 {
			t.Errorf("whoami printed %d extra lines for a caller that declared nothing:\n%s", lines, got.out)
		}
		for _, absent := range []string{"harness", "provider", "model", "server", "tier"} {
			if strings.Contains(got.out, absent+":") {
				t.Errorf("whoami reports %s for a caller that declared none:\n%s", absent, got.out)
			}
		}
	})

	t.Run("a model the table lists nowhere", func(t *testing.T) {
		t.Setenv("DINAH_PROVIDER", tierTableProvider)
		t.Setenv("DINAH_MODEL", "unknown-model")
		got := runCLI(t, root, "--json", "whoami")
		if got.code != 0 {
			t.Fatalf("whoami: %d %s", got.code, got.errw)
		}
		var identity verb.Identity
		if err := json.Unmarshal([]byte(got.out), &identity); err != nil {
			t.Fatalf("the machine answer does not parse: %v", err)
		}
		if identity.Model != "unknown-model" {
			t.Errorf("the answer reports the model as %q", identity.Model)
		}
		if identity.Tier != "" {
			t.Errorf("the answer resolves a tier for a model the table lists nowhere: %q", identity.Tier)
		}
	})
}

// TestAMalformedHarnessRefusesAWriteAndNoRead drives dinah-496's harness
// criterion. A name matching one segment of the declared-field key grammar is
// accepted and stamped; a value outside it is refused under
// dinah.malformed-harness rather than dropped. The refusal reaches every act
// that writes a journal line and no read: with a malformed name set, a claim is
// refused while show, ls and whoami all succeed, and whoami reports the value
// as malformed.
func TestAMalformedHarnessRefusesAWriteAndNoRead(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	if got := runCLI(t, root, "add", "a card to take up"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "plain"); got.code != 0 {
		t.Fatalf("move: %d %s", got.code, got.errw)
	}

	t.Run("a legal name is accepted and stamped", func(t *testing.T) {
		t.Setenv("DINAH_HARNESS", "claude-code")
		if got := runCLI(t, root, "claim", "fx-1"); got.code != 0 {
			t.Fatalf("a claim under a legal harness name was refused: %d %s", got.code, got.errw)
		}
		found := false
		for _, event := range cardEvents(t, root, "fx-1") {
			if event.Actor.Harness == "claude-code" {
				found = true
			}
		}
		if !found {
			t.Error("no journal line carries the declared harness")
		}
		if got := runCLI(t, root, "release", "fx-1"); got.code != 0 {
			t.Fatalf("release: %d %s", got.code, got.errw)
		}
	})

	t.Run("an illegal name refuses the write", func(t *testing.T) {
		t.Setenv("DINAH_HARNESS", "Claude Code")
		refused := runCLI(t, root, "claim", "fx-1")
		if refused.code != 2 {
			t.Fatalf("a claim under a malformed harness name exited %d, wanted 2: %s%s", refused.code, refused.out, refused.errw)
		}
		if name := refusalNameOf(refused.errw); name != contract.MalformedHarness {
			t.Errorf("the refusal name is %s, wanted %s", name, contract.MalformedHarness)
		}
		if !strings.Contains(refused.errw, "Claude Code") {
			t.Errorf("the sentence does not name the value, so a person cannot see what to repair:\n%s", refused.errw)
		}
		// The three reads all answer, which is what keeps a mistyped variable
		// from taking the whole tool away from whoever has to repair it.
		for _, argv := range [][]string{{"show", "fx-1"}, {"list", "cards"}, {"whoami"}} {
			if got := runCLI(t, root, argv...); got.code != 0 {
				t.Errorf("`dinah %s` exited %d under a malformed harness name: %s", strings.Join(argv, " "), got.code, got.errw)
			}
		}
		whoami := runCLI(t, root, "whoami")
		if !strings.Contains(whoami.out, "Claude Code") || !strings.Contains(whoami.out, "malformed") {
			t.Errorf("whoami does not report the value as malformed:\n%s", whoami.out)
		}
	})
}

// TestTheRetiredTierFlagIsRefusedAsAnUnknownArgument drives dinah-496's
// retirement criterion on the command line. Passing --tier to claim, next or
// pull is refused as an argument the command does not take rather than accepted
// and ignored, and the two commands that still write a requirement keep theirs.
func TestTheRetiredTierFlagIsRefusedAsAnUnknownArgument(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "plain"); got.code != 0 {
		t.Fatalf("move: %d %s", got.code, got.errw)
	}
	// The control: the same three invocations without the retired flag are
	// accepted, so each refusal below is the flag rather than the command.
	for _, argv := range [][]string{{"next"}, {"claim", "fx-1"}, {"release", "fx-1"}} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Fatalf("`dinah %s` exited %d, so this case is not testing the flag: %s", strings.Join(argv, " "), got.code, got.errw)
		}
	}
	retired := [][]string{
		{"claim", "fx-1", "--tier", "workhorse"},
		{"next", "--tier", "workhorse"},
		{"pull", "plain", "--tier", "workhorse"},
	}
	for _, argv := range retired {
		got := runCLI(t, root, argv...)
		if got.code != 2 {
			t.Errorf("`dinah %s` exited %d, wanted the refused code 2: %s", strings.Join(argv, " "), got.code, got.errw)
			continue
		}
		if name := refusalNameOf(got.errw); name != contract.Usage {
			t.Errorf("`dinah %s` refused under %s, wanted %s", strings.Join(argv, " "), name, contract.Usage)
		}
	}
	if len(retired) != 3 {
		t.Fatalf("the sweep drove %d commands, wanted the three that retired the flag", len(retired))
	}
	// The two that write a requirement keep the argument, which is what says
	// the retirement is about declaring what a caller is rather than about the
	// word.
	kept := 0
	for _, argv := range [][]string{
		{"column", "new", "Review", "--tier", "apex"},
		{"help", "raise"},
	} {
		kept++
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Errorf("`dinah %s` exited %d: %s", strings.Join(argv, " "), got.code, got.errw)
		}
	}
	if kept != 2 {
		t.Fatalf("the sweep drove %d surviving commands, wanted 2", kept)
	}
}

// illegalHarness is a value outside the one-segment grammar, chosen so that a
// journal line carrying it is findable by one search over the whole store.
const illegalHarness = "Bad_Harness!"

// TestNoWritingCommandTakesAMalformedHarnessName is the completeness half of
// dinah-496's harness criterion, and it is the guard Agent Code Review's first
// round asked for.
//
// The first draft ran the refusal from three places, and the reviewer set an
// illegal DINAH_HARNESS and watched comment, link and file each succeed and
// each write the illegal name into the journal's actor object. So the assertion
// here is not that some verbs refuse: it is that after every writing command
// this tool offers has been driven with that name set, no journal anywhere in
// the store carries it. That property cannot be satisfied by remembering to
// edit a verb, and a verb added later that forgets the refusal fails here.
//
// The reads are driven in the same run and asserted to succeed, because the
// refusal is for an act that writes and a mistyped variable that stopped show
// and ls would take the whole tool away from whoever has to repair it.
func TestNoWritingCommandTakesAMalformedHarnessName(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	// The fixture is built with no harness set at all, so every reference the
	// sweep below names exists before the illegal name is exported.
	for _, argv := range [][]string{
		{"add", "a card to work"},
		{"add", "a card to link to"},
		{"add", "a card to archive"},
		{"move", "fx-1", "plain"},
		{"move", "fx-2", "plain"},
		{"file", "fx-1", "decision", "something to settle"},
		{"comment", "fx-1", "a comment written before the sweep"},
		{"workstream", "new", "A workstream"},
	} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Fatalf("`dinah %s` exited %d building the fixture: %s", strings.Join(argv, " "), got.code, got.errw)
		}
	}

	t.Setenv("DINAH_HARNESS", illegalHarness)
	writes := [][]string{
		{"add", "a card filed under a malformed harness"},
		{"claim", "fx-1"},
		{"move", "fx-1", "test"},
		{"release", "fx-1"},
		{"block", "fx-1", "an obstacle"},
		{"unblock", "fx-1"},
		{"pull", "test"},
		{"comment", "fx-1", "a comment written under a malformed harness"},
		{"link", "fx-1", "relates_to", "fx-2"},
		{"unlink", "fx-1", "relates_to", "fx-2"},
		{"file", "fx-1", "decision", "a decision filed under a malformed harness"},
		{"resolve", "fx-1/decisions/1", "settled"},
		{"cite", "fx-1/decisions/1", "test", "somewhere"},
		{"reopen", "fx-1/decisions/1", "not settled after all"},
		{"set", "fx-1", "severity", "major"},
		{"set", "fx-1", "tier", "workhorse"},
		{"raise", "fx-1", "apex", "this needs somebody senior"},
		{"rename", "fx-1", "A renamed card"},
		{"archive", "fx-3"},
		{"restore", "fx-3", "--archived"},
		{"delete", "fx-3", "--yes"},
		{"column", "new", "Another station"},
		{"workstream", "new", "Another workstream"},
		{"join", "fx-1", "a-workstream"},
		{"leave", "fx-1", "a-workstream"},
		{"check", "--witness"},
	}
	refused, admitted := 0, []string{}
	for _, argv := range writes {
		got := runCLI(t, root, argv...)
		// The name is read only where one was printed. A command that
		// succeeded prints nothing on the error stream, and reading a refusal
		// name out of an empty stream is how this guard panicked instead of
		// reporting the first time it was armed.
		name := ""
		if strings.TrimSpace(got.errw) != "" {
			name = refusalNameOf(got.errw)
		}
		if got.code == 2 && name == contract.MalformedHarness {
			refused++
			continue
		}
		admitted = append(admitted, strings.Join(argv, " ")+" -> exit "+strconv.Itoa(got.code)+" "+name)
	}
	if len(admitted) > 0 {
		t.Errorf("%d of %d writing commands did not refuse a malformed harness name:\n  %s",
			len(admitted), len(writes), strings.Join(admitted, "\n  "))
	}
	if refused < 20 {
		t.Errorf("the sweep drove %d writing commands to the refusal, and the surface carries more than twenty", refused)
	}

	// The property the verbs above are only a way of reaching: nothing in the
	// store carries the name, whatever any one command did.
	journals, lines := 0, 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Base(path) != bench.JournalName {
			return err
		}
		journals++
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines++
			if strings.Contains(line, illegalHarness) {
				t.Errorf("%s carries the malformed harness name: %s", path, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the store: %v", err)
	}
	if journals == 0 || lines == 0 {
		t.Fatalf("the walk read %d journals and %d lines, so it is asserting nothing", journals, lines)
	}

	// The reads all answer under the same name.
	for _, argv := range [][]string{{"show", "fx-1"}, {"list", "cards"}, {"whoami"}, {"list", "fx-1/journal"}, {"check"}, {"list", "columns"}, {"next"}} {
		if got := runCLI(t, root, argv...); got.code != 0 && got.code != 5 {
			t.Errorf("`dinah %s` exited %d under a malformed harness name: %s", strings.Join(argv, " "), got.code, got.errw)
		}
	}
}

// TestACommandRefusesAnyFlagItDoesNotDeclare is the test for the rule
// undeclaredFlagOn states, rather than for the one flag dinah-496 retired.
//
// Agent Code Review's first round asked for it by name: the refusal itself is
// right, and a behaviour change with no test of its own scope is a behaviour
// change nobody can read. The rule fires widely, and each case below was
// accepted and ignored before this card: show carries no expires, ls carries no
// override, log carries no kind, comment carries no at. Each is a flag another
// command declares, which is why the parser admitted it at all.
//
// The second half is the other direction, and it is what stops the rule being
// a refusal every flag satisfies: the same flag on the command that does
// declare it is accepted.
func TestACommandRefusesAnyFlagItDoesNotDeclare(t *testing.T) {
	root := newBenchFromDefinition(t, tierDefinition)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "fx-1", "plain"); got.code != 0 {
		t.Fatalf("move: %d %s", got.code, got.errw)
	}

	refused := []struct {
		argv []string
		flag string
	}{
		{argv: []string{"show", "fx-1", "--expires", "1h"}, flag: "--expires"},
		{argv: []string{"list", "cards", "--override"}, flag: "--override"},
		{argv: []string{"list", "fx-1/journal", "--kind", "external"}, flag: "--kind"},
		{argv: []string{"comment", "fx-1", "a comment", "--at", "plain"}, flag: "--at"},
	}
	for _, want := range refused {
		got := runCLI(t, root, want.argv...)
		if got.code != 2 {
			t.Errorf("`dinah %s` exited %d, wanted the refused code 2: %s", strings.Join(want.argv, " "), got.code, got.errw)
			continue
		}
		if name := refusalNameOf(got.errw); name != contract.Usage {
			t.Errorf("`dinah %s` refused under %s, wanted %s", strings.Join(want.argv, " "), name, contract.Usage)
		}
		if !strings.Contains(got.errw, want.flag) {
			t.Errorf("`dinah %s` does not name %s, so a reader cannot see what to drop:\n%s",
				strings.Join(want.argv, " "), want.flag, got.errw)
		}
	}
	if len(refused) != 4 {
		t.Fatalf("the sweep drove %d commands, wanted the four it names", len(refused))
	}

	// The same four flags on the four commands that do declare them.
	for _, argv := range [][]string{
		{"claim", "fx-1", "--expires", "1h"},
		{"release", "fx-1"},
		{"block", "fx-1", "an obstacle", "--kind", "external"},
		{"unblock", "fx-1"},
		{"set", "fx-1", "tier", "workhorse", "--at", "plain"},
	} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Errorf("`dinah %s` exited %d, so the rule refuses a flag the command declares: %s",
				strings.Join(argv, " "), got.code, got.errw)
		}
	}
}
