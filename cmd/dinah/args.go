package main

import (
	"strings"

	"dinah/internal/contract"
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

// arguments are one command line taken apart: the words in order and the
// flags with whatever each carried.
type arguments struct {
	// positional are the words that are not flags, in order.
	positional []string
	// flags are the flags given, by name without the leading dashes.
	flags map[string]string
}

// parseArgs takes a command line apart. A flag may appear anywhere, because
// the global flags belong to the invocation rather than to the command, and a
// reader who writes them last should not be told off for it.
//
// A single dash is a word rather than a flag, since it is how a command reads
// its argument from a pipe.
func parseArgs(argv []string, valued map[string]bool) (*arguments, error) {
	parsed := &arguments{flags: map[string]string{}}
	known := map[string]bool{}
	for _, flag := range append(append([]string{}, valuedFlags...), markerFlags...) {
		known[flag] = true
	}
	for i := 0; i < len(argv); i++ {
		word := argv[i]
		if word == "-" || !strings.HasPrefix(word, "--") {
			parsed.positional = append(parsed.positional, word)
			continue
		}
		name, inline, joined := strings.Cut(strings.TrimPrefix(word, "--"), "=")
		if !known[name] {
			return nil, contract.Refuse(contract.Usage, word)
		}
		if !valued[name] {
			parsed.flags[name] = ""
			continue
		}
		if joined {
			parsed.flags[name] = inline
			continue
		}
		if i+1 >= len(argv) {
			return nil, contract.Refuse(contract.Usage, word)
		}
		i++
		parsed.flags[name] = argv[i]
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
