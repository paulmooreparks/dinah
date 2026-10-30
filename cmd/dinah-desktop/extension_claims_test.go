package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	extension "github.com/paulmooreparks/dinah-extension-go"
)

// claimsTestApp is linkTestApp with an extension store of its own, whose
// claims are forgotten when the test ends so that no other test sees them.
func claimsTestApp(t *testing.T) (*App, WorkspaceRecord, string, string) {
	t.Helper()
	a, record, dir, elsewhere := linkTestApp(t)
	manager := a.extensionManager()
	t.Cleanup(func() {
		manager.Close()
		mediaClaims.Lock()
		mediaClaims.extensions, mediaClaims.extensionsLoaded = nil, false
		mediaClaims.Unlock()
	})
	for _, name := range []string{"change.diff", "fix.patch", "notes.mdx", "model.glb"} {
		os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644)
	}
	os.WriteFile(filepath.Join(elsewhere, "loose.diff"), []byte("x\n"), 0o644)
	return a, record, dir, elsewhere
}

// claimantKeys are the publisher keys of the test extensions, one per ID, so
// that an update is signed by the publisher of the version it replaces.
var claimantKeys = map[string]ed25519.PrivateKey{}

// installClaimant signs and installs an extension whose one window view
// declares these file types, through the same entry point Settings uses.
func installClaimant(t *testing.T, a *App, id, title, version, glyph string, types ...extension.FileType) {
	t.Helper()
	folder := t.TempDir()
	m := extension.Manifest{ID: id, Version: version, Contract: 1, Title: title, Publisher: extension.Publisher{Name: "Test"}, Backends: map[string]string{"windows": "backend.exe", "linux": "backend.exe", "darwin": "backend.exe"}, Permissions: []string{"workspace.read"},
		Actions: []extension.Action{{Name: "inspect", Title: "Inspect", Replay: "pure", InputSchema: json.RawMessage(`{"type":"object"}`), OutputSchema: json.RawMessage(`{"type":"object"}`)}},
		Views:   []extension.View{{ID: "viewer", Title: title + " Viewer", Glyph: glyph, Page: "index.html", Opens: types}, {ID: "pane", Title: title + " Pane", Page: "index.html", Pane: true}}}
	data, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(folder, extension.ManifestFile), data, 0o600)
	os.WriteFile(filepath.Join(folder, "backend.exe"), []byte("fixture"), 0o600)
	os.WriteFile(filepath.Join(folder, "index.html"), []byte("<p>viewer</p>"), 0o600)
	key := claimantKeys[id]
	if key == nil {
		_, key, _ = ed25519.GenerateKey(rand.Reader)
		claimantKeys[id] = key
	}
	signed, err := extension.Sign(folder, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.InstallExtension(folder, extension.Approval{Fingerprint: extension.Fingerprint(signed.Publisher.Key), Signature: signed.Signature}); err != nil {
		t.Fatal(err)
	}
	// Enabling is ordered by time; the next installation must come later.
	time.Sleep(30 * time.Millisecond)
}

var diffType = extension.FileType{MediaType: "text/x-diff", Extensions: []string{".diff", ".patch"}}

// A link to a file of a type an enabled extension declares opens the
// extension's view, in the file's workspace, with the file in its state.
func TestExtensionClaimOpensTheViewWithTheFile(t *testing.T) {
	a, record, dir, elsewhere := claimsTestApp(t)
	installClaimant(t, a, "test.patches", "Patches", "0.1.0", "git-branch", diffType)
	l, _ := a.ResolveDocumentLink(dir, "change.diff:12")
	if !l.Found || l.Opens != opensExtension || l.Extension != "test.patches" || l.ExtensionView != "viewer" || l.Label != "Open in Patches Viewer" || l.Glyph != "git-branch" || l.MediaType != "text/x-diff" || l.Line != 12 {
		t.Fatalf("the claim was not used: %+v", l)
	}
	file, ok := extension.ParseOpenState(l.State)
	if !ok || file.Path != filepath.Join(dir, "change.diff") || file.MediaType != "text/x-diff" || file.Line != 12 || file.Resource.WorkspaceID != record.ID || file.Resource.Path != "change.diff" || file.Resource.SourceID == "" {
		t.Fatalf("the view's state is wrong: %s", l.State)
	}
	if l, _ := a.ResolveDocumentLink(dir, "fix.patch"); l.Opens != opensExtension || l.Line != 0 || strings.Contains(l.State, `"line"`) {
		t.Errorf("the second extension of the type did not open in the view: %+v", l)
	}
	// A file outside every workspace source is left to the system, since
	// the view could not read it.
	if l, _ := a.ResolveDocumentLink(dir, filepath.Join(elsewhere, "loose.diff")); l.Opens != opensReveal || l.Extension != "" {
		t.Errorf("a file outside the workspaces went to the extension: %+v", l)
	}
}

// A view with no glyph gets the generic one.
func TestExtensionClaimWithoutAGlyph(t *testing.T) {
	a, _, dir, _ := claimsTestApp(t)
	installClaimant(t, a, "test.patches", "Patches", "0.1.0", "", diffType)
	if l, _ := a.ResolveDocumentLink(dir, "change.diff"); l.Opens != opensExtension || l.Glyph != "app" {
		t.Errorf("%+v", l)
	}
}

// Dinah's own parts come first: an extension never takes pictures from the
// media viewer, or the Editor's kinds of text from the Editor, and never
// changes the media type of a file extension Dinah knows. A type Dinah
// leaves to the system, such as PDF, is the extension's.
func TestExtensionClaimsComeAfterDinahsOwn(t *testing.T) {
	a, _, dir, _ := claimsTestApp(t)
	installClaimant(t, a, "test.greedy", "Greedy", "0.1.0", "",
		extension.FileType{MediaType: "image/svg+xml", Extensions: []string{".svg"}},
		extension.FileType{MediaType: "text/markdown", Extensions: []string{".md"}},
		extension.FileType{MediaType: "application/pdf", Extensions: []string{".pdf"}},
		extension.FileType{MediaType: "text/x-sheet", Extensions: []string{".xlsx"}},
		extension.FileType{MediaType: "model/gltf-binary", Extensions: []string{".glb"}})
	for ref, opens := range map[string]string{"drawing.svg": opensMedia, "picture.png": opensMedia, "plan.md": opensEditor, "main.go": opensEditor, "report.pdf": opensExtension, "sheet.xlsx": opensSystem, "model.glb": opensExtension} {
		if l, _ := a.ResolveDocumentLink(dir, ref); l.Opens != opens {
			t.Errorf("%s opened in %q: %+v", ref, l.Opens, l)
		}
	}
	list, err := a.Extensions()
	if err != nil || len(list) != 1 || len(list[0].FileTypes) != 5 {
		t.Fatalf("%+v %v", list, err)
	}
	elsewhere := map[string]string{}
	for _, ft := range list[0].FileTypes {
		for _, e := range ft.Elsewhere {
			elsewhere[e.FileExtension] = e.Why + ":" + e.By
		}
	}
	if elsewhere[".svg"] != "dinah:the Media Viewer" || elsewhere[".md"] != "dinah:the Editor" || elsewhere[".markdown"] != "dinah:the Editor" || elsewhere[".xlsx"] != "type:"+mediaTypes[".xlsx"] || elsewhere[".pdf"] != "" || elsewhere[".glb"] != "" {
		t.Errorf("Settings shows the wrong places: %v", elsewhere)
	}
}

// Two extensions that declare the same type: the one enabled first opens
// it, and Settings shows the other where its files open instead. Disabling
// the first gives the type to the second, and enabling it again puts it
// behind the second. Removing or updating an extension removes its claims.
func TestExtensionClaimPrecedenceAndLifecycle(t *testing.T) {
	a, _, dir, _ := claimsTestApp(t)
	installClaimant(t, a, "test.zeta", "Zeta", "0.1.0", "", diffType)
	installClaimant(t, a, "test.alpha", "Alpha", "0.1.0", "", extension.FileType{MediaType: "text/x-patch", Extensions: []string{".patch"}}, extension.FileType{MediaType: "text/x-diff", Extensions: []string{".diff"}})
	opens := func(ref string) string {
		l, _ := a.ResolveDocumentLink(dir, ref)
		if l.Opens != opensExtension {
			return l.Opens
		}
		return l.Extension
	}
	if opens("change.diff") != "test.zeta" || opens("fix.patch") != "test.zeta" {
		t.Fatalf("the extension enabled first did not win: %s %s", opens("change.diff"), opens("fix.patch"))
	}
	list, _ := a.Extensions()
	var alpha ExtensionInfo
	for _, v := range list {
		if v.Manifest.ID == "test.alpha" {
			alpha = v
		}
	}
	conflicts := 0
	for _, ft := range alpha.FileTypes {
		for _, e := range ft.Elsewhere {
			if e.By != "Zeta" || e.Why != "earlier" {
				t.Errorf("unexpected place: %+v", e)
			}
			conflicts++
		}
	}
	if conflicts != 2 {
		t.Errorf("Settings does not show both of Alpha's file extensions opening in Zeta: %+v", alpha.FileTypes)
	}

	if err := a.EnableExtension("test.zeta", false); err != nil {
		t.Fatal(err)
	}
	if opens("change.diff") != "test.alpha" || opens("fix.patch") != "test.alpha" {
		t.Fatalf("disabling did not unregister: %s %s", opens("change.diff"), opens("fix.patch"))
	}
	time.Sleep(30 * time.Millisecond)
	if err := a.EnableExtension("test.zeta", true); err != nil {
		t.Fatal(err)
	}
	if opens("change.diff") != "test.alpha" {
		t.Fatal("enabling again took the type back from the extension enabled before it")
	}

	// An update keeps Alpha's place; one that no longer declares the type
	// gives it back to Zeta.
	installClaimant(t, a, "test.alpha", "Alpha", "0.2.0", "", extension.FileType{MediaType: "text/x-patch", Extensions: []string{".patch"}})
	if opens("change.diff") != "test.zeta" || opens("fix.patch") != "test.alpha" {
		t.Fatalf("an update did not replace the claims: %s %s", opens("change.diff"), opens("fix.patch"))
	}
	if err := a.RemoveExtension("test.alpha"); err != nil {
		t.Fatal(err)
	}
	if opens("fix.patch") != "test.zeta" {
		t.Fatal("removing did not unregister")
	}
	if err := a.RemoveExtension("test.zeta"); err != nil {
		t.Fatal(err)
	}
	if got := opens("change.diff"); got != opensReveal {
		t.Fatalf("with no extension a diff opens in %q", got)
	}
}

// The claims are read from the store the first time a link needs them, so
// an extension installed before Dinah started opens its files.
func TestExtensionClaimsLoadWhenFirstNeeded(t *testing.T) {
	a, _, dir, _ := claimsTestApp(t)
	installClaimant(t, a, "test.patches", "Patches", "0.1.0", "", diffType)
	mediaClaims.Lock()
	mediaClaims.extensions, mediaClaims.extensionsLoaded = nil, false
	mediaClaims.Unlock()
	if l, _ := a.ResolveDocumentLink(dir, "change.diff"); l.Opens != opensExtension {
		t.Fatalf("%+v", l)
	}
}

// A damaged package claims nothing.
func TestDamagedExtensionClaimsNothing(t *testing.T) {
	a, _, dir, _ := claimsTestApp(t)
	installClaimant(t, a, "test.patches", "Patches", "0.1.0", "", diffType)
	list, _ := a.extensionManager().Store.List()
	os.WriteFile(filepath.Join(a.extensionManager().Store.Path(list[0].Manifest), "index.html"), []byte("changed"), 0o600)
	a.syncExtensionClaims()
	if l, _ := a.ResolveDocumentLink(dir, "change.diff"); l.Opens == opensExtension {
		t.Fatalf("a damaged package opened a file: %+v", l)
	}
}
