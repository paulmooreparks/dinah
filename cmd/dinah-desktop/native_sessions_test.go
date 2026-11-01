package main

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"studio/internal/acp"
)

func TestNativeUnsentHandoffAllocatesAndPromptsWithoutResume(t *testing.T) {
	a, c := providerTestApp(t)
	parent := continuationSource(t, a)
	tr := &conversationTranscript{SessionID: "codex:untouched", Provider: "codex", Workspace: parent.Cwd(), ParentSessionID: parent.ID(), ParentHistory: []ConversationMessage{{Role: "you", Text: "Complete the report."}}, PendingHandoff: true, Handoff: "Complete the report.", Complete: true}
	if err := writeTranscript(tr); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(transcriptPath(tr.SessionID))
	info, handled, err := a.beforeNativeOpen(parent.Cwd(), tr.SessionID)
	if err != nil || !handled || info.ID == tr.SessionID || info.ID == "" {
		t.Fatalf("recovery: %+v %v %v", info, handled, err)
	}
	a.turnIO().Close(3 * time.Second)
	n, p := 0, 0
	for _, call := range c.calls {
		switch call.method {
		case "session/new":
			n++
		case "session/prompt":
			p++
		case "session/resume", "session/load":
			t.Fatalf("untouched allocation resumed: %s", call.method)
		}
	}
	if n != 1 || p != 1 {
		t.Fatalf("new=%d prompt=%d", n, p)
	}
	after, _ := os.ReadFile(transcriptPath(tr.SessionID))
	if !bytes.Equal(original, after) {
		t.Fatal("source history changed")
	}
	if !a.nativeRecord(info.ID).Succeeded {
		t.Fatal("successful first prompt not recorded")
	}
}

func TestNativeFirstPromptFailureAndCrashBlockResumeAcrossReload(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "interrupted"
		if failed {
			name = "limit"
		}
		t.Run(name, func(t *testing.T) {
			a, c := providerTestApp(t)
			s := newSessionState(&sessionSeed{ID: "codex:unverified", Provider: "codex", Cwd: t.TempDir()}, sessionInitial{})
			if err := a.beforeNativePrompt(s); err != nil {
				t.Fatal(err)
			}
			if failed {
				if err := a.afterNativePrompt(s, errors.New("weekly limit")); err != nil {
					t.Fatal(err)
				}
			}
			settings, err := loadSettings()
			if err != nil {
				t.Fatal(err)
			}
			a.settings = settings
			for range 2 {
				_, handled, err := a.beforeNativeOpen(s.Cwd(), s.ID())
				var detail *missingNativeError
				if !handled || !errors.As(err, &detail) || detail.detail.Kind != "session-unverified" {
					t.Fatalf("unexpected: %v %v", handled, err)
				}
			}
			if len(c.calls) != 0 {
				t.Fatal("unverified session contacted provider")
			}
		})
	}
}

func TestNativeRefusalPersistsWithoutRepeatedProviderCalls(t *testing.T) {
	a, c := providerTestApp(t)
	id := "codex:absent"
	_, err := a.nativeSessionMissing(t.TempDir(), id, errors.New("Resource not found"))
	var detail *missingNativeError
	if !errors.As(err, &detail) || detail.detail.Kind != "session-not-found" || detail.detail.Recoverable {
		t.Fatalf("missing detail: %v", err)
	}
	settings, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	a.settings = settings
	for range 2 {
		_, handled, err := a.beforeNativeOpen(t.TempDir(), id)
		if !handled || !errors.As(err, &detail) {
			t.Fatalf("refusal not retained: %v", err)
		}
	}
	if len(c.calls) != 0 {
		t.Fatal("refused identity contacted")
	}
}

func TestNativeAutomaticAllocationFailureDoesNotChainAcrossReload(t *testing.T) {
	a, c := providerTestApp(t)
	parent := continuationSource(t, a)
	id := "codex:allocation-failed"
	tr := &conversationTranscript{SessionID: id, Provider: "codex", Workspace: parent.Cwd(), ParentSessionID: parent.ID(), ParentHistory: []ConversationMessage{{Role: "you", Text: "Keep the report."}}, PendingHandoff: true, Handoff: "Keep the report.", Complete: true}
	if err := writeTranscript(tr); err != nil {
		t.Fatal(err)
	}
	c.newErr = errors.New("allocation unavailable")
	if _, _, err := a.beforeNativeOpen(parent.Cwd(), id); err == nil {
		t.Fatal("allocation failure hidden")
	}
	settings, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	a.settings = settings
	if _, _, err := a.beforeNativeOpen(parent.Cwd(), id); err == nil {
		t.Fatal("failed attempt repeated")
	}
	n := 0
	for _, call := range c.calls {
		if call.method == "session/new" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("allocation attempted %d times", n)
	}
}

func TestNativeLifecycleMissingRecoverySurvivesTwoReopens(t *testing.T) {
	a, c := providerTestApp(t)
	withResume(a)
	ws := t.TempDir()
	source := wholeCopy(t, ws, nil)
	c.resumeErr = &acp.Error{Code: acp.CodeResourceNotFound, Message: "Resource not found"}
	info, err := a.OpenSessionAt(ws, source)
	if err != nil || info.ID == source {
		t.Fatalf("automatic recovery: %+v %v", info, err)
	}
	a.turnIO().Close(3 * time.Second)
	c.resumeErr = nil
	for range 2 {
		closeChat(a, info.ID)
		settings, err := loadSettings()
		if err != nil {
			t.Fatal(err)
		}
		a.settings = settings
		reopened, err := a.OpenSessionAt(ws, source)
		if err != nil || reopened.ID != info.ID {
			t.Fatalf("reopen: %+v %v", reopened, err)
		}
	}
	if countCalls(c, "session/new") != 1 || countCalls(c, "session/prompt") != 1 || countCalls(c, "session/resume") != 3 {
		t.Fatalf("unexpected calls: %+v", c.calls)
	}
}

func TestNativeExplicitRecoveryPromptsThenSurvivesTwoCloseRestartCycles(t *testing.T) {
	a, c := providerTestApp(t)
	withResume(a)
	ws := t.TempDir()
	source := wholeCopy(t, ws, nil)
	original, _ := os.ReadFile(transcriptPath(source))
	p, err := a.PreviewSessionRecovery(source, "codex")
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := a.RecoverSession(source, "codex", p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !a.turnIO().Close(3 * time.Second) {
		t.Fatal("recovery prompt did not finish")
	}
	if countCalls(c, "session/prompt") != 1 {
		t.Fatal("explicit recovery did not submit its handoff immediately")
	}
	for range 2 {
		for _, requested := range []string{recovered.Session.ID, source} {
			closeChat(a, recovered.Session.ID)
			settings, err := loadSettings()
			if err != nil {
				t.Fatal(err)
			}
			a.settings = settings
			info, err := a.OpenSessionAt(ws, requested)
			if err != nil || info.ID != recovered.Session.ID {
				t.Fatalf("reopen: %+v %v", info, err)
			}
			tr, err := readTranscript(info.ID)
			if err != nil || tr.PendingHandoff || len(tr.ParentHistory) < 2 || tr.ParentHistory[0].Text != "Hello" || tr.ParentHistory[1].Text != "Hi there" || len(tr.Messages) == 0 {
				t.Fatalf("handoff/history: %+v %v", tr, err)
			}
		}
	}
	after, _ := os.ReadFile(transcriptPath(source))
	if !bytes.Equal(original, after) {
		t.Fatal("recovery modified source history")
	}
	if countCalls(c, "session/new") != 1 || countCalls(c, "session/resume") != 4 {
		t.Fatalf("recovery churned native identities: %+v", c.calls)
	}
}
