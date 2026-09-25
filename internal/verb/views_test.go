package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// declareViews writes a dinah.views block into the workbench anchor, on the
// terms declareLevels writes the levels block.
func (h *harness) declareViews(block string) {
	h.t.Helper()
	path := filepath.Join(h.root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		h.t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.ViewsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		h.t.Fatalf("write the workbench anchor: %v", err)
	}
	h.reopen()
}

// userViews writes the user's config.md carrying a dinah.views block, or
// removes it where the block is empty.
func (h *harness) userViews(block string) {
	h.t.Helper()
	path := bench.UserViewsPath(h.home)
	if block == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			h.t.Fatalf("remove the user's config: %v", err)
		}
		return
	}
	if err := bench.WriteText(path, "---\n"+block+"---\n"); err != nil {
		h.t.Fatalf("write the user's config: %v", err)
	}
}

// draw asks for one view as one actor.
func (h *harness) draw(name, actor string) (*ViewAnswer, error) {
	h.t.Helper()
	return h.library.DrawView(&Request{Verb: "view", Actor: actor, View: name})
}

// mustDraw draws a view and fails the test unless it drew.
func (h *harness) mustDraw(name, actor string) *ViewAnswer {
	h.t.Helper()
	answer, err := h.draw(name, actor)
	if err != nil {
		h.t.Fatalf("view %s as %q: %v", name, actor, err)
	}
	return answer
}

// refuseDraw draws a view expected to refuse and returns the refusal.
func (h *harness) refuseDraw(name, actor string) *contract.Refusal {
	h.t.Helper()
	answer, err := h.draw(name, actor)
	if err == nil {
		h.t.Fatalf("view %s as %q drew %+v, and was expected to refuse", name, actor, answer)
	}
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		h.t.Fatalf("view %s answered %T rather than a refusal: %v", name, err, err)
	}
	return refusal
}

// sectionRefs is the card references one section of a drawn view carries, in
// the order it carries them.
func sectionRefs(answer *ViewAnswer, section int) []string {
	refs := []string{}
	for _, card := range answer.View.Sections[section].Cards {
		refs = append(refs, card.Ref)
	}
	return refs
}

// wantSection fails unless one section of a drawn view carries exactly the
// cards named, in order.
func wantSection(t *testing.T, answer *ViewAnswer, section int, want ...string) {
	t.Helper()
	got := sectionRefs(answer, section)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("view %s section %d drew %v, want %v", answer.View.Name, section+1, got, want)
	}
}

// oneSection is a view declaring one section with the given query.
func oneSection(name, query string) string {
	return "  " + name + ":\n    sections:\n      - query: '" + query + "'\n"
}

// TestMeExpandsOnlyAsAWholeBarePart is dinah-600/criteria/9.
func TestMeExpandsOnlyAsAWholeBarePart(t *testing.T) {
	h := newHarness(t)
	mine := h.ready("held by the caller")
	xs := h.ready("held by x")
	literal := h.ready("held by an owner named @me")
	megs := h.ready("held by @meg")
	comma := h.ready("held by an owner with a comma")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: mine})
	h.mustDo(&Request{Verb: Claim, Actor: "x", Card: xs})
	h.mustDo(&Request{Verb: Claim, Actor: "@me", Card: literal})
	h.mustDo(&Request{Verb: Claim, Actor: "@meg", Card: megs})
	h.mustDo(&Request{Verb: Claim, Actor: "Anne, Marie", Card: comma})
	h.declareViews(bench.ViewsKey + ":\n" +
		oneSection("bare", "holder:@me") +
		oneSection("list", "holder:x,@me") +
		oneSection("quoted", `holder:"@me"`) +
		oneSection("longer", "holder:@meg") +
		oneSection("negated", "holder!=@me state:active"))
	wantSection(t, h.mustDraw("bare", "alka"), 0, mine)
	wantSection(t, h.mustDraw("list", "alka"), 0, mine, xs)
	wantSection(t, h.mustDraw("quoted", "alka"), 0, literal)
	wantSection(t, h.mustDraw("longer", "alka"), 0, megs)
	wantSection(t, h.mustDraw("negated", "alka"), 0, xs, literal, megs, comma)
	wantSection(t, h.mustDraw("bare", "Anne, Marie"), 0, comma)
	if query := h.mustDraw("bare", "alka").View.Sections[0].Query; query != "holder:@me" {
		t.Errorf("the section reports its query as %q, want it as declared", query)
	}
}

// TestAViewNeedingTheCallerRefusesWithoutOne is dinah-600/criteria/10's view
// half: with no actor a section using @me refuses no-owner, carrying the
// harness where one was declared, and a view that never uses @me draws.
func TestAViewNeedingTheCallerRefusesWithoutOne(t *testing.T) {
	h := newHarness(t)
	h.declareViews(bench.ViewsKey + ":\n" + oneSection("needs-me", "holder:@me") + oneSection("anybody", "state:ready"))
	refusal := h.refuseDraw("needs-me", "")
	if refusal.Name != contract.NoOwner {
		t.Errorf("a view using @me with no actor was refused %s, want %s", refusal.Name, contract.NoOwner)
	}
	if refusal.Extra[contract.ValueHarness] != "" {
		t.Errorf("a request declaring no harness carried %q", refusal.Extra[contract.ValueHarness])
	}
	_, err := h.library.DrawView(&Request{Verb: "view", View: "needs-me", Harness: "claude-code"})
	harnessed, _ := err.(*contract.Refusal)
	if harnessed == nil || harnessed.Name != contract.NoOwner || harnessed.Extra[contract.ValueHarness] != "claude-code" {
		t.Errorf("a harnessed request was answered %v, want no-owner carrying the harness", err)
	}
	h.ready("a ready card")
	if answer := h.mustDraw("anybody", ""); answer.View.Sections[0].Count != 1 {
		t.Errorf("a view without @me drew %d cards with no actor, want 1", answer.View.Sections[0].Count)
	}
}

// TestTheMeStepSitsAfterCheckThreeAndBeforeCheckFour is
// dinah-600/criteria/28, the hard position: with no actor, a mistake checks
// 1 to 3 catch is refused for itself, and one check 4 would catch is not
// reached, because the step refuses for the missing caller first.
func TestTheMeStepSitsAfterCheckThreeAndBeforeCheckFour(t *testing.T) {
	h := newHarness(t)
	h.declareViews(bench.ViewsKey + ":\n" +
		oneSection("field", "holder:@me bogus:x") +
		oneSection("parse", "holder:@me x") +
		oneSection("value", "holder:@me state:reday"))
	for _, c := range []struct {
		view, refusal, detail string
	}{
		{"field", contract.UnknownField, "bogus"},
		{"parse", contract.Malformed, "x"},
		{"value", contract.NoOwner, ""},
	} {
		refusal := h.refuseDraw(c.view, "")
		if refusal.Name != c.refusal || refusal.Detail != c.detail {
			t.Errorf("%s was refused %s %q, want %s %q", c.view, refusal.Name, refusal.Detail, c.refusal, c.detail)
		}
		if refusal.Extra[viewValueView] != c.view || refusal.Extra[viewValueSection] != "1" {
			t.Errorf("%s's refusal carried view %q and section %q", c.view, refusal.Extra[viewValueView], refusal.Extra[viewValueSection])
		}
	}
}

// TestAVocabularyRefusalMarksItsSectionAndTheRestDraws is
// dinah-600/criteria/15, the hard position, over both layers: a section
// naming a column the workbench lacks is drawn refused, carrying every refused
// member, and the valid section beside it draws its cards.
func TestAVocabularyRefusalMarksItsSectionAndTheRestDraws(t *testing.T) {
	block := bench.ViewsKey + ":\n  stale:\n    sections:\n      - query: column:nosuch\n      - query: state:ready\n"
	for _, layer := range []string{"workbench", "user"} {
		t.Run(layer, func(t *testing.T) {
			h := newHarness(t)
			ready := h.ready("a ready card")
			if layer == "workbench" {
				h.declareViews(block)
			} else {
				h.userViews(block)
			}
			answer := h.mustDraw("stale", "alka")
			refused := answer.View.Sections[0]
			if refused.Refused != contract.UnknownColumn || refused.RefusedDetail != "nosuch" ||
				refused.RefusedField != FieldColumn || refused.RefusedTerm != "column:nosuch" ||
				len(refused.RefusedContext) != 0 || refused.Count != 0 || len(refused.Cards) != 0 || len(refused.Items) != 0 {
				t.Errorf("the refused section reads %+v", refused)
			}
			wantSection(t, answer, 1, ready)
		})
	}
}

// TestTheCheckThatRaisedARefusalClassifiesIt is dinah-600/criteria/32 and
// the library half of dinah-600/criteria/30. The same refusal name is a
// vocabulary refusal when check 7 or check 9 raised it and a whole-view
// refusal when check 2 or check 4 did.
func TestTheCheckThatRaisedARefusalClassifiesIt(t *testing.T) {
	h := newHarness(t)
	h.declareLevels("levels:\n  severity: [minor, major]\n")
	h.declareViews(bench.ViewsKey + ":\n" +
		oneSection("level", "severity:now") +
		oneSection("declared", "team.lead:x") +
		"  state-typo:\n    sections:\n      - query: state:ready\n      - title: Typo\n        query: state:reday\n" +
		oneSection("builtin-field", "bogus:x") +
		"  mixed:\n    sections:\n      - query: severity:now\n      - query: state:ready\n")

	level := h.mustDraw("level", "alka").View.Sections[0]
	if level.Refused != contract.UnknownValue || level.RefusedField != FieldSeverity || level.RefusedTerm != "severity:now" ||
		level.RefusedContext["term"] != "severity:now" || level.RefusedContext["field"] != FieldSeverity ||
		level.RefusedContext["legal"] != "minor, major" {
		t.Errorf("the level section reads %+v", level)
	}
	declared := h.mustDraw("declared", "alka").View.Sections[0]
	if declared.Refused != contract.UnknownField || declared.RefusedField != "team.lead" {
		t.Errorf("the declared-key section reads %+v", declared)
	}

	typo := h.refuseDraw("state-typo", "alka")
	if typo.Name != contract.UnknownValue || typo.Extra[viewValueView] != "state-typo" || typo.Extra[viewValueSection] != "2" {
		t.Errorf("state:reday in section 2 was refused %s with context %v", typo.Name, typo.Extra)
	}
	if bogus := h.refuseDraw("builtin-field", "alka"); bogus.Name != contract.UnknownField {
		t.Errorf("bogus:x was refused %s, want the whole view refused %s", bogus.Name, contract.UnknownField)
	}

	asked := h.mustDraw("mixed", "alka").View.Sections[1]
	if asked.Refused != "" || asked.RefusedDetail != "" || asked.RefusedField != "" || asked.RefusedTerm != "" || asked.RefusedContext == nil || len(asked.RefusedContext) != 0 {
		t.Errorf("a section that was asked carries refused members %+v", asked)
	}
}

// TestANameResolvesUserThenWorkbenchThenBuiltIn is dinah-600/criteria/6, 7
// and 8: a user view shadows a workbench view of its name, a malformed user
// view shadows it too and refuses rather than falling through, and a
// workbench view replaces the built-in mine until a user view replaces both.
func TestANameResolvesUserThenWorkbenchThenBuiltIn(t *testing.T) {
	h := newHarness(t)
	h.declareViews(bench.ViewsKey + ":\n  waiting-on-me:\n    sections:\n      - title: Workbench section\n        query: state:ready\n" +
		"  mine:\n    sections:\n      - title: Workbench mine\n        query: state:ready\n")
	h.userViews(bench.ViewsKey + ":\n  waiting-on-me:\n    sections:\n      - title: User section\n        query: state:blocked\n")

	if got := h.mustDraw("waiting-on-me", "alka").View.Sections[0].Title; got != "User section" {
		t.Errorf("the shadowed name drew %q, want the user's section", got)
	}
	listing, err := h.library.ListViews(&Request{Verb: "view", Actor: "alka"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var rows []string
	for _, row := range listing.Views {
		if row.Name == "waiting-on-me" || row.Name == "mine" {
			used := "no"
			if row.Used {
				used = "yes"
			}
			rows = append(rows, row.Name+" "+row.Source+" "+used)
		}
	}
	want := "mine workbench yes,mine built-in no,waiting-on-me user yes,waiting-on-me workbench no"
	if got := strings.Join(rows, ","); got != want {
		t.Errorf("the listing reads %s, want %s", got, want)
	}
	if got := h.mustDraw("mine", "alka").View.Sections[0].Title; got != "Workbench mine" {
		t.Errorf("mine drew %q, want the workbench's replacement", got)
	}

	h.userViews(bench.ViewsKey + ":\n  waiting-on-me:\n    sections: []\n  mine:\n    sections:\n      - title: User mine\n        query: state:ready\n")
	refusal := h.refuseDraw("waiting-on-me", "alka")
	if refusal.Name != contract.MalformedView || refusal.Extra["source"] != bench.ViewSourceUser || refusal.Extra["defect"] != bench.ViewNoSections {
		t.Errorf("a malformed user view was answered %s %v, want malformed-view from the user layer", refusal.Name, refusal.Extra)
	}
	if got := h.mustDraw("mine", "alka").View.Sections[0].Title; got != "User mine" {
		t.Errorf("mine drew %q, want the user's replacement", got)
	}

	h.userViews("")
	if got := h.mustDraw("waiting-on-me", "alka").View.Sections[0].Title; got != "Workbench section" {
		t.Errorf("with the user's view gone the name drew %q, want the workbench's section", got)
	}
}

// TestAnUnknownViewNamesEveryVisibleViewOnce is dinah-600/criteria/16.
func TestAnUnknownViewNamesEveryVisibleViewOnce(t *testing.T) {
	h := newHarness(t)
	h.declareViews(bench.ViewsKey + ":\n" + oneSection("zeta", "state:ready") + oneSection("alpha", "state:ready"))
	h.userViews(bench.ViewsKey + ":\n" + oneSection("zeta", "state:ready"))
	refusal := h.refuseDraw("nosuch", "alka")
	if refusal.Name != contract.UnknownView || refusal.Detail != "nosuch" || refusal.Extra["views"] != "agenda, alpha, board, mine, zeta" {
		t.Errorf("an unknown view was answered %s %q with views %q", refusal.Name, refusal.Detail, refusal.Extra["views"])
	}
}

// TestTheListingSortsByNameThenLayer is dinah-600/criteria/17's library
// half: one row per declaration, sorted by name and then user, workbench and
// built-in, each row carrying exactly the seven members.
func TestTheListingSortsByNameThenLayer(t *testing.T) {
	h := newHarness(t)
	h.declareViews(bench.ViewsKey + ":\n" + oneSection("mine", "state:ready") + oneSection("beta", "state:ready") + "  broken:\n    layout: grid\n    sections:\n      - query: state:ready\n")
	h.userViews(bench.ViewsKey + ":\n" + oneSection("mine", "state:ready") + oneSection("alpha", "state:ready"))
	listing, err := h.library.ListViews(&Request{Verb: "view", Actor: "alka"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var order []string
	for _, row := range listing.Views {
		order = append(order, row.Name+"/"+row.Source)
	}
	want := "agenda/built-in alpha/user beta/workbench board/built-in broken/workbench mine/user mine/workbench mine/built-in"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("the listing orders %s, want %s", got, want)
	}
	encoded, err := json.Marshal(listing)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded map[string][]map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil || len(decoded) != 1 {
		t.Fatalf("the listing encodes as %s", encoded)
	}
	for _, row := range decoded["views"] {
		if len(row) != 7 {
			t.Errorf("a row carries %d members, want seven: %v", len(row), row)
		}
		for _, member := range []string{"name", "title", "layout", "order", "source", "used", "malformed"} {
			if _, ok := row[member]; !ok {
				t.Errorf("a row carries no %s: %v", member, row)
			}
		}
		if row["name"] == "broken" && (row["malformed"] != bench.ViewUnknownLayout || row["layout"] != "grid") {
			t.Errorf("the malformed row reads %v", row)
		}
	}
}

// TestAnEmptySectionCarriesEveryMemberAndNoNull is dinah-600/criteria/18,
// the hard position, read off the encoded answer rather than off the Go
// value, because null is a property of the encoding.
func TestAnEmptySectionCarriesEveryMemberAndNoNull(t *testing.T) {
	h := newHarness(t)
	h.declareViews(bench.ViewsKey + ":\n" + oneSection("empty", "holder:nobody-holds-this"))
	encoded, err := json.Marshal(h.mustDraw("empty", "alka"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded struct {
		View map[string]json.RawMessage `json:"view"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.View) != 9 {
		t.Errorf("the view carries %d members, want nine: %s", len(decoded.View), encoded)
	}
	var sections []map[string]json.RawMessage
	if err := json.Unmarshal(decoded.View["sections"], &sections); err != nil || len(sections) != 1 {
		t.Fatalf("the sections decode as %v (%v)", sections, err)
	}
	section := sections[0]
	if len(section) != 12 {
		t.Errorf("the section carries %d members, want twelve: %s", len(section), encoded)
	}
	want := map[string]string{
		"cards": "[]", "count": "0", "items": "{}", "scope": `""`, "urgency": "{}", "refused": `""`, "refused_detail": `""`,
		"refused_field": `""`, "refused_term": `""`, "refused_context": "{}",
	}
	for member, literal := range want {
		if got := string(section[member]); got != literal {
			t.Errorf("%s encodes as %s, want %s", member, got, literal)
		}
	}
	var typed struct {
		Cards          []any          `json:"cards"`
		Items          map[string]any `json:"items"`
		RefusedContext map[string]any `json:"refused_context"`
	}
	raw, _ := json.Marshal(section)
	if err := json.Unmarshal(raw, &typed); err != nil || typed.Cards == nil || typed.Items == nil || typed.RefusedContext == nil {
		t.Errorf("a decoder reading the section as an array and two objects failed: %v", err)
	}
}

// TestACardMatchingTwoSectionsAppearsInBoth is dinah-600/criteria/19.
func TestACardMatchingTwoSectionsAppearsInBoth(t *testing.T) {
	h := newHarness(t)
	both := h.ready("in both sections")
	h.declareViews(bench.ViewsKey + ":\n  twice:\n    sections:\n      - query: state:ready\n      - query: column:aftercare\n")
	answer := h.mustDraw("twice", "alka")
	for i := range answer.View.Sections {
		wantSection(t, answer, i, both)
		if answer.View.Sections[i].Count != 1 {
			t.Errorf("section %d counts %d, want 1", i+1, answer.View.Sections[i].Count)
		}
	}
}

// TestTheItemsMemberNamesEveryWitness is dinah-600/criteria/20's JSON half:
// a card two items witnessed lists both, in stored order, and a section
// naming no item field carries an empty items object.
func TestTheItemsMemberNamesEveryWitness(t *testing.T) {
	h := newHarness(t)
	two := h.add("two operator questions")
	h.plantItem(two, "b00000000001", "open_question", "pending", "operator", 1)
	h.plantItem(two, "b00000000002", "decision", "pending", "holder", 2)
	h.plantItem(two, "b00000000003", "open_question", "pending", "operator", 3)
	h.declareViews(bench.ViewsKey + ":\n  asks:\n    sections:\n      - query: item_owner:operator item_state:pending\n      - query: column:intake\n")
	answer := h.mustDraw("asks", "alka")
	got := answer.View.Sections[0].Items[two]
	if strings.Join(got, " ") != two+"/questions/1 "+two+"/questions/2" {
		t.Errorf("the witnesses read %v", got)
	}
	if len(answer.View.Sections[1].Items) != 0 {
		t.Errorf("a section naming no item field carries items %v", answer.View.Sections[1].Items)
	}
}

// TestTheColumnOrderFollowsTheFlow is dinah-600/criteria/22: column orders a
// section by the flow and by arrival within one column, and arrival is the
// order query returns.
func TestTheColumnOrderFollowsTheFlow(t *testing.T) {
	h := newHarness(t)
	late := h.readyAt("filed first, standing late in the flow", aftercare)
	early := h.readyAt("filed second, standing early", intake)
	earlyToo := h.readyAt("filed third, standing early too", intake)
	h.declareViews(bench.ViewsKey + ":\n  by-column:\n    order: column\n    sections:\n      - query: state:ready\n" +
		"  by-arrival:\n    sections:\n      - query: state:ready\n")
	wantSection(t, h.mustDraw("by-column", "alka"), 0, early, earlyToo, late)
	arrival := refs(h.ask("state:ready"))
	wantSection(t, h.mustDraw("by-arrival", "alka"), 0, arrival...)
}

// TestTheLaterLayoutAndOrderAreMalformedHere is dinah-600/criteria/4 as
// dinah-602 and dinah-288/criteria/15 amend it: the urgency order and the
// columns layout both draw, and a layout word this build does not know keeps
// the view malformed.
func TestTheLaterLayoutAndOrderAreMalformedHere(t *testing.T) {
	h := newHarness(t)
	h.declareViews(bench.ViewsKey + ":\n  board:\n    layout: columns\n    sections:\n      - query: state:ready\n" +
		"  grid:\n    layout: grid\n    sections:\n      - query: state:ready\n" +
		"  ranked:\n    order: urgency\n    sections:\n      - query: state:ready\n" +
		"  flow:\n    order: column\n    sections:\n      - query: state:ready\n" +
		"  arrived:\n    order: arrival\n    sections:\n      - query: state:ready\n")
	if drawn := h.mustDraw("board", "alka"); drawn.View.Layout != bench.ViewLayoutColumns {
		t.Errorf("layout: columns drew layout %q", drawn.View.Layout)
	}
	if refusal := h.refuseDraw("grid", "alka"); refusal.Extra["defect"] != bench.ViewUnknownLayout {
		t.Errorf("layout: grid was answered %s %v", refusal.Name, refusal.Extra)
	}
	h.mustDraw("ranked", "alka")
	h.mustDraw("flow", "alka")
	h.mustDraw("arrived", "alka")
}

// TestMineShowsWhatTheCallerHoldsAndBlocked is dinah-600/criteria/23: the
// built-in view's declaration, its queries reported as written, a card moving
// from the first section to the second when its holder blocks it, and a card
// the caller blocked without ever holding it.
func TestMineShowsWhatTheCallerHoldsAndBlocked(t *testing.T) {
	h := newHarness(t)
	held := h.ready("claimed, then blocked")
	passing := h.ready("blocked in passing")
	h.mustDo(&Request{Verb: Claim, Actor: "alka", Card: held})
	answer := h.mustDraw("mine", "alka")
	body := answer.View
	if body.Title != "My cards" || body.Layout != bench.ViewLayoutList || body.Order != bench.ViewOrderColumn || body.Source != bench.ViewSourceBuiltIn || len(body.Sections) != 2 {
		t.Fatalf("mine reads %+v", body)
	}
	if body.Sections[0].Query != "holder:@me" || body.Sections[1].Query != "state:blocked actor:@me event:blocked" {
		t.Errorf("mine's queries read %q and %q", body.Sections[0].Query, body.Sections[1].Query)
	}
	wantSection(t, answer, 0, held)
	wantSection(t, answer, 1)

	h.mustDo(&Request{Verb: Block, Actor: "alka", Card: held, Reason: "waiting on a vendor"})
	h.mustDo(&Request{Verb: Block, Actor: "alka", Card: passing, Reason: "spotted in passing"})
	answer = h.mustDraw("mine", "alka")
	wantSection(t, answer, 0)
	wantSection(t, answer, 1, held, passing)
	if answer.View.Sections[1].Query != "state:blocked actor:@me event:blocked" {
		t.Errorf("the drawn query reads %q, and it is reported as declared", answer.View.Sections[1].Query)
	}
}

// TestALayerThatCannotBeReadRefusesTheViews is dinah-600/criteria/29, the
// hard position: an unreadable config.md refuses the listing and a draw even
// where the workbench declares the name, a scalar dinah.views refuses from
// either layer, and an absent config.md draws the workbench's views.
func TestALayerThatCannotBeReadRefusesTheViews(t *testing.T) {
	h := newHarness(t)
	h.declareViews(bench.ViewsKey + ":\n" + oneSection("shared", "state:ready"))
	h.mustDraw("shared", "alka")

	path := bench.UserViewsPath(h.home)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("plant a directory at the config path: %v", err)
	}
	for _, draw := range []func() error{
		func() error { _, err := h.library.ListViews(&Request{Verb: "view", Actor: "alka"}); return err },
		func() error { _, err := h.draw("shared", "alka"); return err },
	} {
		refusal, _ := draw().(*contract.Refusal)
		if refusal == nil || refusal.Name != contract.ViewsUnreadable || refusal.Extra["source"] != bench.ViewSourceUser || refusal.Extra["reason"] != "unreadable" {
			t.Errorf("an unreadable config.md was answered %v", refusal)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove the planted directory: %v", err)
	}
	h.userViews(bench.ViewsKey + ": 12\n")
	if refusal := h.refuseDraw("shared", "alka"); refusal.Name != contract.ViewsUnreadable || refusal.Extra["reason"] != "not-a-mapping" || refusal.Extra["source"] != bench.ViewSourceUser {
		t.Errorf("a scalar user block was answered %s %v", refusal.Name, refusal.Extra)
	}
	h.userViews("")
	h.declareViews(bench.ViewsKey + ": 12\n")
	if refusal := h.refuseDraw("shared", "alka"); refusal.Name != contract.ViewsUnreadable || refusal.Extra["reason"] != "not-a-mapping" || refusal.Extra["source"] != bench.ViewSourceWorkbench {
		t.Errorf("a scalar workbench block was answered %s %v", refusal.Name, refusal.Extra)
	}
}

// TestCheckReportsAWorkbenchViewItsWorkbenchRefuses is dinah-600/criteria/15's
// check half: a workbench view naming a column the workbench lacks is reported
// by section position and refusal, and the identical view in the user's
// settings is not reported at all.
func TestCheckReportsAWorkbenchViewItsWorkbenchRefuses(t *testing.T) {
	block := bench.ViewsKey + ":\n  stale:\n    sections:\n      - query: column:nosuch\n      - query: state:ready\n"
	count := func(h *harness) []string {
		report, err := h.library.Check(&Request{Verb: "check", Actor: "alka"})
		if err != nil {
			t.Fatalf("check: %v", err)
		}
		var details []string
		for _, finding := range report.Findings {
			if finding.Key == bench.FindingViewQueryRefused {
				details = append(details, finding.Detail+"/"+finding.Severity)
			}
		}
		return details
	}
	shared := newHarness(t)
	shared.declareViews(block)
	if got := strings.Join(count(shared), ","); got != "stale 1 unknown-column/"+bench.SeverityCleanup {
		t.Errorf("check reported %q for the workbench view", got)
	}
	personal := newHarness(t)
	personal.userViews(block)
	if got := count(personal); len(got) != 0 {
		t.Errorf("check reported %v for a view declared only in the user's settings", got)
	}
}
