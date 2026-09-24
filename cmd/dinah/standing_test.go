package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// doingStandingItems is the declaration the command-level cases plant on the
// Doing column of a fresh workbench: one criterion demanding a scheme and one
// question, which between them reach every surface this file reads.
const doingStandingItems = `standing_items:
  deposit-paid:
    kind: acceptance_criterion
    text: The vendor's deposit has been paid and the receipt is on the card.
    evidence: receipt
  date-confirmed:
    kind: open_question
    text: Has the vendor confirmed the wedding date in writing?
`

// declareStandingItems writes a standing_items block into one column's own
// anchor, found by its title, the way declareGateItems writes the hold: no
// command writes the block, and a person editing column.md is how it arrives.
func declareStandingItems(t *testing.T, root, title, block string) {
	t.Helper()
	columns := filepath.Join(soleBenchDir(t, root), bench.ColumnsDir)
	ids, err := bench.ListIDs(columns)
	if err != nil {
		t.Fatalf("listing %s: %v", columns, err)
	}
	for _, id := range ids {
		path := filepath.Join(columns, id, bench.ColumnAnchor)
		text, err := bench.ReadText(path)
		if err != nil {
			t.Fatalf("read a column: %v", err)
		}
		fm, body := bench.ParseAnchor(text)
		if fm.Value("title") != title {
			continue
		}
		fm.SetRaw(bench.StandingItemsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
		if err := os.WriteFile(path, []byte(fm.Render(body)), 0o644); err != nil {
			t.Fatalf("write a column: %v", err)
		}
		return
	}
	t.Fatalf("the fixture flow carries no column titled %s", title)
}

// TestTheJournalDetailNamesTheDeclarationThatFiledAnItem drives the human
// form of criterion 3: an item_filed line a declaration wrote draws the column
// title and the entry key in its detail column, a hand filing's line draws the
// empty detail it always drew, and the machine form carries the three members
// as written.
func TestTheJournalDetailNamesTheDeclarationThatFiledAnItem(t *testing.T) {
	root := newBench(t)
	declareStandingItems(t, root, "Doing", doingStandingItems)
	card := addCard(t, root, "a card")
	carryToDoing(t, root, card)
	if got := runCLI(t, root, "file", card, "decision", "A hand-filed decision."); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}

	log := runCLI(t, root, "list", card+"/journal")
	if log.code != 0 {
		t.Fatalf("journal: %d %s", log.code, log.errw)
	}
	catalog := msg.For(msg.Base)
	filed := 0
	for _, line := range strings.Split(log.out, "\n") {
		if !strings.Contains(line, "item_filed") {
			continue
		}
		filed++
		switch {
		case strings.Contains(line, "deposit-paid"):
			if want := catalog.T("log.item-filed.standing", "column", "Doing", "key", "deposit-paid"); !strings.Contains(line, want) {
				t.Errorf("the minted criterion's row does not carry %q: %q", want, line)
			}
		case strings.Contains(line, "date-confirmed"):
			if !strings.Contains(line, "Doing: date-confirmed") {
				t.Errorf("the minted question's row does not name the column and the key: %q", line)
			}
		default:
			if strings.Contains(line, "Doing") {
				t.Errorf("the hand filing's row draws a detail: %q", line)
			}
		}
	}
	if filed != 3 {
		t.Fatalf("the journal draws %d item_filed rows, wanted 3:\n%s", filed, log.out)
	}

	machine := runCLI(t, root, "--json", "list", card+"/journal")
	if machine.code != 0 {
		t.Fatalf("journal --json: %d %s", machine.code, machine.errw)
	}
	var events []bench.Event
	if err := json.Unmarshal([]byte(machine.out), &events); err != nil {
		t.Fatalf("decode the journal: %v\n%s", err, machine.out)
	}
	minted, hand := 0, 0
	for _, ev := range events {
		if ev.Event != contract.EventItemFiled {
			continue
		}
		if ev.Standing == "" {
			hand++
			if ev.Column != "" || ev.ColumnTitle != "" {
				t.Errorf("the hand filing's line carries a column: %+v", ev)
			}
			continue
		}
		minted++
		if ev.Column != columnIdentifier(t, root, "doing") || ev.ColumnTitle != "Doing" {
			t.Errorf("a minted line names %s %q, wanted the Doing column", ev.Column, ev.ColumnTitle)
		}
	}
	if minted != 2 || hand != 1 {
		t.Errorf("the machine journal carries %d minted and %d hand filings, wanted 2 and 1", minted, hand)
	}

	// A line carrying a standing key and no column title, which a minting
	// never writes but an older or a foreign build might, draws the stored
	// identifier in the title's place rather than nothing.
	doingID := columnIdentifier(t, root, "doing")
	journal := filepath.Join(soleBenchDir(t, root), bench.CardsDir, cardID(t, root, card), bench.JournalName)
	line := `{"ts":"2026-09-24T09:14:02Z","event":"item_filed","actor":{"name":"alka"},"item":"deadbeefcafe","kind":"decision","column":"` + doingID + `","standing":"handoff-posted"}` + "\n"
	handle, err := os.OpenFile(journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open the journal: %v", err)
	}
	if _, err := handle.WriteString(line); err != nil {
		t.Fatalf("append the line: %v", err)
	}
	handle.Close()
	again := runCLI(t, root, "list", card+"/journal")
	if again.code != 0 || !strings.Contains(again.out, doingID+": handoff-posted") {
		t.Errorf("a filing line with no column title does not fall back to the identifier: %d\n%s", again.code, again.out)
	}
}

// TestShowPrintsAnInstancesStandingKeyAndEvidence drives the reading surfaces
// of criterion 6 at a terminal: show of the item prints its standing and
// evidence keys, get reads the scheme back, set writes and clears it, and
// standing is refused as a field either way.
func TestShowPrintsAnInstancesStandingKeyAndEvidence(t *testing.T) {
	root := newBench(t)
	declareStandingItems(t, root, "Doing", doingStandingItems)
	card := addCard(t, root, "a card")
	carryToDoing(t, root, card)

	shown := runCLI(t, root, "show", card+"/criteria/1")
	if shown.code != 0 {
		t.Fatalf("show the item: %d %s", shown.code, shown.errw)
	}
	for _, want := range []string{"standing: deposit-paid", "evidence: receipt"} {
		if !strings.Contains(shown.out, want) {
			t.Errorf("show does not print %q:\n%s", want, shown.out)
		}
	}
	if got := runCLI(t, root, "get", card+"/criteria/1", "evidence"); got.code != 0 || strings.TrimSpace(got.out) != "receipt" {
		t.Errorf("get evidence: %d %q %s", got.code, got.out, got.errw)
	}
	if got := runCLI(t, root, "get", card+"/questions/1", "evidence"); got.code != 0 || strings.TrimSpace(got.out) != "" {
		t.Errorf("get evidence on an item demanding none: %d %q %s", got.code, got.out, got.errw)
	}
	if got := runCLI(t, root, "set", card+"/questions/1", "evidence", "record"); got.code != 0 {
		t.Errorf("set evidence: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "get", card+"/questions/1", "evidence"); strings.TrimSpace(got.out) != "record" {
		t.Errorf("after the write get evidence answered %q", got.out)
	}
	if got := runCLI(t, root, "set", card+"/questions/1", "evidence"); got.code != 0 {
		t.Errorf("clear evidence: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "get", card+"/questions/1", "evidence"); strings.TrimSpace(got.out) != "" {
		t.Errorf("after the clear get evidence answered %q", got.out)
	}
	for _, argv := range [][]string{{"get", card + "/criteria/1", "standing"}, {"set", card + "/criteria/1", "standing", "other"}} {
		got := runCLI(t, root, argv...)
		if got.code == 0 || refusalNameOf(got.errw) != contract.UnknownField {
			t.Errorf("%s: wanted %s, got %d %q", strings.Join(argv, " "), contract.UnknownField, got.code, got.errw)
		}
	}

	// The column's own declaration is listed on the column view, in
	// declaration order, and the column's show prints the block itself.
	listed := runCLI(t, root, "--json", "list", "columns")
	if listed.code != 0 {
		t.Fatalf("list columns: %d %s", listed.code, listed.errw)
	}
	var columns []verb.ColumnView
	if err := json.Unmarshal([]byte(listed.out), &columns); err != nil {
		t.Fatalf("decode the columns: %v", err)
	}
	for _, column := range columns {
		if column.Slug != "doing" {
			if len(column.StandingItems) != 0 {
				t.Errorf("%s carries standing items it does not declare: %+v", column.Slug, column.StandingItems)
			}
			continue
		}
		if len(column.StandingItems) != 2 || column.StandingItems[0].Key != "deposit-paid" || column.StandingItems[0].Evidence != "receipt" || column.StandingItems[1].Kind != "open_question" {
			t.Errorf("the doing column's view carries %+v", column.StandingItems)
		}
	}
	columnShown := runCLI(t, root, "show", "doing")
	if columnShown.code != 0 || !strings.Contains(columnShown.out, "deposit-paid") || !strings.Contains(columnShown.out, "evidence: receipt") {
		t.Errorf("show of the column does not print the declaration: %d\n%s", columnShown.code, columnShown.out)
	}
}

// TestTheRepairFlagPreviewsThenFilesAtATerminal drives criterion 8 at the
// command: the preview prints its marker and the instances it would file and
// changes no file, the confirmed run files them and reports each, a check
// afterwards reports nothing under the finding, and a caller who is not the
// operator is refused by name. The help page names the flag.
func TestTheRepairFlagPreviewsThenFilesAtATerminal(t *testing.T) {
	root := newBench(t)
	card := addCard(t, root, "filed before the declaration")
	carryToDoing(t, root, card)
	declareStandingItems(t, root, "Doing", doingStandingItems)
	dir := soleBenchDir(t, root)

	if page := runCLI(t, root, "help", "check"); !strings.Contains(page.out, "--file-standing") {
		t.Errorf("the help page does not name the flag:\n%s", page.out)
	}
	checked := runCLI(t, root, "check")
	if checked.code == 0 || !strings.Contains(checked.out, bench.FindingStandingItemMissing) && !strings.Contains(checked.out, "deposit-paid") {
		t.Fatalf("check reports nothing over a card missing its instances: %d\n%s", checked.code, checked.out)
	}

	t.Setenv("DINAH_ACTOR", "bo")
	refused := runCLI(t, root, "check", "--file-standing", "--yes")
	if refused.code == 0 || refusalNameOf(refused.errw) != contract.NotOperator {
		t.Errorf("a non-operator running the repair: wanted %s, got %d %q", contract.NotOperator, refused.code, refused.errw)
	}
	t.Setenv("DINAH_ACTOR", "alka")

	before := treeDigest(t, dir)
	preview := runCLI(t, root, "check", "--file-standing")
	if after := treeDigest(t, dir); after != before {
		t.Fatal("the preview wrote to the workbench")
	}
	catalog := msg.For(msg.Base)
	for _, want := range []string{
		catalog.T("check.standing-preview"),
		catalog.T("check.standing-would-filing", "card", card, "column", "doing", "key", "deposit-paid"),
		catalog.T("check.standing-would-filing", "card", card, "column", "doing", "key", "date-confirmed"),
	} {
		if !strings.Contains(preview.out, want) {
			t.Errorf("the preview does not say %q:\n%s", want, preview.out)
		}
	}

	applied := runCLI(t, root, "check", "--file-standing", "--yes")
	if applied.code != 0 {
		t.Fatalf("the confirmed run exited %d: %s\n%s", applied.code, applied.errw, applied.out)
	}
	for _, want := range []string{
		catalog.T("check.standing-filing", "card", card, "column", "doing", "key", "deposit-paid"),
		catalog.T("check.standing-filing", "card", card, "column", "doing", "key", "date-confirmed"),
	} {
		if !strings.Contains(applied.out, want) {
			t.Errorf("the confirmed run does not say %q:\n%s", want, applied.out)
		}
	}
	if strings.Contains(applied.out, catalog.T("check.standing-preview")) {
		t.Errorf("the confirmed run prints the preview marker:\n%s", applied.out)
	}
	shown := runCLI(t, root, "show", card, "--fields", "checklist")
	if !strings.Contains(shown.out, "deposit-paid") && !strings.Contains(shown.out, "deposit has been paid") {
		t.Errorf("the repaired card carries no instance:\n%s", shown.out)
	}
	if again := runCLI(t, root, "check"); strings.Contains(again.out, "standing-item-missing") {
		t.Errorf("after the repair check still reports a missing instance:\n%s", again.out)
	}
}

// TestAReshapeReportsTheStandingItemsItWithdraws drives the terminal's view
// of the withdrawal step: the preview and the apply both print the count
// under the retirement, and the withdrawn instance carries the comment naming
// the retired column.
func TestAReshapeReportsTheStandingItemsItWithdraws(t *testing.T) {
	root := newBench(t)
	declareStandingItems(t, root, "Doing", doingStandingItems)
	card := addCard(t, root, "a card")
	carryToDoing(t, root, card)
	var doingID, intakeID string
	source := reshapeSourceFrom(t, root, func(columns []map[string]any) []map[string]any {
		doingID = idOf(t, columns, "Doing")
		intakeID = idOf(t, columns, "Intake")
		return without(columns, "Doing")
	})
	catalog := msg.For(msg.Base)
	want := catalog.T("reshape.withdrawn", "count", "2")
	preview := runCLI(t, root, "reshape", "--from", source, "--map", doingID+"="+intakeID)
	if preview.code != 0 || !strings.Contains(preview.out, want) {
		t.Errorf("the preview does not say %q: %d\n%s", want, preview.code, preview.out)
	}
	applied := runCLI(t, root, "reshape", "--from", source, "--yes", "--map", doingID+"="+intakeID)
	if applied.code != 0 || !strings.Contains(applied.out, want) {
		t.Errorf("the apply does not say %q: %d\n%s", want, applied.code, applied.out)
	}
	shown := runCLI(t, root, "show", card+"/criteria/1")
	if !strings.Contains(shown.out, "state: withdrawn") || !strings.Contains(shown.out, catalog.T("reshape.standing-withdrawn", "column", "Doing")) {
		t.Errorf("the withdrawn instance does not carry the reshape's comment:\n%s", shown.out)
	}
}
