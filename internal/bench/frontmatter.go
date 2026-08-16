// Package bench implements Dinah's on-disk format: the entity shape, the
// anchor files, the append-only journals, the card lock, bench discovery and
// the user base. It knows nothing about verbs, heads or catalogs, so the
// contract's rules live above it and the filesystem's rules live here.
package bench

import (
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
type Frontmatter struct {
	keys  []string
	block map[string][]string
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
	if _, ok := f.block[key]; !ok {
		f.keys = append(f.keys, key)
	}
	f.block[key] = []string{key + ": " + quote(value)}
}

// SetSeq writes a sequence as a block of dashed entries. An empty sequence
// deletes the key, because absent and empty mean the same thing here and the
// shorter form is the one a reader can scan.
func (f *Frontmatter) SetSeq(key string, items []string) {
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
	inner = strings.ReplaceAll(inner, `\n`, "\n")
	inner = strings.ReplaceAll(inner, `\"`, `"`)
	return strings.ReplaceAll(inner, `\\`, `\`)
}

// quote wraps a value in double quotes when leaving it bare would change how
// it reads back: an empty value, one with leading or trailing space, or one
// whose first character would start some other YAML construct.
func quote(value string) string {
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
