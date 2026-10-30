package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/applet"
	"studio/internal/applet/broker"
	"studio/internal/ollamaagent"
)

// A chat's applet action that has an effect asks the person in that chat
// first: a read runs without asking, Always Allow is remembered for the
// workspace until forgotten, and Don't Allow fails the action before it
// changes anything.
func TestAppletConsent(t *testing.T) {
	a, _, _ := settingsAppletFixture(t)
	ws := t.TempDir()
	var mu sync.Mutex
	var asked []map[string]any
	a.sink = func(name string, data any) {
		if name == "agent:permission" {
			mu.Lock()
			asked = append(asked, data.(map[string]any))
			mu.Unlock()
		}
	}
	question := func(n int) map[string]any {
		t.Helper()
		deadline := time.Now().Add(testWait)
		for time.Now().Before(deadline) {
			mu.Lock()
			if len(asked) >= n {
				q := asked[n-1]
				mu.Unlock()
				return q
			}
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("question %d was not asked", n)
		return nil
	}
	start := func(ref, input string) AppletInvocation {
		t.Helper()
		v, err := a.startAppletInvocation(ws, "chat", "chat-1", ref, input, "", time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	update := func() string {
		read := awaitApplet(t, a, ws, "chat", "chat-1", start("dinah.settings/read@1", `{}`).ID, "done")
		var s AppletSettingsSnapshot
		json.Unmarshal([]byte(read.OutputJSON), &s)
		input, _ := json.Marshal(map[string]any{"revision": s.Revision, "changes": map[string]any{"showThinking": !s.Preferences.ShowThinking}})
		return string(input)
	}

	input := update()
	mu.Lock()
	readAsked := len(asked)
	mu.Unlock()
	if readAsked != 0 {
		t.Fatal("reading the settings asked the person")
	}
	v := start("dinah.settings/update@1", input)
	q := question(1)
	if call, _ := q["toolCall"].(json.RawMessage); q["sessionId"] != "chat-1" || !strings.Contains(string(call), `"title":`) || !strings.Contains(string(call), `"rawInput":`) {
		t.Fatalf("the question was %+v", q)
	}
	a.Answer(q["key"].(string), consentAlways)
	awaitApplet(t, a, ws, "chat", "chat-1", v.ID, "done")
	if grants := a.AppletGrants(); len(grants) != 1 || grants[0].Workspace != ws || grants[0].Action != "dinah.settings/update@1" {
		t.Fatalf("the grant is %+v", grants)
	}
	// Allowed always: the next update runs without asking.
	awaitApplet(t, a, ws, "chat", "chat-1", start("dinah.settings/update@1", update()).ID, "done")
	mu.Lock()
	after := len(asked)
	mu.Unlock()
	if after != 1 {
		t.Fatal("an action allowed always asked again")
	}
	// Forgotten: it asks again, and Don't Allow fails it unchanged.
	if err := a.ForgetAppletGrant(ws, "dinah.settings/update@1"); err != nil || len(a.AppletGrants()) != 0 {
		t.Fatalf("forget: %v %+v", err, a.AppletGrants())
	}
	before := a.settings.ShowThinking
	v = start("dinah.settings/update@1", update())
	a.Answer(question(2)["key"].(string), consentRefuse)
	failed := awaitApplet(t, a, ws, "chat", "chat-1", v.ID, "failed")
	if !strings.Contains(failed.Error, "did not allow") || a.settings.ShowThinking != before {
		t.Fatalf("a refused action ran: %+v", failed)
	}
}

// A chat whose provider's settings let it run unattended is not asked about an
// applet action, and one on the default settings is. The provider says it: the
// consent code asks the provider's module and names none.
func TestAChatRunUnattendedIsNotAskedForConsent(t *testing.T) {
	a, _ := providerTestApp(t)
	action := applet.Action{Ref: "dinah.settings/write@1", Effect: "history"}
	chat := func(id string) broker.Invocation {
		a.runtime.PutConversationLocked(id, newSessionState(&sessionSeed{ID: id, Provider: providerForSession(id), Cwd: t.TempDir()}, sessionInitial{Open: true}))
		return broker.Invocation{ID: "call-" + id, Workspace: t.TempDir(), OwnerKind: "chat", OwnerID: id}
	}
	asked := make(chan struct{}, 4)
	a.sink = func(name string, data any) {
		if name == "agent:permission" {
			asked <- struct{}{}
		}
	}
	codex, ollama := chat("codex:one"), chat("ollama:two")
	// On their settings as they come, a question is asked, which the context's end ends.
	for _, v := range []broker.Invocation{codex, ollama} {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		err := a.appletConsent(ctx, v, action)
		cancel()
		select {
		case <-asked:
		default:
			t.Errorf("%s: no question was asked on the default settings (%v)", v.OwnerID, err)
		}
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.CodexAppletApprovals = "allow"
		s.OllamaApprovals = ollamaagent.ApprovalsAll
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, v := range []broker.Invocation{codex, ollama} {
		// A question that is asked is left unanswered, so the context's end ends it
		// and the test fails instead of waiting for a person.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := a.appletConsent(ctx, v, action)
		cancel()
		if err != nil {
			t.Errorf("%s was asked on settings that let it run unattended: %v", v.OwnerID, err)
		}
	}
}
