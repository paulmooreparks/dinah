package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"studio/internal/applet"
	"studio/internal/docservice"
)

// The registry takes the stand-alone documents' actions, and the two that
// reach into another program's window ask first.
func TestTheStandaloneDocumentsActionsAreOffered(t *testing.T) {
	a := NewApp()
	b, err := a.standaloneAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applet.New(b); err != nil {
		t.Fatalf("the registry refused the actions: %v", err)
	}
	asks := map[string]bool{}
	for _, act := range b.Manifest.Actions {
		asks[act.Ref] = applet.Asks(act)
	}
	if asks["dinah.standalone/list@1"] || !asks["dinah.standalone/open@1"] || !asks["dinah.standalone/act@1"] {
		t.Fatalf("which actions ask: %v", asks)
	}
	if !strings.Contains(b.Manifest.Description, documentBrand+" Editor") {
		t.Errorf("the description does not name the application: %s", b.Manifest.Description)
	}
}

// An action on a stand-alone document names the application and the
// document. One that names neither a live action of that application nor an
// open document is refused before anything reaches a window, and with no
// service there is nothing open.
func TestAnActionOnAStandaloneDocumentNamesItsDocument(t *testing.T) {
	a := NewApp()
	ctx := applet.WithInvocation(context.Background(), applet.Invocation{Workspace: t.TempDir(), OwnerKind: "chat", OwnerID: "c", ID: "i"})
	run := func(ref string, input any) (string, error) {
		raw, _ := json.Marshal(input)
		res, err := a.executeStandaloneApplet(ctx, ref, string(raw))
		return res.OutputJSON, err
	}
	t.Cleanup(func() {
		findDocumentService = func() (*docservice.Client, error) { return docservice.Find(documentsDir()) }
	})
	findDocumentService = func() (*docservice.Client, error) { return nil, os.ErrNotExist }
	if out, err := run("dinah.standalone/list@1", map[string]any{}); err != nil || out != `{"documents":[]}` {
		t.Fatalf("the list with no service: %s %v", out, err)
	}
	if _, err := run("dinah.standalone/act@1", map[string]any{"application": "editor", "document": "x", "action": "yavc.editor/document@1"}); err == nil || !strings.Contains(err.Error(), "no stand-alone document is open") {
		t.Fatalf("an action with no service: %v", err)
	}

	dir := t.TempDir()
	files := fstest.MapFS{"document.html": {Data: []byte("x")}}
	service, err := docservice.New(docservice.Options{Dir: dir, Files: http.FileServerFS(files), Page: "document.html", Cookie: "doc", Headless: true,
		Idle: time.Hour, Applications: documentApplications})
	if err != nil {
		t.Fatal(err)
	}
	go service.Serve()
	t.Cleanup(service.Stop)
	var client *docservice.Client
	for i := 0; i < 200 && client == nil; i++ {
		time.Sleep(10 * time.Millisecond)
		client, _ = docservice.Find(dir)
	}
	if client == nil {
		t.Fatal("the service did not start")
	}
	findDocumentService = func() (*docservice.Client, error) { return docservice.Find(dir) }
	file := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(file, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opened, err := client.Open(ctx, docservice.OpenRequest{Application: "editor", Path: file})
	if err != nil {
		t.Fatal(err)
	}
	out, err := run("dinah.standalone/list@1", map[string]any{})
	if err != nil || !strings.Contains(out, `"document":"`+opened.ID+`"`) || !strings.Contains(out, `"name":"notes.md"`) || !strings.Contains(out, `"application":"editor"`) {
		t.Fatalf("the list: %s %v", out, err)
	}
	for name, input := range map[string]map[string]any{
		"an action that opens a file":  {"application": "editor", "document": opened.ID, "action": "yavc.editor/open-file@1"},
		"another application's action": {"application": "editor", "document": opened.ID, "action": "yavc.docs/document@1"},
		"an application that is not":   {"application": "sheets", "document": opened.ID, "action": "yavc.editor/document@1"},
		"a document that is not open":  {"application": "editor", "document": "nothing", "action": "yavc.editor/document@1"},
	} {
		if out, err := run("dinah.standalone/act@1", input); err == nil {
			t.Errorf("%s was accepted: %s", name, out)
		}
	}
	// A proper action reaches the document's page, which nothing answers
	// here, so it waits until its context ends.
	short, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	raw, _ := json.Marshal(map[string]any{"application": "editor", "document": opened.ID, "action": "yavc.editor/document@1", "input": map[string]any{}})
	if _, err := a.executeStandaloneApplet(short, "dinah.standalone/act@1", string(raw)); err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("a proper action with no page to answer: %v", err)
	}
	// A file is handed over by its full path, to an application there is.
	if err := a.OpenStandalone("editor", "notes.md"); err == nil {
		t.Error("a relative path was handed over")
	}
	if err := a.OpenStandalone("sheets", file); err == nil {
		t.Error("a file was handed to an application there is not")
	}
}
