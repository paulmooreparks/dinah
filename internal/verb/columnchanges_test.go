package verb

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestTheColumnHalfOfTheWalkIsAnchorsAndNoJournals asserts the first clause of
// dinah-515 criterion 20: bench.WatchedEntities answers a column half
// carrying one entry per live column, keyed columns/<id>, with the column
// anchor path and an empty journal.
func TestTheColumnHalfOfTheWalkIsAnchorsAndNoJournals(t *testing.T) {
	l := columnFixture(t)
	_, _, columns, err := l.Bench.WatchedEntities()
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(columns) == 0 {
		t.Fatal("the column half is empty, so this check read nothing")
	}
	if len(columns) != len(l.Bench.Columns) {
		t.Errorf("the column half carries %d entries and the flow declares %d columns", len(columns), len(l.Bench.Columns))
	}
	swept := 0
	for _, entry := range columns {
		id := strings.TrimPrefix(entry.Key, bench.ColumnsDir+"/")
		if id == entry.Key {
			t.Errorf("the entry %q is not keyed under the columns collection", entry.Key)
			continue
		}
		if l.Bench.Column(id) == nil {
			t.Errorf("the entry %q names no live column", entry.Key)
		}
		if entry.Journal != "" {
			t.Errorf("the entry %q carries the journal %q, and a column has none", entry.Key, entry.Journal)
		}
		if entry.Size != 0 {
			t.Errorf("the entry %q carries the journal size %d, and a column has no journal", entry.Key, entry.Size)
		}
		if entry.Anchor != l.Bench.ColumnAnchorPath(id) {
			t.Errorf("the entry %q carries the anchor %q, wanted %q", entry.Key, entry.Anchor, l.Bench.ColumnAnchorPath(id))
		}
		if entry.Revision == "" {
			t.Errorf("the entry %q carries no revision, so the term rests on nothing", entry.Key)
		}
		swept++
	}
	if swept != len(columns) {
		t.Errorf("swept %d entries, wanted %d", swept, len(columns))
	}
}

// TestReadHalfSkipsAnEntryCarryingNoJournal asserts the second clause of
// dinah-515 criterion 20: the skip is decided by the empty string rather than
// by the error reading an empty path happens to give on one platform.
//
// The assertion is over the source rather than over a run, because a run
// cannot see which of the two decided: bench.ReadJournal("") answers an
// error either way on this machine, so a build relying on the errno passes a
// behavioural test and fails on a platform nobody here runs.
func TestReadHalfSkipsAnEntryCarryingNoJournal(t *testing.T) {
	// The behavioural half: a column entry contributes no delivered line and
	// is never named unreadable, whatever bench.ReadJournal would say.
	entries := []bench.Watched{{Key: bench.ColumnsDir + "/aaaaaaaaaaaa", Anchor: "nowhere", Revision: "sha256:0"}}
	delivered, unreadable := readHalf(entries, cursor{}, nil)
	if len(delivered) != 0 {
		t.Errorf("a journal-less entry delivered %d lines", len(delivered))
	}
	if len(unreadable) != 0 {
		t.Errorf("a journal-less entry was named unreadable: %v", unreadable)
	}

	// The source half: the skip tests the empty string.
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, "changes.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse changes.go: %v", err)
	}
	guarded := false
	ast.Inspect(file, func(node ast.Node) bool {
		declared, ok := node.(*ast.FuncDecl)
		if !ok || declared.Name.Name != "readHalf" {
			return true
		}
		ast.Inspect(declared, func(inner ast.Node) bool {
			comparison, ok := inner.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			left, leftOK := comparison.X.(*ast.SelectorExpr)
			right, rightOK := comparison.Y.(*ast.BasicLit)
			if leftOK && rightOK && left.Sel.Name == "Journal" && right.Value == `""` {
				guarded = true
			}
			return true
		})
		return false
	})
	if !guarded {
		t.Error("readHalf does not test Journal against the empty string, so the skip rests on an errno rather than on the struct")
	}
}

// TestAHandEditedColumnMovesTheColumnTermAlone asserts the third and fourth
// clauses of dinah-515 criterion 20: a hand edit to a column anchor moves the
// new column term and leaves the live term where it was, and the answer
// reports the change as a Columns entry with the cards array empty.
func TestAHandEditedColumnMovesTheColumnTermAlone(t *testing.T) {
	l := columnFixture(t)
	minted, err := l.Changes(&Request{Verb: "changes"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	before := mustDecodeCursor(t, minted.Cursor)

	column := l.Bench.Columns[0]
	anchor := l.Bench.ColumnAnchorPath(column.ID)
	raw, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	edited := strings.Replace(string(raw), "title: "+column.Title, "title: "+column.Title+"RENAMED", 1)
	if edited == string(raw) {
		t.Fatalf("the column anchor carries no title line to edit:\n%s", raw)
	}
	if err := os.WriteFile(anchor, []byte(edited), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	l = reopenFixture(t, l)

	answer, err := l.Changes(&Request{Verb: "changes", Since: minted.Cursor})
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	if !answer.Changed {
		t.Fatal("a hand-edited column anchor reported no change at all")
	}
	after := mustDecodeCursor(t, answer.Cursor)
	if after.Live != before.Live {
		t.Error("the live term moved on a column edit, which would resync every card on the board")
	}
	if after.Columns == before.Columns {
		t.Error("the column term did not move on a column edit")
	}
	if len(answer.Cards) != 0 {
		t.Errorf("a column edit reported %d cards, and it must report none", len(answer.Cards))
	}
	if len(answer.Columns) == 0 {
		t.Fatal("a column edit reported no Columns entry")
	}
	found := false
	for _, reported := range answer.Columns {
		if reported.ID != column.ID {
			continue
		}
		found = true
		if reported.Title != column.Title+"RENAMED" {
			t.Errorf("the entry reports the title %q, wanted the edited %q", reported.Title, column.Title+"RENAMED")
		}
		if reported.HoldsOnEntry || reported.HoldsOnExit {
			t.Errorf("the entry reports the holds %v and %v, and the fixture's column holds neither way", reported.HoldsOnEntry, reported.HoldsOnExit)
		}
	}
	if !found {
		t.Errorf("no entry names the column that was edited: %+v", answer.Columns)
	}
}

// TestAHandEditedGateFlipsOneHoldAndNotTheOther asserts the gate clause of
// dinah-515 criterion 20: gate_items edited from absent to out flips
// HoldsOnExit while HoldsOnEntry stays false, which is the one fact this
// notice exists to report and the one ColumnView cannot carry.
func TestAHandEditedGateFlipsOneHoldAndNotTheOther(t *testing.T) {
	l := columnFixture(t)
	minted, err := l.Changes(&Request{Verb: "changes"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	column := l.Bench.Columns[0]
	if column.HoldsOnEntry() || column.HoldsOnExit() {
		t.Fatalf("the fixture's first column already holds, so the flip below proves nothing")
	}
	anchor := l.Bench.ColumnAnchorPath(column.ID)
	raw, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gated := strings.Replace(string(raw), "---\ntitle:", "---\ngate_items: out\ntitle:", 1)
	if gated == string(raw) {
		t.Fatalf("the column anchor could not be given a gate_items line:\n%s", raw)
	}
	if err := os.WriteFile(anchor, []byte(gated), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	l = reopenFixture(t, l)

	answer, err := l.Changes(&Request{Verb: "changes", Since: minted.Cursor})
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	for _, reported := range answer.Columns {
		if reported.ID != column.ID {
			continue
		}
		if !reported.HoldsOnExit {
			t.Error("the entry reports no exit hold after gate_items was edited to out")
		}
		if reported.HoldsOnEntry {
			t.Error("the entry reports an entry hold, and gate_items out holds only on the way out")
		}
		return
	}
	t.Errorf("no entry names the column whose gate was edited: %+v", answer.Columns)
}

// TestColumnChangeCarriesFiveMembersAndNoOccupancy asserts the shape clause of
// dinah-515 criterion 20. A later pass filling an occupancy member turns this
// red, which is the point: counting the cards a column holds is a full read of
// every card anchor, and that read is the cost the separate column digest term
// exists to avoid.
func TestColumnChangeCarriesFiveMembersAndNoOccupancy(t *testing.T) {
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, "changes.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse changes.go: %v", err)
	}
	var members []string
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "ColumnChange" {
			return true
		}
		structure, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structure.Fields.List {
			for _, name := range field.Names {
				members = append(members, name.Name)
			}
		}
		return false
	})
	want := []string{"ID", "Slug", "Title", "HoldsOnEntry", "HoldsOnExit"}
	if len(members) != len(want) {
		t.Fatalf("ColumnChange declares %v, wanted exactly %v", members, want)
	}
	for i, name := range members {
		if name != want[i] {
			t.Errorf("ColumnChange's member %d is %s, wanted %s", i, name, want[i])
		}
	}
	for _, forbidden := range []string{"Count", "Occupancy", "Cards"} {
		for _, name := range members {
			if name == forbidden {
				t.Errorf("ColumnChange carries %s, and a truthful one costs a read of every card anchor", forbidden)
			}
		}
	}
}

// TestTheColumnNoticeCostsNoCardRead asserts the last clause of dinah-515
// criterion 20, through a plant rather than through an instrumented bench:
// on a workbench where Cards() cannot succeed, the walk still can, so a
// Changes call that reports a column change proves it read no card anchor.
//
// Library.Bench is a concrete *bench.Bench, so instrumenting it would mean an
// interface the library does not have or a counter inside bench, and the
// implementation must not be refactored to satisfy a test. The plant is a
// card.md whose front matter does not parse, which cardsWith reports as the
// first load error while the walk only stats the journal and hashes the
// anchor's bytes.
func TestTheColumnNoticeCostsNoCardRead(t *testing.T) {
	l := columnFixture(t)
	answer := l.Add(&Request{Verb: "add", Actor: "alka", Title: "A card the plant will damage"})
	if answer.Outcome != contract.OutcomeOK {
		t.Fatalf("add: %s %s", answer.Outcome, answer.Refusal)
	}
	l = reopenFixture(t, l)
	minted, err := l.Changes(&Request{Verb: "changes"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	damaged := filepath.Join(l.Bench.CardsRoot(), answer.Card.ID, bench.CardAnchor)
	if err := os.WriteFile(damaged, []byte("this file carries no anchor at all\n"), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}
	column := l.Bench.Columns[0]
	anchor := l.Bench.ColumnAnchorPath(column.ID)
	raw, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	edited := strings.Replace(string(raw), "title: "+column.Title, "title: "+column.Title+"RENAMED", 1)
	if err := os.WriteFile(anchor, []byte(edited), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	l = reopenFixture(t, l)

	// The plant is live rather than assumed: reading the cards refuses.
	if _, err := l.Bench.Cards(); err == nil {
		t.Fatal("the plant did not take: Cards() still succeeds, so the half below proves nothing")
	}

	set, err := l.Changes(&Request{Verb: "changes", Since: minted.Cursor})
	if err != nil {
		t.Fatalf("the checkpoint refused on a workbench whose cards cannot be read: %v", err)
	}
	if len(set.Columns) == 0 {
		t.Error("the checkpoint reported no column change on a workbench whose cards cannot be read")
	}
}

// TestTheCursorVersionIsThreeAndRefusesAnOlderToken asserts dinah-515
// criterion 21: the constant is 3, a token minted at version 2 is refused as
// malformed, and a call carrying no token mints a fresh one and reports
// nothing.
func TestTheCursorVersionIsThreeAndRefusesAnOlderToken(t *testing.T) {
	if cursorVersion != 3 {
		t.Errorf("cursorVersion is %d, and this card takes it to 3", cursorVersion)
	}
	l := columnFixture(t)

	minted, err := l.Changes(&Request{Verb: "changes"})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if minted.Cursor == "" {
		t.Error("a call carrying no token minted nothing")
	}
	if minted.Changed {
		t.Error("a call carrying no token reported a change")
	}
	if len(minted.Events) != 0 || len(minted.Cards) != 0 || len(minted.Columns) != 0 {
		t.Errorf("a minting call reported %d events, %d cards and %d columns, and it reports nothing",
			len(minted.Events), len(minted.Cards), len(minted.Columns))
	}

	// A token of the retired shape, spelled exactly as version 2 spelled one.
	old := map[string]any{
		"v":         2,
		"workbench": l.Bench.Slug,
		"live":      "sha256:0",
		"archive":   "sha256:0",
	}
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if _, err := decodeCursor(token); err == nil {
		t.Fatal("a token minted at version 2 was accepted")
	} else if name := refusalNameOfChange(err); name != contract.Malformed {
		t.Errorf("a version 2 token was refused %s, wanted %s", name, contract.Malformed)
	}

	// The accepting case beside the refusing one: a token this build minted
	// is read back without complaint, so the guard is not refusing everything.
	if _, err := decodeCursor(minted.Cursor); err != nil {
		t.Errorf("a token this build minted was refused: %v", err)
	}
}

// columnFixture builds a throwaway workbench and answers a library over it.
func columnFixture(t *testing.T) *Library {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "wb")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	written, err := Init(root, "fx", "alka", "", "", "")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	opened, err := bench.Open(written)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return New(opened, "")
}

// reopenFixture rereads the workbench a fixture wrote to, which a caller does
// after editing an anchor by hand.
func reopenFixture(t *testing.T, l *Library) *Library {
	t.Helper()
	opened, err := bench.Open(l.Bench.Root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	return New(opened, "")
}

// mustDecodeCursor reads a token back, failing the test rather than answering
// a cursor nothing read.
func mustDecodeCursor(t *testing.T, token string) cursor {
	t.Helper()
	read, err := decodeCursor(token)
	if err != nil {
		t.Fatalf("decode the cursor: %v", err)
	}
	return read
}

// refusalNameOfChange answers the name a refusal carries.
func refusalNameOfChange(err error) string {
	var refusal *contract.Refusal
	if errors.As(err, &refusal) {
		return refusal.Name
	}
	return ""
}

// TestTheRootScopedWalkCarriesTheColumnsMember asserts the third caller's half
// of dinah-515 criterion 21: ChangesForest carries the new columns member for
// a workbench whose column anchor moved by hand, and carries none for one
// nobody touched.
func TestTheRootScopedWalkCarriesTheColumnsMember(t *testing.T) {
	l := columnFixture(t)
	// The walk starts above the container directory, which is where a caller
	// standing in a repository would start it.
	root := filepath.Dir(filepath.Dir(filepath.Dir(l.Bench.Root)))
	home := filepath.Join(root, "home")

	minted, err := ChangesForest(root, home, &Request{Verb: "changes"}, 4)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if minted.Cursor == "" {
		t.Fatal("the forest walk minted no cursor")
	}
	quiet, err := ChangesForest(root, home, &Request{Verb: "changes", Since: minted.Cursor}, 4)
	if err != nil {
		t.Fatalf("quiet: %v", err)
	}
	if len(quiet.Workbenches) == 0 {
		t.Fatalf("the forest walk beneath %s found no workbench", root)
	}
	for _, member := range quiet.Workbenches {
		if member.Changes == nil {
			continue
		}
		if len(member.Changes.Columns) != 0 {
			t.Errorf("a workbench nobody touched reported %d columns", len(member.Changes.Columns))
		}
	}

	column := l.Bench.Columns[0]
	anchor := l.Bench.ColumnAnchorPath(column.ID)
	raw, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	edited := strings.Replace(string(raw), "title: "+column.Title, "title: "+column.Title+"RENAMED", 1)
	if edited == string(raw) {
		t.Fatalf("the column anchor carries no title line to edit:\n%s", raw)
	}
	if err := os.WriteFile(anchor, []byte(edited), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	moved, err := ChangesForest(root, home, &Request{Verb: "changes", Since: minted.Cursor}, 4)
	if err != nil {
		t.Fatalf("moved: %v", err)
	}
	named := 0
	for _, member := range moved.Workbenches {
		if member.Changes == nil {
			t.Logf("row %s carries no changes (new=%v unanswered=%q)", member.Candidate.Path, member.New, member.Unanswered)
			continue
		}
		for _, reported := range member.Changes.Columns {
			if reported.ID == column.ID && reported.Title == column.Title+"RENAMED" {
				named++
			}
		}
		if len(member.Changes.Columns) != 0 && len(member.Changes.Cards) != 0 {
			t.Errorf("a column edit resynced %d cards", len(member.Changes.Cards))
		}
	}
	if named != 1 {
		t.Errorf("the forest walk named the edited column %d times, wanted once", named)
	}
}
