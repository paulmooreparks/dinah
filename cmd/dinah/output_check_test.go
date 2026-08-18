package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/msg"
)

// The output check, and the three assertions that say what it covers.
//
// Every invocation any test in this package makes through runCLI has both its
// streams folded into columnar blocks and every block checked for alignment in
// display columns. The check reads bytes and never source, so it holds
// whatever route drew the block: a table, a printing call at the stream, a
// helper handed the stream, or a row composed into a translated message all
// arrive as the same bytes.
//
// Three review cycles were spent raising a source scan's ceiling one route at
// a time, and two routes cannot be closed by any scan of Go sources. This is
// where the load sits instead, and what follows is written down so that a
// later reader does not have to rediscover where it stops.
//
// What the output check cannot see:
//
//   - A hand-rolled table that is correctly aligned. It reaches a reader as a
//     table that works, so this check has nothing against it. Only the source
//     patterns speak to the abstraction it violates.
//   - A table whose gutter is one space. A single space is read as being
//     inside a field, and a block built that way is not a table by this
//     card's rules, where the gutter is two.
//   - A table of one row, and a pair of rows whose field counts differ. A run
//     of one line is not folded, so a block whose every row stops short at a
//     different field is invisible.
//   - A table whose rows are not adjacent. The fold takes consecutive lines
//     only, so the comments of dinah show, which print a body between one row
//     and the next, and the four groups of the command list, which are
//     separated by blank lines and share one set of widths, are never folded
//     into one block at all. The rendered-output sweep is what covers those
//     two, since it reads a block through its own heading row rather than
//     through adjacency, and a disagreement between two groups of the command
//     list is invisible here.
//   - A block no test prints. The coverage check below bounds that and does
//     not remove it.
//   - A block of ASCII tokens alone. A table padded by counting bytes lines up
//     for as long as every field is ASCII, so the fixtures that carry the
//     three wide-script card titles are what make a byte count visible, and a
//     command whose every field is a token proves nothing about the measure.
//   - A block whose only field wider in display columns than in bytes is the
//     last one. Padding by counting bytes lines a block up whenever nothing
//     follows the field the count is wrong about, so a hand-rolled table can
//     carry the wide-script titles the fixtures exist to supply and still
//     reach this check aligned. That is the shape of `dinah ls`, whose title
//     is both the wide field and the last one, and it is the listing a reader
//     meets most: a hand-rolled block planted there is invisible here, and
//     only the source patterns speak to it. A plant reaches this check only
//     when the block carries another field after its wide one, and the states
//     listing is where this card's arming proofs planted theirs for that
//     reason.
//   - Anything written to somewhere other than the two streams run was given.
//     Pattern 11 of the source guard is what holds that, and pattern 11 is a
//     source scan.
//
// None of the checks in this file establishes that no hand-rolled table
// exists. They establish that every table the source draws is drawn under the
// corpus, that every statement of the rendering head executes or is named,
// that nothing writes anywhere but the two streams, and that every columnar
// block among those bytes lines up in display columns.

// checkColumnsLineUp folds a stream into columnar blocks and asserts that each
// one lines up. It runs after every invocation runCLI makes, over both
// streams.
func checkColumnsLineUp(t *testing.T, stream, text string) {
	t.Helper()
	for _, block := range foldColumnarBlocks(text) {
		for _, finding := range block.findings() {
			t.Errorf("%s: %s\n%s", stream, finding, strings.Join(block.lines, "\n"))
		}
	}
}

// columnarBlock is a maximal run of two or more consecutive lines that share a
// leading indent and split into the same number of fields.
type columnarBlock struct {
	lines  []string
	fields [][]columnarField
}

// columnarField is one field of one line: where it begins, in display columns,
// and what it draws.
type columnarField struct {
	at   int
	text string
}

// findings reports every way a block fails to line up: a field beginning at a
// different display column from its counterpart on the block's first line, and
// a line ending in a space.
func (b columnarBlock) findings() []string {
	var found []string
	for _, line := range b.lines {
		if strings.HasSuffix(line, " ") {
			found = append(found, "a line ends in a space")
			break
		}
	}
	first := b.fields[0]
	for _, fields := range b.fields[1:] {
		for i, field := range fields {
			if field.at != first[i].at {
				found = append(found, "column "+strconv.Itoa(i+1)+" starts at "+strconv.Itoa(first[i].at)+" on one row and at "+strconv.Itoa(field.at)+" on another")
			}
		}
	}
	return found
}

// foldColumnarBlocks reads a stream into the columnar blocks it carries. A run
// of lines belongs to one block while every line shares the indent and the
// field count of the line that opened the run, and a run of fewer than two
// lines, or of lines yielding fewer than two fields, is not a table.
func foldColumnarBlocks(text string) []columnarBlock {
	var blocks []columnarBlock
	var open columnarBlock
	close := func() {
		if len(open.lines) > 1 {
			blocks = append(blocks, open)
		}
		open = columnarBlock{}
	}
	for _, line := range strings.Split(text, "\n") {
		fields := columnarFields(line)
		if len(fields) < 2 {
			close()
			continue
		}
		if len(open.lines) > 0 {
			opening := open.fields[0]
			if len(fields) != len(opening) || fields[0].at != opening[0].at {
				close()
			}
		}
		open.lines = append(open.lines, line)
		open.fields = append(open.fields, fields)
	}
	close()
	return blocks
}

// columnarFields splits one line into the fields it draws, where a field
// boundary is a run of two or more spaces and each field is reported at the
// display column it begins in.
//
// The scanner has to end a field on a run of spaces that reaches the end of
// the line before it looks for the next one. A line ending in a single space
// otherwise leaves it looking for a field it never finds and never advancing
// past, and a trailing space is itself a finding, so the scanner has to
// survive one in order to report it.
func columnarFields(line string) []columnarField {
	runes := []rune(line)
	var fields []columnarField
	at := 0
	column := 0
	for at < len(runes) && runes[at] == ' ' {
		at++
		column++
	}
	for at < len(runes) {
		start := column
		var text strings.Builder
		for at < len(runes) {
			if runes[at] != ' ' {
				text.WriteRune(runes[at])
				column += displayWidth(string(runes[at]))
				at++
				continue
			}
			gap := at
			for gap < len(runes) && runes[gap] == ' ' {
				gap++
			}
			if gap-at > 1 || gap == len(runes) {
				break
			}
			text.WriteRune(' ')
			column++
			at++
		}
		fields = append(fields, columnarField{at: start, text: text.String()})
		for at < len(runes) && runes[at] == ' ' {
			at++
			column++
		}
	}
	return fields
}

// TestTheOutputCheckReportsAMisalignedBlock arms the check above. A check that
// passes proves nothing on its own, since it also passes when what it guards
// is absent, so this hands it a table padded by counting bytes and requires it
// to report exactly what is wrong.
//
// The three lines are what a padder counting bytes emits for a card listing
// carrying a Japanese title: the title measures ten display columns and counts
// as five characters, so the field after it starts five columns early.
func TestTheOutputCheckReportsAMisalignedBlock(t *testing.T) {
	byteCounted := []string{
		"  fx-1     " + wideTitle + "     ready",
		"  fx-2     a plain card    ready",
		"  fx-3     a second card   ready",
	}
	blocks := foldColumnarBlocks(strings.Join(byteCounted, "\n"))
	if len(blocks) != 1 {
		t.Fatalf("the fold read %d blocks out of a table of three rows, want 1", len(blocks))
	}
	found := blocks[0].findings()
	if len(found) == 0 {
		t.Errorf("the check reported nothing about a table padded by counting bytes:\n%s", strings.Join(byteCounted, "\n"))
	}

	aligned := []string{
		"  fx-1  " + wideTitle + "  ready",
		"  fx-2  a plain card    ready",
	}
	if blocks := foldColumnarBlocks(strings.Join(aligned, "\n")); len(blocks) != 1 || len(blocks[0].findings()) != 0 {
		t.Errorf("the check reported a block whose columns do line up:\n%s", strings.Join(aligned, "\n"))
	}

	trailing := []string{"  fx-1  ready ", "  fx-2  ready "}
	blocks = foldColumnarBlocks(strings.Join(trailing, "\n"))
	if len(blocks) != 1 {
		t.Fatalf("the fold read %d blocks out of two rows ending in a space, want 1", len(blocks))
	}
	if len(blocks[0].findings()) == 0 {
		t.Errorf("the check reported nothing about a line ending in a space: %q", trailing[0])
	}
}

// theRenderingHeadDir is where this package's own sources sit, relative to the
// directory a test runs in.
const theRenderingHeadDir = "."

// tableSite is one call to s.table or s.tableLines: the file it sits in,
// relative to this package, and the line.
type tableSite struct {
	file string
	line int
}

// String renders a site the way sweptBlocks names one.
func (s tableSite) String() string {
	return s.file + ":" + strconv.Itoa(s.line)
}

// tableSitesInSource walks this package's non-test sources and reports every
// call to s.table or s.tableLines, by file and line. table.go is left out: it
// is where the two live, and its own call from table to tableLines is not a
// site that draws a block.
func tableSitesInSource() (map[tableSite]bool, error) {
	entries, err := os.ReadDir(theRenderingHeadDir)
	if err != nil {
		return nil, err
	}
	sites := map[tableSite]bool{}
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "table.go" {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(theRenderingHeadDir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		scanned++
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "table" && selector.Sel.Name != "tableLines") {
				return true
			}
			sites[tableSite{file: name, line: fset.Position(call.Pos()).Line}] = true
			return true
		})
	}
	if scanned == 0 {
		return nil, errors.New("the walk scanned no source of the rendering head, so it proves nothing")
	}
	if len(sites) == 0 {
		return nil, errors.New("the walk found no table call site at all, so it proves nothing")
	}
	return sites, nil
}

// TestEveryTableSiteIsRegistered pairs the sites the source draws tables at
// against the entries the sweep registers, in both directions: every entry
// names a site that exists, and every site the walk finds is named by at least
// one entry.
//
// Both directions are subset tests over sets of sites rather than counts, so
// the two entries that share formatCandidateRows and the two that share
// renderOffers satisfy them once each. A rule demanding one entry per site
// would make an implementer either delete those fixtures or never reach a
// green build.
func TestEveryTableSiteIsRegistered(t *testing.T) {
	sites, err := tableSitesInSource()
	if err != nil {
		t.Fatalf("walk the head's sources: %v", err)
	}
	named := map[string]bool{}
	for _, block := range sweptBlocks() {
		named[block.site] = true
	}
	drawn := map[string]bool{}
	for site := range sites {
		drawn[site.String()] = true
	}
	for site := range drawn {
		if !named[site] {
			t.Errorf("%s draws a table and no entry of sweptBlocks names it, so nothing asserts what it renders", site)
		}
	}
	for site := range named {
		if !drawn[site] {
			t.Errorf("sweptBlocks names %s and the source draws no table there, so the entry is stale and may be covering the wrong block", site)
		}
	}
}

// reachedTableSites are the sites the corpus actually drew a table through,
// recorded by the hook table.go arms for the whole run.
var reachedTableSites = map[tableSite]bool{}

// recordReachedTableSite is what the hook calls. It keeps the file's own name
// rather than the absolute path the runtime reports, which is what the walk
// above collects.
//
// A record from table.go is dropped, for the reason the walk skips that file:
// the call table makes to tableLines is where the two live rather than a site
// that draws a block. A record from a test source is dropped for the matching
// reason, since a table a unit test builds by hand is not a site of the head
// and the walk reads no test source. Counting either would leave the two sets
// unable to agree.
func recordReachedTableSite(file string, line int) {
	name := filepath.Base(file)
	if name == "table.go" || strings.HasSuffix(name, "_test.go") {
		return
	}
	reachedTableSites[tableSite{file: name, line: line}] = true
}

// unreachedTableSites pairs the sites the corpus reached against the sites the
// source declares, in both directions, and reports what disagrees.
//
// It runs from TestMain once the whole suite has finished, because the corpus
// is the suite itself and no test in the package can be relied on to run last.
// Both directions are subset tests over sets of sites rather than counts, so a
// site drawing two blocks satisfies them once and a site drawn from a loop
// satisfies them once. On a filtered run it stands down, since the corpus is
// then whatever the filter let through.
func unreachedTableSites() []string {
	if !fullSuiteRan() {
		return nil
	}
	sites, err := tableSitesInSource()
	if err != nil {
		return []string{"the table-site walk could not run: " + err.Error()}
	}
	var complaints []string
	for site := range sites {
		if !reachedTableSites[site] {
			complaints = append(complaints, site.String()+" draws a table and no test in this package reached it, so no output check ever read what it draws")
		}
	}
	for site := range reachedTableSites {
		if !sites[site] {
			complaints = append(complaints, "a table was drawn at "+site.String()+" and the walk over this package's sources found no call there")
		}
	}
	sort.Strings(complaints)
	return complaints
}

// fullSuiteRan reports whether this binary was asked to run its whole suite. A
// filtered run reaches a subset of the corpus by construction, and a reach
// check against a subset would fail for the wrong reason.
func fullSuiteRan() bool {
	filter := flagValue("test.run")
	return filter == "" || filter == ".*" || filter == ".+"
}

// flagValue reads a testing flag's value without importing the flag package's
// own parse order into every caller.
func flagValue(name string) string {
	for i, argument := range os.Args {
		if argument == "-"+name || argument == "--"+name {
			if i+1 < len(os.Args) {
				return os.Args[i+1]
			}
			return ""
		}
		for _, prefix := range []string{"-" + name + "=", "--" + name + "="} {
			if strings.HasPrefix(argument, prefix) {
				return strings.TrimPrefix(argument, prefix)
			}
		}
	}
	return ""
}

// TestOnlyRunCLIDrivesTheHead asserts that every test in this package reaches
// the head through the one helper, so no test can hand run a writer the output
// check never reads. It is the companion to the source guard's eleventh
// pattern: that one holds that the head writes nowhere but the two streams run
// was given, and this one holds that those two streams are always the ones the
// check is watching.
func TestOnlyRunCLIDrivesTheHead(t *testing.T) {
	entries, err := os.ReadDir(theRenderingHeadDir)
	if err != nil {
		t.Fatalf("read the head's sources: %v", err)
	}
	drivers := map[string]bool{"runCLI": true, "runCLIWithInput": true}
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(theRenderingHeadDir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		scanned++
		for _, declared := range file.Decls {
			function, ok := declared.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			within := function.Name.Name
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isBareIdentCall(call.Fun, "run") || drivers[within] {
					return true
				}
				t.Errorf("%s:%d: %s calls run directly, and every test reaches the head through runCLI so that the output check reads both its streams",
					name, fset.Position(call.Pos()).Line, within)
				return true
			})
		}
	}
	if scanned == 0 {
		t.Fatal("the walk read no test source, so it proves nothing")
	}
}

// isBareIdentCall reports whether a callee is the identifier named, on its own.
func isBareIdentCall(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

// TestEveryColumnHeadingIsCarriedByTheBaseCatalog asserts both halves of the
// heading contract: every column. key the head asks for exists in the base
// catalog, and every column. key the base catalog carries is asked for by some
// block. A heading key nobody reads is a heading that was renamed and left
// behind.
//
// The keys the head asks for are read out of its own s.columns calls, since
// the key is composed from the block and the column name and no scan for a
// literal can see it.
func TestEveryColumnHeadingIsCarriedByTheBaseCatalog(t *testing.T) {
	asked := columnKeysTheHeadAsksFor(t)
	if len(asked) == 0 {
		t.Fatal("no column heading was found in the head's sources, so this test proves nothing")
	}
	for _, key := range asked {
		if _, ok := msg.BaseEntry(key); !ok {
			t.Errorf("the head asks for the heading %s and the base catalog does not carry it", key)
		}
	}
	held := map[string]bool{}
	for _, key := range asked {
		held[key] = true
	}
	for _, key := range msg.Keys() {
		if !strings.HasPrefix(key, "column.") {
			continue
		}
		if !held[key] {
			t.Errorf("the base catalog carries the heading %s and no block asks for it", key)
		}
	}
}

// columnKeysTheHeadAsksFor reads every key an s.columns call composes, which is
// the block name and one name per column.
func columnKeysTheHeadAsksFor(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(theRenderingHeadDir)
	if err != nil {
		t.Fatalf("read the head's sources: %v", err)
	}
	var keys []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(theRenderingHeadDir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "columns" || len(call.Args) < 2 {
				return true
			}
			block, ok := stringLiteral(call.Args[0])
			if !ok {
				t.Errorf("%s: a columns call names its block with something other than a literal, which no reader of this test can follow", name)
				return true
			}
			for _, argument := range call.Args[1:] {
				column, ok := stringLiteral(argument)
				if !ok {
					t.Errorf("%s: a columns call names a column with something other than a literal", name)
					continue
				}
				keys = append(keys, "column."+block+"."+column)
			}
			return true
		})
	}
	sort.Strings(keys)
	return keys
}

// stringLiteral reads a plain string literal, reporting false for anything a
// reader would have to evaluate.
func stringLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
