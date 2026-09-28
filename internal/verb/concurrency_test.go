package verb

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/durable"
	"dinah/internal/msg"
)

// defaultRetryBudget is durable.RetryBudget as the package declares it, read
// before any test in this binary can shorten it.
var defaultRetryBudget = durable.RetryBudget

// cardsIn counts the live cards standing in a column.
func (h *harness) cardsIn(column string) int {
	h.t.Helper()
	cards, err := h.library.Bench.Cards()
	if err != nil {
		h.t.Fatalf("cards: %v", err)
	}
	count := 0
	for _, card := range cards {
		if card.Column == column {
			count++
		}
	}
	return count
}

// atCapacityCounted arms the library so that inner runs once, at the
// capacity-counted window of the next move, pull or add.
func (h *harness) atCapacityCounted(inner func()) {
	fired := false
	h.library.Interpose = func(step string) {
		if step != stepCapacityCounted || fired {
			return
		}
		fired = true
		inner()
	}
}

// TestACapacityColumnCannotBeOverfilledByOverlappingWrites asserts that while
// a move of one card into a column with one free place stands between its
// count and its write, a complete move, pull or add of another card into the
// same column is refused dinah.locked naming the first mover, and the column
// ends holding one card.
func TestACapacityColumnCannotBeOverfilledByOverlappingWrites(t *testing.T) {
	inners := map[string]func(h *harness, other string) *Response{
		"a move": func(h *harness, other string) *Response {
			return h.second().Do(&Request{Verb: Move, Actor: "brin", Card: other, Column: doing})
		},
		"a pull": func(h *harness, other string) *Response {
			return h.second().Pull(&Request{Verb: Pull, Actor: "brin", Column: doing})
		},
		"an add": func(h *harness, other string) *Response {
			return h.second().Add(&Request{Verb: "add", Actor: "brin", Title: "third", Column: doing})
		},
	}
	for name, inner := range inners {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			other := h.add("waiting first")
			mover := h.add("moving")
			var refused *Response
			h.atCapacityCounted(func() { refused = inner(h, other) })
			moved := h.library.Do(&Request{Verb: Move, Actor: "alka", Card: mover, Column: doing})
			h.library.Interpose = nil
			h.reopen()
			if moved.Outcome != contract.OutcomeOK {
				t.Fatalf("the outer move answered %s %s", moved.Outcome, moved.Refusal)
			}
			if refused == nil {
				t.Fatal("the capacity-counted window never opened")
			}
			if refused.Refusal != contract.Locked || refused.Detail != "alka" {
				t.Errorf("%s during the window answered %s %s %q, wanted dinah.locked naming alka",
					name, refused.Outcome, refused.Refusal, refused.Detail)
			}
			if held := h.cardsIn(doing); held != 1 {
				t.Errorf("the column of capacity one holds %d cards", held)
			}
		})
	}
}

// TestSequentialMovesFillACapacityColumnAndNoCapacityTakesNoLock asserts the
// accepting side of the occupancy lock: two moves one after the other into a
// column of capacity two both land and a third is refused at-capacity, and
// two overlapping moves into a column declaring no capacity both land with no
// occupancy lock file created.
func TestSequentialMovesFillACapacityColumnAndNoCapacityTakesNoLock(t *testing.T) {
	h := newHarness(t)
	h.declare(doing, "wip_limit", "2")
	for _, title := range []string{"one", "two"} {
		ref := h.add(title)
		h.mustDo(&Request{Verb: Move, Actor: "alka", Card: ref, Column: doing})
	}
	third := h.add("three")
	if response := h.do(&Request{Verb: Move, Actor: "alka", Card: third, Column: doing}); response.Refusal != contract.AtCapacity {
		t.Errorf("a third move into a column of capacity two answered %s %s", response.Outcome, response.Refusal)
	}

	first := h.add("first overlapping")
	second := h.add("second overlapping")
	occupancy := filepath.Join(h.library.Bench.ColumnDir(aftercare), bench.LockName)
	var inner *Response
	lockSeen := false
	h.atCapacityCounted(func() {
		lockSeen = bench.Exists(occupancy)
		inner = h.second().Do(&Request{Verb: Move, Actor: "brin", Card: second, Column: aftercare})
	})
	outer := h.library.Do(&Request{Verb: Move, Actor: "alka", Card: first, Column: aftercare})
	h.library.Interpose = nil
	h.reopen()
	if outer.Outcome != contract.OutcomeOK || inner == nil || inner.Outcome != contract.OutcomeOK {
		t.Errorf("two overlapping moves into a column with no capacity answered %+v and %+v", outer, inner)
	}
	if lockSeen || bench.Exists(occupancy) {
		t.Error("a move into a column declaring no capacity created an occupancy lock")
	}
}

// TestADeadHoldersLockIsReclaimedByAComment asserts that a comment on a card
// whose lock a running process holds is refused dinah.locked and leaves the
// lock file as it was, and that once that process has ended the comment
// answers ok, and the card's journal carries lock_reclaimed followed by the
// commented line.
func TestADeadHoldersLockIsReclaimedByAComment(t *testing.T) {
	h := newHarness(t)
	ref := h.add("held")
	dir := h.card(ref).Dir
	lockPath := filepath.Join(dir, bench.LockName)
	child := holdInChild(t, dir, "brin")
	before, err := durable.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read the lock: %v", err)
	}
	refused := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: ref, Text: "while held"})
	if refused.Refusal != contract.Locked || refused.Detail != "brin" {
		t.Errorf("a comment beside a running holder answered %s %s %q", refused.Outcome, refused.Refusal, refused.Detail)
	}
	if after, err := durable.ReadFile(lockPath); err != nil || string(after) != string(before) {
		t.Errorf("the live holder's lock changed: %q to %q (%v)", before, after, err)
	}
	child.endAndWaitDead(t, lockPath)
	h.comment(ref, "after the holder ended")
	events := h.events(ref)
	if len(events) < 2 {
		t.Fatalf("the journal carries %d lines", len(events))
	}
	last := events[len(events)-2:]
	if last[0].Event != contract.EventLockReclaimed || last[1].Event != contract.EventCommented {
		t.Errorf("the journal ends %s then %s, wanted lock_reclaimed then commented", last[0].Event, last[1].Event)
	}
	if !strings.Contains(last[0].Note, `"actor":"brin"`) {
		t.Errorf("the lock_reclaimed line carries %q, wanted the dead record", last[0].Note)
	}
	if bench.Exists(lockPath) {
		t.Error("the reclaimed lock was not released with the comment")
	}
}

// TestARestoreRefusesALockCarriedIntoTheArchive asserts that a card archived
// with a lock standing in its archived directory is refused dinah.locked by
// restore, naming that lock's holder and writing no restored line; that once
// the holder has ended check reports the lock under check.stale-lock; and
// that check --finish clears it and the restore then succeeds.
func TestARestoreRefusesALockCarriedIntoTheArchive(t *testing.T) {
	h := newHarness(t)
	ref := h.add("archived")
	id := h.cardID(ref)
	h.archive(ref)
	archived := h.archivedDir(id)
	lockPath := filepath.Join(archived, bench.LockName)
	child := holdInChild(t, archived, "brin")
	refused := h.library.Restore(&Request{Verb: "restore", Actor: "alka", Ref: ref})
	if refused.Refusal != contract.Locked || refused.Detail != "brin" {
		t.Fatalf("a restore over a carried lock answered %s %s %q", refused.Outcome, refused.Refusal, refused.Detail)
	}
	journal, _, err := bench.ReadJournal(filepath.Join(archived, bench.JournalName))
	if err != nil {
		t.Fatalf("read the archived journal: %v", err)
	}
	for _, ev := range journal {
		if ev.Event == contract.EventRestored {
			t.Error("a refused restore wrote a restored line")
		}
	}
	child.endAndWaitDead(t, lockPath)
	reported := false
	for _, found := range h.check() {
		if found.Key == bench.FindingStaleLock && found.Path == lockPath && found.Detail == "brin dead" {
			reported = true
		}
	}
	if !reported {
		t.Error("check did not report the carried lock as a dead stale lock")
	}
	if _, err := h.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if bench.Exists(lockPath) {
		t.Error("the finish left the dead carried lock standing")
	}
	h.reopen()
	restored := h.library.Restore(&Request{Verb: "restore", Actor: "alka", Ref: ref})
	if restored.Outcome != contract.OutcomeOK {
		t.Errorf("the restore after the finish answered %s %s", restored.Outcome, restored.Refusal)
	}
}

// TestTheBusyRefusalStatesTheDefaultBudget asserts that the wait the English
// dinah.busy sentence states is the retry budget durable declares.
func TestTheBusyRefusalStatesTheDefaultBudget(t *testing.T) {
	entry, ok := msg.BaseEntry("refusal.dinah.busy")
	if !ok {
		t.Fatal("the base catalog carries no refusal.dinah.busy")
	}
	if !strings.Contains(entry.Text, "five seconds") || defaultRetryBudget != 5*time.Second {
		t.Errorf("the sentence %q and the budget %v disagree", entry.Text, defaultRetryBudget)
	}
}
