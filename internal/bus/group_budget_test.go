package bus

import (
	"context"
	"strings"
	"testing"
)

// budgetGroup is a group of three chats of one workspace, with no trusted
// link, so only the hop limit and the group's budget could stop them.
type budgetGroup struct {
	t       *testing.T
	b       *Bus
	a, c, d Session
	name    string
	root    string
}

func newBudgetGroup(t *testing.T) *budgetGroup {
	t.Helper()
	ctx := context.Background()
	b, root := open(t)
	studio := workspace(t, root, "studio")
	a, _ := b.RegisterSession(ctx, "window-a", studio, "Architect")
	c, _ := b.RegisterSession(ctx, "window-c", studio, "Engine")
	d, _ := b.RegisterSession(ctx, "window-d", studio, "Front End")
	g, err := b.CreateGroup(ctx, Sender{Session: &a}, "Work", []string{"Engine", "Front End"})
	if err != nil {
		t.Fatal(err)
	}
	return &budgetGroup{t: t, b: b, a: a, c: c, d: d, name: g.Name, root: root}
}

func (g *budgetGroup) say(from *Session, text string, mentions ...string) {
	g.t.Helper()
	sender := Sender{}
	if from != nil {
		sender = Sender{Session: from}
	}
	if _, err := g.b.SendMentioning(context.Background(), sender, "#"+g.name, text, nil, 0, mentions); err != nil {
		g.t.Fatal(err)
	}
}

func (g *budgetGroup) rings(s Session) int {
	n, _, _ := g.b.Doorbell(context.Background(), s.ID)
	return n
}

func (g *budgetGroup) take(s Session, event string) []Message {
	msgs, _, err := g.b.Deliver(context.Background(), s.ID, event)
	if err != nil {
		g.t.Fatal(err)
	}
	return msgs
}

func (g *budgetGroup) budget(n int) {
	g.t.Helper()
	if err := g.b.SetGroupBudget(context.Background(), g.name, n); err != nil {
		g.t.Fatal(err)
	}
}

func (g *budgetGroup) room() Room {
	g.t.Helper()
	r, err := g.b.Room(context.Background(), g.name)
	if err != nil {
		g.t.Fatal(err)
	}
	return r
}

func (g *budgetGroup) wakes(s Session) bool {
	g.t.Helper()
	wakes, err := g.b.Waking(context.Background())
	if err != nil {
		g.t.Fatal(err)
	}
	for _, w := range wakes {
		if w.Session == s.ID {
			return true
		}
	}
	return false
}

// A group has no hop limit: two chats of one workspace, with no link, answer
// each other far past the three hops that stop direct messages.
func TestAGroupHasNoHopLimit(t *testing.T) {
	g := newBudgetGroup(t)
	if g.room().Budget != NoTurnBudget {
		t.Fatalf("a new group's budget is %d", g.room().Budget)
	}
	// Sessions far past the limit, as direct handoffs would leave them.
	if _, err := g.b.db.Exec(`UPDATE sessions SET hops = 9`); err != nil {
		t.Fatal(err)
	}
	pair := []Session{g.a, g.c}
	for turn := 1; turn <= 3*MaxHops; turn++ {
		from, to := pair[turn%2], pair[(turn+1)%2]
		g.say(&from, "your turn", to.ID)
		if g.rings(to) != 1 || !g.wakes(to) {
			t.Fatalf("exchange %d did not wake its chat (doorbell %d)", turn, g.rings(to))
		}
		msgs := g.take(to, DoorbellTurn)
		if len(msgs) != 1 || !msgs[0].Asked || msgs[0].Held {
			t.Fatalf("exchange %d: %+v", turn, msgs)
		}
	}
}

// A message outside a group is still stopped by the hop limit.
func TestDirectMessagesStillMeetTheHopLimit(t *testing.T) {
	g := newBudgetGroup(t)
	ctx := context.Background()
	pair := []Session{g.a, g.c}
	for hop := 1; hop <= MaxHops+1; hop++ {
		from, to := pair[hop%2], pair[(hop+1)%2]
		if _, err := g.b.Send(ctx, Sender{Session: &from}, to.Title, "direct", nil, 0); err != nil {
			t.Fatal(err)
		}
		n := g.rings(to)
		if want := hop <= MaxHops; (n == 1) != want {
			t.Fatalf("hop %d rang %d, want ringing %v", hop, n, want)
		}
		g.take(to, "PostToolUse")
	}
}

// With a budget of two, the first two requests to a chat wake it, and the
// third is still delivered, as background that does not ask the chat to
// answer, and wakes nothing.
func TestASpentBudgetDeliversMentionsWithoutWaking(t *testing.T) {
	g := newBudgetGroup(t)
	g.budget(2)
	for i := 1; i <= 2; i++ {
		g.say(&g.a, "please look", g.c.ID)
		if g.rings(g.c) != 1 {
			t.Fatalf("request %d did not wake", i)
		}
		if msgs := g.take(g.c, DoorbellTurn); len(msgs) != 1 || !msgs[0].Asked {
			t.Fatalf("request %d: %+v", i, msgs)
		}
	}
	if r := g.room(); r.Budget != 2 || r.Used != 2 {
		t.Fatalf("budget %d used %d", r.Budget, r.Used)
	}
	g.say(&g.a, "one more", g.c.ID)
	if g.rings(g.c) != 0 || g.wakes(g.c) {
		t.Fatal("a mention after the budget was spent woke its chat")
	}
	if msgs, block, _ := g.b.Deliver(context.Background(), g.c.ID, "Stop"); len(msgs) != 0 || block {
		t.Fatal("a spent budget kept a turn going at Stop")
	}
	msgs := g.take(g.c, "UserPromptSubmit")
	if len(msgs) != 1 || msgs[0].Asked || !msgs[0].Held {
		t.Fatalf("the held mention: %+v", msgs)
	}
	text := FormatOne(msgs[0], "Paul")
	if !strings.Contains(text, "has used its turns for now") || strings.Contains(text, "asks you to answer") {
		t.Fatalf("the held mention reads:\n%s", text)
	}
	if g.room().Used != 2 {
		t.Fatal("a held delivery spent a turn")
	}
}

// The person's post refills the budget, whoever it mentions or whether it
// mentions anyone, and the mention that was waiting then wakes its chat.
func TestThePersonsPostRefillsTheBudget(t *testing.T) {
	for _, everyone := range []bool{false, true} {
		g := newBudgetGroup(t)
		g.budget(1)
		g.say(&g.a, "first", g.c.ID)
		g.take(g.c, DoorbellTurn)
		g.say(&g.a, "second", g.c.ID)
		if g.rings(g.c) != 0 {
			t.Fatal("the spent budget woke a chat")
		}
		if everyone {
			g.say(nil, "carry on", Everyone)
		} else {
			g.say(nil, "carry on", g.c.ID)
		}
		if r := g.room(); r.Used != 0 {
			t.Fatalf("the person's post left %d used", r.Used)
		}
		// The chat that had a message waiting is rung for it and for the post.
		// The other member is rung for the notices of all three messages, and
		// the second of them counts again once the budget is whole.
		if g.rings(g.c) != 2 || !g.wakes(g.c) {
			t.Fatalf("the waiting mention did not wake after the refill (everyone %v)", everyone)
		}
		if want := 3; g.rings(g.d) != want {
			t.Fatalf("the person's post rang the other member %d times (everyone %v)", g.rings(g.d), everyone)
		}
	}
}

// Messages collected for one chat into one delivery spend one turn, however
// many there are; a message that mentions two chats spends one for each.
func TestABudgetCountsRequestsToChatsAtTheBus(t *testing.T) {
	g := newBudgetGroup(t)
	g.budget(5)
	for i := 0; i < 3; i++ {
		g.say(&g.a, "again", g.c.ID)
	}
	if msgs := g.take(g.c, DoorbellTurn); len(msgs) != 3 {
		t.Fatalf("got %d messages", len(msgs))
	}
	if g.room().Used != 1 {
		t.Fatalf("three messages in one delivery spent %d", g.room().Used)
	}
	g.say(&g.a, "both of you", g.c.ID, g.d.ID)
	g.take(g.c, DoorbellTurn)
	g.take(g.d, DoorbellTurn)
	if g.room().Used != 3 {
		t.Fatalf("a message to two chats left %d used, want 3", g.room().Used)
	}
	// A request that reaches a chat in the middle of its turn counts too.
	g.say(&g.a, "while you work", g.c.ID)
	g.take(g.c, "PostToolUse")
	if g.room().Used != 4 {
		t.Fatalf("a request read mid-turn left %d used, want 4", g.room().Used)
	}
	// Messages that ask nobody, and the person's, spend nothing.
	g.say(&g.a, "for everyone's information")
	g.take(g.c, "UserPromptSubmit")
	g.say(nil, "from the person", g.c.ID)
	g.take(g.c, DoorbellTurn)
	if g.room().Used != 0 {
		t.Fatalf("the person's post should have refilled it, %d used", g.room().Used)
	}
}

// With one turn left, two chats asked by one message cannot both take it.
func TestTwoChatsCannotTakeTheLastTurn(t *testing.T) {
	g := newBudgetGroup(t)
	g.budget(1)
	g.say(&g.a, "both of you", g.c.ID, g.d.ID)
	first := g.take(g.c, DoorbellTurn)
	second := g.take(g.d, DoorbellTurn)
	if len(first) != 1 || !first[0].Asked || len(second) != 1 || second[0].Asked || !second[0].Held {
		t.Fatalf("first %+v, second %+v", first, second)
	}
}

// A budget of nothing leaves the starting of turns to the person: the person's
// mention wakes a chat, and that chat's request to another does not.
func TestABudgetOfZeroLeavesTurnsToThePerson(t *testing.T) {
	g := newBudgetGroup(t)
	g.budget(0)
	g.say(nil, "start", g.c.ID)
	if g.rings(g.c) != 1 {
		t.Fatal("the person's mention did not wake its chat with a budget of 0")
	}
	g.take(g.c, DoorbellTurn)
	// The person's post woke the other member too, which then takes it.
	g.take(g.d, DoorbellTurn)
	g.say(&g.c, "over to you", g.d.ID)
	if g.rings(g.d) != 0 {
		t.Fatal("an agent's mention woke a chat with a budget of 0")
	}
}

// At Stop, an asking mention within the budget keeps the turn going, and one
// past it waits.
func TestStopContinuesOnlyWithinTheBudget(t *testing.T) {
	g := newBudgetGroup(t)
	g.budget(1)
	g.say(&g.a, "first", g.c.ID)
	msgs, block, _ := g.b.Deliver(context.Background(), g.c.ID, "Stop")
	if len(msgs) != 1 || !block {
		t.Fatalf("Stop within the budget: %d messages, block %v", len(msgs), block)
	}
	g.say(&g.a, "second", g.c.ID)
	if msgs, block, _ := g.b.Deliver(context.Background(), g.c.ID, "Stop"); len(msgs) != 0 || block {
		t.Fatalf("Stop past the budget: %d messages, block %v", len(msgs), block)
	}
}

// A long exchange in a group does not use up the members' hop counts, so one
// of them can still wake a chat of another workspace afterwards.
func TestGroupMessagesDoNotAdvanceAChatsHopCount(t *testing.T) {
	g := newBudgetGroup(t)
	ctx := context.Background()
	other, _ := g.b.RegisterSession(ctx, "window-z", workspace(t, g.root, "pudl"), "Docs")
	pair := []Session{g.a, g.c}
	for turn := 1; turn <= 4*MaxHops; turn++ {
		from, to := pair[turn%2], pair[(turn+1)%2]
		g.say(&from, "your turn", to.ID)
		g.take(to, DoorbellTurn)
	}
	if _, err := g.b.Send(ctx, Sender{Session: &g.c}, "Docs", "a direct question", nil, 0); err != nil {
		t.Fatal(err)
	}
	if g.rings(other) != 1 {
		t.Fatal("a direct message after a long group exchange was past the hop limit")
	}
}

func TestSettingAGroupBudget(t *testing.T) {
	g := newBudgetGroup(t)
	ctx := context.Background()
	if err := g.b.SetGroupBudget(ctx, g.name, -2); err == nil {
		t.Fatal("a budget below -1 was accepted")
	}
	if err := g.b.SetGroupBudget(ctx, "no such group", 3); err == nil {
		t.Fatal("a group that does not exist was accepted")
	}
	g.budget(2)
	g.say(&g.a, "x", g.c.ID)
	g.take(g.c, DoorbellTurn)
	g.budget(4)
	if r := g.room(); r.Budget != 4 || r.Used != 0 {
		t.Fatalf("choosing a budget left %d of %d used", r.Used, r.Budget)
	}
	g.budget(NoTurnBudget)
	if r := g.room(); r.Budget != NoTurnBudget {
		t.Fatalf("budget %d", r.Budget)
	}
	rooms, err := g.b.Rooms(ctx)
	if err != nil || len(rooms) != 1 || rooms[0].Budget != NoTurnBudget {
		t.Fatalf("the group list: %+v %v", rooms, err)
	}
}

// A database made before budgets gets the columns when it is opened, and
// every group in it has no limit.
func TestAnOlderDatabaseGetsGroupBudgets(t *testing.T) {
	g := newBudgetGroup(t)
	for _, col := range []string{"turn_budget", "turns_used"} {
		if _, err := g.b.db.Exec(`ALTER TABLE rooms DROP COLUMN ` + col); err != nil {
			t.Fatal(err)
		}
	}
	if err := upgrade(g.b.db); err != nil {
		t.Fatal(err)
	}
	if r := g.room(); r.Budget != NoTurnBudget || r.Used != 0 || len(r.Members) == 0 {
		t.Fatalf("after the upgrade: %+v", r)
	}
}
