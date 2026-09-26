package main

import (
	"io"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// The tables below declare what the terminal UI's command line does with
// flags, settings and keys. They build untagged, beside the command table's
// own terminal class, so the plain dinah build carries them and the
// correspondence check over them without linking any Charm code.

// terminalRefusedFlags names every flag the terminal UI's command line
// refuses although its command runs there, keyed by command, each mapped to
// the reason token its refusal carries.
//
// The check cannot see a flag or a command that waits: a new flag that
// blocks until something happens, or a command that never returns, can be
// classed line and every test still passes. Whoever adds one names it here.
var terminalRefusedFlags = map[string]map[string]string{
	"view":    {"watch": "watch"},
	"changes": {"wait": "wait"},
}

// terminalSessionFlags says, for every flag that belongs to the invocation
// rather than to a command, whether the terminal UI's command line passes
// it. A line runs on the workbench, in the language and as the identity the
// head resolved at start, so the three flags that would change one are
// refused.
var terminalSessionFlags = map[string]bool{
	"workbench": false, "lang": false, "actor": false,
	"json": true, "format": true, "quiet": true, "help": true, "version": true,
}

// terminalPinnedSettings are the configuration keys the terminal UI resolves
// once at start, which a config set typed at its command line writes for the
// next start without changing this one.
var terminalPinnedSettings = []string{"workbench", "actor", "lang", "glyphs"}

// terminalCardReads names every command that takes a card, an item or a
// reference as a positional and writes nothing, each mapped to why. Every
// other command taking one acts on a card and sets actsOnCard, so a new card
// verb declared outside the work group cannot be classed line unseen.
//
// The list is typed by hand. TestEveryCommandHasATerminalClass holds each
// entry to the HTTP head's route table where it can: an entry the pages serve
// must be served by GET alone, and an entry the pages do not serve must be
// named in terminalCardReadsUnrouted with the reason no route checks it.
var terminalCardReads = map[string]string{
	"show":         "reads a card",
	"get":          "reads one field",
	"instructions": "reads the instructions of a position",
	"path":         "prints a file path",
	"view":         "draws a view",
}

// terminalCardReadsUnrouted names the entries of terminalCardReads the HTTP
// head serves no route for, so the route table cannot say whether they
// write. Each is mapped to the reason, and a diff reviewer is the only guard
// on it.
var terminalCardReadsUnrouted = map[string]string{
	"get":  "the HTTP head holds it for dinah-612 and routes no read for it yet",
	"path": "it resolves a filesystem path for a shell, which the HTTP head serves no route for",
}

// terminalReservedKeys are the keys the terminal UI reads in browse and card
// mode, which a key binding may not take, each mapped to the card or the
// reason that holds it.
var terminalReservedKeys = map[string]string{
	"a": "dinah-603: advance or accept the card",
	"b": "dinah-603: send the card back",
	"c": "dinah-603: comment on the card",
	"h": "dinah-603: focus the lane to the left",
	"i": "dinah-623: open item mode",
	"j": "dinah-603: select the card below",
	"k": "dinah-603: select the card above",
	"l": "dinah-603: focus the lane to the right",
	"m": "dinah-603: open the move menu",
	"q": "dinah-603: quit",
	"r": "dinah-603: release the card",
	"t": "dinah-603: claim the card",
	"x": "dinah-623: open the actions menu",
	"C": "dinah-623: the changes read",
	"F": "dinah-623: the search read",
	"S": "dinah-623: the status read",
	"W": "dinah-623: the whoami read",
}

// lineStart is what the terminal UI's head resolved once when it started,
// which every line session hands on in place of resolving it again.
type lineStart struct {
	// values are the start values of the configuration keys in
	// terminalPinnedSettings, which a line session's configuration answers
	// through bench.Config.Pinned.
	values map[string]string
	// settings are the rows the bare config listing reports for the
	// workbench, the actor and the language, as the head resolved each and
	// named the rung that answered it.
	settings map[string]verb.SettingView
}

// pinStart records the start values a line session hands on. The head calls
// it once, after its first open has resolved the workbench, with the actor
// and language flags its own command line carried. It also makes the
// resolved workbench the top rung of discovery, benchFlag, with the rung that
// resolved it as its source, so every later open and every sweep reaches the
// same workbench and reports the same source.
func (s *session) pinStart(actorFlag, langFlag string) {
	values := make(map[string]string, len(terminalPinnedSettings))
	for _, key := range terminalPinnedSettings {
		values[key] = s.cfg.Get(key)
	}
	_, actorSource := bench.ResolveActorSource(actorFlag, s.agent.Harness, s.cfg)
	if s.actor == "" {
		actorSource = bench.SourceUnset
	}
	lang, langSource := bench.ResolveLangSource(langFlag, s.cfg)
	s.start = &lineStart{
		values: values,
		settings: map[string]verb.SettingView{
			"workbench": {Key: "workbench", Value: s.workbenchRoot, Source: s.workbenchSource},
			"actor":     {Key: "actor", Value: s.actor, Source: actorSource},
			"lang":      {Key: "lang", Value: lang, Source: langSource},
		},
	}
	s.benchFlag = s.workbenchRoot
	s.benchFlagSource = s.workbenchSource
}

// namedBench answers the workbench a person named with --workbench or
// DINAH_WORKBENCH, and which of the two named it. A terminal UI head and its
// lines answer neither: their benchFlag holds the workbench the head resolved
// at start, put there by pinStart so every line opens it, and a person at the
// command line cannot give either (--workbench is refused as dinah.usage and
// DINAH_WORKBENCH was read once, at start). init and a walk from --root read
// a named workbench as a conflict with what they were asked to do, so they
// ask here rather than reading benchFlag.
func (s *session) namedBench() (string, string) {
	if s.start != nil {
		return "", ""
	}
	return s.benchFlag, s.benchFlagSource
}

// lineSession is the session every command the terminal UI runs by words
// runs in, and the session Tab completion reads through. It is the one place
// the head's start values are handed on: the workbench as the top rung of
// discovery, the configuration keys of terminalPinnedSettings as an overlay
// on the configuration file, and the identity and renderer as copied fields.
// Nothing reached through it can resolve one of them afresh.
//
// The configuration file itself is read afresh, so an alias, an editor or a
// key binding set during the session takes effect at once. The session finds
// no terminal to colour or to take, and its standard input refuses every read
// as dinah.not-in-tui, so a value of - is refused on the path every command
// already reports a read error on, before the command writes anything. The
// lend builds its session here and then hands it the head's own terminal.
func (s *session) lineSession(out, errw io.Writer, width int, args []string) *session {
	cfg := bench.LoadConfig(s.home)
	var settings map[string]verb.SettingView
	if s.start != nil {
		cfg = cfg.Pinned(s.start.values)
		settings = s.start.settings
	}
	return &session{
		out:             out,
		errw:            errw,
		in:              refusedInput{},
		r:               s.r,
		home:            s.home,
		nativeHome:      s.nativeHome,
		cfg:             cfg,
		actor:           s.actor,
		agent:           s.agent,
		benchFlag:       s.benchFlag,
		benchFlagSource: s.benchFlagSource,
		cwd:             s.cwd,
		width:           width,
		rawWidth:        width,
		args:            args,
		start:           s.start,
		pinnedSettings:  settings,
		onOpen:          s.lineOnOpen,
		lineOnOpen:      s.lineOnOpen,
	}
}

// refusedInput is a line session's standard input: every read answers the
// refusal that standard input is the screen's keyboard.
type refusedInput struct{}

// Read refuses, so a command reading its argument from a pipe reports
// dinah.not-in-tui with the stdin reason and writes nothing.
func (refusedInput) Read([]byte) (int, error) {
	return 0, contract.RefuseWith(contract.NotInTUI, "-", map[string]string{"reason": "stdin"})
}
