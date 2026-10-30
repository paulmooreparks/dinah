package main

import (
	"bytes"
	"strings"
	"studio/internal/applet"
	"studio/internal/applet/host"
	"testing"
	"time"
)

// A window chosen in a chat tells the agent how to drive it through the
// Windows applet, and which applet gives the same tool's data; a reference
// to something that is not a tool is refused.
func TestWindowMentionTellsTheAgentHowToDriveIt(t *testing.T) {
	a := NewApp()
	blocks, err := a.blocks(newSessionState(&sessionSeed{Cwd: t.TempDir()}), "@Files", []Attachment{{Kind: "window", Window: "files", Provider: "dinah.files", Name: "Files"}})
	if err != nil || len(blocks) != 2 {
		t.Fatalf("blocks = %+v, %v", blocks, err)
	}
	text := blocks[1].Text
	for _, want := range []string{`"Files"`, `(tool "files")`, windowOpenAction, windowRunAction, "dinah.files (Files)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the reference lacks %q: %s", want, text)
		}
	}
	if _, err := a.blocks(newSessionState(&sessionSeed{Cwd: t.TempDir()}), "x", []Attachment{{Kind: "window", Window: "../etc", Name: "x"}}); err == nil {
		t.Fatal("a window that is not a tool was accepted")
	}
}

func TestBuiltinAppletCatalogAndNotification(t *testing.T) {
	a := NewApp()
	ws := t.TempDir()
	catalog, err := a.AppletCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Workspace", "Chat", "Notifications", "Settings"} {
		found := false
		for _, m := range catalog {
			if m.Title == title && len(m.Actions) > 0 {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing callable %s", title)
		}
		blocks, err := a.blocks(newSessionState(&sessionSeed{Cwd: ws}), "@"+title, []Attachment{{Kind: "applet", Provider: "dinah." + strings.ToLower(title)}})
		if err != nil || len(blocks) != 2 {
			t.Fatalf("mention %s: %v", title, err)
		}
	}
	for _, owner := range []string{"chat", "applet"} {
		v, err := a.startAppletInvocation(ws, owner, "test-owner", "dinah.notifications/post@1", `{"title":"Review Ready","body":"Review the saved notes."}`, "", time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		v = awaitApplet(t, a, ws, owner, "test-owner", v.ID, "done")
		if !strings.Contains(v.OutputJSON, "Review Ready") {
			t.Fatal("missing notification")
		}
	}
	b := NewApp()
	results, err := b.AppletResults()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, v := range results {
		if v.Workspace == ws {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("persisted notices: %d", count)
	}
	v, err := a.startAppletInvocation(ws, "chat", "owner", "dinah.notifications/request@1", `{"title":"Question","body":"Which workspace?"}`, "", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	v = awaitApplet(t, a, ws, "chat", "owner", v.ID, "waiting")
	if _, err := a.AnswerAppletRequest(ws, v.ID, v.Version, `{"answer":""}`); err == nil {
		t.Fatal("accepted empty answer")
	}
	if _, err := a.AnswerAppletRequest(ws, v.ID, v.Version, `{"answer":"This workspace"}`); err != nil {
		t.Fatal(err)
	}
}

func TestBrokerHostSourceChangesDiscoveryDigest(t *testing.T) {
	bundles, err := NewApp().builtinAppletBundles()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, bundle := range bundles {
		source, ok := bundle.Sources["host/service.go"]
		if !ok {
			continue
		}
		found = true
		if !bytes.Equal(source, host.Source) {
			t.Fatal("host implementation missing")
		}
		first, err := applet.New(bundle)
		if err != nil {
			t.Fatal(err)
		}
		ref := bundle.Manifest.Actions[0].Ref
		before, err := first.Digest(ref)
		if err != nil {
			t.Fatal(err)
		}
		bundle.Sources["host/service.go"] = append(append([]byte{}, source...), []byte("\n// changed implementation\n")...)
		second, err := applet.New(bundle)
		if err != nil {
			t.Fatal(err)
		}
		after, err := second.Digest(ref)
		if err != nil {
			t.Fatal(err)
		}
		if before == after {
			t.Fatal("host change did not change discovery digest")
		}
	}
	if !found {
		t.Fatal("no built-in includes broker host source")
	}
}
