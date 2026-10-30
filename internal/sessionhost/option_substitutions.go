package sessionhost

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"studio/internal/acp"
	"studio/internal/agents"
)

//go:embed option_substitutions.go
var OptionSubstitutionsSource []byte

// OptionSubstitution is one substitution the person stored: when a chat's
// saved value From of the option Option is not offered and To is, the chat
// is reopened with To. The names are kept as they were when the person chose,
// for Settings and the chat's note to show.
type OptionSubstitution struct {
	Option     string          `json:"option"`
	OptionName string          `json:"optionName,omitempty"`
	From       json.RawMessage `json:"from"`
	FromName   string          `json:"fromName,omitempty"`
	To         json.RawMessage `json:"to"`
	ToName     string          `json:"toName,omitempty"`
}

// OptionSubstitutions are the stored substitutions by provider. A settings
// file edited by hand may contain entries that are not substitutions; each one
// that cannot be read is left out, and the rest are kept, so that one bad
// entry neither stops Dinah reading its settings nor takes the others with
// it (checkStoredSubstitutions removes the ones that read but make no sense).
type OptionSubstitutions map[string][]OptionSubstitution

func (o *OptionSubstitutions) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) != nil {
		*o = nil
		return nil
	}
	out := OptionSubstitutions{}
	for provider, list := range raw {
		var entries []json.RawMessage
		if json.Unmarshal(list, &entries) != nil {
			continue
		}
		for _, entry := range entries {
			var sub OptionSubstitution
			if json.Unmarshal(entry, &sub) == nil {
				out[provider] = append(out[provider], sub)
			}
		}
	}
	*o = out
	return nil
}

// optionValue is a value as a stored substitution or the agent may give it:
// a select option's string or a boolean option's true or false. Anything
// else is no value an option can have.
func OptionValue(v json.RawMessage) (any, bool) {
	var x any
	if json.Unmarshal(v, &x) != nil {
		return nil, false
	}
	switch x.(type) {
	case string, bool:
		return x, true
	}
	return nil, false
}

// sameValue reports whether two encoded values are the same value, however
// they are spaced.
func sameValue(x, y json.RawMessage) bool {
	return SameOptionValue(x, y)
}

// valueText is a value as the page writes it: a string as it is, a boolean
// as true or false.
func ValueText(v json.RawMessage) string {
	x, ok := OptionValue(v)
	if !ok {
		return string(bytes.TrimSpace(v))
	}
	return fmt.Sprint(x)
}

// valueWords is a value for the person to read: its name, with the value
// itself beside it when the two differ.
func valueWords(name string, v json.RawMessage) string {
	text := ValueText(v)
	switch {
	case name == "":
		return text
	case strings.EqualFold(name, text):
		return name
	}
	return name + " (" + text + ")"
}

// findSubstitution is the index of the substitution for an option's saved
// value, or -1.
func FindSubstitution(list []OptionSubstitution, option string, from json.RawMessage) int {
	for i, sub := range list {
		if sub.Option == option && sameValue(sub.From, from) {
			return i
		}
	}
	return -1
}

// reopening is what restoreOptions needs to reopen a saved chat rather
// than to start a new one: the chat's provider, the substitutions the person
// stored for it, and the options the provider offered most recently, where
// the saved value's name may still be found.
type Reopening struct {
	provider      string
	substitutions []OptionSubstitution
	catalog       []acp.ConfigOption
	providerName  func(string) string
}

// AppliedSubstitution is a stored substitution restoreOptions used.
type AppliedSubstitution struct {
	OptionSubstitution
	// note is the line the chat shows about it.
	note string
}

// OfferedValue is one value the agent offers for an option, with its name.
// Suggested marks a value whose name is the saved value's name or the saved
// value itself, ignoring case; it is listed first, and never chosen for the
// person.
type OfferedValue struct {
	Value     json.RawMessage `json:"value"`
	Name      string          `json:"name"`
	Suggested bool            `json:"suggested,omitempty"`
}

// UnofferedOption is what the page needs to offer Open With… when a chat's
// saved value of an option is not offered: the option, the saved value and
// its name where Dinah still knows it, and the values offered instead.
type UnofferedOption struct {
	Kind       string          `json:"kind"`
	Provider   string          `json:"provider"`
	Option     string          `json:"option"`
	OptionName string          `json:"optionName"`
	Saved      json.RawMessage `json:"saved"`
	SavedName  string          `json:"savedName,omitempty"`
	Offered    []OfferedValue  `json:"offered"`
}

// UnofferedOptionError is the refusal to reopen a chat whose saved value of
// an option the agent offers no longer. Its message is the refusal's as it
// always was; its detail goes to the page with it (bridge.DetailedError).
type UnofferedOptionError struct {
	detail UnofferedOption
}

func (e *UnofferedOptionError) Error() string {
	return fmt.Sprintf("this chat's saved %s setting is no longer available from its provider", e.detail.Option)
}

func (e *UnofferedOptionError) ErrorDetail() any { return e.detail }

// unoffered builds the refusal for a saved value an option does not offer.
func (r *Reopening) unoffered(option acp.ConfigOption, saved json.RawMessage) error {
	detail := UnofferedOption{Kind: "unoffered-option", Provider: r.provider, Option: option.ID, OptionName: option.Name,
		Saved: append(json.RawMessage(nil), saved...), SavedName: r.SavedName(option.ID, saved), Offered: []OfferedValue{}}
	if detail.OptionName == "" {
		detail.OptionName = option.ID
	}
	savedText := ValueText(saved)
	var suggested, rest []OfferedValue
	for _, choice := range agents.OptionChoices(option) {
		value, _ := json.Marshal(choice.Value)
		v := OfferedValue{Value: value, Name: choice.Name}
		if choice.Name != "" && (strings.EqualFold(choice.Name, savedText) || (detail.SavedName != "" && strings.EqualFold(choice.Name, detail.SavedName))) {
			v.Suggested = true
			suggested = append(suggested, v)
		} else {
			rest = append(rest, v)
		}
	}
	detail.Offered = append(append(detail.Offered, suggested...), rest...)
	return &UnofferedOptionError{detail: detail}
}

// savedName is the name of a saved value: the one in the provider's most
// recent catalog when it still lists the value, or else the one a stored
// substitution kept, or none.
func (r *Reopening) SavedName(option string, saved json.RawMessage) string {
	for _, o := range r.catalog {
		if o.ID != option {
			continue
		}
		for _, choice := range agents.OptionChoices(o) {
			if choice.Name != "" && ValueText(saved) == choice.Value {
				return choice.Name
			}
		}
	}
	if i := FindSubstitution(r.substitutions, option, saved); i >= 0 {
		return r.substitutions[i].FromName
	}
	return ""
}

// substitute is the stored substitution for an option's saved value whose
// value the option offers now, if there is one.
func (r *Reopening) substitute(option acp.ConfigOption, saved json.RawMessage) (AppliedSubstitution, bool) {
	i := FindSubstitution(r.substitutions, option.ID, saved)
	if i < 0 || !agents.Allowed(option, r.substitutions[i].To) {
		return AppliedSubstitution{}, false
	}
	sub := r.substitutions[i]
	return AppliedSubstitution{OptionSubstitution: sub,
		note: "Opened with " + valueWords(sub.ToName, sub.To) + " because " + valueWords(sub.FromName, sub.From) + " wasn't offered; you set this in Settings › Agents › " + r.providerName(r.provider) + "."}, true
}

// unofferedSetting is a saved setting other than the model that the agent
// does not offer for the model the chat reopened with: the option is missing,
// or it is there without the saved value. The chat opens with the agent's
// value, says so, and keeps its saved choice for when the agent offers it
// again (restoreOptions).
func unofferedSetting(id string, offered *acp.ConfigOption, saved json.RawMessage) AppliedSubstitution {
	sub := AppliedSubstitution{OptionSubstitution: OptionSubstitution{Option: id, OptionName: id, From: append(json.RawMessage(nil), saved...)}}
	if offered == nil {
		sub.note = "The saved " + id + " setting (" + ValueText(saved) + ") isn't offered with this chat's model, so it is left out; the chat keeps it for when it is offered."
		return sub
	}
	if offered.Name != "" {
		sub.OptionName = offered.Name
	}
	sub.To = append(json.RawMessage(nil), offered.CurrentValue...)
	current := ValueText(offered.CurrentValue)
	for _, choice := range agents.OptionChoices(*offered) {
		if choice.Value == current {
			sub.ToName = choice.Name
		}
	}
	sub.note = sub.OptionName + " " + ValueText(saved) + " isn't offered with this chat's model, so it is " + valueWords(sub.ToName, sub.To) + "; the chat keeps its choice for when it is offered."
	return sub
}

// NewReopening snapshots settings before provider calls can replace the catalog.
func NewReopening(provider string, substitutions []OptionSubstitution, catalog []acp.ConfigOption, providerName func(string) string) *Reopening {
	copied := append([]OptionSubstitution(nil), substitutions...)
	for i := range copied {
		copied[i].From = bytes.Clone(copied[i].From)
		copied[i].To = bytes.Clone(copied[i].To)
	}
	return &Reopening{provider: provider, substitutions: copied, catalog: copyOptions(catalog), providerName: providerName}
}
func (s AppliedSubstitution) Note() string { return s.note }
