package main

import (
	"encoding/json"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// The grounds a table may be exempted from the address sweep on. The set is
// closed: an exemption carrying anything else fails the run rather than
// passing unread, so a ground is an argument a reviewer weighs rather than a
// word anybody can invent.
const (
	// groundNoEntity is a table that draws no entity of a workbench:
	// commands, flags, settings, catalogues and the guide topics.
	groundNoEntity = "no-entity"
	// groundActNotEntity is a table whose rows are recorded acts rather than
	// entities, so a row's subject is an event and the entity it names is the
	// one the caller asked about.
	groundActNotEntity = "act-not-entity"
	// groundNamedByCaller is a table of one entity's own stored fields,
	// reached by naming that entity, so the reader already holds its address.
	groundNamedByCaller = "named-by-caller"
	// groundOutsideWorkbench is a table whose rows are workbenches rather than
	// entities within one, addressed by the filesystem path the row prints.
	groundOutsideWorkbench = "outside-workbench"
)

// addressExemption is one table site the address rule does not reach, with the
// ground it is excused on and the reasoning behind it.
type addressExemption struct {
	site   renderSite
	ground string
	reason string
}

// addressExemptions are the table sites that draw no addressable entity. Each
// was examined rather than assumed, and dinah-454 section 9 settled the
// reasoning.
var addressExemptions = []addressExemption{
	{
		site:   renderSite{File: "commands.go", Function: "runGuide", Label: "topics", Ordinal: 1},
		ground: groundNoEntity, reason: "the rows are guide topics, which are documents rather than entities of a workbench",
	},
	{
		site:   renderSite{File: "help.go", Function: "argumentLines", Label: "arguments", Ordinal: 1},
		ground: groundNoEntity, reason: "the rows are one command's arguments",
	},
	{
		site:   renderSite{File: "help.go", Function: "helpBlock", Label: "commandListing", Ordinal: 1},
		ground: groundNoEntity, reason: "the rows are commands",
	},
	{
		site:   renderSite{File: "help.go", Function: "helpBlock", Label: "flags", Ordinal: 1},
		ground: groundNoEntity, reason: "the rows are flags",
	},
	{
		site:   renderSite{File: "help.go", Function: "verbHelp", Label: "preconditions", Ordinal: 1},
		ground: groundNoEntity, reason: "the rows are the checks a verb runs and the refusals they raise",
	},
	{
		site:   renderSite{File: "render.go", Function: "formatCandidateRows", Label: "t", Ordinal: 1},
		ground: groundOutsideWorkbench, reason: "the rows are workbenches under a directory, addressed by the path the row already prints and passed back through --workbench",
	},
	{
		site:   renderSite{File: "render.go", Function: "renderCheck", Label: "assigned", Ordinal: 1},
		ground: groundActNotEntity, reason: "the rows are the column slugs one repair assigned",
	},
	{
		site:   renderSite{File: "render.go", Function: "renderCheck", Label: "workstreams", Ordinal: 1},
		ground: groundActNotEntity, reason: "the rows are the workstream slugs one repair assigned",
	},
	{
		site:   renderSite{File: "render.go", Function: "renderCheck", Label: "removed", Ordinal: 1},
		ground: groundActNotEntity, reason: "the rows name what one repair removed, and a column the repair has just taken away has no address left to draw",
	},
	{
		site:   renderSite{File: "render.go", Function: "renderCheck", Label: "witnessed", Ordinal: 1},
		ground: groundActNotEntity, reason: "the rows name what one repair witnessed",
	},
	{
		site:   renderSite{File: "render.go", Function: "renderFindings", Label: "t", Ordinal: 1},
		ground: groundActNotEntity, reason: "a finding names the file a defect was found in, and several finding kinds name a file that by construction has no reference at all, a missing anchor among them",
	},
	{
		site:   renderSite{File: "render.go", Function: "renderHistory", Label: "t", Ordinal: 1},
		ground: groundActNotEntity, reason: "the rows are one card's recorded acts, and the caller named that card to reach the log",
	},
	{
		site:   renderSite{File: "render.go", Function: "renderSettings", Label: "t", Ordinal: 1},
		ground: groundNoEntity, reason: "the rows are configuration keys",
	},
	{
		site:   renderSite{File: "render.go", Function: "renderVersion", Label: "t", Ordinal: 1},
		ground: groundNoEntity, reason: "the rows are the languages this build ships",
	},
	{
		site:   renderSite{File: "render.go", Function: "renderWorkbenchFields", Label: "t", Ordinal: 1},
		ground: groundNamedByCaller, reason: "the table lists the workbench's own stored fields, and the reader named the workbench to reach it",
	},
}

// addressExpectation is what one drawn row's reference cell must hold, and
// whether that text is an address at all.
type addressExpectation struct {
	// text is what the cell must carry, read off the machine payload the same
	// command answers with rather than composed here.
	text string
	// resolves says the text must round-trip through `dinah path`. A grouping
	// row of the containment tree carries the value the group gathered at,
	// which nothing addresses, so such a row is compared and not resolved.
	resolves bool
	// skip says the row draws no entity at all, so its cell is neither
	// compared nor resolved. A column with nothing ready draws a sentence
	// where `dinah next` would draw a card, and that is the only case.
	skip bool
}

// addressCase is one drawn block, the cell of it that carries a reference, and
// where the expectation for that cell comes from.
type addressCase struct {
	// site is the table call site this case drives, which is the key
	// TestEveryEntityDrawingSurfaceIsSwept counts coverage by.
	site renderSite
	// label names the case in a failure.
	label string
	// argv is the command drawing the block.
	argv []string
	// stderr reads the block off standard error rather than standard output,
	// which is where a refusal draws its listing.
	stderr bool
	// opensAt is the catalog key of the sentence the block is drawn under, for
	// a block that draws no heading row of its own. A case leaving it empty
	// finds its block by the heading row the site's own columns declare.
	opensAt string
	// headingless says the block draws neither a heading row nor an opening
	// sentence this test can name, which is the shape a refusal's listing
	// takes: every indented line of the answer is a row of it.
	headingless bool
	// at is the index of the field carrying the reference, counting the
	// block's own columns from zero.
	at int
	// guided says the block is a tree, whose first field carries the drawing
	// prefix ahead of the reference.
	guided bool
	// wantHeadings are the catalog keys the block's heading row must carry,
	// exactly and in this order. It is what fails a column added beside the
	// reference rather than in place of what stood there, which the location
	// rule above cannot see, since that reads the site's own declaration and
	// so agrees with whatever the source says.
	wantHeadings []string
	// want is the references the same command's machine payload carries, in
	// the order the rows are drawn.
	want func(t *testing.T, w *addressWorkbench) []addressExpectation
	// companion is a further act one drawn cell has to survive, run once per
	// row. It is where a case asserts that the spelling it prints reaches a
	// command the round trip does not go through.
	companion func(t *testing.T, w *addressWorkbench, cell string)
}

// TestEveryPrintedReferenceResolves asserts dinah-454 AC-1, AC-2, AC-6 and
// AC-11: every reference this head prints is one a reader can type back.
//
// Each case reads the reference cell out of a rendered block and asserts two
// things about it. The round trip hands the exact text to `dinah path` and
// requires exit 0 and a file that is there, which fails a reference that is
// misspelled, composed against the wrong parent, numbered from zero, or simply
// absent, since an empty argument is refused. The identity assertion requires
// the cell to hold what the same command's own machine payload carries for
// that row.
//
// Both arms are needed and neither is enough on its own. The round trip is
// satisfied by any reference that resolves, so a renderer printing the card's
// own address on every attachment row would pass it. The identity assertion
// compares the head against the payload, so it would pass if both were wrong
// together.
func TestEveryPrintedReferenceResolves(t *testing.T) {
	w := newAddressWorkbench(t)
	for _, drawn := range addressCases() {
		t.Run(drawn.label, func(t *testing.T) {
			cells := w.cells(t, drawn)
			want := drawn.want(t, w)
			if len(cells) == 0 {
				t.Fatalf("%s drew no row at all, so this case asserts nothing", drawn.label)
			}
			if len(cells) != len(want) {
				t.Fatalf("%s drew %d rows and the payload carries %d entries:\ndrawn %q\nwanted %v",
					drawn.label, len(cells), len(want), cells, want)
			}
			for i, cell := range cells {
				if want[i].skip {
					continue
				}
				if cell != want[i].text {
					t.Errorf("%s row %d draws %q, and the reference for that row is %q", drawn.label, i+1, cell, want[i].text)
					continue
				}
				if !want[i].resolves {
					continue
				}
				located := runCLI(t, w.root, "path", cell)
				if located.code != 0 {
					t.Errorf("%s row %d draws %q, which `dinah path` refuses: %s", drawn.label, i+1, cell, strings.TrimSpace(located.errw))
					continue
				}
				if _, err := os.Stat(strings.TrimSpace(located.out)); err != nil {
					t.Errorf("%s row %d draws %q, which resolves to %q and no file is there: %v",
						drawn.label, i+1, cell, strings.TrimSpace(located.out), err)
				}
				if drawn.companion != nil {
					drawn.companion(t, w, cell)
				}
			}
		})
	}
}

// TestEveryEntityDrawingSurfaceIsSwept asserts dinah-454 AC-8: no table this
// head draws reaches a green build without being swept or argued.
//
// The roster is renderSitesInSource, the walk this package already declares
// over every s.table and s.tableLines call outside table.go and which
// TestEveryTableSiteIsRegistered already holds in both directions. Reading it
// rather than parsing again is what closes the hole a search for s.columns
// alone leaves: five tables here are built through listColumn and carry no
// such call.
//
// Say plainly what this does not prove. A ground is a declaration a reviewer
// weighs rather than a fact the machine checks, and a site drawing more than
// one collection is proven only for the one its case drives.
func TestEveryEntityDrawingSurfaceIsSwept(t *testing.T) {
	sites, err := renderSitesInSource()
	if err != nil {
		t.Fatalf("walk the head's sources: %v", err)
	}
	swept := map[renderSite]bool{}
	for _, drawn := range addressCases() {
		if _, found := sites[drawn.site]; !found {
			t.Errorf("the case %q drives %s and the source draws no table there, so the case is stale", drawn.label, drawn.site)
		}
		swept[drawn.site] = true
	}
	excused := map[renderSite]addressExemption{}
	for _, exemption := range addressExemptions {
		switch exemption.ground {
		case groundNoEntity, groundActNotEntity, groundNamedByCaller, groundOutsideWorkbench:
		default:
			t.Errorf("%s is excused on the ground %q, which is outside the declared set", exemption.site, exemption.ground)
		}
		if _, found := sites[exemption.site]; !found {
			t.Errorf("%s is excused and the source draws no table there, so the exemption is stale", exemption.site)
		}
		if exemption.reason == "" {
			t.Errorf("%s is excused with no reason, and a ground with no reasoning behind it is a word rather than an argument", exemption.site)
		}
		excused[exemption.site] = exemption
	}
	var unargued []string
	for site := range sites {
		if swept[site] {
			continue
		}
		if _, ok := excused[site]; ok {
			continue
		}
		unargued = append(unargued, site.String())
	}
	sort.Strings(unargued)
	for _, site := range unargued {
		t.Errorf("%s draws a table and is neither a case of TestEveryPrintedReferenceResolves nor a declared exemption", site)
	}
	assertTheHeadDeclaresTwoTableColumnConstructors(t)
}

// assertTheHeadDeclaresTwoTableColumnConstructors requires this package to
// declare exactly the two functions returning []tableColumn that
// renderSitesInSource knows how to read, which are (*session).columns and
// listColumn.
//
// A site whose columns come from anywhere else is reported by that walk as
// unresolvable rather than as swept, so a third constructor would quietly
// shrink the roster this guard rests on. Failing here names the constructor
// instead, which tells the next author that the guard needs teaching.
func assertTheHeadDeclaresTwoTableColumnConstructors(t *testing.T) {
	t.Helper()
	_, files, err := parseTheRenderingHead()
	if err != nil {
		t.Fatalf("parse the head's sources: %v", err)
	}
	var found []string
	for _, file := range files {
		for _, declared := range file.Decls {
			function, ok := declared.(*ast.FuncDecl)
			if !ok || function.Type.Results == nil || len(function.Type.Results.List) != 1 {
				continue
			}
			slice, ok := function.Type.Results.List[0].Type.(*ast.ArrayType)
			if !ok {
				continue
			}
			name, ok := slice.Elt.(*ast.Ident)
			if !ok || name.Name != "tableColumn" {
				continue
			}
			found = append(found, function.Name.Name)
		}
	}
	sort.Strings(found)
	want := []string{"columns", "listColumn"}
	if strings.Join(found, ",") != strings.Join(want, ",") {
		t.Errorf("the head declares %v returning []tableColumn, wanted %v; renderSitesInSource reads a columns field only where it calls one of those two, so a third constructor drops sites out of the roster this sweep rests on",
			found, want)
	}
}

// addressDefinition is the flow the sweep runs against. It carries two work
// columns rather than the one `dinah init` lays down, because a bare pull
// finding more than one column it could land in is what draws the carried
// listing a refusal prints, and that listing is one of the sites this sweep
// covers.
const addressDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Address fixture",
  "columns": [
    { "id": "a00000000001", "title": "Intake", "kind": "intake" },
    { "id": "a00000000002", "title": "Doing", "kind": "work" },
    { "id": "a00000000003", "title": "Review", "kind": "work" },
    { "id": "a00000000004", "title": "Done", "kind": "done" }
  ]
}`

// addressWorkbench is the one workbench every case is drawn from, holding one
// of every kind the address grammar reaches.
type addressWorkbench struct {
	root string
	// cursor is a checkpoint minted before the acts below, which is what the
	// changes case walks from.
	cursor string
}

// newAddressWorkbench builds that workbench. It reshapes the flow first, then
// files the cards, the comments, the attachments, the checklist items, the
// links and the workstream, and leaves one card held and one card blocked so
// that the status blocks draw rows.
func newAddressWorkbench(t *testing.T) *addressWorkbench {
	t.Helper()
	t.Setenv("COLUMNS", "200")
	root := newBench(t)
	definition := filepath.Join(t.TempDir(), "flow.json")
	if err := os.WriteFile(definition, []byte(addressDefinition), 0o644); err != nil {
		t.Fatalf("write the flow definition: %v", err)
	}
	mustRun(t, root, "reshape", "--from", definition, "--yes")
	for _, title := range []string{"a card with things below it", "a second card", "a third card", "a fourth card about haystack prose", "a fifth card"} {
		mustRun(t, root, "add", title)
	}
	mustRun(t, root, "workstream", "new", "Autumn release", "--slug", "autumn")
	mustRun(t, root, "join", "fx-1", "autumn")
	mustRun(t, root, "join", "fx-2", "autumn")
	mustRun(t, root, "comment", "fx-1", "a first thought")
	mustRun(t, root, "comment", "fx-1", "a second thought")
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write the attachment source: %v", err)
	}
	mustRun(t, root, "attach", "fx-1", source, "--description", "the first file")
	second := filepath.Join(t.TempDir(), "evidence.txt")
	if err := os.WriteFile(second, []byte("more bytes"), 0o644); err != nil {
		t.Fatalf("write the second attachment source: %v", err)
	}
	mustRun(t, root, "attach", "fx-1", second, "--description", "the second file")
	mustRun(t, root, "file", "fx-1", "open_question", "a question somebody has to answer")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "a criterion somebody has to verify")
	// The question is answered rather than left pending, because a card
	// carrying an unresolved question refuses a claim and this fixture goes on
	// to hold one. The item stays on the card, which is what the checklist
	// block draws.
	mustRun(t, root, "resolve", "fx-1/oq/1", "the fixture answers it so the card can be claimed")
	writeAddressLinks(t, root, "fx-1")
	writeAddressHaystack(t, root)

	w := &addressWorkbench{root: root}
	w.cursor = mintedCursor(t, root)
	mustRun(t, root, "move", "fx-1", "doing")
	mustRun(t, root, "claim", "fx-1")
	mustRun(t, root, "move", "fx-2", "doing")
	mustRun(t, root, "claim", "fx-2")
	mustRun(t, root, "block", "fx-2", "the vendor has not answered")
	// The fifth card stands ready in the second column, which is what makes a
	// bare pull ambiguous: the first work column could take a card from
	// intake and the second could take this one, so the refusal carries both
	// and draws the listing one of the cases below reads.
	mustRun(t, root, "move", "fx-5", "doing")
	return w
}

// mustRun runs one command and fails the test unless it succeeded.
func mustRun(t *testing.T, root string, argv ...string) invocation {
	t.Helper()
	got := runCLI(t, root, argv...)
	if got.code != 0 {
		t.Fatalf("%v: %d %s", argv, got.code, got.errw)
	}
	return got
}

// writeAddressLinks plants a links sequence in a card's frontmatter, since no
// command creates a link and the reader reads one.
func writeAddressLinks(t *testing.T, root, ref string) {
	t.Helper()
	located := mustRun(t, root, "path", ref)
	path := strings.TrimSpace(located.out)
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	source := string(text)
	cut := strings.Index(source[4:], "\n---\n")
	if cut < 0 {
		t.Fatalf("%s carries no frontmatter to plant a link in", path)
	}
	links := "links:\n  - kind: blocks\n    to: fx-3\n  - kind: relates-to\n    to: fx-4\n"
	rewritten := source[:cut+4] + "\n" + links + source[cut+5:]
	if err := os.WriteFile(path, []byte(rewritten), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeAddressHaystack plants one phrase in the framing of a column, a
// workstream and the workbench itself, which is what makes the search block
// draw a row of more than one kind. The card carries the same phrase in the
// title it was filed under.
func writeAddressHaystack(t *testing.T, root string) {
	t.Helper()
	appendBody(t, root, locate(t, root, "review"), "haystack prose")
	appendBody(t, root, locate(t, root, "workbench"), "haystack prose")
	appendBody(t, root, filepath.Join(locate(t, root, "workstream/autumn"), bench.WorkstreamAnchor), "haystack prose")
}

// locate is the absolute path one reference names, which is what `dinah path`
// answers and what lets this fixture write framing text no command sets.
func locate(t *testing.T, root, ref string) string {
	t.Helper()
	return strings.TrimSpace(mustRun(t, root, "path", ref).out)
}

// appendBody adds a line to the end of one anchor's body, which is the framing
// text a search runs over.
func appendBody(t *testing.T, root, path, text string) {
	t.Helper()
	existing, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(existing) + "\n" + text + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// cells reads the reference cell out of every row of one case's block.
func (w *addressWorkbench) cells(t *testing.T, drawn addressCase) []string {
	t.Helper()
	argv := make([]string, 0, len(drawn.argv))
	for _, word := range drawn.argv {
		if word == addressCursorToken {
			word = w.cursor
		}
		argv = append(argv, word)
	}
	got := runCLI(t, w.root, argv...)
	out := got.out
	if drawn.stderr {
		out = got.errw
	}
	lines := w.blockLines(t, drawn, out)
	cells := make([]string, 0, len(lines))
	for i, line := range lines {
		fields := line
		if drawn.guided {
			lead, prefix, ok := guidedLead(line)
			if !ok {
				t.Errorf("%s row %d draws %q, which carries no guide at all, so its first field cannot be read as a reference",
					drawn.label, i+1, line)
				continue
			}
			fields = line[lead+len(prefix):]
		}
		kept := addressFields(fields)
		if drawn.at >= len(kept) {
			t.Errorf("%s row %d draws %q, which carries %d fields and the reference is declared at index %d",
				drawn.label, i+1, line, len(kept), drawn.at)
			continue
		}
		cells = append(cells, kept[drawn.at])
	}
	return cells
}

// blockLines is the rows of one case's block, found either under the sentence
// the block opens at or under the heading row the site's own columns declare.
//
// A note line beneath a row is skipped rather than read as a row of its own: a
// comment's body is drawn unindented under the row it belongs to, and the
// block is what carries the indent.
func (w *addressWorkbench) blockLines(t *testing.T, drawn addressCase, out string) []string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	start := -1
	if drawn.opensAt != "" {
		opener := msg.For(msg.Base).T(drawn.opensAt)
		for i, line := range lines {
			if line == opener {
				start = i + 1
				break
			}
		}
		if start < 0 {
			t.Fatalf("%s drew no line reading %q, so its block was never found:\n%s", drawn.label, opener, out)
		}
	} else if drawn.headingless {
		start = 0
	} else {
		heading := w.headingOf(t, drawn)
		for i, line := range lines {
			if !addressHeadingMatches(addressFields(line), heading) {
				continue
			}
			if len(drawn.wantHeadings) > 0 {
				texts := make([]string, 0, len(drawn.wantHeadings))
				for _, key := range drawn.wantHeadings {
					texts = append(texts, msg.For(msg.Base).T(key))
				}
				if strings.Join(addressFields(line), "|") != strings.Join(texts, "|") {
					t.Errorf("%s draws the headings %q, wanted exactly %q, so a column has been added beside the reference rather than in place of what stood there",
						drawn.label, addressFields(line), texts)
				}
			}
			start = i + 2
			break
		}
		if start < 0 {
			t.Fatalf("%s drew no heading row carrying the columns %q, so its block was never found:\n%s", drawn.label, heading, out)
		}
	}
	var rows []string
	for _, line := range lines[start:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		rows = append(rows, line)
	}
	return rows
}

// headingOf is the heading row one case's block draws, composed from the
// catalog texts of the columns the site's own source declares. Reading the
// keys off the site rather than typing them here is what makes a renamed key
// fail loudly instead of leaving the block unfound.
func (w *addressWorkbench) headingOf(t *testing.T, drawn addressCase) []string {
	t.Helper()
	sites, err := renderSitesInSource()
	if err != nil {
		t.Fatalf("walk the head's sources: %v", err)
	}
	info, ok := sites[drawn.site]
	if !ok || len(info.Keys) == 0 {
		t.Fatalf("%s drives %s, which declares no columns the walk can read, so a case there needs an opensAt", drawn.label, drawn.site)
	}
	texts := make([]string, 0, len(info.Keys))
	for _, key := range info.Keys {
		texts = append(texts, msg.For(msg.Base).T(key))
	}
	return texts
}

// addressHeadingMatches reports whether one drawn line is the heading row of a
// block declaring these columns.
//
// The drawn headings are matched as a subsequence of the declared ones rather
// than against all of them, because the renderer drops a column no row of the
// block carries a value in, and it drops one from the middle as readily as
// from the end: a listing whose cards carry neither severity nor priority
// draws three headings where the site declares five.
func addressHeadingMatches(drawn, declared []string) bool {
	if len(drawn) < 2 {
		return false
	}
	at := 0
	for _, field := range drawn {
		for at < len(declared) && declared[at] != field {
			at++
		}
		if at == len(declared) {
			return false
		}
		at++
	}
	return true
}

// addressFields splits one drawn line into its fields. Every column but the
// last is padded to a fixed width and separated by the gutter, so a run of two
// or more spaces is where one field ends and the next begins.
func addressFields(line string) []string {
	var kept []string
	for _, field := range strings.Split(strings.TrimSpace(line), "  ") {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return kept
}

// payload decodes the machine answer to one command as an object.
func (w *addressWorkbench) payload(t *testing.T, argv ...string) map[string]any {
	t.Helper()
	got := mustRun(t, w.root, append([]string{"--json"}, argv...)...)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(got.out), &decoded); err != nil {
		t.Fatalf("decode the machine answer to %v: %v\n%s", argv, err, got.out)
	}
	return decoded
}

// refsOf is the value one field carries on every member of one array of a
// payload, which is the identity side of a case whose rows are one collection.
func refsOf(t *testing.T, payload map[string]any, key, field string) []addressExpectation {
	t.Helper()
	members, ok := payload[key].([]any)
	if !ok {
		t.Fatalf("the payload carries no array under %q: %v", key, payload)
	}
	return refsOfArray(t, members, field)
}

// refsOfArray is refsOf for a command whose whole machine answer is the array.
func refsOfArray(t *testing.T, members []any, field string) []addressExpectation {
	t.Helper()
	var want []addressExpectation
	for _, member := range members {
		entry, ok := member.(map[string]any)
		if !ok {
			t.Fatalf("a member of the answer is not an object: %v", member)
		}
		text, _ := entry[field].(string)
		want = append(want, addressExpectation{text: text, resolves: true})
	}
	return want
}

// addressCursorToken stands in an argument list for the checkpoint the fixture
// minted, which is not known until the workbench is built.
const addressCursorToken = "<cursor>"

// array decodes the machine answer to one command as an array, which is the
// shape `dinah next` answers in.
func (w *addressWorkbench) array(t *testing.T, argv ...string) []any {
	t.Helper()
	got := mustRun(t, w.root, append([]string{"--json"}, argv...)...)
	var decoded []any
	if err := json.Unmarshal([]byte(got.out), &decoded); err != nil {
		t.Fatalf("decode the machine answer to %v: %v; %s", argv, err, got.out)
	}
	return decoded
}

// addressCases is every block this sweep drives, one entry per drawn table.
// Each entry names the table site it drives, so the completeness guard counts
// what is covered rather than inferring it from the command.
//
// One site can appear twice. The attachments block is drawn by `dinah show`
// and by `dinah attachments` through one renderer, and both are driven here so
// that neither command can drift from the other.
func addressCases() []addressCase {
	show := renderSite{File: "render.go", Function: "renderDetail", Label: "comments", Ordinal: 1}
	attachments := renderSite{File: "render.go", Function: "renderAttachments", Label: "attachments", Ordinal: 1}
	tree := renderSite{File: "render.go", Function: "renderTree", Label: "t", Ordinal: 1}
	return []addressCase{
		{
			site: show, label: "show, comments block",
			argv: []string{"show", "fx-1"}, at: 0,
			wantHeadings: []string{"column.comments.ref", "column.comments.when", "column.comments.who"},
			// The expectation is composed here rather than read off the
			// payload, and that is the one case in this sweep where it has to
			// be. Both heads fill a comment's reference from one composer, so
			// a payload-derived expectation moves with the code under test and
			// a wrongly composed reference would pass against itself. The
			// fixture writes its two comments in order and deletes neither, so
			// their positions in the collection are 1 and 2.
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return []addressExpectation{
					{text: "fx-1/" + bench.CommentsDir + "/1", resolves: true},
					{text: "fx-1/" + bench.CommentsDir + "/2", resolves: true},
				}
			},
		},
		{
			site: attachments, label: "show, attachments block",
			argv: []string{"show", "fx-1"}, at: 0,
			wantHeadings: []string{"column.attachments.ref", "column.attachments.filename", "column.attachments.description"},
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "show", "fx-1"), "attachments", "ref")
			},
		},
		{
			site: attachments, label: "attachments command",
			argv: []string{"attachments", "fx-1"}, at: 0,
			wantHeadings: []string{"column.attachments.ref", "column.attachments.filename", "column.attachments.description"},
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "attachments", "fx-1"), "attachments", "ref")
			},
		},
		{
			// The checklist block draws no heading row of its own, so it is
			// found under the sentence it opens at rather than by a heading.
			site:  renderSite{File: "render.go", Function: "renderDetail", Label: "checklist", Ordinal: 1},
			label: "show, checklist block",
			argv:  []string{"show", "fx-1"}, opensAt: "show.checklist", at: 0,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "show", "fx-1"), "checklist", "ref")
			},
		},
		{
			site:  renderSite{File: "render.go", Function: "renderDetail", Label: "links", Ordinal: 1},
			label: "show, links block",
			argv:  []string{"show", "fx-1"}, at: 1,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "show", "fx-1"), "links", "ref")
			},
		},
		{
			site: tree, label: "contents, containment table",
			argv: []string{"contents", "fx-1"}, at: 0, guided: true,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return treeExpectations(t, w.payload(t, "contents", "fx-1"))
			},
		},
		{
			site: tree, label: "tree, grouped table",
			argv: []string{"tree"}, at: 0, guided: true,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return treeExpectations(t, w.payload(t, "tree"))
			},
		},
		{
			site:  renderSite{File: "render.go", Function: "renderListing", Label: "t", Ordinal: 1},
			label: "ls",
			argv:  []string{"ls", "intake"}, at: 0,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "ls", "intake"), "cards", "ref")
			},
		},
		{
			site:  renderSite{File: "render.go", Function: "renderMatches", Label: "t", Ordinal: 1},
			label: "query",
			argv:  []string{"query", "column:doing"}, at: 0,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "query", "column:doing"), "cards", "ref")
			},
		},
		{
			// A column with nothing ready draws a sentence where the card
			// would be, and that cell names no entity, so it is skipped by
			// declaration rather than compared against a sentence.
			site:  renderSite{File: "render.go", Function: "renderOffers", Label: "t", Ordinal: 1},
			label: "next",
			argv:  []string{"next"}, at: 1,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				var want []addressExpectation
				for _, member := range w.array(t, "next") {
					entry, ok := member.(map[string]any)
					if !ok {
						t.Fatalf("an offer is not an object: %v", member)
					}
					card, ok := entry["card"].(map[string]any)
					if !ok {
						want = append(want, addressExpectation{skip: true})
						continue
					}
					ref, _ := card["ref"].(string)
					want = append(want, addressExpectation{text: ref, resolves: true})
				}
				return want
			},
		},
		{
			site:  renderSite{File: "render.go", Function: "renderColumns", Label: "t", Ordinal: 1},
			label: "columns",
			argv:  []string{"columns"}, at: 0,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOfArray(t, w.array(t, "columns"), "slug")
			},
		},
		{
			// The legal-moves table draws a destination column's own
			// reference in its first cell, which is an entity of the
			// workbench rather than decoration.
			site:  renderSite{File: "render.go", Function: "renderInstructions", Label: "t", Ordinal: 1},
			label: "the legal moves served for a card",
			argv:  []string{"instructions", "fx-1"}, at: 0,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "instructions", "fx-1"), "legal_moves", "ref")
			},
		},
		{
			// A refusal naming no column of this workbench lists the columns
			// it does declare, one reference to a row, through listColumn.
			site:  renderSite{File: "render.go", Function: "composeRefusal", Label: "t", Ordinal: 1},
			label: "the columns a refusal lists",
			argv:  []string{"ls", "nosuch"}, stderr: true, headingless: true, at: 0,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOfArray(t, w.array(t, "columns"), "slug")
			},
		},
		{
			// A bare pull finding more than one column it could land in
			// carries those columns on the refusal, and they are drawn by a
			// table of their own.
			site:  renderSite{File: "render.go", Function: "composeRefusal", Label: "carriedTable", Ordinal: 1},
			label: "the columns an ambiguous pull carries",
			argv:  []string{"pull"}, stderr: true, headingless: true, at: 0,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return []addressExpectation{
					{text: "doing", resolves: true},
					{text: "review", resolves: true},
				}
			},
		},
		{
			site:  renderSite{File: "render.go", Function: "renderSearch", Label: "t", Ordinal: 1},
			label: "search",
			argv:  []string{"search", "haystack"}, at: 1,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "search", "haystack"), "hits", "ref")
			},
		},
		{
			site:  renderSite{File: "render.go", Function: "renderWorkstreams", Label: "t", Ordinal: 1},
			label: "workstream listing",
			argv:  []string{"workstream"}, at: 0,
			wantHeadings: []string{"column.workstreams.reference", "column.workstreams.name", "column.workstreams.status", "column.workstreams.cards"},
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "workstream"), "workstreams", "ref")
			},
			// Printing a spelling the general reference commands take is only
			// half of it. The commands that take a workstream and nothing else
			// have to take the same spelling, or the listing sends a reader to
			// a refusal.
			companion: func(t *testing.T, w *addressWorkbench, cell string) {
				if got := runCLI(t, w.root, "get", cell, "title"); got.code != 0 {
					t.Errorf("the listing prints %q and `dinah get` refuses it: %s", cell, strings.TrimSpace(got.errw))
				}
			},
		},
		{
			site:  renderSite{File: "render.go", Function: "renderStatus", Label: "held", Ordinal: 1},
			label: "status, the cards you hold",
			argv:  []string{"status"}, at: 0,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "status"), "holding", "ref")
			},
		},
		{
			site:  renderSite{File: "render.go", Function: "renderStatus", Label: "blocked", Ordinal: 1},
			label: "status, the blocked cards",
			argv:  []string{"status"}, at: 0,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "status"), "blocked", "ref")
			},
		},
		{
			site:  renderSite{File: "render.go", Function: "renderChangesBody", Label: "t", Ordinal: 1},
			label: "changes",
			argv:  []string{"changes", "--since", addressCursorToken}, at: 1,
			want: func(t *testing.T, w *addressWorkbench) []addressExpectation {
				return refsOf(t, w.payload(t, "changes", "--since", w.cursor), "events", "ref")
			},
		},
	}
}

// treeExpectations pairs the drawn rows of a containment table against the
// payload's own pre-order walk of the root's children.
//
// The pairing is the renderer's own walk rather than a claim about two heads
// agreeing: runTree and runContents each take one tree from one library call
// and hand that same value either to the marshaller or to renderTree, and
// treeRows appends exactly one row per node before recursing into its
// children, with the root drawing no row of its own.
//
// A grouping row addresses nothing. Its cell carries the value the group
// gathered at, which column.tree.reference's own context already declares, so
// such a row is compared and no round trip is attempted for it. The Entity
// cell cannot tell the two kinds of row apart, which is why the payload does
// it: verb.FieldColumn and bench.KindColumn are both the word column, and the
// default chain groups on column first, so `dinah tree` prints that word
// beside a grouping row while `contents` prints it beside a column row.
func treeExpectations(t *testing.T, payload map[string]any) []addressExpectation {
	t.Helper()
	root, ok := payload["root"].(map[string]any)
	if !ok {
		t.Fatalf("the payload carries no root node: %v", payload)
	}
	var want []addressExpectation
	var walk func(nodes []any)
	walk = func(nodes []any) {
		for _, member := range nodes {
			node, ok := member.(map[string]any)
			if !ok {
				t.Fatalf("a tree node is not an object: %v", member)
			}
			if kind, _ := node["kind"].(string); kind == verb.NodeGroup {
				value, _ := node["value"].(string)
				if value == "" {
					value = msg.For(msg.Base).T("tree.unset")
				}
				want = append(want, addressExpectation{text: value})
			} else {
				ref, _ := node["ref"].(string)
				want = append(want, addressExpectation{text: ref, resolves: true})
			}
			children, _ := node["children"].([]any)
			walk(children)
		}
	}
	children, _ := root["children"].([]any)
	walk(children)
	return want
}

// TestAChecklistItemIsAddressedByAWordAndTheShortFormStillResolves asserts
// dinah-454 AC-13: the three checklist collections are addressed by a word on
// every surface Dinah prints, the three short forms the tool printed before go
// on resolving when a person types one, and the kind tokens are untouched by
// either.
//
// The arm that can fail is the last pair. A guard asserting only that the new
// words resolve would pass against a resolver that accepted every string, so
// the singular of each word and the kind token itself are handed to the same
// command and have to be refused. `decision` is the sharpest of the five,
// because it is a real kind token and this rename is about addressing rather
// than about kinds.
func TestAChecklistItemIsAddressedByAWordAndTheShortFormStillResolves(t *testing.T) {
	root := newBench(t)
	mustRun(t, root, "add", "a card carrying one item of each kind")
	mustRun(t, root, "file", "fx-1", "open_question", "does the deadline move?")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "the endpoint answers 404 for an unknown id")
	mustRun(t, root, "file", "fx-1", "decision", "the write path takes the card's own lock")

	shown := mustRun(t, root, "--lang", "en", "show", "fx-1").out
	for _, word := range []string{"fx-1/questions/1", "fx-1/criteria/1", "fx-1/decisions/1"} {
		if !strings.Contains(shown, word) {
			t.Errorf("show draws no row carrying the reference %q:\n%s", word, shown)
		}
	}
	for _, short := range []string{"fx-1/oq/1", "fx-1/ac/1", "fx-1/d/1"} {
		if strings.Contains(shown, short) {
			t.Errorf("show still prints the short form %q, and a word is what it composes now:\n%s", short, shown)
		}
	}

	// Both spellings open the same file, which is what keeps a reference
	// written down before the words landed working. The comparison is between
	// two answers from the tool rather than against a path this test builds,
	// so it cannot agree with itself.
	for _, spelling := range []struct{ word, short string }{
		{word: "fx-1/questions/1", short: "fx-1/oq/1"},
		{word: "fx-1/criteria/1", short: "fx-1/ac/1"},
		{word: "fx-1/decisions/1", short: "fx-1/d/1"},
	} {
		byWord := strings.TrimSpace(mustRun(t, root, "path", spelling.word).out)
		byShort := strings.TrimSpace(mustRun(t, root, "path", spelling.short).out)
		if byWord != byShort {
			t.Errorf("%s opens %s and %s opens %s", spelling.word, byWord, spelling.short, byShort)
		}
	}

	for _, refused := range []string{
		"fx-1/question/1", "fx-1/criterion/1", "fx-1/decision/1",
		"fx-1/open_question/1", "fx-1/acceptance_criterion/1",
	} {
		got := runCLI(t, root, "path", refused)
		if got.code == 0 {
			t.Errorf("the segment %q was accepted and opened %s, and nothing declares that spelling", refused, strings.TrimSpace(got.out))
		}
	}

	// The kind tokens travel on the machine surface and this ruling did not
	// reach them, so the payload still carries all three.
	payload := mustRun(t, root, "show", "fx-1", "--json").out
	for _, token := range []string{`"open_question"`, `"acceptance_criterion"`, `"decision"`} {
		if !strings.Contains(payload, token) {
			t.Errorf("the payload carries no %s, and the kind tokens do not change with the addressing:\n%s", token, payload)
		}
	}
}
