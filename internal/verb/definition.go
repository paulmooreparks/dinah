package verb

import "strings"

// Param is one argument of a command. The list of them is the single
// definition both heads project: the cli head composes its syntax line from
// it, and the mcp head generates its input schema from it, so the two
// surfaces cannot drift apart.
type Param struct {
	// Name is the argument's name, which is the member name on the machine
	// surface and the catalog key's last segment.
	Name string
	// Display is what the syntax line shows inside its brackets, when that
	// differs from the name because the argument accepts two shapes.
	Display string
	// Flag marks an argument given as --name rather than by position.
	Flag bool
	// Marker marks a flag that carries no value.
	Marker bool
	// Required marks an argument the command cannot run without.
	Required bool
	// Value is the placeholder a valued flag shows.
	Value string
	// Rest marks a positional argument that takes every remaining word,
	// which is what a prose reason or a title needs.
	Rest bool
}

// Type is the JSON schema type of a parameter on a machine surface.
func (p Param) Type() string {
	if p.Marker {
		return "boolean"
	}
	return "string"
}

// shown is what the syntax line prints for a parameter, without its brackets.
func (p Param) shown() string {
	name := p.Name
	if p.Display != "" {
		name = p.Display
	}
	if !p.Flag {
		return name
	}
	if p.Marker {
		return "--" + name
	}
	return "--" + name + " <" + p.Value + ">"
}

// params is every command's argument list.
var params = map[string][]Param{
	"add": {
		{Name: "title", Required: true, Rest: true},
		{Name: "state", Flag: true, Value: "state"},
	},
	Claim: {
		{Name: "card", Required: true},
		{Name: "expires", Flag: true, Value: "duration"},
	},
	Move: {
		{Name: "card", Required: true},
		{Name: "state", Required: true},
		{Name: "override", Flag: true, Marker: true},
	},
	Release: {{Name: "card", Required: true}},
	Block: {
		{Name: "card", Required: true},
		{Name: "reason", Required: true, Rest: true},
		{Name: "kind", Flag: true, Value: "kind"},
	},
	Unblock: {{Name: "card", Required: true}},
	"comment": {
		{Name: "card", Required: true},
		{Name: "text", Display: "text|-", Required: true, Rest: true},
	},
	"attach": {
		{Name: "ref", Required: true},
		{Name: "file", Required: true},
		{Name: "replace", Flag: true, Marker: true},
	},
	"archive": {{Name: "ref", Required: true}},
	"delete": {
		{Name: "ref", Required: true},
		{Name: "yes", Flag: true, Marker: true, Required: true},
	},
	"status": {},
	"states": {},
	"ls": {
		{Name: "state"},
		{Name: "ready", Flag: true, Marker: true},
	},
	"next":         {{Name: "state"}},
	"show":         {{Name: "card", Display: "card|path", Required: true}},
	"log":          {{Name: "card", Required: true}},
	"instructions": {{Name: "card", Display: "card|state", Required: true}},
	"guide":        {{Name: "topic"}},
	"init":         {{Name: "from", Flag: true, Value: "source"}},
	"export":       {},
	"extract":      {{Name: "dir", Required: true}},
	"path":         {{Name: "card", Display: "card|path", Required: true}},
	"edit":         {{Name: "card", Display: "card|path", Required: true}},
	"config": {
		{Name: "action", Display: "get|set", Required: true},
		{Name: "key", Required: true},
		{Name: "value"},
	},
	"fsck":   {},
	"whoami": {},
	"version": {
		{Name: "catalogs", Flag: true, Marker: true},
	},
	"mcp":  {},
	"help": {{Name: "command", Required: true}},
}

// Params returns a command's argument list.
func Params(name string) []Param {
	return params[name]
}

// Usage composes a command's syntax line from its argument list. A required
// argument is angle-bracketed and an optional one square-bracketed, which is
// the convention the ratified help block uses throughout.
func Usage(name string) string {
	parts := []string{name}
	for _, p := range params[name] {
		shown := p.shown()
		if p.Required {
			if p.Flag {
				parts = append(parts, shown)
				continue
			}
			parts = append(parts, "<"+shown+">")
			continue
		}
		if p.Flag {
			parts = append(parts, "["+shown+"]")
			continue
		}
		parts = append(parts, "["+shown+"]")
	}
	return strings.Join(parts, " ")
}

// Commands lists every command name the library defines, which is what a head
// enumerates when it projects the surface.
func Commands() []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	return names
}
