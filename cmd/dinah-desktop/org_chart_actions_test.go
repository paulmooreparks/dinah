package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"

	"studio/internal/bus"
)

func orgActionsFixture(t *testing.T) (*App, *testProviderConnection, string) {
	t.Helper()
	orgChartForbidProviderStarts(t)
	a, c := providerTestApp(t)
	ws := t.TempDir()
	for _, id := range []string{"codex:a", "codex:b", "codex:c", "codex:d"} {
		cleanupFixtureChat(t, a, id, "codex", ws, "window-"+id)
	}
	if err := a.updateSettings(func(s *Settings) error { s.WorkspaceRecords = []WorkspaceRecord{newWorkspaceRecord(ws)}; return nil }); err != nil {
		t.Fatal(err)
	}
	return a, c, ws
}

func orgAction(t *testing.T, a *App, id, kind string) OrgChartAction {
	t.Helper()
	v, err := a.OrgChartActions(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range v.Actions {
		if action.Kind == kind {
			return action
		}
	}
	t.Fatalf("missing %s for %s: %+v", kind, id, v)
	return OrgChartAction{}
}

func applyOrg(t *testing.T, a *App, r OrgChartActionRequest) OrgChartActionResult {
	t.Helper()
	r.Token = orgAction(t, a, r.ChatID, r.Kind).Token
	v, err := a.ApplyOrgChartAction(r)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestOrgChartActionsQueryAndCanonicalConversion(t *testing.T) {
	a, c, _ := orgActionsFixture(t)
	if err := a.updateSettings(func(s *Settings) error {
		s.ChatBindings = map[string]string{"codex:a": "codex:b"}
		s.BusSessions["codex:a"] = s.BusSessions["codex:b"]
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	before := len(c.calls.Snapshot())
	v := applyOrg(t, a, OrgChartActionRequest{Kind: "create_team", LeadID: "codex:a", Title: "Invented canonical team", MemberIDs: []string{"codex:c"}})
	if v.Status != "succeeded" || v.GroupID == "" || len(v.Members) != 1 || v.Members[0].ChatID != "codex:c" {
		t.Fatalf("conversion %+v", v)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		lead, _, err := b.LeadOf(context.Background(), "window-codex:c")
		if lead != "window-codex:b" {
			t.Errorf("non-head bus identity %s", lead)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(c.calls.Snapshot()) != before {
		t.Fatal("relation management contacted a provider")
	}
	view, err := a.OrgChartActions("codex:b")
	if err != nil || len(view.Actions) != 0 {
		t.Fatal("physical alias accepted as canonical", view, err)
	}
}

func TestOrgChartActionsAssignmentMovesAndRejectsCycles(t *testing.T) {
	a, _, _ := orgActionsFixture(t)
	for _, r := range []OrgChartActionRequest{
		{Kind: "create_team", LeadID: "codex:a", Title: "First invented team", MemberIDs: []string{"codex:b"}},
		{Kind: "create_team", LeadID: "codex:c", Title: "Second invented team", MemberIDs: []string{"codex:d"}},
		{Kind: "assign_team", ChatID: "codex:b", LeadID: "codex:c"},
		{Kind: "assign_team", ChatID: "codex:b", LeadID: "codex:c"},
	} {
		if v := applyOrg(t, a, r); v.Status != "succeeded" {
			t.Fatal(v)
		}
	}
	view, err := a.OrgChartActions("codex:c")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range view.Actions {
		if v.Kind == "assign_team" && slices.ContainsFunc(v.Chats, func(c OrgChartChoice) bool { return c.ID == "codex:c" }) {
			t.Fatal("self edge offered")
		}
	}
	if _, err := a.SetTeamLead("codex:c", "codex:a"); err == nil {
		t.Fatal("current lead was assigned to another team")
	}
	view, err = a.OrgChartActions("codex:a")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range view.Actions {
		if v.Kind == "assign_team" && slices.ContainsFunc(v.Chats, func(c OrgChartChoice) bool { return c.ID == "codex:a" }) {
			t.Fatal("self assignment offered")
		}
	}
	if _, err := a.SetTeamLead("codex:a", "codex:c"); err != nil {
		t.Fatal("former lead was not assignable after its final member moved", err)
	}
}

func TestOrgChartActionsStaleAndForgedRequestsHaveNoEffect(t *testing.T) {
	a, c, _ := orgActionsFixture(t)
	action := orgAction(t, a, "", "create_group")
	r := OrgChartActionRequest{Kind: "create_group", Token: action.Token, Title: "Invented stale group", MemberIDs: []string{"codex:a"}}
	if _, err := a.SetTeamLead("codex:b", "codex:c"); err != nil {
		t.Fatal(err)
	}
	v, err := a.ApplyOrgChartAction(r)
	if err != nil || v.Status != "refused" {
		t.Fatal(v, err)
	}
	for _, bad := range []OrgChartActionRequest{
		{Kind: "create_group", Title: "Invented forged group", MemberIDs: []string{"outside"}},
		{Kind: "create_group", Title: "Invented extra fields", MemberIDs: []string{"codex:a"}, Provider: "codex"},
		{Kind: "create_group", Title: "Invented duplicate member", MemberIDs: []string{"codex:a", "codex:a"}},
	} {
		if v := applyOrg(t, a, bad); v.Status != "refused" {
			t.Fatal(v)
		}
	}
	if len(c.calls.Snapshot()) != 0 {
		t.Fatal("refusals contacted provider")
	}
}

func TestOrgChartActionsTokensIgnoreTitleChanges(t *testing.T) {
	a, _, _ := orgActionsFixture(t)
	before := orgAction(t, a, "codex:a", "create_group").Token
	if err := a.updateSettings(func(s *Settings) error { s.SessionTitles["codex:a"] = "Changed invented title"; return nil }); err != nil {
		t.Fatal(err)
	}
	if after := orgAction(t, a, "codex:a", "create_group").Token; after != before {
		t.Fatal("display-only change invalidated form")
	}
}

func TestOrgChartActionsMakeLeadAndPartialRecovery(t *testing.T) {
	a, c, ws := orgActionsFixture(t)
	// A new team keeps a member's existing reporting line as an explicit left-out outcome.
	if _, err := a.SetTeamLead("codex:b", "codex:c"); err != nil {
		t.Fatal(err)
	}
	ordinary := applyOrg(t, a, OrgChartActionRequest{Kind: "create_group", Title: "Invented partial team", MemberIDs: []string{"codex:a", "codex:b", "codex:d"}})
	v := applyOrg(t, a, OrgChartActionRequest{Kind: "make_lead", ChatID: "codex:a", GroupID: ordinary.GroupID})
	if v.Status != "partial" || len(v.Members) != 2 || !slices.ContainsFunc(v.Members, func(m OrgChartMemberOutcome) bool {
		return m.ChatID == "codex:b" && m.Status == "left_out" && m.Reason != ""
	}) {
		t.Fatal(v)
	}
	// Existing team channels cannot be relabelled as an ordinary group conversion.
	view, err := a.OrgChartActions("codex:d")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range view.Actions {
		if action.Kind == "make_lead" && slices.ContainsFunc(action.Groups, func(g OrgChartChoice) bool { return g.ID == v.GroupID }) {
			t.Fatal("existing team leadership offered")
		}
	}
	// Interleave another legal team creation after a group commit, before conversion.
	cleanupFixtureChat(t, a, "codex:e", "codex", ws, "window-codex:e")
	cleanupFixtureChat(t, a, "codex:f", "codex", ws, "window-codex:f")
	r := OrgChartActionRequest{Kind: "create_team", LeadID: "codex:e", Title: "Invented retained group", MemberIDs: []string{"codex:f"}}
	r.Token = orgAction(t, a, "", "create_team").Token
	partial, err := a.applyOrgChartAction(r, func(string) {
		if _, err := a.SetTeamLead("codex:f", "codex:e"); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || partial.Status != "partial" || partial.GroupID == "" || !strings.Contains(partial.Message, "retained") {
		t.Fatal(partial, err)
	}
	if len(partial.Members) != 1 || partial.Members[0].ChatID != "codex:f" || partial.Members[0].Status != "left_out" || partial.Members[0].Reason == "" {
		t.Fatal("conversion failure lost member outcomes", partial)
	}
	room, err := a.BusGroup(partial.GroupID)
	if err != nil || room.Lead != "" {
		t.Fatal("retained ordinary group missing", room, err)
	}
	// Recovery can choose a different eligible member of that same retained group.
	if err := a.ClearTeamLead("codex:f"); err != nil {
		t.Fatal(err)
	}
	recovered := applyOrg(t, a, OrgChartActionRequest{Kind: "make_lead", ChatID: "codex:f", GroupID: partial.GroupID})
	if recovered.Status != "succeeded" || recovered.GroupID != partial.GroupID {
		t.Fatal(recovered)
	}
	if len(c.calls.Snapshot()) != 0 {
		t.Fatal("composition started a provider")
	}
}

func TestOrgChartActionsStartIsExplicitAndNeverPrompts(t *testing.T) {
	a, c, _ := orgActionsFixture(t)
	action := orgAction(t, a, "", "start_chat")
	if len(c.calls.Snapshot()) != 0 {
		t.Fatal("query contacted provider")
	}
	r := OrgChartActionRequest{Kind: "start_chat", WorkspaceID: action.Workspaces[0].ID, Provider: "codex"}
	v := applyOrg(t, a, r)
	if v.Status != "succeeded" || v.Created == nil || v.Created.ID == "" {
		t.Fatal(v)
	}
	newCalls := 0
	for _, call := range c.calls.Snapshot() {
		if call.method == "session/new" {
			newCalls++
		}
		if call.method == "session/prompt" {
			t.Fatal("start sent a prompt")
		}
	}
	if newCalls != 1 {
		t.Fatalf("positive creation control called new %d times", newCalls)
	}
	c.newErr = errors.New("invented allocation failure")
	v = applyOrg(t, a, r)
	if v.Status != "failed" || v.Created != nil || !strings.Contains(v.Message, "invented allocation failure") {
		t.Fatal(v)
	}
}

func TestOrgChartActionsDeleteUsesWholeChainAndRefusals(t *testing.T) {
	a, c, _ := orgActionsFixture(t)
	if err := a.updateSettings(func(s *Settings) error {
		s.ChatBindings = map[string]string{"codex:a": "codex:b"}
		s.BusSessions["codex:a"] = s.BusSessions["codex:b"]
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	v := applyOrg(t, a, OrgChartActionRequest{Kind: "delete_chat", ChatID: "codex:a"})
	if v.Status != "succeeded" || v.Cleanup == nil || !v.Cleanup.OK {
		t.Fatal(v)
	}
	for _, id := range []string{"codex:a", "codex:b"} {
		if !a.removedChat(id) {
			t.Fatalf("member %s not binned", id)
		}
	}
	if a.removedChat("codex:c") {
		t.Fatal("unrelated chat binned")
	}
	if len(c.calls.Snapshot()) != 0 {
		t.Fatal("closed staging touched provider")
	}
	a.transferRunning.Store("codex:c", true)
	defer a.transferRunning.Delete("codex:c")
	view, err := a.OrgChartActions("codex:c")
	if err != nil || len(view.Actions) != 0 {
		t.Fatal("allocation actions offered", view, err)
	}
	if _, err := a.PinWorkspaceItem(a.settings.WorkspaceRecords[0].ID, WorkspacePin{Kind: "chat", Session: "codex:d", Title: "Invented pinned chat"}); err != nil {
		t.Fatal(err)
	}
	view, err = a.OrgChartActions("codex:d")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range view.Actions {
		if action.Kind == "delete_chat" {
			t.Fatal("pinned deletion offered")
		}
	}
	if !slices.ContainsFunc(view.Unavailable, func(v OrgChartUnavailable) bool { return v.Kind == "delete_chat" && v.Reason != "" }) {
		t.Fatal("missing pinned reason", view)
	}
}

func TestOrgChartActionsQueryDoesNotWriteAndAddressesRoundTrip(t *testing.T) {
	a, c, _ := orgActionsFixture(t)
	var before int64
	if err := a.busHost().With(func(b *bus.Bus) error { var e error; before, e = b.ChangeCounter(context.Background()); return e }); err != nil {
		t.Fatal(err)
	}
	v, err := a.OrgChartActions("codex:a")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		n, e := b.ChangeCounter(context.Background())
		if n != before {
			t.Error("query changed bus state")
		}
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if len(c.calls.Snapshot()) != 0 {
		t.Fatal("query contacted provider")
	}
	for _, action := range v.Actions {
		u, e := url.Parse(action.Href)
		if e != nil {
			t.Fatal(e)
		}
		state, e := url.ParseQuery(u.Query().Get("view.org-chart"))
		if e != nil || state.Get("action") != action.Kind || state.Get("chat") != "codex:a" || strings.Contains(action.Href, action.Token) {
			t.Fatal(action.Href, e)
		}
	}
	u, _ := url.Parse(orgChartActionHref("make_lead", "codex:a", "retained-group"))
	state, _ := url.ParseQuery(u.Query().Get("view.org-chart"))
	if state.Get("group") != "retained-group" {
		t.Fatal(u)
	}
	u, _ = url.Parse(orgChartGroupHref("invented-group"))
	raw, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(u.Query().Get("open"), "item-"))
	if e != nil {
		t.Fatal(e)
	}
	var item []string
	if json.Unmarshal(raw, &item) != nil || !slices.Equal(item, []string{"bus-room", "", "invented-group"}) {
		t.Fatal(string(raw))
	}
}

func TestOrgChartActionsDuplicateGroupAndBuslessChat(t *testing.T) {
	a, _, _ := orgActionsFixture(t)
	r := OrgChartActionRequest{Kind: "create_group", Title: "Invented duplicate title", MemberIDs: []string{"codex:a"}}
	if v := applyOrg(t, a, r); v.Status != "succeeded" {
		t.Fatal(v)
	}
	if v := applyOrg(t, a, r); v.Status != "failed" || !strings.Contains(v.Message, "already exists") {
		t.Fatal(v)
	}
	if err := a.updateSettings(func(s *Settings) error { delete(s.BusSessions, "codex:d"); return nil }); err != nil {
		t.Fatal(err)
	}
	if v := applyOrg(t, a, OrgChartActionRequest{Kind: "delete_chat", ChatID: "codex:d"}); v.Status != "succeeded" {
		t.Fatal("local chat without a bus place could not be binned", v)
	}
}

func TestOrgChartActionsWorkingAndSharedQueueRefusals(t *testing.T) {
	a, _, ws := orgActionsFixture(t)
	a.runtime.PutConversationLocked("codex:a", newSessionState(&sessionSeed{ID: "codex:a", Cwd: ws, Provider: "codex"}, sessionInitial{Open: true, Working: true}))
	v, err := a.OrgChartActions("codex:a")
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(v.Actions, func(v OrgChartAction) bool { return v.Kind == "delete_chat" }) || !slices.ContainsFunc(v.Unavailable, func(v OrgChartUnavailable) bool {
		return v.Kind == "delete_chat" && strings.Contains(v.Reason, "working")
	}) {
		t.Fatal(v)
	}
	if err := a.updateSettings(func(s *Settings) error { s.BusSessions["codex:b"] = s.BusSessions["codex:c"]; return nil }); err != nil {
		t.Fatal(err)
	}
	v, err = a.OrgChartActions("codex:b")
	if err != nil || len(v.Actions) != 0 {
		t.Fatal("ambiguous bus queue granted management", v, err)
	}
}
