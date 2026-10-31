package bus

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

// teamBus is a bus with a lead, two members (Docs Pass and Fix the Parser) and an outside chat, and
// the group Review they are all in with the person.
func teamBus(t *testing.T) (b *Bus, review Room, lead, docs, parser, outside Session) {
	t.Helper()
	ctx := context.Background()
	b, review, lead, docs, parser = noticeGroup(t)
	outside, err := b.RegisterSession(ctx, "window-outside", docs.WorkspacePath, "Outside")
	if err != nil {
		t.Fatal(err)
	}
	return b, review, lead, docs, parser, outside
}

func sendTo(t *testing.T, b *Bus, from Session, group, text string, mentions ...string) Message {
	t.Helper()
	m, err := b.SendMentioning(context.Background(), Sender{Session: &from}, "#"+group, text, nil, 0, mentions)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func personMentions(t *testing.T, b *Bus) []Message {
	t.Helper()
	ms, err := b.Mentions(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

func mustGroup(t *testing.T, b *Bus, lead string) string {
	t.Helper()
	g, err := b.TeamGroup(context.Background(), lead)
	if err != nil || g == "" {
		t.Fatalf("the lead's team group: %q %v", g, err)
	}
	return g
}

// A member's mention of the person goes to its lead as a request to relay: it shows in the group for
// the person, who is not notified, and the lead is asked to answer even when it is not in the group.
func TestAMembersMentionOfThePersonGoesToItsLead(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, parser, outside := teamBus(t)
	if _, err := b.SetLead(ctx, docs.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	// An ordinary chat's mention notifies the person.
	m := sendTo(t, b, parser, review.Name, "Ordinary.", "paul")
	if got := personMentions(t, b); len(got) != 1 || got[0].ID != m.ID {
		t.Fatalf("an ordinary mention: %v", got)
	}
	// A member's does not, and it reaches the lead, which is in no group with it.
	relayed := sendTo(t, b, docs, review.Name, "Needs the person's call.", "paul")
	if got := personMentions(t, b); len(got) != 1 {
		t.Fatalf("a member's mention notified the person: %d mentions", len(got))
	}
	shown, err := b.Message(ctx, relayed.ID)
	if err != nil || !shown.ForPerson || shown.RelayLead != outside.ID || shown.RelayLeadTitle == "" {
		t.Fatalf("the message as the group shows it: %+v %v", shown, err)
	}
	for _, x := range shown.Mentioned {
		if x.Key == Human {
			t.Fatalf("the message still mentions the person: %+v", shown.Mentioned)
		}
	}
	got, _, err := b.Deliver(ctx, outside.ID, DoorbellTurn)
	if err != nil || len(got) != 1 || got[0].ID != relayed.ID || !got[0].Relay || !got[0].Asked {
		t.Fatalf("the lead's delivery: %+v %v", got, err)
	}
	if text := FormatForAgent(got, PersonFallback); !strings.Contains(text, "request to relay") || !strings.Contains(text, "your own message") {
		t.Fatalf("the lead is not told what to do:\n%s", text)
	}
	// The other members read it as an ordinary message of the group, not as a request.
	if others, _, _ := b.Deliver(ctx, lead.ID, "UserPromptSubmit"); len(others) == 0 || others[0].Relay {
		t.Fatalf("another member's delivery: %+v", others)
	}
	// The lead relays what the person should act on as its own message, which notifies him.
	if _, err := b.SetLead(ctx, parser.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	own := sendTo(t, b, outside, mustGroup(t, b, outside.ID), "Please decide.", "paul")
	if got := personMentions(t, b); len(got) != 2 || got[1].ID != own.ID {
		t.Fatalf("the lead's relay: %v", got)
	}
	// A message that does not mention the person is not rerouted.
	plain := sendTo(t, b, docs, review.Name, "Nothing for the person.", parser.ID)
	if plain.ForPerson {
		t.Fatal("a message that does not mention the person was rerouted")
	}
}

// A nested member's message rises one lead at a time: to its lead, whose own relay goes to the lead above.
func TestANestedMembersMessageRisesThroughTwoLeads(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, parser, outside := teamBus(t)
	// outside leads lead, lead leads docs and parser.
	if _, err := b.SetLead(ctx, lead.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, docs.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, parser.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	first := sendTo(t, b, docs, review.Name, "Up to the person?", "paul")
	if got := personMentions(t, b); len(got) != 0 {
		t.Fatalf("the person was notified: %v", got)
	}
	if m, _ := b.Message(ctx, first.ID); m.RelayLead != lead.ID {
		t.Fatalf("the first hop went to %s, not %s", m.RelayLead, lead.ID)
	}
	// The lead relays in its team's group, and that message is a member's too.
	teamGroup := mustGroup(t, b, lead.ID)
	second := sendTo(t, b, lead, teamGroup, "Relaying.", "paul")
	if m, _ := b.Message(ctx, second.ID); m.RelayLead != outside.ID {
		t.Fatalf("the second hop went to %s, not %s", m.RelayLead, outside.ID)
	}
	if got := personMentions(t, b); len(got) != 0 {
		t.Fatalf("the person was notified before the top lead acted: %v", got)
	}
	// The top lead's relay reaches the person.
	top := sendTo(t, b, outside, mustGroup(t, b, outside.ID), "For the person.", "paul")
	if got := personMentions(t, b); len(got) != 1 || got[0].ID != top.ID {
		t.Fatalf("the top lead's message: %v", got)
	}
}

// One lead for a chat, moving a member leaves the old team's group, and a relation that would make a cycle is refused.
func TestTheLeadRelationIsOneLeadAndNeverACycle(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, parser, outside := teamBus(t)
	if _, err := b.SetLead(ctx, docs.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, parser.ID, docs.ID); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string][2]string{"itself": {docs.ID, docs.ID}, "own lead": {lead.ID, docs.ID}, "through a chain": {lead.ID, parser.ID}} {
		if _, err := b.SetLead(ctx, c[0], c[1]); err == nil {
			t.Fatalf("%s: a cycle was accepted", name)
		} else if name != "itself" && !strings.Contains(err.Error(), "already leads") {
			t.Fatalf("%s: the refusal does not say why: %v", name, err)
		}
	}
	if l, _, _ := b.LeadOf(ctx, lead.ID); l != "" {
		t.Fatalf("a refused relation changed the lead: %q", l)
	}
	old := mustGroup(t, b, lead.ID)
	if _, err := b.SetLead(ctx, docs.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	if l, _, _ := b.LeadOf(ctx, docs.ID); l != outside.ID {
		t.Fatalf("docs is led by %q", l)
	}
	members, _ := b.RoomMembers(ctx, old)
	for _, m := range members {
		if m.Key == docs.ID {
			t.Fatal("a moved member is still in its old team's group")
		}
	}
	if got, _ := b.TeamMembers(ctx, outside.ID); len(got) != 1 || got[0].Session != docs.ID {
		t.Fatalf("the new team: %+v", got)
	}
	// Turned back into an ordinary chat, its mention of the person reaches him directly.
	if err := b.ClearLead(ctx, docs.ID); err != nil {
		t.Fatal(err)
	}
	sendTo(t, b, docs, review.Name, "Direct again.", "paul")
	if got := personMentions(t, b); len(got) != 1 {
		t.Fatalf("a chat turned back did not reach the person: %v", got)
	}
}

// A deleted lead sends its members back to the person directly, and its group stays as an ordinary one.
func TestADeletedLeadSendsItsMembersBackToThePerson(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, parser, _ := teamBus(t)
	if _, err := b.SetLead(ctx, docs.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, parser.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	group := mustGroup(t, b, lead.ID)
	orphans, err := b.LeadRemoved(ctx, lead.ID)
	if err != nil || !slices.Equal(orphans, []string{docs.ID, parser.ID}) {
		t.Fatalf("orphans %v %v", orphans, err)
	}
	if l, _, _ := b.LeadOf(ctx, docs.ID); l != "" {
		t.Fatalf("a member of a deleted lead still has %q", l)
	}
	if g, _ := b.TeamGroup(ctx, lead.ID); g != "" {
		t.Fatalf("the deleted lead still has the team group %q", g)
	}
	if _, err := b.Room(ctx, group); err != nil {
		t.Fatalf("the group did not stay: %v", err)
	}
	sendTo(t, b, docs, review.Name, "Direct.", "paul")
	if got := personMentions(t, b); len(got) != 1 {
		t.Fatalf("the person was not reached directly: %v", got)
	}
}

// A retired member is left out of the group's wake and notice set, cannot be mentioned, and is brought back by
// SetRetired(false).
func TestARetiredMemberIsNotWokenByTheTeamGroup(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, parser, _ := teamBus(t)
	group, err := b.SetLead(ctx, docs.ID, lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, parser.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.SetRetired(ctx, docs.ID, true); err != nil {
		t.Fatal(err)
	}
	m := sendTo(t, b, lead, group, "To the parser.", parser.ID)
	got, _, _ := b.Deliver(ctx, docs.ID, DoorbellTurn)
	for _, x := range got {
		if x.ID == m.ID {
			t.Fatal("a retired member was given the group's message")
		}
	}
	if _, err := b.SendMentioning(ctx, Sender{Session: &lead}, "#"+group, "Hello.", nil, 0, []string{docs.ID}); err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("a mention of a retired member: %v", err)
	}
	wakes, _ := b.Waking(ctx)
	for _, w := range wakes {
		if w.Session == docs.ID {
			t.Fatal("a retired member was woken")
		}
	}
	// It is still a member and listed in the group.
	members, _ := b.RoomMembers(ctx, group)
	if !slices.ContainsFunc(members, func(m RoomMember) bool { return m.Key == docs.ID }) {
		t.Fatal("a retired member left the group")
	}
	if err := b.SetRetired(ctx, docs.ID, false); err != nil {
		t.Fatal(err)
	}
	sendTo(t, b, lead, group, "Back.", docs.ID)
	if got, _, _ := b.Deliver(ctx, docs.ID, DoorbellTurn); len(got) == 0 {
		t.Fatal("a member brought back was not delivered to")
	}
}

// Making a group a team makes its other chats members, except one that has another lead, which keeps it.
func TestMakingAGroupATeam(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, parser, outside := teamBus(t)
	if _, err := b.SetLead(ctx, parser.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.MakeGroupTeam(ctx, review.Name, outside.ID); err == nil {
		t.Fatal("a chat that is not in the group was made its lead")
	}
	made, left, err := b.MakeGroupTeam(ctx, review.Name, lead.ID)
	if err != nil || !slices.Equal(made, []string{docs.ID}) || len(left) != 1 || left[0].Session != parser.ID || !strings.Contains(left[0].Why, "already has a lead") {
		t.Fatalf("made %v, left %+v, %v", made, left, err)
	}
	if g, _ := b.TeamGroup(ctx, lead.ID); g != review.Name {
		t.Fatalf("the team channel is %q, not the group", g)
	}
	if l, _, _ := b.LeadOf(ctx, parser.ID); l != outside.ID {
		t.Fatalf("the chat that had a lead lost it: %q", l)
	}
	// The chat's mention of the person elsewhere still goes through its lead.
	sendTo(t, b, docs, review.Name, "Question.", "paul")
	if got := personMentions(t, b); len(got) != 0 {
		t.Fatalf("a member of the new team notified the person: %v", got)
	}
}

// Each of the failures marks the relay failed once with its reason, the person is told once, and a lead that
// simply does not relay is not a failure.
func TestARelayThatFailsIsMarkedAndReportedOnce(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, parser, _ := teamBus(t)
	for _, m := range []Session{docs, parser} {
		if _, err := b.SetLead(ctx, m.ID, lead.ID); err != nil {
			t.Fatal(err)
		}
	}
	one := sendTo(t, b, docs, review.Name, "One.", "paul")
	two := sendTo(t, b, parser, review.Name, "Two.", "paul")
	three := sendTo(t, b, docs, review.Name, "Three.", "paul")
	if pending, _ := b.PendingRelays(ctx); len(pending) != 3 {
		t.Fatalf("%d relays are pending", len(pending))
	}
	// The delivery to the lead fails.
	failed, err := b.FailPendingRelaysTo(ctx, lead.ID, "Claude could not be started")
	if err != nil || !slices.Equal(failed, []int64{one.ID, two.ID, three.ID}) {
		t.Fatalf("failed %v %v", failed, err)
	}
	if pending, _ := b.PendingRelays(ctx); len(pending) != 0 {
		t.Fatalf("%d relays are still pending", len(pending))
	}
	relays, messages, err := b.UnreportedFailures(ctx)
	if err != nil || len(relays) != 3 || relays[0].Reason != "Claude could not be started" || messages[0].ID != one.ID {
		t.Fatalf("unreported %+v %v", relays, err)
	}
	for _, r := range relays {
		if err := b.RelayReported(ctx, r.MessageID); err != nil {
			t.Fatal(err)
		}
	}
	if again, _, _ := b.UnreportedFailures(ctx); len(again) != 0 {
		t.Fatalf("%d failures were reported again", len(again))
	}
	// A failed relay keeps its first reason.
	if err := b.FailRelay(ctx, one.ID, "another reason"); err != nil {
		t.Fatal(err)
	}
	if r, ok, _ := b.RelayOf(ctx, one.ID); !ok || r.Reason != "Claude could not be started" {
		t.Fatalf("the reason changed: %+v", r)
	}
	// A turn that ended in an error fails the relays it was given, and only those.
	four := sendTo(t, b, docs, review.Name, "Four.", "paul")
	five := sendTo(t, b, parser, review.Name, "Five.", "paul")
	got, err := b.FailRelaysIn(ctx, lead.ID, []int64{four.ID}, "the turn failed: no usage left")
	if err != nil || !slices.Equal(got, []int64{four.ID}) {
		t.Fatalf("FailRelaysIn %v %v", got, err)
	}
	if r, _, _ := b.RelayOf(ctx, five.ID); r.State != RelayPending {
		t.Fatalf("a relay the turn was not given failed: %+v", r)
	}
	// A lead that has taken the delivery and chosen not to relay is no failure.
	if _, _, err := b.Deliver(ctx, lead.ID, DoorbellTurn); err != nil {
		t.Fatal(err)
	}
	if pending, _ := b.PendingRelays(ctx); len(pending) != 0 {
		t.Fatalf("a delivered relay is pending: %v", pending)
	}
	if r, _, _ := b.RelayOf(ctx, five.ID); r.State != RelayPending {
		t.Fatalf("a relay the lead took was marked failed: %+v", r)
	}
}

// The group of a deleted lead keeps a line that says its members now reach the person directly.
func TestADeletedLeadsGroupKeepsALineSayingSo(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, _, _ := teamBus(t)
	group, err := b.SetLead(ctx, docs.ID, lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.LeadRemoved(ctx, lead.ID); err != nil {
		t.Fatal(err)
	}
	msgs, _ := b.RoomMessages(ctx, group, 20)
	for _, m := range msgs {
		if m.Kind == MemberKind && strings.Contains(m.MemberLine("Paul"), "lead of this team was deleted") && strings.Contains(m.MemberLine("Paul"), "reach Paul directly") {
			return
		}
	}
	t.Fatalf("no line says the lead was deleted: %+v", msgs)
}

// A team's usage is the sum of its members' counted turns, nested teams included, by provider and period; a turn the
// provider reported nothing for is counted separately as unknown and adds nothing, and allowances are kept per provider.
func TestATeamsUsageIsPerProviderAndNeverCountsUnknownAsZero(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, parser, outside := teamBus(t)
	for _, s := range []Session{docs, parser} {
		if _, err := b.SetLead(ctx, s.ID, lead.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.SetLead(ctx, lead.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	now := b.now()
	from, to := now.Add(-time.Hour), now.Add(time.Hour)
	for _, r := range []struct {
		s        Session
		provider string
		amount   int
		known    bool
	}{{docs, "codex", 1000, true}, {docs, "codex", 500, true}, {parser, "codex", 0, false}, {parser, "claude", 0, false}, {docs, "ollama", 70, true}} {
		unit := "tokens"
		if err := b.RecordTurnUsage(ctx, r.s.ID, r.provider, unit, r.amount, r.known); err != nil {
			t.Fatal(err)
		}
	}
	// An older turn is outside the period.
	if err := b.RecordTurnUsage(ctx, docs.ID, "codex", "tokens", 99999, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.ExecContext(ctx, `UPDATE team_usage SET at = ? WHERE amount = 99999`, now.Add(-48*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	got, err := b.TeamUsage(ctx, lead.ID, from, to)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Usage{"codex": {Provider: "codex", Unit: "tokens", Used: 1500, Unknown: 1, Turns: 3}, "claude": {Provider: "claude", Unit: "tokens", Used: 0, Unknown: 1, Turns: 1}, "ollama": {Provider: "ollama", Unit: "tokens", Used: 70, Turns: 1}}
	if len(got) != 3 {
		t.Fatalf("usage %+v", got)
	}
	for _, u := range got {
		if u != want[u.Provider] {
			t.Fatalf("%s: %+v, want %+v", u.Provider, u, want[u.Provider])
		}
	}
	// The lead's lead counts the nested team's usage too.
	top, _ := b.TeamUsage(ctx, outside.ID, from, to)
	if len(top) != 3 {
		t.Fatalf("the top lead's usage: %+v", top)
	}
	// Allowances are per provider and are replaced as a set.
	if err := b.SetTeamAllowances(ctx, lead.ID, []TeamAllowance{{"codex", "tokens", 5000}, {"ollama", "tokens", 100}}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetTeamAllowances(ctx, lead.ID, []TeamAllowance{{"codex", "", 5}}); err == nil {
		t.Fatal("an allowance with no unit was accepted")
	}
	al, _ := b.TeamAllowances(ctx, lead.ID)
	if len(al) != 2 || al[0].Provider != "codex" || al[0].Allowance != 5000 {
		t.Fatalf("allowances %+v", al)
	}
	if err := b.SetTeamAllowances(ctx, lead.ID, nil); err != nil {
		t.Fatal(err)
	}
	if al, _ := b.TeamAllowances(ctx, lead.ID); len(al) != 0 {
		t.Fatalf("allowances after clearing %+v", al)
	}
}

// The person's message in a team's group with no mention is a request to the lead. A mention of a particular
// chat still reaches only that chat, and the other members still get the notice.
func TestThePersonsUnmentionedMessageInATeamGoesToItsLead(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, _, _ := teamBus(t)
	if _, _, err := b.MakeGroupTeam(ctx, review.Name, lead.ID); err != nil {
		t.Fatal(err)
	}
	m, err := b.SendMentioning(ctx, Sender{}, "#"+review.Name, "Where are we?", nil, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := b.Deliver(ctx, lead.ID, DoorbellTurn)
	if err != nil || len(got) != 1 || got[0].ID != m.ID || !got[0].Asked {
		t.Fatalf("the lead's delivery: %+v %v", got, err)
	}
	others, _, err := b.Deliver(ctx, docs.ID, "UserPromptSubmit")
	if err != nil || len(others) != 1 || others[0].ID != m.ID || others[0].Asked {
		t.Fatalf("another member should get the notice only: %+v %v", others, err)
	}
	// A mention of a member reaches that member and not the lead.
	n, err := b.SendMentioning(ctx, Sender{}, "#"+review.Name, "Docs, a question.", nil, 0, []string{docs.ID})
	if err != nil {
		t.Fatal(err)
	}
	asked, _, _ := b.Deliver(ctx, docs.ID, DoorbellTurn)
	if len(asked) != 1 || asked[0].ID != n.ID || !asked[0].Asked {
		t.Fatalf("the mentioned member: %+v", asked)
	}
	if leadGot, _, _ := b.Deliver(ctx, lead.ID, "UserPromptSubmit"); len(leadGot) != 1 || leadGot[0].Asked {
		t.Fatalf("the lead should only be told: %+v", leadGot)
	}
}

// A group that is not a team keeps its rule: an unmentioned message asks nobody.
func TestThePersonsUnmentionedMessageInAnOrdinaryGroupAsksNobody(t *testing.T) {
	ctx := context.Background()
	b, review, lead, _, _, _ := teamBus(t)
	if _, err := b.SendMentioning(ctx, Sender{}, "#"+review.Name, "Hello all.", nil, 0, nil); err != nil {
		t.Fatal(err)
	}
	got, _, _ := b.Deliver(ctx, lead.ID, DoorbellTurn)
	for _, m := range got {
		if m.Asked {
			t.Fatalf("an ordinary group's unmentioned message asked a chat: %+v", m)
		}
	}
}

// A lead that has left its team's group cannot be asked there, so the message stays an ordinary unmentioned one.
func TestAnUnmentionedMessageIsNotSentToALeadWhoLeftTheGroup(t *testing.T) {
	ctx := context.Background()
	b, review, lead, _, _, _ := teamBus(t)
	if _, _, err := b.MakeGroupTeam(ctx, review.Name, lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveFromGroup(ctx, Sender{}, review.Name, lead.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SendMentioning(ctx, Sender{}, "#"+review.Name, "Anyone?", nil, 0, nil); err != nil {
		t.Fatal(err)
	}
	got, _, _ := b.Deliver(ctx, lead.ID, DoorbellTurn)
	for _, m := range got {
		if m.Asked {
			t.Fatalf("a lead outside the group was asked: %+v", m)
		}
	}
}

func isMember(t *testing.T, b *Bus, group, session string) bool {
	t.Helper()
	var n int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM room_members WHERE room = ? AND member_kind = 'session' AND member = ?`, group, session).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

// Adding a chat to a team's group makes it a member of the team in the same step.
func TestAddingToATeamsGroupMakesTheChatAMember(t *testing.T) {
	ctx := context.Background()
	b, review, lead, _, _, outside := teamBus(t)
	if _, _, err := b.MakeGroupTeam(ctx, review.Name, lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.AddToGroup(ctx, Sender{}, review.Name, []string{outside.ID}); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := teamLeadOf(b.db, outside.ID); got != lead.ID {
		t.Fatalf("the added chat's lead is %q", got)
	}
	if !isMember(t, b, review.Name, outside.ID) {
		t.Fatal("the chat is not in the group")
	}
}

func TestAddingToATeamsGroupRefusesAChatOfAnotherTeamAndAddsNobody(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, parser, outside := teamBus(t)
	if _, _, err := b.MakeGroupTeam(ctx, review.Name, lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveFromGroup(ctx, Sender{}, review.Name, parser.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, parser.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	_ = docs
	fresh, err := b.RegisterSession(ctx, "window-fresh", docs.WorkspacePath, "Fresh")
	if err != nil {
		t.Fatal(err)
	}
	err = b.AddToGroup(ctx, Sender{}, review.Name, []string{fresh.ID, parser.ID})
	if err == nil || !strings.Contains(err.Error(), "already a member of") {
		t.Fatalf("expected a refusal naming the other team, got %v", err)
	}
	if isMember(t, b, review.Name, fresh.ID) {
		t.Fatal("a chat was added although the step was refused")
	}
	if got, _, _ := teamLeadOf(b.db, parser.ID); got != outside.ID {
		t.Fatalf("the refused chat's lead changed to %q", got)
	}
}

func TestAddingToATeamsGroupRefusesACycleAndAddsNobody(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, _, outside := teamBus(t)
	// The lead of review leads outside's chain: outside is led by lead, so adding outside's own lead... use lead under outside.
	if _, err := b.SetLead(ctx, lead.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.MakeGroupTeam(ctx, review.Name, docs.ID); err != nil {
		t.Fatal(err)
	}
	// docs leads review's team; docs is under lead? make the chain docs -> lead -> outside, then add outside to docs's group.
	if _, err := b.SetLead(ctx, docs.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.AddToGroup(ctx, Sender{}, review.Name, []string{outside.ID}); err == nil {
		t.Fatal("a cycle was allowed")
	}
	if isMember(t, b, review.Name, outside.ID) {
		t.Fatal("the chat was added although the step was refused")
	}
}

func TestAddingToAnOrdinaryGroupIsUnchanged(t *testing.T) {
	ctx := context.Background()
	b, review, _, _, _, outside := teamBus(t)
	if err := b.AddToGroup(ctx, Sender{}, review.Name, []string{outside.ID}); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := teamLeadOf(b.db, outside.ID); got != "" {
		t.Fatalf("an ordinary group made a team member: %q", got)
	}
	if !isMember(t, b, review.Name, outside.ID) {
		t.Fatal("the chat is not in the group")
	}
}
