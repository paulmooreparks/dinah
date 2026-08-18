package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/msg"
)

// The three titles the sweep files its cards under. Each is a script whose
// drawn width disagrees with its rune count in a different way, and each
// reaches every block that renders a card title.
const (
	// wideTitle draws twice its rune count, since every rune is East Asian
	// Wide.
	wideTitle = "作業台のカード"
	// matraTitle carries spacing vowel signs, which take a column each, and
	// nonspacing ones, which take none, so a rune count measures it long.
	matraTitle = "हिन्दी कार्ड"
	// joinedTitle carries an emoji sequence a terminal draws as one glyph and
	// a rune count measures as three.
	joinedTitle = "\U0001F468\u200D\U0001F469\u200D\U0001F467 family"
)

// sweptBlock is one site of the spec's inventory: what it draws, the columns
// it declares, and how a fixture reaches it.
//
// widths are the declared cell widths in print order. Field 0 is the first
// cell and the field after the last cell is the tail, which declares no width
// and takes whatever is left of the line.
type sweptBlock struct {
	// site is the number the spec's inventory gives this block.
	site int
	// label names the block for a failure message.
	label string
	// widths are the declared cell widths, in print order. An empty slice is
	// a block of one field under an indent, which declares no column.
	widths []int
	// varies is the cell whose display width the fixture makes differ between
	// two rows, which is what stops the tail assertion holding whatever the
	// measure does. It defaults to the last cell. A negative value declares
	// that no cell of this block varies, and constantReason says why.
	varies int
	// constantReason is why this block's fields take values no command
	// varies. It is empty on every block whose cells vary.
	constantReason string
	// render runs the command that draws this block and returns its lines.
	render func(t *testing.T, w *sweptWorkbenches, tag string) []string
}

// sweptWorkbenches are the four trees the sweep renders from. Thirteen sites
// draw from a workbench holding a few cards and nothing unusual. The other
// seven draw only in a state a healthy workbench never reaches, so each of
// those gets a tree built for it.
type sweptWorkbenches struct {
	// healthy holds three cards under three scripts, one held, one blocked,
	// one carrying a link and a comment, with an operator-owned state and a
	// state under a limit.
	healthy string
	// base is where a tree a repair mutates is built. A migration repairs the
	// workbench it is run against, so the three blocks reached by one cannot
	// share a tree across the language pass and each builds its own.
	base string
	// ambiguous holds two workbenches under one base, which is the search
	// that resolves to a choice rather than to a workbench.
	ambiguous string
	// card is a reference the healthy tree carries.
	card string
	// held is the reference of the card claimed in the healthy tree.
	held string
}

// TestEveryRowStartsItsColumnsAtOneDisplayColumn renders every block of the
// spec's inventory in every shipped language and asserts that each field,
// the tail included, starts at the display column the block declares for it.
//
// It is the backstop for what the source guard cannot see. A row padded with
// a byte length, a row padded by filling a slice of bytes with spaces, and a
// row composed inside a message catalog all reach a person as output, and this
// assertion holds whatever produced the line.
//
// Four assertions keep the sweep from covering less than it claims. It fails
// when a site emitted no row at all, which is what leaves a whole block
// untested while the suite reports green. It fails when a block emitted fewer
// than two rows, since one row cannot disagree with itself. It asserts where
// the tail starts and not only where the declared columns do, since the tail
// carries most of the variable content and a rune count misplaces it. And it
// requires the cell in front of the tail to differ in display width between
// two rows, since a block whose last cell is the same on every row starts its
// tail in the same place whatever the measure does.
func TestEveryRowStartsItsColumnsAtOneDisplayColumn(t *testing.T) {
	benches := buildSweptWorkbenches(t)
	blocks := sweptBlocks()
	for _, block := range blocks {
		rendered := 0
		for _, tag := range msg.Tags() {
			lines := block.render(t, benches, tag)
			if len(lines) == 0 {
				t.Errorf("site %d (%s), locale %s: the fixture reached no row of this block, so nothing about it is asserted", block.site, block.label, tag)
				continue
			}
			rendered++
			rows := readSweptRows(t, block, tag, lines)
			if len(block.widths) > 0 && len(rows) < 2 {
				t.Errorf("site %d (%s), locale %s: the block rendered %d row, and one row cannot disagree with itself", block.site, block.label, tag, len(rows))
				continue
			}
			assertACellVaries(t, block, tag, rows)
		}
		if rendered == 0 {
			t.Errorf("site %d (%s) rendered in no locale at all", block.site, block.label)
		}
	}
}

// readSweptRows folds a block's rendered lines back into rows and asserts, for
// every field of every row, that it begins at the display column the block
// declares. It returns each row's fields.
//
// The walk mirrors what formatRow does. A field under its column is padded to
// it, so the next field begins at the declared column with a space in front of
// it. A field that reaches its column takes the rest of its own line, and the
// fields after it resume on the next line at the column that field's own would
// have ended in. Anything else is a row whose fields have drifted, which is
// what this test exists to catch.
func readSweptRows(t *testing.T, block sweptBlock, tag string, lines []string) [][]string {
	t.Helper()
	columns := sweptBoundaries(block)
	var rows [][]string
	var row []string
	next := 0
	fail := func(format string, args ...any) [][]string {
		t.Helper()
		t.Errorf("site %d (%s), locale %s: "+format, append([]any{block.site, block.label, tag}, args...)...)
		return rows
	}
	for i, line := range lines {
		if sweptLead(line) != columns[next] {
			return fail("field %d begins at display column %d and the block declares %d:\n%q",
				next, sweptLead(line), columns[next], line)
		}
		for {
			if next == len(columns)-1 {
				row = append(row, sweptField(line, columns[next], -1))
				rows, row, next = append(rows, row), nil, 0
				break
			}
			edge := columns[next+1]
			resumes := i+1 < len(lines) && sweptLead(lines[i+1]) == edge
			if sweptBlank(line, columns[next], edge) {
				row = append(row, "")
				next++
				continue
			}
			if sweptSpaceAt(line, columns[next]) {
				return fail("field %d begins past the display column %d the block declares:\n%q", next, columns[next], line)
			}
			if displayWidth(line) < edge {
				return fail("field %d was not widened to the display column %d the block declares, so whatever follows it on the line is not where it belongs:\n%q",
					next, edge, line)
			}
			if displayWidth(line) == edge {
				row = append(row, sweptField(line, columns[next], -1))
				next++
				if resumes {
					break
				}
				for len(row) < len(columns) {
					row = append(row, "")
				}
				rows, row, next = append(rows, row), nil, 0
				break
			}
			if sweptSpaceAt(line, edge-1) && !sweptSpaceAt(line, edge) {
				row = append(row, sweptField(line, columns[next], edge))
				next++
				continue
			}
			if !resumes {
				return fail("field %d runs through display column %d, where field %d begins, and no continuation line resumes there:\n%q",
					next, edge, next+1, line)
			}
			row = append(row, sweptField(line, columns[next], -1))
			next++
			break
		}
	}
	if next != 0 {
		return fail("a row ran out of lines with field %d still to come", next)
	}
	return rows
}

// sweptBlank reports whether every display column between two boundaries holds
// a space, which is what a field carrying no text at all leaves behind.
func sweptBlank(line string, from, to int) bool {
	for column := from; column < to; column++ {
		if !sweptSpaceAt(line, column) {
			return false
		}
	}
	return true
}

// sweptBoundaries returns the display column each field of a block begins at,
// the tail's included.
func sweptBoundaries(block sweptBlock) []int {
	columns := make([]int, 0, len(block.widths)+1)
	column := 2
	for _, width := range block.widths {
		columns = append(columns, column)
		column += width
	}
	return append(columns, column)
}

// sweptLead reports the display column a line's content begins at.
func sweptLead(line string) int {
	return displayWidth(line) - displayWidth(strings.TrimLeft(line, " "))
}

// sweptSpaceAt reports whether the display column at holds a space. A column
// past the line's end holds nothing and is not a space.
func sweptSpaceAt(line string, at int) bool {
	column := 0
	for _, r := range line {
		if column == at {
			return r == ' '
		}
		column += displayWidth(string(r))
		if column > at {
			return false
		}
	}
	return false
}

// sweptField returns the text between two display columns, trimmed of the
// padding that carried it to the next one. An end of minus one takes the rest
// of the line.
func sweptField(line string, from, to int) string {
	var b strings.Builder
	column := 0
	for _, r := range line {
		drawn := displayWidth(string(r))
		if column >= from && (to < 0 || column < to) {
			b.WriteRune(r)
		}
		column += drawn
	}
	return strings.TrimRight(b.String(), " ")
}

// assertACellVaries asserts that the cell a block declares as its varying one
// really does differ in display width between two of the block's rows. Without
// it the tail's start column is the same on every row whatever the measure
// does, and asserting it proves nothing.
func assertACellVaries(t *testing.T, block sweptBlock, tag string, rows [][]string) {
	t.Helper()
	if len(block.widths) == 0 || block.varies == noCell {
		if block.varies == noCell && block.constantReason == "" {
			t.Errorf("site %d (%s) declares that no cell varies and gives no reason", block.site, block.label)
		}
		return
	}
	field := block.varies
	if field == lastCell {
		field = len(block.widths) - 1
	}
	widths := map[int]bool{}
	for _, row := range rows {
		if field < len(row) {
			widths[displayWidth(row[field])] = true
		}
	}
	if len(widths) < 2 {
		t.Errorf("site %d (%s), locale %s: cell %d draws the same width on every row, so the tail begins in the same place whatever the measure does",
			block.site, block.label, tag, field)
	}
}

// indentedBlock returns the indented lines of an output that follow a heading,
// stopping at the first line that is neither indented nor blank. A heading of
// the empty string starts at the top of the output.
func indentedBlock(out, heading string) []string {
	var lines []string
	started := heading == ""
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !started {
			started = line == heading
			continue
		}
		if strings.TrimSpace(line) == "" {
			if len(lines) > 0 {
				break
			}
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			if len(lines) > 0 {
				break
			}
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// indentedLinesAfter returns every indented line following a heading, over any
// number of unindented lines between them. A comment's body sits unindented
// between two comment headers, so the comment block cannot stop at the first
// line that is not indented.
func indentedLinesAfter(out, heading string) []string {
	var lines []string
	started := false
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !started {
			started = line == heading
			continue
		}
		if strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// The two values sweptBlock.varies takes beyond a field index.
const (
	// lastCell asks for the cell in front of the tail, which is the one whose
	// width decides where the tail begins.
	lastCell = -2
	// noCell declares that no cell of a block varies, which a block may only
	// do with a reason.
	noCell = -1
)

// sweptBlocks are the twenty sites of the spec's inventory, at the
// twenty-one points they render at.
func sweptBlocks() []sweptBlock {
	rendered := func(tag, key string) string { return msg.For(tag).T(key) }
	return []sweptBlock{
		{
			site: 1, label: "the legal moves under a served instruction", widths: []int{14, 32}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "instructions", w.card)
				return indentedBlock(out, rendered(tag, "instructions.moves"))
			},
		},
		{
			site: 2, label: "dinah states", widths: []int{14, 24, 32, 10, 8}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "states"), "")
			},
		},
		{
			site: 3, label: "dinah ls", widths: []int{14, 10}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "ls"), "")
			},
		},
		{
			site: 4, label: "dinah config", widths: []int{12, 24}, varies: noCell,
			constantReason: "a setting's value is whatever the user has stored, and this fixture stores none, " +
				"so the value column is empty on every row; the eight-language pass carries the source token in the tail",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "config"), "")
			},
		},
		{
			site: 5, label: "dinah workbenches", widths: []int{32, 16}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.ambiguous, tag, "workbenches"), "")
			},
		},
		{
			site: 5, label: "the ambiguous-workbench refusal, written to stderr", widths: []int{32, 16}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRefused(t, w.ambiguous, tag, "status"), "")
			},
		},
		{
			site: 6, label: "dinah next, a state offering nothing", widths: []int{32}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return sweptOffers(t, w, tag, false)
			},
		},
		{
			site: 7, label: "dinah next, a state offering a card", widths: []int{32, 14}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return sweptOffers(t, w, tag, true)
			},
		},
		{
			site: 8, label: "dinah log", widths: []int{22, 14, 16}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "log", w.held), "")
			},
		},
		{
			site: 9, label: "the slugs check --migrate-slugs assigned", widths: []int{24}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, sweptStrippedTree(t, w, "slugs-"+tag), tag, "check", "--migrate-slugs"), "")
			},
		},
		{
			site: 10, label: "dinah help <command>", widths: []int{3, 52}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "help", "add")
				return indentedBlock(out, rendered(tag, "help.refusals"))
			},
		},
		{
			site: 11, label: "the command list of bare dinah", widths: []int{39}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag)
				return indentedBlock(out, rendered(tag, "help.group.work"))
			},
		},
		{
			site: 12, label: "the global flag list", widths: []int{20}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag)
				return indentedBlock(out, rendered(tag, "help.flags"))
			},
		},
		{
			site: 13, label: "the cards you hold", widths: []int{14}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "status")
				return indentedBlock(out, rendered(tag, "status.holding"))
			},
		},
		{
			site: 14, label: "the cards that are blocked", widths: []int{14}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "status")
				return indentedBlock(out, rendered(tag, "status.blocked"))
			},
		},
		{
			site: 15, label: "a card's links", widths: []int{14}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "show", w.card)
				return indentedBlock(out, rendered(tag, "show.links"))
			},
		},
		{
			site: 16, label: "catalog coverage", widths: []int{8}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "version", "--catalogs")
				return indentedBlock(out, rendered(tag, "version.catalogs"))
			},
		},
		{
			site: 17, label: "the guide topics", widths: []int{20}, varies: lastCell,
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, w.healthy, tag, "guide"), "")
			},
		},
		{
			site: 18, label: "a comment's header", widths: []int{22}, varies: noCell,
			constantReason: "a timestamp is one format in one time zone, so every comment header draws its stamp " +
				"at the same width; the author in the tail is what this block's assertion rests on",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				out := sweptRun(t, w.healthy, tag, "show", w.card)
				return indentedLinesAfter(out, rendered(tag, "show.comments"))
			},
		},
		{
			site: 19, label: "one removed stranded state", varies: noCell,
			constantReason: "this block declares no cell at all, so it has no column to misplace",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRun(t, sweptStrandedTree(t, w, "stranded-"+tag), tag, "check", "--migrate-states"), "")
			},
		},
		{
			site: 20, label: "one finding", varies: noCell,
			constantReason: "this block declares no cell at all, so it has no column to misplace",
			render: func(t *testing.T, w *sweptWorkbenches, tag string) []string {
				return indentedBlock(sweptRefused(t, sweptStrippedTree(t, w, "findings-"+tag), tag, "check"), "")
			},
		},
	}
}

// sweptOffers splits the rows dinah next prints into the two shapes it draws:
// a state offering a card carries the card's reference and title, and a state
// offering nothing carries the catalog's own sentence instead.
func sweptOffers(t *testing.T, w *sweptWorkbenches, tag string, offering bool) []string {
	t.Helper()
	none := msg.For(tag).T("next.none")
	var lines []string
	for _, line := range indentedBlock(sweptRun(t, w.healthy, tag, "next"), "") {
		if strings.HasSuffix(line, none) != offering {
			lines = append(lines, line)
		}
	}
	return lines
}

// sweptRun runs a command in a language and returns what it wrote to stdout,
// failing when the command refused.
func sweptRun(t *testing.T, dir, tag string, argv ...string) string {
	t.Helper()
	got := runCLI(t, dir, append([]string{"--lang", tag}, argv...)...)
	if got.code != 0 {
		t.Fatalf("%v in %s: exit %d\n%s", argv, tag, got.code, got.errw)
	}
	return got.out
}

// sweptRefused runs a command expected to refuse and returns what it wrote,
// stdout and stderr alike, since one of the two blocks reached this way is
// written to each.
func sweptRefused(t *testing.T, dir, tag string, argv ...string) string {
	t.Helper()
	got := runCLI(t, dir, append([]string{"--lang", tag}, argv...)...)
	if got.code == 0 {
		t.Fatalf("%v in %s was expected to refuse and exited 0\n%s", argv, tag, got.out)
	}
	return got.out + got.errw
}

// sweptTitles are the titles the sweep files its cards under, cycling so that
// every block rendering a card title meets each script.
var sweptTitles = []string{wideTitle, matraTitle, joinedTitle, "a plain card"}

// reviewState is the fourth state the sweep writes into the healthy workbench.
// The flow init builds has three, which leaves dinah next with one state
// offering a card and one offering nothing, and a block of one row cannot
// disagree with itself. This state is operator-owned as well, so dinah states
// draws its own tail on at least one row.
// waitingState is the fifth, which leaves two states offering nothing, since a
// block of one row cannot disagree with itself either.
const (
	reviewState  = "e00000000001"
	waitingState = "e00000000002"
)

// reviewTitle is written in a script drawing two columns per rune, so the
// title column of dinah states and of dinah next carries text a rune count
// measures short.
const reviewTitle = "検討"

// buildSweptWorkbenches builds the four trees the sweep renders from, under
// one user base so that one language pass reaches all of them.
func buildSweptWorkbenches(t *testing.T) *sweptWorkbenches {
	t.Helper()
	base := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("COLUMNS", "")

	benches := &sweptWorkbenches{
		base:      base,
		healthy:   filepath.Join(base, "healthy"),
		ambiguous: filepath.Join(base, "ambiguous"),
		card:      "fx-1",
		held:      "fx-1",
	}
	for _, dir := range []string{benches.healthy, benches.ambiguous} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	sweptInit(t, benches.healthy)
	sweptAddState(t, benches.healthy, reviewState, reviewTitle, "work", "operator_owned: true\n")
	sweptAddState(t, benches.healthy, waitingState, "Waiting", "work", "")
	for i := 0; i < 12; i++ {
		sweptDo(t, benches.healthy, "add", sweptTitles[i%len(sweptTitles)])
	}
	sweptDo(t, benches.healthy, "claim", "fx-1")
	sweptDo(t, benches.healthy, "claim", "fx-11")
	sweptDo(t, benches.healthy, "claim", "fx-2", "--actor", "bo")
	sweptDo(t, benches.healthy, "block", "fx-2", "waiting on a decision", "--actor", "bo")
	sweptDo(t, benches.healthy, "claim", "fx-10", "--actor", "bo")
	sweptDo(t, benches.healthy, "block", "fx-10", "waiting on a decision", "--actor", "bo")
	sweptDo(t, benches.healthy, "claim", "fx-4")
	sweptDo(t, benches.healthy, "move", "fx-4", "doing")
	sweptDo(t, benches.healthy, "release", "fx-4")
	sweptDo(t, benches.healthy, "claim", "fx-12")
	sweptDo(t, benches.healthy, "move", "fx-12", "doing")
	sweptDo(t, benches.healthy, "move", "fx-12", "done")
	sweptDo(t, benches.healthy, "release", "fx-12")
	sweptDo(t, benches.healthy, "comment", "fx-1", "the first note")
	sweptDo(t, benches.healthy, "comment", "fx-1", "the second note", "--actor", "bo")
	sweptWriteLinks(t, benches.healthy, "fx-1")

	rooms := populateBase(t, filepath.Join(benches.ambiguous, bench.UserBaseName), "one", "twoandthree")
	sweptRetitle(t, rooms[0], wideTitle)
	sweptRetitle(t, rooms[1], matraTitle)

	return benches
}

// sweptStrippedTree builds a workbench whose states carry no slug, which is
// the defect a plain check reports one finding at a time and check
// --migrate-slugs repairs one row at a time. Each call builds its own, since
// the repair is what the block renders and a repaired workbench draws nothing
// the second time.
func sweptStrippedTree(t *testing.T, w *sweptWorkbenches, name string) string {
	t.Helper()
	dir := filepath.Join(w.base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sweptInit(t, dir)
	sweptAddState(t, dir, reviewState, "Review under an operator", "work", "")
	sweptStripSlugs(t, dir)
	return dir
}

// sweptStrandedTree builds a workbench naming a state its own directory does
// not hold, which is what check --migrate-states removes. Each call builds its
// own, for the reason sweptStrippedTree gives.
func sweptStrandedTree(t *testing.T, w *sweptWorkbenches, name string) string {
	t.Helper()
	dir := filepath.Join(w.base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sweptInit(t, dir)
	sweptStrandState(t, dir)
	return dir
}

// sweptInit creates a workbench in a directory the sweep made.
func sweptInit(t *testing.T, dir string) {
	t.Helper()
	sweptDo(t, dir, "init", "--slug", "fx", "--operator", "alka")
}

// sweptDo runs a command that has to succeed while the fixture is being built.
func sweptDo(t *testing.T, dir string, argv ...string) {
	t.Helper()
	if got := runCLI(t, dir, argv...); got.code != 0 {
		t.Fatalf("%v: exit %d\n%s", argv, got.code, got.errw)
	}
}

// sweptRoot returns the one workbench directory a container holds.
func sweptRoot(t *testing.T, dir string) string {
	t.Helper()
	return soleBenchDir(t, dir)
}

// sweptAddState writes a state into a workbench by hand and appends it to the
// states list, since no command creates one.
func sweptAddState(t *testing.T, dir, id, title, kind, extra string) {
	t.Helper()
	root := sweptRoot(t, dir)
	states := filepath.Join(root, bench.StatesDir, id)
	if err := os.MkdirAll(states, 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	anchor := "---\ntitle: " + title + "\nkind: " + kind + "\n" + extra + "---\n"
	if err := os.WriteFile(filepath.Join(states, bench.StateAnchor), []byte(anchor), 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	sweptRewrite(t, filepath.Join(root, bench.WorkbenchAnchor), func(source string) string {
		return strings.Replace(source, "\nstates:\n", "\nstates:\n  - "+id+"\n", 1)
	})
}

// sweptStrandState names a state the workbench does not hold, which is the
// defect check --migrate-states removes.
func sweptStrandState(t *testing.T, dir string) {
	t.Helper()
	sweptRewrite(t, filepath.Join(sweptRoot(t, dir), bench.WorkbenchAnchor), func(source string) string {
		return strings.Replace(source, "\nstates:\n", "\nstates:\n  - ffffffffffff\n", 1)
	})
}

// sweptStripSlugs removes the slug line from every state, which is the defect
// a plain check reports and check --migrate-slugs repairs one row at a time.
func sweptStripSlugs(t *testing.T, dir string) {
	t.Helper()
	states := filepath.Join(sweptRoot(t, dir), bench.StatesDir)
	entries, err := os.ReadDir(states)
	if err != nil {
		t.Fatalf("read states: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sweptRewrite(t, filepath.Join(states, entry.Name(), bench.StateAnchor), func(source string) string {
			var kept []string
			for _, line := range strings.Split(source, "\n") {
				if !strings.HasPrefix(line, "slug:") {
					kept = append(kept, line)
				}
			}
			return strings.Join(kept, "\n")
		})
	}
}

// sweptWriteLinks writes a links sequence into a card's frontmatter, since no
// command creates a link and the reader reads one.
func sweptWriteLinks(t *testing.T, dir, ref string) {
	t.Helper()
	located := runCLI(t, dir, "path", ref)
	if located.code != 0 {
		t.Fatalf("path %s: %d %s", ref, located.code, located.errw)
	}
	path := strings.TrimSpace(located.out)
	sweptRewrite(t, path, func(source string) string {
		links := "links:\n  - kind: blocks\n    to: fx-2\n  - kind: relates-to\n    to: fx-3\n"
		cut := strings.Index(source[4:], "\n---\n")
		return source[:cut+4] + "\n" + links + source[cut+5:]
	})
}

// sweptRetitle rewrites a workbench's own title, so two workbenches under one
// base draw titles of different widths.
func sweptRetitle(t *testing.T, root, title string) {
	t.Helper()
	sweptRewrite(t, filepath.Join(root, bench.WorkbenchAnchor), func(source string) string {
		var kept []string
		for _, line := range strings.Split(source, "\n") {
			if strings.HasPrefix(line, "title:") {
				line = "title: " + title
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	})
}

// sweptRewrite reads a file, hands it to a change, and writes it back.
func sweptRewrite(t *testing.T, path string, change func(string) string) {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(change(string(source))), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
