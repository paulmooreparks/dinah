package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// handEditColumn rewrites one card's column in its anchor and leaves the
// journal alone, which is the edit `check --witness` exists to reconcile and
// which no command of the tool performs. It answers with the column the
// journal still believes.
func handEditColumn(t *testing.T, root, card, column string) string {
	t.Helper()
	machine := runCLI(t, root, "--json", "show", card)
	if machine.code != 0 {
		t.Fatalf("show %s: %d %s", card, machine.code, machine.errw)
	}
	var detail struct {
		Card struct {
			ID     string `json:"id"`
			Column string `json:"column"`
		} `json:"card"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(machine.out), &detail); err != nil {
		t.Fatalf("decode the card: %v\n%s", err, machine.out)
	}
	text, err := os.ReadFile(detail.Path)
	if err != nil {
		t.Fatalf("read %s: %v", detail.Path, err)
	}
	was := "column: " + detail.Card.Column
	if !strings.Contains(string(text), was) {
		t.Fatalf("the anchor at %s carries no %q", detail.Path, was)
	}
	edited := strings.Replace(string(text), was, "column: "+column, 1)
	if err := os.WriteFile(detail.Path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write %s: %v", detail.Path, err)
	}
	return detail.Card.ID
}

// columnIdentifier resolves a column's identifier from its slug, since the
// anchor names a column by identifier and a person names one by slug.
func columnIdentifier(t *testing.T, root, slug string) string {
	t.Helper()
	machine := runCLI(t, root, "--json", "columns")
	if machine.code != 0 {
		t.Fatalf("columns: %d %s", machine.code, machine.errw)
	}
	var columns []verb.ColumnView
	if err := json.Unmarshal([]byte(machine.out), &columns); err != nil {
		t.Fatalf("decode the columns: %v\n%s", err, machine.out)
	}
	for _, column := range columns {
		if column.Slug == slug {
			return column.ID
		}
	}
	t.Fatalf("the workbench declares no column with the slug %q", slug)
	return ""
}

// cardJournal reads one card's journal off disk by identifier.
func cardJournal(t *testing.T, root, id string) []bench.Event {
	t.Helper()
	path := filepath.Join(soleBenchDir(t, root), bench.CardsDir, id, bench.JournalName)
	events, torn, err := bench.ReadJournal(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if torn {
		t.Fatalf("%s ends in a partial line", path)
	}
	return events
}

// countWitnesses counts the manual_correction lines of a card's journal.
func countWitnesses(events []bench.Event) int {
	count := 0
	for _, ev := range events {
		if ev.Event == contract.EventManualCorrection {
			count++
		}
	}
	return count
}

// TestCheckWitnessRecordsTheDivergingCardAndLeavesTheOtherAlone drives the
// batch repair through the command surface, which is the only place the flag,
// the request field and the report are wired together. A card nobody edited
// must come through the same run untouched, or the repair is writing lines
// about nothing.
func TestCheckWitnessRecordsTheDivergingCardAndLeavesTheOtherAlone(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "An edited card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "An untouched card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	untouched := cardIdentifier(t, root, "fx-2")
	edited := handEditColumn(t, root, "fx-1", columnIdentifier(t, root, "doing"))

	machine := runCLI(t, root, "--json", "check", "--witness")
	var report verb.CheckReport
	if err := json.Unmarshal([]byte(machine.out), &report); err != nil {
		t.Fatalf("decode the report: %v\n%s", err, machine.out)
	}
	if !report.MigratedWitness {
		t.Error("the report does not say the witness repair ran")
	}
	if len(report.WitnessedCards) != 1 || report.WitnessedCards[0] != edited {
		t.Fatalf("the repair witnessed %v, wanted the edited card %s alone", report.WitnessedCards, edited)
	}
	if got := countWitnesses(cardJournal(t, root, edited)); got != 1 {
		t.Errorf("the edited card's journal carries %d witnesses, wanted one", got)
	}
	if got := countWitnesses(cardJournal(t, root, untouched)); got != 0 {
		t.Errorf("the untouched card's journal carries %d witnesses, wanted none", got)
	}
	after := runCLI(t, root, "check")
	if after.code != 0 {
		t.Errorf("a check straight after the repair still reports something:\n%s", after.out)
	}
}

// cardIdentifier resolves a card's own identifier from the reference a person
// types, which is what a journal path is named by.
func cardIdentifier(t *testing.T, root, ref string) string {
	t.Helper()
	machine := runCLI(t, root, "--json", "show", ref)
	if machine.code != 0 {
		t.Fatalf("show %s: %d %s", ref, machine.code, machine.errw)
	}
	var detail struct {
		Card struct {
			ID string `json:"id"`
		} `json:"card"`
	}
	if err := json.Unmarshal([]byte(machine.out), &detail); err != nil {
		t.Fatalf("decode the card: %v\n%s", err, machine.out)
	}
	return detail.Card.ID
}

// TestTheWitnessKeysAreAbsentFromACheckThatWasNotAskedForOne pins the
// presence convention the sibling repairs already use: the flag is what
// separates an empty answer from a repair nobody asked for, and both keys go
// missing together when nobody asked.
func TestTheWitnessKeysAreAbsentFromACheckThatWasNotAskedForOne(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}

	plain := runCLI(t, root, "--json", "check")
	if plain.code != 0 {
		t.Fatalf("check: %d %s", plain.code, plain.errw)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(plain.out), &keys); err != nil {
		t.Fatalf("decode: %v\n%s", err, plain.out)
	}
	if _, carried := keys["migrated_witness"]; carried {
		t.Error("a check nobody asked to witness carries migrated_witness")
	}
	if _, carried := keys["witnessed_cards"]; carried {
		t.Error("a check nobody asked to witness carries witnessed_cards")
	}

	asked := runCLI(t, root, "--json", "check", "--witness")
	if asked.code != 0 {
		t.Fatalf("check --witness: %d %s", asked.code, asked.errw)
	}
	if err := json.Unmarshal([]byte(asked.out), &keys); err != nil {
		t.Fatalf("decode: %v\n%s", err, asked.out)
	}
	if _, carried := keys["migrated_witness"]; !carried {
		t.Error("a check asked to witness does not say that it ran")
	}
	if _, carried := keys["witnessed_cards"]; carried {
		t.Error("a run that witnessed nothing carries witnessed_cards, which should be absent when the list is empty")
	}
}

// TestTheLogGivesAWitnessedCorrectionADetailOfItsOwn asserts the one event
// this build writes that a person is most likely to want explained does not
// render a blank cell, and that it reads differently from a move, because
// nobody chose the transition a witness records.
func TestTheLogGivesAWitnessedCorrectionADetailOfItsOwn(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "An edited card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")
	handEditColumn(t, root, "fx-1", columnIdentifier(t, root, "intake"))
	if got := runCLI(t, root, "check", "--witness"); got.code != 0 {
		t.Fatalf("check --witness: %d %s", got.code, got.errw)
	}

	log := runCLI(t, root, "log", "fx-1")
	if log.code != 0 {
		t.Fatalf("log: %d %s", log.code, log.errw)
	}
	corrected := ""
	for _, line := range strings.Split(log.out, "\n") {
		if strings.Contains(line, "manual correction") {
			corrected = line
		}
	}
	if corrected == "" {
		t.Fatalf("the log names no manual correction:\n%s", log.out)
	}
	if !strings.Contains(corrected, "Doing") || !strings.Contains(corrected, "Intake") {
		t.Errorf("the correction's detail names neither column it stands between:\n%s", corrected)
	}
	moved := ""
	for _, line := range strings.Split(log.out, "\n") {
		if strings.Contains(line, "moved") {
			moved = line
		}
	}
	if moved == "" {
		t.Fatalf("the log names no move, so this test cannot tell the two details apart:\n%s", log.out)
	}
	if detailOf(corrected) == detailOf(moved) {
		t.Errorf("a witnessed correction renders the same detail as a move:\n%s\n%s", corrected, moved)
	}
}

// detailOf cuts the trailing detail off a rendered journal row, which is
// whatever follows the event token. The rows are drawn in one table, so the
// event name is the last field the two kinds share.
func detailOf(row string) string {
	fields := strings.Fields(row)
	if len(fields) < 3 {
		return ""
	}
	return strings.Join(fields[2:], " ")
}
