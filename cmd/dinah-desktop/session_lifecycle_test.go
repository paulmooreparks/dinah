package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestLifecycleChannelDoesNotExposeItsAuthority(t *testing.T) {
	const secret = "channel-secret-sentinel"
	capability := &lifecycleChannel{app: &App{}, channel: &appletChannel{token: secret}}
	data, err := json.Marshal(capability)
	if err != nil || string(data) != "{}" {
		t.Fatalf("capability JSON %s %v", data, err)
	}
	if strings.Contains(fmt.Sprintf("%v %+v %#v", capability, capability, capability), secret) {
		t.Fatal("formatted capability exposes its token")
	}
}

func TestNewChatOptionsYieldToSavedDefaults(t *testing.T) {
	own := map[string]string{"mode": "default"}
	got := withNewChatOptions(nil, own)
	if string(got["mode"]) != `"default"` {
		t.Fatalf("a new chat with no saved mode got %s, want the provider's own", got["mode"])
	}
	saved := map[string]json.RawMessage{"mode": json.RawMessage(`"bypassPermissions"`), "model": json.RawMessage(`"opus"`)}
	got = withNewChatOptions(saved, own)
	if string(got["mode"]) != `"bypassPermissions"` || string(got["model"]) != `"opus"` {
		t.Fatalf("saved defaults were changed: %v", got)
	}
	if got := withNewChatOptions(nil, nil); len(got) != 0 {
		t.Fatalf("a provider with no settings of its own added %v", got)
	}
}
