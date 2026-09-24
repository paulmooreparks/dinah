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

// TestProfileMembershipByNameAndCount pins dinah-544's three profiles: the
// thirty station tools (dinah-573 adds prime, dinah-582 adds workstream,
// dinah-600 adds view), the forty-four operator tools (station's thirty plus
// fourteen), and that ProfileAll and the empty string (what every existing
// call site now passes) both answer the whole registry, unfiltered, at fifty.
func TestProfileMembershipByNameAndCount(t *testing.T) {
	station := namesOf(toolsFor(ProfileStation))
	if len(station) != 30 {
		t.Errorf("ProfileStation carries %d tools, wanted 30: %v", len(station), station)
	}
	wantStation := []string{
		"add_card", "attach", "block", "cite_item", "claim", "comment",
		"changes", "file_item", "get_field", "instructions", "join_workstream",
		"leave_workstream", "link_card", "list", "move", "next_card",
		"prime", "pull", "query", "raise", "release", "search_cards", "set_field",
		"settle", "show", "tree", "unlink_card", "view", "whoami", "workstream",
	}
	sort.Strings(wantStation)
	if got := strings.Join(station, " "); got != strings.Join(wantStation, " ") {
		t.Errorf("ProfileStation is\n  %s\nwanted\n  %s", got, strings.Join(wantStation, " "))
	}

	operator := namesOf(toolsFor(ProfileOperator))
	if len(operator) != 44 {
		t.Errorf("ProfileOperator carries %d tools, wanted 44: %v", len(operator), operator)
	}
	wantOperator := append(append([]string{}, wantStation...), operatorOnlyMembers...)
	sort.Strings(wantOperator)
	if got := strings.Join(operator, " "); got != strings.Join(wantOperator, " ") {
		t.Errorf("ProfileOperator is\n  %s\nwanted\n  %s", got, strings.Join(wantOperator, " "))
	}

	all := namesOf(toolsFor(ProfileAll))
	bare := namesOf(toolsFor(""))
	registry := namesOf(tools)
	if len(all) != 50 {
		t.Errorf("ProfileAll carries %d tools, wanted 50: %v", len(all), all)
	}
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
	// be absent from the operator-only list, or the counts above would still
	// balance while the tool arrived from the wrong half.
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
// under ProfileStation sees exactly the thirty station tools over
// tools/list, and one served under ProfileOperator sees exactly the
// forty-four.
func TestToolsListRespectsTheServedProfile(t *testing.T) {
	library := newLibrary(t)

	station := servedNames(t, askUnderProfile(t, library.Bench.Root, ProfileStation, library, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if len(station) != 30 {
		t.Errorf("tools/list under station carried %d tools, wanted 30: %v", len(station), station)
	}
	wantStation := namesOf(toolsFor(ProfileStation))
	if got := strings.Join(station, " "); got != strings.Join(wantStation, " ") {
		t.Errorf("tools/list under station is\n  %s\nwanted\n  %s", got, strings.Join(wantStation, " "))
	}

	operator := servedNames(t, askUnderProfile(t, library.Bench.Root, ProfileOperator, library, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if len(operator) != 44 {
		t.Errorf("tools/list under operator carried %d tools, wanted 44: %v", len(operator), operator)
	}
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
