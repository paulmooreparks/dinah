package durable

// The guard below holds every non-test package of the module except this one
// and the four named in excludedFromGuard to durable's primitives: it
// type-checks each package and fails on every identifier the type checker
// resolves to a forbidden function, whether it is called, stored, passed or
// selected through an aliased import.
//
// What it cannot see is a third-party library opening a path Dinah hands it.
// Such a call is inside the library's own package, which the guard does not
// check, and resolves to nothing in the module's own source.

import (
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// forbiddenFunctions are the functions no package but this one may use, keyed
// by import path, each set holding the function names.
var forbiddenFunctions = map[string]map[string]bool{
	"os": setOf("Open", "OpenFile", "ReadFile", "WriteFile", "Create", "CreateTemp",
		"Rename", "Remove", "RemoveAll", "Truncate", "DirFS"),
	"io/ioutil":                setOf("ReadFile", "WriteFile", "TempFile"),
	"syscall":                  setOf("Open", "CreateFile", "Rename", "Unlink", "DeleteFile", "MoveFile"),
	"golang.org/x/sys/windows": setOf("CreateFile", "MoveFile", "MoveFileEx", "DeleteFile"),
	"golang.org/x/sys/unix":    setOf("Open", "Openat", "Rename", "Renameat", "Unlink", "Unlinkat"),
}

// excludedFromGuard are the packages the guard leaves out, each with its
// reason, beside this package itself.
var excludedFromGuard = map[string]string{
	"dinah/internal/durable":          "it is where the primitives live",
	"dinah/cmd/dinah-release":         "it writes release notes and archives inside the repository checkout and never a workbench",
	"dinah/internal/screen":           "it reads the terminfo database",
	"dinah/internal/testenv":          "it is test support that writes its own logs under the test's temporary directory",
	"dinah/internal/bench/compattest": "it reads a manifest checked in under testdata and is imported only by tests",
}

// guardTargets are the platforms the guard checks the module for.
var guardTargets = []string{"windows", "linux", "darwin"}

// minimumGuardedPackages is the fewest packages a target may check, so that a
// listing that silently returned nothing, or returned part of the module,
// fails rather than passing. It is the number every target checks on the
// commit that added the guard: the module lists 32 packages, and the five
// excluded above leave 27. A package added later raises the count past it,
// and removing one is the edit that lowers it.
const minimumGuardedPackages = 27

// setOf builds a set from names.
func setOf(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// violation is one use of a forbidden function.
type violation struct {
	file     string
	line     int
	function string
}

// String names the file, the line and the function.
func (v violation) String() string {
	return v.file + ":" + strconv.Itoa(v.line) + ": uses " + v.function
}

// forbiddenUses type-checks one package's files and answers every identifier
// resolving to a forbidden function.
func forbiddenUses(fset *token.FileSet, files []*ast.File, path string, imp types.Importer) ([]violation, error) {
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
	config := types.Config{Importer: imp}
	if _, err := config.Check(path, fset, files, info); err != nil {
		return nil, err
	}
	var found []violation
	for ident, object := range info.Uses {
		function, ok := object.(*types.Func)
		if !ok || function.Pkg() == nil {
			continue
		}
		names := forbiddenFunctions[function.Pkg().Path()]
		if !names[function.Name()] {
			continue
		}
		position := fset.Position(ident.Pos())
		found = append(found, violation{
			file:     position.Filename,
			line:     position.Line,
			function: function.Pkg().Path() + "." + function.Name(),
		})
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].file != found[j].file {
			return found[i].file < found[j].file
		}
		return found[i].line < found[j].line
	})
	return found, nil
}

// listedPackage is one line of go list's answer.
type listedPackage struct {
	path  string
	dir   string
	files []string
}

// listModule lists the module's packages for one target.
func listModule(t *testing.T, root, goos string) []listedPackage {
	t.Helper()
	format := `{{.ImportPath}} {{.Dir}} {{join .GoFiles " "}}`
	command := exec.Command("go", "list", "-f", format, "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=amd64", "CGO_ENABLED=0")
	out, err := command.Output()
	if err != nil {
		t.Fatalf("go list for %s: %v", goos, err)
	}
	var listed []listedPackage
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		listed = append(listed, listedPackage{path: fields[0], dir: fields[1], files: fields[2:]})
	}
	return listed
}

// TestNoPackageOutsideDurableUsesAFilePrimitive asserts that, for Windows,
// Linux and macOS, no package of the module but durable and the four named in
// excludedFromGuard uses a function the concurrency contract forbids, as the
// type checker resolves each use, and that each target checked at least
// minimumGuardedPackages packages.
func TestNoPackageOutsideDurableUsesAFilePrimitive(t *testing.T) {
	if testing.Short() {
		t.Skip("the guard type-checks the module three times from source")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve the module root: %v", err)
	}
	saved := build.Default
	t.Cleanup(func() { build.Default = saved })
	for _, goos := range guardTargets {
		build.Default.GOOS = goos
		build.Default.GOARCH = "amd64"
		build.Default.CgoEnabled = false
		fset := token.NewFileSet()
		imp := importer.ForCompiler(fset, "source", nil)
		packages, files := 0, 0
		for _, listed := range listModule(t, root, goos) {
			if _, excluded := excludedFromGuard[listed.path]; excluded || len(listed.files) == 0 {
				continue
			}
			parsed := make([]*ast.File, 0, len(listed.files))
			for _, name := range listed.files {
				file, err := parser.ParseFile(fset, filepath.Join(listed.dir, name), nil, 0)
				if err != nil {
					t.Fatalf("%s: parse %s: %v", goos, name, err)
				}
				parsed = append(parsed, file)
			}
			found, err := forbiddenUses(fset, parsed, listed.path, imp)
			if err != nil {
				t.Fatalf("%s: type-check %s: %v", goos, listed.path, err)
			}
			for _, use := range found {
				t.Errorf("%s: %s", goos, use)
			}
			packages++
			files += len(parsed)
		}
		t.Logf("%s: checked %d packages and %d files", goos, packages, files)
		if packages < minimumGuardedPackages {
			t.Errorf("%s: checked %d packages, fewer than %d, so the listing returned less than the module", goos, packages, minimumGuardedPackages)
		}
	}
}

// plantedUses are the shapes the guard is armed against, each a file that
// uses a forbidden function one way, with the line and function it has to name.
var plantedUses = []struct {
	name     string
	source   string
	line     int
	function string
}{
	{
		name:     "a call",
		source:   "package planted\n\nimport \"os\"\n\nfunc move() error {\n\treturn os.Rename(\"a\", \"b\")\n}\n",
		line:     6,
		function: "os.Rename",
	},
	{
		name:     "a function value",
		source:   "package planted\n\nimport \"os\"\n\nvar x = os.Rename\n",
		line:     5,
		function: "os.Rename",
	},
	{
		name:     "an aliased import",
		source:   "package planted\n\nimport files \"os\"\n\nfunc read() ([]byte, error) {\n\treturn files.ReadFile(\"a\")\n}\n",
		line:     6,
		function: "os.ReadFile",
	},
	{
		name:     "os.DirFS",
		source:   "package planted\n\nimport (\n\t\"io/fs\"\n\t\"os\"\n)\n\nfunc root() fs.FS {\n\treturn os.DirFS(\".\")\n}\n",
		line:     9,
		function: "os.DirFS",
	},
}

// TestTheGuardNamesEachPlantedUse asserts that the guard's own check turns
// red on each shape it is armed against, naming the file, the line and the
// function, and stays quiet on a file that uses only durable's permitted
// neighbours.
func TestTheGuardNamesEachPlantedUse(t *testing.T) {
	for _, planted := range plantedUses {
		t.Run(planted.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "planted.go", planted.source, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			found, err := forbiddenUses(fset, []*ast.File{file}, "planted", importer.Default())
			if err != nil {
				t.Fatalf("type-check: %v", err)
			}
			if len(found) != 1 {
				t.Fatalf("wanted one violation, got %v", found)
			}
			want := violation{file: "planted.go", line: planted.line, function: planted.function}
			if found[0] != want {
				t.Errorf("wanted %s, got %s", want, found[0])
			}
		})
	}
	clean := "package planted\n\nimport \"os\"\n\nfunc look() (bool, error) {\n\t_, err := os.Stat(\"a\")\n\treturn err == nil, os.MkdirAll(\"b\", 0o755)\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "clean.go", clean, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	found, err := forbiddenUses(fset, []*ast.File{file}, "planted", importer.Default())
	if err != nil {
		t.Fatalf("type-check: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("os.Stat and os.MkdirAll are permitted, and the guard named %v", found)
	}
}
