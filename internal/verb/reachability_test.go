package verb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// PinnedCallSite is one place msg.For is called with a fixed language
// instead of the caller's own. Every one the parser finds has to be entered
// here, with a reason, or the guard below fails, which turns an invisible
// English-only surface into a line somebody reviewed and signed off on.
type PinnedCallSite struct {
	// File is the call site's path from the repository root, spelled with
	// forward slashes so the entry reads the same on either platform.
	File string
	// Line is the line the call starts on.
	Line int
	// Reason says why this surface is allowed to answer in one language.
	Reason string
}

// declaredPinnedCallSites is the reviewed inventory, seeded with the three
// call sites Agent Code Review found on dinah-245.
var declaredPinnedCallSites = []PinnedCallSite{
	{File: "internal/mcp/mcp.go", Line: 202, Reason: "the MCP surface is deliberately pinned to English; dinah-245"},
	{File: "internal/mcp/tools.go", Line: 200, Reason: "the MCP surface is deliberately pinned to English; dinah-245"},
	{File: "internal/mcp/tools.go", Line: 269, Reason: "the MCP surface is deliberately pinned to English; dinah-245"},
}

// scanDirs are the directories this guard parses, named relative to this
// package's own directory. internal/msg is excluded: every msg.For call there
// is the renderer's own fallback logic rather than a caller choosing a
// language.
var scanDirs = []string{"../../cmd/dinah", "../../internal/mcp", "../../internal/verb"}

// TestEveryLanguagePinnedCallSiteIsDeclared asserts that the set of msg.For
// calls pinned to one language is exactly the set declared above. A new
// pinned call site fails until somebody enters it with a reason, and a
// declared entry the parser no longer finds fails as a stale exemption, so
// the inventory cannot go on asserting after the surface it describes moves.
//
// This is not full reachability. A key can still be unreachable in a language
// for a reason no msg.For call site shows, and finding that would need call
// graph analysis this guard does not attempt. What it does is stop the pinned
// set growing where nobody looks.
func TestEveryLanguagePinnedCallSiteIsDeclared(t *testing.T) {
	found := findPinnedCallSites(t)
	declared := map[string]bool{}
	for _, site := range declaredPinnedCallSites {
		declared[site.File+":"+strconv.Itoa(site.Line)] = true
	}
	seen := map[string]bool{}
	for _, site := range found {
		key := site.File + ":" + strconv.Itoa(site.Line)
		seen[key] = true
		if declared[key] {
			continue
		}
		t.Errorf("%s:%d: msg.For is called with a fixed language and is not in declaredPinnedCallSites; parameterize it on the caller's language, or add it there with a reason", site.File, site.Line)
	}
	for key := range declared {
		if seen[key] {
			continue
		}
		t.Errorf("declaredPinnedCallSites carries %s, but the parser found no pinned msg.For call there any more; remove the stale entry", key)
	}
}

// findPinnedCallSites parses every non-test Go file under scanDirs and
// returns each msg.For call whose one argument is a string literal or
// msg.Base, sorted by file and line.
func findPinnedCallSites(t *testing.T) []PinnedCallSite {
	var out []PinnedCallSite
	fset := token.NewFileSet()
	for _, dir := range scanDirs {
		walk := func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			out = append(out, pinnedCallsIn(t, fset, file, path)...)
			return nil
		}
		if err := filepath.WalkDir(dir, walk); err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// pinnedCallsIn returns the pinned msg.For calls one parsed file makes.
func pinnedCallsIn(t *testing.T, fset *token.FileSet, file *ast.File, path string) []PinnedCallSite {
	var out []PinnedCallSite
	rel := repoPath(t, path)
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "For" {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "msg" {
			return true
		}
		line := fset.Position(call.Pos()).Line
		switch arg := call.Args[0].(type) {
		case *ast.BasicLit:
			out = append(out, PinnedCallSite{File: rel, Line: line, Reason: "msg.For called with a string literal"})
		case *ast.SelectorExpr:
			if arg.Sel.Name == "Base" {
				out = append(out, PinnedCallSite{File: rel, Line: line, Reason: "msg.For called with msg.Base"})
			}
		}
		return true
	})
	return out
}

// repoPath turns a path the walk produced, which is relative to this
// package's own directory, into the repository-root-relative form
// declaredPinnedCallSites spells, so an entry reads the same however the
// walk reached the file.
func repoPath(t *testing.T, path string) string {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("resolving %s: %v", path, err)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		t.Fatalf("relating %s to %s: %v", abs, root, err)
	}
	return filepath.ToSlash(rel)
}
