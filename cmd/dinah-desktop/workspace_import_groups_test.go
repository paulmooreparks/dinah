package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"studio/internal/bus"
	"studio/internal/workspaces"
)

// sharedTeamArchive is the export of one of two workspaces, Harbor or Docks,
// whose chats share the group Shared Team, led by Harbor's chat, which also
// leads Docks's chat as a team. Each export lists the whole group and the
// same two messages, as one computer's exports of the two workspaces do;
// the first carries a file. Harbor's chat received the second message and a
// direct message that did not travel.
func sharedTeamArchive(t *testing.T, workspace string) string {
	t.Helper()
	plan := []byte("the plan")
	sum := sha256.Sum256(plan)
	harbor := workspaces.ChatRef{ChatID: "codex:harbor", Title: "Harbor Lead", Workspace: "Harbor"}
	docks := workspaces.ChatRef{ChatID: "codex:docks", Title: "Docks Builder", Workspace: "Docks"}
	own, other := harbor, docks
	if workspace == "Docks" {
		own, other = docks, harbor
	}
	own.Key = "c1"
	export := workspaces.Export{Format: workspaces.ExportFormat, Name: workspace, Exporter: "exp_desktop",
		Sources: []workspaces.ExportSource{{Key: "s1", Kind: workspaces.LocalFolder, Name: workspace, Main: true, Local: &workspaces.ExportLocation{Absolute: "C:/Desk/" + workspace}}},
		Chats:   []workspaces.ExportChat{{Key: "c1", ChatID: own.ChatID, Stages: []string{own.ChatID}, Provider: "codex", Title: own.Title}},
		Links:   []workspaces.ExportLink{{A: "Docks", B: "Harbor"}},
	}
	lead, member := harbor, docks
	if workspace == "Harbor" {
		lead = own
	} else {
		member = own
	}
	export.Groups = []workspaces.ExportGroup{{Key: "g1", Title: "Shared Team", Lead: &lead, Budget: 4, Person: true, Members: []workspaces.ChatRef{own, other}, Messages: 2}}
	export.Teams = []workspaces.ExportTeam{{Lead: lead, Members: []workspaces.ExportTeamMember{{Chat: member}}, Budget: []workspaces.ExportAllowance{{Provider: "codex", Unit: "tokens", Allowance: 500}}}}
	tr := &conversationTranscript{SessionID: own.ChatID, Provider: "codex", Workspace: "C:/Desk/" + workspace, Complete: true, Messages: []ConversationMessage{
		{Role: "delivered", Text: "Docks takes the receipts.", Delivered: &DeliveredMessage{ID: 11, Kind: "text", From: "Harbor Lead", Room: "shared-team", RoomTitle: "Shared Team"}},
		{Role: "delivered", Text: "A direct note.", Delivered: &DeliveredMessage{ID: 99, Kind: "text", From: "Elsewhere"}},
		{Role: "agent", Text: "On it."}}}
	file := filepath.Join(t.TempDir(), workspace+workspaces.ExportExtension)
	if err := workspaces.WriteArchiveFile(file, func(w io.Writer) error {
		archive := workspaces.NewArchiveWriter(w)
		data, _ := json.Marshal(tr)
		if err := archive.WriteHistory("c1", data); err != nil {
			return err
		}
		entry, err := archive.WriteFile(plan)
		if err != nil {
			return err
		}
		if err := archive.WriteGroup("g1", []workspaces.GroupMessage{
			{ID: 10, CreatedAt: "2026-11-17T21:04:00Z", Kind: "text", Person: true, FromName: "paul", Text: "Plan the receipts.", Attachments: []workspaces.GroupAttachment{{Name: "plan.txt", Size: int64(len(plan)), SHA256: hex.EncodeToString(sum[:]), File: entry}}},
			{ID: 11, CreatedAt: "2026-11-17T21:06:00Z", Kind: "text", From: &harbor, FromName: "Harbor Lead", Text: "Docks takes the receipts.", ReplyTo: 10, Mentions: []workspaces.ChatRef{docks}},
		}); err != nil {
			return err
		}
		return archive.Finish(export)
	}); err != nil {
		t.Fatal(err)
	}
	return file
}

// groupState is what a computer has of Shared Team: its members, by logical
// chat ID, its messages, Docks's lead, and Harbor's team budget.
type groupState struct {
	members  []string
	messages []string
	lead     string
	budget   int
}

func sharedTeamState(t *testing.T, a *App) groupState {
	t.Helper()
	a.mu.Lock()
	s, _ := a.settings.clone()
	a.mu.Unlock()
	chatOf := map[string]string{}
	for session, recipient := range s.BusSessions {
		chatOf[recipient] = session
	}
	var out groupState
	if err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		room, err := b.Room(ctx, "Shared Team")
		if err != nil {
			return err
		}
		for _, m := range room.Members {
			if m.Kind == "session" {
				out.members = append(out.members, chatOf[m.Key])
			}
		}
		slices.Sort(out.members)
		messages, err := b.RoomMessages(ctx, room.Name, 100)
		if err != nil {
			return err
		}
		for _, m := range messages {
			out.messages = append(out.messages, m.Text)
		}
		lead, _, err := b.LeadOf(ctx, s.BusSessions["codex:docks"])
		if err != nil {
			return err
		}
		out.lead = chatOf[lead]
		allowances, err := b.TeamAllowances(ctx, s.BusSessions["codex:harbor"])
		for _, a := range allowances {
			out.budget += a.Allowance
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// Two workspaces whose chats share a group and a team give the same group,
// members, team and conversation whichever is imported first. Before the
// second, its chat waits for its workspace and nothing is stored for it.
func TestGroupsAcrossTwoWorkspacesImportInEitherOrder(t *testing.T) {
	codexHereOnly(t)
	harborFile, docksFile := sharedTeamArchive(t, "Harbor"), sharedTeamArchive(t, "Docks")
	var states []groupState
	for _, order := range [][2]string{{"Harbor", "Docks"}, {"Docks", "Harbor"}} {
		a, _ := providerTestApp(t)
		files := map[string]string{"Harbor": harborFile, "Docks": docksFile}
		first := order[0]
		main := filepath.Join(t.TempDir(), first)
		makeFolders(t, main)
		plan, err := a.PlanWorkspaceImport(files[first], []WorkspaceImportChoice{{Key: "s1", Path: main}})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Groups) != 1 || plan.Groups[0].Existing || plan.Groups[0].InFile != 1 || len(plan.Groups[0].Waiting) != 1 || plan.Groups[0].New != 2 {
			t.Fatalf("%s first, plan: %+v", first, plan.Groups)
		}
		if len(plan.Links) != 1 || plan.Links[0].A != "Docks" || plan.Links[0].B != "Harbor" || len(plan.Teams) != 1 || len(plan.Teams[0].Waiting) != 1 {
			t.Fatalf("%s first, links and teams: %+v %+v", first, plan.Links, plan.Teams)
		}
		result := importOn(t, a, files[first], main)
		if result.Groups != 1 || result.Messages != 2 {
			t.Fatalf("%s first: %+v", first, result)
		}
		half := sharedTeamState(t, a)
		if len(half.members) != 1 || half.lead != "" {
			t.Fatalf("%s first, a member that is not here was stored: %+v", first, half)
		}
		second := order[1]
		main2 := filepath.Join(t.TempDir(), second)
		makeFolders(t, main2)
		plan, err = a.PlanWorkspaceImport(files[second], []WorkspaceImportChoice{{Key: "s1", Path: main2}})
		if err != nil || !plan.Groups[0].Existing || plan.Groups[0].Here != 1 || len(plan.Groups[0].Waiting) != 0 || plan.Groups[0].New != 0 {
			t.Fatalf("%s second, plan: %+v %v", second, plan.Groups, err)
		}
		result = importOn(t, a, files[second], main2)
		if result.Messages != 0 {
			t.Fatalf("a message came twice: %+v", result)
		}
		states = append(states, sharedTeamState(t, a))
		// Links are never imported.
		if err := a.busHost().With(func(b *bus.Bus) error {
			links, err := b.Links(context.Background())
			if len(links) != 0 {
				t.Errorf("a trusted link was imported: %+v", links)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, state := range states {
		if !slices.Equal(state.members, []string{"codex:docks", "codex:harbor"}) || len(state.messages) != 2 || state.lead != "codex:harbor" || state.budget != 500 {
			t.Fatalf("group state: %+v", state)
		}
	}
	if !slices.Equal(states[0].members, states[1].members) || !slices.Equal(states[0].messages, states[1].messages) {
		t.Fatalf("the order of the imports changed the group: %+v %+v", states[0], states[1])
	}
}

// The imported messages keep their file in the bus's own store, their reply
// and their mention, and the reference in the chat's history to a message
// that travelled points at its ID here; one to a message that did not
// travel keeps its text and loses its link. Importing the file again adds no
// message.
func TestImportedMessagesAreDeduplicatedAndReferencesRewritten(t *testing.T) {
	codexHereOnly(t)
	a, _ := providerTestApp(t)
	main := filepath.Join(t.TempDir(), "Harbor")
	makeFolders(t, main)
	file := sharedTeamArchive(t, "Harbor")
	importOn(t, a, file, main)
	var person, lead bus.Message
	if err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		room, err := b.Room(ctx, "Shared Team")
		if err != nil {
			return err
		}
		messages, err := b.RoomMessages(ctx, room.Name, 100)
		if err != nil {
			return err
		}
		for _, m := range messages {
			switch m.Text {
			case "Plan the receipts.":
				person = m
			case "Docks takes the receipts.":
				lead = m
			}
		}
		if room.Budget != 4 {
			t.Errorf("the new group's turn budget: %d", room.Budget)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if person.ID == 0 || person.FromSession != "" || len(person.Attachments) != 1 || !person.Attachments[0].Stored {
		t.Fatalf("the person's message: %+v", person)
	}
	if data, err := os.ReadFile(person.Attachments[0].Path); err != nil || string(data) != "the plan" {
		t.Fatalf("the attached file was not kept in the bus's store: %q %v", data, err)
	}
	if lead.ID == 0 || lead.ReplyTo != person.ID || lead.FromSession == "" {
		t.Fatalf("the lead's message: %+v", lead)
	}
	tr, err := readTranscript("codex:harbor")
	if err != nil {
		t.Fatal(err)
	}
	travelled, direct := tr.Messages[0].Delivered, tr.Messages[1].Delivered
	if travelled.ID != lead.ID || tr.Messages[0].Text != "Docks takes the receipts." || travelled.Room == "" {
		t.Fatalf("the reference to a message that travelled: %+v", travelled)
	}
	if direct.ID != 0 || tr.Messages[1].Text != "A direct note." {
		t.Fatalf("the reference to a message that did not travel: %+v", direct)
	}
	again := importOn(t, a, file, main)
	if again.Messages != 0 {
		t.Fatalf("the second import added messages: %+v", again)
	}
}
