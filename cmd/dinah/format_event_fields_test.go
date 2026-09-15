package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// This file pins the journal event table in docs/design/format.md to the lines
// the build actually writes.
//
// dinah-518 gave a column comment two locator fields on its `commented` line
// and left that table saying the event carries a comment and, on an item, an
// item. The table is not a summary: format.md is the closed statement a second
// implementation of the format reads, so a reader of it would have written no
// locator at all and had no way to tell a column comment from a card comment
// in the workbench journal. Nothing on the card caught it, because prose is
// checked by people and the people all read the code.
//
// So the table is checked against the journal rather than against a reading.
// The check runs a fixture through the tool, reads every journal the run
// wrote, and asks of every field on every line whether the table's row for
// that event names it. A field a verb starts writing fails here until the row
// admits it.
//
// What the check does not do, stated because a green run otherwise reads as
// more than it is. It walks the events its own fixture produces and no others,
// so an event no act below writes is unpinned; the list of pinned events is
// logged on every run so that a reader can see which. It checks that the row
// names a field the build writes, and not the other way round, so a row naming
// a field no build writes any longer goes unreported. And it says nothing
// about whether a field is listed in the right one of the row's two columns,
// which is a judgement about when a field is written rather than about
// whether it exists.

// universalEventFields are the three fields every line carries whatever the
// event, which the paragraph above the table states in prose and no row
// repeats.
var universalEventFields = map[string]bool{"ts": true, "event": true, "actor": true}

// eventsTheTableDoesNotDescribe are the events this build writes and the
// event table carries no row for at all.
//
// They are older than dinah-518 and they are not that card's subject, which is
// why they are recorded here rather than repaired under it. Recording them is
// what stops the next reader rediscovering them and what stops this check
// falling silent about an event added tomorrow: a name not in the table and
// not in this list fails.
//
// The list is held to be exactly right in both directions by the check below.
// A name here that the table has since gained fails too, so writing the row is
// what deletes the entry rather than somebody remembering to.
var eventsTheTableDoesNotDescribe = []string{
	"item_filed",
	"item_cited",
	"item_resolved",
	"item_verified",
	"item_failed",
	"item_reopened",
	"linked",
	"unlinked",
	"renumbered",
}

// formatEventRow matches one row of the event table, capturing the event name
// out of the backticks that open it.
var formatEventRow = regexp.MustCompile("^\\| `([a-z_]+)` \\|")

// TestTheFormatDocumentNamesEveryFieldAJournalLineCarries walks a fixture's
// journals and fails on a field the event table does not name.
func TestTheFormatDocumentNamesEveryFieldAJournalLineCarries(t *testing.T) {
	rows := formatEventTable(t)
	if len(rows) < 20 {
		t.Fatalf("the event table parsed to %d rows, which is too few to be the table; the document's shape has moved", len(rows))
	}

	undescribed := map[string]bool{}
	for _, event := range eventsTheTableDoesNotDescribe {
		if _, named := rows[event]; named {
			t.Errorf("the event table now carries a row for %q, so its entry in eventsTheTableDoesNotDescribe is stale and the event should be checked rather than excused", event)
		}
		undescribed[event] = true
	}

	root := exerciseTheJournalWriters(t)
	seen := map[string]bool{}
	carriedAColumn := false
	for _, line := range everyJournalLine(t, benchDir(t, root)) {
		event, _ := line["event"].(string)
		if event == "" {
			t.Fatalf("a journal line carries no event: %v", line)
		}
		row, named := rows[event]
		if !named {
			if !undescribed[event] {
				t.Errorf("the event table names no row for %q, which this run wrote, and no entry excuses it", event)
			}
			continue
		}
		seen[event] = true
		for field := range line {
			if universalEventFields[field] {
				continue
			}
			if event == "commented" && field == "column" {
				carriedAColumn = true
			}
			if !strings.Contains(row, "`"+field+"`") {
				t.Errorf("a %s line carries %q, and the event table's row for %s does not name it:\n%s", event, field, event, row)
			}
		}
	}

	// The arming. A run that wrote no column comment would pass this check
	// against exactly the stale table it was written to catch, so the shape
	// that went stale is asserted to have been walked.
	if !carriedAColumn {
		t.Error("no commented line in this run carried a column, so the shape this check was written for was never put through it")
	}
	pinned := make([]string, 0, len(seen))
	for event := range seen {
		pinned = append(pinned, event)
	}
	sort.Strings(pinned)
	t.Logf("the fixture pinned %d events: %s", len(pinned), strings.Join(pinned, ", "))
}

// formatEventTable reads docs/design/format.md and returns each event row's
// whole text, keyed by the event it describes.
func formatEventTable(t *testing.T) map[string]string {
	t.Helper()
	path := filepath.Join(repositoryRoot, "docs", "design", "format.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimRight(line, "\r")
		hit := formatEventRow.FindStringSubmatch(line)
		if hit == nil {
			continue
		}
		rows[hit[1]] = line
	}
	return rows
}

// exerciseTheJournalWriters runs the acts whose lines this check pins and
// returns the container they were run against.
//
// Every act here is one somebody performs at a terminal, and the three comment
// shapes are the subject of the card this check was written on: a comment on a
// card, a comment on one of its checklist items, and a comment on a column.
func exerciseTheJournalWriters(t *testing.T) string {
	t.Helper()
	root := newBench(t)
	card := addCard(t, root, "a card")
	acts := [][]string{
		{"comment", card, "a card remark"},
		{"file", "--column", "done", card, "open_question", "Somebody has to settle this."},
		{"comment", card + "/questions/1", "a remark on the question"},
		{"comment", "intake", "a station note"},
		{"move", card, "doing"},
		{"claim", card},
		{"block", card, "the upstream service is unreachable"},
		{"unblock", card},
		{"set", card, "title", "a renamed card"},
		{"set", "intake", "title", "Intake Queue"},
		{"set", "workbench", "title", "A renamed workbench"},
	}
	for _, act := range acts {
		if got := runCLI(t, root, act...); got.code != 0 {
			t.Fatalf("%s: %d %s", strings.Join(act, " "), got.code, got.errw)
		}
	}
	return root
}

// everyJournalLine reads every journal below a workbench, as decoded objects,
// so that a line's fields are read as the fields they were written as rather
// than through the struct that happens to model them today. A field a verb
// writes that no struct member spells would still be reported.
func everyJournalLine(t *testing.T, dir string) []map[string]any {
	t.Helper()
	var lines []map[string]any
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != bench.JournalName {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(body), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(line), &decoded); err != nil {
				t.Fatalf("decode a line of %s: %v", path, err)
			}
			lines = append(lines, decoded)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	if len(lines) == 0 {
		t.Fatalf("no journal stands below %s, so this check read nothing", dir)
	}
	return lines
}
