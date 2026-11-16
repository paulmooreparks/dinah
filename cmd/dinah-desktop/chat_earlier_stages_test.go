package main

import (
	"context"
	"os"
	"slices"
	"sort"
	"testing"

	"studio/internal/sessionhost"
)

// earlierStageFixture is a chat with the recipient given, whose saved
// conversation continues parent. Nothing in the settings links it to parent,
// which is how switches made before bindings were recorded left their stages.
func earlierStageFixture(t *testing.T, a *App, id, provider, workspace, recipient, parent string) {
	t.Helper()
	cleanupFixtureChat(t, a, id, provider, workspace, recipient)
	if err := writeTranscript(&sessionhost.Transcript{SessionID: id, Provider: provider, Workspace: workspace, ParentSessionID: parent, Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "Keep this invented question"}, {Role: "agent", Text: "Keep this invented answer"}}}); err != nil {
		t.Fatal(err)
	}
}

func listedChatIDs(t *testing.T, a *App) []string {
	t.Helper()
	view, err := a.SavedChats()
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, row := range view.Chats {
		ids = append(ids, row.ID)
	}
	sort.Strings(ids)
	return ids
}

func TestRemovingAChatTakesTheEarlierStagesItsSavedConversationsLink(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	// One chat switched twice. Each stage kept a recipient of its own, so the
	// settings hold three separate chats and only the saved conversations say
	// that they are one.
	earlierStageFixture(t, a, "claude:first", "claude", ws, "window-first", "")
	earlierStageFixture(t, a, "ollama:second", "ollama", ws, "window-second", "claude:first")
	earlierStageFixture(t, a, "codex:third", "codex", ws, "window-third", "ollama:second")
	stages := []string{"claude:first", "ollama:second", "codex:third"}
	deleted := []string{}
	a.chatCleanupHooks.DeleteProvider = func(_ context.Context, _, native string) (bool, error) {
		deleted = append(deleted, native)
		return true, nil
	}
	if got := listedChatIDs(t, a); !slices.Equal(got, []string{"codex:third"}) {
		t.Fatalf("the chat is listed as %v", got)
	}
	listing, err := a.ListChatCleanup()
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Rows) != 1 || !slices.Equal(listing.Rows[0].SessionIDs, stages) || listing.Rows[0].MessageCount != 6 || !listing.Rows[0].Eligible {
		t.Fatalf("the cleanup row does not cover the chat's stages: %+v", listing.Rows)
	}
	if err := a.RemoveChat("codex:third"); err != nil {
		t.Fatal(err)
	}
	for _, id := range stages {
		record, ok := a.cleanupRecord(id)
		if !ok || record.Group != "codex:third" || record.Stage != "retained" {
			t.Fatalf("%s has the removal record %+v", id, record)
		}
		if _, err := os.Stat(transcriptPath(id)); !os.IsNotExist(err) {
			t.Fatalf("the saved conversation of %s stayed in place: %v", id, err)
		}
	}
	if got := listedChatIDs(t, a); len(got) != 0 {
		t.Fatalf("removing the chat left %v in the list", got)
	}
	if listing, err = a.ListChatCleanup(); err != nil || len(listing.Rows) != 0 || len(listing.Removed) != 1 {
		t.Fatalf("the Recycle Bin shows %+v %v", listing, err)
	}
	if restored := a.RestoreRemovedChat("codex:third"); !restored.OK {
		t.Fatalf("restore %+v", restored)
	}
	for _, id := range stages {
		if tr, err := readTranscript(id); err != nil || len(tr.Messages) != 2 {
			t.Fatalf("restoring did not bring back %s: %v", id, err)
		}
	}
	if got := listedChatIDs(t, a); !slices.Equal(got, []string{"codex:third"}) {
		t.Fatalf("the restored chat is listed as %v", got)
	}
	if err := a.RemoveChat("codex:third"); err != nil {
		t.Fatal(err)
	}
	if result := a.DeleteRemovedChat("codex:third"); !result.OK || len(result.ProgramCopies) != 3 {
		t.Fatalf("final deletion %+v", result)
	}
	if !slices.Equal(deleted, stages) {
		t.Fatalf("the programs were asked to delete %v, want %v oldest first", deleted, stages)
	}
	for _, id := range stages {
		if record, _ := a.cleanupRecord(id); record.Stage != "deleted" {
			t.Fatalf("%s has the record %+v after final deletion", id, record)
		}
	}
	if got := listedChatIDs(t, a); len(got) != 0 {
		t.Fatalf("final deletion brought %v back into the list", got)
	}
}

func TestRemovingAChatLeavesAStageAnotherChatGoesOnFrom(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	earlierStageFixture(t, a, "claude:shared", "claude", ws, "window-shared", "")
	earlierStageFixture(t, a, "codex:left", "codex", ws, "window-left", "claude:shared")
	earlierStageFixture(t, a, "ollama:right", "ollama", ws, "window-right", "claude:shared")
	if err := a.RemoveChat("codex:left"); err != nil {
		t.Fatal(err)
	}
	if _, reserved := a.cleanupRecord("claude:shared"); reserved {
		t.Fatal("removing one chat took a stage the other chat goes on from")
	}
	if got := listedChatIDs(t, a); !slices.Equal(got, []string{"ollama:right"}) {
		t.Fatalf("the list shows %v", got)
	}
	if err := a.RemoveChat("ollama:right"); err != nil {
		t.Fatal(err)
	}
	record, reserved := a.cleanupRecord("claude:shared")
	if !reserved || record.Group != "ollama:right" {
		t.Fatalf("the last chat to go did not take the shared stage: %+v", record)
	}
	if got := listedChatIDs(t, a); len(got) != 0 {
		t.Fatalf("the list shows %v", got)
	}
}

func TestRemovingAChatTakesWhatIsOlderThanAStageRemovedForGood(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	earlierStageFixture(t, a, "claude:first", "claude", ws, "window-first", "")
	earlierStageFixture(t, a, "ollama:second", "ollama", ws, "", "claude:first")
	earlierStageFixture(t, a, "codex:third", "codex", ws, "window-third", "ollama:second")
	// An older build removed the middle stage by itself: its saved conversation
	// is marked deleted and its record is permanent.
	if err := writeTranscript(&sessionhost.Transcript{SessionID: "ollama:second", Provider: "ollama", Workspace: ws, ParentSessionID: "claude:first", Complete: true, Deleted: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.RemovedChats = map[string]chatRemoval{"ollama:second": {Stage: "removed"}}
		s.Hidden = []string{"ollama:second"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveChat("codex:third"); err != nil {
		t.Fatal(err)
	}
	if record, _ := a.cleanupRecord("claude:first"); record.Group != "codex:third" {
		t.Fatalf("the oldest stage was left behind: %+v", record)
	}
	if record, _ := a.cleanupRecord("ollama:second"); record.Stage != "removed" || record.Group != "" {
		t.Fatalf("the stage removed for good was taken again: %+v", record)
	}
	if got := listedChatIDs(t, a); len(got) != 0 {
		t.Fatalf("the list shows %v", got)
	}
}

// The case of 16 November 2026: a chat pinned at its first stage, whose newest
// stage an older build had left marked hidden. The list showed the middle
// stage, opening it opened the hidden one, deleting from that window said the
// chat was not in the inventory, and Clean Up Chats asked for a pin to be
// taken off that the chat did not show.
func TestAChatLeftMarkedHiddenIsOneChatToListOpenPinAndRemove(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	earlierStageFixture(t, a, "claude:first", "claude", ws, "", "")
	earlierStageFixture(t, a, "ollama:second", "ollama", ws, "window-second", "claude:first")
	earlierStageFixture(t, a, "codex:third", "codex", ws, "window-third", "ollama:second")
	// A switch from the first stage that did not finish left its target too.
	if err := writeTranscript(&sessionhost.Transcript{SessionID: "codex:never-started", Provider: "codex", Workspace: ws, ParentSessionID: "claude:first", Complete: true, PendingHandoff: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.Hidden = []string{"codex:third", "codex:never-started"}
		s.WorkspaceRecords = []WorkspaceRecord{{ID: "w", Name: "W", ExecutionPath: ws, Pins: []WorkspacePin{{Kind: "chat", Title: "Invented", Session: "claude:first"}}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := listedChatIDs(t, a); !slices.Equal(got, []string{"codex:third"}) {
		t.Fatalf("the chat is listed as %v", got)
	}
	if opened, err := a.ResolveSessionContinuation("claude:first"); err != nil || opened != "codex:third" {
		t.Fatalf("the pin opens %q: %v", opened, err)
	}
	// The start moves the pin to the session the chat is listed and opened as.
	if err := a.repinContinuedChats(); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	pins := append([]WorkspacePin{}, a.settings.WorkspaceRecords[0].Pins...)
	a.mu.Unlock()
	if len(pins) != 1 || pins[0].Session != "codex:third" {
		t.Fatalf("the pin is on %+v", pins)
	}
	pinsNow := func() []WorkspacePin {
		a.mu.Lock()
		defer a.mu.Unlock()
		return append([]WorkspacePin{}, a.settings.WorkspaceRecords[0].Pins...)
	}
	// The window is on the newest stage, and deleting from it takes the chat
	// and its pin. Restoring the chat puts the pin back.
	if err := a.RemoveChat("codex:third"); err != nil {
		t.Fatal(err)
	}
	if left := pinsNow(); len(left) != 0 {
		t.Fatalf("the deleted chat is still pinned: %+v", left)
	}
	if restored := a.RestoreRemovedChat("codex:third"); !restored.OK {
		t.Fatalf("restore %+v", restored)
	}
	if back := pinsNow(); len(back) != 1 || back[0].Session != "codex:third" || back[0].Title != "Invented" {
		t.Fatalf("restoring did not put the pin back: %+v", back)
	}
	if err := a.RemoveChat("codex:third"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"claude:first", "ollama:second", "codex:third", "codex:never-started"} {
		if record, ok := a.cleanupRecord(id); !ok || record.Group != "codex:third" {
			t.Fatalf("%s has the removal record %+v", id, record)
		}
	}
	if got := listedChatIDs(t, a); len(got) != 0 {
		t.Fatalf("the list shows %v", got)
	}
}

func TestAPinFollowsItsChatWhenASwitchCommits(t *testing.T) {
	s := Settings{WorkspaceRecords: []WorkspaceRecord{{ID: "w", Pins: []WorkspacePin{
		{Kind: "chat", Title: "Moves", Session: "claude:first"},
		{Kind: "chat", Title: "Stays", Session: "claude:other"},
		{Kind: "chat", Title: "Already there", Session: "codex:second"},
	}}}}
	repinChat(&s, "claude:first", "codex:second")
	pins := s.WorkspaceRecords[0].Pins
	if len(pins) != 2 || pins[0].Session != "codex:second" || pins[0].Title != "Moves" || pins[1].Session != "claude:other" {
		t.Fatalf("the pins are %+v", pins)
	}
}

func TestRemovingAnEarlierStageByItselfLeavesTheChat(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	earlierStageFixture(t, a, "claude:first", "claude", ws, "window-first", "")
	earlierStageFixture(t, a, "ollama:second", "ollama", ws, "window-second", "claude:first")
	earlierStageFixture(t, a, "codex:third", "codex", ws, "window-third", "ollama:second")
	// The notice about a stage whose messages cannot be routed removes that
	// stage by its identity. What is older than the stage goes with it.
	if err := a.RemoveChat("ollama:second"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"claude:first", "ollama:second"} {
		if record, ok := a.cleanupRecord(id); !ok || record.Group != "ollama:second" {
			t.Fatalf("%s has the removal record %+v", id, record)
		}
	}
	if _, reserved := a.cleanupRecord("codex:third"); reserved {
		t.Fatal("removing an earlier stage removed the chat")
	}
	if got := listedChatIDs(t, a); !slices.Equal(got, []string{"codex:third"}) {
		t.Fatalf("the list shows %v", got)
	}
	listing, err := a.ListChatCleanup()
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Rows) != 1 || !slices.Equal(listing.Rows[0].SessionIDs, []string{"codex:third"}) || !listing.Rows[0].Eligible {
		t.Fatalf("the chat's cleanup row is %+v", listing.Rows)
	}
}
