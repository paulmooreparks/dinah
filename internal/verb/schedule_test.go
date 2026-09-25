package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// scheduleToday is the day every case below reads as today, and the fixture's
// clock stands at nine in the morning of it, UTC, which the workbench's default
// zone reads as the same day.
const scheduleToday = "2026-10-03"

// scheduleHarness is the standard fixture with its clock at scheduleToday and a
// dinah.schedule block declaring a seven-day soon window.
func scheduleHarness(t *testing.T) *harness {
	t.Helper()
	return scheduled(t, newHarness(t))
}

// scheduled sets a harness's clock to scheduleToday and declares the soon
// window, whatever fixture the harness was built from.
func scheduled(t *testing.T, h *harness) *harness {
	t.Helper()
	h.clock = time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	h.writeAnchorBlock(bench.ScheduleKey, bench.ScheduleKey+":\n  soon_days: 7\n")
	return h
}

// dated files a card at a column and gives it the dates named, as field and
// value pairs.
func (h *harness) dated(title, column string, dates ...string) string {
	h.t.Helper()
	ref := h.add(title)
	if column != intake && column != agendaIntake {
		h.at(ref, column)
	}
	for i := 0; i+1 < len(dates); i += 2 {
		h.mustSet(ref, dates[i], dates[i+1])
	}
	return ref
}

// offerAt is the offer next makes a caller at one column.
func (h *harness) offerAt(who caller, column string) Offer {
	h.t.Helper()
	req := who.request("")
	req.Verb, req.Column = "next", column
	offers, err := h.library.Next(req)
	if err != nil || len(offers) != 1 {
		h.t.Fatalf("next at %s answered %+v %v", column, offers, err)
	}
	return offers[0]
}

// journals is every journal file of the workbench and its bytes, which is how
// a case asserts that an answer wrote nothing to any of them.
func (h *harness) journals() map[string]string {
	h.t.Helper()
	found := map[string]string{}
	err := filepath.WalkDir(h.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != bench.JournalName {
			return err
		}
		data, readErr := os.ReadFile(path)
		found[path] = string(data)
		return readErr
	})
	if err != nil || len(found) == 0 {
		h.t.Fatalf("read the journals: %d read, %v", len(found), err)
	}
	return found
}

// TestTheBoundaryTableHolds is dinah-605/criteria/4: every row of the boundary
// table in the specification's section 3, read off the JSON card view with
// today at 2026-10-03 and a seven-day window.
//
// Two rows were re-pinned at dinah-608 under its decision 10, which makes
// started a position in the flow. The fixture's commitment column defaults to
// Doing, so a card standing in Aftercare has started whether or not anybody
// claimed it. The rows that exercise late_start now file their card in Intake,
// before the commitment column, where it still reads; the row that stood a
// never-claimed card in Aftercare now pins decision 10's answer there, which
// is no late_start.
func TestTheBoundaryTableHolds(t *testing.T) {
	h := scheduleHarness(t)
	rows := []struct {
		name   string
		column string
		dates  []string
		after  func(ref string)
		want   string
	}{
		{name: "due yesterday", column: aftercare, dates: []string{bench.DueField, "2026-10-02"}, want: "overdue"},
		{name: "due today", column: aftercare, dates: []string{bench.DueField, "2026-10-03"}, want: "due_soon"},
		{name: "due at the window's edge", column: aftercare, dates: []string{bench.DueField, "2026-10-10"}, want: "due_soon"},
		{name: "due past the window", column: aftercare, dates: []string{bench.DueField, "2026-10-11"}, want: ""},
		{name: "due yesterday and finished", column: finished, dates: []string{bench.DueField, "2026-10-02"}, want: ""},
		{name: "start_by yesterday, never claimed", column: intake, dates: []string{bench.StartByField, "2026-10-02"}, want: "late_start"},
		{name: "start_by yesterday, never claimed, past the commitment column", column: aftercare, dates: []string{bench.StartByField, "2026-10-02"}, want: ""},
		{name: "start_by yesterday, claimed and released", column: aftercare, dates: []string{bench.StartByField, "2026-10-02"}, after: func(ref string) {
			h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: ref})
			h.mustDo(&Request{Verb: Release, Actor: "alka", Card: ref})
		}, want: ""},
		{name: "start_after tomorrow", column: aftercare, dates: []string{bench.StartAfterField, "2026-10-04"}, want: "not_yet"},
		{name: "start_after today", column: aftercare, dates: []string{bench.StartAfterField, "2026-10-03"}, want: ""},
		{name: "late to start and due soon", column: intake, dates: []string{bench.StartByField, "2026-10-01", bench.DueField, "2026-10-08"}, want: "late_start,due_soon"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			ref := h.dated(row.name, row.column, row.dates...)
			if row.after != nil {
				row.after(ref)
			}
			view := h.cardView(ref)
			if got := strings.Join(view.Schedule, ","); got != row.want {
				t.Errorf("the card holds [%s], want [%s]", got, row.want)
			}
			encoded, err := json.Marshal(view)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			member := `"schedule":["` + strings.ReplaceAll(row.want, ",", `","`) + `"]`
			if row.want == "" {
				if strings.Contains(string(encoded), `"schedule"`) {
					t.Errorf("a card holding nothing carries a schedule member: %s", encoded)
				}
				return
			}
			if !strings.Contains(string(encoded), member) {
				t.Errorf("the JSON view lacks %s: %s", member, encoded)
			}
		})
	}
}

// TestEverySurfaceReadsTodayInTheWorkbenchZone is dinah-605/criteria/3: with
// the clock at 2026-10-02T17:00:00Z, a workbench in Singapore reads today as
// 2026-10-03 and one naming no zone reads 2026-10-02, and the conditions, the
// selection and a relative query all follow the one reading.
func TestEverySurfaceReadsTodayInTheWorkbenchZone(t *testing.T) {
	for _, c := range []struct {
		zone   string
		notYet bool
	}{
		{zone: "dinah.schedule:\n  time_zone: Asia/Singapore\n", notYet: false},
		{zone: "", notYet: true},
	} {
		h := newHarness(t)
		h.clock = time.Date(2026, 10, 2, 17, 0, 0, 0, time.UTC)
		if c.zone != "" {
			h.writeAnchorBlock(bench.ScheduleKey, c.zone)
		}
		ref := h.dated("starts on the third", aftercare, bench.StartAfterField, "2026-10-03")
		held := strings.Join(h.cardView(ref).Schedule, ",") == contract.ScheduleNotYet
		if held != c.notYet {
			t.Errorf("zone %q: the card holds not_yet %v, want %v", c.zone, held, c.notYet)
		}
		offer := h.offerAt(asOperator, aftercare)
		if (offer.Card == nil) != c.notYet {
			t.Errorf("zone %q: next offered %+v", c.zone, offer)
		}
		matches, err := h.library.Query(&Request{Verb: "query", Actor: "alka", Query: "start_after:today"})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if (matches.Count == 1) == c.notYet {
			t.Errorf("zone %q: start_after:today matched %d", c.zone, matches.Count)
		}
	}
}

// TestNextWithholdsANotYetCard is dinah-605/criteria/5, over every position the
// specification's section 4.2 table names, for an agent and for the operator.
func TestNextWithholdsANotYetCard(t *testing.T) {
	for _, who := range []caller{asOperator, {actor: "brin"}} {
		t.Run(who.actor, func(t *testing.T) {
			h := scheduleHarness(t)
			early := h.dated("not yet", aftercare, bench.StartAfterField, "2026-10-04")
			plain := h.dated("no dates", aftercare)
			if offer := h.offerAt(who, aftercare); offer.Card == nil || offer.Card.Ref != plain || offer.NotYet {
				t.Errorf("with a dateless card behind it, the column offered %+v", offer)
			}
			h.mustDo(&Request{Verb: Move, Actor: "alka", Card: plain, Column: "closed"})
			offer := h.offerAt(who, aftercare)
			if offer.Card != nil || !offer.NotYet || offer.StartableFrom != "2026-10-04" || offer.ReadyCount != 1 || offer.AboveTier || offer.NoTaker {
				t.Errorf("with the not-yet card alone, the column offered %+v", offer)
			}
			h.mustSet(early, bench.StartAfterField, scheduleToday)
			if offer := h.offerAt(who, aftercare); offer.Card == nil || offer.Card.Ref != early {
				t.Errorf("a card whose start_after is today was not offered: %+v", offer)
			}
		})
	}
}

// TestABufferHoldingOnlyNotYetWorkIsNotANoTaker is the buffer half of
// dinah-605/criteria/5: a buffer whose ready card will be carried into a
// station once its date comes reports the date, and a buffer whose ready card
// has no landing still reports that nothing is taken from it.
func TestABufferHoldingOnlyNotYetWorkIsNotANoTaker(t *testing.T) {
	h := scheduled(t, agendaHarnessFrom(t, offersDefinition))
	waiting := h.dated("waiting in the buffer", agendaQueue, bench.StartAfterField, "2026-10-06")
	offer := h.offerAt(asOperator, agendaQueue)
	if offer.Card != nil || !offer.NotYet || offer.StartableFrom != "2026-10-06" || offer.NoTaker {
		t.Errorf("the buffer holding only a not-yet card offered %+v", offer)
	}
	h.onRoute(waiting, "stuck")
	offer = h.offerAt(asOperator, agendaQueue)
	if !offer.NoTaker || offer.NotYet {
		t.Errorf("the buffer holding only a card with no landing offered %+v", offer)
	}
}

// TestTierAndScheduleWithholdTogether is the tier half of
// dinah-605/criteria/5: a column holding one card withheld on each ground
// carries both answers.
func TestTierAndScheduleWithholdTogether(t *testing.T) {
	h := scheduled(t, agendaHarness(t))
	h.dated("not yet", agendaStation, bench.StartAfterField, "2026-10-05")
	senior := h.dated("frontier work", agendaStation)
	h.setTier(senior, "frontier")
	offer := h.offerAt(asWorkhorse, agendaStation)
	if offer.Card != nil || !offer.AboveTier || !offer.NotYet || offer.StartableFrom != "2026-10-05" {
		t.Errorf("the column offered %+v, want above_tier and not_yet both", offer)
	}
}

// TestAnInjectedHoldIsTheOnlyThingTheScanReads is dinah-605/criteria/21: the
// scan asks the start hold it is handed and reads no date itself, so a hold
// reporting a dateless card held is obeyed, and the production hold offers the
// same card.
func TestAnInjectedHoldIsTheOnlyThingTheScanReads(t *testing.T) {
	h := scheduleHarness(t)
	ref := h.dated("no dates at all", aftercare)
	cards, err := h.library.Bench.Cards()
	if err != nil {
		t.Fatalf("cards: %v", err)
	}
	column := h.library.Bench.Column(aftercare)
	admit := selectionAdmission(h.library.Bench, &Request{Actor: "alka"})
	injected := func(*bench.Card) holdAnswer { return holdAnswer{Held: true, From: "2026-10-09"} }
	offer, err := h.library.offerFor(column, cards, admit, injected, h.library.dayOf(&Request{}))
	if err != nil || offer.Card != nil || !offer.NotYet || offer.StartableFrom != "2026-10-09" {
		t.Errorf("the injected hold gave %+v %v", offer, err)
	}
	offer, err = h.library.offerFor(column, cards, admit, mustSelectionHold(t, h.library), h.library.dayOf(&Request{}))
	if err != nil || offer.Card == nil || offer.Card.Ref != ref {
		t.Errorf("the production hold gave %+v %v", offer, err)
	}
}

// TestPullAnswersNotYet is dinah-605/criteria/6: both forms, with and without
// --no-claim, answer the not-yet message with the earliest withheld date, at
// ok, writing nothing to any journal.
func TestPullAnswersNotYet(t *testing.T) {
	for _, noClaim := range []bool{false, true} {
		h := scheduleHarness(t)
		h.dated("later", intake, bench.StartAfterField, "2026-10-09")
		h.dated("sooner", intake, bench.StartAfterField, "2026-10-06")
		before := h.journals()
		named := h.library.Pull(&Request{Verb: Pull, Actor: "brin", Column: "doing", NoClaim: noClaim})
		if named.Outcome != contract.OutcomeOK || named.Message != "answer.pull.not-yet.named" || named.MessageValues["date"] != "2026-10-06" || named.Card != nil {
			t.Errorf("no-claim %v: the named pull answered %+v", noClaim, named)
		}
		bare := h.library.Pull(&Request{Verb: Pull, Actor: "brin", NoClaim: noClaim})
		if bare.Outcome != contract.OutcomeOK || bare.Message != "answer.pull.not-yet.bare" || bare.MessageValues["date"] != "2026-10-06" {
			t.Errorf("no-claim %v: the bare pull answered %+v", noClaim, bare)
		}
		if after := h.journals(); len(after) != len(before) {
			t.Errorf("no-claim %v: the journals changed in number", noClaim)
		} else {
			for path, text := range before {
				if after[path] != text {
					t.Errorf("no-claim %v: %s was written", noClaim, path)
				}
			}
		}
	}
}

// TestANamedPullReportsADateFromFurtherBack is the further-walk half of
// dinah-605/criteria/6: the not-yet card stands at intake, behind an empty
// buffer, and the named pull into the station reports its date.
func TestANamedPullReportsADateFromFurtherBack(t *testing.T) {
	h := scheduled(t, agendaHarness(t))
	h.dated("waiting at intake", agendaIntake, bench.StartAfterField, "2026-10-07")
	answer := h.library.Pull(&Request{Verb: Pull, Actor: "alka", Column: "skipped"})
	if answer.Message != "answer.pull.not-yet.named" || answer.MessageValues["date"] != "2026-10-07" {
		t.Errorf("the named pull answered %+v", answer)
	}
}

// TestTierWinsOverScheduleInThePullAnswer is the precedence half of
// dinah-605/criteria/6.
func TestTierWinsOverScheduleInThePullAnswer(t *testing.T) {
	h := scheduled(t, agendaHarness(t))
	h.dated("not yet", agendaQueue, bench.StartAfterField, "2026-10-05")
	senior := h.dated("frontier work", agendaQueue)
	h.setTier(senior, "frontier")
	req := asWorkhorse.request("")
	req.Verb, req.Column = Pull, "skipped"
	if answer := h.library.Pull(req); answer.Message != "answer.pull.above-tier.named" {
		t.Errorf("the named pull answered %+v", answer)
	}
	bare := asWorkhorse.request("")
	bare.Verb = Pull
	if answer := h.library.Pull(bare); answer.Message != "answer.pull.above-tier.bare" {
		t.Errorf("the bare pull answered %+v", answer)
	}
}

// TestAClaimByNameWarnsAndSucceeds is dinah-605/criteria/7.
func TestAClaimByNameWarnsAndSucceeds(t *testing.T) {
	h := scheduleHarness(t)
	early := h.dated("not yet", aftercare, bench.StartAfterField, "2026-10-06")
	claimed := h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: early})
	if claimed.Warning != "warn.before-start-after" || claimed.WarningDetail != "2026-10-06" {
		t.Errorf("the claim carried %q %q", claimed.Warning, claimed.WarningDetail)
	}
	today := h.dated("starts today", aftercare, bench.StartAfterField, scheduleToday)
	if response := h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: today}); response.Warning != "" {
		t.Errorf("a claim of a card starting today carried %q", response.Warning)
	}
	moved := h.dated("moved early", intake, bench.StartAfterField, "2026-10-06")
	if response := h.mustDo(&Request{Verb: Move, Actor: "alka", Card: moved, Column: "aftercare"}); response.Warning != "" {
		t.Errorf("a move carried %q", response.Warning)
	}
}

// TestAnOutOfOrderWriteSucceedsAndWarns is dinah-605/criteria/8, for set and
// for add, naming the first violated pair.
func TestAnOutOfOrderWriteSucceedsAndWarns(t *testing.T) {
	h := scheduleHarness(t)
	ref := h.dated("dated", aftercare, bench.DueField, "2026-10-10")
	response := h.mustSet(ref, bench.StartByField, "2026-10-12")
	if response.Warning != "warn.schedule-order" || response.WarningDetail != "start_by 2026-10-12 is after due 2026-10-10" {
		t.Errorf("set answered %q %q", response.Warning, response.WarningDetail)
	}
	if h.card(ref).StartBy != "2026-10-12" {
		t.Error("the out-of-order write did not store")
	}
	response = h.mustSet(ref, bench.StartAfterField, "2026-10-13")
	if response.WarningDetail != "start_after 2026-10-13 is after start_by 2026-10-12" {
		t.Errorf("with every pair out of order the warning named %q", response.WarningDetail)
	}
	added := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "added", StartAfter: "2026-10-20", Due: "2026-10-01"})
	if added.Outcome != contract.OutcomeOK || added.Warning != "warn.schedule-order" || added.WarningDetail != "start_after 2026-10-20 is after due 2026-10-01" {
		t.Errorf("add answered %s %q %q", added.Outcome, added.Warning, added.WarningDetail)
	}
	if clean := h.mustSet(ref, bench.StartAfterField, "2026-10-01"); clean.Warning == "warn.schedule-order" && strings.HasPrefix(clean.WarningDetail, "start_after") {
		t.Errorf("an in-order start_after still warned %q", clean.WarningDetail)
	}
}

// TestAddStoresTheThreeDates is dinah-605/criteria/9: each flag stores its
// value, and a malformed one refuses the whole filing and leaves no card.
func TestAddStoresTheThreeDates(t *testing.T) {
	h := scheduleHarness(t)
	added := h.library.Add(&Request{Verb: "add", Actor: "alka", Title: "trip", StartAfter: "2026-10-06", StartBy: "2026-10-08", Due: "2026-10-10"})
	if added.Outcome != contract.OutcomeOK {
		t.Fatalf("add answered %s %s", added.Outcome, added.Refusal)
	}
	h.reopen()
	card := h.card(added.Card.Ref)
	if card.StartAfter != "2026-10-06" || card.StartBy != "2026-10-08" || card.Due != "2026-10-10" {
		t.Errorf("the card stores %q %q %q", card.StartAfter, card.StartBy, card.Due)
	}
	before, _ := bench.ListIDs(h.library.Bench.CardsRoot())
	for _, bad := range []Request{
		{StartAfter: "2026-10-1"},
		{StartBy: "today"},
		{Due: "2026-02-30"},
	} {
		bad.Verb, bad.Actor, bad.Title = "add", "alka", "refused"
		response := h.library.Add(&bad)
		if response.Outcome != contract.OutcomeRefused || response.Refusal != contract.Malformed {
			t.Errorf("%+v answered %s %s", bad, response.Outcome, response.Refusal)
		}
	}
	after, _ := bench.ListIDs(h.library.Bench.CardsRoot())
	if len(after) != len(before) {
		t.Errorf("a refused filing left a card: %d cards before, %d after", len(before), len(after))
	}
}

// TestADateWriteIsAcceptedAtFormatNineAndTen is dinah-605/criteria/1 and the
// write half of criteria/10: set stores the value top-level, get reads it,
// a set with no value clears it, the guard refuses what the calendar does not
// carry, and a format-9 workbench is written without its format moving.
func TestADateWriteIsAcceptedAtFormatNineAndTen(t *testing.T) {
	for _, format := range []string{"9", "10"} {
		h := scheduleHarness(t)
		anchor := filepath.Join(h.root, bench.WorkbenchAnchor)
		text, _ := bench.ReadText(anchor)
		fm, body := bench.ParseAnchor(text)
		fm.Set("format", format)
		if err := bench.WriteText(anchor, fm.Render(body)); err != nil {
			t.Fatalf("stamp %s: %v", format, err)
		}
		h.reopen()
		ref := h.dated("dated", aftercare)
		for _, field := range bench.ScheduleFields {
			h.mustSet(ref, field, "2026-10-06")
			stored, _ := bench.ReadText(h.card(ref).AnchorPath())
			if !strings.Contains(stored, "\n"+field+": 2026-10-06\n") {
				t.Errorf("format %s: %s is not stored top-level:\n%s", format, field, stored)
			}
			got, err := h.library.GetField(&Request{Verb: "get", Actor: "alka", Ref: ref, Field: field})
			if err != nil || got != "2026-10-06" {
				t.Errorf("format %s: get %s answered %q %v", format, field, got, err)
			}
			for _, refused := range []string{"2026-02-30", "2026-10-1", "today"} {
				if response := h.set(ref, field, refused); response.Refusal != contract.Malformed {
					t.Errorf("format %s: %s %s answered %s %s", format, field, refused, response.Outcome, response.Refusal)
				}
			}
			h.mustSet(ref, field, "2026-02-28")
			h.mustSet(ref, field, "")
			if stored, _ := bench.ReadText(h.card(ref).AnchorPath()); strings.Contains(stored, field+":") {
				t.Errorf("format %s: clearing %s left it behind", format, field)
			}
		}
		if after, _ := bench.ReadText(anchor); !strings.Contains(after, "\nformat: "+format+"\n") {
			t.Errorf("a date write moved the format off %s", format)
		}
	}
}

// queryRefs is the references a query selects, in arrival order.
func (h *harness) queryRefs(text string) ([]string, error) {
	h.t.Helper()
	matches, err := h.library.Query(&Request{Verb: "query", Actor: "alka", Query: text})
	if err != nil {
		return nil, err
	}
	var refs []string
	for _, card := range matches.Cards {
		refs = append(refs, card.Ref)
	}
	return refs, nil
}

// refusalOf is the refusal name an error carries, empty where it carries none.
func refusalOf(err error) string {
	if refusal, ok := err.(*contract.Refusal); ok {
		return refusal.Name
	}
	return ""
}

// TestDateTermsCompareAndResolve is dinah-605/criteria/11.
func TestDateTermsCompareAndResolve(t *testing.T) {
	h := scheduleHarness(t)
	h.declareFields("fields:\n  trip.depart:\n    type: date\n    meaning: the day the trip leaves\n    on: [card]\n")
	hotel := h.dated("hotel", aftercare, bench.StartAfterField, "2026-10-06", bench.DueField, "2026-10-10")
	approval := h.dated("approval", aftercare, bench.StartByField, "2026-10-01", bench.DueField, "2026-10-08")
	passport := h.dated("passport", aftercare, bench.DueField, "2026-10-02")
	claim := h.dated("claim", aftercare, bench.StartAfterField, "2026-10-20", bench.DueField, "2026-11-19")
	rail := h.dated("rail", aftercare)
	h.mustSet(rail, "trip.depart", "2026-10-05")
	cases := map[string][]string{
		"due>=today due<=today+7": {hotel, approval},
		"start_by<today":          {approval},
		`due:""`:                  {rail},
		`due!=""`:                 {hotel, approval, passport, claim},
		"due:today-1":             {passport},
		"due!=2026-10-02":         {hotel, approval, claim, rail},
		"due>2026-10-10":          {claim},
		"due>=2026-10-10":         {hotel, claim},
		"due<=2026-10-08":         {approval, passport},
		"start_after>today+10":    {claim},
		"trip.depart>=today":      {rail},
		"trip.depart<today+2":     {},
		"trip.depart:2026-10-05":  {rail},
	}
	for query, want := range cases {
		got, err := h.queryRefs(query)
		if err != nil {
			t.Errorf("%s refused: %v", query, err)
			continue
		}
		if refSet(got) != refSet(want) {
			t.Errorf("%s selected [%s], want [%s]", query, refSet(got), refSet(want))
		}
	}
	for query, want := range map[string]string{
		"due>=tomorrow":              contract.Malformed,
		"due:today+":                 contract.Malformed,
		"due>=2026-10-01,2026-10-02": contract.Malformed,
		"trip.depart:2026-1-5":       contract.Malformed,
		"holder>=x":                  contract.UnknownField,
		"at>=today":                  contract.Malformed,
		`due>=""`:                    contract.Malformed,
		`due<=""`:                    contract.Malformed,
		`trip.depart>=""`:            contract.Malformed,
		`trip.depart<""`:             contract.Malformed,
	} {
		if _, err := h.queryRefs(query); refusalOf(err) != want {
			t.Errorf("%s answered %v, want %s", query, err, want)
		}
	}
}

// TestTheScheduleFieldSelectsByCondition is dinah-605/criteria/12. Its late
// card stands in Intake since dinah-608's decision 10, because a card standing
// past the commitment column has started and reads no late_start.
func TestTheScheduleFieldSelectsByCondition(t *testing.T) {
	h := scheduleHarness(t)
	both := h.dated("late and due soon", intake, bench.StartByField, "2026-10-01", bench.DueField, "2026-10-08")
	overdue := h.dated("overdue", aftercare, bench.DueField, "2026-10-02")
	early := h.dated("not yet and due soon", aftercare, bench.StartAfterField, "2026-10-06", bench.DueField, "2026-10-10")
	plain := h.dated("nothing", aftercare)
	cases := map[string][]string{
		"schedule:overdue,late_start": {both, overdue},
		"schedule:due_soon":           {both, early},
		"schedule!=not_yet":           {both, overdue, plain},
		`schedule:""`:                 {plain},
		`schedule!=""`:                {both, overdue, early},
	}
	for query, want := range cases {
		got, err := h.queryRefs(query)
		if err != nil || refSet(got) != refSet(want) {
			t.Errorf("%s selected [%s] %v, want [%s]", query, refSet(got), err, refSet(want))
		}
	}
	_, err := h.queryRefs("schedule:late")
	refusal, ok := err.(*contract.Refusal)
	if !ok || refusal.Name != contract.UnknownValue || refusal.Extra["legal"] != "overdue, late_start, due_soon, start_soon, not_yet, waiting" {
		t.Errorf("schedule:late answered %+v", err)
	}
	_, err = h.queryRefs("schedule>=x")
	refusal, ok = err.(*contract.Refusal)
	if !ok || refusal.Name != contract.UnknownField || refusal.Extra["orderedFields"] != "start_after, start_by, due, at" {
		t.Errorf("schedule>=x answered %+v", err)
	}
	h.declareFields("fields:\n  trip.depart:\n    type: date\n    meaning: the day the trip leaves\n    on: [card]\n  trip.note:\n    type: string\n    meaning: a note\n    on: [card]\n")
	_, err = h.queryRefs("trip.note>=x")
	refusal, ok = err.(*contract.Refusal)
	if !ok || refusal.Name != contract.UnknownField || refusal.Extra["orderedFields"] != "start_after, start_by, due, at, trip.depart" {
		t.Errorf("a string field under an ordered operator answered %+v", err)
	}
}

// TestTheAgendaWithholdsNotYetWorkFromItsOfferedArm is dinah-605/criteria/19.
func TestTheAgendaWithholdsNotYetWorkFromItsOfferedArm(t *testing.T) {
	h := scheduled(t, agendaHarness(t))
	early := h.dated("not yet at the station", agendaStation, bench.StartAfterField, "2026-10-06")
	review := h.dated("not yet at his review", agendaReview, bench.StartAfterField, "2026-10-06")
	if got := ranked(h.explained("agenda", asWorkhorse), 0); strings.Contains(" "+refSet(got)+" ", " "+early+" ") {
		t.Errorf("an agent's agenda holds the not-yet card: %v", got)
	}
	if got := h.nextRefs(asWorkhorse); len(got) != 0 {
		t.Errorf("next offers the agent %v", got)
	}
	got := ranked(h.explained("agenda", asOperator), 0)
	if !strings.Contains(" "+refSet(got)+" ", " "+review+" ") {
		t.Errorf("the operator's agenda lost the not-yet card at his own column: %v", got)
	}
	if strings.Contains(" "+refSet(got)+" ", " "+early+" ") {
		t.Errorf("the operator's agenda holds the not-yet card at a station he does not own: %v", got)
	}
}

// TestPrimeListsANotYetColumn is the verb half of dinah-605/criteria/20: the
// ready list carries a column whose only ready work waits on a date, with the
// date, and one also holding work above the caller carries both flags.
func TestPrimeListsANotYetColumn(t *testing.T) {
	h := scheduled(t, agendaHarness(t))
	h.dated("not yet", agendaStation, bench.StartAfterField, "2026-10-06")
	primer, err := h.library.Prime(asWorkhorse.request(""))
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	if len(primer.Ready) != 1 || primer.Ready[0].Column != agendaStation || !primer.Ready[0].NotYet || primer.Ready[0].StartableFrom != "2026-10-06" {
		t.Errorf("the ready list is %+v", primer.Ready)
	}
	senior := h.dated("frontier work", agendaStation)
	h.setTier(senior, "frontier")
	primer, err = h.library.Prime(asWorkhorse.request(""))
	if err != nil || len(primer.Ready) != 1 || !primer.Ready[0].AboveTier || !primer.Ready[0].NotYet {
		t.Errorf("with work above the tier the ready list is %+v %v", primer.Ready, err)
	}
}

// TestAMalformedStoredDateWithholdsNothing is the stored-value half of
// dinah-605/criteria/15: a date that does not parse contributes no condition
// and withholds nothing.
func TestAMalformedStoredDateWithholdsNothing(t *testing.T) {
	h := scheduleHarness(t)
	ref := h.dated("hand-edited", aftercare)
	card := h.card(ref)
	text, _ := bench.ReadText(card.AnchorPath())
	if err := bench.WriteText(card.AnchorPath(), strings.Replace(text, "state: ready", "state: ready\nstart_after: 2099-1-1\ndue: soon", 1)); err != nil {
		t.Fatalf("plant: %v", err)
	}
	h.reopen()
	if view := h.cardView(ref); len(view.Schedule) != 0 || view.Due != "soon" {
		t.Errorf("the view reads %+v", view)
	}
	if offer := h.offerAt(asOperator, aftercare); offer.Card == nil || offer.Card.Ref != ref {
		t.Errorf("a malformed start_after withheld the card: %+v", offer)
	}
}

// TestAUserConfigCarriesNoSchedule is the config half of
// dinah-605/criteria/2: dinah.schedule in the user's config.md changes nothing.
func TestAUserConfigCarriesNoSchedule(t *testing.T) {
	h := newHarness(t)
	h.clock = time.Date(2026, 10, 2, 17, 0, 0, 0, time.UTC)
	config := filepath.Join(h.home, bench.UserBaseName, "config.md")
	if err := bench.WriteText(config, "---\ndinah.schedule:\n  time_zone: Asia/Singapore\n---\n"); err != nil {
		t.Fatalf("write the config: %v", err)
	}
	h.reopen()
	if got := h.library.Bench.Today(h.library.Now()).String(); got != "2026-10-02" {
		t.Errorf("a config naming Singapore moved today to %s", got)
	}
}

// viewToday is a card's view drawn on the day the fixture's clock reads, for
// the cases that ask about one card and have no request to read the day from.
func (l *Library) viewToday(card *bench.Card) (*CardView, error) {
	return l.view(card, l.dayOf(&Request{}))
}

// mustSelectionHold is the start hold a fresh request on library builds,
// failing the test where the hold index cannot be read.
func mustSelectionHold(t *testing.T, library *Library) startHold {
	t.Helper()
	hold, err := library.selectionHold(&Request{})
	if err != nil {
		t.Fatalf("selectionHold: %v", err)
	}
	return hold
}

// TestOneAnswerIsDrawnOnOneDay runs each reading verb on a clock that moves a
// whole day forward every time anything reads it, so a verb reading today
// again for each card it draws draws each card on a later day than the one
// before. Every card is due on the fixture's day, which reads as due_soon on
// that day and as overdue on any later one. Each answer must draw all its
// cards on one day, and a query or a view section asking for due_soon must
// draw every card it selected as due_soon, which it can only do on the day it
// selected them.
func TestOneAnswerIsDrawnOnOneDay(t *testing.T) {
	h := scheduleHarness(t)
	h.declareViews(bench.ViewsKey + ":\n  soon:\n    sections:\n      - query: schedule:due_soon\n")
	first := h.dated("first", aftercare, bench.DueField, scheduleToday)
	second := h.dated("second", aftercare, bench.DueField, scheduleToday)
	third := h.dated("third", doing, bench.DueField, scheduleToday)
	reads := 0
	advancing := func() time.Time {
		reads++
		return time.Date(2026, 10, 3, 23, 0, 0, 0, time.UTC).AddDate(0, 0, reads-1)
	}
	h.library.Now = advancing
	// drawn fails the case unless the answer drew want cards, all on one day,
	// and, where condition is not empty, every one carrying that condition.
	drawn := func(verb string, views []CardView, want int, condition string) {
		t.Helper()
		if len(views) != want {
			t.Fatalf("%s drew %d cards, want %d", verb, len(views), want)
		}
		for _, view := range views {
			if view.ScheduleDay != views[0].ScheduleDay {
				t.Errorf("%s drew %s on %s and %s on %s", verb, views[0].Ref, views[0].ScheduleDay, view.Ref, view.ScheduleDay)
			}
			if got := strings.Join(view.Schedule, ","); condition != "" && got != condition {
				t.Errorf("%s selected %s as %s and drew it on %s as %s", verb, view.Ref, condition, view.ScheduleDay, got)
			}
		}
	}

	reads = 0
	matches, err := h.library.Query(&Request{Verb: "query", Actor: "alka", Query: "schedule:due_soon"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	drawn("query", matches.Cards, 3, contract.ScheduleDueSoon)

	reads = 0
	answer, err := h.library.DrawView(&Request{Verb: "view", Actor: "alka", View: "soon"})
	if err != nil || len(answer.View.Sections) != 1 {
		t.Fatalf("view: %+v %v", answer, err)
	}
	drawn("view", answer.View.Sections[0].Cards, 3, contract.ScheduleDueSoon)

	reads = 0
	listing, err := h.library.List(&Request{Verb: "list", Actor: "alka"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	drawn("list", listing.Cards, 3, "")

	reads = 0
	offers, err := h.library.Next(&Request{Verb: "next", Actor: h.library.Bench.Operator})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	var offered []CardView
	for _, offer := range offers {
		if offer.Card != nil {
			offered = append(offered, *offer.Card)
		}
	}
	drawn("next", offered, 2, "")

	for _, ref := range []string{first, second, third} {
		if response := h.do(&Request{Verb: "claim", Actor: "alka", Card: ref}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("claim %s: %+v", ref, response)
		}
	}
	// A claim reopens the library on the fixture's still clock, so the
	// advancing one goes back in before prime reads it.
	h.library.Now = advancing
	reads = 0
	primer, err := h.library.Prime(&Request{Verb: "prime", Actor: "alka"})
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	drawn("prime", primer.Holding, 3, "")
}

// TestARootWalkJudgesEachWorkbenchOnItsOwnDay pins the day to the workbench
// that read it. A root walk hands one request to every workbench in turn, and
// a workbench in Auckland and one in Los Angeles fall on different dates for
// most of every day: at 2026-10-02T17:00:00Z Auckland reads 2026-10-03 and
// Los Angeles reads 2026-10-02. A card starting on the third is therefore
// offered, listed as startable and held without a condition in Auckland, and
// withheld, listed as not_yet and held as not_yet in Los Angeles, whichever
// workbench the walk reaches first. A request that carried the first
// workbench's day into the second would judge one of the two on the wrong
// day, and the walk's order decides which.
func TestARootWalkJudgesEachWorkbenchOnItsOwnDay(t *testing.T) {
	root := t.TempDir()
	zones := map[string]string{"auckland": "Pacific/Auckland", "los-angeles": "America/Los_Angeles"}
	wantDay := map[string]string{"auckland": "2026-10-03", "los-angeles": "2026-10-02"}
	for name, zone := range zones {
		h := newHarness(t)
		h.writeAnchorBlock(bench.ScheduleKey, bench.ScheduleKey+":\n  time_zone: "+zone+"\n")
		h.dated("starts on the third", aftercare, bench.StartAfterField, "2026-10-03")
		held := h.dated("held from the third", aftercare, bench.StartAfterField, "2026-10-03")
		if response := h.do(&Request{Verb: Claim, Actor: "alka", Card: held}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("claim in %s: %+v", name, response)
		}
		if err := copyTree(h.root, filepath.Join(root, name, bench.UserBaseName, harnessWorkbenchID)); err != nil {
			t.Fatalf("copy %s: %v", name, err)
		}
	}
	forestClock = func() time.Time { return time.Date(2026, 10, 2, 17, 0, 0, 0, time.UTC) }
	defer func() { forestClock = time.Now }()
	// nameOf is which workbench a row is, read from the path the walk found
	// it at, which is root/<name>/<user base>/<id>.
	nameOf := func(path string) string {
		return filepath.Base(filepath.Dir(filepath.Dir(path)))
	}
	// judged fails the case unless the view was drawn on its own
	// workbench's day and carries not_yet exactly where that day is before
	// the third.
	judged := func(verb, name string, view CardView) {
		t.Helper()
		if got := view.ScheduleDay.String(); got != wantDay[name] {
			t.Errorf("%s drew %s in %s on %s, want %s", verb, view.Ref, name, got, wantDay[name])
		}
		notYet := strings.Join(view.Schedule, ",") == contract.ScheduleNotYet
		if want := wantDay[name] == "2026-10-02"; notYet != want {
			t.Errorf("%s drew %s in %s with not_yet %v, want %v", verb, view.Ref, name, notYet, want)
		}
	}

	offers, err := NextForest(root, "", &Request{Verb: "next", Actor: "alka", Column: aftercare}, 0)
	if err != nil || len(offers.Workbenches) != 2 {
		t.Fatalf("next walked %+v %v, want two workbenches", offers, err)
	}
	for _, member := range offers.Workbenches {
		name := nameOf(member.Path)
		if member.Unanswered != "" || len(member.Offers) != 1 {
			t.Fatalf("next in %s answered %+v", name, member)
		}
		offered := member.Offers[0].Card
		if withheld := offered == nil; withheld != (wantDay[name] == "2026-10-02") {
			t.Errorf("next in %s offered %+v", name, offered)
		}
		if offered != nil {
			judged("next", name, *offered)
		}
	}

	status, err := StatusForest(root, "", &Request{Verb: "status", Actor: "alka"}, 0)
	if err != nil || len(status.Workbenches) != 2 {
		t.Fatalf("status walked %+v %v, want two workbenches", status, err)
	}
	for _, member := range status.Workbenches {
		name := nameOf(member.Path)
		if member.Unanswered != "" || member.Status == nil || len(member.Status.Holding) != 1 {
			t.Fatalf("status in %s answered %+v", name, member)
		}
		judged("status", name, member.Status.Holding[0])
	}

	listing, err := ListForest(root, "", &Request{Verb: "list", Actor: "alka", Ref: aftercare}, 0)
	if err != nil || len(listing.Workbenches) != 2 {
		t.Fatalf("list walked %+v %v, want two workbenches", listing, err)
	}
	for _, member := range listing.Workbenches {
		name := nameOf(member.Path)
		if member.Unanswered != "" || member.Listing == nil || len(member.Listing.Cards) != 2 {
			t.Fatalf("list in %s answered %+v", name, member)
		}
		for _, view := range member.Listing.Cards {
			judged("list", name, view)
		}
	}
}
