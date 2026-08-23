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

// globalFlags are the flags that belong to the invocation rather than to any
// one command, in the order the help block prints them.
var globalFlags = []struct {
	// name is the flag's key in the catalog and its own spelling.
	name string
	// usage is the flag with its value placeholder.
	usage string
	// value is the placeholder a valued flag shows, empty on a marker. It is
	// declared rather than recovered from the usage string, since looking for
	// an angle bracket there would read a summary carrying one as a value.
	value string
	// marker says the flag carries no value, which is what the argument
	// parser reads when it derives the flags it accepts.
	marker bool
}{
	{name: "workbench", usage: "--workbench <dir>", value: "dir"},
	{name: "json", usage: "--json", marker: true},
	{name: "quiet", usage: "--quiet", marker: true},
	{name: "lang", usage: "--lang <tag>", value: "tag"},
	{name: "actor", usage: "--actor <name>", value: "name"},
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
	// hasCeiling caps the syntax column at half the window: the top-level
	// listing is the one table this tool draws with values wide enough to
	// swallow the whole line on their own (check's flags run past ninety
	// display columns), and no other table opts into this. wrapTail is the
	// same opt-in the arguments table already uses, so a long summary wraps
	// at the right edge rather than running past it into whatever the
	// terminal does with the overrun.
	list := table{indent: 2, columns: s.columns("commands", "command", "what"), labels: labelInTheStack, ceilingColumn: 0, hasCeiling: true, wrapTail: true, wrapOptions: true}
	for _, group := range groups {
		opening := true
		for _, c := range commands {
			if c.group != group {
				continue
			}
			entry := tableRow{fields: []string{verb.Usage(c.name), s.r.T("cmd." + c.name + ".summary")}}
			if opening {
				entry.section = s.r.T("help.group." + group)
				opening = false
			}
			list.rows = append(list.rows, entry)
		}
	}
	for _, line := range s.tableLines(list) {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + s.r.T("help.flags") + "\n")
	flags := table{indent: 2, columns: s.columns("flags", "option", "what")}
	for _, flag := range globalFlags {
		flags.rows = append(flags.rows, tableRow{fields: []string{flag.usage, s.r.T("flag." + flag.name + ".summary")}})
	}
	for _, line := range s.tableLines(flags) {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + s.r.T("help.environment") + "\n")
	b.WriteString("\n" + s.r.T("help.exitcodes") + "\n")
	b.WriteString("\n" + s.r.T("help.footer") + "\n")
	b.WriteString(s.r.T("help.reading") + "\n")
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
	// The syntax line is broken on option boundaries when the syntax itself
	// runs past the window, indented under itself, for the same reason the
	// arguments table breaks its last column: a command declaring enough
	// flags draws a line no eighty-column terminal can hold, and every line
	// of this page is held to the window. A syntax that fits the window is
	// written whole, the same way every help page draws one today.
	usage := verb.Usage(name)
	b.WriteString(s.renderSyntaxLine(usage, 2) + "\n\n")
	b.WriteString(s.r.T("cmd."+name+".summary") + "\n")
	if key := "cmd." + name + ".note"; s.r.Has(key) {
		b.WriteString("\n" + s.r.T(key) + "\n")
	}
	for _, line := range s.argumentLines(name) {
		b.WriteString(line + "\n")
	}
	if checks := verb.Checks(name); len(checks) > 0 {
		b.WriteString("\n" + s.r.T("help.refusals") + "\n")
		preconditions := table{indent: 2, columns: s.columns("help", "order", "check", "refusal")}
		for i, check := range checks {
			fields := []string{strconv.Itoa(i + 1), s.r.T(check.Key), check.Refusal}
			preconditions.rows = append(preconditions.rows, tableRow{fields: fields})
		}
		for _, line := range s.tableLines(preconditions) {
			b.WriteString(line + "\n")
		}
	}
	for _, topic := range verb.Guides(name) {
		b.WriteString("\n" + s.r.T("help.guide", "topic", topic) + "\n")
	}
	b.WriteString("\n" + s.r.T("help.exitcodes") + "\n")
	return b.String()
}

// argumentLines are the section of a command's help that says what a reader
// may write: a heading, then one row per argument, spelled on the left exactly
// as the syntax line above spells it and explained on the right.
//
// A command declaring no argument gets no section at all, so the pages of
// status, states, whoami, workbenches, export and mcp are unchanged.
//
// The table is the one table in the tool that breaks its last column at the
// window, because an argument's meaning together with the values it accepts
// runs wider than any other cell the tool prints.
func (s *session) argumentLines(name string) []string {
	declared := verb.Params(name)
	if len(declared) == 0 {
		return nil
	}
	arguments := table{
		indent:      2,
		columns:     s.columns("arguments", "argument", "what"),
		wrapTail:    true,
		wrapOptions: true,
	}
	for _, param := range declared {
		row := tableRow{fields: []string{param.Token(), s.argumentMeaning(name, param)}}
		arguments.rows = append(arguments.rows, row)
	}
	lines := []string{"", s.r.T("help.arguments")}
	return append(lines, s.tableLines(arguments)...)
}

// argumentMeaning is what one argument's row says: the sentence written for it,
// with the values it accepts appended where it names a closed set that
// resolves.
//
// A set living in the reader's own workbench resolves only where a workbench
// opens from where the command was run. Every way that can fail is swallowed
// here and the sentence stands alone, because `dinah help ls` has to answer
// anywhere, and a help page that refuses because somebody's configured default
// moved is worse than the gap it would have closed.
func (s *session) argumentMeaning(command string, param verb.Param) string {
	summary := s.r.T(param.SummaryKey(command))
	values := s.vocabularyValues(command, param)
	if len(values) == 0 {
		return summary
	}
	return s.r.T("help.vocabulary", "summary", summary, "values", strings.Join(values, ", "))
}

// vocabularyValues resolves the closed set an argument accepts, or nothing
// where the argument declares no set or the set cannot be read from here.
func (s *session) vocabularyValues(command string, param verb.Param) []string {
	set, ok := verb.VocabularyFor(command, param.Name)
	if !ok {
		return nil
	}
	if set.Source == "" {
		return set.Values
	}
	resolve, ok := refusalListings[set.Source]
	if !ok {
		return nil
	}
	if set.Source == statesVocabulary && s.library == nil {
		if _, err := s.open(); err != nil {
			return nil
		}
	}
	return resolve(s)
}

// statesVocabulary is the one vocabulary source that lives in the reader's own
// workbench rather than in the binary, so it is the one that needs a workbench
// opened before it can answer.
const statesVocabulary = "states"
