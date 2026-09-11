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
// a plain file reaches a different branch and a different refusal, which
// unreadableContainerTree below plants and
// TestAContainerTheWalkCannotListIsReportedRatherThanReadAsEmpty asserts.
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

// refusingSession builds a session standing in tree, alongside the buffers its
// two streams write to.
//
// The boundary the walk stops its climb at is the tree's own parent rather
// than the tree, because benchIn skips a directory's .dinah once the climb has
// reached the native home, and the tree's .dinah is the whole fixture.
func refusingSession(tree string, format outputFormat) (*session, *bytes.Buffer, *bytes.Buffer) {
	out, errw := &bytes.Buffer{}, &bytes.Buffer{}
	return &session{
		out:        out,
		errw:       errw,
		r:          msg.For(msg.Base),
		width:      100,
		format:     format,
		home:       filepath.Join(tree, "home"),
		nativeHome: filepath.Dir(tree),
		cwd:        tree,
	}, out, errw
}

// reportedRefusal runs one refusal through reportError from a session standing
// in tree and returns what the named machine form wrote to stdout.
func reportedRefusal(t *testing.T, tree string, format outputFormat, refusal *contract.Refusal) string {
	t.Helper()
	s, out, _ := refusingSession(tree, format)
	s.reportError(refusal)
	return out.String()
}

// composedRefusal runs one refusal through reportError from a session standing
// in tree and returns what the human form wrote to stderr, which is the form a
// person reading dinah status sees.
func composedRefusal(t *testing.T, tree string, refusal *contract.Refusal) string {
	t.Helper()
	s, _, errw := refusingSession(tree, formatHuman)
	s.reportError(refusal)
	return errw.String()
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

// unreadableContainerTree returns a tree whose own .dinah container has been
// replaced by a plain file, which is the corruption the walk used to read as a
// container holding nothing.
//
// The fixture proves its own fault before it is used. os.ReadDir has to refuse
// the planted path, because a plant that left a readable directory behind
// would send every assertion below through the ordinary empty-container path
// and pass for the wrong reason.
func unreadableContainerTree(t *testing.T) string {
	t.Helper()
	tree := emptyTree(t)
	container := filepath.Join(tree, bench.UserBaseName)
	if err := os.WriteFile(container, []byte("a file sitting where the container belongs"), 0o644); err != nil {
		t.Fatalf("write %s: %v", container, err)
	}
	if _, err := os.ReadDir(container); err == nil {
		t.Fatalf("%s still reads as a directory, so this fixture plants no fault", container)
	}
	return tree
}

// TestAContainerTheWalkCannotListIsReportedRatherThanReadAsEmpty asserts
// dinah-433 AC-5. A .dinah container replaced by a plain file now reaches
// dinah.unreadable-container and travels out through the same
// workbenches_refusal field dinah-432 added, which is the scenario dinah-432's
// own description opened with and could not reach.
func TestAContainerTheWalkCannotListIsReportedRatherThanReadAsEmpty(t *testing.T) {
	tree := unreadableContainerTree(t)

	// The fixture has to reach this refusal through the walk itself rather
	// than through some other error on the way, so the walk is run directly
	// first.
	rows, err := bench.Reachable(tree, "", filepath.Join(tree, "home"), filepath.Dir(tree))
	walked, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("the fixture should make the walk refuse, got rows %v and err %v", rows, err)
	}
	if walked.Name != contract.UnreadableContainer {
		t.Fatalf("the fixture should reach %s, got %s", contract.UnreadableContainer, walked.Name)
	}

	report := decodeReport(t, reportedRefusal(t, tree, formatJSON, contract.Refuse(contract.AmbiguousWorkbench, tree)))
	carried, ok := report["workbenches_refusal"].(map[string]any)
	if !ok {
		t.Fatalf("the reply should carry workbenches_refusal as an object, got %v", report["workbenches_refusal"])
	}
	if name, _ := carried["name"].(string); name != contract.UnreadableContainer {
		t.Errorf("workbenches_refusal.name: wanted %s, got %v", contract.UnreadableContainer, carried["name"])
	}
	if _, present := report["workbenches"]; present {
		t.Errorf("a failed walk should leave no workbenches key, got %v", report["workbenches"])
	}
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
	tree, rooms := ambiguousTree(t)

	// The reply is read out of a run of the binary, which climbs to the
	// volume root and can therefore be answered by a directory above the
	// fixture on somebody's machine. That arm skips rather than asserting
	// against whatever the machine happens to hold, so the fixture is put
	// through bench.Reachable directly first, where the climb is bounded by
	// arguments and the answer is the same on every machine.
	rows, err := bench.Reachable(tree, "", filepath.Join(tree, "home"), filepath.Dir(tree))
	if err != nil {
		t.Fatalf("the uncorrupted fixture should answer, got %v", err)
	}
	if len(rows) != len(rooms) {
		t.Fatalf("the walk should find %d candidates, got %v", len(rooms), rows)
	}

	machine := runCLI(t, tree, "--json", "status")
	report := decodeReport(t, machine.out)
	if report["refusal"] != contract.AmbiguousWorkbench {
		t.Skipf("a directory above the temporary tree answered the search with %v", report["refusal"])
	}
	carried, ok := report["workbenches"].([]any)
	if !ok || len(carried) < 2 {
		t.Fatalf("the reply should carry the candidate rows, got %v", report["workbenches"])
	}
	if _, present := report["workbenches_refusal"]; present {
		t.Errorf("a walk that answered should leave no workbenches_refusal key, got %v", report["workbenches_refusal"])
	}
}

// TestOnlyTheAmbiguousRefusalCarriesACandidateWalk asserts dinah-432 AC-4 over
// the whole named-refusal set rather than over the one case read while the
// spec was drafted. contract.Introduced carries every name Dinah mints beyond
// the profile's own, which is 71 names on this build and so 70 subtests
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

// TestAFailedCandidateWalkNamesItselfInTheHumanListing asserts that the block
// drawing the ambiguous refusal's candidates for a person keeps the same error
// the machine form keeps. Before this, refusalBlocks["workbenches"] discarded
// it and formatCandidateRows drew a table of no rows, so the reader was told
// the directory holds several workbenches and shown none of them, with nothing
// saying the list could not be produced.
func TestAFailedCandidateWalkNamesItselfInTheHumanListing(t *testing.T) {
	tree := corruptedAmbiguousTree(t)

	// The fixture has to reach the failure this listing reports rather than
	// some other error by another path, so the walk is run directly first.
	rows, err := bench.Reachable(tree, "", filepath.Join(tree, "home"), filepath.Dir(tree))
	walked, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("the fixture should make the walk refuse, got rows %v and err %v", rows, err)
	}
	if walked.Name != contract.DamagedBench {
		t.Fatalf("the fixture should reach %s, got %s", contract.DamagedBench, walked.Name)
	}

	text := composedRefusal(t, tree, contract.Refuse(contract.AmbiguousWorkbench, tree))
	if !strings.HasPrefix(text, contract.AmbiguousWorkbench+" ") {
		t.Errorf("the refusal name should still lead stderr, got %q", text)
	}
	s, _, _ := refusingSession(tree, formatHuman)
	cell := s.refusedCell(contract.DamagedBench)
	if !strings.Contains(text, cell) {
		t.Errorf("the listing should carry %q in place of the rows it could not produce, got %q", cell, text)
	}
}

// TestASucceedingCandidateWalkStillDrawsTheCandidatesForAPerson holds the
// other arm of the same block: a walk that answered draws its rows and names
// no refusal, so a change reporting the failure on every walk fails here.
func TestASucceedingCandidateWalkStillDrawsTheCandidatesForAPerson(t *testing.T) {
	tree, rooms := ambiguousTree(t)

	text := composedRefusal(t, tree, contract.Refuse(contract.AmbiguousWorkbench, tree))
	for _, room := range rooms {
		if !strings.Contains(text, room) {
			t.Errorf("the listing should carry the candidate %q, got %q", room, text)
		}
	}
	s, _, _ := refusingSession(tree, formatHuman)
	for _, name := range []string{contract.DamagedBench, contract.UnknownRoot} {
		if cell := s.refusedCell(name); strings.Contains(text, cell) {
			t.Errorf("a walk that answered should name no refusal, got %q", text)
		}
	}
}
