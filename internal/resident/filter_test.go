package resident

import (
	"errors"
	"strings"
	"testing"
)

// TestPlaceJudgesEveryRecord is part of dinah-619/criteria/19. It tables
// place over a prefix of five elements, the workbench root and its four
// ancestors below the volume root, two of which carry a short alias that
// differs from the long name. A file three levels below the root is inside
// under its long names, its short names and mixed case; the root and each
// ancestor are moved under every action but Modified and elsewhere under
// Modified; a sibling of the root whose name shares its leading characters,
// and a path below one, is elsewhere under every action; a path in another
// branch of the volume is elsewhere; and a name with a character outside the
// Basic Multilingual Plane below the root is inside.
//
// The specification's table speaks of a prefix of four elements and of the
// root's four ancestors, which cannot both hold; the prefix here is five
// elements so that the root has four ancestors to table.
//
// Arming: comparing the root's element as a text prefix rather than element
// by element places <root>-old\x inside.
func TestPlaceJudgesEveryRecord(t *testing.T) {
	prefix := []element{
		{long: "Users", alias: "Users"},
		{long: "Paul Documents", alias: "PAULDO~1"},
		{long: "source", alias: "source"},
		{long: "repos", alias: "repos"},
		{long: "My Workbench", alias: "MYWORK~1"},
	}
	type row struct {
		name   string
		action Action
		want   placed
		path   string
	}
	var rows []row
	below := `cards\0123456789ab\card.md`
	rows = append(rows,
		row{`Users\Paul Documents\source\repos\My Workbench\` + below, Modified, inside, below},
		row{`Users\PAULDO~1\source\repos\MYWORK~1\` + below, Added, inside, below},
		row{`USERS\paul documents\SOURCE\Repos\my workbench\` + below, RenamedNew, inside, below},
	)
	all := []Action{Added, Removed, Modified, RenamedOld, RenamedNew}
	for depth := 1; depth <= len(prefix); depth++ {
		var elements []string
		for _, e := range prefix[:depth] {
			elements = append(elements, e.long)
		}
		name := strings.Join(elements, `\`)
		for _, action := range all {
			want := moved
			if action == Modified {
				want = elsewhere
			}
			rows = append(rows, row{name, action, want, ""})
		}
	}
	parent := `Users\Paul Documents\source\repos\`
	for _, sibling := range []string{"My Workbench-old", "My Workbench2"} {
		for _, action := range all {
			rows = append(rows, row{parent + sibling, action, elsewhere, ""})
			rows = append(rows, row{parent + sibling + `\x`, action, elsewhere, ""})
		}
	}
	rows = append(rows,
		row{`Windows\System32\drivers\etc\hosts`, Modified, elsewhere, ""},
		row{`Users\Paul Documents\source\repos\My Workbench\notes\note-` + "\U0001F600" + `.md`, Added, inside, `notes\note-` + "\U0001F600" + `.md`},
	)
	const cases = 3 + 5*5 + 2*5*2 + 2
	if len(rows) != cases {
		t.Fatalf("the table holds %d cases, wanted %d", len(rows), cases)
	}
	for _, r := range rows {
		got, where := place(prefix, r.name, r.action)
		if where != r.want {
			t.Errorf("place(%q, %v) answered %v, wanted %v", r.name, r.action, where, r.want)
			continue
		}
		if where == inside && (got.Path != r.path || got.Action != r.action) {
			t.Errorf("place(%q, %v) answered %+v, wanted the path %q", r.name, r.action, got, r.path)
		}
	}
}

// TestClassifyJudgesEveryPathShape is part of dinah-619/criteria/21. One case
// per row of section 4.6's table, and a workbench at the volume root itself,
// which dinah init never makes but which must not panic.
//
// Arming: comparing the elements case-insensitively accepts a pair that
// differs only by case as well formed.
func TestClassifyJudgesEveryPathShape(t *testing.T) {
	const volume = `\\?\Volume{0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0}\`
	type row struct {
		name, guid, dos string
		root            string
		long            []string
		why             Why
		fails           bool
	}
	rows := []row{
		{"a well-formed pair", volume + `Users\paul\wb`, `\\?\C:\Users\paul\wb`, volume, []string{"Users", "paul", "wb"}, "", false},
		{"a GUID path without the volume GUID shape", `\\?\Volume{not-a-guid}\wb`, `\\?\C:\wb`, "", nil, "", true},
		{"a DOS path without a drive", volume + `wb`, `\\?\UNC\server\share\wb`, "", nil, "", true},
		{"a workbench on a volume mounted in a folder", volume + `wb`, `\\?\C:\mnt\vol\wb`, "", nil, WhyMountedInFolder, false},
		{"a pair whose elements differ in the middle", volume + `Users\paul\wb`, `\\?\C:\Users\other\wb`, "", nil, "", true},
		{"a pair differing only in the case of one element", volume + `Users\paul\wb`, `\\?\C:\Users\Paul\wb`, "", nil, "", true},
		{"a workbench at the volume root", volume, `\\?\C:\`, volume, nil, "", false},
	}
	if len(rows) != 7 {
		t.Fatalf("the table holds %d cases, wanted 7", len(rows))
	}
	for _, r := range rows {
		root, long, why, err := classify(r.guid, r.dos)
		switch {
		case r.fails:
			if err == nil {
				t.Errorf("%s: classify answered %q %v %q and no error", r.name, root, long, why)
			}
		case err != nil:
			t.Errorf("%s: classify answered %v", r.name, err)
		case root != r.root || why != r.why || strings.Join(long, `\`) != strings.Join(r.long, `\`):
			t.Errorf("%s: classify answered %q %q %q, wanted %q %q %q", r.name, root, long, why, r.root, r.long, r.why)
		}
	}
	var unsupported *Unsupported
	if err := error(&Unsupported{Why: WhyMountedInFolder}); !errors.Is(err, ErrUnsupported) || !errors.As(err, &unsupported) {
		t.Error("an *Unsupported does not answer to ErrUnsupported")
	}
}
