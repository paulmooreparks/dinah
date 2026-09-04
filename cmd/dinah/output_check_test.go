package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
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
//     when the block carries another field after its wide one, and the columns
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
	if b.guided() {
		return found
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

// guided reports whether this block draws a tree, which is what takes the
// column-alignment comparison off it. One guided line is enough, because a
// tree's heading row and separator carry no guides and its stacked form
// carries them only on the line holding the reference.
//
// A tree draws an empty cell in the middle of a row wherever a node has
// nothing to say under a column: a group row carries no title, and a card row
// under `dinah tree` carries no count. The visible fields of one row are
// therefore a subset of the block's columns rather than all of them, so the
// third visible field of one row is a title where the third of another is a
// count, and every comparison this check can make between two rows of a tree
// compares two different columns. What a tree gets instead is the
// trailing-space check alone, and that is a limitation of this check rather
// than a licence. Reading a ragged block takes the block's declared column
// list, which is what the rendered-output sweep holds a tree against, empty
// cell by empty cell.
//
// The exemption reads the drawn characters rather than naming the block, so a
// block that is not a tree and happens to draw a guided line loses its column
// comparison too. Keying it on the registered block would need a block
// identity this check does not have, since it folds blocks out of whatever a
// command printed.
func (b columnarBlock) guided() bool {
	for _, line := range b.lines {
		if _, _, guided := guidedLead(line); guided {
			return true
		}
	}
	return false
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
	for at < len(runes) {
		// The column is recomputed from the whole prefix rather than carried
		// forward one field at a time, per columnAt's own reasoning: a width
		// is a property of a grapheme cluster rather than of a rune, and a
		// running sum drifts the moment a wide-glyph field sits ahead of
		// another.
		column := columnAt(runes, at)
		start := column
		var text strings.Builder
		// A tree's guides live inside the field they lead, and they open with
		// blanks wherever an ancestor was the last of its siblings, so the
		// field begins before its first visible glyph. guidedLead is asked at
		// the head of every field rather than at the head of the line alone,
		// since the stacked form draws that same field after a label.
		if lead, prefix, guided := guidedLead(string(runes[at:])); guided {
			start = column + lead
			text.WriteString(prefix)
			at += lead + len([]rune(prefix))
		} else {
			for at < len(runes) && runes[at] == ' ' {
				at++
			}
			start = columnAt(runes, at)
		}
		if at >= len(runes) {
			break
		}
		for at < len(runes) {
			if runes[at] != ' ' {
				text.WriteRune(runes[at])
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
			at++
		}
		fields = append(fields, columnarField{at: start, text: text.String()})
		// The gap after this field is left for the next iteration to
		// consume, rather than skipped here. A stacked tree row's value can
		// open with the guide's own run of blank continuation spaces, which
		// is content rather than gutter, and only guidedLead's own search
		// over how many of the leading spaces are gutter tells the two
		// apart. Skipping every space here first, the way an ordinary table
		// column would, feeds guidedLead a string that no longer carries the
		// blanks it needs and reads a deeper row's guide as flush with its
		// shallower sibling's.
	}
	return fields
}

// guidePieces are the four pieces a tree's guides are composed of, built from
// the same glyphs table.go draws them with rather than written out here.
func guidePieces() []string {
	return []string{
		guidePiece(guideTrunk, guideBlank),
		guidePiece(guideBlank, guideBlank),
		guidePiece(guideTrunk, guideRun),
		guidePiece(guideElbow, guideRun),
	}
}

// guidedLead reports where a tree row's first field begins, and the guide
// prefix that field carries, for a line that is one.
//
// A tree writes its guides into the row's first field, so the field carries
// spaces of its own and a scanner splitting on a run of two spaces would read
// one field as several and one row's indent as deeper than its neighbour's. A
// prefix always ends with the piece that joins the row to its parent, and no
// ordinary field ever ends that way, so the join is what tells a guided line
// from a line that merely begins with padding. The smallest lead that
// decomposes wins, since a deeper row's own prefix opens with blanks that
// would otherwise read as indent.
func guidedLead(line string) (int, string, bool) {
	runes := []rune(line)
	raw := 0
	for raw < len(runes) && runes[raw] == ' ' {
		raw++
	}
	for lead := 0; lead <= raw; lead++ {
		if prefix, ok := guideDecomposition(string(runes[lead:])); ok {
			return lead, prefix, true
		}
	}
	return 0, "", false
}

// guideDecomposition reads the run of whole guide pieces a string opens with,
// and reports it only when the last of them joins a row to its parent.
func guideDecomposition(text string) (string, bool) {
	pieces := guidePieces()
	joins := map[string]bool{
		guidePiece(guideTrunk, guideRun): true,
		guidePiece(guideElbow, guideRun): true,
	}
	runes := []rune(text)
	at, last := 0, ""
	for at+guideWidth <= len(runes) {
		piece := string(runes[at : at+guideWidth])
		matched := false
		for _, known := range pieces {
			if piece == known {
				matched = true
				break
			}
		}
		if !matched {
			break
		}
		at += guideWidth
		last = piece
	}
	if at == 0 || !joins[last] {
		return "", false
	}
	return string(runes[:at]), true
}

// columnAt is the display column a rune index begins in, measured over the
// whole prefix in one call rather than by adding one rune's width at a time.
//
// The running sum this replaces reports a column no terminal puts the field
// in, because a width is a property of a grapheme cluster rather than of a
// rune: a joined emoji sequence draws one glyph nine columns wide out of five
// runes measuring thirteen. It agreed with the renderer for as long as every
// field ahead of another was Latin, Han or Devanagari, where the two measures
// coincide, so a card title in any column but the last is what this measure
// has to get right.
func columnAt(runes []rune, index int) int {
	return displayWidth(string(runes[:index]))
}

// TestTheOutputCheckReportsAMisalignedBlock arms the check above. A check that
// passes proves nothing on its own, since it also passes when what it guards
// is absent, so this hands it a table padded by counting bytes and requires it
// to report exactly what is wrong.
//
// The three lines are what a padder counting bytes emits for a card listing
// carrying a Japanese title: the title measures ten display columns and counts
// as five characters, so the field after it starts five columns early.
//
// The last case records what the check cannot see rather than what it catches.
// A stacked block whose labels pad to the widest heading of each record rather
// than of the whole table puts the values of one block at two different display
// columns, and the check reports nothing: it reads each record as a two-column
// table of its own, finds that table's own values lined up, and the blank line
// between records closes its run before it can compare one against the next.
// The stacked form is asserted in the rendered-output sweep instead, where the
// declared heading keys of each block are known.
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

	stacked := []string{
		"  Card      demo-1",
		"  Standing  ready",
		"  Title     a card",
		"",
		"  Card   demo-2",
		"  Title  another card",
	}
	if found := foldedFindings(strings.Join(stacked, "\n")); len(found) != 0 {
		t.Errorf("this case records what the check cannot see and it now sees it, so the record is out of date:\n%s", strings.Join(found, "\n"))
	}
}

// foldedFindings folds an output into blocks and returns every finding the
// check reports over all of them.
func foldedFindings(out string) []string {
	var found []string
	for _, block := range foldColumnarBlocks(out) {
		found = append(found, block.findings()...)
	}
	return found
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

// parseTheRenderingHead parses every non-test Go source directly under this
// package and returns the fileset it read them with alongside the parsed files
// by base name. Three walks ask three different questions of the same sources
// and each wants a different subset of them, so the parse is shared here and
// the filtering stays with the caller that knows what its own question is
// about.
func parseTheRenderingHead() (*token.FileSet, map[string]*ast.File, error) {
	entries, err := os.ReadDir(theRenderingHeadDir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(theRenderingHeadDir, name), nil, 0)
		if err != nil {
			return nil, nil, err
		}
		files[name] = file
	}
	if len(files) == 0 {
		return nil, nil, errors.New("the walk scanned no source of the rendering head, so it proves nothing")
	}
	return fset, files, nil
}

// tableCallAt reports whether a node is a call to s.table or s.tableLines, and
// hands the call back when it is.
func tableCallAt(node ast.Node) (*ast.CallExpr, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (selector.Sel.Name != "table" && selector.Sel.Name != "tableLines") {
		return nil, false
	}
	return call, true
}

// tableSitesInSource walks this package's non-test sources and reports every
// call to s.table or s.tableLines, by file and line. table.go is left out: it
// is where the two live, and its own call from table to tableLines is not a
// site that draws a block.
//
// This is the positional half, and it stays positional on purpose. Both sides
// of the comparison it feeds are computed fresh on every run, one by this walk
// and one by the hook table.go arms, so no line number here is ever typed by
// hand and none can go stale. What sweptBlocks names is renderSite below.
func tableSitesInSource() (map[tableSite]bool, error) {
	fset, files, err := parseTheRenderingHead()
	if err != nil {
		return nil, err
	}
	sites := map[tableSite]bool{}
	for name, file := range files {
		if name == "table.go" {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := tableCallAt(node)
			if !ok {
				return true
			}
			sites[tableSite{file: name, line: fset.Position(call.Pos()).Line}] = true
			return true
		})
	}
	if len(sites) == 0 {
		return nil, errors.New("the walk found no table call site at all, so it proves nothing")
	}
	return sites, nil
}

// renderSite is one call to s.table or s.tableLines, named by the function it
// sits in and the name of the local identifier its argument resolves to (or,
// for a call whose argument is itself a call expression, the name of the
// function called). Two renderSites in the same function are distinguished by
// Label whenever their calls bind different locals, which survives any reorder
// of the calls; Ordinal exists only to give every site a total key and to let
// the checker detect the one case Label cannot resolve on its own (two
// same-function calls sharing both Label and declared columns), which
// assertNoAmbiguousSiblingSites treats as a failure rather than as a silently
// accepted collision. A renderSite survives any edit outside its own function;
// an edit inside it that adds, removes or renames a call is the case a human
// has to look at the entry again, which this key makes into a test failure
// rather than a silent repoint.
type renderSite struct {
	// File is the source's base name, relative to cmd/dinah.
	File string
	// Function is the enclosing declaration's own name.
	Function string
	// Label is the argument's resolved identifier name, or the called
	// function's name for the one-hop call-expression case. It is empty when
	// the argument is an inline composite literal, which no site in the tree
	// is today.
	Label string
	// Ordinal is the 1-based rank of this call among the calls sharing its
	// (File, Function, Label), in source order. It is 1 for every site in the
	// tree, since every one of them has a Label unique within its function.
	Ordinal int
}

// String renders a site the way sweptBlocks names one.
func (s renderSite) String() string {
	if s.Label != "" {
		return fmt.Sprintf("%s:%s.%s#%d", s.File, s.Function, s.Label, s.Ordinal)
	}
	return fmt.Sprintf("%s:%s#%d", s.File, s.Function, s.Ordinal)
}

// renderSiteInfo is what the walk reads about a site beyond its own identity:
// where it sits, for a diagnostic, and which catalog keys its columns
// declaration says its headings carry.
type renderSiteInfo struct {
	// Line is for a failure message and for nothing else. It is never typed
	// into an entry and never compared.
	Line int
	// Keys are the catalog keys derived from the site's own columns
	// declaration, in column order. They are nil for a listColumn site, which
	// is the one-column shape sweptBlock spells as no keys at all, and nil
	// when Derivable is false.
	Keys []string
	// Derivable says the columns declaration was one of the two shapes this
	// walk reads. A false here fails the run rather than dropping the site
	// out of the comparison.
	Derivable bool
	// Unresolved says what the walk found instead, and is empty when Derivable
	// is true.
	Unresolved string
}

// renderSitesInSource walks the same sources tableSitesInSource walks and
// reports every table call site by the key sweptBlocks names one under,
// together with the columns its own source declares.
//
// Calls within a function are sorted by position before ordinals are assigned.
// ast.Inspect walks the tree's structure rather than the file's lines, and
// while the two agree for every function in this package today, an ordinal
// resting on that agreement would be a key resting on something no
// documentation promises.
func renderSitesInSource() (map[renderSite]renderSiteInfo, error) {
	fset, files, err := parseTheRenderingHead()
	if err != nil {
		return nil, err
	}
	sites := map[renderSite]renderSiteInfo{}
	for name, file := range files {
		if name == "table.go" {
			continue
		}
		for _, declared := range file.Decls {
			function, ok := declared.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			var calls []*ast.CallExpr
			ast.Inspect(function.Body, func(node ast.Node) bool {
				if call, ok := tableCallAt(node); ok {
					calls = append(calls, call)
				}
				return true
			})
			sort.Slice(calls, func(i, j int) bool { return calls[i].Pos() < calls[j].Pos() })
			ranked := map[string]int{}
			for _, call := range calls {
				label, info := readRenderSite(fset, file, function, call)
				ranked[label]++
				sites[renderSite{File: name, Function: function.Name.Name, Label: label, Ordinal: ranked[label]}] = info
			}
		}
	}
	if len(sites) == 0 {
		return nil, errors.New("the walk found no table call site at all, so it proves nothing")
	}
	return sites, nil
}

// readRenderSite resolves one call's label and the columns its argument
// declares. Both answers come from one resolution of the argument, since the
// declaration that names the label is the declaration that carries the
// columns.
func readRenderSite(fset *token.FileSet, file *ast.File, function *ast.FuncDecl, call *ast.CallExpr) (string, renderSiteInfo) {
	info := renderSiteInfo{Line: fset.Position(call.Pos()).Line}
	if len(call.Args) != 1 {
		info.Unresolved = fmt.Sprintf("the call passes %d arguments, and a table call passes one", len(call.Args))
		return "", info
	}
	switch argument := call.Args[0].(type) {
	case *ast.Ident:
		literal := tableLiteralBoundTo(function.Body, argument.Name, call.Pos())
		if literal == nil {
			info.Unresolved = "the argument is the identifier " + argument.Name + ", and no table literal is assigned to that name in this function before the call"
			return argument.Name, info
		}
		return argument.Name, keysOfTableLiteral(literal, info)
	case *ast.CallExpr:
		name, callee := zeroArgumentCalleeInThisFile(file, argument)
		if callee == nil {
			info.Unresolved = "the argument is a call the walk resolves no declaration for, and it resolves one hop to a zero-argument function declared in the same file"
			return name, info
		}
		literal := tableLiteralReturnedBy(callee)
		if literal == nil {
			info.Unresolved = "the argument is a call to " + name + ", which returns no table literal the walk can read"
			return name, info
		}
		return name, keysOfTableLiteral(literal, info)
	case *ast.CompositeLit:
		return "", keysOfTableLiteral(argument, info)
	default:
		info.Unresolved = fmt.Sprintf("the argument is a %T, which is none of the shapes the walk reads", argument)
		return "", info
	}
}

// tableLiteralBoundTo finds the table literal assigned to a name in a function
// body before a given position, taking the nearest such assignment. Two
// branches of one function may each bind the same name to a table of their
// own, and the nearest preceding assignment is the one the call at that
// position is passing.
func tableLiteralBoundTo(body *ast.BlockStmt, name string, before token.Pos) *ast.CompositeLit {
	var found *ast.CompositeLit
	ast.Inspect(body, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || assign.Pos() >= before || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		target, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || target.Name != name {
			return true
		}
		literal, ok := assign.Rhs[0].(*ast.CompositeLit)
		if !ok {
			return true
		}
		if found == nil || literal.Pos() > found.Pos() {
			found = literal
		}
		return true
	})
	return found
}

// zeroArgumentCalleeInThisFile resolves one hop from a call expression to the
// zero-argument function declared in the same file, and reports the name it
// resolved whether or not a declaration was found, since the name is the label
// either way.
func zeroArgumentCalleeInThisFile(file *ast.File, call *ast.CallExpr) (string, *ast.FuncDecl) {
	var name string
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name = fun.Name
	case *ast.SelectorExpr:
		name = fun.Sel.Name
	default:
		return "", nil
	}
	if len(call.Args) != 0 {
		return name, nil
	}
	for _, declared := range file.Decls {
		function, ok := declared.(*ast.FuncDecl)
		if !ok || function.Name.Name != name || function.Body == nil {
			continue
		}
		if function.Type.Params != nil && len(function.Type.Params.List) != 0 {
			continue
		}
		return name, function
	}
	return name, nil
}

// tableLiteralReturnedBy finds the table literal a zero-argument function hands
// back, whether it returns the literal itself or a local it built the literal
// into.
func tableLiteralReturnedBy(callee *ast.FuncDecl) *ast.CompositeLit {
	var found *ast.CompositeLit
	ast.Inspect(callee.Body, func(node ast.Node) bool {
		returned, ok := node.(*ast.ReturnStmt)
		if !ok || len(returned.Results) != 1 {
			return true
		}
		switch result := returned.Results[0].(type) {
		case *ast.CompositeLit:
			found = result
		case *ast.Ident:
			if literal := tableLiteralBoundTo(callee.Body, result.Name, returned.Pos()); literal != nil {
				found = literal
			}
		}
		return true
	})
	return found
}

// keysOfTableLiteral reads a table literal's columns field and derives the
// catalog keys its headings carry. Two shapes are read, which is every shape
// the tree declares: s.columns with string literals throughout, and
// listColumn, which is the one-column block that takes no heading at all.
func keysOfTableLiteral(literal *ast.CompositeLit, info renderSiteInfo) renderSiteInfo {
	if name, ok := literal.Type.(*ast.Ident); !ok || name.Name != "table" {
		info.Unresolved = "the argument resolves to a composite literal that is not a table"
		return info
	}
	var columns ast.Expr
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := pair.Key.(*ast.Ident); ok && key.Name == "columns" {
			columns = pair.Value
		}
	}
	if columns == nil {
		info.Unresolved = "the table literal declares no columns field"
		return info
	}
	call, ok := columns.(*ast.CallExpr)
	if !ok {
		info.Unresolved = fmt.Sprintf("the columns field is a %T rather than a call to s.columns or to listColumn", columns)
		return info
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		if fun.Name == "listColumn" && len(call.Args) == 0 {
			info.Derivable = true
			return info
		}
	case *ast.SelectorExpr:
		if fun.Sel.Name == "columns" {
			return keysOfColumnsCall(call, info)
		}
	}
	info.Unresolved = "the columns field is a call the walk does not read, and it reads s.columns with string literals throughout and listColumn"
	return info
}

// keysOfColumnsCall spells the keys s.columns would build from the same
// arguments. It mirrors that helper's own construction, column.<block>.<name>,
// so a site's declared headings and its derived keys cannot drift apart
// without one of the two being edited.
func keysOfColumnsCall(call *ast.CallExpr, info renderSiteInfo) renderSiteInfo {
	if len(call.Args) < 2 {
		info.Unresolved = "s.columns is called with fewer arguments than a block and one name"
		return info
	}
	var spelled []string
	for _, argument := range call.Args {
		basic, ok := argument.(*ast.BasicLit)
		if !ok || basic.Kind != token.STRING {
			info.Unresolved = "s.columns carries an argument that is not a string literal, so its keys are not readable from the source"
			return info
		}
		unquoted, err := strconv.Unquote(basic.Value)
		if err != nil {
			info.Unresolved = "s.columns carries a string literal the walk cannot unquote: " + basic.Value
			return info
		}
		spelled = append(spelled, unquoted)
	}
	for _, name := range spelled[1:] {
		info.Keys = append(info.Keys, "column."+spelled[0]+"."+name)
	}
	info.Derivable = true
	return info
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
	sites, err := renderSitesInSource()
	if err != nil {
		t.Fatalf("walk the head's sources: %v", err)
	}
	assertNoAmbiguousSiblingSites(t, sites)
	named := map[renderSite]bool{}
	for _, block := range sweptBlocks() {
		named[block.site] = true
	}
	for site := range sites {
		if !named[site] {
			t.Errorf("%s draws a table and no entry of sweptBlocks names it, so nothing asserts what it renders", site)
		}
	}
	for site := range named {
		if _, drawn := sites[site]; !drawn {
			t.Errorf("sweptBlocks names %s and the source draws no table there, so the entry is stale and may be covering the wrong block", site)
		}
	}
	assertEveryCoveredEntryExpects(t)
	assertEveryRegisteredSiteMatchesItsColumns(t, sites)
}

// assertNoAmbiguousSiblingSites refuses any pair of call sites in one function
// that share a label and declare the same columns, which is the one shape the
// key cannot tell apart on its own.
//
// A label belongs to a declaration rather than to a position, so two calls
// binding different locals keep their own keys however the source is reordered
// around them. Two calls binding the same name are the exception, and if they
// also declare the same columns then nothing about either site distinguishes
// it from the other: an entry written for one would sit just as plausibly on
// the other, and a reorder would move both entries with nothing failing. The
// fix, whenever this fires, is to rename one of the two locals, which is what
// composeRefusal's carriedTable is.
//
// Two calls sharing a label and declaring different columns are left alone.
// The difference in columns is itself a fact the source carries, so their
// ordinals are honest rather than inferred from where the calls happen to sit,
// and assertEveryRegisteredSiteMatchesItsColumns holds each entry to the
// columns its own site declares.
func assertNoAmbiguousSiblingSites(t *testing.T, sites map[renderSite]renderSiteInfo) {
	t.Helper()
	type sibling struct {
		file     string
		function string
		label    string
	}
	groups := map[sibling][]renderSite{}
	for site := range sites {
		key := sibling{file: site.File, function: site.Function, label: site.Label}
		groups[key] = append(groups[key], site)
	}
	shared := 0
	for _, members := range groups {
		if len(members) < 2 {
			continue
		}
		shared++
		sort.Slice(members, func(i, j int) bool { return members[i].Ordinal < members[j].Ordinal })
		for i := range members {
			for j := i + 1; j < len(members); j++ {
				if !slices.Equal(sites[members[i]].Keys, sites[members[j]].Keys) {
					continue
				}
				t.Errorf("%s and %s are two calls in %s:%s binding the name %q and declaring the same columns %v, so neither site is distinguishable from the other and an entry naming one would sit as plausibly on the other; rename one of the two locals, the way composeRefusal's carriedTable is named",
					members[i], members[j], members[i].File, members[i].Function, members[i].Label, sites[members[i]].Keys)
			}
		}
	}
	t.Logf("%d call sites fall into %d (file, function, label) groups, %d of which hold more than one site", len(sites), len(groups), shared)
}

// assertEveryRegisteredSiteMatchesItsColumns holds every entry to the columns
// its own call site declares, so an entry that has landed on a neighbouring
// call in the same function is caught rather than read as valid.
//
// Membership alone never asked what an entry names. An entry could sit on a
// site drawing something else entirely and satisfy both directions of the
// pairing, which is the gap this closes: the keys an entry claims are compared
// against the keys the site's own s.columns call builds.
//
// The comparison is containment in order rather than equality, and the reason
// is withoutEmptyColumns in table.go: a column no row carries a value in is
// dropped from the drawn block, heading and all. An entry names the headings
// its own fixture draws, so a block whose declaration offers a column the
// fixture never fills draws fewer columns than the call site declares, in the
// declaration's own order. The two tree entries are that case today, since the
// tree table offers a Not shown column that neither fixture fills. Requiring
// equality would fail them for rendering correctly.
//
// Containment in order is weaker than equality and still closes the gap this
// assertion exists for. A key an entry claims that the site does not declare
// fails, so a stale entry that has landed on another call fails as soon as
// that call declares any different block, which every differently-named block
// in the tree does. What it cannot separate is two calls declaring the same
// columns, and nothing derived from columns could; Label is what separates
// those, and assertNoAmbiguousSiblingSites refuses the case where Label
// cannot.
//
// A site whose columns the walk cannot read fails the run rather than dropping
// out of the comparison. No site in the tree is shaped that way today, so this
// changes nothing about the current build; what it refuses is a later site
// quietly falling outside a check nobody would notice it had left. The way
// forward from such a failure is to make the declaration readable, or to widen
// what the walk reads, and either is a reviewed edit rather than a silence.
func assertEveryRegisteredSiteMatchesItsColumns(t *testing.T, sites map[renderSite]renderSiteInfo) {
	t.Helper()
	unreadable := make([]renderSite, 0, len(sites))
	for site, info := range sites {
		if !info.Derivable {
			unreadable = append(unreadable, site)
		}
	}
	sort.Slice(unreadable, func(i, j int) bool { return unreadable[i].String() < unreadable[j].String() })
	for _, site := range unreadable {
		t.Errorf("%s (line %d) declares columns this walk cannot read, so nothing checks that the entry naming it still names the right block: %s",
			site, sites[site].Line, sites[site].Unresolved)
	}
	for _, block := range sweptBlocks() {
		info, drawn := sites[block.site]
		if !drawn || !info.Derivable {
			continue
		}
		if len(block.keys) == 0 {
			if info.Keys != nil {
				t.Errorf("%s (%s) declares no columns and its call site declares %v, so the entry has landed on a call it does not describe",
					block.site, block.label, info.Keys)
			}
			continue
		}
		if !keysRunInOrderThrough(block.keys, info.Keys) {
			t.Errorf("%s (%s) declares the keys %v and its call site's own source declares %v, so either the entry has landed on a call it does not describe or its keys are stale",
				block.site, block.label, block.keys, info.Keys)
		}
	}
}

// keysRunInOrderThrough reports whether every key an entry declares appears
// among the keys its call site declares, in the same order. A drawn block's
// columns are its call site's columns with the ones no row filled taken out,
// so the entry's list runs through the site's list rather than matching it.
func keysRunInOrderThrough(declared, derived []string) bool {
	at := 0
	for _, key := range declared {
		for at < len(derived) && derived[at] != key {
			at++
		}
		if at == len(derived) {
			return false
		}
		at++
	}
	return true
}

// assertEveryCoveredEntryExpects holds every entry declaring two or more
// columns to carrying an expectation, so a table site a later card adds cannot
// reach the trunk with nothing asserting which value sits under which label.
//
// The condition is len(block.keys) < 2 rather than a list of names. An entry
// declaring one column prints a list under a sentence that already names it
// and has no second label for a value to sit under, and the covered set
// follows the inventory wherever a later card takes it.
func assertEveryCoveredEntryExpects(t *testing.T) {
	t.Helper()
	for _, block := range sweptBlocks() {
		if len(block.keys) < 2 || block.expect != nil {
			continue
		}
		t.Errorf("%s (%s) declares %d columns and carries no expectation, so nothing asserts that its values sit under their own labels",
			block.site, block.label, len(block.keys))
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
