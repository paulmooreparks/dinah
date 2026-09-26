package verb

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// heavyCard builds, on a harness workbench, the card the count and timing
// tests of dinah-618 read, and answers its reference. It holds 36 items, 12 of
// them unstamped so the checklist sorts through fallbackRank and one filed
// item removed so the stamped ordinals carry a gap, 8 resolved decisions each
// designating one comment below its item, 40 card comments, and one
// attachment on the card and one on its first comment.
func heavyCard(t *testing.T, h *harness) string {
	t.Helper()
	ref := h.ready("heavy")
	kinds := []string{"acceptance_criterion", "decision", "open_question"}
	for i := 0; i < 25; i++ {
		h.file(ref, kinds[i%len(kinds)], fmt.Sprintf("Filed item %d.", i+1))
	}
	filed, err := bench.Items(h.card(ref).Dir)
	if err != nil {
		t.Fatalf("read the filed items back: %v", err)
	}
	if len(filed) != 25 {
		t.Fatalf("the card holds %d filed items, wanted 25", len(filed))
	}
	// The thirteenth filed item goes, leaving its ordinal unused.
	if err := os.RemoveAll(filed[12].Dir); err != nil {
		t.Fatalf("remove the thirteenth item: %v", err)
	}
	for i := 1; i <= 12; i++ {
		h.item(ref, fmt.Sprintf("e%011d", i), "kind: acceptance_criterion\nstate: pending\n", fmt.Sprintf("Unstamped item %d.", i))
	}
	// Each decision is addressed by its position among the card's decisions,
	// read back from the checklist as it now stands.
	items, err := bench.Items(h.card(ref).Dir)
	if err != nil {
		t.Fatalf("read the items back: %v", err)
	}
	decisions := 0
	for _, item := range items {
		if item.Kind != "decision" {
			continue
		}
		decisions++
		h.mustResolve(fmt.Sprintf("%s/decisions/%d", ref, decisions))
	}
	if decisions != 8 {
		t.Fatalf("the card holds %d decisions, wanted 8", decisions)
	}
	for i := 1; i <= 40; i++ {
		h.comment(ref, fmt.Sprintf("Card comment %d.", i))
	}
	h.attach(ref, "card.txt", "card bytes")
	h.attach(ref+"/comments/1", "comment.txt", "comment bytes")

	card := h.card(ref)
	items, err = bench.Items(card.Dir)
	if err != nil {
		t.Fatalf("read the finished checklist: %v", err)
	}
	unstamped, designated := 0, 0
	for _, item := range items {
		if item.Ordinal == 0 {
			unstamped++
		}
		if _, found := h.library.Bench.DesignatedCommentDir(item); found {
			designated++
		}
	}
	comments, err := bench.Comments(card.Dir)
	if err != nil {
		t.Fatalf("read the card's comments: %v", err)
	}
	cardAttachments, err := bench.CountAttachments(card.Dir)
	if err != nil {
		t.Fatalf("count the card's attachments: %v", err)
	}
	if len(items) != 36 || unstamped != 12 || designated != 8 || len(comments) != 40 || cardAttachments != 1 {
		t.Fatalf("the heavy card holds %d items (%d unstamped, %d designating a comment), %d comments and %d attachments; wanted 36 (12, 8), 40 and 1",
			len(items), unstamped, designated, len(comments), cardAttachments)
	}
	commentAttachments, err := bench.CountAttachments(comments[0].Dir)
	if err != nil || commentAttachments != 1 {
		t.Fatalf("the first comment holds %d attachments (%v), wanted 1", commentAttachments, err)
	}
	return ref
}

// mustResolve resolves one checklist item with an answer, which writes the
// comment the item designates.
func (h *harness) mustResolve(ref string) {
	h.t.Helper()
	response := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: ref, Text: "answer"})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("resolve %s: %s %s", ref, response.Outcome, response.Refusal)
	}
	h.reopen()
}

// readCounter records the anchor reads and collection listings a composition
// makes, by path, through the two bench seams. It declares the test that uses
// it non-parallel and puts both seams back.
type readCounter struct {
	reads  map[string]int
	listed map[string]int
}

func countReads(t *testing.T) *readCounter {
	t.Helper()
	counter := &readCounter{reads: map[string]int{}, listed: map[string]int{}}
	bench.AnchorReadObserver = func(path string) { counter.reads[filepath.Clean(path)]++ }
	bench.ListIDsObserver = func(collection string) { counter.listed[filepath.Clean(collection)]++ }
	t.Cleanup(func() {
		bench.AnchorReadObserver = nil
		bench.ListIDsObserver = nil
	})
	return counter
}

func (c *readCounter) stop() {
	bench.AnchorReadObserver = nil
	bench.ListIDsObserver = nil
}

// exactlyOnce asserts that every path of one set was counted once, and that
// the set held as many paths as the fixture put there.
func exactlyOnce(t *testing.T, what string, counts map[string]int, paths []string, want int) {
	t.Helper()
	if len(paths) != want {
		t.Fatalf("the %s set holds %d paths, wanted %d, so the sweep is not over what the fixture built", what, len(paths), want)
	}
	for _, path := range paths {
		if n := counts[filepath.Clean(path)]; n != 1 {
			t.Errorf("%s %s was touched %d times, wanted once", what, path, n)
		}
	}
}

// TestShowReadsEachAnchorAndListsEachCollectionOnce drives dinah-618
// criteria/17. One show of the heavy card with default fields opens each item
// anchor, each card comment anchor and each designated item comment anchor
// once, opens the card's own anchor once for the one card load show makes,
// and lists the card's mounts, each item's comments and each card comment's
// attachments once.
//
// Arming: restoring memberPosition in detailOf reddens it, with each item
// anchor opened 37 times or more.
func TestShowReadsEachAnchorAndListsEachCollectionOnce(t *testing.T) {
	h := newHarness(t)
	ref := heavyCard(t, h)
	card := h.card(ref)
	items, err := bench.Items(card.Dir)
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	comments, err := bench.Comments(card.Dir)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	var itemAnchors, designatedAnchors, itemComments []string
	for _, item := range items {
		itemAnchors = append(itemAnchors, filepath.Join(item.Dir, bench.ItemAnchor))
		itemComments = append(itemComments, filepath.Join(item.Dir, bench.CommentsDir))
		if dir, found := h.library.Bench.DesignatedCommentDir(item); found {
			designatedAnchors = append(designatedAnchors, filepath.Join(dir, bench.CommentAnchor))
		}
	}
	var commentAnchors, commentAttachments []string
	for _, comment := range comments {
		commentAnchors = append(commentAnchors, filepath.Join(comment.Dir, bench.CommentAnchor))
		commentAttachments = append(commentAttachments, filepath.Join(comment.Dir, bench.AttachmentsDir))
	}
	var mounts []string
	for _, mount := range bench.Contains(bench.KindCard) {
		mounts = append(mounts, filepath.Join(card.Dir, mount.Dir))
	}

	counter := countReads(t)
	if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref}); err != nil {
		counter.stop()
		t.Fatalf("show: %v", err)
	}
	counter.stop()

	exactlyOnce(t, "item anchor", counter.reads, itemAnchors, 36)
	exactlyOnce(t, "card comment anchor", counter.reads, commentAnchors, 40)
	exactlyOnce(t, "designated item comment anchor", counter.reads, designatedAnchors, 8)
	exactlyOnce(t, "card mount", counter.listed, mounts, 3)
	exactlyOnce(t, "item comments collection", counter.listed, itemComments, 36)
	exactlyOnce(t, "card comment attachments collection", counter.listed, commentAttachments, 40)
	if n := counter.reads[filepath.Clean(card.AnchorPath())]; n != 1 {
		t.Errorf("show opened the card anchor %d times, wanted once for its one card load", n)
	}
}

// TestStatusReadsEachCardAndItemOnce drives dinah-618 criteria/18. On a
// workbench holding the heavy card and two filled cards, with no claim lapsed
// and no hold rule declared, one status opens each card anchor once and each
// item anchor once, and lists each card's mounts once.
//
// Arming: restoring the second Revision read in loadCard reddens it, with each
// card anchor opened twice.
func TestStatusReadsEachCardAndItemOnce(t *testing.T) {
	h := newHarness(t)
	heavyCard(t, h)
	filledCard(t, h)
	filledCard(t, h)
	if settings, _ := h.library.Bench.Holds(); len(settings.Rules) != 0 {
		t.Fatalf("the harness workbench declares %d hold rules, and a hold rule reads every card again", len(settings.Rules))
	}
	cards, err := h.library.Bench.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	var cardAnchors, itemAnchors, mounts []string
	for _, card := range cards {
		if card.Lapsed(h.library.Now()) {
			t.Fatalf("card %s carries a lapsed claim, and a lapse re-reads the card under its lock", card.ID)
		}
		cardAnchors = append(cardAnchors, card.AnchorPath())
		ids, err := bench.ListIDs(filepath.Join(card.Dir, bench.ChecklistDir))
		if err != nil {
			t.Fatalf("list the checklist of %s: %v", card.ID, err)
		}
		for _, id := range ids {
			itemAnchors = append(itemAnchors, filepath.Join(card.Dir, bench.ChecklistDir, id, bench.ItemAnchor))
		}
		for _, mount := range bench.Contains(bench.KindCard) {
			mounts = append(mounts, filepath.Join(card.Dir, mount.Dir))
		}
	}

	counter := countReads(t)
	if _, err := h.library.Status(&Request{Verb: "status", Actor: "alka"}); err != nil {
		counter.stop()
		t.Fatalf("status: %v", err)
	}
	counter.stop()

	exactlyOnce(t, "card anchor", counter.reads, cardAnchors, 3)
	exactlyOnce(t, "item anchor", counter.reads, itemAnchors, 38)
	exactlyOnce(t, "card mount", counter.listed, mounts, 9)
}

// TestShowOfAHeavyCardStaysNearItsReads drives dinah-618 criteria/10. It
// times show of the heavy card against C, the time to read each of the card's
// 36 item anchors and 40 comment anchors six times each, measured in the same
// run on the same machine. The design expects show to cost about 90 reads,
// about 90 listings and a few milliseconds of composing, which on Windows is
// about 12 to 15 ms against a C of about 31 ms.
//
// The bound is asserted on Windows only. On Linux and macOS a file read costs
// a few microseconds and composing dominates, so a bound expressed in reads
// would measure the wrong thing there; the two count tests above carry this
// card's guarantee on those platforms. Every platform logs the numbers.
//
// Arming: restoring memberPosition in detailOf on Windows puts the median
// near 200 ms and reddens it.
func TestShowOfAHeavyCardStaysNearItsReads(t *testing.T) {
	if testing.Short() {
		t.Skip("times show of a heavy card, which -short leaves out")
	}
	h := newHarness(t)
	ref := heavyCard(t, h)
	card := h.card(ref)
	items, err := bench.Items(card.Dir)
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	comments, err := bench.Comments(card.Dir)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	var anchors []string
	for _, item := range items {
		anchors = append(anchors, filepath.Join(item.Dir, bench.ItemAnchor))
	}
	for _, comment := range comments {
		anchors = append(anchors, filepath.Join(comment.Dir, bench.CommentAnchor))
	}
	if len(anchors) != 76 {
		t.Fatalf("the heavy card offers %d anchors to time, wanted 76", len(anchors))
	}

	show := func() time.Duration {
		start := time.Now()
		if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref}); err != nil {
			t.Fatalf("show: %v", err)
		}
		return time.Since(start)
	}
	measure := func() (median, reads time.Duration, runs []time.Duration) {
		show()
		show()
		for i := 0; i < 5; i++ {
			runs = append(runs, show())
		}
		start := time.Now()
		for i := 0; i < 6; i++ {
			for _, anchor := range anchors {
				if _, err := os.ReadFile(anchor); err != nil {
					t.Fatalf("read %s: %v", anchor, err)
				}
			}
		}
		reads = time.Since(start)
		sorted := append([]time.Duration(nil), runs...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		return sorted[len(sorted)/2], reads, runs
	}

	median, reads, runs := measure()
	t.Logf("show of the heavy card: median %v over %v; C, %d reads, %v", median, runs, 6*len(anchors), reads)
	if runtime.GOOS != "windows" || median <= reads {
		return
	}
	// One retry, as dinah-621's harness retries, because one slow pass on a
	// busy machine is not the regression this guards.
	second, secondReads, secondRuns := measure()
	t.Logf("retry: median %v over %v; C %v", second, secondRuns, secondReads)
	if second > secondReads {
		t.Errorf("show of the heavy card is slower than reading its anchors six times over, twice running:\n first  median %v, C %v, runs %s\n second median %v, C %v, runs %s",
			median, reads, durations(runs), second, secondReads, durations(secondRuns))
	}
}

// durations prints a run of timings on one line.
func durations(runs []time.Duration) string {
	parts := make([]string, len(runs))
	for i, run := range runs {
		parts[i] = run.String()
	}
	return strings.Join(parts, " ")
}
