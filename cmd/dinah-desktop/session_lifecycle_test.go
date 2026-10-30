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
