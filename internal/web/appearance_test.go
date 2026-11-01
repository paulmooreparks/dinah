package web

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/appearance"
)

func TestThemeDataCopiesAgree(t *testing.T) {
	desktop, err := os.ReadFile("../../cmd/dinah-desktop/frontend/theme-data.js")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(desktop, []byte(themeBuiltinsSource)) {
		t.Fatal("desktop and web theme data differ")
	}
}

func TestWebPaletteSelectionAndPersistence(t *testing.T) {
	for id, doc := range appearanceLibrary().Builtins() {
		t.Run(id, func(t *testing.T) {
			if err := appearanceLibrary().ValidateTheme(doc); err != nil {
				t.Fatal(err)
			}
			if failures := appearance.ContrastErrors(doc); len(failures) != 0 {
				t.Fatal(failures)
			}
			state := defaultAppearance()
			state.Active = id
			checkWebPalette(t, state, doc)
		})
	}
	t.Run("custom with font overrides", func(t *testing.T) {
		state := defaultAppearance()
		doc := appearanceLibrary().Builtins()[state.Active]
		doc.Name = "My preserved palette"
		face := doc.Typography["interface"]
		face.Families = []string{`Font "; color:red; /*`}
		doc.Typography["interface"] = face
		state.Active = "custom"
		state.Themes = []appearance.ThemeRecord{{ID: "custom", Document: doc}}
		state.Overrides["chat"] = appearance.ContentAppearance{Size: 19}
		checkWebPalette(t, state, doc)
	})
}

func checkWebPalette(t *testing.T, state appearance.AppearanceState, doc appearance.ThemeDocument) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	withHome(t, map[string]string{"appearance.json": string(data)})
	s, _ := newServer(t)
	cookie := signIn(t, s)
	for _, mode := range []string{"light", "dark", "system"} {
		if err := os.WriteFile(filepath.Join(skinHome(), "ui-state.json"), []byte(`{"version":1,"values":{"pudl-theme":"`+mode+`"}}`), 0600); err != nil {
			t.Fatal(err)
		}
		response := do(s, "GET", "/appearance.css", nil, map[string]string{"Cookie": cookie})
		if response.Code != 200 || response.Header().Get("Content-Type") != "text/css; charset=utf-8" || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Dinah-Appearance-Fallback") != "" {
			t.Fatalf("stylesheet response: %d %v", response.Code, response.Header())
		}
		css := response.Body.String()
		for _, paletteMode := range []string{"light", "dark"} {
			for key, value := range doc.Palettes[paletteMode] {
				if !strings.Contains(css, "--"+key+":"+value+";") {
					t.Fatalf("%s lacks %s", paletteMode, key)
				}
			}
		}
		for _, want := range []string{"@media(forced-colors:none)", "@media(prefers-color-scheme:light)", "--font:" + cssFont(doc.Typography["interface"]), "--diff-add-bg:color-mix"} {
			if !strings.Contains(css, want) {
				t.Fatalf("missing CSS %s", want)
			}
		}
		page := do(s, "GET", "/", nil, map[string]string{"Cookie": cookie}).Body.String()
		if !strings.Contains(root(page), `data-theme-pref="`+mode+`"`) || strings.Index(page, `href="/appearance.css"`) <= strings.Index(page, `href="/static/dinah.css"`) {
			t.Fatal("palette stylesheet order or independent mode preference changed")
		}
	}
	after, err := os.ReadFile(filepath.Join(skinHome(), "appearance.json"))
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("reading a palette changed the appearance library")
	}
}

func TestWebAppearanceFallbackAndAuthentication(t *testing.T) {
	for _, data := range []string{`{`, `{"version":1,"active":"missing","themes":[],"overrides":{}}`, `{"version":2,"active":"mineral","themes":[],"overrides":{}}`} {
		t.Run(data, func(t *testing.T) {
			withHome(t, map[string]string{"appearance.json": data})
			s, _ := newServer(t)
			cookie := signIn(t, s)
			response := do(s, "GET", "/appearance.css", nil, map[string]string{"Cookie": cookie})
			if response.Header().Get("X-Dinah-Appearance-Fallback") != "invalid-library" {
				t.Fatal("invalid library did not report fallback")
			}
			anonymous := do(s, "GET", "/appearance.css", nil, nil)
			if anonymous.Code != 200 || anonymous.Header().Get("Content-Type") != "text/css; charset=utf-8" || anonymous.Header().Get("X-Dinah-Appearance-Fallback") != "" {
				t.Fatal("signed-out page did not receive default CSS")
			}
		})
	}
}

func TestCSSFontEscaping(t *testing.T) {
	got := cssFont(appearance.ThemeFace{Families: []string{`a";x:y;`, `b\c`}, Fallback: "sans-serif"})
	if got != `"a\";x:y;", "b\\c", sans-serif` {
		t.Fatalf("unsafe CSS font quoting: %s", got)
	}
}
