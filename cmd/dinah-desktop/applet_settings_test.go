package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"studio/internal/applet"
)

func settingsAppletFixture(t *testing.T) (*App, *applet.Registry, context.Context) {
	t.Helper()
	a, _ := providerTestApp(t)
	r, err := a.appletRegistry()
	if err != nil {
		t.Fatal(err)
	}
	ctx := applet.WithInvocation(context.Background(), applet.Invocation{Workspace: t.TempDir(), OwnerKind: "chat", OwnerID: "settings-test", ID: "settings-call"})
	return a, r, ctx
}

func readAppletSettingsTest(t *testing.T, r *applet.Registry, ctx context.Context) AppletSettingsSnapshot {
	t.Helper()
	result, err := r.Execute(ctx, "dinah.settings/read@1", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot AppletSettingsSnapshot
	if err := json.Unmarshal([]byte(result.OutputJSON), &snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestSettingsAppletReadAndRevisionUpdate(t *testing.T) {
	a, r, ctx := settingsAppletFixture(t)
	a.settings.Approvals = ApprovalsAsk
	a.settings.ProviderExecutables = map[string]string{"codex": "private-executable"}
	a.settings.Workspace = "private-workspace"
	a.settings.UsagePage = "https://private.example/usage?token=private-token"
	before := readAppletSettingsTest(t, r, ctx)
	encoded, _ := json.Marshal(before)
	if strings.Contains(string(encoded), "private") || strings.Contains(string(encoded), "approvals") || len(before.Revision) != 64 {
		t.Fatalf("unsafe read: %s", encoded)
	}
	var events int
	a.sink = func(name string, data any) {
		if name == "settings:changed" {
			events++
			if m, ok := data.(map[string]any); !ok || m["revision"] == nil || m["preferences"] == nil || m["stateVer"] == nil {
				t.Error("unexpected event shape")
			}
		}
	}
	input, _ := json.Marshal(map[string]any{"revision": before.Revision, "changes": map[string]any{"newWindowMode": "maximized", "defaultProvider": "devin", "showThinking": true}})
	result, err := r.Execute(ctx, "dinah.settings/update@1", string(input))
	if err != nil {
		t.Fatal(err)
	}
	var after AppletSettingsSnapshot
	_ = json.Unmarshal([]byte(result.OutputJSON), &after)
	if after.Revision == before.Revision || after.Preferences.NewWindowMode != "maximized" || after.Preferences.DefaultProvider != "devin" || !after.Preferences.ShowThinking || events != 1 {
		t.Fatalf("incorrect update: %+v events %d", after, events)
	}
	if a.settings.Approvals != ApprovalsAsk || a.settings.ProviderExecutables["codex"] != "private-executable" || a.settings.UsagePage != "https://private.example/usage?token=private-token" {
		t.Fatal("unexposed settings were changed")
	}
	if got := settingsAppletSnapshot(closeBusWhenDone(t, NewApp()).settings); !reflect.DeepEqual(got, after) {
		t.Fatalf("preferences did not persist: %+v", got)
	}
	if _, err := r.Execute(ctx, "dinah.settings/update@1", string(input)); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("stale revision accepted: %v", err)
	}
	if events != 1 {
		t.Fatal("failed update emitted a changed event")
	}
}

// A kind of window with a choice of its own reports it, an agent may change
// it or reset it with null, and the person's setter does the same.
func TestWindowKindModes(t *testing.T) {
	a, r, ctx := settingsAppletFixture(t)
	before := readAppletSettingsTest(t, r, ctx)
	if before.Preferences.WindowModes["editor"] != "maximized" || a.State().WindowModesShipped["editor"] != "maximized" {
		t.Fatalf("the Editor's shipped choice %+v", before.Preferences.WindowModes)
	}
	input := `{"revision":"` + before.Revision + `","changes":{"windowModes":{"editor":"same"}}}`
	if _, err := r.Execute(ctx, "dinah.settings/update@1", input); err != nil {
		t.Fatal(err)
	}
	if got := closeBusWhenDone(t, NewApp()).State(); got.WindowModes["editor"] != "same" || got.WindowModesShipped["editor"] != "maximized" {
		t.Fatalf("after setting %+v", got.WindowModes)
	}
	changed := readAppletSettingsTest(t, r, ctx)
	input = `{"revision":"` + changed.Revision + `","changes":{"windowModes":{"editor":null}}}`
	if _, err := r.Execute(ctx, "dinah.settings/update@1", input); err != nil {
		t.Fatal(err)
	}
	if a.settings.WindowModes != nil || a.State().WindowModes["editor"] != "maximized" {
		t.Fatalf("reset left %+v", a.settings.WindowModes)
	}
	if err := a.SetWindowKindMode("editor", "floating"); err != nil || closeBusWhenDone(t, NewApp()).State().WindowModes["editor"] != "floating" {
		t.Fatalf("setter %v", err)
	}
	for _, bad := range [][2]string{{"editor", "minimized"}, {"chat", "maximized"}, {"chat", ""}} {
		if err := a.SetWindowKindMode(bad[0], bad[1]); err == nil {
			t.Fatalf("accepted %v", bad)
		}
	}
	if err := a.SetWindowKindMode("editor", ""); err != nil || a.settings.WindowModes != nil {
		t.Fatalf("reset %v %+v", err, a.settings.WindowModes)
	}
	// A choice for a kind this version does not list is kept, and an
	// unreadable choice for a listed kind is refused, as an unknown Open New
	// Windows value is.
	stateTestWrite(t, filepath.Join(configDir(), "settings.json"), `{"windowModes":{"editor":"same","later":"maximized"}}`)
	if s, err := loadSettings(); err != nil || s.WindowModes["later"] != "maximized" || effectiveWindowModes(s)["editor"] != "same" {
		t.Fatalf("load %+v %v", s.WindowModes, err)
	}
	stateTestWrite(t, filepath.Join(configDir(), "settings.json"), `{"windowModes":{"editor":"sideways"}}`)
	if _, err := loadSettings(); err == nil {
		t.Fatal("an unreadable window choice was accepted")
	}
}

func TestSettingsAppletSchemaAndContextBoundaries(t *testing.T) {
	_, r, ctx := settingsAppletFixture(t)
	snapshot := readAppletSettingsTest(t, r, ctx)
	for _, changes := range []string{`{}`, `{"approvals":"all"}`, `{"usageURL":"https://example.com"}`, `{"providerExecutables":{}}`, `{"workspace":"C:/elsewhere"}`, `{"newWindowMode":"minimized"}`, `{"defaultProvider":"gemini"}`, `{"showThinking":"yes"}`, `{"showThinking":null}`,
		`{"windowModes":{}}`, `{"windowModes":{"editor":"minimized"}}`, `{"windowModes":{"chat":"maximized"}}`, `{"windowModes":"maximized"}`} {
		input := `{"revision":"` + snapshot.Revision + `","changes":` + changes + `}`
		if _, err := r.Execute(ctx, "dinah.settings/update@1", input); err == nil {
			t.Fatalf("unsafe input accepted: %s", changes)
		}
	}
	for _, input := range []string{`{"path":"settings.json"}`, `{"workspace":"other"}`} {
		if _, err := r.Execute(ctx, "dinah.settings/read@1", input); err == nil {
			t.Fatalf("caller override accepted: %s", input)
		}
	}
	if _, err := r.Execute(context.Background(), "dinah.settings/read@1", `{}`); err == nil {
		t.Fatal("missing invocation accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.Execute(cancelled, "dinah.settings/read@1", `{}`); err == nil {
		t.Fatal("cancelled read accepted")
	}
}

func TestSettingsAppletUIChangeConflictsAndPersistenceFailure(t *testing.T) {
	a, r, ctx := settingsAppletFixture(t)
	before := readAppletSettingsTest(t, r, ctx)
	events := 0
	a.sink = func(name string, _ any) {
		if name == "settings:changed" {
			events++
		}
	}
	if err := a.SetShowThinking(true); err != nil {
		t.Fatal(err)
	}
	input := `{"revision":"` + before.Revision + `","changes":{"newWindowMode":"maximized"}}`
	if _, err := r.Execute(ctx, "dinah.settings/update@1", input); err == nil {
		t.Fatal("applet overwrote newer UI change")
	}
	if err := a.SetNewWindowMode("maximized"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetDefaultProvider("devin"); err != nil {
		t.Fatal(err)
	}
	if events != 3 {
		t.Fatalf("UI setters emitted %d events", events)
	}
	current := readAppletSettingsTest(t, r, ctx)
	// An invalid pending record makes persistence fail without changing memory.
	stateTestWrite(t, filepath.Join(configDir(), "settings.json.pending"), "broken")
	input = `{"revision":"` + current.Revision + `","changes":{"showThinking":false}}`
	if _, err := r.Execute(ctx, "dinah.settings/update@1", input); err == nil {
		t.Fatal("failed persistence reported success")
	}
	if !reflect.DeepEqual(readAppletSettingsTest(t, r, ctx), current) || events != 3 {
		t.Fatal("failed persistence changed memory or emitted an event")
	}
	if err := a.SetShowThinking(false); err == nil || !a.settings.ShowThinking {
		t.Fatal("UI setter changed memory after failed persistence")
	}
	if err := a.SetDefaultProvider("codex"); err == nil || a.settings.DefaultProvider != "devin" {
		t.Fatal("provider setter changed memory after failed persistence")
	}
	if err := a.SetNewWindowMode("floating"); err == nil || a.settings.NewWindowMode != "maximized" {
		t.Fatal("window setter changed memory after failed persistence")
	}
}

func TestSettingsAppletOwnersAndOnceRecovery(t *testing.T) {
	a, r, _ := settingsAppletFixture(t)
	ws := t.TempDir()
	// The person has allowed the update always here, so a chat's update runs
	// without asking (applet_consent.go has its own test).
	a.settings.AppletGrants = map[string][]string{ws: {"dinah.settings/update@1"}}
	for _, owner := range []string{"chat", "applet"} {
		v, err := a.startAppletInvocation(ws, owner, "settings-owner", "dinah.settings/read@1", `{}`, "", time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		v = awaitApplet(t, a, ws, owner, "settings-owner", v.ID, "done")
		if !strings.Contains(v.OutputJSON, "newWindowMode") {
			t.Fatal("settings not callable through shared owner interfaces")
		}
		var read AppletSettingsSnapshot
		if err := json.Unmarshal([]byte(v.OutputJSON), &read); err != nil {
			t.Fatal(err)
		}
		input, _ := json.Marshal(map[string]any{"revision": read.Revision, "changes": map[string]any{"showThinking": !read.Preferences.ShowThinking}})
		updated, err := a.startAppletInvocation(ws, owner, "settings-owner", "dinah.settings/update@1", string(input), "", time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		awaitApplet(t, a, ws, owner, "settings-owner", updated.ID, "done")
		stale, err := a.startAppletInvocation(ws, owner, "settings-owner", "dinah.settings/update@1", string(input), "", time.Now().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		failed := awaitApplet(t, a, ws, owner, "settings-owner", stale.ID, "failed")
		if !strings.Contains(failed.Error, "changed") {
			t.Fatalf("host did not report revision conflict: %+v", failed)
		}
	}
	if err := a.SetShowThinking(false); err != nil {
		t.Fatal(err)
	}
	action, err := r.Describe("dinah.settings/update@1")
	if err != nil || action.Replay != "once" || len(action.Capabilities) != 1 || action.Capabilities[0] != "settings.write" {
		t.Fatalf("incorrect write policy: %+v %v", action, err)
	}
	digest, _ := r.Digest(action.Ref)
	snapshot := settingsAppletSnapshot(a.settings)
	input := `{"revision":"` + snapshot.Revision + `","changes":{"showThinking":true}}`
	v := AppletInvocation{ID: "interrupted-settings", Workspace: ws, OwnerKind: "chat", OwnerID: "settings-owner", Action: action.Ref, Digest: digest, InputJSON: input, Status: "running", Deadline: time.Now().Add(time.Hour), Created: time.Now()}
	if err := saveAppletInvocation(&v); err != nil {
		t.Fatal(err)
	}
	got, err := closeBusWhenDone(t, NewApp()).AppletInvocationAt(ws, v.ID)
	if err != nil || got.Status != "failed" {
		t.Fatalf("interrupted write replayed: %+v %v", got, err)
	}
	if closeBusWhenDone(t, NewApp()).settings.ShowThinking {
		t.Fatal("recovery changed preferences")
	}
	if _, err := os.Stat(filepath.Join(configDir(), "settings.json")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestSettingsAppletDigestCoversItsImplementation(t *testing.T) {
	a := closeBusWhenDone(t, NewApp())
	bundle, err := a.settingsAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"host/applet_settings.go", "host/settings.go", "host/ui_state.go", "host/providers.go", "host/options.go", "host/state_file.go", "host/state_directory.go"} {
		if len(bundle.Sources[path]) == 0 {
			t.Fatalf("missing implementation dependency %s", path)
		}
	}
}
