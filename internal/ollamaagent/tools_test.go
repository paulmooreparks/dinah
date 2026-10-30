package ollamaagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// treeOf makes a folder of count files named f000.txt, f001.txt and so on,
// each holding body, which a walk reaches in that order.
func treeOf(t *testing.T, count int, body string) string {
	t.Helper()
	root := t.TempDir()
	for i := range count {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%03d.txt", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func withSearchLimits(t *testing.T, files, bytes int) {
	t.Helper()
	before := searchLimits
	searchLimits.files, searchLimits.bytes = files, bytes
	t.Cleanup(func() { searchLimits = before })
}

// The limits on one call's work are 20,000 files visited and 256 MiB read.
func TestSearchLimitsAreTheOnesDecided(t *testing.T) {
	if SearchMaxFiles != 20000 || SearchMaxBytes != 256<<20 {
		t.Fatalf("the limits are %d files and %d bytes", SearchMaxFiles, SearchMaxBytes)
	}
	if searchLimits.files != SearchMaxFiles || searchLimits.bytes != SearchMaxBytes {
		t.Fatalf("the limits in force are %+v", searchLimits)
	}
}

// A search or a find that is stopped says so. Both once ignored the stop,
// went on through the whole tree, and a search then answered that no line
// matched, as though it had looked.
func TestAStoppedSearchOrFindSaysItWasStopped(t *testing.T) {
	root := treeOf(t, 40, "a needle here\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for name, out := range map[string]func() (string, bool){
		"search": func() (string, bool) { return search(ctx, root, "needle", "") },
		"find":   func() (string, bool) { return find(ctx, root, "**/*.txt") },
	} {
		text, ok := out()
		if ok || !strings.Contains(text, "stopped before it finished") || strings.Contains(text, "No line matches") || strings.Contains(text, "No file matches") {
			t.Errorf("%s of a stopped call gave %v %q", name, ok, text)
		}
	}
}

// A search that reaches its file limit returns the matches it found, in the
// order it walked, and a note that it stopped early and how far it got. The
// files past the limit are not looked at.
func TestASearchStopsAtItsFileLimit(t *testing.T) {
	withSearchLimits(t, 10, SearchMaxBytes)
	root := treeOf(t, 30, "a needle here\n")
	text, ok := search(context.Background(), root, "needle", "")
	if !ok {
		t.Fatalf("a search at its limit failed: %q", text)
	}
	lines := strings.Split(text, "\n")
	if len(lines) != 11 || !strings.HasPrefix(lines[0], "f000.txt:1:") || !strings.HasPrefix(lines[9], "f009.txt:1:") {
		t.Fatalf("the matches before the limit: %q", text)
	}
	note := lines[10]
	if !strings.HasPrefix(note, "[The search stopped early.") || !strings.Contains(note, "visited 10 files") || !strings.Contains(note, `"f009.txt"`) || !strings.Contains(note, "narrower") {
		t.Fatalf("the note: %q", note)
	}
	if strings.Contains(text, "f010.txt") {
		t.Fatal("a file past the limit was searched")
	}
}

// A search that reaches its limit with nothing found must not read as having
// found nothing.
func TestAPartialSearchWithNoMatchesDoesNotSayNoMatch(t *testing.T) {
	withSearchLimits(t, 5, SearchMaxBytes)
	root := treeOf(t, 20, "nothing to see\n")
	text, ok := search(context.Background(), root, "needle", "")
	if !ok || !strings.HasPrefix(text, "[The search stopped early.") || strings.Contains(text, "No line matches") {
		t.Fatalf("got %v %q", ok, text)
	}
}

func TestASearchStopsAtItsByteLimit(t *testing.T) {
	withSearchLimits(t, SearchMaxFiles, 1000)
	root := treeOf(t, 10, "needle"+strings.Repeat("x", 394)+"\n")
	text, ok := search(context.Background(), root, "needle", "")
	// Two files of 401 bytes fit in 1000; the third would pass it.
	if !ok || strings.Count(text, "\n") != 2 || !strings.Contains(text, "f001.txt:1:") || strings.Contains(text, "f002.txt:1:") || !strings.Contains(text, "visited 3 files") {
		t.Fatalf("got %v %q", ok, text)
	}
}

func TestAFindStopsAtItsFileLimit(t *testing.T) {
	withSearchLimits(t, 10, SearchMaxBytes)
	root := treeOf(t, 30, "x")
	text, ok := find(context.Background(), root, "**/*.txt")
	lines := strings.Split(text, "\n")
	if !ok || len(lines) != 11 || lines[0] != "f000.txt" || lines[9] != "f009.txt" || !strings.Contains(lines[10], "visited 10 files") {
		t.Fatalf("got %v %q", ok, text)
	}
}

// Under the limits, nothing changes: a complete answer carries no note.
func TestASearchUnderTheLimitsCarriesNoNote(t *testing.T) {
	root := treeOf(t, 5, "a needle here\n")
	text, ok := search(context.Background(), root, "needle", "")
	if !ok || strings.Contains(text, "stopped") || strings.Count(text, "\n") != 4 {
		t.Fatalf("got %v %q", ok, text)
	}
}
