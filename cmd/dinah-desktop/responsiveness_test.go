package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	extension "github.com/paulmooreparks/dinah-extension-go"
	"studio/internal/bridge"
)

func TestResponsivenessShellContractClearsAndExcludesOtherSurfaces(t *testing.T) {
	a := &App{}
	if a.SetResponsivenessRecording(false).Enabled || a.responsiveness.Load() != nil {
		t.Fatal("disabled initialization allocated a recorder")
	}
	if len(a.GetResponsivenessTimingSnapshot().Slots) != 0 || a.responsiveness.Load() != nil {
		t.Fatal("reading disabled state started recording")
	}
	b := bridge.New(struct{}{})
	a.events = b
	state := a.SetResponsivenessRecording(true)
	defer a.SetResponsivenessRecording(false)
	if !state.Enabled || state.Generation == 0 {
		t.Fatal("enable did not return a recording generation")
	}
	canary := "PRIVATE title/prompt/path/provider-output https://private.invalid --secret-command"
	a.emit(canary, map[string]any{"title": canary, "path": canary, "prompt": canary, "text": canary, "url": canary, "command": canary})
	snapshot := a.GetResponsivenessTimingSnapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), "private.invalid") {
		t.Fatal("canary reached shell export")
	}
	if len(snapshot.Slots) == 0 || *snapshot.Slots[0].Categories.Other.App.Count.Value != 1 || *snapshot.Slots[0].Categories.Other.Bridge.Count.Value != 1 {
		t.Fatal("shell and bridge recording not connected")
	}
	state = a.SetResponsivenessRecording(false)
	if state.Enabled || state.Generation <= snapshot.Recording.Generation || len(a.GetResponsivenessTimingSnapshot().Slots) != 0 {
		t.Fatal("disable retained old data")
	}
	bundles, err := a.builtinAppletBundles()
	if err != nil {
		t.Fatal(err)
	}
	for _, bundle := range bundles {
		manifest, err := json.Marshal(bundle.Manifest)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"SetResponsivenessRecording", "GetResponsivenessTimingSnapshot"} {
			if strings.Contains(string(manifest), name) {
				t.Fatal("shell method appeared in applet manifest", name)
			}
		}
	}
	iframeAPI, err := os.ReadFile("frontend/extensions.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SetResponsivenessRecording", "GetResponsivenessTimingSnapshot"} {
		if pageMethods[name] != pageQuery {
			t.Fatal("timing method has wrong page class", name)
		}
		if strings.Contains(string(extension.BrowserBridge), name) || strings.Contains(string(iframeAPI), name) {
			t.Fatal("timing method exposed to contributed frame", name)
		}
	}
}
