// Package frontmatter reads and changes the YAML front matter of a Markdown
// document, such as a skill's name, description and argument hint, so a form
// can show its keys and change one of them.
//
// It is not a YAML parser and must not grow into one. It recognises the
// top-level "key: value" lines of a block and tells a scalar on one line from
// a value that continues on the lines below it. Anything else is skipped
// rather than guessed at, and a change touches one line and leaves every
// other line as it was.
package frontmatter

import "strings"

// splitKeyValue splits a trimmed "key: value" line. It reports false for any
// line whose key is not a plain scalar key (a YAML list item, a comment, a
// bare word), which is how unrecognised constructs get skipped instead of
// misread. The value has surrounding quotes stripped.
func splitKeyValue(trimmed string) (key, value string, ok bool) {
	colon := strings.Index(trimmed, ":")
	if colon <= 0 {
		return "", "", false
	}
	key = trimmed[:colon]
	if !isScalarKey(key) {
		return "", "", false
	}
	return key, unquote(strings.TrimSpace(trimmed[colon+1:])), true
}

// isScalarKey reports whether s has the shape of a front-matter key: letters,
// digits, underscore, hyphen and dot only.
func isScalarKey(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return true
}

// unquote strips one layer of matching single or double quotes, so a value
// written quoted and the same value written bare read the same.
func unquote(s string) string {
	if len(s) < 2 {
		return s
	}
	q := s[0]
	if (q == '"' || q == '\'') && s[len(s)-1] == q {
		return s[1 : len(s)-1]
	}
	return s
}

// indentOf returns the number of leading spaces or tabs on a line.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}
