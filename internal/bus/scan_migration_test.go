package bus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestBusScanMigrationTrial is used by the cross-version runner on invented
// state. Ordinary package runs do not create a persistent fixture.
func TestBusScanMigrationTrial(t *testing.T) {
	root := os.Getenv("BUS_SCAN_TRIAL_ROOT")
	if root == "" {
		t.Skip("cross-version fixture only")
	}
	b, err := OpenAt(filepath.Join(root, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ctx := context.Background()
	var count int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name IN ('deliveries_pending','deliveries_human_cursor','deliveries_human_unread','deliveries_human_decisions')`).Scan(&count); err != nil || count != 4 {
		t.Fatalf("migration indexes: %d %v", count, err)
	}
	if os.Getenv("BUS_SCAN_TRIAL_PHASE") == "migrate" {
		m, err := b.Message(ctx, 1)
		if err != nil || m.Text != "Old fixture." {
			t.Fatalf("old message: %+v %v", m, err)
		}
		m, err = b.Send(ctx, Sender{}, "", "New fixture.", nil, 1)
		if err != nil || m.ID != 2 {
			t.Fatalf("new reply: %+v %v", m, err)
		}
	} else {
		for i, text := range []string{"Old fixture.", "New fixture.", "Old after migration."} {
			m, err := b.Message(ctx, int64(i+1))
			if err != nil || m.Text != text {
				t.Fatalf("message %d: %+v %v", i+1, m, err)
			}
		}
		var delivered int
		if err := b.db.QueryRow(`SELECT COUNT(*) FROM deliveries WHERE message_id=2 AND delivered_at IS NOT NULL`).Scan(&delivered); err != nil || delivered != 1 {
			t.Fatalf("old delivery: %d %v", delivered, err)
		}
	}
	var integrity string
	if err := b.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity: %s %v", integrity, err)
	}
}
