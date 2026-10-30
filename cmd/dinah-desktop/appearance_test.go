package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAppearanceBuiltinsAndDurableSelection(t *testing.T) {
	a, _ := providerTestApp(t)
	for name, doc := range builtinThemes() {
		if err := validateTheme(doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if failures := themeContrastErrors(doc); len(failures) > 0 {
			t.Fatalf("%s: %v", name, failures)
		}
	}
	snapshot, err := a.ReadAppearance()
	if err != nil || snapshot.State.Active != "mineral" {
		t.Fatalf("initial state: %+v %v", snapshot, err)
	}
	snapshot.State.Active = "linen"
	snapshot.State.Overrides["terminal"] = ContentAppearance{Size: 18}
	saved, err := a.SaveAppearance(snapshot.State, snapshot.Revision)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := a.ReadAppearance()
	if err != nil || restored.State.Active != "linen" || restored.State.Overrides["terminal"].Size != 18 {
		t.Fatalf("round trip: %+v %v", restored, err)
	}
	if _, err := a.SaveAppearance(snapshot.State, snapshot.Revision); err == nil {
		t.Fatal("stale revision overwrote preferences")
	}
	if saved.Revision == snapshot.Revision {
		t.Fatal("save did not change revision")
	}
	// Every built-in theme can be chosen and saved, not only the first two
	// (Solarized, Nord, Gruvbox and Graphite were refused as "active theme
	// does not exist" on 27 October 2026), and a person's own theme cannot
	// take a built-in theme's ID.
	current := saved
	for id := range builtinThemes() {
		next := current.State
		next.Active = id
		if current, err = a.SaveAppearance(next, current.Revision); err != nil {
			t.Fatalf("choosing the built-in theme %s: %v", id, err)
		}
	}
	clash := current.State
	clash.Themes = []ThemeRecord{{ID: "solarized", Document: builtinThemes()["mineral"]}}
	if _, err := a.SaveAppearance(clash, current.Revision); err == nil {
		t.Fatal("a person's theme took a built-in theme's ID")
	}
}

func TestAppearanceDraftAndApplyValidation(t *testing.T) {
	a, _ := providerTestApp(t)
	snapshot, _ := a.ReadAppearance()
	doc := builtinThemes()["mineral"]
	doc.Name = "Unreadable draft"
	doc.Palettes["light"]["text"] = "#eef2f3"
	snapshot.State.Themes = append(snapshot.State.Themes, ThemeRecord{ID: "personal", Document: doc})
	saved, err := a.SaveAppearance(snapshot.State, snapshot.Revision)
	if err != nil {
		t.Fatalf("inactive draft should save: %v", err)
	}
	snapshot = saved
	snapshot.State.Active = "personal"
	if _, err := a.SaveAppearance(snapshot.State, snapshot.Revision); err == nil {
		t.Fatal("unreadable theme was activated")
	}
	current, _ := a.ReadAppearance()
	if current.State.Active != "mineral" {
		t.Fatal("failed apply changed active selection")
	}
}

func TestAppearanceRefusesUnsafeAndUnknownDocuments(t *testing.T) {
	for _, change := range []func(*ThemeDocument){
		func(d *ThemeDocument) { d.Version = 2 },
		func(d *ThemeDocument) { d.Palettes["light"]["accent"] = "url(https://example.com)" },
		func(d *ThemeDocument) { d.Palettes["dark"]["--arbitrary"] = "#000000" },
		func(d *ThemeDocument) { d.Palettes["light"]["lit"] = "100.01%" },
		func(d *ThemeDocument) {
			d.Typography["interface"] = ThemeFace{Families: []string{"evil\nfont"}, Fallback: "system-ui"}
		},
		func(d *ThemeDocument) { d.Typography["monospace"] = ThemeFace{Fallback: "sans-serif"} },
	} {
		doc := builtinThemes()["mineral"]
		change(&doc)
		if err := validateTheme(doc); err == nil {
			t.Fatal("invalid theme accepted")
		}
	}
	doc := builtinThemes()["linen"]
	doc.Extensions = map[string]json.RawMessage{"other.app": json.RawMessage(`{"note":"round trip"}`)}
	data, _ := json.Marshal(doc)
	var restored ThemeDocument
	if err := strictAppearanceJSON(data, &restored, maxThemeBytes); err != nil {
		t.Fatal(err)
	}
	if err := validateTheme(restored); err != nil || string(restored.Extensions["other.app"]) != `{"note":"round trip"}` {
		t.Fatalf("extension round trip: %v", err)
	}
	for _, data := range []string{`null`, `{"format":"dinah-theme","unknown":true}`, `{} {}`, `[]`} {
		var target ThemeDocument
		if err := strictAppearanceJSON([]byte(data), &target, maxThemeBytes); err == nil && validateTheme(target) == nil {
			t.Fatal("invalid import accepted")
		}
	}
}

func TestAppearanceCorruptionIsNeverOverwritten(t *testing.T) {
	for _, data := range []string{`null`, `{"version":2}`, `{"version":1}`, `{`} {
		t.Run(data, func(t *testing.T) {
			a, _ := providerTestApp(t)
			file := filepath.Join(configDir(), "appearance.json")
			stateTestWrite(t, file, data)
			if _, err := a.ReadAppearance(); err == nil {
				t.Fatal("corrupt appearance accepted")
			}
			state := AppearanceState{Version: 1, Active: "mineral", Themes: []ThemeRecord{}, Overrides: map[string]ContentAppearance{}}
			if _, err := a.SaveAppearance(state, appearanceRevision([]byte(data))); err == nil {
				t.Fatal("corrupt appearance replaced")
			}
			got, _ := os.ReadFile(file)
			if string(got) != data {
				t.Fatal("corrupt file was modified")
			}
		})
	}
}

func TestAppearanceSmallContentSizes(t *testing.T) {
	for _, kind := range []string{"chat", "editor", "preview", "terminal"} {
		state := AppearanceState{Version: 1, Active: "mineral", Themes: []ThemeRecord{}, Overrides: map[string]ContentAppearance{kind: {Size: 6}}}
		if err := validateAppearance(state); err != nil {
			t.Fatalf("%s at 6px: %v", kind, err)
		}
		state.Overrides[kind] = ContentAppearance{Size: 5}
		if err := validateAppearance(state); err == nil {
			t.Fatalf("%s accepted below minimum", kind)
		}
	}
}
