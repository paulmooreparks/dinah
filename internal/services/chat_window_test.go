package services

import (
	"context"
	"errors"
	"testing"

	"studio/internal/servicechats"
	"studio/internal/taskhost"
)

type chatWindowBackend struct {
	servicechats.Backend
	changes []servicechats.Change
}

func (b *chatWindowBackend) PublishChange(_ context.Context, change servicechats.Change) error {
	b.changes = append(b.changes, change)
	return nil
}

// samplePage is a page service that reaches the port it was made with.
type samplePage struct{ port servicechats.Port }

func (s samplePage) Saved(ctx context.Context, workspace, key string) error {
	return s.port.PublishChange(ctx, workspace, key)
}

func registerChatWindowTest(t *testing.T, name string, factory ChatWindowFactory) {
	t.Helper()
	RegisterChatWindow(name, "Sample", []string{"sample-page"}, factory)
	t.Cleanup(func() { mu.Lock(); delete(chatWindowFactories, name); delete(windowPresentations, name); mu.Unlock() })
}

func TestChatWindowFactoryReceivesThePortOfItsOwnName(t *testing.T) {
	participant := &windowParticipant{}
	registerChatWindowTest(t, "sample-chats", func(port servicechats.Port) (Service, Participant, error) {
		if port.Service() != "sample-chats" {
			t.Errorf("the port acts as %q", port.Service())
		}
		return Object(samplePage{port}), participant, nil
	})
	if !HasWindowServices() {
		t.Fatal("a chat window service is a window service")
	}
	found := false
	for _, name := range Names() {
		found = found || name == "sample-chats"
	}
	if !found {
		t.Fatal("the service is not among the names the page reads")
	}
	if _, err := NewWindow(nil); err == nil {
		t.Fatal("a window without the chat port made a chat service")
	}
	backend := &chatWindowBackend{}
	chats := servicechats.New(backend)
	w, err := NewWindowWithChats(nil, chats)
	if err != nil {
		t.Fatal(err)
	}
	s, ok := w.Lookup("sample-chats")
	if !ok {
		t.Fatal("the service was not published to page callers")
	}
	if _, err := s.Call(context.Background(), "Saved", []byte(`["folder","record-1"]`)); err != nil {
		t.Fatal(err)
	}
	if len(backend.changes) != 1 || backend.changes[0].Service != "sample-chats" || backend.changes[0].Key != "record-1" {
		t.Fatalf("change: %+v", backend.changes)
	}
	// The window's shutdown order: the port's admission closes first, then
	// the participant finishes.
	chats.CloseAdmission()
	if err := w.Shutdown(context.Background()); err != nil || !participant.closed {
		t.Fatalf("shutdown: %v, participant closed %v", err, participant.closed)
	}
	if _, ok := w.Lookup("sample-chats"); ok {
		t.Fatal("a closed window still published the service")
	}
	if _, err := s.Call(context.Background(), "Saved", []byte(`["folder","record-2"]`)); !errors.Is(err, servicechats.ErrClosed) {
		t.Fatalf("a call after the port closed: %v", err)
	}
}

func TestChatWindowNameIsRegisteredOnceAcrossKinds(t *testing.T) {
	factory := func(servicechats.Port) (Service, Participant, error) {
		return Object(samplePage{}), &windowParticipant{}, nil
	}
	registerChatWindowTest(t, "sample-once", factory)
	for name, register := range map[string]func(){
		"chat window": func() { RegisterChatWindow("sample-once", "Sample", nil, factory) },
		"task window": func() {
			RegisterWindow("sample-once", func(taskhost.Client) (Service, Participant, error) { return nil, nil, nil })
		},
		"service": func() { Register("sample-once", Object(samplePage{})) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("a second registration as a %s did not panic", name)
				}
			}()
			register()
		}()
	}
}

func TestChatWindowFactoryFailureClosesThePort(t *testing.T) {
	registerChatWindowTest(t, "sample-failing", func(servicechats.Port) (Service, Participant, error) {
		return nil, nil, errors.New("invented start failure")
	})
	chats := servicechats.New(&chatWindowBackend{})
	w, err := NewWindowWithChats(nil, chats)
	if err == nil {
		t.Fatal("a failed factory made a window without an error")
	}
	if _, ok := w.Lookup("sample-failing"); ok {
		t.Fatal("a failed service was published")
	}
	if err := chats.ForService("sample-failing").PublishChange(context.Background(), "folder", "key"); !errors.Is(err, servicechats.ErrClosed) {
		t.Fatalf("the port stayed open after a failed start: %v", err)
	}
}
