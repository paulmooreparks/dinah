package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/skins"
)

// The skin is one choice for the whole application, kept in the appearance store beside the colour theme. A skin the
// build lacks is kept as the choice and Standard is drawn; the skins offered come from the release's catalogue.
func TestTheSkinIsKeptInTheAppearanceStoreAndTheCatalogueDecidesWhatIsDrawn(t *testing.T) {
	a, _ := providerTestApp(t)
	var events []map[string]any
	a.sink = func(name string, data any) {
		if name == "appearance:changed" {
			events = append(events, data.(map[string]any))
		}
	}
	snapshot, err := a.ReadAppearance()
	if err != nil || snapshot.State.Skin != skins.Standard || snapshot.Effective != skins.Standard {
		t.Fatalf("the default: %+v %v", snapshot, err)
	}
	if len(snapshot.Skins) == 0 || snapshot.Skins[0].ID != skins.Standard {
		t.Fatalf("the skins offered: %+v", snapshot.Skins)
	}
	// Choosing an admitted skin draws it, and tells the other windows.
	chosen := snapshot.Skins[len(snapshot.Skins)-1].ID
	snapshot.State.Skin = chosen
	saved, err := a.SaveAppearance(snapshot.State, snapshot.Revision)
	if err != nil || saved.State.Skin != chosen || saved.Effective != chosen {
		t.Fatalf("choosing %s: %+v %v", chosen, saved, err)
	}
	if len(events) != 1 || events[0]["skin"] != chosen || events[0]["revision"] != saved.Revision {
		t.Fatalf("the change was announced as %v", events)
	}
	// A page that does not say which skin keeps the stored one.
	next := saved.State
	next.Skin = ""
	next.Active = "linen"
	kept, err := a.SaveAppearance(next, saved.Revision)
	if err != nil || kept.State.Skin != chosen || kept.State.Active != "linen" {
		t.Fatalf("a save without a skin: %+v %v", kept, err)
	}
	// A skin this build lacks is kept as the choice, and Standard is what is drawn.
	later := kept.State
	later.Skin = "a-later-skin"
	stored, err := a.SaveAppearance(later, kept.Revision)
	if err != nil || stored.State.Skin != "a-later-skin" || stored.Effective != skins.Standard {
		t.Fatalf("a skin from a later build: %+v %v", stored, err)
	}
	read, _ := a.ReadAppearance()
	if read.State.Skin != "a-later-skin" || read.Effective != skins.Standard {
		t.Fatalf("read back: %+v", read)
	}
	// A value that cannot be a skin's ID is refused and changes nothing.
	bad := read.State
	bad.Skin = "Not A Skin!"
	if _, err := a.SaveAppearance(bad, read.Revision); err == nil {
		t.Fatal("an invalid skin was saved")
	}
	if again, _ := a.ReadAppearance(); again.State.Skin != "a-later-skin" {
		t.Fatalf("a refused save changed the skin to %q", again.State.Skin)
	}
}

// A library saved before skins has the default skin.
func TestALibrarySavedBeforeSkinsHasTheDefaultSkin(t *testing.T) {
	state, _, err := decodeAppearance([]byte(`{"version":1,"active":"mineral","themes":[],"overrides":{}}`))
	if err != nil || state.Skin != skins.Standard {
		t.Fatalf("%+v %v", state, err)
	}
}

// The window and the bus's page take the same release, so their catalogues and every skin stylesheet in them agree.
func TestBothPUDLFoldersHoldTheSameSkins(t *testing.T) {
	window := filepath.Join("frontend", "pudl")
	web := filepath.Join("..", "..", "internal", "web", "static", "pudl")
	files := []string{"pudl-skins.json", "pudl-skins.js"}
	for _, s := range skinCatalogue.Skins {
		if s.Stylesheet != nil {
			files = append(files, *s.Stylesheet)
		}
	}
	for _, name := range files {
		a, errA := os.ReadFile(filepath.Join(window, name))
		b, errB := os.ReadFile(filepath.Join(web, name))
		if errA != nil || errB != nil || !bytes.Equal(a, b) {
			t.Errorf("%s differs between the window's PUDL folder and the web page's (%v, %v)", name, errA, errB)
		}
	}
	// Each stylesheet is the one the catalogue promises.
	for _, s := range skinCatalogue.Skins {
		if s.Stylesheet == nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(window, *s.Stylesheet))
		sum := sha256.Sum256(data)
		if err != nil || hex.EncodeToString(sum[:]) != s.SHA256 {
			t.Errorf("%s does not match its catalogue checksum (%v)", *s.Stylesheet, err)
		}
	}
}
