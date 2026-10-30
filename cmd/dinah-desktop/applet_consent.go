package main

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"

	"studio/internal/acp"
	"studio/internal/applet"
	"studio/internal/applet/broker"
)

// A chat reaches every applet action through one tool, so the agent's own
// approval of that tool cannot tell reading a file from writing one, and one
// "allow" for a read would allow every write. Dinah therefore asks the
// person itself, in the chat, before an action from a chat has an effect:
// writing workspace files, changing settings, running a window's command,
// or putting a question to another chat (applet.Effectful). Reading runs as
// before. A signed extension action may declare an effect level instead,
// and then only "history" and "network" ask (applet.Asks). The person may
// allow an action once, allow it always in the workspace, or refuse it;
// Settings › Agents lists the actions allowed always and forgets them on
// request.

const (
	consentOnce   = "allow_once"
	consentAlways = "allow_always"
	consentRefuse = "reject_once"
)

// errRefused is an action the person did not allow.
var errRefused = errors.New("the person did not allow this action; ask them before trying it again")

// appletConsent is the broker's Consent: nil when the action may run.
func (a *App) appletConsent(ctx context.Context, v broker.Invocation, action applet.Action) error {
	if v.OwnerKind != "chat" || !applet.Asks(action) {
		return nil
	}
	a.mu.Lock()
	all := a.settings.Approvals == ApprovalsAll
	granted := slices.Contains(a.settings.AppletGrants[v.Workspace], action.Ref)
	// A chat let loose to run unattended is not asked either: an agent that
	// must be answered here cannot be left to work, which is the whole point
	// of the setting.
	unattended := false
	if s := a.runtime.ConversationLocked(v.OwnerID); s != nil {
		if m, ok := providerModules[s.Provider()]; ok && m.Unattended != nil {
			unattended = m.Unattended(a.settings)
		}
	}
	a.mu.Unlock()
	// Approve Everything is the person saying they do not want to be asked,
	// which covers this question as it covers the agents' own.
	if all || granted || unattended {
		return nil
	}
	call, _ := json.Marshal(map[string]any{
		"toolCallId": v.ID,
		"title":      action.Title,
		"kind":       "other",
		"rawInput":   json.RawMessage(v.InputJSON),
		"content":    []any{map[string]any{"type": "content", "content": map[string]string{"type": "text", "text": action.Description}}},
	})
	choice, ok := a.permissions().Ask(ctx, v.OwnerID, nil, call, []acp.PermissionOption{
		{OptionID: consentOnce, Name: "Allow", Kind: "allow_once"},
		{OptionID: consentAlways, Name: "Always Allow in This Workspace", Kind: "allow_always"},
		{OptionID: consentRefuse, Name: "Don't Allow", Kind: "reject_once"},
	})
	switch {
	case !ok || choice == consentRefuse:
		return errRefused
	case choice == consentAlways:
		return a.updateSettings(func(s *Settings) error {
			if s.AppletGrants == nil {
				s.AppletGrants = map[string][]string{}
			}
			if !slices.Contains(s.AppletGrants[v.Workspace], action.Ref) {
				s.AppletGrants[v.Workspace] = append(s.AppletGrants[v.Workspace], action.Ref)
			}
			return nil
		})
	}
	return nil
}

// AppletGrant is an applet action the person allowed always in a workspace.
type AppletGrant struct {
	Workspace string `json:"workspace"`
	Action    string `json:"action"`
	Title     string `json:"title"`
}

// AppletGrants lists the applet actions allowed always, for Settings.
func (a *App) AppletGrants() []AppletGrant {
	a.mu.Lock()
	grants := map[string][]string{}
	for ws, refs := range a.settings.AppletGrants {
		grants[ws] = slices.Clone(refs)
	}
	a.mu.Unlock()
	out := []AppletGrant{}
	registry, _ := a.appletRegistry()
	for _, ws := range slices.Sorted(maps.Keys(grants)) {
		for _, ref := range grants[ws] {
			title := ref
			if registry != nil {
				if action, err := registry.Describe(ref); err == nil {
					title = action.Title
				}
			}
			out = append(out, AppletGrant{Workspace: ws, Action: ref, Title: title})
		}
	}
	return out
}

// ForgetAppletGrant stops allowing an action always in a workspace, so a
// chat there asks again.
func (a *App) ForgetAppletGrant(workspace, action string) error {
	return a.updateSettings(func(s *Settings) error {
		s.AppletGrants[workspace] = slices.DeleteFunc(s.AppletGrants[workspace], func(r string) bool { return r == action })
		if len(s.AppletGrants[workspace]) == 0 {
			delete(s.AppletGrants, workspace)
		}
		return nil
	})
}
