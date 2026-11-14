//go:build bus_growth

package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type growthTiming struct {
	FirstMS  float64 `json:"firstMs"`
	MedianMS float64 `json:"medianMs"`
	P95MS    float64 `json:"p95Ms"`
	Rows     int     `json:"rows"`
}
type growthReceipt struct {
	Factor           int                     `json:"factor"`
	Messages         int                     `json:"messages"`
	Deliveries       int                     `json:"deliveries"`
	DatabaseBytes    int64                   `json:"databaseBytes"`
	SeedPeakWALBytes int64                   `json:"seedPeakWalBytes"`
	PinnedWALBytes   int64                   `json:"pinnedWalBytes"`
	UnpinnedWALBytes int64                   `json:"unpinnedWalBytes"`
	SeedSeconds      float64                 `json:"seedSeconds"`
	Queries          map[string]growthTiming `json:"queries"`
	Plans            map[string][]string     `json:"plans"`
}

func growthBytes(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// TestSyntheticBusGrowth is opt-in because its largest invented database is
// several GB. It never opens Home or copies any existing state. Only aggregate
// receipts leave t.TempDir; Go removes the fixture databases after the run.
func TestSyntheticBusGrowth(t *testing.T) {
	factors := []int{1, 10, 100}
	if raw := os.Getenv("BUS_GROWTH_FACTORS"); raw != "" {
		factors = nil
		for _, part := range strings.Split(raw, ",") {
			n, err := strconv.Atoi(part)
			if err != nil || (n != 1 && n != 10 && n != 100) {
				t.Fatal("factors must be 1, 10 or 100")
			}
			factors = append(factors, n)
		}
	}
	var receipts []growthReceipt
	for _, factor := range factors {
		receipts = append(receipts, measureGrowth(t, factor))
		encoded, _ := json.Marshal(receipts[len(receipts)-1])
		t.Log(string(encoded))
		if path := os.Getenv("BUS_GROWTH_RECEIPT"); path != "" {
			encoded, err := json.MarshalIndent(receipts, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(encoded, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func measureGrowth(t *testing.T, factor int) growthReceipt {
	t.Helper()
	ctx := context.Background()
	n := 7825 * factor
	path := filepath.Join(t.TempDir(), "invented-bus.db")
	b, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	b.db.SetMaxOpenConns(1)
	r := growthReceipt{Factor: factor, Messages: n, Queries: map[string]growthTiming{}, Plans: map[string][]string{}}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := b.db.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	stamp := "2026-11-14T00:00:00Z"
	exec(`INSERT INTO workspaces(key,path,name) VALUES('fixture','C:/invented/fixture','Fixture')`)
	exec(`WITH RECURSIVE seq(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM seq WHERE i<148)
 INSERT INTO sessions(id,name,workspace,title,started_at,last_seen) SELECT printf('chat-%03d',i),printf('agent-%03d',i),'fixture',printf('Fixture %03d',i),?,? FROM seq`, stamp, stamp)
	exec(`WITH RECURSIVE seq(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM seq WHERE i<20)
 INSERT INTO rooms(name,created_at,title) SELECT printf('group-%02d',i),?,printf('Group %02d',i) FROM seq`, stamp)
	exec(`INSERT INTO room_members SELECT r.name,'session',s.id FROM rooms r CROSS JOIN sessions s`)
	start := time.Now()
	text := strings.Repeat("Invented benchmark text. ", 342)[:8192]
	for low := 1; low <= n; low += 5000 {
		high := min(n, low+4999)
		tx, err := b.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		queries := []struct {
			q    string
			args []any
		}{
			{`WITH RECURSIVE seq(i) AS (VALUES(?) UNION ALL SELECT i+1 FROM seq WHERE i<?)
 INSERT INTO messages(id,created_at,kind,from_session,from_name,from_workspace,to_label,room,reply_to,text,mentions)
 SELECT i,strftime('%Y-%m-%dT%H:%M:%SZ','2026-11-14',printf('-%d seconds',?-i)),
 CASE WHEN i%10=0 THEN 'host_result' ELSE 'text' END,
 CASE WHEN i%23=0 THEN '' ELSE printf('chat-%03d',i%148+1) END,
 CASE WHEN i%23=0 THEN 'paul' ELSE 'Fixture agent' END,'fixture','Fixture group',
 CASE WHEN i%5=0 THEN '' ELSE printf('group-%02d',i%20+1) END,
 CASE WHEN i%19=0 THEN i-1 ELSE 0 END,printf('%09d ',i)||?,',*,paul,' FROM seq`, []any{low, high, n, text}},
			{`INSERT INTO deliveries(message_id,recipient_kind,recipient,delivered_session,delivered_at)
 SELECT m.id,'session',printf('chat-%03d',(m.id+x.i)%148+1),printf('chat-%03d',(m.id+x.i)%148+1),?
 FROM messages m CROSS JOIN (SELECT 1 i UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4) x WHERE m.id BETWEEN ? AND ?`, []any{stamp, low, high}},
			{`INSERT INTO deliveries(message_id,recipient_kind,recipient,delivered_at,inbox_dismissed_at)
 SELECT id,'human','paul',?,? FROM messages WHERE id BETWEEN ? AND ? AND id*14/100>(id-1)*14/100`, []any{stamp, stamp, low, high}},
			{`INSERT INTO host_results SELECT 'fixture-result-'||id,'chat-001',id FROM messages WHERE id BETWEEN ? AND ? AND kind='host_result'`, []any{low, high}},
			{`INSERT INTO attachments(message_id,idx,path,rel,sha256,size) SELECT id,1,'C:/invented/missing.txt','missing.txt',?,65536 FROM messages WHERE id BETWEEN ? AND ? AND id%20=0`, []any{strings.Repeat("0", 64), low, high}},
			{`INSERT INTO steering_submissions(session,payload,state,created_at) SELECT 'chat-001',?,'delivered',? FROM messages WHERE id BETWEEN ? AND ? AND id%10=1`, []any{`{"text":"` + strings.Repeat("Invented steering. ", 56) + `"}`, stamp, low, high}},
		}
		for _, query := range queries {
			if _, err := tx.ExecContext(ctx, query.q, query.args...); err != nil {
				tx.Rollback()
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		r.SeedPeakWALBytes = max(r.SeedPeakWALBytes, growthBytes(path+"-wal"))
	}
	exec(`UPDATE messages SET room='',kind='text',from_session=printf('chat-%03d',1+(id%4)),from_name='Fixture agent' WHERE id>?`, n-5)
	exec(`INSERT INTO deliveries(message_id,recipient_kind,recipient) SELECT id,'session','chat-001' FROM messages WHERE id>?`, n-5)
	exec(`INSERT INTO deliveries(message_id,recipient_kind,recipient) SELECT id,'human','paul' FROM messages m WHERE id>? AND NOT EXISTS(SELECT 1 FROM deliveries d WHERE d.message_id=m.id AND d.recipient_kind='human')`, n-5)
	exec(`UPDATE deliveries SET delivered_at=NULL,inbox_dismissed_at=NULL WHERE recipient_kind='human' AND message_id>?`, n-5)
	exec(`WITH RECURSIVE seq(i) AS (VALUES(?) UNION ALL SELECT i+1 FROM seq WHERE i<?)
 INSERT INTO person_requests(message_id,asker,operation_id,fingerprint,revision,closed,questions)
 SELECT i,'chat-002','fixture-op-'||i,'fixture',1,0,'[{"number":1,"text":"Choose fixture work.","recommendation":"Proceed.","choices":[],"answered":false,"answer":""}]' FROM seq`, n-11, n-5)
	// This watermark is set once per home, so it must stay fixed as that
	// home's later history grows. Advancing it would hide the growing range.
	exec(`UPDATE decision_start SET after_message=?`, 7825-1000)
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM deliveries`).Scan(&r.Deliveries); err != nil {
		t.Fatal(err)
	}
	exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	r.DatabaseBytes = growthBytes(path)
	r.SeedSeconds = time.Since(start).Seconds()
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	b.db.SetMaxOpenConns(1)
	measure := func(name string, prepare func(), query func() (int, error)) {
		t.Helper()
		var samples []float64
		var rows int
		for i := 0; i < 22; i++ {
			if prepare != nil {
				prepare()
			}
			start := time.Now()
			count, err := query()
			elapsed := float64(time.Since(start).Nanoseconds()) / 1e6
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if i == 0 {
				r.Queries[name] = growthTiming{FirstMS: elapsed, Rows: count}
				rows = count
			} else {
				if count != rows {
					t.Fatalf("%s changed row count %d -> %d", name, rows, count)
				}
				samples = append(samples, elapsed)
			}
		}
		sort.Float64s(samples)
		value := r.Queries[name]
		value.MedianMS = samples[10]
		value.P95MS = samples[19]
		r.Queries[name] = value
	}
	measure("wake", nil, func() (int, error) { v, e := b.Waking(ctx); return len(v), e })
	measure("doorbell", nil, func() (int, error) { v, _, e := b.Doorbell(ctx, "chat-001"); return v, e })
	measure("decisions", nil, func() (int, error) {
		v, e := b.DecisionEntries(ctx)
		if e == nil && v.Count < 7 {
			e = fmt.Errorf("lost structured requests")
		}
		return v.Count, e
	})
	exec(`UPDATE decision_start SET after_message=?`, n-1000)
	measure("decisionsFreshBoundary", nil, func() (int, error) {
		v, e := b.DecisionEntries(ctx)
		return v.Count, e
	})
	exec(`UPDATE decision_start SET after_message=?`, 7825-1000)
	measure("group100", nil, func() (int, error) { v, e := b.RoomMessages(ctx, "group-02", 100); return len(v), e })
	measure("attentionImport", nil, func() (int, error) { v, e := b.Mentions(ctx, 0); return len(v), e })
	measure("attentionIncremental", nil, func() (int, error) { v, e := b.AttentionMentions(ctx, int64(n-100)); return len(v), e })
	measure("attentionAnswered5", nil, func() (int, error) {
		v, e := b.MentionsAnswered(ctx, []int64{int64(n), int64(n - 1), int64(n - 2), int64(n - 3), int64(n - 4)})
		return len(v), e
	})
	measure("delivery5", func() {
		exec(`UPDATE deliveries SET delivered_at=NULL WHERE recipient='chat-001' AND message_id>?`, n-5)
	}, func() (int, error) { v, _, e := b.Deliver(ctx, "chat-001", "tool"); return len(v), e })
	measure("deliveryEmpty", nil, func() (int, error) { v, _, e := b.Deliver(ctx, "chat-001", "tool"); return len(v), e })
	for name, query := range map[string]string{
		"globalPending":   `SELECT d.id FROM deliveries d WHERE d.delivered_at IS NULL AND d.cancelled_at IS NULL`,
		"attentionImport": `SELECT m.id FROM deliveries d JOIN messages m ON m.id=d.message_id WHERE d.recipient_kind='human' AND d.cancelled_at IS NULL AND d.delivered_at IS NULL AND m.from_session<>'' AND m.id>0 AND m.kind NOT IN ('host_result','room_member','chat_question') ORDER BY m.id`,
		"decisions":       `SELECT m.id` + openDecisionSQL + ` ORDER BY m.id DESC`,
	} {
		rows, err := b.db.Query(`EXPLAIN QUERY PLAN ` + query)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			r.Plans[name] = append(r.Plans[name], detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	// A separate reader pins an old snapshot while ordinary-size transactions
	// rewrite 100 message bodies. No product PRAGMA or schema is changed.
	exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	reader, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	var top int
	if err := tx.QueryRow(`SELECT MAX(id) FROM messages`).Scan(&top); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		exec(`UPDATE messages SET text=substr(text,1,8192)||? WHERE id>?`, fmt.Sprintf("%09d", i), n-100)
	}
	exec(`PRAGMA wal_checkpoint(PASSIVE)`)
	r.PinnedWALBytes = growthBytes(path + "-wal")
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	r.UnpinnedWALBytes = growthBytes(path + "-wal")
	return r
}
