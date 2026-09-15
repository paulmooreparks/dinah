package main

import (
	"encoding/json"
	"strings"
	"testing"

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
		for _, argv := range [][]string{{"show", "fx-1"}, {"ls"}, {"whoami"}} {
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
