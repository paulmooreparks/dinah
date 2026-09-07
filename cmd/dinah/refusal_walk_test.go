package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// corruptedAmbiguousTree returns ambiguousTree with every candidate's own
// workbench.md overwritten by a document this tool does not recognise, which
// is what makes the walk that gathers the candidates refuse rather than
// answer.
//
// Every candidate has to be corrupted. soleBench (internal/bench/bench.go)
// collects the recognised candidates first and refuses over the damaged ones
// only when none was recognised, so a single healthy survivor resolves the
// base quietly and proves nothing. Replacing the .dinah container itself with
// a plain file does not reach this branch at all, because ListWorkbenchIDs
// (internal/bench/storage.go) discards the resulting os.ReadDir error and
// reports an empty container rather than a failed one.
func corruptedAmbiguousTree(t *testing.T) string {
	t.Helper()
	tree, rooms := ambiguousTree(t)
	for _, room := range rooms {
		anchor := filepath.Join(room, bench.WorkbenchAnchor)
		if err := os.WriteFile(anchor, []byte("# somebody else's document\n"), 0o644); err != nil {
			t.Fatalf("overwrite %s: %v", anchor, err)
		}
	}
	return tree
}

// reportedRefusal runs one refusal through reportError from a session standing
// in tree and returns what the named machine form wrote to stdout.
//
// The boundary the walk stops its climb at is the tree's own parent rather
// than the tree, because benchIn skips a directory's .dinah once the climb has
// reached the native home, and the tree's .dinah is the whole fixture.
func reportedRefusal(t *testing.T, tree string, format outputFormat, refusal *contract.Refusal) string {
	t.Helper()
	out := &bytes.Buffer{}
	s := &session{
		out:        out,
		errw:       &bytes.Buffer{},
		r:          msg.For(msg.Base),
		width:      100,
		format:     format,
		home:       filepath.Join(tree, "home"),
		nativeHome: filepath.Dir(tree),
		cwd:        tree,
	}
	s.reportError(refusal)
	return out.String()
}

// decodeReport reads one machine refusal back as the generic map a script
// parsing --json holds, which is what a test asserting over an absent key
// needs and what a decode into refusalReport cannot tell apart from a zero.
func decodeReport(t *testing.T, payload string) map[string]any {
	t.Helper()
	report := map[string]any{}
	if err := json.Unmarshal([]byte(payload), &report); err != nil {
		t.Fatalf("--json wrote nothing a caller can parse: %q (%v)", payload, err)
	}
	return report
}

// TestAFailedCandidateWalkIsReportedRatherThanDroppingTheKey asserts dinah-432
// AC-2: the walk that gathers the candidates for a dinah.ambiguous-workbench
// refusal no longer has its error discarded, so a walk that failed and a walk
// that found nothing stop arriving in the same shape.
func TestAFailedCandidateWalkIsReportedRatherThanDroppingTheKey(t *testing.T) {
	tree := corruptedAmbiguousTree(t)

	// The fixture has to reach the failure this field reports rather than
	// some other error by another path, so the walk is run directly first.
	rows, err := bench.Reachable(tree, "", filepath.Join(tree, "home"), filepath.Dir(tree))
	walked, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("the fixture should make the walk refuse, got rows %v and err %v", rows, err)
	}
	if walked.Name != contract.DamagedBench {
		t.Fatalf("the fixture should reach %s, got %s", contract.DamagedBench, walked.Name)
	}

	report := decodeReport(t, reportedRefusal(t, tree, formatJSON, contract.Refuse(contract.AmbiguousWorkbench, tree)))
	carried, ok := report["workbenches_refusal"].(map[string]any)
	if !ok {
		t.Fatalf("the reply should carry workbenches_refusal as an object, got %v", report["workbenches_refusal"])
	}
	if name, _ := carried["name"].(string); name != contract.DamagedBench {
		t.Errorf("workbenches_refusal.name: wanted %s, got %v", contract.DamagedBench, carried["name"])
	}
	if _, present := report["workbenches"]; present {
		t.Errorf("a failed walk should leave no workbenches key, got %v", report["workbenches"])
	}
}

// TestASucceedingCandidateWalkStillServesTheCandidates asserts dinah-432 AC-3:
// the same fixture without the injected fault answers exactly as it did before
// this card, so a change populating both fields at once, or neither, fails
// here.
func TestASucceedingCandidateWalkStillServesTheCandidates(t *testing.T) {
	tree, _ := ambiguousTree(t)
	machine := runCLI(t, tree, "--json", "status")
	report := decodeReport(t, machine.out)
	// The walk climbs to the volume root, so on a machine whose own home
	// carries a .dinah holding one workbench the search resolves before it
	// reaches the fixture's ambiguity. The case is skipped rather than
	// asserted against whatever the machine happens to hold.
	if report["refusal"] != contract.AmbiguousWorkbench {
		t.Skipf("a directory above the temporary tree answered the search with %v", report["refusal"])
	}
	rows, ok := report["workbenches"].([]any)
	if !ok || len(rows) < 2 {
		t.Fatalf("the reply should carry the candidate rows, got %v", report["workbenches"])
	}
	if _, present := report["workbenches_refusal"]; present {
		t.Errorf("a walk that answered should leave no workbenches_refusal key, got %v", report["workbenches_refusal"])
	}
}

// TestOnlyTheAmbiguousRefusalCarriesACandidateWalk asserts dinah-432 AC-4 over
// the whole named-refusal set rather than over the one case read while the
// spec was drafted. contract.Introduced carries every name Dinah mints beyond
// the profile's own, which is 58 names on this build and so 57 subtests
// here, and none of them but the ambiguous one runs a walk at all.
func TestOnlyTheAmbiguousRefusalCarriesACandidateWalk(t *testing.T) {
	tree := emptyTree(t)
	for _, name := range contract.Introduced {
		if name == contract.AmbiguousWorkbench {
			continue
		}
		t.Run(strings.ReplaceAll(name, "/", "-"), func(t *testing.T) {
			report := decodeReport(t, reportedRefusal(t, tree, formatJSON, contract.Refuse(name, tree)))
			for _, key := range []string{"workbenches", "workbenches_refusal"} {
				if _, present := report[key]; present {
					t.Errorf("%s should carry no %s key, got %v", name, key, report[key])
				}
			}
		})
	}
}
