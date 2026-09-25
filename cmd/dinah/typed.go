package main

import (
	"reflect"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// TypedCommand is a command line a reader typed into the pages' command log,
// taken apart by the terminal's own parser into the shape answer.Build takes.
type TypedCommand struct {
	// Command is the command the line names, after any alias expanded.
	Command string
	// Arguments are the line's values keyed by verb.Param.Name: a string for
	// a valued parameter and true for a marker that was written. A parameter
	// the line left out is absent.
	Arguments map[string]any
	// Actor is what the line gave --actor, and empty when it gave none.
	Actor string
}

// parseTypedLine parses the words of a typed line with the terminal's own
// steps, in the terminal's order, and nothing else: a leading "dinah" is
// dropped so a line the log printed can be pasted back, the reader's aliases
// expand, the words are parsed as os.Args would be, every session flag but
// --actor is refused because the pages serve one workbench in one language and
// render their own answer, the command is looked up, the open-tail flags are
// resolved, an unread word and an undeclared flag are refused, and the
// arguments are read off the parsed words in the order the parameter table
// declares them.
func parseTypedLine(cfg *bench.Config, words []string) (TypedCommand, *contract.Refusal) {
	if len(words) > 0 && words[0] == "dinah" {
		words = words[1:]
	}
	expanded, err := expandAlias(words, cfg)
	if err != nil {
		return TypedCommand{}, asRefusal(err)
	}
	valued := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
	}
	parsed, err := parseArgs(expanded, valued)
	if err != nil {
		return TypedCommand{}, asRefusal(err)
	}
	for _, name := range sortedNames(sessionFlagNames) {
		if name != "actor" && parsed.has(name) {
			return TypedCommand{}, contract.Refuse(contract.Usage, "--"+name)
		}
	}
	if len(parsed.positional) == 0 {
		return TypedCommand{}, contract.Refuse(contract.Usage, "dinah")
	}
	name := parsed.positional[0]
	command, ok := lookup(name)
	if !ok {
		return TypedCommand{}, contract.Refuse(contract.UnknownVerb, name)
	}
	if resolvesOpenTail(command.name) {
		if refusal := resolveOpenTailFlags(parsed, command); refusal != nil {
			return TypedCommand{}, refusal
		}
	}
	if word := unreadWordIn(command, parsed.rest()); word != "" {
		return TypedCommand{}, contract.Refuse(contract.Usage, word)
	}
	if flag := undeclaredFlagOn(command, parsed); flag != "" {
		return TypedCommand{}, contract.Refuse(contract.Usage, "--"+flag)
	}
	s := &session{r: msg.For(bench.ResolveLang("", cfg)), command: command.name}
	arguments, refusal := argumentsFrom(s, command.name, parsed)
	if refusal != nil {
		return TypedCommand{}, refusal
	}
	return TypedCommand{Command: command.name, Arguments: arguments, Actor: parsed.value("actor")}, nil
}

// argumentsFrom reads a command's arguments off parsed words, one declared
// parameter at a time in declared order. A marker is present when it was
// written, a valued flag carries what it was given, a repeatable flag every
// occurrence in order, and a positional the next unread word; a positional the table
// also accepts as a flag takes the flag's value when the flag was written. A
// Rest positional takes the remaining words under freeText's rule, so two or
// more refuse as the terminal refuses them, and a Rest value of exactly "-" is
// refused, because the pages have no standard input to read it from. A value
// the line did not give is left out of the map.
func argumentsFrom(s *session, command string, parsed *arguments) (map[string]any, *contract.Refusal) {
	arguments := map[string]any{}
	words := parsed.rest()
	lead := []string{command}
	slot := 0
	for _, p := range verb.Params(command) {
		switch {
		case p.Marker:
			if parsed.has(p.Name) {
				arguments[p.Name] = true
			}
		case p.Flag && repeatable(p):
			if carried := parsed.values(p.Name); len(carried) > 0 {
				arguments[p.Name] = carried
			}
		case p.Flag:
			if parsed.has(p.Name) {
				arguments[p.Name] = parsed.value(p.Name)
			}
		case p.AlsoFlag && parsed.has(p.Name):
			arguments[p.Name] = parsed.value(p.Name)
		case p.Rest:
			remaining := words[min(slot, len(words)):]
			slot = len(words)
			if len(remaining) == 0 {
				continue
			}
			text, refusal := s.freeText(lead, remaining, slotLabel(command, p.Name))
			if refusal != nil {
				return nil, refusal
			}
			if text == "-" {
				return nil, contract.Refuse(contract.Usage, "-")
			}
			arguments[p.Name] = text
		default:
			if slot < len(words) {
				arguments[p.Name] = words[slot]
				lead = append(lead, words[slot])
				slot++
			}
		}
	}
	return arguments, nil
}

// repeatable reports whether a flag may be written several times, which is
// what a parameter bound to a list field of the request declares.
func repeatable(p verb.Param) bool {
	field, ok := reflect.TypeOf(verb.Request{}).FieldByName(p.Field)
	return ok && field.Type.Kind() == reflect.Slice
}

// slotLabel is the catalog key naming a free-text slot in the multiple-words
// refusal, which is the key the command's own run function hands freeText.
// A slot this table does not name is called the value, which is the key
// config set and set pass.
func slotLabel(command, param string) string {
	switch {
	case command == "comment" && param == "text":
		return "slot.comment"
	case command == "file" && param == "text":
		return "slot.item"
	case param == "phrase":
		return "slot.search"
	case param == "title", param == "reason", param == "query", param == "name":
		return "slot." + param
	}
	return "slot.value"
}

// asRefusal reads an error the parser or the alias expansion returned as the
// refusal it is. Every error either returns is a *contract.Refusal; any other
// is reported as a usage refusal naming nothing, rather than dropped.
func asRefusal(err error) *contract.Refusal {
	if refusal, ok := err.(*contract.Refusal); ok {
		return refusal
	}
	return contract.Refuse(contract.Usage, err.Error())
}
