package main

import (
	"sort"
	"strings"
	"testing"
)

// A folder's entries are in the order a person counts in: a run of digits
// in a name is a number, so the second entry comes before the tenth.
func TestAFoldersEntriesAreOrderedByTheNumbersInTheirNames(t *testing.T) {
	names := []string{"REC-10", "REC-2", "REC-1", "REC-100", "REC-19", "REC-9", "rec-3", "notes", "Part 10.md", "part 2.md", "part 02.md", "2026", "0999", "a", "A"}
	sort.Slice(names, func(i, j int) bool { return folderOrder(false, names[i], false, names[j]) })
	want := "0999|2026|A|a|notes|part 02.md|part 2.md|Part 10.md|REC-1|REC-2|rec-3|REC-9|REC-10|REC-19|REC-100"
	if got := strings.Join(names, "|"); got != want {
		t.Fatalf("the order is\n%s\nand should be\n%s", got, want)
	}
}

// Folders still come before files, whatever their names.
func TestFoldersComeBeforeFilesInNumberOrder(t *testing.T) {
	if !folderOrder(true, "z10", false, "a1") || folderOrder(false, "a1", true, "z10") {
		t.Fatal("a folder did not come before a file")
	}
	if !folderOrder(true, "set 2", true, "set 10") {
		t.Fatal("folders were not ordered by the numbers in their names")
	}
}

// The order is the same on every listing, so no two different names are
// ever equal, and a name is never before itself.
func TestTheOrderOfNamesIsStrict(t *testing.T) {
	names := []string{"a2", "a02", "A2", "a002", "a2b", "a2B", "", "2", "02"}
	for _, a := range names {
		for _, b := range names {
			ab, ba := folderOrder(false, a, false, b), folderOrder(false, b, false, a)
			if a == b && (ab || ba) {
				t.Fatalf("%q is before itself", a)
			}
			if a != b && ab == ba {
				t.Fatalf("%q and %q have no order between them", a, b)
			}
		}
	}
}
