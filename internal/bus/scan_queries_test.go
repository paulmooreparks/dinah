package bus

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// This is the previous selection predicate, kept only as an independent
// reference for indexed selection. It is never used by the application.
const previousDecisionSQL = ` FROM messages m JOIN (
 SELECT message_id AS id FROM person_requests WHERE closed=0
 UNION SELECT id FROM messages WHERE id>(SELECT after_message FROM decision_start WHERE id=1)
) candidates ON candidates.id=m.id LEFT JOIN person_requests p ON p.message_id=m.id
 WHERE ((p.message_id IS NOT NULL AND p.closed=0) OR
 (p.message_id IS NULL AND m.room<>'' AND m.from_session<>'' AND m.kind='text'
 AND EXISTS (SELECT 1 FROM deliveries d WHERE d.message_id=m.id AND d.recipient_kind='human' AND d.cancelled_at IS NULL AND d.inbox_dismissed_at IS NULL)
 AND NOT EXISTS (SELECT 1 FROM messages later WHERE later.room=m.room AND later.id>m.id AND later.from_session='' AND later.from_name='paul')
 AND NOT EXISTS (SELECT 1 FROM messages reply WHERE reply.reply_to=m.id AND reply.from_session='' AND reply.from_name='paul' AND reply.kind='text')))`

func TestScanCandidatesPreservePreviousRows(t *testing.T) {
	b, root := open(t)
	ctx := context.Background()
	s, err := b.RegisterSession(ctx, "fixture", workspace(t, root, "fixture"), "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := b.db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO rooms(name,created_at,title) VALUES('group-a','2026-11-14T00:00:00Z','Fixture')`)
	exec(`WITH RECURSIVE seq(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM seq WHERE i<2000)
 INSERT INTO messages(id,created_at,kind,from_session,from_name,from_workspace,to_label,room,reply_to,text,mentions)
 SELECT i,'2026-11-14T00:00:00Z',CASE WHEN i%10=0 THEN 'host_result' ELSE 'text' END,
 CASE WHEN i%37=0 THEN '' ELSE ? END,CASE WHEN i%37=0 THEN 'paul' ELSE 'Fixture' END,?,'Fixture',
 CASE WHEN i%11=0 THEN '' ELSE 'group-a' END,CASE WHEN i%37=0 THEN i-1 ELSE 0 END,'Invented.',',*,paul,' FROM seq`, s.ID, s.Workspace)
	exec(`INSERT INTO deliveries(message_id,recipient_kind,recipient,delivered_at,inbox_dismissed_at,cancelled_at)
 SELECT id,'human','paul',CASE WHEN id%3=0 THEN 'read' ELSE NULL END,
 CASE WHEN id%5=0 THEN 'dismissed' ELSE NULL END,CASE WHEN id%7=0 THEN 'cancelled' ELSE NULL END FROM messages`)
	exec(`INSERT INTO deliveries(message_id,recipient_kind,recipient) SELECT id,'human','paul' FROM messages WHERE id%41=0`)
	exec(`INSERT INTO deliveries(message_id,recipient_kind,recipient) SELECT id,'session',? FROM messages WHERE id>1995`, s.ID)
	exec(`INSERT INTO person_requests(message_id,asker,operation_id,fingerprint,revision,closed,questions)
 SELECT id,?,'fixture-'||id,'fixture',1,CASE WHEN id%60=1 THEN 1 ELSE 0 END,'[]' FROM messages WHERE id%20=1`, s.ID)
	exec(`UPDATE decision_start SET after_message=500`)
	ids := func(q string, args ...any) []int64 {
		t.Helper()
		rows, err := b.db.Query(q, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := []int64{}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			out = append(out, id)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	want := ids(`SELECT m.id` + previousDecisionSQL + ` ORDER BY m.id DESC`)
	got := ids(`SELECT m.id` + openDecisionSQL + ` ORDER BY m.id DESC`)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions: got %v want %v", got, want)
	}
	for _, unread := range []bool{false, true} {
		read := ""
		if unread {
			read = " AND d.delivered_at IS NULL"
		}
		for _, after := range []int64{0, 500, 1990, 2000} {
			want := ids(`SELECT m.id FROM deliveries d JOIN messages m ON m.id=d.message_id WHERE d.recipient_kind='human' AND d.cancelled_at IS NULL AND m.from_session<>'' AND m.id>?`+read+` AND m.kind NOT IN `+kindsNotMentions+` ORDER BY m.id`, after)
			got := ids(mentionIDsQuery(unread), after)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("mentions unread=%t after=%d: got %v want %v", unread, after, got, want)
			}
		}
	}
	before, err := b.Waking(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []string{"deliveries_pending", "deliveries_human_cursor", "deliveries_human_unread", "deliveries_human_decisions"} {
		exec(`DROP INDEX ` + index)
	}
	without, err := b.Waking(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(before, func(i, j int) bool { return before[i].Session < before[j].Session })
	sort.Slice(without, func(i, j int) bool { return without[i].Session < without[j].Session })
	if !reflect.DeepEqual(before, without) {
		t.Fatalf("indexed wake rows changed: %v / %v", before, without)
	}
	if err := upgrade(b.db); err != nil {
		t.Fatal(err)
	}
}

func TestScanQueriesUseBoundedIndexes(t *testing.T) {
	b, _ := open(t)
	for _, tc := range []struct {
		name, query, index, rangeText string
		args                          []any
	}{
		{"wake", wakingQuery(), "deliveries_pending", "", nil},
		{"attention cursor", mentionIDsQuery(false), "deliveries_human_cursor", "message_id>?", []any{0}},
		{"attention unread", mentionIDsQuery(true), "deliveries_human_unread", "message_id>?", []any{0}},
		{"decisions", `SELECT m.id` + openDecisionSQL, "deliveries_human_decisions", "message_id>?", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := b.db.Query(`EXPLAIN QUERY PLAN `+tc.query, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			found := false
			var plan []string
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				plan = append(plan, detail)
				if strings.Contains(detail, tc.index) && strings.Contains(detail, tc.rangeText) {
					found = true
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !found {
				t.Fatalf("missing bounded index: %v", plan)
			}
		})
	}
}
