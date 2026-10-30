package main

import (
	"testing"
	"time"
)

func TestAppletResultsAcrossWorkspacesAndDismissal(t *testing.T) {
	previousConfig := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previousConfig })
	a := withTestApplets(t, NewApp())
	workspaces := []string{t.TempDir(), t.TempDir(), ""}
	var completed []AppletInvocation
	for _, workspace := range workspaces {
		v, err := a.InvokeAppletAt(workspace, testPictureID, testPictureRef, `{"text":"12345"}`)
		if err != nil {
			t.Fatal(err)
		}
		completed = append(completed, awaitApplet(t, a, workspace, "applet", testPictureID, v.ID, "done"))
	}
	// Failures are not entries: the caller has them.
	for _, status := range []string{"failed", "cancelled", "timeout"} {
		v := AppletInvocation{ID: status, Workspace: workspaces[0], OwnerKind: "chat", OwnerID: "c1", Status: status, Created: time.Now()}
		if err := saveAppletInvocation(&v); err != nil {
			t.Fatal(err)
		}
	}
	// A successful result with nothing the caller cannot show is not one;
	// a posted notification is, and so are artifacts a chat made, which no
	// chat window can open. The picture applet's own artifacts above are not.
	for id, action := range map[string]string{"described": "dinah.workspace/describe@1", "posted": "dinah.notifications/post@1", "chat-artifact": testPictureRef} {
		v := AppletInvocation{ID: id, Workspace: workspaces[0], OwnerKind: "chat", OwnerID: "run1", Action: action, Status: "done", OutputJSON: `{"title":"Hello"}`, Created: time.Now()}
		if id == "chat-artifact" {
			v.OwnerKind, v.OwnerID, v.Artifacts = "chat", "c1", []AppletArtifact{{ID: id + "_0", InvocationID: id, Name: "code.svg", MIME: "image/svg+xml"}}
		}
		if err := saveAppletInvocation(&v); err != nil {
			t.Fatal(err)
		}
	}
	pending := AppletInvocation{ID: "pending", Workspace: workspaces[1], Status: "waiting", Deadline: time.Now().Add(time.Hour), Created: time.Now()}
	if err := saveAppletInvocation(&pending); err != nil {
		t.Fatal(err)
	}
	b := withTestApplets(t, NewApp())
	results, err := b.AppletResults()
	if err != nil || len(results) != 2 || results[0].ID == results[1].ID {
		t.Fatalf("results after restart: %#v, %v", results, err)
	}
	for _, result := range results {
		if result.ID != "posted" && result.ID != "chat-artifact" {
			t.Fatalf("%s is shown, though its caller has it", result.ID)
		}
	}
	requests, err := b.AppletRequests()
	if err != nil || len(requests) != 1 || requests[0].ID != pending.ID {
		t.Fatalf("request contract changed: %#v, %v", requests, err)
	}
	both, err := b.AppletAttention()
	if err != nil || len(both.Requests) != 1 || both.Requests[0].ID != pending.ID || len(both.Results) != 2 {
		t.Fatalf("one pass lists other entries: %#v, %v", both, err)
	}
	if err := b.DismissAppletResultAt(workspaces[0], "posted"); err != nil {
		t.Fatal(err)
	}
	selected := completed[0]
	before, err := b.AppletArtifactAt(selected.Workspace, selected.Artifacts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.DismissAppletResultAt(workspaces[1], selected.ID); err == nil {
		t.Fatal("dismissed a result through the wrong workspace")
	}
	if err := b.DismissAppletResultAt(selected.Workspace, selected.ID); err != nil {
		t.Fatal(err)
	}
	stored, err := b.AppletInvocationAt(selected.Workspace, selected.ID)
	if err != nil || !stored.ResultDismissed || stored.OutputJSON != selected.OutputJSON {
		t.Fatalf("dismissal lost invocation output: %#v, %v", stored, err)
	}
	if err := b.DismissAppletResultAt(selected.Workspace, selected.ID); err != nil {
		t.Fatal(err)
	}
	c := withTestApplets(t, NewApp())
	after, err := c.AppletArtifactAt(selected.Workspace, selected.Artifacts[0].ID)
	if err != nil || after.DataBase64 != before.DataBase64 {
		t.Fatalf("dismissal lost artifact: %v", err)
	}
	storedAgain, err := c.AppletInvocationAt(selected.Workspace, selected.ID)
	if err != nil || storedAgain.Version != stored.Version {
		t.Fatalf("repeated dismissal changed record: %#v, %v", storedAgain, err)
	}
	results, err = c.AppletResults()
	if err != nil || len(results) != 1 || results[0].ID != "chat-artifact" {
		t.Fatalf("dismissal did not survive restart: %#v, %v", results, err)
	}
	all, err := c.AppletInvocationsAt(selected.Workspace)
	if err != nil || len(all) != 7 {
		t.Fatalf("dismissal removed history: %d, %v", len(all), err)
	}
}

func TestAppletResultsRejectPendingDismissal(t *testing.T) {
	previousConfig := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previousConfig })
	a := withTestApplets(t, NewApp())
	workspace := t.TempDir()
	for _, status := range []string{"running", "waiting"} {
		v := AppletInvocation{ID: status, Workspace: workspace, Status: status, Deadline: time.Now().Add(time.Hour), Created: time.Now()}
		if err := saveAppletInvocation(&v); err != nil {
			t.Fatal(err)
		}
		if err := a.DismissAppletResultAt(workspace, v.ID); err == nil {
			t.Fatalf("dismissed %s invocation", status)
		}
		stored, err := readAppletInvocation(workspace, v.ID)
		if err != nil || stored.ResultDismissed || stored.Version != v.Version {
			t.Fatalf("rejected dismissal changed record: %#v, %v", stored, err)
		}
	}
}
