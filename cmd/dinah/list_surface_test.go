package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// listBench builds the workbench the checks below read: one card carrying a
// comment, a checklist item and an attachment, one workstream the card has
// joined, and an attachment on the workbench itself.
func listBench(t *testing.T) string {
	t.Helper()
	root := newBench(t)
	mustRun(t, root, "add", "a card the listing reads")
	mustRun(t, root, "comment", "fx-1", "a thought")
	mustRun(t, root, "file", "fx-1", "decision", "a decision")
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write the attachment source: %v", err)
	}
	mustRun(t, root, "attach", "fx-1", source, "--description", "a file")
	mustRun(t, root, "attach", "workbench", source, "--description", "the workbench's own file")
	mustRun(t, root, "workstream", "new", "Autumn release", "--slug", "autumn")
	mustRun(t, root, "join", "fx-1", "workstream/autumn")

	// The archive half is seeded too, because --archived reads a reference
	// there and a flag that reads is asserted by an answer rather than by an
	// acceptance: a live reference under the flag refuses, correctly, that
	// nothing in the archive answers to it.
	mustRun(t, root, "add", "a card the archive holds")
	mustRun(t, root, "attach", "fx-2", source, "--description", "an archived file")
	mustRun(t, root, "archive", "fx-2")
	mustRun(t, root, "column", "new", "Retired", "--kind", "work", "--slug", "retired")
	mustRun(t, root, "archive", "retired")
	mustRun(t, root, "workstream", "new", "Spring release", "--slug", "spring")
	mustRun(t, root, "archive", "workstream/spring")
	return root
}

// TestListReadsOrRefusesEachFlagAgainstEachReferenceShape is
// dinah-523/criteria/30, one row per cell of the flag table.
//
// The table the criterion names carries four flag columns over ten reference
// shapes, and the fifth flag the command declares, --max-depth, is absent from
// it because it bounds --root alone and keeps the dinah.depth-without-root
// refusal this card does not touch. So the cells are counted off the table and
// the fifth flag is asserted once, by that refusal, below.
//
// Both halves are asserted. A refused cell exits the refused code with
// dinah.usage as the first whitespace-separated token of stderr and carries the
// combination rather than the flag alone; a read cell exits zero and prints a
// non-empty answer, which is what stops the whole run passing against a build
// that refuses everything.
func TestListReadsOrRefusesEachFlagAgainstEachReferenceShape(t *testing.T) {
	root := listBench(t)
	forest := t.TempDir()
	mustRun(t, filepath.Join(root), "add", "a second card")

	type cell struct {
		flag  []string
		reads bool
	}
	shapes := []struct {
		name string
		ref  []string
		// archived is the reference the --archived cell uses, empty where
		// the shape's own reference already answers under the flag. A flag
		// that reads is asserted by an answer, so a shape whose live
		// reference is absent from the mirror names one that is there.
		archived []string
		cells    map[string]cell
	}{
		{name: "bare", ref: nil, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}, reads: true},
			"--ready":    {flag: []string{"--ready"}},
			"--archived": {flag: []string{"--archived"}, reads: true},
			"--root":     {flag: []string{"--root", forest}, reads: true},
		}},
		{name: "the workbench", ref: []string{"workbench"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}, reads: true},
			"--ready":    {flag: []string{"--ready"}},
			"--archived": {flag: []string{"--archived"}, reads: true},
			"--root":     {flag: []string{"--root", forest}, reads: true},
		}},
		{name: "the roster word columns", ref: []string{"columns"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}},
			"--ready":    {flag: []string{"--ready"}},
			"--archived": {flag: []string{"--archived"}},
			"--root":     {flag: []string{"--root", forest}, reads: true},
		}},
		{name: "the roster word cards", ref: []string{"cards"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}},
			"--ready":    {flag: []string{"--ready"}, reads: true},
			"--archived": {flag: []string{"--archived"}},
			"--root":     {flag: []string{"--root", forest}, reads: true},
		}},
		{name: "the roster word workstreams", ref: []string{"workstreams"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}},
			"--ready":    {flag: []string{"--ready"}},
			"--archived": {flag: []string{"--archived"}},
			"--root":     {flag: []string{"--root", forest}, reads: true},
		}},
		{name: "the roster word attachments", ref: []string{"attachments"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}},
			"--ready":    {flag: []string{"--ready"}},
			"--archived": {flag: []string{"--archived"}},
			"--root":     {flag: []string{"--root", forest}, reads: true},
		}},
		{name: "the roster word workbenches", ref: []string{"workbenches"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}},
			"--ready":    {flag: []string{"--ready"}},
			"--archived": {flag: []string{"--archived"}},
			"--root":     {flag: []string{"--root", forest}, reads: true},
		}},
		{name: "a column", ref: []string{"intake"}, archived: []string{"retired"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}, reads: true},
			"--ready":    {flag: []string{"--ready"}, reads: true},
			"--archived": {flag: []string{"--archived"}, reads: true},
			"--root":     {flag: []string{"--root", forest}, reads: true},
		}},
		// The workstream is the one shape that refuses --archived while
		// resolving in the archive half, on the ruling recorded as
		// dinah-523/decisions/12. Both of its readings answer the
		// membership, which is held in the live half alone, so the flag has
		// nothing to read here and is refused by name. The archived
		// reference is still named, because the resolver's own
		// not-archived refusal would otherwise fire ahead of the flag check
		// and this cell would pin the wrong refusal.
		{name: "a workstream", ref: []string{"workstream/autumn"}, archived: []string{"workstream/spring"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}, reads: true},
			"--ready":    {flag: []string{"--ready"}, reads: true},
			"--archived": {flag: []string{"--archived"}},
			"--root":     {flag: []string{"--root", forest}, reads: true},
		}},
		{name: "a card", ref: []string{"fx-1"}, archived: []string{"fx-2"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}, reads: true},
			"--ready":    {flag: []string{"--ready"}},
			"--archived": {flag: []string{"--archived"}, reads: true},
			"--root":     {flag: []string{"--root", forest}},
		}},
		{name: "a journal", ref: []string{"fx-1/journal"}, archived: []string{"fx-2/journal"}, cells: map[string]cell{
			"--depth":    {flag: []string{"--depth", "all"}},
			"--ready":    {flag: []string{"--ready"}},
			"--archived": {flag: []string{"--archived"}, reads: true},
			"--root":     {flag: []string{"--root", forest}},
		}},
	}

	cells, refusedCells, readCells := 0, 0, 0
	for _, shape := range shapes {
		for name, one := range shape.cells {
			cells++
			reference := shape.ref
			if name == "--archived" && len(shape.archived) > 0 {
				reference = shape.archived
			}
			argv := append(append([]string{"list"}, reference...), one.flag...)
			got := runCLI(t, root, argv...)
			if one.reads {
				readCells++
				if got.code != 0 {
					t.Errorf("%s reads %s and `dinah %s` exited %d: %s", shape.name, name, strings.Join(argv, " "), got.code, got.errw)
					continue
				}
				if strings.TrimSpace(got.out) == "" {
					t.Errorf("%s reads %s and `dinah %s` printed nothing", shape.name, name, strings.Join(argv, " "))
				}
				continue
			}
			refusedCells++
			if got.code != contract.ExitCode(contract.OutcomeRefused) {
				t.Errorf("%s refuses %s and `dinah %s` exited %d: %s%s", shape.name, name, strings.Join(argv, " "), got.code, got.out, got.errw)
				continue
			}
			if leading := refusalNameOf(got.errw); leading != contract.Usage {
				t.Errorf("%s beside %s refused %s, wanted %s", name, shape.name, leading, contract.Usage)
			}
			if !strings.Contains(got.errw, name+" beside ") {
				t.Errorf("the refusal for %s beside %s names the flag alone rather than the combination: %s", name, shape.name, got.errw)
			}
		}
	}
	if cells != 44 {
		t.Fatalf("the sweep ran %d cells and the table carries eleven reference shapes over four flags", cells)
	}
	if refusedCells == 0 || readCells == 0 {
		t.Fatalf("the sweep read %d cells and refused %d, and a run reaching only one kind proves nothing", readCells, refusedCells)
	}

	// The fifth flag the command declares is not in the table, and this is
	// where it is accounted for: --max-depth bounds --root and refuses with a
	// name of its own when there is no root to bound.
	unbounded := runCLI(t, root, "list", "cards", "--max-depth", "2")
	if unbounded.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Errorf("--max-depth with no root exited %d", unbounded.code)
	}
	if name := refusalNameOf(unbounded.errw); name != contract.DepthWithoutRoot {
		t.Errorf("--max-depth with no root refused %s, wanted %s", name, contract.DepthWithoutRoot)
	}
	t.Logf("%d cells run: %d read, %d refused, plus the one refusal --max-depth carries", cells, readCells, refusedCells)
}

// TestListAnswersEachAdmittedFlagPairOrRefusesItByName is the pair half of the
// sweep above, which runs one cell per flag and so reads no combination at all.
//
// The rulings it pins are dinah-523/decisions/14, that --depth and --ready over
// a workstream narrow one membership rather than two, and
// dinah-523/decisions/15, that a root-scoped question whose answer would be a
// containment walk is refused by name rather than computed per workbench and
// published nowhere. The pairs a reader can actually write are what the rows
// enumerate: a flag the table refuses beside a shape is already refused alone,
// so the pairs worth reading are the ones both cells admit, plus the two the
// workstream ruling of dinah-523/decisions/12 refuses.
//
// A row saying reads asserts an answer rather than an exit code alone, because
// the failure this test exists to catch is a flag admitted and then dropped,
// and a dropped flag exits zero.
func TestListAnswersEachAdmittedFlagPairOrRefusesItByName(t *testing.T) {
	root := listBench(t)
	// The fan-out is rooted at the fixture's own directory rather than at an
	// empty one, so every --root row carries a workbench that answers and a
	// pair whose answer went missing is visible as a row with nothing in it.
	forest := filepath.Dir(root)

	const (
		reads   = "reads"
		refused = "refused"
		// perRow is the fan-out answering while every row carries the
		// refusal that workbench's own list raised, which is what a
		// reference no workbench can answer under the flag looks like.
		perRow = "per row"
	)
	rows := []struct {
		name string
		argv []string
		want string
		// refusal is the name a refused row expects, dinah.usage where the
		// row leaves it empty, because a pair refused for naming a
		// combination is the common case and a pair the resolver refuses
		// on its own grounds says so in its own name.
		refusal string
		detail  string
	}{
		{name: "--depth with --ready over a workstream", argv: []string{"list", "workstream/autumn", "--depth", "all", "--ready"}, want: reads},
		{name: "--ready with --root over a workstream", argv: []string{"list", "workstream/autumn", "--ready", "--root", forest}, want: reads},
		{name: "--depth with --root over a workstream", argv: []string{"list", "workstream/autumn", "--depth", "all", "--root", forest}, want: refused, detail: "--depth beside --root"},
		{name: "--archived with --depth over a workstream", argv: []string{"list", "workstream/spring", "--archived", "--depth", "all"}, want: refused, detail: "--archived beside a workstream reference"},
		{name: "--archived with --ready over a workstream", argv: []string{"list", "workstream/spring", "--archived", "--ready"}, want: refused, detail: "--archived beside a workstream reference"},
		{name: "--archived with --root over a workstream", argv: []string{"list", "workstream/spring", "--archived", "--root", forest}, want: perRow},
		{name: "--depth with --ready over a column", argv: []string{"list", "intake", "--depth", "all", "--ready"}, want: reads},
		{name: "--ready with --root over a column", argv: []string{"list", "intake", "--ready", "--root", forest}, want: reads},
		{name: "--depth with --root over a column", argv: []string{"list", "intake", "--depth", "all", "--root", forest}, want: refused, detail: "--depth beside --root"},
		{name: "--archived with --root over a column", argv: []string{"list", "retired", "--archived", "--root", forest}, want: refused, detail: "--archived beside --root"},
		// --archived alone over the workbench counts the archive half's
		// rosters, and --depth switches it onto the walk, which has no
		// archived workbench to walk from. The resolver says so in its own
		// name, so this row reads dinah.not-archived rather than a usage
		// refusal about the combination.
		{name: "--archived with --depth over the workbench", argv: []string{"list", "workbench", "--archived", "--depth", "all"}, want: refused, refusal: contract.NotArchived, detail: "is never archived"},
		{name: "--archived with --root over the workbench", argv: []string{"list", "workbench", "--archived", "--root", forest}, want: reads},
		{name: "--depth with --root over the workbench", argv: []string{"list", "workbench", "--depth", "all", "--root", forest}, want: refused, detail: "--depth beside --root"},
		{name: "--ready with --root over the roster word cards", argv: []string{"list", "cards", "--ready", "--root", forest}, want: reads},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			got := runCLI(t, root, append([]string{"--lang", "en"}, row.argv...)...)
			switch row.want {
			case refused:
				wanted := row.refusal
				if wanted == "" {
					wanted = contract.Usage
				}
				if got.code != contract.ExitCode(contract.OutcomeRefused) {
					t.Fatalf("`dinah %s` exited %d, and this pair is refused: %s%s", strings.Join(row.argv, " "), got.code, got.out, got.errw)
				}
				if name := refusalNameOf(got.errw); name != wanted {
					t.Errorf("`dinah %s` refused %s, wanted %s", strings.Join(row.argv, " "), name, wanted)
				}
				if !strings.Contains(got.errw, row.detail) {
					t.Errorf("the refusal does not name %q, so it does not say which combination was written:\n%s", row.detail, got.errw)
				}
			case perRow:
				if got.code != 0 {
					t.Fatalf("`dinah %s` exited %d, and the fan-out answers: %s%s", strings.Join(row.argv, " "), got.code, got.out, got.errw)
				}
				if !strings.Contains(got.out, contract.Usage) {
					t.Errorf("no row names the refusal the flag raises inside a workbench:\n%s", got.out)
				}
			default:
				if got.code != 0 {
					t.Fatalf("`dinah %s` exited %d, and this pair reads: %s", strings.Join(row.argv, " "), got.code, got.errw)
				}
				if strings.TrimSpace(got.out) == "" {
					t.Errorf("`dinah %s` printed nothing, so one of the two flags was admitted and dropped", strings.Join(row.argv, " "))
				}
			}
		})
	}
	t.Logf("%d flag pairs run", len(rows))
}

// TestListPublishesTheEnvelopeEachReferenceNames is dinah-523/criteria/29: the
// members --json carries for each reference shape are the members of the
// library call that reference routes to, unchanged.
//
// The members are read off the decoded object rather than compared as bytes,
// because an envelope's identity is which members it carries and a byte
// comparison would also be asserting the order the encoder writes them in.
func TestListPublishesTheEnvelopeEachReferenceNames(t *testing.T) {
	root := listBench(t)
	rows := []struct {
		name    string
		argv    []string
		members []string
		array   bool
	}{
		{name: "bare", argv: []string{"list"}, members: []string{"rosters"}},
		{name: "the workbench", argv: []string{"list", "workbench"}, members: []string{"rosters"}},
		{name: "the workbench, written .", argv: []string{"list", "."}, members: []string{"rosters"}},
		{name: "columns", argv: []string{"list", "columns"}, array: true},
		{name: "cards", argv: []string{"list", "cards"}, members: []string{"query", "cards", "count"}},
		{name: "workstreams", argv: []string{"list", "workstreams"}, members: []string{"workstreams"}},
		{name: "workbenches", argv: []string{"list", "workbenches"}, array: true},
		{name: "attachments", argv: []string{"list", "attachments"}, members: []string{"kind", "ref", "attachments"}},
		{name: "a card's attachments", argv: []string{"list", "fx-1/attachments"}, members: []string{"kind", "ref", "attachments"}},
		{name: "a column", argv: []string{"list", "intake"}, members: []string{"column", "cards"}},
		{name: "a workstream", argv: []string{"list", "workstream/autumn"}, members: []string{"query", "cards", "count"}},
		{name: "a journal", argv: []string{"list", "fx-1/journal"}, array: true},
		{name: "a card", argv: []string{"list", "fx-1"}, members: []string{"producer", "subject", "depth", "root"}},
		{name: "a card under a depth", argv: []string{"list", "fx-1", "--depth", "all"}, members: []string{"producer", "subject", "depth", "root"}},
		{name: "a collection", argv: []string{"list", "fx-1/comments"}, members: []string{"producer", "subject", "depth", "root"}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			got := mustRun(t, root, append(append([]string{}, row.argv...), "--json")...)
			if row.array {
				var listed []any
				if err := json.Unmarshal([]byte(got.out), &listed); err != nil {
					t.Fatalf("the answer is not a bare array: %v\n%s", err, got.out)
				}
				if len(listed) == 0 {
					t.Errorf("the answer is an empty array, so this row asserts nothing about its members:\n%s", got.out)
				}
				return
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(got.out), &payload); err != nil {
				t.Fatalf("the answer will not parse as an object: %v\n%s", err, got.out)
			}
			for _, member := range row.members {
				if _, carried := payload[member]; !carried {
					t.Errorf("the envelope carries no %q member: %s", member, got.out)
				}
			}
			for member := range payload {
				if !carries(row.members, member) {
					t.Errorf("the envelope carries the member %q, which the table does not name for this reference: %s", member, got.out)
				}
			}
		})
	}

	// The cards roster word and a bare query answer one table through one
	// library call, which is what keeps the two from drifting.
	if listed, queried := mustRun(t, root, "list", "cards").out, mustRun(t, root, "query").out; listed != queried {
		t.Errorf("`dinah list cards` prints\n%s\nand `dinah query` prints\n%s", listed, queried)
	}
	if listed, queried := mustRun(t, root, "list", "cards", "--ready").out, mustRun(t, root, "query", "state:ready").out; listed != queried {
		t.Errorf("`dinah list cards --ready` prints\n%s\nand `dinah query state:ready` prints\n%s", listed, queried)
	}
}

// carries reports whether a member is one the row names.
func carries(members []string, name string) bool {
	for _, member := range members {
		if member == name {
			return true
		}
	}
	return false
}

// TestARosterWordOutranksASlugSpelledLikeOne is dinah-523/criteria/28. A
// workbench mints its own column slugs and nothing stops one being spelled
// `cards`, so the command's own grammar has to win over the data it reads.
func TestARosterWordOutranksASlugSpelledLikeOne(t *testing.T) {
	root := newBench(t)
	mustRun(t, root, "column", "new", "Cards", "--kind", "work", "--slug", "cards")
	mustRun(t, root, "add", "a card the roster word answers with")

	roster := mustRun(t, root, "list", "cards", "--json").out
	var matches verb.Matches
	if err := json.Unmarshal([]byte(roster), &matches); err != nil {
		t.Fatalf("the roster word answered something that is not the query envelope: %v\n%s", err, roster)
	}
	if len(matches.Cards) != 1 {
		t.Errorf("the roster word answered %d cards, wanted the one the workbench carries", len(matches.Cards))
	}

	// The column stays reachable from show by its slug, because show takes no
	// roster word at all, and from list by its identifier and by its name.
	shown := mustRun(t, root, "show", "cards").out
	if !strings.Contains(shown, "Cards") && !strings.Contains(strings.ToLower(shown), "cards") {
		t.Errorf("`dinah show cards` did not answer the column: %s", shown)
	}
	byName := runCLI(t, root, "list", "Cards")
	if byName.code != 0 {
		t.Errorf("the column slugged like a roster word is unreachable from list by its name: %d %s", byName.code, byName.errw)
	}
	identifier := listedColumnIdentifier(t, root, "cards")
	byID := runCLI(t, root, "list", identifier)
	if byID.code != 0 {
		t.Errorf("the column slugged like a roster word is unreachable from list by its identifier: %d %s", byID.code, byID.errw)
	}
}

// listedColumnIdentifier reads one column's identifier off the columns
// listing.
func listedColumnIdentifier(t *testing.T, root, slug string) string {
	t.Helper()
	var columns []verb.ColumnView
	if err := json.Unmarshal([]byte(mustRun(t, root, "list", "columns", "--json").out), &columns); err != nil {
		t.Fatalf("the columns listing will not parse: %v", err)
	}
	for _, column := range columns {
		if column.Slug == slug {
			return column.ID
		}
	}
	t.Fatalf("the workbench declares no column slugged %q", slug)
	return ""
}

// TestTheDepthLadderHasFiveRungsAndMembersIsTheDefault is the depth half of
// dinah-523/criteria/11: the rung a reference draws with no --depth, and the
// refusal an unknown level raises.
func TestTheDepthLadderHasFiveRungsAndMembersIsTheDefault(t *testing.T) {
	root := listBench(t)

	// members is relative to the reference, which is what makes it the only
	// rung that can be the default. A walk rooted at a below-card reference
	// sits at rank two, which no absolute rung can cut one level below.
	below := mustRun(t, root, "list", "fx-1/comments/1", "--json").out
	var walk verb.Tree
	if err := json.Unmarshal([]byte(below), &walk); err != nil {
		t.Fatalf("the walk will not parse: %v\n%s", err, below)
	}
	if walk.Depth != verb.LevelMembers {
		t.Errorf("the default rung reads %q, wanted %s", walk.Depth, verb.LevelMembers)
	}

	unknown := runCLI(t, root, "list", "fx-1", "--depth", "nosuchlevel")
	if unknown.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("an unknown level exited %d", unknown.code)
	}
	if name := refusalNameOf(unknown.errw); name != contract.UnknownDepth {
		t.Errorf("an unknown level refused %s, wanted %s", name, contract.UnknownDepth)
	}
	if want := strings.Join(verb.ListLevels, ", "); !strings.Contains(unknown.errw, want) {
		t.Errorf("the refusal lists %q and the ladder is %q", unknown.errw, want)
	}
	if got, want := strings.Join(verb.ListLevels, " "), "root members cards entities all"; got != want {
		t.Errorf("the ladder reads [%s], wanted [%s]", got, want)
	}
}
