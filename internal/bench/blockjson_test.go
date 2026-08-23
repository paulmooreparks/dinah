package bench

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// cloneThrough runs a workbench through the interchange the way dinah init
// --from does: export it, read the definition back, and instantiate a second
// workbench from it. It answers with the new workbench's root.
func cloneThrough(t *testing.T, opened *Bench) string {
	t.Helper()
	encoded, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	definition, err := ReadDefinition(encoded)
	if err != nil {
		t.Fatalf("read the definition back: %v", err)
	}
	root := filepath.Join(t.TempDir(), "clone")
	if err := Instantiate(root, "cl", "sam", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	return root
}

// exportOf is one workbench's interchange form as text.
func exportOf(t *testing.T, root string) string {
	t.Helper()
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open %s: %v", root, err)
	}
	encoded, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return string(encoded)
}

// profileMember matches the interchange form's profile line.
var profileMember = regexp.MustCompile(`"profile": "[^"]*"`)

// stampedProfile puts one revision in place of whatever the object declares,
// because Instantiate stamps this build's revision into the anchor it writes
// rather than copying the source's claim, and the fixture these tests build on
// declares an older one. That is the one member a clone is meant to change,
// and setting it aside is what leaves the rest of the byte comparison to say
// something.
func stampedProfile(text string) string {
	return profileMember.ReplaceAllString(text, `"profile": "stamped"`)
}

// TestAHintedAxisSurvivesACloneEntryByEntry asserts dinah-196 AC-6. A severity
// declared with hints reads back on the clone with the same name, the same
// hint and the same rank for every entry, and the clone's anchor carries the
// dashed form, since a flow entry has no spelling for a hint.
func TestAHintedAxisSurvivesACloneEntryByEntry(t *testing.T) {
	block := "levels:\n  severity:\n    - trivial\n    - minor\n" +
		"    - major: A person's work is wrong or blocked.\n" +
		"    - critical: Data loss or money; drop everything.\n"
	source := benchDeclaring(t, block)
	root := cloneThrough(t, source)
	anchor := readWorkbenchAnchor(t, root)
	if !strings.Contains(anchor, "levels:\n  severity:\n    - trivial\n    - minor\n"+
		"    - major: A person's work is wrong or blocked.\n") {
		t.Errorf("the clone's anchor does not carry the dashed form:\n%s", anchor)
	}
	clone, err := Open(root)
	if err != nil {
		t.Fatalf("open the clone: %v", err)
	}
	want, got := source.Levels("severity"), clone.Levels("severity")
	if len(want) != len(got) {
		t.Fatalf("the clone declares %d severities, the source declares %d", len(got), len(want))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("entry %d: the source has %+v and the clone has %+v", i, want[i], got[i])
		}
	}
}

// TestASecondExportMatchesTheFirst asserts dinah-196 AC-7 at the level the two
// functions work at: export, instantiate, export again gives the same bytes,
// for every shape a frontmatter block can carry.
func TestASecondExportMatchesTheFirst(t *testing.T) {
	for _, declaration := range []struct {
		name  string
		block string
	}{
		{"flow levels", "levels:\n  severity: [trivial, minor]\n  priority: [later, now]\n"},
		{"hinted levels", "levels:\n  severity:\n    - minor\n    - major: fix it\n"},
		{"axes in the other order", "levels:\n  priority: [later, now]\n  severity: [minor, major]\n"},
		{"numeric levels", "levels:\n  priority: [1, 2, 3, 4]\n"},
		{"numeric levels, dashed", "levels:\n  priority:\n    - 1\n    - 2\n"},
		{"a mapping nobody reads", "groups:\n  ZEBRA: [one, two]\n  ALPHA: [three]\n"},
		{"a mapping two deep", "contacts:\n  oncall:\n    rota: [ana, bo]\n"},
		{"a plain sequence", "rituals:\n  - standup\n  - retro\n"},
		{"a scalar", "owner: ana\n"},
		{"a duplicated member name", "dup: {\"a\":1,\"a\":2}\n"},
	} {
		t.Run(declaration.name, func(t *testing.T) {
			source := benchDeclaring(t, declaration.block)
			first, err := source.Export()
			if err != nil {
				t.Fatalf("export: %v", err)
			}
			root := cloneThrough(t, source)
			second := exportOf(t, root)
			if stampedProfile(string(first)) != stampedProfile(second) {
				t.Errorf("the round trip changed the interchange form:\nfirst:\n%s\nsecond:\n%s", first, second)
			}
		})
	}
}

// TestAWorkbenchDeclaringPriorityFirstExportsSeverityFirst asserts dinah-196
// AC-13's first half at this level: the author's axis order is normalised on
// the way out, which is what keeps the two sides of the interchange from
// disagreeing about it.
func TestAWorkbenchDeclaringPriorityFirstExportsSeverityFirst(t *testing.T) {
	source := benchDeclaring(t, "levels:\n  priority: [later, now]\n  severity: [minor, major]\n")
	encoded, err := source.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	object := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	members, read := jsonMembers(object[LevelsKey])
	if !read {
		t.Fatalf("the levels member is not an object: %s", object[LevelsKey])
	}
	if len(members) != 2 || members[0].name != "severity" || members[1].name != "priority" {
		t.Errorf("export published the axes as %v, wanted severity then priority", members)
	}
	anchor := readWorkbenchAnchor(t, cloneThrough(t, source))
	if !strings.Contains(anchor, "levels:\n  severity: [minor, major]\n  priority: [later, now]\n") {
		t.Errorf("the clone's anchor does not list severity first:\n%s", anchor)
	}
}

// TestABlockChildTheReaderCannotReadLeavesItsSiblingsAlone asserts dinah-196
// AC-10's first run. A tab is not indentation the format promises, so the line
// carrying one is skipped and the two readable members still export.
func TestABlockChildTheReaderCannotReadLeavesItsSiblingsAlone(t *testing.T) {
	source := benchDeclaring(t, "team:\n  ana: one\n  bo: two\n\tcy: three\n")
	encoded, err := source.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	object := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !sameJSON(object["team"], json.RawMessage(`{"ana":"one","bo":"two"}`)) {
		t.Errorf("the block exported as %s, wanted the two readable members alone", object["team"])
	}
}

// TestTheRendererRefusesWhatTheReaderWouldMisread walks the stability
// invariant the two functions owe each other: for every value renderBlock
// agrees to write, reading the lines back gives that same value, and every
// value it refuses is one the reader would have read as something else.
func TestTheRendererRefusesWhatTheReaderWouldMisread(t *testing.T) {
	for _, value := range []string{
		`"plain"`,
		`""`,
		`"a string with spaces"`,
		`"[a, b]"`,
		`"{\"n\": 1}"`,
		`"12"`,
		`"trailing "`,
		`["a","b"]`,
		`["1","2"]`,
		`[1,2,3]`,
		`[1.0,2.50]`,
		`[true,false,null]`,
		`[{"major":"fix it"},"minor"]`,
		`[{"major":"fix it"},1]`,
		`{"one":["a"],"two":{"three":"four"}}`,
		`{"a":1,"a":2}`,
		`{"one":"x","two":{"n":1,"n":2}}`,
		`{}`,
		`[]`,
		`12`,
		`true`,
		`null`,
	} {
		raw := json.RawMessage(value)
		lines, renderable := renderBlock("key", 0, raw)
		if !renderable {
			// A refusal owes the same fidelity by the other route, so
			// the walk follows the fallback rather than skipping the
			// value it refused.
			refused := NewFrontmatter()
			writeMember(refused, "key", raw)
			if got := blockValue(refused, "key"); !sameJSON(got, raw) {
				t.Errorf("%s was refused and its raw line read back as %s", value, got)
			}
			continue
		}
		fm, _ := ParseAnchor("---\n" + strings.Join(lines, "\n") + "\n---\n")
		if got := blockValue(fm, "key"); !sameJSON(got, raw) {
			t.Errorf("%s rendered as\n%s\nand read back as %s", value, strings.Join(lines, "\n"), got)
		}
	}
}

// TestADuplicatedMemberNameIsPreservedRatherThanHalfWritten asserts dinah-196
// AC-15 at the level the defect lived at. A JSON object carrying the same
// member name twice has no frontmatter spelling, because a block of `name:`
// lines reads back keeping the first occurrence alone. The renderer therefore
// refuses it and the caller writes the one raw JSON line, which reads back as
// the value it was handed.
//
// This is the case the object arm shipped wrong: jsonMembers dropped the
// second member and reported success, so the clone carried `dup:` with one
// child and the second export differed from the first.
func TestADuplicatedMemberNameIsPreservedRatherThanHalfWritten(t *testing.T) {
	for _, value := range []string{
		`{"a":1,"a":2}`,
		`{"one":"x","two":{"n":1,"n":2}}`,
		`{"levels":{"severity":["minor"],"severity":["major"]}}`,
	} {
		raw := json.RawMessage(value)
		if lines, renderable := renderBlock("dup", 0, raw); renderable {
			t.Errorf("%s rendered as a block:\n%s", value, strings.Join(lines, "\n"))
		}
		fm := NewFrontmatter()
		writeMember(fm, "dup", raw)
		if lines := fm.Raw("dup"); len(lines) != 1 {
			t.Errorf("%s drew %d lines, wanted the one raw line: %v", value, len(lines), lines)
		}
		if got := blockValue(fm, "dup"); !sameJSON(got, raw) {
			t.Errorf("%s came back as %s", value, got)
		}
	}
}

// TestADuplicatedLevelsAxisIsPreservedRatherThanHalfWritten asserts dinah-196
// AC-15 on the one path that does not go through renderBlock. A definition
// hand-written with the same axis declared twice reaches renderLevelsMember,
// which orders the axes itself, so the same rule has to hold there: refuse,
// and let the raw JSON line preserve the member.
func TestADuplicatedLevelsAxisIsPreservedRatherThanHalfWritten(t *testing.T) {
	raw := json.RawMessage(`{"severity":["minor"],"severity":["major"]}`)
	if lines, renderable := renderLevelsMember(raw); renderable {
		t.Errorf("the duplicated axis rendered as a block:\n%s", strings.Join(lines, "\n"))
	}
	if got := orderedLevels(raw); !sameJSON(got, raw) {
		t.Errorf("export reordered the duplicated axis into %s, wanted it carried unchanged", got)
	}
}

// TestAnUnrenderableMemberFallsBackToOneRawLine asserts that the fallback
// preserves rather than loses, which is what lets the renderer refuse freely.
// A member the renderer will not write travels as the raw JSON line every
// unrecognized member has always travelled as, and reads back as itself.
func TestAnUnrenderableMemberFallsBackToOneRawLine(t *testing.T) {
	fm := NewFrontmatter()
	raw := json.RawMessage(`[{"major":"fix it"},1]`)
	writeMember(fm, "mixed", raw)
	if lines := fm.Raw("mixed"); len(lines) != 1 {
		t.Fatalf("the refused member drew %d lines, wanted the one raw line: %v", len(lines), lines)
	}
	if got := blockValue(fm, "mixed"); !sameJSON(got, raw) {
		t.Errorf("the raw line reads back as %s and the member was %s", got, raw)
	}
}
