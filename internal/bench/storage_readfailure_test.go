package bench

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// plantUnreadable writes a plain file where a directory belongs and proves the
// plant took, which is the fixture every read-failure test on this card uses.
//
// It proves its own fixture rather than trusting it. A fixture that quietly
// produced an empty directory would make every assertion resting on it
// vacuous, and a fixture reading cleanly looks exactly like one that does not.
func plantUnreadable(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir above %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("plant a file at %s: %v", path, err)
	}
	if _, err := os.ReadDir(path); err == nil {
		t.Fatalf("the fixture at %s reads cleanly, so this test proves nothing", path)
	}
	return path
}

// TestListIDsSeparatesAnAbsentCollectionFromAnUnreadableOne pins all three
// cases in one run: a collection that is there and will not read answers the
// error, and an absent one and a real empty one both answer no identifiers and
// no error, which is the absent-means-empty rule dinah-455 made load-bearing.
//
// The two succeeding cases are what stop a build that answered an error for
// everything from passing this test.
func TestListIDsSeparatesAnAbsentCollectionFromAnUnreadableOne(t *testing.T) {
	root := t.TempDir()
	unreadable := plantUnreadable(t, filepath.Join(root, "cards-unreadable"))
	absent := filepath.Join(root, "cards-absent")
	empty := filepath.Join(root, "cards-empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", empty, err)
	}

	if ids, err := ListIDs(unreadable); err == nil {
		t.Fatalf("ListIDs read a plain file as an empty collection, answering %v with no error", ids)
	}

	ids, err := ListIDs(absent)
	if err != nil {
		t.Fatalf("ListIDs answered an error for an absent collection, which is an empty one: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("an absent collection listed %v", ids)
	}

	ids, err = ListIDs(empty)
	if err != nil {
		t.Fatalf("ListIDs answered an error for a real empty directory: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("an empty collection listed %v", ids)
	}
}

// TestEveryCollectionListingReadsThroughTheOneReader asserts that
// readCollection is the one place in this package that decides whether a
// directory-read failure means absence, and that the functions reading
// through it are exactly the seven this card routes there.
//
// The set is named rather than counted, so a function added to it and a
// function dropped from it both redden. The succeeding case is pinned in the
// same run: readCollection itself makes exactly one os.ReadDir call and
// exactly one os.Stat call, so a build that simply deleted the classification
// everywhere fails rather than passing.
func TestEveryCollectionListingReadsThroughTheOneReader(t *testing.T) {
	wanted := []string{
		"ListIDs",
		"ListWorkbenchIDs",
		"checkAttachmentFilename",
		"heldLocks",
		"interruptions",
		"listIdentifiers",
		"resumableLift",
	}

	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob the package: %v", err)
	}
	var readers []string
	classifiers := map[string]string{}
	readCalls, statCalls := 0, 0
	examined := 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Body == nil {
				continue
			}
			examined++
			readsThrough := false
			readDirLocal := 0
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}
				switch callee := call.Fun.(type) {
				case *ast.Ident:
					if callee.Name == "readCollection" {
						readsThrough = true
					}
				case *ast.SelectorExpr:
					pkg, named := callee.X.(*ast.Ident)
					if !named {
						return true
					}
					if pkg.Name == "os" && callee.Sel.Name == "ReadDir" {
						readDirLocal++
						if fn.Name.Name == "readCollection" {
							readCalls++
						}
					}
					if pkg.Name == "os" && callee.Sel.Name == "Stat" && fn.Name.Name == "readCollection" {
						statCalls++
					}
					if pkg.Name == "os" && (callee.Sel.Name == "IsNotExist" || callee.Sel.Name == "IsPermission") && readDirLocal > 0 {
						classifiers[fn.Name.Name] = name
					}
				}
				return true
			})
			if readsThrough && fn.Name.Name != "readCollection" {
				readers = append(readers, fn.Name.Name)
			}
		}
	}
	t.Logf("%d functions examined across the package's non-test files", examined)
	if examined == 0 {
		t.Fatal("the sweep examined no functions at all, so it proves nothing")
	}

	sort.Strings(readers)
	if strings.Join(readers, ",") != strings.Join(wanted, ",") {
		t.Errorf("the functions reading through readCollection are %v, wanted %v", readers, wanted)
	}
	for name, file := range classifiers {
		if name == "readCollection" {
			continue
		}
		t.Errorf("%s in %s classifies os.ReadDir's own error, which is the platform trap readCollection exists to hold", name, file)
	}
	if readCalls != 1 {
		t.Errorf("readCollection makes %d os.ReadDir calls, wanted exactly one", readCalls)
	}
	if statCalls != 1 {
		t.Errorf("readCollection makes %d os.Stat calls, wanted exactly one", statCalls)
	}
}
