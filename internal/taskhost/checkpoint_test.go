package taskhost

import (
	"context"
	"encoding/json"
	"testing"
)

func TestReceiptRecoveryResultSurvivesUnknownOutcomeAndRestart(t *testing.T) {
	w, options, now, _ := fixture(t)
	w.options.Backend = backendFunc(func(_ context.Context, _ Scope, _ Operation, capture Capture) Outcome {
		if err := capture(Progress{Result: json.RawMessage(`{"original":"initial","applied":false}`)}); err != nil {
			t.Error(err)
		}
		return Outcome{State: "unknown", Code: "transport_failed"}
	})
	p := plan(*now)
	h := begin(t, w, p)
	op := Operation{ID: "checkpoint", Kind: "TakeTurn", Target: "chat"}
	if _, err := w.Start(context.Background(), h, op); err != nil {
		t.Fatal(err)
	}
	r := await(t, w, p.Scope, op.ID)
	if r.State != "unknown" || string(r.Result) != `{"original":"initial","applied":false}` {
		t.Fatalf("lost recovery checkpoint: %+v", r)
	}
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(*options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restarted.Shutdown(context.Background()) })
	read, err := restarted.Read(context.Background(), p.Scope, op.ID)
	if err != nil || string(read.Result) != string(r.Result) || read.State != "unknown" {
		t.Fatalf("restart lost recovery checkpoint: %+v %v", read, err)
	}
}
