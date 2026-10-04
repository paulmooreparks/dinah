package mcp

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// namesOf sorts and returns the tool names in a slice of tool, for comparing
// a served surface by name rather than by count alone.
func namesOf(list []tool) []string {
	names := make([]string, 0, len(list))
	for _, t := range list {
		names = append(names, t.name)
	}
	sort.Strings(names)
	return names
}

// TestProfileMembership pins dinah-544's three profiles: the station profile
// serves exactly the station members, the operator profile serves those and
// the operator-only members, and ProfileAll and the empty string (what every
// existing call site now passes) both answer the whole registry, unfiltered.
// The members are read off the declarations, so a tool added to a profile is
// reviewed where it is declared rather than again in a count here.
func TestProfileMembership(t *testing.T) {
	station := namesOf(toolsFor(ProfileStation))
	wantStation := append([]string{}, stationMembers...)
	sort.Strings(wantStation)
	if len(station) == 0 {
		t.Fatal("ProfileStation serves no tool")
	}
	if got := strings.Join(station, " "); got != strings.Join(wantStation, " ") {
		t.Errorf("ProfileStation is\n  %s\nwanted\n  %s", got, strings.Join(wantStation, " "))
	}

	operator := namesOf(toolsFor(ProfileOperator))
	wantOperator := append(append([]string{}, wantStation...), operatorOnlyMembers...)
	sort.Strings(wantOperator)
	if got := strings.Join(operator, " "); got != strings.Join(wantOperator, " ") {
		t.Errorf("ProfileOperator is\n  %s\nwanted\n  %s", got, strings.Join(wantOperator, " "))
	}
	if len(operator) <= len(station) {
		t.Errorf("ProfileOperator serves %d tools and ProfileStation %d; operator must be a strict superset", len(operator), len(station))
	}

	all := namesOf(toolsFor(ProfileAll))
	bare := namesOf(toolsFor(""))
	registry := namesOf(tools)
	if strings.Join(all, " ") != strings.Join(registry, " ") {
		t.Errorf("ProfileAll is\n  %s\nand the unfiltered registry is\n  %s", strings.Join(all, " "), strings.Join(registry, " "))
	}
	if strings.Join(bare, " ") != strings.Join(registry, " ") {
		t.Errorf("the empty profile is\n  %s\nand the unfiltered registry is\n  %s", strings.Join(bare, " "), strings.Join(registry, " "))
	}

	// station is a strict subset of operator, which is a strict subset of
	// all, as the spec's own accounting states.
	stationSet := map[string]bool{}
	for _, name := range station {
		stationSet[name] = true
	}
	for _, name := range station {
		found := false
		for _, opName := range operator {
			if opName == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is in station and not in operator", name)
		}
	}
	for _, name := range operator {
		found := false
		for _, allName := range all {
			if allName == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is in operator and not in all", name)
		}
	}

	// dinah-582 moved the workstream tool across, so that an agent which may
	// write a workstream's fields may also bring one into existence. The two
	// halves are asserted together: a name the station profile carries has to
	// be absent from the operator-only list, or the comparisons above would still
	// pass while the tool arrived from the wrong half.
	if !stationSet["workstream"] {
		t.Error("ProfileStation does not carry workstream, which dinah-582 moved into it")
	}
	for _, name := range operatorOnlyMembers {
		if name == "workstream" {
			t.Error("operatorOnlyMembers still carries workstream, which dinah-582 moved out of it")
		}
	}
}

// TestToolsListRespectsTheServedProfile asserts that a connection served
// under ProfileStation sees exactly the station tools over tools/list, and
// one served under ProfileOperator sees exactly the operator tools.
func TestToolsListRespectsTheServedProfile(t *testing.T) {
	library := newLibrary(t)

	station := servedNames(t, askUnderProfile(t, library.Bench.Root, ProfileStation, library, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if len(station) == 0 {
		t.Fatal("tools/list under station carried no tool")
	}
	wantStation := namesOf(toolsFor(ProfileStation))
	if got := strings.Join(station, " "); got != strings.Join(wantStation, " ") {
		t.Errorf("tools/list under station is\n  %s\nwanted\n  %s", got, strings.Join(wantStation, " "))
	}

	operator := servedNames(t, askUnderProfile(t, library.Bench.Root, ProfileOperator, library, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	wantOperator := namesOf(toolsFor(ProfileOperator))
	if got := strings.Join(operator, " "); got != strings.Join(wantOperator, " ") {
		t.Errorf("tools/list under operator is\n  %s\nwanted\n  %s", got, strings.Join(wantOperator, " "))
	}
}

// servedNames reads the sorted tool names off a tools/list response.
func servedNames(t *testing.T, answer *response) []string {
	t.Helper()
	if answer.Error != nil {
		t.Fatalf("tools/list failed at the transport: %+v", answer.Error)
	}
	encoded, err := json.Marshal(answer.Result)
	if err != nil {
		t.Fatalf("marshal the tools/list result: %v", err)
	}
	var listed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(encoded, &listed); err != nil {
		t.Fatalf("decode tools/list: %v (%s)", err, encoded)
	}
	names := make([]string, 0, len(listed.Tools))
	for _, entry := range listed.Tools {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return names
}

// TestAnOutOfProfileCallIsRefusedLikeAnUnknownName asserts decision D3: a
// call naming a tool that exists but stands outside the connection's served
// profile is refused exactly the way a call naming a tool this head serves
// no tool for at all is refused, contract.UnknownVerb, ahead of any argument
// check. The same call against ProfileOperator, which does serve archive, is
// pinned beside it as the accepting case.
func TestAnOutOfProfileCallIsRefusedLikeAnUnknownName(t *testing.T) {
	library := newLibrary(t)

	refused := askUnderProfile(t, library.Bench.Root, ProfileStation, library, callLine(t, 1, "archive", map[string]any{"actor": "alka", "ref": "fx-2"}))
	if refused.Error == nil {
		t.Fatalf("archive was accepted under station, which does not serve it: %+v", refused)
	}
	if !strings.Contains(refused.Error.Message, contract.UnknownVerb) {
		t.Errorf("the message %q does not refuse the tool name with %s", refused.Error.Message, contract.UnknownVerb)
	}

	accepted := askUnderProfile(t, library.Bench.Root, ProfileOperator, library, callLine(t, 2, "archive", map[string]any{"actor": "alka", "ref": "fx-2"}))
	if accepted.Error != nil {
		t.Fatalf("archive under operator, which does serve it, was refused: %+v", accepted.Error)
	}
	decoded := payload(t, accepted)
	if decoded["outcome"] != contract.OutcomeOK {
		t.Errorf("archive under operator did not succeed: %v", decoded)
	}
}
