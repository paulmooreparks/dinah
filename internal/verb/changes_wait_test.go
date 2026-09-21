package verb

import (
	"os"
	"testing"
	"time"

	"dinah/internal/contract"
)

// changes_wait_test.go covers dinah-546: `changes --wait` holds a checkpoint
// open until the cursor advances or its own timeout lapses. It is a new file
// rather than an extension of changes_test.go because --wait is genuinely new
// behavior with no home there: every test in that file asserts one immediate
// checkpoint's answer, and this file's tests are concurrent and timing-based,
// each running a second, independent *Library against the same bench root
// while the first blocks in Library.Changes.
//
// Every test below is bounded by a hard deadline distinct from whatever
// timeout the call under test carries, per dinah-546/criteria/13 (a select
// against time.After, or a synchronous call whose own worst case is checked
// against a generous bound), so a defect that made the wait loop hang would
// fail the test rather than hang the suite.

// h.secondLibrary is declared in checklist_test.go, alongside
// TestTheCardLockCoversTheWholeTransaction, and is reused here rather than
// redeclared: it is a distinct *Library value rather than h.library reused
// from another goroutine, which is what a real second process needs standing
// in for it, and what keeps two goroutines from sharing one *Library across a
// read that is blocked in Changes and a write from reopen().

// mustDoOn runs one of the five contract verbs (Library.Do's own set, which
// Move is a member of) against a library that is not a harness's own (the
// "second process" a wait test drives concurrently) and fails the test
// unless it succeeded. Called from the test's own goroutine only: every
// waiting call this file makes runs on a separate goroutine of its own, and
// the second writer always acts from the test goroutine instead, which is
// what makes this helper safe to call t.Fatalf from.
func mustDoOn(t *testing.T, library *Library, req *Request) *Response {
	t.Helper()
	response := library.Do(req)
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("%s: wanted ok, got %s %s", req.Verb, response.Outcome, response.Refusal)
	}
	return response
}

// mustCommentOn writes a comment through a library that is not a harness's
// own, mirroring harness.comment (fixture_test.go) for the second-process
// case this file needs. Comment sits outside Library.Do's five contract
// verbs, so it is called directly rather than through mustDoOn.
func mustCommentOn(t *testing.T, library *Library, ref, text string) *Response {
	t.Helper()
	response := library.Comment(&Request{Verb: "comment", Actor: "alka", Card: ref, Text: text})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment on %s: %s %s", ref, response.Outcome, response.Refusal)
	}
	return response
}

// waitOutcome is one waiting checkpoint's answer, carried off its own
// goroutine so the test's own goroutine is the one that reads and fails it:
// t.Fatal and friends are unsafe to call from any goroutine but the one
// running the test.
type waitOutcome struct {
	set *ChangeSet
	err error
}

// waitAsync starts a waiting checkpoint on its own goroutine and returns a
// channel carrying its answer. req.Wait is set here rather than by every
// caller, since every call this file makes through it is a waiting one.
func waitAsync(library *Library, req *Request) <-chan waitOutcome {
	req.Wait = true
	out := make(chan waitOutcome, 1)
	go func() {
		set, err := library.Changes(req)
		out <- waitOutcome{set: set, err: err}
	}()
	return out
}

// TestAWaitingCallWakesWithinOnePollIntervalOfAChange covers dinah-546 AC-1's
// accepting case (dinah-546/criteria/1): a second library changes the bench
// while the first is waiting, and the wait returns quickly, reporting the
// change. The refusing half of the same acceptance criterion, proving a wait
// that always returned fast would not pass this test alone, is
// TestAWaitingCallWithNothingChangingReturnsAtItsTimeout below, in this same
// file.
//
// Hard test deadline: 5s, via the select's time.After, comfortably above
// waitPollInterval (500ms) and the ~80ms the fixture waits before making its
// change.
func TestAWaitingCallWakesWithinOnePollIntervalOfAChange(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card")
	cursor := h.mint()
	second := h.secondLibrary()

	results := waitAsync(h.library, &Request{Since: cursor, Timeout: 5 * time.Second})

	// Give the first iteration a moment to run its own checkpoint and enter
	// its sleep before the change lands, so the wake this test measures is
	// the sleeping iteration noticing a change rather than a lucky first
	// check racing the goroutine's own start.
	time.Sleep(80 * time.Millisecond)
	changedAt := time.Now()
	mustCommentOn(t, second, ref, "a note the wait should notice")

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("waiting changes: %v", result.err)
		}
		if !result.set.Changed {
			t.Fatal("the wait returned without reporting the change that woke it")
		}
		elapsed := time.Since(changedAt)
		if elapsed > 2*waitPollInterval {
			t.Errorf("the wait took %s to notice a change, more than two poll intervals (%s)", elapsed, 2*waitPollInterval)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the wait never returned; hard test deadline hit")
	}
}

// TestAWaitingCallWithNothingChangingReturnsAtItsTimeout covers dinah-546
// AC-1's refusing case, in the same file as the accepting case above: nothing
// touches the bench, and the wait still returns, at its own timeout, with
// Changed false and the cursor handed back unchanged. Read together, the two
// tests are what proves the wake in the accepting case is a real wake rather
// than a wait that simply always returns fast.
//
// Hard test deadline: 3s, comfortably above the 900ms timeout under test.
func TestAWaitingCallWithNothingChangingReturnsAtItsTimeout(t *testing.T) {
	h := newHarness(t)
	h.add("A card")
	cursor := h.mint()

	timeout := 900 * time.Millisecond
	started := time.Now()
	results := waitAsync(h.library, &Request{Since: cursor, Timeout: timeout})

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("waiting changes: %v", result.err)
		}
		elapsed := time.Since(started)
		if result.set.Changed {
			t.Error("the wait reported a change nothing made")
		}
		if result.set.Cursor != cursor {
			t.Errorf("the cursor came back rewritten:\nwanted %q\ngot    %q", cursor, result.set.Cursor)
		}
		if elapsed < timeout {
			t.Errorf("the wait returned after %s, before its own %s timeout", elapsed, timeout)
		}
		if elapsed > timeout+waitPollInterval {
			t.Errorf("the wait returned after %s, more than one poll interval past its own %s timeout: the deadline cap did not hold", elapsed, timeout)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the wait never returned; hard test deadline hit")
	}
}

// TestAFilteredWaitingCallStillWakesOnAnUnrelatedChange covers dinah-546
// AC-3 (dinah-546/criteria/3): a --column filter narrowing a waiting call
// still wakes it on a change elsewhere on the bench, with Changed true and
// the event/card arrays empty, exactly as
// TestAFilteredCallStillAdvancesPastWhatItDidNotReport (changes_test.go:397)
// already proves for an immediate call. The paired control is an unfiltered
// wait against the same fixture and the same change, which wakes with the
// arrays populated: that is what proves the filtered case's empty arrays come
// from the filter narrowing the report rather than from the wait failing to
// see the change at all.
//
// Hard test deadline: 5s per wait, via each select's time.After.
func TestAFilteredWaitingCallStillWakesOnAnUnrelatedChange(t *testing.T) {
	h := newHarness(t)
	// Filed at intake (h.add's own default) and moved to doing: neither end
	// of the move names the aftercare column the wait below filters on, which
	// is what makes this "elsewhere" rather than a departure from the watched
	// column (inColumn, changes.go, admits a move whose From or To names the
	// wanted column, so a card leaving aftercare would not be a clean control).
	elsewhere := h.add("A card that moves while the filtered wait blocks")
	cursor := h.mint()
	second := h.secondLibrary()

	filtered := waitAsync(h.library, &Request{Since: cursor, Column: aftercareSlug, Timeout: 5 * time.Second})
	time.Sleep(80 * time.Millisecond)
	mustDoOn(t, second, &Request{Verb: Move, Actor: "alka", Card: elsewhere, Column: doing})

	select {
	case result := <-filtered:
		if result.err != nil {
			t.Fatalf("waiting changes (filtered): %v", result.err)
		}
		if !result.set.Changed {
			t.Fatal("the filtered wait did not wake on an unrelated change")
		}
		if len(result.set.Events) != 0 || len(result.set.Cards) != 0 || len(result.set.Gone) != 0 {
			t.Errorf("the column filter let an unrelated change through: %d events, %d cards, %d gone",
				len(result.set.Events), len(result.set.Cards), len(result.set.Gone))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the filtered wait never returned; hard test deadline hit")
	}

	// The control: the same cursor, unfiltered, waiting on a fresh change
	// after the first has already landed. Independent Library value again, so
	// the two waits do not share mutable state.
	third := h.secondLibrary()
	control := waitAsync(third, &Request{Since: cursor, Timeout: 5 * time.Second})
	select {
	case result := <-control:
		if result.err != nil {
			t.Fatalf("waiting changes (control): %v", result.err)
		}
		if names := eventNames(result.set); len(names) != 1 || names[0] != contract.EventMoved {
			t.Fatalf("the move was never delivered to anybody unfiltered, so the filtered answer above proves nothing: %v", names)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the control wait never returned; hard test deadline hit")
	}
}

// TestAWaitingCallReturnsUnreachableWhenTheWorkbenchDisappears covers
// dinah-546 AC-4 (dinah-546/criteria/4): the workbench directory disappears
// while a call is waiting, and the call returns an error rather than hanging,
// panicking or retrying forever, within about one poll interval of the
// disappearance. The error itself, and its mapping to OutcomeUnreachable and
// exit 4, is cmd/dinah's reportError (main.go:517-522), unchanged by this
// card and exercised directly by
// TestChangesWaitMapsADisappearedWorkbenchToExitFour in
// cmd/dinah/changes_test.go; what belongs here is that Library.Changes itself
// propagates a plain, non-refusal error immediately rather than swallowing or
// retrying it, which is what that mapping needs to have something to map.
// The paired refusing case, in the same file, is
// TestAWaitingCallDoesNotSpuriouslyReturnAnErrorWhenNothingIsDeleted below.
//
// Hard test deadline: 5s, via the select's time.After.
func TestAWaitingCallReturnsUnreachableWhenTheWorkbenchDisappears(t *testing.T) {
	h := newHarness(t)
	h.add("A card")
	cursor := h.mint()

	results := waitAsync(h.library, &Request{Since: cursor, Timeout: 5 * time.Second})
	time.Sleep(80 * time.Millisecond)
	renamedTo := h.root + "-renamed-out-from-under-the-wait"
	if err := os.Rename(h.root, renamedTo); err != nil {
		t.Fatalf("rename the workbench root out from under the wait: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(renamedTo) })

	select {
	case result := <-results:
		if result.err == nil {
			t.Fatal("the wait answered with no error after its own workbench disappeared")
		}
		if _, isRefusal := result.err.(*contract.Refusal); isRefusal {
			t.Errorf("the error is a refusal (%v); AC-4 wants the plain, non-refusal shape an ordinary WatchedEntities failure already returns, which is what maps to OutcomeUnreachable rather than to a refused exit", result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the wait never returned after its own workbench disappeared; hard test deadline hit")
	}
}

// TestAWaitingCallDoesNotSpuriouslyReturnAnErrorWhenNothingIsDeleted is AC-4's
// refusing case, in the same file as the accepting case above: a bench that
// is never deleted does not spuriously answer an error before its own
// timeout, which is what makes the accepting case's error mean "the
// workbench disappeared" rather than "a wait errors before its timeout
// regardless".
//
// Hard test deadline: 3s, comfortably above the 300ms timeout under test.
func TestAWaitingCallDoesNotSpuriouslyReturnAnErrorWhenNothingIsDeleted(t *testing.T) {
	h := newHarness(t)
	h.add("A card")
	cursor := h.mint()

	results := waitAsync(h.library, &Request{Since: cursor, Timeout: 300 * time.Millisecond})
	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("an untouched, undeleted workbench returned an error: %v", result.err)
		}
		if result.set.Changed {
			t.Error("an untouched workbench reported a change")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the wait never returned; hard test deadline hit")
	}
}

// TestAWaitWithNoTimeoutBlocksPastTwoIntervalsThenWakes covers dinah-546
// AC-10/D-4 (dinah-546/criteria/12): omitting --timeout means the wait blocks
// unbounded rather than applying a hidden finite default, proven by blocking
// past at least two poll intervals with nothing changing and then waking on a
// change.
//
// Hard test deadline: 5s, via the select's time.After, well past the
// 2*waitPollInterval (1s) floor this test asserts the wait crossed.
func TestAWaitWithNoTimeoutBlocksPastTwoIntervalsThenWakes(t *testing.T) {
	h := newHarness(t)
	ref := h.add("A card")
	cursor := h.mint()
	second := h.secondLibrary()

	started := time.Now()
	results := waitAsync(h.library, &Request{Since: cursor})

	// Nothing changes for past two poll intervals, so a hidden finite default
	// shorter than this would already have answered by the time this sleep
	// returns, and the select below would read a stale, already-closed
	// answer rather than the one the change below produces.
	time.Sleep(2*waitPollInterval + 100*time.Millisecond)
	select {
	case result := <-results:
		t.Fatalf("the wait answered after %s with nothing changed, before this test made a change: %+v", time.Since(started), result)
	default:
	}

	mustCommentOn(t, second, ref, "wakes the unbounded wait")
	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("waiting changes: %v", result.err)
		}
		elapsed := time.Since(started)
		if elapsed < 2*waitPollInterval {
			t.Errorf("the wait answered after only %s, under the two poll intervals this test needs to prove no hidden default applied", elapsed)
		}
		if !result.set.Changed {
			t.Fatal("the wait returned without reporting the change that woke it")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the wait never returned after being woken; hard test deadline hit")
	}
}
