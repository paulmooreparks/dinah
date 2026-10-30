package frontmatter

import (
	"strconv"
	"strings"

	"studio/internal/preview"
)

// setLine returns text with one top-level key set to value. It changes one
// line and leaves every other line as it was, comments, unknown keys, line
// endings and indentation included, because the person's own front matter is
// not this package's to reformat. A key already present is changed where it
// is, keeping its quote character; a key that appears twice is changed at its
// last line, which is the one a YAML reader keeps. A new key is added before
// the closing fence, and a document without front matter gets a block holding
// the one key.
func setLine(text, key, value string) (string, error) {
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	raw := strings.Split(text, "\n")
	lines := make([]string, len(raw))
	for i, l := range raw {
		lines[i] = strings.TrimSuffix(l, "\r")
	}

	end := preview.FrontMatterEnd(lines)
	if end == 0 {
		block := []string{"---", key + ": " + format(value, 0), "---", ""}
		return strings.Join(block, eol) + eol + text, nil
	}

	if i := find(lines, end, key); i >= 0 {
		lines[i] = replaceValue(lines[i], value)
		return join(raw, lines, i, eol), nil
	}

	closing := end - 1
	out := make([]string, 0, len(raw)+1)
	out = append(out, raw[:closing]...)
	out = append(out, key+": "+format(value, 0)+lineEnd(raw, closing, eol))
	out = append(out, raw[closing:]...)
	return strings.Join(out, "\n"), nil
}

// find returns the last top-level line of the block that sets key, or -1.
func find(lines []string, end int, key string) int {
	at := -1
	for i := 1; i < end-1 && i < len(lines); i++ {
		line := lines[i]
		if indentOf(line) > 0 {
			continue
		}
		if k, _, ok := splitKeyValue(strings.TrimSpace(line)); ok && k == key {
			at = i
		}
	}
	return at
}

// replaceValue rewrites the value part of one "key: value" line, keeping its
// indentation, its key as written and the quote character it used.
func replaceValue(line, value string) string {
	colon := strings.Index(line, ":")
	old := strings.TrimSpace(line[colon+1:])
	quote := byte(0)
	if len(old) >= 2 && (old[0] == '"' || old[0] == '\'') && old[len(old)-1] == old[0] {
		quote = old[0]
	}
	return line[:colon] + ": " + format(value, quote)
}

// format writes a value as a YAML scalar. A value is quoted only when YAML
// would read it as something other than this string, and a value that was
// quoted keeps its quote character.
func format(value string, quote byte) string {
	if quote == 0 && needsQuotes(value) {
		quote = '"'
	}
	switch quote {
	case '\'':
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	case '"':
		return strconv.Quote(value)
	}
	return value
}

// needsQuotes reports whether YAML would read a plain scalar as something
// other than this string.
func needsQuotes(v string) bool {
	if v == "" || strings.TrimSpace(v) != v {
		return true
	}
	if strings.ContainsAny(v[:1], "-?:,[]{}#&*!|>'\"%@`") {
		return true
	}
	if strings.Contains(v, ": ") || strings.Contains(v, " #") || strings.HasSuffix(v, ":") {
		return true
	}
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return true
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return true
	}
	return false
}

// join puts the original lines back together with line i replaced, keeping
// that line's own ending.
func join(raw, lines []string, i int, eol string) string {
	out := make([]string, len(raw))
	copy(out, raw)
	out[i] = lines[i] + lineEnd(raw, i, eol)
	return strings.Join(out, "\n")
}

// lineEnd is the "\r" a line at i had before its "\n", so a changed or new
// line ends the way the document's lines do.
func lineEnd(raw []string, i int, eol string) string {
	if i < len(raw) && strings.HasSuffix(raw[i], "\r") {
		return "\r"
	}
	if eol == "\r\n" {
		return "\r"
	}
	return ""
}
