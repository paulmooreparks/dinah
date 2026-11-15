package main

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"studio/internal/taskhost"
)

func decodeTaskQuestionAnswer(raw string) (taskhost.QuestionAnswer, error) {
	bad := func() (taskhost.QuestionAnswer, error) {
		return taskhost.QuestionAnswer{}, taskRefusal("not_authorized", "the task answer must match its declared JSON shape")
	}
	if len(raw) > 131072 || !utf8.ValidString(raw) || !json.Valid([]byte(raw)) || !pairedJSONSurrogates(raw) {
		return bad()
	}
	d := json.NewDecoder(strings.NewReader(raw))
	opening, err := d.Token()
	if err != nil || opening != json.Delim('{') {
		return bad()
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return bad()
		}
		name, ok := key.(string)
		if !ok || fields[name] != nil {
			return bad()
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return bad()
		}
		fields[name] = value
	}
	if _, err := d.Token(); err != nil {
		return bad()
	}
	if _, err := d.Token(); err != io.EOF {
		return bad()
	}
	var answer taskhost.QuestionAnswer
	if err := json.Unmarshal(fields["type"], &answer.Type); err != nil {
		return bad()
	}
	switch answer.Type {
	case "text":
		if len(fields) != 2 || len(fields["text"]) == 0 || fields["text"][0] != '"' {
			return bad()
		}
		if err := json.Unmarshal(fields["text"], &answer.Text); err != nil {
			return bad()
		}
	case "multiple":
		if len(fields) != 2 || len(fields["optionIds"]) == 0 || fields["optionIds"][0] != '[' {
			return bad()
		}
		if err := json.Unmarshal(fields["optionIds"], &answer.OptionIDs); err != nil {
			return bad()
		}
	default:
		return bad()
	}
	return answer, nil
}

// encoding/json replaces unpaired escaped surrogates. Reject those wire values
// before decoding so the person's submitted text is never silently changed.
func pairedJSONSurrogates(raw string) bool {
	quoted := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		n, err := strconv.ParseUint(raw[i+1:i+5], 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n < 0xd800 || n > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1:i+3] != `\u` {
			return false
		}
		low, err := strconv.ParseUint(raw[i+3:i+7], 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}
