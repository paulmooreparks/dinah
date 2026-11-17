package main

import (
	"encoding/json"

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
// or the selected value when the option lists no name for it. The chat is
// told the value rather than nothing, since it reads it only to know itself;
// the lists the person reads show a model only by its name (agents.ModelName).
func currentModelName(opts []acp.ConfigOption) string {
	if name := agents.ModelName(opts); name != "" {
		return name
	}
	option, ok := agents.ModelOption(opts)
	if !ok {
		return ""
	}
	var value string
	if json.Unmarshal(option.CurrentValue, &value) != nil {
		return ""
	}
	return value
}
