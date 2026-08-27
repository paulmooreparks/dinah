package bench

import (
	"errors"
	"path/filepath"
	"testing"

	"dinah/internal/contract"
)

// preVocabularyFixture writes a workbench in the vocabulary this build retired:
// a states sequence on its own anchor, a states directory, and a state.md in
// each member. The sequence is supplied, so a caller can write a duplicate
// identifier or one with no directory behind it.
func preVocabularyFixture(t *testing.T, title string, sequence []string, present []string) string {
	t.Helper()
	root := t.TempDir()
	fm := NewFrontmatter()
	fm.Set("format", "1")
	fm.Set("profile", "dinah-core/0.6")
	if title != "" {
		fm.Set("title", title)
	}
	fm.Set("slug", "fx")
	fm.Set("operator", "alka")
	fm.SetSeq(preVocabularySequenceKey, sequence)
	write(t, filepath.Join(root, WorkbenchAnchor), fm.Render("Standing text.\n"))
	for _, id := range present {
		write(t, filepath.Join(root, PreVocabularyDir, id, PreVocabularyAnchor), columnAnchor("Column "+id, "", "work"))
	}
	return root
}

// currentFixture writes the same workbench in the vocabulary this build reads,
// so each refusal can be compared against the one Open gives for the same
// defect rather than merely asserted on its own.
func currentFixture(t *testing.T, title string, sequence []string, present []string) string {
	t.Helper()
	root := t.TempDir()
	fm := NewFrontmatter()
	fm.Set("format", "1")
	fm.Set("profile", ProfileVersion)
	if title != "" {
		fm.Set("title", title)
	}
	fm.Set("slug", "fx")
	fm.Set("operator", "alka")
	fm.SetSeq("columns", sequence)
	write(t, filepath.Join(root, WorkbenchAnchor), fm.Render("Standing text.\n"))
	for _, id := range present {
		write(t, filepath.Join(root, ColumnsDir, id, ColumnAnchor), columnAnchor("Column "+id, "", "work"))
	}
	return root
}

// refusalOf answers the refusal an error carries, or nil when it carries none.
func refusalOf(err error) *contract.Refusal {
	refusal := &contract.Refusal{}
	if errors.As(err, &refusal) {
		return refusal
	}
	return nil
}

// TestTheLenientOpenerRefusesWhatTheOrdinaryOneRefuses asserts dinah-287 AC-18:
// the opener the vocabulary migration reads through is lenient about the
// vocabulary and about nothing else. Each defect below is written twice, once
// in each vocabulary, and the two refusals are compared rather than each being
// asserted against a name written here, so a later change to either opener's
// answer fails this test rather than drifting past it.
func TestTheLenientOpenerRefusesWhatTheOrdinaryOneRefuses(t *testing.T) {
	t.Run("no title", func(t *testing.T) {
		_, ordinary := Open(currentFixture(t, "", []string{"b00000000001"}, []string{"b00000000001"}))
		_, lenient := OpenPreVocabulary(preVocabularyFixture(t, "", []string{"b00000000001"}, []string{"b00000000001"}))
		compareRefusals(t, ordinary, lenient, "title")
	})

	t.Run("a duplicate identifier in the sequence", func(t *testing.T) {
		ids := []string{"b00000000001", "b00000000001"}
		_, ordinary := Open(currentFixture(t, "Fixture", ids, []string{"b00000000001"}))
		_, lenient := OpenPreVocabulary(preVocabularyFixture(t, "Fixture", ids, []string{"b00000000001"}))
		if refusalOf(ordinary) == nil || refusalOf(lenient) == nil {
			t.Fatalf("a duplicate identifier was admitted: ordinary %v, lenient %v", ordinary, lenient)
		}
		if refusalOf(ordinary).Name != refusalOf(lenient).Name {
			t.Errorf("the ordinary opener refuses a duplicate with %s and the lenient one with %s",
				refusalOf(ordinary).Name, refusalOf(lenient).Name)
		}
	})

	t.Run("an identifier with no directory behind it is collected", func(t *testing.T) {
		ids := []string{"b00000000001", "b00000000002"}
		opened, err := OpenPreVocabulary(preVocabularyFixture(t, "Fixture", ids, []string{"b00000000001"}))
		if err != nil {
			t.Fatalf("a stranded identifier ended the open: %v", err)
		}
		if len(opened.Columns) != 1 {
			t.Errorf("the workbench opened with %d columns, wanted the one that is there", len(opened.Columns))
		}
		if len(opened.StrandedColumns) != 1 || opened.StrandedColumns[0] != "b00000000002" {
			t.Errorf("the stranded identifiers are %v, wanted the one with no directory", opened.StrandedColumns)
		}
	})
}

// compareRefusals asserts that two openers refused the same defect with the
// same refusal name and the same detail.
func compareRefusals(t *testing.T, ordinary, lenient error, detail string) {
	t.Helper()
	first, second := refusalOf(ordinary), refusalOf(lenient)
	if first == nil || second == nil {
		t.Fatalf("the defect was admitted by an opener: ordinary %v, lenient %v", ordinary, lenient)
	}
	if first.Name != second.Name || first.Detail != second.Detail {
		t.Errorf("the ordinary opener refuses %s/%s and the lenient one %s/%s",
			first.Name, first.Detail, second.Name, second.Detail)
	}
	if first.Detail != detail {
		t.Errorf("the refusal names %q, wanted %q", first.Detail, detail)
	}
}

// TestTheLenientOpenerWantsThePreVocabularyWindow asserts the other half of
// the mirror: the opener the migration reads through refuses a workbench this
// build reads ordinarily, so a second run over an already-migrated workbench
// finds nothing to do and says so rather than rewriting it.
func TestTheLenientOpenerWantsThePreVocabularyWindow(t *testing.T) {
	_, err := OpenPreVocabulary(currentFixture(t, "Fixture", []string{"b00000000001"}, []string{"b00000000001"}))
	refusal := refusalOf(err)
	if refusal == nil {
		t.Fatalf("the lenient opener admitted a workbench already carrying the current vocabulary: %v", err)
	}
	if refusal.Name != contract.NeedsVocabularyMigration {
		t.Errorf("an already-migrated workbench was refused %s, wanted %s", refusal.Name, contract.NeedsVocabularyMigration)
	}
}
