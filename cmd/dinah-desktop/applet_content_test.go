package main

import (
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
	if counts["yavc.editor"] != 25 || counts["yavc.docs"] != 27 || counts["yavc.diagrams"] != 16 || counts["yavc.slides"] != 31 {
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
