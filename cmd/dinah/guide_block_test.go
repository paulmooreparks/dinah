package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The guide block guard, and the boundary it stops at.
//
// The rule this file replaces forbade a fenced block whose first line opened
// with a dollar sign, and it told authors to write the command without one.
// The instruction was the evasion: a transcript written the way the rule asked
// for was invisible to the rule, and the one guide transcript in the tree went
// stale for four releases underneath it. A rule keyed on a shape always tells
// authors to write the other shape, so this file keys on the block instead.
// Every fenced block of every embedded guide is declared in
// testdata/guide-blocks.txt, and a block nobody declares fails however its
// author spells it.
//
// What this guard cannot see:
//
//   - Whether a declared block teaches the right thing. The class checks hold
//     the shape of a block against what the entry says it is, and a JSON
//     payload showing a call nobody should make parses exactly as well as one
//     showing a call they should.
//   - Anything but the header row of a `shows=table` block. The data rows are
//     invented for the lesson and the separator row's widths follow them, so
//     comparing either would fail on a lesson that chose a wider example.
//   - The two blocks nothing here can drive. A `shows=shell` block runs
//     another program and a `shows=tree` block draws a directory listing, so
//     each declares a reason and the reason is the whole of what it says.

// guideBlockLedger is the file declaring every fenced block of every guide.
var guideBlockLedger = filepath.Join("testdata", "guide-blocks.txt")

// guideBlockKeys are the directives an entry may carry.
var guideBlockKeys = []string{"shows", "command", "fragment", "reason"}

// guideBlockClasses are the values `shows=` takes, each with its own check.
var guideBlockClasses = []string{"commands", "json", "table", "shell", "tree"}

// guideBlockEntry is one declared block.
type guideBlockEntry struct {
	// guide is the guide's file, named as the ledger spells it.
	guide string
	// fence is the one-based line the block's opening marker stands on.
	fence int
	// shows is the block's class.
	shows string
	// command names the command a shows=table block draws.
	command string
	// fragment says a shows=json block shows the members of an object
	// rather than a whole document.
	fragment string
	// reason is why nothing drives the block, or why it takes the shape it
	// takes.
	reason string
	// source is the one-based line of the ledger, for a finding to name.
	source int
}

// guideBlock is one fenced block as the guides carry it.
type guideBlock struct {
	guide string
	fence int
	body  []string
}

// readGuideBlockLedger reads the ledger, reporting a malformed line rather
// than skipping it. A blank line and a line opening with a hash are
// commentary.
func readGuideBlockLedger(t *testing.T) []guideBlockEntry {
	t.Helper()
	source, err := os.ReadFile(guideBlockLedger)
	if err != nil {
		t.Fatalf("read %s: %v", guideBlockLedger, err)
	}
	var entries []guideBlockEntry
	for number, line := range strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		entry, err := parseGuideBlockEntry(trimmed)
		if err != nil {
			t.Errorf("%s:%d: %v", guideBlockLedger, number+1, err)
			continue
		}
		entry.source = number + 1
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		t.Fatalf("%s declares no block, so every check over it reads nothing", guideBlockLedger)
	}
	return entries
}

// parseGuideBlockEntry reads one entry, whose first word is the guide and the
// line its opening fence stands on.
func parseGuideBlockEntry(line string) (guideBlockEntry, error) {
	entry := guideBlockEntry{}
	first, rest, _ := strings.Cut(line, " ")
	guide, number, ok := strings.Cut(first, ":")
	if !ok {
		return entry, fmt.Errorf("the entry opens %q, and an entry opens with the guide and the line its opening fence stands on", first)
	}
	at, err := strconv.Atoi(number)
	if err != nil {
		return entry, fmt.Errorf("the entry names the line %q, which is not a number", number)
	}
	entry.guide, entry.fence = guide, at
	for key, value := range splitDirectives(rest, guideBlockKeys) {
		switch key {
		case "shows":
			entry.shows = value
		case "command":
			entry.command = value
		case "fragment":
			entry.fragment = value
		case "reason":
			entry.reason = value
		}
	}
	if err := entry.validate(); err != nil {
		return entry, err
	}
	return entry, nil
}

// validate reports what a well-formed entry must carry and this one does not.
func (e guideBlockEntry) validate() error {
	if e.shows == "" {
		return fmt.Errorf("the entry declares no shows=, and every entry says what its block is")
	}
	if !namesADirective(e.shows, guideBlockClasses) {
		return fmt.Errorf("the entry declares shows=%s, and the classes are %s", e.shows, strings.Join(guideBlockClasses, ", "))
	}
	switch e.shows {
	case "shell", "tree":
		if e.reason == "" {
			return fmt.Errorf("a shows=%s entry declares no reason=, and nothing here drives such a block", e.shows)
		}
	case "table":
		if e.command == "" {
			return fmt.Errorf("a shows=table entry declares no command=, so nothing says which command draws the table")
		}
	case "json":
		if e.fragment != "" && e.fragment != "members" {
			return fmt.Errorf("a shows=json entry writes fragment=%s, and the one fragment a block may show is members", e.fragment)
		}
		if e.fragment != "" && e.reason == "" {
			return fmt.Errorf("a fragment=members entry declares no reason=, and the reason says which members the block shows")
		}
	}
	if e.command != "" && e.shows != "table" {
		return fmt.Errorf("the entry declares command=%s on a shows=%s block, and only a table names a command", e.command, e.shows)
	}
	return nil
}

// guideBlocksOfTheCorpus walks every embedded guide into its fenced blocks,
// with the fence arithmetic quickStartMarkerRun gives every reading of a fence
// in this package.
func guideBlocksOfTheCorpus(t *testing.T) []guideBlock {
	t.Helper()
	var blocks []guideBlock
	for _, document := range embeddedGuides(t) {
		lines := strings.Split(document.text, "\n")
		for i := 0; i < len(lines); i++ {
			run := quickStartMarkerRun(lines[i])
			if run == 0 {
				continue
			}
			block := guideBlock{guide: document.name, fence: i + 1}
			j := i + 1
			for ; j < len(lines) && quickStartMarkerRun(lines[j]) != run; j++ {
				block.body = append(block.body, lines[j])
			}
			blocks = append(blocks, block)
			i = j
		}
	}
	if len(blocks) == 0 {
		t.Fatal("no embedded guide carries a fenced block, so every check over the ledger reads nothing")
	}
	return blocks
}

// guideBlockKey identifies one block by the guide and the line its fence
// stands on.
func guideBlockKey(guide string, fence int) string {
	return fmt.Sprintf("%s:%d", guide, fence)
}

// TestEveryGuideBlockIsDeclared fails on a fenced block the ledger does not
// name. This is the check that sees an undriven transcript however its author
// spells it, which the dollar-sign rule it replaces could not.
func TestEveryGuideBlockIsDeclared(t *testing.T) {
	declared := map[string]bool{}
	for _, entry := range readGuideBlockLedger(t) {
		declared[guideBlockKey(entry.guide, entry.fence)] = true
	}
	for _, block := range guideBlocksOfTheCorpus(t) {
		if declared[guideBlockKey(block.guide, block.fence)] {
			continue
		}
		t.Errorf("%s:%d opens a fenced block and %s carries no entry for it; declare what the block shows",
			block.guide, block.fence, guideBlockLedger)
	}
}

// TestNoGuideBlockEntryIsStale fails on an entry naming a line that carries no
// opening fence, so an edit that moves a block is repaired rather than leaving
// the block silently unheld.
func TestNoGuideBlockEntryIsStale(t *testing.T) {
	standing := map[string]bool{}
	for _, block := range guideBlocksOfTheCorpus(t) {
		standing[guideBlockKey(block.guide, block.fence)] = true
	}
	for _, entry := range readGuideBlockLedger(t) {
		if standing[guideBlockKey(entry.guide, entry.fence)] {
			continue
		}
		t.Errorf("%s:%d: the entry expects a block to open at %s:%d, and no fence opens there",
			guideBlockLedger, entry.source, entry.guide, entry.fence)
	}
}

// TestEveryGuideBlockShowsWhatItDeclares runs each class's own check over the
// block the entry names.
func TestEveryGuideBlockShowsWhatItDeclares(t *testing.T) {
	entries := readGuideBlockLedger(t)
	blocks := map[string]guideBlock{}
	for _, block := range guideBlocksOfTheCorpus(t) {
		blocks[guideBlockKey(block.guide, block.fence)] = block
	}
	checked := 0
	for _, entry := range entries {
		block, standing := blocks[guideBlockKey(entry.guide, entry.fence)]
		if !standing {
			// The stale rule owns this, and reporting it twice would
			// name one defect in two voices.
			continue
		}
		checked++
		switch entry.shows {
		case "commands":
			checkBlockShowsCommands(t, entry, block)
		case "json":
			checkBlockShowsJSON(t, entry, block)
		case "table":
			checkBlockShowsTheDrawnTable(t, entry, block)
		}
	}
	if checked == 0 {
		t.Fatalf("%s named no standing block, so this check read nothing", guideBlockLedger)
	}
}

// checkBlockShowsCommands holds a block declared as commands to being nothing
// but commands a reader types. A line opening with a dollar sign gets the
// message the retired rule gave authors, and it gets it wherever in the block
// the line stands rather than only on the first line. Any other line that is
// not a command fails too, because a block declared as commands that shows
// output is a transcript nothing drives, which is the defect class this ledger
// exists to catch.
//
// The check does not read the command name. TestTheGuidesTeachOnlyDeclaredFlags
// already holds every command and flag a guide teaches against verb.Params.
func checkBlockShowsCommands(t *testing.T, entry guideBlockEntry, block guideBlock) {
	t.Helper()
	read := 0
	for i, line := range block.body {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		read++
		at := block.fence + 1 + i
		if strings.HasPrefix(trimmed, "$ ") {
			t.Errorf("%s:%d: the block declares commands and this line opens with a command prompt, which is the shape the quick start's replay drives and nothing drives here; write the command without its leading dollar sign",
				entry.guide, at)
			continue
		}
		if !strings.HasPrefix(trimmed, "dinah ") && trimmed != "dinah" {
			t.Errorf("%s:%d: the block declares commands and this line is not one, so the block shows output nothing drives:\n  %s",
				entry.guide, at, trimmed)
		}
	}
	if read == 0 {
		t.Errorf("%s:%d: the block declares commands and carries no line, so this check read nothing", entry.guide, entry.fence)
	}
}

// checkBlockShowsJSON holds a block declared as json to parsing. A block
// showing the members of an object rather than a whole document declares
// fragment=members and is parsed inside a wrapping pair of braces, and such a
// declaration on a block that parses on its own fails, so the declaration
// cannot spread to blocks that do not need it.
func checkBlockShowsJSON(t *testing.T, entry guideBlockEntry, block guideBlock) {
	t.Helper()
	body := strings.TrimSpace(strings.Join(block.body, "\n"))
	if body == "" {
		t.Errorf("%s:%d: the block declares json and carries no body, so this check read nothing", entry.guide, entry.fence)
		return
	}
	var whole json.RawMessage
	parsedAlone := json.Unmarshal([]byte(body), &whole) == nil
	if entry.fragment == "" {
		if !parsedAlone {
			t.Errorf("%s:%d: the block declares json and does not parse: %v",
				entry.guide, entry.fence, json.Unmarshal([]byte(body), &whole))
		}
		return
	}
	if parsedAlone {
		t.Errorf("%s:%d: the block declares fragment=members and parses as a whole document on its own, so the declaration says nothing; remove it",
			entry.guide, entry.fence)
		return
	}
	if err := json.Unmarshal([]byte("{"+body+"}"), &whole); err != nil {
		t.Errorf("%s:%d: the block declares the members of an object and does not parse inside a wrapping pair of braces: %v",
			entry.guide, entry.fence, err)
	}
}

// checkBlockShowsTheDrawnTable holds a block declared as a table to the header
// row the tool draws for the command the entry names.
//
// The header alone is compared. A block's data rows are invented for the
// lesson and the separator row's widths follow them, so comparing either would
// fail on a lesson that chose a wider example while catching nothing the
// header does not. The header is where the audit's fifth finding sat: the
// renderer grew a column and the guide's table did not.
func checkBlockShowsTheDrawnTable(t *testing.T, entry guideBlockEntry, block guideBlock) {
	t.Helper()
	shown := ""
	for _, line := range block.body {
		if strings.TrimSpace(line) != "" {
			shown = line
			break
		}
	}
	if shown == "" {
		t.Errorf("%s:%d: the block declares a table and carries no row, so this check read nothing", entry.guide, entry.fence)
		return
	}
	root := newBench(t)
	got := runCLI(t, root, strings.Fields(entry.command)...)
	if got.code != 0 {
		t.Fatalf("%s:%d: `dinah %s` exited %d: %s", entry.guide, entry.fence, entry.command, got.code, got.errw)
	}
	drawn := ""
	for _, line := range strings.Split(got.out, "\n") {
		if strings.TrimSpace(line) != "" {
			drawn = line
			break
		}
	}
	if drawn == "" {
		t.Errorf("%s:%d: `dinah %s` drew nothing, so this check read nothing", entry.guide, entry.fence, entry.command)
		return
	}
	if linesAgree(shown, drawn) {
		return
	}
	t.Errorf("%s:%d: the table's header row is not the one `dinah %s` draws:\n  the guide draws: %s\n  the tool draws:  %s",
		entry.guide, entry.fence, entry.command, strings.TrimRight(shown, " "), strings.TrimRight(drawn, " "))
}
