package bus

import (
	"context"
	"errors"
	"testing"
)

// A question wakes the chat it is for, closed or not and whatever its hops,
// is never handed over as an ordinary message, and is taken once or
// withdrawn.
func TestAQuestionIsTakenOnceOrWithdrawn(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	ws := workspace(t, root, "studio")
	asker, err := b.RegisterSession(ctx, "window-asker", ws, "Asker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.KnowSession(ctx, "window-asked", ws, "Asked"); err != nil {
		t.Fatal(err)
	}
	// The asker is past the hop limit, which a question does not obey.
	if _, err := b.db.Exec(`UPDATE sessions SET hops = ? WHERE id = ?`, MaxHops+2, asker.ID); err != nil {
		t.Fatal(err)
	}
	q, err := b.Ask(ctx, asker, "window-asked", "Which parser files changed?")
	if err != nil {
		t.Fatal(err)
	}
	if q.Kind != QuestionKind || q.From != "Asker" || q.To != "Asked" {
		t.Fatalf("the question was recorded as %+v", q)
	}
	wakes, err := b.Waking(ctx)
	if err != nil || len(wakes) != 1 || wakes[0].Session != "window-asked" || wakes[0].Newest != q.ID {
		t.Fatalf("the question did not wake the closed chat: %+v %v", wakes, err)
	}
	if n, _, err := b.Doorbell(ctx, "window-asked"); err != nil || n != 0 {
		t.Fatalf("the doorbell counted a question: %d %v", n, err)
	}
	if got, _, err := b.Deliver(ctx, "window-asked", DoorbellTurn); err != nil || len(got) != 0 {
		t.Fatalf("a question was handed over as a message: %+v %v", got, err)
	}
	if waiting, err := b.WaitingQuestions(ctx); err != nil || len(waiting) != 1 || waiting[0] != q.ID {
		t.Fatalf("the question does not wait: %v %v", waiting, err)
	}
	if _, err := b.Send(ctx, Sender{}, "", "Answer", nil, q.ID); err == nil {
		t.Fatal("a question was answered with a reply on the bus")
	}
	if _, err := b.TakeQuestion(ctx, "window-asker", q.ID); !errors.Is(err, ErrQuestionGone) {
		t.Fatalf("a chat that was not asked took the question: %v", err)
	}
	got, err := b.TakeQuestion(ctx, "window-asked", q.ID)
	if err != nil || got.ID != q.ID || !got.Asked || got.Text != "Which parser files changed?" {
		t.Fatalf("the question was taken as %+v %v", got, err)
	}
	if _, err := b.TakeQuestion(ctx, "window-asked", q.ID); !errors.Is(err, ErrQuestionGone) {
		t.Fatalf("a question was taken twice: %v", err)
	}
	if wakes, _ := b.Waking(ctx); len(wakes) != 0 {
		t.Fatalf("a taken question still wakes: %+v", wakes)
	}
	// A withdrawn question wakes nobody and cannot be taken, and the bus
	// keeps it as a record.
	second, err := b.Ask(ctx, asker, "window-asked", "Still there?")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.WithdrawQuestion(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if wakes, _ := b.Waking(ctx); len(wakes) != 0 {
		t.Fatalf("a withdrawn question still wakes: %+v", wakes)
	}
	if _, err := b.TakeQuestion(ctx, "window-asked", second.ID); !errors.Is(err, ErrQuestionGone) {
		t.Fatalf("a withdrawn question was taken: %v", err)
	}
	if _, err := b.Message(ctx, second.ID); err != nil {
		t.Fatalf("the withdrawn question is gone from the record: %v", err)
	}
	// Withdrawing touches only questions.
	m, err := b.Send(ctx, Sender{Session: &asker}, "Asked", "An ordinary message", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.WithdrawQuestion(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := b.Deliver(ctx, "window-asked", DoorbellTurn); len(got) != 1 || got[0].ID != m.ID {
		t.Fatalf("withdrawing removed an ordinary message: %+v", got)
	}
	if _, err := b.Ask(ctx, asker, "window-unknown", "Anyone?"); err == nil {
		t.Fatal("a question went to a chat the bus does not know")
	}
}
