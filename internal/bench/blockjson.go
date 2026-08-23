package bench

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// The two functions in this file are the interchange's two halves for a
// frontmatter value whose shape is more than a scalar. blockValue reads one
// key's raw lines as the JSON value of the same shape, and renderBlock writes
// a JSON value back as the lines of that shape.
//
// They owe each other one invariant: reading back what renderBlock wrote gives
// the same JSON value. The renderer never writes a form the reader would read
// as something else, and it reports false instead, so the caller falls back to
// the one raw JSON line every unrecognized member has always travelled as. The
// invariant is what makes a second export byte-identical to the first, and
// each refusal below names the reader rule that forced it.
//
// No YAML parser stands here. The reader covers the shapes docs/design/format.md
// declares, reusing the splitting rules Seq and splitLevelEntry already state,
// and skips what it cannot read, which is the reader posture readLevels states
// for its own key.

// blockName matches a frontmatter key a reader can write and read back, which
// is the character class ParseAnchor's own key pattern admits. A JSON member
// named anything else has no frontmatter spelling, so renderBlock refuses it.
var blockName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// blockChild splits a child line of a nested block into its leading spaces and
// its text. A line indented with a tab matches nothing, because the pattern
// admits spaces alone and then demands a non-space character, and a tab is not
// indentation the format promises anywhere.
var blockChild = regexp.MustCompile(`^( *)(\S.*)$`)

// blockEntry matches one dashed entry of a block sequence, once the line's own
// indentation has been cut away.
var blockEntry = regexp.MustCompile(`^-\s*(.*)$`)

// blockMember matches one `name:` line of a nested mapping, once the line's own
// indentation has been cut away.
var blockMember = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_.-]*):(.*)$`)

// jsonMember is one member of a JSON object, kept beside its siblings in the
// order the object carries rather than in a Go map, because a map sorts its
// keys and a mapping's declared order is what the format asks a reader to
// preserve.
type jsonMember struct {
	name  string
	value json.RawMessage
}

// blockLine is one readable child line of a nested block: the count of leading
// spaces, and the text after them.
type blockLine struct {
	indent int
	text   string
}

// blockValue reads one frontmatter key's raw lines as the JSON value of the
// same shape. A nested mapping reads as a JSON object, a sequence as a JSON
// array, and a scalar as a JSON string.
//
// A key the header does not carry, and a block whose children read as nothing
// at all, both read as the empty string, which is the answer this build gives
// every structured value today.
func blockValue(fm *Frontmatter, key string) json.RawMessage {
	lines := fm.Raw(key)
	if len(lines) == 0 {
		return mustMarshal("")
	}
	if len(lines) == 1 {
		return scalarValue(fm.Value(key))
	}
	return childValue(readChildren(lines[1:]))
}

// scalarValue reads the text written after a key's colon, already trimmed and
// unquoted the way Frontmatter.Value hands it over, as a JSON value.
//
// Three rules, in this order. Text that parses as JSON travels as that JSON,
// which is the answer a preserved member has always got and what keeps the one
// raw line an unrenderable member falls back to stable whatever value that
// member carries. Text
// opening with a bracket and closing with one is a flow sequence and travels
// as an array of strings, split on commas by Seq's own inline rule. Anything
// else travels as a JSON string, which is what a hand-written frontmatter
// value is.
func scalarValue(text string) json.RawMessage {
	if json.Valid([]byte(text)) {
		return json.RawMessage(text)
	}
	if !strings.HasPrefix(text, "[") || !strings.HasSuffix(text, "]") {
		return mustMarshal(text)
	}
	items := []string{}
	for _, raw := range strings.Split(strings.Trim(text, "[]"), ",") {
		if item := unquote(strings.TrimSpace(raw)); item != "" {
			items = append(items, item)
		}
	}
	return mustMarshal(items)
}

// readChildren keeps the lines beneath a key that a reader can read at all,
// which is every line that is neither blank, nor an annotation comment, nor
// indented with a tab.
func readChildren(lines []string) []blockLine {
	var children []blockLine
	for _, line := range lines {
		m := blockChild.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if strings.HasPrefix(m[2], "#") {
			continue
		}
		children = append(children, blockLine{indent: len(m[1]), text: m[2]})
	}
	return children
}

// childValue reads the children of a multi-line block as a JSON value. The
// children at the shallowest indent present decide the shape: dashed entries
// make an array, `name:` lines make an object, and a child of neither shape is
// skipped while its siblings still read.
//
// The shape is settled by the first child that carries one, so a block mixing
// the two spellings reads as the shape it opens with rather than as nothing.
// The renderer never writes such a block, so nothing this file wrote takes
// that path.
func childValue(children []blockLine) json.RawMessage {
	if len(children) == 0 {
		return mustMarshal("")
	}
	shallowest := children[0].indent
	for _, child := range children {
		if child.indent < shallowest {
			shallowest = child.indent
		}
	}
	if value, read := arrayFromChildren(children, shallowest); read {
		return value
	}
	if value, read := objectFromChildren(children, shallowest); read {
		return value
	}
	return mustMarshal("")
}

// arrayFromChildren reads the block as a JSON array, and reports whether the
// first child at the shallowest indent was a dashed entry at all. A dashed
// entry always reads as text, so a block of dashed digits gives an array of
// strings.
func arrayFromChildren(children []blockLine, shallowest int) (json.RawMessage, bool) {
	var entries []json.RawMessage
	opened := false
	for _, child := range children {
		if child.indent != shallowest {
			continue
		}
		m := blockEntry.FindStringSubmatch(child.text)
		if m == nil {
			if !opened && blockMember.MatchString(child.text) {
				return nil, false
			}
			continue
		}
		opened = true
		entries = append(entries, dashedValue(m[1]))
	}
	if !opened {
		return nil, false
	}
	return jsonArray(entries), true
}

// dashedValue reads one dashed entry. An entry splitting at its first colon is
// a JSON object of one member, name to hint, on the cut splitLevelEntry makes.
// An entry with no colon is a JSON string, with a trailing annotation comment
// stripped as Seq strips it.
func dashedValue(entry string) json.RawMessage {
	name, hint := splitLevelEntry(entry)
	if _, _, mapped := strings.Cut(entry, ":"); mapped {
		return jsonObject([]jsonMember{{name: name, value: mustMarshal(hint)}})
	}
	return mustMarshal(name)
}

// objectFromChildren reads the block as a JSON object, whose members are
// written in the order the file declares them, and reports whether the block
// opened with a member line at all. Each member's value is read by the same
// rules, applied to its own scalar and to the lines indented deeper than it.
// A duplicated name keeps its first occurrence, as a duplicated level name
// does.
func objectFromChildren(children []blockLine, shallowest int) (json.RawMessage, bool) {
	var members []jsonMember
	seen := map[string]bool{}
	opened := false
	for i, child := range children {
		if child.indent != shallowest {
			continue
		}
		m := blockMember.FindStringSubmatch(child.text)
		if m == nil {
			continue
		}
		opened = true
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		var deeper []blockLine
		for _, follower := range children[i+1:] {
			if follower.indent <= shallowest {
				break
			}
			deeper = append(deeper, follower)
		}
		if len(deeper) > 0 {
			members = append(members, jsonMember{name: m[1], value: childValue(deeper)})
			continue
		}
		scalar := unquote(strings.TrimSpace(m[2]))
		members = append(members, jsonMember{name: m[1], value: scalarValue(scalar)})
	}
	if !opened {
		return nil, false
	}
	return jsonObject(members), true
}

// blockPad is the leading spaces of a line at the given indent. It is built a
// space at a time rather than through the standard library's repeat, which the
// row-layout guard reserves for the one renderer that lays out columns, and an
// indent here is two spaces per nesting level rather than a padded field.
func blockPad(indent int) string {
	var pad strings.Builder
	for i := 0; i < indent; i++ {
		pad.WriteByte(' ')
	}
	return pad.String()
}

// renderBlock renders a JSON value as the frontmatter lines of the same shape,
// beneath a key at the given indent, and reports whether it could. A caller
// given false writes the one raw JSON line instead, which loses nothing.
func renderBlock(key string, indent int, raw json.RawMessage) ([]string, bool) {
	if !blockName.MatchString(key) {
		return nil, false
	}
	pad := blockPad(indent)
	switch jsonShape(raw) {
	case '{':
		members, read := jsonMembers(raw)
		// An empty object has no block spelling: the key's own line alone
		// reads back as the empty string rather than as an object.
		if !read || len(members) == 0 {
			return nil, false
		}
		lines := []string{pad + key + ":"}
		for _, member := range members {
			// A member this cannot render makes the whole block
			// unrenderable, so a block is never written half complete.
			rendered, ok := renderBlock(member.name, indent+2, member.value)
			if !ok {
				return nil, false
			}
			lines = append(lines, rendered...)
		}
		return lines, true
	case '[':
		return renderArray(key, indent, raw)
	default:
		text, ok := renderScalar(raw)
		if !ok {
			return nil, false
		}
		return []string{pad + key + ": " + text}, true
	}
}

// renderArray renders a JSON array, taking the flow form on the key's own line
// when reading that line back gives the same array, and the dashed form
// otherwise.
//
// The flow test is the guard the scalar rule has, applied to a list. An array
// of the strings "1" and "2" would print as a line scalarValue reads back as
// numbers, so it goes to the dashed form, where an entry is always text. An
// array of JSON numbers prints flow and reads back as itself.
func renderArray(key string, indent int, raw json.RawMessage) ([]string, bool) {
	entries, read := jsonEntries(raw)
	if !read {
		return nil, false
	}
	pad := blockPad(indent)
	if texts, safe := flowTexts(entries); safe {
		flow := "[" + strings.Join(texts, ", ") + "]"
		if sameJSON(scalarValue(flow), raw) {
			return []string{pad + key + ": " + flow}, true
		}
	}
	if len(entries) == 0 {
		return nil, false
	}
	lines := []string{pad + key + ":"}
	child := blockPad(indent + 2)
	for _, entry := range entries {
		text, ok := dashedText(entry)
		if !ok || !sameJSON(dashedValue(text), entry) {
			return nil, false
		}
		lines = append(lines, child+"- "+text)
	}
	return lines, true
}

// flowTexts renders each entry as it would print inside a flow sequence, and
// reports whether every one of them prints safely on one line. A string prints
// safely when it is non-empty, equals its own trim, and holds none of the
// characters the line's own syntax spends. A number, true, false or null
// prints as the literal the JSON carries, copied rather than reformatted, so
// 1.0 stays 1.0.
func flowTexts(entries []json.RawMessage) ([]string, bool) {
	texts := make([]string, 0, len(entries))
	for _, entry := range entries {
		literal := strings.TrimSpace(string(entry))
		if strings.HasPrefix(literal, `"`) {
			var value string
			if err := json.Unmarshal(entry, &value); err != nil {
				return nil, false
			}
			if value == "" || value != strings.TrimSpace(value) {
				return nil, false
			}
			if strings.ContainsAny(value, ",[]{}#:\"'") {
				return nil, false
			}
			texts = append(texts, value)
			continue
		}
		if literal != "true" && literal != "false" && literal != "null" && !jsonNumber(literal) {
			return nil, false
		}
		texts = append(texts, literal)
	}
	return texts, true
}

// dashedText renders one entry as the text of a dashed line. A string renders
// as a bare name and an object of one member whose value is a string renders
// as a name mapping to a hint. An entry of any other shape has no dashed
// spelling, which is what sends an array mixing numbers with hinted entries to
// the raw JSON line.
func dashedText(entry json.RawMessage) (string, bool) {
	if strings.HasPrefix(strings.TrimSpace(string(entry)), `"`) {
		var value string
		if err := json.Unmarshal(entry, &value); err != nil {
			return "", false
		}
		return quote(value), true
	}
	members, read := jsonMembers(entry)
	if !read || len(members) != 1 {
		return "", false
	}
	var hint string
	if err := json.Unmarshal(members[0].value, &hint); err != nil {
		return "", false
	}
	if hint == "" {
		return quote(members[0].name) + ":", true
	}
	return quote(members[0].name) + ": " + quote(hint), true
}

// renderScalar renders a JSON string, number or boolean as the text after a
// key's colon, and reports whether it could.
//
// A string is refused when reading the rendered scalar back would give
// something other than that string, which is a string that parses as JSON and
// a string that opens with a bracket and closes with one. Either would drift
// in type on every hop through the interchange. Anything that is not a string,
// a number or a boolean has no scalar spelling.
func renderScalar(raw json.RawMessage) (string, bool) {
	literal := strings.TrimSpace(string(raw))
	if strings.HasPrefix(literal, `"`) {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", false
		}
		text := quote(value)
		if !sameJSON(scalarValue(unquote(strings.TrimSpace(text))), raw) {
			return "", false
		}
		return text, true
	}
	if literal == "true" || literal == "false" || jsonNumber(literal) {
		return literal, true
	}
	return "", false
}

// jsonShape reports the first character of a JSON value, which is what tells
// an object from an array from a scalar without decoding it.
func jsonShape(raw json.RawMessage) byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return 0
	}
	return trimmed[0]
}

// jsonNumber reports whether a literal is a JSON number.
func jsonNumber(literal string) bool {
	var number json.Number
	return json.Unmarshal([]byte(literal), &number) == nil
}

// jsonMembers decodes a JSON object into its members in the order the text
// carries them, which json.Unmarshal into a Go map cannot do. A duplicated
// name keeps its first occurrence.
func jsonMembers(raw json.RawMessage) ([]jsonMember, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Token(json.Delim('{')) {
		return nil, false
	}
	var members []jsonMember
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, false
		}
		name, isName := key.(string)
		if !isName {
			return nil, false
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, false
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		members = append(members, jsonMember{name: name, value: value})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, false
	}
	return members, true
}

// jsonEntries decodes a JSON array into its entries, each kept as the raw text
// it was written with so a number's own spelling survives.
func jsonEntries(raw json.RawMessage) ([]json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Token(json.Delim('[')) {
		return nil, false
	}
	var entries []json.RawMessage
	for decoder.More() {
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, false
		}
		entries = append(entries, value)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, false
	}
	return entries, true
}

// jsonObject encodes members in the order they stand, which json.Marshal of a
// Go map would not do.
func jsonObject(members []jsonMember) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, member := range members {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(mustMarshal(member.name))
		b.WriteByte(':')
		b.Write(member.value)
	}
	b.WriteByte('}')
	return b.Bytes()
}

// jsonArray encodes entries in the order they stand.
func jsonArray(entries []json.RawMessage) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, entry := range entries {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(entry)
	}
	b.WriteByte(']')
	return b.Bytes()
}

// sameJSON reports whether two JSON values are the same value, comparing their
// compact forms so that the spacing a writer chose carries no weight and a
// number's own spelling still does.
func sameJSON(left, right json.RawMessage) bool {
	var a, b bytes.Buffer
	if json.Compact(&a, left) != nil {
		return false
	}
	if json.Compact(&b, right) != nil {
		return false
	}
	return a.String() == b.String()
}
