//go:build tui

package main

import (
	"strings"
	"testing"

	"dinah/internal/contract"
)

// itemLetters are the letters item mode's footer lists for the highlighted
// item, read from the footer the head draws.
func itemLetters(m *interactiveModel) []string {
	footer := m.footerText()
	var letters []string
	for _, name := range []string{"resolve", "verify", "fail", "waive", "withdraw", "reopen", "cite"} {
		if strings.Contains(footer, actKeyOf(m, name)) {
			letters = append(letters, name)
		}
	}
	return letters
}

// itemsAt opens item mode over fx-1 as an owner, with keys moving the
// highlight, and answers the run.
func itemsAt(t *testing.T, root, actor, keys string) tuiRun {
	t.Helper()
	return fixtureRun(t, root, actor, "i"+keys)
}

// TestTheOperatorsItemsAreHisAlone is dinah-623/criteria/18: as an agent, an
// operator-owned pending question offers no r in item mode, and pressing r
// writes nothing, while the operator is offered r and answering journals the
// resolution. waive is offered to the operator alone, on pending and failed
// items only, and never to an agent.
func TestTheOperatorsItemsAreHisAlone(t *testing.T) {
	root := tuiBench(t)
	step(t, root, "file", "fx-1", "open_question", "Which vendor?", "--owner", "operator")
	agent := itemsAt(t, root, "brin", "")
	if letters := itemLetters(agent.model); strings.Contains(strings.Join(letters, " "), "resolve") || strings.Contains(strings.Join(letters, " "), "waive") {
		t.Errorf("the agent is offered %v on the operator's question", letters)
	}
	before := benchBytes(t, root)
	itemsAt(t, root, "brin", "r"+keyCtrlG)
	if !sameBytes(before, benchBytes(t, root)) {
		t.Error("r pressed by the agent changed the workbench")
	}
	operator := itemsAt(t, root, "alka", "")
	if letters := itemLetters(operator.model); !containsAll(letters, "resolve", "waive") {
		t.Errorf("the operator is offered %v on his own question", letters)
	}
	itemsAt(t, root, "alka", "r"+"the usual vendor"+keyCtrlD)
	if !journaled(t, root, "fx-1", contract.EventItemResolved) {
		t.Error("the operator's answer journaled no resolution")
	}
	resolved := itemsAt(t, root, "alka", "")
	if letters := itemLetters(resolved.model); containsAll(letters, "waive") {
		t.Errorf("waive is offered on a resolved item: %v", letters)
	}
	step(t, root, "file", "fx-1", "acceptance_criterion", "It reads every line")
	step(t, root, "fail", "fx-1/criteria/1", "--text", "it dropped one")
	failed := itemsAt(t, root, "alka", "j")
	if letters := itemLetters(failed.model); !containsAll(letters, "waive") {
		t.Errorf("waive is not offered to the operator on a failed criterion: %v", letters)
	}
	if letters := itemLetters(itemsAt(t, root, "brin", "j").model); containsAll(letters, "waive") {
		t.Errorf("waive is offered to an agent on a failed criterion: %v", letters)
	}
}

// TestWithdrawingACriterionTakesTheGrant is dinah-623/criteria/19: as an
// agent on a card without the retirement grant, a pending criterion offers
// no x; with the grant, x is offered on a pending criterion and withdraws it,
// and is not offered on a failed or waived criterion or an operator-owned
// one.
func TestWithdrawingACriterionTakesTheGrant(t *testing.T) {
	root := tuiBench(t)
	step(t, root, "file", "fx-1", "acceptance_criterion", "It reads every line")
	if letters := itemLetters(itemsAt(t, root, "brin", "").model); containsAll(letters, "withdraw") {
		t.Errorf("x is offered without the grant: %v", letters)
	}
	step(t, root, "grant", "fx-1", "criterion-retirement", "--actor", "alka")
	if letters := itemLetters(itemsAt(t, root, "brin", "").model); !containsAll(letters, "withdraw") {
		t.Errorf("x is not offered under the grant: %v", letters)
	}
	itemsAt(t, root, "brin", "x"+"nobody needs it"+keyCtrlD)
	if !journaled(t, root, "fx-1", contract.EventItemWithdrawn) {
		t.Error("x under the grant journaled no withdrawal")
	}
	step(t, root, "file", "fx-1", "acceptance_criterion", "It rejects a torn line")
	step(t, root, "fail", "fx-1/criteria/2", "--text", "it did not")
	step(t, root, "file", "fx-1", "acceptance_criterion", "It logs")
	step(t, root, "waive", "fx-1/criteria/3", "--text", "not needed", "--actor", "alka")
	step(t, root, "file", "fx-1", "acceptance_criterion", "It is fast", "--owner", "operator")
	step(t, root, "grant", "fx-1", "criterion-retirement", "--actor", "alka")
	for i, what := range []string{"failed", "waived", "operator-owned"} {
		run := itemsAt(t, root, "brin", strings.Repeat("j", i+1))
		if letters := itemLetters(run.model); containsAll(letters, "withdraw") {
			t.Errorf("x is offered on the %s criterion: %v", what, letters)
		}
	}
}

// TestReopenFollowsTheOperatorsRules is dinah-623/criteria/20: as an agent,
// reopen is not offered on a failed or waived criterion or an operator-owned
// item, and is offered on a resolved question the holder owns; the operator
// is offered reopen on a failed criterion.
func TestReopenFollowsTheOperatorsRules(t *testing.T) {
	root := tuiBench(t)
	step(t, root, "file", "fx-1", "open_question", "Which vendor?", "--owner", "holder")
	step(t, root, "resolve", "fx-1/questions/1", "--text", "the usual")
	step(t, root, "file", "fx-1", "acceptance_criterion", "It reads every line")
	step(t, root, "fail", "fx-1/criteria/1", "--text", "it dropped one")
	step(t, root, "file", "fx-1", "acceptance_criterion", "It logs")
	step(t, root, "waive", "fx-1/criteria/2", "--text", "not needed", "--actor", "alka")
	step(t, root, "file", "fx-1", "open_question", "Which colour?", "--owner", "operator")
	step(t, root, "resolve", "fx-1/questions/2", "--text", "blue")
	cases := []struct {
		keys, what string
		actor      string
		offered    bool
	}{
		{"", "a resolved question the holder owns", "brin", true},
		{"j", "a failed criterion", "brin", false},
		{"jj", "a waived criterion", "brin", false},
		{"jjj", "an operator-owned question", "brin", false},
		{"j", "a failed criterion, to the operator", "alka", true},
	}
	for _, c := range cases {
		letters := itemLetters(itemsAt(t, root, c.actor, c.keys).model)
		if containsAll(letters, "reopen") != c.offered {
			t.Errorf("reopen on %s: offered %v, wanted %v", c.what, letters, c.offered)
		}
	}
}

// TestCitingComesBeforeVerifying is dinah-623/criteria/21: on a workbench
// declaring evidence, a pending criterion with no citation offers c and not v
// or f; after citing through c with the scheme menu and the target prompt, v
// and f are offered and verifying journals the verification.
func TestCitingComesBeforeVerifying(t *testing.T) {
	root := newBenchFromDefinition(t, evidenceStatesDefinition)
	step(t, root, "add", "Build the parser")
	step(t, root, "move", "fx-1", "doing")
	step(t, root, "file", "fx-1", "acceptance_criterion", "It reads every line")
	letters := itemLetters(itemsAt(t, root, "brin", "").model)
	if !containsAll(letters, "cite") || containsAll(letters, "verify") || containsAll(letters, "fail") {
		t.Errorf("an uncited criterion offers %v", letters)
	}
	itemsAt(t, root, "brin", "c"+"1"+"https://example.test/run"+keyEnter)
	if !journaled(t, root, "fx-1", contract.EventItemCited) {
		t.Fatal("c journaled no citation")
	}
	letters = itemLetters(itemsAt(t, root, "brin", "").model)
	if !containsAll(letters, "verify", "fail") {
		t.Errorf("a cited criterion offers %v", letters)
	}
	itemsAt(t, root, "brin", "v"+"the run passed"+keyCtrlD)
	if !journaled(t, root, "fx-1", contract.EventItemVerified) {
		t.Error("v journaled no verification")
	}
}

// TestItemModeListsWhatIsOffered is dinah-623/criteria/22 and /34: i is
// offered only when some item of the target card carries an offered act, item
// mode lists only such items with every value cleaned, the footer lists only
// the letters offered on the highlighted item, and item mode scrolls: moving
// the highlight past the last drawn row keeps it shown, Page Up, Page Down,
// Home and End move it, and the digits choose nothing.
func TestItemModeListsWhatIsOffered(t *testing.T) {
	root := tuiBench(t)
	browse := fixtureRun(t, root, "brin", "")
	if strings.Contains(browse.model.footerText(), browse.model.keys().items.Help().Key+" ") {
		t.Error("i is offered on a card with no item")
	}
	for i := 0; i < 12; i++ {
		step(t, root, "file", "fx-1", "open_question", "Question \x1b[31mnumber\x07 "+string(rune('a'+i)))
	}
	run := itemsAt(t, root, "brin", "")
	wantModel(t, "the rows", len(run.model.itemRows), 12)
	for _, row := range strings.Split(run.model.content(), "\n") {
		if strings.Contains(row, "\x1b[31mnumber") || strings.Contains(row, "\x07") {
			t.Fatalf("an item row carries a control character: %q", row)
		}
	}
	short := 14
	scrolled := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("i"+strings.Repeat("j", 11)+keyCtrlC), actWidth, short))
	if !strings.Contains(scrolled.model.content(), "Question") || !strings.Contains(scrolled.model.content(), " l") {
		t.Errorf("the highlighted last row is not drawn:\n%s", scrolled.model.content())
	}
	for keys, want := range map[string]int{xtermEnd: 11, xtermEnd + xtermHome: 0, xtermPageDown: short - 3 - 1 - 2, "5": 0} {
		run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("i"+keys+keyCtrlC), actWidth, short))
		if keys == xtermPageDown {
			if run.model.itemHighlight <= 0 {
				t.Errorf("page down left the highlight at %d", run.model.itemHighlight)
			}
			continue
		}
		wantModel(t, "the highlight after "+keys, run.model.itemHighlight, want)
	}
}

// TestTheActionsMenuListsWhatTheWorkbenchWouldAccept is dinah-623/criteria/36
// and /37: x opens the actions menu over the target card and lists, in the
// command table's order, exactly the card verbs whose offer answers true. As
// an agent who does not hold the card, block, release and move are absent,
// and pressing the number a refused verb would have had writes nothing. As an
// agent on a card whose only pending question is the operator's and whose
// only criterion has failed, no answer, waive, withdraw or reopen entry is
// listed and the item step never lists the operator's question; as the
// operator, answer, waive and reopen are listed and each writes its event.
func TestTheActionsMenuListsWhatTheWorkbenchWouldAccept(t *testing.T) {
	root := tuiBench(t)
	step(t, root, "claim", "fx-1", "--actor", "cato")
	seen := fixtureRun(t, root, "brin", "x")
	var listed []string
	for _, row := range seen.model.menu.rows {
		listed = append(listed, row.value)
	}
	var want []string
	for _, row := range seen.model.actionRows() {
		want = append(want, row.verb)
	}
	if strings.Join(listed, " ") != strings.Join(want, " ") {
		t.Errorf("the menu lists %v, and the offered entries in the table's order are %v", listed, want)
	}
	for _, refused := range []string{"block", "release", "move"} {
		if containsAll(listed, refused) {
			t.Errorf("the menu lists %s for an agent who does not hold the card", refused)
		}
		before := benchBytes(t, root)
		number := wouldBeNumber(refused)
		press := "x"
		if number <= 9 && digitIsSafe(seen.model, number) {
			press += string(rune('0' + number))
		}
		fixtureRun(t, root, "brin", press+keyCtrlG+keyCtrlG)
		if !sameBytes(before, benchBytes(t, root)) {
			t.Errorf("pressing %q changed the workbench", press)
		}
	}

	root = tuiBench(t)
	step(t, root, "file", "fx-1", "open_question", "Which vendor?", "--owner", "operator")
	step(t, root, "file", "fx-1", "acceptance_criterion", "It reads every line")
	step(t, root, "fail", "fx-1/criteria/1", "--text", "it dropped one")
	agent := fixtureRun(t, root, "brin", "x")
	for _, verb := range []string{"resolve", "waive", "withdraw", "reopen"} {
		if menuIndex(agent.model, verb) >= 0 {
			t.Errorf("the agent's menu lists %s", verb)
		}
	}
	// Cite is the one item entry the agent is offered here, and its item
	// step lists the operator's question: Cite runs no owner row, so the
	// library accepts a citation on the operator's item from anybody, and
	// the menu offers what the library accepts. Every other item entry the
	// agent is offered must leave the question out.
	checked := 0
	for _, verb := range []string{"resolve", "verify", "fail", "waive", "withdraw", "reopen", "cite"} {
		index := menuIndex(agent.model, verb)
		if index < 0 {
			continue
		}
		step := fixtureRun(t, root, "brin", "x"+strings.Repeat("j", index)+keyEnter)
		for _, row := range step.model.menu.rows {
			checked++
			if verb != "cite" && strings.Contains(row.label, "questions/1") {
				t.Errorf("the item step of %s lists the operator's question to an agent: %q", verb, row.label)
			}
		}
	}
	if checked == 0 {
		t.Error("no item step was read, so the check proves nothing")
	}
	operator := fixtureRun(t, root, "alka", "x")
	for verb, keys := range map[string]string{"resolve": "1the usual vendor" + keyCtrlD, "waive": "1not needed" + keyCtrlD, "reopen": "1closed wrongly" + keyCtrlD} {
		index := menuIndex(operator.model, verb)
		if index < 0 {
			t.Errorf("the operator's menu does not list %s", verb)
			continue
		}
		fresh := copyWorkbench(t, root)
		fixtureRun(t, fresh, "alka", "x"+strings.Repeat("j", index)+keyEnter+keys)
		event := map[string]string{"resolve": contract.EventItemResolved, "waive": contract.EventItemWaived, "reopen": contract.EventItemReopened}[verb]
		if !journaled(t, fresh, "fx-1", event) {
			t.Errorf("%s from the operator's menu journaled no %s", verb, event)
		}
	}
}

// TestTheMenusOfferLegalValues is dinah-623/criteria/38: an actions-menu
// entry gathers its arguments from menus of legal values where section 6.3
// names a menu: set a field lists only the fields set would write and then
// the closed values of a closed field, join lists only live workstreams the
// card is not in, and a verb refused when it runs, such as a link to a card
// that does not exist, shows the CLI's refusal and writes nothing.
func TestTheMenusOfferLegalValues(t *testing.T) {
	root := newBenchFromDefinition(t, offerDefinition)
	step(t, root, "add", "Build the parser")
	step(t, root, "move", "fx-1", "work")
	step(t, root, "workstream", "new", "Parser", "--slug", "parser")
	step(t, root, "workstream", "new", "Release", "--slug", "release")
	step(t, root, "join", "fx-1", "parser")
	seen := fixtureRun(t, root, "brin", "x")
	fields := fixtureRun(t, root, "brin", "x"+strings.Repeat("j", menuIndex(seen.model, "set"))+keyEnter)
	var rows []string
	for _, row := range fields.model.menu.rows {
		rows = append(rows, row.value)
	}
	if strings.Join(rows, " ") != strings.Join(fields.model.acts().Fields, " ") || !containsAll(rows, "tier", "card.lane") {
		t.Errorf("set a field lists %v, and set would write %v", rows, fields.model.acts().Fields)
	}
	tier := 0
	for i, row := range rows {
		if row == "tier" {
			tier = i
		}
	}
	values := fixtureRun(t, root, "brin", "x"+strings.Repeat("j", menuIndex(seen.model, "set"))+keyEnter+strings.Repeat("j", tier)+keyEnter)
	var closed []string
	for _, row := range values.model.menu.rows {
		closed = append(closed, row.value)
	}
	if strings.Join(closed, " ") != "workhorse frontier" {
		t.Errorf("the tier's values are listed %v", closed)
	}
	join := fixtureRun(t, root, "brin", "x"+strings.Repeat("j", menuIndex(seen.model, "join"))+keyEnter)
	var streams []string
	for _, row := range join.model.menu.rows {
		streams = append(streams, row.value)
	}
	if strings.Join(streams, " ") != "release" {
		t.Errorf("join lists %v, wanted the one live workstream the card is not in", streams)
	}
	before := benchBytes(t, root)
	link := fixtureRun(t, root, "brin", "x"+strings.Repeat("j", menuIndex(seen.model, "link"))+keyEnter+"relates_to"+keyEnter+"fx-99"+keyEnter)
	want := cliLines(t, copyWorkbench(t, root), []string{"link", "fx-1", "relates_to", "fx-99"})
	if strings.Join(link.model.message, "\n") != strings.Join(want, "\n") {
		t.Errorf("the refused link shows\n%s\nand dinah link writes\n%s", strings.Join(link.model.message, "\n"), strings.Join(want, "\n"))
	}
	if !sameBytes(before, benchBytes(t, root)) {
		t.Error("the refused link changed the workbench")
	}
}

// TestEditIsOfferedOnlyWhereAnEditorResolves holds the actions menu's edit
// entry to the editor ladder runEdit climbs: with no editor named anywhere
// and none of the fallbacks on PATH, the card still has something to edit
// and the menu does not list edit, so choosing it can never end the program
// only to be refused as dinah.no-editor; with DINAH_EDITOR set it does.
func TestEditIsOfferedOnlyWhereAnEditorResolves(t *testing.T) {
	root := tuiBench(t)
	for _, name := range []string{"DINAH_EDITOR", "VISUAL", "EDITOR"} {
		t.Setenv(name, "")
	}
	t.Setenv("PATH", t.TempDir())
	if index := menuIndex(fixtureRun(t, root, "alka", "x").model, "edit"); index >= 0 {
		t.Errorf("the actions menu lists edit with no editor to run, at row %d", index+1)
	}
	t.Setenv("DINAH_EDITOR", "an-editor")
	if index := menuIndex(fixtureRun(t, root, "alka", "x").model, "edit"); index < 0 {
		t.Error("the actions menu does not list edit with DINAH_EDITOR set")
	}
}
