package sessionhost

import (
	"context"
	"testing"

	"studio/internal/acp"
)

type unadvertisedSteerer struct{ *lifecycleConnection }

func (*unadvertisedSteerer) CanSteer(string) bool { return false }
func (*unadvertisedSteerer) Steer(context.Context, acp.PromptParams, func()) error {
	return acp.ErrSteeringRejected
}

func TestReachedHooksRemainUsableWhenNativeSteeringIsNotAdvertised(t *testing.T) {
	io, _ := turnFixture()
	c := NewConversation(Identity{ID: "chat", Provider: "claude"}, InitialActivity{Open: true, Working: true}, false, false)
	c.JoinBus("fixture")
	io.host.LookupConversation = func(string) *Conversation { return c }
	io.host.SessionConnection = func(*Conversation) Connection { return &unadvertisedSteerer{&lifecycleConnection{}} }
	io.host.HooksReach = func(string) (bool, error) { return true, nil }
	if !io.CanSteer(c.ID(), nil) {
		t.Fatal("absent native capability hid reached hooks")
	}
	if io.CanSteer(c.ID(), []Attachment{{Name: "fixture.png", Kind: "file", MimeType: "image/png"}}) {
		t.Fatal("text hooks accepted an image")
	}
	io.host.HooksReach = func(string) (bool, error) { return false, nil }
	if io.CanSteer(c.ID(), nil) {
		t.Fatal("unreached hooks offered steering")
	}
}
