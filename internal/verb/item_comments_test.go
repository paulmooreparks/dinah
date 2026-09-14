package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestCommentOnAnItemWritesUnderTheItemsOwnCollection asserts dinah-502
// AC-1. An item mounts its own comments collection, so a comment on an item
// and a comment on the card land in two separate collections rather than one
// holding two.
func TestCommentOnAnItemWritesUnderTheItemsOwnCollection(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card carrying a question")
	item := h.file(ref, "open_question", "does the deadline move?")

	response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: item, Text: "reasoning"})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment on the item: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	h.comment(ref, "card text")

	itemDir := h.card(ref).Dir
	itemComments, err := bench.Comments(filepath.Join(itemDir, bench.ChecklistDir, itemID(t, itemDir)))
	if err != nil {
		t.Fatalf("read the item's comments: %v", err)
	}
	if len(itemComments) != 1 {
		t.Fatalf("the item's comments collection holds %d, wanted 1", len(itemComments))
	}
	if itemComments[0].Body != "reasoning" {
		t.Errorf("the item's comment reads %q, wanted %q", itemComments[0].Body, "reasoning")
	}
	cardComments, err := bench.Comments(itemDir)
	if err != nil {
		t.Fatalf("read the card's comments: %v", err)
	}
	if len(cardComments) != 1 {
		t.Fatalf("the card's comments collection holds %d, wanted 1", len(cardComments))
	}
	if cardComments[0].Body != "card text" {
		t.Errorf("the card's comment reads %q, wanted %q", cardComments[0].Body, "card text")
	}
	anchor, err := os.ReadFile(filepath.Join(itemDir, bench.ChecklistDir, itemID(t, itemDir), bench.CommentsDir, itemComments[0].ID, bench.CommentAnchor))
	if err != nil {
		t.Fatalf("read the comment anchor: %v", err)
	}
	fm, _ := bench.ParseAnchor(string(anchor))
	for _, key := range []string{"ts", "author", bench.OrdinalField} {
		if fm.Value(key) == "" {
			t.Errorf("the comment anchor carries no %s", key)
		}
	}
}

// itemID reads the one checklist item id a fixture card carries, which is
// how a test that planted exactly one item finds its directory name.
func itemID(t *testing.T, cardDir string) string {
	t.Helper()
	ids, err := bench.ListIDs(filepath.Join(cardDir, bench.ChecklistDir))
	if err != nil {
		t.Fatalf("list checklist ids: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("wanted exactly one checklist item, found %d", len(ids))
	}
	return ids[0]
}

// TestAnItemCommentsReferenceComposesAndResolvesBothWays asserts dinah-502
// AC-2. dinah path answers a comment's own address, show prints the same
// address back, the card's own comments are unaffected, and a position the
// item does not carry is refused dinah.unknown-path.
func TestAnItemCommentsReferenceComposesAndResolvesBothWays(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card carrying a question")
	item := h.file(ref, "open_question", "does the deadline move?")
	h.comment(item, "reasoning")
	h.comment(ref, "card text")

	commentRef := item + "/comments/1"
	if _, err := h.library.Bench.ResolvePath(commentRef); err != nil {
		t.Fatalf("path %s: %v", commentRef, err)
	}

	detail, _, itemDetail, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: item})
	if err != nil {
		t.Fatalf("show %s: %v", item, err)
	}
	if detail != nil {
		t.Fatalf("show %s answered a card detail rather than an item detail", item)
	}
	if itemDetail == nil || len(itemDetail.Comments) != 1 {
		t.Fatalf("show %s carried %v, wanted one comment", item, itemDetail)
	}
	if itemDetail.Comments[0].Ref != commentRef {
		t.Errorf("the comment's printed reference is %q, wanted %q", itemDetail.Comments[0].Ref, commentRef)
	}

	cardDetail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		t.Fatalf("show %s: %v", ref, err)
	}
	if len(cardDetail.Comments) != 1 || cardDetail.Comments[0].Ref != ref+"/comments/1" {
		t.Fatalf("the card's own first comment is unaffected, got %+v", cardDetail.Comments)
	}

	if _, err := h.library.Bench.ResolvePath(item + "/comments/2"); err == nil {
		t.Fatal("a second comment position on an item carrying one was resolved rather than refused")
	} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnknownPath {
		t.Fatalf("wanted %s, got %v", contract.UnknownPath, err)
	}
}

// TestCommentRefusesByNameAndAdmitsByName asserts dinah-502 AC-3. A comment
// mounts on a card and on an item, and is refused by name on every other
// kind the containment grammar declares plus on a reference that resolves to
// nothing at all.
func TestCommentRefusesByNameAndAdmitsByName(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card")
	item := h.file(ref, "open_question", "a question")
	h.comment(ref, "a first remark")

	admits := []string{ref, item}
	for _, target := range admits {
		response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: target, Text: "text"})
		if response.Outcome != contract.OutcomeOK {
			t.Errorf("comment on %s: wanted ok, got %s %s", target, response.Outcome, response.Refusal)
		}
	}
	h.reopen()

	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("bytes"), 0o644); err != nil {
		t.Fatalf("write attachment source: %v", err)
	}
	attachResponse := h.library.Attach(&Request{Verb: "attach", Actor: "alka", Ref: ref, File: source})
	if attachResponse.Outcome != contract.OutcomeOK {
		t.Fatalf("attach: %s %s", attachResponse.Outcome, attachResponse.Refusal)
	}
	h.reopen()

	refuses := []struct {
		name string
		ref  string
		kind string
	}{
		{"a comment", ref + "/comments/1", bench.KindComment},
		{"an attachment", ref + "/attachments/1", bench.KindAttachment},
		{"a column", "intake", bench.KindColumn},
		{"the workbench", "workbench", bench.KindWorkbench},
	}
	for _, c := range refuses {
		t.Run(c.name, func(t *testing.T) {
			response := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: c.ref, Text: "text"})
			if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.NotCommentable {
				t.Fatalf("comment on %s: wanted %s, got %s %s", c.ref, contract.NotCommentable, response.Outcome, response.Refusal)
			}
			if response.Context["kind"] != c.kind {
				t.Errorf("the refusal carries kind %q, wanted %q", response.Context["kind"], c.kind)
			}
		})
	}

	bare := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: "", Text: "text"})
	if bare.Outcome != contract.OutcomeRefused || bare.Refusal != contract.UnknownCard {
		t.Fatalf("comment on a bare reference: wanted %s, got %s %s", contract.UnknownCard, bare.Outcome, bare.Refusal)
	}

	nothing := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: ref + "/questions/99", Text: "text"})
	if nothing.Outcome != contract.OutcomeRefused || nothing.Refusal != contract.UnknownPath {
		t.Fatalf("comment on a reference reaching nothing: wanted %s, got %s %s", contract.UnknownPath, nothing.Outcome, nothing.Refusal)
	}
}

// TestTheJournalTellsAnItemCommentFromACardComment asserts dinah-502 AC-4.
// An item comment's line carries the item's identifier beside the comment's
// own, and a card comment's line carries no item member at all.
func TestTheJournalTellsAnItemCommentFromACardComment(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card carrying a question")
	item := h.file(ref, "open_question", "does the deadline move?")

	events, _, err := bench.ReadJournal(h.card(ref).JournalPath())
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	before := len(events)

	itemResponse := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: item, Text: "reasoning"})
	if itemResponse.Outcome != contract.OutcomeOK {
		t.Fatalf("comment on the item: %s %s", itemResponse.Outcome, itemResponse.Refusal)
	}
	h.reopen()
	cardResponse := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: ref, Text: "card text"})
	if cardResponse.Outcome != contract.OutcomeOK {
		t.Fatalf("comment on the card: %s %s", cardResponse.Outcome, cardResponse.Refusal)
	}
	h.reopen()

	events, _, err = bench.ReadJournal(h.card(ref).JournalPath())
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	if len(events) != before+2 {
		t.Fatalf("the journal grew by %d lines, wanted 2", len(events)-before)
	}
	itemLine := events[before]
	if itemLine.Event != contract.EventCommented || itemLine.Comment == "" || itemLine.Item == "" {
		t.Fatalf("the item comment's line is %+v, wanted commented carrying comment and item", itemLine)
	}
	cardLine := events[before+1]
	if cardLine.Event != contract.EventCommented || cardLine.Comment == "" || cardLine.Item != "" {
		t.Fatalf("the card comment's line is %+v, wanted commented carrying comment and no item", cardLine)
	}
}

// TestShowOnAnItemPrintsTheItemThenItsComments asserts dinah-502 AC-5. An
// item carrying no comments prints byte for byte what show printed for it
// before item comments existed, and an item carrying comments prints that
// same text followed by the comments in ordinal order.
func TestShowOnAnItemPrintsTheItemThenItsComments(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card carrying a question")
	item := h.file(ref, "open_question", "does the deadline move?")

	baseline, err := bench.ReadText(filepath.Join(h.card(ref).Dir, bench.ChecklistDir, itemID(t, h.card(ref).Dir), bench.ItemAnchor))
	if err != nil {
		t.Fatalf("read the item anchor: %v", err)
	}

	_, _, empty, text, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: item})
	if err != nil {
		t.Fatalf("show %s: %v", item, err)
	}
	if text != "" {
		t.Fatalf("show %s answered text rather than an item detail: %q", item, text)
	}
	if empty == nil || empty.Text != baseline || len(empty.Comments) != 0 {
		t.Fatalf("an item carrying no comments answered %+v, wanted the baseline text and no comments", empty)
	}

	h.comment(item, "first")
	h.comment(item, "second")

	_, _, full, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: item})
	if err != nil {
		t.Fatalf("show %s: %v", item, err)
	}
	if full == nil || full.Text != baseline {
		t.Fatalf("an item carrying comments changed its own text: %+v", full)
	}
	if len(full.Comments) != 2 {
		t.Fatalf("wanted two comments, got %d", len(full.Comments))
	}
	if full.Comments[0].Body != "first" || full.Comments[1].Body != "second" {
		t.Fatalf("the comments are not in ordinal order: %+v", full.Comments)
	}
	for i, comment := range full.Comments {
		if comment.Ref != item+"/comments/"+strconv.Itoa(i+1) {
			t.Errorf("comment %d's reference is %q, wanted it spelled against the item", i, comment.Ref)
		}
	}

	encoded, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, encoded)
	}
	if _, ok := payload["comments"]; !ok {
		t.Errorf("the payload does not carry comments: %s", encoded)
	}
}

// TestTheChecklistTableCountsRatherThanQuotes asserts dinah-502 AC-6. The
// checklist payload carries a comment count on an item that carries
// comments and omits the member on one that carries none.
func TestTheChecklistTableCountsRatherThanQuotes(t *testing.T) {
	h := newHarness(t)
	ref := h.add("a card carrying two questions")
	first := h.file(ref, "open_question", "the first question")
	h.file(ref, "open_question", "the second question")
	h.comment(first, "one")
	h.comment(first, "two")
	h.comment(first, "three")

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		t.Fatalf("show %s: %v", ref, err)
	}
	if len(detail.Checklist) != 2 {
		t.Fatalf("wanted two checklist items, got %d", len(detail.Checklist))
	}
	if detail.Checklist[0].CommentCount != 3 {
		t.Errorf("the first item's comment count is %d, wanted 3", detail.Checklist[0].CommentCount)
	}
	if detail.Checklist[1].CommentCount != 0 {
		t.Errorf("the second item's comment count is %d, wanted 0", detail.Checklist[1].CommentCount)
	}

	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload struct {
		Checklist []map[string]json.RawMessage `json:"checklist"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, encoded)
	}
	if _, ok := payload.Checklist[0]["comment_count"]; !ok {
		t.Errorf("the first item's payload carries no comment_count: %s", encoded)
	}
	if _, ok := payload.Checklist[1]["comment_count"]; ok {
		t.Errorf("the second item's payload carries comment_count though it holds none: %s", encoded)
	}
}

// TestACloseLeavesTheArgumentStanding asserts dinah-502 AC-7. Resolving,
// verifying and failing an item write the note as they always have, and the
// comment written on the item beforehand is unaffected.
func TestACloseLeavesTheArgumentStanding(t *testing.T) {
	// Each item stands on its own card, because the harness's file helper
	// always answers the first position of its kind, which two items of one
	// kind on one card would collide on.
	h := newHarness(t)
	questionCard := h.add("a card carrying a question")
	question := h.file(questionCard, "open_question", "does the deadline move?")
	h.comment(question, "reasoning")
	resolve := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: question, Note: "answer"})
	if resolve.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", resolve.Outcome, resolve.Refusal)
	}
	h.reopen()
	assertNoteAndComment(t, h, question, "answer", "reasoning")

	criterionCard := h.add("a card carrying a criterion to verify")
	criterion := h.file(criterionCard, "acceptance_criterion", "the endpoint returns 404")
	h.comment(criterion, "verification plan")
	verify := h.library.Verify(&Request{Verb: "verify", Actor: "alka", Ref: criterion, Note: "verified"})
	if verify.Outcome != contract.OutcomeOK {
		t.Fatalf("verify: %s %s", verify.Outcome, verify.Refusal)
	}
	h.reopen()
	assertNoteAndComment(t, h, criterion, "verified", "verification plan")

	failingCard := h.add("a card carrying a criterion to fail")
	failing := h.file(failingCard, "acceptance_criterion", "the endpoint returns 500")
	h.comment(failing, "why it fails")
	fail := h.library.Fail(&Request{Verb: "fail", Actor: "alka", Ref: failing, Note: "failed"})
	if fail.Outcome != contract.OutcomeOK {
		t.Fatalf("fail: %s %s", fail.Outcome, fail.Refusal)
	}
	h.reopen()
	assertNoteAndComment(t, h, failing, "failed", "why it fails")

	// An item resolved with no comment on it carries the same note and the
	// same absent comments collection as before this change.
	quietCard := h.add("a card carrying a question with no argument")
	quiet := h.file(quietCard, "open_question", "a question with no argument")
	settle := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: quiet, Note: "settled"})
	if settle.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", settle.Outcome, settle.Refusal)
	}
	h.reopen()
	_, _, itemDetail, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: quiet})
	if err != nil {
		t.Fatalf("show %s: %v", quiet, err)
	}
	if itemDetail == nil || len(itemDetail.Comments) != 0 {
		t.Fatalf("an item resolved with no comment carries comments: %+v", itemDetail)
	}
}

func assertNoteAndComment(t *testing.T, h *harness, item, wantNote, wantComment string) {
	t.Helper()
	_, _, itemDetail, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: item})
	if err != nil {
		t.Fatalf("show %s: %v", item, err)
	}
	if itemDetail == nil || len(itemDetail.Comments) != 1 || itemDetail.Comments[0].Body != wantComment {
		t.Fatalf("show %s carried %+v, wanted the comment %q standing", item, itemDetail, wantComment)
	}
	entity, err := h.library.Bench.ResolveEntity(item)
	if err != nil {
		t.Fatalf("resolve %s: %v", item, err)
	}
	stored, err := bench.LoadItem(entity.Dir)
	if err != nil {
		t.Fatalf("load item %s: %v", item, err)
	}
	if stored.Note != wantNote {
		t.Errorf("the item's note reads %q, wanted %q", stored.Note, wantNote)
	}
}
