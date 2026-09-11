package bench

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTheVocabularyMigrationRefusesADirectoryItCannotList asserts that a plain
// file standing where the pre-vocabulary flow collection belongs is reported
// rather than renamed to columns with the migration answering success.
//
// The fixture builds its path from PreVocabularyDir rather than from a
// literal, so a rename of that constant is a compile error here instead of a
// silent pass. Round one of this card's spec planted a file called flow, which
// the code spells states, so the migration skipped it, renamed nothing, and
// every assertion passed against unfixed code.
//
// The assertion that catches the defect is the state of the disk rather than
// the returned error: a file still standing at PreVocabularyDir with nothing
// created at ColumnsDir.
func TestTheVocabularyMigrationRefusesADirectoryItCannotList(t *testing.T) {
	t.Run("a plain file where the flow collection belongs", func(t *testing.T) {
		root := t.TempDir()
		write(t, filepath.Join(root, WorkbenchAnchor), benchDefinition)
		planted := plantUnreadable(t, filepath.Join(root, PreVocabularyDir))

		err := migrateColumnDirectories(&Bench{Root: root})

		info, statErr := os.Stat(planted)
		if statErr != nil {
			t.Fatalf("a plain file named %s no longer stands at %s: %v", PreVocabularyDir, planted, statErr)
		}
		if info.IsDir() {
			t.Fatalf("a plain file named %s was replaced by a directory", PreVocabularyDir)
		}
		if Exists(filepath.Join(root, ColumnsDir)) {
			t.Fatalf("a plain file named %s was renamed to %s", PreVocabularyDir, ColumnsDir)
		}
		if err == nil {
			t.Fatal("the migration reported success over a directory it could not list")
		}
	})

	t.Run("a genuine pre-vocabulary tree still migrates", func(t *testing.T) {
		root := t.TempDir()
		write(t, filepath.Join(root, WorkbenchAnchor), benchDefinition)
		members := []string{"b00000000001", "b00000000002"}
		for _, id := range members {
			write(t, filepath.Join(root, PreVocabularyDir, id, PreVocabularyAnchor), columnDefinition)
		}

		if err := migrateColumnDirectories(&Bench{Root: root}); err != nil {
			t.Fatalf("the migration refused a genuine pre-vocabulary tree: %v", err)
		}

		if Exists(filepath.Join(root, PreVocabularyDir)) {
			t.Errorf("%s still stands after the migration", PreVocabularyDir)
		}
		checked := 0
		for _, id := range members {
			if !Exists(filepath.Join(root, ColumnsDir, id, ColumnAnchor)) {
				t.Errorf("%s carries no %s after the migration", id, ColumnAnchor)
				continue
			}
			if Exists(filepath.Join(root, ColumnsDir, id, PreVocabularyAnchor)) {
				t.Errorf("%s still carries %s after the migration", id, PreVocabularyAnchor)
			}
			checked++
		}
		t.Logf("%d migrated members checked", checked)
		if checked != len(members) {
			t.Fatalf("the sweep checked %d members and the fixture writes %d", checked, len(members))
		}
	})
}
