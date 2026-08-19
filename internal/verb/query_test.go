package verb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// ask runs a query against the harness's bench and fails unless it succeeded.
func (h *harness) ask(text string) *Matches {
	h.t.Helper()
	matches, err := h.library.Query(&Request{Verb: "query", Actor: "alka", Query: text})
	if err != nil {
		h.t.Fatalf("query %q: %v", text, err)
	}
	return matches
}

// refuse runs a query expected to refuse and returns the refusal, failing the
// test when the query succeeded or failed for some other reason.
func (h *harness) refuse(text string) *contract.Refusal {
	h.t.Helper()
	matches, err := h.library.Query(&Request{Verb: "query", Actor: "alka", Query: text})
	if err == nil {
		h.t.Fatalf("query %q was expected to refuse and returned %d cards", text, matches.Count)
	}
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		h.t.Fatalf("query %q returned %T rather than a refusal: %v", text, err, err)
	}
	return refusal
}

// refs is the card references a result carries, in the order it carries them.
func refs(matches *Matches) []string {
	found := make([]string, 0, len(matches.Cards))
	for _, card := range matches.Cards {
		found = append(found, card.Ref)
	}
	return found
}

// wantRefs fails unless a query selected exactly the cards named, in order.
func wantRefs(t *testing.T, text string, matches *Matches, want ...string) {
	t.Helper()
	got := refs(matches)
	if len(got) != len(want) {
		t.Fatalf("query %q selected %v, want %v", text, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("query %q selected %v, want %v", text, got, want)
		}
	}
	if matches.Count != len(want) {
		t.Errorf("query %q reported a count of %d over %d cards", text, matches.Count, len(want))
	}
}

// setWorkstreams writes a card's workstreams list into its anchor by hand,
// because no verb assigns membership and the field reaches a card only through
// frontmatter somebody wrote.
func (h *harness) setWorkstreams(ref string, names ...string) {
	h.t.Helper()
	path := filepath.Join(h.card(ref).Dir, bench.CardAnchor)
	text, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("read %s: %v", path, err)
	}
	var block strings.Builder
	block.WriteString("workstreams:\n")
	for _, name := range names {
		block.WriteString("  - " + name + "\n")
	}
	edited := strings.Replace(string(text), "---\n", "---\n"+block.String(), 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		h.t.Fatalf("write %s: %v", path, err)
	}
	h.reopen()
}

// TestAQueryOverActsFindsTheCardsAnOwnerTouched asserts the touched question of
// the spec's section 4: a lone act-plane term asks for a card carrying at least
// one act by that owner, whatever the card looks like now, and the answer comes
// back in arrival order.
func TestAQueryOverActsFindsTheCardsAnOwnerTouched(t *testing.T) {
	h := newHarness(t)
	first := h.add("first")
	second := h.add("second")
	third := h.add("third")
	h.mustDo(&Request{Verb: Claim, Actor: "bo", Card: second, Holder: "bo"})
	h.mustDo(&Request{Verb: Move, Actor: "bo", Card: second, State: doing})
	h.mustDo(&Request{Verb: Claim, Actor: "bo", Card: third, Holder: "bo"})
	h.mustDo(&Request{Verb: Release, Actor: "bo", Card: third})

	wantRefs(t, "actor:bo", h.ask("actor:bo"), second, third)
	wantRefs(t, "actor:alka", h.ask("actor:alka"), first, second, third)
	wantRefs(t, "actor:nobody", h.ask("actor:nobody"))
}

// TestOneActWitnessesEveryActPlaneTerm asserts the single-witness rule: an
// interval query over an entry into a state selects the card whose entry falls
// inside it, keeps a card that has since moved on, and does not select a card
// whose entry and whose later act fall on either side of the window.
func TestOneActWitnessesEveryActPlaneTerm(t *testing.T) {
	h := newHarness(t)
	h.clock = time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	h.reopen()
	early := h.add("entered in June")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: early, Holder: "alka"})
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: early, State: doing})
	// Doing holds one card, so the June card leaves before the August card
	// arrives. Its entry stays in the journal, which is where the query looks.
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: early, State: review})

	h.clock = time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	h.reopen()
	inside := h.add("entered in August")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: inside, Holder: "alka"})
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: inside, State: doing})
	// The August card moves on, so the query has to find the entry in the
	// journal rather than the state the card stands in now.
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: inside, State: review})
	// The June card is commented on inside the window, which is the second
	// witness the looser reading would accept.
	h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: early, Text: "a note"})
	h.reopen()

	window := "entered:doing at>=2026-08-01 at<2026-09-01"
	wantRefs(t, window, h.ask(window), inside)
	wantRefs(t, "entered:doing", h.ask("entered:doing"), early, inside)
	// The same query with the two terms free to find their own witnesses
	// would return the June card, so each half is asserted to select it.
	wantRefs(t, "at>=2026-08-01", h.ask("at>=2026-08-01"), early, inside)
}

// TestAnUnknownFieldIsNamedBackToTheReader asserts check 2 and the rule that
// locates a field token: the token is the run before the first character of an
// operator, whatever characters it carries, and a token this tool does not know
// is named back with the ten legal names beside it.
func TestAnUnknownFieldIsNamedBackToTheReader(t *testing.T) {
	h := newHarness(t)
	h.add("a card")
	for _, query := range []struct {
		text  string
		field string
	}{
		{"substate:ready priority>=next", "priority"},
		{"State:doing", "State"},
		{"Priority>=next", "Priority"},
		{"field1:x", "field1"},
		{"at-2:x", "at-2"},
	} {
		refusal := h.refuse(query.text)
		if refusal.Name != contract.UnknownField {
			t.Errorf("query %q refused %s, want %s", query.text, refusal.Name, contract.UnknownField)
		}
		if refusal.Detail != query.field {
			t.Errorf("query %q named %q, want %q", query.text, refusal.Detail, query.field)
		}
		for _, field := range QueryFields {
			if !strings.Contains(refusal.Extra["fields"], field) {
				t.Errorf("query %q did not list the legal field %s", query.text, field)
			}
		}
	}
	// A term with no operator character at all, and one whose field token
	// would be empty, have no derivation and stay malformed.
	for _, text := range []string{":doing", "doing"} {
		if refusal := h.refuse(text); refusal.Name != contract.Malformed {
			t.Errorf("query %q refused %s, want %s", text, refusal.Name, contract.Malformed)
		}
	}
}

// TestAnOperatorTheFieldDoesNotTakeIsRefusedAsAField asserts check 3: at is the
// one field whose values rank, so an ordered operator on any other field and an
// equality operator on at are both the same mistake to a reader.
func TestAnOperatorTheFieldDoesNotTakeIsRefusedAsAField(t *testing.T) {
	h := newHarness(t)
	h.add("a card")
	for _, text := range []string{"state>doing", "substate>=ready", "at:2026-08-01", "at!=2026-08-01"} {
		refusal := h.refuse(text)
		if refusal.Name != contract.UnknownField {
			t.Errorf("query %q refused %s, want %s", text, refusal.Name, contract.UnknownField)
		}
		if !strings.HasPrefix(text, refusal.Detail) {
			t.Errorf("query %q named %q, which is not the combination the term wrote", text, refusal.Detail)
		}
	}
}

// TestAClosedVocabularyRefusesATypoAndAnOpenOneDoesNot asserts check 4, check 5
// and the three fields that carry no vocabulary at all. A typo on a closed
// field refuses and lists what is legal; the same text on an open field is a
// value nothing carries, which is an empty result rather than a mistake.
func TestAClosedVocabularyRefusesATypoAndAnOpenOneDoesNot(t *testing.T) {
	h := newHarness(t)
	h.add("a card")
	for _, text := range []string{"substate:reday", "event:comment"} {
		refusal := h.refuse(text)
		if refusal.Name != contract.UnknownValue {
			t.Errorf("query %q refused %s, want %s", text, refusal.Name, contract.UnknownValue)
		}
		if refusal.Extra["term"] != text {
			t.Errorf("query %q named the term %q", text, refusal.Extra["term"])
		}
		if refusal.Extra["legal"] == "" {
			t.Errorf("query %q listed no legal value", text)
		}
	}
	if legal := h.refuse("substate:reday").Extra["legal"]; !strings.Contains(legal, contract.SubstateBlocked) {
		t.Errorf("the substate vocabulary reported was %q", legal)
	}
	if legal := h.refuse("event:comment").Extra["legal"]; !strings.Contains(legal, contract.EventCommented) {
		t.Errorf("the event vocabulary reported was %q", legal)
	}
	if refusal := h.refuse("state:nosuchstate"); refusal.Name != contract.UnknownState {
		t.Errorf("state:nosuchstate refused %s, want %s", refusal.Name, contract.UnknownState)
	}
	for _, text := range []string{"holder:reday", "actor:reday", "block_kind:reday"} {
		wantRefs(t, text, h.ask(text))
	}
}

// TestAnAtValueIsReadByTheQueryRatherThanByParseStamp asserts that a malformed
// instant refuses before any card is read, and that a date reads as midnight
// UTC at the start of its day.
func TestAnAtValueIsReadByTheQueryRatherThanByParseStamp(t *testing.T) {
	h := newHarness(t)
	h.clock = time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC)
	h.reopen()
	before := h.add("stamped one second early")
	h.clock = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	h.reopen()
	onTheBoundary := h.add("stamped at midnight")

	for _, text := range []string{"at>=notadate", "at>=2026-13-01", "at>=2026-08-01T25:00:00Z"} {
		refusal := h.refuse(text)
		if refusal.Name != contract.Malformed {
			t.Errorf("query %q refused %s, want %s", text, refusal.Name, contract.Malformed)
		}
		if refusal.Detail != text {
			t.Errorf("query %q named %q rather than the offending term", text, refusal.Detail)
		}
	}
	wantRefs(t, "at>=2026-08-01", h.ask("at>=2026-08-01"), onTheBoundary)
	wantRefs(t, "at<2026-08-01", h.ask("at<2026-08-01"), before)
	wantRefs(t, "at>=2026-08-01T00:00:00Z", h.ask("at>=2026-08-01T00:00:00Z"), onTheBoundary)
}

// TestTheParseRulesOfTheQueryLanguageHold asserts the rules of section 2 that
// no other test reaches: what whitespace does, what quoting does, what a comma
// does, and what a repeated field does.
func TestTheParseRulesOfTheQueryLanguageHold(t *testing.T) {
	h := newHarness(t)
	first := h.add("first")
	second := h.add("second")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: first, Holder: "alka"})
	h.mustDo(&Request{Verb: Claim, Actor: "bo", Card: second, Holder: "bo"})
	third := h.add("third")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: third, Holder: "alka"})
	h.mustDo(&Request{Verb: Block, Actor: "alka", Card: third, Reason: "waiting on a ruling"})

	// Trimming, and a tab between two terms separating them as a space does.
	wantRefs(t, `" "`, h.ask(" "), first, second, third)
	wantRefs(t, `""`, h.ask(""), first, second, third)
	wantRefs(t, "a tab between terms", h.ask("substate:active\tholder:alka"), first)

	// A comma reads as or under : and as neither under !=.
	wantRefs(t, "substate:ready,active", h.ask("substate:ready,active"), first, second)
	wantRefs(t, "substate!=ready,active", h.ask("substate!=ready,active"), third)

	// A quoted value is one value, and quoting turns the comma split off.
	if got := h.ask(`holder:"alka,bo"`); got.Count != 0 {
		t.Errorf(`holder:"alka,bo" selected %v, and a quoted value is compared whole`, refs(got))
	}
	// The blocked card was freed by the block, so alka holds the first alone.
	wantRefs(t, `holder:"alka"`, h.ask(`holder:"alka"`), first)

	// An escape inside quotes admits the quotation mark and the backslash and
	// nothing else, and a backslash outside quotes is not admitted at all.
	if got := h.ask(`holder:"a\"b"`); got.Count != 0 {
		t.Errorf(`holder:"a\"b" selected %v and no card is held by a"b`, refs(got))
	}
	for _, text := range []string{`holder:"a\nb"`, `holder:a\b`, `holder:`, `substate:ready,`, `holder:"unterminated`} {
		if refusal := h.refuse(text); refusal.Name != contract.Malformed {
			t.Errorf("query %q refused %s, want %s", text, refusal.Name, contract.Malformed)
		}
	}

	// A repeated field is legal and every one of its terms still has to hold.
	wantRefs(t, "state:doing state:done", h.ask("state:"+doing+" state:"+finished))
}

// TestAFieldTokenCarryingAQuotationMarkIsMalformed asserts the answer this card
// gives to its own open question. fchar admits no quotation mark, so a term
// written with its quotes around the whole thing has no derivation and dies at
// check 1 rather than reaching check 2 with a field token nobody typed.
//
// The backslash is the same clause read the other way. fchar does admit one, so
// a field token carrying a backslash has a derivation and reaches check 2,
// which names it back; only a value carrying an unquoted backslash is
// malformed. Both are asserted here because one clause settles both, and a
// later reading that widened either would have to break this test.
func TestAFieldTokenCarryingAQuotationMarkIsMalformed(t *testing.T) {
	h := newHarness(t)
	h.add("a card")
	for _, text := range []string{`"holder:alka"`, `"state:doing"`, `ho"lder:alka`} {
		refusal := h.refuse(text)
		if refusal.Name != contract.Malformed {
			t.Errorf("query %q refused %s, want %s", text, refusal.Name, contract.Malformed)
		}
		if refusal.Detail != text {
			t.Errorf("query %q named %q rather than the offending term", text, refusal.Detail)
		}
	}
	refusal := h.refuse(`\x:y`)
	if refusal.Name != contract.UnknownField {
		t.Errorf(`the term \x:y refused %s, want %s`, refusal.Name, contract.UnknownField)
	}
	if refusal.Detail != `\x` {
		t.Errorf(`the term \x:y named %q rather than its field token`, refusal.Detail)
	}
}

// TestTheEmptyValueAsksForAbsenceOnEveryFieldButAt asserts the second answer
// this card gives to its own open questions. Absence beats the vocabulary
// checks on the nine fields that take an equality operator, and at admits no
// empty value at all, because every recorded act carries a stamp.
func TestTheEmptyValueAsksForAbsenceOnEveryFieldButAt(t *testing.T) {
	h := newHarness(t)
	held := h.add("held")
	free := h.add("free")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: held, Holder: "alka"})
	h.library.Comment(&Request{Verb: "comment", Actor: "alka", Card: free, Text: "a note"})
	h.reopen()

	wantRefs(t, `holder:""`, h.ask(`holder:""`), free)
	wantRefs(t, `holder!=""`, h.ask(`holder!=""`), held)
	wantRefs(t, `block_kind:""`, h.ask(`block_kind:""`), held, free)
	// A comment act and a created act both moved the card nowhere, so both
	// carry no destination and both witness the absence query.
	wantRefs(t, `entered:""`, h.ask(`entered:""`), held, free)
	for _, text := range []string{`substate:""`, `event:""`, `state:""`, `workstream:""`} {
		h.ask(text)
	}
	if refusal := h.refuse(`at>=""`); refusal.Name != contract.Malformed {
		t.Errorf(`at>="" refused %s, want %s`, refusal.Name, contract.Malformed)
	}
}

// TestWorkstreamMatchesByMembershipAndItsRosterIsTheLiveCards asserts the
// membership rule, the complement rule for !=, and check 6 with its roster read
// from every live card rather than from the cards the other terms leave.
func TestWorkstreamMatchesByMembershipAndItsRosterIsTheLiveCards(t *testing.T) {
	h := newHarness(t)
	both := h.add("in a and b")
	sole := h.add("in c")
	none := h.add("in nothing")
	h.setWorkstreams(both, "a", "b")
	h.setWorkstreams(sole, "c")

	wantRefs(t, "workstream:a", h.ask("workstream:a"), both)
	wantRefs(t, "workstream!=a", h.ask("workstream!=a"), sole, none)
	wantRefs(t, "workstream:c", h.ask("workstream:c"), sole)
	wantRefs(t, "workstream!=c", h.ask("workstream!=c"), both, none)
	wantRefs(t, "workstream:c,a", h.ask("workstream:c,a"), both, sole)
	wantRefs(t, "workstream!=c,a", h.ask("workstream!=c,a"), none)
	wantRefs(t, `workstream:""`, h.ask(`workstream:""`), none)

	// The roster is every live card's list, so a workstream only a card the
	// other terms exclude lists is still a value the query may name.
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: sole, Holder: "alka"})
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: sole, State: doing})
	wantRefs(t, "state:intake workstream:c", h.ask("state:"+intake+" workstream:c"))

	for _, text := range []string{"workstream:d", "workstream!=d", "workstream:a,d"} {
		refusal := h.refuse(text)
		if refusal.Name != contract.UnknownValue {
			t.Errorf("query %q refused %s, want %s", text, refusal.Name, contract.UnknownValue)
		}
		if refusal.Detail != "d" {
			t.Errorf("query %q named %q rather than the offending value", text, refusal.Detail)
		}
		if refusal.Extra["legal"] != "a, b, c" {
			t.Errorf("query %q listed the roster as %q", text, refusal.Extra["legal"])
		}
	}
}

// TestAnArchivedCardIsOutOfReachOfAQuery asserts the scope of section 5: the
// live half of the cards collection is read and the archive is not, so an
// archived card is never returned and an identifier only it lists is not in the
// workstream roster.
func TestAnArchivedCardIsOutOfReachOfAQuery(t *testing.T) {
	h := newHarness(t)
	live := h.add("live")
	going := h.add("going")
	h.setWorkstreams(going, "gone")
	response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: going})
	if response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()

	wantRefs(t, "", h.ask(""), live)
	if refusal := h.refuse("workstream:gone"); refusal.Name != contract.UnknownValue {
		t.Errorf("workstream:gone refused %s, want %s", refusal.Name, contract.UnknownValue)
	}
	if legal := h.refuse("workstream:gone").Extra["legal"]; legal != "" {
		t.Errorf("the roster reported %q, and no live card lists a workstream", legal)
	}
}

// TestAQueryReadsALapsedClaimAsLapsed asserts that every card passes through
// Library.lapseRead before it is compared, so a claim whose lease has run out
// is reported as ready rather than as active.
func TestAQueryReadsALapsedClaimAsLapsed(t *testing.T) {
	h := newHarness(t)
	card := h.add("leased")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: card, Holder: "alka", Expires: time.Hour})
	wantRefs(t, "substate:active", h.ask("substate:active"), card)

	h.advance(2 * time.Hour)
	h.reopen()
	wantRefs(t, "substate:ready", h.ask("substate:ready"), card)
	wantRefs(t, "substate:active", h.ask("substate:active"))
}

// TestAQueryReturnsTheArrivalOrderAndRepeats asserts that an empty query
// returns the whole live set in the order CORE-QUEUE-3 fixes, and that two runs
// over an unchanged workbench return an identical sequence.
func TestAQueryReturnsTheArrivalOrderAndRepeats(t *testing.T) {
	h := newHarness(t)
	first := h.add("first")
	second := h.add("second")
	third := h.add("third")
	// A card's arrival is when it reached the state it stands in, so moving
	// the first card an hour later sends it to the back. The expected
	// sequence is therefore the one ByArrival produces and not the ascending
	// identifier order Bench.Cards reads the directory in, which is what
	// makes this assertion able to fail.
	h.advance(time.Hour)
	h.reopen()
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: first, Holder: "alka"})
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: first, State: doing})

	wantRefs(t, "", h.ask(""), second, third, first)
	wantRefs(t, "", h.ask(""), second, third, first)
	wantRefs(t, "actor:alka", h.ask("actor:alka"), second, third, first)
}

// TestAQueryEchoesItsArgumentAsReceived asserts that Matches.Query carries the
// string the caller passed, byte for byte, rather than the string the parser
// trimmed, so a caller comparing a stored result against what it sent finds its
// own string there.
func TestAQueryEchoesItsArgumentAsReceived(t *testing.T) {
	h := newHarness(t)
	h.add("a card")
	for _, text := range []string{" actor:alka ", " ", "", "\tactor:alka"} {
		if got := h.ask(text).Query; got != text {
			t.Errorf("query %q echoed %q", text, got)
		}
	}
	spaced := h.ask(" actor:alka ")
	tight := h.ask("actor:alka")
	if len(spaced.Cards) != len(tight.Cards) || spaced.Count != tight.Count {
		t.Errorf("the trimmed and untrimmed forms selected %v and %v", refs(spaced), refs(tight))
	}
}

// TestAQueryMatchingNothingCarriesAnEmptyArray asserts that a result selecting
// no card carries cards as an empty array rather than as null, which is what
// Listing already does and what a reader decoding the document relies on.
func TestAQueryMatchingNothingCarriesAnEmptyArray(t *testing.T) {
	h := newHarness(t)
	h.add("a card")
	matches := h.ask("holder:nobody")
	if matches.Cards == nil {
		t.Error("a query matching nothing carried a nil cards member")
	}
	if matches.Count != 0 {
		t.Errorf("a query matching nothing reported a count of %d", matches.Count)
	}
}

// TestTheStateFieldsCompareOnTheResolvedIdentifier asserts that state, entered
// and left accept a slug or a title and compare on what it resolved to, so a
// query written against a slug keeps working when the state is renamed.
func TestTheStateFieldsCompareOnTheResolvedIdentifier(t *testing.T) {
	h := newHarness(t)
	card := h.add("moving")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: card, Holder: "alka"})
	h.mustDo(&Request{Verb: Move, Actor: "alka", Card: card, State: aftercareSlug})

	for _, text := range []string{"state:" + aftercareSlug, "state:" + aftercare, "state:Aftercare"} {
		wantRefs(t, text, h.ask(text), card)
	}
	wantRefs(t, "left:"+intake, h.ask("left:"+intake), card)
	wantRefs(t, "entered:"+aftercareSlug, h.ask("entered:"+aftercareSlug), card)
}

// TestAQueryCarryingTwoMistakesIsRefusedForTheEarlier asserts the one thing
// about the check list that is normative rather than presentational: the order.
// A second implementation's output is comparable only if two tools shown the
// same wrong query name the same mistake, so each pair below carries two
// mistakes at once and the earlier check has to win.
func TestAQueryCarryingTwoMistakesIsRefusedForTheEarlier(t *testing.T) {
	h := newHarness(t)
	card := h.add("a card")
	h.setWorkstreams(card, "a")
	for _, pair := range []struct {
		text string
		want string
	}{
		{"doing Priority>=next", contract.Malformed},
		{"Priority>=next substate:reday", contract.UnknownField},
		{"at:2026-08-01 substate:reday", contract.UnknownField},
		{"substate:reday state:nosuchstate", contract.UnknownValue},
		{"state:nosuchstate workstream:d", contract.UnknownState},
	} {
		if refusal := h.refuse(pair.text); refusal.Name != pair.want {
			t.Errorf("query %q refused %s, want the earlier %s", pair.text, refusal.Name, pair.want)
		}
	}
	// The catalog key of each row carries its own order, so a row moved in the
	// list without its key moving with it prints a sentence for the wrong
	// position in the help block.
	checks := Checks("query")
	if len(checks) != 6 {
		t.Fatalf("the query command declares %d checks, and the spec's section 10 fixes six", len(checks))
	}
	for i, check := range checks {
		if want := CheckKey("query", i+1); check.Key != want {
			t.Errorf("check %d carries the key %s, want %s", i+1, check.Key, want)
		}
	}
}
