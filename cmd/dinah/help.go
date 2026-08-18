package main

import (
	"strconv"
	"strings"

	"dinah/internal/verb"
)

// command is one entry of the surface: the name a caller types, the group it
// is listed under, its argument syntax and what it does.
//
// The syntax line is generated from the library's own argument list rather
// than written here, so the two heads project one definition. Syntax is not
// prose: command names, flag names and the placeholder words inside angle
// brackets are machine vocabulary the way a refusal name is, spelled one way
// in every language, and only the summary beside them reaches the reader
// through the catalog.
type command struct {
	// name is the word a caller types.
	name string
	// group is the heading the command is listed under.
	group string
	// run does the work and returns the process exit code.
	run func(*session, *arguments) int
	// bounded is how many of the command's own leading positional words are
	// checked against the vocabulary the command knows (a card reference, a
	// state name, a guide topic, or a path handed to the operating system).
	// A word occupying one of these positions that looks like a mistyped
	// flag is refused before the command's own run function ever sees it.
	// config is not declared here; it dispatches on its own first word and
	// runs the same check itself.
	bounded int
	// openTail says the words after bounded are free text the caller
	// composed on purpose (a title, a reason, a comment, a config value),
	// left untouched however they begin. False means the command declares no
	// open tail, so every word past bounded is refused, not only one that
	// looks like a mistyped flag: nothing about a bounded, no-open-tail
	// command reads or stores a word past what it declares, so any word
	// there, dash-led or plain, is unread and worth refusing.
	openTail bool
}

// The four groups of the surface, in the order the help block prints them.
const (
	groupWork  = "work"
	groupRead  = "read"
	groupBench = "workbench"
	groupServe = "serve"
)

// groups are the headings in print order.
var groups = []string{groupWork, groupRead, groupBench, groupServe}

// usageColumn is the column the summary of a command starts in, counted from
// the left margin of the help block. The two-space indent plus the padded
// usage string reaches it.
const usageColumn = 39

// flagColumn is the same measure for the global flag list, whose syntax is
// shorter than a command's. The widest entry is `--workbench <dir>` at
// seventeen runes, so the column sits three past it and every summary in the
// list still starts at the same place.
const flagColumn = 20

// checkColumn is the column a precondition's refusal name starts in within the
// per-command help, counted from the left margin of the block. The two-space
// indent, the three-column ordinal, and the padded check sentence reach it.
const checkColumn = 52

// globalFlags are the flags that belong to the invocation rather than to any
// one command, in the order the help block prints them.
var globalFlags = []struct {
	// name is the flag's key in the catalog and its own spelling.
	name string
	// usage is the flag with its value placeholder.
	usage string
}{
	{name: "workbench", usage: "--workbench <dir>"},
	{name: "json", usage: "--json"},
	{name: "quiet", usage: "--quiet"},
	{name: "lang", usage: "--lang <tag>"},
	{name: "actor", usage: "--actor <name>"},
}

// lookup finds a command by name.
func lookup(name string) (*command, bool) {
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
	}
	return nil, false
}

// helpBlock composes the whole surface, which is what `dinah` with no
// arguments prints. A command absent from this block does not ship.
func (s *session) helpBlock() string {
	var b strings.Builder
	b.WriteString(s.r.T("help.tagline") + "\n\n")
	b.WriteString(s.r.T("help.usage") + "\n")
	for _, group := range groups {
		b.WriteString("\n" + s.r.T("help.group."+group) + "\n")
		for _, c := range commands {
			if c.group != group {
				continue
			}
			b.WriteString(s.rowLine(row{
				indent: 2,
				cells:  []cell{{verb.Usage(c.name), usageColumn}},
				tail:   s.r.T("cmd." + c.name + ".summary"),
			}) + "\n")
		}
	}
	b.WriteString("\n" + s.r.T("help.flags") + "\n")
	for _, flag := range globalFlags {
		b.WriteString(s.rowLine(row{
			indent: 2,
			cells:  []cell{{flag.usage, flagColumn}},
			tail:   s.r.T("flag." + flag.name + ".summary"),
		}) + "\n")
	}
	b.WriteString("\n" + s.r.T("help.environment") + "\n")
	b.WriteString("\n" + s.r.T("help.exitcodes") + "\n")
	b.WriteString("\n" + s.r.T("help.footer") + "\n")
	return b.String()
}

// verbHelp composes the help of one command: what it takes, then its checks
// in the profile's order with each check's refusal name beside it.
//
// The list is generated from the verb definition rather than written by hand,
// so a profile revision that reorders a check changes the help text and the
// behaviour together or fails its test.
func (s *session) verbHelp(name string) string {
	if _, ok := lookup(name); !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString(verb.Usage(name) + "\n\n")
	b.WriteString(s.r.T("cmd."+name+".summary") + "\n")
	checks := verb.Checks(name)
	if len(checks) == 0 {
		b.WriteString("\n" + s.r.T("help.exitcodes") + "\n")
		return b.String()
	}
	b.WriteString("\n" + s.r.T("help.refusals") + "\n")
	for i, check := range checks {
		b.WriteString(s.rowLine(row{
			indent: 2,
			cells:  []cell{{strconv.Itoa(i + 1), 3}, {s.r.T(check.Key), checkColumn}},
			tail:   check.Refusal,
		}) + "\n")
	}
	b.WriteString("\n" + s.r.T("help.exitcodes") + "\n")
	return b.String()
}
