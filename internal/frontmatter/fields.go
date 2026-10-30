package frontmatter

import (
	"errors"
	"fmt"
	"strings"

	"studio/internal/preview"
)

// Field is one key of a front-matter block, as a form shows it. A key whose
// value is on its own line (a folded or literal block, a list or a nested
// map) is not Scalar, and a form leaves it to the text.
type Field struct {
	Key    string `json:"key"`
	Value  string `json:"value"` // the scalar value, unquoted; "" when not Scalar
	Line   int    `json:"line"`  // 1-based line of the key
	Scalar bool   `json:"scalar"`
}

// Fields lists the top-level keys of a document's front matter in order.
// Present reports whether the document has a front-matter block at all.
func Fields(text string) (fields []Field, present bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	end := preview.FrontMatterEnd(lines)
	if end == 0 {
		return nil, false
	}
	fields = []Field{}
	for i := 1; i < end-1; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" || indentOf(line) > 0 {
			continue
		}
		key, value, ok := splitKeyValue(strings.TrimSpace(line))
		if !ok {
			continue
		}
		f := Field{Key: key, Line: i + 1, Scalar: true}
		raw := strings.TrimSpace(line[strings.Index(line, ":")+1:])
		// A block indicator, a flow collection or a value continued on the
		// indented lines that follow is not one scalar line.
		if raw == "" || strings.HasPrefix(raw, ">") || strings.HasPrefix(raw, "|") || strings.HasPrefix(raw, "[") || strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "&") || strings.HasPrefix(raw, "*") {
			f.Scalar = false
		} else if i+1 < end-1 && strings.TrimSpace(lines[i+1]) != "" && indentOf(lines[i+1]) > 0 {
			f.Scalar = false
		}
		if f.Scalar {
			f.Value = value
		}
		fields = append(fields, f)
	}
	return fields, true
}

// SetAny returns text with one top-level front-matter key set to value, for
// any key: a skill's name, description or argument-hint, or a field of a
// provider's own. A key whose value is not one scalar line is refused, so a
// form never overwrites a list or a block it could not show. The key must
// have the shape of a YAML key, and the value must be one line.
func SetAny(text, key, value string) (string, error) {
	if !isScalarKey(key) {
		return "", fmt.Errorf("%q is not a front-matter key", key)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", errors.New("a front-matter value must be one line")
	}
	if fields, present := Fields(text); present {
		for _, f := range fields {
			if f.Key == key && !f.Scalar {
				return "", fmt.Errorf("%s is not one line in the text; change it there", key)
			}
		}
	}
	return setLine(text, key, value)
}
