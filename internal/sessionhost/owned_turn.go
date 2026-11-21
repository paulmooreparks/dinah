package sessionhost

import (
	"context"
	"errors"
	"time"

	"studio/internal/acp"
)

// PromptDispatcher identifies native dispatch without borrowing a session-wide
// cancellation context or stopping a provider shared with unrelated chats.
type PromptDispatcher interface {
	PromptWithDispatch(context.Context, acp.PromptParams, any, func()) error
}

var ErrOwnedTurnBusy = errors.New("the chat is not idle for an exclusive task turn")

// TakeTurn uses the ordinary prompt construction and turn runner, but refuses
// occupied chats instead of steering or queueing work into another owner's turn.
// The native runner keeps its work reservation until the native call settles.
func (t *TurnIO) TakeTurn(ctx context.Context, s *Conversation, text, label string, prepared func() error) (string, End) {
	return t.TakeTurnStoppable(ctx, nil, s, text, label, prepared)
}

// TakeTurnStoppable is TakeTurn with a second way to stop the turn. When stop
// is closed, the agent is asked to cancel this call's dispatched prompt, under
// the guard a cancelled context has, and the call goes on waiting for the
// turn's end. It therefore returns the agent's own stop reason and the words
// written so far, which a cancelled context gives up. A nil stop never fires,
// and the call is then TakeTurn exactly.
func (t *TurnIO) TakeTurnStoppable(ctx context.Context, stop <-chan struct{}, s *Conversation, text, label string, prepared func() error) (string, End) {
	fail := func(err error) (string, End) { return "", End{StopReason: "error", Err: err} }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := t.host.HoldingTurns(); err != nil {
		return fail(err)
	}
	if current, err := t.host.ResolveContinuation(s.ID()); err != nil || current != s.ID() {
		if err == nil {
			err = errors.New("the task chat has a different current conversation")
		}
		return fail(err)
	}
	if _, ok := t.host.SessionConnection(s).(PromptDispatcher); !ok {
		return fail(errors.New("the provider has no owned prompt dispatch adapter"))
	}
	p, err := t.host.MakePrompt(s, text, nil, nil)
	if err != nil {
		return fail(err)
	}
	p = s.WithReply(ctx, p.WithLabel(label))
	dispatched, settled := make(chan struct{}), make(chan struct{})
	p.ownedCall = func(conn Connection, params acp.PromptParams, out any) error {
		dispatcher, ok := conn.(PromptDispatcher)
		if !ok {
			return errors.New("the current provider has no owned prompt dispatch adapter")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := prepared(); err != nil {
			return err
		}
		return dispatcher.PromptWithDispatch(context.Background(), params, out, func() { close(dispatched) })
	}
	submission := s.BeginSubmission()
	s.mu.Lock()
	admitted := s.details.BackgroundCountLocked() == 0 && s.details.RunningSubagentsLocked() == 0 && s.activity.AdmitLabelledLocked()
	s.mu.Unlock()
	if !admitted {
		submission.Release()
		return fail(ErrOwnedTurnBusy)
	}
	t.mu.Lock()
	if t.closing {
		t.mu.Unlock()
		s.ReleaseWork()
		submission.Release()
		return fail(errors.New("turn admission is closed"))
	}
	t.running.Add(1)
	t.mu.Unlock()
	EmitSession(t.host.Emit, s)
	EmitChat(t.host.Emit, s, EventSent, SentEvent(s.ID(), text, SentKind(&p, "asked"), p.Label(), p.Delivered()))
	go func() {
		defer t.running.Done()
		defer close(settled)
		t.Run(s, &p)
	}()
	submission.Release()
	cancellationDone := make(chan struct{})
	go func() {
		defer close(cancellationDone)
		select {
		case <-ctx.Done():
		case <-stop:
		case <-p.reply.done:
			return
		case <-settled:
			return
		}
		select {
		case <-dispatched:
		case <-p.reply.done:
			return
		case <-settled:
			return
		}
		gate := s.BeginSubmission()
		defer gate.Release()
		s.mu.Lock()
		matches := s.turns.capture != nil && s.turns.capture.SameReply(p)
		s.mu.Unlock()
		if matches {
			_ = t.CancelConversation(s)
		}
	}()
	words, end, complete := p.Wait(ctx)
	if complete {
		<-cancellationDone
		return words, end
	}
	s.RemoveQuestion(p)
	// A nonsettling provider stays visibly working. No new turn is admitted,
	// and task shutdown may checkpoint unknown before shared teardown.
	settlement := time.NewTimer(2 * time.Second)
	defer settlement.Stop()
	select {
	case <-p.reply.done:
		<-cancellationDone
		return fail(ctx.Err())
	case <-settled:
		<-cancellationDone
		return fail(ctx.Err())
	case <-settlement.C:
		return fail(ctx.Err())
	}
}
