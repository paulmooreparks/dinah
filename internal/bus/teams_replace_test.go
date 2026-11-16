package bus

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTeamLeadCannotBeAssignedUntilReportsMove(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, parser, outside := teamBus(t)
	group, err := b.SetLead(ctx, docs.ID, lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.CheckTeamLead(lead.ID, outside.ID); err == nil || !strings.Contains(err.Error(), "Reassign its members") {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, lead.ID, outside.ID); err == nil || !strings.Contains(err.Error(), "Reassign its members") {
		t.Fatal(err)
	}
	if err := b.AddToGroup(ctx, Sender{}, review.Name, []string{lead.ID}); err != nil {
		t.Fatal("lead could not join ordinary group", err)
	}
	if _, err := b.SetLead(ctx, parser.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	outsideGroup := mustGroup(t, b, outside.ID)
	if err := b.AddToGroup(ctx, Sender{}, outsideGroup, []string{lead.ID}); err == nil || !strings.Contains(err.Error(), "Reassign its members") {
		t.Fatal("team-group join bypassed lead refusal", err)
	}
	if _, err := b.SetLead(ctx, docs.ID, outside.ID); err != nil {
		t.Fatal(err)
	}
	if old, err := b.TeamGroup(ctx, lead.ID); err != nil || old != "" {
		t.Fatal("empty former team still belongs to its lead", old, err)
	}
	if room, err := b.Room(ctx, group); err != nil || room.Lead != "" {
		t.Fatal("former channel was not retained as an ordinary group", room, err)
	}
	if _, err := b.SetLead(ctx, lead.ID, outside.ID); err != nil {
		t.Fatal("former lead could not be assigned", err)
	}
}

func TestReplaceTeamLeadIsAtomicAndRetainsChannel(t *testing.T) {
	ctx := context.Background()
	b, review, lead, docs, parser, _ := teamBus(t)
	group, err := b.SetLead(ctx, docs.ID, lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, parser.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.SetRetired(ctx, parser.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.Exec(`INSERT INTO team_budgets (lead, provider, unit, allowance) VALUES (?, 'invented', 'tokens', 23)`, lead.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := b.TeamMembers(ctx, lead.ID)
	channel, _ := b.RoomMembers(ctx, group)
	ordinary, _ := b.RoomMembers(ctx, review.Name)
	stamp, _ := b.ChangeCounter(ctx)
	if _, err := b.db.Exec(`CREATE TRIGGER refuse_lead_change BEFORE UPDATE OF lead ON rooms BEGIN SELECT RAISE(ABORT, 'invented late write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := b.ReplaceTeamLead(ctx, group, lead.ID, docs.ID); err == nil {
		t.Fatal("injected write failure succeeded")
	}
	if after, _ := b.TeamMembers(ctx, lead.ID); !reflect.DeepEqual(before, after) {
		t.Fatal("reporting rows escaped rollback", after)
	}
	if owner, _ := b.TeamGroup(ctx, lead.ID); owner != group {
		t.Fatal("channel owner escaped rollback", owner)
	}
	if after, _ := b.ChangeCounter(ctx); after != stamp {
		t.Fatal("failed transaction advanced change counter")
	}
	if _, err := b.db.Exec(`DROP TRIGGER refuse_lead_change`); err != nil {
		t.Fatal(err)
	}
	if err := b.ReplaceTeamLead(ctx, group, lead.ID, docs.ID); err != nil {
		t.Fatal(err)
	}
	if owner, _ := b.TeamGroup(ctx, docs.ID); owner != group {
		t.Fatal("replacement changed channel identity", owner)
	}
	if after, _ := b.RoomMembers(ctx, group); !reflect.DeepEqual(channel, after) {
		t.Fatal("replacement changed channel membership", after)
	}
	if after, _ := b.RoomMembers(ctx, review.Name); !reflect.DeepEqual(ordinary, after) {
		t.Fatal("replacement changed an ordinary group", after)
	}
	if parent, _, _ := b.LeadOf(ctx, docs.ID); parent != "" {
		t.Fatal("new lead still reports to old lead", parent)
	}
	if parent, _, _ := b.LeadOf(ctx, lead.ID); parent != docs.ID {
		t.Fatal("old lead is not a member", parent)
	}
	if parent, retired, _ := b.LeadOf(ctx, parser.ID); parent != docs.ID || !retired {
		t.Fatal("another reporting row changed state", parent, retired)
	}
	var budget int
	if err := b.db.QueryRow(`SELECT allowance FROM team_budgets WHERE lead = ?`, docs.ID).Scan(&budget); err != nil || budget != 23 {
		t.Fatal("team allowance did not follow the team", budget, err)
	}
	if err := b.ReplaceTeamLead(ctx, group, lead.ID, parser.ID); err == nil {
		t.Fatal("stale expected lead accepted")
	}
}

// Historical nested rows must remain readable, but cannot authorize a lead
// replacement. This fixture bypasses new admission only to represent old data.
func legacyAssignLead(t *testing.T, b *Bus, member, lead string) {
	t.Helper()
	if err := b.tx(context.Background(), func(tx *sql.Tx) error { _, err := b.assignLead(tx, member, lead); return err }); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceTeamLeadRefusesNestedLeadAndForeignMember(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, parser, outside := teamBus(t)
	group, err := b.SetLead(ctx, docs.ID, lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	legacyAssignLead(t, b, parser.ID, docs.ID)
	for _, candidate := range []string{docs.ID, outside.ID, lead.ID} {
		if err := b.CheckReplaceTeamLead(group, lead.ID, candidate); err == nil {
			t.Fatal("ineligible lead offered", candidate)
		}
		if err := b.ReplaceTeamLead(ctx, group, lead.ID, candidate); err == nil {
			t.Fatal("ineligible replacement succeeded", candidate)
		}
	}
	if err := b.ReplaceTeamLead(ctx, group, lead.ID, docs.ID); err == nil || !strings.Contains(err.Error(), "resolve its own team") {
		t.Fatal(err)
	}
	if parent, _, _ := b.LeadOf(ctx, parser.ID); parent != docs.ID {
		t.Fatal("nested team was moved", parent)
	}
}

func TestReplaceTeamLeadPublishesOnlyCommittedRelations(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, parser, _ := teamBus(t)
	group, err := b.SetLead(ctx, docs.ID, lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, parser.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	var sequence int
	var schemaName, path string
	if err := b.db.QueryRow(`PRAGMA database_list`).Scan(&sequence, &schemaName, &path); err != nil {
		t.Fatal(err)
	}
	// A second connection uses a deferred read transaction so it can observe
	// committed records while the writer is held at each partial write in WAL.
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&_txlock=deferred&_pragma=busy_timeout(10000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reader := &Bus{db: db}
	before, err := reader.TeamRelations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observations := 0
	if err := b.replaceTeamLead(ctx, group, lead.ID, docs.ID, func() error {
		observed, err := reader.TeamRelations(ctx)
		if err != nil {
			return err
		}
		observations++
		if !reflect.DeepEqual(before, observed) {
			t.Errorf("partial transaction became visible after write %d: %+v", observations, observed)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if observations != 4 {
		t.Fatal("not every write checkpoint was observed", observations)
	}
	after, err := reader.TeamRelations(ctx)
	if err != nil || reflect.DeepEqual(before, after) {
		t.Fatal("committed replacement was not visible", after, err)
	}
	if len(after.Members) != 2 {
		t.Fatal("replacement lost or duplicated members", after)
	}
	for _, row := range after.Members {
		if row.Lead != docs.ID || row.Session == docs.ID {
			t.Fatal("replacement published inconsistent ownership", after)
		}
	}
}

// The case of 16 November 2026: the person deleted a team's only member and
// then could not delete its lead, which was told to reassign members that
// were in the Recycle Bin and in no list.
func TestALeadWhoseMembersAreAllBinnedCanBeBinned(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, _, _ := teamBus(t)
	if _, err := b.SetLead(ctx, docs.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.BinRecipient(ctx, docs.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.BinRecipient(ctx, lead.ID); err != nil {
		t.Fatal("a lead with no member left to reassign was refused", err)
	}
	// Restoring the member alone leaves it in no team, and restoring the lead
	// as well gives the team back.
	if err := b.RestoreRecipient(ctx, docs.ID); err != nil {
		t.Fatal(err)
	}
	if of, _, err := b.LeadOf(ctx, docs.ID); err != nil || of != "" {
		t.Fatalf("the restored member reports to a binned lead %q: %v", of, err)
	}
	if err := b.RestoreRecipient(ctx, lead.ID); err != nil {
		t.Fatal(err)
	}
	if of, _, err := b.LeadOf(ctx, docs.ID); err != nil || of != lead.ID {
		t.Fatalf("restoring both did not give the team back: %q, %v", of, err)
	}
	if err := b.BinRecipient(ctx, lead.ID); err == nil || !strings.Contains(err.Error(), "Reassign its members") {
		t.Fatal("a lead with an available member was binned", err)
	}
}

func TestCurrentLeadCannotBeRemovedOrBinned(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, parser, _ := teamBus(t)
	if _, err := b.SetLead(ctx, docs.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	before, err := b.TeamRelations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, remove := range []func() error{
		func() error { return b.RemoveRecipient(ctx, lead.ID) },
		func() error { return b.BinRecipient(ctx, lead.ID) },
		func() error { return b.BinRecipients(ctx, []string{parser.ID, lead.ID}) },
	} {
		if err := remove(); err == nil || !strings.Contains(err.Error(), "Reassign its members") {
			t.Fatal("lead removal accepted", err)
		}
	}
	after, err := b.TeamRelations(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("refusal changed committed relations", after, err)
	}
	if available, err := b.RecipientAvailable(ctx, parser.ID); err != nil || !available {
		t.Fatal("atomic preflight withdrew another recipient", available, err)
	}
}

func TestTeamMembersOutsideTheirGroupAreDroppedWhenGone(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, parser, _ := teamBus(t)
	group, err := b.SetLead(ctx, docs.ID, lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, parser.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	// Both members are out of the team's group, as leaving it used to leave
	// them. Only the one the caller calls gone loses its membership.
	if _, err := b.db.Exec(`DELETE FROM room_members WHERE room = ? AND member IN (?, ?)`, group, docs.ID, parser.ID); err != nil {
		t.Fatal(err)
	}
	dropped, err := b.DropTeamMembersOutsideTheirGroup(ctx, func(member string) bool { return member == docs.ID })
	if err != nil || dropped != 1 {
		t.Fatalf("dropped %d, %v", dropped, err)
	}
	if of, _, _ := b.LeadOf(ctx, docs.ID); of != "" {
		t.Fatal("the gone member is still in the team")
	}
	if of, _, _ := b.LeadOf(ctx, parser.ID); of != lead.ID {
		t.Fatal("a member that is not gone lost its team")
	}
}

// A chat that leaves its team's group by itself, and one that the person takes
// out of it, are both out of the team.
func TestLeavingOrBeingRemovedFromATeamsGroupLeavesTheTeam(t *testing.T) {
	ctx := context.Background()
	b, _, lead, docs, parser, _ := teamBus(t)
	group, err := b.SetLead(ctx, docs.ID, lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetLead(ctx, parser.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveFromGroup(ctx, Sender{Session: &docs}, group, docs.ID); err != nil {
		t.Fatal(err)
	}
	if of, _, err := b.LeadOf(ctx, docs.ID); err != nil || of != "" {
		t.Fatalf("a chat that left the team's group still reports to %q: %v", of, err)
	}
	if of, _, _ := b.LeadOf(ctx, parser.ID); of != lead.ID {
		t.Fatal("the other member lost its team")
	}
	if err := b.RemoveFromGroup(ctx, Sender{}, group, parser.ID); err != nil {
		t.Fatal(err)
	}
	if of, _, err := b.LeadOf(ctx, parser.ID); err != nil || of != "" {
		t.Fatalf("a chat the person took out of the team's group still reports to %q: %v", of, err)
	}
}
