package main

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/httphead"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// cardPositionals are the completers a positional parameter names when it
// takes a card, an item or a reference, which is what makes a command one
// that reaches a card whatever group it is listed under.
var cardPositionals = []string{
	verb.CompleteCard,
	verb.CompleteItem,
	verb.CompleteReference,
	verb.CompleteCardOrColumn,
}

// takesACard reports whether a command declares a positional parameter
// completing as a card, an item or a reference.
func takesACard(name string) bool {
	for _, param := range verb.Params(name) {
		if param.Flag {
			continue
		}
		if slices.Contains(cardPositionals, param.Complete) {
			return true
		}
	}
	return false
}

// TestEveryCommandHasATerminalClass is dinah-623/criteria/1, 29, 30 and 49:
// every entry of the command table says how the terminal UI supports it, and
// the declarations around the class agree with the class. It sweeps the
// command table, the flag and setting tables the command line reads, and the
// catalog's reason sentences, and states the size of each set it swept.
func TestEveryCommandHasATerminalClass(t *testing.T) {
	legal := map[terminalClass]bool{terminalDirect: true, terminalLine: true, terminalAbsent: true}
	counts := map[terminalClass]int{}
	actsOnCard, frequentReads, walked := 0, 0, 0
	for _, c := range commands {
		walked++
		if !legal[c.terminal] {
			t.Errorf("%s carries the terminal class %q, which is none of direct, line and absent, so nobody has said how the terminal UI reaches it", c.name, c.terminal)
			continue
		}
		counts[c.terminal]++
		if c.group == groupWork && !c.actsOnCard {
			t.Errorf("%s is in the work group and does not set actsOnCard, and every command in that group acts on a card", c.name)
		}
		if c.actsOnCard {
			actsOnCard++
			if c.terminal != terminalDirect {
				t.Errorf("%s acts on a card and is classed %s, and the operator ruled that every card verb is a direct act in the terminal UI", c.name, c.terminal)
			}
		}
		if c.frequentRead {
			frequentReads++
			if c.terminal != terminalDirect {
				t.Errorf("%s is a frequent read and is classed %s, and a frequent read has a key of its own", c.name, c.terminal)
			}
			if c.actsOnCard {
				t.Errorf("%s sets both frequentRead and actsOnCard, and a read writes nothing", c.name)
			}
		}
		if takesACard(c.name) && !c.actsOnCard {
			if _, read := terminalCardReads[c.name]; !read {
				t.Errorf("%s takes a card, an item or a reference as a positional and neither sets actsOnCard nor is named in terminalCardReads, so a card verb could be classed line unseen", c.name)
			}
		}
		if c.lendsTerminal && c.name != "edit" {
			t.Errorf("%s sets lendsTerminal, and edit is the one command that hands the terminal to another program", c.name)
		}
		reason := "refusal.dinah.not-in-tui." + c.name
		_, carried := msg.BaseEntry(reason)
		if c.terminal == terminalAbsent && !carried {
			t.Errorf("%s is absent and the base catalog carries no %s, so the command line has no reason to give", c.name, reason)
		}
	}
	checkNotInTUIReasons(t)
	checkRefusedFlags(t)
	checkSessionFlags(t)
	checkPinnedSettings(t)
	checkCardReads(t)

	exempted := len(verb.Commands()) - len(commandExemptions)
	if walked != len(commands) || len(commands) != exempted {
		t.Errorf("walked %d commands of a table of %d, and the library's commands less the exemptions are %d", walked, len(commands), exempted)
	}
	t.Logf("swept %d commands: %d direct, %d line, %d absent; %d act on a card, %d are frequent reads",
		walked, counts[terminalDirect], counts[terminalLine], counts[terminalAbsent], actsOnCard, frequentReads)
	for _, class := range []terminalClass{terminalDirect, terminalLine, terminalAbsent} {
		if counts[class] == 0 {
			t.Errorf("no command is classed %s, so this sweep read an empty set", class)
		}
	}
	if actsOnCard == 0 || frequentReads == 0 {
		t.Errorf("the sweep counted %d card verbs and %d frequent reads, and an empty count proves nothing", actsOnCard, frequentReads)
	}
}

// checkNotInTUIReasons fails on a reason sentence in the base catalog naming
// a command that is not absent, which is a reason the command line would
// never give.
func checkNotInTUIReasons(t *testing.T) {
	t.Helper()
	tokens := map[string]bool{"stdin": true, "next": true}
	for _, flags := range terminalRefusedFlags {
		for _, token := range flags {
			tokens[token] = true
		}
	}
	swept := 0
	for _, key := range msg.Keys() {
		token, ok := strings.CutPrefix(key, "refusal.dinah.not-in-tui.")
		if !ok {
			continue
		}
		swept++
		if tokens[token] {
			continue
		}
		c, known := lookup(token)
		if !known || c.terminal != terminalAbsent {
			t.Errorf("the base catalog carries %s, and %s is not a command classed absent, so the reason names a command the line runs", key, token)
		}
	}
	if swept == 0 {
		t.Error("the base catalog carries no reason sentence of dinah.not-in-tui, so this check read nothing")
	}
}

// checkRefusedFlags fails on a refused flag whose command is absent, which
// the line refuses whole, or whose command does not declare the flag.
func checkRefusedFlags(t *testing.T) {
	t.Helper()
	swept := 0
	for name, flags := range terminalRefusedFlags {
		c, known := lookup(name)
		if !known || c.terminal == terminalAbsent {
			t.Errorf("terminalRefusedFlags names %s, which is not a command the line runs", name)
			continue
		}
		for flag := range flags {
			swept++
			declared := false
			for _, param := range verb.Params(name) {
				if param.Flag && param.Name == flag {
					declared = true
				}
			}
			if !declared {
				t.Errorf("terminalRefusedFlags names --%s on %s, which %s does not declare", flag, name, name)
			}
		}
	}
	if swept == 0 {
		t.Error("terminalRefusedFlags names no flag, so this check read nothing")
	}
}

// checkSessionFlags fails when terminalSessionFlags and sessionFlagNames name
// different flags, in either direction.
func checkSessionFlags(t *testing.T) {
	t.Helper()
	for name := range sessionFlagNames {
		if _, ok := terminalSessionFlags[name]; !ok {
			t.Errorf("--%s is a session flag and terminalSessionFlags does not say whether the command line passes it", name)
		}
	}
	for name := range terminalSessionFlags {
		if !sessionFlagNames[name] {
			t.Errorf("terminalSessionFlags names --%s, which is not a session flag", name)
		}
	}
	if len(terminalSessionFlags) == 0 {
		t.Error("terminalSessionFlags is empty, so this check read nothing")
	}
}

// checkPinnedSettings fails on a known setting that is neither pinned nor
// the editor, and on a pinned setting the tool does not know.
func checkPinnedSettings(t *testing.T) {
	t.Helper()
	for _, key := range bench.ConfigKeys {
		if key != "editor" && !slices.Contains(terminalPinnedSettings, key) {
			t.Errorf("%s is a setting and terminalPinnedSettings does not pin it, so a line could resolve it afresh; pin it or say why it is not", key)
		}
	}
	for _, key := range terminalPinnedSettings {
		if !slices.Contains(bench.ConfigKeys, key) {
			t.Errorf("terminalPinnedSettings pins %s, which is not a setting the tool knows", key)
		}
	}
	if len(bench.ConfigKeys) == 0 {
		t.Error("the tool knows no setting at all, so this check read nothing")
	}
}

// checkCardReads fails on a terminalCardReads entry that acts on a card or
// takes no card, and holds each entry to the HTTP head's route table: an
// entry the pages route must be served on GET alone, since a route whose
// unsafe method performs it is a route that writes, and an entry the pages do
// not route must be named in terminalCardReadsUnrouted with the reason the
// route table cannot check it.
func checkCardReads(t *testing.T) {
	t.Helper()
	routed := httphead.RoutedCommands()
	acts := httphead.RoutedActs()
	checked, unrouted := 0, 0
	names := make([]string, 0, len(terminalCardReads))
	for name := range terminalCardReads {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		c, known := lookup(name)
		if !known {
			t.Errorf("terminalCardReads names %s, which is no command", name)
			continue
		}
		if c.actsOnCard {
			t.Errorf("terminalCardReads names %s, which acts on a card", name)
		}
		if !takesACard(name) {
			t.Errorf("terminalCardReads names %s, which declares no card, item or reference positional", name)
		}
		_, excused := terminalCardReadsUnrouted[name]
		switch {
		case slices.Contains(acts, name):
			t.Errorf("terminalCardReads names %s as a read, and the HTTP head routes it through an unsafe method, which is a write", name)
		case slices.Contains(routed, name):
			checked++
			if excused {
				t.Errorf("terminalCardReadsUnrouted excuses %s, which the HTTP head routes, so the route table can check it after all", name)
			}
		case excused:
			unrouted++
		default:
			t.Errorf("terminalCardReads names %s, which the HTTP head does not route, so nothing checks that it writes nothing; name it in terminalCardReadsUnrouted with the reason", name)
		}
	}
	for name := range terminalCardReadsUnrouted {
		if _, listed := terminalCardReads[name]; !listed {
			t.Errorf("terminalCardReadsUnrouted excuses %s, which terminalCardReads does not name", name)
		}
	}
	t.Logf("held %d card reads to the route table's GET reads; %d are unrouted and rest on their stated reason", checked, unrouted)
	if checked == 0 {
		t.Error("no card read was held to the route table, so this check read nothing")
	}
}
