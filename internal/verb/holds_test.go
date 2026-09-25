package verb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The columns of holdsDefinition, which the holds tests name directly.
const (
	hIntake      = "c00000000001"
	hTriage      = "c00000000002"
	hDesignQueue = "c00000000003"
	hBuildQueue  = "c00000000004"
	hImplement   = "c00000000005"
	hAcceptance  = "c00000000006"
	hDone        = "c00000000007"
)

// holdsDefinition is a flow shaped like the Dinah development workbench's:
// an intake, a station that claims before commitment, two queues, the
// working station the tests declare as the commitment column, a station
// after it, and a done column.
const holdsDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Holds",
  "instructions": "The standing text.\n",
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake", "instructions": "i\n" },
    { "id": "c00000000002", "title": "Triage", "kind": "work", "instructions": "i\n" },
    { "id": "c00000000003", "title": "Design Queue", "kind": "dinah.buffer", "instructions": "i\n" },
    { "id": "c00000000004", "title": "Build Queue", "kind": "dinah.buffer", "instructions": "i\n" },
    { "id": "c00000000005", "title": "Implement", "kind": "work", "instructions": "i\n" },
    { "id": "c00000000006", "title": "Acceptance", "kind": "work", "instructions": "i\n" },
    { "id": "c00000000007", "title": "Done", "kind": "done", "instructions": "i\n" }
  ]
}`

// The columns of initDefinition, the flow dinah init creates.
const (
	iIntake = "d00000000001"
	iDoing  = "d00000000002"
	iDone   = "d00000000003"
)

// initDefinition is the flow dinah init creates: Intake, Doing, Done.
const initDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Init",
  "instructions": "The standing text.\n",
  "columns": [
    { "id": "d00000000001", "title": "Intake", "kind": "intake", "instructions": "i\n" },
    { "id": "d00000000002", "title": "Doing", "kind": "work", "instructions": "i\n" },
    { "id": "d00000000003", "title": "Done", "kind": "done", "instructions": "i\n" }
  ]
}`

// holdsToday is the day the holds tests run on, in the workbench's zone.
var holdsToday = time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)

// holdsHarness is holdsDefinition at holdsToday with the holds block given,
// whole, written to the workbench anchor.
func holdsHarness(t *testing.T, block string) *harness {
	t.Helper()
	h := harnessFromDefinition(t, "hd", holdsDefinition)
	h.clock = holdsToday
	h.declareHolds(block)
	return h
}

// initHarness is initDefinition at holdsToday with the holds block given.
func initHarness(t *testing.T, block string) *harness {
	t.Helper()
	h := harnessFromDefinition(t, "wed", initDefinition)
	h.clock = holdsToday
	h.declareHolds(block)
	return h
}

// declareHolds writes a dinah.holds block, given whole, or removes it where
// the block is empty.
func (h *harness) declareHolds(block string) {
	h.t.Helper()
	h.writeAnchorBlock(bench.HoldsKey, block)
}

// link writes one link and fails the test unless it was written, answering
// the response so a case can read its warning.
func (h *harness) link(from, kind, to string) *Response {
	h.t.Helper()
	response := h.library.Link(&Request{Verb: "link", Actor: "alka", Card: from, Kind: kind, LinkTo: to})
	if response.Outcome != contract.OutcomeOK {
		h.t.Fatalf("link %s %s %s: %s %s", from, kind, to, response.Outcome, response.Refusal)
	}
	h.reopen()
	return response
}

// filedAt files a card and carries it to the column named, leaving it where
// add puts it when that is the intake column.
func (h *harness) filedAt(title, column string) string {
	h.t.Helper()
	ref := h.add(title)
	if column != hIntake && column != iIntake {
		h.at(ref, column)
	}
	return ref
}

// offerIn is the offer next makes a non-operator caller at one column.
func (h *harness) offerIn(column string) Offer {
	h.t.Helper()
	offers, err := h.library.Next(&Request{Verb: "next", Actor: "brin", Column: column})
	if err != nil || len(offers) != 1 {
		h.t.Fatalf("next at %s answered %+v %v", column, offers, err)
	}
	return offers[0]
}

// offered reports the reference of the card a column offers, empty where it
// offers none.
func (h *harness) offered(column string) string {
	h.t.Helper()
	if offer := h.offerIn(column); offer.Card != nil {
		return offer.Card.Ref
	}
	return ""
}

// pullFrom runs a pull as brin and answers the response; the pull is named
// where column is not empty.
func (h *harness) pullFrom(column string, noClaim bool) *Response {
	h.t.Helper()
	response := h.library.Pull(&Request{Verb: Pull, Actor: "brin", Column: column, NoClaim: noClaim})
	h.reopen()
	return response
}

// waitsOn is the references of the cards a card's view waits on, in order,
// joined by commas, and empty where it waits on none.
func (h *harness) waitsOn(ref string) string {
	h.t.Helper()
	var refs []string
	for _, wait := range h.showCard(ref).WaitsOn {
		refs = append(refs, wait.Ref)
	}
	return strings.Join(refs, ",")
}

// showCard is the card view show answers for one card, which is the path
// that lapses only the card shown.
func (h *harness) showCard(ref string) CardView {
	h.t.Helper()
	detail, _, _, _, err := h.library.Show(&Request{Verb: "show", Actor: "brin", Card: ref, Fields: "card"})
	if err != nil || detail == nil {
		h.t.Fatalf("show %s: %v", ref, err)
	}
	return detail.Card
}

// matchesWaiting reports whether schedule:waiting selects the card.
func (h *harness) matchesWaiting(ref string) bool {
	h.t.Helper()
	refs, err := h.queryRefs("schedule:waiting")
	if err != nil {
		h.t.Fatalf("query: %v", err)
	}
	for _, got := range refs {
		if got == ref {
			return true
		}
	}
	return false
}

// claimAs claims a card by name as actor and answers the response.
func (h *harness) claimAs(actor, ref string) *Response {
	h.t.Helper()
	return h.mustDo(&Request{Verb: Claim, Actor: actor, Card: ref})
}

// releaseAs releases a card as actor.
func (h *harness) releaseAs(actor, ref string) {
	h.t.Helper()
	h.mustDo(&Request{Verb: Release, Actor: actor, Card: ref})
}

// finishBlocks is a holds block declaring blocks, held named, finishing at
// Acceptance, with the commitment column at Implement.
const finishBlocks = "dinah.holds:\n  start_at: implement\n  kinds:\n    blocks:\n      held: named\n      finish_at: acceptance\n"

// TestNoDeclaredLayerLeavesSelectionAsItWas is dinah-608/criteria/1. The same
// cards are read with their blocks and parked_behind links and with the links
// removed, and every answer but the Links section is the same; no journal is
// read on the links' account.
func TestNoDeclaredLayerLeavesSelectionAsItWas(t *testing.T) {
	build := func(withLinks bool) string {
		h := initHarness(t, "")
		first := h.filedAt("first", iIntake)
		second := h.filedAt("second", iIntake)
		third := h.filedAt("third", iDoing)
		if withLinks {
			h.link(first, "blocks", second)
			h.link(second, "parked_behind", third)
			h.link(third, "blocks", first)
		}
		journals := 0
		h.library.Observe = func(event, target string) {
			if event == ObserveJournal {
				journals++
			}
		}
		var out []any
		offers, err := h.library.Next(&Request{Verb: "next", Actor: "brin"})
		out = append(out, offers, err)
		primer, err := h.library.Prime(&Request{Verb: "prime", Actor: "brin"})
		if primer != nil {
			out = append(out, primer.Ready)
		}
		out = append(out, err)
		for _, ref := range []string{first, second, third} {
			out = append(out, h.showCard(ref))
		}
		matches, err := h.library.Query(&Request{Verb: "query", Actor: "brin", Query: ""})
		out = append(out, matches, err)
		if journals != 0 {
			t.Errorf("with links %v, the reads opened %d journals", withLinks, journals)
		}
		out = append(out, h.pullFrom(iDoing, false).Card)
		encoded, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(encoded)
	}
	identity := regexp.MustCompile(`"(id|revision|claim_since|expires)":"[^"]*"`)
	withLinks := identity.ReplaceAllString(build(true), "")
	without := identity.ReplaceAllString(build(false), "")
	if withLinks != without {
		t.Errorf("a workbench declaring no dinah.holds answers differently with links:\n%s\nwithout:\n%s", withLinks, without)
	}
	if !strings.Contains(withLinks, `"ref":"wed-1"`) {
		t.Fatalf("the reading carried no card, so it compared nothing: %s", withLinks)
	}
}

// TestFinishToStartHoldsBothEnds is dinah-608/criteria/2.
func TestFinishToStartHoldsBothEnds(t *testing.T) {
	block := "dinah.holds:\n  kinds:\n    blocks:\n      held: named\n    parked_behind:\n      held: carrier\n"
	for _, c := range []struct {
		name  string
		kind  string
		named bool
	}{
		{name: "blocks holds the named card", kind: "blocks", named: true},
		{name: "parked_behind holds the carrier", kind: "parked_behind", named: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := initHarness(t, block)
			holder := h.filedAt("holder", iDoing)
			held := h.filedAt("held", iIntake)
			if c.named {
				h.link(holder, c.kind, held)
			} else {
				h.link(held, c.kind, holder)
			}
			if got := h.offered(iIntake); got != "" {
				t.Errorf("next offers %s while its holder stands in Doing", got)
			}
			offer := h.offerIn(iIntake)
			if !offer.Waiting || strings.Join(offer.WaitingOn, ",") != holder || offer.NoTaker {
				t.Errorf("the intake offer is %+v, want waiting on %s", offer, holder)
			}
			primer, err := h.library.Prime(&Request{Verb: "prime", Actor: "brin"})
			if err != nil {
				t.Fatalf("prime: %v", err)
			}
			if !primerHasWaiting(primer, iIntake) {
				t.Errorf("prime does not list Intake as waiting: %+v", primer.Ready)
			}
			for _, column := range []string{iDoing, ""} {
				response := h.pullFrom(column, false)
				if response.Card != nil || !strings.HasPrefix(response.Message, "answer.pull.waiting.") || response.MessageValues["cards"] != holder {
					t.Errorf("pull %q took %v with %s %v", column, response.Card, response.Message, response.MessageValues)
				}
			}
			// The accepting case: the holder moved into the done column
			// lifts the ground, and the held card is offered and pulled.
			h.at(holder, iDone)
			if got := h.offered(iIntake); got != held {
				t.Errorf("next offers %q once the holder finished, want %s", got, held)
			}
			if response := h.pullFrom(iDoing, false); response.Card == nil || response.Card.Ref != held {
				t.Errorf("pull doing took %+v once the holder finished", response.Card)
			}
		})
	}
}

// primerHasWaiting reports whether prime lists a column as waiting.
func primerHasWaiting(primer *Primer, column string) bool {
	for _, offer := range primer.Ready {
		if offer.Column == column && offer.Waiting {
			return true
		}
	}
	return false
}

// TestFinishAtCountsTheColumnAndEverythingAfter is dinah-608/criteria/3.
func TestFinishAtCountsTheColumnAndEverythingAfter(t *testing.T) {
	h := holdsHarness(t, finishBlocks)
	holder := h.filedAt("holder", hImplement)
	queued := h.filedAt("queued", hBuildQueue)
	committed := h.filedAt("committed", hImplement)
	h.link(holder, "blocks", queued)
	h.link(holder, "blocks", committed)
	if response := h.pullFrom(hImplement, false); response.Card != nil || !strings.HasPrefix(response.Message, "answer.pull.waiting.") {
		t.Fatalf("pull implement took %+v while the holder stands in Implement: %s", response.Card, response.Message)
	}
	if h.waitsOn(committed) != "" {
		t.Errorf("a card standing at the commitment column waits on %s", h.waitsOn(committed))
	}
	for _, column := range []string{hAcceptance, hDone} {
		h.at(holder, column)
		if got := h.waitsOn(queued); got != "" {
			t.Errorf("the holder in %s still holds the queued card: %s", column, got)
		}
	}
	h.at(holder, hImplement)
	if got := h.waitsOn(queued); got != holder {
		t.Errorf("the holder moved back before its finish_at holds the queued card on %q", got)
	}
	if got := h.waitsOn(committed); got != "" {
		t.Errorf("the holder moved back holds a card standing at the commitment column: %s", got)
	}
	active := h.filedAt("active", hTriage)
	h.link(holder, "blocks", active)
	h.claimAs("brin", active)
	if got := h.waitsOn(active); got != "" {
		t.Errorf("an active card before the commitment column waits on %s", got)
	}
}

// startRules is a holds block with two start-to-start kinds, one with a lag
// of two days, and the commitment column at Implement.
const startRules = "dinah.holds:\n  start_at: implement\n  kinds:\n    after_start_of:\n      held: carrier\n      waits_for: start\n    two_days_after_start_of:\n      held: carrier\n      waits_for: start\n      lag_days: 2\n"

// TestStartToStartReadsPositionAndClaim is dinah-608/criteria/4.
func TestStartToStartReadsPositionAndClaim(t *testing.T) {
	h := holdsHarness(t, startRules)
	holder := h.filedAt("holder", hTriage)
	held := h.filedAt("held", hBuildQueue)
	lagged := h.filedAt("lagged", hBuildQueue)
	h.link(held, "after_start_of", holder)
	h.link(lagged, "two_days_after_start_of", holder)
	if h.waitsOn(held) != holder {
		t.Fatalf("the held card waits on %q while its holder stands in Triage", h.waitsOn(held))
	}
	// A claim at a station before the commitment column starts the holder
	// while it is held, and the lag counts from the claim day.
	h.claimAs("brin", holder)
	if got := h.waitsOn(held); got != "" {
		t.Errorf("the held card waits on %s while its holder is claimed", got)
	}
	lagging := h.showCard(lagged).WaitsOn
	if len(lagging) != 1 || lagging[0].Reached != "2026-10-03" || lagging[0].Until != "2026-10-05" {
		t.Errorf("the lagged ground reads %+v, want reached 2026-10-03 and until 2026-10-05", lagging)
	}
	h.releaseAs("brin", holder)
	if got := h.waitsOn(held); got != holder {
		t.Errorf("released at Triage, the holder holds on %q, want awaiting again", got)
	}
	// At the commitment column it has started, and a claim released there
	// leaves it started.
	h.at(holder, hImplement)
	h.claimAs("brin", holder)
	h.releaseAs("brin", holder)
	if got := h.waitsOn(held); got != "" {
		t.Errorf("a holder released at the commitment column still holds: %s", got)
	}
	// A holder carried by move into a column before the commitment column
	// has not started.
	h.at(holder, hDesignQueue)
	if got := h.waitsOn(held); got != holder {
		t.Errorf("a holder moved back into Design Queue does not hold: %q", got)
	}
	// Carried into the done column by move and never claimed, it has
	// started on the day of that crossing, and the lag counts from it.
	h.clock = holdsToday.AddDate(0, 0, 1)
	h.at(holder, hDone)
	lagging = h.showCard(lagged).WaitsOn
	if len(lagging) != 1 || lagging[0].Reached != "2026-10-04" || lagging[0].Until != "2026-10-06" {
		t.Errorf("the lag after a move into done reads %+v, want from 2026-10-04", lagging)
	}
	// Moved back out of the done column to a column before the commitment
	// column, and not active, it has not started.
	h.at(holder, hTriage)
	if got := h.waitsOn(held); got != holder {
		t.Errorf("a holder moved back to Triage does not hold: %q", got)
	}
	// A pull that carries no claim into the commitment column starts it on
	// the day of that crossing.
	h.at(holder, hBuildQueue)
	h.clock = holdsToday.AddDate(0, 0, 3)
	// The held cards stand in Build Queue too and wait, so the pull takes
	// the holder, which is the one card there that nothing withholds.
	if response := h.pullFrom(hImplement, true); response.Card == nil || response.Card.Ref != holder {
		t.Fatalf("pull --no-claim took %+v, want %s", response.Card, holder)
	}
	lagging = h.showCard(lagged).WaitsOn
	if len(lagging) != 1 || lagging[0].Reached != "2026-10-06" {
		t.Errorf("the lag after pull --no-claim reads %+v, want from 2026-10-06", lagging)
	}
}

// TestALagIsCountedInTheWorkbenchZone is dinah-608/criteria/5.
func TestALagIsCountedInTheWorkbenchZone(t *testing.T) {
	block := "dinah.holds:\n  kinds:\n    cures_before:\n      held: named\n      lag_days: 7\n"
	definition := strings.Replace(initDefinition,
		`{ "id": "d00000000003", "title": "Done", "kind": "done", "instructions": "i\n" }`,
		`{ "id": "d00000000003", "title": "Done", "kind": "done", "instructions": "i\n" },
    { "id": "d00000000004", "title": "Archived", "kind": "done", "instructions": "i\n" }`, 1)
	h := harnessFromDefinition(t, "site", definition)
	h.writeAnchorBlock(bench.ScheduleKey, bench.ScheduleKey+":\n  time_zone: Asia/Singapore\n")
	h.declareHolds(block)
	// 17:00 UTC on the 2nd is the 3rd in Singapore, so the crossing is dated
	// the 3rd, where a reading in UTC would date it the 2nd.
	h.clock = time.Date(2026, 10, 2, 17, 0, 0, 0, time.UTC)
	slab := h.filedAt("slab", iDoing)
	walls := h.filedAt("walls", iIntake)
	h.link(slab, "cures_before", walls)
	h.at(slab, "d00000000004")
	for day, want := range map[int]string{0: "2026-10-10", 6: "2026-10-10"} {
		h.clock = time.Date(2026, 10, 2, 17, 0, 0, 0, time.UTC).AddDate(0, 0, day)
		offer := h.offerIn(iIntake)
		if offer.Card != nil || !offer.NotYet || offer.StartableFrom != want || offer.Waiting {
			t.Errorf("on day %d the intake offer is %+v, want not yet until %s", day, offer, want)
		}
	}
	h.clock = time.Date(2026, 10, 9, 17, 0, 0, 0, time.UTC)
	if got := h.offered(iIntake); got != walls {
		t.Errorf("on the seventh day after the crossing next offers %q", got)
	}
	// A move between two done columns is no crossing and moves nothing.
	h.clock = time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)
	h.at(slab, iDone)
	if waits := h.showCard(walls).WaitsOn; len(waits) != 1 || waits[0].Until != "2026-10-10" {
		t.Errorf("a move between two done columns moved the lag: %+v", waits)
	}
	// Pushed out and returned on a later day, the lag counts from the return.
	h.at(slab, iDoing)
	h.clock = time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)
	h.at(slab, iDone)
	if waits := h.showCard(walls).WaitsOn; len(waits) != 1 || waits[0].Reached != "2026-10-05" || waits[0].Until != "2026-10-12" {
		t.Errorf("the lag after a return reads %+v, want from 2026-10-05", waits)
	}
	// A holder standing in a done column with no crossing in its journal
	// makes no lagging ground.
	unrecorded := h.filedAt("unrecorded", iDoing)
	blocked := h.filedAt("blocked", iIntake)
	h.link(unrecorded, "cures_before", blocked)
	h.handMove(unrecorded, iDone)
	if waits := h.showCard(blocked).WaitsOn; len(waits) != 0 {
		t.Errorf("a holder in done with no crossing recorded holds: %+v", waits)
	}
}

// handMove rewrites a card's column in its anchor, as a person editing the
// file would, so no journal event records the move.
func (h *harness) handMove(ref, column string) {
	h.t.Helper()
	path := filepath.Join(h.root, bench.CardsDir, h.cardID(ref), bench.CardAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		h.t.Fatalf("read %s: %v", path, err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set("column", column)
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		h.t.Fatalf("write %s: %v", path, err)
	}
	h.reopen()
}

// TestAHolderCarriesItsOwnStartAfter is dinah-608/criteria/6.
func TestAHolderCarriesItsOwnStartAfter(t *testing.T) {
	h := initHarness(t, "dinah.holds:\n  kinds:\n    needs:\n      held: carrier\n")
	venue := h.filedAt("Book the venue", iIntake)
	h.mustSet(venue, bench.StartAfterField, "2026-11-01")
	invitations := h.filedAt("Send the invitations", iIntake)
	h.link(invitations, "needs", venue)
	waits := h.showCard(invitations).WaitsOn
	if len(waits) != 1 || waits[0].NotBefore != "2026-11-01" || waits[0].WaitsFor != contract.HoldWaitsFinish {
		t.Fatalf("the invitations wait %+v, want on the venue to finish, not before 2026-11-01", waits)
	}
	for _, day := range []time.Time{time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC), time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)} {
		h.clock = day
		if got := h.offered(iIntake); got != venue {
			t.Errorf("on %s next offers %q, want the venue and not the invitations", day.Format("2006-01-02"), got)
		}
		if waits := h.showCard(invitations).WaitsOn; len(waits) != 1 || waits[0].NotBefore != "" {
			t.Errorf("on %s the invitations wait %+v, want on the venue with no date left to report", day.Format("2006-01-02"), waits)
		}
	}
	h.at(venue, iDone)
	if got := h.offered(iIntake); got != invitations {
		t.Errorf("with the venue booked next offers %q", got)
	}
}

// TestNextReportsWaitingBesideADate is the verb half of
// dinah-608/criteria/7: a column whose only ready cards wait reports Waiting
// with the holders, and beside a card held only by a date it carries NotYet
// as well.
func TestNextReportsWaitingBesideADate(t *testing.T) {
	h := initHarness(t, "dinah.holds:\n  kinds:\n    needs:\n      held: carrier\n")
	holder := h.filedAt("holder", iDoing)
	waiting := h.filedAt("waiting", iIntake)
	h.link(waiting, "needs", holder)
	offer := h.offerIn(iIntake)
	if !offer.Waiting || strings.Join(offer.WaitingOn, ",") != holder || offer.NotYet || offer.ReadyCount != 1 {
		t.Errorf("the waiting offer is %+v", offer)
	}
	dated := h.filedAt("dated", iIntake)
	h.mustSet(dated, bench.StartAfterField, "2026-10-09")
	offer = h.offerIn(iIntake)
	if !offer.Waiting || !offer.NotYet || offer.StartableFrom != "2026-10-09" || offer.ReadyCount != 2 {
		t.Errorf("the offer beside a dated card is %+v", offer)
	}
	encoded, _ := json.Marshal(offer)
	if !strings.Contains(string(encoded), `"not_yet":true,"startable_from":"2026-10-09","waiting":true,"waiting_on":["`+holder+`"]`) {
		t.Errorf("the JSON offer reads %s", encoded)
	}
}

// TestPullChoosesItsEmptyAnswerInOrder is dinah-608/criteria/8: above the
// tier, then waiting, then not yet, then empty, on both forms, at exit 0 with
// nothing journaled, and a card withheld further back in the named form
// reaches the answer.
func TestPullChoosesItsEmptyAnswerInOrder(t *testing.T) {
	h := holdsHarness(t, finishBlocks)
	h.writeAnchorBlock("levels", "levels:\n  tier: [workhorse, frontier]\n")
	h.writeAnchorBlock("tiers", "tiers:\n  workhorse:\n    meaning: scoped\n    models:\n      - {provider: acme, model: workhorse}\n"+
		"  frontier:\n    meaning: novel\n    models:\n      - {provider: acme, model: frontier}\n")
	pull := func(column string) *Response {
		response := h.library.Pull(&Request{Verb: Pull, Actor: "brin", Provider: "acme", Model: "workhorse", Column: column})
		h.reopen()
		return response
	}
	answers := func(named, bare string) {
		t.Helper()
		before := h.journals()
		for _, column := range []string{hImplement, ""} {
			want := named
			if column == "" {
				want = bare
			}
			response := pull(column)
			if response.Outcome != contract.OutcomeOK || response.Card != nil || response.Message != want {
				t.Errorf("pull %q answered %s %s %v, want %s", column, response.Outcome, response.Message, response.MessageValues, want)
			}
		}
		after := h.journals()
		for path, text := range before {
			if after[path] != text {
				t.Errorf("an empty pull wrote %s", path)
			}
		}
	}
	// The holder is held by somebody, so no pull can take it on.
	holder := h.filedAt("holder", hImplement)
	h.claimAs("alka", holder)
	// Waiting further back than the destination's own upstream still
	// reaches the named answer.
	far := h.filedAt("far back", hDesignQueue)
	h.link(holder, "blocks", far)
	answers("answer.pull.waiting.named", "answer.pull.waiting.bare")
	if response := pull(hImplement); response.MessageValues["cards"] != holder {
		t.Errorf("the waiting answer names %v, want %s", response.MessageValues, holder)
	}
	dated := h.filedAt("dated", hBuildQueue)
	h.mustSet(dated, bench.StartAfterField, "2026-10-09")
	answers("answer.pull.waiting.named", "answer.pull.waiting.bare")
	senior := h.filedAt("senior", hBuildQueue)
	h.mustSet(senior, "tier", "frontier")
	answers("answer.pull.above-tier.named", "answer.pull.above-tier.bare")
	h.at(senior, hDone)
	h.at(far, hDone)
	answers("answer.pull.not-yet.named", "answer.pull.not-yet.bare")
	h.at(dated, hDone)
	answers("answer.pull.empty.named", "answer.pull.empty.bare")
	// The waiting answer carries at most three references and a count.
	waiter := h.filedAt("waiter", hDesignQueue)
	for _, title := range []string{"a", "b", "c", "d"} {
		other := h.filedAt(title, hImplement)
		h.claimAs("alka", other)
		h.link(other, "blocks", waiter)
	}
	response := pull(hImplement)
	if response.Message != "answer.pull.waiting.named.more" || response.MessageValues["n"] != "1" || strings.Count(response.MessageValues["cards"], ", ") != 2 {
		t.Errorf("four holders answered %s %v", response.Message, response.MessageValues)
	}
}

// TestAClaimByNameStartsTheCardWhileItHolds is dinah-608/criteria/9, with the
// expired claim the third design review named.
func TestAClaimByNameStartsTheCardWhileItHolds(t *testing.T) {
	for _, actor := range []string{"brin", "alka"} {
		t.Run(actor, func(t *testing.T) {
			h := holdsHarness(t, finishBlocks)
			holder := h.filedAt("holder", hImplement)
			held := h.filedAt("held", hTriage)
			h.link(holder, "blocks", held)
			response := h.claimAs(actor, held)
			if response.Warning != "warn.waiting-on" || response.WarningDetail != holder {
				t.Errorf("the claim warns %q %q, want warn.waiting-on naming %s", response.Warning, response.WarningDetail, holder)
			}
			if h.waitsOn(held) != "" || h.matchesWaiting(held) {
				t.Errorf("the claimed card still waits")
			}
			h.releaseAs(actor, held)
			if h.waitsOn(held) != holder || !h.matchesWaiting(held) {
				t.Errorf("released at Triage the card does not wait again")
			}
			h.at(held, hImplement)
			h.claimAs(actor, held)
			h.releaseAs(actor, held)
			if h.waitsOn(held) != "" || h.matchesWaiting(held) {
				t.Errorf("released at the commitment column the card waits")
			}
		})
	}
	h := holdsHarness(t, "dinah.holds:\n  start_at: implement\n  kinds:\n    blocks:\n      held: named\n      lag_days: 3\n")
	dated := h.filedAt("dated", hTriage)
	h.mustSet(dated, bench.StartAfterField, "2026-10-08")
	if response := h.claimAs("brin", dated); response.Warning != "warn.before-start-after" || response.WarningDetail != "2026-10-08" {
		t.Errorf("a card held by its own date warns %q %q", response.Warning, response.WarningDetail)
	}
	holder := h.filedAt("holder", hImplement)
	lagged := h.filedAt("lagged", hTriage)
	h.link(holder, "blocks", lagged)
	h.at(holder, hDone)
	if response := h.claimAs("brin", lagged); response.Warning != "warn.before-start-after" || response.WarningDetail != "2026-10-06" {
		t.Errorf("a card held by a lag warns %q %q", response.Warning, response.WarningDetail)
	}
	// move, release and block carry no hold warning, and nothing refuses.
	waiting := h.filedAt("waiting", hTriage)
	other := h.filedAt("other", hImplement)
	h.link(other, "blocks", waiting)
	for _, req := range []*Request{
		{Verb: Move, Actor: "brin", Card: waiting, Column: hBuildQueue},
		{Verb: Move, Actor: "brin", Card: waiting, Column: hTriage},
		{Verb: Claim, Actor: "brin", Card: waiting},
		{Verb: Release, Actor: "brin", Card: waiting},
		{Verb: Block, Actor: "brin", Card: waiting, Reason: "an obstacle"},
	} {
		response := h.mustDo(req)
		if req.Verb != Claim && strings.HasPrefix(response.Warning, "warn.waiting") {
			t.Errorf("%s carries %s", req.Verb, response.Warning)
		}
	}
	h.mustDo(&Request{Verb: Unblock, Actor: "alka", Card: waiting})
	h.at(waiting, hBuildQueue)
	if refused := h.do(&Request{Verb: Claim, Actor: "brin", Card: waiting}); refused.Outcome != contract.OutcomeRefused {
		t.Errorf("a claim at a queue answered %s", refused.Outcome)
	}
	// A claim that has expired is no claim, whether or not a read has
	// lapsed it on disk: the held card's show re-holds with no read of the
	// holder in between.
	h2 := holdsHarness(t, startRules)
	claimed := h2.filedAt("claimed", hTriage)
	waiter := h2.filedAt("waiter", hBuildQueue)
	h2.link(waiter, "after_start_of", claimed)
	h2.mustDo(&Request{Verb: Claim, Actor: "brin", Card: claimed, Expires: time.Hour})
	if h2.waitsOn(waiter) != "" {
		t.Fatalf("the waiter waits while its holder's claim holds")
	}
	h2.advance(2 * time.Hour)
	if got := h2.waitsOn(waiter); got != claimed {
		t.Errorf("after the claim expired the waiter waits on %q", got)
	}
	if state := h2.card(claimed).State; state != contract.StateActive {
		t.Fatalf("the holder's claim was lapsed on disk (%s), so the case read nothing unlapsed", state)
	}
}

// TestACycleHoldsAndIsReported is dinah-608/criteria/10.
func TestACycleHoldsAndIsReported(t *testing.T) {
	h := holdsHarness(t, finishBlocks)
	first := h.filedAt("first", hBuildQueue)
	second := h.filedAt("second", hBuildQueue)
	outside := h.filedAt("outside", hBuildQueue)
	if response := h.link(first, "blocks", second); response.Warning != "" {
		t.Errorf("a link closing no cycle warns %s", response.Warning)
	}
	response := h.link(second, "blocks", first)
	if response.Warning != "warn.hold-cycle" || response.WarningDetail != first+", "+second {
		t.Errorf("the closing link warns %q %q", response.Warning, response.WarningDetail)
	}
	if got := h.offered(hBuildQueue); got != outside {
		t.Errorf("next offers %q, want the card outside the cycle", got)
	}
	findings := h.cycleFindings()
	if len(findings) != 1 || findings[0].Detail != first+", "+second || !strings.Contains(findings[0].Path, h.cardID(first)) {
		t.Errorf("check reports %+v", findings)
	}
	self := h.filedAt("self", hBuildQueue)
	if response := h.link(self, "blocks", self); response.Warning != "warn.hold-cycle" || response.WarningDetail != self {
		t.Errorf("a self link warns %q %q", response.Warning, response.WarningDetail)
	}
	if findings := h.cycleFindings(); len(findings) != 2 {
		t.Errorf("check reports %d cycles, want the pair and the self link: %+v", len(findings), findings)
	}
	// Claiming one card of the live cycle starts it, and releasing it before
	// the commitment column makes the cycle live again.
	h.at(first, hTriage)
	h.claimAs("brin", first)
	if findings := h.cycleFindings(); len(findings) != 1 {
		t.Errorf("with one card claimed check still reports %+v", findings)
	}
	h.releaseAs("brin", first)
	if findings := h.cycleFindings(); len(findings) != 2 {
		t.Errorf("released before the commitment column the cycle is not live again: %+v", findings)
	}
	if _, err := h.library.Next(&Request{Verb: "next", Actor: "brin"}); err != nil {
		t.Errorf("next over a live cycle: %v", err)
	}
	// Moving it to the commitment column starts it for good, and the other
	// is offered once the started card finishes.
	h.at(first, hImplement)
	if findings := h.cycleFindings(); len(findings) != 1 {
		t.Errorf("with one card at the commitment column check still reports %+v", findings)
	}
	if h.waitsOn(second) != first {
		t.Errorf("the second card does not wait on the first while it works")
	}
	h.at(first, hAcceptance)
	if h.waitsOn(second) != "" || h.offered(hBuildQueue) != second {
		t.Errorf("the second card is not offered once the first reached its finish_at")
	}
	// The same pair linked with one card at the commitment column, one card
	// active, one edge's holder at its finish_at, or one edge only lagging
	// warns nothing and is not reported.
	lagRules := "dinah.holds:\n  start_at: implement\n  kinds:\n    blocks:\n      held: named\n      finish_at: acceptance\n    cures:\n      held: named\n      lag_days: 9\n"
	for _, c := range []struct {
		name  string
		place func(g *harness, x string)
		kind  string
	}{
		{name: "at the commitment column", place: func(g *harness, x string) { g.at(x, hImplement) }, kind: "blocks"},
		{name: "active", place: func(g *harness, x string) { g.claimAs("brin", x) }, kind: "blocks"},
		{name: "at its finish_at", place: func(g *harness, x string) { g.at(x, hAcceptance) }, kind: "blocks"},
		{name: "only lagging", place: func(g *harness, x string) { g.at(x, hDone) }, kind: "cures"},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := holdsHarness(t, lagRules)
			x := g.filedAt("x", hTriage)
			y := g.filedAt("y", hBuildQueue)
			c.place(g, x)
			g.link(x, c.kind, y)
			if response := g.link(y, "blocks", x); response.Warning != "" {
				t.Errorf("the pair warns %s", response.Warning)
			}
			if findings := g.cycleFindings(); len(findings) != 0 {
				t.Errorf("check reports %+v", findings)
			}
		})
	}
}

// cycleFindings are the check.hold-cycle findings dinah check reports.
func (h *harness) cycleFindings() []bench.Finding {
	h.t.Helper()
	report, err := h.library.Check(&Request{Verb: "check", Actor: "alka"})
	if err != nil {
		h.t.Fatalf("check: %v", err)
	}
	var found []bench.Finding
	for _, finding := range report.Findings {
		if finding.Key == bench.FindingHoldCycle {
			found = append(found, finding)
		}
	}
	return found
}

// TestAnArchivedOrMissingHolderHoldsNothing is dinah-608/criteria/11.
func TestAnArchivedOrMissingHolderHoldsNothing(t *testing.T) {
	h := initHarness(t, "dinah.holds:\n  kinds:\n    needs:\n      held: carrier\n")
	holder := h.filedAt("holder", iIntake)
	held := h.filedAt("held", iIntake)
	h.link(held, "needs", holder)
	if h.waitsOn(held) != holder {
		t.Fatalf("the held card does not wait on its live holder")
	}
	if response := h.library.Archive(&Request{Verb: "archive", Actor: "alka", Ref: holder}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("archive: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if h.waitsOn(held) != "" || h.offered(iIntake) != held {
		t.Errorf("an archived holder still holds")
	}
	if response := h.library.Restore(&Request{Verb: "restore", Actor: "alka", Ref: holder}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("restore: %s %s", response.Outcome, response.Refusal)
	}
	h.reopen()
	if h.waitsOn(held) != holder {
		t.Errorf("a restored holder does not hold again")
	}
	// A link naming a card the workbench does not carry holds nothing.
	orphan := h.filedAt("orphan", iIntake)
	path := filepath.Join(h.root, bench.CardsDir, h.cardID(orphan), bench.CardAnchor)
	text, _ := bench.ReadText(path)
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw("links", []string{"links:", "  - kind: needs", "    to: 0123456789ab"})
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write: %v", err)
	}
	h.reopen()
	if got := h.waitsOn(orphan); got != "" {
		t.Errorf("a link to no card holds on %q", got)
	}
}

// TestWaitingIsAConditionAndAQueryValue is the verb half of
// dinah-608/criteria/13.
func TestWaitingIsAConditionAndAQueryValue(t *testing.T) {
	h := initHarness(t, "dinah.holds:\n  kinds:\n    needs:\n      held: carrier\n")
	holder := h.filedAt("holder", iDoing)
	waiting := h.filedAt("waiting", iIntake)
	plain := h.filedAt("plain", iIntake)
	finished := h.filedAt("finished", iDone)
	h.link(waiting, "needs", holder)
	h.link(finished, "needs", holder)
	if got, err := h.queryRefs("schedule:waiting"); err != nil || strings.Join(got, ",") != waiting {
		t.Errorf("schedule:waiting selected %v %v", got, err)
	}
	if got, err := h.queryRefs("schedule!=waiting"); err != nil || refSet(got) != refSet([]string{holder, plain, finished}) {
		t.Errorf("schedule!=waiting selected %v %v", got, err)
	}
	if view := h.showCard(finished); len(view.WaitsOn) != 0 || len(view.Schedule) != 0 {
		t.Errorf("a card in a done column carries %+v %v", view.WaitsOn, view.Schedule)
	}
	view := h.showCard(waiting)
	if strings.Join(view.Schedule, ",") != contract.ScheduleWaiting || len(view.WaitsOn) != 1 || view.WaitsOn[0].Kind != "needs" || view.WaitsOn[0].Ref != holder {
		t.Errorf("the waiting card's view is %v %+v", view.Schedule, view.WaitsOn)
	}
}

// TestBlocksOthersCountsTheCardsWaiting is dinah-608/criteria/14.
func TestBlocksOthersCountsTheCardsWaiting(t *testing.T) {
	h := agendaHarness(t)
	h.clock = holdsToday
	h.rankedView()
	holder := h.placeAt("holder", agendaStation)
	first := h.placeAt("first", agendaIntake)
	second := h.placeAt("second", agendaIntake)
	finished := h.placeAt("finished", agendaIntake)
	lagging := h.placeAt("lagging", agendaIntake)
	for _, held := range []string{first, second, finished} {
		h.link(held, "needs", holder)
	}
	term := func() UrgencyTerm {
		return termOf(t, scoreOf(t, h.explained("everything", asOperator), 0, holder), bench.UrgencyBlocksOthers)
	}
	if got := term(); got.Basis["read"] != "false" || got.Points.String() != "0.0" {
		t.Errorf("with no dinah.holds the term reads %+v", got)
	}
	h.declareHolds("dinah.holds:\n  kinds:\n    needs:\n      held: carrier\n    cures:\n      held: carrier\n      lag_days: 5\n")
	h.at(finished, agendaFinished)
	// A card waiting on a holder only through a lagging ground does not
	// count, because the holder has done its part.
	done := h.placeAt("done holder", agendaStation)
	h.link(lagging, "cures", done)
	h.at(done, agendaFinished)
	if waits := h.showCard(lagging).WaitsOn; len(waits) != 1 || waits[0].Until == "" {
		t.Fatalf("the lagging card is not lagging: %+v", waits)
	}
	index, err := h.library.holdsOn(&Request{})
	if err != nil || index.awaitingOn(h.card(done)) != 0 || index.awaitingOn(h.card(holder)) != 2 {
		t.Errorf("the index counts %d cards waiting on the finished holder and %d on the other: %v", index.awaitingOn(h.card(done)), index.awaitingOn(h.card(holder)), err)
	}
	weights, _ := h.library.Bench.Urgency()
	weight := bench.FormatTenths(weights.BlocksOthers * 2)
	if got := term(); got.Basis["read"] != "true" || got.Basis["counted"] != "2" || got.Points.String() != weight {
		t.Errorf("two waiting cards score %+v, want counted 2 and %s", got, weight)
	}
}

// TestStartedIsAPositionInTheFlow is dinah-608/criteria/17.
func TestStartedIsAPositionInTheFlow(t *testing.T) {
	h := holdsHarness(t, "dinah.holds:\n  start_at: implement\n  kinds:\n    blocks:\n      held: named\n")
	holder := h.filedAt("holder", hTriage)
	held := h.filedAt("held", hTriage)
	h.link(holder, "blocks", held)
	h.claimAs("brin", held)
	if h.waitsOn(held) != "" || h.matchesWaiting(held) {
		t.Errorf("a card claimed at Triage waits")
	}
	h.releaseAs("brin", held)
	if h.waitsOn(held) != holder || !h.matchesWaiting(held) || h.offered(hTriage) == held {
		t.Errorf("released at Triage the card is not withheld")
	}
	h.at(held, hBuildQueue)
	if response := h.pullFrom(hImplement, false); response.Card != nil {
		t.Errorf("pull implement took %s from Build Queue", response.Card.Ref)
	}
	if h.offered(hBuildQueue) != "" {
		t.Errorf("next offers the held card at Build Queue")
	}
	h.at(held, hImplement)
	if h.offered(hImplement) != held || h.waitsOn(held) != "" || h.matchesWaiting(held) {
		t.Errorf("carried into Implement the card is still withheld")
	}
	h.at(held, hBuildQueue)
	if h.offered(hBuildQueue) != "" || h.waitsOn(held) != holder {
		t.Errorf("moved back to Build Queue the card is not withheld again")
	}
	// A holder moved back out of its finishing point withholds again only
	// the held cards standing before the commitment column that are not
	// active.
	h.at(holder, hDone)
	committed := h.filedAt("committed", hImplement)
	active := h.filedAt("active", hTriage)
	h.link(holder, "blocks", committed)
	h.link(holder, "blocks", active)
	h.claimAs("brin", active)
	h.at(holder, hTriage)
	if h.waitsOn(held) != holder || h.waitsOn(committed) != "" || h.waitsOn(active) != "" {
		t.Errorf("reopened, the holder holds %q, %q and %q", h.waitsOn(held), h.waitsOn(committed), h.waitsOn(active))
	}
}

// TestNotBeforeIsCarriedThroughSeveralCards is dinah-608/criteria/18.
func TestNotBeforeIsCarriedThroughSeveralCards(t *testing.T) {
	block := "dinah.holds:\n  kinds:\n    after:\n      held: carrier\n    after1:\n      held: carrier\n      lag_days: 1\n    after3:\n      held: carrier\n      lag_days: 3\n    after7:\n      held: carrier\n      lag_days: 7\n    after9:\n      held: carrier\n      lag_days: 9\n"
	notBefore := func(h *harness, ref string) string {
		waits := h.showCard(ref).WaitsOn
		if len(waits) == 0 {
			return "none"
		}
		var dates []string
		for _, wait := range waits {
			dates = append(dates, wait.NotBefore)
		}
		return strings.Join(dates, ",")
	}
	t.Run("a chain", func(t *testing.T) {
		h := initHarness(t, block)
		a, b, c := h.filedAt("a", iIntake), h.filedAt("b", iIntake), h.filedAt("c", iIntake)
		h.link(a, "after", b)
		h.link(b, "after", c)
		h.mustSet(c, bench.StartAfterField, "2026-10-20")
		if got := notBefore(h, a); got != "2026-10-20" {
			t.Errorf("a reads not before %q, want 2026-10-20", got)
		}
	})
	t.Run("a chain of lags", func(t *testing.T) {
		h := initHarness(t, block)
		a, b, c := h.filedAt("a", iIntake), h.filedAt("b", iIntake), h.filedAt("c", iDoing)
		h.link(a, "after3", b)
		h.link(b, "after7", c)
		h.at(c, iDone)
		if got := notBefore(h, a); got != "2026-10-13" {
			t.Errorf("a reads not before %q, want E+10 2026-10-13", got)
		}
	})
	for _, order := range []string{"b first", "c first"} {
		t.Run("the diamond, "+order, func(t *testing.T) {
			h := initHarness(t, block)
			a, b, c, d := h.filedAt("a", iIntake), h.filedAt("b", iIntake), h.filedAt("c", iIntake), h.filedAt("d", iIntake)
			if order == "b first" {
				h.link(a, "after", b)
				h.link(a, "after", c)
			} else {
				h.link(a, "after", c)
				h.link(a, "after", b)
			}
			h.link(b, "after1", d)
			h.link(c, "after9", d)
			h.mustSet(d, bench.StartAfterField, "2026-10-20")
			got := strings.Split(notBefore(h, a), ",")
			want := map[string]bool{"2026-10-21": true, "2026-10-29": true}
			latest := ""
			for _, date := range got {
				if !want[date] {
					t.Errorf("a reads not before %v, want the grounds through b and c at X+1 and X+9", got)
				}
				if date > latest {
					latest = date
				}
			}
			if latest != "2026-10-29" {
				t.Errorf("the latest ground reads %q, want X+9", latest)
			}
		})
	}
	// One level down, the diamond is read inside a single ground: z waits on
	// a alone, so the two routes from a to d meet on one path walk, and a
	// walk that remembered every card it visited would read d once, through
	// b, and report X+1.
	for _, order := range []string{"b first", "c first"} {
		t.Run("the diamond one level down, "+order, func(t *testing.T) {
			h := initHarness(t, block)
			z, a, b, c, d := h.filedAt("z", iIntake), h.filedAt("a", iIntake), h.filedAt("b", iIntake), h.filedAt("c", iIntake), h.filedAt("d", iIntake)
			h.link(z, "after", a)
			if order == "b first" {
				h.link(a, "after", b)
				h.link(a, "after", c)
			} else {
				h.link(a, "after", c)
				h.link(a, "after", b)
			}
			h.link(b, "after1", d)
			h.link(c, "after9", d)
			h.mustSet(d, bench.StartAfterField, "2026-10-20")
			// The index is read once on a fresh request, because a second
			// reading on the same request answers from what the first
			// memoised, and that would hide a walk the first got wrong.
			index, err := h.library.holdsOn(&Request{})
			if err != nil {
				t.Fatalf("holdsOn: %v", err)
			}
			grounds := index.holdsOf(h.library.Bench, h.card(z), bench.DateOf(2026, time.October, 3))
			if len(grounds) != 1 || grounds[0].NotBefore != "2026-10-29" {
				t.Errorf("z reads %+v, want one ground not before X+9 2026-10-29", grounds)
			}
			if got := notBefore(h, z); got != "2026-10-29" {
				t.Errorf("z's view reads not before %q, want X+9 2026-10-29", got)
			}
		})
	}
	t.Run("a cycle", func(t *testing.T) {
		h := initHarness(t, block)
		a, b := h.filedAt("a", iIntake), h.filedAt("b", iIntake)
		h.link(a, "after", b)
		h.link(b, "after", a)
		h.mustSet(a, bench.StartAfterField, "2026-10-20")
		if got := notBefore(h, b); got != "2026-10-20" {
			t.Errorf("b reads not before %q, want a's start_after", got)
		}
		if got := notBefore(h, a); got != "" {
			t.Errorf("a reads not before %q from its own start_after", got)
		}
	})
	t.Run("a started holder", func(t *testing.T) {
		h := initHarness(t, block)
		a, b, c := h.filedAt("a", iIntake), h.filedAt("b", iDoing), h.filedAt("c", iIntake)
		h.link(a, "after", b)
		h.link(b, "after", c)
		h.mustSet(b, bench.StartAfterField, "2026-10-20")
		h.mustSet(c, bench.StartAfterField, "2026-10-25")
		if got := notBefore(h, a); got != "" {
			t.Errorf("a holder standing in Doing contributes not before %q", got)
		}
	})
}

// TestLateStartFollowsTheCommitmentColumn is dinah-608/criteria/19.
func TestLateStartFollowsTheCommitmentColumn(t *testing.T) {
	h := holdsHarness(t, "dinah.holds:\n  start_at: implement\n")
	h.writeAnchorBlock(bench.ScheduleKey, bench.ScheduleKey+":\n  soon_days: 7\n")
	conditions := func(ref string) string {
		return strings.Join(h.showCard(ref).Schedule, ",")
	}
	late := h.filedAt("late", hTriage)
	h.mustSet(late, bench.StartByField, "2026-10-01")
	soon := h.filedAt("soon", hTriage)
	h.mustSet(soon, bench.StartByField, "2026-10-05")
	for _, ref := range []string{late, soon} {
		h.claimAs("brin", ref)
		h.releaseAs("brin", ref)
	}
	if conditions(late) != contract.ScheduleLateStart || conditions(soon) != contract.ScheduleStartSoon {
		t.Errorf("claimed and released at Triage, the cards read %q and %q", conditions(late), conditions(soon))
	}
	h.at(late, hImplement)
	if got := conditions(late); got != "" {
		t.Errorf("carried into Implement unclaimed the card reads %q", got)
	}
	h.claimAs("brin", late)
	h.releaseAs("brin", late)
	h.at(late, hTriage)
	if got := conditions(late); got != contract.ScheduleLateStart {
		t.Errorf("moved back to Triage the card reads %q", got)
	}
	journals := 0
	h.library.Observe = func(event, target string) {
		if event == ObserveJournal {
			journals++
		}
	}
	day := h.library.dayOf(&Request{})
	if _, err := h.library.scheduleOf(h.card(late), day); err != nil || journals != 0 {
		t.Errorf("scheduleOf opened %d journals: %v", journals, err)
	}
	// On the flow dinah init creates with no dinah.holds, a card claimed in
	// Doing answers as at trunk.
	g := initHarness(t, "")
	claimed := g.filedAt("claimed", iDoing)
	g.mustSet(claimed, bench.StartByField, "2026-10-01")
	g.claimAs("brin", claimed)
	if got := strings.Join(g.showCard(claimed).Schedule, ","); got != "" {
		t.Errorf("a card claimed in Doing reads %q", got)
	}
}

// TestTheDefaultCommitmentColumn is dinah-608/criteria/20.
func TestTheDefaultCommitmentColumn(t *testing.T) {
	rules := "  kinds:\n    needs:\n      held: carrier\n"
	h := initHarness(t, "dinah.holds:\n"+rules)
	holder := h.filedAt("holder", iIntake)
	held := h.filedAt("held", iIntake)
	h.link(held, "needs", holder)
	if response := h.pullFrom(iDoing, false); response.Card != nil && response.Card.Ref == held {
		t.Errorf("pull doing took the held card")
	}
	h.reopen()
	h.at(held, iDoing)
	if h.offered(iDoing) != held || h.matchesWaiting(held) {
		t.Errorf("carried into Doing the held card is not offered")
	}

	prepared := strings.Replace(initDefinition,
		`{ "id": "d00000000002", "title": "Doing"`,
		`{ "id": "d00000000009", "title": "Prepared", "kind": "work", "instructions": "i\n" },
    { "id": "d00000000002", "title": "Doing"`, 1)
	p := harnessFromDefinition(t, "site", prepared)
	p.clock = holdsToday
	p.declareHolds("dinah.holds:\n  start_at: doing\n" + rules)
	pHolder := p.filedAt("holder", iIntake)
	pHeld := p.filedAt("held", "d00000000009")
	p.link(pHeld, "needs", pHolder)
	if p.waitsOn(pHeld) != pHolder {
		t.Errorf("a card moved into Prepared is not withheld")
	}
	p.at(pHeld, iDoing)
	if p.waitsOn(pHeld) != "" {
		t.Errorf("a card moved into Doing is withheld")
	}

	waiting := strings.Replace(initDefinition,
		`{ "id": "d00000000002", "title": "Doing"`,
		`{ "id": "d00000000009", "title": "Waiting", "kind": "work", "instructions": "i\n" },
    { "id": "d00000000002", "title": "Doing"`, 1)
	w := harnessFromDefinition(t, "wt", waiting)
	w.declare("d00000000009", "awaiting_outside", "true")
	if settings, _ := w.library.Bench.Holds(); settings.StartAt != iDoing {
		t.Errorf("behind a column awaiting somebody outside the default is %s, want Doing", settings.StartAt)
	}

	bare := `{"profile": "dinah-core/0.7", "title": "Bare", "instructions": "x\n", "columns": [
    { "id": "d00000000001", "title": "Intake", "kind": "intake", "instructions": "i\n" },
    { "id": "d00000000003", "title": "Done", "kind": "done", "instructions": "i\n" }]}`
	b := harnessFromDefinition(t, "br", bare)
	b.clock = holdsToday
	b.declareHolds("dinah.holds:\n  kinds:\n    after_start_of:\n      held: carrier\n      waits_for: start\n")
	bHolder := b.filedAt("holder", iIntake)
	bHeld := b.filedAt("held", iIntake)
	b.link(bHeld, "after_start_of", bHolder)
	if b.waitsOn(bHeld) != bHolder {
		t.Errorf("on a flow of Intake and Done the held card is not withheld")
	}
	b.at(bHolder, iDone)
	if b.waitsOn(bHeld) != "" {
		t.Errorf("with the holder in Done the held card is still withheld")
	}

	for _, startAt := range []string{"done", "intake"} {
		h.declareHolds("dinah.holds:\n  start_at: " + startAt + "\n" + rules)
		settings, defects := h.library.Bench.Holds()
		if settings.StartAt != iDoing || len(defects) != 1 || defects[0].Member != "start_at" {
			t.Errorf("start_at %s gives %s and %+v", startAt, settings.StartAt, defects)
		}
	}
}

// TestTheHoldIndexReadsOnlyTheJournalsItNeeds pins that the index opens a
// journal only for the holder of an edge whose rule carries a lag.
func TestTheHoldIndexReadsOnlyTheJournalsItNeeds(t *testing.T) {
	h := initHarness(t, "dinah.holds:\n  kinds:\n    needs:\n      held: carrier\n    cures:\n      held: carrier\n      lag_days: 2\n")
	first := h.filedAt("first", iIntake)
	second := h.filedAt("second", iIntake)
	third := h.filedAt("third", iIntake)
	h.link(first, "needs", second)
	count := func() int {
		journals := 0
		h.library.Observe = func(event, target string) {
			if event == ObserveJournal {
				journals++
			}
		}
		h.offerIn(iIntake)
		return journals
	}
	if got := count(); got != 0 {
		t.Errorf("rules with no lag opened %d journals", got)
	}
	h.link(first, "cures", third)
	if got := count(); got != 1 {
		t.Errorf("one lagged edge opened %d journals, want the holder's one", got)
	}
}

// TestAHoldDoesNotTouchTheStoreItReads guards that no read of a hold writes:
// the journals are byte for byte what they were.
func TestAHoldDoesNotTouchTheStoreItReads(t *testing.T) {
	h := holdsHarness(t, finishBlocks)
	holder := h.filedAt("holder", hImplement)
	held := h.filedAt("held", hBuildQueue)
	h.link(holder, "blocks", held)
	before := h.journals()
	h.offerIn(hBuildQueue)
	h.showCard(held)
	h.cycleFindings()
	after := h.journals()
	for path, text := range before {
		if after[path] != text {
			t.Errorf("a read wrote %s", path)
		}
	}
	if _, err := os.Stat(h.root); err != nil {
		t.Fatalf("the workbench is gone: %v", err)
	}
}

// TestOneUnreadableCardTakesNoOtherCardDown guards that a card whose anchor
// will not load, on a workbench declaring holds, takes down no answer about
// any other card. Every card view reads the hold index, so an index that
// failed on the one damaged card would fail show, next, the response of
// every mutating verb, and every claim. Here each of show, next, move, claim
// and link succeeds for the healthy cards, the move reports the column it
// carried the card to, the damaged card's own links hold nothing, and dinah
// check still names the damaged card.
func TestOneUnreadableCardTakesNoOtherCardDown(t *testing.T) {
	h := holdsHarness(t, finishBlocks)
	holder := h.filedAt("holder", hTriage)
	held := h.filedAt("held", hBuildQueue)
	free := h.filedAt("free", hBuildQueue)
	mover := h.filedAt("mover", hBuildQueue)
	claimed := h.filedAt("claimed", hTriage)
	damaged := h.filedAt("damaged", hBuildQueue)
	h.link(holder, "blocks", held)
	h.link(holder, "blocks", damaged)
	h.link(damaged, "blocks", free)
	cycleA := h.filedAt("cycle a", hBuildQueue)
	cycleB := h.filedAt("cycle b", hBuildQueue)
	h.link(cycleA, "blocks", cycleB)
	damagedID, moverID := h.cardID(damaged), h.cardID(mover)
	anchor := filepath.Join(h.card(damaged).Dir, bench.CardAnchor)
	if err := os.WriteFile(anchor, []byte("this file carries no anchor at all\n"), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}
	h.reopen()
	if _, err := h.library.Bench.Cards(); err == nil {
		t.Fatal("the plant did not take: Cards() still succeeds, so this proves nothing")
	}

	if got := h.waitsOn(held); got != holder {
		t.Errorf("show of the held card waits on %q, want %s", got, holder)
	}
	if got := h.waitsOn(free); got != "" {
		t.Errorf("a card the damaged card blocked waits on %q, want nothing", got)
	}
	// Next lists every card of the workbench before it reads a hold, and at
	// trunk that listing already fails on a card that will not load,
	// whatever the workbench declares. What the holds must not do is add a
	// failure of their own, so next is asked at the damaged card's own
	// column and at a column it does not stand in, with and without the
	// layer, and the answers must agree.
	nextAt := func(column string) string {
		offers, err := h.library.Next(&Request{Verb: "next", Actor: "brin", Column: column})
		if err != nil {
			return "error " + err.Error()
		}
		if len(offers) != 1 || offers[0].Card == nil {
			return fmt.Sprintf("%+v", offers)
		}
		return offers[0].Card.Ref
	}
	withLayer := []string{nextAt(hBuildQueue), nextAt(hTriage)}
	h.declareHolds("")
	h.reopen()
	withoutLayer := []string{nextAt(hBuildQueue), nextAt(hTriage)}
	h.declareHolds(finishBlocks)
	h.reopen()
	for i := range withLayer {
		if withLayer[i] != withoutLayer[i] {
			t.Errorf("next answers %q with the layer declared and %q without it", withLayer[i], withoutLayer[i])
		}
	}

	moved := h.do(&Request{Verb: Move, Card: mover, Actor: "alka", Column: hImplement})
	if moved.Outcome != contract.OutcomeOK || moved.Card == nil || moved.Card.Column != hImplement {
		t.Errorf("the move answered %s %s %+v, want ok with the card at Implement", moved.Outcome, moved.Refusal, moved.Card)
	}
	if got := h.card(mover).Column; got != hImplement {
		t.Errorf("the moved card stands at %s", got)
	}

	claim := h.do(&Request{Verb: Claim, Card: claimed, Actor: "brin"})
	if claim.Outcome != contract.OutcomeOK || claim.Card == nil || claim.Card.Ref != claimed {
		t.Errorf("the claim answered %s %s", claim.Outcome, claim.Refusal)
	}

	linked := h.library.Link(&Request{Verb: "link", Actor: "alka", Card: free, Kind: "blocks", LinkTo: claimed})
	if linked.Outcome != contract.OutcomeOK || linked.Warning != "" {
		t.Errorf("the link answered %s %s %s", linked.Outcome, linked.Refusal, linked.Warning)
	}
	h.reopen()
	// A link closing a cycle among the healthy cards still warns, and check
	// still reports that cycle, beside the damaged card.
	closing := h.library.Link(&Request{Verb: "link", Actor: "alka", Card: cycleB, Kind: "blocks", LinkTo: cycleA})
	if closing.Outcome != contract.OutcomeOK || closing.Warning != "warn.hold-cycle" || closing.WarningDetail != cycleA+", "+cycleB {
		t.Errorf("the closing link answered %s %s %q %q", closing.Outcome, closing.Refusal, closing.Warning, closing.WarningDetail)
	}
	h.reopen()
	if findings := h.cycleFindings(); len(findings) != 1 || findings[0].Detail != cycleA+", "+cycleB {
		t.Errorf("check reports the cycles %+v, want the healthy pair", findings)
	}

	report, err := h.library.Check(&Request{Verb: "check", Actor: "alka"})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	named := false
	for _, finding := range report.Findings {
		if strings.Contains(finding.Path, damagedID) {
			named = true
		}
		if strings.Contains(finding.Path, moverID) {
			t.Errorf("check reports the healthy moved card: %+v", finding)
		}
	}
	if !named {
		t.Errorf("check no longer names the damaged card: %+v", report.Findings)
	}
}

// TestAnUnreadableJournalWithholdsNothing is dinah-608/decisions/19: a lag
// holder whose journal will not read has no readable day, so it makes no
// lagging ground, and the held card's view and next both still answer.
func TestAnUnreadableJournalWithholdsNothing(t *testing.T) {
	h := initHarness(t, "dinah.holds:\n  kinds:\n    cures:\n      held: carrier\n      lag_days: 9\n")
	holder := h.filedAt("holder", iDoing)
	held := h.filedAt("held", iIntake)
	h.link(held, "cures", holder)
	h.at(holder, iDone)
	if got := h.waitsOn(held); got != holder {
		t.Fatalf("before the plant the held card waits on %q, want the lag on %s", got, holder)
	}
	journal := h.card(holder).JournalPath()
	raw, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(journal, append([]byte("not an event\n"), raw...), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}
	h.reopen()
	if _, _, err := bench.ReadJournal(journal); err == nil {
		t.Fatal("the plant did not take: the journal still reads")
	}
	if got := h.waitsOn(held); got != "" {
		t.Errorf("with the holder's journal unreadable the held card waits on %q, want nothing", got)
	}
	if got := h.offered(iIntake); got != held {
		t.Errorf("next at Intake offers %q, want %s", got, held)
	}
}

// ladder files a hold graph of layers cards two wide in Intake, every card
// waiting on both cards of the layer below, and the last layer's first card
// waiting on the first card of the top, which closes every card into one
// cycle. It answers the cards by layer.
func (h *harness) ladder(layers int) [][2]string {
	h.t.Helper()
	cards := make([][2]string, layers)
	for layer := range cards {
		for side := range cards[layer] {
			cards[layer][side] = h.filedAt(fmt.Sprintf("layer %d side %d", layer, side), iIntake)
		}
	}
	for layer := 0; layer+1 < layers; layer++ {
		for _, from := range cards[layer] {
			for _, to := range cards[layer+1] {
				h.link(from, "after", to)
			}
		}
	}
	h.link(cards[layers-1][0], "after", cards[0][0])
	return cards
}

// TestNotBeforeOnALargeCycleIsQuick is the Agent Code Review's layered case
// from dinah-608's first code review. The walk that kept its path read every
// simple path of the cycle, about 1.6 times as long for each layer added, so
// the 24 layers here took it well over half a minute. The small ladder pins
// the dates, which that walk gives too, because no edge of it carries a lag.
func TestNotBeforeOnALargeCycleIsQuick(t *testing.T) {
	block := "dinah.holds:\n  kinds:\n    after:\n      held: carrier\n    after3:\n      held: carrier\n      lag_days: 3\n"
	notBefore := func(h *harness, ref string) string {
		var dates []string
		for _, wait := range h.showCard(ref).WaitsOn {
			dates = append(dates, wait.Ref+"@"+wait.NotBefore)
		}
		return strings.Join(dates, ",")
	}
	t.Run("the dates on a small ladder", func(t *testing.T) {
		h := initHarness(t, block)
		cards := h.ladder(3)
		h.mustSet(cards[2][1], bench.StartAfterField, "2026-10-20")
		h.mustSet(cards[1][0], bench.StartAfterField, "2026-10-10")
		want := map[string]string{
			// The top reaches both dates through the layers below it.
			cards[0][0]: cards[1][0] + "@2026-10-20," + cards[1][1] + "@2026-10-20",
			cards[0][1]: cards[1][0] + "@2026-10-20," + cards[1][1] + "@2026-10-20",
			// The middle reads round the cycle from the bottom through the
			// top to its partner and on to the dated bottom card.
			cards[1][0]: cards[2][0] + "@2026-10-20," + cards[2][1] + "@2026-10-20",
			cards[1][1]: cards[2][0] + "@2026-10-20," + cards[2][1] + "@2026-10-20",
			// The bottom reads round the cycle and down to its dated
			// partner.
			cards[2][0]: cards[0][0] + "@2026-10-20",
		}
		for ref, expected := range want {
			if got := notBefore(h, ref); got != expected {
				t.Errorf("%s reads %q, want %q", ref, got, expected)
			}
		}
	})
	// A lag on an edge inside a cycle past the ground being read is not
	// added, which is dinah-608/decisions/18: a waits on b, b waits on c
	// with a lag of three, c waits on a, and c carries start_after X. The
	// path walk read a's ground on b as X+3, and this reads it as X, which is
	// still a floor on a ground no date releases. The ground's own lag is
	// always added, so c's ground on a, which reads round to b, is X+0 read
	// from b's own date, and b's ground on c is c's date plus three.
	t.Run("a lag inside a cycle", func(t *testing.T) {
		h := initHarness(t, block)
		a, b, c := h.filedAt("a", iIntake), h.filedAt("b", iIntake), h.filedAt("c", iIntake)
		h.link(a, "after", b)
		h.link(b, "after3", c)
		h.link(c, "after", a)
		h.mustSet(c, bench.StartAfterField, "2026-10-20")
		for ref, expected := range map[string]string{
			a: b + "@2026-10-20",
			b: c + "@2026-10-23",
			c: a + "@",
		} {
			if got := notBefore(h, ref); got != expected {
				t.Errorf("%s reads %q, want %q", ref, got, expected)
			}
		}
	})
	t.Run("twenty-four layers", func(t *testing.T) {
		h := initHarness(t, block)
		cards := h.ladder(24)
		h.mustSet(cards[23][1], bench.StartAfterField, "2026-10-20")
		done := make(chan string, 1)
		go func() {
			offers, err := h.library.Next(&Request{Verb: "next", Actor: "brin", Column: iIntake})
			if err != nil || len(offers) != 1 {
				done <- fmt.Sprintf("next answered %+v %v", offers, err)
				return
			}
			grounds, err := h.library.holdsOn(&Request{})
			if err != nil {
				done <- err.Error()
				return
			}
			var answers []string
			for _, ref := range []string{cards[0][0], cards[12][1], cards[23][0]} {
				for _, ground := range grounds.holdsOf(h.library.Bench, h.card(ref), bench.DateOf(2026, time.October, 3)) {
					answers = append(answers, ground.NotBefore)
				}
			}
			done <- strings.Join(answers, ",")
		}()
		select {
		case got := <-done:
			if want := "2026-10-20,2026-10-20,2026-10-20,2026-10-20,2026-10-20"; got != want {
				t.Errorf("the ladder reads %q, want %q", got, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("next and three cards' grounds on a 48-card cycle took more than ten seconds")
		}
	})
}
