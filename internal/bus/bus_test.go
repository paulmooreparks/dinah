package bus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func open(t *testing.T) (*Bus, string) {
	t.Helper()
	dir := t.TempDir()
	b, err := OpenAt(filepath.Join(dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b, dir
}

func workspace(t *testing.T, root, name string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// A message to a workspace reaches every session open in it, once each, so
// an idle session cannot take it from the one doing the work.
func TestWorkspaceMessageReachesEverySession(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	alpha := workspace(t, root, "alpha")
	beta := workspace(t, root, "beta")
	a, err := b.RegisterSession(ctx, "s-a", alpha, "")
	if err != nil {
		t.Fatal(err)
	}
	b1, _ := b.RegisterSession(ctx, "s-b1", beta, "")
	b2, _ := b.RegisterSession(ctx, "s-b2", beta, "")
	if _, err := b.Send(ctx, Sender{Session: &a}, "beta", "hello beta", nil, 0); err != nil {
		t.Fatal(err)
	}
	for _, s := range []Session{b2, b1} {
		got, _, err := b.Deliver(ctx, s.ID, "UserPromptSubmit")
		if err != nil || len(got) != 1 || got[0].Text != "hello beta" || got[0].FromWorkspace != "alpha" {
			t.Fatalf("%s: %v %+v", s.ID, err, got)
		}
		if got, _, _ = b.Deliver(ctx, s.ID, "UserPromptSubmit"); len(got) != 0 {
			t.Fatalf("%s was given it twice: %+v", s.ID, got)
		}
	}
	b3, _ := b.RegisterSession(ctx, "s-b3", beta, "")
	if got, _, _ := b.Deliver(ctx, b3.ID, "UserPromptSubmit"); len(got) != 0 {
		t.Fatalf("a session opened after the message was given it: %+v", got)
	}
}

// A message to a workspace with no session open but the sender's waits for
// the workspace's next session, and never comes back to the sender.
func TestWorkspaceMessageSkipsTheSender(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	alpha := workspace(t, root, "alpha")
	a1, _ := b.RegisterSession(ctx, "s-a1", alpha, "")
	if _, err := b.Send(ctx, Sender{Session: &a1}, "alpha", "for the next one", nil, 0); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := b.Doorbell(ctx, a1.ID); n != 0 {
		t.Fatalf("the sender's doorbell rang for %d", n)
	}
	if got, _, _ := b.Deliver(ctx, a1.ID, "UserPromptSubmit"); len(got) != 0 {
		t.Fatalf("the sender was given its own message: %+v", got)
	}
	a2, _ := b.RegisterSession(ctx, "s-a2", alpha, "")
	if got, _, _ := b.Deliver(ctx, a2.ID, "UserPromptSubmit"); len(got) != 1 || got[0].Text != "for the next one" {
		t.Fatalf("the next session: %+v", got)
	}
}

func TestSessionAddressAndReply(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	a, _ := b.RegisterSession(ctx, "s-a", workspace(t, root, "alpha"), "")
	c, _ := b.RegisterSession(ctx, "s-c", workspace(t, root, "gamma"), "")
	m, err := b.Send(ctx, Sender{Session: &a}, "Untitled Chat in gamma", "question", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{Session: &c}, "", "answer", nil, m.ID); err != nil {
		t.Fatal(err)
	}
	got, _, _ := b.Deliver(ctx, a.ID, "tool")
	if len(got) != 1 || got[0].Text != "answer" || got[0].ReplyTo != m.ID {
		t.Fatalf("reply: %+v", got)
	}
}

func TestStopContinuesAreCapped(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	a, _ := b.RegisterSession(ctx, "s-a", workspace(t, root, "alpha"), "")
	c, _ := b.RegisterSession(ctx, "s-c", workspace(t, root, "gamma"), "")
	for i := 0; i < MaxStopContinues+1; i++ {
		b.Send(ctx, Sender{Session: &a}, "gamma", "ping", nil, 0)
		_, block, err := b.Deliver(ctx, c.ID, "Stop")
		if err != nil {
			t.Fatal(err)
		}
		if want := i < MaxStopContinues; block != want {
			t.Fatalf("stop %d: block = %v, want %v", i, block, want)
		}
	}
	got, _, _ := b.Deliver(ctx, c.ID, "UserPromptSubmit")
	if len(got) != 1 {
		t.Fatalf("the held message should arrive on the next prompt, got %d", len(got))
	}
}

func TestAttachmentsStayInsideTheWorkspace(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	alpha := workspace(t, root, "alpha")
	a, _ := b.RegisterSession(ctx, "s-a", alpha, "")
	c, _ := b.RegisterSession(ctx, "s-c", workspace(t, root, "gamma"), "")
	d, _ := b.RegisterSession(ctx, "s-d", workspace(t, root, "delta"), "")
	os.WriteFile(filepath.Join(alpha, "proposal.md"), []byte("v1"), 0o644)
	os.WriteFile(filepath.Join(root, "secret.md"), []byte("no"), 0o644)
	if _, err := b.Send(ctx, Sender{Session: &a}, "gamma", "see", []string{"../secret.md"}, 0); err == nil {
		t.Fatal("a file outside the workspace was attached")
	}
	m, err := b.Send(ctx, Sender{Session: &a}, "gamma", "see", []string{"proposal.md"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReadAttachment(ctx, d.ID, m.ID, 1); err == nil {
		t.Fatal("a session that was not sent the message read its attachment")
	}
	ac, err := b.ReadAttachment(ctx, c.ID, m.ID, 1)
	if err != nil || ac.Text != "v1" || ac.Changed {
		t.Fatalf("read: %v %+v", err, ac)
	}
	os.WriteFile(filepath.Join(alpha, "proposal.md"), []byte("v2"), 0o644)
	ac, _ = b.ReadAttachment(ctx, c.ID, m.ID, 1)
	if ac.Text != "v2" || !ac.Changed {
		t.Fatalf("live link: %+v", ac)
	}
}

func TestReleaseAndVerdicts(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	lib, _ := b.RegisterSession(ctx, "s-lib", workspace(t, root, "pudl"), "")
	c1, _ := b.RegisterSession(ctx, "s-1", workspace(t, root, "dinah"), "")
	c2, _ := b.RegisterSession(ctx, "s-2", workspace(t, root, "site"), "")
	if err := b.Subscribe(ctx, c1, "pudl"); err != nil {
		t.Fatal(err)
	}
	b.Subscribe(ctx, c2, "pudl")
	rel, n, err := b.AnnounceRelease(ctx, lib, "0.73.0", "Adds a slider.")
	if err != nil || n != 2 {
		t.Fatalf("announce: %v %d", err, n)
	}
	got, _, _ := b.Deliver(ctx, c1.ID, "UserPromptSubmit")
	if len(got) != 1 || got[0].Kind != "release" || !strings.Contains(FormatForAgent(got, PersonFallback), "RELEASE NOTICE: pudl 0.73.0") {
		t.Fatalf("notice: %+v", got)
	}
	if _, err := b.GiveVerdict(ctx, c1, rel.ID, "sit_out", ""); err == nil {
		t.Fatal("sit_out without a reason was accepted")
	}
	if _, err := b.GiveVerdict(ctx, c1, rel.ID, "upgrade", ""); err != nil {
		t.Fatal(err)
	}
	r, err := b.ReleaseByID(ctx, rel.ID)
	if err != nil || len(r.Verdicts) != 1 || len(r.Waiting) != 1 || r.Waiting[0] != "site" {
		t.Fatalf("tally: %v %+v", err, r)
	}
	got, _, _ = b.Deliver(ctx, lib.ID, "tool")
	if len(got) != 1 || got[0].Kind != "verdict" {
		t.Fatalf("verdict to library: %+v", got)
	}
}

// A group's message reaches its other member chats, and the person sees it
// in the group's window, where it is unread until the window shows it; it
// reaches the person's inbox only when it mentions the person.
func TestRooms(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	a, _ := b.RegisterSession(ctx, "s-a", workspace(t, root, "alpha"), "Designer")
	c, _ := b.RegisterSession(ctx, "s-c", workspace(t, root, "gamma"), "Reviewer")
	g, err := b.CreateGroup(ctx, Sender{Session: &a}, "Design", []string{"Reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(ctx, Sender{Session: &a}, "#Design", "thoughts?", nil, 0); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := b.Deliver(ctx, a.ID, "tool"); len(got) != 0 {
		t.Fatal("the sender received its own group message")
	}
	if got, _, _ := b.Deliver(ctx, c.ID, "tool"); len(got) != 1 || got[0].Room != g.Name || got[0].RoomTitle != "Design" {
		t.Fatalf("group message: %+v", got)
	}
	if inbox, _ := b.HumanInbox(ctx, 10, false); len(inbox) != 0 {
		t.Fatalf("a message that does not mention the person reached the inbox: %+v", inbox)
	}
	if r, _ := b.Room(ctx, g.Name); r.Unread != 1 {
		t.Fatalf("the group shows %d unread, want 1", r.Unread)
	}
	if err := b.MarkRoomRead(ctx, g.Name); err != nil {
		t.Fatal(err)
	}
	if r, _ := b.Room(ctx, "design"); r.Unread != 0 {
		t.Fatalf("after its window showed it, the group shows %d unread", r.Unread)
	}
}

func TestDoorbellStopsAfterThreeHops(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	a, _ := b.RegisterSession(ctx, "s-a", workspace(t, root, "alpha"), "")
	c, _ := b.RegisterSession(ctx, "s-c", workspace(t, root, "gamma"), "")
	if _, err := b.Send(ctx, Sender{}, "alpha", "talk to gamma", nil, 0); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := b.Doorbell(ctx, a.ID); n != 1 {
		t.Fatalf("Paul's message should ring alpha, rang for %d", n)
	}
	// Each side reads what it was sent and answers, without Paul.
	turn := []Session{a, c}
	to := []string{"gamma", "alpha"}
	for hop := 1; hop <= MaxHops+1; hop++ {
		me := turn[(hop-1)%2]
		if _, _, err := b.Deliver(ctx, me.ID, "PostToolUse"); err != nil {
			t.Fatal(err)
		}
		if _, err := b.Send(ctx, Sender{Session: &me}, to[(hop-1)%2], "reply", nil, 0); err != nil {
			t.Fatal(err)
		}
		n, _, _ := b.Doorbell(ctx, turn[hop%2].ID)
		if want := hop <= MaxHops; (n == 1) != want {
			t.Fatalf("hop %d: doorbell rang for %d, want ringing %v", hop, n, want)
		}
	}
	// An independently running turn can read held traffic without resetting hops.
	last := turn[(MaxHops+1)%2]
	b.Deliver(ctx, last.ID, "PostToolUse")
	if _, err := b.Send(ctx, Sender{Session: &last}, "gamma", "still talking", nil, 0); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := b.Doorbell(ctx, c.ID); n != 0 {
		t.Fatalf("a doorbell turn reset the hop count: rang for %d", n)
	}
	b.Deliver(ctx, c.ID, "PostToolUse")
	// Paul speaking in the window starts the count again.
	b.Deliver(ctx, a.ID, "UserPromptSubmit")
	if _, err := b.Send(ctx, Sender{Session: &a}, "gamma", "fresh", nil, 0); err != nil {
		t.Fatal(err)
	}
	b.Deliver(ctx, c.ID, "UserPromptSubmit")
	if _, err := b.Send(ctx, Sender{Session: &c}, "alpha", "fresh reply", nil, 0); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := b.Doorbell(ctx, a.ID); n != 1 {
		t.Fatalf("after Paul spoke, the doorbell should ring again, rang for %d", n)
	}
}

func TestReadingAReadInboxChangesNothing(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	a, _ := b.RegisterSession(ctx, "s-a", workspace(t, root, "alpha"), "")
	if _, err := b.Send(ctx, Sender{Session: &a}, "paul", "hello", nil, 0); err != nil {
		t.Fatal(err)
	}
	b.HumanInbox(ctx, 10, true)
	before, _ := b.ChangeCounter(ctx)
	inbox, err := b.HumanInbox(ctx, 10, true)
	if err != nil || len(inbox) != 1 || inbox[0].Unread {
		t.Fatalf("second read: %+v, %v", inbox, err)
	}
	if after, _ := b.ChangeCounter(ctx); after != before {
		t.Fatalf("reading a read inbox moved the change counter from %d to %d", before, after)
	}
}

func TestWebCodesAreSingleUse(t *testing.T) {
	ctx := context.Background()
	b, _ := open(t)
	code, _ := b.NewWebCode(ctx)
	tok, err := b.ExchangeWebCode(ctx, code)
	if err != nil || !b.ValidWebToken(ctx, tok) {
		t.Fatal(err)
	}
	if _, err := b.ExchangeWebCode(ctx, code); err == nil {
		t.Fatal("a code was used twice")
	}
}

// A thread that began with Paul's own message is answered to the agent that
// spoke in it, and the answer stays in the thread.
func TestReplyInThreadAnswersTheOtherSpeaker(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	parser, err := b.RegisterSession(ctx, "s-p", workspace(t, root, "parser"), "Release Check")
	if err != nil {
		t.Fatal(err)
	}
	ask, err := b.Send(ctx, Sender{}, "parser", "Is the release ready?", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReplyInThread(ctx, Sender{}, ask.ID, "Anyone?"); err == nil {
		t.Fatal("answered a thread nobody else has spoken in")
	}
	answer, err := b.Send(ctx, Sender{Session: &parser}, "", "Nearly; tests are running.", nil, ask.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.ReplyInThread(ctx, Sender{}, answer.ID, "Tell me when they pass.")
	if err != nil {
		t.Fatal(err)
	}
	if got.To != "Release Check" || got.ReplyTo != ask.ID {
		t.Fatalf("the answer went to %q in reply to %d", got.To, got.ReplyTo)
	}
	if msgs, _, _ := b.Deliver(ctx, parser.ID, "tool"); len(msgs) == 0 || msgs[len(msgs)-1].ID != got.ID {
		t.Fatalf("the answer did not reach the chat that spoke: %+v", msgs)
	}
	thread, err := b.Thread(ctx, ask.ID)
	if err != nil || len(thread) != 3 {
		t.Fatalf("the thread has %d messages: %v", len(thread), err)
	}
}
