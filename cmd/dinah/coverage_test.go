package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// coverageChildMarker is set in the environment of the run this test starts,
// so the child runs the suite and does not start a third one.
const coverageChildMarker = "DINAH_COVERAGE_PASS"

// uncoveredAllowlist is where the blocks no test reaches are named, one per
// line, each with the reason that block is unreachable. It ships in the tree
// beside the check that reads it.
var uncoveredAllowlist = filepath.Join("testdata", "uncovered.txt")

// TestEveryStatementOfTheRenderingHeadIsCoveredOrNamed reads the profile
// go test -coverprofile writes for this package and requires every statement
// block of it to be either executed under the suite or named individually in
// an allowlist that ships in the tree.
//
// This is what bounds the output check's one real blind spot, which is a block
// no test prints. A hand-rolled table in a branch no test reaches fails here.
// It cannot hide behind an aggregate percentage that nobody reads statement by
// statement, which is what a coverage floor invites and what an allowlist of
// named statements refuses.
//
// The allowlist is what this claim costs, and it is the honest part of it.
// Every entry is a place the output check does not look, so the entries are
// reviewed as findings rather than kept as configuration, and two assertions
// keep the pressure on: an entry naming a block that is covered fails, so a
// stale entry has to be pruned rather than accumulate, and an entry carrying
// no reason fails, so nobody adds one without saying why the block cannot be
// reached.
//
// The scope is every statement that could draw a table: the three files that
// hold the rendering, plus, in any other file, the functions that draw a table
// themselves. It is derived from the same AST walk the site registration uses
// rather than typed out, so a table drawn in a new file widens the scope on
// its own.
//
// That is narrower than every statement of the package and the limit is worth
// stating rather than discovering. The argument parser and the row renderer
// carry uncovered branches this check never reads. What covers them is not
// this: the walk that sets the scope finds every table call site in the
// package, so a table drawn in one of those files pulls its function into the
// scope, and the source guard's tenth and eleventh patterns refuse the measure
// and the stream mention a hand-rolled table needs wherever it is written. A
// correctly aligned hand-rolled table in a branch no test reaches remains
// outside all of it, which is the admission this card makes rather than
// papers over.
func TestEveryStatementOfTheRenderingHeadIsCoveredOrNamed(t *testing.T) {
	if os.Getenv(coverageChildMarker) != "" {
		t.Skip("this is the run that produces the profile, and it does not start another")
	}
	if !fullSuiteRan() {
		t.Skip("this run was filtered, so the profile would report the filter rather than the suite")
	}
	profile := filepath.Join(t.TempDir(), "cover.out")
	command := exec.Command("go", "test", "-count=1", "-coverprofile="+profile, ".")
	command.Env = append(os.Environ(), coverageChildMarker+"=1")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("the coverage run failed, so nothing below can be read: %v\n%s", err, out)
	}
	scope, err := coverageScope()
	if err != nil {
		t.Fatalf("read the head's sources: %v", err)
	}
	spans, err := renderingHeadFunctionSpans()
	if err != nil {
		t.Fatalf("read the head's function spans: %v", err)
	}
	read, err := uncoveredBlocks(profile, scope, spans)
	if err != nil {
		t.Fatalf("read the profile: %v", err)
	}
	named, err := readUncoveredAllowlist()
	if err != nil {
		t.Fatalf("read %s: %v", uncoveredAllowlist, err)
	}
	for _, site := range sortedStatementSites(read.uncovered) {
		if _, ok := named[site]; !ok {
			t.Errorf("%s (the profile spells it %s) is executed by no test in this package and the allowlist does not name it; cover it, or name it in %s with the reason it cannot be reached", site, read.uncovered[site], uncoveredAllowlist)
		}
	}
	for _, site := range sortedStatementSites(named) {
		span, held := read.uncovered[site]
		if !held {
			if covered, exists := read.ranked[site]; exists {
				t.Errorf("%s (the profile spells it %s) is named in %s and the suite executes it, so the entry is stale and has to be pruned", site, covered, uncoveredAllowlist)
				continue
			}
			t.Errorf("%s is named in %s and the profile ranks no block of that function there, so the entry names nothing; take the identifier from this check's own report of the block you meant", site, uncoveredAllowlist)
			continue
		}
		if strings.TrimSpace(named[site]) == "" {
			t.Errorf("%s (the profile spells it %s) is named in %s with no reason, and an entry is a place the output check does not look rather than a line of configuration", site, span, uncoveredAllowlist)
		}
	}
}

// sortedStatementSites orders a set of sites so that a run reports them the
// same way twice, since a map hands them back in whatever order it likes.
func sortedStatementSites(sites map[statementSite]string) []statementSite {
	ordered := make([]statementSite, 0, len(sites))
	for site := range sites {
		ordered = append(ordered, site)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	return ordered
}

// statementSite is one statement block reported by a coverage profile, named
// by the function it sits in and its rank among every block the profile
// reports for that function (covered and uncovered alike), ordered by (start
// line, start column) ascending. Ranking against every block rather than only
// the uncovered ones is what keeps an entry's ordinal from moving the day a
// sibling statement in the same function goes from uncovered to covered.
//
// statementSite carries no label the way renderSite does. A table call's
// identity survives a reorder of two source lines because the call is bound to
// a declared local that keeps its own name regardless of position; a coverage
// statement's identity is its position, full stop, because moving one
// uncovered statement above another is not a two-line swap of otherwise
// identical calls, it is rewriting the function's control flow, which is
// already a change a reviewer has to look at directly. The renderSite reorder
// defect exploited two declarations that could trade positions with no
// behavioural difference at all; two statement blocks cannot trade positions
// without the compiler and every existing test already asking why, since a
// statement's position is its behaviour.
type statementSite struct {
	File     string
	Function string
	Ordinal  int
}

// String renders a site the way the allowlist names one.
func (s statementSite) String() string {
	return fmt.Sprintf("%s:%s#%d", s.File, s.Function, s.Ordinal)
}

// parseStatementSite reads the identifier an allowlist line opens with, which
// is the identifier a failing run prints for the block it is reporting.
func parseStatementSite(text string) (statementSite, error) {
	file, rest, found := strings.Cut(text, ":")
	if !found || file == "" {
		return statementSite{}, fmt.Errorf("%q names no file, and an entry opens file:function#ordinal", text)
	}
	function, ordinal, found := strings.Cut(rest, "#")
	if !found || function == "" {
		return statementSite{}, fmt.Errorf("%q names no function, and an entry opens file:function#ordinal", text)
	}
	rank, err := strconv.Atoi(ordinal)
	if err != nil || rank < 1 {
		return statementSite{}, fmt.Errorf("%q carries no ordinal, and an entry opens file:function#ordinal", text)
	}
	return statementSite{File: file, Function: function, Ordinal: rank}, nil
}

// functionSpan is one declared function and the lines it runs between, which is
// what turns a profile block's start line into the function it sits in.
type functionSpan struct {
	Name   string
	Opens  int
	Closes int
}

// renderingHeadFunctionSpans reads every declaration this package writes
// statements inside, by file. Every file is read rather than only the ones
// outside theRenderingFiles, because the coverage scope needs spans only where
// it narrows and the statement key needs them everywhere.
//
// A function declaration is the ordinary case. A package-level variable whose
// value is built out of function literals is the other one, and refusalListings
// and refusalBlocks in render.go are both of that shape: the profile reports
// statements inside those literals, and no function declaration runs across
// them. Such a variable is named by its own declared name, which is as stable
// as a function's and reads the same way in an allowlist entry.
func renderingHeadFunctionSpans() (map[string][]functionSpan, error) {
	fset, files, err := parseTheRenderingHead()
	if err != nil {
		return nil, err
	}
	spans := map[string][]functionSpan{}
	for name, file := range files {
		for _, declared := range file.Decls {
			switch declaration := declared.(type) {
			case *ast.FuncDecl:
				if declaration.Body == nil {
					continue
				}
				spans[name] = append(spans[name], functionSpan{
					Name:   declaration.Name.Name,
					Opens:  fset.Position(declaration.Pos()).Line,
					Closes: fset.Position(declaration.End()).Line,
				})
			case *ast.GenDecl:
				if declaration.Tok != token.VAR && declaration.Tok != token.CONST {
					continue
				}
				for _, spec := range declaration.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok || len(value.Names) == 0 || !carriesFunctionLiteral(value) {
						continue
					}
					spans[name] = append(spans[name], functionSpan{
						Name:   value.Names[0].Name,
						Opens:  fset.Position(value.Pos()).Line,
						Closes: fset.Position(value.End()).Line,
					})
				}
			}
		}
	}
	return spans, nil
}

// carriesFunctionLiteral reports whether a declared value is built out of
// function literals, which is what makes it a place the profile reports
// statements from.
func carriesFunctionLiteral(value *ast.ValueSpec) bool {
	found := false
	ast.Inspect(value, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			found = true
		}
		return !found
	})
	return found
}

// functionAt reports the declared function a line sits in.
func functionAt(spans []functionSpan, line int) (string, bool) {
	for _, span := range spans {
		if line >= span.Opens && line <= span.Closes {
			return span.Name, true
		}
	}
	return "", false
}

// profileSites is what one profile says about this package: every in-scope
// statement block it reports, by the site each one is named under, and the
// subset of those no test executed. Both halves are kept because a stale
// allowlist entry has two different causes, and telling a reader which one it
// is costs one map: the block exists and the suite now reaches it, or no block
// of that function is ranked there at all.
type profileSites struct {
	ranked    map[statementSite]string
	uncovered map[statementSite]string
}

// profileBlock is one statement block of a coverage profile: where the profile
// says it sits, and how many times the suite ran it.
type profileBlock struct {
	file   string
	span   string
	opens  int
	column int
	count  int
}

// uncoveredBlocks reads a coverage profile and reports every statement block of
// this package that no test executed, keyed by the function it sits in and its
// rank among that function's blocks, with the profile's own raw span kept
// alongside for a failure message to print.
//
// Every in-scope block is ranked, not only the uncovered ones. A rank taken
// over the uncovered blocks alone would move every entry below a block the day
// a test started reaching that block, which is the churn this key exists to
// remove.
func uncoveredBlocks(profile string, scope func(file string, line int) bool, spans map[string][]functionSpan) (profileSites, error) {
	source, err := os.ReadFile(profile)
	if err != nil {
		return profileSites{}, err
	}
	var blocks []profileBlock
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		where := fields[0]
		if at := strings.LastIndex(where, "/"); at >= 0 {
			where = where[at+1:]
		}
		file, span, found := strings.Cut(where, ":")
		if !found {
			continue
		}
		start := strings.SplitN(span, ",", 2)[0]
		opens, err := strconv.Atoi(strings.SplitN(start, ".", 2)[0])
		if err != nil || !scope(file, opens) {
			continue
		}
		column := 0
		if _, after, found := strings.Cut(start, "."); found {
			if read, err := strconv.Atoi(after); err == nil {
				column = read
			}
		}
		blocks = append(blocks, profileBlock{file: file, span: where, opens: opens, column: column, count: count})
	}
	if len(blocks) == 0 {
		return profileSites{}, errors.New("the profile reported no statement block in scope at all, so it proves nothing")
	}
	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i].file != blocks[j].file {
			return blocks[i].file < blocks[j].file
		}
		if blocks[i].opens != blocks[j].opens {
			return blocks[i].opens < blocks[j].opens
		}
		return blocks[i].column < blocks[j].column
	})
	read := profileSites{uncovered: map[statementSite]string{}, ranked: map[statementSite]string{}}
	ranks := map[string]int{}
	for _, block := range blocks {
		function, found := functionAt(spans[block.file], block.opens)
		if !found {
			return profileSites{}, fmt.Errorf("the profile reports %s and no function this package declares runs across that line, so the block cannot be named", block.span)
		}
		key := block.file + ":" + function
		ranks[key]++
		site := statementSite{File: block.file, Function: function, Ordinal: ranks[key]}
		read.ranked[site] = block.span
		if block.count == 0 {
			read.uncovered[site] = block.span
		}
	}
	return read, nil
}

// theRenderingFiles are the files whose every statement is in scope, since
// every one of them exists to render.
var theRenderingFiles = map[string]bool{"render.go": true, "help.go": true, "table.go": true}

// coverageScope reports whether a statement block could draw a table, which is
// what the coverage check reads and what a hand-rolled table in an unreached
// branch would hide in.
//
// Every statement of the three rendering files is in scope. Elsewhere the
// scope is the function that draws a table, found through the same walk that
// registers the sites, so runGuide and reportAmbiguousWorkbench are in and the
// argument parser around them is not.
func coverageScope() (func(file string, line int) bool, error) {
	sites, err := tableSitesInSource()
	if err != nil {
		return nil, err
	}
	spans := map[string][][2]int{}
	fset, files, err := parseTheRenderingHead()
	if err != nil {
		return nil, err
	}
	for name, file := range files {
		if theRenderingFiles[name] {
			continue
		}
		for _, declared := range file.Decls {
			function, ok := declared.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			opens := fset.Position(function.Pos()).Line
			closes := fset.Position(function.End()).Line
			draws := false
			for site := range sites {
				if site.file == name && site.line >= opens && site.line <= closes {
					draws = true
					break
				}
			}
			if draws {
				spans[name] = append(spans[name], [2]int{opens, closes})
			}
		}
	}
	return func(file string, line int) bool {
		if theRenderingFiles[file] {
			return true
		}
		for _, span := range spans[file] {
			if line >= span[0] && line <= span[1] {
				return true
			}
		}
		return false
	}, nil
}

// readUncoveredAllowlist reads the allowlist into the statement site each entry
// names and the reason it gives. A line is a site, whitespace, and the reason;
// a blank line and a line opening with a hash are commentary.
//
// A line whose identifier will not parse is an error rather than a skipped
// line. An unreadable entry that was quietly dropped would read exactly like
// an entry nobody had written, so the block it names would be reported as
// unnamed and the reason it carries would be lost.
func readUncoveredAllowlist() (map[statementSite]string, error) {
	source, err := os.ReadFile(uncoveredAllowlist)
	if err != nil {
		return nil, err
	}
	named := map[statementSite]string{}
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		spelled, reason, _ := strings.Cut(line, " ")
		site, err := parseStatementSite(spelled)
		if err != nil {
			return nil, err
		}
		named[site] = strings.TrimSpace(reason)
	}
	return named, nil
}
