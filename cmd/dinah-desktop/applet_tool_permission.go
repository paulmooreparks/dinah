package main

import (
	"encoding/json"
	"strings"

	"studio/internal/acp"
)

// An agent asks the person before it calls a tool, as its own rules say,
// and Dinah asks the person before an applet action has an effect
// (applet_consent.go). For Dinah's own applet tools that made two or three
// questions of one small edit (Paul, 9 November 2026): the agent's question
// about applet_discover, its question about applet_invoke, and Dinah's
// about the action itself. Only Dinah's can tell a read from a write, so
// Dinah answers the agent's questions about its own tools itself, with the
// agent's "allow once", and the person is asked once, and only for an
// effect.

// appletToolPrefix is how an agent names a tool of the dinah-applets MCP
// server in its permission request.
const appletToolPrefix = "mcp__dinah-applets__"

var appletToolNames = map[string]bool{"applet_discover": true, "applet_invoke": true, "applet_status": true, "applet_artifact": true}

// ownToolPermission answers an agent's permission request for one of
// Dinah's own applet tools, with the option that allows the call once; it
// returns false for any other request, or when the agent offers no such
// option, and the person is asked as before.
func ownToolPermission(toolCall json.RawMessage, options []acp.PermissionOption) (string, bool) {
	var call struct {
		Title    string `json:"title"`
		RawInput struct {
			Server string `json:"server"`
			Tool   string `json:"tool"`
		} `json:"rawInput"`
	}
	if json.Unmarshal(toolCall, &call) != nil {
		return "", false
	}
	title := strings.TrimSpace(call.Title)
	name := title
	if i := strings.IndexAny(title, " \n:"); i >= 0 {
		name = title[:i]
	}
	own := strings.HasPrefix(name, appletToolPrefix) || appletToolNames[name] ||
		(call.RawInput.Server == "dinah-applets" && appletToolNames[call.RawInput.Tool])
	if !own {
		return "", false
	}
	for _, o := range options {
		if o.Kind == "allow_once" {
			return o.OptionID, true
		}
	}
	return "", false
}
