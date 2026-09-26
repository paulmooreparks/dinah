package bench

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// holdShared opens a file the way dinah serve's resident reads one, sharing
// read, write and delete, and answers a function that closes it.
func holdShared(t *testing.T, path string) func() {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	close := func() {
		if !closed {
			closed = true
			syscall.CloseHandle(handle)
		}
	}
	t.Cleanup(close)
	return close
}

// entityWithAFile makes an entity directory holding one file and answers both
// paths.
func entityWithAFile(t *testing.T) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cards", "0123456789ab")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "journal.jsonl")
	if err := os.WriteFile(file, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, file
}

// TestAFolderMoveWaitsOutAReaderBelowIt is the act side of the blocker in
// dinah-619/comments/12. A reader holds a file below an entity's directory
// open, as the resident does for the length of one read, and the directory's
// move first meets the refusal an unretried rename answers, then succeeds
// once the reader lets go, well inside the budget.
func TestAFolderMoveWaitsOutAReaderBelowIt(t *testing.T) {
	dir, file := entityWithAFile(t)
	release := holdShared(t, file)
	target := filepath.Join(filepath.Dir(filepath.Dir(dir)), ArchiveDir, "cards", "0123456789ab")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, target); err == nil || !transientRenameRefusal(err) {
		t.Fatalf("a plain rename with a reader below the directory answered %v, wanted the transient refusal the retry exists for, so this test is not in the position it means to test", err)
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		release()
	}()
	started := time.Now()
	if err := MoveEntity(dir, target); err != nil {
		t.Fatalf("the move with a reader below the directory for 150ms answered %v, wanted it to wait the reader out", err)
	}
	if elapsed := time.Since(started); elapsed < 100*time.Millisecond || elapsed > folderRetryBudget {
		t.Errorf("the move took %v, wanted it to wait for the reader (about 150ms) and no longer than the budget", elapsed)
	}
	if _, err := os.Stat(filepath.Join(target, "journal.jsonl")); err != nil {
		t.Errorf("the moved directory does not hold its file: %v", err)
	}
}

// TestAFolderRemovalWaitsOutAReaderBelowIt is the same for a delete. The
// reader here opens the file as os.Open does, sharing read and write but not
// delete, which is how an editor, an indexer or any other ordinary reader
// holds it, so deleting the file is refused while the handle is open. (A
// reader that shares delete, as the resident's does, may leave the file only
// marked for deletion, and the directory's removal then answers
// ERROR_DIR_NOT_EMPTY, which removeFolder retries too.)
func TestAFolderRemovalWaitsOutAReaderBelowIt(t *testing.T) {
	dir, file := entityWithAFile(t)
	held, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { held.Close() })
	if err := os.Remove(file); err == nil || !transientRemoveRefusal(err) {
		t.Fatalf("a plain delete of the held file answered %v, wanted the transient refusal the retry exists for, so this test is not in the position it means to test", err)
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		held.Close()
	}()
	started := time.Now()
	if err := DeleteEntity(dir); err != nil {
		t.Fatalf("the delete with a reader below the directory for 150ms answered %v, wanted it to wait the reader out", err)
	}
	if elapsed := time.Since(started); elapsed < 100*time.Millisecond || elapsed > folderRetryBudget {
		t.Errorf("the delete took %v, wanted it to wait for the reader (about 150ms) and no longer than the budget", elapsed)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the directory is still there after the delete: %v", err)
	}
}

// TestAFolderHeldPastTheBudgetFailsAsBefore holds the reader past a shortened
// budget. The move answers the refusal itself, unchanged, which the act turns
// into the interruption it has always reported, and it gave up once the budget
// was spent rather than at once or never.
func TestAFolderHeldPastTheBudgetFailsAsBefore(t *testing.T) {
	previous := folderRetryBudget
	folderRetryBudget = 200 * time.Millisecond
	t.Cleanup(func() { folderRetryBudget = previous })
	dir, file := entityWithAFile(t)
	holdShared(t, file)
	target := filepath.Join(filepath.Dir(dir), "moved")
	started := time.Now()
	err := MoveEntity(dir, target)
	elapsed := time.Since(started)
	var link *os.LinkError
	if err == nil || !errors.As(err, &link) || !transientRenameRefusal(err) {
		t.Fatalf("a move refused past the budget answered %v, wanted the rename's own refusal", err)
	}
	if elapsed < folderRetryBudget || elapsed > folderRetryBudget+time.Second {
		t.Errorf("the move gave up after %v, wanted it to spend the %v budget and stop", elapsed, folderRetryBudget)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("the refused move disturbed the directory: %v", err)
	}
}
