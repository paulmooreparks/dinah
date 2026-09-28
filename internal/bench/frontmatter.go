// Package bench implements Dinah's on-disk format: the entity shape, the
// anchor files, the append-only journals, the card lock, bench discovery and
// the user base. It knows nothing about verbs, heads or catalogs, so the
// contract's rules live above it and the filesystem's rules live here.
package bench

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Frontmatter is the header of an anchor file, held as the raw lines of each
// top-level key in the order they were read.
//
// Holding raw lines rather than decoded values is what makes CORE-CARD-9 and
// CORE-LAYER-2 free: a key the tool has never heard of survives a read and a
// write untouched, whatever shape its value has, because nothing here decoded
// it in the first place. Only the keys a verb actually changes are rewritten.
//
// A Frontmatter is copy-on-write. Clone answers one that shares this one's
// keys and block until either is written, and every method that writes calls
// own first, so a header a resident snapshot hands to many requests is never
// changed by any of them.
type Frontmatter struct {
	keys   []string
	block  map[string][]string
	shared bool // keys and block belong to another Frontmatter too
}

// Clone answers a Frontmatter that reads as f does and that its holder may
// change without changing f. It copies nothing until one of them is written.
//
// A header a memo holds is marked shared before any request can reach it, so
// Clone writes nothing to such a header and concurrent clones of it only read
// it.
func (f *Frontmatter) Clone() *Frontmatter {
	if !f.shared {
		f.shared = true
	}
	return &Frontmatter{keys: f.keys[:len(f.keys):len(f.keys)], block: f.block, shared: true}
}

// markShared records that f's keys and block are held by a memo, which hands
// out clones of f and never writes it.
func (f *Frontmatter) markShared() {
	f.shared = true
}

// own gives f its own keys and block before a write, when they are shared.
// The map's values are shared still, and that is safe because no writer
// changes a key's slice in place: each replaces it.
func (f *Frontmatter) own() {
	if !f.shared {
		return
	}
	f.keys = append([]string(nil), f.keys...)
	block := make(map[string][]string, len(f.block))
	for key, lines := range f.block {
		block[key] = lines
	}
	f.block = block
	f.shared = false
}

// topKey matches a top-level frontmatter key at column one. An indented line
// cannot match it, which is how a nested block stays attached to the key
// above it rather than becoming a key of its own.
var topKey = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_.-]*):(.*)$`)

// seqItem matches one entry of a block sequence.
var seqItem = regexp.MustCompile(`^\s*-\s*(.*)$`)

// NewFrontmatter returns an empty header.
func NewFrontmatter() *Frontmatter {
	return &Frontmatter{block: map[string][]string{}}
}

// ParseAnchor splits an anchor file into its frontmatter and its body. A file
// with no leading fence has no frontmatter and is all body, which is what
// makes a hand-written note readable rather than an error.
//
// Carriage returns are stripped per line, because the format tolerates the
// CRLF a Windows editor or a misconfigured git filter introduces rather than
// failing on it.
func ParseAnchor(text string) (*Frontmatter, string) {
	fm := NewFrontmatter()
	lines := SplitLines(text)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, text
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return fm, text
	}
	key := ""
	for _, line := range lines[1:end] {
		m := topKey.FindStringSubmatch(line)
		if m == nil {
			if key != "" {
				fm.block[key] = append(fm.block[key], line)
			}
			continue
		}
		key = m[1]
		if _, seen := fm.block[key]; !seen {
			fm.keys = append(fm.keys, key)
		}
		fm.block[key] = []string{line}
	}
	return fm, strings.Join(lines[end+1:], "\n")
}

// SplitLines splits text on LF and strips a trailing carriage return from
// each line, which is the reader tolerance the format's encoding section
// requires.
func SplitLines(text string) []string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// Keys returns the top-level keys in the order they were read.
func (f *Frontmatter) Keys() []string {
	return append([]string(nil), f.keys...)
}

// Has reports whether the header carries a key at all, which is the question
// an absent value and an empty value answer the same way.
func (f *Frontmatter) Has(key string) bool {
	_, ok := f.block[key]
	return ok
}

// Recognized reports whether frontmatter carries a Dinah workbench's claim to
// its directory: the profile key, or the format or columns key without it. It
// does not validate what those keys hold, only that they are there, so a
// workbench whose profile line is missing or malformed is still recognized
// and left to Open to refuse by name.
func (f *Frontmatter) Recognized() bool {
	return f.Has("profile") || f.Has("format") || f.Has("columns")
}

// Value returns the scalar a key carries, trimmed of surrounding space and of
// one layer of quoting. A key whose value is a block returns the empty string.
func (f *Frontmatter) Value(key string) string {
	lines, ok := f.block[key]
	if !ok {
		return ""
	}
	m := topKey.FindStringSubmatch(lines[0])
	if m == nil {
		return ""
	}
	return unquote(strings.TrimSpace(m[2]))
}

// Seq returns the entries of a sequence, whether written inline as a flow
// sequence on the key's own line or as a block of dashed entries beneath it.
// A trailing comment on an entry is annotation for a person and is dropped.
func (f *Frontmatter) Seq(key string) []string {
	lines, ok := f.block[key]
	if !ok {
		return nil
	}
	inline := f.Value(key)
	if strings.HasPrefix(inline, "[") && strings.HasSuffix(inline, "]") {
		var items []string
		for _, raw := range strings.Split(strings.Trim(inline, "[]"), ",") {
			if item := unquote(strings.TrimSpace(raw)); item != "" {
				items = append(items, item)
			}
		}
		return items
	}
	var items []string
	for _, line := range lines[1:] {
		m := seqItem.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if item := unquote(stripComment(m[1])); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// Set writes a scalar value, appending the key when it is new and leaving the
// position of an existing key alone.
func (f *Frontmatter) Set(key, value string) {
	f.own()
	if _, ok := f.block[key]; !ok {
		f.keys = append(f.keys, key)
	}
	f.block[key] = []string{key + ": " + quote(value)}
}

// SetAfter writes a scalar value, placing a new key directly after another key
// rather than at the end of the header, and leaving the position of an
// existing key alone. A key the header does not carry to place after leaves
// the new key at the end, which is where Set would have put it.
//
// A field added to an anchor by a migration reads where a reader expects it
// when the writer that creates the anchor puts it in the same place, so the
// two anchors of one workbench do not differ by who wrote them.
func (f *Frontmatter) SetAfter(key, value, after string) {
	f.own()
	_, existing := f.block[key]
	f.Set(key, value)
	if existing || key == after {
		return
	}
	position := -1
	for i, k := range f.keys {
		if k == after {
			position = i
			break
		}
	}
	if position < 0 {
		return
	}
	f.keys = f.keys[:len(f.keys)-1]
	rest := append([]string{key}, f.keys[position+1:]...)
	f.keys = append(f.keys[:position+1], rest...)
}

// SetSeq writes a sequence as a block of dashed entries. An empty sequence
// deletes the key, because absent and empty mean the same thing here and the
// shorter form is the one a reader can scan.
func (f *Frontmatter) SetSeq(key string, items []string) {
	f.own()
	if len(items) == 0 {
		f.Delete(key)
		return
	}
	if _, ok := f.block[key]; !ok {
		f.keys = append(f.keys, key)
	}
	lines := []string{key + ":"}
	for _, item := range items {
		lines = append(lines, "  - "+quote(item))
	}
	f.block[key] = lines
}

// SetRaw writes a key's lines verbatim, for a value whose shape the typed
// setters do not cover. The first line must carry the key.
func (f *Frontmatter) SetRaw(key string, lines []string) {
	f.own()
	if _, ok := f.block[key]; !ok {
		f.keys = append(f.keys, key)
	}
	f.block[key] = lines
}

// Raw returns a key's lines as they stand, which is what a caller preserving
// a value rather than reading it needs.
func (f *Frontmatter) Raw(key string) []string {
	return append([]string(nil), f.block[key]...)
}

// Delete removes a key and its block.
func (f *Frontmatter) Delete(key string) {
	f.own()
	if _, ok := f.block[key]; !ok {
		return
	}
	delete(f.block, key)
	for i, k := range f.keys {
		if k != key {
			continue
		}
		f.keys = append(f.keys[:i], f.keys[i+1:]...)
		return
	}
}

// Render writes the anchor file: the frontmatter between its fences, then the
// body. Line endings are LF, which the format mandates for writers.
func (f *Frontmatter) Render(body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, key := range f.keys {
		for _, line := range f.block[key] {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	b.WriteString("---\n")
	b.WriteString(body)
	return b.String()
}

// stripComment drops a trailing annotation comment from a sequence entry. The
// comment has to be preceded by whitespace, so a value carrying a hash of its
// own survives.
func stripComment(s string) string {
	if i := strings.Index(s, " #"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// unquote removes one layer of matching quotes.
//
// The double-quoted branch undoes escape's three escapes in one left-to-right
// pass rather than as three sequential ReplaceAll calls, because that order
// misreads a backslash that was itself escaped and is followed by the letter
// n: `\\n`, escape's rendering of a literal backslash followed by a literal
// n, reads back under three ReplaceAll calls as a newline (the first call
// matches the `\n` sitting inside `\\n` before the second call gets to turn
// `\\` into `\`), which is not the text escape was given and is not what the
// stored bytes spell. A single pass reads each backslash together with the
// one character after it, so an escape produced by escape is undone
// regardless of what precedes or follows it. An escape this reader does not
// recognise is kept literally, on the tolerant-reader posture this file
// keeps everywhere else: a line it cannot make sense of is carried rather
// than dropped.
func unquote(s string) string {
	if len(s) < 2 {
		return s
	}
	first, last := s[0], s[len(s)-1]
	if first != last {
		return s
	}
	if first == '\'' {
		return s[1 : len(s)-1]
	}
	if first != '"' {
		return s
	}
	inner := s[1 : len(s)-1]
	var b strings.Builder
	b.Grow(len(inner))
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c != '\\' || i+1 >= len(inner) {
			b.WriteByte(c)
			continue
		}
		i++
		switch inner[i] {
		case 'n':
			b.WriteByte('\n')
		case '"':
			b.WriteByte('"')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte('\\')
			b.WriteByte(inner[i])
		}
	}
	return b.String()
}

// quote wraps a value in double quotes when leaving it bare would change how
// it reads back: an empty value, one with leading or trailing space, or one
// whose first character would start some other YAML construct.
//
// It normalises the value's line endings on entry, and it is the one function
// in this codebase where a frontmatter scalar can be normalised: every scalar
// that becomes a frontmatter line passes through here, Set and SetSeq and
// SetRaw's own renderers alike, so normalising at the callers instead would be
// an enumeration that goes stale the next time somebody renders a line. The
// escape below turns a line feed into the two characters backslash and n and
// leaves a carriage return raw, so a value carrying CRLF that reached this
// function unnormalised would be stored as a carriage return followed by that
// escape, which is a stored line ending no search for a CRLF pair can find.
//
// Normalising changes no rendering decision. A value carrying CRLF carries a
// line feed, so it takes the quoted branch before and after alike.
func quote(value string) string {
	return quoteVerbatim(NormalizeNewlines(value))
}

// quoteVerbatim is quote's rendering with no normalisation, for the one reader
// that has to ask what a value's stored lines would have been before the
// normalisation existed. The newline migration is that reader: it decides
// whether a key's stored lines are a plain re-render of the value it parsed,
// and comparing against the normalising form would answer no for every key it
// is about to repair.
func quoteVerbatim(value string) string {
	if value == "" {
		return `""`
	}
	if value != strings.TrimSpace(value) {
		return `"` + escape(value) + `"`
	}
	if strings.ContainsAny(value[:1], "-[]{}&*!|>%@`\"'#") {
		return `"` + escape(value) + `"`
	}
	if strings.Contains(value, ": ") || strings.Contains(value, " #") || strings.Contains(value, "\n") {
		return `"` + escape(value) + `"`
	}
	return value
}

// escape prepares a value for double-quoted form.
func escape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

// ErrRenameCollides reports a rename onto a name the header already carries.
// Two keys of one name is not a header any reader could resolve, and the only
// way to make room is to destroy whatever the target holds, so the rename
// refuses and leaves the header exactly as it found it.
//
// The refusal is the contract rather than a defensive extra. An earlier
// version of Rename made room by deleting the target, which turned a caller's
// mistake about which keys a header carried into the silent loss of a value
// nothing could recover, and a header is the one place in this format where a
// lost value has no journal behind it.
var ErrRenameCollides = errors.New("frontmatter: the header already carries the name this rename would take")

// Rename changes a key's name, keeping its position in the header and its
// stored lines, which is the same preservation Set and SetAfter give a value.
// A key the header does not carry is left alone and answers no error, since
// there is nothing there to rename and nothing there to lose. A rename onto a
// name the header already carries answers ErrRenameCollides and changes
// nothing, and a caller that means to replace the target deletes it first, in
// its own statement, where a reader can see the deletion happening.
//
// A migration that renames a key rather than rewriting a value needs this:
// re-Setting the value would quote it afresh and move the key to the end,
// where a reader expects to find it where its neighbours left it.
func (f *Frontmatter) Rename(from, to string) error {
	f.own()
	lines, ok := f.block[from]
	if !ok || from == to {
		return nil
	}
	if _, taken := f.block[to]; taken {
		return fmt.Errorf("%w: %s onto %s", ErrRenameCollides, from, to)
	}
	renamed := append([]string(nil), lines...)
	if len(renamed) > 0 {
		renamed[0] = to + strings.TrimPrefix(renamed[0], from)
	}
	delete(f.block, from)
	f.block[to] = renamed
	for i, key := range f.keys {
		if key == from {
			f.keys[i] = to
			return nil
		}
	}
	return nil
}
