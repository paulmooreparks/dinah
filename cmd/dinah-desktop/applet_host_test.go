package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/applet"
)

func awaitApplet(t *testing.T, a *App, ws, kind, owner, id, status string) AppletInvocation {
	t.Helper()
	deadline := time.Now().Add(testWait)
	for time.Now().Before(deadline) {
		v, err := a.appletInvocation(ws, kind, owner, id)
		if err != nil {
			t.Fatal(err)
		}
		if v.Status == status {
			return v
		}
		if v.Status == "failed" || v.Status == "cancelled" || v.Status == "timeout" {
			t.Fatalf("wanted %s, got %#v", status, v)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("applet did not reach " + status)
	return AppletInvocation{}
}

func TestAppletHumanRequestSurvivesRestart(t *testing.T) {
	ws := t.TempDir()
	a := withTestApplets(t, NewApp())
	v, err := a.startAppletInvocation(ws, "chat", "chat-one", testChecklistRef, `{"title":"Release Review","items":["Evidence checked","Owner assigned"]}`, "", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	v = awaitApplet(t, a, ws, "chat", "chat-one", v.ID, "waiting")
	if _, err := a.appletInvocation(ws, "chat", "chat-two", v.ID); err == nil {
		t.Fatal("foreign chat read request")
	}
	if err := a.cancelAppletInvocation(ws, "chat", "chat-two", v.ID); err == nil {
		t.Fatal("foreign chat cancelled request")
	}
	b := withTestApplets(t, NewApp())
	requests, err := b.AppletRequests()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, request := range requests {
		if request.ID == v.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("pending request missing after restart")
	}
	if _, err := b.AnswerAppletRequest(ws, v.ID, v.Version, `{"item_1":false}`); err == nil {
		t.Fatal("missing required false-capable field accepted")
	}
	if _, err := b.AnswerAppletRequest(ws, v.ID, v.Version, `{"item_1":false,"item_2":true,"unexpected":true}`); err == nil {
		t.Fatal("unknown field accepted")
	}
	final, err := b.AnswerAppletRequest(ws, v.ID, v.Version, `{"item_1":false,"item_2":true,"notes":"Follow up"}`)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "done" || !strings.Contains(final.OutputJSON, `"item_1":false`) {
		t.Fatalf("response lost: %#v", final)
	}
	if _, err := b.AnswerAppletRequest(ws, v.ID, v.Version, `{"item_1":true,"item_2":true}`); err == nil {
		t.Fatal("duplicate completion accepted")
	}
	if _, err := b.AppletInvocationAt(t.TempDir(), v.ID); err == nil {
		t.Fatal("other workspace read invocation")
	}
}

func TestAppletArtifactOwnershipAndPersonalScope(t *testing.T) {
	a := withTestApplets(t, NewApp())
	v, err := a.InvokeAppletAt("", testPictureID, testPictureRef, `{"text":"000123"}`)
	if err != nil {
		t.Fatal(err)
	}
	v = awaitApplet(t, a, "", "applet", testPictureID, v.ID, "done")
	if len(v.Artifacts) != 1 || v.Artifacts[0].DataBase64 != "" {
		t.Fatalf("unexpected descriptor %#v", v.Artifacts)
	}
	artifact, err := a.appletArtifact("", "applet", testPictureID, v.Artifacts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(artifact.DataBase64)
	if err != nil || !strings.Contains(string(data), "<svg") {
		t.Fatalf("invalid SVG %v", err)
	}
	if _, err := a.appletArtifact("", "chat", "unrelated", artifact.ID); err == nil {
		t.Fatal("foreign owner read artifact")
	}
	if _, err := a.InvokeAppletAt("", "test.checklist", testPictureRef, `{}`); err == nil {
		t.Fatal("provider impersonation allowed")
	}
	if _, err := a.startAppletInvocation("", "chat", "chat-one", testPictureRef, `{"text":"test"}`, "", time.Now().Add(time.Minute)); err == nil {
		t.Fatal("chat acquired personal authority")
	}
}

func TestAppletStatusRecoversInterruptedAction(t *testing.T) {
	a := withTestApplets(t, NewApp())
	ws := t.TempDir()
	r, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := r.Digest(testPictureRef)
	id, _ := newAppletID()
	v := AppletInvocation{ID: id, Workspace: ws, OwnerKind: "chat", OwnerID: "chat-one", Action: testPictureRef, Digest: digest, InputJSON: `{"text":"Recovered"}`, Status: "running", Deadline: time.Now().Add(time.Hour), Created: time.Now()}
	if err := saveAppletInvocation(&v); err != nil {
		t.Fatal(err)
	}
	got := awaitApplet(t, a, ws, "chat", "chat-one", id, "done")
	if got.ID != id || len(got.Artifacts) != 1 {
		t.Fatalf("recovery replaced invocation: %#v", got)
	}
	stored, err := readAppletInvocation(ws, id)
	if err != nil {
		t.Fatal(err)
	}
	stored.Status = "running"
	stored.Digest = "different"
	if err := saveAppletInvocation(&stored); err != nil {
		t.Fatal(err)
	}
	if _, err := withTestApplets(t, NewApp()).appletInvocation(ws, "chat", "chat-one", id); !errors.Is(err, errAppletRecovery) {
		t.Fatalf("changed implementation replay: %v", err)
	}
}

func TestAppletRequestCancellationDeadlineAndDigest(t *testing.T) {
	for _, operation := range []string{"cancel", "deadline", "digest"} {
		t.Run(operation, func(t *testing.T) {
			a := withTestApplets(t, NewApp())
			ws := t.TempDir()
			v, err := a.startAppletInvocation(ws, "chat", "run-one", testChecklistRef, `{"title":"Review","items":["Check"]}`, "", time.Now().Add(time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			v = awaitApplet(t, a, ws, "chat", "run-one", v.ID, "waiting")
			switch operation {
			case "cancel":
				if err := a.cancelAppletInvocation(ws, "chat", "run-one", v.ID); err != nil {
					t.Fatal(err)
				}
			case "deadline":
				saved, _ := readAppletInvocation(ws, v.ID)
				saved.Deadline = time.Now().Add(-time.Second)
				if err := saveAppletInvocation(&saved); err != nil {
					t.Fatal(err)
				}
				v.Version = saved.Version
			case "digest":
				saved, _ := readAppletInvocation(ws, v.ID)
				saved.Digest = "changed"
				if err := saveAppletInvocation(&saved); err != nil {
					t.Fatal(err)
				}
				v.Version = saved.Version
			}
			if _, err := a.AnswerAppletRequest(ws, v.ID, v.Version, `{"item_1":true}`); err == nil {
				t.Fatal("inactive or changed request completed")
			}
		})
	}
}

func TestAppletCorruptionFailsClosed(t *testing.T) {
	a := withTestApplets(t, NewApp())
	ws := t.TempDir()
	id, _ := newAppletID()
	root := appletStoreRoot(ws)
	v := AppletInvocation{ID: id, Workspace: ws, OwnerKind: "chat", OwnerID: "run-one", Action: testPictureRef, InputJSON: `{}`, Status: "running"}
	if err := saveAppletInvocation(&v); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateLayout().Runs(root).SlotPath(id, 0), []byte(`{"broken":`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.appletInvocation(ws, "chat", "run-one", id); !errors.Is(err, errAppletRecovery) {
		t.Fatalf("corruption did not fail closed: %v", err)
	}
	if _, err := a.startAppletInvocationID(ws, "chat", "run-one", id, testPictureRef, `{"text":"test"}`, "", time.Now().Add(time.Minute)); !errors.Is(err, errAppletRecovery) {
		t.Fatalf("corrupt invocation recreated: %v", err)
	}
}

func TestAppletPersistenceFailureBlocksConsumption(t *testing.T) {
	ws := t.TempDir()
	started := make(chan struct{})
	release := make(chan struct{})
	handler := func(ctx context.Context, input string) (applet.Result, error) {
		close(started)
		<-release
		return applet.Result{OutputJSON: input}, nil
	}
	a := NewApp()
	b, err := a.appletHostService().Broker()
	if err != nil {
		t.Fatal(err)
	}
	b.Registry = testEchoRegistry(t, handler)
	v, err := a.startAppletInvocation(ws, "chat", "chat-one", testEchoRef, `{"value":9007199254740993}`, "", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	<-started
	blocked := stateLayout().Runs(appletStoreRoot(ws)).SlotPath(v.ID, 1)
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	close(release)
	// Reading the obstructed slot can fail before the worker finishes. Wait
	// for its attempt before checking recovery or removing the obstruction.
	for wait := time.Now().Add(3 * time.Second); b.Executing() > 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(wait) {
			t.Fatal("applet worker did not finish")
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, err = a.appletInvocation(ws, "chat", "chat-one", v.ID)
		if errors.Is(err, errAppletRecovery) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !errors.Is(err, errAppletRecovery) {
		t.Fatalf("uncertain result remained consumable: %v", err)
	}
	if _, err := a.appletInvocation(ws, "chat", "chat-one", v.ID); !errors.Is(err, errAppletRecovery) {
		t.Fatal("persistence failure disappeared")
	}
	if _, err := os.Stat(filepath.Dir(blocked)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
}
