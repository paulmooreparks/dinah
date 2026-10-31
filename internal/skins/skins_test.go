package skins

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

const catalogue = `{"version":1,"skins":[
 {"id":"standard","label":"Standard","stylesheet":null,"admitted":true},
 {"id":"alpha","label":"Alpha","stylesheet":"pudl-skin-alpha.css","sha256":"0000000000000000000000000000000000000000000000000000000000000000","admitted":true},
 {"id":"beta","label":"Beta","stylesheet":"pudl-skin-beta.css","sha256":"11","admitted":false}]}`

func TestTheCatalogueOffersOnlyAdmittedSkinsAndFallsBackToStandard(t *testing.T) {
	c, err := Parse([]byte(catalogue))
	if err != nil {
		t.Fatal(err)
	}
	got := c.Choices()
	if len(got) != 2 || got[0] != (Choice{"standard", "Standard"}) || got[1] != (Choice{"alpha", "Alpha"}) {
		t.Fatalf("the choices: %+v", got)
	}
	for pref, want := range map[string]string{"alpha": "alpha", "standard": "standard", "beta": "standard", "gone": "standard", "": "standard"} {
		if got := c.Effective(pref); got != want {
			t.Errorf("the skin drawn for %q is %q, want %q", pref, got, want)
		}
	}
}

func TestCatalogueRejectsUnusableAdmissionMetadata(t *testing.T) {
	for name, change := range map[string]func(*Catalogue){
		"unsupported version":   func(c *Catalogue) { c.Version = 2 },
		"missing version":       func(c *Catalogue) { c.Version = 0 },
		"empty label":           func(c *Catalogue) { c.Skins[1].Label = " " },
		"standard not admitted": func(c *Catalogue) { c.Skins[0].Admitted = false },
		"standard stylesheet":   func(c *Catalogue) { c.Skins[0].Stylesheet = c.Skins[1].Stylesheet },
		"missing stylesheet":    func(c *Catalogue) { c.Skins[1].Stylesheet = nil },
		"stylesheet traversal":  func(c *Catalogue) { s := "../alpha.css"; c.Skins[1].Stylesheet = &s },
		"absolute stylesheet":   func(c *Catalogue) { s := "/alpha.css"; c.Skins[1].Stylesheet = &s },
		"windows path":          func(c *Catalogue) { s := `..\alpha.css`; c.Skins[1].Stylesheet = &s },
		"non CSS":               func(c *Catalogue) { s := "alpha.js"; c.Skins[1].Stylesheet = &s },
		"missing digest":        func(c *Catalogue) { c.Skins[1].SHA256 = "" },
		"short digest":          func(c *Catalogue) { c.Skins[1].SHA256 = "00" },
		"invalid digest":        func(c *Catalogue) { c.Skins[1].SHA256 = strings.Repeat("z", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			c, err := Parse([]byte(catalogue))
			if err != nil {
				t.Fatal(err)
			}
			change(&c)
			data, _ := json.Marshal(c)
			if _, err := Parse(data); err == nil {
				t.Fatal("accepted unusable catalogue")
			}
		})
	}
}

func TestAdmissionRequiresTheBundledStylesheetDigest(t *testing.T) {
	c, err := Parse([]byte(catalogue))
	if err != nil {
		t.Fatal(err)
	}
	css := []byte(":root:where([data-skin=alpha]) { --entry-shadow: none; }")
	sum := sha256.Sum256(css)
	c.Skins[1].SHA256 = hex.EncodeToString(sum[:])
	files := fstest.MapFS{"pudl-skin-alpha.css": &fstest.MapFile{Data: css}}
	if err := c.ValidateAssets(files); err != nil {
		t.Fatal(err)
	}
	// Unadmitted candidates do not become required assets or effective choices.
	if c.Effective("beta") != Standard {
		t.Fatal("unadmitted skin became effective")
	}
	files["pudl-skin-alpha.css"].Data = []byte("changed compiler output")
	if err := c.ValidateAssets(files); err == nil {
		t.Fatal("accepted changed admitted CSS")
	}
	delete(files, "pudl-skin-alpha.css")
	if err := c.ValidateAssets(files); err == nil {
		t.Fatal("accepted missing admitted CSS")
	}
}

func TestACatalogueWithoutStandardOrWithARepeatedIDIsRefused(t *testing.T) {
	for _, bad := range []string{
		`{"version":1,"skins":[{"id":"alpha","label":"A","admitted":false}]}`,
		`{"version":1,"skins":[{"id":"standard","label":"S","admitted":true},{"id":"standard","label":"S","admitted":true}]}`,
		`{"version":1,"skins":[{"id":"Standard!","label":"S","admitted":true}]}`,
		`not json`,
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestThePreferencesAreReadFromTheStoresAndNeverFailAPage(t *testing.T) {
	dir := t.TempDir()
	if got := ReadPreference(dir); got != Standard {
		t.Fatalf("with no store: %q", got)
	}
	if ReadPreference("") != Standard || ReadThemePreference("") != "" {
		t.Fatal("an unavailable home must not read relative preference files")
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("appearance.json", `{"version":1,"active":"mineral","skin":"alpha","themes":[],"overrides":{}}`)
	if got := ReadPreference(dir); got != "alpha" {
		t.Fatalf("a chosen skin: %q", got)
	}
	// A skin the build lacks is kept as the preference; the catalogue decides what is drawn.
	write("appearance.json", `{"skin":"future-skin"}`)
	if got := ReadPreference(dir); got != "future-skin" {
		t.Fatalf("a skin from a later build: %q", got)
	}
	write("appearance.json", `{"skin":"Bad Skin!"}`)
	if got := ReadPreference(dir); got != Standard {
		t.Fatalf("an invalid skin: %q", got)
	}
	write("appearance.json", `{broken`)
	if got := ReadPreference(dir); got != Standard {
		t.Fatalf("a broken store: %q", got)
	}
	write("ui-state.json", `{"version":1,"values":{"pudl-theme":"dark"}}`)
	if got := ReadThemePreference(dir); got != "dark" {
		t.Fatalf("a chosen theme mode: %q", got)
	}
	write("ui-state.json", `{"version":1,"values":{"pudl-theme":" dark "}}`)
	if got := ReadThemePreference(dir); got != "dark" {
		t.Fatalf("normalized mode: %q", got)
	}
	write("ui-state.json", `{"version":1,"values":{"pudl-theme":"purple"}}`)
	if got := ReadThemePreference(dir); got != "" {
		t.Fatalf("an unknown theme mode: %q", got)
	}
	if got := ReadThemePreference(t.TempDir()); got != "" {
		t.Fatalf("with no store: %q", got)
	}
}
