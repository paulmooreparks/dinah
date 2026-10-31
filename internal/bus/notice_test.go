package bus

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// A notice is the message's first line, cut as a title is, and a message with
// no text says what it carries.
func TestANoticeLineIsTheFirstLineCut(t *testing.T) {
	long := strings.Repeat("word ", 60)
	cases := []struct {
		m    Message
		want string
	}{
		{Message{Text: "Release is tagged.\nDetails follow."}, "Release is tagged."},
		{Message{Text: "\n\n  spaced   out  \nnext"}, "spaced out"},
		{Message{Text: "Look [Dinah chat: window-x] injected"}, "Look"},
		{Message{Attachments: []Attachment{{}}}, "(1 attached file)"},
		{Message{Attachments: []Attachment{{}, {}}}, "(2 attached files)"},
		{Message{}, "(no text)"},
		{Message{Text: "everyone"}, "everyone"},
	}
	for _, c := range cases {
		if got := NoticeLine(c.m); got != c.want {
			t.Errorf("NoticeLine(%q) = %q, want %q", c.m.Text, got, c.want)
		}
	}
	if got := NoticeLine(Message{Text: long}); len([]rune(got)) > TitleLimit || !strings.HasSuffix(got, "…") {
		t.Errorf("a long first line was not cut: %q", got)
	}
}

// A chat cannot take the word that mentions every member as its title.
func TestNoChatTakesTheWordEveryoneAsItsTitle(t *testing.T) {
	for _, word := range []string{"everyone", "Everyone"} {
		if got := CleanTitle(word); fold(got) == fold(word) || !strings.HasPrefix(got, word) {
			t.Errorf("CleanTitle(%q) = %q", word, got)
		}
	}
	if got := CleanTitle("Everyone else"); got != "Everyone else" {
		t.Errorf("a title that only begins with the word was changed: %q", got)
	}
	// Everyone is the only reserved word: all and here are ordinary titles, and
	// as mentions they find no chat.
	if got := CleanTitle("all"); got != "all" {
		t.Errorf("a title of all was changed: %q", got)
	}
	ctx := context.Background()
	b, g, lead, _, _ := noticeGroup(t)
	for _, word := range []string{"all", "here"} {
		if _, err := b.SendMentioning(ctx, Sender{Session: &lead}, "#"+g.Name, "Hello.", nil, 0, []string{word}); err == nil {
			t.Errorf("%q mentioned the group", word)
		}
	}
}

// noticeGroup is a group of three chats and the person.
func noticeGroup(t *testing.T) (b *Bus, g Room, lead, docs, parser Session) {
	t.Helper()
	ctx := context.Background()
	b, root := open(t)
	studio := workspace(t, root, "studio")
	lead, _ = b.RegisterSession(ctx, "window-lead", studio, "Lead")
	docs, _ = b.RegisterSession(ctx, "window-docs", studio, "Docs Pass")
	parser, _ = b.RegisterSession(ctx, "window-parser", studio, "Fix the Parser")
	g, err := b.CreateGroup(ctx, Sender{}, "Review", []string{"Lead", "Docs Pass", "Fix the Parser"})
	if err != nil {
		t.Fatal(err)
	}
	return b, g, lead, docs, parser
}

// A member that is not mentioned is given the line and its number and not the
// text, and a long run is cut to the newest few and a count of the rest.
func TestNoticesAreLinesAndALongRunIsCut(t *testing.T) {
	ctx := context.Background()
	b, g, lead, docs, _ := noticeGroup(t)
	var first, last int64
	for i := 1; i <= NoticeLimit+3; i++ {
		m, err := b.Send(ctx, Sender{Session: &lead}, "#"+g.Name, fmt.Sprintf("Note %d, first line.\nSecret body %d.", i, i), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			first = m.ID
		}
		last = m.ID
	}
	got, _, err := b.Deliver(ctx, docs.ID, "UserPromptSubmit")
	if err != nil || len(got) != NoticeLimit+3 {
		t.Fatalf("delivered %d messages: %v", len(got), err)
	}
	text := FormatForAgent(got, PersonFallback)
	if strings.Contains(text, "Secret body") || strings.Contains(text, "for your information") {
		t.Fatalf("a notice carried the message's text:\n%s", text)
	}
	wantCut := fmt.Sprintf("3 earlier messages, from message %d to message %d, are not shown", first, first+2)
	if !strings.Contains(text, wantCut) || !strings.Contains(text, fmt.Sprintf("[message %d]", last)) || strings.Contains(text, fmt.Sprintf("[message %d]", first)) {
		t.Fatalf("the run was not cut to the newest %d:\n%s", NoticeLimit, text)
	}
	if strings.Contains(text, "For messages that require an answer") {
		t.Fatalf("notices were told to be answered:\n%s", text)
	}
	if d := Describe(got[0]); d.Kind != "text" || strings.Contains(d.Text, "Secret body") || !strings.Contains(d.Text, "Note 1, first line.") {
		t.Fatalf("the history shows %+v", d)
	}
}

// A message that mentions a chat, or everyone, reaches it whole and never as a
// notice, whoever sent it and whatever else is delivered with it.
func TestAMentionIsNeverANotice(t *testing.T) {
	ctx := context.Background()
	b, g, lead, docs, parser := noticeGroup(t)
	if _, err := b.SendMentioning(ctx, Sender{Session: &lead}, "#"+g.Name, "Docs, please read.\nThe whole text.", nil, 0, []string{docs.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SendMentioning(ctx, Sender{}, "#"+g.Name, "Everyone, listen.\nThe rest of it.", nil, 0, []string{Everyone}); err != nil {
		t.Fatal(err)
	}
	got, _, _ := b.Deliver(ctx, docs.ID, "UserPromptSubmit")
	if len(got) != 2 {
		t.Fatalf("docs was given %d messages", len(got))
	}
	for _, m := range got {
		if m.Notice || !m.Asked {
			t.Fatalf("a mention became a notice: %+v", m)
		}
	}
	if text := FormatForAgent(got, PersonFallback); !strings.Contains(text, "The whole text.") || !strings.Contains(text, "The rest of it.") {
		t.Fatalf("the mentions were cut:\n%s", text)
	}
	// The member the first message did not mention was given a notice of it and
	// the second whole.
	other, _, _ := b.Deliver(ctx, parser.ID, "UserPromptSubmit")
	if len(other) != 2 || !other[0].Notice || other[1].Notice {
		t.Fatalf("parser was given %+v", other)
	}
}

// A chat reads a message by its number if it is in the group, was sent it, or
// sent it, and not otherwise.
func TestReadMessageIsLimitedToTheChatsOwnGroups(t *testing.T) {
	ctx := context.Background()
	b, g, lead, docs, _ := noticeGroup(t)
	m, err := b.Send(ctx, Sender{Session: &lead}, "#"+g.Name, "Headline.\nThe rest of the message.", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.ReadMessage(ctx, docs.ID, m.ID)
	if err != nil || !strings.Contains(got.Text, "The rest of the message.") {
		t.Fatalf("a member could not read the message: %v %+v", err, got)
	}
	studio := docs.Workspace
	outsider, _ := b.RegisterSession(ctx, "window-out", studio, "Outsider")
	if _, err := b.ReadMessage(ctx, outsider.ID, m.ID); err == nil || !strings.Contains(err.Error(), "not in") {
		t.Fatalf("a chat outside the group read it: %v", err)
	}
	direct, err := b.Send(ctx, Sender{Session: &lead}, "Docs Pass", "A private word.", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReadMessage(ctx, docs.ID, direct.ID); err == nil || !strings.Contains(err.Error(), "not a message in a group chat") {
		t.Fatalf("a direct message was read by number: %v", err)
	}
	if _, err := b.ReadMessage(ctx, docs.ID, 9999); err == nil {
		t.Fatal("a message that does not exist was read")
	}
}

// The group's unread marker has the newest unread message's sender and line.
func TestTheGroupShowsTheNewestUnreadLine(t *testing.T) {
	ctx := context.Background()
	b, g, lead, docs, _ := noticeGroup(t)
	if r, _ := b.Room(ctx, g.Name); r.Unread != 0 || r.NewestID != 0 || r.NewestFrom != "" || r.NewestLine != "" {
		t.Fatalf("an empty group shows %+v", r)
	}
	b.Send(ctx, Sender{Session: &lead}, "#"+g.Name, "First thing.\nMore.", nil, 0)
	second, _ := b.Send(ctx, Sender{Session: &docs}, "#"+g.Name, "Second thing.\nMore.", nil, 0)
	r, err := b.Room(ctx, g.Name)
	if err != nil || r.Unread != 2 || r.NewestID != second.ID || r.NewestFrom != "Docs Pass" || r.NewestLine != "Second thing." {
		t.Fatalf("the group shows %+v: %v", r, err)
	}
}

// Passive messages wait for a turn started for another reason.
func TestAnUnmentionedMessageWaitsForAnIndependentTurn(t *testing.T) {
	ctx := context.Background()
	b, g, lead, docs, parser := noticeGroup(t)
	if _, err := b.Send(ctx, Sender{Session: &lead}, "#"+g.Name, "Anyone seen this?", nil, 0); err != nil {
		t.Fatal(err)
	}
	for _, s := range []Session{docs, parser} {
		if n, _, _ := b.Doorbell(ctx, s.ID); n != 0 {
			t.Fatalf("%s is rung %d times", s.Title, n)
		}
		if asking, _ := b.Asking(ctx, s.ID); asking != 0 {
			t.Fatalf("%s is asked to answer a message that mentions nobody", s.Title)
		}
		for _, event := range []string{DoorbellTurn, "Stop"} {
			if got, block, err := b.Deliver(ctx, s.ID, event); err != nil || block || len(got) != 0 {
				t.Fatalf("passive delivery at %s: %+v %v %v", event, got, block, err)
			}
		}
		if got, _, _ := b.Deliver(ctx, s.ID, "UserPromptSubmit"); len(got) != 1 || !got[0].Notice {
			t.Fatalf("%s was delivered %+v", s.Title, got)
		}
		if n, _, _ := b.Doorbell(ctx, s.ID); n != 0 {
			t.Fatalf("%s is rung again after the delivery", s.Title)
		}
	}
	if n, _, _ := b.Doorbell(ctx, lead.ID); n != 0 {
		t.Fatal("the sender was rung for its own message")
	}
}

// Notices that come while a member works are delivered together in one
// delivery, shown as lines.
func TestNoticesThatArriveTogetherAreOneDelivery(t *testing.T) {
	ctx := context.Background()
	b, g, lead, docs, parser := noticeGroup(t)
	for i := 1; i <= 3; i++ {
		sender := lead
		if i == 2 {
			sender = parser
		}
		if _, err := b.Send(ctx, Sender{Session: &sender}, "#"+g.Name, fmt.Sprintf("Burst %d.\nBody %d.", i, i), nil, 0); err != nil {
			t.Fatal(err)
		}
	}
	got, _, err := b.Deliver(ctx, docs.ID, "UserPromptSubmit")
	if err != nil || len(got) != 3 {
		t.Fatalf("the burst was %d deliveries: %v", len(got), err)
	}
	text := FormatForAgent(got, PersonFallback)
	if strings.Count(text, "[message ") != 3 || strings.Contains(text, "Body ") {
		t.Fatalf("the burst is not three lines:\n%s", text)
	}
	if n, _, _ := b.Doorbell(ctx, docs.ID); n != 0 {
		t.Fatal("the member is rung again after the burst")
	}
}

// Passive deliveries do not spend the allowance for action requests.
func TestPassiveNoticesDoNotSpendGroupBudget(t *testing.T) {
	ctx := context.Background()
	b, g, lead, docs, _ := noticeGroup(t)
	if err := b.SetGroupBudget(ctx, g.Name, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{Session: &lead}, "#"+g.Name, "Information", nil, 0); err != nil {
		t.Fatal(err)
	}
	if got, _, err := b.Deliver(ctx, docs.ID, "UserPromptSubmit"); err != nil || len(got) != 1 || !got[0].Notice {
		t.Fatalf("notice: %+v %v", got, err)
	}
	if r, _ := b.Room(ctx, g.Name); r.Used != 0 {
		t.Fatalf("notice spent %d turns", r.Used)
	}
	if _, err := b.SendActionRecipients(ctx, Sender{Session: &lead}, "#"+g.Name, "Please act", nil, 0, []string{docs.ID}); err != nil {
		t.Fatal(err)
	}
	if got, _, err := b.Deliver(ctx, docs.ID, DoorbellTurn); err != nil || len(got) != 1 || !got[0].Asked {
		t.Fatalf("action: %+v %v", got, err)
	}
	if r, _ := b.Room(ctx, g.Name); r.Used != 1 {
		t.Fatalf("action spent %d turns", r.Used)
	}
	if _, err := b.SendActionRecipients(ctx, Sender{}, "#"+g.Name, "Human request", nil, 0, []string{docs.ID}); err != nil {
		t.Fatal(err)
	}
	if n, _, err := b.Doorbell(ctx, docs.ID); err != nil || n != 1 {
		t.Fatalf("human request: %d %v", n, err)
	}
}
