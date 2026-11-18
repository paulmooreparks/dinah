package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/applet"
	"studio/internal/bus"
	"studio/internal/sessionhost"
)

//go:embed team_duplication.go
var teamDuplicationSource []byte

// Duplicating a team (docs/design/team-duplication.md). A lead reads a team in its own tree as data with
// dinah.team/describe@1 and builds a copy with dinah.team/start@2, which can start a member under any chat in the
// caller's tree and give it settings. Paul decided on 18 November 2026 that a chat reads and builds only in its own
// tree, and that a member's permission mode is never more permissive than the caller's own.

const (
	teamDescribeAction = "dinah.team/describe@1"
	teamStart2Action   = "dinah.team/start@2"
)

// teamDescribeNode is one chat of a tree as dinah.team/describe@1 returns it.
type teamDescribeNode struct {
	SessionID string `json:"sessionId"`
	BusID     string `json:"busId"`
	Title     string `json:"title"`
	// Remit is what the member is for, and RemitSource says where it came from: "kept" when it was kept as the member
	// was started, "start brief" when it was recovered from the brief its lead posted then, and "" when neither is
	// known.
	Remit       string `json:"remit"`
	RemitSource string `json:"remitSource"`
	Provider    string `json:"provider"`
	Model       struct {
		Value string `json:"value"`
		Name  string `json:"name"`
	} `json:"model"`
	// Options are the chat's settings other than its model, by the setting's ID, as dinah.team/start@2 takes them;
	// Settings are the same with the names the program gives them, for reading.
	Options       map[string]json.RawMessage `json:"options"`
	Settings      []teamDescribeSetting      `json:"settings"`
	Workspace     string                     `json:"workspace"`
	WorkspaceName string                     `json:"workspaceName"`
	Lead          string                     `json:"lead"`
	LeadBusID     string                     `json:"leadBusId"`
	State         string                     `json:"state"`
	StateWhy      string                     `json:"stateWhy"`
	Retired       bool                       `json:"retired"`
	Members       []teamDescribeNode         `json:"members"`
}

// teamDescribeSetting is one setting of a chat with the names its program gives the setting and its value. Category is
// the protocol's ("mode" for a permission mode, "thought_level" for effort), empty when the program gives none.
type teamDescribeSetting struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Category  string          `json:"category"`
	Value     json.RawMessage `json:"value"`
	ValueName string          `json:"valueName"`
}

func (a *App) teamDescribeHandler(ctx context.Context, input string) (applet.Result, error) {
	var in struct{ Member string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return applet.Result{}, err
	}
	caller, callerBus, err := a.teamCaller(ctx)
	if err != nil {
		return applet.Result{}, err
	}
	rootChat, rootBus := caller.ID(), callerBus
	if member := strings.TrimSpace(in.Member); member != "" && member != caller.ID() {
		rootChat, rootBus = member, a.knownBusID(member)
		if rootBus == "" {
			return applet.Result{}, fmt.Errorf("there is no chat %s in your team's tree; dinah.team/describe@1 without a member lists it", member)
		}
	}
	var root teamDescribeNode
	err = a.busHost().With(func(b *bus.Bus) error {
		if rootBus != callerBus {
			inside, err := b.InTree(ctx, callerBus, rootBus)
			if err != nil {
				return err
			}
			if !inside {
				return fmt.Errorf("%s is not in your team's tree, so you cannot read it, because a chat reads only its own members and theirs", rootChat)
			}
		}
		lead, retired, err := b.LeadOf(ctx, rootBus)
		if err != nil {
			return err
		}
		m := bus.TeamMember{Session: rootBus, Lead: lead, Retired: retired}
		if lead != "" {
			if remit, err := b.Remit(ctx, rootBus); err == nil {
				m.Remit = remit
			}
		}
		root = a.describeNode(ctx, b, m, map[string]bool{rootBus: true})
		return nil
	})
	if err != nil {
		return applet.Result{}, err
	}
	return builtinJSON(map[string]any{"team": root})
}

// describeNode reads one chat of a tree, and the members under it, without opening or changing anything.
func (a *App) describeNode(ctx context.Context, b *bus.Bus, m bus.TeamMember, seen map[string]bool) teamDescribeNode {
	chat := a.chatOfBus(m.Session)
	n := teamDescribeNode{SessionID: chat, BusID: m.Session, LeadBusID: m.Lead, Retired: m.Retired, Options: map[string]json.RawMessage{}, Settings: []teamDescribeSetting{}, Members: []teamDescribeNode{}}
	n.Title = a.teamChat(b, ctx, m.Session).Title
	if m.Lead != "" {
		n.Lead = a.chatOfBus(m.Lead)
		n.Remit, n.RemitSource = a.memberRemit(ctx, b, m)
	}
	if s, err := b.Session(ctx, m.Session); err == nil {
		n.Workspace, n.WorkspaceName = s.WorkspacePath, s.WorkspaceLabel
		if n.WorkspaceName == "" {
			n.WorkspaceName = s.WorkspaceName
		}
	}
	st := a.chatState(ctx, b, m.Session, chat, m.Retired, false)
	n.State, n.StateWhy, _ = a.memberState(b, ctx, m, st)
	a.describeSettings(chat, &n)
	members, err := b.TeamMembers(ctx, m.Session)
	if err != nil {
		return n
	}
	for _, child := range members {
		if seen[child.Session] {
			continue
		}
		seen[child.Session] = true
		n.Members = append(n.Members, a.describeNode(ctx, b, child, seen))
	}
	return n
}

// describeSettings fills in a chat's program, model and other settings: from the open chat when it is open, and
// otherwise from what was saved for it with the program's last list of options, which a closed chat gets back when it
// opens.
func (a *App) describeSettings(chat string, n *teamDescribeNode) {
	if chat == "" {
		return
	}
	if s := a.session(chat); s != nil {
		n.Provider = normalizedProvider(s.Provider())
		if s.Cwd() != "" {
			n.Workspace = s.Cwd()
		}
		opts := s.Options()
		if o, ok := agents.ModelOption(opts); ok {
			n.Model.Value, n.Model.Name = judgeOptionString(o.CurrentValue), agents.ModelName(opts)
		}
		for _, o := range opts {
			if o.Category == "model" || !hasValue(o.CurrentValue) {
				continue
			}
			n.addSetting(o, o.CurrentValue)
		}
		return
	}
	provider, err := a.savedSessionProvider(chat)
	if err != nil {
		provider = providerForSession(chat)
	}
	n.Provider = provider
	a.mu.Lock()
	saved := a.settings
	values := copyPreferenceValues(a.settings.SessionOptions[chat])
	catalog := append([]acp.ConfigOption(nil), savedCatalog(a.settings, provider)...)
	n.Model.Value, n.Model.Name = savedModelChoice(saved, provider, chat)
	a.mu.Unlock()
	model, _ := agents.ModelOption(catalog)
	ids := make([]string, 0, len(values))
	for id := range values {
		if id != model.ID || model.ID == "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		o := acp.ConfigOption{ID: id}
		for _, c := range catalog {
			if c.ID == id {
				o = c
			}
		}
		if o.Category == "model" || !hasValue(values[id]) {
			continue
		}
		n.addSetting(o, values[id])
	}
}

func (n *teamDescribeNode) addSetting(o acp.ConfigOption, value json.RawMessage) {
	value = append(json.RawMessage(nil), value...)
	n.Options[o.ID] = value
	n.Settings = append(n.Settings, teamDescribeSetting{ID: o.ID, Name: o.Name, Category: o.Category, Value: value, ValueName: settingValueName(o, value)})
}

func hasValue(v json.RawMessage) bool {
	v = bytes.TrimSpace(v)
	return len(v) > 0 && !bytes.Equal(v, []byte("null"))
}

// settingValueName is the name a select setting's list gives a value, or "" when it gives none.
func settingValueName(o acp.ConfigOption, value json.RawMessage) string {
	var s string
	if json.Unmarshal(value, &s) != nil {
		return ""
	}
	return agents.ValueName(o.Options, s)
}

// memberRemit is a member's remit: the one kept when it was started, or, for a member started before remits were
// kept, the brief its lead posted when it started it. That brief is the lead's first message mentioning the member,
// right after the line saying the lead started a member, which ends with the brief's first line (startLine).
func (a *App) memberRemit(ctx context.Context, b *bus.Bus, m bus.TeamMember) (remit, source string) {
	if strings.TrimSpace(m.Remit) != "" {
		return m.Remit, "kept"
	}
	brief, line, ok, err := b.StartBrief(ctx, m.Lead, m.Session)
	if err != nil || !ok || !strings.HasSuffix(line, ": "+firstLine(brief.Text)) {
		return "", ""
	}
	return brief.Text, "start brief"
}

// teamStartRequest is what starting a member asks for. Lead is the chat to start it under, the caller when it is
// empty; Remit is kept as the member's remit; Options are settings for the new chat by their IDs.
type teamStartRequest struct {
	Title, Brief, Workspace, Provider, Model string
	Lead                                     string
	Remit                                    string
	Options                                  map[string]json.RawMessage
}

// teamStarted is what a start did. NotApplied are the settings that could not be given, each with why.
type teamStarted struct {
	SessionID, BusID, Title, Group, Lead string
	NotApplied                           []map[string]string
}

func (a *App) teamStart2Handler(ctx context.Context, input string) (applet.Result, error) {
	var in struct {
		Title, Brief, Workspace, Provider, Model, Lead, Remit string
		Options                                               map[string]json.RawMessage
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return applet.Result{}, err
	}
	out, err := a.teamStart(ctx, teamStartRequest{Title: in.Title, Brief: in.Brief, Workspace: in.Workspace, Provider: in.Provider, Model: in.Model, Lead: in.Lead, Remit: in.Remit, Options: in.Options})
	if err != nil {
		return applet.Result{}, err
	}
	notApplied := out.NotApplied
	if notApplied == nil {
		notApplied = []map[string]string{}
	}
	return builtinJSON(map[string]any{"sessionId": out.SessionID, "busId": out.BusID, "title": out.Title, "group": out.Group, "lead": out.Lead, "notApplied": notApplied})
}

// teamLeadFor finds the chat a member is to be started under: the caller itself, or a chat in its tree that is not
// retired.
func (a *App) teamLeadFor(ctx context.Context, caller *Session, callerBus, lead string) (string, string, error) {
	lead = strings.TrimSpace(lead)
	if lead == "" || lead == caller.ID() {
		return caller.ID(), callerBus, nil
	}
	leadBus := a.knownBusID(lead)
	if leadBus == "" {
		return "", "", fmt.Errorf("there is no chat %s in your team's tree to start a member under; dinah.team/describe@1 lists your tree with its session IDs", lead)
	}
	err := a.busHost().With(func(b *bus.Bus) error {
		inside, err := b.InTree(ctx, callerBus, leadBus)
		if err != nil {
			return err
		}
		if !inside {
			return fmt.Errorf("%s is not in your team's tree, so you cannot start a member under it, because a chat builds only under itself, its members and theirs", lead)
		}
		_, retired, err := b.LeadOf(ctx, leadBus)
		if err != nil {
			return err
		}
		if retired {
			return fmt.Errorf("%s is retired; it must be reopened from the window before a member is started under it", lead)
		}
		return nil
	})
	return lead, leadBus, err
}

// The permission rule (Paul's decision 1 of 18 November 2026). A permission mode is a setting the protocol gives the
// category "mode" (sessionhost.AuthorityCategory). No program's documentation, as Dinah records it, puts its modes in
// an order from least to most permissive, so Dinah never decides that one mode is less permissive than another. A new
// member may have the caller's own mode, when it runs the same program as the caller, or the program's default: the
// mode a new chat of that program is given as it is made, which is what dinah.team/start@1 gives every member.

// permittedModes are the mode values a caller may give a new chat of a program for one mode setting: its own on the
// same program, and the program's default when it is known.
func permittedModes(caller *Session, provider string, option acp.ConfigOption, programDefault json.RawMessage) []json.RawMessage {
	var out []json.RawMessage
	if normalizedProvider(caller.Provider()) == normalizedProvider(provider) {
		for _, o := range caller.Options() {
			if o.ID == option.ID && o.Category == sessionhost.AuthorityCategory && hasValue(o.CurrentValue) {
				out = append(out, o.CurrentValue)
			}
		}
	}
	if hasValue(programDefault) && !containsValue(out, programDefault) {
		out = append(out, programDefault)
	}
	return out
}

func containsValue(list []json.RawMessage, v json.RawMessage) bool {
	for _, x := range list {
		if sameValue(x, v) {
			return true
		}
	}
	return false
}

// modeRefusal says why a mode cannot be given and which modes can.
func modeRefusal(provider string, option acp.ConfigOption, value json.RawMessage, permitted []json.RawMessage) error {
	label := func(v json.RawMessage) string {
		text := judgeOptionString(v)
		if name := settingValueName(option, v); name != "" && name != text {
			return fmt.Sprintf("%s (%s)", name, text)
		}
		return text
	}
	var choices []string
	for _, v := range permitted {
		choices = append(choices, label(v))
	}
	setting := option.Name
	if setting == "" {
		setting = option.ID
	}
	reason := fmt.Sprintf("the %s %s cannot be given to the new member, because %s has no documented order of %s's permission modes from least to most permissive, so a member may have only your own mode on the same program or the program's default", setting, label(value), product.Name, providerDisplayName(provider))
	if len(choices) > 0 {
		reason += "; you may give " + strings.Join(choices, " or ") + ", or leave the setting out for the program's default"
	} else {
		reason += "; leave the setting out for the program's default"
	}
	return errors.New(reason)
}

// checkTeamOptions refuses, before anything is started, a setting the program does not offer, a model given among the
// settings, and a permission mode the caller may not give. The program's last list of options says what it offers;
// a setting of a program whose list is not known yet is checked against the new chat's own list once it is made.
func (a *App) checkTeamOptions(caller *Session, provider string, requested map[string]json.RawMessage) error {
	a.mu.Lock()
	catalog := append([]acp.ConfigOption(nil), savedCatalog(a.settings, provider)...)
	a.mu.Unlock()
	if len(catalog) == 0 {
		return nil
	}
	defaults := a.newChatDefaults(provider)
	for _, id := range sortedKeys(requested) {
		value := requested[id]
		var option *acp.ConfigOption
		for i := range catalog {
			if catalog[i].ID == id {
				option = &catalog[i]
			}
		}
		if option == nil {
			var offered []string
			for _, o := range catalog {
				if o.Category != "model" {
					offered = append(offered, o.ID)
				}
			}
			return fmt.Errorf("%s offers no setting %q; the settings it offers are %s", providerDisplayName(provider), id, strings.Join(offered, ", "))
		}
		if option.Category == "model" {
			return fmt.Errorf("give the model as model, not as the setting %q", id)
		}
		if option.Category == sessionhost.AuthorityCategory {
			programDefault, known := defaults[id]
			permitted := permittedModes(caller, provider, *option, programDefault)
			// When Dinah gives the program's new chats no value for the mode, the default is the program's own, known
			// only once the chat is made; the value is checked against it then.
			if !containsValue(permitted, value) && known {
				return modeRefusal(provider, *option, value, permitted)
			}
			continue
		}
		if (option.Type == "select" || option.Type == "boolean") && !agents.Allowed(*option, value) {
			return fmt.Errorf("%s does not offer %s for the setting %q", providerDisplayName(provider), judgeOptionString(value), id)
		}
	}
	return nil
}

// applyTeamOptions gives a new chat its settings the way the window does (SetSessionOption), so the chat's menus show
// them and they are saved with it. The permission rule is applied again with the new chat's own list, whose mode is the
// program's default as the chat was made. A setting that cannot be given is returned with why; nothing more permissive
// than the rule allows is ever set.
func (a *App) applyTeamOptions(chat string, caller *Session, provider string, requested map[string]json.RawMessage) []map[string]string {
	var notApplied []map[string]string
	refuse := func(id string, err error) {
		notApplied = append(notApplied, map[string]string{"id": id, "reason": err.Error()})
	}
	s := a.session(chat)
	if s == nil {
		for _, id := range sortedKeys(requested) {
			refuse(id, errors.New("the new chat is not open"))
		}
		return notApplied
	}
	for _, id := range sortedKeys(requested) {
		value := requested[id]
		var option *acp.ConfigOption
		for _, o := range s.Options() {
			if o.ID == id {
				copy := o
				option = &copy
			}
		}
		switch {
		case option == nil:
			refuse(id, fmt.Errorf("%s offers no setting %q now", providerDisplayName(provider), id))
			continue
		case option.Category == "model":
			refuse(id, fmt.Errorf("give the model as model, not as the setting %q", id))
			continue
		case sameValue(option.CurrentValue, value):
			continue
		case option.Category == sessionhost.AuthorityCategory:
			permitted := permittedModes(caller, provider, *option, option.CurrentValue)
			if !containsValue(permitted, value) {
				refuse(id, modeRefusal(provider, *option, value, permitted))
				continue
			}
		}
		if _, err := a.SetSessionOption(chat, id, value); err != nil {
			refuse(id, err)
		}
	}
	return notApplied
}

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// startLineFor is the line a team's group keeps when a member is started in it. A member started under a chat of the
// caller's tree by the caller says who asked; the brief's first line ends the line either way, which memberRemit
// relies on.
func startLineFor(title, provider, model, brief, requester string) string {
	if requester == "" {
		return startLine(title, provider, model, brief)
	}
	what := providerName(provider)
	if model != "" {
		what += ", " + model
	}
	return fmt.Sprintf("%s (%s), at the request of %s: %s", title, what, requester, firstLine(brief))
}
