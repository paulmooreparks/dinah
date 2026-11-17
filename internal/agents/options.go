package agents

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"studio/internal/acp"
)

// An agent's config options (https://agentclientprotocol.com/protocol/session-config-options)
// are the settings it offers for a session, such as its model and mode.
// Nothing here assumes that a particular option or value exists.

// OptionValues lists the values a select option allows, from either a flat
// list of values or a list of groups of values.
func OptionValues(o acp.ConfigOption) []string {
	var flat []struct {
		Value   string            `json:"value"`
		Options []json.RawMessage `json:"options"`
	}
	if json.Unmarshal(o.Options, &flat) != nil {
		return nil
	}
	var out []string
	for _, v := range flat {
		if v.Value != "" {
			out = append(out, v.Value)
			continue
		}
		for _, g := range v.Options {
			var gv struct {
				Value string `json:"value"`
			}
			if json.Unmarshal(g, &gv) == nil && gv.Value != "" {
				out = append(out, gv.Value)
			}
		}
	}
	return out
}

// OptionChoice is one value a select option allows, with the name the agent
// gives it for people to read.
type OptionChoice struct {
	Value string
	Name  string
}

// OptionChoices lists the values a select option allows with their names, in
// the agent's order, from either a flat list or a list of groups.
func OptionChoices(o acp.ConfigOption) []OptionChoice {
	type entry struct {
		Value   string            `json:"value"`
		Name    string            `json:"name"`
		Options []json.RawMessage `json:"options"`
	}
	var flat []entry
	if json.Unmarshal(o.Options, &flat) != nil {
		return nil
	}
	var out []OptionChoice
	for _, v := range flat {
		if v.Value != "" {
			out = append(out, OptionChoice{Value: v.Value, Name: v.Name})
			continue
		}
		for _, g := range v.Options {
			var gv entry
			if json.Unmarshal(g, &gv) == nil && gv.Value != "" {
				out = append(out, OptionChoice{Value: gv.Value, Name: gv.Name})
			}
		}
	}
	return out
}

// Allowed reports whether value is one the option accepts now.
func Allowed(o acp.ConfigOption, value json.RawMessage) bool {
	if len(bytes.TrimSpace(value)) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return false
	}
	switch o.Type {
	case "boolean":
		var b bool
		return json.Unmarshal(value, &b) == nil
	case "select":
		var s string
		if json.Unmarshal(value, &s) != nil {
			return false
		}
		return slices.Contains(OptionValues(o), s)
	}
	return false
}

// ModelOption is a session's model selector, or false when the agent offers
// none.
func ModelOption(opts []acp.ConfigOption) (acp.ConfigOption, bool) {
	for _, o := range opts {
		if o.Category == "model" && o.Type == "select" {
			return o, true
		}
	}
	return acp.ConfigOption{}, false
}

// ModelName is the name the agent's own list gives the model its options
// select now, or "" when the agent offers no model option or its list gives
// the selected value no name. The name is what the agent reports, so a
// value such as "Adaptive", where the agent picks the model itself, is
// shown as it is and never resolved further.
func ModelName(opts []acp.ConfigOption) string {
	option, ok := ModelOption(opts)
	if !ok {
		return ""
	}
	var value string
	if json.Unmarshal(option.CurrentValue, &value) != nil || value == "" {
		return ""
	}
	return ValueName(option.Options, value)
}

// ModelValueName is the name an agent's options give a model value saved
// for a chat that is not open, or "" when the agent's list no longer gives
// that value a name.
func ModelValueName(opts []acp.ConfigOption, saved json.RawMessage) string {
	option, ok := ModelOption(opts)
	if !ok {
		return ""
	}
	var value string
	if json.Unmarshal(saved, &value) != nil || value == "" {
		return ""
	}
	return ValueName(option.Options, value)
}

// ValueName finds the name a select option's list gives a value, in a flat
// list or a list of groups, and returns "" when the list gives it none.
func ValueName(options json.RawMessage, value string) string {
	var entries []struct {
		Value   string            `json:"value"`
		Name    string            `json:"name"`
		Options []json.RawMessage `json:"options"`
	}
	if json.Unmarshal(options, &entries) != nil {
		return ""
	}
	for _, e := range entries {
		if e.Value == value && strings.TrimSpace(e.Name) != "" {
			return strings.TrimSpace(e.Name)
		}
		for _, raw := range e.Options {
			var inner struct{ Value, Name string }
			if json.Unmarshal(raw, &inner) == nil && inner.Value == value && strings.TrimSpace(inner.Name) != "" {
				return strings.TrimSpace(inner.Name)
			}
		}
	}
	return ""
}
