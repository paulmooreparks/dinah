package fileaccess

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file-access.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestAuditPolicyChangesAreAtomicAndVersioned(t *testing.T) {
	s, path := testStore(t)
	if err := s.Change(1, "add", "custom-a", "/synthetic/secret", true); err != nil {
		t.Fatal(err)
	}
	version, custom, _, err := s.Snapshot()
	if err != nil || version != 2 || custom["custom-a"] != "/synthetic/secret" {
		t.Fatalf("snapshot: %d %v %v", version, custom, err)
	}
	if err := s.Change(1, "remove", "custom-a", "", false); err == nil {
		t.Fatal("stale change accepted")
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_audit BEFORE INSERT ON audit BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := s.Change(2, "remove", "custom-a", "", false); err == nil {
		t.Fatal("policy changed without its audit")
	}
	version, custom, _, err = s.Snapshot()
	if err != nil || version != 2 || len(custom) != 1 {
		t.Fatal("failed audit did not roll back policy")
	}
	if _, err := s.db.Exec(`DROP TRIGGER fail_audit`); err != nil {
		t.Fatal(err)
	}
	if err := s.Change(2, "default", "ssh", "", false); err != nil {
		t.Fatal(err)
	}
	if err := s.Change(3, "default", "ssh", "", false); err != nil {
		t.Fatal(err)
	}
	v, _, defaults, err := s.Snapshot()
	if err != nil || v != 3 || defaults["ssh"] {
		t.Fatal("default switch or idempotent change failed")
	}
	other, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	_, saved, switches, err := other.Snapshot()
	if err != nil || saved["custom-a"] == "" || switches["ssh"] {
		t.Fatal("policy did not survive reopen")
	}
	page, err := s.Log(Filter{})
	if err != nil || len(page.Rows) != 2 || page.Rows[0].Outcome != "policy-change" {
		t.Fatalf("audit: %+v %v", page, err)
	}
}

func TestAuditCursorsPreserveNewerAndOtherChats(t *testing.T) {
	s, _ := testStore(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 123, time.UTC)
	s.now = func() time.Time { return now }
	first, err := s.Append(Record{Chat: "a", Outcome: "denied", RuleID: "ssh"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Nanosecond)
	second, err := s.Append(Record{Chat: "a", Outcome: "denied"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Append(Record{Chat: "b", Outcome: "denied"}); err != nil {
		t.Fatal(err)
	}
	if count, err := s.Acknowledge("a", first.ID); err != nil || count != 1 {
		t.Fatalf("ack: %d %v", count, err)
	}
	counts, err := s.RefusedCounts()
	if err != nil || counts["a"] != 1 || counts["b"] != 1 {
		t.Fatal("ack changed newer or other refusals")
	}
	if _, err = s.Acknowledge("a", 99); err == nil {
		t.Fatal("future cursor accepted")
	}
	page, err := s.Log(Filter{Chat: "a", Since: now.Format(time.RFC3339Nano), Limit: 1})
	if err != nil || len(page.Rows) != 1 || page.Rows[0].ID != second.ID {
		t.Fatalf("nanosecond filter: %+v %v", page, err)
	}
	page, err = s.Log(Filter{Limit: 1})
	if err != nil || page.NextBefore != page.Rows[0].ID {
		t.Fatal("missing stable next cursor")
	}
	older, err := s.Log(Filter{Before: page.NextBefore, Limit: 1})
	if err != nil || len(older.Rows) != 1 || older.Rows[0].ID >= page.Rows[0].ID {
		t.Fatal("pagination repeated its selected row")
	}
}

func TestAuditRotationBoundsFilesAndRetainsPolicy(t *testing.T) {
	s, path := testStore(t)
	if err := s.Change(1, "add", "custom-a", "/synthetic", true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 650; i++ {
		if _, err := s.Append(Record{Chat: "a", Outcome: "allowed", Bytes: 1, Reason: strings.Repeat("x", 10000)}); err != nil {
			t.Fatal(err)
		}
	}
	var generations, payloadBytes int
	if err := s.db.QueryRow(`SELECT COUNT(DISTINCT generation),SUM(bytes) FROM audit`).Scan(&generations, &payloadBytes); err != nil {
		t.Fatal(err)
	}
	if generations > AuditGenerations || payloadBytes > AuditGenerations*AuditGenerationBytes {
		t.Fatalf("rotation bounds: %d %d", generations, payloadBytes)
	}
	var maxGeneration int
	if err := s.db.QueryRow(`SELECT MAX(n) FROM (SELECT SUM(bytes) n FROM audit GROUP BY generation)`).Scan(&maxGeneration); err != nil || maxGeneration > AuditGenerationBytes {
		t.Fatal("generation exceeds bound")
	}
	page, err := s.Log(Filter{})
	if err != nil || page.RetentionStart <= 1 || page.HighWater != 651 {
		t.Fatalf("rotation metadata: %+v %v", page, err)
	}
	if _, err = s.Log(Filter{Before: 2}); err == nil {
		t.Fatal("expired cursor silently moved")
	}
	version, custom, _, err := s.Snapshot()
	if err != nil || version != 2 || custom["custom-a"] == "" {
		t.Fatal("rotation removed policy")
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() > MaxDatabaseBytes {
		t.Fatal("database exceeds physical bound")
	}
	if _, err = os.Stat(path + "-journal"); !os.IsNotExist(err) {
		t.Fatalf("rollback journal remains after commit: %v", err)
	}
	if _, err = s.Append(Record{Outcome: "allowed", Reason: strings.Repeat("x", MaxAuditRecordBytes)}); err == nil {
		t.Fatal("oversized metadata accepted")
	}
}

func TestAuditDatabasePathIsNotAURIFragment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "folder with # marker", "audit.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("database path was interpreted as URI syntax", err)
	}
}

func TestAuditDatabaseRefusesNewerSchemaWithoutWriting(t *testing.T) {
	s, path := testStore(t)
	if _, err := s.db.Exec(`PRAGMA user_version=2`); err != nil {
		t.Fatal(err)
	}
	if opened, err := Open(path); err == nil {
		opened.Close()
		t.Fatal("newer schema accepted")
	}
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 2 {
		t.Fatal("refusal rewrote the schema version")
	}
}
