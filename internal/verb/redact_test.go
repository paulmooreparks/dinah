package verb

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// storeHits counts the occurrences of a token in every file under the
// workbench, byte for byte, which is the search a redaction has to defeat.
func (h *harness) storeHits(token string) int {
	h.t.Helper()
	hits := 0
	err := filepath.WalkDir(h.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hits += bytes.Count(data, []byte(token))
		return nil
	})
	if err != nil {
		h.t.Fatalf("walk %s: %v", h.root, err)
	}
	return hits
}

// journalBytes reads a journal's bytes.
func journalBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// journalLines splits a journal into its lines, without their newlines.
func journalLines(data []byte) [][]byte {
	return bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
}

// digestOf is the value a redaction writes for one text.
func digestOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// redact runs dinah redact as the operator with the confirmation and fails
// the test unless it succeeded.
func (h *harness) redact(ref string) *RedactionReport {
	h.t.Helper()
	report, err := h.library.Redact(&Request{Verb: "redact", Actor: "alka", Ref: ref, Confirm: true})
	if err != nil {
		h.t.Fatalf("redact %s: %v", ref, err)
	}
	h.reopen()
	return report
}

// refusedAs asserts an error is the named refusal.
func refusedAs(t *testing.T, what string, err error, name string) {
	t.Helper()
	var refusal *contract.Refusal
	if !errors.As(err, &refusal) || refusal.Name != name {
		t.Errorf("%s answered %v, wanted %s", what, err, name)
	}
}

// tryAct runs one library act and reopens the bench after it.
func (h *harness) tryAct(act func(*Request) *Response, req *Request) *Response {
	h.t.Helper()
	response := act(req)
	h.reopen()
	return response
}

// mustAct runs one library act and fails the test unless it succeeded.
func (h *harness) mustAct(act func(*Request) *Response, req *Request) *Response {
	h.t.Helper()
	response := h.tryAct(act, req)
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("%s %s: %s %s", req.Verb, req.Ref+req.Card, response.Outcome, response.Refusal)
	}
	return response
}

// firstItem reads a card's first live checklist item from its record.
func (h *harness) firstItem(ref string) *bench.Item {
	h.t.Helper()
	record, err := h.library.Bench.LoadCardRecord(h.card(ref))
	if err != nil {
		h.t.Fatalf("record %s: %v", ref, err)
	}
	items := record.ItemsIn(bench.LiveHalf, "")
	if len(items) == 0 {
		h.t.Fatalf("%s carries no checklist item", ref)
	}
	return items[0]
}

// eventOf decodes one journal line.
func eventOf(t *testing.T, line []byte) bench.Event {
	t.Helper()
	var ev bench.Event
	if err := json.Unmarshal(line, &ev); err != nil {
		t.Fatalf("decode %s: %v", line, err)
	}
	return ev
}

// TestRedactReplacesACommentsTextEverywhere drives dinah-637/criteria/35. A
// card comment whose body and one later edit carry a unique token is found
// by a byte search of the store; redacting it as the operator leaves the token
// in no file, each rewritten line carries the digest of the text it carried
// and redacted true, the journal ends in a redacted line naming the comment
// with lines equal to the rewritten count and no text, and every other line
// is byte-identical. The same command from an actor who is not the operator
// is refused not-operator and leaves the journal byte-identical.
//
// Arming: making replaceText leave the text in place keeps the token in the
// journal, so the byte search after the redaction finds it.
func TestRedactReplacesACommentsTextEverywhere(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.add("A card carrying something that has to go")
	const token = "tok35-f1d8a2c9e7"
	h.comment(card, "The first body names "+token+".")
	comment := card + "/comments/1"
	h.mustSet(comment, "body", "The edited body still names "+token+".")
	if hits := h.storeHits(token); hits == 0 {
		t.Fatal("the token is nowhere in the store before the redaction, so this test proves nothing")
	}
	journal := h.card(card).JournalPath()
	before := journalBytes(t, journal)

	_, err := h.library.Redact(&Request{Verb: "redact", Actor: "bob", Ref: comment, Confirm: true})
	refusedAs(t, "redact by an actor who is not the operator", err, contract.NotOperator)
	if !bytes.Equal(journalBytes(t, journal), before) {
		t.Error("the refused redaction changed the journal")
	}

	report := h.redact(comment)
	if hits := h.storeHits(token); hits != 0 {
		t.Errorf("after the redaction a byte search of the store still finds the token %d times", hits)
	}
	old, now := journalLines(before), journalLines(journalBytes(t, journal))
	if len(now) != len(old)+1 {
		t.Fatalf("the journal holds %d lines after the redaction, wanted %d", len(now), len(old)+1)
	}
	rewritten := 0
	for i, line := range old {
		if !bytes.Contains(line, []byte(token)) {
			if !bytes.Equal(line, now[i]) {
				t.Errorf("line %d carried no text of the comment and changed:\n  was %s\n  now %s", i+1, line, now[i])
			}
			continue
		}
		rewritten++
		was, is := eventOf(t, line), eventOf(t, now[i])
		if is.Text != digestOf(was.Text) || !is.Redacted {
			t.Errorf("line %d carries text %q redacted %v, wanted the digest of its text and redacted true", i+1, is.Text, is.Redacted)
		}
		was.Text, was.Redacted = is.Text, true
		if !bytes.Equal(mustEncode(t, was), now[i]) {
			t.Errorf("line %d changed beyond its text:\n  was %s\n  now %s", i+1, line, now[i])
		}
	}
	if rewritten != 2 {
		t.Errorf("%d lines carried the comment's text, wanted its commented line and its edit", rewritten)
	}
	last := eventOf(t, now[len(now)-1])
	if last.Event != contract.EventRedacted || last.Kind != bench.KindComment || last.Comment == "" || last.Lines != rewritten || last.Text != "" {
		t.Errorf("the journal ends in %+v, wanted a redacted line naming the comment with lines %d and no text", last, rewritten)
	}
	if report.Lines != rewritten || report.OwnLines+report.LegacyLines != report.Lines {
		t.Errorf("the answer counts %d own and %d legacy lines, %d in all, and %d were rewritten", report.OwnLines, report.LegacyLines, report.Lines, rewritten)
	}
}

// mustEncode encodes a line the way the journal does.
func mustEncode(t *testing.T, ev bench.Event) []byte {
	t.Helper()
	line, err := bench.EncodeEvent(ev)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return line
}

// TestRedactReachesEveryMemberAndRefusesTheRest drives dinah-637/criteria/36.
// An item, an item comment, a column comment, a deleted comment named by
// identifier and a comment an unblock minted are each redacted, and a byte
// search finds each one's token nowhere afterwards; the redacted item keeps
// its kind, state, column, owner and citations. A card, a column, an
// attachment and a payload are refused not-redactable and a format-11 store
// store-awaiting-migration, each writing nothing. Legacy note lines of an
// item whose final answer became a designated comment lose the versions the
// comment's redaction names and then, with the item's, the rest, and no value
// is hashed twice.
//
// Arming: dropping the unblocked row of ownTexts leaves the unblock's reason
// in the journal, so the byte search after the redactions finds its token.
func TestRedactReachesEveryMemberAndRefusesTheRest(t *testing.T) {
	h := newCardUnitHarness(t)
	h.declareEvidence(evidenceBlock)
	card := h.ready("A card whose members are redacted")

	const itemToken = "tok36-item-5a1c"
	item := h.file(card, "acceptance_criterion", "The criterion names "+itemToken+".")
	h.mustSet(item, "text", "The criterion still names "+itemToken+".")
	h.mustAct(h.library.Cite, &Request{Verb: "cite", Actor: "alka", Ref: item, Scheme: "test", CiteTarget: "internal/x_test.go#TestX", Observed: "fail:pass"})
	h.mustAct(h.library.Verify, &Request{Verb: "verify", Actor: "alka", Ref: item, Text: "checked it"})
	was := h.firstItem(card)

	const itemCommentToken = "tok36-itemcomment-77e0"
	question := h.file(card, "open_question", "What does the question ask?")
	h.mustAct(h.library.Comment, &Request{Verb: "comment", Actor: "alka", Card: question, Text: "An argument naming " + itemCommentToken + "."})

	const columnToken = "tok36-column-c3b2"
	h.mustAct(h.library.Comment, &Request{Verb: "comment", Actor: "alka", Card: aftercareSlug, Text: "A column note naming " + columnToken + "."})

	const deletedToken = "tok36-deleted-91fa"
	h.comment(card, "A comment naming "+deletedToken+".")
	deletedID := ""
	for _, ev := range h.events(card) {
		if ev.Event == contract.EventCommented && strings.Contains(ev.Text, deletedToken) {
			deletedID = ev.Comment
		}
	}
	h.mustAct(h.library.Delete, &Request{Verb: "delete", Actor: "alka", Ref: card + "/comments/1", Confirm: true})

	const unblockToken = "tok36-unblock-0b4d"
	h.mustDo(&Request{Verb: "block", Actor: "alka", Card: card, Reason: "waiting on something"})
	h.mustDo(&Request{Verb: "unblock", Actor: "alka", Card: card, Reason: "It arrived, see " + unblockToken + "."})
	unblockID := ""
	for _, ev := range h.events(card) {
		if ev.Event == contract.EventUnblocked {
			unblockID = ev.Comment
		}
	}

	for _, token := range []string{itemToken, itemCommentToken, columnToken, deletedToken, unblockToken} {
		if h.storeHits(token) == 0 {
			t.Fatalf("the token %s is nowhere in the store before the redactions, so this test proves nothing", token)
		}
	}
	h.redact(item)
	h.redact(question + "/comments/1")
	h.redact(aftercareSlug + "/comments/1")
	h.redact(card + "/comments/" + deletedID)
	h.redact(card + "/comments/" + unblockID)
	for _, token := range []string{itemToken, itemCommentToken, columnToken, deletedToken, unblockToken} {
		if hits := h.storeHits(token); hits != 0 {
			t.Errorf("after the redactions a byte search still finds %s %d times", token, hits)
		}
	}
	for _, ev := range h.events(card) {
		if ev.Event == contract.EventUnblocked && !strings.HasPrefix(ev.Reason, "sha256:") {
			t.Errorf("the unblocked line's reason reads %q, wanted the digest of the reason", ev.Reason)
		}
	}
	redactedItem := h.firstItem(card)

	if redactedItem.Kind != was.Kind || redactedItem.State != was.State || redactedItem.Column != was.Column ||
		redactedItem.Owner != was.Owner || len(redactedItem.Citations) != len(was.Citations) || redactedItem.Text != "" || redactedItem.Redacted == nil {
		t.Errorf("the redacted item reads %+v, wanted %+v with its text empty and redacted true", redactedItem, was)
	}

	cardJournal := h.card(card).JournalPath()
	benchJournal := h.library.Bench.JournalPath()
	h.attach(card, "notes.txt", "attached bytes")
	for _, ref := range []string{card, aftercareSlug, card + "/attachments/1", card + "/attachments/1/payload"} {
		cardBefore, benchBefore := journalBytes(t, cardJournal), journalBytes(t, benchJournal)
		_, err := h.library.Redact(&Request{Verb: "redact", Actor: "alka", Ref: ref, Confirm: true})
		refusedAs(t, "redact "+ref, err, contract.NotRedactable)
		if !bytes.Equal(journalBytes(t, cardJournal), cardBefore) || !bytes.Equal(journalBytes(t, benchJournal), benchBefore) {
			t.Errorf("the refused redaction of %s wrote to a journal", ref)
		}
	}
}

// TestRedactRewritesLegacyAnswerLines drives the legacy half of
// dinah-637/criteria/36. An item carries two item_updated note lines, as the
// answers written before dinah-525 did, whose values hold two versions of its
// answer, each with a token of its own; the final version is the body of the
// comment the item designates, as --migrate-designations left it. Redacting
// the comment replaces the values equal to its text and leaves the earlier
// version; redacting the item replaces every remaining note value; afterwards
// neither token is anywhere in the store, and no value is a digest of a
// digest.
//
// Arming: comparing the note values against the comment's text without
// dropping the trailing newline the note's final value carries and the
// comment's body lacks leaves the second version in place, so the count of
// legacy lines the comment's redaction reports is zero.
func TestRedactRewritesLegacyAnswerLines(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.ready("A card whose answer predates designated comments")
	question := h.file(card, "open_question", "Which way do we go?")
	const first, second = "tok36-legacy-first-4411", "tok36-legacy-second-9d2e"
	earlier, final := "Go left, per "+first+".", "Go right, per "+second+"."
	itemID := ""
	for _, ev := range h.events(card) {
		if ev.Event == contract.EventItemFiled {
			itemID = ev.Item
		}
	}
	journal := h.card(card).JournalPath()
	lock, err := bench.Acquire(h.card(card).Dir, "alka", "2026-08-17T09:00:00Z")
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	for _, legacy := range []bench.Event{
		{TS: "2026-08-17T09:00:00Z", Event: contract.EventItemUpdated, Actor: bench.NamedActor("alka"), Field: "note", Note: itemID, To: earlier},
		{TS: "2026-08-17T09:00:01Z", Event: contract.EventItemUpdated, Actor: bench.NamedActor("alka"), Field: "note", Note: itemID, From: earlier, To: final + "\n"},
	} {
		if err := bench.AppendEvent(lock, journal, legacy); err != nil {
			t.Fatalf("plant a legacy line: %v", err)
		}
	}
	lock.Release()
	h.reopen()
	h.mustAct(h.library.Resolve, &Request{Verb: "resolve", Actor: "alka", Ref: question, Text: final})

	commentReport := h.redact(question + "/comments/1")
	if commentReport.LegacyLines != 1 {
		t.Errorf("the comment's redaction rewrote %d legacy lines, wanted the one carrying its text", commentReport.LegacyLines)
	}
	if h.storeHits(second) != 0 || h.storeHits(first) == 0 {
		t.Errorf("after the comment's redaction the store holds the final version %d times and the earlier %d times, wanted none and some", h.storeHits(second), h.storeHits(first))
	}
	itemReport := h.redact(question)
	if itemReport.LegacyLines != 2 {
		t.Errorf("the item's redaction rewrote %d legacy lines, wanted both note lines", itemReport.LegacyLines)
	}
	if h.storeHits(first) != 0 || h.storeHits(second) != 0 {
		t.Error("after both redactions a token of the legacy answer is still in the store")
	}
	digests := map[string]bool{}
	for _, ev := range h.events(card) {
		for _, value := range []string{ev.From, ev.To, ev.Text} {
			if strings.HasPrefix(value, "sha256:") {
				digests[value] = true
			}
		}
	}
	for value := range digests {
		if digests[digestOf(value)] {
			t.Errorf("%s is in the journal and so is its own digest, so a value was hashed twice", value)
		}
	}
}

// TestRedactRefusesAStoreBelowTheCardUnitFormat drives the format clause of
// dinah-637/criteria/36: a store below format 12 keeps text in files a
// rewrite of the journal would not reach, so dinah redact is refused
// store-awaiting-migration there and writes nothing.
func TestRedactRefusesAStoreBelowTheCardUnitFormat(t *testing.T) {
	h := newSerialHarness(t)
	card := h.add("A card on a store in the old layout")
	h.comment(card, "A comment in the old layout.")
	journal := h.card(card).JournalPath()
	before := journalBytes(t, journal)
	_, err := h.library.Redact(&Request{Verb: "redact", Actor: "alka", Ref: card + "/comments/1", Confirm: true})
	refusedAs(t, "redact on a format-11 store", err, contract.StoreAwaitingMigration)
	if !bytes.Equal(journalBytes(t, journal), before) {
		t.Error("the refused redaction changed the journal")
	}
}

// TestARedactedMemberReadsAsRedacted drives dinah-637/criteria/37. After a
// redaction the comment's view carries redacted true and an empty body, a
// search for its token matches nothing, and the item designating it stays
// settled with its resolution. A second redaction is refused
// already-redacted; a body write and accept-divergence are refused redacted;
// and archiving and deleting the member succeed.
//
// Arming: dropping the redacted refusal from writeField lets the body write
// through, which the refusal assertion catches.
func TestARedactedMemberReadsAsRedacted(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.ready("A card whose answer is redacted")
	question := h.file(card, "open_question", "Is it safe?")
	const token = "tok37-answer-6c0e"
	h.mustAct(h.library.Resolve, &Request{Verb: "resolve", Actor: "alka", Ref: question, Text: "Yes, per " + token + "."})
	answer := question + "/comments/1"
	h.redact(answer)

	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "alka", Card: question})
	if err != nil {
		t.Fatalf("show %s: %v", question, err)
	}
	_ = detail
	entity, err := h.library.Bench.ResolveEntity(question)
	if err != nil {
		t.Fatalf("resolve %s: %v", question, err)
	}
	shown, err := h.library.itemDetailOf(entity)
	if err != nil {
		t.Fatalf("item detail: %v", err)
	}
	if len(shown.Comments) != 1 || !shown.Comments[0].Redacted || shown.Comments[0].Body != "" || shown.Comments[0].Size != 0 {
		t.Errorf("the item's comment reads %+v, wanted redacted true with an empty body", shown.Comments)
	}
	if shown.Comments[0].Redaction == nil || shown.Comments[0].Redaction.By != "alka" {
		t.Errorf("the comment's redaction reads %+v, wanted the operator's", shown.Comments[0].Redaction)
	}
	results, err := h.library.Search(&Request{Verb: "search", Actor: "alka", SearchText: token})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if encoded, _ := json.Marshal(results); strings.Contains(string(encoded), card) {
		t.Errorf("a search for the redacted token matched: %s", encoded)
	}
	item := h.firstItem(card)
	if item.State != bench.ItemResolved || item.Resolution == "" {
		t.Errorf("the item designating the redacted comment reads %s with resolution %q, wanted it resolved and naming the comment", item.State, item.Resolution)
	}

	_, err = h.library.Redact(&Request{Verb: "redact", Actor: "alka", Ref: answer, Confirm: true})
	refusedAs(t, "a second redaction", err, contract.AlreadyRedacted)
	if response := h.tryAct(h.library.SetField, &Request{Verb: "set", Actor: "alka", Ref: answer, Field: "body", Value: "a new body"}); response.Refusal != contract.Redacted {
		t.Errorf("a body write to the redacted comment answered %s %s, wanted %s", response.Outcome, response.Refusal, contract.Redacted)
	}
	if response := h.tryAct(h.library.AcceptDivergence, &Request{Verb: "accept-divergence", Actor: "alka", Ref: answer}); response.Refusal != contract.Redacted {
		t.Errorf("accept-divergence on the redacted comment answered %s %s, wanted %s", response.Outcome, response.Refusal, contract.Redacted)
	}
	plain := h.file(card, "decision", "A decision whose text goes.")
	h.redact(plain)
	if response := h.tryAct(h.library.SetField, &Request{Verb: "set", Actor: "alka", Ref: plain, Field: "text", Value: "new text"}); response.Refusal != contract.Redacted {
		t.Errorf("a text write to the redacted item answered %s %s, wanted %s", response.Outcome, response.Refusal, contract.Redacted)
	}
	h.mustAct(h.library.Archive, &Request{Verb: "archive", Actor: "alka", Ref: plain})
	h.mustAct(h.library.Delete, &Request{Verb: "delete", Actor: "alka", Ref: answer, Confirm: true, Force: true})
}

// TestNoCommandTakesARedactionBack drives the last clause of
// dinah-637/criteria/37: no command of the verb table names an act undoing a
// redaction, and no source under internal outside the tests clears a
// member's redaction.
//
// Arming: adding an unredact command to the table, or a line setting a
// member's Redacted back to nil, reddens it.
func TestNoCommandTakesARedactionBack(t *testing.T) {
	for _, command := range Commands() {
		if strings.Contains(command, "redact") && command != "redact" {
			t.Errorf("the verb table carries %s, and no command takes a redaction back", command)
		}
	}
	clearing := regexp.MustCompile(`\.Redacted\s*=\s*nil|Redacted:\s*nil|\.Redacted\s*=\s*false`)
	// Every package under internal is read, the library and the store
	// among them, since a clearing written anywhere there would reach a
	// reader.
	files, err := filepath.Glob(filepath.Join("..", "*", "*.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("the glob found no source, so this test proves nothing")
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if clearing.Match(source) {
			t.Errorf("%s clears a member's redaction", file)
		}
	}
}

// storeSnapshot reads every file under the workbench, keyed by its path, so
// a test can say the store is byte-identical.
func (h *harness) storeSnapshot() map[string]string {
	h.t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(h.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = string(data)
		return nil
	})
	if err != nil {
		h.t.Fatalf("walk %s: %v", h.root, err)
	}
	return files
}

// sameStore reports the first difference between two snapshots.
func sameStore(before, after map[string]string) string {
	for path, data := range before {
		if now, ok := after[path]; !ok {
			return path + " is gone"
		} else if now != data {
			return path + " changed"
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			return path + " appeared"
		}
	}
	return ""
}

// TestRedactWithoutConfirmationAndAfterAFailureWritesNothing drives the first
// three clauses of dinah-637/criteria/38. Without --yes the answer names the
// member, its line counts and its journal and the store stays byte-identical;
// a failure planted before the rename leaves the journal byte-identical and no
// journal.ndjson.redact behind; and a planted leftover is reported by dinah
// check as check.redact-leftover and removed.
//
// Arming: dropping the removal of the composed file on a failed rename leaves
// journal.ndjson.redact behind, which the second assertion finds.
func TestRedactWithoutConfirmationAndAfterAFailureWritesNothing(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.add("A card whose comment is nearly redacted")
	h.comment(card, "A comment somebody means to redact.")
	comment := card + "/comments/1"
	journal := h.card(card).JournalPath()

	before := h.storeSnapshot()
	report, err := h.library.Redact(&Request{Verb: "redact", Actor: "alka", Ref: comment})
	if err != nil {
		t.Fatalf("redact without --yes: %v", err)
	}
	if difference := sameStore(before, h.storeSnapshot()); difference != "" {
		t.Errorf("redact without --yes wrote to the store: %s", difference)
	}
	if report.Written || report.Member != comment || report.Journal != journal || report.OwnLines != 1 || report.OwnLines+report.LegacyLines != report.Lines {
		t.Errorf("the dry run answered %+v, wanted the comment, its journal and one own line, unwritten", report)
	}

	planted := errors.New("the disk refused the rename")
	bench.SetRedactStepForTest(t, func(string) error { return planted })
	journalBefore := journalBytes(t, journal)
	if _, err := h.library.Redact(&Request{Verb: "redact", Actor: "alka", Ref: comment, Confirm: true}); !errors.Is(err, planted) {
		t.Errorf("the redaction with a failure planted before its rename answered %v, wanted the planted failure", err)
	}
	if !bytes.Equal(journalBytes(t, journal), journalBefore) {
		t.Error("the failed redaction changed the journal")
	}
	if bench.Exists(journal + bench.RedactLeftoverSuffix) {
		t.Error("the failed redaction left journal.ndjson.redact behind")
	}
	bench.SetRedactStepForTest(t, nil)

	leftover := journal + bench.RedactLeftoverSuffix
	if err := os.WriteFile(leftover, []byte("a stale composition\n"), 0o644); err != nil {
		t.Fatalf("plant %s: %v", leftover, err)
	}
	if detail, ok := finding(h.check(), bench.FindingRedactLeftover); !ok || detail != filepath.Base(leftover) {
		t.Errorf("check reported the leftover as %q (found %v), wanted %s", detail, ok, filepath.Base(leftover))
	}
	if bench.Exists(leftover) {
		t.Error("check left the stale journal.ndjson.redact in place")
	}
}

// TestRedactRefusesBesideATornSidecarAndLeavesAttachments drives the rest of
// dinah-637/criteria/38. With a journal.torn.* sidecar beside the journal the
// redaction is refused torn-sidecar-present naming it, and succeeds once the
// sidecar is deleted. A comment carrying two attachments keeps both payloads
// byte-identical and the answer lists both by reference and filename; a
// comment carrying none answers an empty list.
//
// Arming: dropping the sidecar test from bench.Redact lets the first
// redaction through, which the refusal assertion catches.
func TestRedactRefusesBesideATornSidecarAndLeavesAttachments(t *testing.T) {
	h := newCardUnitHarness(t)
	card := h.add("A card with a torn journal fragment beside it")
	h.comment(card, "A comment with two attachments.")
	comment := card + "/comments/1"
	h.attach(comment, "first.txt", "the first payload")
	h.attach(comment, "second.txt", "the second payload")
	h.comment(card, "A comment with none.")
	sidecar := filepath.Join(h.card(card).Dir, bench.TornSidecarPrefix+"20260817T090000Z")
	if err := os.WriteFile(sidecar, []byte(`{"ts":"2026-08-17T0`), 0o644); err != nil {
		t.Fatalf("plant %s: %v", sidecar, err)
	}
	_, err := h.library.Redact(&Request{Verb: "redact", Actor: "alka", Ref: comment, Confirm: true})
	var refusal *contract.Refusal
	if !errors.As(err, &refusal) || refusal.Name != contract.TornSidecarPresent || refusal.Detail != sidecar {
		t.Fatalf("the redaction beside a sidecar answered %v, wanted %s naming %s", err, contract.TornSidecarPresent, sidecar)
	}
	if err := os.Remove(sidecar); err != nil {
		t.Fatalf("remove %s: %v", sidecar, err)
	}

	payloads := func() map[string]string {
		found := map[string]string{}
		for path, data := range h.storeSnapshot() {
			if strings.Contains(path, string(filepath.Separator)+bench.AttachmentsDir+string(filepath.Separator)) && !strings.HasSuffix(path, bench.AttachmentAnchor) {
				found[path] = data
			}
		}
		return found
	}
	before := payloads()
	if len(before) != 2 {
		t.Fatalf("the store holds %d payloads, wanted the comment's two", len(before))
	}
	report := h.redact(comment)
	if difference := sameStore(before, payloads()); difference != "" {
		t.Errorf("the redaction touched an attachment: %s", difference)
	}
	want := []AttachmentLeft{{Ref: comment + "/attachments/1", Filename: "first.txt"}, {Ref: comment + "/attachments/2", Filename: "second.txt"}}
	if len(report.AttachmentsLeft) != 2 || report.AttachmentsLeft[0] != want[0] || report.AttachmentsLeft[1] != want[1] {
		t.Errorf("the answer lists %+v as left, wanted %+v", report.AttachmentsLeft, want)
	}
	bare := h.redact(card + "/comments/2")
	if bare.AttachmentsLeft == nil || len(bare.AttachmentsLeft) != 0 {
		t.Errorf("a comment carrying no attachment answered %#v, wanted an empty list", bare.AttachmentsLeft)
	}
}
