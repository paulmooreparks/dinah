package agents

import (
	"bytes"
	"encoding/json"
	"slices"

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
