package perfstore_test

import (
	"os/exec"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/perfstore"
	"dinah/internal/verb"
)

// generate writes the small shape at the default seed into a fresh temporary
// directory and fails the test on any error.
func generate(t *testing.T, seed uint64) *perfstore.Store {
	t.Helper()
	store, err := perfstore.Generate(t.TempDir(), seed, perfstore.SmallShape())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return store
}

// TestGenerateIsDeterministic asserts that two generations of one seed and
// shape have one digest and one file count, that the count Digest reads off
// the tree is the count Generate kept and is no smaller than one file per
// anchor, journal and payload the shape names, and that the next seed gives a
// different digest.
func TestGenerateIsDeterministic(t *testing.T) {
	t.Parallel()
	shape := perfstore.SmallShape()
	first := generate(t, perfstore.DefaultSeed)
	second := generate(t, perfstore.DefaultSeed)
	if first.Digest != second.Digest {
		t.Errorf("one seed gave two digests: %s and %s", first.Digest, second.Digest)
	}
	if first.Files != second.Files {
		t.Errorf("one seed gave two file counts: %d and %d", first.Files, second.Files)
	}
	digest, files, err := perfstore.Digest(first.Root)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if digest != first.Digest {
		t.Errorf("Digest of the tree is %s, Generate reported %s", digest, first.Digest)
	}
	if files != first.Files {
		t.Errorf("the tree holds %d files, Generate counted %d", files, first.Files)
	}
	floor := shape.Cards + shape.ArchivedCards + shape.Items + shape.ItemComments +
		shape.CardComments + 2*shape.Attachments + 2*shape.Workstreams
	if files < floor {
		t.Errorf("the tree holds %d files, fewer than the %d the shape names", files, floor)
	}
	other := generate(t, perfstore.DefaultSeed+1)
	if other.Digest == first.Digest {
		t.Errorf("seeds %d and %d gave one digest %s", perfstore.DefaultSeed, perfstore.DefaultSeed+1, first.Digest)
	}
}

// TestGenerateRefusesTooFewItemComments asserts that a shape with fewer item
// comments than settled items is refused, beside the accepting case one
// comment higher, so a Generate refusing every shape cannot pass.
func TestGenerateRefusesTooFewItemComments(t *testing.T) {
	t.Parallel()
	shape := perfstore.SmallShape()
	settled := shape.Items - shape.PendingItems
	shape.ItemComments = settled - 1
	if _, err := perfstore.Generate(t.TempDir(), perfstore.DefaultSeed, shape); err == nil {
		t.Errorf("a shape with %d item comments for %d settled items was accepted", shape.ItemComments, settled)
	}
	shape.ItemComments = settled
	if _, err := perfstore.Generate(t.TempDir(), perfstore.DefaultSeed, shape); err != nil {
		t.Errorf("a shape with one item comment per settled item was refused: %v", err)
	}
}

// TestGeneratedStoreChecksClean asserts that the small store opens and that
// dinah check finds nothing in it.
func TestGeneratedStoreChecksClean(t *testing.T) {
	t.Parallel()
	store := generate(t, perfstore.DefaultSeed)
	b, err := bench.Open(store.Root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings, err := b.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		t.Errorf("finding %s %s at %s", finding.Key, finding.Detail, finding.Path)
	}
}

// admittedStates names the states each item kind can stand in: an acceptance
// criterion is verified or failed, a question or a decision is resolved, and
// every kind can be pending, waived or withdrawn.
var admittedStates = map[string]map[string]bool{
	"acceptance_criterion": {
		bench.ItemPending: true, bench.ItemVerified: true, bench.ItemFailed: true,
		bench.ItemWaived: true, bench.ItemWithdrawn: true,
	},
	"open_question": {
		bench.ItemPending: true, bench.ItemResolved: true, bench.ItemWaived: true, bench.ItemWithdrawn: true,
	},
	"decision": {
		bench.ItemPending: true, bench.ItemResolved: true, bench.ItemWaived: true, bench.ItemWithdrawn: true,
	},
}

// storeCounts is what the read-back test counts in a generated store.
type storeCounts struct {
	items, pending, comments, attachments int
}

// TestGeneratedStoreReadsBack asserts, through bench and verb alone, that the
// small store holds each count its shape names, that every column holds a
// live card, and the three properties dinah check does not reach: every
// archived card opens and its journal ends in an archived event, every item's
// state is one its kind admits, and each live card's journal carries exactly
// one creating event for each comment, item and attachment written on it.
func TestGeneratedStoreReadsBack(t *testing.T) {
	t.Parallel()
	store := generate(t, perfstore.DefaultSeed)
	shape := store.Shape
	b, err := bench.Open(store.Root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cards, err := b.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	if len(cards) != shape.Cards {
		t.Errorf("%d live cards, the shape names %d", len(cards), shape.Cards)
	}
	var counts storeCounts
	for _, card := range cards {
		readBackCard(t, card, &counts)
	}
	if counts.items != shape.Items {
		t.Errorf("%d items, the shape names %d", counts.items, shape.Items)
	}
	if counts.pending != shape.PendingItems {
		t.Errorf("%d pending items, the shape names %d", counts.pending, shape.PendingItems)
	}
	if want := shape.ItemComments + shape.CardComments; counts.comments != want {
		t.Errorf("%d comments, the shape names %d", counts.comments, want)
	}
	if counts.attachments != shape.Attachments {
		t.Errorf("%d attachments, the shape names %d", counts.attachments, shape.Attachments)
	}
	workstreams, err := b.Workstreams()
	if err != nil {
		t.Fatalf("workstreams: %v", err)
	}
	if len(workstreams) != shape.Workstreams {
		t.Errorf("%d workstreams, the shape names %d", len(workstreams), shape.Workstreams)
	}
	readBackArchive(t, b, shape.ArchivedCards)
	status, err := verb.New(b, t.TempDir()).Status(&verb.Request{Actor: "perf"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if len(status.Columns) != 14 {
		t.Errorf("status reports %d columns, the generated flow has 14", len(status.Columns))
	}
	for _, column := range status.Columns {
		if column.Count == 0 {
			t.Errorf("column %s holds no live card", column.Title)
		}
	}
}

// readBackCard adds one live card's items, comments and attachments to the
// running counts, and checks each item's state against its kind and the
// card's journal against what the card holds.
func readBackCard(t *testing.T, card *bench.Card, counts *storeCounts) {
	t.Helper()
	items, err := bench.Items(card.Dir)
	if err != nil {
		t.Fatalf("items of %s: %v", card.ID, err)
	}
	written := map[string]string{}
	cardComments, err := bench.Comments(card.Dir)
	if err != nil {
		t.Fatalf("comments of %s: %v", card.ID, err)
	}
	for _, comment := range cardComments {
		written[comment.ID] = contract.EventCommented
	}
	for _, item := range items {
		counts.items++
		if item.State == bench.ItemPending {
			counts.pending++
		}
		if !admittedStates[item.Kind][item.State] {
			t.Errorf("item %s of %s is a %s standing %s", item.ID, card.ID, item.Kind, item.State)
		}
		written[item.ID] = contract.EventItemFiled
		itemComments, err := bench.Comments(item.Dir)
		if err != nil {
			t.Fatalf("comments of item %s: %v", item.ID, err)
		}
		for _, comment := range itemComments {
			written[comment.ID] = contract.EventCommented
		}
	}
	attachments, err := bench.Attachments(card.Dir)
	if err != nil {
		t.Fatalf("attachments of %s: %v", card.ID, err)
	}
	for _, attachment := range attachments {
		written[attachment.ID] = contract.EventAttached
	}
	counts.attachments += len(attachments)
	counts.comments += countOf(written, contract.EventCommented)
	events, _, err := bench.ReadJournal(card.JournalPath())
	if err != nil {
		t.Fatalf("journal of %s: %v", card.ID, err)
	}
	journalled := map[string]int{}
	for _, ev := range events {
		switch ev.Event {
		case contract.EventCommented:
			journalled[ev.Comment]++
		case contract.EventItemFiled:
			journalled[ev.Item]++
		case contract.EventAttached:
			journalled[ev.Attachment]++
		}
	}
	for id, event := range written {
		if journalled[id] != 1 {
			t.Errorf("the journal of %s carries %d %s events for %s, wanted 1", card.ID, journalled[id], event, id)
		}
	}
	if len(journalled) != len(written) {
		t.Errorf("the journal of %s creates %d entities, the card holds %d", card.ID, len(journalled), len(written))
	}
}

// countOf counts the entries of a map carrying one value.
func countOf(written map[string]string, value string) int {
	count := 0
	for _, event := range written {
		if event == value {
			count++
		}
	}
	return count
}

// readBackArchive asserts that the archive holds the shape's archived cards,
// that each opens through bench, and that each journal ends in an archived
// event.
func readBackArchive(t *testing.T, b *bench.Bench, want int) {
	t.Helper()
	ids, err := bench.ListIDs(b.ArchivedCardsRoot())
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if len(ids) != want {
		t.Errorf("%d archived cards, the shape names %d", len(ids), want)
	}
	for _, id := range ids {
		card, err := b.LoadCardIn(b.ArchivedCardsRoot(), id)
		if err != nil {
			t.Errorf("archived card %s does not open: %v", id, err)
			continue
		}
		events, _, err := bench.ReadJournal(card.JournalPath())
		if err != nil || len(events) == 0 {
			t.Errorf("archived card %s has no readable journal: %v", id, err)
			continue
		}
		if last := events[len(events)-1].Event; last != contract.EventArchived {
			t.Errorf("the journal of archived card %s ends in %s, not %s", id, last, contract.EventArchived)
		}
	}
}

// TestPerfstoreIsNotInTheBinary asserts that no production binary links the
// generator, and that the generator imports nothing from this module beyond
// internal/bench and internal/contract, which is what lets an external test
// package of either import it.
func TestPerfstoreIsNotInTheBinary(t *testing.T) {
	t.Parallel()
	deps := goList(t, "-deps", "dinah/cmd/dinah")
	if !contains(deps, "dinah/internal/bench") {
		t.Fatalf("go list -deps dinah/cmd/dinah does not list dinah/internal/bench, so it listed nothing useful: %v", deps)
	}
	if contains(deps, "dinah/internal/perfstore") {
		t.Errorf("dinah/cmd/dinah depends on dinah/internal/perfstore")
	}
	imports := goList(t, "-f", "{{join .Imports \"\\n\"}}", "dinah/internal/perfstore")
	allowed := map[string]bool{"dinah/internal/bench": true, "dinah/internal/contract": true}
	moduleImports := 0
	for _, path := range imports {
		if !strings.HasPrefix(path, "dinah/") {
			continue
		}
		moduleImports++
		if !allowed[path] {
			t.Errorf("perfstore imports %s", path)
		}
	}
	if moduleImports == 0 {
		t.Errorf("go list reports no module import of perfstore, so it read nothing: %v", imports)
	}
}

// goList runs go list with the given arguments and answers the fields of its
// output.
func goList(t *testing.T, args ...string) []string {
	t.Helper()
	command := exec.Command("go", append([]string{"list"}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go list %v: %v\n%s", args, err, out)
	}
	return strings.Fields(string(out))
}

// contains reports whether a list holds a value.
func contains(list []string, value string) bool {
	for _, entry := range list {
		if entry == value {
			return true
		}
	}
	return false
}
