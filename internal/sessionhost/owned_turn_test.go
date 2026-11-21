package sessionhost

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/acp"
)

type ownedPromptPeer struct {
	Connection
	entered chan struct{}
	release chan struct{}
	cancels atomic.Int32
	prompts atomic.Int32
}

func (p *ownedPromptPeer) PromptWithDispatch(_ context.Context, params acp.PromptParams, out any, dispatched func()) error {
	p.prompts.Add(1)
	dispatched()
	close(p.entered)
	<-p.release
	return acp.Result(acp.PromptResult{StopReason: "cancelled"}, out)
}

func (p *ownedPromptPeer) Notify(method string, params any) error {
	if method == "session/cancel" {
		p.cancels.Add(1)
		close(p.release)
	}
	return nil
}

func TestOwnedTurnRefusesBusyAndCancelsOnlyItsDispatchedPrompt(t *testing.T) {
	io, chat := turnFixture()
	peer := &ownedPromptPeer{Connection: io.host.SessionConnection(chat), entered: make(chan struct{}), release: make(chan struct{})}
	io.host.SessionConnection = func(*Conversation) Connection { return peer }
	io.host.ResolveContinuation = func(id string) (string, error) { return id, nil }
	io.host.MakePrompt = func(_ *Conversation, text string, _ *DocRef, _ []Attachment) (Prompt, error) {
		return NewPrompt(text, []acp.ContentBlock{{Type: "text", Text: text}}), nil
	}
	io.host.CancelQuestions = func(string) {}
	if _, end := io.TakeTurn(context.Background(), chat, "busy", "Task", func() error { return nil }); !errors.Is(end.Err, ErrOwnedTurnBusy) {
		t.Fatal(end)
	}
	if peer.prompts.Load() != 0 {
		t.Fatal("busy task dispatched")
	}
	chat.ReleaseWork()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan End, 1)
	go func() { _, end := io.TakeTurn(ctx, chat, "owned", "Task", func() error { return nil }); done <- end }()
	<-peer.entered
	if _, end := io.TakeTurn(context.Background(), chat, "overlap", "Other Task", func() error { return nil }); !errors.Is(end.Err, ErrOwnedTurnBusy) {
		t.Fatal(end)
	}
	cancel()
	<-done
	if !io.Close(time.Second) {
		t.Fatal("owned turn did not join")
	}
	if peer.prompts.Load() != 1 || peer.cancels.Load() != 1 || chat.Working() {
		t.Fatalf("prompts=%d cancels=%d working=%v", peer.prompts.Load(), peer.cancels.Load(), chat.Working())
	}
}

func TestOwnedTurnBuildsPromptAndPersistsBeforeNativeDispatch(t *testing.T) {
	io, chat := turnFixture()
	chat.ReleaseWork()
	peer := &ownedPromptPeer{Connection: io.host.SessionConnection(chat), entered: make(chan struct{}), release: make(chan struct{})}
	io.host.SessionConnection = func(*Conversation) Connection { return peer }
	io.host.ResolveContinuation = func(id string) (string, error) { return id, nil }
	built := false
	io.host.MakePrompt = func(_ *Conversation, text string, _ *DocRef, _ []Attachment) (Prompt, error) {
		built = true
		return NewPrompt(text, nil), nil
	}
	io.host.CancelQuestions = func(string) {}
	failure := errors.New("invented receipt write failure")
	_, end := io.TakeTurn(context.Background(), chat, "task", "Task", func() error { return failure })
	if !built || !errors.Is(end.Err, failure) || peer.prompts.Load() != 0 {
		t.Fatalf("built=%v prompts=%d end=%+v", built, peer.prompts.Load(), end)
	}
	if !io.Close(time.Second) {
		t.Fatal("failed task did not join")
	}
}

func TestStoppableOwnedTurnReturnsTheAgentsCancelledEnd(t *testing.T) {
	io, chat := turnFixture()
	chat.ReleaseWork()
	peer := &ownedPromptPeer{Connection: io.host.SessionConnection(chat), entered: make(chan struct{}), release: make(chan struct{})}
	io.host.SessionConnection = func(*Conversation) Connection { return peer }
	io.host.ResolveContinuation = func(id string) (string, error) { return id, nil }
	io.host.MakePrompt = func(_ *Conversation, text string, _ *DocRef, _ []Attachment) (Prompt, error) {
		return NewPrompt(text, []acp.ContentBlock{{Type: "text", Text: text}}), nil
	}
	io.host.CancelQuestions = func(string) {}
	stop := make(chan struct{})
	done := make(chan End, 1)
	go func() {
		_, end := io.TakeTurnStoppable(context.Background(), stop, chat, "owned", "Task", func() error { return nil })
		done <- end
	}()
	<-peer.entered
	close(stop)
	end := <-done
	if end.Err != nil || end.StopReason != "cancelled" {
		t.Fatalf("a stopped turn: %+v", end)
	}
	if !io.Close(time.Second) {
		t.Fatal("the stopped turn did not join")
	}
	if peer.prompts.Load() != 1 || peer.cancels.Load() != 1 || chat.Working() {
		t.Fatalf("prompts=%d cancels=%d working=%v", peer.prompts.Load(), peer.cancels.Load(), chat.Working())
	}
}
