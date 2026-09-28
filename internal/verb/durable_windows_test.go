//go:build windows

package verb

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/durable"
)

// foreignHandle opens path for reading the way a process Dinah does not
// control might, sharing only what share names. The answer closes it at most
// once, and the test's cleanup closes it if nothing did.
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

// recordNotices records every wait notice for the rest of the test, and
// calls onSecond once, with the second notice, when it is not nil.
func recordNotices(t *testing.T, onSecond func()) *[]durable.Wait {
	t.Helper()
	var mu sync.Mutex
	notices := &[]durable.Wait{}
	durable.Waiting = func(wait durable.Wait) {
		mu.Lock()
		*notices = append(*notices, wait)
		mu.Unlock()
		if wait.Notice == 2 && onSecond != nil {
			onSecond()
		}
	}
	t.Cleanup(func() { durable.Waiting = nil })
	return notices
}

// assertRisingNotices fails unless at least two notices arrived, numbered one
// upwards and each naming path.
func assertRisingNotices(t *testing.T, notices []durable.Wait, path string) {
	t.Helper()
	if len(notices) < 2 {
		t.Fatalf("Waiting was called %d times, wanted at least two", len(notices))
	}
	for i, notice := range notices {
		if notice.Notice != i+1 || notice.Path != path {
			t.Errorf("notice %d was %+v, wanted number %d naming %s", i, notice, i+1, path)
		}
	}
}

// findingsAbout answers the findings naming a card by its identifier or by a
// path inside its directory.
func findingsAbout(findings []bench.Finding, card *bench.Card) []bench.Finding {
	var about []bench.Finding
	for _, found := range findings {
		if found.Detail == card.ID || strings.HasPrefix(found.Path, card.Dir) {
			about = append(about, found)
		}
	}
	return about
}

// TestAnArchiveSurvivesAReaderInsideTheCard asserts that archiving a card
// while a process Dinah does not control reads its card.md, sharing read and
// write but not delete, answers ok once the reader closes, carries the whole
// directory into the archive, leaves no sibling, and leaves check with nothing
// to say; and that with the budget at 100 ms and the reader outlasting it, the
// act answers dinah.interrupted with the sibling standing, which check
// --finish then completes.
func TestAnArchiveSurvivesAReaderInsideTheCard(t *testing.T) {
	h := newHarness(t)
	ref := h.add("archived while read")
	card := h.card(ref)
	closeReader := foreignHandle(t, filepath.Join(card.Dir, bench.CardAnchor), windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
	moves := 0
	durable.Observe = func(step durable.Step) error {
		if step.Op == "move" {
			moves++
			if moves == 2 {
				closeReader()
			}
		}
		return nil
	}
	t.Cleanup(func() { durable.Observe = nil })
	response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: ref})
	durable.Observe = nil
	h.reopen()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("the archive answered %s %s", response.Outcome, response.Refusal)
	}
	if moves < 2 {
		t.Errorf("the directory move was attempted %d times, so the reader never refused it", moves)
	}
	archived := h.archivedDir(card.ID)
	if !bench.Exists(filepath.Join(archived, bench.CardAnchor)) || bench.Exists(card.Dir) {
		t.Error("the card directory did not move whole into the archive")
	}
	if bench.Exists(bench.SiblingPath(card.Dir)) {
		t.Error("a sibling lock stands after the archive answered ok")
	}
	if about := findingsAbout(h.check(), card); len(about) != 0 {
		t.Errorf("check reports %+v for the archived card", about)
	}

	shortBudget(t, 100*time.Millisecond)
	second := h.add("archived while held")
	held := h.card(second)
	closeHeld := foreignHandle(t, filepath.Join(held.Dir, bench.CardAnchor), windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
	interrupted := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: second})
	if interrupted.Refusal != contract.Interrupted {
		t.Fatalf("an archive the reader outlasted answered %s %s, wanted dinah.interrupted", interrupted.Outcome, interrupted.Refusal)
	}
	if !bench.Exists(bench.SiblingPath(held.Dir)) {
		t.Fatal("the interrupted archive left no sibling standing")
	}
	closeHeld()
	if _, err := h.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if !bench.Exists(filepath.Join(h.archivedDir(held.ID), bench.CardAnchor)) || bench.Exists(bench.SiblingPath(held.Dir)) {
		t.Error("check --finish did not complete the interrupted archive")
	}
}

// TestAReleasedLockDoesNotStickUnderDinahsOwnReader asserts that a lock
// released while Dinah's own reader holds it is gone once the reader closes,
// and that an acquisition started while the reader still holds it succeeds
// once it closes rather than refusing.
func TestAReleasedLockDoesNotStickUnderDinahsOwnReader(t *testing.T) {
	h := newHarness(t)
	ref := h.add("locked")
	dir := h.card(ref).Dir
	path := filepath.Join(dir, bench.LockName)

	lock := h.hold(dir, "alka")
	reader, err := durable.Open(path)
	if err != nil {
		t.Fatalf("read the lock the way Dinah does: %v", err)
	}
	lock.Release()
	reader.Close()
	if bench.Exists(path) {
		t.Error("the released lock still stands once the reader closed")
	}

	lock = h.hold(dir, "alka")
	reader, err = durable.Open(path)
	if err != nil {
		t.Fatalf("read the lock the way Dinah does: %v", err)
	}
	lock.Release()
	timer := time.AfterFunc(300*time.Millisecond, func() { reader.Close() })
	defer timer.Stop()
	next, err := bench.Acquire(dir, "brin", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("an acquisition while the released lock was still read answered %v", err)
	}
	next.Release()
}

// TestALockThatCannotBeCreatedRefusesBusy asserts that with the budget at 100
// ms and Dinah's own reader holding a released lock past it, a comment's
// acquisition refuses dinah.busy naming the lock relative to the workbench
// root and carrying the last error, and changes nothing on the card.
func TestALockThatCannotBeCreatedRefusesBusy(t *testing.T) {
	h := newHarness(t)
	ref := h.add("locked")
	card := h.card(ref)
	path := filepath.Join(card.Dir, bench.LockName)
	before := everyFileOf(t, card.Dir)
	lock := h.hold(card.Dir, "alka")
	reader, err := durable.Open(path)
	if err != nil {
		t.Fatalf("read the lock the way Dinah does: %v", err)
	}
	defer reader.Close()
	lock.Release()
	shortBudget(t, 100*time.Millisecond)
	response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: ref, Text: "busy"})
	want := "cards/" + card.ID + "/lock"
	if response.Refusal != contract.Busy || response.Detail != want || response.Context["error"] == "" {
		t.Errorf("the comment answered %s %s %q %v, wanted dinah.busy naming %s with the error", response.Outcome, response.Refusal, response.Detail, response.Context, want)
	}
	reader.Close()
	after := everyFileOf(t, card.Dir)
	for name, text := range before {
		if after[name] != text {
			t.Errorf("%s changed under a busy refusal", name)
		}
	}
}

// everyFileOf answers every file below dir with its bytes, keyed by its path
// relative to dir.
func everyFileOf(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := durable.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(dir, path)
		files[relative] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return files
}

// TestNoActStopsBetweenItsOwnWrites asserts that a move, a pull and an add
// whose later write meets a handle that shares read only, held past a 100 ms
// budget, wait after their first write instead of giving up: each answers ok
// with every line written, Waiting is called at least twice with rising
// notice numbers naming the held file, and check reports nothing for the
// card. Beside it, a move whose first write meets a handle on card.md gives
// up with dinah.busy and leaves the card byte for byte as it was.
func TestNoActStopsBetweenItsOwnWrites(t *testing.T) {
	t.Run("a move", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("moving")
		card := h.card(ref)
		journal := card.JournalPath()
		shortBudget(t, 100*time.Millisecond)
		closeReader := foreignHandle(t, journal, windows.FILE_SHARE_READ)
		notices := recordNotices(t, closeReader)
		response := h.do(&Request{Verb: Move, Actor: "alka", Card: ref, Column: aftercare, NoArchive: true})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("the move answered %s %s", response.Outcome, response.Refusal)
		}
		if column := h.card(ref).Column; column != aftercare {
			t.Errorf("the anchor names %s", column)
		}
		assertLastEvents(t, h.events(ref), contract.EventMoved)
		assertRisingNotices(t, *notices, journal)
		if about := findingsAbout(h.check(), card); len(about) != 0 {
			t.Errorf("check reports %+v", about)
		}
	})
	t.Run("a pull", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("pulled")
		card := h.card(ref)
		journal := card.JournalPath()
		shortBudget(t, 100*time.Millisecond)
		closeReader := foreignHandle(t, journal, windows.FILE_SHARE_READ)
		notices := recordNotices(t, closeReader)
		response := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: doing})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("the pull answered %s %s", response.Outcome, response.Refusal)
		}
		assertLastEvents(t, h.events(ref), contract.EventClaimed, contract.EventMoved)
		assertRisingNotices(t, *notices, journal)
		if about := findingsAbout(h.check(), card); len(about) != 0 {
			t.Errorf("check reports %+v", about)
		}
	})
	t.Run("an add", func(t *testing.T) {
		h := newHarness(t)
		h.add("first")
		numbers := filepath.Join(h.root, bench.CardNumbersName)
		shortBudget(t, 100*time.Millisecond)
		closeReader := foreignHandle(t, numbers, windows.FILE_SHARE_READ)
		notices := recordNotices(t, closeReader)
		response := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "second"})
		h.reopen()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("the add answered %s %s", response.Outcome, response.Refusal)
		}
		assertLastEvents(t, h.events(response.Card.Ref), contract.EventCreated)
		registry, err := durable.ReadFile(numbers)
		if err != nil || !strings.Contains(string(registry), h.cardID(response.Card.Ref)) {
			t.Errorf("the registry carries no line for the new card: %q (%v)", registry, err)
		}
		assertRisingNotices(t, *notices, numbers)
	})
	t.Run("a first write that gives up", func(t *testing.T) {
		h := newHarness(t)
		ref := h.add("untouched")
		card := h.card(ref)
		before := everyFileOf(t, card.Dir)
		shortBudget(t, 100*time.Millisecond)
		notices := recordNotices(t, nil)
		closeReader := foreignHandle(t, filepath.Join(card.Dir, bench.CardAnchor), windows.FILE_SHARE_READ)
		began := time.Now()
		response := h.do(&Request{Verb: Move, Actor: "alka", Card: ref, Column: aftercare, NoArchive: true})
		elapsed := time.Since(began)
		closeReader()
		if response.Refusal != contract.Busy || elapsed > time.Second {
			t.Errorf("a move whose first write was refused answered %s %s after %v, wanted dinah.busy within the budget", response.Outcome, response.Refusal, elapsed)
		}
		if len(*notices) != 0 {
			t.Errorf("a first write that gave up called Waiting %d times", len(*notices))
		}
		after := everyFileOf(t, card.Dir)
		for name, text := range before {
			if after[name] != text {
				t.Errorf("%s changed under a busy refusal", name)
			}
		}
	})
}

// assertLastEvents fails unless a journal ends with the named events in
// order.
func assertLastEvents(t *testing.T, events []bench.Event, names ...string) {
	t.Helper()
	if len(events) < len(names) {
		t.Fatalf("the journal carries %d lines, wanted at least %d", len(events), len(names))
	}
	tail := events[len(events)-len(names):]
	for i, name := range names {
		if tail[i].Event != name {
			t.Errorf("line %d from the end is %s, wanted %s", len(names)-i, tail[i].Event, name)
		}
	}
}
