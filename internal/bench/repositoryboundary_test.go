package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// This file covers dinah-541: the ancestor walk stops at the nearest git
// repository root rather than climbing past it, so a scratch tree that sits
// inside a repository never reaches a workbench above that repository, and
// init refuses to write into a directory it did not create. Every fixture
// here builds its own tree under t.TempDir(), which this package's TestMain
// (check_test.go) has already pointed at a directory outside both the
// developer's home and any repository (testenv.IsolateTempDir creates it at
// the volume root), so nothing here depends on where the test binary's own
// temporary directory happens to sit, and a copy of this tree checked out
// inside this repository's own working copy would not change any assertion
// below: every test names its start, its home, and its nativeHome directly
// rather than reading them from the environment or from cwd.

// TestDiscoveryStopsAtTheRepositoryRootWhenAWorkbenchSitsAbove is dinah-541
// AC-2 and AC-3 (position 1): a workbench several levels above a synthetic
// .git directory is not found from a starting directory below the .git, and
// the refusal names both the starting directory and the boundary, and never
// names the user base it never reached.
func TestDiscoveryStopsAtTheRepositoryRootWhenAWorkbenchSitsAbove(t *testing.T) {
	tree := t.TempDir()
	home := filepath.Join(tree, "operator-home")
	writeWorkbench(t, filepath.Join(home, UserBaseName, "d00000000101"), "The operator's own")

	repoRoot := filepath.Join(tree, "scratch", "checkout")
	start := filepath.Join(repoRoot, "internal", "part")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	_, _, err := Discover(start, "", home, "")
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if refusal.Name != contract.WorkbenchBoundary {
		t.Errorf("refusal name: wanted %s, got %s", contract.WorkbenchBoundary, refusal.Name)
	}
	if refusal.Detail != start {
		t.Errorf("the refusal should name where the search began, wanted %q, got %q", start, refusal.Detail)
	}
	if got := refusal.Extra["boundary"]; got != repoRoot {
		t.Errorf("the refusal should name the repository root it stopped at, wanted %q, got %q", repoRoot, got)
	}
	rendered := refusal.Error()
	if strings.Contains(rendered, home) {
		t.Errorf("the rendered refusal should never name the operator's user base, got %q", rendered)
	}
	if got, ok := refusal.Extra["home"]; ok {
		t.Errorf("the refusal should carry no home value at all, since the fallback never ran, got %q", got)
	}
}

// TestDiscoveryFindsAWorkbenchAtTheRepositoryRootItself is dinah-541 AC-1
// (position 2): a repository whose own root carries the .dinah still
// resolves it, both at the root itself and one level below it, since the
// boundary directory's own .dinah is consulted before the climb decides to
// stop there.
func TestDiscoveryFindsAWorkbenchAtTheRepositoryRootItself(t *testing.T) {
	t.Run("workbench at the repository root", func(t *testing.T) {
		tree := t.TempDir()
		repoRoot := filepath.Join(tree, "checkout")
		want := filepath.Join(repoRoot, UserBaseName, "d00000000102")
		writeWorkbench(t, want, "The checkout's own")
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir .git: %v", err)
		}
		start := filepath.Join(repoRoot, "cmd", "dinah")
		if err := os.MkdirAll(start, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		found, _, err := Discover(start, "", filepath.Join(tree, "home"), "")
		if err != nil {
			t.Fatalf("a workbench at the repository root should resolve, got %v", err)
		}
		if found != want {
			t.Errorf("the found workbench: wanted %q, got %q", want, found)
		}
	})

	t.Run("workbench nested inside the repository, still under the root", func(t *testing.T) {
		tree := t.TempDir()
		repoRoot := filepath.Join(tree, "checkout")
		if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir .git: %v", err)
		}
		nested := filepath.Join(repoRoot, "sub")
		want := filepath.Join(nested, UserBaseName, "d00000000103")
		writeWorkbench(t, want, "One rung below the root")
		start := filepath.Join(nested, "deeper")
		if err := os.MkdirAll(start, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		found, _, err := Discover(start, "", filepath.Join(tree, "home"), "")
		if err != nil {
			t.Fatalf("a workbench below the repository root should resolve, got %v", err)
		}
		if found != want {
			t.Errorf("the found workbench: wanted %q, got %q", want, found)
		}
	})
}

// TestDiscoveryOutsideARepositoryStillClimbsToTheUserBase is dinah-541
// position 3: the bound costs nothing outside a repository. A workbench
// several directories above a starting point that carries no .git anywhere
// between them is still found, exactly as it was before this card, and the
// existing discovery tests in this file (TestDiscoveryReportsAnExhaustedWalk
// and its neighbours, none of which plant a .git anywhere in their fixture
// trees) are the regression that pins this outside every case they already
// cover.
func TestDiscoveryOutsideARepositoryStillClimbsToTheUserBase(t *testing.T) {
	tree := t.TempDir()
	want := filepath.Join(tree, UserBaseName, "d00000000104")
	writeWorkbench(t, want, "Several rungs up, no repository between")
	start := filepath.Join(tree, "a", "b", "c", "d")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	found, _, err := Discover(start, "", filepath.Join(t.TempDir(), "home"), "")
	if err != nil {
		t.Fatalf("a workbench above a non-repository start should still resolve, got %v", err)
	}
	if found != want {
		t.Errorf("the found workbench: wanted %q, got %q", want, found)
	}
}

// TestDiscoveryStopsAtALinkedWorktreesGitdirFile is dinah-541 position 4 and
// restates AC-2 with the file form of .git specifically: git-worktree(1)
// DETAILS documents a linked worktree's own root as carrying "a .git file in
// that directory containing gitdir: <path>" rather than a .git directory, and
// the boundary test does not require .git to be a directory to recognise it.
func TestDiscoveryStopsAtALinkedWorktreesGitdirFile(t *testing.T) {
	tree := t.TempDir()
	home := filepath.Join(tree, "operator-home")
	writeWorkbench(t, filepath.Join(home, UserBaseName, "d00000000105"), "The operator's own")

	worktree := filepath.Join(tree, "scratch", "dinah-541-impl", "wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gitdirTarget := filepath.Join(tree, "checkout", ".git", "worktrees", "wt")
	write(t, filepath.Join(worktree, ".git"), "gitdir: "+gitdirTarget+"\n")

	_, _, err := Discover(worktree, "", home, "")
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if refusal.Name != contract.WorkbenchBoundary {
		t.Errorf("refusal name: wanted %s, got %s", contract.WorkbenchBoundary, refusal.Name)
	}
	if got := refusal.Extra["boundary"]; got != worktree {
		t.Errorf("the refusal should name the worktree root as the boundary, wanted %q, got %q", worktree, got)
	}
	if strings.Contains(refusal.Error(), home) {
		t.Errorf("the rendered refusal should never name the operator's user base, got %q", refusal.Error())
	}
}

// TestDiscoveryStopsAtASubmoduleGitdirFile pins the design review's finding
// that a submodule's own root takes the identical file form gitsubmodules(5)
// documents ("a text file... containing... gitdir:"), the same shape
// git-worktree(1) gives a linked worktree, so the boundary test already
// covers it with no separate branch. The fixture nests a submodule's root
// inside a superproject that itself carries a .git directory, with a
// workbench above both, and asserts the walk stops at the inner boundary
// (the submodule) rather than passing through to the outer one.
func TestDiscoveryStopsAtASubmoduleGitdirFile(t *testing.T) {
	tree := t.TempDir()
	home := filepath.Join(tree, "operator-home")
	writeWorkbench(t, filepath.Join(home, UserBaseName, "d00000000106"), "The operator's own")

	superproject := filepath.Join(tree, "scratch", "superproject")
	if err := os.MkdirAll(filepath.Join(superproject, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir superproject .git: %v", err)
	}
	submodule := filepath.Join(superproject, "vendor", "lib")
	start := filepath.Join(submodule, "src")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gitdirTarget := filepath.Join(superproject, ".git", "modules", "vendor", "lib")
	write(t, filepath.Join(submodule, ".git"), "gitdir: "+gitdirTarget+"\n")

	_, _, err := Discover(start, "", home, "")
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		t.Fatalf("wanted a refusal, got %v", err)
	}
	if refusal.Name != contract.WorkbenchBoundary {
		t.Errorf("refusal name: wanted %s, got %s", contract.WorkbenchBoundary, refusal.Name)
	}
	if got := refusal.Extra["boundary"]; got != submodule {
		t.Errorf("the walk should stop at the submodule's own root, the nearest boundary, wanted %q, got %q", submodule, got)
	}
}

// TestDiscoveryBoundaryStillOffersTheNamedRouteAround is dinah-541 AC-4: an
// explicit --workbench naming the workbench found above the boundary
// resolves it, unaffected by the boundary, because DiscoverSource's override
// branch is taken before walk ever runs.
func TestDiscoveryBoundaryStillOffersTheNamedRouteAround(t *testing.T) {
	tree := t.TempDir()
	above := filepath.Join(tree, "above", UserBaseName, "d00000000107")
	writeWorkbench(t, above, "Named explicitly")

	repoRoot := filepath.Join(tree, "above", "scratch", "checkout")
	start := filepath.Join(repoRoot, "src")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	// Unnamed, the boundary refuses.
	if _, _, err := Discover(start, "", filepath.Join(tree, "home"), ""); err == nil {
		t.Fatalf("wanted the boundary to refuse with no override named")
	}

	// Named explicitly, the override resolves it whatever the boundary says.
	found, _, err := Discover(start, above, filepath.Join(tree, "home"), "")
	if err != nil {
		t.Fatalf("an explicit --workbench should resolve past the boundary, got %v", err)
	}
	if found != above {
		t.Errorf("the found workbench: wanted %q, got %q", above, found)
	}
}
