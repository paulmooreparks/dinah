package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestApproveEverything checks that the level asks nothing and allows every
// documented tool.
func TestApproveEverything(t *testing.T) {
	if len(allRules.Ask) != 0 {
		t.Errorf("approve everything asks about %v", allRules.Ask)
	}
	allow := strings.Join(allRules.Allow, "|")
	for _, want := range []string{"read", "edit", "grep", "glob", "exec", "Read(**)", "Write(**)", "mcp__*"} {
		if !strings.Contains("|"+allow+"|", "|"+want+"|") {
			t.Errorf("allow lacks %q: %v", want, allRules.Allow)
		}
	}
}

// TestDinahLevelOnlyLooks checks that Dinah's own level allows reading and
// looking but none of the bare tools that change things, so those are put
// to the person.
func TestDinahLevelOnlyLooks(t *testing.T) {
	allow := "|" + strings.Join(dinahRules.Allow, "|") + "|"
	for _, want := range []string{"read", "grep", "glob", "Exec(ls)", "Exec(git status)"} {
		if !strings.Contains(allow, "|"+want+"|") {
			t.Errorf("allow lacks %q: %v", want, dinahRules.Allow)
		}
	}
	for _, refused := range []string{"edit", "exec", "Write(**)", "mcp__*"} {
		if strings.Contains(allow, "|"+refused+"|") {
			t.Errorf("allow has %q: %v", refused, dinahRules.Allow)
		}
	}
}

// TestFlowsFolderAllowed checks that Dinah's own levels let the
// agent write the person's own flows, which are outside every workspace.
func TestFlowsFolderAllowed(t *testing.T) {
	for _, level := range []string{ApprovalsAll, ApprovalsDinah} {
		path, err := devinConfig(level)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			Permissions permissions `json:"permissions"`
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range cfg.Permissions.Allow {
			found = found || r == flowsWriteRule()
		}
		if !found {
			t.Errorf("%s: no %s in %v", level, flowsWriteRule(), cfg.Permissions.Allow)
		}
	}
}

// TestMergeSavedKeepsAnswers checks that the answers Devin saved into the
// Dinah's configuration file survive the file being written again, and
// that a saved allow cannot lift one of the level's asks or denies.
func TestMergeSavedKeepsAnswers(t *testing.T) {
	base := permissions{Allow: []string{"read"}, Ask: []string{"Exec(git push)"}, Deny: []string{"Exec(git config)"}}
	saved := permissions{
		Allow: []string{"read", "Exec(Write-Output)", "Exec(git config)", "Exec(git push)"},
		Deny:  []string{"Exec(rm)"},
	}
	got := mergeSaved(base, saved)
	if strings.Join(got.Allow, "|") != "read|Exec(Write-Output)" {
		t.Errorf("allow = %v", got.Allow)
	}
	if strings.Join(got.Ask, "|") != "Exec(git push)" {
		t.Errorf("ask = %v", got.Ask)
	}
	if strings.Join(got.Deny, "|") != "Exec(git config)|Exec(rm)" {
		t.Errorf("deny = %v", got.Deny)
	}
}
