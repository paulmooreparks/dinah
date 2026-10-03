package verb

import (
	"os"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// newCardUnitHarness is newSerialHarness over a workbench instantiated at the
// card-unit format, which is what the switch has to be on for. The switch is
// process-global, so the test stays out of the parallel set, which runs only
// once every serial test has finished. It fails where the workbench did not
// come out at that format, because every assertion in a card-unit test is
// about a layout the store has to be in first.
func newCardUnitHarness(t *testing.T) *harness {
	t.Helper()
	bench.EnableCardUnitForTest(t)
	h := newSerialHarness(t)
	if h.library.Bench.Format != bench.CardUnitFormat {
		t.Fatalf("the harness opened a workbench at format %d, wanted the card-unit format %d", h.library.Bench.Format, bench.CardUnitFormat)
	}
	return h
}

// shownBody answers the body show prints for a comment reference, or the text
// it prints for an item reference, read from the anchor show composes.
func (h *harness) shownBody(ref string) string {
	h.t.Helper()
	_, _, item, text, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: ref})
	if err != nil {
		h.t.Fatalf("show %s: %v", ref, err)
	}
	if item != nil {
		text = item.Text
	}
	_, body := bench.ParseAnchor(text)
	return body
}

// lineOf answers the one journal line of a card carrying an event about a
// member, failing unless exactly one does.
func (h *harness) lineOf(ref, event string, about func(bench.Event) bool) bench.Event {
	h.t.Helper()
	var found []bench.Event
	for _, ev := range h.events(ref) {
		if ev.Event == event && about(ev) {
			found = append(found, ev)
		}
	}
	if len(found) != 1 {
		h.t.Fatalf("%s carries %d %s lines about the member, wanted one", ref, len(found), event)
	}
	return found[0]
}

// TestTheCardUnitJournalCarriesEveryText drives dinah-637/criteria/6. Every
// text a comment or an item is given lands on the journal line that gave it,
// and show answers the latest.
//
// Arming: dropping the text CompleteMemberLine sets on a comment_updated line
// reddens the second line's assertion and show's answer at once, since the
// replay reads the body from that line.
func TestTheCardUnitJournalCarriesEveryText(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.ready("a card whose members carry text")
	h.comment(card, "one")
	comment := card + "/comments/1"
	if response := h.set(comment, bench.BodyField, "two"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("set the comment's body: %s %s", response.Outcome, response.Refusal)
	}
	item := h.file(card, "acceptance_criterion", "a")
	if response := h.set(item, "text", "b"); response.Outcome != contract.OutcomeOK {
		t.Fatalf("set the item's text: %s %s", response.Outcome, response.Refusal)
	}

	commented := h.lineOf(card, contract.EventCommented, func(bench.Event) bool { return true })
	if commented.Text != "one" {
		t.Errorf("the commented line carries %q, wanted %q", commented.Text, "one")
	}
	updated := h.lineOf(card, contract.EventCommentUpdated, func(ev bench.Event) bool { return ev.Note == commented.Comment })
	if updated.Text != "two" {
		t.Errorf("the comment_updated line carries %q, wanted %q", updated.Text, "two")
	}
	filed := h.lineOf(card, contract.EventItemFiled, func(bench.Event) bool { return true })
	if filed.Text != "a" {
		t.Errorf("the item_filed line carries %q, wanted %q", filed.Text, "a")
	}
	changed := h.lineOf(card, contract.EventItemUpdated, func(ev bench.Event) bool { return ev.Note == filed.Item })
	if changed.Text != "b" {
		t.Errorf("the item_updated line carries %q, wanted %q", changed.Text, "b")
	}
	if got := h.shownBody(comment); got != "two" {
		t.Errorf("show answers the comment's body as %q, wanted %q", got, "two")
	}
	if got := strings.TrimRight(h.shownBody(item), "\n"); got != "b" {
		t.Errorf("show answers the item's text as %q, wanted %q", got, "b")
	}
	for _, dir := range []string{bench.CommentsDir, bench.ChecklistSegment} {
		if bench.Exists(h.card(card).Dir + string(os.PathSeparator) + dir) {
			t.Errorf("the card carries a %s directory, and the card-unit layout keeps its members in the journal", dir)
		}
	}
}

// TestResolveWithTextDesignatesTheCommentItWrote drives dinah-637/criteria/7.
//
// Arming: leaving Resolution off the settling line, which CompleteMemberLine
// fills from the anchor the act left, reddens the first and second
// assertions; the replay then reads the item as settled with no answer.
func TestResolveWithTextDesignatesTheCommentItWrote(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.ready("a card with a question")
	question := h.file(card, "open_question", "does the deadline move?")
	answered := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: question, Text: "it does not"})
	if answered.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve --text: %s %s", answered.Outcome, answered.Refusal)
	}
	h.reopen()
	events := h.events(card)
	var commented, resolved *bench.Event
	for i := range events {
		switch events[i].Event {
		case contract.EventCommented:
			commented = &events[i]
		case contract.EventItemResolved:
			resolved = &events[i]
		}
	}
	if commented == nil || resolved == nil {
		t.Fatalf("the journal carries commented %v and item_resolved %v, wanted both", commented != nil, resolved != nil)
	}
	if commented.Item == "" || commented.Item != resolved.Item {
		t.Errorf("the commented line names item %q and the settling line %q, wanted the one item on both", commented.Item, resolved.Item)
	}
	if resolved.Resolution != commented.Comment {
		t.Errorf("the settling line designates %q, wanted the comment the act wrote, %q", resolved.Resolution, commented.Comment)
	}
	if commented.TS != resolved.TS {
		t.Errorf("the two lines carry %s and %s, wanted one timestamp", commented.TS, resolved.TS)
	}

	// Naming an existing comment writes the settling line alone.
	second := h.file(card, "decision", "whose contract?")
	second = strings.TrimSuffix(second, "/1") + "/1"
	h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: second, Text: "ours"})
	h.reopen()
	named := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: second, Note: second + "/comments/1"})
	if named.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve naming a comment: %s %s", named.Outcome, named.Refusal)
	}
	h.reopen()
	var decisionComment string
	for _, ev := range h.events(card) {
		if ev.Event == contract.EventCommented && ev.Text == "ours" {
			decisionComment = ev.Comment
		}
	}
	settled := 0
	for _, ev := range h.events(card) {
		if ev.Event == contract.EventItemResolved && ev.Resolution == decisionComment {
			settled++
		}
	}
	if decisionComment == "" || settled != 1 {
		t.Errorf("resolving by reference wrote %d settling lines designating %q, wanted one", settled, decisionComment)
	}

	reopened := h.library.Reopen(&Request{Verb: "reopen", Actor: "alka", Ref: question, Reason: "the date moved after all"})
	if reopened.Outcome != contract.OutcomeOK {
		t.Fatalf("reopen: %s %s", reopened.Outcome, reopened.Refusal)
	}
	h.reopen()
	record, err := h.library.Bench.LoadCardRecord(h.card(card))
	if err != nil {
		t.Fatalf("read the card's record: %v", err)
	}
	item, ok := record.Item(resolved.Item)
	if !ok {
		t.Fatalf("the reopened item is gone from the record")
	}
	if item.State != bench.ItemPending || item.Resolution != "" {
		t.Errorf("after reopen the item stands %s designating %q, wanted pending with no designation", item.State, item.Resolution)
	}
}

// TestAMebibyteCommentRoundTripsWithItsMarkup drives dinah-637/criteria/14.
//
// Arming: dropping SetEscapeHTML(false) from EncodeEvent leaves the round trip
// green, since both spellings decode alike, and reddens the literal-character
// assertion on the journal's bytes.
func TestAMebibyteCommentRoundTripsWithItsMarkup(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.ready("a card carrying a long comment")
	unit := "<b>fish & chips</b> "
	var built strings.Builder
	for built.Len() < 1<<20 {
		built.WriteString(unit)
	}
	body := built.String()
	h.comment(card, body)
	if got := h.shownBody(card + "/comments/1"); got != body {
		t.Fatalf("show answers %d bytes, wanted the %d written", len(got), len(body))
	}
	raw, err := os.ReadFile(h.card(card).JournalPath())
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	if !strings.Contains(string(raw), unit) {
		t.Error("the journal line does not carry the markup as itself")
	}
	if strings.Contains(string(raw), "\\"+"u003c") {
		t.Error("the journal line escapes < as \\u003c")
	}
}

// mustOK fails the test unless a response is ok.
func mustOK(t *testing.T, what string, response *Response) {
	t.Helper()
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("%s: %s %s %q", what, response.Outcome, response.Refusal, response.Detail)
	}
}

// TestEveryMemberActWorksInTheCardUnitLayout walks every act on a comment or
// an item through a card-unit store and asks what the reads answer after
// each, so an act that writes a line the replay cannot read, or a read that
// still looks for a directory, fails here by name.
func TestEveryMemberActWorksInTheCardUnitLayout(t *testing.T) {
	h := newCardUnitHarness(t)
	h.declareEvidence(evidenceBlock)
	card := h.ready("a card every member act reaches")
	h.comment(card, "a remark on the card with the word pelican")
	criterion := h.file(card, "acceptance_criterion", "the endpoint answers 404")
	question := h.file(card, "open_question", "who signs off?")
	decision := h.file(card, "decision", "which vendor?")
	mustOK(t, "comment on the criterion", h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: criterion, Text: "measured on staging"}))
	mustOK(t, "comment on the column", h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: "review", Text: "a remark on the station"}))
	h.reopen()
	h.attach(card+"/comments/1", "evidence.txt", "the evidence bytes")

	path, err := h.library.Bench.ResolvePath(card + "/comments/1/attachments/1/payload")
	if err != nil {
		t.Fatalf("path of the comment's attachment payload: %v", err)
	}
	if bytes, err := os.ReadFile(path); err != nil || string(bytes) != "the evidence bytes" {
		t.Errorf("the payload path answers %q (%v), wanted the attached bytes", bytes, err)
	}
	for _, ref := range []string{card + "/comments/1", criterion} {
		if _, err := h.library.Bench.ResolvePath(ref); err == nil || !strings.Contains(err.Error(), contract.NotAFile) {
			t.Errorf("path %s answered %v, wanted %s", ref, err, contract.NotAFile)
		}
	}

	mustOK(t, "cite", h.library.Cite(&Request{Verb: "cite", Actor: "alka", Ref: criterion, Scheme: "test", CiteTarget: "a_test.go#TestIt", Observed: "fail:pass"}))
	mustOK(t, "verify", h.library.Verify(&Request{Verb: "verify", Actor: "alka", Ref: criterion, Text: "the test passes"}))
	mustOK(t, "waive", h.library.Waive(&Request{Verb: "waive", Actor: "alka", Ref: question, Text: "nobody needs to"}))
	mustOK(t, "resolve", h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: decision, Text: "the cheaper one"}))
	h.reopen()

	record, err := h.library.Bench.LoadCardRecord(h.card(card))
	if err != nil {
		t.Fatalf("read the card's record: %v", err)
	}
	items := record.ItemsIn(bench.LiveHalf, "")
	if len(items) != 3 {
		t.Fatalf("the record holds %d live items, wanted 3", len(items))
	}
	states := map[string]string{}
	for _, item := range items {
		states[item.Kind] = item.State
		if item.Resolution == "" {
			t.Errorf("the %s stands %s with no answer of record", item.Kind, item.State)
		}
	}
	if states["acceptance_criterion"] != bench.ItemVerified || states["open_question"] != bench.ItemWaived || states["decision"] != bench.ItemResolved {
		t.Errorf("the items stand %v, wanted verified, waived and resolved", states)
	}
	if cited := items[0].Citations; len(cited) != 1 || cited[0].Scheme != "test" {
		t.Errorf("the criterion carries the citations %+v, wanted the one cited", cited)
	}

	view := h.cardView(card)
	if view.ChildCount == 0 {
		t.Errorf("the card view counts no member below the card")
	}
	results, err := h.library.Search(&Request{Verb: "search", Actor: "alka", SearchText: "pelican"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if results.Count != 1 {
		t.Errorf("a search for a word only a comment carries drew %d hits, wanted the card", results.Count)
	}

	mustOK(t, "archive the card comment", h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: card + "/comments/1"}))
	h.reopen()
	if _, _, _, live, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card + "/comments/1"}); err == nil {
		t.Errorf("the archived comment still answers at comments/1 in the live half: %q", live)
	}
	_, _, _, archived, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card + "/comments/1", Archived: true})
	if err != nil || !strings.Contains(archived, "pelican") {
		t.Errorf("show --archived comments/1 answered %q (%v), wanted the archived comment", archived, err)
	}
	mustOK(t, "restore the card comment", h.library.Restore(&Request{Verb: "restore", Actor: "alka", Ref: card + "/comments/1"}))
	h.reopen()
	if got := h.shownBody(card + "/comments/1"); !strings.Contains(got, "pelican") {
		t.Errorf("the restored comment answers %q at comments/1", got)
	}

	mustOK(t, "archive the decision", h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: decision}))
	h.reopen()
	mustOK(t, "delete the question", h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: question, Confirm: true}))
	h.reopen()
	record, err = h.library.Bench.LoadCardRecord(h.card(card))
	if err != nil {
		t.Fatalf("read the card's record: %v", err)
	}
	if live, gone := len(record.ItemsIn(bench.LiveHalf, "")), len(record.ItemsIn(bench.ArchivedHalf, "")); live != 1 || gone != 1 {
		t.Errorf("after an archive and a deletion the record holds %d live and %d archived items, wanted 1 and 1", live, gone)
	}

	h.mustDo(&Request{Verb: Block, Card: card, Actor: "alka", Reason: "the supplier went quiet"})
	h.mustDo(&Request{Verb: Unblock, Card: card, Actor: "alka", Reason: "the supplier answered"})
	unblocked := h.lineOf(card, contract.EventUnblocked, func(bench.Event) bool { return true })
	if unblocked.Comment == "" {
		t.Fatalf("the unblock carried a reason and minted no comment")
	}
	record, err = h.library.Bench.LoadCardRecord(h.card(card))
	if err != nil {
		t.Fatalf("read the card's record: %v", err)
	}
	if minted, ok := record.Comment(unblocked.Comment); !ok || minted.Body != "the supplier answered" {
		t.Errorf("the comment the unblock minted reads %+v, wanted the reason", minted)
	}

	comments, err := h.library.Bench.ColumnComments(h.library.Bench.ColumnByRef("review").ID, bench.LiveHalf)
	if err != nil || len(comments) != 1 || comments[0].Body != "a remark on the station" {
		t.Errorf("the column answers comments %+v (%v), wanted the one written", comments, err)
	}
	if bench.Exists(h.card(card).Dir + string(os.PathSeparator) + bench.ChecklistSegment) {
		t.Errorf("an act wrote a checklist directory into a card-unit card")
	}
	for _, finding := range h.check() {
		if finding.Severity != bench.SeverityCleanup {
			t.Errorf("dinah check reports %s %s on a store every act wrote through a verb", finding.Key, finding.Detail)
		}
	}
}

// TestAnOrdinalIsNeverReissuedAfterADeletion drives the first half of
// dinah-637/criteria/15.
//
// Arming: counting NextOrdinal from the live members instead of every member
// the journal ever recorded reissues the deleted ordinal and reddens this.
func TestAnOrdinalIsNeverReissuedAfterADeletion(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.ready("a card losing its last comment")
	h.comment(card, "first")
	h.comment(card, "second")
	deleted := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: card + "/comments/2", Confirm: true})
	if deleted.Outcome != contract.OutcomeOK {
		t.Fatalf("delete the second comment: %s %s", deleted.Outcome, deleted.Refusal)
	}
	h.reopen()
	h.comment(card, "third")
	third := h.lineOf(card, contract.EventCommented, func(ev bench.Event) bool { return ev.Text == "third" })
	if third.Ordinal != 3 {
		t.Errorf("the comment written after the deletion carries ordinal %d, wanted 3", third.Ordinal)
	}
	if got := h.shownBody(card + "/comments/2"); got != "third" {
		t.Errorf("comments/2 answers %q after the deletion, wanted the comment written after it", got)
	}
}
