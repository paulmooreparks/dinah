package bus

import (
	"context"
	"sort"
	"strings"
	"testing"
)

// A group's members are chats and the person. Its message reaches every
// other member chat, not the other chats of their workspaces; only a chat
// the message mentions is woken, and the others read it at their next turn.
// Mentions are a list the message keeps, so @ and a title in the text wake
// nobody.
func TestAGroupIsMadeOfChats(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio, pudl := workspace(t, root, "studio"), workspace(t, root, "pudl")
	lead, _ := b.RegisterSession(ctx, "window-lead", studio, "Lead")
	other, _ := b.RegisterSession(ctx, "window-other", studio, "Unrelated Work")
	docs, _ := b.RegisterSession(ctx, "window-docs", pudl, "Docs Pass")
	parser, _ := b.RegisterSession(ctx, "window-parser", pudl, "Fix the Parser")
	g, err := b.CreateGroup(ctx, Sender{Session: &lead}, "", []string{"Docs Pass", "fix the parser (pudl)"})
	if err != nil {
		t.Fatal(err)
	}
	if g.Title != "Lead, Docs Pass and Fix the Parser" || g.Name != "lead-docs-pass-and-fix-the-parser" {
		t.Fatalf("the group without a name is %q (%s)", g.Title, g.Name)
	}
	var kinds []string
	for _, m := range g.Members {
		kinds = append(kinds, m.Kind+":"+m.Title)
	}
	if strings.Join(kinds, ",") != "session:Lead,session:Docs Pass,session:Fix the Parser,human:paul" {
		t.Fatalf("members %v", kinds)
	}
	rings := func(s Session) int { n, _, _ := b.Doorbell(ctx, s.ID); return n }
	take := func(s Session) []Message { got, _, _ := b.Deliver(ctx, s.ID, "UserPromptSubmit"); return got }

	b.Send(ctx, Sender{Session: &lead}, "#"+g.Title, "The parser is tagged. @Docs Pass please look.", nil, 0)
	// Text alone routes no action. Every recipient keeps a passive notice.
	if rings(docs) != 0 || rings(parser) != 0 || rings(lead) != 0 || rings(other) != 0 {
		t.Fatalf("a message without mentions rang docs %d, parser %d, lead %d and a chat outside %d times", rings(docs), rings(parser), rings(lead), rings(other))
	}
	got, block, _ := b.Deliver(ctx, docs.ID, "Stop")
	if block || len(got) != 0 {
		t.Fatal("the notice kept a turn going at Stop")
	}
	got = take(docs)
	if len(got) != 1 || got[0].Asked || !got[0].Notice || !strings.Contains(FormatForAgent(got, PersonFallback), `In the group chat "Lead, Docs Pass and Fix the Parser", these messages do not mention you`) {
		t.Fatalf("docs got %+v", got)
	}
	if len(take(parser)) != 1 || len(take(other)) != 0 || len(take(lead)) != 0 || rings(parser) != 0 {
		t.Fatal("the message did not reach the other members alone")
	}

	m, err := b.SendMentioning(ctx, Sender{Session: &lead}, "#"+g.Name, "Can you check the docs?", nil, 0, []string{"@Docs Pass (pudl)"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Mentioned) != 1 || m.Mentioned[0].Key != docs.ID || m.Mentioned[0].Address != "Docs Pass (pudl)" {
		t.Fatalf("the message mentions %+v", m.Mentioned)
	}
	if rings(docs) != 1 || rings(parser) != 0 {
		t.Fatalf("a mention of docs rang docs %d times and parser %d, for its notice", rings(docs), rings(parser))
	}
	got, _, _ = b.Deliver(ctx, docs.ID, DoorbellTurn)
	if len(got) != 1 || !got[0].Asked || !strings.Contains(FormatOne(got[0], "Paul"), "It mentions Docs Pass (pudl).") {
		t.Fatalf("docs was not asked: %+v", got)
	}
	take(parser)

	// The person's message asks the chats it mentions, as a chat's does, and
	// gives the other open members a notice.
	if _, err := b.SendMentioning(ctx, Sender{}, "#"+g.Name, "Parser, anything blocking?", nil, 0, []string{parser.ID}); err != nil {
		t.Fatal(err)
	}
	if rings(parser) != 1 || rings(docs) != 0 || rings(lead) != 0 {
		t.Fatal("the person's request did not ring only its action recipient")
	}
	if got := take(parser); len(got) != 1 || !got[0].Asked || got[0].Notice {
		t.Fatalf("the chat it mentions got %+v", got)
	}
	if got := take(docs); len(got) != 1 || got[0].Asked || !got[0].Notice {
		t.Fatalf("a chat it does not mention got %+v", got)
	}
	take(lead)
}

// An agent may mention only the group's members: a chat outside the group
// is refused with the reason, and so is a name no chat has, and a mention
// in a message that is not a group's. A chat outside the group cannot write
// to it.
func TestMentionsMustBeMembers(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio := workspace(t, root, "studio")
	lead, _ := b.RegisterSession(ctx, "window-lead", studio, "Lead")
	b.RegisterSession(ctx, "window-docs", studio, "Docs Pass")
	outsider, _ := b.RegisterSession(ctx, "window-out", studio, "Outsider")
	g, err := b.CreateGroup(ctx, Sender{Session: &lead}, "Release", []string{"Docs Pass"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.SendMentioning(ctx, Sender{Session: &lead}, "#Release", "Outsider, look.", nil, 0, []string{"Outsider"})
	if err == nil || !strings.Contains(err.Error(), "Outsider (studio) is not in the group chat \"Release\"") || !strings.Contains(err.Error(), "Docs Pass (studio)") {
		t.Fatalf("a mention of a chat outside the group: %v", err)
	}
	if _, err := b.SendMentioning(ctx, Sender{Session: &lead}, "#Release", "Hello?", nil, 0, []string{"Nobody Here"}); err == nil || !strings.Contains(err.Error(), "no chat in the group chat") {
		t.Fatalf("a mention of nobody: %v", err)
	}
	if _, err := b.SendMentioning(ctx, Sender{Session: &lead}, "Docs Pass", "Hello", nil, 0, []string{"Docs Pass"}); err == nil || !strings.Contains(err.Error(), "only a message in a group chat") {
		t.Fatalf("a mention in a direct message: %v", err)
	}
	if _, err := b.Send(ctx, Sender{Session: &outsider}, "#"+g.Name, "Let me in.", nil, 0); err == nil || !strings.Contains(err.Error(), "not in the group chat") {
		t.Fatalf("a chat outside the group wrote to it: %v", err)
	}
	var n int
	b.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE room = ?`, g.Name).Scan(&n)
	if n != 0 {
		t.Fatalf("%d refused messages were kept", n)
	}
}

// Everyone wakes every member, closed ones too, which the window opens in the
// background. A mention of the person also puts the message in the person's
// inbox.
func TestEveryoneAndThePerson(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio := workspace(t, root, "studio")
	lead, _ := b.RegisterSession(ctx, "window-lead", studio, "Lead")
	docs, _ := b.RegisterSession(ctx, "window-docs", studio, "Docs Pass")
	closed, err := b.KnowSession(ctx, "window-closed", studio, "Timesheet Applet")
	if err != nil || !closed.Ended {
		t.Fatalf("a closed chat is known as %+v: %v", closed, err)
	}
	g, err := b.CreateGroup(ctx, Sender{}, "Review", []string{"Lead", "Docs Pass", "Timesheet Applet"})
	if err != nil {
		t.Fatal(err)
	}
	if !g.Members[0].Open || g.Members[2].Open {
		t.Fatalf("members' states %+v", g.Members)
	}
	rings := func(id string) int { n, _, _ := b.Doorbell(ctx, id); return n }
	if _, err := b.SendMentioning(ctx, Sender{Session: &lead}, "#Review", "Anything blocking?", nil, 0, []string{"everyone"}); err != nil {
		t.Fatal(err)
	}
	if rings(docs.ID) != 1 || rings(lead.ID) != 0 {
		t.Fatal("everyone did not wake the open members but the sender")
	}
	asked := false
	wakes, _ := b.Waking(ctx)
	for _, w := range wakes {
		asked = asked || w.Session == closed.ID
	}
	if !asked {
		t.Fatalf("everyone did not ask the window to open the closed member: %+v", wakes)
	}
	b.RegisterSession(ctx, closed.ID, studio, "")
	if got, _, _ := b.Deliver(ctx, closed.ID, "UserPromptSubmit"); len(got) != 1 || !got[0].Asked || got[0].Notice {
		t.Fatalf("the closed member did not find the message waiting, whole: %+v", got)
	}
	if shown := FormatOne(Message{Room: "r", RoomTitle: "Review", Kind: "text", Asked: true, Mentioned: []Mention{{Key: Everyone, Title: Everyone, Address: Everyone}}}, "Paul"); !strings.Contains(shown, "It mentions everyone.") {
		t.Fatalf("a message to everyone shows %q", shown)
	}
	if inbox, _ := b.HumanInbox(ctx, 10, false); len(inbox) != 0 {
		t.Fatal("a message that does not mention the person reached the inbox")
	}
	if _, err := b.SendMentioning(ctx, Sender{Session: &lead}, "#Review", "Paul, please decide.", nil, 0, []string{"paul"}); err != nil {
		t.Fatal(err)
	}
	inbox, _ := b.HumanInbox(ctx, 10, false)
	if len(inbox) != 1 || inbox[0].RoomTitle != "Review" || !inbox[0].Unread {
		t.Fatalf("the person's mention: %+v", inbox)
	}
	// Only the everyone message rings docs.
	if rings(docs.ID) != 1 {
		t.Fatalf("docs was rung %d times", rings(docs.ID))
	}
}

// A group takes and loses chats, keeping a line for each; it refuses an
// untitled chat and a name another group has, and never loses the person.
// Deleting it keeps its messages.
func TestGroupMembersChange(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio, stray := workspace(t, root, "studio"), workspace(t, root, "stray")
	lead, _ := b.RegisterSession(ctx, "window-lead", studio, "Lead")
	b.RegisterSession(ctx, "window-docs", studio, "Docs Pass")
	b.RegisterSession(ctx, "window-blank", studio, "")
	b.KnowSession(ctx, "window-stray", stray, "Stray Chat")
	if _, err := b.CreateGroup(ctx, Sender{}, "Talk", []string{"Untitled Chat in studio"}); err == nil || !strings.Contains(err.Error(), "has no title yet") {
		t.Fatalf("an untitled chat joined a group: %v", err)
	}
	if _, err := b.CreateGroup(ctx, Sender{}, "Talk", nil); err == nil {
		t.Fatal("a group of nobody was made")
	}
	g, err := b.CreateGroup(ctx, Sender{}, "  Talk  ", []string{"window-lead"})
	if err != nil || g.Title != "Talk" || g.Name != "talk" {
		t.Fatalf("made %+v: %v", g, err)
	}
	if _, err := b.CreateGroup(ctx, Sender{}, "talk", []string{"Docs Pass"}); err == nil {
		t.Fatal("a second group took the name talk")
	}
	if err := b.AddToGroup(ctx, Sender{}, "talk", []string{"Docs Pass", "Stray Chat", "Lead"}); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveFromGroup(ctx, Sender{}, "talk", "Docs Pass"); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveFromGroup(ctx, Sender{Session: &lead}, "Talk", lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveFromGroup(ctx, Sender{}, "talk", Human); err == nil {
		t.Fatal("the person was taken out of a group")
	}
	msgs, _ := b.RoomMessages(ctx, g.Name, 50)
	var lines []string
	for _, m := range msgs {
		if m.Kind == MemberKind {
			lines = append(lines, m.MemberLine("Paul"))
		}
	}
	if strings.Join(lines, " ") != "Paul added Docs Pass to the group. Paul added Stray Chat to the group. Paul removed Docs Pass from the group. Lead left the group." {
		t.Fatalf("the lines: %q", lines)
	}
	if got, _, _ := b.Deliver(ctx, "window-docs", "tool"); len(got) != 0 {
		t.Fatal("a line about members was delivered to a chat")
	}
	members, _ := b.RoomMembers(ctx, g.Name)
	if len(members) != 2 || members[0].Title != "Stray Chat" || members[0].Open || members[1].Kind != "human" {
		t.Fatalf("members %+v", members)
	}
	if err := b.RemoveWorkspace(ctx, "stray"); err != nil {
		t.Fatal(err)
	}
	if members, _ := b.RoomMembers(ctx, g.Name); len(members) != 1 {
		t.Fatalf("the removed workspace's chat stayed in the group: %+v", members)
	}
	if err := b.DeleteRoom(ctx, "#Talk"); err != nil {
		t.Fatal(err)
	}
	if rooms, _ := b.Rooms(ctx); len(rooms) != 0 {
		t.Fatal("the group was not deleted")
	}
	if msgs, _ := b.RoomMessages(ctx, g.Name, 50); len(msgs) != 4 {
		t.Fatalf("the deleted group kept %d messages, want 4", len(msgs))
	}
}

// A room of workspaces from before groups were made of chats becomes a
// group of the chats the bus knows in them, once: the open chats and the
// window's closed ones, not a session outside the window that has ended.
// The room keeps its messages and takes its name as its title, and the
// person is a member. Converting again changes nothing.
func TestOldRoomsAreConverted(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio, pudl := workspace(t, root, "studio"), workspace(t, root, "pudl")
	live, _ := b.RegisterSession(ctx, "cli-live", studio, "Terminal Chat")
	b.RegisterSession(ctx, "cli-gone", studio, "Gone")
	b.EndSession(ctx, "cli-gone")
	b.KnowSession(ctx, "window-closed", pudl, "Closed Docs")
	b.RegisterSession(ctx, "window-open", pudl, "Open Docs")
	var studioKey, pudlKey string
	b.db.QueryRow(`SELECT key FROM workspaces WHERE name = 'studio'`).Scan(&studioKey)
	b.db.QueryRow(`SELECT key FROM workspaces WHERE name = 'pudl'`).Scan(&pudlKey)
	for _, q := range []string{
		`INSERT INTO rooms (name, created_at) VALUES ('release', '2026-10-20T00:00:00Z')`,
		`INSERT INTO room_members (room, member_kind, member) VALUES ('release', 'workspace', '` + studioKey + `')`,
		`INSERT INTO room_members (room, member_kind, member) VALUES ('release', 'workspace', '` + pudlKey + `')`,
		`INSERT INTO messages (created_at, kind, from_session, from_name, to_label, room, text, mentions)
			VALUES ('2026-10-20T01:00:00Z', 'text', 'cli-live', 'studio-1', '#release', 'release', 'Tagged.', ',` + pudlKey + `,')`,
	} {
		if _, err := b.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := convertRooms(b.db); err != nil {
		t.Fatal(err)
	}
	g, err := b.Room(ctx, "release")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range g.Members {
		ids = append(ids, m.Key)
	}
	sort.Strings(ids)
	if g.Title != "release" || strings.Join(ids, ",") != "cli-live,paul,window-closed,window-open" {
		t.Fatalf("converted to %q with %v", g.Title, ids)
	}
	msgs, _ := b.RoomMessages(ctx, "release", 10)
	if len(msgs) != 1 || msgs[0].Text != "Tagged." || msgs[0].From != "Terminal Chat" || len(msgs[0].Mentioned) != 1 || msgs[0].Mentioned[0].Title != "pudl" {
		t.Fatalf("the room's message: %+v", msgs)
	}
	before, _ := b.ChangeCounter(ctx)
	if err := convertRooms(b.db); err != nil {
		t.Fatal(err)
	}
	if after, _ := b.ChangeCounter(ctx); after != before {
		t.Fatal("converting again wrote to the bus")
	}
	if g2, _ := b.Room(ctx, "release"); len(g2.Members) != len(g.Members) {
		t.Fatalf("converting again changed the members: %+v", g2.Members)
	}
	// The converted group works as any other.
	if _, err := b.SendMentioning(ctx, Sender{Session: &live}, "#release", "Docs?", nil, 0, []string{"Open Docs"}); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := b.Doorbell(ctx, "window-open"); n != 1 {
		t.Fatal("a mention in the converted group did not wake its chat")
	}
}

// A chat's folder-based handle is no longer an address or a mention.
func TestHandlesAreGone(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio := workspace(t, root, "studio")
	me, _ := b.RegisterSession(ctx, "window-me", studio, "Me")
	b.RegisterSession(ctx, "window-you", studio, "You There")
	if _, err := b.db.Exec(`UPDATE sessions SET name = 'studio-2' WHERE id = 'window-you'`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{Session: &me}, "studio-2", "By the old handle.", nil, 0); err == nil {
		t.Fatal("a handle still reaches a chat")
	}
	b.CreateGroup(ctx, Sender{Session: &me}, "Pair", []string{"You There"})
	if _, err := b.SendMentioning(ctx, Sender{Session: &me}, "#Pair", "Hi.", nil, 0, []string{"studio-2"}); err == nil {
		t.Fatal("a handle is still a mention")
	}
	var name string
	b.db.QueryRow(`SELECT name FROM sessions WHERE id = 'window-me'`).Scan(&name)
	if name != "window-me" {
		t.Fatalf("a new session was given the handle %q", name)
	}
}

// A message the person writes to a group is treated like a chat's: it asks
// only the chats it mentions, and everyone asks every member, open or closed.
// Members it does not ask keep a passive notice without waking.
func TestThePersonsMessagesFollowTheMentions(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio := workspace(t, root, "studio")
	lead, _ := b.RegisterSession(ctx, "window-lead", studio, "Lead")
	docs, _ := b.RegisterSession(ctx, "window-docs", studio, "Docs Pass")
	closed, _ := b.KnowSession(ctx, "window-closed", studio, "Timesheet Applet")
	g, err := b.CreateGroup(ctx, Sender{}, "Review", []string{"Lead", "Docs Pass", "Timesheet Applet"})
	if err != nil {
		t.Fatal(err)
	}
	rings := func(id string) int { n, _, _ := b.Doorbell(ctx, id); return n }

	if _, err := b.Send(ctx, Sender{Session: &lead}, "#"+g.Name, "No one in particular.", nil, 0); err != nil {
		t.Fatal(err)
	}
	if rings(docs.ID) != 0 || rings(lead.ID) != 0 {
		t.Fatal("an informational post rang a member")
	}
	if wakes, _ := b.Waking(ctx); len(wakes) != 0 {
		t.Fatalf("a chat's message that mentions nobody has chats opened for it: %+v", wakes)
	}
	b.Deliver(ctx, docs.ID, "UserPromptSubmit")

	sent, err := b.Send(ctx, Sender{}, "#"+g.Name, "Where are we?", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sent.Mentioned) != 0 {
		t.Fatalf("the message shows mentions it did not make: %+v", sent.Mentioned)
	}
	if rings(lead.ID) != 0 || rings(docs.ID) != 0 {
		t.Fatalf("the person's message that mentions nobody rang lead %d times and docs %d", rings(lead.ID), rings(docs.ID))
	}
	if wakes, _ := b.Waking(ctx); len(wakes) != 0 {
		t.Fatalf("the person's message that mentions nobody has chats opened for it: %+v", wakes)
	}
	got, _, _ := b.Deliver(ctx, docs.ID, "UserPromptSubmit")
	if len(got) != 1 || got[0].Asked || !got[0].Notice {
		t.Fatalf("docs was not given a notice: %+v", got)
	}

	// A mention reaches the chat it names whole, and the others get a notice.
	if _, err := b.SendMentioning(ctx, Sender{}, "#"+g.Name, "Lead, status?", nil, 0, []string{lead.ID}); err != nil {
		t.Fatal(err)
	}
	// Only the explicit request rings Lead.
	if rings(lead.ID) != 1 || rings(docs.ID) != 0 {
		t.Fatalf("a mention rang lead %d times and docs %d", rings(lead.ID), rings(docs.ID))
	}
	named, _, _ := b.Deliver(ctx, lead.ID, "UserPromptSubmit")
	other, _, _ := b.Deliver(ctx, docs.ID, "UserPromptSubmit")
	if len(named) == 0 || !named[len(named)-1].Asked || named[len(named)-1].Notice || len(other) != 1 || !other[0].Notice {
		t.Fatalf("lead got %+v and docs got %+v", named, other)
	}

	// Everyone wakes the closed member too, which the window opens.
	if _, err := b.SendMentioning(ctx, Sender{}, "#"+g.Name, "All hands.", nil, 0, []string{Everyone}); err != nil {
		t.Fatal(err)
	}
	if rings(lead.ID) != 1 || rings(docs.ID) != 1 {
		t.Fatal("everyone did not wake the open members")
	}
	found := false
	wakes, _ := b.Waking(ctx)
	for _, w := range wakes {
		found = found || w.Session == closed.ID
	}
	if !found {
		t.Fatalf("the closed member is not woken: %+v", wakes)
	}
}
