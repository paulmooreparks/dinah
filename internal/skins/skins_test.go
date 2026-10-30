package skins

import (
	"os"
	"path/filepath"
	"testing"
)

const catalogue = `{"version":1,"skins":[
 {"id":"standard","label":"Standard","stylesheet":null,"admitted":true},
 {"id":"alpha","label":"Alpha","stylesheet":"pudl-skin-alpha.css","sha256":"00","admitted":true},
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

func TestACatalogueWithoutStandardOrWithARepeatedIDIsRefused(t *testing.T) {
	for _, bad := range []string{
		`{"skins":[{"id":"alpha","label":"A","admitted":true}]}`,
		`{"skins":[{"id":"standard","label":"S","admitted":true},{"id":"standard","label":"S","admitted":true}]}`,
		`{"skins":[{"id":"Standard!","label":"S","admitted":true}]}`,
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
	write("ui-state.json", `{"version":1,"values":{"pudl-theme":"purple"}}`)
	if got := ReadThemePreference(dir); got != "" {
		t.Fatalf("an unknown theme mode: %q", got)
	}
	if got := ReadThemePreference(t.TempDir()); got != "" {
		t.Fatalf("with no store: %q", got)
	}
}
