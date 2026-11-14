package bus

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestDecisionFirstStartAndGroupConversation(t *testing.T) {
	b, _, request := personRequestFixture(t)
	ctx := context.Background()
	add := func(group string, person bool) int64 {
		t.Helper()
		from, name := "fixture-agent", "Fixture"
		if person {
			from, name = "", Human
		}
		r, err := b.db.Exec(`INSERT INTO messages(created_at,kind,from_session,from_name,to_label,room,text) VALUES('2026-11-14T00:00:00Z','text',?,?,?,?,'Fixture message')`, from, name, "#"+group, group)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := r.LastInsertId()
		if !person {
			if _, err = b.db.Exec(`INSERT INTO deliveries(message_id,recipient_kind,recipient) VALUES(?,'human','paul')`, id); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	// Model an existing home before this feature's first open, including an open request.
	old := add("quiet", false)
	if _, err := b.db.Exec(`DELETE FROM decision_start`); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var started string
	var boundary int64
	if err = reopened.db.QueryRow(`SELECT started_at,after_message FROM decision_start`).Scan(&started, &boundary); err != nil {
		t.Fatal(err)
	}
	if boundary != old || started == "" {
		t.Fatalf("first start %q %d", started, boundary)
	}
	check := func(want ...int64) {
		t.Helper()
		page, err := b.Decisions(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := b.DecisionEntries(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != len(want) || entries.Count != len(want) {
			t.Fatalf("list %d entries %d want %d", len(page.Items), entries.Count, len(want))
		}
		for i, id := range want {
			if page.Items[i].MessageID != id || entries.Items[i].MessageID != id {
				t.Fatalf("row %d differs", i)
			}
		}
	}
	check(request.MessageID)
	add("answered", false)
	add("answered", false)
	quiet := add("quiet", false)
	add("answered", true) // This is ordinary group conversation, not a reply.
	check(quiet, request.MessageID)
	fresh := add("answered", false)
	check(fresh, quiet, request.MessageID)
	again, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	var nextStart string
	var nextBoundary int64
	again.db.QueryRow(`SELECT started_at,after_message FROM decision_start`).Scan(&nextStart, &nextBoundary)
	if nextStart != started || nextBoundary != boundary {
		t.Fatal("first-start boundary moved")
	}
	check(fresh, quiet, request.MessageID)
}

func TestDecisionBacklogIndexedSevenThousand(t *testing.T) {
	b, _, request := personRequestFixture(t)
	ctx := context.Background()
	err := b.tx(ctx, func(tx *sql.Tx) error {
		for i := 0; i < 7000; i++ {
			if _, err := tx.Exec(`INSERT INTO messages(created_at,kind,from_session,from_name,to_label,room,text) VALUES('2026-11-14T00:00:00Z','text','fixture','Fixture','#old','old','Old fixture mention')`); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`UPDATE decision_start SET after_message=(SELECT MAX(id) FROM messages)`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	page, err := b.Decisions(ctx, 0)
	elapsed := time.Since(start)
	if err != nil || len(page.Items) != 1 || page.Items[0].MessageID != request.MessageID {
		t.Fatalf("backlog selection %+v %v", page, err)
	}
	start = time.Now()
	entries, err := b.DecisionEntries(ctx)
	entryElapsed := time.Since(start)
	if err != nil || entries.Count != 1 {
		t.Fatalf("backlog entries %+v %v", entries, err)
	}
	t.Logf("7000 old messages: list %s; entries %s", elapsed, entryElapsed)
	rows, err := b.db.Query(`EXPLAIN QUERY PLAN SELECT m.id` + openDecisionSQL)
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
		if detail == "SEARCH messages USING INTEGER PRIMARY KEY (rowid>?)" {
			indexed = true
		}
	}
	if !indexed {
		t.Fatal("candidate range is not indexed")
	}
}

func TestDecisionGroupConversationKeepsStructuredRequest(t *testing.T) {
	b, s, _ := personRequestFixture(t)
	ctx := context.Background()
	group, err := b.CreateGroup(ctx, Sender{}, "Still needs an answer", []string{s.ID})
	if err != nil {
		t.Fatal(err)
	}
	request, err := b.AskPerson(ctx, s, AskPersonInput{OperationID: "group-still-open", Group: "#" + group.Name, Questions: []PersonQuestion{{Number: 1, Text: "Choose a release", Recommendation: "Use the current release"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Send(ctx, Sender{}, "#"+group.Name, "I have read the conversation", nil, 0); err != nil {
		t.Fatal(err)
	}
	// Establish a later feature boundary as well. Neither rule closes structured requests.
	if _, err = b.db.Exec(`UPDATE decision_start SET after_message=(SELECT MAX(id) FROM messages)`); err != nil {
		t.Fatal(err)
	}
	page, err := b.Decisions(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != request.ID {
		t.Fatalf("structured request closed: %+v", page)
	}
	entries, err := b.DecisionEntries(ctx)
	if err != nil || entries.Count != 2 || entries.Items[0].ID != request.ID {
		t.Fatalf("structured entries %+v %v", entries, err)
	}
}
