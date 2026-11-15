package bus

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGroupPresentationSurvivesReopenWithoutChangingRoutingOrAgentText(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	ws := workspace(t, root, "invented")
	for _, id := range []string{"one", "two"} {
		if _, err := b.RegisterSession(ctx, id, ws, id); err != nil {
			t.Fatal(err)
		}
	}
	g, err := b.CreateGroup(ctx, Sender{}, "References", []string{"one", "two"})
	if err != nil {
		t.Fatal(err)
	}
	p := MessagePresentation{ShownText: "@Other inspect this", References: []MessageReference{{Kind: "chat", Name: "Other", Token: "@Other", Workspace: "invented"}, {Kind: "window", Name: "Files", Token: ""}}}
	wire := p.ShownText + "\n\nThe user mentioned the chat Other."
	m, err := b.SendFilesPresented(ctx, "#"+g.Name, wire, nil, 0, []string{"one"}, p)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := b.SendFiles(ctx, "#"+g.Name, wire, nil, 0, []string{"one"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.Mentioned, prior.Mentioned) || !reflect.DeepEqual(m.Reach.Reach, prior.Reach.Reach) || m.Reach.Everyone != prior.Reach.Everyone {
		t.Fatalf("presentation changed routing: %+v %+v", m, prior)
	}
	reopened, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Message(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != wire || got.ShownText == nil || *got.ShownText != p.ShownText || !reflect.DeepEqual(got.References, p.References) {
		t.Fatalf("presentation did not survive: %+v", got)
	}
	old, err := reopened.Message(ctx, prior.ID)
	if err != nil || old.ShownText != nil || old.References != nil {
		t.Fatalf("legacy fields appeared: %+v %v", old, err)
	}
	msgs, _, err := reopened.Deliver(ctx, "one", "tool")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, delivered := range msgs {
		if delivered.ID == m.ID {
			found = true
			if delivered.Text != wire {
				t.Fatal("agent text changed")
			}
		}
	}
	if !found {
		t.Fatal("explicit recipient did not receive message")
	}
}

func TestGroupPresentationFailureRollsBackMessageAndDelivery(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	ws := workspace(t, root, "invented")
	if _, err := b.RegisterSession(ctx, "one", ws, "One"); err != nil {
		t.Fatal(err)
	}
	g, err := b.CreateGroup(ctx, Sender{}, "References", []string{"One"})
	if err != nil {
		t.Fatal(err)
	}
	var before int
	if err = b.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err = b.db.Exec(`CREATE TRIGGER refuse_presentation BEFORE INSERT ON message_presentation BEGIN SELECT RAISE(ABORT,'invented write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = b.SendFilesPresented(ctx, "#"+g.Name, "context", nil, 0, []string{"one"}, MessagePresentation{ShownText: "", References: []MessageReference{{Kind: "window", Name: "Files"}}}); err == nil {
		t.Fatal("presentation failure was hidden")
	}
	var after int
	if err = b.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("message escaped failed presentation transaction")
	}
}
