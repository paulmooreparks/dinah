package web

import (
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"studio/internal/appearance"
	"studio/internal/profile"
	"studio/internal/statefile"
)

//go:embed static/theme-data.js
var themeBuiltinsSource string

var appearanceFallbackHeader = http.CanonicalHeaderKey("X-" + profile.Current().ID + "-Appearance-Fallback")

func appearanceLibrary() appearance.Library {
	return appearance.Library{Source: themeBuiltinsSource, Default: profile.Current().DefaultTheme}
}

func defaultAppearance() appearance.AppearanceState {
	state, _, _ := appearanceLibrary().Decode(nil)
	return state
}

func (s *Server) appearanceCSS(w http.ResponseWriter, r *http.Request) {
	home := skinHome()
	if home == "" {
		writeAppearanceCSS(w, defaultAppearance(), "unavailable-home")
		return
	}
	data, err := statefile.ReadShared(filepath.Join(home, "appearance.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		writeAppearanceCSS(w, defaultAppearance(), "unreadable-library")
		return
	}
	state, notice, err := appearanceLibrary().Decode(data)
	if err != nil {
		writeAppearanceCSS(w, defaultAppearance(), "invalid-library")
		return
	}
	if notice != "" {
		notice = "legacy-category-fallback"
	}
	writeAppearanceCSS(w, state, notice)
}

func writeAppearanceCSS(w http.ResponseWriter, state appearance.AppearanceState, notice string) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if notice != "" {
		w.Header().Set(appearanceFallbackHeader, notice)
	}
	doc, found := appearanceLibrary().Builtins()[state.Active]
	if !found {
		for _, record := range state.Themes {
			if record.ID == state.Active {
				doc = record.Document
				break
			}
		}
	}
	_, _ = w.Write([]byte(themeCSS(doc, state.Overrides)))
}

// cssFont quotes every family as a CSS string. Font names remain data even when
// they contain punctuation meaningful outside a string.
func cssFont(face appearance.ThemeFace) string {
	if face.Fallback == "inherit" {
		return "var(--font)"
	}
	parts := make([]string, 0, len(face.Families)+1)
	for _, family := range face.Families {
		parts = append(parts, `"`+strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(family)+`"`)
	}
	return strings.Join(append(parts, face.Fallback), ", ")
}

func themeCSS(doc appearance.ThemeDocument, overrides map[string]appearance.ContentAppearance) string {
	var css strings.Builder
	css.WriteString(":root{--font:" + cssFont(doc.Typography["interface"]) + ";--font-display:" + cssFont(doc.Typography["heading"]) + ";--mono:" + cssFont(doc.Typography["monospace"]) + ";--font-mono:var(--mono);--reading-font:" + cssFont(doc.Typography["reading"]) + ";")
	for _, kind := range []string{"chat", "editor", "preview", "terminal"} {
		role, size := "reading", 14
		if kind == "editor" || kind == "terminal" {
			role = "monospace"
		}
		if kind == "chat" {
			size = 15
		}
		face := doc.Typography[role]
		if override := overrides[kind]; override.Font != nil {
			face = *override.Font
		}
		if overrides[kind].Size != 0 {
			size = overrides[kind].Size
		}
		fmt.Fprintf(&css, "--%s-font:%s;--%s-size:%dpx;", kind, cssFont(face), kind, size)
	}
	css.WriteString("}\n")
	for _, mode := range []string{"dark", "light"} {
		selector := `:root[data-theme="` + mode + `"]`
		if mode == "dark" {
			selector += `,:root:not([data-theme])`
		} else {
			selector += `,:root:not([data-theme])[data-theme-pref="light"]`
		}
		writePaletteCSS(&css, selector, doc.Palettes[mode])
	}
	css.WriteString("@media(prefers-color-scheme:light){\n")
	writePaletteCSS(&css, `:root:not([data-theme])[data-theme-pref="system"]`, doc.Palettes["light"])
	css.WriteString("}\n")
	return css.String()
}

func writePaletteCSS(css *strings.Builder, selector string, palette map[string]string) {
	keys := make([]string, 0, len(palette))
	for key := range palette {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	css.WriteString(selector + "{")
	for _, key := range keys {
		if !strings.HasPrefix(key, "category-") {
			fmt.Fprintf(css, "--%s:%s;", key, palette[key])
		}
	}
	add, del := appearance.DiffTints(palette)
	fmt.Fprintf(css, "--diff-add-bg:color-mix(in srgb,var(--diff-add) %g%%,transparent);--diff-del-bg:color-mix(in srgb,var(--diff-del) %g%%,transparent);", add, del)
	css.WriteString("}\n@media(forced-colors:none){" + selector + "{")
	for _, key := range keys {
		if strings.HasPrefix(key, "category-") {
			fmt.Fprintf(css, "--%s:%s;", key, palette[key])
		}
	}
	css.WriteString("}}\n")
}
