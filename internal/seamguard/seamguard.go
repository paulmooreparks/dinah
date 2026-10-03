// Package seamguard is the parse that dinah-619's two read-seam guards share:
// the bench guard (internal/bench, TestTheBenchReadsOnlyThroughItsSource) and
// the library guard (internal/verb, TestTheLibraryReadsOnlyThroughTheBench).
// Only tests import it: those two, and dinah-640's guard in internal/durable,
// which uses its listing importer from the external test package
// durable_test. The packages of Dinah's it imports are internal/durable and
// internal/shipped, which import none, so package bench can use it from its
// own tests without a cycle.
//
// Every guard resolves imports through List and ListedImporter: one
// `go list -deps -json` run per configuration answers every package and its
// files, where go/build in module mode starts a go list process for each
// import path it resolves outside GOROOT, which was most of what the guards
// cost.
//
// The seam guards stand beside dinah-640's guard in internal/durable
// (TestNoPackageOutsideDurableUsesAFilePrimitive), which refuses every open,
// write, rename and removal outside durable, and they judge only what that
// guard leaves open. Its allowlist refuses every function of io/ioutil that
// takes a path, and os.OpenFile, os.Open, os.ReadFile and every os write,
// in every package but durable, so neither io/ioutil nor those os members
// need a judgement here, and the flag-value rule that once told a write-only
// os.OpenFile from a read is gone with them. What durable's guard permits and
// these guards still judge a read: os.ReadDir, os.Stat, os.Lstat and the
// other os functions it permits, the walkers of io/fs and path/filepath,
// syscall, whose list of refusals in durable's guard is shorter than an
// allowlist, and durable's own readers, Open, ReadFile, OpenDir and the rest,
// which that guard exists to route every read through.
//
// Each guard loads its package once for every file set the configurations in
// Shipped select, so a file a build constraint keeps out of one binary is
// still judged under the configuration that builds it, and each guard asserts
// that the files it loaded are every non-test Go file of its directory.
//
// Both guards judge a read by what the named thing is, through go/types, and
// never by how it is spelled. A use is resolved to the object Info.Uses or
// Info.Selections gives it, so an import renamed or dotted, a method value, a
// method expression, a method promoted through an embedded field and a type
// alias all reach the object they denote. The operator ruled so on 2026-09-27,
// after a name-keyed form of these guards was walked past by an alias of
// Disk, a struct embedding it, and a method of another type sharing the
// seam's name (the corpus entry "A guard that parses the construct and then
// matches an identifier's name inside it", docs/practice/
// convention-counterexamples-3.md).
//
// The rules a guard applies are these, each stated where it is implemented.
// The member rule (Classify) judges every member of the packages Judged
// names, and every method of a type they declare, a read unless Allowed names
// it. The bottom
// rule (OfTheBottom, HoldsTheBottom) finds every expression whose type is the
// seam's bottom, Disk, or holds it in a field at any depth. The outside-Go
// rule (OutsideGo) refuses every //go:linkname directive and every function
// declared without a body, since the reads of code written outside Go are
// invisible to any graph of Go syntax.
//
// What stays unseen, in both guards, each with the plant that reproduces it.
// A guard's own header lists its share and names the plant.
//
//  1. A write used as a read, os.IsExist(os.Mkdir(p, 0o755)), because writes
//     are allowed. Plants: bench residue/mkdirprobe.go, verb
//     residue/mkdirprobe.go.
//  2. A read delegated to a package that is neither judged nor the one being
//     walked: another Dinah package's helper, a third-party library, or
//     os/exec running a program. Plants: bench residue/delegated.go, verb
//     residue/delegated.go, both through os/exec.
//  3. A handle of a judged type converted to an interface of a package that is
//     not judged and read through that interface (io.Reader over an
//     *os.File), because such conversions are also how every write is made.
//     Plant: bench residue/ioreader.go.
//  4. In package bench, a Disk handed as an argument to a Source parameter of
//     a function that stores it, by code no root reaches. Plant: bench
//     residue/handedsource.go.
//  5. A second read of the same member inside an exempt function, because an
//     exemption is keyed on the function and the member together. Plants:
//     bench residue/samepair.go and verb residue/samepair.go.
//  6. In package verb, a Source that reaches verb under a static type in
//     which no Source-shaped type appears, such as any, and is then read by
//     reflection, since reflect is not judged. Plant: verb
//     residue/reflectsource.go.
//  7. A read through golang.org/x/sys/windows or golang.org/x/sys/unix, which
//     neither guard judges beyond the opens durable's guard refuses: a method
//     of *Bench calling windows.FindFirstFile(pattern, &data) lists a
//     directory and passes both. No plant reproduces it, because a plant is
//     loaded outside the build constraints that select it, and a file
//     importing either package type-checks on one family of platforms only.
package seamguard

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"dinah/internal/durable"
	"dinah/internal/shipped"
)

// DurablePath is the import path of the package every open, write, rename and
// removal of a workbench file goes through (dinah-640).
const DurablePath = "dinah/internal/durable"

// Judged are the import paths whose members, and the methods of whose types,
// are reads unless Allowed names them: the standard library's filesystem
// surface that durable's guard permits outside durable, the raw system calls,
// and durable itself. io/ioutil is not judged, because durable's guard
// refuses every one of its functions that takes a path.
var Judged = []string{"os", "io/fs", "path/filepath", "syscall", DurablePath}

// Allowed names, for each judged package, the members that read no file, each
// with its reason. A method is written as its type and its name, as
// "File.Close". Every name in it is one package bench or package verb uses
// today on some platform; a member neither uses stays out, so its first use is
// refused and has to be argued for here. os's writes are not listed, because
// durable's guard refuses each of them outside durable.
var Allowed = map[string]map[string]string{
	"os": {
		"Mkdir":           "creates a directory",
		"MkdirAll":        "creates directories",
		"IsExist":         "classifies an error already returned",
		"IsNotExist":      "classifies an error already returned",
		"IsPathSeparator": "classifies a byte",
		"SameFile":        "compares two FileInfo values, which on Windows opens each file by its path to read its identity when the FileInfo does not already carry one, so it is a read only of files a caller has just stat-ed; every use pairs it with an os.Stat that is judged where it is named",
		"ErrNotExist":     "an error value",
		"DirEntry":        "a type",
		"File":            "the handle type durable hands out; the open that makes one is judged where it is named, and each method that reads through it is judged by its own name",
		"File.Close":      "releases a handle",
		"LinkError":       "a type",
		"ModeSymlink":     "a mode bit",
		"Executable":      "the running program's own path, never a path below a workbench",
		"Getenv":          "the process environment",
		"Getpid":          "the process's own identifier",
		"Hostname":        "the machine's own name, never a path below a workbench",
		"UserHomeDir":     "the home directory's path, read from the environment",
	},
	"io/fs": {
		"DirEntry":           "a type; each of its methods is judged by its own name",
		"DirEntry.Info":      "answers for an entry a source listed, which a resident snapshot answers from memory and Disk as os.ReadDir's own entry does",
		"DirEntry.IsDir":     "answers for an entry a source listed",
		"DirEntry.Name":      "answers for an entry a source listed",
		"DirEntry.Type":      "answers for an entry a source listed",
		"FileInfo":           "a type; each of its methods is judged by its own name",
		"FileInfo.IsDir":     "answers for an info a source stat-ed",
		"FileInfo.Mode":      "answers for an info a source stat-ed",
		"FileInfo.Size":      "answers for an info a source stat-ed",
		"FileInfo.Sys":       "answers for an info already stat-ed",
		"FileMode.IsRegular": "classifies a mode already read",
		"ErrExist":           "an error value",
		"ErrNotExist":        "an error value",
	},
	"path/filepath": {
		"Abs":       "joins a path with the working directory without touching the file it names",
		"Base":      "lexical",
		"Clean":     "lexical",
		"Dir":       "lexical",
		"Ext":       "lexical",
		"IsLocal":   "lexical",
		"Join":      "lexical",
		"Rel":       "lexical",
		"Separator": "a constant",
		"SkipDir":   "an error value a walk callback answers",
		"ToSlash":   "lexical",
	},
	"syscall": {
		"ESRCH":  "an error number",
		"EXDEV":  "an error number",
		"Kill":   "signals a process, here with signal 0 to ask whether it exists",
		"Stat_t": "the type a FileInfo's Sys answers on Linux",
	},
	DurablePath: {
		"AppendLine":      "appends a line to a journal",
		"BusyError":       "a type",
		"CloseLockFile":   "releases a lock file's handle",
		"CreateExclusive": "creates a lock file that must not exist",
		"CurrentAct":      "answers the act the calling goroutine runs in",
		"ActRef.Enter":    "joins a goroutine to an act",
		"BeginAct":        "starts an act",
		"Act.End":         "ends an act",
		"DeleteHeld":      "deletes a lock file through its holder's handle",
		"Hold":            "a type",
		"Hold.Unregister": "gives back an entry of the in-process lock registry",
		"MoveDir":         "renames a directory",
		"Register":        "takes an entry of the in-process lock registry",
		"Remove":          "removes a file",
		"RemoveAll":       "removes a tree",
		"RemoveLock":      "removes a lock file",
		"Replace":         "renames a file over another",
		"SamePath":        "compares two paths lexically: cleaned, and folded to lower case where the platform's file systems fold names",
		"StillNamed":      "asks whether a handle already open still refers to the file its path names, which only the file system can answer",
		"TakeOSLock":      "takes the operating-system lock on a handle already open",
		"Truncate":        "cuts a journal back to a length, which writes",
		"TryOSLock":       "asks for the operating-system lock on a handle already open, which only the file system can answer",
		"Wait":            "a type",
		"WriteFile":       "writes a file",
		"WriteHeld":       "writes a lock file through its holder's handle",
	},
}

// AllowedSize is the number of names Allowed carries across every package,
// which each guard asserts so that a widened list is a visible edit.
const AllowedSize = 70

// IsJudged reports whether an import path is one of Judged.
func IsJudged(path string) bool {
	for _, judged := range Judged {
		if judged == path {
			return true
		}
	}
	return false
}

// Member names a judged object: its package's import path, a dot, and its
// name, with a method written as its type and its name, as "os.File.Read".
// It answers "" for an object no judged package declares.
func Member(obj types.Object) string {
	if obj == nil || obj.Pkg() == nil {
		return ""
	}
	if fn, ok := obj.(*types.Func); ok {
		if recv := fn.Type().(*types.Signature).Recv(); recv != nil {
			named := namedOf(recv.Type())
			if named == nil || named.Obj().Pkg() == nil || !IsJudged(named.Obj().Pkg().Path()) {
				return ""
			}
			return named.Obj().Pkg().Path() + "." + named.Obj().Name() + "." + fn.Name()
		}
	}
	if obj.Parent() != obj.Pkg().Scope() || !IsJudged(obj.Pkg().Path()) {
		return ""
	}
	return obj.Pkg().Path() + "." + obj.Name()
}

// namedOf answers the named type a type is, after one pointer, or nil.
func namedOf(t types.Type) *types.Named {
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, _ := t.(*types.Named)
	return named
}

// Allows reports whether Allowed names a member.
func Allows(member string) bool {
	for _, path := range Judged {
		if rest, ok := strings.CutPrefix(member, path+"."); ok {
			if _, allowed := Allowed[path][rest]; allowed {
				return true
			}
		}
	}
	return false
}

// Classify judges one use under the member rule. It answers the judged
// member and whether the use is a read. Every member Allowed does not name is
// a read, os.OpenFile whatever its flag and every os.O_ flag with it: durable's
// guard refuses os.OpenFile outside durable, so no use of it or of its flags
// reaches a package this rule judges.
func Classify(obj types.Object) (string, bool) {
	member := Member(obj)
	if member == "" {
		return "", false
	}
	return member, !Allows(member)
}

// Package is one type-checked package: its syntax, its types, and the Info
// the rules read.
type Package struct {
	Path  string
	Fset  *token.FileSet
	Types *types.Package
	Files []*ast.File
	// Names are the files' paths, parallel to Files.
	Names []string
	Info  *types.Info
}

// Config is one build configuration, as internal/shipped names it.
type Config = shipped.Config

// Shipped are the configurations the project builds a binary for, which
// internal/shipped lists and holds to promote.yml. internal/durable's guard
// loads the module under the same list.
var Shipped = shipped.Configs

// Host is the configuration this process was built for, with no tags, which
// is what a planted file is checked under.
func Host() Config {
	return shipped.Host()
}

// Distinct answers the configurations among configs that select distinct
// sets of non-test Go files from dirs, taken together, each the first of
// configs to select its set; how many files they select between them; and
// every non-test Go file of dirs, as a path, that none of configs selects.
// Two configurations selecting the same files are checked once, because the
// rules judge the files' syntax and the objects it resolves to. That assumes
// a package imported under both declares the members the files name alike.
func Distinct(configs []Config, dirs ...string) (distinct []Config, loaded int, unselected []string, err error) {
	type listing struct {
		abs string
		all []string
	}
	var listings []listing
	for _, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, 0, nil, err
		}
		all, err := nonTestGoFiles(abs)
		if err != nil {
			return nil, 0, nil, err
		}
		listings = append(listings, listing{abs: abs, all: all})
	}
	selected := map[string]bool{}
	seen := map[string]bool{}
	for _, config := range configs {
		var key []string
		for _, l := range listings {
			names, err := matching(l.abs, l.all, config.Context())
			if err != nil {
				return nil, 0, nil, err
			}
			for _, name := range names {
				full := filepath.Join(l.abs, name)
				key = append(key, full)
				selected[full] = true
			}
		}
		joined := strings.Join(key, "\n")
		if !seen[joined] {
			seen[joined] = true
			distinct = append(distinct, config)
		}
	}
	for _, l := range listings {
		for _, name := range l.all {
			full := filepath.Join(l.abs, name)
			if !selected[full] {
				unselected = append(unselected, full)
			}
		}
	}
	return distinct, len(selected), unselected, nil
}

// nonTestGoFiles answers every file of a directory whose name ends in .go and
// not in _test.go, sorted.
func nonTestGoFiles(abs string) ([]string, error) {
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// matching answers the names ctxt.MatchFile accepts.
func matching(abs string, names []string, ctxt build.Context) ([]string, error) {
	var matched []string
	for _, name := range names {
		ok, err := ctxt.MatchFile(abs, name)
		if err != nil {
			return nil, err
		}
		if ok {
			matched = append(matched, name)
		}
	}
	return matched, nil
}

// LoadOptions shape a Load.
type LoadOptions struct {
	// Importer answers every import. SourceImporter, SourceImporterFor and
	// NewImporter build one; a guard makes one per configuration and reuses
	// it, so each imported package is checked once. The package's own files
	// are the ones the Importer's configuration selects.
	Importer types.Importer
}

// Load parses and type-checks one package: the directory's non-test Go files
// that the configuration of opts.Importer selects, the host's when it is not
// an *Importer, plus extra (a planted file). A type error is returned, and
// the caller fails naming it, so a plant that does not compile can never pass
// by going unread.
func Load(dir string, extra []string, opts LoadOptions) (*Package, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	path, err := importPathOf(abs)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	importer := opts.Importer
	if im, ok := importer.(*Importer); ok {
		fset = im.fset
	}
	if importer == nil {
		importer = SourceImporter(fset)
	}
	ctxt := build.Default
	if im, ok := importer.(*Importer); ok {
		ctxt = im.ctxt
	}
	all, err := nonTestGoFiles(abs)
	if err != nil {
		return nil, err
	}
	matched, err := matching(abs, all, ctxt)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range matched {
		names = append(names, filepath.Join(abs, name))
	}
	for _, name := range extra {
		full, err := filepath.Abs(name)
		if err != nil {
			return nil, err
		}
		names = append(names, full)
	}
	pkg := &Package{Path: path, Fset: fset, Names: names, Info: &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Implicits:  map[ast.Node]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}}
	for _, name := range names {
		file, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		pkg.Files = append(pkg.Files, file)
	}
	if len(pkg.Files) == 0 {
		return nil, fmt.Errorf("seamguard: %s holds no Go file for this configuration", abs)
	}
	var errs []error
	conf := types.Config{Importer: importer, Sizes: types.SizesFor("gc", ctxt.GOARCH), Error: func(err error) { errs = append(errs, err) }}
	pkg.Types, _ = conf.Check(path, fset, pkg.Files, pkg.Info)
	if len(errs) > 0 {
		if len(errs) > 10 {
			errs = errs[:10]
		}
		return nil, fmt.Errorf("seamguard: %s does not type-check: %w", path, errors.Join(errs...))
	}
	return pkg, nil
}

// importPathOf answers a directory's import path from the module that holds
// it: the module path in the nearest go.mod above it, joined with the
// directory's path below that file.
func importPathOf(dir string) (string, error) {
	for at := dir; ; at = filepath.Dir(at) {
		data, err := durable.ReadFile(filepath.Join(at, "go.mod"))
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				if module, ok := strings.CutPrefix(strings.TrimSpace(scanner.Text()), "module "); ok {
					rel, err := filepath.Rel(at, dir)
					if err != nil {
						return "", err
					}
					if rel == "." {
						return strings.TrimSpace(module), nil
					}
					return strings.TrimSpace(module) + "/" + filepath.ToSlash(rel), nil
				}
			}
			return "", fmt.Errorf("seamguard: %s names no module", filepath.Join(at, "go.mod"))
		}
		if filepath.Dir(at) == at {
			return "", fmt.Errorf("seamguard: no go.mod above %s", dir)
		}
	}
}

// Importer type-checks packages from source through go/build and go/types.
//
// Two named types are identical under go/types only when their Obj methods
// answer the same TypeName, so every package a guard compares must come from
// one table of checked packages: a second importer checking io/fs again would
// hand out an fs.FileInfo that is a different object from the one package
// bench's Source mentions, and a comparison across the two would silently
// fail (dinah-619/questions/3). One base Importer therefore checks every
// package, and an Importer that NewImporter builds over it shares the base's
// table for every package that does not depend on a package it overrides.
type Importer struct {
	// mu guards a base's table while views reach it from several goroutines.
	mu       sync.Mutex
	fset     *token.FileSet
	ctxt     build.Context
	base     *Importer
	override map[string]*types.Package
	checked  map[string]*types.Package
	// affected memoises whether a package depends, at any depth, on a path
	// override names.
	affected map[string]bool
	// listing, when set, answers every import from one go list run rather
	// than from go/build, which in module mode starts a go list process for
	// each import path it resolves outside GOROOT (Listing).
	listing *Listing
}

// Listed is one package as `go list -deps -json` answers it for one
// configuration.
type Listed struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	Imports    []string
	ImportMap  map[string]string
	Goroot     bool
	DepOnly    bool
}

// Listing is every package a go list run named, and the module's own
// packages among them in the order it named them.
type Listing struct {
	byPath map[string]*Listed
	byDir  map[string]*Listed
	// Module are the packages the patterns matched, as against the
	// dependencies -deps added.
	Module []*Listed
}

// List runs `go list -deps -json` over patterns in dir as config builds, and
// answers the listing. The go command selects each package's files by the
// configuration's GOOS, GOARCH and tags, with cgo off.
func List(dir string, config Config, patterns ...string) (*Listing, error) {
	args := append([]string{"list", "-deps", "-json=ImportPath,Dir,GoFiles,Imports,ImportMap,Goroot,DepOnly"}, config.Flags()...)
	args = append(args, patterns...)
	command := exec.Command("go", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), config.Env()...)
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("go list for %s: %w", config, err)
	}
	listing := &Listing{byPath: map[string]*Listed{}, byDir: map[string]*Listed{}}
	decoder := json.NewDecoder(bytes.NewReader(out))
	for decoder.More() {
		pkg := &Listed{}
		if err := decoder.Decode(pkg); err != nil {
			return nil, fmt.Errorf("go list for %s: %w", config, err)
		}
		listing.byPath[pkg.ImportPath] = pkg
		listing.byDir[filepath.Clean(pkg.Dir)] = pkg
		if !pkg.DepOnly {
			listing.Module = append(listing.Module, pkg)
		}
	}
	return listing, nil
}

// ListedImporter lists patterns from dir as config builds them and answers a
// base Importer over that listing, which is what a guard wants: every import
// resolved from one go list run.
func ListedImporter(dir string, config Config, patterns ...string) (*Importer, error) {
	listing, err := List(dir, config, patterns...)
	if err != nil {
		return nil, err
	}
	return SourceImporterListed(token.NewFileSet(), config, listing), nil
}

// SourceImporterListed answers a base Importer for config that resolves
// every import from listing and never runs go/build's own resolution.
func SourceImporterListed(fset *token.FileSet, config Config, listing *Listing) *Importer {
	im := SourceImporterFor(fset, config)
	im.listing = listing
	return im
}

// find answers the package an import path names from a directory: from the
// listing when the importer has one and the listing names the path, through
// the importing package's ImportMap, and from go/build otherwise.
func (im *Importer) find(path, dir string) (*build.Package, error) {
	if im.listing == nil {
		return im.ctxt.Import(path, dir, 0)
	}
	if from, ok := im.listing.byDir[filepath.Clean(dir)]; ok {
		if mapped, ok := from.ImportMap[path]; ok {
			path = mapped
		}
	}
	pkg, ok := im.listing.byPath[path]
	if !ok {
		// A planted file may import a package nothing listed depends on.
		return im.ctxt.Import(path, dir, 0)
	}
	imports := make([]string, 0, len(pkg.Imports))
	for _, imported := range pkg.Imports {
		if mapped, ok := pkg.ImportMap[imported]; ok {
			imported = mapped
		}
		imports = append(imports, imported)
	}
	return &build.Package{ImportPath: pkg.ImportPath, Dir: pkg.Dir, GoFiles: pkg.GoFiles, Imports: imports, Goroot: pkg.Goroot}, nil
}

// SourceImporter answers a base Importer for the host's configuration: every
// package it is asked for is checked from source once, with function bodies
// ignored, and kept.
func SourceImporter(fset *token.FileSet) *Importer {
	return SourceImporterFor(fset, Host())
}

// SourceImporterFor answers a base Importer that checks every package, and
// selects the files of the package a Load checks, as config builds them.
func SourceImporterFor(fset *token.FileSet, config Config) *Importer {
	return &Importer{fset: fset, ctxt: config.Context(), checked: map[string]*types.Package{}}
}

// NewImporter answers an Importer over base in which each import path named in
// override answers the package given there, at every depth: a package that
// imports an overridden path, directly or through another, is checked afresh
// against the override, and every other package is base's own. The verb guard
// hands in package bench checked with a planted companion file.
//
// This departs from dinah-619's specification, section 12.0, which gave
// NewImporter a file set and had it build its own source importer; that left
// open which importer checks package bench with its companion, and a second
// one would break the comparison the type comment above describes.
func NewImporter(base *Importer, override map[string]*types.Package) *Importer {
	return &Importer{fset: base.fset, ctxt: base.ctxt, base: base, override: override, listing: base.listing,
		checked: map[string]*types.Package{}, affected: map[string]bool{}}
}

// Import is ImportFrom from the current directory.
func (im *Importer) Import(path string) (*types.Package, error) {
	return im.ImportFrom(path, ".", 0)
}

// ImportFrom answers the package an import path names from a directory.
func (im *Importer) ImportFrom(path, dir string, _ types.ImportMode) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	if pkg, ok := im.override[path]; ok {
		return pkg, nil
	}
	bp, err := im.find(path, dir)
	if err != nil {
		return nil, err
	}
	if pkg, ok := im.checked[bp.ImportPath]; ok {
		return pkg, nil
	}
	if im.base != nil && !im.dependsOnOverride(bp) {
		// Views run on several goroutines at once, one per planted file, and
		// the base's table is shared, so a view reaches it under the base's
		// lock. The base never takes its own lock, so its recursion through
		// its own imports cannot deadlock.
		im.base.mu.Lock()
		defer im.base.mu.Unlock()
		return im.base.ImportFrom(path, dir, 0)
	}
	var files []*ast.File
	for _, name := range bp.GoFiles {
		file, err := parser.ParseFile(im.fset, filepath.Join(bp.Dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	var firstErr error
	conf := types.Config{Importer: im, IgnoreFuncBodies: true, FakeImportC: true, Sizes: types.SizesFor("gc", im.ctxt.GOARCH),
		Error: func(err error) {
			if firstErr == nil {
				firstErr = err
			}
		}}
	pkg, _ := conf.Check(bp.ImportPath, im.fset, files, nil)
	if firstErr != nil {
		return nil, fmt.Errorf("seamguard: import %s: %w", bp.ImportPath, firstErr)
	}
	im.checked[bp.ImportPath] = pkg
	return pkg, nil
}

// dependsOnOverride reports whether a package imports a path override names,
// directly or through a package outside the standard library.
func (im *Importer) dependsOnOverride(bp *build.Package) bool {
	if done, ok := im.affected[bp.ImportPath]; ok {
		return done
	}
	im.affected[bp.ImportPath] = false
	result := false
	if !bp.Goroot {
		for _, path := range bp.Imports {
			if _, ok := im.override[path]; ok {
				result = true
				break
			}
			dep, err := im.find(path, bp.Dir)
			if err == nil && im.dependsOnOverride(dep) {
				result = true
				break
			}
		}
	}
	im.affected[bp.ImportPath] = result
	return result
}

// HoldsTheBottom reports whether a type is the bottom, a pointer to it, or a
// named or struct type with a field of the bottom's type, or a pointer to it,
// at any depth, embedded or named. Identity is types.Identical against the
// bottom's type, so an alias of the bottom is the bottom.
func HoldsTheBottom(t types.Type, bottom *types.TypeName) bool {
	return holdsBottom(t, bottom.Type(), map[types.Type]bool{})
}

func holdsBottom(t, bottom types.Type, seen map[types.Type]bool) bool {
	t = types.Unalias(t)
	if types.Identical(t, bottom) {
		return true
	}
	switch u := t.(type) {
	case *types.Pointer:
		return holdsBottom(u.Elem(), bottom, seen)
	case *types.Named:
		if seen[u] {
			return false
		}
		seen[u] = true
		if s, ok := u.Underlying().(*types.Struct); ok {
			return holdsBottom(s, bottom, seen)
		}
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if holdsBottom(u.Field(i).Type(), bottom, seen) {
				return true
			}
		}
	}
	return false
}

// OfTheBottom reports whether an expression is a value whose type holds the
// bottom, which is what "an expression of the bottom" means in both guards.
func OfTheBottom(info *types.Info, e ast.Expr, bottom *types.TypeName) bool {
	tv, ok := info.Types[e]
	return ok && tv.IsValue() && tv.Type != nil && HoldsTheBottom(tv.Type, bottom)
}

// Violation is one finding.
type Violation struct {
	Root, Via, What string
	// File is the file holding the offending expression.
	File string
	// RootFile is the file declaring the root the walk started from; for a
	// placement or outside-Go violation it equals File.
	RootFile string
}

// OutsideGo answers the outside-Go rule's violations over every file of a
// package: every comment whose text begins with //go:linkname, of which Go
// 1.26's cmd/compile documentation says "This directive determines the
// object-file symbol used for a Go var or func declaration, allowing two Go
// symbols to alias the same object-file symbol, thereby enabling one package
// to access a symbol in another package even when this would violate the
// usual encapsulation of unexported declarations, or even type safety.", and
// every function declared without a body, which the Go specification says
// "provides the signature for a function implemented outside Go, such as an
// assembly routine". Neither is ever exempt. The same documentation enables
// the directive only in files that import unsafe, and the rule is still not
// keyed on that import, which package bench makes for reasons of its own.
func OutsideGo(pkg *Package) []Violation {
	var found []Violation
	for i, file := range pkg.Files {
		name := pkg.Names[i]
		for _, group := range file.Comments {
			for _, c := range group.List {
				if strings.HasPrefix(c.Text, "//go:linkname") {
					found = append(found, Violation{Root: "outside Go", Via: "outside Go", File: name, RootFile: name,
						What: "code outside Go: //go:linkname directive at " + pkg.At(c.Pos())})
				}
			}
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body == nil {
				found = append(found, Violation{Root: "outside Go", Via: "outside Go", File: name, RootFile: name,
					What: "code outside Go: function declared without a body, " + fn.Name.Name + ", at " + pkg.At(fn.Pos())})
			}
		}
	}
	return found
}

// At answers a position as "file:line".
func (p *Package) At(pos token.Pos) string {
	at := p.Fset.Position(pos)
	return filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line)
}

// FileOf answers the path of the file holding a position.
func (p *Package) FileOf(pos token.Pos) string {
	return p.Fset.Position(pos).Filename
}

// Read is one read a node makes.
type Read struct {
	// Member is the judged member, as "path/filepath.WalkDir", which is what
	// an exemption is keyed on.
	Member string
	// At is where it is named, as "file:line".
	At string
	// File is the path of the file naming it.
	File string
}

// Site is one expression of the bottom, or one use of a method of the
// bottom's type, inside a node.
type Site struct {
	What string
	At   string
	File string
}

// Node is one function, method, package variable with an initializer, or
// stored function literal of the graph.
type Node struct {
	// Key is the node's name: a function's or method's types.Func FullName,
	// "var " and a variable's package path and name, or "lit " and a
	// literal's position.
	Key string
	// Name is what an exemption is keyed on: the node's own Key, or for a
	// literal the Key of the declaration it sits in.
	Name string
	// File is the file declaring the node.
	File string
	// Receiver is a method's receiver type, after one pointer, and nil
	// otherwise.
	Receiver *types.TypeName
	// Root reports that the walk starts from the node whatever the guard's
	// own roots are: a stored literal, or a function or variable some body
	// names as a value.
	Root   bool
	Edges  []string
	Reads  []Read
	Bottom []Site
	// isVar marks a package variable's node, whose initializer holds what it
	// names rather than handing it on.
	isVar bool
}

// GraphOptions shape a graph.
type GraphOptions struct {
	// Bottom is the seam's bottom, whose methods read by definition: the
	// walk does not follow an edge into them, and a use of one is a bottom
	// site.
	Bottom *types.TypeName
	// Leaf is the interface whose method calls are where a read goes through
	// the seam, and add no edge.
	Leaf *types.TypeName
}

// Graph is every node of one package, and the reads its type declarations
// name.
type Graph struct {
	Pkg   *Package
	Nodes map[string]*Node
	// TypeReads are the reads a package-level type declaration names: a
	// handle a method would read through, held where no signature shows it.
	TypeReads []Read
}

// BuildGraph answers a package's graph. An identifier in a body adds an edge
// to the node of the object Info.Uses resolves it to, called or only taken as
// a value. A call of an interface method adds an edge to every method of the
// package with that name whose receiver type T or *T implements the
// interface, except a method of opts.Leaf, which adds none.
func BuildGraph(pkg *Package, opts GraphOptions) *Graph {
	g := &Graph{Pkg: pkg, Nodes: map[string]*Node{}}
	b := &graphBuilder{g: g, opts: opts, methods: map[string][]*types.Func{}, values: map[string]bool{}}
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		for i := 0; i < named.NumMethods(); i++ {
			m := named.Method(i)
			b.methods[m.Name()] = append(b.methods[m.Name()], m)
		}
	}
	for i, file := range pkg.Files {
		name := pkg.Names[i]
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				obj, ok := pkg.Info.Defs[d.Name].(*types.Func)
				if !ok {
					continue
				}
				key := obj.FullName()
				n := b.node(key, key, name)
				if recv := obj.Type().(*types.Signature).Recv(); recv != nil {
					if named := namedOf(recv.Type()); named != nil {
						n.Receiver = named.Obj()
					}
				}
				b.scan(n, d.Type, nil)
				b.scan(n, d.Body, d.Body)
			case *ast.GenDecl:
				switch d.Tok {
				case token.TYPE:
					ast.Inspect(d, func(node ast.Node) bool {
						if id, ok := node.(*ast.Ident); ok {
							if member, read := Classify(pkg.Info.Uses[id]); read {
								g.TypeReads = append(g.TypeReads, Read{Member: member, At: pkg.At(id.Pos()), File: name})
							}
						}
						return true
					})
				case token.VAR:
					for _, spec := range d.Specs {
						vs := spec.(*ast.ValueSpec)
						if len(vs.Values) == 0 {
							continue
						}
						for _, id := range vs.Names {
							v, ok := pkg.Info.Defs[id].(*types.Var)
							if !ok {
								continue
							}
							key := varKey(v)
							n := b.node(key, key, name)
							n.isVar = true
							if vs.Type != nil {
								b.scan(n, vs.Type, nil)
							}
							for _, value := range vs.Values {
								b.scan(n, value, value)
							}
						}
					}
				}
			}
		}
	}
	for key := range b.values {
		if n, ok := g.Nodes[key]; ok {
			n.Root = true
		}
	}
	return g
}

// varKey keys a package variable's node.
func varKey(v *types.Var) string {
	return "var " + v.Pkg().Path() + "." + v.Name()
}

type graphBuilder struct {
	g       *Graph
	opts    GraphOptions
	methods map[string][]*types.Func
	// values are the functions and variables some function body names other
	// than as the function of a call. A value a variable's initializer names
	// is held by that variable, whose every use is an edge or is itself a
	// value named, so the walk reaches it through the variable.
	values map[string]bool
}

func (b *graphBuilder) node(key, name, file string) *Node {
	n, ok := b.g.Nodes[key]
	if !ok {
		n = &Node{Key: key, Name: name, File: file}
		b.g.Nodes[key] = n
	}
	return n
}

// scan records into n what one piece of syntax reads, holds and names. A
// function literal that is neither called in place nor handed straight to a
// function of another package is a node of its own, and a root, because its
// value can be called from anywhere later.
func (b *graphBuilder) scan(n *Node, root ast.Node, body ast.Node) {
	pkg := b.g.Pkg
	info := pkg.Info
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		parent := ast.Node(nil)
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		if lit, ok := node.(*ast.FuncLit); ok && body != nil && !b.inPlace(lit, parent) {
			at := pkg.Fset.Position(lit.Pos())
			key := "lit " + filepath.Base(at.Filename) + ":" + strconv.Itoa(at.Line) + ":" + strconv.Itoa(at.Column)
			ln := b.node(key, n.Name, n.File)
			ln.Root = true
			n.Edges = append(n.Edges, key)
			b.scan(ln, lit.Body, lit.Body)
			return false
		}
		stack = append(stack, node)
		switch x := node.(type) {
		case ast.Expr:
			if b.opts.Bottom != nil && OfTheBottom(info, x, b.opts.Bottom) && !b.bottomParent(parent) {
				n.Bottom = append(n.Bottom, Site{What: "an expression of type " + types.TypeString(info.Types[x].Type, qualifier), At: pkg.At(x.Pos()), File: pkg.FileOf(x.Pos())})
			}
			if id, ok := x.(*ast.Ident); ok {
				b.use(n, id, stack[:len(stack)-1])
			}
		}
		return true
	})
}

// bottomParent reports whether an expression's parent is itself an
// expression of the bottom, so only the outermost of a nest is a site.
func (b *graphBuilder) bottomParent(parent ast.Node) bool {
	e, ok := parent.(ast.Expr)
	return ok && OfTheBottom(b.g.Pkg.Info, e, b.opts.Bottom)
}

// inPlace reports whether a literal is called where it stands or handed as an
// argument to a function of another package, which runs it and keeps nothing.
func (b *graphBuilder) inPlace(lit *ast.FuncLit, parent ast.Node) bool {
	call, ok := parent.(*ast.CallExpr)
	if !ok {
		return false
	}
	if unparen(call.Fun) == lit {
		return true
	}
	if obj := calleeObject(b.g.Pkg.Info, call); obj != nil && obj.Pkg() != nil && obj.Pkg() != b.g.Pkg.Types {
		return true
	}
	return false
}

// calleeObject answers the object a call's function resolves to.
func calleeObject(info *types.Info, call *ast.CallExpr) types.Object {
	switch fun := unparen(call.Fun).(type) {
	case *ast.Ident:
		return info.Uses[fun]
	case *ast.SelectorExpr:
		return info.Uses[fun.Sel]
	case *ast.IndexExpr:
		if id, ok := unparen(fun.X).(*ast.Ident); ok {
			return info.Uses[id]
		}
	}
	return nil
}

// use records one identifier's use: a read under the member rule, a bottom
// site for a method of the bottom, and an edge.
func (b *graphBuilder) use(n *Node, id *ast.Ident, stack []ast.Node) {
	pkg := b.g.Pkg
	obj := pkg.Info.Uses[id]
	if obj == nil {
		return
	}
	if member, read := Classify(obj); read {
		n.Reads = append(n.Reads, Read{Member: member, At: pkg.At(id.Pos()), File: pkg.FileOf(id.Pos())})
	}
	fn, isFunc := obj.(*types.Func)
	if isFunc {
		fn = fn.Origin()
		sig := fn.Type().(*types.Signature)
		if recv := sig.Recv(); recv != nil {
			if b.opts.Bottom != nil {
				if named := namedOf(recv.Type()); named != nil && named.Obj() == b.opts.Bottom {
					n.Bottom = append(n.Bottom, Site{What: "a use of " + fn.FullName(), At: pkg.At(id.Pos()), File: pkg.FileOf(id.Pos())})
					return
				}
			}
			if iface, ok := recv.Type().Underlying().(*types.Interface); ok {
				if b.opts.Leaf != nil && types.Identical(recv.Type(), b.opts.Leaf.Type()) {
					return
				}
				for _, m := range b.methods[fn.Name()] {
					named := namedOf(m.Type().(*types.Signature).Recv().Type())
					if named == nil {
						continue
					}
					if types.Implements(named, iface) || types.Implements(types.NewPointer(named), iface) {
						n.Edges = append(n.Edges, m.FullName())
					}
				}
				return
			}
		}
		if fn.Pkg() == pkg.Types {
			n.Edges = append(n.Edges, fn.FullName())
			if !called(id, stack) && !n.isVar {
				b.values[fn.FullName()] = true
			}
		}
		return
	}
	if v, ok := obj.(*types.Var); ok && v.Pkg() == pkg.Types && v.Parent() == pkg.Types.Scope() {
		key := varKey(v)
		n.Edges = append(n.Edges, key)
		if !called(id, stack) && !n.isVar {
			b.values[key] = true
		}
	}
}

// enclosingCall answers the call an identifier is the function of, or nil.
func enclosingCall(id *ast.Ident, stack []ast.Node) *ast.CallExpr {
	i := len(stack) - 1
	var inner ast.Node = id
	if i >= 0 {
		if sel, ok := stack[i].(*ast.SelectorExpr); ok && sel.Sel == id {
			inner = sel
			i--
		}
	}
	if i >= 0 {
		if call, ok := stack[i].(*ast.CallExpr); ok && unparen(call.Fun) == inner {
			return call
		}
	}
	return nil
}

// called reports whether an identifier is used where it stands rather than
// handed on as a value: the function of a call, the selected name of a
// selector that is, or the operand of a selector, whose field is read or
// whose method is called through it. The selector itself is judged as its
// own use.
func called(id *ast.Ident, stack []ast.Node) bool {
	if enclosingCall(id, stack) != nil {
		return true
	}
	if len(stack) > 0 {
		if sel, ok := stack[len(stack)-1].(*ast.SelectorExpr); ok && sel.X == id {
			return true
		}
	}
	return false
}

// unparen strips parentheses from an expression.
func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// qualifier writes a type's package by its path.
func qualifier(p *types.Package) string {
	return p.Path()
}

// Walk describes one guard's walk over a graph.
type Walk struct {
	// Root reports whether a node is a root of the walk. The walk also
	// starts from every node the graph marks Root.
	Root func(*Node) bool
	// Exempt are the reads the walk lets through, keyed on a node's Name and
	// then on the member read. The pair is the key, so a read beside the
	// excused one is still refused, and a second read of the same member in
	// the same function passes.
	Exempt map[string]map[string]string
	// Seam is the key of the one function allowed to hold the bottom: the
	// walk does not follow an edge into it, and its own bottom sites are no
	// violation.
	Seam string
}

// Roots answers the keys the walk starts from.
func (g *Graph) Roots(w Walk) []string {
	var keys []string
	for key, n := range g.Nodes {
		if n.Root || w.Root(n) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// Violations walks from every root and answers every read a reachable node
// makes that no exemption covers, every bottom site in a reachable node other
// than the seam, and every read a type declaration names.
func (g *Graph) Violations(w Walk) []Violation {
	var found []Violation
	for _, read := range g.TypeReads {
		found = append(found, Violation{Root: "type", Via: "type", File: read.File, RootFile: read.File,
			What: "a type declaration names " + read.Member + " at " + read.At + ", a handle a method would read through"})
	}
	seen := map[string]bool{}
	for _, root := range g.Roots(w) {
		rootFile := g.Nodes[root].File
		reached := map[string]string{root: root}
		queue := []string{root}
		for len(queue) > 0 {
			key := queue[0]
			queue = queue[1:]
			n := g.Nodes[key]
			if n == nil {
				continue
			}
			for _, read := range n.Reads {
				if _, exempt := w.Exempt[n.Name][read.Member]; exempt {
					continue
				}
				tag := root + "|" + key + "|" + read.Member + "|" + read.At
				if !seen[tag] {
					seen[tag] = true
					found = append(found, Violation{Root: root, Via: reached[key], File: read.File, RootFile: rootFile,
						What: key + " reads " + read.Member + " at " + read.At})
				}
			}
			if key != w.Seam {
				for _, site := range n.Bottom {
					tag := root + "|" + key + "|bottom|" + site.At
					if !seen[tag] {
						seen[tag] = true
						found = append(found, Violation{Root: root, Via: reached[key], File: site.File, RootFile: rootFile,
							What: "bottom rule: " + key + " holds " + site.What + " at " + site.At})
					}
				}
			}
			for _, next := range n.Edges {
				if next == w.Seam {
					continue
				}
				if _, ok := reached[next]; ok {
					continue
				}
				reached[next] = reached[key] + " > " + next
				queue = append(queue, next)
			}
		}
	}
	return found
}

// Reaches reports whether anything reachable from a key reads or holds the
// bottom anywhere but the seam, walking every edge with no exemption, and
// answers how.
func (g *Graph) Reaches(key, seam string) (string, bool) {
	reached := map[string]string{key: key}
	queue := []string{key}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		n := g.Nodes[at]
		if n == nil {
			continue
		}
		if len(n.Reads) > 0 {
			return reached[at] + " reads " + n.Reads[0].Member + " at " + n.Reads[0].At, true
		}
		if at != seam && len(n.Bottom) > 0 {
			return reached[at] + " holds " + n.Bottom[0].What + " at " + n.Bottom[0].At, true
		}
		for _, next := range n.Edges {
			if next == seam {
				continue
			}
			if _, ok := reached[next]; ok {
				continue
			}
			reached[next] = reached[at] + " > " + next
			queue = append(queue, next)
		}
	}
	return "", false
}

// Placement answers the placement rule's violations over every function of a
// package and every package variable's initializer, whether or not a root
// reaches it. An expression of the bottom may appear in two places only:
// inside the seam function, and as a call argument whose parameter, by the
// callee's signature, has the type source. An assignment, a composite
// literal's field, a return value, a conversion, a method call's receiver and
// a package variable's initializer are violations wherever they occur.
func Placement(pkg *Package, bottom, source *types.TypeName, seam string) []Violation {
	var found []Violation
	info := pkg.Info
	check := func(key string, root ast.Node) {
		var stack []ast.Node
		ast.Inspect(root, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			var parent ast.Node
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, node)
			e, ok := node.(ast.Expr)
			if !ok || !OfTheBottom(info, e, bottom) {
				return true
			}
			if p, ok := parent.(ast.Expr); ok && OfTheBottom(info, p, bottom) {
				return true
			}
			if key == seam || sourceArgument(info, e, parent, source) {
				return true
			}
			found = append(found, Violation{Root: key, Via: key, File: pkg.FileOf(e.Pos()), RootFile: pkg.FileOf(e.Pos()),
				What: "placement rule: " + key + " holds an expression of type " + types.TypeString(info.Types[e].Type, qualifier) +
					" at " + pkg.At(e.Pos()) + " other than as an argument to a " + source.Name() + " parameter"})
			return true
		})
	}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Body == nil {
					continue
				}
				if obj, ok := info.Defs[d.Name].(*types.Func); ok {
					check(obj.FullName(), d.Body)
				}
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					for _, value := range vs.Values {
						key := "var " + pkg.Path
						if len(vs.Names) > 0 {
							key += "." + vs.Names[0].Name
						}
						check(key, value)
					}
				}
			}
		}
	}
	return found
}

// sourceArgument reports whether an expression is an argument of a call that
// is not a conversion, in a position whose parameter has the type source.
func sourceArgument(info *types.Info, e ast.Expr, parent ast.Node, source *types.TypeName) bool {
	call, ok := parent.(*ast.CallExpr)
	if !ok {
		return false
	}
	fun, ok := info.Types[call.Fun]
	if !ok || fun.IsType() || fun.Type == nil {
		return false
	}
	sig, ok := fun.Type.Underlying().(*types.Signature)
	if !ok {
		return false
	}
	for i, arg := range call.Args {
		if arg != e {
			continue
		}
		params := sig.Params()
		var param types.Type
		switch {
		case sig.Variadic() && i >= params.Len()-1 && !call.Ellipsis.IsValid():
			param = params.At(params.Len() - 1).Type().(*types.Slice).Elem()
		case i < params.Len():
			param = params.At(i).Type()
		default:
			return false
		}
		return types.Identical(param, source.Type())
	}
	return false
}
