package providers

import "testing"

// The facts the session code reads are what the providers do today.
func TestTheFactsTheSessionCodeReads(t *testing.T) {
	for id, want := range map[string]struct{ managed, own, hooks, accepted bool }{
		"devin":  {managed: false, own: false, hooks: true, accepted: false},
		"codex":  {managed: true, own: true, hooks: false, accepted: true},
		"claude": {managed: true, own: true, hooks: true, accepted: false},
		"ollama": {managed: true, own: true, hooks: false, accepted: false},
	} {
		row := For(id)
		if row.ManagedServers != want.managed || row.OwnPreferences != want.own || row.HookSteering != want.hooks || row.SteerAccepted != want.accepted {
			t.Errorf("%s: %+v", id, row)
		}
	}
	if (For("nobody") != Row{}) || Name("nobody") != "" {
		t.Error("a provider with no row has facts")
	}
}

// The documented limits and handoff budgets are in the rows: a provider with a
// source has a default no higher than its ceiling, and every provider but the
// ones whose window is unknown has a budget.
func TestFileLimitsAndHandoffBudgetsInTheRows(t *testing.T) {
	for id, row := range Registry {
		if row.FileLimit.Source == "" {
			if row.FileLimit.Default != 0 || row.FileLimit.Ceiling != 0 {
				t.Errorf("%s has a limit without a source", id)
			}
			continue
		}
		if row.FileLimit.Ceiling > 0 && row.FileLimit.Default > row.FileLimit.Ceiling {
			t.Errorf("%s: the default %d is over the ceiling %d", id, row.FileLimit.Default, row.FileLimit.Ceiling)
		}
	}
	if For("codex").HandoffChars >= 1_048_576 {
		t.Error("Codex's handoff budget is not under the app-server's request cap")
	}
	if !For("ollama").HandoffFromContext || For("claude").HandoffFromContext {
		t.Error("only Ollama's budget follows the chat's context option")
	}
}

// Where journals are kept and which provider's tool calls are merged are what
// the files on disk and the recorded transcripts already assume.
func TestJournalFoldersAndMergedToolCallsAreWhatIsOnDisk(t *testing.T) {
	for id, row := range Registry {
		wantFolder := ""
		if id == "codex" {
			wantFolder = "codex"
		}
		if row.JournalFolder != wantFolder {
			t.Errorf("%s: the journal folder is %q, not %q, and journals are already on disk there", id, row.JournalFolder, wantFolder)
		}
		if row.MergedToolCalls != (id == "claude") {
			t.Errorf("%s: merged tool calls %v", id, row.MergedToolCalls)
		}
	}
}

// The commands a session offers are the source of one provider, the one whose
// row says it has commands of its own.
func TestOwnCommandsProviderIsTheOneWhoseRowSaysSo(t *testing.T) {
	if got := OwnCommandsProvider(); got != "devin" || !For(got).OwnCommands {
		t.Fatalf("the provider with commands of its own is %q", got)
	}
}

// The command steering hook's session IDs are namespaced by the one provider
// that runs it.
func TestCommandHookNamespaceIsTheOneProviderThatRunsIt(t *testing.T) {
	if got := CommandHookNamespace(); got != "devin:" {
		t.Fatalf("the namespace is %q", got)
	}
	n := 0
	for _, row := range Registry {
		if row.HookNamespace != "" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d providers declare a hook namespace", n)
	}
}

// Every row declares the unit its judgement calls are budgeted in, and Devin's is
// ACUs where the others' is tokens.
func TestEveryRowDeclaresItsBudgetUnit(t *testing.T) {
	for id, row := range Registry {
		if row.BudgetUnit == "" {
			t.Errorf("%s declares no budget unit", id)
		}
	}
	if Registry["devin"].BudgetUnit != "acu" || Registry["claude"].BudgetUnit != "tokens" || Registry["ollama"].BudgetUnit != "tokens" {
		t.Fatal("a unit is not the one the provider counts in")
	}
}
