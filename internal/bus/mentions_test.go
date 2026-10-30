package bus

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func textsOf(msgs []Message) []string {
	var out []string
	for _, m := range msgs {
		out = append(out, m.Text)
	}
	return out
}

// The messages that agents sent the person, by a mention in a group chat or
// directly, are found; a group message that does not mention the person, the
// person's own, and one already read are not; and the number to start after
// is honoured.
func TestMentionsAreWhatAgentsSentThePersonAndTheyHaveNotRead(t *testing.T) {
	g := newBudgetGroup(t)
	ctx := context.Background()
	g.say(&g.a, "for information only")
	g.say(&g.a, "need you in the group", Human)
	if _, err := g.b.Send(ctx, Sender{Session: &g.c}, Human, "a direct message", nil, 0); err != nil {
		t.Fatal(err)
	}
	g.say(nil, "the person speaking")
	got, err := g.b.Mentions(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"need you in the group", "a direct message"}; !slices.Equal(textsOf(got), want) {
		t.Fatalf("mentions %v, want %v", textsOf(got), want)
	}
	first := got[0]
	if first.Room == "" || first.RoomTitle != "Work" || first.From != "Architect" || first.FromWorkspace != "studio" || first.ID == 0 {
		t.Fatalf("the mention lacks its group or sender: %+v", first)
	}
	if got[1].Room != "" {
		t.Fatalf("a direct message has a group: %+v", got[1])
	}
	after, _ := g.b.Mentions(ctx, first.ID)
	if !slices.Equal(textsOf(after), []string{"a direct message"}) {
		t.Fatalf("after the first mention: %v", textsOf(after))
	}
	top, err := g.b.HighestMessage(ctx)
	if err != nil || top != got[1].ID+1 {
		t.Fatalf("the highest message is %d (the last mention is %d, then the person's own): %v", top, got[1].ID, err)
	}
	// Reading the group's window reads the mentions in it, and the inbox
	// reading reads the direct one.
	read, err := g.b.MentionsRead(ctx, []int64{first.ID, got[1].ID})
	if err != nil || len(read) != 0 {
		t.Fatalf("read before anything was read: %v %v", read, err)
	}
	if err := g.b.MarkRoomRead(ctx, g.name); err != nil {
		t.Fatal(err)
	}
	read, _ = g.b.MentionsRead(ctx, []int64{first.ID, got[1].ID})
	if !slices.Equal(read, []int64{first.ID}) {
		t.Fatalf("after the group was read: %v, want only %d", read, first.ID)
	}
	left, _ := g.b.Mentions(ctx, 0)
	if !slices.Equal(textsOf(left), []string{"a direct message"}) {
		t.Fatalf("unread mentions after the group was read: %v", textsOf(left))
	}
	if strings.Contains(strings.Join(textsOf(left), "|"), "need you") {
		t.Fatal("a read mention is still found")
	}
}
