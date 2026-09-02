package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// benchDeclaring returns a workbench root whose anchor carries the given
// levels block verbatim, so a test states the declaration exactly as a person
// would type it into workbench.md.
func benchDeclaring(t *testing.T, block string) *Bench {
	t.Helper()
	root := t.TempDir()
	anchor := strings.Replace(benchDefinition, "columns:\n  - b00000000001\n", "columns:\n  - b00000000001\n"+block, 1)
	write(t, filepath.Join(root, WorkbenchAnchor), anchor)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return opened
}

// namesOf is one axis's members in declaration order, which is what most of
// the assertions below compare.
func namesOf(levels []Level) []string {
	names := make([]string, 0, len(levels))
	for _, level := range levels {
		names = append(names, level.Name)
	}
	return names
}

// TestAFlowSequenceDeclaresLevelsInOrder asserts dinah-193 AC-1: the flow form
// parses to its members in declaration order, ranked from zero, carrying no
// hints.
func TestAFlowSequenceDeclaresLevelsInOrder(t *testing.T) {
	opened := benchDeclaring(t, "levels:\n  severity: [trivial, minor, major, critical]\n")
	levels := opened.Levels("severity")
	want := []string{"trivial", "minor", "major", "critical"}
	if got := namesOf(levels); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the declared severity levels are %v, wanted %v", got, want)
	}
	for i, level := range levels {
		if level.Rank != i {
			t.Errorf("%s ranks %d and its position in the declaration is %d", level.Name, level.Rank, i)
		}
		if level.Hint != "" {
			t.Errorf("%s carries the hint %q and the flow form declares none", level.Name, level.Hint)
		}
	}
}

// TestDashedEntriesDeclareTheSameOrderAndCarryAHint asserts dinah-193 AC-2:
// the dashed form parses to the same names in the same order, and a hint
// reaches Level.Hint on the entry that carries one and on no other.
func TestDashedEntriesDeclareTheSameOrderAndCarryAHint(t *testing.T) {
	const hint = "A person's work is wrong or blocked; fix before new work."
	opened := benchDeclaring(t, "levels:\n  severity:\n    - trivial\n    - minor\n    - major: "+hint+"\n    - critical\n")
	levels := opened.Levels("severity")
	want := []string{"trivial", "minor", "major", "critical"}
	if got := namesOf(levels); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the dashed form declared %v, wanted the same four names the flow form declares, %v", got, want)
	}
	for _, level := range levels {
		switch level.Name {
		case "major":
			if level.Hint != hint {
				t.Errorf("major carries the hint %q, wanted %q", level.Hint, hint)
			}
		default:
			if level.Hint != "" {
				t.Errorf("%s carries the hint %q and its entry is a bare name", level.Name, level.Hint)
			}
		}
	}
}

// TestOneBlockMixesTheTwoFormsAndIgnoresAnAxisItDoesNotRead asserts dinah-193
// AC-3: severity in the flow form and priority in the dashed form both parse
// out of one block, and an axis outside LevelAxes takes no part in the model
// while staying on disk untouched.
func TestOneBlockMixesTheTwoFormsAndIgnoresAnAxisItDoesNotRead(t *testing.T) {
	block := "levels:\n" +
		"  severity: [trivial, minor, major, critical]\n" +
		"  priority:\n    - later\n    - soon\n    - next\n    - now\n" +
		"  urgency: [whenever, immediately]\n"
	opened := benchDeclaring(t, block)
	if got := strings.Join(namesOf(opened.Levels("severity")), ","); got != "trivial,minor,major,critical" {
		t.Errorf("the flow-form axis parsed to %q", got)
	}
	if got := strings.Join(namesOf(opened.Levels("priority")), ","); got != "later,soon,next,now" {
		t.Errorf("the dashed-form axis parsed to %q", got)
	}
	if KnownLevelAxis("urgency") {
		t.Error("urgency reads as an axis this model knows, and LevelAxes carries two")
	}
	if got := opened.Levels("urgency"); got != nil {
		t.Errorf("an axis outside LevelAxes reached the model as %v", got)
	}
	stored, err := os.ReadFile(filepath.Join(opened.Root, WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if !strings.Contains(string(stored), "  urgency: [whenever, immediately]") {
		t.Error("the axis the model ignores is no longer on disk, and an unread key is preserved rather than dropped")
	}
}

// TestRanksAreCountedWithinAnAxisAndNeverAcrossTheBlock asserts dinah-193
// AC-27. The two axes are deliberately different lengths, so a rank counted
// across the block rather than within an axis is caught whichever axis the
// counter starts on, and the shorter axis's first member still ranks zero.
func TestRanksAreCountedWithinAnAxisAndNeverAcrossTheBlock(t *testing.T) {
	block := "levels:\n  severity: [trivial, minor, major, critical]\n  priority: [later, soon, now]\n"
	opened := benchDeclaring(t, block)
	severity := opened.Levels("severity")
	priority := opened.Levels("priority")
	if len(severity) != 4 || len(priority) != 3 {
		t.Fatalf("the block declared %d severity levels and %d priority levels, wanted four and three", len(severity), len(priority))
	}
	for i, level := range severity {
		if level.Rank != i {
			t.Errorf("severity %s ranks %d, wanted %d", level.Name, level.Rank, i)
		}
	}
	for i, level := range priority {
		if level.Rank != i {
			t.Errorf("priority %s ranks %d, wanted %d, so the ranks are being counted across the block", level.Name, level.Rank, i)
		}
	}
	if first := opened.Level("priority", "later"); first == nil || first.Rank != 0 {
		t.Errorf("the shorter axis's first member is %v, and it ranks zero however many members the other axis carries", first)
	}
}

// TestOneAxisIsDeclaredWithoutTheOther asserts the independence the format
// states: an axis carrying no declaration answers nil whether or not the other
// axis carries one, which is what every per-axis check below rests on.
func TestOneAxisIsDeclaredWithoutTheOther(t *testing.T) {
	opened := benchDeclaring(t, "levels:\n  severity: [minor, major]\n")
	if got := opened.Levels("severity"); len(got) != 2 {
		t.Errorf("the declared axis parsed to %v", got)
	}
	if got := opened.Levels("priority"); got != nil {
		t.Errorf("the undeclared axis answered %v, and an axis with no declaration answers nil", got)
	}
	if got := opened.Level("priority", "now"); got != nil {
		t.Errorf("a lookup on the undeclared axis answered %v", got)
	}
	bare := benchDeclaring(t, "")
	for _, axis := range LevelAxes {
		if got := bare.Levels(axis); got != nil {
			t.Errorf("a workbench declaring no block answered %v for %s", got, axis)
		}
	}
}

// TestADuplicatedLevelKeepsItsFirstOccurrence asserts the third parsing rule,
// and TestABlockWithNoParseableChildLeavesEveryAxisUndeclared the fourth: a
// block the reader cannot make sense of is ignored rather than refused, which
// is the format's reader posture.
func TestADuplicatedLevelKeepsItsFirstOccurrence(t *testing.T) {
	opened := benchDeclaring(t, "levels:\n  severity: [minor, major, minor]\n")
	if got := strings.Join(namesOf(opened.Levels("severity")), ","); got != "minor,major" {
		t.Errorf("the axis parsed to %q, and a duplicate keeps its first occurrence", got)
	}
	if found := opened.Level("severity", "minor"); found == nil || found.Rank != 0 {
		t.Errorf("the duplicated name looks up to %v, and the first occurrence is what both the rank and the lookup keep", found)
	}
}

func TestABlockWithNoParseableChildLeavesEveryAxisUndeclared(t *testing.T) {
	opened := benchDeclaring(t, "levels:\n")
	for _, axis := range LevelAxes {
		if got := opened.Levels(axis); got != nil {
			t.Errorf("%s parsed to %v out of a block carrying no child", axis, got)
		}
	}
}

// TestWritingAWorkbenchFieldLeavesTheLevelsBlockByteIdentical asserts
// dinah-193 AC-4. The model now reads the block, and the temptation to write
// it back is new, so this holds Save to writing only the three fields it owns.
func TestWritingAWorkbenchFieldLeavesTheLevelsBlockByteIdentical(t *testing.T) {
	block := "levels:\n" +
		"  # the two axes this workbench declares\n" +
		"  severity:\n" +
		"    - minor\n" +
		"    - major: A person's work is wrong or blocked.\n" +
		"  priority: [later, soon, now]\n"
	opened := benchDeclaring(t, block)
	path := filepath.Join(opened.Root, WorkbenchAnchor)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	opened.SetWorkbenchField("title", "Renamed")
	if err := opened.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reread the anchor: %v", err)
	}
	if !strings.Contains(string(after), block) {
		t.Errorf("the block was rewritten:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if !strings.Contains(string(after), "title: Renamed") {
		t.Error("the field the write named did not land")
	}
	reopened, err := Open(opened.Root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if found := reopened.Level("severity", "major"); found == nil || found.Hint != "A person's work is wrong or blocked." {
		t.Errorf("the hint did not survive the write, reading back as %v", found)
	}
}

// TestACardKeepsEveryFrontmatterKeyItDoesNotKnowAroundALevel asserts dinah-193
// AC-17 at the format layer: writing a level and clearing one both leave a
// key the tool has never heard of in place.
func TestACardKeepsEveryFrontmatterKeyItDoesNotKnowAroundALevel(t *testing.T) {
	root := t.TempDir()
	const anchor = `---
title: A card
number: 1
column: b00000000001
state: ready
project: awan-saya
repository: git@example.com:awan/saya.git
---
Framing.
`
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), anchor)
	collection := filepath.Join(root, CardsDir)
	card, err := LoadCard(collection, "c00000000001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	card.SetLevel(SeverityField, "major")
	if err := card.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	written := readCardAnchor(t, collection)
	for _, key := range []string{"project: awan-saya", "repository: git@example.com:awan/saya.git"} {
		if !strings.Contains(written, key) {
			t.Errorf("writing a level dropped %q:\n%s", key, written)
		}
	}
	if !strings.Contains(written, "state: ready\nseverity: major\n") {
		t.Errorf("the level did not land directly under state:\n%s", written)
	}
	cleared, err := LoadCard(collection, "c00000000001")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	cleared.SetLevel(SeverityField, "")
	if err := cleared.Save(); err != nil {
		t.Fatalf("resave: %v", err)
	}
	after := readCardAnchor(t, collection)
	if strings.Contains(after, "severity") {
		t.Errorf("clearing left the key behind rather than removing it:\n%s", after)
	}
	for _, key := range []string{"project: awan-saya", "repository: git@example.com:awan/saya.git"} {
		if !strings.Contains(after, key) {
			t.Errorf("clearing a level dropped %q:\n%s", key, after)
		}
	}
}

// TestBothLevelsLandUnderStateInOneOrder asserts the placement rule of
// dinah-193 section 2 for the case Card.Save owns: a card carrying neither
// level and written with both reads severity, then priority, under state.
func TestBothLevelsLandUnderStateInOneOrder(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), cleanCard)
	collection := filepath.Join(root, CardsDir)
	card, err := LoadCard(collection, "c00000000001")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	card.Severity, card.Priority = "major", "now"
	if err := card.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if written := readCardAnchor(t, collection); !strings.Contains(written, "state: ready\nseverity: major\npriority: now\n") {
		t.Errorf("the pair did not land under state in order:\n%s", written)
	}
}

// readCardAnchor reads the one card anchor of a collection back as text.
func readCardAnchor(t *testing.T, collection string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(collection, "c00000000001", CardAnchor))
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	return string(data)
}

// TestADefinitionsLevelsMemberIsRenderedAsABlock asserts dinah-193 AC-24: a
// definition declaring both axes is instantiated as the flow form, one axis
// per line and in LevelAxes order, and the model reads both axes back out of
// the workbench that was created.
func TestADefinitionsLevelsMemberIsRenderedAsABlock(t *testing.T) {
	root := containedPath(filepath.Join(t.TempDir(), "created"))
	member := `{"priority": ["later", "soon", "next", "now"], "severity": ["trivial", "minor", "major", "critical"]}`
	instantiateWithLevels(t, root, member)
	anchor := readWorkbenchAnchor(t, root)
	const want = "levels:\n  severity: [trivial, minor, major, critical]\n  priority: [later, soon, next, now]\n"
	if !strings.Contains(anchor, want) {
		t.Errorf("the rendered block is not the flow form in LevelAxes order:\n%s", anchor)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got := strings.Join(namesOf(opened.Levels("severity")), ","); got != "trivial,minor,major,critical" {
		t.Errorf("severity read back as %q", got)
	}
	if got := strings.Join(namesOf(opened.Levels("priority")), ","); got != "later,soon,next,now" {
		t.Errorf("priority read back as %q", got)
	}
}

// TestADefinitionDeclaringOneAxisWritesThatAxisAlone asserts dinah-193 AC-28.
// The case is stated for a priority-only definition, because LevelAxes orders
// priority second and a renderer walking positions rather than presence would
// pass the two-axis case and fail this one.
func TestADefinitionDeclaringOneAxisWritesThatAxisAlone(t *testing.T) {
	root := containedPath(filepath.Join(t.TempDir(), "created"))
	instantiateWithLevels(t, root, `{"priority": ["later", "soon", "now"]}`)
	anchor := readWorkbenchAnchor(t, root)
	if !strings.Contains(anchor, "levels:\n  priority: [later, soon, now]\n") {
		t.Errorf("the one declared axis was not written in the flow form:\n%s", anchor)
	}
	if strings.Contains(anchor, "severity") {
		t.Errorf("the axis the definition does not declare reached the anchor:\n%s", anchor)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got := strings.Join(namesOf(opened.Levels("priority")), ","); got != "later,soon,now" {
		t.Errorf("priority read back as %q", got)
	}
	if got := opened.Levels("severity"); got != nil {
		t.Errorf("severity read back as %v, and the definition declares no set for it", got)
	}
}

// TestALevelsMemberTheRendererCannotReadTravelsAsOneRawLine asserts the
// fallback half of dinah-193 AC-24. A member whose axis is not an array of
// strings loses nothing: it lands as the one raw JSON line every unrecognized
// member has always travelled as.
func TestALevelsMemberTheRendererCannotReadTravelsAsOneRawLine(t *testing.T) {
	root := containedPath(filepath.Join(t.TempDir(), "created"))
	member := `{"severity": [{"name": "major", "rank": 2}]}`
	instantiateWithLevels(t, root, member)
	anchor := readWorkbenchAnchor(t, root)
	lines := 0
	for _, line := range SplitLines(anchor) {
		if strings.HasPrefix(line, LevelsKey+":") {
			lines++
		}
	}
	if lines != 1 {
		t.Errorf("the unreadable member drew %d levels lines, wanted the one raw line:\n%s", lines, anchor)
	}
	fm, _ := ParseAnchor(anchor)
	if got := fm.Value(LevelsKey); got != member {
		t.Errorf("the raw line reads back as %q and the member was %q, so something was lost", got, member)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, axis := range LevelAxes {
		if got := opened.Levels(axis); got != nil {
			t.Errorf("%s parsed to %v out of a line the model cannot read", axis, got)
		}
	}
}

// TestAFurtherAxisIsRenderedAfterTheTwoInSortedOrder asserts the preservation
// half of the renderer's ordering rule: an axis the model does not read is
// written after the two it does, rather than dropped.
func TestAFurtherAxisIsRenderedAfterTheTwoInSortedOrder(t *testing.T) {
	root := containedPath(filepath.Join(t.TempDir(), "created"))
	member := `{"urgency": ["whenever"], "severity": ["minor"], "abandon": ["never"]}`
	instantiateWithLevels(t, root, member)
	anchor := readWorkbenchAnchor(t, root)
	const want = "levels:\n  severity: [minor]\n  abandon: [never]\n  urgency: [whenever]\n"
	if !strings.Contains(anchor, want) {
		t.Errorf("the further axes were not written after the declared ones in sorted order:\n%s", anchor)
	}
}

// instantiateWithLevels writes a workbench from a definition carrying one
// levels member, spelled exactly as the JSON the caller passes.
func instantiateWithLevels(t *testing.T, root, member string) {
	t.Helper()
	source := `{
  "profile": "dinah-core/0.7",
  "title": "Levelled",
  "levels": ` + member + `,
  "columns": [{ "id": "b00000000001", "title": "Only", "kind": "work" }]
}`
	definition, err := ReadDefinition([]byte(source))
	if err != nil {
		t.Fatalf("read the definition: %v", err)
	}
	if err := Instantiate(root, "lv", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
}

// readWorkbenchAnchor reads a workbench anchor back as text.
func readWorkbenchAnchor(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	return string(data)
}

// TestTheInterchangeFormCarriesTheLevelsBlock asserts dinah-196 AC-12, which
// inverts the guard dinah-193 left here. Export used to skip levels for being
// a known key and print no member at all; it now emits the declared block as
// the object of arrays the importer already reads.
func TestTheInterchangeFormCarriesTheLevelsBlock(t *testing.T) {
	opened := benchDeclaring(t, "levels:\n  severity: [minor, major]\n")
	encoded, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	object := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw, carried := object[LevelsKey]
	if !carried {
		t.Fatalf("export carried no levels member:\n%s", encoded)
	}
	declared := map[string][]string{}
	if err := json.Unmarshal(raw, &declared); err != nil {
		t.Fatalf("the levels member is not an object of arrays: %v", err)
	}
	if got := strings.Join(declared["severity"], ","); got != "minor,major" {
		t.Errorf("the exported severity axis is %q, wanted the declared members in order", got)
	}
	if _, invented := declared["priority"]; invented {
		t.Errorf("export carries a priority axis the workbench does not declare: %s", raw)
	}
}
