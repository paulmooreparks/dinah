//go:build tui

package main

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"

	"dinah/internal/msg"
)

// keyRowCase is one row of a key table in section 5.2: the keys written
// through the seam, what to arrange first, and what the final model and the
// workbench must show.
type keyRowCase struct {
	name    string
	arrange func(t *testing.T, root string)
	keys    string
	check   func(t *testing.T, root string, run tuiRun)
}

// selectedRef answers the reference of the card a final model selects.
func selectedRef(m *interactiveModel) string {
	card, ok := m.selectedCard()
	if !ok {
		return ""
	}
	return card.Ref
}

// focusedColumn answers the title of the lane a final model focuses.
func focusedColumn(m *interactiveModel) string {
	lane, ok := m.focusedLane()
	if !ok {
		return ""
	}
	return lane.title
}

// stateOf answers what the anchor of a card says its column and state are.
func stateOf(t *testing.T, root, ref string) string {
	t.Helper()
	anchor := anchorText(t, root, ref)
	var parts []string
	for _, line := range strings.Split(anchor, "\n") {
		if strings.HasPrefix(line, "state: ") || strings.HasPrefix(line, "claim_holder: ") {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, ", ")
}

// wantModel fails when a final model's field differs from what a row states.
func wantModel(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s is %v, wanted %v", what, got, want)
	}
}

// claimFirst claims fx-1 as the operator before the head starts.
func claimFirst(t *testing.T, root string) {
	t.Helper()
	step(t, root, "claim", "fx-1")
}

// longBody gives fx-1 a body of sixty lines, so card mode has something to
// scroll.
func longBody(t *testing.T, root string) {
	t.Helper()
	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, "Line of the body.")
	}
	step(t, root, "set", "fx-1", "body", strings.Join(lines, "\n"))
}

// keyRows are every row of every key table in section 5.2. The window is 100
// by 30 and the head starts on the Implement lane, holding fx-1 and fx-2,
// beside the Acceptance lane, holding fx-3, fx-4 and fx-5, as the operator.
var keyRows = []keyRowCase{
	{name: "browse down selects the next card", keys: xtermDown + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-2")
	}},
	{name: "browse j selects the next card, and nothing past the last", keys: "jjjq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-2")
	}},
	{name: "browse up selects the previous card, and nothing before the first", keys: xtermDown + xtermUp + xtermUp + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-1")
	}},
	{name: "browse k selects the previous card", keys: "jkq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-1")
	}},
	{name: "browse right focuses the next lane on its first card", keys: "j" + xtermRight + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the lane", focusedColumn(run.model), "Acceptance")
		wantModel(t, "the selection", selectedRef(run.model), "fx-3")
	}},
	{name: "browse l focuses the next lane, and nothing past the last", keys: "lllq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the lane", focusedColumn(run.model), "Acceptance")
	}},
	{name: "browse left focuses the previous lane, and nothing before the first", keys: "l" + xtermLeft + xtermLeft + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the lane", focusedColumn(run.model), "Implement")
		wantModel(t, "the selection", selectedRef(run.model), "fx-1")
	}},
	{name: "browse h focuses the previous lane", keys: "lhq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the lane", focusedColumn(run.model), "Implement")
	}},
	{name: "browse page down and page up stop at either end", keys: "l" + xtermPageDown + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-5")
	}},
	{name: "browse page up stops at the first card", keys: "l" + xtermPageDown + xtermPageUp + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-3")
	}},
	{name: "browse end and home select the last and the first card", keys: "l" + xtermEnd + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-5")
	}},
	{name: "browse home selects the first card", keys: "l" + xtermEnd + xtermHome + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-3")
	}},
	{name: "browse enter opens card mode on the selected card", keys: "j" + keyEnter + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the mode", run.model.mode, modeCard)
		wantModel(t, "the card shown", run.model.cardRef, "fx-2")
	}},
	{name: "browse colon opens the jump prompt", keys: ":q" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the mode", run.model.mode, modePrompt)
		wantModel(t, "the prompt", run.model.prompt, promptJump)
		wantModel(t, "the prompt's text", run.model.input.Value(), "q")
	}},
	{name: "browse slash opens the filter prompt holding the current filter", keys: "/state:ready" + keyEnter + "/" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the prompt", run.model.prompt, promptFilter)
		wantModel(t, "the prompt's text", run.model.input.Value(), "state:ready")
	}},
	{name: "browse t claims the selected card", keys: "tq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-1", stateOf(t, root, "fx-1"), "state: active, claim_holder: alka")
	}},
	{name: "browse a advances the card along its route", keys: "aq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-1's column", columnOf(t, root, "fx-1"), "Acceptance")
	}},
	{name: "browse a accepts into the done column", keys: "laq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-3's column", columnOf(t, root, "fx-3"), "Done")
	}},
	{name: "browse b sends the card back", keys: "lbq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-3's column", columnOf(t, root, "fx-3"), "Implement")
	}},
	{name: "browse m opens the move menu", keys: "m" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the mode", run.model.mode, modeMenu)
		if run.model.menu == nil || len(run.model.menu.rows) == 0 {
			t.Error("the menu held no rows")
		}
	}},
	{name: "browse r releases the card", arrange: claimFirst, keys: "rq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-1", stateOf(t, root, "fx-1"), "state: ready")
	}},
	{name: "browse c opens the comment prompt", keys: "c" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the prompt", run.model.prompt, promptText)
		wantModel(t, "the mode", run.model.mode, modePrompt)
	}},
	{name: "browse question mark shows the full help", keys: "?q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the full help", run.model.fullHelp, true)
	}},
	{name: "browse question mark twice hides it again", keys: "??q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the full help", run.model.fullHelp, false)
	}},
	{name: "browse q quits", keys: "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the exit", run.code, 0)
		wantModel(t, "quitting", run.model.quitting, true)
	}},
	{name: "browse ctrl+c quits", keys: keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the exit", run.code, 0)
		wantModel(t, "quitting", run.model.quitting, true)
	}},
	{name: "card down, j, page down and end scroll the viewport", arrange: longBody, keys: keyEnter + xtermDown + "j" + xtermPageDown + "q", check: func(t *testing.T, root string, run tuiRun) {
		if run.model.viewport.YOffset() < 3 {
			t.Errorf("the viewport scrolled to %d", run.model.viewport.YOffset())
		}
	}},
	{name: "card up, k, page up and home scroll back", arrange: longBody, keys: keyEnter + xtermEnd + xtermPageUp + xtermUp + "k" + xtermHome + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the viewport's offset", run.model.viewport.YOffset(), 0)
	}},
	{name: "card end reaches the bottom", arrange: longBody, keys: keyEnter + xtermEnd + "q", check: func(t *testing.T, root string, run tuiRun) {
		if !run.model.viewport.AtBottom() {
			t.Errorf("the viewport stopped at %d", run.model.viewport.YOffset())
		}
	}},
	{name: "card enter returns to browse", keys: keyEnter + keyEnter + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the mode", run.model.mode, modeBrowse)
	}},
	{name: "card backspace returns to browse", keys: keyEnter + keyBackspace + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the mode", run.model.mode, modeBrowse)
	}},
	{name: "card t claims the card shown", keys: keyEnter + "tq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-1", stateOf(t, root, "fx-1"), "state: active, claim_holder: alka")
		wantModel(t, "the mode after a successful act", run.model.mode, modeBrowse)
	}},
	{name: "card a advances the card shown", keys: keyEnter + "aq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-1's column", columnOf(t, root, "fx-1"), "Acceptance")
	}},
	{name: "card b sends the card shown back", keys: "l" + keyEnter + "bq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-3's column", columnOf(t, root, "fx-3"), "Implement")
	}},
	{name: "card m opens the move menu over the card", keys: keyEnter + "m2q", check: func(t *testing.T, root string, run tuiRun) {
		if got := columnOf(t, root, "fx-1"); got == "Implement" {
			t.Error("choosing a row of the menu opened in card mode left the card where it was")
		}
	}},
	{name: "card r releases the card shown", arrange: claimFirst, keys: keyEnter + "rq", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-1", stateOf(t, root, "fx-1"), "state: ready")
	}},
	{name: "card c comments on the card shown", keys: keyEnter + "cfrom card mode" + keyCtrlD + "q", check: func(t *testing.T, root string, run tuiRun) {
		if !strings.Contains(journalText(t, root, "fx-1"), "commented") {
			t.Error("no comment was journaled on fx-1")
		}
	}},
	{name: "card question mark shows the full help", keys: keyEnter + "?q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the full help", run.model.fullHelp, true)
	}},
	{name: "card q quits", keys: keyEnter + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "quitting", run.model.quitting, true)
	}},
	{name: "card ctrl+c quits", keys: keyEnter + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "quitting", run.model.quitting, true)
	}},
	{name: "menu down and j move the highlight", keys: "m" + xtermDown + "j" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the highlight", run.model.menuHighlight(), 2)
	}},
	{name: "menu up and k move the highlight back", keys: "mjj" + xtermUp + "k" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the highlight", run.model.menuHighlight(), 0)
	}},
	{name: "menu digit chooses its row at once", keys: "m1q", check: func(t *testing.T, root string, run tuiRun) {
		if got := columnOf(t, root, "fx-1"); got == "Implement" {
			t.Error("1 in the menu left the card where it was")
		}
	}},
	{name: "menu enter chooses the highlighted row", keys: "mj" + keyEnter + "q", check: func(t *testing.T, root string, run tuiRun) {
		if got := columnOf(t, root, "fx-1"); got == "Implement" {
			t.Error("enter in the menu left the card where it was")
		}
	}},
	{name: "menu ctrl+g closes it doing nothing", keys: "m" + keyCtrlG + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-1's column", columnOf(t, root, "fx-1"), "Implement")
		wantModel(t, "the mode", run.model.mode, modeBrowse)
	}},
	{name: "menu q closes it doing nothing", keys: "mq" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-1's column", columnOf(t, root, "fx-1"), "Implement")
	}},
	{name: "menu ctrl+c quits doing nothing", keys: "m" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "fx-1's column", columnOf(t, root, "fx-1"), "Implement")
		wantModel(t, "quitting", run.model.quitting, true)
	}},
	{name: "jump prompt edits its text", keys: ":fx-2x" + keyBackspace + xtermLeft + xtermDelete + xtermHome + "@" + xtermEnd + "9" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the prompt's text", run.model.input.Value(), "@fx-9")
	}},
	{name: "jump prompt ctrl editing keys", keys: ":one two" + "\x17" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the prompt's text", run.model.input.Value(), "one ")
	}},
	{name: "jump prompt enter submits", keys: ":fx-2" + keyEnter + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-2")
	}},
	{name: "jump prompt ctrl+g closes it keeping no text", keys: ":fx-2" + keyCtrlG + ":" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the prompt's text", run.model.input.Value(), "")
		wantModel(t, "the selection", selectedRef(run.model), "fx-1")
	}},
	{name: "jump prompt ctrl+c quits doing nothing", keys: ":fx-2" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the selection", selectedRef(run.model), "fx-1")
		wantModel(t, "quitting", run.model.quitting, true)
	}},
	{name: "command line paste of two lines takes nothing", keys: ":" + pasteOpen + "fx\r\n-2" + pasteClose + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the prompt's text", run.model.input.Value(), "")
		wantModel(t, "the message", run.model.message, []string{run.model.s.r.T("interactive.line.paste")})
	}},
	{name: "filter prompt enter submits", keys: "/column:acceptance" + keyEnter + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the filter", run.model.filter, "column:acceptance")
		wantModel(t, "the lane left", focusedColumn(run.model), "Acceptance")
	}},
	{name: "filter prompt ctrl+g closes it", keys: "/column:acceptance" + keyCtrlG + "q", check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the filter", run.model.filter, "")
	}},
	{name: "comment prompt edits its text", keys: "cab" + keyBackspace + xtermLeft + "x" + xtermEnd + "y" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the comment's text", run.model.area.Value(), "xay")
	}},
	{name: "comment prompt up, down, page keys and ctrl editing keys", keys: "cone" + keyEnter + "two" + xtermUp + xtermEnd + "!" + xtermDown + xtermPageUp + xtermPageDown + "\x01>" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the comment's text", run.model.area.Value(), "one!\n>two")
	}},
	{name: "comment prompt enter starts a new line", keys: "cone" + keyEnter + "two" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the comment's text", run.model.area.Value(), "one\ntwo")
	}},
	{name: "comment prompt ctrl+d posts", keys: "cposted" + keyCtrlD + "q", check: func(t *testing.T, root string, run tuiRun) {
		if !strings.Contains(journalText(t, root, "fx-1"), "commented") {
			t.Error("no comment was journaled on fx-1")
		}
	}},
	{name: "comment prompt ctrl+g closes it posting nothing", keys: "cunsaid" + keyCtrlG + "q", check: func(t *testing.T, root string, run tuiRun) {
		if strings.Contains(journalText(t, root, "fx-1"), "commented") {
			t.Error("a comment was journaled although the prompt was cancelled")
		}
	}},
	{name: "comment prompt ctrl+c quits posting nothing", keys: "cunsaid" + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		if strings.Contains(journalText(t, root, "fx-1"), "commented") {
			t.Error("a comment was journaled although the head quit")
		}
	}},
	{name: "comment prompt paste keeps its line breaks", keys: "c" + pasteOpen + "one\r\ntwo" + pasteClose + keyCtrlC, check: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the comment's text", run.model.area.Value(), "one\ntwo")
	}},
}

// columnOf answers the title of the column a card stands in.
func columnOf(t *testing.T, root, ref string) string {
	t.Helper()
	got := runCLI(t, root, "show", ref, "--json", "--fields", "card")
	for _, line := range strings.Split(got.out, "\n") {
		if at := strings.Index(line, `"column_title": "`); at >= 0 {
			return strings.TrimSuffix(strings.TrimSpace(line[at+len(`"column_title": "`):]), `",`)
		}
	}
	return ""
}

// TestEveryKeyOfEveryTableDoesWhatItsRowSays is dinah-603/criteria/18: every
// row of every key table in section 5.2 is written through the seam as the
// bytes a terminal sends, and the effect the row states is read off the final
// model or the workbench. It counts the rows it ran.
func TestEveryKeyOfEveryTableDoesWhatItsRowSays(t *testing.T) {
	for _, row := range keyRows {
		t.Run(row.name, func(t *testing.T) {
			root := tuiBench(t)
			if row.arrange != nil {
				row.arrange(t, root)
			}
			run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(row.keys), 100, 30))
			if !run.finished || run.model == nil {
				t.Fatalf("the program never finished: exit %d, stderr %q", run.code, run.errw)
			}
			row.check(t, root, run)
		})
	}
	t.Logf("%d rows", len(keyRows))
	if len(keyRows) < 50 {
		t.Fatalf("the table holds %d rows, fewer than section 5.2 lists", len(keyRows))
	}
}

// TestAnActTheOfferDoesNotAllowWritesNothing is the last clause of
// dinah-603/criteria/18: for an agent that is not the operator, a, b, m and t
// on a card at the operator-owned Acceptance column are offered nothing, and
// pressing them leaves the card's anchor and journal byte-identical.
func TestAnActTheOfferDoesNotAllowWritesNothing(t *testing.T) {
	root := tuiBench(t)
	t.Setenv("DINAH_ACTOR", "brin")
	anchor, journal := anchorText(t, root, "fx-3"), journalText(t, root, "fx-3")
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("labmtr1q"+keyCtrlC), 100, 30))
	if run.model == nil {
		t.Fatalf("the program never finished: %q", run.errw)
	}
	if offer := run.model.offer; offer.claim || offer.forward != nil || offer.back != nil || len(offer.moves) > 0 || offer.release {
		t.Errorf("the agent was offered %+v on fx-3", offer)
	}
	if anchorText(t, root, "fx-3") != anchor || journalText(t, root, "fx-3") != journal {
		t.Error("a key the offer did not allow wrote to fx-3")
	}
}

// TestNoBindingHoldsEscOrAnAltKey is the key-map half of
// dinah-603/criteria/15: no binding the head defines, in any language, and
// no binding of the textinput and textarea key maps it builds, holds esc or
// an alt+ key.
func TestNoBindingHoldsEscOrAnAltKey(t *testing.T) {
	checked := 0
	check := func(where string, binding key.Binding) {
		for _, name := range binding.Keys() {
			checked++
			if name == "esc" || strings.Contains(name, "alt+") {
				t.Errorf("%s binds %q", where, name)
			}
		}
	}
	for _, tag := range msg.Tags() {
		for _, b := range interactiveBindings(msg.For(tag)) {
			check(tag+" "+b.key, b.binding)
		}
	}
	for _, keyMap := range []any{interactiveInputKeys(), interactiveAreaKeys()} {
		value := reflect.ValueOf(keyMap)
		for i := 0; i < value.NumField(); i++ {
			if binding, ok := value.Field(i).Interface().(key.Binding); ok {
				check(value.Type().Name()+"."+value.Type().Field(i).Name, binding)
			}
		}
	}
	t.Logf("%d keys checked", checked)
	if checked < 100 {
		t.Fatalf("checked %d keys, which is fewer than the maps hold", checked)
	}
}
