package bus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The runner uses yesterday's real bus reader/writer between these phases.
// The additional row is invented metadata on the runner's existing reply.
func TestDecisionOriginPreviousBuildTrial(t *testing.T) {
	root := os.Getenv("BUS_SCAN_TRIAL_ROOT")
	if root == "" {
		t.Skip("cross-version fixture only")
	}
	TestBusScanMigrationTrial(t)
	b, err := OpenAt(filepath.Join(root, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if os.Getenv("BUS_SCAN_TRIAL_PHASE") == "migrate" {
		origin := AnswerOrigin{Version: 1, RequestID: "mention:1", MessageID: 1, Kind: "mention", Questions: []AnsweredQuestion{{Number: 0, Text: "Old fixture.", Answer: "New fixture."}}}
		encoded, err := json.Marshal(origin)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.db.Exec(`INSERT INTO decision_answer_origins(message_id,origin) VALUES(2,?)`, string(encoded)); err != nil {
			t.Fatal(err)
		}
	} else {
		m, err := b.Message(context.Background(), 2)
		if err != nil || m.AnswerOrigin == nil || m.AnswerOrigin.RequestID != "mention:1" || m.AnswerOrigin.Questions[0].Answer != "New fixture." {
			t.Fatalf("origin after yesterday's write: %+v %v", m, err)
		}
	}
}
