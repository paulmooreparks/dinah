package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
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
// So the table is checked against journals rather than against a reading. The
// check reads two trees and asks of every field on every event in either one
// whether the table's row for that event names it. A field a verb starts
// writing fails here until the row admits it.
//
// The two trees answer two different questions and neither answers both. The
// compatibility sample under internal/compattest is the tree with the reach,
// because TestTheSampleFixtureCarriesEveryJournalEventTheContractDeclares
// holds it to the whole event vocabulary internal/contract declares, so every
// event but the one absentEvents excuses arrives here through it. What the
// sample cannot carry is a shape younger than the capture, and a column
// comment's locator is exactly that, so exerciseTheJournalWriters runs the
// card's own acts through the tool live. The union of the two is what gets
// walked, and the live half is armed below on the shape the card invented.
//
// What the check does not do, stated because a green run otherwise reads as
// more than it is. It checks that a row names a field some build writes, and
// not the other way round, so a row naming a field no build writes any longer
// goes unreported. And it says nothing about whether a field is listed in the
// right one of the row's two columns, which is a judgement about when a field
// is written rather than about whether it exists.

// universalEventFields are the three fields every line carries whatever the
// event, which the paragraph above the table states in prose and no row
// repeats.
var universalEventFields = map[string]bool{"ts": true, "event": true, "actor": true}

// eventsTheTableDoesNotDescribe are the events this build declares and the
// event table carries no row for at all.
//
// They are older than dinah-518 and they are not that card's subject, which is
// why they are recorded here rather than repaired under it.
//
// The list is held to be exactly right in both directions, and it is held
// against the event vocabulary internal/contract declares rather than against
// whatever a fixture happens to write. A declared event with no row and no
// entry here fails, so an event added tomorrow and left out of the table is
// reported whether or not any tree this check reads carries a line of it. A
// name here that the table has since gained fails too, so writing the row is
// what deletes the entry rather than somebody remembering to.
var eventsTheTableDoesNotDescribe = []string{
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

// TestTheFormatDocumentNamesEveryFieldAJournalLineCarries walks two trees'
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
	for _, event := range declaredEvents(t) {
		if _, named := rows[event]; named {
			continue
		}
		if undescribed[event] {
			continue
		}
		t.Errorf("internal/contract declares the %s event, the event table carries no row for it, and eventsTheTableDoesNotDescribe does not excuse it", event)
	}

	live := readShape(t, benchDir(t, exerciseTheJournalWriters(t))).members
	frozen := readShape(t, sampleFixture(t)).members
	union := map[string]map[string]bool{}
	for _, half := range []map[string]map[string]bool{frozen, live} {
		for event, fields := range half {
			if union[event] == nil {
				union[event] = map[string]bool{}
			}
			for field := range fields {
				union[event][field] = true
			}
		}
	}

	checked := 0
	for event, fields := range union {
		row, named := rows[event]
		if !named {
			if !undescribed[event] {
				t.Errorf("the event table names no row for %q, which one of these trees carries, and no entry excuses it", event)
			}
			continue
		}
		checked++
		for field := range fields {
			if universalEventFields[field] {
				continue
			}
			if !strings.Contains(row, "`"+field+"`") {
				t.Errorf("a %s line carries %q, and the event table's row for %s does not name it:\n%s", event, field, event, row)
			}
		}
	}

	// The arming, on the live half. A run that wrote no column comment would
	// pass this check against exactly the stale table it was written to
	// catch, and the frozen sample cannot carry that shape, so the live tree
	// is asserted to have produced it.
	if !live["commented"]["column"] {
		t.Error("no commented line in the live run carried a column, so the shape this check was written for was never put through it")
	}

	// The reach, asserted rather than reported. Every event the build
	// declares has to arrive here through one half or the other, bar the one
	// absentEvents carries a written reason for, so a capture that stops
	// covering the vocabulary is reported here as well as by the coverage
	// alarm that owns the sample.
	var missing []string
	for _, event := range declaredEvents(t) {
		if union[event] != nil {
			continue
		}
		if _, exempt := absentEvents[event]; exempt {
			continue
		}
		missing = append(missing, event)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("neither tree carries a line of %d declared events, so the table's rows for them are unchecked: %s", len(missing), strings.Join(missing, ", "))
	}
	t.Logf("the check walked %d events, of which %d carry a table row; the live tree reached %d and the sample fixture %d", len(union), checked, len(live), len(frozen))
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

// exerciseTheJournalWriters runs the acts whose lines the live half of the
// check pins and returns the container they were run against.
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
