package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/profile"
)

func withHome(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(profile.Current().HomeVar(), dir)
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func root(body string) string {
	i := strings.Index(body, "<html")
	if i < 0 {
		return ""
	}
	return body[i : i+strings.Index(body[i:], ">")+1]
}

// The page is drawn in the person's skin and theme mode by the server, with the host owning the persistence, and
// links every admitted skin's stylesheet before PUDL's scripts load.
func TestThePageIsDrawnInThePersonsSkinAndThemeMode(t *testing.T) {
	var chosen string
	for _, s := range skinCatalogue.Admitted() {
		if s.Stylesheet != nil {
			chosen = s.ID
		}
	}
	if chosen == "" {
		t.Skip("this release admits no skin beyond Standard")
	}
	withHome(t, map[string]string{
		"appearance.json": `{"version":1,"active":"mineral","skin":"` + chosen + `","themes":[],"overrides":{}}`,
		"ui-state.json":   `{"version":1,"values":{"pudl-theme":"dark"}}`,
	})
	s, _ := newServer(t)
	cookie := signIn(t, s)
	for _, page := range []string{"/", "/inbox"} {
		body := do(s, "GET", page, nil, map[string]string{"Cookie": cookie}).Body.String()
		r := root(body)
		for _, want := range []string{`data-pudl-persistence="host"`, `data-skin="` + chosen + `"`, `data-theme-pref="dark"`} {
			if !strings.Contains(r, want) {
				t.Fatalf("%s root %s lacks %s", page, r, want)
			}
		}
		link := strings.Index(body, `data-pudl-skin="`+chosen+`"`)
		script := strings.Index(body, "pudl-skins.js")
		theme := strings.Index(body, "pudl-theme.js")
		if link < 0 || script < 0 || theme < 0 || link > theme || theme > script {
			t.Fatalf("%s: the skin link must come before PUDL's scripts (link %d, theme %d, skins %d)", page, link, theme, script)
		}
	}
	// The signed-out page is drawn the same way.
	out := do(s, "GET", "/inbox", nil, nil)
	if r := root(out.Body.String()); !strings.Contains(r, `data-skin="`+chosen+`"`) || !strings.Contains(r, `data-pudl-persistence="host"`) {
		t.Fatalf("the signed-out root: %s", r)
	}
}

// A choice the build lacks is kept by the engine, and the page draws Standard.
func TestAnUnknownSkinIsDrawnAsStandard(t *testing.T) {
	withHome(t, map[string]string{"appearance.json": `{"version":1,"active":"mineral","skin":"a-later-skin","themes":[],"overrides":{}}`})
	s, _ := newServer(t)
	cookie := signIn(t, s)
	r := root(do(s, "GET", "/inbox", nil, map[string]string{"Cookie": cookie}).Body.String())
	if !strings.Contains(r, `data-skin="standard"`) || strings.Contains(r, "data-theme-pref") {
		t.Fatalf("the root: %s", r)
	}
}
