package setup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// jnode is one value of a JSON file setup edits, located by byte offsets so
// that setup can change one span and leave every other byte as it found it.
type jnode struct {
	// kind is '{' for an object, '[' for an array, and 's' for any scalar.
	kind byte
	// start and end bound the value's own bytes, end exclusive.
	start, end int
	// members are an object's members in file order.
	members []*jnodeMember
	// twice names each member name an object carries more than once.
	twice map[string]bool
}

// jnodeMember is one member of an object in a file, with the offset its name
// starts at.
type jnodeMember struct {
	name      string
	nameStart int
	value     *jnode
}

// errDuplicate is the defect a member name carried twice raises when setup
// has to walk through or own that name.
var errDuplicate = errors.New("carries a member name twice")

// jsonScanner reads a file's values with encoding/json's Decoder.Token and
// Decoder.InputOffset, both documented, so the offsets it records are the
// decoder's own rather than a second parser's.
type jsonScanner struct {
	data []byte
	dec  *json.Decoder
}

// parseJSONFile reads a file that must hold exactly one JSON object.
func parseJSONFile(data []byte) (*jnode, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	scanner := &jsonScanner{data: data, dec: dec}
	root, err := scanner.value()
	if err != nil {
		return nil, fmt.Errorf("does not parse as JSON (%v)", err)
	}
	if root.kind != '{' {
		return nil, errors.New("does not hold a JSON object")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("holds more than one JSON value")
	}
	return root, nil
}

// skip moves past whitespace, a colon and a comma, which is what lies between
// the end of one token and the start of the next.
func (s *jsonScanner) skip(at int) int {
	for at < len(s.data) && strings.IndexByte(" \t\r\n,:", s.data[at]) >= 0 {
		at++
	}
	return at
}

// value reads the next value with its span.
func (s *jsonScanner) value() (*jnode, error) {
	start := s.skip(int(s.dec.InputOffset()))
	token, err := s.dec.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return &jnode{kind: 's', start: start, end: int(s.dec.InputOffset())}, nil
	}
	switch delim {
	case '{':
		return s.object(start)
	case '[':
		return s.array(start)
	}
	return nil, fmt.Errorf("unexpected %q", string(delim))
}

// object reads an object's members after its opening brace.
func (s *jsonScanner) object(start int) (*jnode, error) {
	node := &jnode{kind: '{', start: start, twice: map[string]bool{}}
	seen := map[string]bool{}
	for s.dec.More() {
		nameStart := s.skip(int(s.dec.InputOffset()))
		token, err := s.dec.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok {
			return nil, errors.New("an object member has no name")
		}
		if seen[name] {
			node.twice[name] = true
		}
		seen[name] = true
		child, err := s.value()
		if err != nil {
			return nil, err
		}
		node.members = append(node.members, &jnodeMember{name: name, nameStart: nameStart, value: child})
	}
	if _, err := s.dec.Token(); err != nil {
		return nil, err
	}
	node.end = int(s.dec.InputOffset())
	return node, nil
}

// array reads an array's elements after its opening bracket. Setup never
// edits inside an array, so the elements are read for their extent alone.
func (s *jsonScanner) array(start int) (*jnode, error) {
	for s.dec.More() {
		if _, err := s.value(); err != nil {
			return nil, err
		}
	}
	if _, err := s.dec.Token(); err != nil {
		return nil, err
	}
	return &jnode{kind: '[', start: start, end: int(s.dec.InputOffset())}, nil
}

// member finds a member of an object by name, refusing a name the object
// carries twice, since setup cannot tell which of the two it owns.
func (n *jnode) member(name string) (*jnodeMember, int, error) {
	if n.twice[name] {
		return nil, -1, errDuplicate
	}
	for i, m := range n.members {
		if m.name == name {
			return m, i, nil
		}
	}
	return nil, -1, nil
}

// splitPointer reads an RFC 6901 JSON Pointer into its reference tokens. The
// empty pointer names the whole document and has none.
func splitPointer(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("the pointer %q does not start with a slash", pointer)
	}
	var tokens []string
	for _, raw := range strings.Split(pointer[1:], "/") {
		if strings.Contains(strings.ReplaceAll(strings.ReplaceAll(raw, "~0", ""), "~1", ""), "~") {
			return nil, fmt.Errorf("the pointer %q carries an escape RFC 6901 does not define", pointer)
		}
		token := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		tokens = append(tokens, token)
	}
	return tokens, nil
}

// joinPointer composes a pointer from reference tokens.
func joinPointer(tokens []string) string {
	var b strings.Builder
	for _, token := range tokens {
		b.WriteByte('/')
		escaped := strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
		b.WriteString(escaped)
	}
	return b.String()
}

// walk follows reference tokens from the root through objects. It returns the
// deepest object reached and how many tokens it consumed, and refuses a value
// on the path that is not an object or a name carried twice.
func walk(root *jnode, tokens []string) (*jnode, int, error) {
	node := root
	for i, token := range tokens {
		child, _, err := node.member(token)
		if err != nil {
			return nil, 0, fmt.Errorf("%s at %s", errDuplicate.Error(), joinPointer(tokens[:i+1]))
		}
		if child == nil {
			return node, i, nil
		}
		if child.value.kind != '{' {
			return nil, 0, fmt.Errorf("holds a value that is not an object at %s", joinPointer(tokens[:i+1]))
		}
		node = child.value
	}
	return node, len(tokens), nil
}

// lineBreakOf is a file's line-ending convention, read off its first line
// break: CRLF when that break is one, and LF otherwise, including for a file
// that holds no line break at all.
func lineBreakOf(data []byte) string {
	at := bytes.IndexByte(data, '\n')
	if at > 0 && data[at-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// leadingWhitespace is the run of spaces and tabs at the start of the line an
// offset stands on.
func leadingWhitespace(data []byte, offset int) string {
	lineStart := bytes.LastIndexByte(data[:offset], '\n') + 1
	end := lineStart
	for end < len(data) && (data[end] == ' ' || data[end] == '\t') {
		end++
	}
	return string(data[lineStart:end])
}

// withLineBreak writes inserted text in the file's own line-ending
// convention, so a CRLF file does not gain LF lines.
func withLineBreak(text, lineBreak string) string {
	if lineBreak == "\n" {
		return text
	}
	return strings.ReplaceAll(text, "\n", lineBreak)
}

// splice replaces a span of a file's bytes.
func splice(data []byte, start, end int, text string) []byte {
	out := make([]byte, 0, len(data)-(end-start)+len(text))
	out = append(out, data[:start]...)
	out = append(out, text...)
	return append(out, data[end:]...)
}

// replaceMemberValue replaces a member's value, rendered with the leading
// whitespace of the line its name starts on as the indentation prefix.
func replaceMemberValue(data []byte, m *jnodeMember, compact []byte) []byte {
	prefix := leadingWhitespace(data, m.nameStart)
	text := withLineBreak(indentJSON(compact, prefix), lineBreakOf(data))
	return splice(data, m.value.start, m.value.end, text)
}

// addMember adds a member to an object, after the last member when it has
// one and inside the braces when it has none.
func addMember(data []byte, object *jnode, name string, compact []byte) []byte {
	lineBreak := lineBreakOf(data)
	if len(object.members) > 0 {
		last := object.members[len(object.members)-1]
		prefix := leadingWhitespace(data, last.nameStart)
		member := jsonString(name) + ": " + indentJSON(compact, prefix)
		text := "," + lineBreak + prefix + withLineBreak(member, lineBreak)
		return splice(data, last.value.end, last.value.end, text)
	}
	outer := leadingWhitespace(data, object.start)
	inner := outer + "  "
	member := jsonString(name) + ": " + indentJSON(compact, inner)
	text := "{" + lineBreak + inner + withLineBreak(member, lineBreak) + lineBreak + outer + "}"
	return splice(data, object.start, object.end, text)
}

// removeMember removes the member at an index. A member after the first goes
// with the separator in front of it; a first member followed by others goes
// up to the next member's name, which keeps that member's own leading
// whitespace; and a sole member goes from just after the brace.
func removeMember(data []byte, object *jnode, index int) []byte {
	m := object.members[index]
	switch {
	case index > 0:
		return splice(data, object.members[index-1].value.end, m.value.end, "")
	case len(object.members) > 1:
		return splice(data, m.nameStart, object.members[1].nameStart, "")
	}
	return splice(data, object.start+1, m.value.end, "")
}

// compactSpan compacts a value's bytes, which is the form a member's digest
// and every comparison of two values read.
func compactSpan(span []byte) []byte {
	var b bytes.Buffer
	if err := json.Compact(&b, span); err != nil {
		return span
	}
	return b.Bytes()
}

// digestOf is the ledger's digest of some bytes.
func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
