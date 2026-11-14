package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/applet"
	"studio/internal/applet/broker"
	"studio/internal/ollamaagent"
)

func TestPersonOnlyFreshApprovalIgnoresBlanketGrants(t *testing.T) {
	testFreshApprovalForAction(t, applet.Action{Ref: "test.install/apply@1", Title: "Install", Effect: "local", Capabilities: []string{"person.approval"}})
}

func testFreshApprovalForAction(t *testing.T, action applet.Action) {
	for _, mode := range []string{"normal", "all", "workspace", "codex-unattended", "ollama-unattended"} {
		t.Run(mode, func(t *testing.T) {
			a, _ := providerTestApp(t)
			ws := t.TempDir()
			id := "codex:fresh"
			if mode == "ollama-unattended" {
				id = "ollama:fresh"
			}
			a.runtime.PutConversationLocked(id, newSessionState(&sessionSeed{ID: id, Provider: providerForSession(id), Cwd: ws}, sessionInitial{Open: true}))
			if err := a.updateSettings(func(s *Settings) error {
				switch mode {
				case "all":
					s.Approvals = ApprovalsAll
				case "workspace":
					s.AppletGrants = map[string][]string{ws: {action.Ref}}
				case "codex-unattended":
					s.CodexAppletApprovals = "allow"
				case "ollama-unattended":
					s.OllamaApprovals = ollamaagent.ApprovalsAll
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			v := broker.Invocation{ID: "fresh", Workspace: ws, OwnerKind: "chat", OwnerID: id, InputJSON: "{}"}
			asked := 0
			answer := consentOnce
			a.sink = func(name string, data any) {
				if name != "agent:permission" {
					return
				}
				asked++
				q := data.(map[string]any)
				for _, option := range q["options"].([]acp.PermissionOption) {
					if option.OptionID == consentAlways {
						t.Error("fresh approval offered a saved grant")
					}
				}
				raw := q["toolCall"].(json.RawMessage)
				if !strings.Contains(string(raw), "approval required every time") {
					t.Error("question did not explain fresh approval")
				}
				notices, err := a.Notifications()
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, notice := range notices {
					if strings.Contains(notice.Body, "approval required every time") {
						found = true
					}
				}
				if !found {
					t.Error("Notifications did not explain fresh approval")
				}
				a.Answer(q["key"].(string), answer)
			}
			for i := 0; i < 2; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				err := a.appletConsent(ctx, v, action)
				cancel()
				if err != nil {
					t.Fatal(err)
				}
			}
			if asked != 2 {
				t.Fatalf("asked %d times", asked)
			}
			answer = consentAlways
			if err := a.appletConsent(context.Background(), v, action); err == nil {
				t.Fatal("an invalid always-allow answer authorized a fresh-only action")
			}
		})
	}
}

func TestPersonOnlyRestartCommandsRefusedBeforePage(t *testing.T) {
	a, _ := providerTestApp(t)
	r, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	a.sink = func(name string, _ any) {
		if name == "page:ask" {
			t.Error("restart command reached the page")
		}
	}
	ctx := applet.WithInvocation(context.Background(), applet.Invocation{Workspace: t.TempDir(), OwnerKind: "chat", OwnerID: "agent", ID: "run"})
	for _, command := range []string{"pause-and-restart", "restart-to-update", "restart-now"} {
		input, _ := json.Marshal(map[string]string{"window": "any", "command": command})
		if _, err := r.Execute(ctx, windowRunAction, string(input)); err == nil || !strings.Contains(err.Error(), "left to the person") {
			t.Fatalf("%s: %v", command, err)
		}
	}
}

func TestPersonOnlyPageMethodsRequireWindowAuthority(t *testing.T) {
	s := &shellServer{addr: "127.0.0.1:47616", token: "window-token", codes: map[string]shellCode{}}
	for _, method := range []string{"AnswerDecision", "DismissDecision", "ConfirmDecisionBroadcast", "PauseAndRestart", "RestartToUpdate", "CancelRestart", "RemoveSetAsidePrograms", "SetBackgroundChatLimit", "CallService", "Answer", "AnswerAppletRequest"} {
		for _, caller := range []string{"local-program", "agent-helper", "wrong-origin"} {
			req := httptest.NewRequest(http.MethodPost, "http://"+s.addr+"/bridge/call", strings.NewReader(`{"method":"`+method+`","args":[]}`))
			want := http.StatusUnauthorized
			if caller == "agent-helper" {
				req.Header.Set("Authorization", "Bearer agent-token")
			}
			if caller == "wrong-origin" {
				req.AddCookie(&http.Cookie{Name: shellCookie, Value: s.token})
				req.Header.Set("Origin", "http://other.invalid")
				want = http.StatusForbidden
			}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, req)
			if w.Code != want {
				t.Fatalf("%s %s: %d", caller, method, w.Code)
			}
		}
	}
}

func TestPersonOnlyAppMethodsAreNotAppletActions(t *testing.T) {
	a, _ := providerTestApp(t)
	r, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ctx := applet.WithInvocation(context.Background(), applet.Invocation{Workspace: t.TempDir(), OwnerKind: "chat", OwnerID: "agent", ID: "guess"})
	for _, ref := range []string{"dinah.decisions/answer@1", "dinah.decisions/dismiss@1", "dinah.decisions/broadcast@1", "dinah.settings/remove-set-aside@1", "dinah.settings/repair-agents@1", "dinah.windows/pause-and-restart@1"} {
		for _, bundle := range r.Catalog() {
			for _, action := range bundle.Actions {
				if action.Ref == ref {
					t.Fatalf("person-only action advertised: %s", ref)
				}
			}
		}
		if _, err := r.Execute(ctx, ref, `{}`); err == nil {
			t.Fatalf("guessed person-only action ran: %s", ref)
		}
	}
}
