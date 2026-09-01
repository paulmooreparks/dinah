package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The two revisions this demonstration names. The first is what a build
// claimed immediately before dinah-346 changed what `dinah check` returns to
// whoever invoked it. The second is the revision that published CORE-OUT-7,
// the statement holding a tool to giving `refused` a number no other outcome
// uses, which is the contract dinah-346 established and nothing published
// until dinah-358.
//
// They are spelled out rather than composed from bench.ProfileVersion. A
// value composed from the constant it is compared against asserts nothing, and
// the whole point here is that the numbers a client reads are numbers somebody
// published on purpose.
const (
	profileBeforeTheReadExitConvention = "dinah-core/0.7"
	profilePublishingTheReadExitRule   = "dinah-core/0.9"
)

// speaksTheReadExitConvention is a client-side gate, written the way a client
// with no knowledge of any particular Dinah release writes one. It reads the
// conformance claim and nothing else: not the tool's own release number, which
// says nothing about conformance, and not the shape of any answer the tool
// has given, which is the guessing dinah-346 and dinah-353 both refuse.
func speaksTheReadExitConvention(profile string) bool {
	floor, ok := splitClaim(profilePublishingTheReadExitRule)
	if !ok {
		return false
	}
	claimed, ok := splitClaim(profile)
	if !ok {
		return false
	}
	if claimed[0] != floor[0] {
		return claimed[0] > floor[0]
	}
	return claimed[1] >= floor[1]
}

// splitClaim reads a conformance claim into its major and minor numbers,
// answering false for anything that will not split. The name is compared as
// well as the numbers, because a claim naming some other profile says nothing
// about this one.
func splitClaim(profile string) ([2]int, bool) {
	name, version, found := strings.Cut(profile, "/")
	if !found || name != bench.ProfileName {
		return [2]int{}, false
	}
	major, minor, found := strings.Cut(version, ".")
	if !found {
		return [2]int{}, false
	}
	first, err := strconv.Atoi(major)
	if err != nil {
		return [2]int{}, false
	}
	second, err := strconv.Atoi(minor)
	if err != nil {
		return [2]int{}, false
	}
	return [2]int{first, second}, true
}

// TestAClientTellsTheReadExitConventionApartFromTheVersionAlone is dinah-358
// AC-5, the concrete case the card exists for. A client that has to know
// whether the binary in front of it exits 5 for a workbench carrying findings,
// rather than the 2 that used to mean both that and a refusal, asks one
// question and gets an answer: the conformance claim. It asks before it runs
// anything, so no response's body shape is available to it when it decides,
// which is the rule dinah-346 and dinah-353 both hold.
//
// The two revisions the gate tells apart are named above. The demonstration
// runs in the order a real client runs in: read the claim, decide, then
// exercise the behaviour the decision predicted.
//
// This test is new rather than an extension of an existing one because
// nothing in the suite joined the two halves. cmd/dinah's compat tests drive
// the profile window, internal/contract's tests drive the exit tables, and
// neither asks whether a client can read the first to predict the second.
func TestAClientTellsTheReadExitConventionApartFromTheVersionAlone(t *testing.T) {
	if speaksTheReadExitConvention(profileBeforeTheReadExitConvention) {
		t.Errorf("the gate reads %s as speaking the convention, and a build claiming it published nothing about the exit status a reading returns", profileBeforeTheReadExitConvention)
	}
	if !speaksTheReadExitConvention(profilePublishingTheReadExitRule) {
		t.Errorf("the gate reads %s as not speaking the convention, and that is the revision CORE-OUT-7 was published at", profilePublishingTheReadExitRule)
	}

	root := newBench(t)

	// Step one, and the only step the decision rests on. The client asks what
	// the binary is and reads the conformance claim out of the answer.
	asked := runCLI(t, root, "--json", "version")
	if asked.code != 0 {
		t.Fatalf("version --json: %d %s", asked.code, asked.errw)
	}
	var reported struct {
		Tool    string `json:"tool"`
		Profile string `json:"profile"`
		Format  int    `json:"format"`
	}
	if err := json.Unmarshal([]byte(asked.out), &reported); err != nil {
		t.Fatalf("read the version report: %v\n%s", err, asked.out)
	}
	if !speaksTheReadExitConvention(reported.Profile) {
		t.Fatalf("this build reports %s, and a client gating on the convention refuses it", reported.Profile)
	}

	// Step two. The workbench acquires a defect, and the behaviour the claim
	// promised is what the tool performs.
	if got := runCLI(t, root, "add", "a card somebody hand-edited"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	handWrite(t, root, "fx-1", "severity: urgent")

	checked := runCLI(t, root, "--json", "check")
	if checked.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Fatalf("check over a workbench carrying findings exited %d, wanted %d:\n%s", checked.code, contract.ExitCodeForRead(contract.ReadFindings), checked.out)
	}
	if checked.code == contract.ExitCode(contract.OutcomeRefused) {
		t.Errorf("a reading that found something exits %d, which is also what a refusal exits, and CORE-OUT-7 forbids the collision", checked.code)
	}
	var read struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal([]byte(checked.out), &read); err != nil {
		t.Fatalf("read the check report: %v\n%s", err, checked.out)
	}
	if read.Outcome != contract.ReadFindings {
		t.Errorf("check reports outcome %q over a workbench carrying a defect, wanted %q", read.Outcome, contract.ReadFindings)
	}
}
