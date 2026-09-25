package verb

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The columns of the agenda fixture, by identifier.
const (
	agendaIntake   = "d00000000001"
	agendaQueue    = "d00000000002"
	agendaSkipped  = "d00000000003"
	agendaStation  = "d00000000004"
	agendaReview   = "d00000000005"
	agendaFinished = "d00000000006"
)

// agendaDefinition is a flow with every shape the actionable set has to agree
// with next about: an intake column whose head is offered by pull, a buffer
// a pull looks through, two work stations, an operator-owned review station
// and a done column. The short route drops the first station, so a card
// standing in the buffer on that route lands somewhere other than a card on
// the full flow does.
const agendaDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Agenda",
  "routes": { "short": ["d00000000001", "d00000000002", "d00000000004", "d00000000005", "d00000000006"] },
  "columns": [
    { "id": "d00000000001", "title": "Intake", "kind": "intake" },
    { "id": "d00000000002", "title": "Queue", "kind": "dinah.buffer" },
    { "id": "d00000000003", "title": "Skipped", "kind": "work" },
    { "id": "d00000000004", "title": "Station", "kind": "work" },
    { "id": "d00000000005", "title": "Review", "kind": "work", "operator_owned": true },
    { "id": "d00000000006", "title": "Finished", "kind": "done" }
  ]
}`

// The agents the tests draw as. The operator is alka, whom every harness
// instantiates as the operator.
const (
	workhorse = "brin"
	frontier  = "cato"
)

// agendaHarness builds the agenda fixture with a tier table of two rungs and
// all three level sets declared.
func agendaHarness(t *testing.T) *harness {
	t.Helper()
	h := harnessFromDefinition(t, "ag", agendaDefinition)
	h.writeAnchorBlock("levels", "levels:\n  tier: [workhorse, frontier]\n  priority: [later, soon, next, now]\n  severity: [minor, major, critical, blocker]\n")
	h.writeAnchorBlock("tiers", "tiers:\n  workhorse:\n    meaning: scoped implementation\n    models:\n      - {provider: acme, model: workhorse}\n"+
		"  frontier:\n    meaning: novel design\n    models:\n      - {provider: acme, model: frontier}\n")
	return h
}

// writeAnchorBlock writes one block of the workbench anchor's frontmatter,
// or removes the key where the block is empty, and reopens the bench.
func (h *harness) writeAnchorBlock(key, block string) {
	h.t.Helper()
	path := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		h.t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	if block == "" {
		fm.Delete(key)
	} else {
		fm.SetRaw(key, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	}
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		h.t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()
}

// declareUrgency writes a dinah.urgency block, given without its key line.
func (h *harness) declareUrgency(members string) {
	h.t.Helper()
	h.writeAnchorBlock(bench.UrgencyKey, bench.UrgencyKey+":\n"+members)
}

// caller is who a draw is asked as: an actor and the model it declares.
type caller struct {
	actor string
	model string
}

// The three callers the tests draw as.
var (
	asOperator  = caller{actor: "alka"}
	asWorkhorse = caller{actor: workhorse, model: "workhorse"}
	asFrontier  = caller{actor: frontier, model: "frontier"}
)

// request is the request a caller draws a view with.
func (c caller) request(view string) *Request {
	req := &Request{Verb: "view", Actor: c.actor, View: view, Explain: true}
	if c.model != "" {
		req.Provider, req.Model = "acme", c.model
	}
	return req
}

// explained draws a view as a caller with --explain and fails unless it drew.
func (h *harness) explained(view string, who caller) *ViewAnswer {
	h.t.Helper()
	answer, err := h.library.DrawView(who.request(view))
	if err != nil {
		h.t.Fatalf("view %s as %s: %v", view, who.actor, err)
	}
	return answer
}

// ranked is the references one section of a drawn view carries, in rank
// order.
func ranked(answer *ViewAnswer, section int) []string {
	return sectionRefs(answer, section)
}

// scoreOf is one card's urgency answer in one section, failing where the
// section does not rank it.
func scoreOf(t *testing.T, answer *ViewAnswer, section int, ref string) UrgencyAnswer {
	t.Helper()
	got, ok := answer.View.Sections[section].Urgency[ref]
	if !ok {
		t.Fatalf("section %d of %s ranks no %s: %v", section+1, answer.View.Name, ref, ranked(answer, section))
	}
	return got
}

// termOf is one term of an explained urgency answer.
func termOf(t *testing.T, answer UrgencyAnswer, term string) UrgencyTerm {
	t.Helper()
	for _, found := range answer.Terms {
		if found.Term == term {
			return found
		}
	}
	t.Fatalf("the answer carries no %s term: %+v", term, answer)
	return UrgencyTerm{}
}

// wantPoints fails unless one term of one card carries the points given.
func wantPoints(t *testing.T, answer *ViewAnswer, ref, term, points string) {
	t.Helper()
	got := termOf(t, scoreOf(t, answer, 0, ref), term)
	if got.Points.String() != points {
		t.Errorf("%s scores %s on %s, want %s (basis %v)", ref, got.Points, term, points, got.Basis)
	}
}

// refSet is a list of references as a sorted, space-joined string, which is
// how two sets are compared in a failure message a person can read.
func refSet(refs []string) string {
	sorted := append([]string(nil), refs...)
	sort.Strings(sorted)
	return strings.Join(sorted, " ")
}

// rankedView declares a view named everything, ordered by urgency, selecting
// every live card, so a term can be read on any card the operator's agenda
// would not hold.
func (h *harness) rankedView() {
	h.t.Helper()
	h.declareViews(bench.ViewsKey + ":\n  everything:\n    order: urgency\n    sections:\n      - query: state:ready,active,blocked\n")
}

// placeAt files a card and moves it to a column as the operator.
func (h *harness) placeAt(title, column string) string {
	h.t.Helper()
	ref := h.add(title)
	if column != agendaIntake {
		h.at(ref, column)
	}
	return ref
}

// setTier gives a card a tier requirement.
func (h *harness) setTier(ref, tier string) {
	h.t.Helper()
	h.mustSet(ref, bench.TierField, tier)
}

// nextRefs is every card next offers a caller, by reference.
func (h *harness) nextRefs(who caller) []string {
	h.t.Helper()
	offers, err := h.library.Next(who.request(""))
	if err != nil {
		h.t.Fatalf("next as %s: %v", who.actor, err)
	}
	var refs []string
	for _, offer := range offers {
		if offer.Card != nil {
			refs = append(refs, offer.Card.Ref)
		}
	}
	return refs
}

// primeRefs is every card prime offers a caller as ready, by reference.
func (h *harness) primeRefs(who caller) []string {
	h.t.Helper()
	primer, err := h.library.Prime(who.request(""))
	if err != nil {
		h.t.Fatalf("prime as %s: %v", who.actor, err)
	}
	var refs []string
	for _, offer := range primer.Ready {
		if offer.Card != nil {
			refs = append(refs, offer.Card.Ref)
		}
	}
	return refs
}

// TestTheAgendaHoldsWhatNextAndPrimeOffer is dinah-602/criteria/2, the
// behavioural half of the protection criteria/28's source guard cannot give.
// The fixture offers a head by pull out of intake, withholds a station's
// only card above the caller's tier, carries a card on the short route
// through the buffer to a landing the full flow would not give it, and
// stands a second ready card behind a station's head. The agenda, next and
// prime must name one set for the one caller.
func TestTheAgendaHoldsWhatNextAndPrimeOffer(t *testing.T) {
	h := agendaHarness(t)
	pulled := h.placeAt("offered by pull out of intake", agendaIntake)
	withheld := h.placeAt("above the workhorse tier", agendaSkipped)
	h.setTier(withheld, "frontier")
	routed := h.placeAt("walking the short road", agendaQueue)
	h.onRoute(routed, "short")
	head := h.placeAt("the station's head", agendaStation)
	behind := h.placeAt("behind the station's head", agendaStation)
	reviewed := h.placeAt("standing at review", agendaReview)

	agenda := ranked(h.explained("agenda", asWorkhorse), 0)
	next := h.nextRefs(asWorkhorse)
	prime := h.primeRefs(asWorkhorse)
	if refSet(agenda) != refSet(next) || refSet(agenda) != refSet(prime) {
		t.Fatalf("the agenda holds [%s], next offers [%s] and prime offers [%s]", refSet(agenda), refSet(next), refSet(prime))
	}
	want := refSet([]string{pulled, routed, head, reviewed})
	if refSet(agenda) != want {
		t.Errorf("the three agree on [%s], want [%s]", refSet(agenda), want)
	}
	for _, absent := range []string{withheld, behind} {
		if strings.Contains(" "+refSet(agenda)+" ", " "+absent+" ") {
			t.Errorf("the agenda holds %s, which next does not offer", absent)
		}
	}
	offers, err := h.library.Next(asWorkhorse.request(""))
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	for _, offer := range offers {
		if offer.Column == agendaQueue && (offer.Card == nil || offer.Landing != "station") {
			t.Errorf("the buffer offers %+v, and the routed card should land at the station", offer)
		}
	}
}

// TestTierExclusionHoldsBetweenCallers is dinah-602/criteria/3.
func TestTierExclusionHoldsBetweenCallers(t *testing.T) {
	h := agendaHarness(t)
	gated := h.placeAt("frontier work carrying the operator's question", agendaStation)
	h.setTier(gated, "frontier")
	h.plantItem(gated, "e00000000001", "open_question", "pending", "operator", 1)

	if got := ranked(h.explained("agenda", asWorkhorse), 0); len(got) != 0 {
		t.Errorf("the workhorse agent's agenda holds %v", got)
	}
	offers, err := h.library.Next(&Request{Verb: "next", Actor: workhorse, Provider: "acme", Model: "workhorse", Column: agendaStation})
	if err != nil || len(offers) != 1 || !offers[0].AboveTier {
		t.Errorf("next for the workhorse agent answered %+v (%v), want above_tier", offers, err)
	}
	if got := ranked(h.explained("agenda", asFrontier), 0); refSet(got) != gated {
		t.Errorf("the frontier agent's agenda holds %v, want %s", got, gated)
	}
	wantPoints(t, h.explained("agenda", asOperator), gated, bench.UrgencyYourQuestion, "4.0")
}

// TestTheOperatorsActionableSetIsTheUnionOfItsArms is dinah-602/criteria/4,
// with the fourth arm the operator ruled on at dinah-602/questions/2 (option
// A): the ready heads next offers him are in his agenda too.
func TestTheOperatorsActionableSetIsTheUnionOfItsArms(t *testing.T) {
	h := agendaHarness(t)
	head := h.placeAt("the station's head, offered by next", agendaStation)
	asked := h.placeAt("behind the head, carrying his question", agendaStation)
	h.plantItem(asked, "e00000000001", "open_question", "pending", "operator", 1)
	unowned := h.placeAt("behind the head, carrying a question nobody owns", agendaStation)
	h.plantItem(unowned, "e00000000002", "decision", "pending", "", 1)
	answered := h.placeAt("behind the head, his question answered", agendaStation)
	h.plantItem(answered, "e00000000003", "open_question", "resolved", "operator", 1)
	criterion := h.placeAt("behind the head, carrying only a criterion", agendaStation)
	h.plantItem(criterion, "e00000000004", "acceptance_criterion", "pending", "operator", 1)
	blocked := h.placeAt("blocked with no items", agendaSkipped)
	h.mustDo(&Request{Verb: Block, Card: blocked, Actor: "alka", Reason: "waiting on a supplier", Kind: "supplier"})
	first := h.placeAt("at his station", agendaReview)
	second := h.placeAt("also at his station", agendaReview)

	got := ranked(h.explained("agenda", asOperator), 0)
	want := refSet([]string{head, asked, unowned, blocked, first, second})
	if refSet(got) != want {
		t.Errorf("the operator's agenda holds [%s], want [%s]", refSet(got), want)
	}
	for _, absent := range []string{answered, criterion} {
		if strings.Contains(" "+refSet(got)+" ", " "+absent+" ") {
			t.Errorf("the operator's agenda holds %s", absent)
		}
	}
}

// TestWaitsOnYouAppliesOnlyToTheOperatorAtHisColumn is dinah-602/criteria/5.
func TestWaitsOnYouAppliesOnlyToTheOperatorAtHisColumn(t *testing.T) {
	h := agendaHarness(t)
	owned := h.placeAt("at his station", agendaReview)
	h.placeAt("the station's head", agendaStation)
	named := h.placeAt("at a station he does not own, naming it", agendaStation)
	h.item(named, "e00000000001", "kind: open_question\nstate: pending\nowner: operator\ncolumn: "+agendaStation+"\nordinal: 1\n", "his question for this station")

	operator := h.explained("agenda", asOperator)
	wantPoints(t, operator, owned, bench.UrgencyWaitsOnYou, "10.0")
	wantPoints(t, operator, named, bench.UrgencyWaitsOnYou, "0.0")

	agent := h.explained("agenda", asWorkhorse)
	checked := 0
	for ref, answer := range agent.View.Sections[0].Urgency {
		checked++
		if got := termOf(t, answer, bench.UrgencyWaitsOnYou).Points.String(); got != "0.0" {
			t.Errorf("%s scores %s on waits-on-you in an agent's agenda", ref, got)
		}
	}
	if checked < 2 {
		t.Fatalf("the agent's agenda ranked %d cards, and the review station's head has to be among them", checked)
	}
}

// TestYourQuestionCountsOnlyTheCallersItemsCapped is dinah-602/criteria/6.
func TestYourQuestionCountsOnlyTheCallersItemsCapped(t *testing.T) {
	h := agendaHarness(t)
	many := h.placeAt("five of his questions", agendaStation)
	for i, id := range []string{"e00000000001", "e00000000002", "e00000000003", "e00000000004", "e00000000005"} {
		h.plantItem(many, id, "open_question", "pending", "operator", i+1)
	}
	h.plantItem(many, "e00000000006", "acceptance_criterion", "pending", "operator", 1)
	h.plantItem(many, "e00000000007", "open_question", "resolved", "operator", 6)

	term := termOf(t, scoreOf(t, h.explained("agenda", asOperator), 0, many), bench.UrgencyYourQuestion)
	if term.Points.String() != "12.0" || term.Basis["owned"] != "5" || term.Basis["counted"] != "3" {
		t.Errorf("five of his questions score %s with basis %v, want 12.0, owned 5, counted 3", term.Points, term.Basis)
	}

	h2 := agendaHarness(t)
	mixed := h2.placeAt("the agent's head, one question each way", agendaStation)
	h2.plantItem(mixed, "e00000000001", "open_question", "pending", "holder", 1)
	h2.plantItem(mixed, "e00000000002", "open_question", "pending", "operator", 2)
	term = termOf(t, scoreOf(t, h2.explained("agenda", asWorkhorse), 0, mixed), bench.UrgencyYourQuestion)
	if term.Points.String() != "4.0" || term.Basis["owned"] != "1" {
		t.Errorf("the agent's head scores %s with basis %v, want 4.0 and owned 1", term.Points, term.Basis)
	}
}

// TestACardWithNoPriorityIsNotTheLowest is dinah-602/criteria/7.
func TestACardWithNoPriorityIsNotTheLowest(t *testing.T) {
	h := agendaHarness(t)
	h.declareUrgency("  priority: [1, 2, 3, 4]\n")
	none := h.placeAt("no priority", agendaStation)
	later := h.placeAt("later", agendaStation)
	h.mustSet(later, bench.PriorityField, "later")
	odd := h.placeAt("an undeclared priority", agendaStation)
	h.mustSet(odd, bench.PriorityField, "later")
	h.rankedView()
	h.levelByHand(odd, "urgent")

	answer := h.explained("everything", asOperator)
	if term := termOf(t, scoreOf(t, answer, 0, none), bench.UrgencyPriority); term.Points.String() != "0.0" || term.Basis["level"] != "" {
		t.Errorf("no priority scores %s with basis %v", term.Points, term.Basis)
	}
	wantPoints(t, answer, later, bench.UrgencyPriority, "1.0")
	if term := termOf(t, scoreOf(t, answer, 0, odd), bench.UrgencyPriority); term.Points.String() != "0.0" || term.Basis["declared"] != "false" {
		t.Errorf("an undeclared priority scores %s with basis %v", term.Points, term.Basis)
	}

	h.writeAnchorBlock("levels", "levels:\n  tier: [workhorse, frontier]\n")
	answer = h.explained("everything", asOperator)
	for _, ref := range []string{none, later, odd} {
		wantPoints(t, answer, ref, bench.UrgencyPriority, "0.0")
	}
}

// levelByHand writes a priority straight into a card's anchor, which is how a
// test plants a name the workbench does not declare.
func (h *harness) levelByHand(ref, priority string) {
	h.t.Helper()
	card := h.card(ref)
	card.Priority = priority
	if err := card.Save(); err != nil {
		h.t.Fatalf("save %s: %v", ref, err)
	}
	h.reopen()
}

// TestLevelWeightsAlignAtTheTop is dinah-602/criteria/8.
func TestLevelWeightsAlignAtTheTop(t *testing.T) {
	sets := []struct {
		levels string
		names  []string
		want   []string
	}{
		{levels: "[low, mid, high]", names: []string{"low", "mid", "high"}, want: []string{"2.0", "4.0", "6.0"}},
		{levels: "[p1, p2, p3, p4, p5]", names: []string{"p1", "p2", "p3", "p4", "p5"}, want: []string{"0.0", "0.0", "2.0", "4.0", "6.0"}},
	}
	for _, set := range sets {
		h := agendaHarness(t)
		h.writeAnchorBlock("levels", "levels:\n  tier: [workhorse, frontier]\n  priority: "+set.levels+"\n")
		var refs []string
		for _, name := range set.names {
			ref := h.placeAt("at "+name, agendaStation)
			h.mustSet(ref, bench.PriorityField, name)
			refs = append(refs, ref)
		}
		h.rankedView()
		answer := h.explained("everything", asOperator)
		for i, ref := range refs {
			wantPoints(t, answer, ref, bench.UrgencyPriority, set.want[i])
		}
		if _, found := finding(h.check(), bench.FindingUrgencyLevelCount); found {
			t.Errorf("a workbench declaring no dinah.urgency block raised %s", bench.FindingUrgencyLevelCount)
		}
		h.declareUrgency("  priority: [0, 2, 4, 6]\n")
		detail, found := finding(h.check(), bench.FindingUrgencyLevelCount)
		wantDetail := "priority 4 " + map[int]string{3: "3", 5: "5"}[len(set.names)]
		if !found || detail != wantDetail {
			t.Errorf("a four-weight list over %d levels raised %q (%v), want %q", len(set.names), detail, found, wantDetail)
		}
	}
}

// TestABlockOverridingOneTermChangesThatTermOnly is dinah-602/criteria/9.
func TestABlockOverridingOneTermChangesThatTermOnly(t *testing.T) {
	h := agendaHarness(t)
	now := h.placeAt("at priority now", agendaReview)
	h.mustSet(now, bench.PriorityField, "now")
	h.mustSet(now, bench.SeverityField, "major")
	h.plantItem(now, "e00000000001", "open_question", "pending", "operator", 1)
	h.advance(36 * time.Hour)
	before := scoreOf(t, h.explained("agenda", asOperator), 0, now)
	h.declareUrgency("  priority: [0, 0, 0, 10]\n")
	after := scoreOf(t, h.explained("agenda", asOperator), 0, now)
	for i, term := range after.Terms {
		want := before.Terms[i].Points.String()
		if term.Term == bench.UrgencyPriority {
			want = "10.0"
		}
		if term.Points.String() != want {
			t.Errorf("%s scores %s under the override, want %s", term.Term, term.Points, want)
		}
	}

	h2 := agendaHarness(t)
	old := h2.placeAt("ten days in its column", agendaStation)
	h2.advance(10 * 24 * time.Hour)
	h2.declareUrgency("  age:\n    per-day: 1\n")
	h2.rankedView()
	wantPoints(t, h2.explained("everything", asOperator), old, bench.UrgencyAge, "5.0")
}

// TestAMalformedBlockRefusesRankedViewsAndNothingElse is dinah-602/criteria/10
// at the library, over the blocks the criterion names. Each block refuses the
// agenda naming its defect and term, leaves mine drawable, and is reported
// by check; a misspelt member is ignored and reported as unknown.
func TestAMalformedBlockRefusesRankedViewsAndNothingElse(t *testing.T) {
	cases := []struct {
		name    string
		members string
		raw     string
		defect  string
		term    string
	}{
		{name: "a weight of 0.25", members: "  blocked: 0.25\n", defect: bench.UrgencyMalformedTerm, term: "blocked"},
		{name: "a quoted 2", members: "  blocked: \"2\"\n", defect: bench.UrgencyMalformedTerm, term: "blocked"},
		{name: "an exponent", members: "  blocked: 1e1\n", defect: bench.UrgencyMalformedTerm, term: "blocked"},
		{name: "1001", members: "  blocked: 1001\n", defect: bench.UrgencyMalformedTerm, term: "blocked"},
		{name: "an empty list", members: "  priority: []\n", defect: bench.UrgencyMalformedTerm, term: "priority"},
		{name: "a cap of 1.5", members: "  age:\n    cap: 1.5\n", defect: bench.UrgencyMalformedTerm, term: "age.cap"},
		{name: "not a mapping", raw: bench.UrgencyKey + ": flat\n", defect: bench.UrgencyNotAMapping},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := agendaHarness(t)
			if c.raw != "" {
				h.writeAnchorBlock(bench.UrgencyKey, c.raw)
			} else {
				h.declareUrgency(c.members)
			}
			_, err := h.library.DrawView(asOperator.request("agenda"))
			refusal, ok := err.(*contract.Refusal)
			if !ok || refusal.Name != contract.MalformedUrgency || refusal.Extra["defect"] != c.defect || refusal.Extra["term"] != c.term {
				t.Fatalf("the agenda answered %v, want %s with defect %s and term %q", err, contract.MalformedUrgency, c.defect, c.term)
			}
			if _, err := h.library.DrawView(&Request{Verb: "view", Actor: "alka", View: "mine"}); err != nil {
				t.Errorf("mine refused: %v", err)
			}
			detail, found := finding(h.check(), bench.FindingUrgencyMalformed)
			wantDetail := c.term + " " + c.defect
			if c.term == "" {
				wantDetail = bench.UrgencyKey + " " + c.defect
			}
			if !found || detail != wantDetail {
				t.Errorf("check reported %q (%v), want %q", detail, found, wantDetail)
			}
		})
	}
	t.Run("a misspelt member", func(t *testing.T) {
		h := agendaHarness(t)
		h.declareUrgency("  age:\n    perday: 1\n")
		if _, err := h.library.DrawView(asOperator.request("agenda")); err != nil {
			t.Fatalf("a misspelt member refused the agenda: %v", err)
		}
		if detail, found := finding(h.check(), bench.FindingUrgencyMemberUnknown); !found || detail != "age.perday" {
			t.Errorf("check reported %q (%v), want age.perday", detail, found)
		}
	})
	t.Run("the accepting case", func(t *testing.T) {
		h := agendaHarness(t)
		h.declareUrgency("  blocked: 0.50\n  priority: [0, 2, 4, 6]\n  age:\n    cap: 3\n")
		if _, err := h.library.DrawView(asOperator.request("agenda")); err != nil {
			t.Fatalf("a well-formed block refused the agenda: %v", err)
		}
		if detail, found := finding(h.check(), bench.FindingUrgencyMalformed); found {
			t.Errorf("check reported a well-formed block malformed: %s", detail)
		}
	})
}

// TestAgeCountsWholeDaysCappedAndReadsSkewAsZero is dinah-602/criteria/11.
func TestAgeCountsWholeDaysCappedAndReadsSkewAsZero(t *testing.T) {
	cases := []struct {
		name    string
		elapsed time.Duration
		want    string
	}{
		{name: "23h59m", elapsed: 23*time.Hour + 59*time.Minute, want: "0.0"},
		{name: "47h", elapsed: 47 * time.Hour, want: "0.5"},
		{name: "ten days", elapsed: 10 * 24 * time.Hour, want: "2.5"},
		{name: "arrival after now", elapsed: -time.Hour, want: "0.0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := agendaHarness(t)
			ref := h.placeAt("aging", agendaStation)
			h.advance(c.elapsed)
			h.rankedView()
			wantPoints(t, h.explained("everything", asOperator), ref, bench.UrgencyAge, c.want)
		})
	}
	t.Run("an unreadable journal", func(t *testing.T) {
		h := agendaHarness(t)
		ref := h.placeAt("its journal will not read", agendaStation)
		h.advance(10 * 24 * time.Hour)
		journal := h.card(ref).JournalPath()
		if err := bench.WriteText(journal, "{not json\n{not json either\n"); err != nil {
			t.Fatalf("spoil the journal: %v", err)
		}
		h.rankedView()
		term := termOf(t, scoreOf(t, h.explained("everything", asOperator), 0, ref), bench.UrgencyAge)
		if term.Points.String() != "0.0" || term.Basis["arrival"] != "" {
			t.Errorf("an unreadable journal scores %s with basis %v, want 0.0 and no arrival", term.Points, term.Basis)
		}
	})
}

// TestAClaimAtItsExpiryReadsAsStale is dinah-602/criteria/12, on both of its
// fixtures.
func TestAClaimAtItsExpiryReadsAsStale(t *testing.T) {
	t.Run("an agent at a station", func(t *testing.T) {
		h := agendaHarness(t)
		ref := h.placeAt("claimed and left", agendaStation)
		h.mustDo(&Request{Verb: Claim, Actor: frontier, Card: ref, Expires: time.Hour})
		expiry := h.clock.Add(time.Hour)
		h.clock = expiry.Add(-time.Nanosecond)
		if got := ranked(h.explained("agenda", asWorkhorse), 0); len(got) != 0 {
			t.Errorf("one nanosecond before expiry the agent's agenda holds %v", got)
		}
		h.clock = expiry
		answer := h.explained("agenda", asWorkhorse)
		term := termOf(t, scoreOf(t, answer, 0, ref), bench.UrgencyStaleClaim)
		if term.Points.String() != "3.0" || term.Basis["expired"] != bench.Stamp(expiry) || term.Basis["holder"] != frontier {
			t.Errorf("at expiry the claim scores %s with basis %v", term.Points, term.Basis)
		}
		h.reopen()
		h.mustDo(&Request{Verb: Claim, Actor: workhorse, Card: ref})
		h.mustDo(&Request{Verb: Release, Actor: workhorse, Card: ref})
		wantPoints(t, h.explained("agenda", asWorkhorse), ref, bench.UrgencyStaleClaim, "0.0")
	})
	t.Run("the operator at his station", func(t *testing.T) {
		h := agendaHarness(t)
		ref := h.placeAt("his, claimed and left", agendaReview)
		h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: ref, Expires: time.Hour})
		expiry := h.clock.Add(time.Hour)
		h.clock = expiry.Add(-time.Nanosecond)
		wantPoints(t, h.explained("agenda", asOperator), ref, bench.UrgencyStaleClaim, "0.0")
		h.clock = expiry
		wantPoints(t, h.explained("agenda", asOperator), ref, bench.UrgencyStaleClaim, "3.0")
		h.reopen()
		h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: ref})
		h.mustDo(&Request{Verb: Release, Actor: "alka", Card: ref})
		wantPoints(t, h.explained("agenda", asOperator), ref, bench.UrgencyStaleClaim, "0.0")
	})
}

// TestBlockedAndBlocksOthersBehaveAsSpecified is dinah-602/criteria/13.
func TestBlockedAndBlocksOthersBehaveAsSpecified(t *testing.T) {
	h := agendaHarness(t)
	blocked := h.placeAt("blocked on a supplier", agendaStation)
	h.mustDo(&Request{Verb: Block, Card: blocked, Actor: "alka", Reason: "the supplier went quiet", Kind: "supplier"})
	term := termOf(t, scoreOf(t, h.explained("agenda", asOperator), 0, blocked), bench.UrgencyBlocked)
	if term.Points.String() != "3.0" || term.Basis["kind"] != "supplier" {
		t.Errorf("a blocked card scores %s with basis %v", term.Points, term.Basis)
	}

	blocker := h.placeAt("blocks three", agendaReview)
	for _, title := range []string{"first blocked", "second blocked", "third blocked"} {
		mustLink(t, h, blocker, "blocks", h.placeAt(title, agendaStation))
	}
	for _, weights := range []string{"", "  blocks-others: 5\n"} {
		if weights != "" {
			h.declareUrgency(weights)
		}
		term := termOf(t, scoreOf(t, h.explained("agenda", asOperator), 0, blocker), bench.UrgencyBlocksOthers)
		if term.Points.String() != "0.0" || term.Basis["read"] != "false" {
			t.Errorf("under %q a card blocking three scores %s with basis %v", weights, term.Points, term.Basis)
		}
	}
}

// TestTiesBreakOnArrivalThenCardNumber is dinah-602/criteria/14.
func TestTiesBreakOnArrivalThenCardNumber(t *testing.T) {
	h := agendaHarness(t)
	h.declareUrgency("  priority: [0.1]\n")
	lower := h.add("the lower number, arriving with its twin")
	higher := h.add("the higher number, arriving with its twin")
	late := h.add("the lowest of the late pair by number, arriving later")
	early := h.add("the highest of the late pair by number, arriving earlier")
	tenth := h.add("one tenth higher, arriving last")
	h.mustSet(tenth, bench.PriorityField, "later")
	h.at(lower, agendaReview)
	h.at(higher, agendaReview)
	h.advance(time.Hour)
	h.at(early, agendaReview)
	h.advance(time.Hour)
	h.at(late, agendaReview)
	h.advance(time.Hour)
	h.at(tenth, agendaReview)
	h.advance(time.Hour)

	got := ranked(h.explained("agenda", asOperator), 0)
	want := []string{tenth, lower, higher, early, late}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the agenda ranks %v, want %v", got, want)
	}
}

// TestUrgencyIsLegalOnAnyViewAndRanksEachSection is dinah-602/criteria/17.
func TestUrgencyIsLegalOnAnyViewAndRanksEachSection(t *testing.T) {
	h := agendaHarness(t)
	low := h.placeAt("low", agendaStation)
	high := h.placeAt("high", agendaStation)
	h.mustSet(high, bench.PriorityField, "now")
	both := h.placeAt("in both sections", agendaReview)
	h.declareViews(bench.ViewsKey + ":\n  two:\n    order: urgency\n    sections:\n      - query: column:station\n      - query: column:review,station\n" +
		"  scoped:\n    order: arrival\n    sections:\n      - scope: actionable\n")
	answer := h.explained("two", asOperator)
	for i, section := range answer.View.Sections {
		previous := ""
		for rank, card := range section.Cards {
			got := section.Urgency[card.Ref]
			if got.Rank != rank+1 {
				t.Errorf("section %d ranks %s at %d, in position %d", i+1, card.Ref, got.Rank, rank+1)
			}
			if previous != "" && strings.Compare(padScore(got.Score.String()), padScore(previous)) > 0 {
				t.Errorf("section %d ranks %s above a lower score", i+1, card.Ref)
			}
			previous = got.Score.String()
		}
	}
	wantSection(t, answer, 0, high, low)
	if scoreOf(t, answer, 0, high).Score != scoreOf(t, answer, 1, high).Score {
		t.Errorf("a card in both sections scores differently in each")
	}
	if scoreOf(t, answer, 1, both).Rank != 1 {
		t.Errorf("the operator's own card does not lead the second section")
	}
	for _, view := range []string{"two", "scoped"} {
		refusal := h.refuseDraw(view, "")
		if refusal.Name != contract.NoOwner {
			t.Errorf("%s with no actor was refused %s, want %s", view, refusal.Name, contract.NoOwner)
		}
	}
}

// padScore right-aligns a score so two scores compare as text in the order
// they compare as numbers, for the non-negative scores this test draws.
func padScore(score string) string {
	for len(score) < 8 {
		score = " " + score
	}
	return score
}

// TestTheScopeMemberSelectsAndComposes is dinah-602/criteria/18.
func TestTheScopeMemberSelectsAndComposes(t *testing.T) {
	h := agendaHarness(t)
	stream := h.newWorkstream("Stream")
	in := h.placeAt("actionable and in the stream", agendaStation)
	outside := h.placeAt("in the stream and behind the head", agendaStation)
	elsewhere := h.placeAt("actionable and not in the stream", agendaSkipped)
	for _, ref := range []string{in, outside} {
		h.mustDo(&Request{Verb: "join", Actor: "alka", Card: ref, Workstream: stream.Ref})
	}
	h.declareViews(bench.ViewsKey + ":\n  mixed:\n    sections:\n      - scope: actionable\n        query: \"workstream:" + stream.Slug + "\"\n      - scope: actionable\n" +
		"  bogus:\n    sections:\n      - scope: bogus\n" +
		"  neither:\n    sections:\n      - title: nothing to select\n")
	req := asWorkhorse.request("mixed")
	req.Explain = false
	answer, err := h.library.DrawView(req)
	if err != nil {
		t.Fatalf("mixed: %v", err)
	}
	wantSection(t, answer, 0, in)
	if refSet(ranked(answer, 1)) != refSet([]string{in, elsewhere}) {
		t.Errorf("the scope alone selects %v", ranked(answer, 1))
	}
	if heading := answer.View.Sections[1].Title; heading != bench.ViewScopeActionable {
		t.Errorf("a scope-only section with no title is headed %q", heading)
	}
	for i, section := range answer.View.Sections {
		if section.Scope != bench.ViewScopeActionable {
			t.Errorf("section %d carries scope %q", i+1, section.Scope)
		}
	}
	if refusal := h.refuseDraw("bogus", "alka"); refusal.Extra["defect"] != bench.ViewUnknownScope {
		t.Errorf("scope: bogus was answered %s %v", refusal.Name, refusal.Extra)
	}
	if refusal := h.refuseDraw("neither", "alka"); refusal.Extra["defect"] != bench.ViewSectionWithoutQuery {
		t.Errorf("a section with neither was answered %s %v", refusal.Name, refusal.Extra)
	}
}

// TestTheCardPositionalNarrowsAndKeepsTheRank is dinah-602/criteria/19.
func TestTheCardPositionalNarrowsAndKeepsTheRank(t *testing.T) {
	h := agendaHarness(t)
	top := h.placeAt("first", agendaReview)
	h.mustSet(top, bench.PriorityField, "now")
	second := h.placeAt("second", agendaReview)
	h.placeAt("intake's head, which next offers him", agendaIntake)
	behind := h.placeAt("behind intake's head, which no arm selects", agendaIntake)

	req := asOperator.request("agenda")
	req.Card = second
	answer, err := h.library.DrawView(req)
	if err != nil {
		t.Fatalf("narrowed agenda: %v", err)
	}
	wantSection(t, answer, 0, second)
	if got := scoreOf(t, answer, 0, second).Rank; got != 2 {
		t.Errorf("the narrowed row carries rank %d, want the 2 it holds in the full section", got)
	}
	req.Card = behind
	if _, err := h.library.DrawView(req); err == nil {
		t.Fatalf("a card no section selects drew")
	} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.CardNotInView || refusal.Extra["view"] != "agenda" {
		t.Errorf("a card no section selects was answered %v", err)
	}
	req.Card = "ag-99"
	if _, err := h.library.DrawView(req); err == nil {
		t.Fatalf("a reference to no card drew")
	} else if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnknownCard {
		t.Errorf("a reference to no card was answered %v", err)
	}
}

// TestExplainRefusesAViewThatRanksNothing is the library half of
// dinah-602/criteria/20: --explain on a view ordered by arrival or column
// refuses, and a draw without it carries no terms.
func TestExplainRefusesAViewThatRanksNothing(t *testing.T) {
	h := agendaHarness(t)
	ref := h.placeAt("at his station", agendaReview)
	h.declareViews(bench.ViewsKey + ":\n  by-arrival:\n    sections:\n      - query: state:ready\n  by-column:\n    order: column\n    sections:\n      - query: state:ready\n")
	for _, view := range []string{"by-arrival", "by-column"} {
		_, err := h.library.DrawView(asOperator.request(view))
		if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.ViewNotRanked || refusal.Extra["view"] != view {
			t.Errorf("--explain on %s answered %v", view, err)
		}
	}
	explained := h.explained("agenda", asOperator)
	if !explained.View.Explained || len(scoreOf(t, explained, 0, ref).Terms) != 8 {
		t.Errorf("an explained draw carries explained %v and %d terms", explained.View.Explained, len(scoreOf(t, explained, 0, ref).Terms))
	}
	plain := asOperator.request("agenda")
	plain.Explain = false
	answer, err := h.library.DrawView(plain)
	if err != nil {
		t.Fatalf("agenda: %v", err)
	}
	if answer.View.Explained || len(scoreOf(t, answer, 0, ref).Terms) != 0 {
		t.Errorf("a plain draw carries explained %v and terms %v", answer.View.Explained, scoreOf(t, answer, 0, ref).Terms)
	}
}

// TestTheWhyTermsAreExactlyTheNonZeroOnes is dinah-602/criteria/21 at the
// library: the terms the Why cell reads are the non-zero ones in term order,
// a negatively weighted term included.
func TestTheWhyTermsAreExactlyTheNonZeroOnes(t *testing.T) {
	h := agendaHarness(t)
	h.declareUrgency("  blocked: -3\n")
	ref := h.placeAt("blocked at his station", agendaReview)
	h.mustSet(ref, bench.PriorityField, "next")
	h.mustDo(&Request{Verb: Block, Card: ref, Actor: "alka", Reason: "stopped"})
	answer := scoreOf(t, h.explained("agenda", asOperator), 0, ref)
	var want []string
	for _, term := range answer.Terms {
		if term.Points.String() != "0.0" {
			want = append(want, term.Term)
		}
	}
	var got []string
	for _, term := range answer.Contributing {
		got = append(got, term.Term)
	}
	if strings.Join(got, " ") != strings.Join(want, " ") || !strings.Contains(strings.Join(got, " "), bench.UrgencyBlocked) {
		t.Errorf("the Why terms are %v, want %v with the negative blocked term among them", got, want)
	}
}

// TestWeightsComeFromTheWorkbenchAlone is dinah-602/criteria/22.
func TestWeightsComeFromTheWorkbenchAlone(t *testing.T) {
	h := agendaHarness(t)
	ref := h.placeAt("at his station", agendaReview)
	before := scoreOf(t, h.explained("agenda", asOperator), 0, ref).Score
	if err := bench.WriteText(bench.UserViewsPath(h.home), "---\n"+bench.UrgencyKey+":\n  waits-on-you: 100\n---\n"); err != nil {
		t.Fatalf("write the user's config: %v", err)
	}
	after := scoreOf(t, h.explained("agenda", asOperator), 0, ref).Score
	if before != after || before.String() != "10.0" {
		t.Errorf("the score read %s before the user's block and %s after, want 10.0 both times", before, after)
	}
}

// TestAnEmptyBlockMeansTheDefaults is dinah-602/criteria/26 at the library:
// the key alone and the key followed only by comments declare nothing.
func TestAnEmptyBlockMeansTheDefaults(t *testing.T) {
	for _, raw := range []string{bench.UrgencyKey + ":\n", bench.UrgencyKey + ":\n  # every weight switched off\n  # for now\n"} {
		h := agendaHarness(t)
		ref := h.placeAt("at his station", agendaReview)
		h.writeAnchorBlock(bench.UrgencyKey, raw)
		wantPoints(t, h.explained("agenda", asOperator), ref, bench.UrgencyWaitsOnYou, "10.0")
		for _, key := range []string{bench.FindingUrgencyMalformed, bench.FindingUrgencyMemberUnknown, bench.FindingUrgencyLevelCount} {
			if detail, found := finding(h.check(), key); found {
				t.Errorf("%q raised %s %s", raw, key, detail)
			}
		}
	}
}

// TestADoneColumnNeverEarnsWaitsOnYou is dinah-602/criteria/27.
func TestADoneColumnNeverEarnsWaitsOnYou(t *testing.T) {
	h := agendaHarness(t)
	h.declare(agendaFinished, "operator_owned", "true")
	finished := h.placeAt("finished with nothing owed", agendaFinished)
	owed := h.placeAt("finished with his question still pending", agendaFinished)
	h.plantItem(owed, "e00000000001", "open_question", "pending", "operator", 1)
	station := h.placeAt("at his station", agendaReview)

	answer := h.explained("agenda", asOperator)
	got := ranked(answer, 0)
	if strings.Contains(" "+refSet(got)+" ", " "+finished+" ") {
		t.Errorf("a finished card owing nothing is in the agenda: %v", got)
	}
	wantPoints(t, answer, owed, bench.UrgencyWaitsOnYou, "0.0")
	wantPoints(t, answer, owed, bench.UrgencyYourQuestion, "4.0")
	if strings.Join(got, " ") != station+" "+owed {
		t.Errorf("the agenda ranks %v, want %s above %s", got, station, owed)
	}
}

// TestEachRankedCardsJournalIsReadOncePerDraw is dinah-602/criteria/29. The
// fixture ranks five cards whose scores tie, so the sort compares arrivals
// many times, and a card sits in two sections, so it is ranked twice.
func TestEachRankedCardsJournalIsReadOncePerDraw(t *testing.T) {
	h := agendaHarness(t)
	var refs []string
	for _, title := range []string{"one", "two", "three", "four", "five"} {
		refs = append(refs, h.placeAt(title, agendaReview))
	}
	h.declareViews(bench.ViewsKey + ":\n  twice:\n    order: urgency\n    sections:\n      - query: column:review\n      - query: column:review\n")
	reads := map[string]int{}
	h.library.Observe = func(event, target string) {
		if event == ObserveJournal {
			reads[target]++
		}
	}
	answer := h.explained("twice", asOperator)
	if len(ranked(answer, 0)) != len(refs) || len(ranked(answer, 1)) != len(refs) {
		t.Fatalf("the view ranked %v and %v", ranked(answer, 0), ranked(answer, 1))
	}
	if len(reads) != len(refs) {
		t.Errorf("the draw read %d journals, want %d: %v", len(reads), len(refs), reads)
	}
	for journal, count := range reads {
		if count != 1 {
			t.Errorf("%s was read %d times in one draw", journal, count)
		}
	}
}

// TestTheBuiltInAgendaIsListedAndReplaceable is dinah-602/criteria/1.
func TestTheBuiltInAgendaIsListedAndReplaceable(t *testing.T) {
	h := agendaHarness(t)
	listing, err := h.library.ListViews(&Request{Verb: "view", Actor: "alka"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var rows []string
	for _, row := range listing.Views {
		rows = append(rows, row.Name+"/"+row.Source+"/"+row.Layout+"/"+row.Order)
	}
	if strings.Join(rows, " ") != "agenda/built-in/list/urgency mine/built-in/list/column" {
		t.Errorf("the listing reads %v", rows)
	}
	answer := h.explained("agenda", asOperator)
	if len(answer.View.Sections) != 1 || answer.View.Sections[0].Title != "Cards you can act on" {
		t.Errorf("the built-in agenda draws %+v", answer.View.Sections)
	}
	h.declareViews(bench.ViewsKey + ":\n  agenda:\n    order: urgency\n    sections:\n      - title: The workbench's own\n        scope: actionable\n")
	if title := h.explained("agenda", asOperator).View.Sections[0].Title; title != "The workbench's own" {
		t.Errorf("the workbench's agenda did not replace the built-in: %q", title)
	}
	h.userViews(bench.ViewsKey + ":\n  agenda:\n    order: urgency\n    sections:\n      - title: My own\n        scope: actionable\n")
	if title := h.explained("agenda", asOperator).View.Sections[0].Title; title != "My own" {
		t.Errorf("the user's agenda did not replace the workbench's: %q", title)
	}
}
