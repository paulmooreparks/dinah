package bench

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// failingMemberWalk stands in for filepath.WalkDir over a member subtree that
// exists and will not walk, which is a condition no fixture on disk can
// produce. filepath.WalkDir over a path holding a plain file calls its
// callback once with a nil error, measured on 2026-09-11 as calls=1
// sawErr=<nil> walkErr=<nil>, and memberPaths filters every member through
// Exists before the walk starts, so the absent case never arrives either.
//
// It counts its own invocations, because a substitution nothing called would
// leave the half of a test that rests on it proving nothing.
func failingMemberWalk(calls *int) func(string, fs.WalkDirFunc) error {
	return func(root string, walk fs.WalkDirFunc) error {
		*calls++
		return walk(root, nil, errors.New("the member subtree would not walk"))
	}
}

// withFailingMemberWalk substitutes the seam for one test and restores it
// afterwards, returning the pointer the caller asserts the call count through.
func withFailingMemberWalk(t *testing.T) *int {
	t.Helper()
	calls := 0
	original := memberWalk
	memberWalk = failingMemberWalk(&calls)
	t.Cleanup(func() { memberWalk = original })
	return &calls
}

// pathListing is every path beneath a directory, relative and sorted, which is
// what a before-and-after comparison of a refused migration is made of. An
// absent directory answers an empty listing rather than failing, because a
// migration that moved a directory away is exactly what this catches.
func pathListing(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(relative))
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(found)
	return found
}

// sameListing fails naming the two listings when a refused act moved anything.
func sameListing(t *testing.T, what string, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: the tree held %d paths before and %d after:\n%v\n%v", what, len(want), len(got), want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("%s: path %d reads %q and read %q before", what, i, got[i], want[i])
		}
	}
}

// TestHeldLocksReportsARootAndAMemberItCannotRead covers both of heldLocks'
// reads, and it reaches them two different ways because one way cannot reach
// both. The root read is exercised by a workbench root replaced by a plain
// file. The member walk is exercised through the memberWalk seam, which is the
// only route to it, for the reason failingMemberWalk's comment gives.
//
// The succeeding cases run in the same test: a healthy workbench with no lock
// answers no locks and no error, and one carrying a lock answers that lock and
// no error, so a build that reported an error for everything fails here.
func TestHeldLocksReportsARootAndAMemberItCannotRead(t *testing.T) {
	t.Run("a root that will not read", func(t *testing.T) {
		unreadable := plantUnreadable(t, filepath.Join(t.TempDir(), "workbench"))
		if _, err := heldLocks(unreadable); err == nil {
			t.Fatal("heldLocks reported no error for a workbench root it could not read")
		}
	})

	t.Run("a member subtree that will not walk", func(t *testing.T) {
		root := populatedBench(t, filepath.Join(t.TempDir(), UserBaseName, fixtureWorkbenchID), benchDefinition)
		calls := withFailingMemberWalk(t)
		_, err := heldLocks(root)
		if *calls == 0 {
			t.Fatal("the substituted walk was never called, so this half of the test proves nothing")
		}
		if err == nil {
			t.Fatal("heldLocks reported no error for a member directory it could not walk")
		}
	})

	t.Run("a healthy workbench holding no lock", func(t *testing.T) {
		root := populatedBench(t, filepath.Join(t.TempDir(), UserBaseName, fixtureWorkbenchID), benchDefinition)
		held, err := heldLocks(root)
		if err != nil {
			t.Fatalf("heldLocks answered an error for a healthy workbench: %v", err)
		}
		if len(held) != 0 {
			t.Errorf("heldLocks answered %v for a workbench holding no lock", held)
		}
	})

	t.Run("a healthy workbench holding one lock", func(t *testing.T) {
		root := populatedBench(t, filepath.Join(t.TempDir(), UserBaseName, fixtureWorkbenchID), benchDefinition)
		lock := filepath.Join(root, CardsDir, "c00000000001", LockName)
		write(t, lock, "{\"holder\":\"alka\"}\n")
		held, err := heldLocks(root)
		if err != nil {
			t.Fatalf("heldLocks answered an error for a workbench carrying one lock: %v", err)
		}
		if len(held) != 1 || held[0] != lock {
			t.Errorf("heldLocks answered %v, wanted the one lock at %s", held, lock)
		}
	})
}

// TestTheContainerMigrationRefusesALockStateItCannotRead asserts that each of
// the three writers refuses rather than acting when it cannot read the lock
// state, and that nothing on disk moved.
//
// Each refusing case is driven through the memberWalk seam rather than through
// a plain file at the workbench root, because a root that is itself a plain
// file is not a workbench and never reaches these callers in the field, while
// a member subtree that exists and will not read is the trigger this card is
// named for.
//
// The succeeding cases run in the same test with the seam left alone, so a
// build that refused everything fails here.
func TestTheContainerMigrationRefusesALockStateItCannotRead(t *testing.T) {
	t.Run("remintInPlace refuses and renames nothing", func(t *testing.T) {
		base := t.TempDir()
		root := populatedBench(t, filepath.Join(base, UserBaseName, fixtureWorkbenchID), benchDefinition)
		before := pathListing(t, base)
		calls := withFailingMemberWalk(t)
		_, err := remintInPlace(root)
		if *calls == 0 {
			t.Fatal("the substituted walk was never called, so this case proves nothing")
		}
		// The tree is compared before the error is asked about, so a build
		// that proceeded on an unreadable lock state is caught by what it
		// did to the disk rather than by what it failed to return.
		sameListing(t, "the workbench directory was renamed while its lock state was unreadable", before, pathListing(t, base))
		if err == nil {
			t.Fatal("remintInPlace reported no error while the lock state was unreadable")
		}
	})

	t.Run("liftIntoContainer refuses and moves nothing", func(t *testing.T) {
		base := t.TempDir()
		root := populatedBench(t, filepath.Join(base, "project"), benchDefinition)
		before := pathListing(t, base)
		calls := withFailingMemberWalk(t)
		_, err := liftIntoContainer(root)
		if *calls == 0 {
			t.Fatal("the substituted walk was never called, so this case proves nothing")
		}
		sameListing(t, "the workbench members were lifted while the lock state was unreadable", before, pathListing(t, base))
		if err == nil {
			t.Fatal("liftIntoContainer reported no error while the lock state was unreadable")
		}
	})

	t.Run("finishContained refuses and stamps nothing", func(t *testing.T) {
		base := t.TempDir()
		root := populatedBench(t, filepath.Join(base, UserBaseName, fixtureWorkbenchID), benchDefinition)
		before := pathListing(t, base)
		anchorBefore, err := ReadText(filepath.Join(root, WorkbenchAnchor))
		if err != nil {
			t.Fatalf("read the anchor: %v", err)
		}
		calls := withFailingMemberWalk(t)
		refusal := finishContained(root)
		if *calls == 0 {
			t.Fatal("the substituted walk was never called, so this case proves nothing")
		}
		sameListing(t, "the stamp ran while the lock state was unreadable", before, pathListing(t, base))
		anchorAfter, err := ReadText(filepath.Join(root, WorkbenchAnchor))
		if err != nil {
			t.Fatalf("read the anchor again: %v", err)
		}
		if anchorAfter != anchorBefore {
			t.Fatalf("the anchor was stamped while the lock state was unreadable:\n%q\n%q", anchorBefore, anchorAfter)
		}
		if refusal == nil {
			t.Fatal("finishContained reported no error while the lock state was unreadable")
		}
	})

	t.Run("the three still act on a healthy workbench", func(t *testing.T) {
		base := t.TempDir()
		root := populatedBench(t, filepath.Join(base, UserBaseName, fixtureWorkbenchID), benchDefinition)
		target, err := remintInPlace(root)
		if err != nil {
			t.Fatalf("remintInPlace on a healthy workbench: %v", err)
		}
		if target == root {
			t.Error("remintInPlace answered the path it was given, so it renamed nothing")
		}
		if Exists(root) {
			t.Errorf("the old directory %s still stands after the remint", root)
		}
		if !Exists(filepath.Join(target, WorkbenchAnchor)) {
			t.Errorf("the reminted workbench at %s carries no anchor", target)
		}

		bare := populatedBench(t, filepath.Join(t.TempDir(), "project"), benchDefinition)
		lifted, err := liftIntoContainer(bare)
		if err != nil {
			t.Fatalf("liftIntoContainer on a healthy workbench: %v", err)
		}
		if !Exists(filepath.Join(lifted, WorkbenchAnchor)) {
			t.Errorf("the lifted workbench at %s carries no anchor", lifted)
		}
		if Exists(filepath.Join(bare, WorkbenchAnchor)) {
			t.Errorf("the anchor still stands at the old path %s", bare)
		}

		stamped := populatedBench(t, filepath.Join(t.TempDir(), UserBaseName, fixtureWorkbenchID), benchDefinition)
		if err := finishContained(stamped); err != nil {
			t.Fatalf("finishContained on a healthy workbench: %v", err)
		}
		declared, declares := declaredFormat(stamped)
		if !declares || declared < ContainerFormat {
			t.Errorf("the stamp left the anchor declaring format %d (declared=%v), wanted at least %d", declared, declares, ContainerFormat)
		}
	})
}

// TestResumableLiftReportsAContainerItCannotRead asserts that a container
// which is there and will not read answers the error rather than answering
// that no interrupted lift exists, which is what made the caller mint a fresh
// target and strand the members an interrupted run had already moved.
//
// Two succeeding cases run in the same test: a container nobody has created
// yet still answers no interrupted lift and no error, which is the ordinary
// first-run path, and a container holding one anchorless member directory
// still answers that directory.
func TestResumableLiftReportsAContainerItCannotRead(t *testing.T) {
	t.Run("a container that will not read", func(t *testing.T) {
		unreadable := plantUnreadable(t, filepath.Join(t.TempDir(), UserBaseName))
		if _, err := resumableLift(unreadable); err == nil {
			t.Fatal("resumableLift answered no interrupted lift for a container it could not read")
		}
	})

	t.Run("a container nobody has created yet", func(t *testing.T) {
		absent := filepath.Join(t.TempDir(), UserBaseName)
		found, err := resumableLift(absent)
		if err != nil {
			t.Fatalf("resumableLift answered an error for an absent container: %v", err)
		}
		if found != "" {
			t.Errorf("resumableLift answered %q for a container nobody has created", found)
		}
	})

	t.Run("a container holding one anchorless member directory", func(t *testing.T) {
		container := filepath.Join(t.TempDir(), UserBaseName)
		partial := filepath.Join(container, fixtureWorkbenchID)
		write(t, filepath.Join(partial, CardsDir, "c00000000001", CardAnchor), cleanCard)
		found, err := resumableLift(container)
		if err != nil {
			t.Fatalf("resumableLift on an interrupted lift: %v", err)
		}
		if found != partial {
			t.Errorf("resumableLift answered %q, wanted the interrupted directory %q", found, partial)
		}
	})
}
