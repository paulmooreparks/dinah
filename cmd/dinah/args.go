package main

import (
	"strconv"
	"strings"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// valuedFlags are the flags that take a value. Every other flag the tool
// accepts is a marker, so an unknown flag is refused rather than swallowing
// the argument behind it.
//
// There is no --basis here on purpose. The library carries a basis on every
// mutating request and computes one automatically, but an explicit basis is
// the remote arbiter's, and the spec defers the flag to that era, so this head
// offers no way to write one.
var valuedFlags = []string{
	"workbench", "lang", "actor", "state", "expires", "kind",
	"from", "description", "slug", "operator",
}

// markerFlags are the flags that carry no value.
var markerFlags = []string{
	"json", "quiet", "ready", "override", "replace", "yes", "catalogs",
	"finish", "migrate-ordinals", "migrate-slugs", "migrate-states",
}

// sessionFlagNames are the five flags read directly off the parsed
// arguments at session build time, before any command is looked up. A
// caller may write one of these anywhere before the marker. dinah-100
// bounds every open-tail command's free text to one already-delimited argv
// word, so one of these flags can no longer occupy a position "inside" that
// free text at all; it is either its own argv token, recognized here, or it
// sits inside the free text's own quoting, where nothing examines it.
var sessionFlagNames = map[string]bool{
	"workbench": true, "lang": true, "actor": true, "json": true, "quiet": true,
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

// parseArgs takes a command line apart. A flag may appear anywhere, because
// the global flags belong to the invocation rather than to the command, and a
// reader who writes them last should not be told off for it.
//
// A single dash is a word rather than a flag, since it is how a command reads
// its argument from a pipe.
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
	markerSeen := false
	for i := 0; i < len(argv); i++ {
		word := argv[i]
		if markerSeen {
			parsed.positional = append(parsed.positional, word)
			continue
		}
		if word == "--" {
			markerSeen = true
			continue
		}
		if word == "-" || !strings.HasPrefix(word, "--") {
			parsed.positional = append(parsed.positional, word)
			continue
		}
		name, inline, joined := strings.Cut(strings.TrimPrefix(word, "--"), "=")
		if !known[name] {
			return nil, contract.RefuseWith(contract.Usage, word, map[string]string{"dashHint": "1"})
		}
		session := sessionFlagNames[name]
		if !valued[name] {
			parsed.flags[name] = ""
			if !session {
				parsed.domainCaptures = append(parsed.domainCaptures, domainCapture{
					name: name, tokens: []string{word}, posAt: len(parsed.positional), complete: true,
				})
			}
			continue
		}
		if joined {
			parsed.flags[name] = inline
			if !session {
				parsed.domainCaptures = append(parsed.domainCaptures, domainCapture{
					name: name, value: inline, tokens: []string{word}, posAt: len(parsed.positional), complete: true,
				})
			}
			continue
		}
		if i+1 >= len(argv) {
			// A session flag missing its value still refuses immediately,
			// exactly as before dinah-96. A domain flag missing its value is
			// recorded instead: whether this refuses at all now depends on
			// where it falls relative to an open-tail command's own
			// free-text boundary, which is not known until the command is
			// looked up.
			if session {
				return nil, contract.RefuseWith(contract.Usage, word, map[string]string{"dashHint": "1"})
			}
			parsed.domainCaptures = append(parsed.domainCaptures, domainCapture{
				name: name, tokens: []string{word}, posAt: len(parsed.positional), complete: false,
			})
			continue
		}
		i++
		parsed.flags[name] = argv[i]
		if !session {
			parsed.domainCaptures = append(parsed.domainCaptures, domainCapture{
				name: name, value: argv[i], tokens: []string{word, argv[i]}, posAt: len(parsed.positional), complete: true,
			})
		}
	}
	return parsed, nil
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
// mark as the reason a ready-made line cannot be offered. lead is the
// command line up to and including the bounded arguments, in the order a
// caller types them, used only to build the example in the no-quote case.
func freeText(lead []string, words []string, label string) (string, *contract.Refusal) {
	if len(words) <= 1 {
		return at(words, 0), nil
	}
	joined := strings.Join(words, " ")
	extra := map[string]string{
		"count": strconv.Itoa(len(words)),
		"label": label,
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
