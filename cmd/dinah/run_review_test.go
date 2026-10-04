package main

import (
	"strings"
	"testing"
	"time"

	"dinah/internal/contract"
	"dinah/internal/testenv"
)

// addReview puts a review column between doing and done that runs the fake
// recipe with fresh eyes and sends rejected work back to doing, holding at
// most capacity cards.
func addReview(t *testing.T, rb *runBench, capacity string) {
	t.Helper()
	if got := runCLI(t, rb.root, "column", "new", "Review", "--slug", "review", "--before", "done", "--capacity", capacity); got.code != 0 {
		t.Fatalf("column new: %d %s", got.code, got.errw)
	}
	column := strings.TrimSpace(runCLI(t, rb.root, "path", "review").out)
	editAnchorAt(t, column, "kind: work", "kind: work\nrun: fake\nworker: fresh\nreject_to: doing")
}

// TestRunReviewSendsWorkBackToTheSameAgent is the choice the run's storage
// rule exists for: implement keeps its session, a fresh review starts its own
// and does not store it, and when review sends the card back, implement
// resumes the session it started with rather than the reviewer's.
func TestRunReviewSendsWorkBackToTheSameAgent(t *testing.T) {
	rb := newRunBench(t)
	addReview(t, rb, "5")
	card := addCard(t, rb.root, "Implement, then review")
	carryToDoing(t, rb.root, card)

	rb.say(t, testenv.HarnessScript{Session: "sess-A", Cost: 0.2, Text: result("Implemented.", "forward", "ready for review")})
	if got := runCLI(t, rb.root, "run", card); got.code != 0 {
		t.Fatalf("implement run: %d %s", got.code, got.errw)
	}
	if column, _, _ := rb.columnOf(t, card); column != "review" {
		t.Fatalf("after implement the card stands at %s, wanted review", column)
	}

	rb.say(t, testenv.HarnessScript{Session: "sess-B", Cost: 0.1, Text: result("The edge case is missing.", "back", "needs the edge case")})
	if got := runCLI(t, rb.root, "run", card); got.code != 0 {
		t.Fatalf("review run: %d %s", got.code, got.errw)
	}
	launches := rb.launches(t)
	if review := launches[len(launches)-1].Args; len(review) == 0 || review[0] != "fresh" {
		t.Errorf("the review ran %q, wanted a fresh session", review)
	}
	if column, state, _ := rb.columnOf(t, card); column != "doing" || state != contract.StateReady {
		t.Errorf("after back the card stands at %s/%s, wanted doing/ready", column, state)
	}
	if got := rb.field(t, card, "run.session"); got != "sess-A" {
		t.Errorf("the review's session displaced the implementer's: run.session is %q", got)
	}
	if got := rb.field(t, card, "run.cumulative.usd"); got != "0.2" {
		t.Errorf("the review's cost displaced the implementer's baseline: %q", got)
	}

	rb.say(t, testenv.HarnessScript{Session: "sess-A", Cost: 0.5, Text: result("Added the edge case.", "stay", "one more look")})
	if got := runCLI(t, rb.root, "run", card); got.code != 0 {
		t.Fatalf("second implement run: %d %s", got.code, got.errw)
	}
	launches = rb.launches(t)
	if got := strings.Join(launches[len(launches)-1].Args, " "); got != "resumed --resume sess-A" {
		t.Errorf("implement after the push-back ran %q, wanted the resume argv naming sess-A", got)
	}
	spends := rb.spends(t, card)
	if last := spends[len(spends)-1]; last.Total == nil || *last.Total != 0.3 {
		t.Errorf("the resumed implement pass recorded %+v, wanted 0.3, the difference from sess-A's baseline", last)
	}
}

// TestRunDoesNotResumeAnotherRecipesSession holds the same-recipe guard: a
// session stored by one recipe is never handed to another recipe's resume
// argv, which starts fresh instead.
func TestRunDoesNotResumeAnotherRecipesSession(t *testing.T) {
	rb := newRunBench(t)
	card := addCard(t, rb.root, "Changes recipe")
	carryToDoing(t, rb.root, card)
	rb.say(t, testenv.HarnessScript{Session: "sess-X", Cost: 0.2, Text: result("first", "stay", "again")})
	if got := runCLI(t, rb.root, "run", card); got.code != 0 {
		t.Fatalf("first run: %d %s", got.code, got.errw)
	}
	column := strings.TrimSpace(runCLI(t, rb.root, "path", "doing").out)
	editAnchorAt(t, column, "run: fake", "run: other")
	rb.say(t, testenv.HarnessScript{Session: "sess-Y", Cost: 0.4, Text: result("second", "stay", "again")})
	if got := runCLI(t, rb.root, "run", card); got.code != 0 {
		t.Fatalf("second run: %d %s", got.code, got.errw)
	}
	launches := rb.launches(t)
	if got := strings.Join(launches[len(launches)-1].Args, " "); got != "fresh-other" {
		t.Errorf("recipe other ran %q, wanted its fresh command rather than a resume of fake's session", got)
	}
	if got := rb.field(t, card, "run.recipe"); got != "other" {
		t.Errorf("run.recipe is %q, wanted other", got)
	}
	spends := rb.spends(t, card)
	if last := spends[len(spends)-1]; last.Total == nil || *last.Total != 0.4 {
		t.Errorf("a fresh session recorded %+v, wanted its whole figure 0.4", last)
	}
}

// TestRunReleasesWhenTheForwardMoveIsRefused fills the next column to its
// capacity, so the move the agent asked for is refused: the card is released
// where it stood, the refusal is posted, and the run exits 1.
func TestRunReleasesWhenTheForwardMoveIsRefused(t *testing.T) {
	rb := newRunBench(t)
	addReview(t, rb, "1")
	occupant := addCard(t, rb.root, "Already in review")
	if got := runCLI(t, rb.root, "move", occupant, "review"); got.code != 0 {
		t.Fatalf("move occupant: %d %s", got.code, got.errw)
	}
	card := addCard(t, rb.root, "Blocked by a full review")
	carryToDoing(t, rb.root, card)
	rb.say(t, testenv.HarnessScript{Session: "sess-C", Cost: 0.2, Text: result("Done here.", "forward", "ready")})
	got := runCLI(t, rb.root, "run", card)
	if got.code != 1 {
		t.Fatalf("a run whose move was refused exited %d, wanted 1: %s %s", got.code, got.out, got.errw)
	}
	if column, state, held := rb.columnOf(t, card); column != "doing" || state != contract.StateReady || held.Holder != "" {
		t.Errorf("the refused move left the card at %s/%s held by %q, wanted it released in doing", column, state, held.Holder)
	}
	comments := rb.comments(t, card)
	if len(comments) != 2 || !strings.Contains(comments[1], contract.AtCapacity) {
		t.Errorf("the comments are %q, wanted the handoff and then the at-capacity refusal", comments)
	}
}

// TestRunClearsABaselineNoSessionOwns covers a receipt carrying a cost and no
// session: nothing can resume it, so the stored session and its baseline are
// cleared, and the next run starts fresh and records its whole figure.
func TestRunClearsABaselineNoSessionOwns(t *testing.T) {
	rb := newRunBench(t)
	card := addCard(t, rb.root, "Loses its session")
	carryToDoing(t, rb.root, card)
	rb.say(t, testenv.HarnessScript{Session: "sess-A", Cost: 0.2, Text: result("first", "stay", "again")})
	if got := runCLI(t, rb.root, "run", card); got.code != 0 {
		t.Fatalf("first run: %d %s", got.code, got.errw)
	}
	rb.say(t, testenv.HarnessScript{Session: "", Cost: 0.9, Text: result("second", "stay", "again")})
	if got := runCLI(t, rb.root, "run", card); got.code != 0 {
		t.Fatalf("second run: %d %s", got.code, got.errw)
	}
	for _, key := range []string{"run.session", "run.recipe", "run.cumulative.usd"} {
		if got := rb.field(t, card, key); got != "" {
			t.Errorf("%s is %q after a receipt naming no session, wanted it cleared", key, got)
		}
	}
	rb.say(t, testenv.HarnessScript{Session: "sess-B", Cost: 0.4, Text: result("third", "stay", "again")})
	if got := runCLI(t, rb.root, "run", card); got.code != 0 {
		t.Fatalf("third run: %d %s", got.code, got.errw)
	}
	launches := rb.launches(t)
	if third := launches[len(launches)-1].Args; len(third) == 0 || third[0] != "fresh" {
		t.Errorf("the run after a lost session ran %q, wanted a fresh session", third)
	}
	spends := rb.spends(t, card)
	if last := spends[len(spends)-1]; last.Total == nil || *last.Total != 0.4 {
		t.Errorf("the fresh session recorded %+v, wanted its whole figure 0.4", last)
	}
}

// TestRunClaimLapsesAfterTheTimeout holds the claim a run takes to an expiry
// a margin past the run's own timeout, so a killed run's claim lapses.
func TestRunClaimLapsesAfterTheTimeout(t *testing.T) {
	rb := newRunBench(t)
	card := addCard(t, rb.root, "Claimed with an expiry")
	carryToDoing(t, rb.root, card)
	rb.say(t, testenv.HarnessScript{Session: "sess-A", Cost: 0.1, Text: result("done", "stay", "again")})
	if got := runCLI(t, rb.root, "run", card, "--timeout", "2m"); got.code != 0 {
		t.Fatalf("run: %d %s", got.code, got.errw)
	}
	held, opened := rb.card(t, card)
	events, _, err := opened.ReadJournal(held.JournalPath())
	if err != nil {
		t.Fatalf("journal: %v", err)
	}
	for _, ev := range events {
		if ev.Event != contract.EventClaimed {
			continue
		}
		at, errAt := time.Parse(time.RFC3339, ev.TS)
		until, errUntil := time.Parse(time.RFC3339, ev.Expires)
		if errAt != nil || errUntil != nil {
			t.Fatalf("the claim carries ts %q and expires %q", ev.TS, ev.Expires)
		}
		if want := 2*time.Minute + runClaimMargin; until.Sub(at) < want-time.Second || until.Sub(at) > want+time.Second {
			t.Errorf("the claim lapses %s after it was taken, wanted %s", until.Sub(at), want)
		}
		return
	}
	t.Fatalf("the run wrote no claimed event")
}
