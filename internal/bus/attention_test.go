package bus

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type attentionFixture struct {
	b       *Bus
	group   Room
	members []Session
	now     time.Time
}

func newAttentionFixture(t *testing.T) *attentionFixture {
	t.Helper()
	b, root := open(t)
	f := &attentionFixture{b: b, now: time.Date(2030, 1, 1, 10, 0, 0, 0, time.UTC)}
	b.now = func() time.Time { return f.now }
	ws := workspace(t, root, "attention")
	ids := []string{}
	for _, name := range []string{"Alpha", "Beta", "Gamma", "Delta"} {
		s, err := b.RegisterSession(context.Background(), WindowSessionPrefix+"attention-"+name, ws, name)
		if err != nil {
			t.Fatal(err)
		}
		f.members = append(f.members, s)
		ids = append(ids, s.ID)
	}
	var err error
	f.group, err = b.CreateGroup(context.Background(), Sender{}, "Attention", ids)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *attentionFixture) person(t *testing.T, mentions ...string) Message {
	t.Helper()
	m, err := f.b.SendMentioning(context.Background(), Sender{}, "#"+f.group.Name, "A person's question", nil, 0, mentions)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func (f *attentionFixture) preview(t *testing.T) GroupReach {
	t.Helper()
	r, err := f.b.GroupReach(context.Background(), f.group.Name, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func reachIDs(reach []ReachMember) string {
	ids := []string{}
	for _, m := range reach {
		ids = append(ids, m.ID)
	}
	return strings.Join(ids, ",")
}

func TestGroupAttentionMentionsBoundsAndExpiry(t *testing.T) {
	f := newAttentionFixture(t)
	ctx := context.Background()
	if r := f.preview(t); r.Reason != "everyone" || !r.Everyone || len(r.Reach) != 4 || len(r.Engaged) != 0 {
		t.Fatalf("empty reach %+v", r)
	}
	broadcast := f.person(t, Everyone)
	if broadcast.Reach.Reason != "explicit" || len(f.preview(t).Engaged) != 0 {
		t.Fatal("broadcast engaged individuals")
	}
	f.person(t, f.members[0].ID, f.members[1].ID)
	if err := f.b.SetGroupAttentionSettings(ctx, 2, 30); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Minute)
	f.person(t, Everyone, f.members[2].ID)
	r := f.preview(t)
	if r.Reason != "engaged" || reachIDs(r.Reach) != f.members[2].ID+","+f.members[0].ID {
		t.Fatalf("bound/order %+v", r)
	}
	if r.Expires != "2030-01-01T10:30:00Z" {
		t.Fatalf("nearest expiry %s", r.Expires)
	}
	f.now = f.now.Add(28 * time.Minute)
	follow := f.person(t)
	if follow.Reach.Reason != "engaged" {
		t.Fatalf("follow %+v", follow.Reach)
	}
	f.now = f.now.Add(2 * time.Minute)
	if r := f.preview(t); r.Reason != "everyone" || len(r.Engaged) != 0 {
		t.Fatalf("inferred send renewed expiry: %+v", r)
	}
	if err := f.b.SetGroupAttentionSettings(ctx, 3, 60); err != nil {
		t.Fatal(err)
	}
	if r := f.preview(t); len(r.Engaged) != 3 || r.Settings.LimitMax != 10 {
		t.Fatalf("retroactive settings %+v", r)
	}
	for _, limits := range [][2]int{{0, 30}, {11, 30}, {3, 0}, {3, 1441}} {
		if err := f.b.SetGroupAttentionSettings(ctx, limits[0], limits[1]); err == nil {
			t.Fatalf("accepted settings %v", limits)
		}
	}
}

func TestGroupAttentionRepliesAreAddressedAndGroupLocal(t *testing.T) {
	f := newAttentionFixture(t)
	ctx := context.Background()
	person := f.person(t)
	post := func(member int, group string, reply int64, mentions ...string) {
		t.Helper()
		_, err := f.b.SendMentioning(ctx, Sender{Session: &f.members[member]}, "#"+group, "An agent's post naming paul", nil, reply, mentions)
		if err != nil {
			t.Fatal(err)
		}
	}
	post(0, f.group.Name, 0)
	if len(f.preview(t).Engaged) != 0 {
		t.Fatal("ordinary chatter engaged")
	}
	post(0, f.group.Name, person.ID)
	f.now = f.now.Add(time.Minute)
	post(1, f.group.Name, 0, Human)
	if r := f.preview(t); reachIDs(r.Reach) != f.members[1].ID+","+f.members[0].ID {
		t.Fatalf("addressed replies %+v", r)
	}
	other, err := f.b.CreateGroup(ctx, Sender{}, "Other", []string{f.members[0].ID, f.members[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	post(0, other.Name, person.ID)
	if r, err := f.b.GroupReach(ctx, other.Name, nil); err != nil || len(r.Engaged) != 0 {
		t.Fatalf("cross-group reply engaged %+v %v", r, err)
	}
	before := f.preview(t)
	f.now = f.now.Add(time.Minute)
	post(2, f.group.Name, person.ID)
	if r := f.preview(t); r.Reach[0].ID != f.members[2].ID || r.Engaged[1] != before.Engaged[0] {
		t.Fatalf("reply order/expiry %+v", r)
	}
}

func TestGroupAttentionClosedRemovedAndRetiredMembers(t *testing.T) {
	f := newAttentionFixture(t)
	ctx := context.Background()
	f.person(t, f.members[0].ID, f.members[1].ID)
	if err := f.b.EndSession(ctx, f.members[0].ID); err != nil {
		t.Fatal(err)
	}
	if r := f.preview(t); r.Reach[0].ID != f.members[0].ID {
		t.Fatalf("closed member dropped %+v", r)
	}
	if err := f.b.RemoveFromGroup(ctx, Sender{}, f.group.Name, f.members[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.b.AddToGroup(ctx, Sender{}, f.group.Name, []string{f.members[0].Title}); err != nil {
		t.Fatal(err)
	}
	if r := f.preview(t); reachIDs(r.Reach) != f.members[1].ID {
		t.Fatalf("rejoin revived old engagement %+v", r)
	}
	if _, err := f.b.SetLead(ctx, f.members[1].ID, f.members[2].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.b.SetRetired(ctx, f.members[1].ID, true); err != nil {
		t.Fatal(err)
	}
	if r := f.preview(t); len(r.Engaged) != 0 || len(r.Reach) != 3 {
		t.Fatalf("retired member retained %+v", r)
	}
}

func TestGroupAttentionPersistsRoutingAndReconstructsAfterRestart(t *testing.T) {
	f := newAttentionFixture(t)
	ctx := context.Background()
	sent := f.person(t, f.members[0].ID)
	f.now = f.now.Add(29 * time.Minute)
	before := f.preview(t)
	if before.Reason != "engaged" {
		t.Fatal(before)
	}
	path := filepath.Join(f.b.dir, "bus.db")
	if err := f.b.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	reopened.now = func() time.Time { return f.now }
	f.b = reopened
	if after := f.preview(t); !reflect.DeepEqual(before, after) {
		t.Fatalf("restart changed state\nbefore %+v\nafter %+v", before, after)
	}
	f.now = f.now.Add(time.Minute)
	actual := f.person(t)
	if actual.Reach.Reason != "everyone" || !actual.Reach.Everyone || len(actual.Reach.Reach) != 4 {
		t.Fatalf("send did not reevaluate expiry %+v", actual)
	}
	if err := f.b.RemoveFromGroup(ctx, Sender{}, f.group.Name, f.members[0].ID); err != nil {
		t.Fatal(err)
	}
	stored, err := f.b.Message(ctx, sent.ID)
	if err != nil || !reflect.DeepEqual(stored.Reach, sent.Reach) {
		t.Fatalf("historical reach changed %+v %v", stored, err)
	}
	var reason, mentions string
	if err := f.b.db.QueryRow(`SELECT attention_reason, mentions FROM messages WHERE id = ?`, actual.ID).Scan(&reason, &mentions); err != nil {
		t.Fatal(err)
	}
	if reason != "everyone" || !hasMention(mentions, WakesAll) {
		t.Fatalf("stored routing %q %q", reason, mentions)
	}
}

func TestGroupAttentionAgentRulesAndErrorsStayStrict(t *testing.T) {
	f := newAttentionFixture(t)
	ctx := context.Background()
	f.person(t, f.members[0].ID)
	m, err := f.b.Send(ctx, Sender{Session: &f.members[1]}, "#"+f.group.Name, "An informational agent post", nil, 0)
	if err != nil || m.mentions != "" || m.Reach != nil {
		t.Fatalf("agent routing changed %+v %v", m, err)
	}
	before, _ := f.b.ChangeCounter(ctx)
	if _, err := f.b.GroupReach(ctx, f.group.Name, []string{"not a member"}); err == nil {
		t.Fatal("invalid preview accepted")
	}
	if after, _ := f.b.ChangeCounter(ctx); before != after {
		t.Fatal("preview changed history")
	}
	if _, err := f.b.db.Exec(`DELETE FROM group_attention_policy`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.b.Send(ctx, Sender{}, "#"+f.group.Name, "Must not silently send", nil, 0); err == nil {
		t.Fatal("routing failure sent a message")
	}
	if after, _ := f.b.ChangeCounter(ctx); before != after {
		t.Fatal("failed send committed")
	}
}

func TestGroupAttentionTeamLeadFallbackAndEngagedFollowup(t *testing.T) {
	f := newAttentionFixture(t)
	ctx := context.Background()
	if _, _, err := f.b.MakeGroupTeam(ctx, f.group.Name, f.members[0].ID); err != nil {
		t.Fatal(err)
	}
	if r := f.preview(t); r.Reason != "lead" || reachIDs(r.Reach) != f.members[0].ID {
		t.Fatalf("team fallback %+v", r)
	}
	f.person(t, f.members[1].ID)
	if r := f.person(t).Reach; r.Reason != "engaged" || reachIDs(r.Reach) != f.members[1].ID {
		t.Fatalf("team followup %+v", r)
	}
	// A worker's explicit address to the person survives the lead relay.
	f.now = f.now.Add(time.Minute)
	if _, err := f.b.SendMentioning(ctx, Sender{Session: &f.members[2]}, "#"+f.group.Name, "For the person", nil, 0, []string{Human}); err != nil {
		t.Fatal(err)
	}
	if r := f.preview(t); r.Reach[0].ID != f.members[2].ID {
		t.Fatalf("relay did not engage sender %+v", r)
	}
}

func TestGroupAttentionIndexedWindowIgnoresOldHistory(t *testing.T) {
	f := newAttentionFixture(t)
	old := f.now.Add(-24 * time.Hour).Format(time.RFC3339Nano)
	tx, err := f.b.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 4000; i++ {
		if _, err := tx.Exec(`INSERT INTO messages (created_at, kind, from_name, to_label, room, text, mentions)
			VALUES (?, 'text', ?, ?, ?, 'Old engagement', ?)`, old, Human, "#"+f.group.Name, f.group.Name, ","+f.members[3].ID+","); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	f.person(t, f.members[0].ID)
	if r := f.preview(t); r.Reason != "engaged" || reachIDs(r.Reach) != f.members[0].ID {
		t.Fatalf("recent preview: %+v", r)
	}
	rows, err := f.b.db.Query("EXPLAIN QUERY PLAN "+engagementHistoryQuery, f.group.Name, old, Human, "%,"+Human+",%", Human)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, "messages_group_attention_time") {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("engagement query did not use its timestamp expression index")
	}
}

func TestGroupAttentionIndexedTimeHandlesClockRollbackAndLegacyFractions(t *testing.T) {
	f := newAttentionFixture(t)
	whole := f.person(t, f.members[0].ID)
	f.now = f.now.Add(500 * time.Millisecond)
	f.person(t, f.members[1].ID)
	f.now = f.now.Add(250 * time.Millisecond)
	legacy := f.person(t, f.members[2].ID)
	if _, err := f.b.db.Exec(`UPDATE messages SET created_at = ?, attention_reason = '' WHERE id = ?`, "2030-01-01T10:00:00.750000000+00:00", legacy.ID); err != nil {
		t.Fatal(err)
	}
	var wholeStamp string
	if err := f.b.db.QueryRow(`SELECT created_at FROM messages WHERE id = ?`, whole.ID).Scan(&wholeStamp); err != nil {
		t.Fatal(err)
	}
	if wholeStamp != "2030-01-01T10:00:00Z" {
		t.Fatalf("whole-second fixture: %q", wholeStamp)
	}
	// A higher ID can have an earlier wall-clock stamp.
	f.now = time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	f.person(t, f.members[3].ID)
	f.now = time.Date(2030, 1, 1, 10, 30, 0, 250000000, time.UTC)
	if r := f.preview(t); reachIDs(r.Reach) != f.members[2].ID+","+f.members[1].ID {
		t.Fatalf("rollback and legacy preview: %+v", r)
	}
	f.now = f.now.Add(250*time.Millisecond - time.Nanosecond)
	if r := f.preview(t); len(r.Engaged) != 2 {
		t.Fatalf("fractional engagement expired early: %+v", r)
	}
	f.now = f.now.Add(time.Nanosecond)
	if r := f.preview(t); reachIDs(r.Reach) != f.members[2].ID {
		t.Fatalf("exact fractional expiry: %+v", r)
	}
}
