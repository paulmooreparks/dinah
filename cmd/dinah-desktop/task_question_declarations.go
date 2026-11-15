package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"studio/internal/taskhost"
)

type typedTaskQuestion struct {
	Title  string                `json:"title"`
	Answer taskhost.QuestionSpec `json:"answer"`
}

// Legacy declarations retain their original decoder. Typed declarations are
// validated during Review and again before execution creates a pending asking.
func decodeTypedTaskQuestion(raw json.RawMessage) (*typedTaskQuestion, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, nil
	}
	typed := false
	for name := range fields {
		if strings.EqualFold(name, "answer") {
			typed = true
		}
	}
	if !typed {
		return nil, nil
	}
	bad := func() (*typedTaskQuestion, error) {
		return nil, taskRefusal("stale", "the typed question declaration is invalid")
	}
	if !utf8.Valid(raw) || !pairedJSONSurrogates(string(raw)) || len(fields) != 2 || fields["title"] == nil || fields["answer"] == nil || bytes.Equal(fields["answer"], []byte("null")) {
		return bad()
	}
	keys := json.NewDecoder(bytes.NewReader(raw))
	_, _ = keys.Token()
	seen := map[string]bool{}
	for keys.More() {
		key, err := keys.Token()
		if err != nil {
			return bad()
		}
		name := key.(string)
		if seen[name] {
			return bad()
		}
		seen[name] = true
		var value json.RawMessage
		if keys.Decode(&value) != nil {
			return bad()
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var input typedTaskQuestion
	if err := d.Decode(&input); err != nil {
		return bad()
	}
	if input.Title == "" || len(input.Title) > 65536 {
		return bad()
	}
	if err := input.Answer.Validate(); err != nil {
		return nil, err
	}
	return &input, nil
}
