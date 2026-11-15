package taskhost

import (
	"context"
	"errors"
	"testing"
)

func TestShutdownBudgetCheckpointsUnknownThroughPrivateCleanup(t *testing.T) {
	w, _, now, _ := fixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	w.options.Backend = backendFunc(func(ctx context.Context, s Scope, op Operation, capture Capture) Outcome {
		if err := capture(Progress{Identity: "invented-native", Dispatched: true}); err != nil {
			t.Error(err)
		}
		close(entered)
		<-release
		return Outcome{State: "unknown", Code: "unknown_outcome", Identity: "invented-native"}
	})
	p := plan(*now)
	h := begin(t, w, p)
	if _, err := w.Start(context.Background(), h, Operation{ID: "pending", Kind: "TakeTurn", Target: "chat"}); err != nil {
		t.Fatal(err)
	}
	<-entered
	w.CloseAdmission()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := w.Shutdown(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	r, err := w.Read(context.Background(), p.Scope, "pending")
	if err != nil || r.State != "unknown" || r.Identity != "invented-native" || !r.Dispatched {
		t.Fatalf("private cleanup lost receipt: %+v %v", r, err)
	}
	if _, err := w.ForService(p.Scope.Service).Read(context.Background(), p.Scope, "pending"); err == nil {
		t.Fatal("revoked public binding remained usable")
	}
	close(release)
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
