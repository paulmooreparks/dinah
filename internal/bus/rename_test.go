package bus

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A group renamed by the person keeps its stable name, members, messages, budget and
// unread marks, and takes the new title in what is delivered afterwards.
func TestARenamedGroupKeepsEverythingButItsTitle(t *testing.T) {
	ctx := context.Background()
	b, g, lead, docs, _ := noticeGroup(t)
	if err := b.SetGroupBudget(ctx, g.Name, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{Session: &lead}, "#"+g.Name, "Before the rename.", nil, 0); err != nil {
		t.Fatal(err)
	}
	before, _ := b.Room(ctx, g.Name)
	room, err := b.RenameGroup(ctx, Sender{}, g.Title, "  #Dinah   Development ")
	if err != nil {
		t.Fatal(err)
	}
	if room.Name != g.Name || room.Title != "Dinah Development" || len(room.Members) != len(g.Members) || room.Budget != 7 || room.Unread != before.Unread+1 {
		// The rename adds a line for the person to see, which counts as unread only if
		// it is delivered to the person; the member lines are delivered to nobody.
		if room.Unread != before.Unread {
			t.Fatalf("room %+v, before %+v", room, before)
		}
	}
	msgs, err := b.RoomMessages(ctx, g.Name, 50)
	if err != nil {
		t.Fatal(err)
	}
	var before1, line bool
	for _, m := range msgs {
		before1 = before1 || m.Text == "Before the rename."
		if m.Kind == MemberKind && strings.HasPrefix(m.Text, "renamed ") {
			line = m.MemberLine("Paul") == "Paul renamed the group to Dinah Development."
		}
	}
	if !before1 || !line {
		t.Fatalf("the messages after a rename: kept %v, line %v", before1, line)
	}
	// What is delivered after the rename uses the new title, and the group is
	// addressed by it and by its stable name.
	if _, err := b.Send(ctx, Sender{Session: &lead}, "#Dinah Development", "After the rename.", nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{Session: &lead}, "#"+g.Name, "By the stable name.", nil, 0); err != nil {
		t.Fatal(err)
	}
	got, _, _ := b.Deliver(ctx, docs.ID, "UserPromptSubmit")
	text := FormatForAgent(got, PersonFallback)
	if !strings.Contains(text, `In the group chat "Dinah Development"`) || strings.Contains(text, `"`+g.Title+`"`) {
		t.Fatalf("a delivery after the rename:\n%s", text)
	}
}

// A chat that addresses the group by an old title is refused, and the refusal gives
// the new title; the old title is remembered through further renames, and a title a
// new group takes is that group's.
func TestAnOldTitleIsRefusedWithTheNewOne(t *testing.T) {
	ctx := context.Background()
	b, _, lead, _, _ := noticeGroup(t)
	// A title of several words has a stable name that is not the title in lower case,
	// so the old title does not find the group by its stable name.
	g, err := b.CreateGroup(ctx, Sender{}, "Release Review", []string{"Lead", "Docs Pass"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.RenameGroup(ctx, Sender{}, g.Name, "Dinah Development"); err != nil {
		t.Fatal(err)
	}
	_, err = b.Send(ctx, Sender{Session: &lead}, "#"+g.Title, "Stale instructions.", nil, 0)
	if err == nil || !strings.Contains(err.Error(), `"Dinah Development"`) || !strings.Contains(err.Error(), "send to #Dinah Development") || !strings.Contains(err.Error(), g.Title) {
		t.Fatalf("a send to the old title: %v", err)
	}
	if !errorsIsNotFound(err) {
		t.Fatalf("the refusal is not a not-found error: %v", err)
	}
	// Renamed again, the first title still leads to the current one.
	if _, err := b.RenameGroup(ctx, Sender{}, "Dinah Development", "Dinah Studio"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{Session: &lead}, "#"+g.Title, "Stale again.", nil, 0); err == nil || !strings.Contains(err.Error(), `"Dinah Studio"`) {
		t.Fatalf("after a second rename: %v", err)
	}
	// The group that was called by an old title but is not now is not found by a
	// message to something else.
	if _, err := b.Send(ctx, Sender{Session: &lead}, "#Nothing Like It", "x", nil, 0); err == nil || strings.Contains(err.Error(), "is now called") {
		t.Fatalf("an unknown group: %v", err)
	}
	// A new group may take the old title, and then the title is that group's.
	other, err := b.CreateGroup(ctx, Sender{}, g.Title, []string{"Lead", "Docs Pass"})
	if err != nil {
		t.Fatal(err)
	}
	if r, err := b.Room(ctx, g.Title); err != nil || r.Name != other.Name {
		t.Fatalf("the old title is not the new group's: %+v %v", r, err)
	}
	// Deleting a group takes its remembered titles with it.
	if err := b.DeleteRoom(ctx, "Dinah Studio"); err != nil {
		t.Fatal(err)
	}
	var n int
	b.db.QueryRow(`SELECT COUNT(*) FROM room_renames`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d remembered titles of a deleted group", n)
	}
}

// A rename is refused for an empty title and for a title another group has, does
// nothing for the title it already has, and changes the case of its own title.
func TestRenamingAGroupHasRules(t *testing.T) {
	ctx := context.Background()
	b, g, _, _, _ := noticeGroup(t)
	if _, err := b.RenameGroup(ctx, Sender{}, g.Name, "  # "); err == nil {
		t.Fatal("an empty title was accepted")
	}
	if _, err := b.CreateGroup(ctx, Sender{}, "Other Group", []string{"Lead", "Docs Pass"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.RenameGroup(ctx, Sender{}, g.Name, "other group"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a title another group has: %v", err)
	}
	same, err := b.RenameGroup(ctx, Sender{}, g.Name, g.Title)
	if err != nil || same.Title != g.Title {
		t.Fatalf("the same title: %+v %v", same, err)
	}
	var n int
	b.db.QueryRow(`SELECT COUNT(*) FROM room_renames`).Scan(&n)
	if n != 0 {
		t.Fatal("a rename to the same title was remembered")
	}
	lower, err := b.RenameGroup(ctx, Sender{}, g.Name, strings.ToLower(g.Title))
	if err != nil || lower.Title != strings.ToLower(g.Title) {
		t.Fatalf("a change of case: %+v %v", lower, err)
	}
	b.db.QueryRow(`SELECT COUNT(*) FROM room_renames`).Scan(&n)
	if n != 0 {
		t.Fatal("a change of case was remembered as an old title")
	}
	if _, err := b.RenameGroup(ctx, Sender{}, "no such group", "X"); err == nil {
		t.Fatal("a group that does not exist was renamed")
	}
}

func errorsIsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
