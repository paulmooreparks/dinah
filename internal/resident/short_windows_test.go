//go:build windows

package resident_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/resident"
	"golang.org/x/sys/windows"
)

// shortPath answers GetShortPathNameW's answer for a path.
func shortPath(t *testing.T, path string) string {
	t.Helper()
	long, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(long, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatalf("GetShortPathName(%s): %v", path, err)
	}
	return windows.UTF16ToString(buf[:n])
}

// TestAShortNameIsResolved is part of dinah-619/criteria/8. A change record
// may carry either of a file's two names, and the documentation leaves
// unspecified which; a change naming a card's directory by its short name
// resolves to the long one, and the snapshot mirrors the disk.
//
// Arming: a resolver that answers the path unchanged leaves the short name
// unplaced, so the change reconciles only the collection and never reads the
// card's anchor again.
func TestAShortNameIsResolved(t *testing.T) {
	root := t.TempDir()
	tree(t, root, map[string]string{"cards/0123456789ab/card.md": "before", "cards/0123456789ab/journal.ndjson": ""})
	card := filepath.Join(root, "cards", "0123456789ab")
	short := filepath.Base(shortPath(t, card))
	if strings.EqualFold(short, "0123456789ab") {
		t.Skip("this volume keeps no 8.3 names, so a record can only carry the long form and the resolver is never needed")
	}
	v := watch(t, root)
	write(t, filepath.Join(card, "card.md"), "after, written in place")
	v.manual.Deliver(resident.Change{Path: filepath.Join("cards", short, "card.md"), Action: resident.Modified})
	p := v.next()
	if !names(p, "cards/0123456789ab/card.md") {
		t.Errorf("the change naming %s resolved to %v, wanted cards/0123456789ab/card.md", short, p.Paths)
	}
	mirrors(t, v.current(), root)
	if _, err := os.Stat(card); err != nil {
		t.Fatal(err)
	}
}
