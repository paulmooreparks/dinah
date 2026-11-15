package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"

	"studio/internal/bus"
	"studio/internal/workspaces"
)

func orgChartGroupHref(group string) string {
	data, _ := json.Marshal([]string{"bus-room", "", group})
	return "?" + url.Values{"open": {"item-" + base64.RawURLEncoding.EncodeToString(data)}}.Encode()
}

func (a *App) ApplyOrgChartAction(r OrgChartActionRequest) (OrgChartActionResult, error) {
	return a.applyOrgChartAction(r, nil)
}

// The internal continuation permits deterministic composition-failure fixtures.
// The page entry always supplies nil and exposes no fixture capability.
func (a *App) applyOrgChartAction(r OrgChartActionRequest, afterGroup func(string)) (OrgChartActionResult, error) {
	out := OrgChartActionResult{Kind: r.Kind, ChatID: r.ChatID, Status: "refused", Members: []OrgChartMemberOutcome{}, Recovery: []OrgChartRecoveryTarget{}}
	// Recycle mutations own this lock themselves. Relation changes share it so a
	// chat cannot enter the bin between their admission check and dispatch.
	if r.Kind != "delete_chat" && r.Kind != "start_chat" {
		a.chatCleanupMu.Lock()
		defer a.chatCleanupMu.Unlock()
	}
	x, err := a.orgChartActionSnapshot()
	if err != nil {
		return out, err
	}
	view, err := a.orgChartActions(x, r.ChatID)
	if err != nil {
		return out, err
	}
	var action *OrgChartAction
	for i := range view.Actions {
		if view.Actions[i].Kind == r.Kind {
			action = &view.Actions[i]
			break
		}
	}
	if action == nil {
		out.Message = "This action is unavailable. Refresh the chart."
		for _, unavailable := range view.Unavailable {
			if unavailable.Kind == r.Kind {
				out.Message = unavailable.Reason
				break
			}
		}
		return out, nil
	}
	if r.Token == "" || r.Token != action.Token {
		out.Message = "The chart's ownership or relations changed. Refresh the form before submitting."
		return out, nil
	}
	if err := validateOrgChartRequest(r, *action); err != nil {
		out.Message = err.Error()
		return out, nil
	}
	refs := func(ids []string) []BusChatRef {
		out := make([]BusChatRef, 0, len(ids))
		for _, id := range ids {
			c := x.chats[id]
			out = append(out, BusChatRef{ID: c.Head, Cwd: c.Workspace, Title: c.Title})
		}
		return out
	}
	out.Status = "succeeded"
	convert := func(group, lead string) {
		made, e := a.MakeGroupATeam(group, x.chats[lead].Head)
		out.GroupID = group
		out.Recovery = append(out.Recovery, OrgChartRecoveryTarget{Kind: "group", ID: group, Href: orgChartGroupHref(group), Label: "Open the retained group"})
		if e != nil {
			out.Status = "partial"
			out.Message = "The group was retained, but it could not be made a team: " + e.Error()
			members := r.MemberIDs
			if r.Kind == "make_lead" {
				for _, candidate := range x.view.Groups {
					if candidate.ID == group {
						members = candidate.MemberIDs
						break
					}
				}
			}
			for _, id := range members {
				if id != lead {
					out.Members = append(out.Members, OrgChartMemberOutcome{ChatID: id, Status: "left_out", Reason: e.Error()})
				}
			}
			if fresh, e := a.OrgChartActions(lead); e == nil {
				for _, v := range fresh.Actions {
					if v.Kind == "make_lead" && slices.ContainsFunc(v.Groups, func(c OrgChartChoice) bool { return c.ID == group }) {
						out.Recovery = append(out.Recovery, OrgChartRecoveryTarget{Kind: "make_lead", ID: group, Href: orgChartActionHref("make_lead", lead, group), Label: "Make this group a team"})
					}
				}
			}
			return
		}
		canonical := func(busID string) string { return x.aliases[busID] }
		for _, m := range made.Made {
			out.Members = append(out.Members, OrgChartMemberOutcome{ChatID: canonical(m.Bus), Status: "assigned"})
		}
		for _, m := range made.LeftOut {
			out.Members = append(out.Members, OrgChartMemberOutcome{ChatID: canonical(m.Bus), Status: "left_out", Reason: m.Why})
			out.Status = "partial"
		}
		out.Message = "The group is now a team."
		if out.Status == "partial" {
			out.Message = "The team was created. Some chats kept their existing reporting lines; review each outcome."
		}
	}
	switch r.Kind {
	case "assign_team":
		out.GroupID, err = a.SetTeamLead(x.chats[r.ChatID].Head, x.chats[r.LeadID].Head)
		if err == nil {
			out.Members = append(out.Members, OrgChartMemberOutcome{ChatID: r.ChatID, Status: "assigned"})
		}
	case "make_lead":
		convert(r.GroupID, r.ChatID)
	case "replace_team_lead":
		for _, group := range x.view.Groups {
			if group.ID != r.GroupID {
				continue
			}
			err = a.busHost().With(func(b *bus.Bus) error {
				return b.ReplaceTeamLead(a.bindingContext(), group.ID, x.busIDs[group.OwnerID], x.busIDs[r.ChatID])
			})
			if err == nil {
				a.teamsActive.Store(true)
				a.teamsChanged()
				out.GroupID = group.ID
				out.Message = "The team has a new lead. Its former lead is now a member."
				out.Recovery = append(out.Recovery, OrgChartRecoveryTarget{Kind: "group", ID: group.ID, Href: orgChartGroupHref(group.ID), Label: "Open the team chat"})
			}
			break
		}
	case "create_group", "create_team":
		ids := append([]string{}, r.MemberIDs...)
		if r.Kind == "create_team" {
			ids = append(ids, r.LeadID)
		}
		room, e := a.BusStartGroup(r.Title, refs(ids))
		err = e
		if err == nil {
			out.GroupID = room.Name
			if afterGroup != nil {
				afterGroup(room.Name)
			}
			if r.Kind == "create_team" {
				convert(room.Name, r.LeadID)
			} else {
				out.Recovery = append(out.Recovery, OrgChartRecoveryTarget{Kind: "group", ID: room.Name, Href: orgChartGroupHref(room.Name), Label: "Open the group"})
			}
		}
	case "start_chat":
		for _, w := range x.saved.WorkspaceRecords {
			if w.ID == r.WorkspaceID {
				var info SessionInfo
				info, err = a.NewSessionWithProviderAt(workspaces.ExecutionPath(w), r.Provider)
				if err == nil {
					out.Created = &info
					out.ChatID = info.ID
				}
				break
			}
		}
	case "delete_chat":
		results := a.RemoveChats([]string{r.ChatID})
		out.Cleanup = &results[0]
		if !results[0].OK {
			out.Status = "refused"
			out.Message = results[0].Error
		}
	}
	if err != nil {
		out.Status = "failed"
		out.Message = err.Error()
	}
	return out, nil
}

func validateOrgChartRequest(r OrgChartActionRequest, v OrgChartAction) error {
	has := func(choices []OrgChartChoice, id string) bool {
		return id != "" && slices.ContainsFunc(choices, func(c OrgChartChoice) bool { return c.ID == id })
	}
	bad := func() error {
		return fmt.Errorf("The submitted fields or choices are not offered for this action. Refresh the form.")
	}
	switch r.Kind {
	case "assign_team":
		if !has(v.Chats, r.LeadID) {
			return bad()
		}
		r.LeadID = ""
	case "make_lead", "replace_team_lead":
		if !has(v.Groups, r.GroupID) {
			return bad()
		}
		r.GroupID = ""
	case "create_team":
		if !has(v.Leads, r.LeadID) || (r.ChatID != "" && r.LeadID != r.ChatID) {
			return bad()
		}
		if len(r.MemberIDs) == 0 || slices.Contains(r.MemberIDs, r.LeadID) {
			return bad()
		}
		r.LeadID = ""
		fallthrough
	case "create_group":
		if len(r.MemberIDs) == 0 {
			return bad()
		}
		seen := map[string]bool{}
		for _, id := range r.MemberIDs {
			if !has(v.Chats, id) || seen[id] {
				return bad()
			}
			seen[id] = true
		}
		r.MemberIDs = nil
		r.Title = ""
	case "start_chat":
		if !has(v.Workspaces, r.WorkspaceID) || !has(v.Providers, r.Provider) {
			return bad()
		}
		r.WorkspaceID = ""
		r.Provider = ""
	case "delete_chat":
	default:
		return bad()
	}
	if r.LeadID != "" || r.GroupID != "" || r.Title != "" || r.WorkspaceID != "" || r.Provider != "" || len(r.MemberIDs) > 0 {
		return bad()
	}
	return nil
}
