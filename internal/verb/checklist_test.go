package verb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// criterionText is the item text every case that needs one is filed with. The
// echo checks compare a note against it, so it is written once here rather
// than retyped at each call site where the two have to be byte-identical.
const criterionText = "the endpoint returns 404 for an unknown id"

// evidenceBlock is the declaration the citation obligation turns on, written
// as a person would type it into workbench.md. The test scheme demands an
// observation and the attachment scheme does not, which is the difference
// cite's own refusal reads.
const evidenceBlock = "evidence:\n" +
	"  test:\n" +
	"    hint: A Go test, written as the file path, a hash, and the test function name.\n" +
	"    observed: required\n" +
	"  attachment:\n" +
	"    hint: The 12-hex id of an attachment on this card.\n"

// declareEvidence writes an evidence block into the workbench anchor, which is
// the only way one arrives: no verb declares a scheme, and the block is
// frontmatter a person edits.
func (h *harness) declareEvidence(block string) {
	h.t.Helper()
	path := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		h.t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	lines := bench.SplitLines(strings.TrimSuffix(block, "\n"))
	fm.SetRaw(bench.EvidenceKey, lines)
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		h.t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()
}

// file files one checklist item and returns the reference the other five verbs
// take, which is the card's reference and the kind-narrowed word.
func (h *harness) file(card, kind, text string) string {
	h.t.Helper()
	response := h.library.File(&Request{Verb: "file", Actor: "alka", Card: card, Kind: kind, Text: text})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("file %s: %s %s", kind, response.Outcome, response.Refusal)
	}
	h.reopen()
	return card + "/" + map[string]string{
		"acceptance_criterion": "criteria",
		"open_question":        "questions",
		"decision":             "decisions",
	}[kind] + "/1"
}

// itemAnchor reads an item's anchor back off disk, which is how a test asserts
// what was written rather than what was returned.
func (h *harness) itemAnchor(ref string) (*bench.Frontmatter, string) {
	h.t.Helper()
	entity, err := h.library.Bench.ResolveEntity(ref)
	if err != nil {
		h.t.Fatalf("resolve %s: %v", ref, err)
	}
	fm, body, err := bench.ReadItemAnchor(entity.Dir)
	if err != nil {
		h.t.Fatalf("read %s: %v", ref, err)
	}
	return fm, body
}

// TestFileCreatesAPendingItemAndOneJournalLine asserts dinah-206 AC-1: filing
// on a card carrying no checklist directory writes the anchor with the kind,
// the pending state, the first ordinal and the given text, and appends exactly
// one item_filed line.
func TestFileCreatesAPendingItemAndOneJournalLine(t *testing.T) {
	h := newHarness(t)
	card := h.add("first card")
	if _, err := os.Stat(filepath.Join(h.card(card).Dir, bench.ChecklistDir)); !os.IsNotExist(err) {
		t.Fatalf("the card already carries a checklist directory, so this case starts from the wrong state: %v", err)
	}
	before := len(h.events(card))

	ref := h.file(card, "acceptance_criterion", criterionText)
	fm, body := h.itemAnchor(ref)
	if got := fm.Value(bench.ItemKindField); got != "acceptance_criterion" {
		t.Errorf("kind: wanted acceptance_criterion, got %q", got)
	}
	if got := fm.Value(bench.ItemStateField); got != bench.ItemPending {
		t.Errorf("state: wanted pending, got %q", got)
	}
	if got := bench.OrdinalOf(fm); got != 1 {
		t.Errorf("ordinal: wanted 1, got %d", got)
	}
	if got := strings.TrimSpace(body); got != criterionText {
		t.Errorf("body: wanted the item's text, got %q", got)
	}
	// The column field is written only when a caller supplies one, and this
	// call supplied none. Nothing reads the field for enforcement today, so
	// absence is what the write commits to rather than a chosen default.
	if fm.Has(bench.ItemColumnField) {
		t.Errorf("column: wanted the field absent, got %q", fm.Value(bench.ItemColumnField))
	}

	filed := 0
	for _, ev := range h.events(card)[before:] {
		if ev.Event != contract.EventItemFiled {
			t.Errorf("wanted only item_filed lines from this call, got %s", ev.Event)
			continue
		}
		filed++
		if ev.Kind != "acceptance_criterion" {
			t.Errorf("item_filed: wanted the kind, got %q", ev.Kind)
		}
		if ev.Item == "" {
			t.Error("item_filed: carries no item identifier")
		}
	}
	if filed != 1 {
		t.Errorf("wanted exactly one item_filed line from one call, got %d", filed)
	}
}

// TestFileWritesTheColumnAndTheOwnerOnlyWhenGiven asserts the other half of
// the absence rule: a caller naming a column and an owner gets both fields,
// so the absence above is the flag not being passed rather than the write
// dropping what it was given.
func TestFileWritesTheColumnAndTheOwnerOnlyWhenGiven(t *testing.T) {
	h := newHarness(t)
	card := h.add("first card")
	response := h.library.File(&Request{
		Verb: "file", Actor: "alka", Card: card,
		Kind: "open_question", Text: "does the deadline move?",
		Column: "review", Owner: "operator",
	})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("file: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	fm, _ := h.itemAnchor(card + "/oq/1")
	if got := fm.Value(bench.ItemColumnField); got != "review" {
		t.Errorf("column: wanted review, got %q", got)
	}
	if got := fm.Value(bench.ItemOwnerField); got != "operator" {
		t.Errorf("owner: wanted operator, got %q", got)
	}
}

// TestFileRefusesAnUnknownKindAndAnEmptyArgument asserts dinah-206 AC-2, each
// of the three refusals by the name the spec fixes for it.
func TestFileRefusesAnUnknownKindAndAnEmptyArgument(t *testing.T) {
	h := newHarness(t)
	card := h.add("first card")
	for _, c := range []struct {
		name    string
		req     *Request
		refusal string
		detail  string
	}{
		{"an unknown kind", &Request{Kind: "nonsense", Text: "text"}, contract.UnknownItemKind, "nonsense"},
		{"empty text", &Request{Kind: "acceptance_criterion", Text: ""}, contract.Malformed, "text"},
		{"empty kind", &Request{Kind: "", Text: "text"}, contract.Malformed, bench.ItemKindField},
	} {
		req := c.req
		req.Verb, req.Actor, req.Card = "file", "alka", card
		response := h.library.File(req)
		if response.Refusal != c.refusal {
			t.Errorf("%s: wanted %s, got %s %s", c.name, c.refusal, response.Outcome, response.Refusal)
			continue
		}
		if response.Detail != c.detail {
			t.Errorf("%s: wanted the detail %q, got %q", c.name, c.detail, response.Detail)
		}
	}
}

// TestResolvingAnOpenQuestionClearsTheClaimRefusal asserts dinah-206 AC-3: the
// state changes, the note is persisted, and the claim CORE-CLAIM-9 was
// refusing on this item's account is admitted afterwards.
func TestResolvingAnOpenQuestionClearsTheClaimRefusal(t *testing.T) {
	h := newHarness(t)
	card := h.ready("first card")
	ref := h.file(card, "open_question", "does the deadline move?")

	if response := h.do(&Request{Verb: Claim, Actor: "alka", Card: card, Holder: "alka"}); response.Refusal != contract.UnresolvedItem {
		t.Fatalf("a pending open question should refuse the claim, got %s %s", response.Outcome, response.Refusal)
	}
	note := "the operator confirmed the deadline is the 15th, per the comment thread"
	if response := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: ref, Note: note}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("resolve: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	fm, _ := h.itemAnchor(ref)
	if got := fm.Value(bench.ItemStateField); got != bench.ItemResolved {
		t.Errorf("state: wanted resolved, got %q", got)
	}
	if got := fm.Value(bench.ItemNoteField); got != note {
		t.Errorf("note: wanted it persisted, got %q", got)
	}
	if response := h.do(&Request{Verb: Claim, Actor: "alka", Card: card, Holder: "alka"}); response.Outcome != contract.OutcomeOK {
		t.Errorf("after the resolution the claim should be admitted, got %s %s", response.Outcome, response.Refusal)
	}
}

// TestATerminalVerbRefusesTheWrongKindAndAClosedItem asserts dinah-206 AC-4:
// resolve on an acceptance criterion and verify on an open question each
// refuse wrong-item-kind, and a second resolve refuses not-pending.
func TestATerminalVerbRefusesTheWrongKindAndAClosedItem(t *testing.T) {
	h := newHarness(t)
	card := h.add("first card")
	criterion := h.file(card, "acceptance_criterion", criterionText)
	question := h.file(card, "open_question", "does the deadline move?")

	if response := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: criterion, Note: "checked"}); response.Refusal != contract.WrongItemKind {
		t.Errorf("resolve on a criterion: wanted wrong-item-kind, got %s %s", response.Outcome, response.Refusal)
	} else if response.Detail != "acceptance_criterion" {
		t.Errorf("resolve on a criterion: wanted the item's own kind in the detail, got %q", response.Detail)
	}
	if response := h.library.Verify(&Request{Verb: "verify", Actor: "alka", Ref: question, Note: "checked"}); response.Refusal != contract.WrongItemKind {
		t.Errorf("verify on a question: wanted wrong-item-kind, got %s %s", response.Outcome, response.Refusal)
	}

	if response := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: question, Note: "the operator ruled it does not move"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("the first resolve: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if response := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: question, Note: "and again"}); response.Refusal != contract.NotPending {
		t.Errorf("a second resolve: wanted not-pending, got %s %s", response.Outcome, response.Refusal)
	}
}

// TestTheCitationObligationFollowsTheEvidenceBlock asserts dinah-206 AC-5: on
// a workbench declaring evidence, verify refuses an uncited criterion and
// succeeds once one citation is there, and on a workbench declaring none the
// same uncited call succeeds.
func TestTheCitationObligationFollowsTheEvidenceBlock(t *testing.T) {
	h := newHarness(t)
	h.declareEvidence(evidenceBlock)
	card := h.add("first card")
	ref := h.file(card, "acceptance_criterion", criterionText)

	if response := h.library.Verify(&Request{Verb: "verify", Actor: "alka", Ref: ref, Note: "closes the gap"}); response.Refusal != contract.Uncited {
		t.Fatalf("an uncited criterion on an evidence-declaring workbench: wanted uncited, got %s %s", response.Outcome, response.Refusal)
	}
	cite := h.library.Cite(&Request{
		Verb: "cite", Actor: "alka", Ref: ref,
		Scheme: "test", CiteTarget: "internal/verb/query_test.go#TestQueryNamesSeverity",
		Observed: "fail:pass",
	})
	if cite.Outcome != contract.OutcomeOK {
		t.Fatalf("cite: %s %s", cite.Outcome, cite.Refusal)
	}
	h.reopen()
	if response := h.library.Verify(&Request{Verb: "verify", Actor: "alka", Ref: ref, Note: "closes the gap"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("a cited criterion: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if fm, _ := h.itemAnchor(ref); fm.Value(bench.ItemStateField) != bench.ItemVerified {
		t.Errorf("state: wanted verified, got %q", fm.Value(bench.ItemStateField))
	}

	undeclaring := newHarness(t)
	other := undeclaring.add("first card")
	bare := undeclaring.file(other, "acceptance_criterion", criterionText)
	if response := undeclaring.library.Verify(&Request{Verb: "verify", Actor: "alka", Ref: bare, Note: "closes the gap"}); response.Outcome != contract.OutcomeOK {
		t.Errorf("an uncited criterion on a workbench declaring no evidence: %s %s", response.Outcome, response.Refusal)
	}
}

// TestCiteChecksTheObservationAndNothingElse asserts dinah-206 AC-6: a
// malformed pair refuses malformed on the observed argument, a scheme
// declaring observed: required refuses without one, and a scheme declaring no
// such requirement succeeds without one.
func TestCiteChecksTheObservationAndNothingElse(t *testing.T) {
	h := newHarness(t)
	h.declareEvidence(evidenceBlock)
	card := h.add("first card")
	ref := h.file(card, "acceptance_criterion", criterionText)
	target := "internal/verb/query_test.go#TestQueryNamesSeverity"

	response := h.library.Cite(&Request{Verb: "cite", Actor: "alka", Ref: ref, Scheme: "test", CiteTarget: target, Observed: "pending:pass"})
	if response.Refusal != contract.Malformed || response.Detail != bench.ItemCitationObserved {
		t.Errorf("a half outside fail and pass: wanted malformed on observed, got %s %s %q", response.Outcome, response.Refusal, response.Detail)
	}
	response = h.library.Cite(&Request{Verb: "cite", Actor: "alka", Ref: ref, Scheme: "test", CiteTarget: target})
	if response.Refusal != contract.ObservationRequired {
		t.Errorf("a scheme declaring observed: required, cited with no pair: wanted observation-required, got %s %s", response.Outcome, response.Refusal)
	}
	response = h.library.Cite(&Request{Verb: "cite", Actor: "alka", Ref: ref, Scheme: "attachment", CiteTarget: "4f2c19ab77e0"})
	if response.Outcome != contract.OutcomeOK {
		t.Errorf("a scheme declaring no observed requirement, cited with no pair: %s %s", response.Outcome, response.Refusal)
	}
}

// TestATerminalVerbRefusesANoteThatEchoesTheItem asserts dinah-206 AC-7: the
// empty note and the literal echo both refuse malformed on the note, and the
// echo is recognised after trimming.
func TestATerminalVerbRefusesANoteThatEchoesTheItem(t *testing.T) {
	h := newHarness(t)
	card := h.add("first card")
	ref := h.file(card, "open_question", criterionText)
	for _, note := range []string{criterionText, "", " " + criterionText + " "} {
		response := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: ref, Note: note})
		if response.Refusal != contract.Malformed || response.Detail != bench.ItemNoteField {
			t.Errorf("the note %q: wanted malformed on the note, got %s %s %q", note, response.Outcome, response.Refusal, response.Detail)
		}
	}
}

// TestReopenReturnsAnItemToPendingWithoutErasingIt asserts dinah-206 AC-8: the
// state goes back to pending, the prior note and the prior citation stay on
// disk, an empty reason refuses malformed, and reopening a pending item
// refuses not-resolved.
func TestReopenReturnsAnItemToPendingWithoutErasingIt(t *testing.T) {
	h := newHarness(t)
	h.declareEvidence(evidenceBlock)
	card := h.add("first card")
	ref := h.file(card, "acceptance_criterion", criterionText)

	if response := h.library.Reopen(&Request{Verb: "reopen", Actor: "alka", Ref: ref, Reason: "too early"}); response.Refusal != contract.NotResolved {
		t.Errorf("reopen on a pending item: wanted not-resolved, got %s %s", response.Outcome, response.Refusal)
	}
	target := "internal/verb/query_test.go#TestQueryNamesSeverity"
	if response := h.library.Cite(&Request{Verb: "cite", Actor: "alka", Ref: ref, Scheme: "test", CiteTarget: target, Observed: "fail:pass"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("cite: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	note := "the new fixture drives the unknown-id path and the handler answers 404"
	if response := h.library.Verify(&Request{Verb: "verify", Actor: "alka", Ref: ref, Note: note}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("verify: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	if response := h.library.Reopen(&Request{Verb: "reopen", Actor: "alka", Ref: ref, Reason: ""}); response.Refusal != contract.Malformed || response.Detail != "reason" {
		t.Errorf("an empty reason: wanted malformed on the reason, got %s %s %q", response.Outcome, response.Refusal, response.Detail)
	}
	reason := "the reviewer found the fixture predates the fix"
	if response := h.library.Reopen(&Request{Verb: "reopen", Actor: "alka", Ref: ref, Reason: reason}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("reopen: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	fm, _ := h.itemAnchor(ref)
	if got := fm.Value(bench.ItemStateField); got != bench.ItemPending {
		t.Errorf("state: wanted pending, got %q", got)
	}
	if got := fm.Value(bench.ItemNoteField); got != note {
		t.Errorf("the prior note should survive the reopen, got %q", got)
	}
	if got := bench.CountCitations(fm); got != 1 {
		t.Errorf("the prior citation should survive the reopen, got %d citations", got)
	}
	if got := strings.Join(fm.Raw(bench.CitationsField), "\n"); !strings.Contains(got, target) {
		t.Errorf("the citation's target should survive the reopen, got %q", got)
	}
}

// TestFailLandsAtFailedUnderTheSameChecksAsVerify asserts dinah-206 AC-11: the
// empty note and the echo both refuse, an uncited criterion on an
// evidence-declaring workbench refuses uncited, and the cited call lands the
// item at failed rather than at verified.
func TestFailLandsAtFailedUnderTheSameChecksAsVerify(t *testing.T) {
	h := newHarness(t)
	h.declareEvidence(evidenceBlock)
	card := h.add("first card")
	ref := h.file(card, "acceptance_criterion", criterionText)

	for _, note := range []string{"", criterionText} {
		response := h.library.Fail(&Request{Verb: "fail", Actor: "alka", Ref: ref, Note: note})
		if response.Refusal != contract.Malformed || response.Detail != bench.ItemNoteField {
			t.Errorf("the note %q: wanted malformed on the note, got %s %s %q", note, response.Outcome, response.Refusal, response.Detail)
		}
	}
	note := "still returns 200 for an unknown id"
	if response := h.library.Fail(&Request{Verb: "fail", Actor: "alka", Ref: ref, Note: note}); response.Refusal != contract.Uncited {
		t.Fatalf("an uncited criterion: wanted uncited, got %s %s", response.Outcome, response.Refusal)
	}
	if response := h.library.Cite(&Request{
		Verb: "cite", Actor: "alka", Ref: ref,
		Scheme: "test", CiteTarget: "internal/verb/query_test.go#TestQueryNamesSeverity",
		Observed: "fail:fail",
	}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("cite: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if response := h.library.Fail(&Request{Verb: "fail", Actor: "alka", Ref: ref, Note: note}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("fail: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	fm, _ := h.itemAnchor(ref)
	if got := fm.Value(bench.ItemStateField); got != bench.ItemFailed {
		t.Errorf("state: wanted failed, got %q", got)
	}
}

// TestAnItemWriteIsACommentShapedActRatherThanAClaimShapedOne asserts the
// spec's decision D-2: none of the six verbs consults the claim system, so a
// card somebody else holds takes a checklist write exactly as it takes a
// comment.
func TestAnItemWriteIsACommentShapedActRatherThanAClaimShapedOne(t *testing.T) {
	h := newHarness(t)
	card := h.ready("first card")
	h.mustDo(&Request{Verb: Claim, Actor: "bern", Card: card, Holder: "bern"})
	if got := h.card(card).Holder; got != "bern" {
		t.Fatalf("the card should be held by somebody else, got %q", got)
	}
	ref := h.file(card, "decision", "the write path takes the card's own lock")
	if response := h.library.Resolve(&Request{Verb: "resolve", Actor: "alka", Ref: ref, Note: "mirrors what Comment already does"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("resolving on a card another owner holds: %s %s", response.Outcome, response.Refusal)
	}
}

// secondLibrary is a second view of the same workbench on the harness's own
// clock, standing for a second process. Two libraries over one bench is how
// the tree already drives a concurrent writer, at
// TestTheCardLockCoversTheWholeTransaction.
func (h *harness) secondLibrary() *Library {
	h.t.Helper()
	opened, err := bench.Open(h.root)
	if err != nil {
		h.t.Fatalf("open a second view of the workbench: %v", err)
	}
	other := New(opened, h.home)
	other.Now = h.library.Now
	return other
}

// TestATwoWriterCiteKeepsBothCitations asserts that an item write reads the
// item under the card's lock rather than before it, by driving a whole second
// cite through the one window where the first holds no lock.
//
// The Interpose hook fires after the reference has been resolved and before
// the lock is taken. A second library files its citation there and finishes,
// so by the time the first takes the lock the item on disk carries an entry
// the first has not seen. Both entries have to survive. A verb that read the
// item before that window would write its own snapshot back over the second
// writer's entry, and the loss would be silent: both calls answer ok and one
// journal line each is appended, so nothing but the anchor reveals it.
func TestATwoWriterCiteKeepsBothCitations(t *testing.T) {
	h := newHarness(t)
	h.declareEvidence(evidenceBlock)
	card := h.add("first card")
	ref := h.file(card, "acceptance_criterion", criterionText)
	other := h.secondLibrary()

	const intruderTarget = "internal/verb/checklist_test.go#TestFileCreatesAPendingItemAndOneJournalLine"
	const ownTarget = "internal/verb/checklist_test.go#TestATwoWriterCiteKeepsBothCitations"
	var intruder *Response
	h.library.Interpose = func(step string) {
		if step != itemStepUnlocked {
			return
		}
		intruder = other.Cite(&Request{
			Verb: "cite", Actor: "bob", Ref: ref,
			Scheme: "test", CiteTarget: intruderTarget, Observed: "fail:pass",
		})
	}
	first := h.library.Cite(&Request{
		Verb: "cite", Actor: "alka", Ref: ref,
		Scheme: "test", CiteTarget: ownTarget, Observed: "fail:pass",
	})
	h.library.Interpose = nil
	h.reopen()

	if intruder == nil {
		t.Fatal("the interposed cite never ran, so this test proves nothing")
	}
	if intruder.Outcome != contract.OutcomeOK {
		t.Fatalf("the interposed cite: %s %s", intruder.Outcome, intruder.Refusal)
	}
	if first.Outcome != contract.OutcomeOK {
		t.Fatalf("the first cite: %s %s", first.Outcome, first.Refusal)
	}
	fm, _ := h.itemAnchor(ref)
	if got := bench.CountCitations(fm); got != 2 {
		t.Errorf("citations: wanted 2, got %d", got)
	}
	raw := strings.Join(fm.Raw(bench.CitationsField), "\n")
	for _, target := range []string{intruderTarget, ownTarget} {
		if !strings.Contains(raw, target) {
			t.Errorf("the citations lost %s:\n%s", target, raw)
		}
	}
}

// TestASecondCloseIsRefusedRatherThanOverwritingTheFirst asserts that
// not-pending is decided against the state on disk at the moment of the write.
//
// A second library resolves the item outright in the window where the first
// holds no lock, so the first arrives at a resolved item. It has to be refused
// not-pending, and the note the second writer left has to survive. A verb
// reading the item before that window would see pending, be admitted, and
// overwrite an answer somebody else had already recorded, which is the failure
// not-pending exists to prevent.
func TestASecondCloseIsRefusedRatherThanOverwritingTheFirst(t *testing.T) {
	h := newHarness(t)
	card := h.add("first card")
	ref := h.file(card, "open_question", criterionText)
	other := h.secondLibrary()

	const intruderNote = "the second writer answered it first, and this is that answer"
	var intruder *Response
	h.library.Interpose = func(step string) {
		if step != itemStepUnlocked {
			return
		}
		intruder = other.Resolve(&Request{Verb: "resolve", Actor: "bob", Ref: ref, Note: intruderNote})
	}
	first := h.library.Resolve(&Request{
		Verb: "resolve", Actor: "alka", Ref: ref,
		Note: "the first writer's answer, which must not land",
	})
	h.library.Interpose = nil
	h.reopen()

	if intruder == nil {
		t.Fatal("the interposed resolve never ran, so this test proves nothing")
	}
	if intruder.Outcome != contract.OutcomeOK {
		t.Fatalf("the interposed resolve: %s %s", intruder.Outcome, intruder.Refusal)
	}
	if first.Refusal != contract.NotPending {
		t.Errorf("the second resolve: wanted %s, got %s %s", contract.NotPending, first.Outcome, first.Refusal)
	}
	fm, _ := h.itemAnchor(ref)
	if got := fm.Value(bench.ItemNoteField); got != intruderNote {
		t.Errorf("note: wanted the interposed writer's answer, got %q", got)
	}
	resolutions := 0
	for _, ev := range h.events(card) {
		if ev.Event == contract.EventItemResolved {
			resolutions++
		}
	}
	if resolutions != 1 {
		t.Errorf("wanted one item_resolved recorded, got %d", resolutions)
	}
}
