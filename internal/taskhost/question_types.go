package taskhost

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

const MaxQuestionTextLength = 16384

type QuestionSpec struct {
	Type          string           `json:"type"`
	Options       []QuestionOption `json:"options,omitempty"`
	MinLength     int              `json:"minLength,omitempty"`
	MaxLength     int              `json:"maxLength,omitempty"`
	MinSelections int              `json:"minSelections,omitempty"`
	MaxSelections int              `json:"maxSelections,omitempty"`
}

type QuestionAnswer struct {
	Type      string   `json:"type"`
	Text      *string  `json:"text,omitempty"`
	OptionIDs []string `json:"optionIds,omitempty"`
}

func (s *QuestionSpec) UnmarshalJSON(data []byte) error {
	bad := func() error { return refusal("stale", "the question declaration must match its type") }
	d := json.NewDecoder(bytes.NewReader(data))
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
		if d.Decode(&value) != nil {
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
	type plain QuestionSpec
	var decoded plain
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&decoded) != nil {
		return bad()
	}
	for _, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return bad()
		}
	}
	for name := range fields {
		switch name {
		case "type", "options", "minLength", "maxLength", "minSelections", "maxSelections":
		default:
			return bad()
		}
	}
	if decoded.Type == "text" {
		if fields["maxLength"] == nil || fields["options"] != nil || fields["minSelections"] != nil || fields["maxSelections"] != nil {
			return bad()
		}
	} else if decoded.Type == "multiple" {
		if fields["minLength"] != nil || fields["maxLength"] != nil {
			return bad()
		}
	} else {
		return bad()
	}
	*s = QuestionSpec(decoded)
	return s.Validate()
}

// Validate checks a declaration before a reviewed operation is admitted.
func (s QuestionSpec) Validate() error { return s.validate() }

func (a QuestionAnswer) MarshalJSON() ([]byte, error) {
	if a.Type == "text" {
		return json.Marshal(struct {
			Type string  `json:"type"`
			Text *string `json:"text"`
		}{a.Type, a.Text})
	}
	ids := a.OptionIDs
	if ids == nil {
		ids = []string{}
	}
	return json.Marshal(struct {
		Type      string   `json:"type"`
		OptionIDs []string `json:"optionIds"`
	}{a.Type, ids})
}

func (s QuestionSpec) validate() error {
	bad := func() error { return refusal("stale", "the question has an invalid answer schema") }
	switch s.Type {
	case "text":
		if s.MaxLength < 1 || s.MaxLength > MaxQuestionTextLength || s.MinLength < 0 || s.MinLength > s.MaxLength || len(s.Options) != 0 || s.MinSelections != 0 || s.MaxSelections != 0 {
			return bad()
		}
		return nil
	case "multiple", "":
		if len(s.Options) < 1 || len(s.Options) > 16 || s.MinLength != 0 || s.MaxLength != 0 {
			return bad()
		}
		if s.Type == "multiple" {
			if s.MinSelections < 0 || s.MaxSelections < 1 || s.MaxSelections > len(s.Options) || s.MinSelections > s.MaxSelections {
				return bad()
			}
		} else if s.MinSelections != 0 || s.MaxSelections != 0 {
			return bad()
		}
	default:
		return bad()
	}
	seen := map[string]bool{}
	for _, o := range s.Options {
		if o.OptionID == "" || o.Name == "" || len(o.OptionID) > 512 || len(o.Name) > 4096 || seen[o.OptionID] {
			return bad()
		}
		if s.Type != "" && (!utf8.ValidString(o.OptionID) || !utf8.ValidString(o.Name)) {
			return bad()
		}
		seen[o.OptionID] = true
	}
	return nil
}

func (s QuestionSpec) validateAnswer(a QuestionAnswer) (QuestionAnswer, error) {
	bad := func() (QuestionAnswer, error) {
		return QuestionAnswer{}, refusal("not_authorized", "the answer is outside the question's declared type or bounds")
	}
	if a.Type != s.Type {
		return bad()
	}
	if s.Type == "text" {
		if a.Text == nil || len(a.OptionIDs) != 0 || !utf8.ValidString(*a.Text) {
			return bad()
		}
		n := utf8.RuneCountInString(*a.Text)
		if n < s.MinLength || n > s.MaxLength {
			return bad()
		}
		return clone(a), nil
	}
	minimum, maximum := s.MinSelections, s.MaxSelections
	if s.Type == "" {
		minimum, maximum = 1, 1
	}
	if a.Text != nil || len(a.OptionIDs) < minimum || len(a.OptionIDs) > maximum {
		return bad()
	}
	selected := map[string]bool{}
	for _, id := range a.OptionIDs {
		if selected[id] {
			return bad()
		}
		selected[id] = true
	}
	result := QuestionAnswer{Type: a.Type, OptionIDs: []string{}}
	for _, o := range s.Options {
		if selected[o.OptionID] {
			result.OptionIDs = append(result.OptionIDs, o.OptionID)
			delete(selected, o.OptionID)
		}
	}
	if len(selected) != 0 {
		return bad()
	}
	return result, nil
}
