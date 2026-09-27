//go:build tui

package main

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// TestTheDirectClassIsWhatTheUIReaches is dinah-623/criteria/2 and the second
// half of /29: the commands classed direct are exactly the commands the
// terminal UI reaches from a key row of interactiveActs or an entry of the
// actions menu. It fails naming a direct command reached by neither, a verb
// of either that is not classed direct, a card verb with no actions-menu
// entry, a menu entry whose verb does not act on a card, and a frequent read
// with no key row, and it reports the size of every set it read.
func TestTheDirectClassIsWhatTheUIReaches(t *testing.T) {
	keyVerbs := map[string]bool{}
	rows := 0
	for _, row := range interactiveActs {
		rows++
		keyVerbs[row.verb] = true
	}
	menu := 0
	for name := range actionsMenu {
		menu++
		c, known := lookup(name)
		switch {
		case !known:
			t.Errorf("the actions menu has an entry for %s, which is no command", name)
		case !c.actsOnCard:
			t.Errorf("the actions menu has an entry for %s, which does not act on a card", name)
		}
	}
	for verb := range keyVerbs {
		if c, known := lookup(verb); !known || c.terminal != terminalDirect {
			t.Errorf("a key row performs %s, which is not classed direct", verb)
		}
	}
	for name := range actionsMenu {
		if c, known := lookup(name); known && c.terminal != terminalDirect {
			t.Errorf("the actions menu offers %s, which is not classed direct", name)
		}
	}
	direct, cardVerbs, reads := 0, 0, 0
	for _, c := range commands {
		if c.terminal == terminalDirect {
			direct++
			if !keyVerbs[c.name] && actionsMenu[c.name].offered == nil {
				t.Errorf("%s is classed direct, and neither a key row nor the actions menu reaches it", c.name)
			}
		}
		if c.actsOnCard {
			cardVerbs++
			if actionsMenu[c.name].offered == nil {
				t.Errorf("%s acts on a card and the actions menu has no entry for it", c.name)
			}
		}
		if c.frequentRead {
			reads++
			if !keyVerbs[c.name] {
				t.Errorf("%s is a frequent read and no key row performs it", c.name)
			}
		}
	}
	t.Logf("%d commands are classed direct, the actions menu has %d entries, and the act table has %d key rows; %d card verbs and %d frequent reads", direct, menu, rows, cardVerbs, reads)
	if direct == 0 || menu == 0 || rows == 0 || cardVerbs == 0 || reads == 0 {
		t.Error("a set this test reads is empty, so it proves nothing")
	}
}

// TestEveryStepNamesAParameter is section 4.2's check: every step of every
// key row and every actions-menu entry gathers a parameter the verb it
// performs declares, so a gathered value always lands on the request.
func TestEveryStepNamesAParameter(t *testing.T) {
	swept := 0
	check := func(where, command string, steps []interactiveStep) {
		declared := map[string]bool{}
		for _, param := range verb.Params(command) {
			declared[param.Name] = true
		}
		for _, step := range steps {
			swept++
			if !declared[step.param] {
				t.Errorf("%s gathers %s, which %s does not declare", where, step.param, command)
			}
		}
	}
	for _, row := range interactiveActs {
		check("the key row "+row.name, row.verb, row.steps)
	}
	for name, entry := range actionsMenu {
		check("the actions menu's entry for "+name, name, entry.steps)
	}
	t.Logf("%d steps swept", swept)
	if swept == 0 {
		t.Error("no step was swept, so this check proves nothing")
	}
}

// TestTheReservedKeysAreTheBrowseKeys is the check of section 7.4, half of
// dinah-623/criteria/41: terminalReservedKeys holds exactly the letters and
// digits a binding of browse or card mode reads, so a card adding a key
// reserves it in the same change and a key binding can never shadow one.
func TestTheReservedKeysAreTheBrowseKeys(t *testing.T) {
	read := map[string]bool{}
	for _, b := range interactiveBindings(msg.For(msg.Base)) {
		if b.mode != bindingBrowse && b.mode != bindingCard {
			continue
		}
		for _, pressed := range b.binding.Keys() {
			if bench.ValidKeyBindingKey(pressed) {
				read[pressed] = true
			}
		}
	}
	for pressed := range read {
		if _, reserved := terminalReservedKeys[pressed]; !reserved {
			t.Errorf("browse or card mode reads %s, and terminalReservedKeys does not reserve it, so a key binding could shadow it", pressed)
		}
	}
	for pressed := range terminalReservedKeys {
		if !read[pressed] {
			t.Errorf("terminalReservedKeys reserves %s, and no binding of browse or card mode reads it", pressed)
		}
	}
	t.Logf("%d letters and digits read in browse and card mode, %d reserved", len(read), len(terminalReservedKeys))
	if len(read) == 0 {
		t.Error("no letter or digit was read from the bindings, so this check proves nothing")
	}
}

// lineRuns records every line the head ran through the seam's lineDone.
type lineRuns struct {
	results []*lineResult
}

// seamed installs the recorder on a seam.
func (r *lineRuns) seamed(seam *interactiveSeams) *interactiveSeams {
	seam.lineDone = func(result *lineResult) { r.results = append(r.results, result) }
	return seam
}

// only answers the one line recorded, failing where there was not exactly one.
func (r *lineRuns) only(t *testing.T) *lineResult {
	t.Helper()
	if len(r.results) != 1 {
		t.Fatalf("the head ran %d lines, wanted one", len(r.results))
	}
	return r.results[0]
}

// lineKeys types each line at the command line and presses Enter after it.
func lineKeys(lines ...string) string {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(":" + line + keyEnter)
	}
	return b.String()
}

// TestEveryLineCommandReachesItsRunFunction is dinah-623/criteria/6: every
// command classed direct or line, typed bare at the command line, reaches the
// point just ahead of its own run function with its own name, and the count
// is len(commands) less the absent count rather than a number written here.
func TestEveryLineCommandReachesItsRunFunction(t *testing.T) {
	root := tuiBench(t)
	var names []string
	absent := 0
	for _, c := range commands {
		if c.terminal == terminalAbsent {
			absent++
			continue
		}
		names = append(names, c.name)
	}
	var reached []string
	seam := tuiSeam(t, strings.NewReader(lineKeys(names...)+keyCtrlC), 120, 30)
	seam.lineDispatch = func(name string) bool {
		reached = append(reached, name)
		return true
	}
	run := runTUIThrough(t, root, seam)
	if run.code != 0 {
		t.Fatalf("the head exited %d: %s", run.code, run.errw)
	}
	want := len(commands) - absent
	if len(reached) != want {
		t.Errorf("%d commands reached the dispatch, wanted %d", len(reached), want)
	}
	for i, name := range names {
		if i >= len(reached) || reached[i] != name {
			t.Errorf("the %dth line was %s and it did not reach the dispatch as itself: %v", i+1, name, reached)
			break
		}
	}
	t.Logf("%d of %d commands reached the dispatch; %d are absent", len(reached), len(commands), absent)
}

// shownLines is what the head shows of a line: the transcript output mode
// holds where it opened, and the message area otherwise.
func shownLines(m *interactiveModel) []string {
	if m.mode == modeOutput {
		return m.output
	}
	return m.message
}

// refusalShown composes a refusal as the command line shows it, through a
// copy of the head's own session naming command.
func refusalShown(m *interactiveModel, command string, refusal *contract.Refusal) []string {
	composer := *m.s
	composer.command = command
	var lines []string
	for _, line := range composer.composeRefusal(refusal) {
		lines = append(lines, withoutControls(line))
	}
	return lines
}

// TestEveryAbsentCommandIsRefusedWithItsReason is dinah-623/criteria/5: each
// absent command and each flag terminalRefusedFlags names, typed at the
// command line, is refused as dinah.not-in-tui with that reason's sentence
// and never reaches the dispatch, and each session flag the line may not
// give is refused as dinah.usage naming it.
func TestEveryAbsentCommandIsRefusedWithItsReason(t *testing.T) {
	type refused struct {
		line    string
		command string
		refusal *contract.Refusal
	}
	var cases []refused
	commandsSwept, flagsSwept, sessionSwept := 0, 0, 0
	for _, c := range commands {
		if c.terminal != terminalAbsent {
			continue
		}
		commandsSwept++
		cases = append(cases, refused{c.name, c.name, contract.RefuseWith(contract.NotInTUI, c.name, map[string]string{"reason": c.name})})
	}
	for _, name := range sortedFlagCommands() {
		for _, flag := range sortedFlagNames(terminalRefusedFlags[name]) {
			flagsSwept++
			token := terminalRefusedFlags[name][flag]
			cases = append(cases, refused{name + " --" + flag, name, contract.RefuseWith(contract.NotInTUI, "--"+flag, map[string]string{"reason": token})})
		}
	}
	for _, name := range sortedNames(sessionFlagNames) {
		if terminalSessionFlags[name] {
			continue
		}
		sessionSwept++
		cases = append(cases, refused{"status --" + name + " x", "", contract.Refuse(contract.Usage, "--"+name)})
	}
	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			root := tuiBench(t)
			var lines lineRuns
			reached := false
			seam := lines.seamed(tuiSeam(t, strings.NewReader(lineKeys(c.line)+keyCtrlC), 120, 30))
			seam.lineDispatch = func(string) bool { reached = true; return true }
			run := runTUIThrough(t, root, seam)
			if run.model == nil {
				t.Fatalf("the run never finished: %q", run.errw)
			}
			if reached {
				t.Errorf("%s reached the dispatch", c.line)
			}
			want := refusalShown(run.model, c.command, c.refusal)
			if got := shownLines(run.model); strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("the head shows\n%s\nwanted\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			if !strings.HasPrefix(want[0], c.refusal.Name+" ") {
				t.Errorf("the refusal composed reads %q", want[0])
			}
		})
	}
	for _, tag := range msg.Tags() {
		for _, c := range commands {
			if c.terminal != terminalAbsent {
				continue
			}
			if _, held := msg.CatalogEntry(tag, "refusal.dinah.not-in-tui."+c.name); !held {
				t.Errorf("%s carries no reason sentence for %s", tag, c.name)
			}
		}
	}
	t.Logf("swept %d commands, %d flags and %d session flags", commandsSwept, flagsSwept, sessionSwept)
	if commandsSwept == 0 || flagsSwept == 0 || sessionSwept == 0 {
		t.Error("a set this test reads is empty, so it proves nothing")
	}
}

// sortedFlagCommands are the commands terminalRefusedFlags names, in order.
func sortedFlagCommands() []string {
	names := make([]string, 0, len(terminalRefusedFlags))
	for name := range terminalRefusedFlags {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestAStdinValueIsRefusedWhenRead is dinah-623/criteria/32: comment and set
// given - at the command line are refused as dinah.not-in-tui with the stdin
// reason when the command reads standard input, and leave the card's anchor
// and journal byte-identical, while add given - files a card titled -, since
// add reads no standard input and a shell accepts the same line.
func TestAStdinValueIsRefusedWhenRead(t *testing.T) {
	for _, line := range []string{"comment fx-1 -", "set fx-1 title -"} {
		t.Run(line, func(t *testing.T) {
			root := tuiBench(t)
			anchor, journal := anchorText(t, root, "fx-1"), journalText(t, root, "fx-1")
			run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(lineKeys(line)+keyCtrlC), 120, 30))
			if run.model == nil {
				t.Fatalf("the run never finished: %q", run.errw)
			}
			command := strings.Fields(line)[0]
			want := refusalShown(run.model, command, contract.RefuseWith(contract.NotInTUI, "-", map[string]string{"reason": "stdin"}))
			if got := shownLines(run.model); strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("the head shows\n%s\nwanted\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			if anchorText(t, root, "fx-1") != anchor || journalText(t, root, "fx-1") != journal {
				t.Errorf("%s changed fx-1", line)
			}
		})
	}
	root := tuiBench(t)
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(lineKeys("add -")+keyCtrlC), 120, 30))
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	shown := runCLI(t, root, "show", "fx-6", "--fields", "card")
	if shown.code != 0 || !strings.Contains(shown.out, "fx-6  -  [") {
		t.Errorf("add - filed no card titled -: %d %s%s", shown.code, shown.out, shown.errw)
	}
}

// containsAll reports whether every one of want stands in got.
func containsAll(got []string, want ...string) bool {
	for _, one := range want {
		if !slices.Contains(got, one) {
			return false
		}
	}
	return true
}
