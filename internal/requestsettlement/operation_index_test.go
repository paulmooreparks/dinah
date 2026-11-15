package requestsettlement

import (
	"errors"
	"testing"
)

func TestOperationIdentitySpansSourcesAndSeparatesActorsAndCommands(t *testing.T) {
	state := t.TempDir()
	first := Input{OperationID: "same-operation", ID: "request:one", Revision: 1}
	claim := func(actor, command string, in Input) error {
		release, err := Claim(state, actor, command, in)
		if err == nil {
			err = release()
		}
		return err
	}
	if err := claim("person", "Dismiss", first); err != nil {
		t.Fatal(err)
	}
	if err := claim("person", "Dismiss", first); err != nil {
		t.Fatal("original input could not be reconciled", err)
	}
	for _, id := range []string{"notification:two", "task-q:three"} {
		changed := first
		changed.ID = id
		if err := claim("person", "Dismiss", changed); !errors.Is(err, ErrOperationChanged) {
			t.Fatal("operation was reused for a different source", id, err)
		}
	}
	for _, actor := range []string{ChatActor("owner"), ServiceActor("owner")} {
		if err := claim(actor, "Withdraw", first); err != nil {
			t.Fatal("distinct actor was conflated", actor, err)
		}
	}
	if err := claim("person", "Answer", first); err != nil {
		t.Fatal("distinct command was conflated", err)
	}
}
