package bus

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestRoomMessageContextIsBoundedOrderedAndReadOnly(t *testing.T) {
	b, _ := open(t)
	ctx := context.Background()
	_, err := b.db.Exec(`WITH RECURSIVE seq(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM seq WHERE i<1000)
 INSERT INTO messages(id,created_at,kind,from_name,to_label,room,text) SELECT i,'2026-11-14T00:00:00Z','text','Fixture','Group','group-a','Invented message' FROM seq`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.db.Exec(`INSERT INTO deliveries(message_id,recipient_kind,recipient) VALUES(8,'human','paul')`); err != nil {
		t.Fatal(err)
	}
	before, err := b.ChangeCounter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		target int64
		want   []int64
	}{{8, []int64{3, 4, 5, 6, 7}}, {3, []int64{1, 2}}, {1, []int64{}}} {
		got, err := b.RoomMessageContext(ctx, "group-a", tc.target)
		if err != nil {
			t.Fatal(err)
		}
		ids := []int64{}
		for _, m := range got {
			ids = append(ids, m.ID)
		}
		if !reflect.DeepEqual(ids, tc.want) {
			t.Fatalf("target %d: %v", tc.target, ids)
		}
	}
	for _, tc := range []struct {
		group string
		id    int64
	}{{"group-b", 8}, {"", 8}, {"group-a", 2000}, {"group-a", 0}} {
		if _, err := b.RoomMessageContext(ctx, tc.group, tc.id); err == nil {
			t.Fatalf("accepted invalid target %+v", tc)
		}
	}
	after, err := b.ChangeCounter(ctx)
	if err != nil || after != before {
		t.Fatal("context changed the bus", err)
	}
	var unread int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM deliveries WHERE delivered_at IS NULL`).Scan(&unread); err != nil || unread != 1 {
		t.Fatal("context marked a message read", err)
	}
	rows, err := b.db.Query(`EXPLAIN QUERY PLAN SELECT id FROM messages WHERE room=? AND id<? ORDER BY id DESC LIMIT 5`, "group-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	indexed := false
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		t.Log(detail)
		if strings.Contains(detail, "messages_group_history") && (strings.Contains(detail, "rowid<?") || strings.Contains(detail, "id<?")) {
			indexed = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !indexed {
		t.Fatal("preceding context did not use the group-ID range index")
	}
}
