package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// renameCardID renames a card's directory to a chosen 12-hex ID and rewrites
// the number registry's own line for it so the card's human reference keeps
// resolving. It is what lets a test force an ID that sorts lexically
// opposite the card's own arrival order: Bench.Cards() walks CardsRoot in
// os.ReadDir's own order, which Go sorts by filename (the ID), and nothing
// in the harness's ordinary card-creation path lets a caller choose that
// filename directly.
func (h *harness) renameCardID(oldID, newID string) {
	h.t.Helper()
	root := h.library.Bench.CardsRoot()
	if err := os.Rename(filepath.Join(root, oldID), filepath.Join(root, newID)); err != nil {
		h.t.Fatalf("rename card %s to %s: %v", oldID, newID, err)
	}
	path := filepath.Join(h.library.Bench.Root, bench.CardNumbersName)
	registry := bench.LoadNumberRegistry(path)
	lines := make([]string, len(registry.Lines))
	rewritten := false
	for at, line := range registry.Lines {
		if line.ID != oldID {
			lines[at] = line.Raw
			continue
		}
		lines[at] = strconv.Itoa(line.Number) + " " + newID
		rewritten = true
	}
	if !rewritten {
		h.t.Fatalf("renameCardID %s: the registry names no line for it", oldID)
	}
	if err := bench.WriteNumberLines(path, lines); err != nil {
		h.t.Fatalf("renameCardID %s: %v", oldID, err)
	}
	h.reopen()
}

// growPrimeColumnInstructions rewrites a column's own instructions text to
// at least n bytes of filler. It is what dinah-573/criteria/22's cap-engaged
// fixture needs: the byte advantage the specification measures comes from
// the column text and the column's attachment listing, which Prime never
// carries and instructions always does for the position it is asked about,
// rather than from the standing text, which both a served chain and Prime
// carry exactly once each.
func growPrimeColumnInstructions(t *testing.T, h *harness, columnID string, n int) {
	t.Helper()
	path := filepath.Join(h.root, bench.ColumnsDir, columnID, bench.ColumnAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the column anchor: %v", err)
	}
	fm, _ := bench.ParseAnchor(text)
	filler := strings.Repeat("Filler text for the measured-size fixture. ", n/44+1)
	if err := bench.WriteText(path, fm.Render(filler)); err != nil {
		t.Fatalf("write the column anchor: %v", err)
	}
	h.reopen()
}

// writePrimeGlobal writes the user-global instruction layer under a
// harness's own home, which is the one layer bench.GlobalInstructions reads
// from disk on each serve.
func writePrimeGlobal(t *testing.T, h *harness, text string) {
	t.Helper()
	dir := bench.UserBase(h.home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("the user base: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, bench.InstructionsName), []byte(text), 0o644); err != nil {
		t.Fatalf("the global layer: %v", err)
	}
}

// mustPrime runs Prime and fails the test unless it answered without error.
func mustPrime(h *harness, actor string) *Primer {
	h.t.Helper()
	primer, err := h.library.Prime(&Request{Verb: "prime", Actor: actor})
	if err != nil {
		h.t.Fatalf("prime: %v", err)
	}
	return primer
}

// filePrimeItem is File with the fields dinah-573's own fixtures need,
// failing the test unless the write succeeded. It differs from the
// package's own fileItem (fields_guard_test.go) in taking an owner and a
// column, which every Pending fixture here has to set.
func filePrimeItem(h *harness, card, kind, text, owner, column string) {
	h.t.Helper()
	response := h.library.File(&Request{
		Verb: "file", Actor: "alka", Card: card,
		Kind: kind, Text: text, Owner: owner, Column: column,
	})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("file %s on %s: %s %s", kind, card, response.Outcome, response.Refusal)
	}
	h.reopen()
}

// TestPrimeIdentityIsWhoamisOwnAnswer is dinah-573/criteria/1: Prime.Identity,
// for a given actor, harness, provider, model and server, is byte-identical
// to the same request's Whoami answer.
func TestPrimeIdentityIsWhoamisOwnAnswer(t *testing.T) {
	h := newHarness(t)
	req := &Request{Verb: "whoami", Actor: "brin", Harness: "claude-code", Provider: "acme", Model: "workhorse", Server: "acme.example"}
	identity, err := h.library.Whoami(req)
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	req.Verb = "prime"
	primer, err := h.library.Prime(req)
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	want, err := json.Marshal(identity)
	if err != nil {
		t.Fatalf("marshal whoami: %v", err)
	}
	got, err := json.Marshal(primer.Identity)
	if err != nil {
		t.Fatalf("marshal prime.identity: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Prime.Identity is %s, wanted Whoami's own answer %s", got, want)
	}
}

// TestPrimeHoldingIsStatussOwnAnswer is dinah-573/criteria/2: Prime.Holding,
// for a given actor, is byte-identical to the same request's Status.Holding.
func TestPrimeHoldingIsStatussOwnAnswer(t *testing.T) {
	h := newHarness(t)
	first := h.ready("first card")
	second := h.ready("second card")
	h.mustDo(&Request{Verb: Claim, Actor: "brin", Card: first})
	h.mustDo(&Request{Verb: Claim, Actor: "brin", Card: second})

	status, err := h.library.Status(&Request{Verb: "status", Actor: "brin"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	primer := mustPrime(h, "brin")

	want, err := json.Marshal(status.Holding)
	if err != nil {
		t.Fatalf("marshal status.holding: %v", err)
	}
	got, err := json.Marshal(primer.Holding)
	if err != nil {
		t.Fatalf("marshal prime.holding: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Prime.Holding is %s, wanted Status.Holding's own answer %s", got, want)
	}
	if len(status.Holding) != 2 {
		t.Fatalf("the fixture holds %d cards, wanted 2", len(status.Holding))
	}
}

// TestPrimeReadyIsFilteredAndCounted is dinah-573/criteria/3,4,6: a column
// carrying a ready card produces exactly one Ready entry whose fields match
// Next's own answer for that column and whose ReadyCount equals the number
// of ready cards standing there; a column carrying nothing ready produces no
// entry at all; and an actor with nothing ready anywhere gets Ready == [].
func TestPrimeReadyIsFilteredAndCounted(t *testing.T) {
	h := newHarness(t)

	// Nothing filed yet: Ready is the empty array, not null.
	empty := mustPrime(h, "brin")
	if empty.Ready == nil || len(empty.Ready) != 0 {
		t.Fatalf("Ready with nothing filed: wanted an empty array, got %#v", empty.Ready)
	}
	if encoded, err := json.Marshal(empty.Ready); err != nil || string(encoded) != "[]" {
		t.Errorf("Ready encodes as %s (err %v), wanted []", encoded, err)
	}

	// One card ready at intake, nothing ready anywhere else.
	h.add("ready at intake")
	offers, err := h.library.Next(&Request{Verb: "next", Actor: "brin"})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	primer := mustPrime(h, "brin")
	if len(primer.Ready) != 1 {
		t.Fatalf("Ready carries %d entries, wanted exactly 1 (intake): %+v", len(primer.Ready), primer.Ready)
	}
	entry := primer.Ready[0]
	if entry.Column != intake {
		t.Errorf("the one Ready entry names %s, wanted intake %s", entry.Column, intake)
	}
	next := offerAt(t, offers, intake)
	if entry.Card == nil || next.Card == nil || entry.Card.Ref != next.Card.Ref {
		t.Errorf("Prime.Ready's intake entry carries %+v, Next's carries %+v", entry.Card, next.Card)
	}
	if entry.Landing != next.Landing || entry.TakenByPull != next.TakenByPull {
		t.Errorf("Prime.Ready's intake entry (landing %q, pull %v) disagrees with Next's (landing %q, pull %v)",
			entry.Landing, entry.TakenByPull, next.Landing, next.TakenByPull)
	}
	if entry.ReadyCount != 1 {
		t.Errorf("ReadyCount is %d, wanted 1", entry.ReadyCount)
	}
	for _, column := range []string{doing, review, aftercare, finished, closed} {
		for _, offer := range primer.Ready {
			if offer.Column == column {
				t.Errorf("Prime.Ready carries an entry for %s, which offers nothing", column)
			}
		}
	}
}

// TestPrimeReadyAboveTierMatchesNext is dinah-573/criteria/5: a column
// carrying ready work above the caller's resolved tier produces an entry
// whose AboveTier, RequiredTier and SatisfiedBy match Next's own answer, and
// whose Card is absent.
func TestPrimeReadyAboveTierMatchesNext(t *testing.T) {
	h := tieredHarness(t)
	requiring(h, "assessed", "workhorse")

	req := runningAs("brin", "minimal")
	req.Verb = "next"
	offers, err := h.library.Next(req)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	withheld := offerAt(t, offers, aftercare)
	if !withheld.AboveTier {
		t.Fatalf("next's own offer does not withhold the card: %+v", withheld)
	}

	req.Verb = "prime"
	primer, err := h.library.Prime(req)
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	var entry *Offer
	for i := range primer.Ready {
		if primer.Ready[i].Column == aftercare {
			entry = &primer.Ready[i]
		}
	}
	if entry == nil {
		t.Fatalf("Prime.Ready carries no entry for aftercare: %+v", primer.Ready)
	}
	if !entry.AboveTier || entry.Card != nil {
		t.Errorf("the aftercare entry is %+v, wanted AboveTier and no card", entry)
	}
	if entry.RequiredTier != withheld.RequiredTier {
		t.Errorf("RequiredTier is %q, wanted %q", entry.RequiredTier, withheld.RequiredTier)
	}
	wantSatisfied, err := json.Marshal(withheld.SatisfiedBy)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	gotSatisfied, err := json.Marshal(entry.SatisfiedBy)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(gotSatisfied) != string(wantSatisfied) {
		t.Errorf("SatisfiedBy is %s, wanted %s", gotSatisfied, wantSatisfied)
	}
}

// TestPrimeReadyCountAlsoReachesNextCard is dinah-573/criteria/23:
// Offer.ReadyCount is present and correct on next_card's own answer, not
// only on prime's, confirming the addition is genuinely shared.
func TestPrimeReadyCountAlsoReachesNextCard(t *testing.T) {
	h := newHarness(t)
	h.add("first")
	h.add("second")
	offers, err := h.library.Next(&Request{Verb: "next", Actor: "brin"})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	entry := offerAt(t, offers, intake)
	if entry.ReadyCount != 2 {
		t.Errorf("next_card's own ReadyCount for intake is %d, wanted 2", entry.ReadyCount)
	}
}

// TestPrimePendingHolderRule is dinah-573/criteria/7,8: an item filed
// --owner holder, pending, on a card this actor holds appears in
// Prime.Pending, whatever its kind; the identical item filed on a card this
// actor does not hold does not appear; and for a non-operator actor,
// Prime.Pending never carries an item whose Owner is operator or empty, even
// on a card this actor holds.
func TestPrimePendingHolderRule(t *testing.T) {
	h := newHarness(t)
	held := h.ready("held card")
	notHeld := h.ready("not held card")
	h.mustDo(&Request{Verb: Claim, Actor: "brin", Card: held})

	filePrimeItem(h, held, "acceptance_criterion", "verify the endpoint", "holder", "")
	filePrimeItem(h, notHeld, "acceptance_criterion", "verify elsewhere", "holder", "")
	filePrimeItem(h, held, "open_question", "operator's own question", "operator", "")
	filePrimeItem(h, held, "decision", "unstamped decision", "", "")

	primer := mustPrime(h, "brin")
	if len(primer.Pending) != 1 {
		t.Fatalf("Pending carries %d items, wanted exactly the one holder-owned item: %+v", len(primer.Pending), primer.Pending)
	}
	item := primer.Pending[0]
	if item.Card != held || item.Owner != "holder" || item.Text != "verify the endpoint" {
		t.Errorf("the one Pending item is %+v, wanted the holder-owned item on the held card", item)
	}
}

// TestPrimePendingOperatorRule is dinah-573/criteria/9,10,11: for the
// operator, Prime.Pending carries every pending open_question or decision
// item whose Owner is operator or empty, on every live card regardless of
// who holds it, and carries no acceptance_criterion under this rule; and an
// actor with nothing pending anywhere gets Pending == [], PendingWithheld
// absent, and PendingByColumn absent.
func TestPrimePendingOperatorRule(t *testing.T) {
	h := newHarness(t)

	// The operator, alka, with nothing filed anywhere: the three absences.
	empty := mustPrime(h, "alka")
	if empty.Pending == nil || len(empty.Pending) != 0 {
		t.Fatalf("Pending with nothing filed: wanted an empty array, got %#v", empty.Pending)
	}
	if empty.PendingWithheld != 0 {
		t.Errorf("PendingWithheld is %d, wanted 0 (absent)", empty.PendingWithheld)
	}
	if empty.PendingByColumn != nil {
		t.Errorf("PendingByColumn is %#v, wanted nil (absent)", empty.PendingByColumn)
	}
	if encoded, err := json.Marshal(empty); err != nil {
		t.Fatalf("marshal: %v", err)
	} else if strings.Contains(string(encoded), "pending_withheld") || strings.Contains(string(encoded), "pending_by_column") {
		t.Errorf("the encoded Primer carries pending_withheld or pending_by_column with nothing to report: %s", encoded)
	}

	card := h.ready("a card nobody claims")
	filePrimeItem(h, card, "open_question", "an unstamped question", "", "")
	filePrimeItem(h, card, "decision", "an operator-owned decision", "operator", "")
	filePrimeItem(h, card, "acceptance_criterion", "an unstamped criterion", "", "")

	// A non-operator actor sees none of this, since rule 2 never fires for
	// it and none of the three items are stamped --owner holder.
	nonOperator := mustPrime(h, "brin")
	if len(nonOperator.Pending) != 0 {
		t.Errorf("a non-operator actor's Pending carries %d items, wanted 0: %+v", len(nonOperator.Pending), nonOperator.Pending)
	}

	primer := mustPrime(h, "alka")
	if len(primer.Pending) != 2 {
		t.Fatalf("the operator's Pending carries %d items, wanted the two open_question/decision items: %+v", len(primer.Pending), primer.Pending)
	}
	for _, item := range primer.Pending {
		if item.Kind == "acceptance_criterion" {
			t.Errorf("the operator's Pending carries an acceptance_criterion under rule 2: %+v", item)
		}
	}

	// dinah-599 criteria/2: the number rule 2 lists for the operator, cap
	// lifted, agrees with the card's own operator_pending, because both read
	// bench.ItemAwaitsOperator over the same items.
	full, err := h.library.Prime(&Request{Verb: "prime", Actor: "alka", FullPending: true})
	if err != nil {
		t.Fatalf("prime with the cap lifted: %v", err)
	}
	view, err := h.library.viewToday(h.card(card))
	if err != nil {
		t.Fatalf("view the card: %v", err)
	}
	counted := 0
	for _, item := range full.Pending {
		if item.Card == view.Ref {
			counted++
		}
	}
	if counted != view.OperatorPending {
		t.Errorf("prime lists %d items for this card, wanted operator_pending's own %d", counted, view.OperatorPending)
	}
}

// TestPrimePendingRuleTwoCallsTheSharedPredicate drives dinah-599 criteria/2's
// second half: primePending's rule 2 calls bench.ItemAwaitsOperator rather
// than repeating its condition inline, on the terms
// TestLibraryViewReachesNoPerCollectionCount already checks a call by name in
// this package. A rule reading item.State, item.Kind and item.Owner directly
// instead would pass every behavioural case above and still drift from the
// card view's own count the day one of the two copies is edited alone.
//
// Arming: restoring the three-clause inline condition primePending carried
// before dinah-599 reddens this test by name, because the walk from
// primePending then reaches no call to ItemAwaitsOperator at all.
func TestPrimePendingRuleTwoCallsTheSharedPredicate(t *testing.T) {
	graph := packageCallGraph(t)
	reachable := reachableFrom(graph, "primePending")
	if !reachable["primePending"] {
		t.Fatal("the walk does not reach primePending itself, so this guard is looking at the wrong graph")
	}
	calls := map[string]int{}
	for name := range reachable {
		for _, called := range graph[name] {
			calls[called]++
		}
	}
	if calls["ItemAwaitsOperator"] == 0 {
		t.Error("ItemAwaitsOperator is called nowhere primePending reaches, and rule 2 is supposed to call it rather than repeat its condition")
	}
}

// TestPrimePendingCapAndFullPending is dinah-573/criteria/19,20,21: for the
// operator, a workbench carrying more than 20 items matching rule 2 places
// exactly the first 20 into Pending, with the rest counted into
// PendingWithheld and every named column counted in full into
// PendingByColumn; full-pending removes the cap and leaves PendingWithheld
// absent; and full-pending changes nothing for a non-operator actor.
func TestPrimePendingCapAndFullPending(t *testing.T) {
	h := newHarness(t)
	const total = 23
	for i := 0; i < total; i++ {
		card := h.ready("card")
		filePrimeItem(h, card, "open_question", "question", "", "review")
	}

	capped := mustPrime(h, "alka")
	if len(capped.Pending) != 20 {
		t.Fatalf("capped Pending carries %d items, wanted 20", len(capped.Pending))
	}
	if capped.PendingWithheld != total-20 {
		t.Errorf("PendingWithheld is %d, wanted %d", capped.PendingWithheld, total-20)
	}
	columnTotal := 0
	for _, column := range capped.PendingByColumn {
		columnTotal += column.Count
	}
	if columnTotal != total {
		t.Errorf("PendingByColumn's counts sum to %d, wanted the full match total %d", columnTotal, total)
	}

	full, err := h.library.Prime(&Request{Verb: "prime", Actor: "alka", FullPending: true})
	if err != nil {
		t.Fatalf("prime --full-pending: %v", err)
	}
	if len(full.Pending) != total {
		t.Fatalf("full-pending's Pending carries %d items, wanted all %d", len(full.Pending), total)
	}
	if full.PendingWithheld != 0 {
		t.Errorf("full-pending's PendingWithheld is %d, wanted 0 (absent)", full.PendingWithheld)
	}

	// full-pending changes nothing for a non-operator actor: rule 2 never
	// contributes to that actor's Pending whether or not the marker is set.
	plainOther, err := h.library.Prime(&Request{Verb: "prime", Actor: "brin"})
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	fullOther, err := h.library.Prime(&Request{Verb: "prime", Actor: "brin", FullPending: true})
	if err != nil {
		t.Fatalf("prime --full-pending: %v", err)
	}
	if len(plainOther.Pending) != 0 || len(fullOther.Pending) != 0 {
		t.Errorf("a non-operator actor's Pending is non-empty: plain %+v, full-pending %+v", plainOther.Pending, fullOther.Pending)
	}

	// dinah-573/criteria/22's second fixture: even with the cap engaged, the
	// operator's capped prime payload is still smaller than the sum of the
	// three reads it replaces. The margin the specification measures comes
	// from the standing text prime never repeats per pending item and status
	// never repeats per column, so this fixture gives the standing layer real
	// weight rather than the one-sentence text the bare harness carries.
	growPrimeColumnInstructions(t, h, intake, 4000)
	identity, err := h.library.Whoami(&Request{Verb: "whoami", Actor: "alka"})
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	status, err := h.library.Status(&Request{Verb: "status", Actor: "alka"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	served, err := h.library.Instructions(&Request{Verb: "instructions", Actor: "alka", Card: "intake"})
	if err != nil {
		t.Fatalf("instructions: %v", err)
	}
	sum := jsonSize(t, identity) + jsonSize(t, status) + jsonSize(t, served)
	if got := jsonSize(t, capped); got >= sum {
		t.Errorf("with the cap engaged, prime's payload is %d bytes, wanted smaller than the %d bytes whoami+status+instructions sum to", got, sum)
	}
}

// TestPrimeRefusesWithNoActor is dinah-573/criteria/15: prime with no actor
// resolved refuses dinah.no-owner, carrying the declared harness in context
// when one was declared, matching Whoami's own refusal for the identical
// request.
func TestPrimeRefusesWithNoActor(t *testing.T) {
	h := newHarness(t)
	req := &Request{Verb: "whoami", Harness: "claude-code"}
	_, wantErr := h.library.Whoami(req)
	req.Verb = "prime"
	_, gotErr := h.library.Prime(req)
	if wantErr == nil || gotErr == nil {
		t.Fatalf("wanted both calls refused, got whoami=%v prime=%v", wantErr, gotErr)
	}
	if wantErr.Error() != gotErr.Error() {
		t.Errorf("prime's refusal (%v) does not match whoami's own (%v) for the identical request", gotErr, wantErr)
	}
	if refusal, ok := gotErr.(*contract.Refusal); ok {
		if refusal.Name != contract.NoOwner {
			t.Errorf("prime refused %s, wanted %s", refusal.Name, contract.NoOwner)
		}
	} else {
		t.Errorf("prime's error is not a *contract.Refusal: %#v", gotErr)
	}
}

// TestPrimeParametersCarryNoEntryBeyondTheDeclaredTwo is
// dinah-573/criteria/18: prime's declared parameter list carries no entries
// beyond full-pending, brief and the injected properties; it carries no
// root, max-depth or basis.
func TestPrimeParametersCarryNoEntryBeyondTheDeclaredTwo(t *testing.T) {
	params := Params("prime")
	if len(params) != 2 {
		t.Fatalf("prime declares %d parameters, wanted 2: %+v", len(params), params)
	}
	byName := map[string]Param{}
	for _, p := range params {
		byName[p.Name] = p
	}
	fullPending, ok := byName["full-pending"]
	if !ok || !fullPending.Marker || fullPending.Field != "FullPending" {
		t.Errorf("full-pending is declared as %+v, wanted a marker naming FullPending", fullPending)
	}
	brief, ok := byName["brief"]
	if !ok || !brief.Marker || brief.Field != "Brief" {
		t.Errorf("brief is declared as %+v, wanted a marker naming Brief", brief)
	}
	for _, unwanted := range []string{"root", "max-depth", "basis"} {
		if _, declared := byName[unwanted]; declared {
			t.Errorf("prime declares %s, which the specification says it must not", unwanted)
		}
	}
}

// TestPrimeIsSmallerThanTheThreeReadsItReplaces is dinah-573/criteria/17,22:
// on a fixture where an actor holds exactly one live card, a prime payload
// is smaller than the combined byte count of whoami, status and instructions
// for the same card, on the same fixture, for a non-operator actor and for
// the operator alike.
func TestPrimeIsSmallerThanTheThreeReadsItReplaces(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("the one card held")
	h.mustDo(&Request{Verb: Claim, Actor: "brin", Card: ref})

	for _, actor := range []string{"brin", "alka"} {
		identity, err := h.library.Whoami(&Request{Verb: "whoami", Actor: actor})
		if err != nil {
			t.Fatalf("whoami: %v", err)
		}
		status, err := h.library.Status(&Request{Verb: "status", Actor: actor})
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		served, err := h.library.Instructions(&Request{Verb: "instructions", Actor: actor, Card: ref})
		if err != nil {
			t.Fatalf("instructions: %v", err)
		}
		primer, err := h.library.Prime(&Request{Verb: "prime", Actor: actor})
		if err != nil {
			t.Fatalf("prime: %v", err)
		}

		sum := jsonSize(t, identity) + jsonSize(t, status) + jsonSize(t, served)
		got := jsonSize(t, primer)
		if got >= sum {
			t.Errorf("%s: prime's payload is %d bytes, wanted smaller than the %d bytes whoami+status+instructions sum to", actor, got, sum)
		}
	}
}

// jsonSize is the byte size of a value's canonical JSON encoding, on the
// terms --json emits it (two-space indent), which is what the specification's
// own "Measured size" table compares.
func jsonSize(t *testing.T, value any) int {
	t.Helper()
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return len(encoded)
}

// TestPrimeInstructionsWithNoHeldChainAlwaysServesInFull is dinah-573/
// criteria/14's library-level half: the CLI never populates req.HeldChain
// on any call, and Library.Prime carries no memory of its own between
// requests, so two independent Prime calls with no HeldChain both carry
// Global and Standing in full.
func TestPrimeInstructionsWithNoHeldChainAlwaysServesInFull(t *testing.T) {
	h := newHarness(t)
	ref := h.ready("held")
	h.mustDo(&Request{Verb: Claim, Actor: "brin", Card: ref})
	for i := 0; i < 2; i++ {
		primer, err := h.library.Prime(&Request{Verb: "prime", Actor: "brin"})
		if err != nil {
			t.Fatalf("call %d: prime: %v", i, err)
		}
		if primer.Instructions.Standing == "" {
			t.Errorf("call %d: Standing was not served in full: %+v", i, primer.Instructions)
		}
		if len(primer.Instructions.Withheld) != 0 {
			t.Errorf("call %d: withheld something with no HeldChain: %+v", i, primer.Instructions.Withheld)
		}
	}
}

// TestPrimeBriefOmitsInstructionsAndPlainDoesNot is dinah-573/criteria/24,25:
// on the command line, prime with no --brief carries Global and Standing in
// full, and prime --brief carries neither, naming both Withheld with no
// Reread, matching the pointer-only shape.
func TestPrimeBriefOmitsInstructionsAndPlainDoesNot(t *testing.T) {
	h := newHarness(t)
	writePrimeGlobal(t, h, "Global text.\n")
	ref := h.ready("held")
	h.mustDo(&Request{Verb: Claim, Actor: "brin", Card: ref})

	plain, err := h.library.Prime(&Request{Verb: "prime", Actor: "brin"})
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	if plain.Instructions.Global == "" || plain.Instructions.Standing == "" || len(plain.Instructions.Withheld) != 0 {
		t.Errorf("the plain call is %+v, wanted Global and Standing in full and nothing withheld", plain.Instructions)
	}

	brief, err := h.library.Prime(&Request{Verb: "prime", Actor: "brin", Brief: true})
	if err != nil {
		t.Fatalf("prime --brief: %v", err)
	}
	if brief.Instructions.Global != "" || brief.Instructions.Standing != "" {
		t.Errorf("--brief carried text: %+v", brief.Instructions)
	}
	if strings.Join(brief.Instructions.Withheld, ",") != strings.Join([]string{LayerGlobal, LayerStanding}, ",") {
		t.Errorf("--brief's Withheld is %v, wanted both layers named", brief.Instructions.Withheld)
	}
	if brief.Instructions.Reread != "" {
		t.Errorf("--brief carried a Reread reference %q, wanted none: the CLI keeps no session memory to recover from", brief.Instructions.Reread)
	}
}

// TestPrimeMCPProfileMembership is a sanity check on decision 6: prime is
// not the operator's alone, so a non-operator actor's Prime.Pending, unlike
// Status.Columns, reaches only its own cards and the columns admission
// already scopes to it. The MCP surface's own profile placement (dinah-573/
// criteria/16) is asserted in internal/mcp, which is where ProfileStation
// and ProfileOperator are declared; this test asserts the library-level fact
// that placement rests on, that a non-operator actor's Prime carries nothing
// workbench-wide the way Status.Columns does.
func TestPrimeMCPProfileMembership(t *testing.T) {
	h := newHarness(t)
	primer := mustPrime(h, "brin")
	encoded, err := json.Marshal(primer)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), `"columns"`) {
		t.Errorf("Prime's answer carries a columns member, which is Status.Columns' own workbench-wide occupancy table: %s", encoded)
	}
}

// TestPrimeRereadNamesTheEarliestArrivalHeldCardNotTheFirstByID is
// dinah-573/criteria/12's own claim, read literally: Reread names the
// column of the earliest-arrival card in Holding, not of whichever held
// card Bench.Cards() happens to list first. Bench.Cards() walks
// CardsRoot in os.ReadDir's own order, sorted by the card's random ID,
// which bench.ByArrival's own doc comment says is deliberately
// meaningless as an ordering; Holding is built in that same order,
// unchanged, because AC2 pins it byte-identical to Status.Holding. This
// fixture forces the two orders to disagree: the earlier-arrival card
// (doing) gets the lexically later ID and the later-arrival card
// (aftercare) gets the lexically earlier one, so holding[0] names
// aftercare while the true earliest arrival is doing. Reviewer finding on
// this card's first Agent Code Review pass: the prior code read
// holding[0].Column directly, which this fixture catches and the
// single-held-card fixture in TestPrimeWithholdsOnASecondCallAndRecovers
// ByColumn (internal/mcp/prime_test.go) cannot.
func TestPrimeRereadNamesTheEarliestArrivalHeldCardNotTheFirstByID(t *testing.T) {
	h := newHarness(t)
	earlierArrival := h.readyAt("arrives first, claimed into doing", doing)
	h.advance(time.Hour)
	laterArrival := h.readyAt("arrives second, claimed into aftercare", aftercare)
	h.mustDo(&Request{Verb: Claim, Actor: "brin", Card: earlierArrival})
	h.mustDo(&Request{Verb: Claim, Actor: "brin", Card: laterArrival})

	// Force the ID order to disagree with the arrival order: the
	// later-arrival card gets the lexically smaller ID, so a
	// holding[0]-based read names its column (aftercare) instead of the
	// earlier-arrival card's own (doing).
	h.renameCardID(h.card(earlierArrival).ID, "ffffffffffff")
	h.renameCardID(h.card(laterArrival).ID, "000000000001")

	// A connection that has already been served the standing layer in
	// full withholds it on this call, which is what gives Reread
	// something to name.
	req := &Request{Verb: "prime", Actor: "brin"}
	req.HeldChain = map[string]bool{
		chainKey("brin", h.library.Bench.Standing): true,
	}
	primer, err := h.library.Prime(req)
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	if len(primer.Instructions.Withheld) == 0 {
		t.Fatalf("nothing was withheld, so Reread proves nothing here: %+v", primer.Instructions)
	}
	if primer.Instructions.Reread != "doing" {
		t.Errorf("Reread is %q, wanted %q: the earliest-arrival held card stands at doing, not at aftercare, which only the later-arrival card's (wrongly) lexically-first ID would produce",
			primer.Instructions.Reread, "doing")
	}
	// Holding's own order is unaffected: it still lists cards in
	// Bench.Cards()' own (ID-sorted) order, matching AC2.
	if len(primer.Holding) != 2 || primer.Holding[0].Ref != laterArrival {
		t.Errorf("Holding is %+v, wanted the later-arrival card first (its ID now sorts first), unchanged by the Reread fix", primer.Holding)
	}
}
