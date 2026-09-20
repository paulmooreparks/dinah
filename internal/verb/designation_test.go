package verb

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// commentDirOf resolves a comment reference to the directory it names, which
// is what a test asserting about a comment's own file needs.
func commentDirOf(t *testing.T, h *harness, ref string) string {
	t.Helper()
	entity, err := h.library.Bench.ResolveEntity(ref)
	if err != nil {
		t.Fatalf("resolve %s: %v", ref, err)
	}
	if entity.Kind != bench.KindComment {
		t.Fatalf("%s resolves to a %s rather than to a comment", ref, entity.Kind)
	}
	return entity.Dir
}

// handEdit rewrites a comment's body underneath the tool, which is what a
// person with an editor does and what the digest exists to make visible. It
// writes the file rather than going through a verb, because a verb write is
// the thing it is standing in contrast to.
func handEdit(t *testing.T, dir, body string) {
	t.Helper()
	fm, _, err := bench.ReadCommentAnchor(dir)
	if err != nil {
		t.Fatalf("read the comment: %v", err)
	}
	if err := bench.WriteText(filepath.Join(dir, bench.CommentAnchor), fm.Render(body)); err != nil {
		t.Fatalf("hand-edit the comment: %v", err)
	}
}

// storedDigest reads the digest a comment's anchor carries.
func storedDigest(t *testing.T, dir string) string {
	t.Helper()
	fm, _, err := bench.ReadCommentAnchor(dir)
	if err != nil {
		t.Fatalf("read the comment: %v", err)
	}
	return fm.Value(bench.CommentDigestField)
}

// TestTheEmptyCommentFormMintsAnEntityWithNoBody asserts
// dinah-525/criteria/1: `dinah comment <ref>` with no text mints a comment
// with an empty body, journals commented, and answers with a reference that
// resolves to the comment it made, while the form carrying text is unchanged.
//
// Abandoning the draft is asserted by state rather than by narration: after
// the delete the comment's directory is gone, the holder's comment count is
// back to what it was, and the journal carries both the create and the delete.
func TestTheEmptyCommentFormMintsAnEntityWithNoBody(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card to comment on")
	before, err := bench.CountComments(h.card(card).Dir)
	if err != nil {
		t.Fatalf("count the comments: %v", err)
	}

	empty := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: card})
	if empty.Outcome != contract.OutcomeOK {
		t.Fatalf("the empty form: %s %s", empty.Outcome, empty.Refusal)
	}
	h.reopen()
	// The answer carries a reference, and the assertion is that it resolves
	// to the comment that was just made. Asserting that it equals the
	// comment's directory name would pass against an identifier, which is
	// what this used to answer and which resolves to nothing: an editor
	// handed one asks `dinah path` for the file and is refused.
	ref := card + "/" + bench.CommentsDir + "/1"
	dir := commentDirOf(t, h, ref)
	if empty.Detail != ref {
		t.Errorf("the answer named %q, wanted the reference %q", empty.Detail, ref)
	}
	if reached := commentDirOf(t, h, empty.Detail); reached != dir {
		t.Errorf("the reference the answer carried reaches %s, wanted %s", reached, dir)
	}
	_, body, err := bench.ReadCommentAnchor(dir)
	if err != nil {
		t.Fatalf("read the minted comment: %v", err)
	}
	if body != "" {
		t.Errorf("the empty form wrote the body %q", body)
	}

	// The one-shot form keeps its present behaviour.
	full := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: card, Text: "a remark"})
	if full.Outcome != contract.OutcomeOK {
		t.Fatalf("the one-shot form: %s %s", full.Outcome, full.Refusal)
	}
	h.reopen()
	comments, err := bench.Comments(h.card(card).Dir)
	if err != nil {
		t.Fatalf("read the comments: %v", err)
	}
	if len(comments) != before+2 {
		t.Fatalf("the card carries %d comments, wanted %d", len(comments), before+2)
	}
	if comments[1].Body != "a remark" {
		t.Errorf("the one-shot form wrote %q", comments[1].Body)
	}

	created := 0
	for _, event := range h.events(card) {
		if event.Event == contract.EventCommented {
			created++
		}
	}
	if created != 2 {
		t.Errorf("the journal carries %d commented lines, wanted one per form", created)
	}

	// Abandoning it.
	removed := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: ref, Confirm: true})
	if removed.Outcome != contract.OutcomeOK {
		t.Fatalf("delete the abandoned draft: %s %s", removed.Outcome, removed.Refusal)
	}
	h.reopen()
	if bench.Exists(dir) {
		t.Errorf("the deleted comment's directory is still at %s", dir)
	}
	after, err := bench.CountComments(h.card(card).Dir)
	if err != nil {
		t.Fatalf("recount the comments: %v", err)
	}
	if after != before+1 {
		t.Errorf("the card carries %d comments after the delete, wanted the one the other form wrote", after)
	}
	deletes := 0
	for _, event := range h.events(card) {
		if event.Event == contract.EventDeleted {
			deletes++
		}
	}
	if deletes != 1 {
		t.Errorf("the journal carries %d deleted lines, wanted one", deletes)
	}
}

// TestEveryWriteOfACommentAnchorRecordsTheDigest asserts
// dinah-525/criteria/2: the digest follows a body rewrite, and a write that
// touches a front-matter key alone leaves it recomputed and unchanged, with
// check reporting nothing.
//
// The second half is the one that matters. Rendering re-serialises the whole
// file, so a write touching one header key rewrites the body's bytes on the
// way past; a digest recorded only on a body write would then disagree with a
// body nobody edited, and the first thing this feature did would be to report
// a divergence that never happened.
func TestEveryWriteOfACommentAnchorRecordsTheDigest(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card to comment on")
	h.comment(card, "the first thought")
	ref := card + "/" + bench.CommentsDir + "/1"
	dir := commentDirOf(t, h, ref)

	first := storedDigest(t, dir)
	if first != bench.CommentDigest("the first thought") {
		t.Errorf("the digest is %q and the body hashes to %q", first, bench.CommentDigest("the first thought"))
	}

	rewritten := h.library.SetField(&Request{
		Verb: "set", Actor: "alka", Ref: ref, Field: bench.BodyField, Value: "the second thought",
	})
	if rewritten.Outcome != contract.OutcomeOK {
		t.Fatalf("rewrite the body: %s %s", rewritten.Outcome, rewritten.Refusal)
	}
	h.reopen()
	if got := storedDigest(t, dir); got != bench.CommentDigest("the second thought") {
		t.Errorf("the digest did not follow the body: %q", got)
	}

	// A write of a front-matter key alone. accept-divergence writes the
	// header and leaves the body where it stands, which is the shape this
	// half needs: the digest is recomputed and comes back the same, and the
	// check that reads it reports nothing.
	was := storedDigest(t, dir)
	accepted := h.library.AcceptDivergence(&Request{Verb: "accept-divergence", Actor: "alka", Ref: ref})
	if accepted.Outcome != contract.OutcomeOK {
		t.Fatalf("accept-divergence: %s %s", accepted.Outcome, accepted.Refusal)
	}
	h.reopen()
	if got := storedDigest(t, dir); got != was {
		t.Errorf("a write touching no body moved the digest from %q to %q", was, got)
	}
	if detail, found := finding(h.check(), bench.FindingCommentBodyDiverged); found {
		t.Errorf("check reports %s as diverged after a header-only write", detail)
	}
}

// TestCheckReportsAHandEditedBody asserts dinah-525/criteria/3: a body edited
// underneath the tool is reported as check.comment-body-diverged at defect
// severity, naming the comment, and a comment carrying no digest is not
// reported at all.
//
// Absence is not divergence. Every comment written before this card carries no
// digest, and reporting those would make the finding useless on the day it
// arrived.
func TestCheckReportsAHandEditedBody(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card to comment on")
	h.comment(card, "what the verb wrote")
	reference := card + "/" + bench.CommentsDir + "/1"
	dir := commentDirOf(t, h, reference)
	handEdit(t, dir, "what somebody typed instead")
	h.reopen()

	findings := h.check()
	detail, found := finding(findings, bench.FindingCommentBodyDiverged)
	if !found {
		t.Fatalf("check reports no divergence over a hand-edited body: %+v", findings)
	}
	// The finding names the reference rather than the identifier, because a
	// finding names what a reader can type: the identifier alone resolves to
	// nothing, so a finding carrying one told a reader which file was wrong
	// in a spelling they could not use to open it.
	if detail != reference {
		t.Errorf("the finding names %q, wanted the reference %q", detail, reference)
	}
	for _, f := range findings {
		if f.Key == bench.FindingCommentBodyDiverged && bench.SeverityOf(f) != bench.SeverityDefect {
			t.Errorf("the finding carries severity %q, wanted defect", bench.SeverityOf(f))
		}
	}

	// A comment carrying no digest is every comment written before this
	// card, and none of them is diverged.
	fm, body, err := bench.ReadCommentAnchor(dir)
	if err != nil {
		t.Fatalf("read the comment: %v", err)
	}
	fm.Delete(bench.CommentDigestField)
	if err := bench.WriteText(filepath.Join(dir, bench.CommentAnchor), fm.Render(body)); err != nil {
		t.Fatalf("strip the digest: %v", err)
	}
	h.reopen()
	if detail, found := finding(h.check(), bench.FindingCommentBodyDiverged); found {
		t.Errorf("check reports %s, and a comment carrying no digest is not diverged", detail)
	}
}

// TestEditClaimsNothingAboutWhenTheEditorReturned asserts
// dinah-525/criteria/4: on the editor's return the body is compared against
// the recorded digest, and an unchanged body journals nothing while a changed
// one records the new digest and journals comment_updated.
//
// The unchanged case is the arm that matters. It is the answer both when the
// author changed nothing and when the editor returned before the author
// started, and the tool cannot tell those apart, so an implementation
// asserting that the editor waited records an attributed edit there and fails.
func TestEditClaimsNothingAboutWhenTheEditorReturned(t *testing.T) {
	t.Run("the editor returned without touching the file", func(t *testing.T) {
		h := newHarness(t)
		card := h.add("a card to comment on")
		h.comment(card, "what the author wrote")
		ref := card + "/" + bench.CommentsDir + "/1"
		dir := commentDirOf(t, h, ref)
		before := len(h.events(card))
		digest := storedDigest(t, dir)

		// The editor is the one that returns at once, which is every GUI
		// editor handing the file to an already-running instance.
		answer := h.library.RecordCommentEdit(&Request{
			Verb: "edit", Actor: "alka", Ref: ref, PriorDigest: bench.CommentDigest("what the author wrote"),
		})
		if answer.Outcome != contract.OutcomeOK {
			t.Fatalf("the return: %s %s", answer.Outcome, answer.Refusal)
		}
		h.reopen()
		if got := len(h.events(card)); got != before {
			t.Errorf("the journal grew by %d lines over an edit that did not happen", got-before)
		}
		if got := storedDigest(t, dir); got != digest {
			t.Errorf("the digest moved from %q to %q over an edit that did not happen", digest, got)
		}
	})

	t.Run("the author finished before the editor returned", func(t *testing.T) {
		h := newHarness(t)
		card := h.add("a card to comment on")
		h.comment(card, "what the author wrote")
		ref := card + "/" + bench.CommentsDir + "/1"
		dir := commentDirOf(t, h, ref)
		before := len(h.events(card))
		prior := bench.CommentDigest("what the author wrote")

		// The editor is the one that rewrote the file, so the change is a
		// fact rather than an assumption.
		handEdit(t, dir, "what the author wrote, revised")
		h.reopen()
		answer := h.library.RecordCommentEdit(&Request{
			Verb: "edit", Actor: "alka", Ref: ref, PriorDigest: prior,
		})
		if answer.Outcome != contract.OutcomeOK {
			t.Fatalf("the return: %s %s", answer.Outcome, answer.Refusal)
		}
		h.reopen()
		if got := storedDigest(t, dir); got != bench.CommentDigest("what the author wrote, revised") {
			t.Errorf("the new digest was not recorded: %q", got)
		}
		updated := 0
		for _, event := range h.events(card) {
			if event.Event == contract.EventCommentUpdated {
				updated++
			}
		}
		if updated != 1 {
			t.Errorf("the journal carries %d comment_updated lines, wanted one", updated)
		}
		if got := len(h.events(card)); got != before+1 {
			t.Errorf("the journal grew by %d lines, wanted one", got-before)
		}
	})
}

// TestATerminalVerbTakesACommentOfTheItemAlone asserts
// dinah-525/criteria/5: resolve, verify and fail take a reference to a comment
// of the item being settled, each refuses a reference naming another item's
// comment, a card comment, or something that is not a comment at all, and the
// item's anchor carries the resolution and no note key.
//
// The clause of criteria/5 saying reopen's reason takes the same is superseded
// by criteria/17 and by the spec's own "reopen's reason stays free prose, and
// the card's framing is amended on this point". The case that clause could not
// satisfy is asserted in TestReopenTakesProseAndClearsTheDesignation below.
func TestATerminalVerbTakesACommentOfTheItemAlone(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card carrying two questions")
	first := h.file(card, "open_question", "does the deadline move?")
	// The harness's file helper always answers the first position of its
	// kind, so the second question's own reference is composed here.
	h.file(card, "open_question", "does the price move?")
	second := card + "/questions/2"
	h.comment(first, "the operator ruled it does not")
	h.comment(second, "somebody else's answer")
	h.comment(card, "a remark about the card")
	h.reopen()

	for _, row := range []struct {
		name  string
		named string
	}{
		{"another item's comment", second + "/" + bench.CommentsDir + "/1"},
		{"a card comment", card + "/" + bench.CommentsDir + "/1"},
		{"the item itself, which is not a comment", first},
		{"a reference naming nothing", card + "/" + bench.CommentsDir + "/99"},
	} {
		response := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: first, Note: row.named})
		if response.Refusal != contract.NotADesignation {
			t.Errorf("%s: wanted %s, got %s %s", row.name, contract.NotADesignation, response.Outcome, response.Refusal)
		}
	}

	settled := h.library.Resolve(&Request{
		Verb: "resolve", Actor: "alka", Ref: first, Note: first + "/" + bench.CommentsDir + "/1",
	})
	if settled.Outcome != contract.OutcomeOK {
		t.Fatalf("a comment of the item itself: %s %s", settled.Outcome, settled.Refusal)
	}
	h.reopen()
	fm, _ := h.itemAnchor(first)
	if got := fm.Value(bench.ItemResolutionField); got != first+"/"+bench.CommentsDir+"/1" {
		t.Errorf("the item designates %q", got)
	}
	if got := fm.Value(bench.ItemNoteRetiredField); got != "" {
		t.Errorf("the item carries the retired note key %q", got)
	}
}

// TestTheIndexedReadOpensNoDesignatedComment asserts
// dinah-525/criteria/6: the indexed checklist read opens no designated comment
// and the full read opens each one once, counted rather than timed.
//
// The count runs over a card carrying thirty-three settled items, so an event
// fired once per read could not pass it: the unit is one comment opened, and
// zero against thirty-three is what tells the two reads apart. It fails against
// the shape read.go used before this card, which built every view and cleared
// the fields afterwards, because that shape pays the opens whatever the caller
// asked for.
func TestTheIndexedReadOpensNoDesignatedComment(t *testing.T) {
	const items = 33
	h := newHarness(t)
	card := h.add("a card carrying thirty-three settled items")
	for i := 0; i < items; i++ {
		h.file(card, "open_question", "question "+strconv.Itoa(i+1))
		ref := card + "/questions/" + strconv.Itoa(i+1)
		h.comment(ref, "the answer to question "+strconv.Itoa(i+1))
		settled := h.library.Resolve(&Request{
			Verb: "resolve", Actor: "alka", Ref: ref, Note: ref + "/" + bench.CommentsDir + "/1",
		})
		if settled.Outcome != contract.OutcomeOK {
			t.Fatalf("settle question %d: %s %s", i+1, settled.Outcome, settled.Refusal)
		}
	}
	h.reopen()

	opened := 0
	h.library.Observe = func(event, target string) {
		if event == ObserveDesignatedComment {
			opened++
		}
	}
	defer func() { h.library.Observe = nil }()

	if _, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card}); err != nil {
		t.Fatalf("the indexed read: %v", err)
	}
	if opened != 0 {
		t.Errorf("the indexed read opened %d designated comments, wanted none", opened)
	}

	opened = 0
	whole, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card, Fields: "checklist.full"})
	if err != nil {
		t.Fatalf("the full read: %v", err)
	}
	if opened != items {
		t.Errorf("the full read opened %d designated comments, wanted one per settled item", opened)
	}
	if len(whole.Checklist) != items {
		t.Fatalf("the full read carried %d items", len(whole.Checklist))
	}
	for _, item := range whole.Checklist {
		if item.Designated == nil || item.Designated.Body == "" {
			t.Fatalf("the full read carried no answer for %s", item.Ref)
		}
	}
}

// TestDeletingADesignatedCommentIsRefusedAndForced asserts
// dinah-525/criteria/7: the delete is refused with a diagnostic naming the
// designating item, reopening the item first frees the comment, --force
// reopens the item as part of the deletion and records a reason naming the
// comment that went, and both routes land with the comment gone and the item
// pending.
func TestDeletingADesignatedCommentIsRefusedAndForced(t *testing.T) {
	settle := func(t *testing.T, h *harness, card string) (item, comment string) {
		t.Helper()
		item = h.file(card, "open_question", "does the deadline move?")
		h.comment(item, "the operator ruled it does not")
		h.reopen()
		comment = item + "/" + bench.CommentsDir + "/1"
		response := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: item, Note: comment})
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("settle: %s %s", response.Outcome, response.Refusal)
		}
		h.reopen()
		return item, comment
	}

	t.Run("refused, then reopened and deleted", func(t *testing.T) {
		h := newHarness(t)
		card := h.add("a card carrying a settled question")
		item, comment := settle(t, h, card)
		dir := commentDirOf(t, h, comment)

		refused := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: comment, Confirm: true})
		if refused.Refusal != contract.NotDesignatable {
			t.Fatalf("the delete: wanted %s, got %s %s", contract.NotDesignatable, refused.Outcome, refused.Refusal)
		}
		if refused.Context["item"] == "" {
			t.Errorf("the diagnostic names no designating item: %+v", refused.Context)
		}
		if !bench.Exists(dir) {
			t.Fatal("the refused delete removed the comment")
		}

		reopened := h.library.Reopen(&Request{Verb: "reopen", Actor: "alka", Ref: item, Reason: "the ruling was wrong"})
		if reopened.Outcome != contract.OutcomeOK {
			t.Fatalf("reopen: %s %s", reopened.Outcome, reopened.Refusal)
		}
		h.reopen()
		removed := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: comment, Confirm: true})
		if removed.Outcome != contract.OutcomeOK {
			t.Fatalf("the delete after the reopen: %s %s", removed.Outcome, removed.Refusal)
		}
		h.reopen()
		if bench.Exists(dir) {
			t.Error("the comment survived the delete")
		}
		fm, _ := h.itemAnchor(item)
		if got := fm.Value(bench.ItemStateField); got != bench.ItemPending {
			t.Errorf("the item stands at %q, wanted pending", got)
		}
	})

	t.Run("forced", func(t *testing.T) {
		h := newHarness(t)
		card := h.add("a card carrying a settled question")
		item, comment := settle(t, h, card)
		dir := commentDirOf(t, h, comment)

		forced := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: comment, Confirm: true, Force: true})
		if forced.Outcome != contract.OutcomeOK {
			t.Fatalf("the forced delete: %s %s", forced.Outcome, forced.Refusal)
		}
		h.reopen()
		if bench.Exists(dir) {
			t.Error("the comment survived the forced delete")
		}
		fm, _ := h.itemAnchor(item)
		if got := fm.Value(bench.ItemStateField); got != bench.ItemPending {
			t.Errorf("the item stands at %q after the forced delete, wanted pending", got)
		}
		if got := fm.Value(bench.ItemResolutionField); got != "" {
			t.Errorf("the item still designates %q", got)
		}
		// The composed reason names the comment that went, which is the
		// only part of it that survives.
		found := false
		for _, event := range h.events(card) {
			if event.Event == contract.EventItemReopened && strings.Contains(event.Reason, comment) {
				found = true
			}
		}
		if !found {
			t.Errorf("no reopened line names the deleted comment %s", comment)
		}
	})
}

// TestForcingADeletionOnAnOperatorOwnedItemIsTheOperatorsAlone asserts
// dinah-525/criteria/8: the forced form is a reopen, so on an operator-owned
// item it is refused to anybody but the operator, on the same terms closeItem
// already refuses a terminal verb there.
//
// A force that did not respect that would be a way to unsettle an operator's
// ruling without being the operator.
func TestForcingADeletionOnAnOperatorOwnedItemIsTheOperatorsAlone(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card carrying the operator's own question")
	filed := h.library.File(&Request{
		Verb: "file", Actor: "alka", Card: card, Kind: "open_question",
		Text: "does the deadline move?", Owner: bench.ItemOwnerOperator,
	})
	if filed.Outcome != contract.OutcomeOK {
		t.Fatalf("file: %s %s", filed.Outcome, filed.Refusal)
	}
	h.reopen()
	item := card + "/questions/1"
	h.comment(item, "the operator ruled it does not")
	h.reopen()
	comment := item + "/" + bench.CommentsDir + "/1"
	// The operator settles it, because closeItem refuses anybody else there.
	settled := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: item, Note: comment})
	if settled.Outcome != contract.OutcomeOK {
		t.Fatalf("the operator's own resolve: %s %s", settled.Outcome, settled.Refusal)
	}
	h.reopen()
	dir := commentDirOf(t, h, comment)

	refused := h.library.Delete(&Request{Verb: "delete", Actor: "bob", Ref: comment, Confirm: true, Force: true})
	if refused.Refusal != contract.NotOperator {
		t.Fatalf("a forced delete by somebody who is not the operator: wanted %s, got %s %s",
			contract.NotOperator, refused.Outcome, refused.Refusal)
	}
	if !bench.Exists(dir) {
		t.Fatal("the refused forced delete removed the comment")
	}

	forced := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: comment, Confirm: true, Force: true})
	if forced.Outcome != contract.OutcomeOK {
		t.Fatalf("the operator's own forced delete: %s %s", forced.Outcome, forced.Refusal)
	}
	h.reopen()
	if bench.Exists(dir) {
		t.Error("the comment survived the operator's forced delete")
	}
}

// TestCheckReportsAnEmptyComment asserts dinah-525/criteria/9: an empty
// undesignated comment is reported at cleanup severity naming dinah delete as
// the remedy, an empty designated comment is reported at defect severity, and
// neither blocks a move.
func TestCheckReportsAnEmptyComment(t *testing.T) {
	h := newHarness(t)
	card := h.ready("a card carrying an abandoned draft")
	abandoned := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: card})
	if abandoned.Outcome != contract.OutcomeOK {
		t.Fatalf("the empty form: %s %s", abandoned.Outcome, abandoned.Refusal)
	}
	h.reopen()

	findings := h.check()
	detail, found := finding(findings, bench.FindingEmptyComment)
	if !found {
		t.Fatalf("check reports no empty comment: %+v", findings)
	}
	// The finding and the verb now answer the same spelling, which is the
	// reference, so the two are compared directly rather than joined by
	// resolving one of them.
	if detail != abandoned.Detail {
		t.Errorf("the finding names %q and the verb answers %q", detail, abandoned.Detail)
	}
	for _, f := range findings {
		if f.Key == bench.FindingEmptyComment && bench.SeverityOf(f) != bench.SeverityCleanup {
			t.Errorf("the empty-comment finding carries severity %q, wanted cleanup", bench.SeverityOf(f))
		}
	}
	// The remedy is named in the sentence the finding renders, which lives
	// in the catalogue rather than in the finding, so the assertion is that
	// the key the catalogue answers to is the one the finding carries.
	if bench.FindingEmptyComment != "check.empty-comment" {
		t.Errorf("the finding key is %q", bench.FindingEmptyComment)
	}

	// An empty comment an item designates is a defect, because an item's
	// answer of record cannot be nothing.
	item := h.file(card, "open_question", "does the deadline move?")
	minted := h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: item})
	if minted.Outcome != contract.OutcomeOK {
		t.Fatalf("the empty form on the item: %s %s", minted.Outcome, minted.Refusal)
	}
	h.reopen()
	settled := h.library.Resolve(&Request{
		Verb: "resolve", Actor: "alka", Ref: item, Note: item + "/" + bench.CommentsDir + "/1",
	})
	if settled.Outcome != contract.OutcomeOK {
		t.Fatalf("settle against the empty comment: %s %s", settled.Outcome, settled.Refusal)
	}
	h.reopen()
	findings = h.check()
	if _, found := finding(findings, bench.FindingEmptyDesignatedComment); !found {
		t.Fatalf("check reports no empty designated comment: %+v", findings)
	}
	for _, f := range findings {
		if f.Key == bench.FindingEmptyDesignatedComment && bench.SeverityOf(f) != bench.SeverityDefect {
			t.Errorf("the empty designated comment carries severity %q, wanted defect", bench.SeverityOf(f))
		}
	}

	// Neither blocks a move.
	moved := h.do(&Request{Verb: Move, Actor: "alka", Card: card, Column: aftercare})
	if moved.Outcome == contract.OutcomeRefused {
		t.Errorf("a card carrying an empty comment was refused a move: %s", moved.Refusal)
	}
}

// TestReopenTakesProseAndClearsTheDesignation asserts
// dinah-525/criteria/17: reopen's reason stays free prose and only the three
// terminal verbs take a designation, so no field holds two kinds of value.
//
// The case asserted here is the one the ordinary route through
// dinah-525/decisions/1 exists for and the one a reference-taking reopen could
// not satisfy: the item's only comment is about to be deleted, so a reason that
// had to be a comment of that item would have to name the comment that is
// going.
func TestReopenTakesProseAndClearsTheDesignation(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card carrying a settled question")
	item := h.file(card, "open_question", "does the deadline move?")
	h.comment(item, "the operator ruled it does not")
	h.reopen()
	comment := item + "/" + bench.CommentsDir + "/1"
	if settled := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: item, Note: comment}); settled.Outcome != contract.OutcomeOK {
		t.Fatalf("settle: %s %s", settled.Outcome, settled.Refusal)
	}
	h.reopen()

	reopened := h.library.Reopen(&Request{
		Verb: "reopen", Actor: "alka", Ref: item,
		Reason: "the ruling rested on a contract we have since replaced",
	})
	if reopened.Outcome != contract.OutcomeOK {
		t.Fatalf("a prose reason: %s %s", reopened.Outcome, reopened.Refusal)
	}
	h.reopen()
	fm, _ := h.itemAnchor(item)
	if got := fm.Value(bench.ItemResolutionField); got != "" {
		t.Errorf("the reopen left the designation %q standing", got)
	}
	// The prose reached the journal as prose, which is what "no field holds
	// two kinds of value" comes to on this side.
	found := false
	for _, event := range h.events(card) {
		if event.Event == contract.EventItemReopened && event.Reason == "the ruling rested on a contract we have since replaced" {
			found = true
		}
	}
	if !found {
		t.Error("the journal carries no reopened line holding the prose reason")
	}
	// And the comment the reopen freed can now be deleted, which is the
	// whole point of the route.
	removed := h.library.Delete(&Request{Verb: "delete", Actor: "alka", Ref: comment, Confirm: true})
	if removed.Outcome != contract.OutcomeOK {
		t.Fatalf("delete the freed comment: %s %s", removed.Outcome, removed.Refusal)
	}
}

// TestAWriteOverADivergenceIsRefused asserts dinah-525/criteria/18: a verb
// writing a comment whose stored digest disagrees with the body it is
// replacing is refused as dinah.comment-body-diverged, on a plain write and on
// an edit; accept-divergence ratifies what is on disk and records its actor;
// restoring the body by hand clears the divergence with no command at all; and
// a comment carrying no digest is written normally.
//
// The refusal is what stops the write absorbing a hand edit. Recomputing the
// digest from a tampered body takes the evidence with it, and on an edit it
// attributes somebody else's words to whoever ran the command.
func TestAWriteOverADivergenceIsRefused(t *testing.T) {
	plant := func(t *testing.T) (*harness, string, string, string) {
		t.Helper()
		h := newHarness(t)
		card := h.add("a card to comment on")
		h.comment(card, "what the verb wrote")
		ref := card + "/" + bench.CommentsDir + "/1"
		dir := commentDirOf(t, h, ref)
		handEdit(t, dir, "what somebody typed instead")
		h.reopen()
		return h, card, ref, dir
	}

	t.Run("a plain write", func(t *testing.T) {
		h, _, ref, _ := plant(t)
		refused := h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: ref, Field: bench.BodyField, Value: "and now the tool's words",
		})
		if refused.Refusal != contract.CommentBodyDiverged {
			t.Fatalf("wanted %s, got %s %s", contract.CommentBodyDiverged, refused.Outcome, refused.Refusal)
		}
	})

	t.Run("an edit that changed nothing does nothing", func(t *testing.T) {
		// The spec's first bullet is unconditional: an unchanged body does
		// nothing and journals nothing. Asking about a divergence ahead of it
		// made dinah edit on a diverged comment answer a refusal for an edit
		// that never happened, which told the reader nothing they could act
		// on and nothing dinah check would not have told them.
		h, card, ref, _ := plant(t)
		before := len(h.events(card))
		answer := h.library.RecordCommentEdit(&Request{
			Verb: "edit", Actor: "alka", Ref: ref,
			PriorDigest: bench.CommentDigest("what somebody typed instead"),
		})
		if answer.Outcome != contract.OutcomeOK {
			t.Fatalf("an edit that changed nothing over a diverged comment: %s %s", answer.Outcome, answer.Refusal)
		}
		h.reopen()
		if got := len(h.events(card)); got != before {
			t.Errorf("the journal grew by %d lines over an edit that did not happen", got-before)
		}
	})

	t.Run("an edit", func(t *testing.T) {
		h, _, ref, dir := plant(t)
		// The author opened the comment, so what they saw is the edited
		// body, and their own change is on top of it. The tool cannot tell
		// the two apart, so it refuses rather than attributing both to them.
		handEdit(t, dir, "what somebody typed instead, and then the author's own line")
		h.reopen()
		refused := h.library.RecordCommentEdit(&Request{
			Verb: "edit", Actor: "alka", Ref: ref,
			PriorDigest: bench.CommentDigest("what somebody typed instead"),
		})
		if refused.Refusal != contract.CommentBodyDiverged {
			t.Fatalf("wanted %s, got %s %s", contract.CommentBodyDiverged, refused.Outcome, refused.Refusal)
		}
	})

	t.Run("accept-divergence ratifies it", func(t *testing.T) {
		h, card, ref, dir := plant(t)
		accepted := h.library.AcceptDivergence(&Request{Verb: "accept-divergence", Actor: "bob", Ref: ref})
		if accepted.Outcome != contract.OutcomeOK {
			t.Fatalf("accept-divergence: %s %s", accepted.Outcome, accepted.Refusal)
		}
		h.reopen()
		if got := storedDigest(t, dir); got != bench.CommentDigest("what somebody typed instead") {
			t.Errorf("the digest was not re-stamped from the body as it stands: %q", got)
		}
		found := false
		for _, event := range h.events(card) {
			if event.Event == contract.EventDivergenceAccepted && event.Actor.Name == "bob" {
				found = true
			}
		}
		if !found {
			t.Error("the journal records no divergence_accepted line naming its actor")
		}
		// An ordinary write works again afterwards.
		written := h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: ref, Field: bench.BodyField, Value: "and now the tool's words",
		})
		if written.Outcome != contract.OutcomeOK {
			t.Fatalf("the write after the acceptance: %s %s", written.Outcome, written.Refusal)
		}
	})

	t.Run("restoring the body by hand needs no command", func(t *testing.T) {
		h, _, ref, dir := plant(t)
		handEdit(t, dir, "what the verb wrote")
		h.reopen()
		if detail, found := finding(h.check(), bench.FindingCommentBodyDiverged); found {
			t.Errorf("check still reports %s after the body was put back", detail)
		}
		written := h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: ref, Field: bench.BodyField, Value: "and now the tool's words",
		})
		if written.Outcome != contract.OutcomeOK {
			t.Fatalf("the write after the restore: %s %s", written.Outcome, written.Refusal)
		}
	})

	t.Run("a comment carrying no digest is written normally", func(t *testing.T) {
		h := newHarness(t)
		card := h.add("a card to comment on")
		h.comment(card, "what the verb wrote")
		ref := card + "/" + bench.CommentsDir + "/1"
		dir := commentDirOf(t, h, ref)
		fm, body, err := bench.ReadCommentAnchor(dir)
		if err != nil {
			t.Fatalf("read the comment: %v", err)
		}
		fm.Delete(bench.CommentDigestField)
		if err := bench.WriteText(filepath.Join(dir, bench.CommentAnchor), fm.Render(body)); err != nil {
			t.Fatalf("strip the digest: %v", err)
		}
		h.reopen()
		written := h.library.SetField(&Request{
			Verb: "set", Actor: "alka", Ref: ref, Field: bench.BodyField, Value: "the tool's words",
		})
		if written.Outcome != contract.OutcomeOK {
			t.Fatalf("a comment carrying no digest: %s %s", written.Outcome, written.Refusal)
		}
	})
}

// TestAnAuthorlessCommentSurvivesEveryPath asserts dinah-525/criteria/19: a
// comment the store cannot attribute walks through the comment reader, the
// checklist row that designates it, the containment listing and dinah check,
// and each carries the absence through rather than refusing the comment,
// substituting a name, or rendering a blank that reads as a defect.
//
// An empty author is representable today, because Comment.Author is a plain
// string and the writer does not require one. What this card owes is that
// nothing between the anchor and the rendered row rejects it or fills it in.
func TestAnAuthorlessCommentSurvivesEveryPath(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card carrying an unattributable answer")
	item := h.file(card, "open_question", "does the deadline move?")
	entity, err := h.library.Bench.ResolveEntity(item)
	if err != nil {
		t.Fatalf("resolve the item: %v", err)
	}
	// The comment the note migration writes for an item whose settling the
	// journal cannot attribute: no author at all, and the absence recorded
	// as a finding.
	dir := filepath.Join(entity.Dir, bench.CommentsDir, "f00000000001")
	fm := bench.NewFrontmatter()
	fm.Set("ts", "2026-08-01T09:00:00Z")
	fm.Set(bench.CommentAuthorUnrecoverableField, "true")
	fm.Set(bench.OrdinalField, "1")
	if err := bench.WriteCommentAnchor(dir, fm, "the answer somebody wrote"); err != nil {
		t.Fatalf("plant the authorless comment: %v", err)
	}
	h.reopen()
	comment := item + "/" + bench.CommentsDir + "/1"
	settled := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: item, Note: comment})
	if settled.Outcome != contract.OutcomeOK {
		t.Fatalf("designate the authorless comment: %s %s", settled.Outcome, settled.Refusal)
	}
	h.reopen()

	// The comment reader.
	read, err := bench.Comments(entity.Dir)
	if err != nil {
		t.Fatalf("read the item's comments: %v", err)
	}
	if len(read) != 1 {
		t.Fatalf("the item carries %d comments", len(read))
	}
	if read[0].Author != "" {
		t.Errorf("the reader filled the author with %q", read[0].Author)
	}
	if !read[0].AuthorUnrecoverable {
		t.Error("the reader dropped author_unrecoverable, so the absence reads as an omission")
	}

	// The checklist row that designates it.
	whole, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: card, Fields: "checklist.full"})
	if err != nil {
		t.Fatalf("the full read: %v", err)
	}
	if len(whole.Checklist) != 1 || whole.Checklist[0].Designated == nil {
		t.Fatalf("the full read carried no designated comment: %+v", whole.Checklist)
	}
	designated := whole.Checklist[0].Designated
	if designated.Author != "" {
		t.Errorf("the row filled the author with %q", designated.Author)
	}
	if !designated.AuthorUnrecoverable {
		t.Error("the row dropped author_unrecoverable")
	}
	if designated.Body != "the answer somebody wrote" {
		t.Errorf("the row carried the body %q", designated.Body)
	}

	// The containment listing.
	listed, err := h.library.ListRef(&Request{Verb: "list", Actor: "alka", Ref: item + "/" + bench.CommentsDir})
	if err != nil {
		t.Fatalf("list the item's comments: %v", err)
	}
	if listed.Comments == nil || len(listed.Comments.Members) != 1 {
		t.Fatalf("the listing carried %+v", listed.Comments)
	}
	if listed.Comments.Members[0].Author != "" {
		t.Errorf("the listing filled the author with %q", listed.Comments.Members[0].Author)
	}

	// And dinah check, which reports nothing about it: an absent author is a
	// recorded finding rather than a defect in the store.
	for _, f := range h.check() {
		if strings.Contains(f.Detail, "f00000000001") {
			t.Errorf("check reports %s over the authorless comment", f.Key)
		}
	}
}

// TestTheOneCommandFormMintsAndDesignates asserts
// dinah-525/criteria/20: --text mints a comment of the item authored by
// whoever ran the command and designates it in one act, the two-command form
// still designates a comment somebody else wrote, and naming both on one
// invocation is refused rather than resolved by precedence.
//
// The two forms record different things, and that is the reason for the shape.
// Under --text the author and the designator are the same person and the
// fields say so; under the two-command form the author stays whoever wrote the
// words and the designation records who chose them.
func TestTheOneCommandFormMintsAndDesignates(t *testing.T) {
	t.Run("--text mints and designates in one act", func(t *testing.T) {
		h := newHarness(t)
		card := h.add("a card carrying a question")
		item := h.file(card, "open_question", "does the deadline move?")
		settled := h.library.Resolve(&Request{
			Verb: "resolve", Actor: "bob", Ref: item, Text: "the operator ruled it does not",
		})
		if settled.Outcome != contract.OutcomeOK {
			t.Fatalf("the one-command form: %s %s", settled.Outcome, settled.Refusal)
		}
		h.reopen()
		entity, err := h.library.Bench.ResolveEntity(item)
		if err != nil {
			t.Fatalf("resolve the item: %v", err)
		}
		comments, err := bench.Comments(entity.Dir)
		if err != nil {
			t.Fatalf("read the item's comments: %v", err)
		}
		if len(comments) != 1 {
			t.Fatalf("the item carries %d comments, wanted the one the settling minted", len(comments))
		}
		if comments[0].Author != "bob" {
			t.Errorf("the minted comment's author is %q, wanted whoever ran the command", comments[0].Author)
		}
		if comments[0].Body != "the operator ruled it does not" {
			t.Errorf("the minted comment reads %q", comments[0].Body)
		}
		fm, _ := h.itemAnchor(item)
		if got := fm.Value(bench.ItemResolutionField); got != item+"/"+bench.CommentsDir+"/1" {
			t.Errorf("the item designates %q, wanted the comment the settling minted", got)
		}
		// One act, two journal lines: the comment that was made and the
		// settling that chose it.
		minted, closed := 0, 0
		for _, event := range h.events(card) {
			switch event.Event {
			case contract.EventCommented:
				minted++
			case contract.EventItemResolved:
				closed++
			}
		}
		if minted != 1 || closed != 1 {
			t.Errorf("the journal carries %d commented and %d item_resolved lines, wanted one of each", minted, closed)
		}
	})

	t.Run("the two-command form designates somebody else's words", func(t *testing.T) {
		h := newHarness(t)
		card := h.add("a card carrying a question")
		item := h.file(card, "open_question", "does the deadline move?")
		// The agent writes the recommendation.
		written := h.library.Comment(&Request{Verb: "comment", Actor: "bob", Card: item, Text: "it does not, on the 2026-08 contract"})
		if written.Outcome != contract.OutcomeOK {
			t.Fatalf("the agent's comment: %s %s", written.Outcome, written.Refusal)
		}
		h.reopen()
		// The operator designates it.
		comment := item + "/" + bench.CommentsDir + "/1"
		settled := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: item, Note: comment})
		if settled.Outcome != contract.OutcomeOK {
			t.Fatalf("the operator's designation: %s %s", settled.Outcome, settled.Refusal)
		}
		h.reopen()
		entity, err := h.library.Bench.ResolveEntity(item)
		if err != nil {
			t.Fatalf("resolve the item: %v", err)
		}
		comments, err := bench.Comments(entity.Dir)
		if err != nil {
			t.Fatalf("read the item's comments: %v", err)
		}
		if len(comments) != 1 || comments[0].Author != "bob" {
			t.Fatalf("the designated comment is %+v, wanted the agent's own", comments)
		}
		// The recommendation is not restated as the operator's, and the
		// ruling is not mistaken for the agent's.
		var designator string
		for _, event := range h.events(card) {
			if event.Event == contract.EventItemResolved {
				designator = event.Actor.Name
			}
		}
		if designator != "alka" {
			t.Errorf("the settling records %q as the designator, wanted the operator", designator)
		}
	})

	t.Run("naming both is refused", func(t *testing.T) {
		h := newHarness(t)
		card := h.add("a card carrying a question")
		item := h.file(card, "open_question", "does the deadline move?")
		h.comment(item, "it does not")
		h.reopen()
		refused := h.library.Resolve(&Request{
			Verb: "resolve", Actor: "alka", Ref: item,
			Note: item + "/" + bench.CommentsDir + "/1",
			Text: "and here are some other words",
		})
		if refused.Refusal != contract.Usage {
			t.Fatalf("naming both: wanted %s, got %s %s", contract.Usage, refused.Outcome, refused.Refusal)
		}
		h.reopen()
		fm, _ := h.itemAnchor(item)
		if got := fm.Value(bench.ItemStateField); got != bench.ItemPending {
			t.Errorf("the refused invocation settled the item, which now stands at %q", got)
		}
	})
}

// TestAnUnmigratedStoreIsRefusedByName asserts dinah-525/criteria/16: a reader
// opening a store the note migration has not reached is refused as
// dinah.store-awaiting-migration.
//
// The refusal exists because a reader that carried on would report every
// settled item on such a store as carrying no answer, which is a false reading
// rather than a degraded one. The other half of the criterion, that the
// migration itself carries no flag on check, no catalogue string and no help
// row, is asserted in cmd/dinah's own suite, where the help output and the
// catalogues are.
func TestAnUnmigratedStoreIsRefusedByName(t *testing.T) {
	h := newHarness(t)
	path := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)

	// Every format below, not the one below. The gate this replaced carried a
	// lower bound, so a store at format 3 or 4 opened with no complaint and
	// reported every settled item on it as carrying no answer; eighteen of the
	// nineteen live stores are below the bound it used. A case exercising one
	// format passes against that bound, which is why this one walks them all.
	refused := 0
	for declared := 1; declared < bench.ResolutionFormat; declared++ {
		fm.Set("format", strconv.Itoa(declared))
		if err := bench.WriteText(path, fm.Render(body)); err != nil {
			t.Fatalf("write the anchor: %v", err)
		}
		_, err := bench.Open(h.root)
		refusal := &contract.Refusal{}
		if !errors.As(err, &refusal) {
			t.Errorf("a store declaring format %d opened, and it awaits the migration: %v", declared, err)
			continue
		}
		if refusal.Name != contract.StoreAwaitingMigration {
			t.Errorf("a store declaring format %d is refused %s, wanted %s", declared, refusal.Name, contract.StoreAwaitingMigration)
			continue
		}
		// And the openers that serve the migration and the diagnostic read
		// it, which is what keeps a store that needs migrating reachable by
		// the thing that migrates it.
		if _, err := bench.OpenAwaitingResolution(h.root); err != nil {
			t.Errorf("the migration's own opener is refused a store declaring format %d: %v", declared, err)
		}
		refused++
	}
	if refused != bench.ResolutionFormat-1 {
		t.Fatalf("%d formats below the current one were refused, wanted %d", refused, bench.ResolutionFormat-1)
	}

	// A store at the current format opens, so the refusals above are the gate
	// rather than a fixture that never opened.
	fm.Set("format", strconv.Itoa(bench.StorageFormat))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("restore the anchor: %v", err)
	}
	if _, err := bench.Open(h.root); err != nil {
		t.Errorf("the store at the current format is refused: %v", err)
	}
}

