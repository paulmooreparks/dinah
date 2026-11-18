package main

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"studio/internal/bus"
	"studio/internal/workspaces"
)

// computer is one computer of a test that moves a workspace between
// several: its app and the folder its state is in, which the test makes
// current before using the app.
type computer struct {
	app   *App
	state string
}

func (c computer) use() { configOverride = c.state }

// exportAll writes the export of a workspace with every chat its plan lists.
func (c computer) exportAll(t *testing.T, workspaceID string) string {
	t.Helper()
	c.use()
	plan, err := c.app.PlanWorkspaceExport(workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	choice := WorkspaceExportChoice{}
	for _, chat := range plan.Chats {
		choice.Chats = append(choice.Chats, chat.ChatID)
	}
	file := filepath.Join(t.TempDir(), "Harbor"+workspaces.ExportExtension)
	if err := c.app.exportWorkspaceTo(workspaceID, file, choice); err != nil {
		t.Fatal(err)
	}
	return file
}

// harborMessages are the texts of the group Harbor Team on a computer, and
// the IDs there of the two messages the first computer made.
func harborMessages(t *testing.T, c computer) (texts []string, ids map[string]int64) {
	t.Helper()
	c.use()
	ids = map[string]int64{}
	if err := c.app.busHost().With(func(b *bus.Bus) error {
		room, err := b.Room(context.Background(), "Harbor Team")
		if err != nil {
			return err
		}
		messages, err := b.RoomMessages(context.Background(), room.Name, 100)
		for _, m := range messages {
			texts = append(texts, m.Text)
			ids[m.Text] = m.ID
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return texts, ids
}

// A group's conversation goes from the desktop to the laptop and back, and
// on through a third computer: no message comes back twice, the laptop's new
// message arrives, and a chat updated on the laptop has its references to
// the desktop's messages pointed at their original IDs again.
func TestGroupMessagesRoundTripWithoutDuplicates(t *testing.T) {
	codexHereOnly(t)
	desktopApp, harbor, _ := harborFixture(t)
	desktop := computer{desktopApp, configOverride}
	before, original := harborMessages(t, desktop)
	asked, answered := original["Plan the checkout review."], original["Docks Builder takes the receipts."]
	if asked == 0 || answered == 0 {
		t.Fatalf("the desktop's messages: %v", before)
	}
	// The hidden chat received the lead's message and sent the person's, as
	// its history records them.
	review, err := readTranscript("codex:review")
	if err != nil {
		t.Fatal(err)
	}
	review.Messages = append(review.Messages,
		ConversationMessage{Role: "delivered", Text: "Docks Builder takes the receipts.", Delivered: &DeliveredMessage{ID: answered, Kind: "text", Room: "harbor-team", RoomTitle: "Harbor Team"}},
		ConversationMessage{Role: "sent", Text: "Plan the checkout review.", Delivered: &DeliveredMessage{ID: asked, Kind: "text", Room: "harbor-team", RoomTitle: "Harbor Team"}})
	if err := writeTranscript(review); err != nil {
		t.Fatal(err)
	}
	toLaptop := desktop.exportAll(t, harbor.ID)

	laptopApp, _ := providerTestApp(t)
	laptop := computer{laptopApp, configOverride}
	laptopMain := filepath.Join(t.TempDir(), "Laptop", "Harbor")
	makeFolders(t, laptopMain, "Checkout")
	imported := importOn(t, laptop.app, toLaptop, laptopMain)
	// On the laptop, the hidden chat is continued and the person writes in
	// the group.
	if _, err := laptop.app.ContinueMovedChat("codex:review"); err != nil {
		t.Fatal(err)
	}
	if err := laptop.app.busHost().With(func(b *bus.Bus) error {
		_, err := b.SendFiles(context.Background(), "#Harbor Team", "Laptop note.", nil, 0, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	toDesktop := laptop.exportAll(t, imported.WorkspaceID)

	thirdApp, _ := providerTestApp(t)
	third := computer{thirdApp, configOverride}
	thirdMain := filepath.Join(t.TempDir(), "Third", "Harbor")
	makeFolders(t, thirdMain, "Checkout")
	onThird := importOn(t, third.app, toDesktop, thirdMain)
	fromThird := third.exportAll(t, onThird.WorkspaceID)

	// Back on the desktop, from the laptop.
	desktop.use()
	result := importOn(t, desktop.app, toDesktop, workspaces.ExecutionPath(harbor))
	if result.Messages != 1 || result.Updated != 1 {
		t.Fatalf("the import back: %+v", result)
	}
	after, ids := harborMessages(t, desktop)
	if len(after) != len(before)+1 || !slices.Contains(after, "Laptop note.") {
		t.Fatalf("the conversation came back with duplicates or without the laptop's message: %v", after)
	}
	if ids["Plan the checkout review."] != asked || ids["Docks Builder takes the receipts."] != answered {
		t.Fatalf("the desktop's own messages changed: %+v", ids)
	}
	desktop.app.mu.Lock()
	head := desktop.app.settings.ChatBindings["codex:review"]
	desktop.app.mu.Unlock()
	updated, err := readTranscript(head)
	if err != nil {
		t.Fatalf("the hidden chat's new stage: %q %v", head, err)
	}
	found := map[string]int64{}
	for _, m := range append(append([]ConversationMessage{}, updated.ParentHistory...), updated.Messages...) {
		if m.Delivered != nil {
			found[m.Role] = m.Delivered.ID
		}
	}
	if found["delivered"] != answered || found["sent"] != asked {
		t.Fatalf("the references did not come back to the desktop's own messages: %+v, want %d and %d", found, answered, asked)
	}

	// And again from the third computer, which had it from the laptop.
	again := importOn(t, desktop.app, fromThird, workspaces.ExecutionPath(harbor))
	if again.Messages != 0 {
		t.Fatalf("the third computer's copy added messages: %+v", again)
	}
	if final, _ := harborMessages(t, desktop); !slices.Equal(final, after) {
		t.Fatalf("the conversation changed: %v", final)
	}
}
