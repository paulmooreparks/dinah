package main

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"studio/internal/bus"
	"studio/internal/workspaces"
)

// harborFixture makes two workspaces, Harbor and Docks. Harbor has a lead
// chat of two stages (switched from Codex to Claude), a hidden chat in a
// folder inside the main folder, and a chat in the Recycle Bin; Docks has
// one chat. The group Harbor Team has the lead, the Docks chat and the
// person, with a message from the person carrying a file and a reply from
// the lead; the group Docks Only has the Docks chat alone. The lead leads
// the hidden chat and the Docks chat, with a budget. Harbor's pins are a
// chat pin to the lead's older stage, one to the Docks chat, one to the
// chat in the Recycle Bin, and a folder.
func harborFixture(t *testing.T) (a *App, harbor, docks WorkspaceRecord) {
	t.Helper()
	a, _ = providerTestApp(t)
	ws, docksWs := filepath.Join(t.TempDir(), "Harbor"), filepath.Join(t.TempDir(), "Docks")
	makeFolders(t, ws, "Checkout")
	makeFolders(t, docksWs)
	harbor, docks = newWorkspaceRecord(ws), newWorkspaceRecord(docksWs)
	harbor.Name, docks.Name = "Harbor", "Docks"
	cleanupFixtureChat(t, a, "codex:old", "codex", ws, "window-lead")
	cleanupFixtureChat(t, a, "claude:lead", "claude", ws, "window-lead")
	cleanupFixtureChat(t, a, "codex:review", "codex", filepath.Join(ws, "Checkout"), "window-review")
	cleanupFixtureChat(t, a, "codex:binned", "codex", ws, "window-binned")
	cleanupFixtureChat(t, a, "codex:docks", "codex", docksWs, "window-docks")
	harbor.Pins = []workspaces.Pin{{Kind: "chat", Title: "Lead", Session: "codex:old"}, {Kind: "chat", Title: "Docks", Session: "codex:docks"}, {Kind: "chat", Title: "Binned", Session: "codex:binned"},
		{Kind: "resource", Title: "Checkout", Ref: &workspaces.Ref{WorkspaceID: harbor.ID, SourceID: harbor.PrimarySourceID, Path: "Checkout"}, Directory: true}}
	if err := a.updateSettings(func(s *Settings) error {
		s.WorkspaceRecords = []WorkspaceRecord{harbor, docks}
		s.SessionTitles["claude:lead"] = "Harbor Lead"
		s.SessionTitles["codex:docks"] = "Docks Builder"
		delete(s.SessionTitles, "codex:review")
		s.JudgedTitles = map[string]JudgedTitle{"codex:review": {Title: "Checkout Review"}}
		s.ChatBindings = map[string]string{"codex:old": "claude:lead"}
		s.Hidden = []string{"codex:review"}
		s.RemovedChats = map[string]chatRemoval{"codex:binned": {Stage: "retaining"}}
		return ensureWorkspaceMountAliases(s.WorkspaceRecords)
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		session := func(id string) bus.Session {
			s, err := b.Session(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			return s
		}
		lead, docksChat, binned := session("window-lead"), session("window-docks"), session("window-binned")
		if _, err := b.CreateGroup(ctx, bus.Sender{}, "Harbor Team", []string{lead.Address(), docksChat.Address(), binned.Address()}); err != nil {
			return err
		}
		if _, err := b.CreateGroup(ctx, bus.Sender{}, "Docks Only", []string{docksChat.Address()}); err != nil {
			return err
		}
		if err := b.SetGroupBudget(ctx, "Harbor Team", 5); err != nil {
			return err
		}
		posted, err := b.SendFiles(ctx, "#Harbor Team", "Plan the checkout review.", []bus.PersonFile{{Name: "plan.txt", Data: []byte("the plan")}}, 0, nil)
		if err != nil {
			return err
		}
		if _, err := b.SendMentioning(ctx, bus.Sender{Session: &lead}, "#Harbor Team", "Docks Builder takes the receipts.", nil, posted.ID, []string{docksChat.Address()}); err != nil {
			return err
		}
		for _, member := range []string{"window-review", "window-docks"} {
			if _, err := b.SetLead(ctx, member, "window-lead"); err != nil {
				return err
			}
		}
		return b.SetTeamAllowances(ctx, "window-lead", []bus.TeamAllowance{{Provider: "claude", Unit: "tokens", Allowance: 1000}})
	}); err != nil {
		t.Fatal(err)
	}
	return a, harbor, docks
}

// The plan lists Harbor's chats, the hidden one included and the one in
// the Recycle Bin left out, with the group that has a member here and the
// lead's team, and members in Docks marked as elsewhere.
func TestPlanWorkspaceExportGathersTheWorkspacesChats(t *testing.T) {
	a, harbor, _ := harborFixture(t)
	plan, err := a.PlanWorkspaceExport(harbor.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, chat := range plan.Chats {
		ids = append(ids, chat.ChatID)
	}
	if !slices.Equal(ids, []string{"codex:old", "codex:review"}) {
		t.Fatalf("chats: %+v", plan.Chats)
	}
	lead, review := plan.Chats[0], plan.Chats[1]
	if lead.Title != "Harbor Lead" || lead.Provider != "claude" || lead.Size == 0 || lead.Open || lead.Hidden {
		t.Fatalf("lead: %+v", lead)
	}
	if review.Title != "Checkout Review" || !review.Hidden {
		t.Fatalf("hidden chat: %+v", review)
	}
	if plan.Name != "Harbor" || len(plan.Sources) != 1 || !plan.Sources[0].Main || plan.Pins != 1 {
		t.Fatalf("sources and pins: %+v", plan)
	}
	// Setting a lead made the team's own group, which has the lead too.
	if len(plan.Groups) != 2 || plan.Groups[0].Title != "Harbor Team" || len(plan.Groups[0].Members) != 2 || len(plan.Groups[1].Members) != 3 {
		t.Fatalf("groups: %+v", plan.Groups)
	}
	if m := plan.Groups[0].Members; !m[0].Here || m[0].ChatID != "codex:old" || m[1].Here || m[1].Workspace != "Docks" || m[1].Title != "Docks Builder" {
		t.Fatalf("group members: %+v", m)
	}
	if len(plan.Teams) != 1 || plan.Teams[0].Lead.ChatID != "codex:old" || len(plan.Teams[0].Members) != 2 || plan.Teams[0].Members[1].Here {
		t.Fatalf("teams: %+v", plan.Teams)
	}
}

// The archive takes the chats chosen with their stages, history, folder
// and pins, the group with its member in Docks referred to without a key,
// its conversation and file, and the team with its budget. Exporting again
// records the same exporter identity.
func TestExportWorkspaceWritesItsChatsGroupsAndTeams(t *testing.T) {
	a, harbor, _ := harborFixture(t)
	file := filepath.Join(t.TempDir(), "Harbor"+workspaces.ExportExtension)
	if err := a.exportWorkspaceTo(harbor.ID, file, WorkspaceExportChoice{Chats: []string{"codex:old", "codex:review", "codex:docks", "codex:binned"}}); err != nil {
		t.Fatal(err)
	}
	archive, err := workspaces.OpenArchive(file)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	e := archive.Export
	if len(e.Chats) != 2 {
		t.Fatalf("only Harbor's chats outside the Recycle Bin go: %+v", e.Chats)
	}
	lead, review := e.Chats[0], e.Chats[1]
	if lead.Key != "c1" || lead.ChatID != "codex:old" || !slices.Equal(lead.Stages, []string{"codex:old", "claude:lead"}) || lead.Provider != "claude" || lead.Title != "Harbor Lead" || lead.Folder != "" {
		t.Fatalf("lead: %+v", lead)
	}
	if review.Key != "c2" || !review.Hidden || review.Folder != "Checkout" || review.JudgedTitle != "Checkout Review" || review.Title != "" {
		t.Fatalf("hidden chat: %+v", review)
	}
	history, err := archive.History("c1")
	if err != nil || !strings.Contains(string(history), `"sessionId":"claude:lead"`) || !strings.Contains(string(history), "Keep this invented answer") {
		t.Fatalf("history: %s %v", history, err)
	}
	chatPins := []string{}
	for _, pin := range e.Pins {
		if pin.Kind == "chat" {
			chatPins = append(chatPins, pin.Title+"="+pin.Chat)
		}
	}
	if !slices.Equal(chatPins, []string{"Lead=c1"}) || len(e.Pins) != 2 {
		t.Fatalf("pins: %+v", e.Pins)
	}
	if len(e.Groups) != 2 || e.Groups[1].Lead == nil || e.Groups[1].Lead.Key != "c1" || e.Groups[1].Budget != -1 {
		t.Fatalf("groups: %+v", e.Groups)
	}
	group := e.Groups[0]
	if group.Title != "Harbor Team" || group.Budget != 5 || !group.Person || group.Messages != 2 || len(group.Members) != 2 {
		t.Fatalf("group: %+v", group)
	}
	if m := group.Members[1]; m.ChatID != "codex:docks" || m.Key != "" || m.Workspace != "Docks" || m.Title != "Docks Builder" || group.Members[0].Key != "c1" {
		t.Fatalf("group members: %+v", group.Members)
	}
	messages, err := archive.GroupMessages(group.Key)
	if err != nil || len(messages) != 2 {
		t.Fatalf("messages: %+v %v", messages, err)
	}
	if first := messages[0]; !first.Person || len(first.Attachments) != 1 || first.Attachments[0].File == "" {
		t.Fatalf("the person's message: %+v", first)
	}
	if data, err := archive.File(messages[0].Attachments[0].File); err != nil || string(data) != "the plan" {
		t.Fatalf("attached file: %q %v", data, err)
	}
	if second := messages[1]; second.From == nil || second.From.Key != "c1" || second.ReplyTo != messages[0].ID || len(second.Mentions) != 1 || second.Mentions[0].ChatID != "codex:docks" {
		t.Fatalf("the lead's message: %+v", second)
	}
	if len(e.Teams) != 1 || e.Teams[0].Lead.Key != "c1" || len(e.Teams[0].Members) != 2 || len(e.Teams[0].Budget) != 1 || e.Teams[0].Budget[0].Allowance != 1000 {
		t.Fatalf("teams: %+v", e.Teams)
	}
	if m := e.Teams[0].Members; m[0].Chat.Key != "c2" || m[1].Chat.Key != "" || m[1].Chat.Workspace != "Docks" {
		t.Fatalf("team members: %+v", m)
	}
	identity, err := workspaces.ExporterIdentity(configDir())
	if err != nil || e.Exporter == "" || e.Exporter != identity {
		t.Fatalf("exporter identity: %q %q %v", e.Exporter, identity, err)
	}
}

// Without the lead, the group the hidden chat is not in stays behind, the
// team's own group goes with the lead referred to without a key, and the
// team pair goes without its budget; the lead's chat pin stays behind.
func TestExportWorkspaceLeavesOutUntickedChats(t *testing.T) {
	a, harbor, _ := harborFixture(t)
	file := filepath.Join(t.TempDir(), "Harbor"+workspaces.ExportExtension)
	if err := a.exportWorkspaceTo(harbor.ID, file, WorkspaceExportChoice{Chats: []string{"codex:review"}}); err != nil {
		t.Fatal(err)
	}
	archive, err := workspaces.OpenArchive(file)
	if err != nil {
		t.Fatal(err)
	}
	archive.Close()
	e := archive.Export
	if len(e.Chats) != 1 || e.Chats[0].ChatID != "codex:review" || e.Chats[0].Key != "c1" || len(e.Pins) != 1 {
		t.Fatalf("export: %+v", e)
	}
	if len(e.Groups) != 1 || e.Groups[0].Lead == nil || e.Groups[0].Lead.Key != "" || e.Groups[0].Members[1].Key != "c1" {
		t.Fatalf("groups: %+v", e.Groups)
	}
	if len(e.Teams) != 1 || e.Teams[0].Lead.Key != "" || e.Teams[0].Lead.ChatID != "codex:old" || len(e.Teams[0].Members) != 1 || e.Teams[0].Members[0].Chat.Key != "c1" || len(e.Teams[0].Budget) != 0 {
		t.Fatalf("team: %+v", e.Teams)
	}
	// With no chats, the export has no exporter and no chat records.
	if err := a.exportWorkspaceTo(harbor.ID, file, WorkspaceExportChoice{}); err != nil {
		t.Fatal(err)
	}
	none, err := workspaces.OpenArchive(file)
	if err != nil {
		t.Fatal(err)
	}
	defer none.Close()
	if len(none.Export.Chats) != 0 || len(none.Export.Groups) != 0 || len(none.Export.Teams) != 0 || none.Export.Exporter != "" {
		t.Fatalf("export without chats: %+v", none.Export)
	}
}

// The command line's export with chats, which the window program carries
// out with no window, takes every chat of the workspace from the state on
// disk and leaves the settings as they were.
func TestWorkspaceExportOptionReadsTheStateWithoutChangingIt(t *testing.T) {
	a, harbor, _ := harborFixture(t)
	before, err := readSettingsForExport()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "Harbor"+workspaces.ExportExtension)
	if err := workspaceExportCommand([]string{"--workspace", harbor.ID, "--out", file}); err != nil {
		t.Fatal(err)
	}
	archive, err := workspaces.OpenArchive(file)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.Export.Chats) != 2 || len(archive.Export.Groups) != 2 || len(archive.Export.Teams) != 1 {
		t.Fatalf("export: %+v", archive.Export)
	}
	after, err := readSettingsForExport()
	if err != nil || savedChatsSettingsStamp(before) != savedChatsSettingsStamp(after) {
		t.Fatalf("the export changed the settings: %v", err)
	}
	if err := workspaceExportCommand([]string{"--workspace", "ws_none", "--out", file}); err == nil {
		t.Fatal("an unknown workspace was exported")
	}
	_ = a
}
