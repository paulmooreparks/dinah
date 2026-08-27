package main

import (
	"sort"
	"strconv"
	"strings"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// valuedFlags are the flags that take a value, and markerFlags are the ones
// that carry no value. Every flag the tool accepts is in one list or the
// other, so an unknown flag is refused rather than swallowing the argument
// behind it.
//
// Both are derived from the parameter table and the global flags rather than
// written out here, so a flag a command declares is a flag the parser accepts
// and a flag no command declares reaches no reader. Writing them out twice is
// what let --description work on attach while appearing on no help page.
//
// There is no --basis in either list. The library carries a basis on every
// mutating request and computes one automatically, but an explicit basis is
// the remote arbiter's, and the spec defers the flag to that era, so this head
// offers no way to write one.
var valuedFlags, markerFlags = declaredFlags()

// declaredFlags reads every command's parameter list and the global flags, and
// returns the valued names and the marker names, each sorted so the two lists
// are stable whatever order the parameter map is walked in.
func declaredFlags() (valued, markers []string) {
	valuedSeen := map[string]bool{}
	markerSeen := map[string]bool{}
	for _, name := range verb.Commands() {
		for _, param := range verb.Params(name) {
			// A positional whose own row says it is also written --name takes
			// a value under that spelling, which is how ls, next and move
			// accept --state beside the bare word.
			if param.AlsoFlag || (param.Flag && !param.Marker) {
				valuedSeen[param.Name] = true
				continue
			}
			if param.Flag {
				markerSeen[param.Name] = true
			}
		}
	}
	for _, flag := range globalFlags {
		if flag.marker {
			markerSeen[flag.name] = true
			continue
		}
		valuedSeen[flag.name] = true
	}
	return sortedNames(valuedSeen), sortedNames(markerSeen)
}

// sortedNames returns a set's members in order.
func sortedNames(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// sessionFlagNames are the flags read directly off the parsed arguments at
// session build time, before any command is looked up. A caller may write one
// of these anywhere before the marker. help and version joined the original
// five in dinah-213, and they belong here for the same reason the others do:
// each is answered by the invocation itself rather than by a command, so no
// command's free-text zone may reclaim one as prose. dinah-100
// bounds every open-tail command's free text to one already-delimited argv
// word, so one of these flags can no longer occupy a position "inside" that
// free text at all; it is either its own argv token, recognized here, or it
// sits inside the free text's own quoting, where nothing examines it.
var sessionFlagNames = map[string]bool{
	"workbench": true, "lang": true, "actor": true, "json": true, "quiet": true,
	"help": true, "version": true,
}

// askedFor maps every spelling of "what does this do" and "what is this" onto
// the flag the head reads for it. A caller who wants the surface has already
// decided how to ask for it before the tool is ever run, from whatever tool
// taught them the habit, so the answer to all of these is the same page rather
// than a refusal telling them they asked wrong.
//
// The single-dash and slash spellings are here because nothing else in the
// tool would accept them: parseArgs reads a word with one leading dash as a
// positional, and looksLikeMistypedFlag then refuses it as a mistyped long
// flag. That refusal is right for `-workbench` and wrong for `-h`, and the
// difference is that these words have exactly one meaning wherever they are
// typed. `dinah --help` was the spelling that prompted this; it refused as an
// unknown option, which reads to a caller, and to an agent reading an exit
// code, as a tool that does not work.
//
// A spelling is recognized only while the flag scan is open, so the POSIX
// `--` marker shields all of them: a caller who means the literal text `-h`
// or the path `/?` writes it after `--`, which is the same escape every other
// flag-shaped word already uses.
//
// `/?` is the only slash spelling here. It is what a Windows caller types and
// it is not a path anyone has, so recognizing it costs nothing. `/h` and
// `/version` were tried and dropped: each is a legitimate absolute path, and a
// POSIX shell rewrites both before the tool ever sees them, so they would have
// bought a collision and no working spelling.
var askedFor = map[string]string{
	"--help":    "help",
	"-help":     "help",
	"-h":        "help",
	"-?":        "help",
	"--?":       "help",
	"/?":        "help",
	"--version": "version",
	"-version":  "version",
	"-V":        "version",
	"-v":        "version",
}

// domainCapture is one occurrence of a domain flag (a flag belonging to some
// command's own vocabulary, not one of the five session flags) that
// parseArgs recognized while scanning. It is recorded rather than applied
// outright, because parseArgs runs before any command is known and cannot
// yet tell whether this occurrence sits inside an open-tail command's own
// free text, where dinah-96's trailing-only rule may reject it.
type domainCapture struct {
	// name is the flag's name, without the leading dashes.
	name string
	// value is what the flag resolves to when it is applied: the value word
	// for a valued flag, empty for a marker.
	value string
	// tokens are the literal argv word(s) this capture consumed, in the
	// shape the caller actually typed them ("--kind", "external"; or
	// "--kind=external"; or "--yes"). Spliced back into positional verbatim
	// when the capture is rejected.
	tokens []string
	// posAt is how many positional words parseArgs had already collected,
	// including the command name, at the moment this capture was made. It
	// is the same "words already collected" count dinah-96's boundary rule
	// compares against.
	posAt int
	// complete is false only for a valued flag whose name was the last word
	// in argv, with nothing left to serve as its value. parseArgs no longer
	// refuses this immediately; whether it refuses at all depends on where
	// it falls relative to the command's own free-text boundary, decided
	// once the command is known.
	complete bool
}

// arguments are one command line taken apart: the words in order and the
// flags with whatever each carried.
type arguments struct {
	// positional are the words that are not flags, in order.
	positional []string
	// flags are the flags given, by name without the leading dashes.
	flags map[string]string
	// domainCaptures are the domain-flag occurrences parseArgs recorded
	// tentatively, in the order it encountered them. resolveOpenTailFlags
	// (main.go) corrects parsed.flags and parsed.positional against these
	// once the command is known.
	domainCaptures []domainCapture
}

// walkFlags is the walk parseArgs and scanLangFlag both run over argv. It
// recognizes the POSIX "--" marker, every askedFor spelling, and every flag
// in known, and for a valued flag it decides whether the following word is
// that flag's own value, all without knowing which command the caller named:
// valuedFlags and markerFlags are declared once across every command's
// parameter table (declaredFlags, above), so the walk is the same walk
// whatever command argv turns out to name. Splitting it out is what makes
// scanLangFlag agree with parseArgs on which word belongs to which flag,
// rather than a second reading of argv reaching its own answer.
//
// onPositional is called for every word the walk does not place as a flag:
// every word once the marker has been seen, and every word before it that
// carries no "--" prefix (or is the bare "-").
//
// visit is called once per recognized flag occurrence, in argv order: name
// is the flag's own name (or the value askedFor maps a spelling to), value
// is what a valued flag's occurrence carried (empty for a marker or an
// askedFor spelling), and complete is false only for a valued flag whose
// name was the last word in argv, with no word left to serve as its value.
// tokens is the literal argv word or words this occurrence consumed, in the
// order the caller wrote them.
//
// onUnknown is called once for each "--name" word the walk cannot place
// (any inline "=value" already split off, the same word contract.RefuseWith
// names today). Its return says whether the walk stops there. parseArgs
// stops, since an unrecognized flag is refused before anything past it is
// read. scanLangFlag does not, since reading past the word that fails to
// parse is the reason it exists. A word onUnknown lets pass is left as one
// consumed word: the walk was never told this name takes a value, so it has
// no ground to claim the next word as that value either, and the next word
// is read on its own terms by the following iteration.
func walkFlags(
	argv []string,
	valued, known map[string]bool,
	onPositional func(word string),
	visit func(name, value string, complete bool, tokens []string),
	onUnknown func(word string) bool,
) {
	markerSeen := false
	for i := 0; i < len(argv); i++ {
		word := argv[i]
		if markerSeen {
			onPositional(word)
			continue
		}
		if word == "--" {
			markerSeen = true
			continue
		}
		if asked, ok := askedFor[word]; ok {
			visit(asked, "", true, nil)
			continue
		}
		if word == "-" || !strings.HasPrefix(word, "--") {
			onPositional(word)
			continue
		}
		name, inline, joined := strings.Cut(strings.TrimPrefix(word, "--"), "=")
		if !known[name] {
			if onUnknown(word) {
				return
			}
			continue
		}
		if !valued[name] {
			visit(name, "", true, []string{word})
			continue
		}
		if joined {
			visit(name, inline, true, []string{word})
			continue
		}
		if i+1 >= len(argv) {
			visit(name, "", false, []string{word})
			continue
		}
		i++
		visit(name, argv[i], true, []string{word, argv[i]})
	}
}

// parseArgs takes a command line apart. A flag may appear anywhere, because
// the global flags belong to the invocation rather than to the command, and a
// reader who writes them last should not be told off for it.
//
// A single dash is a word rather than a flag, since it is how a command reads
// its argument from a pipe.
//
// A refusal comes back with whatever the scan had taken apart so far rather
// than with a nil pointer, so the caller resolves the language from the flags
// the reader actually wrote ahead of the offending word. No second reading of
// argv exists anywhere to disagree with this one about whether a given word is
// a flag at all.
//
// A bare "--" is the POSIX end-of-options marker. The first one seen is
// consumed rather than added to positional, and every word after it,
// including a second literal "--", is taken as positional text without being
// checked against known at all. Nothing about flags already parsed before the
// marker changes; the marker only closes the flag scan for what follows it.
func parseArgs(argv []string, valued map[string]bool) (*arguments, error) {
	parsed := &arguments{flags: map[string]string{}}
	known := map[string]bool{}
	for _, flag := range append(append([]string{}, valuedFlags...), markerFlags...) {
		known[flag] = true
	}
	var refusal *contract.Refusal
	walkFlags(argv, valued, known,
		func(word string) {
			parsed.positional = append(parsed.positional, word)
		},
		func(name, value string, complete bool, tokens []string) {
			// A session flag, help and version among them, is recorded as a
			// flag and never as a domainCapture: it belongs to the
			// invocation the way --json does, so no command's free-text zone
			// reclaims one.
			session := sessionFlagNames[name]
			if !complete {
				// A session flag missing its value still refuses
				// immediately, exactly as before dinah-96. A domain flag
				// missing its value is recorded instead: whether this
				// refuses at all now depends on where it falls relative to
				// an open-tail command's own free-text boundary, which is
				// not known until the command is looked up.
				if session {
					refusal = contract.RefuseWith(contract.Usage, tokens[0], map[string]string{"dashHint": "1"})
					return
				}
				parsed.domainCaptures = append(parsed.domainCaptures, domainCapture{
					name: name, tokens: tokens, posAt: len(parsed.positional), complete: false,
				})
				return
			}
			parsed.flags[name] = value
			if !session {
				parsed.domainCaptures = append(parsed.domainCaptures, domainCapture{
					name: name, value: value, tokens: tokens, posAt: len(parsed.positional), complete: true,
				})
			}
		},
		func(word string) bool {
			refusal = contract.RefuseWith(contract.Usage, word, map[string]string{"dashHint": "1"})
			return true
		},
	)
	if refusal != nil {
		return parsed, refusal
	}
	return parsed, nil
}

// scanLangFlag finds the value the caller gave --lang, walking the whole
// argument list through walkFlags rather than stopping at the first word
// parseArgs cannot place. DINAH_LANG, the user config and the OS locale are
// not attached to argv at all, so nothing about them depends on where a
// word sits; --lang was the one rung a scan that stops early could still
// silence. dinah-97 is the record of that: the same invocation answered in
// two languages depending on where the flag was typed.
//
// Sharing walkFlags with parseArgs, rather than a second pattern match
// against the literal word "--lang", is what keeps this scan honest about a
// word that belongs to somebody else: dinah move card1 --state --lang de
// sets --state's value to the literal text "--lang" and gives the caller no
// --lang at all, and walkFlags is what already knows --state takes a value
// and consumes the next word for it, wherever in argv --state falls,
// including after the word a failed parse stopped at.
//
// The scan stops at the same POSIX "--" marker parseArgs does. A --lang
// with no following word is incomplete and is not reported; the ladder
// falls through to its next rung exactly as it does when --lang is absent
// altogether. An incomplete flag can only be the last word in argv, so
// there is nothing after it the scan could have missed either way. An
// unrecognized "--word" does not stop the scan, since reading past it is
// the whole point, and walkFlags does not treat the word that follows it as
// anyone's value, since nothing declared the unrecognized name as taking
// one. The last complete --lang the scan finds wins, matching parseArgs's
// own last-value-wins rule for a repeated flag.
func scanLangFlag(argv []string) string {
	valued := map[string]bool{}
	known := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
		known[flag] = true
	}
	for _, flag := range markerFlags {
		known[flag] = true
	}
	value := ""
	walkFlags(argv, valued, known,
		func(string) {},
		func(name, v string, complete bool, tokens []string) {
			if complete && name == "lang" {
				value = v
			}
		},
		func(string) bool { return false },
	)
	return value
}

// looksLikeMistypedFlag reports whether a word has exactly one leading dash
// and is not the bare "-", which is the shape a mistyped long flag takes
// (a single dash where two were meant, or a short form the tool has never
// offered) rather than a real value a caller composed on purpose. The bare
// "-" is excluded because it already means read the argument from a pipe.
func looksLikeMistypedFlag(word string) bool {
	return strings.HasPrefix(word, "-") && word != "-" && !strings.HasPrefix(word, "--")
}

// has reports whether a flag was given.
func (a *arguments) has(name string) bool {
	_, ok := a.flags[name]
	return ok
}

// value returns what a flag carried, or the empty string.
func (a *arguments) value(name string) string {
	return a.flags[name]
}

// rest returns the words after the command name.
func (a *arguments) rest() []string {
	if len(a.positional) < 2 {
		return nil
	}
	return a.positional[1:]
}

// at returns the word at a position, or the empty string when the caller gave
// fewer words than that.
func at(words []string, index int) string {
	if index >= len(words) {
		return ""
	}
	return words[index]
}

// freeText resolves an open-tail command's free-text slot (add's title,
// block's reason, comment's text, config set's value) to a single word,
// which is dinah-100's rule: a caller composing several words quotes them
// into one. words is what remains of the command line past the slot's own
// bounded arguments. Zero or one word passes through unchanged, matching
// today's behavior exactly (dinah-100 decision D-6). Two or more refuses
// with dinah.multiple-words, naming the word count.
//
// Which shell is on the other end of the paste is undocumented and cannot be
// asked for, so the rebuilt example this refusal offers is only ever built
// for text with no quotation mark in it, where wrapping in double quotes is
// already correct verbatim in bash, cmd.exe and PowerShell alike and nothing
// needs escaping. The moment the free text itself contains a `"`, no single
// escaping convention reads back correctly in all three: a backslash escape
// round-trips in bash and cmd.exe but is not an escape character inside a
// PowerShell double-quoted string, so it strings a quote-and-backslash
// artifact into the result there instead of the caller's original text
// (dinah-100 cycle-3 review, verified by pasting into all three shells).
// Rather than hand back a command line that is wrong in whichever shell the
// reader is using, freeText refuses with dinah.multiple-words.quote-yourself
// and asks the caller to add the quoting themselves, naming the quotation
// mark as the reason a ready-made line cannot be offered.
//
// label is the catalog key naming the slot, which freeText renders here rather
// than at the call site, so the name reaches the reader in the language the
// rest of the sentence is in. lead is the
// command line up to and including the bounded arguments, in the order a
// caller types them, used only to build the example in the no-quote case.
func (s *session) freeText(lead []string, words []string, label string) (string, *contract.Refusal) {
	if len(words) <= 1 {
		return at(words, 0), nil
	}
	joined := strings.Join(words, " ")
	extra := map[string]string{
		"count": strconv.Itoa(len(words)),
		"label": s.r.T(label),
	}
	if strings.Contains(joined, "\"") {
		extra["quoteInText"] = "1"
	} else {
		extra["example"] = "dinah " + strings.Join(lead, " ") + " \"" + joined + "\""
	}
	return "", contract.RefuseWith(contract.MultipleWords, "", extra)
}

// freeTextBoundary reports where a command's open tail begins, counted as
// the number of positional words (the command name plus its bounded slots)
// that precede it, and whether this command has a free-text zone at all.
// config dispatches on its own first word rather than through the generic
// bounded/openTail table, and only ever gets a boundary when that word is
// "set"; config get and a bare config take no free text.
func freeTextBoundary(c *command, parsed *arguments) (int, bool) {
	if c.name == "config" {
		if len(parsed.positional) < 2 || parsed.positional[1] != "set" {
			return 0, false
		}
		return 3, true
	}
	if !c.openTail {
		return 0, false
	}
	return 1 + c.bounded, true
}

// resolveOpenTailFlags corrects the domain-flag captures parseArgs recorded
// tentatively, now that the command is known. Within an open-tail command's
// own free-text zone, a domain flag is kept as a flag only when it is that
// command's own declared flag and it forms a genuinely trailing run at the
// end of the free text; every other capture recorded in the zone, whatever
// its name or position, is spliced back into positional as literal text and
// removed from parsed.flags. A capture recorded before the zone, or for a
// command with no free-text zone at all, is left exactly as parseArgs
// applied it.
//
// The one case this can still refuse is a valued domain flag recorded with
// no value (parseArgs deferred that refusal here, since it could not yet
// tell whether the flag might fall inside a zone where dinah-96's rule lets
// it fall through to literal text instead): the refusal fires only when the
// capture sits outside a free-text zone, or there is no zone to fall into,
// matching the refusal parseArgs used to raise immediately.
func resolveOpenTailFlags(parsed *arguments, c *command) *contract.Refusal {
	boundary, applies := freeTextBoundary(c, parsed)
	if !applies {
		for _, cap := range parsed.domainCaptures {
			if !cap.complete {
				return contract.RefuseWith(contract.Usage, cap.tokens[0], map[string]string{"dashHint": "1"})
			}
		}
		return nil
	}
	for _, cap := range parsed.domainCaptures {
		if !cap.complete && cap.posAt < boundary {
			return contract.RefuseWith(contract.Usage, cap.tokens[0], map[string]string{"dashHint": "1"})
		}
	}

	ownFlags := map[string]bool{}
	for _, p := range verb.Params(c.name) {
		if p.Flag {
			ownFlags[p.Name] = true
		}
	}

	var zoneIdx []int
	for i, cap := range parsed.domainCaptures {
		if cap.posAt >= boundary {
			zoneIdx = append(zoneIdx, i)
		}
	}
	if len(zoneIdx) == 0 {
		return nil
	}

	// item is one entry of the free-text zone rebuilt in its original order:
	// either a plain positional word, or a recorded capture (capIndex into
	// parsed.domainCaptures; -1 for a plain word).
	type item struct {
		word     string
		capIndex int
	}
	tailStart := min(boundary, len(parsed.positional))
	var items []item
	j := tailStart
	for _, ci := range zoneIdx {
		for j < parsed.domainCaptures[ci].posAt {
			items = append(items, item{word: parsed.positional[j], capIndex: -1})
			j++
		}
		items = append(items, item{capIndex: ci})
	}
	for j < len(parsed.positional) {
		items = append(items, item{word: parsed.positional[j], capIndex: -1})
		j++
	}

	// Peel a genuinely trailing run of the command's own flag, working
	// backward from the end. The first item that is not a matching, complete
	// capture stops the run; everything at or before that point, capture or
	// plain word, is literal text.
	peeled := map[int]bool{}
	end := len(items)
	for end > 0 {
		ci := items[end-1].capIndex
		if ci < 0 {
			break
		}
		cap := parsed.domainCaptures[ci]
		if !cap.complete || !ownFlags[cap.name] {
			break
		}
		peeled[ci] = true
		end--
	}

	zoneNames := map[string]bool{}
	for _, ci := range zoneIdx {
		zoneNames[parsed.domainCaptures[ci].name] = true
	}
	for name := range zoneNames {
		delete(parsed.flags, name)
	}
	for i, cap := range parsed.domainCaptures {
		if !zoneNames[cap.name] {
			continue
		}
		if cap.posAt < boundary {
			if cap.complete {
				parsed.flags[cap.name] = cap.value
			}
			continue
		}
		if cap.complete && peeled[i] {
			parsed.flags[cap.name] = cap.value
		}
	}

	tail := make([]string, 0, len(items))
	for _, it := range items {
		if it.capIndex < 0 {
			tail = append(tail, it.word)
			continue
		}
		if peeled[it.capIndex] {
			continue
		}
		tail = append(tail, parsed.domainCaptures[it.capIndex].tokens...)
	}
	parsed.positional = append(append([]string{}, parsed.positional[:tailStart]...), tail...)
	return nil
}
