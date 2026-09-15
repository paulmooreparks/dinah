package lsp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// permittedDurations are the only two duration-valued declarations this
// package may carry, named so that adding a third fails whatever it is
// called.
//
// The first is the poll interval, which is configured. The second is how long
// the walk that has just finished took, which is measured after the fact and
// read for one purpose only: deciding how long to sleep before the next walk
// starts. Neither means how long a value stays good, and a third duration is
// the cache with an expiry this card exists without.
var permittedDurations = map[string]string{
	"interval": "the configured poll interval",
	"walked":   "the measured duration of the last completed change walk",
}

// expirySpellings are the identifier fragments a cache with an expiry is
// spelled with. A package carrying one has grown the thing section 3.1
// forbids, whatever its type.
var expirySpellings = []string{"ttl", "expiry", "expires", "stale", "staleness", "maxage", "cachefor"}

// TestTheServerHoldsNoExpirySemantics asserts dinah-515 criterion 4, by
// meaning rather than by type name: the package declares exactly the two
// permitted durations, no identifier is spelled the way an expiry is, and no
// annotation carries a timestamp or a deadline.
func TestTheServerHoldsNoExpirySemantics(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package's sources: %v", err)
	}
	fileset := token.NewFileSet()
	found := map[string]string{}
	identifiers := 0
	read := 0
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		read++
		file, err := parser.ParseFile(fileset, source, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", source, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch declared := node.(type) {
			case *ast.Field:
				for _, name := range declared.Names {
					if isDuration(declared.Type) {
						found[name.Name] = source
					}
				}
			case *ast.ValueSpec:
				for _, name := range declared.Names {
					if isDuration(declared.Type) {
						found[name.Name] = source
					}
				}
			case *ast.Ident:
				identifiers++
				lowered := strings.ToLower(declared.Name)
				for _, spelling := range expirySpellings {
					if strings.Contains(lowered, spelling) {
						t.Errorf("%s declares the identifier %s, which is spelled the way a cache with an expiry is", source, declared.Name)
					}
				}
			}
			return true
		})
	}
	if read == 0 {
		t.Fatal("the walk read no source of this package, so it proves nothing")
	}
	if identifiers == 0 {
		t.Fatal("the walk read no identifier, so the spelling half proves nothing")
	}
	for name, source := range found {
		if _, permitted := permittedDurations[name]; !permitted {
			t.Errorf("%s declares the duration %s, and the two this package may hold are the poll interval and the measured walk", source, name)
		}
	}
	for name, why := range permittedDurations {
		if _, declared := found[name]; !declared {
			t.Errorf("the package declares no duration called %s, which is %s, so this guard is checking a set that has moved", name, why)
		}
	}
	if len(found) != len(permittedDurations) {
		t.Errorf("the package declares %d durations and may hold %d", len(found), len(permittedDurations))
	}

	// The wire shape carries no timestamp and no deadline either, which is
	// where an expiry would next appear. The members are read out of the
	// source rather than off a value, so a member added without a test being
	// touched is still seen.
	members := strings.ToLower(annotationMembers(t))
	for _, forbidden := range []string{"timestamp", "deadline", "expires", "until"} {
		if strings.Contains(members, forbidden) {
			t.Errorf("the annotation the server publishes carries a %s member", forbidden)
		}
	}
}

// isDuration reports whether a declared type is time.Duration, which is the
// question the duration census asks.
func isDuration(node ast.Expr) bool {
	selector, ok := node.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "time" && selector.Sel.Name == "Duration"
}

// annotationMembers renders the published annotation's member names, so a
// guard over the shape reads the shape rather than a sentence about it.
func annotationMembers(t *testing.T) string {
	t.Helper()
	var names []string
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, "protocol.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse protocol.go: %v", err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "wireAnnotation" {
			return true
		}
		structure, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structure.Fields.List {
			for _, name := range field.Names {
				names = append(names, name.Name)
			}
		}
		return false
	})
	if len(names) == 0 {
		t.Fatal("the walk found no member of wireAnnotation, so this guard read nothing")
	}
	return strings.Join(names, " ")
}

// TestTheServerTakesNoLockOnTheWorkbench asserts dinah-515 criterion 13: a
// card open as a document in a running server is a card another Library can
// still write to, because the server takes no lock at all.
//
// The plant the criterion names is the arming state: a build whose server
// acquires the card lock for the lifetime of the open document, against which
// the mutation refuses with a held-lock refusal. That plant is applied by
// hand, because writing it into the tree would ship the very thing this
// guards against.
func TestTheServerTakesNoLockOnTheWorkbench(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)
	h.open(f.cardAnchor(f.card))

	// The mutation runs through a second Library over the same directory,
	// which is what a command line alongside the editor would be.
	opened, err := bench.Open(f.root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	second := verb.New(opened, "")
	answer := second.Do(&verb.Request{Verb: "move", Actor: "alka", Card: f.card, Column: f.bench.Columns[1].ID})
	if answer.Outcome != "ok" {
		t.Fatalf("a card open in the server refused a move from a second library: %s %s", answer.Outcome, answer.Refusal)
	}

	// The server holds no handle inside the entity directory either, which is
	// what a rename of that directory would fail on.
	if err := os.Rename(filepath.Join(f.bench.CardsRoot(), f.card), filepath.Join(f.bench.CardsRoot(), f.card+"x")); err != nil {
		t.Errorf("the card directory could not be renamed while the server had it open: %v", err)
	} else if err := os.Rename(filepath.Join(f.bench.CardsRoot(), f.card+"x"), filepath.Join(f.bench.CardsRoot(), f.card)); err != nil {
		t.Fatalf("restore: %v", err)
	}
}

// TestTheWorkbenchLadderIsOneOrderedList asserts dinah-515 criterion 23: the
// seven rungs of section 5.1 are tried in order, and removing each winner in
// turn walks the list down to the process working directory.
func TestTheWorkbenchLadderIsOneOrderedList(t *testing.T) {
	base := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))

	// Seven workbenches, one per rung, each nameable on its own.
	roots := map[string]string{}
	for _, rung := range []string{"workbench", "root", "lspenv", "workbenchvar", "rooturi", "folder", "wd"} {
		dir := filepath.Join(base, rung)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		written, err := verb.Init(dir, "fx"+strings.ToLower(rung[:2]), "alka", "", "", "")
		if err != nil {
			t.Fatalf("init %s: %v", rung, err)
		}
		roots[rung] = written
	}

	ladder := []string{"workbench", "root", "lspenv", "workbenchvar", "rooturi", "folder", "wd"}
	for cut := 0; cut < len(ladder); cut++ {
		expected := ladder[cut]
		t.Run(expected, func(t *testing.T) {
			opts := Options{
				Getenv: func(name string) string {
					switch name {
					case "DINAH_LSP_ROOT":
						if cut <= 2 {
							return filepath.Dir(roots["lspenv"])
						}
					case "DINAH_WORKBENCH":
						if cut <= 3 {
							return roots["workbenchvar"]
						}
					}
					return ""
				},
				Discover: func(start string) (string, error) {
					root, _, err := bench.Discover(start, "", filepath.Join(base, "home"), "")
					return root, err
				},
			}
			if cut <= 0 {
				opts.Workbench = roots["workbench"]
			}
			if cut <= 1 {
				opts.Root = filepath.Dir(roots["root"])
			}
			opts.Wd = filepath.Dir(roots["wd"])

			params := initializeParams{}
			if cut <= 4 {
				params.RootURI = fileURI(filepath.Dir(roots["rooturi"]))
			}
			if cut <= 5 {
				params.WorkspaceFolders = append(params.WorkspaceFolders, struct {
					URI string `json:"uri"`
				}{URI: fileURI(filepath.Dir(roots["folder"]))})
			}

			s := New(opts, strings.NewReader(""), io_Discard{})
			got := s.discover(params)
			if got != roots[expected] {
				t.Errorf("rung %d resolved %q, wanted the %s workbench at %q", cut+1, got, expected, roots[expected])
			}
		})
	}
}

// io_Discard is a writer that keeps nothing, for a server built to answer one
// question rather than to serve.
type io_Discard struct{}

// Write keeps nothing and reports success.
func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }

// TestTheResolvedWorkbenchIsNamedInOneLogLine asserts the second half of
// dinah-515 criterion 23: whichever rung answered, the root it resolved is
// named once in the editor's own log, because a server that picked the wrong
// one of two open repositories gives a reader no other way to find out.
func TestTheResolvedWorkbenchIsNamedInOneLogLine(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)

	sent := h.await(methodLogMessage, 1)
	named := 0
	for _, line := range sent {
		if decode[logMessageParams](t, line.Params).Message == h.server.messages.T(keyLogWorkbench, "root", f.root) {
			named++
		}
	}
	if named != 1 {
		t.Errorf("the server named the resolved workbench in %d log lines, wanted one", named)
	}
}
