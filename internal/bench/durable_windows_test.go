//go:build windows

package bench

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// foreignHandle opens path for reading the way a process Dinah does not
// control might, sharing only what share names, and closes it when the test
// ends unless the test closed it first. The answer closes it at most once.
func foreignHandle(t *testing.T, path string, share uint32) func() {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("convert %s: %v", path, err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, share, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("open %s as a foreign reader: %v", path, err)
	}
	var once sync.Once
	closeIt := func() { once.Do(func() { windows.CloseHandle(handle) }) }
	t.Cleanup(closeIt)
	return closeIt
}

// shortBudget sets durable.RetryBudget for the rest of the test.
func shortBudget(t *testing.T, budget time.Duration) {
	t.Helper()
	saved := durable.RetryBudget
	durable.RetryBudget = budget
	t.Cleanup(func() { durable.RetryBudget = saved })
}

// noticesTo records every wait notice for the rest of the test, calling also
// with each one as it arrives when also is not nil.
func noticesTo(t *testing.T, also func(durable.Wait)) *[]durable.Wait {
	t.Helper()
	var mu sync.Mutex
	notices := &[]durable.Wait{}
	durable.Waiting = func(wait durable.Wait) {
		mu.Lock()
		*notices = append(*notices, wait)
		mu.Unlock()
		if also != nil {
			also(wait)
		}
	}
	t.Cleanup(func() { durable.Waiting = nil })
	return notices
}

// TestARenameIntoPlaceSurvivesAForeignReader asserts that a write holding no
// lock lands over a file another process has open sharing read and write but
// not delete: the first rename is refused, the handle closes, the retry lands
// the new bytes, and no temporary is left.
func TestARenameIntoPlaceSurvivesAForeignReader(t *testing.T) {
	root := newFixture(t)
	path := filepath.Join(fixtureCardDir(root), CardAnchor)
	closeReader := foreignHandle(t, path, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
	replaces := 0
	durable.Observe = func(step durable.Step) error {
		if step.Op == "replace" {
			replaces++
			if replaces == 2 {
				closeReader()
			}
		}
		return nil
	}
	t.Cleanup(func() { durable.Observe = nil })
	if err := WriteText(path, "new bytes\n"); err != nil {
		t.Fatalf("a write over a foreign reader answered %v", err)
	}
	if replaces < 2 {
		t.Errorf("the rename was attempted %d times, so the reader never refused it", replaces)
	}
	if text, _ := ReadText(path); text != "new bytes\n" {
		t.Errorf("the file holds %q after the write", text)
	}
	if left := temporariesIn(t, filepath.Dir(path)); len(left) != 0 {
		t.Errorf("temporaries stand: %v", left)
	}
}

// TestARenameThatNeverLandsGivesUpWithinTheBudget asserts that with the
// budget at 100 ms and the foreign reader held past it, a write before any
// act has written answers *durable.BusyError at about the budget, leaves the
// destination and the directory as they were, and calls durable.Waiting never;
// and that a destination carrying FILE_ATTRIBUTE_READONLY answers its error at
// once.
func TestARenameThatNeverLandsGivesUpWithinTheBudget(t *testing.T) {
	root := newFixture(t)
	path := filepath.Join(fixtureCardDir(root), CardAnchor)
	before, _ := ReadText(path)
	shortBudget(t, 100*time.Millisecond)
	notices := noticesTo(t, nil)
	foreignHandle(t, path, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
	began := time.Now()
	err := WriteText(path, "new bytes\n")
	elapsed := time.Since(began)
	var busy *durable.BusyError
	if !errors.As(err, &busy) {
		t.Fatalf("a write the reader outlasted answered %v, wanted *durable.BusyError", err)
	}
	if elapsed < 100*time.Millisecond || elapsed > time.Second {
		t.Errorf("the write gave up after %v, wanted about 100ms", elapsed)
	}
	if after, _ := ReadText(path); after != before {
		t.Errorf("the destination changed to %q", after)
	}
	if left := temporariesIn(t, filepath.Dir(path)); len(left) != 0 {
		t.Errorf("temporaries stand: %v", left)
	}
	if len(*notices) != 0 {
		t.Errorf("a write before any act wrote called Waiting %d times", len(*notices))
	}

	shortBudget(t, 5*time.Second)
	readOnly := filepath.Join(fixtureCardDir(root), JournalName)
	if err := os.Chmod(readOnly, 0o444); err != nil {
		t.Fatalf("mark the journal read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(readOnly, 0o644) })
	began = time.Now()
	err = WriteText(readOnly, "replaced\n")
	elapsed = time.Since(began)
	if err == nil || errors.As(err, &busy) {
		t.Errorf("a write over a read-only file answered %v, wanted its own error", err)
	}
	if elapsed > time.Second {
		t.Errorf("a write over a read-only file waited %v, wanted an answer at once", elapsed)
	}
}

// TestAReaderAndAnAppenderCoexist asserts that an append to a journal Dinah's
// own reader holds lands without waiting; that with the journal held by a
// handle that does not share write and the budget at 100 ms, an append made
// under the card's lock before the act has written gives up within the
// budget; and that one made after the act's anchor write waits, calls
// Waiting at least twice with rising notice numbers, and lands once the handle
// closes.
func TestAReaderAndAnAppenderCoexist(t *testing.T) {
	root := newFixture(t)
	card := fixtureCardDir(root)
	journal := filepath.Join(card, JournalName)
	ev := Event{TS: "2026-09-28T00:00:00Z", Event: contract.EventCommented, Actor: NamedActor("alka")}

	reader, err := durable.Open(journal)
	if err != nil {
		t.Fatalf("open the journal as Dinah reads it: %v", err)
	}
	notices := noticesTo(t, nil)
	if err := AppendEvent(journal, ev); err != nil {
		t.Errorf("an append beside Dinah's own reader answered %v", err)
	}
	reader.Close()
	if len(*notices) != 0 {
		t.Errorf("an append beside Dinah's own reader waited %d times", len(*notices))
	}

	shortBudget(t, 100*time.Millisecond)
	lock, err := Acquire(card, "alka", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer lock.Release()
	closeReader := foreignHandle(t, journal, windows.FILE_SHARE_READ)
	began := time.Now()
	err = AppendEvent(journal, ev)
	elapsed := time.Since(began)
	var busy *durable.BusyError
	if !errors.As(err, &busy) || elapsed > time.Second {
		t.Fatalf("an append before the act wrote answered %v after %v, wanted *durable.BusyError within the budget", err, elapsed)
	}

	notices = noticesTo(t, func(wait durable.Wait) {
		if wait.Notice == 2 {
			closeReader()
		}
	})
	if err := WriteText(filepath.Join(card, CardAnchor), cleanCard); err != nil {
		t.Fatalf("the act's anchor write: %v", err)
	}
	if err := AppendEvent(journal, ev); err != nil {
		t.Fatalf("an append after the act's anchor write answered %v", err)
	}
	if len(*notices) < 2 {
		t.Fatalf("the waiting append called Waiting %d times, wanted at least two", len(*notices))
	}
	for i, notice := range *notices {
		if notice.Notice != i+1 || notice.Path != journal {
			t.Errorf("notice %d was %+v, wanted number %d naming the journal", i, notice, i+1)
		}
	}
}
