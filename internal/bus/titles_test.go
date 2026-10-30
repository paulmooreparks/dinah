package bus

import (
	"context"
	"strings"
	"testing"
)

// A chat joins the bus with its title, a new title replaces it at once in
// every heading, including those of messages sent before, and a chat with
// no title is shown as untitled in its workspace's label.
func TestATitleIsRegisteredAndChanged(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	alpha := workspace(t, root, "alpha")
	a, err := b.RegisterSession(ctx, "s-a", alpha, "Parser Release")
	if err != nil {
		t.Fatal(err)
	}
	if a.Display() != "Parser Release" || a.Address() != "Parser Release (alpha)" {
		t.Fatalf("registered as %q, addressed as %q", a.Display(), a.Address())
	}
	sent, err := b.Send(ctx, Sender{Session: &a}, Human, "Tagged.", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetTitle(ctx, a.ID, "  Parser   2.4 "); err != nil {
		t.Fatal(err)
	}
	m, _ := b.Message(ctx, sent.ID)
	if m.From != "Parser 2.4" || m.FromName == "Parser 2.4" {
		t.Fatalf("after the rename the message is from %q (stored as %q)", m.From, m.FromName)
	}
	if err := b.SetTitle(ctx, a.ID, ""); err != nil {
		t.Fatal(err)
	}
	if s, _ := b.Session(ctx, a.ID); s.Display() != "Untitled Chat in alpha" {
		t.Fatalf("an untitled chat is shown as %q", s.Display())
	}
	if _, err := b.LabelWorkspace(ctx, alpha, "Alpha Project"); err != nil {
		t.Fatal(err)
	}
	if s, _ := b.Session(ctx, a.ID); s.Display() != "Untitled Chat in Alpha Project" || s.WorkspaceName != "alpha" {
		t.Fatalf("with its workspace labelled, an untitled chat is shown as %q in %q", s.Display(), s.WorkspaceName)
	}
	if m, _ := b.Message(ctx, sent.ID); m.From != "Untitled Chat in Alpha Project" || m.FromWorkspaceLabel != "Alpha Project" || m.FromWorkspace != "alpha" {
		t.Fatalf("the message is from %q in %q (%q)", m.From, m.FromWorkspaceLabel, m.FromWorkspace)
	}
	if err := b.SetTitle(ctx, "nobody", "x"); err == nil {
		t.Fatal("titled a session that does not exist")
	}
}

// An agent sends to a chat by its title, in any case, and adds the
// workspace, by its name or its label, when two chats share the title.
// Such a title alone is refused with the chats it could be.
func TestAChatIsAddressedByTitle(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	alphaDir, betaDir := workspace(t, root, "alpha"), workspace(t, root, "beta")
	if _, err := b.LabelWorkspace(ctx, betaDir, "Beta Site"); err != nil {
		t.Fatal(err)
	}
	a, _ := b.RegisterSession(ctx, "s-a", alphaDir, "Fix the Parser")
	other, _ := b.RegisterSession(ctx, "s-b", betaDir, "Fix the Parser")
	docs, _ := b.RegisterSession(ctx, "s-d", betaDir, "Docs Pass")
	me, _ := b.RegisterSession(ctx, "s-me", alphaDir, "Coordinator")

	delivered := func(s Session) int {
		got, _, err := b.Deliver(ctx, s.ID, "tool")
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}
	m, err := b.Send(ctx, Sender{Session: &me}, "docs pass", "Check the guide.", nil, 0)
	if err != nil || m.To != "Docs Pass" || delivered(docs) != 1 || delivered(other) != 0 {
		t.Fatalf("by title: %v, to %q", err, m.To)
	}
	_, err = b.Send(ctx, Sender{Session: &me}, "Fix the parser", "Which one?", nil, 0)
	if err == nil || !strings.Contains(err.Error(), `"Fix the Parser" (workspace alpha)`) || !strings.Contains(err.Error(), `"Fix the Parser" (workspace beta)`) {
		t.Fatalf("an ambiguous title was not refused with its candidates: %v", err)
	}
	for _, addr := range []string{"Fix the Parser (beta)", "fix the parser (Beta Site)", "Fix the Parser (workspace beta)"} {
		if _, err := b.Send(ctx, Sender{Session: &me}, addr, "This one.", nil, 0); err != nil {
			t.Fatalf("%s: %v", addr, err)
		}
		if delivered(other) != 1 || delivered(a) != 0 {
			t.Fatalf("%s did not reach the chat in beta alone", addr)
		}
	}
	// A workspace keeps its address, by name and by its label.
	if m, err := b.Send(ctx, Sender{Session: &me}, "Beta Site", "Everyone in beta.", nil, 0); err != nil || m.To != "beta" || m.ToPerson != "Beta Site" {
		t.Fatalf("the workspace by its label: %v, to %q for an agent and %q for a person", err, m.To, m.ToPerson)
	}
	if _, err := b.Send(ctx, Sender{Session: &me}, "Nobody Here", "Lost.", nil, 0); err == nil || !strings.Contains(err.Error(), "chat's title") {
		t.Fatalf("an unknown address: %v", err)
	}
	// A group takes a chat by its title, and refuses a title two chats have.
	if _, err := b.CreateGroup(ctx, Sender{Session: &me}, "release", []string{"Docs Pass"}); err != nil {
		t.Fatal(err)
	}
	if err := b.AddToGroup(ctx, Sender{}, "release", []string{"Fix the Parser"}); err == nil {
		t.Fatal("an ambiguous title joined a group")
	}
	members, _ := b.RoomMembers(ctx, "release")
	var labels []string
	for _, m := range members {
		labels = append(labels, m.Address)
	}
	if strings.Join(labels, ",") != "Coordinator (alpha),Docs Pass (beta),paul" {
		t.Fatalf("the group's members are %v", labels)
	}
}

// What an agent is told about a message gives the sender's and the
// recipient's titles as they are now, and the sender's workspace by its
// name on the bus, never a handle.
func TestFormatForAgentGivesTitles(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	alphaDir := workspace(t, root, "alpha")
	b.LabelWorkspace(ctx, alphaDir, "Alpha Project")
	a, _ := b.RegisterSession(ctx, "s-a", alphaDir, "Parser Release")
	c, _ := b.RegisterSession(ctx, "s-c", workspace(t, root, "gamma"), "")
	if _, err := b.Send(ctx, Sender{Session: &a}, "Untitled Chat in gamma", "Ready?", nil, 0); err != nil {
		t.Fatal(err)
	}
	got, _, _ := b.Deliver(ctx, c.ID, "tool")
	text := FormatForAgent(got, "Paul")
	if !strings.Contains(text, "from Parser Release (workspace alpha) to Untitled Chat in gamma") || strings.Contains(text, "alpha-1") || strings.Contains(text, "gamma-1") {
		t.Fatalf("the heading:\n%s", text)
	}
	if d := Describe(got[0]); d.From != "Parser Release" || d.Workspace != "Alpha Project" || d.FromSession != a.ID {
		t.Fatalf("a person reads it as from %q in %q (%q)", d.From, d.Workspace, d.FromSession)
	}
}

// Messages sent before the bus recorded whom a message was for are given
// their recipient once, so their headings show titles too.
func TestOldMessagesAreAddressed(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	a, _ := b.RegisterSession(ctx, "s-a", workspace(t, root, "alpha"), "Asker")
	c, _ := b.RegisterSession(ctx, "s-c", workspace(t, root, "gamma"), "Answerer")
	toChat, _ := b.Send(ctx, Sender{Session: &a}, "Answerer", "One.", nil, 0)
	toWorkspace, _ := b.Send(ctx, Sender{Session: &a}, "gamma", "Two.", nil, 0)
	if _, err := b.db.Exec(`UPDATE messages SET to_kind = '', to_key = ''`); err != nil {
		t.Fatal(err)
	}
	if err := b.SetTitle(ctx, c.ID, "Renamed Answerer"); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.Message(ctx, toChat.ID); m.To != "Answerer" {
		t.Fatalf("before, the message is to %q, the title it was sent with", m.To)
	}
	if err := addressOldMessages(b.db); err != nil {
		t.Fatal(err)
	}
	if m, _ := b.Message(ctx, toChat.ID); m.To != "Renamed Answerer" {
		t.Fatalf("a message to a chat is now to %q", m.To)
	}
	if m, _ := b.Message(ctx, toWorkspace.ID); m.To != "gamma" {
		t.Fatalf("a message to a workspace is now to %q", m.To)
	}
}
