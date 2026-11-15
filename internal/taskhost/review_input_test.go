package taskhost

import (
	"context"
	"testing"
)

func TestReviewedInputCannotChangeBeforeFirstDispatch(t *testing.T) {
	w, _, now, count := fixture(t)
	p := plan(*now)
	p.Permits[0].Input = []byte(`{"text":"reviewed"}`)
	h := begin(t, w, p)
	if _, err := w.Start(context.Background(), h, Operation{ID: "changed", Kind: "TakeTurn", Target: "chat", Input: []byte(`{"text":"different"}`)}); err == nil {
		t.Fatal("changed input was admitted")
	}
	if *count != 0 {
		t.Fatal("changed input reached backend")
	}
	if _, err := w.Start(context.Background(), h, Operation{ID: "reviewed", Kind: "TakeTurn", Target: "chat", Input: p.Permits[0].Input}); err != nil {
		t.Fatal(err)
	}
	if r := await(t, w, p.Scope, "reviewed"); r.State != "succeeded" {
		t.Fatal(r)
	}
}
