package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// viewsBlock is the declaration of the specification's section 2.2: a view
// declaring every member, with sections of two members, and a view declaring
// only sections, with a section of one.
const viewsBlock = ViewsKey + ":\n" +
	"  waiting-on-me:\n" +
	"    title: Waiting on me\n" +
	"    layout: list\n" +
	"    order: column\n" +
	"    sections:\n" +
	"      - title: At my stations\n" +
	"        query: \"column:operator-design-review,operator-code-review,acceptance\"\n" +
	"      - title: Questions only I can answer\n" +
	"        query: \"item_owner:operator item_state:pending\"\n" +
	"      - title: Blocked\n" +
	"        query: \"state:blocked\"\n" +
	"  urgent-in-flight:\n" +
	"    sections:\n" +
	"      - query: \"priority:now,next column!=intake column!=done\"\n"

// TestAViewsBlockReadsEveryMemberAndItsDefaults asserts the reader of section
// 2.2 over both views of the block, and the defaults the accessors apply to a
// view declaring only its sections.
func TestAViewsBlockReadsEveryMemberAndItsDefaults(t *testing.T) {
	opened := benchDeclaring(t, viewsBlock)
	views, blockDefect := opened.Views()
	if blockDefect {
		t.Fatal("a well-formed block was reported as not a mapping")
	}
	if len(views) != 2 {
		t.Fatalf("read %d views, want 2: %+v", len(views), views)
	}
	full, bare := views[0], views[1]
	if full.Name != "waiting-on-me" || full.Title != "Waiting on me" || full.Layout != "list" || full.Order != "column" || full.Defect != "" || full.Source != ViewSourceWorkbench {
		t.Errorf("the full view read as %+v", full)
	}
	wantSections := []ViewSection{
		{Title: "At my stations", Query: "column:operator-design-review,operator-code-review,acceptance"},
		{Title: "Questions only I can answer", Query: "item_owner:operator item_state:pending"},
		{Title: "Blocked", Query: "state:blocked"},
	}
	if len(full.Sections) != len(wantSections) {
		t.Fatalf("the full view carries %d sections, want %d", len(full.Sections), len(wantSections))
	}
	for i, want := range wantSections {
		if full.Sections[i] != want {
			t.Errorf("section %d read as %+v, want %+v", i+1, full.Sections[i], want)
		}
	}
	if bare.EffectiveTitle() != "urgent-in-flight" || bare.EffectiveLayout() != ViewLayoutList || bare.EffectiveOrder() != ViewOrderArrival {
		t.Errorf("the bare view's defaults read as %q, %q, %q", bare.EffectiveTitle(), bare.EffectiveLayout(), bare.EffectiveOrder())
	}
	if got := bare.Sections[0].Heading(); got != "priority:now,next column!=intake column!=done" {
		t.Errorf("a section with no title is headed %q, want its query as declared", got)
	}
	if bare.Defect != "" {
		t.Errorf("the bare view reads as malformed: %s", bare.Defect)
	}
}

// TestEachDefectIsProducedByItsOwnFixture asserts dinah-600/criteria/3 at the
// reader and at check: each of the seven tokens is recorded on a view built
// for it, and check reports each one under check.view-malformed with the
// view's name and the token. A well-formed sibling in the same block reads
// clean, so a reader that marked every view malformed fails here.
func TestEachDefectIsProducedByItsOwnFixture(t *testing.T) {
	block := ViewsKey + ":\n" +
		"  Bad_Name:\n    sections:\n      - query: \"state:ready\"\n" +
		"  flat: just some text\n" +
		"  nested-title:\n    title:\n      deep: value\n    sections:\n      - query: \"state:ready\"\n" +
		"  empty-sections:\n    sections: []\n" +
		"  no-query:\n    sections:\n      - title: only a title\n" +
		"  columns-layout:\n    layout: columns\n    sections:\n      - query: \"state:ready\"\n" +
		"  urgency-order:\n    order: urgency\n    sections:\n      - query: \"state:ready\"\n" +
		"  sibling:\n    sections:\n      - query: \"state:ready\"\n"
	opened := benchDeclaring(t, block)
	views, _ := opened.Views()
	want := map[string]string{
		"Bad_Name":       ViewInvalidName,
		"flat":           ViewNotAMapping,
		"nested-title":   ViewMalformedMember,
		"empty-sections": ViewNoSections,
		"no-query":       ViewSectionWithoutQuery,
		"columns-layout": ViewUnknownLayout,
		"urgency-order":  ViewUnknownOrder,
		"sibling":        "",
	}
	if len(views) != len(want) {
		t.Fatalf("read %d views, want %d", len(views), len(want))
	}
	seen := map[string]bool{}
	for _, view := range views {
		expected, known := want[view.Name]
		if !known {
			t.Errorf("read a view named %q, which the fixture does not declare", view.Name)
			continue
		}
		seen[view.Defect] = true
		if view.Defect != expected {
			t.Errorf("%s carries the defect %q, want %q", view.Name, view.Defect, expected)
		}
	}
	for _, token := range ViewDefects {
		if !seen[token] {
			t.Errorf("no fixture produced the defect %s", token)
		}
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	reported := map[string]bool{}
	for _, finding := range findings {
		if finding.Key == FindingViewMalformed {
			reported[finding.Detail] = true
		}
	}
	for name, token := range want {
		detail := name + " " + token
		if token == "" {
			if reported[name+" "] {
				t.Errorf("check reported the well-formed sibling")
			}
			continue
		}
		if !reported[detail] {
			t.Errorf("check did not report %q", detail)
		}
	}
	if len(reported) != len(ViewDefects) {
		t.Errorf("check reported %d malformed views, want %d: %v", len(reported), len(ViewDefects), reported)
	}
}

// TestAMemberThisBuildDoesNotKnowIsIgnoredAndReported asserts the reader and
// check halves of dinah-600/criteria/5: an unrecognised member at the view
// level and inside a section leaves the view well formed, a read leaves the
// file byte for byte as it was, and check reports each member at cleanup
// severity by its path.
func TestAMemberThisBuildDoesNotKnowIsIgnoredAndReported(t *testing.T) {
	block := ViewsKey + ":\n  held:\n    titel: Oops\n    sections:\n      - query: holder:@me\n        colour: red\n"
	opened := benchDeclaring(t, block)
	anchor := filepath.Join(opened.Root, WorkbenchAnchor)
	before, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	views, _ := opened.Views()
	if len(views) != 1 || views[0].Defect != "" {
		t.Fatalf("the view with unknown members read as %+v", views)
	}
	if got := strings.Join(views[0].Unknown, ","); got != "titel,sections/1/colour" {
		t.Errorf("the unknown members read as %q", got)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	details := map[string]string{}
	for _, finding := range findings {
		if finding.Key == FindingViewMemberUnknown {
			details[finding.Detail] = finding.Severity
		}
	}
	for _, detail := range []string{"held titel", "held sections/1/colour"} {
		severity, found := details[detail]
		if !found {
			t.Errorf("check did not report %q: %v", detail, details)
			continue
		}
		if severity != SeverityCleanup {
			t.Errorf("%q was reported at %q, want cleanup", detail, severity)
		}
	}
	after, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor again: %v", err)
	}
	if string(before) != string(after) {
		t.Error("reading and checking the views rewrote the workbench definition")
	}
}

// TestAViewsValueThatIsNotAMappingReadsNoView asserts the block-level defect:
// a dinah.views value that is not a mapping reads no view, is flagged, and is
// reported by check as dinah.views not-a-mapping.
func TestAViewsValueThatIsNotAMappingReadsNoView(t *testing.T) {
	opened := benchDeclaring(t, ViewsKey+": just a scalar\n")
	views, blockDefect := opened.Views()
	if !blockDefect || len(views) != 0 {
		t.Fatalf("a scalar block read as %d views with blockDefect %v", len(views), blockDefect)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	found := false
	for _, finding := range findings {
		if finding.Key == FindingViewMalformed && finding.Detail == ViewsKey+" "+ViewNotAMapping {
			found = true
		}
	}
	if !found {
		t.Errorf("check did not report the block: %+v", findings)
	}
}

// TestTheUserLayerReportsEachOfItsFourStates asserts section 2.9's four
// states, the unreadable one reached by a directory standing where config.md
// should be.
func TestTheUserLayerReportsEachOfItsFourStates(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(UserBase(home), 0o755); err != nil {
		t.Fatalf("user base: %v", err)
	}
	if views, state := LoadUserViews(home); state != UserViewsAbsent || len(views) != 0 {
		t.Errorf("an absent config.md answered %v with %d views", state, len(views))
	}
	path := UserViewsPath(home)
	write(t, path, "---\nlang: en\n"+viewsBlock+"---\n")
	views, state := LoadUserViews(home)
	if state != UserViewsRead || len(views) != 2 || views[0].Source != ViewSourceUser {
		t.Errorf("a readable config.md answered %v with %+v", state, views)
	}
	write(t, path, "---\nlang: en\n---\n")
	if views, state := LoadUserViews(home); state != UserViewsRead || len(views) != 0 {
		t.Errorf("a config.md declaring no views answered %v with %d views", state, len(views))
	}
	write(t, path, "---\n"+ViewsKey+": 12\n---\n")
	if _, state := LoadUserViews(home); state != UserViewsNotAMapping {
		t.Errorf("a scalar dinah.views answered %v, want not-a-mapping", state)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("plant a directory at the config path: %v", err)
	}
	if _, state := LoadUserViews(home); state != UserViewsUnreadable {
		t.Errorf("a directory at config.md answered %v, want unreadable", state)
	}
}

// TestAViewsBlockSurvivesTheInterchangeAsABlock asserts the library half of
// dinah-600/criteria/1: after export and an instantiation from the export,
// the new anchor carries dinah.views as a nested block rather than one line of
// JSON, the views read back identically, and a second export is
// byte-identical to the first.
func TestAViewsBlockSurvivesTheInterchangeAsABlock(t *testing.T) {
	source := benchDeclaring(t, viewsBlock)
	first, err := source.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	root := cloneThrough(t, source)
	anchor := readWorkbenchAnchor(t, root)
	if !strings.Contains(anchor, ViewsKey+":\n  waiting-on-me:\n") {
		t.Errorf("the clone does not carry the views as a block:\n%s", anchor)
	}
	if !strings.Contains(anchor, "      - title: At my stations\n        query: ") {
		t.Errorf("a two-member section was not written as a dashed entry with its second member under its first:\n%s", anchor)
	}
	clone, err := Open(root)
	if err != nil {
		t.Fatalf("open the clone: %v", err)
	}
	want, _ := source.Views()
	got, _ := clone.Views()
	if len(want) != len(got) {
		t.Fatalf("the clone reads %d views, the source %d", len(got), len(want))
	}
	for i := range want {
		if want[i].Name != got[i].Name || want[i].Title != got[i].Title || len(want[i].Sections) != len(got[i].Sections) {
			t.Errorf("view %d: the source reads %+v and the clone %+v", i, want[i], got[i])
			continue
		}
		for j := range want[i].Sections {
			if want[i].Sections[j] != got[i].Sections[j] {
				t.Errorf("view %s section %d: the source reads %+v and the clone %+v", want[i].Name, j+1, want[i].Sections[j], got[i].Sections[j])
			}
		}
	}
	if second := exportOf(t, root); stampedProfile(string(first)) != stampedProfile(second) {
		t.Errorf("a second export differs from the first:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestTheMultiMemberFormReachesTheViewsMemberAlone asserts
// dinah-600/criteria/31: members whose consuming reader is not blockValue
// keep landing as one raw JSON line through the import, levels declaring no
// level at all, while dinah.views in the same definition lands as a block.
// Every member here carries an array of objects of two members, so each would
// render as a block under a writer that gave the form to every member.
func TestTheMultiMemberFormReachesTheViewsMemberAlone(t *testing.T) {
	for _, levels := range []string{
		`{"severity": [{"name": "major", "rank": 2}]}`,
		`{"severity": [{"major": "big", "minor": "small"}, "trivial"]}`,
	} {
		t.Run(levels, func(t *testing.T) {
			views := `{"waiting": {"sections": [{"title": "Mine", "query": "holder:@me"}]}}`
			routes := `{"short": [{"column": "b00000000001", "note": "only"}, {"column": "b00000000001", "note": "again"}]}`
			notes := `[{"who": "ana", "said": "hello"}, {"who": "bo", "said": "hi"}]`
			source := `{
  "profile": "dinah-core/0.7",
  "title": "Several",
  "levels": ` + levels + `,
  "routes": ` + routes + `,
  "acme.notes": ` + notes + `,
  "` + ViewsKey + `": ` + views + `,
  "columns": [{ "id": "b00000000001", "title": "Only", "kind": "work" }]
}`
			definition, err := ReadDefinition([]byte(source))
			if err != nil {
				t.Fatalf("read the definition: %v", err)
			}
			root := containedPath(filepath.Join(t.TempDir(), "created"))
			if err := Instantiate(root, "sv", "ana", definition); err != nil {
				t.Fatalf("instantiate: %v", err)
			}
			anchor := readWorkbenchAnchor(t, root)
			fm, _ := ParseAnchor(anchor)
			for key, member := range map[string]string{LevelsKey: levels, RoutesKey: routes, "acme.notes": notes} {
				lines := fm.Raw(key)
				if len(lines) != 1 {
					t.Errorf("%s landed as %d lines, want the one raw line:\n%s", key, len(lines), anchor)
					continue
				}
				if got := fm.Value(key); !sameJSON(json.RawMessage(got), json.RawMessage(member)) {
					t.Errorf("%s reads back as %q, and the member was %q", key, got, member)
				}
			}
			if lines := fm.Raw(ViewsKey); len(lines) < 2 {
				t.Errorf("dinah.views landed as %d lines, want a block:\n%s", len(lines), anchor)
			}
			opened, err := Open(root)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			for _, axis := range LevelAxes {
				if got := opened.Levels(axis); got != nil {
					t.Errorf("%s declares %v out of a line the model cannot read", axis, got)
				}
			}
			read, _ := opened.Views()
			if len(read) != 1 || read[0].Sections[0].Title != "Mine" || read[0].Sections[0].Query != "holder:@me" {
				t.Errorf("the views read back as %+v", read)
			}
		})
	}
}
