package main

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/workspaces"
)

// codexHereOnly says that only Codex is on the computer a test imports on,
// as the stub provider is.
func codexHereOnly(t *testing.T) {
	t.Helper()
	previous := providerHere
	providerHere = func(_ *App, provider string) bool { return provider == "codex" }
	t.Cleanup(func() { providerHere = previous })
}

// otherMain is the main folder the chats of a crafted archive had on the
// computer that exported them.
const otherMain = "C:/Elsewhere/Harbor"

// craftedExport is an export of the workspace Harbor from another computer,
// with the chats given.
func craftedExport(chats ...workspaces.ExportChat) workspaces.Export {
	return workspaces.Export{Format: workspaces.ExportFormat, Name: "Harbor", Exporter: "exp_other",
		Sources: []workspaces.ExportSource{{Key: "s1", Kind: workspaces.LocalFolder, Name: "Harbor", Main: true, Local: &workspaces.ExportLocation{Absolute: otherMain}}},
		Chats:   chats}
}

// writeCrafted writes an archive with the export and a history for each of
// its chats, by key, and returns its file.
func writeCrafted(t *testing.T, export workspaces.Export, histories map[string]*conversationTranscript) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "Harbor"+workspaces.ExportExtension)
	if err := workspaces.WriteArchiveFile(file, func(w io.Writer) error {
		archive := workspaces.NewArchiveWriter(w)
		for _, chat := range export.Chats {
			data, err := json.Marshal(histories[chat.Key])
			if err != nil {
				return err
			}
			if err := archive.WriteHistory(chat.Key, data); err != nil {
				return err
			}
		}
		return archive.Finish(export)
	}); err != nil {
		t.Fatal(err)
	}
	return file
}

// twoStageChat is a Codex chat of two stages, as the other computer saved
// it: its newest stage's history keeps a copy of the first stage's.
func twoStageChat(first, second string) (workspaces.ExportChat, *conversationTranscript) {
	chat := workspaces.ExportChat{Key: "c1", ChatID: first, Stages: []string{first, second}, Provider: "codex", Title: "Checkout Review"}
	tr := &conversationTranscript{SessionID: second, Provider: "codex", Workspace: otherMain, ParentSessionID: first, Complete: true,
		ParentHistory: []ConversationMessage{{Role: "you", Text: "Plan the checkout review."}, {Role: "agent", Text: "Payment screens first."}, {Role: "provider_switch", Text: "Continued with codex"}},
		Messages:      []ConversationMessage{{Role: "you", Text: "And the receipts?"}, {Role: "agent", Text: "Receipts after the payment screens."}}}
	return chat, tr
}

// importOn imports file into a workspace whose main folder is main.
func importOn(t *testing.T, a *App, file, main string, extra ...WorkspaceImportChoice) WorkspaceImportResult {
	t.Helper()
	result, err := a.ImportWorkspace(file, append([]WorkspaceImportChoice{{Key: "s1", Path: main}}, extra...))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// A workspace exported with its chats on one computer is imported on
// another whose folders are elsewhere. The chats come as moved chats, with
// their identities, titles, hidden mark, stages and chat pin, a bus identity
// made on this computer and no native session record, and the paths in
// their histories rewritten to this computer's main folder. Importing the
// same file again changes nothing.
func TestImportMovesChatsToAnotherComputer(t *testing.T) {
	codexHereOnly(t)
	desktop, harbor, _ := harborFixture(t)
	file := filepath.Join(t.TempDir(), "Harbor"+workspaces.ExportExtension)
	if err := desktop.exportWorkspaceTo(harbor.ID, file, WorkspaceExportChoice{Chats: []string{"codex:old", "codex:review"}}); err != nil {
		t.Fatal(err)
	}
	laptop, _ := providerTestApp(t)
	main := filepath.Join(t.TempDir(), "Laptop", "OneDrive", "Harbor")
	makeFolders(t, main, "Checkout")

	plan, err := laptop.PlanWorkspaceImport(file, []WorkspaceImportChoice{{Key: "s1", Path: main}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Chats) != 2 || plan.Chats[0].Mark != importNew || plan.Chats[1].Mark != importNew {
		t.Fatalf("plan: %+v", plan.Chats)
	}
	if !plan.Chats[0].ProviderMissing || plan.Chats[1].ProviderMissing || plan.Chats[0].Title != "Harbor Lead" || plan.Chats[1].Title != "Checkout Review" {
		t.Fatalf("a chat whose provider is not here was not said to be: %+v", plan.Chats)
	}
	result := importOn(t, laptop, file, main)
	if result.Chats != 2 || result.Updated != 0 || result.Separate != 0 {
		t.Fatalf("import: %+v", result)
	}
	lead, err := readTranscript("claude:lead")
	if err != nil {
		t.Fatal(err)
	}
	if lead.Workspace != main {
		t.Fatalf("the lead's history was not moved to this computer's main folder: %s", lead.Workspace)
	}
	last := lead.Messages[len(lead.Messages)-1]
	if last.Role != "provider_switch" || !strings.Contains(last.Text, "This chat moved here from another computer on") || !strings.Contains(last.Text, "new Claude session") {
		t.Fatalf("the moved line is missing: %+v", last)
	}
	review, err := readTranscript("codex:review")
	if err != nil || review.Workspace != filepath.Join(main, "Checkout") {
		t.Fatalf("the hidden chat's folder was not kept relative to the main folder: %+v %v", review, err)
	}
	laptop.mu.Lock()
	s, _ := laptop.settings.clone()
	laptop.mu.Unlock()
	if s.SessionTitles["claude:lead"] != "Harbor Lead" || s.JudgedTitles["codex:review"].Title != "Checkout Review" || !slices.Contains(s.Hidden, "codex:review") {
		t.Fatalf("titles or hidden mark: %+v %+v %+v", s.SessionTitles, s.JudgedTitles, s.Hidden)
	}
	if s.ChatBindings["codex:old"] != "claude:lead" || s.SessionProviders["claude:lead"] != "claude" {
		t.Fatalf("stages: %+v", s.ChatBindings)
	}
	if len(s.NativeSessions) != 0 {
		t.Fatalf("a native session record was copied: %+v", s.NativeSessions)
	}
	recipient := s.BusSessions["claude:lead"]
	if recipient == "" || recipient == "window-lead" {
		t.Fatalf("the bus identity was not made on this computer: %q", recipient)
	}
	if moved := s.MovedChats["claude:lead"]; moved.Exporter == "" || moved.File != filepath.Base(file) || moved.At.IsZero() {
		t.Fatalf("the chat is not recorded as moved here: %+v", moved)
	}
	record := s.WorkspaceRecords[workspaces.Index(s.WorkspaceRecords, result.WorkspaceID)]
	if !slices.ContainsFunc(record.Pins, func(p workspaces.Pin) bool { return p.Kind == "chat" && p.Session == "claude:lead" }) {
		t.Fatalf("the chat pin did not come: %+v", record.Pins)
	}
	if err := laptop.busHost().With(func(b *bus.Bus) error {
		session, err := b.Session(t.Context(), recipient)
		if err == nil && session.Title != "Harbor Lead" {
			t.Errorf("bus identity title: %q", session.Title)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	inventory, err := laptop.SavedChats()
	if err != nil {
		t.Fatal(err)
	}
	listed := []string{}
	for _, chat := range inventory.Chats {
		listed = append(listed, chat.ChatID+"="+chat.ID)
	}
	if !slices.Contains(listed, "codex:old=claude:lead") {
		t.Fatalf("the moved chat is not listed with its logical ID: %v", listed)
	}

	// Importing the same file again finds every chat up to date.
	again, err := laptop.PlanWorkspaceImport(file, []WorkspaceImportChoice{{Key: "s1", Path: main}})
	if err != nil || again.Chats[0].Mark != importUpToDate || again.Chats[1].Mark != importUpToDate || again.ExistingID != result.WorkspaceID {
		t.Fatalf("second plan: %+v %v", again, err)
	}
	twice := importOn(t, laptop, file, main)
	if twice.Chats != 0 || twice.Updated != 0 || twice.Pins != 0 || twice.Sources != 0 {
		t.Fatalf("second import: %+v", twice)
	}
	laptop.mu.Lock()
	after, _ := laptop.settings.clone()
	laptop.mu.Unlock()
	if savedChatsSettingsStamp(after) != savedChatsSettingsStamp(s) {
		t.Fatal("importing the same file again changed the chats")
	}
}

// A chat worked on elsewhere is updated; one changed on both computers is
// left as it is, or added as a separate chat when the person asks; one that
// is open here is not updated until it is closed.
func TestImportUpdatesOrSeparatesAChatAlreadyHere(t *testing.T) {
	codexHereOnly(t)
	a, _ := providerTestApp(t)
	main := filepath.Join(t.TempDir(), "Harbor")
	makeFolders(t, main)
	chat, tr := twoStageChat("codex:first", "codex:second")
	importOn(t, a, writeCrafted(t, craftedExport(chat), map[string]*conversationTranscript{"c1": tr}), main)

	// The other computer went on with a third stage.
	later := chat
	later.Stages = []string{"codex:first", "codex:second", "codex:third"}
	third := *tr
	third.SessionID, third.ParentSessionID = "codex:third", "codex:second"
	third.ParentHistory = append(append([]ConversationMessage{}, tr.ParentHistory...), tr.Messages...)
	third.Messages = []ConversationMessage{{Role: "you", Text: "Done?"}, {Role: "agent", Text: "Done."}}
	updatedFile := writeCrafted(t, craftedExport(later), map[string]*conversationTranscript{"c1": &third})

	// While the chat is open here, it is not updated.
	open := newSessionState(&sessionSeed{ID: "codex:second", Provider: "codex", Cwd: main, Title: "Checkout Review"}, sessionInitial{Open: true})
	a.mu.Lock()
	a.runtime.PutConversationLocked(open.ID(), open)
	a.mu.Unlock()
	plan, err := a.PlanWorkspaceImport(updatedFile, []WorkspaceImportChoice{{Key: "s1", Path: main}})
	if err != nil || plan.Chats[0].Mark != importCloseToUpdate {
		t.Fatalf("an open chat: %+v %v", plan.Chats, err)
	}
	if result := importOn(t, a, updatedFile, main); result.Updated != 0 {
		t.Fatalf("an open chat was updated: %+v", result)
	}
	a.mu.Lock()
	a.runtime.RemoveConversationLocked(open.ID())
	a.mu.Unlock()

	plan, err = a.PlanWorkspaceImport(updatedFile, []WorkspaceImportChoice{{Key: "s1", Path: main}})
	if err != nil || plan.Chats[0].Mark != importUpdated {
		t.Fatalf("a chat worked on elsewhere: %+v %v", plan.Chats, err)
	}
	a.mu.Lock()
	recipient := a.settings.BusSessions["codex:second"]
	a.mu.Unlock()
	if result := importOn(t, a, updatedFile, main); result.Updated != 1 {
		t.Fatalf("update: %+v", result)
	}
	a.mu.Lock()
	s, _ := a.settings.clone()
	a.mu.Unlock()
	if s.ChatBindings["codex:second"] != "codex:third" || s.BusSessions["codex:third"] != recipient || s.BusSessions["codex:second"] != "" {
		t.Fatalf("the update did not continue the chat with its bus identity: %+v %+v", s.ChatBindings, s.BusSessions)
	}
	if _, moved := a.movedHere("codex:third"); !moved {
		t.Fatal("the updated chat is not recorded as moved here")
	}

	// Here, the chat went on its own way from the second stage.
	local := &conversationTranscript{SessionID: "codex:here", Provider: "codex", Workspace: main, ParentSessionID: "codex:third", ParentHistory: third.ParentHistory, Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "Here only."}}}
	if err := writeTranscript(local); err != nil {
		t.Fatal(err)
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.ChatBindings["codex:third"] = "codex:here"
		s.SessionProviders["codex:here"] = "codex"
		s.BusSessions["codex:here"] = s.BusSessions["codex:third"]
		delete(s.BusSessions, "codex:third")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	elsewhere := later
	elsewhere.Stages = []string{"codex:first", "codex:second", "codex:there"}
	there := third
	there.SessionID = "codex:there"
	bothFile := writeCrafted(t, craftedExport(elsewhere), map[string]*conversationTranscript{"c1": &there})
	plan, err = a.PlanWorkspaceImport(bothFile, []WorkspaceImportChoice{{Key: "s1", Path: main}})
	if err != nil || plan.Chats[0].Mark != importChangedOnBoth || plan.Chats[0].Separate {
		t.Fatalf("a chat changed on both computers: %+v %v", plan.Chats, err)
	}
	if result := importOn(t, a, bothFile, main); result.Updated != 0 || result.Separate != 0 || result.Chats != 0 {
		t.Fatalf("a chat changed on both was not left alone: %+v", result)
	}
	if _, err := readTranscript("codex:there"); err == nil {
		t.Fatal("the other computer's stage was written over this computer's chat")
	}
	result := importOn(t, a, bothFile, main, WorkspaceImportChoice{Chat: "c1", Separate: true})
	if result.Separate != 1 {
		t.Fatalf("separate chat: %+v", result)
	}
	a.mu.Lock()
	s, _ = a.settings.clone()
	a.mu.Unlock()
	var copyHead string
	for id, title := range s.SessionTitles {
		if title == "Checkout Review (Imported)" {
			copyHead = id
		}
	}
	if !strings.HasPrefix(copyHead, "codex:imported-") || s.ChatBindings["codex:third"] != "codex:here" || s.BusSessions[copyHead] == "" || s.BusSessions[copyHead] == s.BusSessions["codex:here"] {
		t.Fatalf("the separate chat: %q %+v", copyHead, s.SessionTitles)
	}
	copied, err := readTranscript(copyHead)
	if err != nil || !strings.HasPrefix(copied.ParentSessionID, "codex:imported-") {
		t.Fatalf("the separate chat's history: %+v %v", copied, err)
	}
}

// A moved chat is never loaded by its provider. Opening it is refused with
// what the page needs to show its saved history, and its first message
// continues it in a new session briefed from that history, which becomes the
// chat's next stage. The earlier stage, which is on the other computer, is
// read from the copy the history keeps.
func TestAMovedChatContinuesWithItsFirstMessage(t *testing.T) {
	codexHereOnly(t)
	a, c := providerTestApp(t)
	main := filepath.Join(t.TempDir(), "Harbor")
	makeFolders(t, main)
	chat, tr := twoStageChat("codex:first", "codex:second")
	importOn(t, a, writeCrafted(t, craftedExport(chat), map[string]*conversationTranscript{"c1": tr}), main)

	_, err := a.OpenSessionAt(main, "codex:second")
	var moved *movedChatError
	if !errors.As(err, &moved) || moved.detail.Kind != "moved-chat" || moved.detail.ProviderMissing {
		t.Fatalf("opening a moved chat: %v", err)
	}
	view := a.SavedConversation("codex:second")
	if !view.Available || len(view.Messages) != 6 || !strings.Contains(view.Messages[5].Text, "moved here from another computer") {
		t.Fatalf("saved history: %+v", view)
	}
	result, err := a.ContinueMovedChat("codex:second")
	if err != nil {
		t.Fatal(err)
	}
	next := result.Session.ID
	if next == "codex:second" || result.ParentSessionID != "codex:second" {
		t.Fatalf("continuation: %+v", result)
	}
	handoff, err := a.continuations().PendingContext(a.session(next))
	if err != nil || !strings.Contains(handoff, "Payment screens first.") || !strings.Contains(handoff, "Receipts after the payment screens.") {
		t.Fatalf("the handoff does not hold the saved history: %q %v", handoff, err)
	}
	if err := a.Send(next, "Carry on with the receipts.", nil, nil); err != nil {
		t.Fatal(err)
	}
	if !a.turnIO().Close(3 * time.Second) {
		t.Fatal("the first message did not finish")
	}
	prompted := false
	for _, call := range c.calls.Snapshot() {
		switch call.method {
		case "session/load", "session/resume":
			t.Fatalf("the moved chat was loaded: %s", call.method)
		case "session/prompt":
			p := call.params.(acp.PromptParams)
			text := ""
			for _, block := range p.Prompt {
				text += block.Text
			}
			if p.SessionID == next && strings.Contains(text, "Carry on with the receipts.") && strings.Contains(text, "Payment screens first.") {
				prompted = true
			}
			if strings.Contains(text, "Continue the saved conversation") {
				t.Fatal("the recovery's own message was sent in place of the person's")
			}
		}
	}
	if !prompted {
		t.Fatalf("the first message did not carry the handoff: %+v", c.calls.Snapshot())
	}
	if head, err := a.ResolveSessionContinuation("codex:first"); err != nil || head != next {
		t.Fatalf("the new session is not the chat's next stage: %s %v", head, err)
	}
	if _, still := a.movedHere("codex:second"); still {
		t.Fatal("the chat is still waiting to be continued")
	}
	if _, err := a.ContinueMovedChat("codex:second"); err == nil {
		t.Fatal("a continued chat was continued again")
	}
	// A moved chat whose provider is not here says so when it is opened.
	providerHere = func(*App, string) bool { return false }
	chat2, tr2 := twoStageChat("codex:alpha", "codex:beta")
	chat2.Key = "c1"
	importOn(t, a, writeCrafted(t, craftedExport(chat2), map[string]*conversationTranscript{"c1": tr2}), main)
	if _, err := a.OpenSessionAt(main, "codex:beta"); !errors.As(err, &moved) || !moved.detail.ProviderMissing {
		t.Fatalf("a moved chat whose provider is missing: %v", err)
	}
}
