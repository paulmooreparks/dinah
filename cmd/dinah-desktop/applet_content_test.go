package main

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"studio/internal/applet"
)

// Every action of the four content packages is offered to agents under the
// release's own reference and schemas, with capabilities in Dinah's words:
// a change to an open document asks, as a workspace write does.
func TestTheContentApplicationsActionsAreOfferedWithDinahsCapabilities(t *testing.T) {
	a := NewApp()
	var bundles []applet.Bundle
	counts := map[string]int{}
	for _, app := range contentApps {
		b, err := a.contentAppletBundle(app)
		if err != nil {
			t.Fatal(err)
		}
		bundles = append(bundles, b)
		counts[b.Manifest.ID] = len(b.Manifest.Actions)
	}
	if _, err := applet.New(bundles...); err != nil {
		t.Fatalf("the registry refused the content actions: %v", err)
	}
	if counts["yavc.editor"] != 25 || counts["yavc.docs"] != 27 || counts["yavc.diagrams"] != 17 || counts["yavc.slides"] != 31 {
		t.Fatalf("action counts %v", counts)
	}
	caps := map[string][]string{}
	for _, b := range bundles {
		for _, act := range b.Manifest.Actions {
			caps[act.Ref] = act.Capabilities
		}
	}
	asks := func(ref string) bool { return applet.Asks(applet.Action{Capabilities: caps[ref]}) }
	for _, ref := range []string{"yavc.editor/insert@1", "yavc.editor/set-text@1", "yavc.editor/save@1", "yavc.docs/format@1", "yavc.diagrams/canvas-edit@1", "yavc.editor/open-file@1"} {
		if !asks(ref) {
			t.Errorf("%s changes a document but would not ask: %v", ref, caps[ref])
		}
	}
	for _, ref := range []string{"yavc.editor/documents@1", "yavc.editor/document@1", "yavc.editor/outline@1"} {
		if asks(ref) {
			t.Errorf("%s only reads but would ask: %v", ref, caps[ref])
		}
	}
}

// In a product whose agents propose their changes, an action the release
// marks as changing a document is absent from what agents are offered and
// cannot be called, the rest are as they were, and the description says
// how to propose. In any other product every action is offered.
func TestAnActionThatChangesADocumentIsAbsentWhereAgentsPropose(t *testing.T) {
	data := []byte(`{"id":"yavc.sample","version":"1.0.0","apiVersion":1,"title":"Sample","description":"A sample.","actions":[` +
		`{"ref":"yavc.sample/read@1","title":"Read","description":"Read it.","live":true,"replay":"pure","changes":"none","capabilities":["files.read"],"inputSchema":{"type":"object"},"outputSchema":{"type":"object"}},` +
		`{"ref":"yavc.sample/unsaid@1","title":"Unsaid","description":"Says nothing of what it changes.","live":true,"replay":"once","capabilities":[],"inputSchema":{"type":"object"},"outputSchema":{"type":"object"}},` +
		`{"ref":"yavc.sample/set-text@1","title":"Set","description":"Set it.","live":true,"replay":"once","changes":"document","capabilities":[],"inputSchema":{"type":"object"},"outputSchema":{"type":"object"}}]}`)
	a := NewApp()
	refs := func(propose bool) (string, []string) {
		b, err := a.contentBundleFrom("sample", "sample/actions.json", data, propose)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := applet.New(b); err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, act := range b.Manifest.Actions {
			if b.Handlers[act.Ref] == nil {
				t.Fatalf("%s is offered and cannot be run", act.Ref)
			}
			out = append(out, act.Ref)
		}
		if len(b.Handlers) != len(out) {
			t.Fatalf("%d handlers for %d actions", len(b.Handlers), len(out))
		}
		return b.Manifest.Description, out
	}
	if note, got := refs(false); len(got) != 3 || strings.Contains(note, "dinah.documents/suggest@1") {
		t.Fatalf("with no rule: %v %q", got, note)
	}
	if note, got := refs(true); len(got) != 1 || got[0] != "yavc.sample/read@1" || !strings.Contains(note, "dinah.documents/suggest@1") {
		t.Fatalf("where agents propose: %v %q", got, note)
	}
	// The release taken says of every action which it is, and the rule
	// reaches the two applications that show suggestions and no other.
	for _, app := range contentApps {
		name := "frontend/yavc/content/" + app + "/actions.json"
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			t.Fatal(err)
		}
		var m contentManifest
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		changing := 0
		for _, act := range m.Actions {
			if act.Changes != "document" && act.Changes != "none" {
				t.Fatalf("%s does not say whether it changes a document", act.Ref)
			}
			if act.Changes == "document" {
				changing++
			}
		}
		all, err := a.contentBundleFrom(app, name, data, false)
		if err != nil {
			t.Fatal(err)
		}
		kept, err := a.contentBundleFrom(app, name, data, suggestionApps[app])
		if err != nil {
			t.Fatal(err)
		}
		want := len(all.Manifest.Actions)
		if suggestionApps[app] {
			want -= changing
			if changing == 0 {
				t.Fatalf("%s marks no action as changing a document", app)
			}
		}
		if len(kept.Manifest.Actions) != want {
			t.Fatalf("%s offers %d actions where agents propose, not %d", app, len(kept.Manifest.Actions), want)
		}
	}
}
