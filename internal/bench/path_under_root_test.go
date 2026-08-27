package bench

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestAStatThatFailsRefusesOutsideRoot asserts AC-17: a stat that fails with
// fs.ErrPermission, driven through the package's stat seam on the target and
// again on an ancestor between the target and the root, is refused
// dinah.outside-root. PathUnderRoot cannot settle containment when a rung of
// the ancestor chain answers nothing, so the refusal travels rather than a
// guess about the path's standing.
//
// Two cases are tested: the failure on the candidate itself, and on an
// ancestor between the candidate and the root. Both exercises go through
// statPath, mirroring the readAnchorContent seam.
func TestAStatThatFailsRefusesOutsideRoot(t *testing.T) {
	t.Run("fail on the candidate itself", func(t *testing.T) {
		root, child := makeStatFixture(t)
		failing := child
		installStatSeam(t, func(path string) bool { return path == failing })

		contained, err := PathUnderRoot(root, child)
		assertRefused(t, contained, err, failing)
	})
	t.Run("fail on an ancestor below the root", func(t *testing.T) {
		root, child := makeStatFixture(t)
		// The "deeper" rung is between the root and the child candidate.
		failing := filepath.Join(root, "deeper")
		installStatSeam(t, func(path string) bool { return path == failing })

		contained, err := PathUnderRoot(root, child)
		assertRefused(t, contained, err, failing)
	})
}

// makeStatFixture builds a root that holds deeper/child. The child is the
// candidate the stat seam is wired against; the "deeper" directory is the
// rung between the root and the child that the second case breaks.
func makeStatFixture(t *testing.T) (root, child string) {
	t.Helper()
	root = t.TempDir()
	child = filepath.Join(root, "deeper", "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return root, child
}

// installStatSeam overrides statPath to return fs.ErrPermission for paths
// the predicate names. The cleanup restores the previous value so a later
// test in the same package sees the real os.Stat.
func installStatSeam(t *testing.T, failing func(path string) bool) {
	t.Helper()
	previous := statPath
	statPath = func(path string) (os.FileInfo, error) {
		if failing(path) {
			return nil, fs.ErrPermission
		}
		return previous(path)
	}
	t.Cleanup(func() { statPath = previous })
}

// assertRefused confirms PathUnderRoot refused the walk, did not also report
// contained, and that the error the seam raises travels rather than a
// synthetic one.
func assertRefused(t *testing.T, contained bool, err error, failing string) {
	t.Helper()
	if err == nil {
		t.Fatalf("a stat failure on the containment walk should refuse, got contained=%v", contained)
	}
	if contained {
		t.Errorf("a refused containment walk must not also report contained=true")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("the seam error should travel unchanged, got %v", err)
	}
	// The failing path is recorded as the canonical answer the resolveLibrary
	// caller names in the refusal, not here: PathUnderRoot returns the bare
	// stat error and the caller composes it. Keeping that boundary intact is
	// what stops PathUnderRoot from re-rendering for a caller it doesn't know
	// about.
	_ = failing
}

// TestTwoSpellingsOfOneDirectoryAreAdmitted asserts AC-17's first clause: two
// different spellings of one directory are admitted, so containment is settled
// by the filesystem rather than by a string comparison.
//
// The second spelling comes from changing into the directory and asking
// os.Getwd what it is called, which is the mechanism resolvedDir in
// cmd/dinah/main_test.go uses and the reason its comment gives: macOS keeps
// its temporary directory behind a symlink, and a Windows runner can hand out
// an 8.3 short name, so the directory a test built and the directory a running
// process reports are spelled differently there. Reproducing the sequence is
// what keeps this right on every platform without asserting which quirk is in
// play on any one of them.
//
// That mechanism produces one spelling on a platform with no such quirk, and a
// test comparing two identical strings has shown nothing. So the uncleaned
// spelling below carries the clause on those platforms. It names the root by
// descending into a child and climbing back out, which every filesystem
// resolves and no string comparison does.
func TestTwoSpellingsOfOneDirectoryAreAdmitted(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	resolved := resolvedDir(t, root)
	detour := root + string(filepath.Separator) + "child" + string(filepath.Separator) + ".."

	for _, held := range []struct {
		name      string
		root      string
		candidate string
	}{
		{"the root against the spelling a running process reports", root, resolved},
		{"the spelling a running process reports against the root", resolved, root},
		{"a child of the root against the spelling a running process reports", resolved, filepath.Join(root, "child")},
		{"the root spelled through a child and back out", detour, child},
		{"a child spelled through itself and back out", root, filepath.Join(child, "..", "child")},
	} {
		t.Run(held.name, func(t *testing.T) {
			contained, err := PathUnderRoot(held.root, held.candidate)
			if err != nil {
				t.Fatalf("PathUnderRoot(%q, %q): %v", held.root, held.candidate, err)
			}
			if !contained {
				t.Errorf("PathUnderRoot(%q, %q) refused two spellings of one directory", held.root, held.candidate)
			}
		})
	}
}

// resolvedDir returns the spelling a process running in dir reports for it,
// which is what a head resolving its own working directory ends up holding.
// It mirrors resolvedDir in cmd/dinah/main_test.go, which carries the reason
// the two spellings can differ; the mechanism is copied here rather than
// shared because the two live in different packages and a test helper is not
// an exported surface.
func resolvedDir(t *testing.T, dir string) string {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir into %q: %v", dir, err)
	}
	defer os.Chdir(previous)
	resolved, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return resolved
}

// TestAPathJustOutsideTheRootIsRefused asserts AC-17's second clause: a path
// just outside the root is refused, so the bound acts.
//
// The two candidates are the two ways a path lands just outside. A sibling
// whose name opens with the root's own spelling is what a prefix comparison
// admits, and the root's parent is what a comparison run the other way round
// admits. Neither is refused by an error, because nothing failed. The walk
// reached the top of the candidate's ancestor chain and found no rung the
// root stands on.
func TestAPathJustOutsideTheRootIsRefused(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	sibling := filepath.Join(base, "root-sibling")
	for _, dir := range []string{root, sibling} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", dir, err)
		}
	}

	for _, held := range []struct {
		name      string
		candidate string
	}{
		{"a sibling whose name opens with the root's own spelling", sibling},
		{"the root's own parent", base},
	} {
		t.Run(held.name, func(t *testing.T) {
			contained, err := PathUnderRoot(root, held.candidate)
			if err != nil {
				t.Fatalf("a path outside a root that stats cleanly should refuse without an error, got %v", err)
			}
			if contained {
				t.Errorf("PathUnderRoot(%q, %q) admitted a path outside the root", root, held.candidate)
			}
		})
	}
}

// TestARootRemovedSinceTheServerStartedIsRefused asserts AC-17's third
// clause: a path under a root the filesystem no longer holds is refused.
//
// This is the not-exist branch, and it is a different branch from the stat
// failure the first test in this file drives. That one replaces the package's
// stat seam to force a permission error; this one removes the directories and
// lets the real os.Stat answer, which is the failure a long-running server
// actually meets when somebody deletes the root out from under it. A
// containment test built on samePath would pass here and still admit every
// path the process cannot read, so the two branches get separate tests.
func TestARootRemovedSinceTheServerStartedIsRefused(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	child := filepath.Join(root, "deeper", "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if contained, err := PathUnderRoot(root, child); err != nil || !contained {
		t.Fatalf("the fixture should be contained before the removal, got contained=%v err=%v", contained, err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove the root: %v", err)
	}

	contained, err := PathUnderRoot(root, child)
	if err == nil {
		t.Fatalf("a root removed since the server started should refuse, got contained=%v", contained)
	}
	if contained {
		t.Errorf("a refused containment walk must not also report contained=true")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the removal should travel as a not-exist error, got %v", err)
	}
}
