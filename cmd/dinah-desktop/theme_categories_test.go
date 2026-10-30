package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// Every built-in palette, eleven themes in two forms, sets all eight category
// colours and keeps every rule of PUDL's chapter 3.
func TestEveryBuiltinPaletteKeepsTheCategoryRules(t *testing.T) {
	palettes := 0
	for id, doc := range builtinThemes() {
		for _, mode := range []string{"light", "dark"} {
			p := doc.Palettes[mode]
			for _, key := range themeCategoryKeys {
				if !themeHex.MatchString(p[key]) {
					t.Fatalf("%s %s does not set %s", id, mode, key)
				}
			}
			if failures := themePaletteCategoryErrors(mode, p); len(failures) > 0 {
				t.Errorf("%s: %v", id, failures)
			}
			palettes++
		}
	}
	if palettes != 22 {
		t.Fatalf("checked %d palettes, want 22", palettes)
	}
}

// The checks refuse a category colour that breaks each rule in turn.
func TestCategoryRulesRefuseEachBreach(t *testing.T) {
	for name, change := range map[string]func(map[string]string){
		"needs 3:1 contrast":       func(p map[string]string) { p["category-1"] = p["surface"] },
		"is too close to warn":     func(p map[string]string) { p["category-3"] = p["warn"] },
		"is too close to danger":   func(p map[string]string) { p["category-5"] = p["danger"] },
		"is too close to accent":   func(p map[string]string) { p["category-2"] = p["accent"] },
		"are too alike":            func(p map[string]string) { p["category-7"] = p["category-6"] },
		"is too close to positive": func(p map[string]string) { p["category-8"] = p["positive"] },
	} {
		doc := builtinThemes()["mineral"]
		change(doc.Palettes["light"])
		if failures := themeContrastErrors(doc); !strings.Contains(strings.Join(failures, "; "), name) {
			t.Errorf("%s: got %v", name, failures)
		}
	}
}

func withoutCategories(doc ThemeDocument) ThemeDocument {
	for _, p := range doc.Palettes {
		for _, key := range themeCategoryKeys {
			delete(p, key)
		}
	}
	return doc
}

// A theme saved before category colours takes those of the nearest built-in
// palette, so a copy of Nord gets Nord's, and a theme with other surfaces
// gets colours that keep the rules.
func TestThemesSavedBeforeCategoryColoursAreFilled(t *testing.T) {
	nord := withoutCategories(builtinThemes()["nord"])
	if !fillThemeCategories(&nord) {
		t.Fatal("nothing was filled")
	}
	for _, mode := range []string{"light", "dark"} {
		for _, key := range themeCategoryKeys {
			if nord.Palettes[mode][key] != builtinThemes()["nord"].Palettes[mode][key] {
				t.Fatalf("%s %s: got %s", mode, key, nord.Palettes[mode][key])
			}
		}
	}
	odd := withoutCategories(builtinThemes()["mineral"])
	odd.Palettes["light"]["bg"], odd.Palettes["light"]["surface"] = "#f7f2fa", "#ffffff"
	odd.Palettes["dark"]["bg"], odd.Palettes["dark"]["surface"] = "#000000", "#101418"
	fillThemeCategories(&odd)
	if err := validateTheme(odd); err != nil {
		t.Fatal(err)
	}
	if failures := themeCategoryErrors(odd); len(failures) > 0 {
		t.Fatalf("filled colours break the rules: %v", failures)
	}
	partial := builtinThemes()["mineral"]
	delete(partial.Palettes["dark"], "category-4")
	if fillThemeCategories(&partial) || validateTheme(partial) == nil {
		t.Fatal("a palette with seven category colours was filled or accepted")
	}
}

// A stored library whose theme in use predates category colours still loads
// with that theme in use, and saving it writes the filled colours.
func TestStoredThemeWithoutCategoryColoursStaysInUse(t *testing.T) {
	a, _ := providerTestApp(t)
	old := withoutCategories(builtinThemes()["gruvbox"])
	old.Name = "My Gruvbox"
	data, _ := json.Marshal(AppearanceState{Version: 1, Active: "theme-old", Themes: []ThemeRecord{{ID: "theme-old", Document: old}}, Overrides: map[string]ContentAppearance{}})
	stateTestWrite(t, filepath.Join(configDir(), "appearance.json"), string(data))
	snapshot, err := a.ReadAppearance()
	if err != nil || snapshot.State.Active != "theme-old" || snapshot.Notice != "" {
		t.Fatalf("read: %v %q %q", err, snapshot.State.Active, snapshot.Notice)
	}
	if got := snapshot.State.Themes[0].Document.Palettes["dark"]["category-1"]; got != builtinThemes()["gruvbox"].Palettes["dark"]["category-1"] {
		t.Fatalf("dark category-1 is %s", got)
	}
	if _, err := a.SaveAppearance(snapshot.State, snapshot.Revision); err != nil {
		t.Fatal(err)
	}
	again, err := a.ReadAppearance()
	if err != nil || again.State.Themes[0].Document.Palettes["light"]["category-8"] == "" {
		t.Fatalf("saved library lacks category colours: %v", err)
	}
}

// When the colours chosen for an old theme in use cannot keep the rules, the
// default theme is used instead and the notice says why, rather than the
// library being refused.
func TestStoredThemeWhoseFilledColoursFailFallsBackWithANotice(t *testing.T) {
	a, _ := providerTestApp(t)
	old := withoutCategories(builtinThemes()["mineral"])
	old.Name = "Grey Study"
	p := old.Palettes["light"]
	for _, key := range []string{"bg", "surface", "surface-alt", "light", "tb-bg"} {
		p[key] = "#a0a0a0"
	}
	for key := range p {
		if key == "text" || key == "text-muted" || key == "dinah-skill" || key == "dinah-title" || key == "tb-fg" || key == "border" || strings.HasPrefix(key, "syntax-") {
			p[key] = "#000000"
		}
	}
	p["accent"], p["accent-hover"], p["on-accent"], p["on-warn"] = "#101830", "#0c1224", "#ffffff", "#ffffff"
	p["warn"], p["danger"], p["positive"], p["danger-hover"] = "#2c2a00", "#300018", "#1c0c34", "#300018"
	// The fixture breaks only category rules once filled, so the fallback is
	// for the category colours alone.
	filled := withoutCategories(builtinThemes()["mineral"])
	filled.Palettes["light"] = map[string]string{}
	for key, value := range p {
		filled.Palettes["light"][key] = value
	}
	fillThemeCategories(&filled)
	all, categories := themeContrastErrors(filled), themeCategoryErrors(filled)
	if len(categories) == 0 || len(all) != len(categories) {
		t.Fatalf("fixture: %v", all)
	}
	data, _ := json.Marshal(AppearanceState{Version: 1, Active: "theme-grey", Themes: []ThemeRecord{{ID: "theme-grey", Document: old}}, Overrides: map[string]ContentAppearance{}})
	stateTestWrite(t, filepath.Join(configDir(), "appearance.json"), string(data))
	snapshot, err := a.ReadAppearance()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State.Active != product.DefaultTheme || !strings.Contains(snapshot.Notice, "Grey Study") || !strings.Contains(snapshot.Notice, "Theme Editor") {
		t.Fatalf("active %q, notice %q", snapshot.State.Active, snapshot.Notice)
	}
	if len(snapshot.State.Themes) != 1 {
		t.Fatal("the old theme was dropped")
	}
}
