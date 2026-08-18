package main

import (
	"go/ast"
	"go/parser"
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
	uncovered, err := uncoveredBlocks(profile, scope)
	if err != nil {
		t.Fatalf("read the profile: %v", err)
	}
	named, err := readUncoveredAllowlist()
	if err != nil {
		t.Fatalf("read %s: %v", uncoveredAllowlist, err)
	}
	for _, block := range uncovered {
		if _, ok := named[block]; !ok {
			t.Errorf("%s is executed by no test in this package and the allowlist does not name it; cover it, or name it in %s with the reason it cannot be reached", block, uncoveredAllowlist)
		}
	}
	held := map[string]bool{}
	for _, block := range uncovered {
		held[block] = true
	}
	for block, reason := range named {
		if !held[block] {
			t.Errorf("%s is named in %s and the suite executes it, so the entry is stale and has to be pruned", block, uncoveredAllowlist)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s is named in %s with no reason, and an entry is a place the output check does not look rather than a line of configuration", block, uncoveredAllowlist)
		}
	}
}

// uncoveredBlocks reads a coverage profile and reports every statement block of
// this package that no test executed, spelled file:start,end the way the
// allowlist spells one.
func uncoveredBlocks(profile string, scope func(file string, line int) bool) ([]string, error) {
	source, err := os.ReadFile(profile)
	if err != nil {
		return nil, err
	}
	var blocks []string
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
		if err != nil || count > 0 {
			continue
		}
		where := fields[0]
		at := strings.LastIndex(where, "/")
		if at >= 0 {
			where = where[at+1:]
		}
		file, span, found := strings.Cut(where, ":")
		if !found {
			continue
		}
		opens, err := strconv.Atoi(strings.SplitN(span, ".", 2)[0])
		if err != nil || !scope(file, opens) {
			continue
		}
		blocks = append(blocks, where)
	}
	sort.Strings(blocks)
	return blocks, nil
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
	entries, err := os.ReadDir(theRenderingHeadDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if theRenderingFiles[name] {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(theRenderingHeadDir, name), nil, 0)
		if err != nil {
			return nil, err
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

// readUncoveredAllowlist reads the allowlist into the block each entry names
// and the reason it gives. A line is a block, whitespace, and the reason; a
// blank line and a line opening with a hash are commentary.
func readUncoveredAllowlist() (map[string]string, error) {
	source, err := os.ReadFile(uncoveredAllowlist)
	if err != nil {
		return nil, err
	}
	named := map[string]string{}
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		block, reason, _ := strings.Cut(line, " ")
		named[block] = strings.TrimSpace(reason)
	}
	return named, nil
}
