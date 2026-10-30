package main

import (
	"encoding/json"
	"strings"

	"studio/internal/acp"
	"studio/internal/agents"
)

// chatIdentityLine tells a chat which provider it runs on and which model, in
// the context Dinah supplies with every turn, so a chat whose model changes
// during the conversation is told the new one. An Ollama chat once went
// searching the person's files because it did not know which model it was. The
// model is left out when the provider offers no model option.
func (a *App) chatIdentityLine(session string) string {
	s := a.session(session)
	if s == nil {
		return ""
	}
	line := "This chat runs on " + providerDisplayName(s.Provider())
	if model := currentModelName(s.Options()); model != "" {
		line += " with the model " + model
	}
	return line + ".\n"
}

// currentModelName is the display name of the model a chat's options select,
// or the selected value when the option lists no name for it.
func currentModelName(opts []acp.ConfigOption) string {
	option, ok := agents.ModelOption(opts)
	if !ok {
		return ""
	}
	var value string
	if json.Unmarshal(option.CurrentValue, &value) != nil || value == "" {
		return ""
	}
	return optionValueName(option.Options, value)
}

// optionValueName finds the name an option's list gives a value, in a flat
// list or a list of groups, and returns the value itself when there is none.
func optionValueName(options json.RawMessage, value string) string {
	var entries []struct {
		Value   string            `json:"value"`
		Name    string            `json:"name"`
		Options []json.RawMessage `json:"options"`
	}
	if json.Unmarshal(options, &entries) != nil {
		return value
	}
	for _, e := range entries {
		if e.Value == value && strings.TrimSpace(e.Name) != "" {
			return e.Name
		}
		for _, raw := range e.Options {
			var inner struct{ Value, Name string }
			if json.Unmarshal(raw, &inner) == nil && inner.Value == value && strings.TrimSpace(inner.Name) != "" {
				return inner.Name
			}
		}
	}
	return value
}
