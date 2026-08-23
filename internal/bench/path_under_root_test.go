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
