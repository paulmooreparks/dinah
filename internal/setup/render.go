package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Facts are what setup already knows when it renders a recipe, which is the
// closed set of values a placeholder may name. A recipe never hard-codes any
// of them, so one recipe serves every project, every agent name and every
// model.
type Facts struct {
	// Harness is the manifest's harness, written as DINAH_HARNESS.
	Harness string
	// Agent is the actor name the harness's agents act under.
	Agent string
	// Tools is the MCP tool profile the server entry serves.
	Tools string
	// Provider is written as DINAH_PROVIDER.
	Provider string
	// Model is written as DINAH_MODEL, and is empty when none was named.
	Model string
	// Server is written as DINAH_SERVER, and is empty when none was named.
	Server string
	// Scope is project or user.
	Scope string
	// Base is the scope's base directory, absolute.
	Base string
	// Workbench is the resolved workbench's directory at project scope, and
	// empty at user scope.
	Workbench string
	// WorkbenchTitle is the resolved workbench's title at project scope, and
	// empty at user scope.
	WorkbenchTitle string
	// Recipe is the recipe's own name.
	Recipe string
}

// placeholderNames is the closed set of names a placeholder may carry, in the
// order the specification lists them.
var placeholderNames = []string{
	"harness", "agent", "tools", "provider", "model", "server",
	"scope", "base", "workbench", "workbench_title", "recipe",
}

// tomlSuffix is the one suffix a placeholder may carry, which renders the fact
// as a complete TOML basic string.
const tomlSuffix = "|toml"

// value returns the fact a placeholder name stands for.
func (f Facts) value(name string) string {
	switch name {
	case "harness":
		return f.Harness
	case "agent":
		return f.Agent
	case "tools":
		return f.Tools
	case "provider":
		return f.Provider
	case "model":
		return f.Model
	case "server":
		return f.Server
	case "scope":
		return f.Scope
	case "base":
		return f.Base
	case "workbench":
		return f.Workbench
	case "workbench_title":
		return f.WorkbenchTitle
	case "recipe":
		return f.Recipe
	}
	return ""
}

// knownPlaceholder reports whether a name is one a placeholder may carry.
func knownPlaceholder(name string) bool {
	for _, known := range placeholderNames {
		if known == name {
			return true
		}
	}
	return false
}

// placeholder is one placeholder found in a text: where it starts and ends,
// the fact it names, and whether it carries the TOML suffix.
type placeholder struct {
	start, end int
	name       string
	toml       bool
}

// scanPlaceholders finds every placeholder in a text and refuses the first
// `{{` that does not open a well-formed one. allowTOML says whether the text
// may carry the TOML suffix, which only prompt.md, remove.md and templates
// may.
//
// Every `{{` opens a placeholder, so a text cannot carry a literal pair of
// braces. The error quotes the text from the braces onward, cut short, so a
// recipe author can find the defect.
func scanPlaceholders(text string, allowTOML bool) ([]placeholder, error) {
	var found []placeholder
	for at := 0; ; {
		open := strings.Index(text[at:], "{{")
		if open < 0 {
			return found, nil
		}
		start := at + open
		rest := text[start+2:]
		closing := strings.Index(rest, "}}")
		if closing < 0 {
			return nil, fmt.Errorf("%s does not close a placeholder", quoteFragment(text[start:]))
		}
		inner := rest[:closing]
		name, suffixed := strings.CutSuffix(inner, tomlSuffix)
		if !knownPlaceholder(name) {
			return nil, fmt.Errorf("%s names no placeholder setup knows", quoteFragment(text[start:start+2+closing+2]))
		}
		if suffixed && !allowTOML {
			return nil, fmt.Errorf("%s carries %s, which only prompt.md, remove.md and a template may carry", quoteFragment(text[start:start+2+closing+2]), tomlSuffix)
		}
		end := start + 2 + closing + 2
		found = append(found, placeholder{start: start, end: end, name: name, toml: suffixed})
		at = end
	}
}

// quoteFragment quotes the first part of a text for an error message.
func quoteFragment(text string) string {
	const limit = 40
	if len(text) > limit {
		text = text[:limit]
	}
	return fmt.Sprintf("%q", text)
}

// renderText substitutes every placeholder in a text that scanPlaceholders
// has already accepted. A fact is written as it stands, and a placeholder
// carrying the TOML suffix is written as a TOML basic string.
func renderText(text string, facts Facts, allowTOML bool) (string, error) {
	found, err := scanPlaceholders(text, allowTOML)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	at := 0
	for _, p := range found {
		b.WriteString(text[at:p.start])
		fact := facts.value(p.name)
		if p.toml {
			fact = tomlBasicString(fact)
		}
		b.WriteString(fact)
		at = p.end
	}
	b.WriteString(text[at:])
	return b.String(), nil
}

// wholePlaceholder reports the fact a string names when the string is exactly
// one placeholder and nothing else, which is the one shape the omission rule
// reads.
func wholePlaceholder(text string) (string, bool) {
	inner, ok := strings.CutPrefix(text, "{{")
	if !ok {
		return "", false
	}
	name, ok := strings.CutSuffix(inner, "}}")
	if !ok || !knownPlaceholder(name) {
		return "", false
	}
	return name, true
}

// tomlBasicString renders a value as a TOML basic string, quotes included. It
// is the one place this package writes TOML, and nothing else duplicates it.
//
// The TOML 1.0.0 specification says a basic string may hold "any Unicode
// character ... except those that must be escaped: quotation mark, backslash,
// and the control characters other than tab". So a backslash is written as
// `\\`, a double quote as `\"`, and U+0000 to U+0008, U+000A to U+001F and
// U+007F as `\uXXXX` with upper-case hexadecimal digits. Every other
// character, the apostrophe and the tab included, is written as it stands.
func tomlBasicString(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// jvalue is one JSON value of a recipe's step, held with its members in the
// order the recipe wrote them, since Go's maps would lose that order and the
// order is what a person reading the written file sees.
type jvalue struct {
	// kind is 'o' for an object, 'a' for an array, 's' for a string and 'l'
	// for any other literal: a number, true, false or null.
	kind byte
	// text is a string's own value, or a literal's JSON spelling.
	text string
	// members are an object's members, in order.
	members []jmember
	// elements are an array's elements, in order.
	elements []*jvalue
}

// jmember is one member of an object value.
type jmember struct {
	name  string
	value *jvalue
}

// parseValue reads one JSON value into a jvalue, refusing trailing content.
func parseValue(data []byte) (*jvalue, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	value, err := readValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("the value is followed by more JSON")
	}
	return value, nil
}

// readValue reads the next value from a decoder.
func readValue(dec *json.Decoder) (*jvalue, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := token.(type) {
	case json.Delim:
		if t == '{' {
			return readObject(dec)
		}
		if t == '[' {
			return readArray(dec)
		}
		return nil, fmt.Errorf("unexpected %q", string(t))
	case string:
		return &jvalue{kind: 's', text: t}, nil
	case json.Number:
		return &jvalue{kind: 'l', text: t.String()}, nil
	case bool:
		if t {
			return &jvalue{kind: 'l', text: "true"}, nil
		}
		return &jvalue{kind: 'l', text: "false"}, nil
	case nil:
		return &jvalue{kind: 'l', text: "null"}, nil
	}
	return nil, fmt.Errorf("unexpected token %v", token)
}

// readObject reads an object's members after its opening brace.
func readObject(dec *json.Decoder) (*jvalue, error) {
	object := &jvalue{kind: 'o'}
	seen := map[string]bool{}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok {
			return nil, errors.New("an object member has no name")
		}
		if seen[name] {
			return nil, fmt.Errorf("the member %q appears twice", name)
		}
		seen[name] = true
		member, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		object.members = append(object.members, jmember{name: name, value: member})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return object, nil
}

// readArray reads an array's elements after its opening bracket.
func readArray(dec *json.Decoder) (*jvalue, error) {
	array := &jvalue{kind: 'a'}
	for dec.More() {
		element, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		array.elements = append(array.elements, element)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return array, nil
}

// checkValuePlaceholders refuses a malformed placeholder anywhere in a value's
// strings, member names excepted, since a member name is never rendered.
func checkValuePlaceholders(value *jvalue) error {
	switch value.kind {
	case 's':
		_, err := scanPlaceholders(value.text, false)
		return err
	case 'o':
		for _, member := range value.members {
			if err := checkValuePlaceholders(member.value); err != nil {
				return err
			}
		}
	case 'a':
		for _, element := range value.elements {
			if err := checkValuePlaceholders(element); err != nil {
				return err
			}
		}
	}
	return nil
}

// renderValue substitutes the facts into a value's strings and applies the
// omission rule: a string that is exactly one placeholder whose fact is empty
// removes the member or element holding it. It reports whether the value
// itself is to be omitted.
func renderValue(value *jvalue, facts Facts) (*jvalue, bool) {
	switch value.kind {
	case 's':
		if name, whole := wholePlaceholder(value.text); whole && facts.value(name) == "" {
			return nil, true
		}
		text, _ := renderText(value.text, facts, false)
		return &jvalue{kind: 's', text: text}, false
	case 'o':
		rendered := &jvalue{kind: 'o'}
		for _, member := range value.members {
			child, omit := renderValue(member.value, facts)
			if omit {
				continue
			}
			rendered.members = append(rendered.members, jmember{name: member.name, value: child})
		}
		return rendered, false
	case 'a':
		rendered := &jvalue{kind: 'a'}
		for _, element := range value.elements {
			child, omit := renderValue(element, facts)
			if omit {
				continue
			}
			rendered.elements = append(rendered.elements, child)
		}
		return rendered, false
	}
	return value, false
}

// compact writes a value as compact JSON, members in their own order.
func (v *jvalue) compact() []byte {
	var b bytes.Buffer
	v.writeCompact(&b)
	return b.Bytes()
}

// writeCompact appends a value's compact JSON to a buffer.
func (v *jvalue) writeCompact(b *bytes.Buffer) {
	switch v.kind {
	case 's':
		b.WriteString(jsonString(v.text))
	case 'l':
		b.WriteString(v.text)
	case 'o':
		b.WriteByte('{')
		for i, member := range v.members {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(jsonString(member.name))
			b.WriteByte(':')
			member.value.writeCompact(b)
		}
		b.WriteByte('}')
	case 'a':
		b.WriteByte('[')
		for i, element := range v.elements {
			if i > 0 {
				b.WriteByte(',')
			}
			element.writeCompact(b)
		}
		b.WriteByte(']')
	}
}

// jsonString writes a string as a JSON string. HTML characters are written as
// they stand, because the files setup writes are read by people and by
// harnesses rather than embedded in a page.
func jsonString(text string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(text)
	return strings.TrimSuffix(b.String(), "\n")
}

// indentJSON renders compact JSON with two-space indentation. The first line
// carries no prefix, since it continues the line a member's name stands on,
// and every later line carries the prefix, which is json.Indent's documented
// behaviour.
func indentJSON(compact []byte, prefix string) string {
	var b bytes.Buffer
	if err := json.Indent(&b, compact, prefix, "  "); err != nil {
		return string(compact)
	}
	return b.String()
}
