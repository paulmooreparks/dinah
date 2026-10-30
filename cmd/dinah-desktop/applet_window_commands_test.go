package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"studio/internal/applet"
)

// The Windows applet asks the page and gives back the page's answer, with
// the calling workspace added to what it asks; the page's refusal is the
// action's error. The page's own side is tested in e2e/page-view.js.
// A page that missed page:ask events reads the questions still waiting, each
// as its event carried it and oldest first; a question the page has answered
// is no longer pending, though the action has not yet returned.
func TestPendingPageAsksAreWhatTheEventCarried(t *testing.T) {
	a, _ := providerTestApp(t)
	var mu sync.Mutex
	var events []map[string]any
	emitted := make(chan struct{}, 2)
	a.sink = func(name string, data any) {
		if name == "page:ask" {
			mu.Lock()
			events = append(events, data.(map[string]any))
			mu.Unlock()
			emitted <- struct{}{}
		}
	}
	if got := a.PendingPageAsks(); len(got) != 0 {
		t.Fatalf("pending before any ask: %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for _, op := range []string{"windows", "commands"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.askPage(ctx, op, map[string]any{"n": op})
		}()
		<-emitted
	}
	// The event is a map and the pending list a struct, so their key order
	// differs; what the page reads is the same.
	var got, want any
	pending, _ := json.Marshal(a.PendingPageAsks())
	sent, _ := json.Marshal(events)
	json.Unmarshal(pending, &got)
	json.Unmarshal(sent, &want)
	if !reflect.DeepEqual(got, want) || len(events) != 2 {
		t.Fatalf("pending %s differs from the events %s", pending, sent)
	}
	a.AnswerPage(events[0]["id"].(string), `{}`, "")
	left := a.PendingPageAsks()
	if len(left) != 1 || left[0].ID != events[1]["id"] || left[0].Op != "commands" {
		t.Fatalf("after answering the first: %+v", left)
	}
	cancel()
	wg.Wait()
	if got := a.PendingPageAsks(); len(got) != 0 {
		t.Fatalf("pending after every ask ended: %+v", got)
	}
}

func TestWindowsAppletAsksThePage(t *testing.T) {
	a, _ := providerTestApp(t)
	r, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	ctx := applet.WithInvocation(context.Background(), applet.Invocation{Workspace: ws, OwnerKind: "chat", OwnerID: "windows-test", ID: "windows-call"})
	var asked map[string]any
	a.sink = func(name string, data any) {
		if name != "page:ask" {
			return
		}
		ask := data.(map[string]any)
		asked = ask
		args := ask["args"].(map[string]any)
		switch ask["op"] {
		case "windows":
			go a.AnswerPage(ask["id"].(string), `{"windows":[{"window":"item-1","kind":"editor","title":"Roadmap","front":true}]}`, "")
		case "run":
			if args["command"] == "discard" {
				go a.AnswerPage(ask["id"].(string), "", "Discard would destroy something, so it is left to the person to press.")
			} else {
				go a.AnswerPage(ask["id"].(string), `{"ran":"Split"}`, "")
			}
		}
	}
	result, err := r.Execute(ctx, windowsListAction, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	var listed struct {
		Windows []struct{ Window, Kind string } `json:"windows"`
	}
	if err := json.Unmarshal([]byte(result.OutputJSON), &listed); err != nil || len(listed.Windows) != 1 || listed.Windows[0].Kind != "editor" {
		t.Fatalf("windows = %s, %v", result.OutputJSON, err)
	}
	if args := asked["args"].(map[string]any); args["workspace"] != ws {
		t.Fatalf("the page was not told the calling workspace: %v", args)
	}
	if _, err := r.Execute(ctx, windowRunAction, `{"window":"item-1","command":"view-split"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Execute(ctx, windowRunAction, `{"window":"item-1","command":"discard"}`); err == nil || !strings.Contains(err.Error(), "left to the person") {
		t.Fatalf("a refused command ran: %v", err)
	}
}

// An action started outside a workspace is refused before the page is asked.
func TestWindowsAppletNeedsAWorkspace(t *testing.T) {
	a, _ := providerTestApp(t)
	r, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	a.sink = func(name string, data any) {
		if name == "page:ask" {
			t.Errorf("the page was asked without a workspace")
		}
	}
	if _, err := r.Execute(context.Background(), windowsListAction, `{}`); err == nil {
		t.Fatal("listed windows without a workspace")
	}
}
