package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/bench/compattest"
)

// migrateNotesBinary is the note migration, built once for this file.
//
// It is a program of its own rather than a flag on check, per the operator's
// rule of 2026-09-15, so the only way to run it is to run it. One `go build`
// is the whole cost, shared across every fixture below.
var migrateNotesBinary = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "dinah-migrate-notes-")
	if err != nil {
		return "", err
	}
	name := "dinah-migrate-notes"
	if os.PathSeparator == '\\' {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	build := exec.Command("go", "build", "-o", path, "./cmd/dinah-migrate-notes")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build the note migration: %v\n%s", err, out)
	}
	return path, nil
})

// migrateNotes carries one staged fixture across the note migration, which is
// the last of the chain an older store walks before anything can read it.
//
// --unattributed is passed because a historical fixture's journal records no
// settling for the notes its items carry, and the run refuses rather than
// inventing an actor. That refusal is the migration's own case and is asserted
// in cmd/dinah-migrate-notes; what this file needs is a store on the other
// side of it.
func migrateNotes(t *testing.T, root string) {
	t.Helper()
	binary, err := migrateNotesBinary()
	if err != nil {
		t.Fatalf("%v", err)
	}
	run := exec.Command(binary, root, "--apply", "--unattributed")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("the note migration on %s: %v\n%s", root, err, out)
	}
}

// runMigrateNotes runs the note migration and hands back what it did rather
// than failing the test, which is what a case asserting a refusal needs.
//
// migrateNotes beside it is the form for a case that only wants the migration
// to have happened; this one is for a case whose subject is the migration
// declining to run.
func runMigrateNotes(t *testing.T, root string, args ...string) invocation {
	t.Helper()
	binary, err := migrateNotesBinary()
	if err != nil {
		t.Fatalf("%v", err)
	}
	run := exec.Command(binary, append([]string{root}, args...)...)
	var out, errw bytes.Buffer
	run.Stdout = &out
	run.Stderr = &errw
	code := 0
	if err := run.Run(); err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("the note migration on %s: %v", root, err)
		}
		code = exit.ExitCode()
	}
	return invocation{code: code, out: out.String(), errw: errw.String()}
}

// itemKindsFiled are the three kinds this case files on every fixture, in the
// order the format declares them.
var itemKindsFiled = []string{"acceptance_criterion", "open_question", "decision"}

// TestFilingAnItemOnEveryHistoricalFixtureWritesOneShape asserts dinah-206
// AC-10's second half: a card inside a copy of any fixture workbench takes a
// checklist item, and the anchor written there carries the same frontmatter
// keys as one filed on a workbench this build created a moment ago.
//
// The comparison is between key sets rather than between bytes, because the
// identifiers, the ordinals and the timestamps differ between two workbenches
// by construction and say nothing about the shape. What it catches is a write
// path that reaches for something an older workbench does not carry, which is
// the one way filing on an old card could need the card migrated first.
//
// It works on a copy. A fixture is evidence rather than scratch, and the
// digest guard beside it would fail on the next run if this case wrote into
// the tree.
func TestFilingAnItemOnEveryHistoricalFixtureWritesOneShape(t *testing.T) {
	fresh := shapeOfFiledItems(t, freshWorkbench(t))
	if len(fresh) != len(itemKindsFiled) {
		t.Fatalf("filing on a fresh workbench produced %d items, wanted %d", len(fresh), len(itemKindsFiled))
	}
	// The shape a fresh workbench produces is asserted outright before it is
	// used as the standard the fixtures are held to. Without this the
	// comparison below is symmetric, and a write that drops a key drops it on
	// both sides and reads as agreement: the plant that proved this case can
	// fail was exactly that, a key written for one kind and not the others.
	// column and owner are absent because this case names neither, which is
	// the spec's own default for both.
	for _, kind := range itemKindsFiled {
		if got := fresh[kind]; got != "kind, ordinal, state, ts" {
			t.Fatalf("a %s filed on a workbench this build created carries [%s], and the format declares [kind, ordinal, state, ts]", kind, got)
		}
	}

	manifest, err := compattest.ReadFixtureManifest(compatDir)
	if err != nil {
		t.Fatalf("read the fixture manifest: %v", err)
	}
	if len(manifest.Fixtures) == 0 {
		t.Fatal("the manifest names no fixture, so this case would assert nothing")
	}
	for _, row := range manifest.Fixtures {
		t.Run(row.Directory, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "workbench")
			copyFixture(t, filepath.Join(compatDir, row.Directory), root)
			t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
			t.Setenv("DINAH_ACTOR", "sam")
			t.Setenv("DINAH_LANG", "")
			t.Setenv("DINAH_FORMAT", "")
			t.Setenv("DINAH_WORKBENCH", "")
			// An older fixture is carried forward by the migrations the tool
			// itself prescribes before anything opens it, which is what a
			// person meeting the refusal would do. check exits 2 when it
			// reports a finding, so its code is not read here: what this case
			// is about is what filing writes afterwards, and the open below
			// fails loudly if a migration did not happen.
			runCLI(t, root, "--workbench", root, "check", "--migrate-container", "--yes")
			opened := benchDir(t, root)
			runCLI(t, opened, "--workbench", opened, "check", "--migrate-vocabulary", "--yes")
			// Then the number registry and the notes, which is the rest of
			// the chain. The note migration refuses a store whose numbers
			// still live in card frontmatter, because stamping the current
			// format over one would silence the refusal that protects those
			// numbers, and since dinah-525 nothing reads a store below the
			// current format at all.
			runCLI(t, opened, "--workbench", opened, "check", "--migrate-numbers", "--yes")
			migrateNotes(t, opened)
			got := shapeOfFiledItems(t, opened)
			for kind, keys := range fresh {
				carried, filed := got[kind]
				if !filed {
					t.Errorf("filing a %s on this fixture produced no item", kind)
					continue
				}
				if carried != keys {
					t.Errorf("a %s filed on this fixture carries [%s], and one filed on a workbench this build created carries [%s]",
						kind, carried, keys)
				}
			}
		})
	}
}

// freshWorkbench creates a workbench with the build under test and returns the
// directory the anchor sits in, which is what the fixture copies stand beside.
func freshWorkbench(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workbench")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "sam")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, root, "init", "--slug", "shape", "--operator", "sam"); got.code != 0 {
		t.Fatalf("init: exit %d, %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "a card to file items on"); got.code != 0 {
		t.Fatalf("add: exit %d, %s", got.code, got.errw)
	}
	return benchDir(t, root)
}

// shapeOfFiledItems files one item of each kind on the first card of a
// workbench and returns, per kind, the frontmatter keys the anchor carries,
// sorted and joined.
func shapeOfFiledItems(t *testing.T, root string) map[string]string {
	t.Helper()
	card := firstCardRef(t, root)
	shape := map[string]string{}
	for _, kind := range itemKindsFiled {
		if got := runCLI(t, root, "--workbench", root, "file", card, kind, "a judgement this case records"); got.code != 0 {
			t.Fatalf("file %s on %s: exit %d, %s", kind, card, got.code, got.errw)
		}
	}
	listed19, listedErr19 := bench.ListIDs(filepath.Join(cardDirOf(t, root, card), bench.ChecklistDir))
	if listedErr19 != nil {
		t.Fatalf("listing %s: %v", filepath.Join(cardDirOf(t, root, card), bench.ChecklistDir), listedErr19)
	}
	for _, dir := range listed19 {
		anchor := filepath.Join(cardDirOf(t, root, card), bench.ChecklistDir, dir, bench.ItemAnchor)
		text, err := bench.ReadText(anchor)
		if err != nil {
			t.Fatalf("read %s: %v", anchor, err)
		}
		fm, _ := bench.ParseAnchor(text)
		keys := append([]string(nil), fm.Keys()...)
		sort.Strings(keys)
		shape[fm.Value(bench.ItemKindField)] = strings.Join(keys, ", ")
	}
	return shape
}

// firstCardRef is the reference of the workbench's lowest-numbered live card,
// read out of the listing the binary prints rather than out of the tree, so
// this case reaches an old fixture's cards the same way a person would.
func firstCardRef(t *testing.T, root string) string {
	t.Helper()
	listed := runCLI(t, root, "--workbench", root, "--json", "list", "cards")
	if listed.code != 0 {
		t.Fatalf("ls: exit %d, %s", listed.code, listed.errw)
	}
	ref := firstRefIn(listed.out)
	if ref == "" {
		t.Fatalf("the listing names no card, so there is nothing to file on:\n%s", listed.out)
	}
	return ref
}

// firstRefIn reads the first card reference out of a JSON listing, by the one
// member every card view carries.
func firstRefIn(payload string) string {
	const key = `"ref":`
	at := strings.Index(payload, key)
	if at < 0 {
		return ""
	}
	rest := payload[at+len(key):]
	open := strings.Index(rest, `"`)
	if open < 0 {
		return ""
	}
	rest = rest[open+1:]
	shut := strings.Index(rest, `"`)
	if shut < 0 {
		return ""
	}
	return rest[:shut]
}

// cardDirOf is the directory a card's anchor sits in, asked of the binary
// rather than composed here, so an older container shape resolves the same way
// this build resolves its own.
func cardDirOf(t *testing.T, root, ref string) string {
	t.Helper()
	anchor := runCLI(t, root, "--workbench", root, "path", ref)
	if anchor.code != 0 {
		t.Fatalf("path %s: exit %d, %s", ref, anchor.code, anchor.errw)
	}
	return filepath.Dir(strings.TrimSpace(anchor.out))
}
