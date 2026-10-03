package bench

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// oldLayoutNames are the three names of the layout before the card-unit
// format: a comment's anchor file, an item's anchor file, and the directory a
// card's items stood in. They stay declared, because the code that reads and
// migrates the old layout needs them, and nothing else may name them.
var oldLayoutNames = map[string]bool{"CommentAnchor": true, "ItemAnchor": true, "ChecklistDir": true}

// oldLayoutReaders are the files that may name the old layout, each with the
// reason it has to. Paths are relative to this package's own directory,
// slash separated.
var oldLayoutReaders = map[string]string{
	"storagemigrate.go":                     "reads the old layout for the storage migration, and holds the legacy member writers a store below the card-unit format is written with",
	"migratestorage_phases.go":              "holds the storage migration's phases, which read the old layout, carry its files and remove them",
	"designationmigrate.go":                 "runs the designation conversion, which reads a store in the old layout",
	"../../cmd/dinah-migrate-notes/main.go": "converts the notes of a store older still, in the old layout",
	"../resident/residenttest/fixture.go":   "builds the resident snapshot's test store, which a build with the card-unit layout switched off writes in the old layout, and finds its item and item-comment anchors to change them",
}

// TestNothingButTheOldLayoutsReadersNamesIt drives dinah-637/criteria/17. It
// parses every non-test Go file of the repository outside oldLayoutReaders
// and finds no identifier naming CommentAnchor, ItemAnchor or ChecklistDir
// outside the declarations of the three, and it says how many files it
// scanned, so a walk that reached nothing cannot pass.
//
// Arming: writing `_ = bench.ChecklistDir` into a file of internal/verb turns
// it red, naming the file and the line.
func TestNothingButTheOldLayoutsReadersNamesIt(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	exempt := map[string]bool{}
	for path := range oldLayoutReaders {
		absolute, err := filepath.Abs(filepath.FromSlash(path))
		if err != nil || !Exists(absolute) {
			t.Errorf("oldLayoutReaders names %s, which does not exist", path)
			continue
		}
		exempt[absolute] = true
	}
	scanned := 0
	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if strings.HasPrefix(name, ".") || name == "testdata" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		if exempt[path] {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		scanned++
		// The declarations themselves are the one mention every file may
		// carry, so the names a value spec declares are set aside first.
		declared := map[*ast.Ident]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			if spec, ok := node.(*ast.ValueSpec); ok {
				for _, name := range spec.Names {
					declared[name] = true
				}
			}
			return true
		})
		ast.Inspect(file, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if ok && oldLayoutNames[ident.Name] && !declared[ident] {
				t.Errorf("%s names %s, which only the old layout's readers may", fset.Position(ident.Pos()), ident.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if scanned < 100 {
		t.Fatalf("the walk scanned %d files, which is too few to be the repository", scanned)
	}
	t.Logf("scanned %d non-test Go files outside the %d files that read the old layout", scanned, len(oldLayoutReaders))
}
