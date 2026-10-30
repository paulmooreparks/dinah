package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// The strip shows each member's state with its reason, its task from the lead's last message to it with that
// message's number, the group's stable key, nested members under their lead, and how many members under a lead are
// stuck; the lead's information says which team a member is in, with the group's key.
func TestTheTeamStripShowsStateReasonTaskAndNesting(t *testing.T) {
	a, lead, member, leadBus, memberBus, _ := teamApp(t)
	// A third chat led by the member, so teams nest.
	ctx := context.Background()
	err := a.busHost().With(func(b *bus.Bus) error {
		s, err := b.RegisterSession(ctx, "window-nested", lead.Cwd(), "Nested Worker")
		if err != nil {
			return err
		}
		if _, err := b.SetLead(ctx, s.ID, memberBus); err != nil {
			return err
		}
		// The lead's assignment to the member.
		ls, _ := b.Session(ctx, leadBus)
		group, _ := b.TeamGroup(ctx, leadBus)
		_, err = b.SendMentioning(ctx, bus.Sender{Session: &ls}, "#"+group, "Fix the parser.\nThe long brief follows.", nil, 0, []string{memberBus})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := a.Team(lead.ID())
	if err != nil {
		t.Fatal(err)
	}
	if view.Lead.Bus != leadBus || view.Group == "" || len(view.Members) != 1 || view.StateGen == "" || view.StateVer == 0 {
		t.Fatalf("the strip: %+v", view)
	}
	m := view.Members[0]
	if m.Bus != memberBus || m.Chat != member.ID() || m.Task != "Fix the parser." || m.TaskMsg == 0 || m.State != "idle" || m.StateWhy == "" || len(m.Members) != 1 {
		t.Fatalf("the member: %+v", m)
	}
	if m.Members[0].Title != "Nested Worker" || m.Members[0].State != "closed" {
		t.Fatalf("the nested member: %+v", m.Members[0])
	}
	// A member that is stuck is counted under every lead above it.
	member.SetHold(&sessionhost.ContextHold{Kind: "context", Used: 1, Size: 2, Actions: []string{sessionhost.HoldCompact}})
	again, _ := a.Team(lead.ID())
	if again.Members[0].State != "stuck" || !strings.Contains(again.Members[0].StateWhy, "context window is full") || again.Stuck != 1 || again.StateVer <= view.StateVer {
		t.Fatalf("a stuck member: %+v", again)
	}
	// The member's information says which team it is in.
	l, err := a.ChatLead(member.ID())
	if err != nil || l == nil || l.Bus != leadBus || l.Group != view.Group || l.Title == "" {
		t.Fatalf("the member's lead: %+v %v", l, err)
	}
	if none, _ := a.ChatLead(lead.ID()); none != nil {
		t.Fatalf("a chat without a lead has %+v", none)
	}
	// A member waiting for its lead to relay shows as waiting.
	member.SetHold(nil)
	a.busHost().With(func(b *bus.Bus) error {
		s, _ := b.Session(ctx, memberBus)
		group, _ := b.TeamGroup(ctx, leadBus)
		_, err := b.SendMentioning(ctx, bus.Sender{Session: &s}, "#"+group, "Paul, a question.", nil, 0, []string{"paul"})
		return err
	})
	waiting, _ := a.Team(lead.ID())
	if waiting.Members[0].State != "waiting" || !strings.Contains(waiting.Members[0].StateWhy, "relay") {
		t.Fatalf("a waiting member: %+v", waiting.Members[0])
	}
}

// The page makes and ends the relations, and a group becomes a team with the chats that cannot be members left out.
func TestThePageMakesAndEndsTeams(t *testing.T) {
	a, lead, member, leadBus, _, _ := teamApp(t)
	if err := a.ClearTeamLead(member.ID()); err != nil {
		t.Fatal(err)
	}
	if l, _ := a.ChatLead(member.ID()); l != nil {
		t.Fatalf("a chat turned back has %+v", l)
	}
	group, err := a.SetTeamLead(member.ID(), lead.ID())
	if err != nil || group == "" {
		t.Fatalf("SetTeamLead %q %v", group, err)
	}
	if _, err := a.SetTeamLead(lead.ID(), member.ID()); err == nil || !strings.Contains(err.Error(), "already leads") {
		t.Fatalf("a cycle: %v", err)
	}
	if _, err := a.SetTeamLead("", lead.ID()); err == nil {
		t.Fatal("an empty chat was accepted")
	}
	// A group that is already the lead's team channel is made its team again without change, and says who is in it.
	made, err := a.MakeGroupATeam(group, lead.ID())
	if err != nil || len(made.Made) != 1 || made.Made[0].Chat != member.ID() || len(made.LeftOut) != 0 || made.Group != group {
		t.Fatalf("MakeGroupATeam %+v %v", made, err)
	}
	_ = leadBus
}

// The strip is sent as a numbered event when what it shows changes, and not again while it is the same.
func TestTheStripIsSentAsANumberedEventWhenItChanges(t *testing.T) {
	a, lead, member, _, _, _ := teamApp(t)
	var mu sync.Mutex
	var versions []uint64
	a.sink = func(name string, data any) {
		if name == "team:state" {
			if v, ok := data.(TeamView); ok {
				mu.Lock()
				versions = append(versions, v.StateVer)
				mu.Unlock()
			}
		}
	}
	a.sendTeamStates()
	member.SetHold(&sessionhost.ContextHold{Kind: "context", Used: 1, Size: 2, Actions: []string{sessionhost.HoldCompact}})
	a.sendTeamStates()
	a.sendTeamStates()
	mu.Lock()
	got := append([]uint64(nil), versions...)
	mu.Unlock()
	if len(got) != 2 || got[1] <= got[0] {
		t.Fatalf("the events %v: one for the first state and one for the change, and none when nothing changed", got)
	}
	_ = lead
	// A change is sent on its own a moment after the app says something changed.
	member.SetHold(nil)
	a.teamsChanged()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(versions)
		mu.Unlock()
		if n == 3 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("a change was not sent after the delay")
}

// A member's information says which team it is in and the team group's key, and says again when that changes; a delivered
// message says which group it was posted in; and the strip gives a member's own usage and the group an assignment was posted in.
func TestAMembersInformationSaysWhichTeamItIsInAndAssignmentsSayWhere(t *testing.T) {
	a, lead, member, leadBus, memberBus, _ := teamApp(t)
	var infos []SessionInfo
	a.sink = func(name string, data any) {
		if name == "agent:session" {
			if info, ok := data.(SessionInfo); ok && info.ID == member.ID() {
				infos = append(infos, info)
			}
		}
	}
	a.refreshChatLeads()
	info := member.Info()
	view, _ := a.Team(lead.ID())
	if info.Lead == nil || info.Lead.Bus != leadBus || info.Lead.Group != view.Group || info.Lead.Chat != lead.ID() || info.Lead.Title == "" {
		t.Fatalf("the member's information: %+v", info.Lead)
	}
	if len(infos) != 1 {
		t.Fatalf("the information was sent %d times", len(infos))
	}
	a.refreshChatLeads()
	if len(infos) != 1 {
		t.Fatalf("an unchanged lead sent the information again: %d", len(infos))
	}
	if lead.Info().Lead != nil {
		t.Fatalf("the lead has a lead: %+v", lead.Info().Lead)
	}
	if err := a.ClearTeamLead(member.ID()); err != nil {
		t.Fatal(err)
	}
	a.refreshChatLeads()
	if member.Info().Lead != nil || len(infos) != 2 {
		t.Fatalf("after the member was turned back: %+v, %d sends", member.Info().Lead, len(infos))
	}
	// A delivered message says its group.
	entries := sessionhost.DeliveredEntries([]bus.Message{{ID: 9, Kind: "text", FromName: "x", Text: "hello", Room: "team-room", RoomTitle: "The Team"}})
	if len(entries) != 1 || entries[0].Delivered.Room != "team-room" || entries[0].Delivered.RoomTitle != "The Team" {
		t.Fatalf("a delivered message's group: %+v", entries)
	}
	// The strip gives a member's own usage, with whether it is complete, and the group an assignment was posted in.
	if _, err := a.SetTeamLead(member.ID(), lead.ID()); err != nil {
		t.Fatal(err)
	}
	costUpdate(t, a, member.ID(), 400)
	a.recordTurnUsage(member)
	a.recordTurnUsage(member)
	err := a.busHost().With(func(b *bus.Bus) error {
		ls, _ := b.Session(context.Background(), leadBus)
		group, _ := b.TeamGroup(context.Background(), leadBus)
		_, err := b.SendMentioning(context.Background(), bus.Sender{Session: &ls}, "#"+group, "Do the thing.", nil, 0, []string{memberBus})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	strip, _ := a.Team(lead.ID())
	m := strip.Members[0]
	if m.TaskGroup != strip.Group || m.TaskGroup == "" || len(m.Usage) != 1 || m.Usage[0].Used != 400 || m.Usage[0].Unknown != 1 || m.Usage[0].Complete {
		t.Fatalf("the member in the strip: %+v", m)
	}
}

// A chat that is not on the bus has no lead and leads no one, asking about it makes no bus identity for it, and a change that
// names one is refused.
func TestAChatNotOnTheBusHasNoTeamAndIsNotGivenAnIdentity(t *testing.T) {
	a, lead, _, _, _, _ := teamApp(t)
	before := len(a.settings.BusSessions)
	if l, err := a.ChatLead("codex:unmapped"); err != nil || l != nil {
		t.Fatalf("ChatLead of an unmapped chat: %+v %v", l, err)
	}
	view, err := a.Team("codex:unmapped")
	if err != nil || len(view.Members) != 0 || view.StateGen == "" {
		t.Fatalf("Team of an unmapped chat: %+v %v", view, err)
	}
	if _, err := a.SetTeamLead("codex:unmapped", lead.ID()); err == nil {
		t.Fatal("an unmapped chat was made a member")
	}
	if _, err := a.SetTeamBudget("codex:unmapped", nil); err == nil {
		t.Fatal("a budget was set for an unmapped chat")
	}
	if err := a.ClearTeamLead("codex:unmapped"); err != nil {
		t.Fatalf("clearing an unmapped chat: %v", err)
	}
	if len(a.settings.BusSessions) != before {
		t.Fatal("asking about an unmapped chat gave it an identity")
	}
}

// An allowance of 0 is an allowance, which no allowance is not, and a member's use says today's apart from the period's.
func TestAnExplicitZeroAllowanceDiffersFromNoAllowance(t *testing.T) {
	a, lead, member, _, _, _ := teamApp(t)
	if _, err := a.SetTeamBudget(lead.ID(), []TeamAllowanceInput{{Provider: "codex", Allowance: 0}}); err != nil {
		t.Fatal(err)
	}
	costUpdate(t, a, member.ID(), 5)
	a.recordTurnUsage(member)
	view, _ := a.Team(lead.ID())
	var codex *TeamBudgetRow
	for i := range view.Budget.Rows {
		if view.Budget.Rows[i].Provider == "codex" {
			codex = &view.Budget.Rows[i]
		}
	}
	if codex == nil || !codex.HasAllowance || codex.Allowance != 0 || !codex.Over {
		t.Fatalf("an explicit zero allowance: %+v", codex)
	}
	if u := view.Members[0].Usage; len(u) != 1 || u[0].Today != 5 || u[0].TodayUnknown != 0 || u[0].Used != 5 {
		t.Fatalf("a member's use: %+v", u)
	}
	if _, err := a.SetTeamBudget(lead.ID(), nil); err != nil {
		t.Fatal(err)
	}
	view, _ = a.Team(lead.ID())
	for _, r := range view.Budget.Rows {
		if r.Provider == "codex" && r.HasAllowance {
			t.Fatalf("a cleared allowance is still set: %+v", r)
		}
	}
}

// The lead's own successful sends to the team, and a member's, are marked in the history of the chat that sent them with the
// group and the message's number, once each and in order, with Paul's direct conversation and the traffic coming in between them
// kept in their places; a send that failed never reached the bus and is not marked. The marks are saved, so they survive a reopen.
func TestATeamChatsSendsAreMarkedInItsHistoryInOrder(t *testing.T) {
	a, lead, member, leadBus, memberBus, _ := teamApp(t)
	ctx := context.Background()
	var events []map[string]any
	a.sink = func(name string, data any) {
		if name == sessionhost.EventTeamSent {
			if m, ok := data.(map[string]any); ok {
				events = append(events, m)
			}
		}
	}
	pass := func() {
		if err := a.busHost().With(func(b *bus.Bus) error { a.recordTeamSends(ctx, b); return nil }); err != nil {
			t.Fatal(err)
		}
	}
	pass() // finds where the bus is
	var group string
	send := func(fromBus, text string, mentions ...string) int64 {
		var id int64
		err := a.busHost().With(func(b *bus.Bus) error {
			s, _ := b.Session(ctx, fromBus)
			group, _ = b.TeamGroup(ctx, leadBus)
			m, err := b.SendMentioning(ctx, bus.Sender{Session: &s}, "#"+group, text, nil, 0, mentions)
			id = m.ID
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := send(leadBus, "Do the first thing.", memberBus)
	pass()
	// Incoming traffic for the lead, then Paul talks to the lead directly, then the lead sends again.
	incoming := send(memberBus, "Done with the first.", leadBus)
	pass()
	a.transcriptCapture().RecordDelivered(lead, "", sessionhost.DeliveredEntries([]bus.Message{{ID: incoming, Kind: "text", FromName: "Member", Text: "Done with the first.", Room: group}}))
	a.transcriptCapture().RecordMessage(lead, "you", "Paul says: how is it going?", false, false)
	second := send(leadBus, "Do the second thing.", memberBus)
	pass()
	// A send that cannot succeed, a mention of a chat that is not in the group, is refused by the bus and marked nowhere.
	err := a.busHost().With(func(b *bus.Bus) error {
		s, _ := b.Session(ctx, leadBus)
		_, err := b.SendMentioning(ctx, bus.Sender{Session: &s}, "#"+group, "Nobody there.", nil, 0, []string{"Nobody Here"})
		return err
	})
	if err == nil {
		t.Fatal("a mention of a chat that is not a member was sent")
	}
	pass()
	pass() // nothing is marked twice
	tr, err := readTranscript(lead.ID())
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, m := range tr.Messages {
		switch {
		case m.Role == "sent":
			order = append(order, fmt.Sprintf("sent %d in %s", m.Delivered.ID, m.Delivered.Room))
		case m.Role == "delivered":
			order = append(order, fmt.Sprintf("delivered %d", m.Delivered.ID))
		case m.Role == "you":
			order = append(order, "you")
		}
	}
	want := []string{fmt.Sprintf("sent %d in %s", first, group), fmt.Sprintf("delivered %d", incoming), "you", fmt.Sprintf("sent %d in %s", second, group)}
	if !slices.Equal(order, want) {
		t.Fatalf("the history: %v, want %v", order, want)
	}
	// The sends the lead made are marked once each, around the traffic that came in and Paul's own words.
	var sentOnly []string
	for _, o := range order {
		if strings.HasPrefix(o, "sent ") {
			sentOnly = append(sentOnly, o)
		}
	}
	if len(sentOnly) != 2 || sentOnly[0] != want[0] || sentOnly[1] != want[3] {
		t.Fatalf("the lead's marked sends: %v", order)
	}
	if len(events) != 3 || events[0]["message"] == nil || events[0]["text"] != "Do the first thing." {
		t.Fatalf("%d events", len(events))
	}
	// The member's own send is marked in the member's history.
	if mt, err := readTranscript(member.ID()); err != nil || len(mt.Messages) == 0 || mt.Messages[len(mt.Messages)-1].Role != "sent" || mt.Messages[len(mt.Messages)-1].Delivered.ID != incoming {
		t.Fatalf("the member's history: %+v %v", mt, err)
	}
}

// Starting a member leaves a line in the team's group with its title, provider, model and the first line of its brief, and the
// budget says which providers are not measured.
func TestStartingAMemberLeavesALineAndTheBudgetListsEveryProvider(t *testing.T) {
	a, c := providerTestApp(t)
	c.newIDs = []string{"codex:lead", "codex:new"}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lead := a.session(info.ID)
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "w", Name: "W", ExecutionPath: lead.Cwd()}}
	out, err := call(t, a.teamStartHandler, asChat(a, lead), map[string]any{"title": "Fix the Parser", "brief": "Fix the parser.\nThe long brief."})
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	a.busHost().With(func(b *bus.Bus) error {
		msgs, _ := b.RoomMessages(context.Background(), out["group"].(string), 20)
		for _, m := range msgs {
			if m.Kind == bus.MemberKind {
				lines = append(lines, m.MemberLine("Paul"))
			}
		}
		return nil
	})
	var started string
	for _, l := range lines {
		if strings.Contains(l, " started ") {
			started = l
		}
	}
	if !strings.Contains(started, "Fix the Parser") || !strings.Contains(started, "Codex") || !strings.Contains(started, "Fix the parser.") || strings.Contains(started, "The long brief") {
		t.Fatalf("the start line is %q among %v", started, lines)
	}
	view, _ := a.Team(lead.ID())
	var names []string
	for _, p := range view.Budget.Providers {
		names = append(names, p.Provider+":"+p.Unit)
	}
	if !slices.Contains(names, "claude:tokens") || !slices.Contains(names, "devin:acu") || !slices.Contains(view.Budget.Unmeasured, "claude") || slices.Contains(view.Budget.Unmeasured, "codex") {
		t.Fatalf("the budget's providers %v, unmeasured %v", names, view.Budget.Unmeasured)
	}
}
