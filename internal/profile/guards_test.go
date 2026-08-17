package profile

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// lockAcquisition matches the exclusive-create primitive a lock is taken with.
var lockAcquisition = regexp.MustCompile(`\bO_EXCL\b`)

// lockWrite matches a line that both names a lock file and writes one, which
// is the other shape a hand-rolled acquisition takes.
var lockWrite = regexp.MustCompile(`\b(OpenFile|Create|WriteText|WriteFile)\b`)
var lockNamed = regexp.MustCompile(`\b(LockName|SiblingSuffix)\b`)

// theOneAcquirer is the file every lock in this codebase is created in.
const theOneAcquirer = "internal/bench/lock.go"

// TestNoLockIsCreatedOutsideTheOneAcquirer asserts that nothing in the shipped
// binary creates a lock file except the acquisition in internal/bench/lock.go.
//
// The sibling check that stops a writer from working inside a structural act's
// window lives inside that acquisition rather than at each call site, so a
// second place that took a lock would be a place the check could be forgotten.
// An earlier enumeration of the call sites missed one, which is why the rule
// is a guard rather than a comment.
//
// Test sources are outside the scan: a test that plants a lock by hand is
// standing in for another process rather than acquiring one, and no such line
// reaches a caller of this library.
func TestNoLockIsCreatedOutsideTheOneAcquirer(t *testing.T) {
	root := filepath.Join("..", "..")
	scanned := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		name := filepath.ToSlash(relative)
		if name == theOneAcquirer {
			return nil
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		scanned++
		for number, line := range strings.Split(string(source), "\n") {
			if lockAcquisition.MatchString(line) {
				t.Errorf("%s:%d takes a lock with the exclusive-create primitive: %s", name, number+1, strings.TrimSpace(line))
			}
			if lockNamed.MatchString(line) && lockWrite.MatchString(line) {
				t.Errorf("%s:%d writes a lock file: %s", name, number+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if scanned == 0 {
		t.Error("no source was scanned, so this guard proves nothing")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(theOneAcquirer))); err != nil {
		t.Errorf("the one acquirer is not where this guard exempts it: %v", err)
	}
}
