package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"studio/internal/skins"
	"studio/internal/statefile"
)

//go:embed frontend/theme-data.js
var themeBuiltinsSource string

// skinCatalogueSource is PUDL's catalogue of the skins the release admits (docs/plans/pudl-skins.md).
//
//go:embed frontend/pudl/pudl-skins.json
var skinCatalogueSource []byte

// skinCatalogue is the release's catalogue; a build whose catalogue cannot be read is a build that was never
// tested, so it fails at once.
var skinCatalogue = func() skins.Catalogue {
	c, err := skins.Parse(skinCatalogueSource)
	if err != nil {
		panic(err)
	}
	return c
}()

const maxThemeBytes = 256 << 10
const maxAppearanceBytes = 8 << 20

type ThemeFace struct {
	Families []string `json:"families"`
	Fallback string   `json:"fallback"`
}
type ThemeDocument struct {
	Format     string                       `json:"format"`
	Version    int                          `json:"version"`
	Name       string                       `json:"name"`
	Typography map[string]ThemeFace         `json:"typography"`
	Palettes   map[string]map[string]string `json:"palettes"`
	Extensions map[string]json.RawMessage   `json:"extensions,omitempty"`
}
type ThemeRecord struct {
	ID       string        `json:"id"`
	Document ThemeDocument `json:"document"`
}
type ContentAppearance struct {
	Font *ThemeFace `json:"font,omitempty"`
	Size int        `json:"size,omitempty"`
}
type AppearanceState struct {
	Version int    `json:"version"`
	Active  string `json:"active"`
	// Skin is the person's choice of PUDL skin, one for the whole application. It is the choice, not what is drawn:
	// a skin the build lacks is kept, and Effective says what is drawn.
	Skin      string                       `json:"skin"`
	Themes    []ThemeRecord                `json:"themes"`
	Overrides map[string]ContentAppearance `json:"overrides"`
}

// AppearanceSnapshot is the library as read, with its revision. Notice, when
// set, tells the person what reading it changed, such as a theme saved before
// category colours that could not be given colours keeping PUDL's rules.
type AppearanceSnapshot struct {
	State    AppearanceState `json:"state"`
	Revision string          `json:"revision"`
	// Skins are the skins the build admits, from the release's catalogue, and Effective the skin to draw for the
	// state's choice: the choice when the build admits it, else Standard.
	Skins     []skins.Choice `json:"skins"`
	Effective string         `json:"effective"`
	Notice    string         `json:"notice,omitempty"`
}

var themeHex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var themePercent = regexp.MustCompile(`^(?:100|[0-9]{1,2})(?:\.[0-9]{1,2})?%$`)

func builtinThemes() map[string]ThemeDocument {
	var result map[string]ThemeDocument
	source := strings.TrimSpace(strings.TrimPrefix(themeBuiltinsSource, "window.dinahThemeBuiltins = "))
	if err := json.Unmarshal([]byte(strings.TrimSuffix(source, ";")), &result); err != nil {
		panic(err)
	}
	return result
}
func strictAppearanceJSON(data []byte, target any, maximum int) error {
	if len(data) > maximum {
		return errors.New("appearance data exceeds its size limit")
	}
	if trimmed := bytes.TrimSpace(data); len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("appearance data must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("appearance data must contain one JSON object")
	}
	return nil
}
func validateThemeFace(face ThemeFace, role string) error {
	if face.Families == nil {
		return errors.New("font families must be an array")
	}
	allowed := face.Fallback == "sans-serif" || face.Fallback == "system-ui" || face.Fallback == "serif" || face.Fallback == "monospace"
	if (role == "heading" || role == "reading") && face.Fallback == "inherit" && len(face.Families) == 0 {
		return nil
	}
	if !allowed || len(face.Families) > 8 {
		return errors.New("invalid font fallback or family list")
	}
	if role == "interface" && face.Fallback != "sans-serif" && face.Fallback != "system-ui" {
		return errors.New("interface fonts require a sans-serif fallback")
	}
	if (role == "monospace" || role == "editor" || role == "terminal") && face.Fallback != "monospace" {
		return errors.New("code and terminal fonts require a monospace fallback")
	}
	for _, name := range face.Families {
		if strings.TrimSpace(name) == "" || len(name) > 128 || strings.ContainsAny(name, "\x00\r\n") || strings.Contains(strings.ToLower(name), "url(") {
			return errors.New("invalid font family name")
		}
		for _, r := range name {
			if unicode.IsControl(r) {
				return errors.New("font family names cannot contain control characters")
			}
		}
	}
	return nil
}
func validateTheme(doc ThemeDocument) error {
	if doc.Format != "dinah-theme" || doc.Version != 1 {
		return errors.New("unsupported theme format or version")
	}
	if strings.TrimSpace(doc.Name) == "" || len(doc.Name) > 100 {
		return errors.New("theme name must contain 1 to 100 bytes")
	}
	if len(doc.Typography) != 4 {
		return errors.New("theme must define all four font roles")
	}
	for _, role := range []string{"interface", "heading", "reading", "monospace"} {
		face, ok := doc.Typography[role]
		if !ok {
			return fmt.Errorf("missing %s font", role)
		}
		if err := validateThemeFace(face, role); err != nil {
			return err
		}
	}
	base := builtinThemes()["mineral"]
	if len(doc.Palettes) != 2 {
		return errors.New("theme must contain light and dark palettes")
	}
	for _, mode := range []string{"light", "dark"} {
		values, ok := doc.Palettes[mode]
		if !ok || len(values) != len(base.Palettes[mode]) {
			return fmt.Errorf("incomplete %s palette", mode)
		}
		for key := range base.Palettes[mode] {
			value, ok := values[key]
			if !ok {
				return fmt.Errorf("missing palette role %s", key)
			}
			if key == "lit" || key == "depth" {
				if !themePercent.MatchString(value) {
					return errors.New("lighting must be a percentage between 0 and 100")
				}
				var n float64
				fmt.Sscanf(value, "%f%%", &n)
				if n > 100 {
					return errors.New("lighting exceeds 100 percent")
				}
			} else if !themeHex.MatchString(value) {
				return fmt.Errorf("%s must be an opaque six-digit hex color", key)
			}
		}
	}
	data, _ := json.Marshal(doc)
	if len(data) > maxThemeBytes {
		return errors.New("theme exceeds 256 KiB")
	}
	return nil
}
func themeContrastErrors(doc ThemeDocument) []string {
	var failures []string
	rgb := func(value string) [3]float64 {
		data, _ := hex.DecodeString(strings.TrimPrefix(value, "#"))
		return [3]float64{float64(data[0]) / 255, float64(data[1]) / 255, float64(data[2]) / 255}
	}
	lum := func(c [3]float64) float64 {
		n := 0.0
		for i, v := range c {
			if v <= 0.04045 {
				v /= 12.92
			} else {
				v = math.Pow((v+0.055)/1.055, 2.4)
			}
			n += v * []float64{0.2126, 0.7152, 0.0722}[i]
		}
		return n
	}
	ratio := func(a, b [3]float64) float64 {
		x, y := lum(a), lum(b)
		return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
	}
	for mode, p := range doc.Palettes {
		backgrounds := [][3]float64{rgb(p["bg"]), rgb(p["surface"]), rgb(p["surface-alt"])}
		var percent float64
		fmt.Sscanf(p["lit"], "%f%%", &percent)
		surface, light := rgb(p["surface"]), rgb(p["light"])
		var input [3]float64
		for i := range input {
			input[i] = light[i]*percent/100 + surface[i]*(1-percent/100)
		}
		backgrounds = append(backgrounds, input)
		for key, value := range p {
			if key == "text" || key == "text-muted" || key == "accent" || key == "warn" || key == "danger" || key == "positive" || key == "dinah-skill" || key == "dinah-title" || strings.HasPrefix(key, "syntax-") {
				for _, background := range backgrounds {
					if ratio(rgb(value), background) < 4.5 {
						failures = append(failures, mode+" "+key+" needs 4.5:1 contrast")
						break
					}
				}
			}
		}
		if ratio(rgb(p["on-accent"]), rgb(p["accent"])) < 4.5 {
			failures = append(failures, mode+" primary action text needs 4.5:1")
		}
		if ratio(rgb(p["on-warn"]), rgb(p["warn"])) < 4.5 {
			failures = append(failures, mode+" warning badge text needs 4.5:1")
		}
		if ratio(rgb(p["border"]), input) < 3 {
			failures = append(failures, mode+" input boundary needs 3:1")
		}
		var inputBorder, dangerTop [3]float64
		text, hover := rgb(p["text"]), rgb(p["danger-hover"])
		for i := range inputBorder {
			inputBorder[i] = text[i]*0.55 + surface[i]*0.45
			dangerTop[i] = hover[i]*0.78 + 0.22
		}
		if ratio(inputBorder, input) < 3 {
			failures = append(failures, mode+" derived input boundary needs 3:1")
		}
		if ratio(rgb(p["on-accent"]), rgb(p["accent-hover"])) < 4.5 {
			failures = append(failures, mode+" primary hover text needs 4.5:1")
		}
		if ratio(rgb(p["on-accent"]), dangerTop) < 4.5 || ratio(rgb(p["on-accent"]), hover) < 4.5 {
			failures = append(failures, mode+" destructive hover text needs 4.5:1")
		}
		if ratio(rgb(p["tb-fg"]), rgb(p["tb-bg"])) < 4.5 {
			failures = append(failures, mode+" topbar text needs 4.5:1")
		}
	}
	return append(failures, themeCategoryErrors(doc)...)
}
func validateAppearance(state AppearanceState) error {
	if state.Version != 1 || state.Themes == nil || state.Overrides == nil || len(state.Themes) > 32 {
		return errors.New("invalid appearance library")
	}
	// An empty skin is the page not saying (an older page, or one drawn from its startup cache); saving keeps the
	// stored choice then.
	if state.Skin != "" && !skins.ValidID(state.Skin) {
		return errors.New("invalid skin")
	}
	// The built-in themes come from theme-data.js, so a theme added there is
	// both choosable and reserved against a person's own theme of that ID.
	ids := map[string]bool{}
	for id := range builtinThemes() {
		ids[id] = true
	}
	active := ids[state.Active]
	for _, record := range state.Themes {
		if len(record.ID) < 1 || len(record.ID) > 128 || ids[record.ID] {
			return errors.New("invalid or duplicate theme identity")
		}
		ids[record.ID] = true
		if err := validateTheme(record.Document); err != nil {
			return err
		}
		if record.ID == state.Active {
			active = true
			if failures := themeContrastErrors(record.Document); len(failures) > 0 {
				return errors.New(strings.Join(failures, "; "))
			}
		}
	}
	if !active {
		return errors.New("active theme does not exist")
	}
	for kind, override := range state.Overrides {
		if kind != "chat" && kind != "editor" && kind != "preview" && kind != "terminal" {
			return errors.New("unknown applet font override")
		}
		if override.Size != 0 && (override.Size < 6 || override.Size > 32 || (kind == "terminal" && override.Size > 22)) {
			return errors.New("content text size is out of range")
		}
		if override.Font != nil {
			if err := validateThemeFace(*override.Font, kind); err != nil {
				return err
			}
		}
	}
	return nil
}
func appearanceRevision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// decodeAppearance reads a stored library. Themes saved before category
// colours get them from fillThemeCategories; the record on disk changes only
// when the library is next saved. When the theme in use was filled and the
// filled colours break a category rule, the default theme is used in its
// place and the notice says why, so the person can set the colours in the
// Theme Editor rather than find the library refused.
func decodeAppearance(data []byte) (AppearanceState, string, error) {
	state := AppearanceState{Version: 1, Active: product.DefaultTheme, Skin: skins.Standard, Themes: []ThemeRecord{}, Overrides: map[string]ContentAppearance{}}
	if len(data) == 0 {
		return state, "", nil
	}
	state = AppearanceState{}
	if err := strictAppearanceJSON(data, &state, maxAppearanceBytes); err != nil {
		return state, "", err
	}
	if state.Skin == "" {
		// A library saved before skins has the default.
		state.Skin = skins.Standard
	}
	notice := ""
	for i := range state.Themes {
		if fillThemeCategories(&state.Themes[i].Document) && state.Themes[i].ID == state.Active {
			if failures := themeCategoryErrors(state.Themes[i].Document); len(failures) > 0 {
				notice = "The theme \"" + state.Themes[i].Document.Name + "\" was made before themes had category colours, and the colours chosen for it do not pass (" + strings.Join(failures, "; ") + "). " + builtinThemes()[product.DefaultTheme].Name + " is in use until its category colours are set in the Theme Editor."
				state.Active = product.DefaultTheme
			}
		}
	}
	return state, notice, validateAppearance(state)
}
func (a *App) ReadAppearance() (AppearanceSnapshot, error) {
	data, err := statefile.ReadShared(filepath.Join(configDir(), "appearance.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return AppearanceSnapshot{}, err
	}
	state, notice, err := decodeAppearance(data)
	return AppearanceSnapshot{State: state, Revision: appearanceRevision(data), Notice: notice, Skins: skinCatalogue.Choices(), Effective: skinCatalogue.Effective(state.Skin)}, err
}
func (a *App) SaveAppearance(state AppearanceState, revision string) (AppearanceSnapshot, error) {
	if err := validateAppearance(state); err != nil {
		return AppearanceSnapshot{}, err
	}
	var written AppearanceState
	var data []byte
	err := statefile.Update(filepath.Join(configDir(), "appearance.json"), func(previous []byte) ([]byte, error) {
		before, _, err := decodeAppearance(previous)
		if err != nil {
			return nil, err
		}
		if appearanceRevision(previous) != revision {
			return nil, errors.New("appearance changed in another window; reload before saving")
		}
		written = state
		if written.Skin == "" {
			written.Skin = before.Skin
		}
		data, err = json.MarshalIndent(written, "", "  ")
		if err != nil {
			return nil, err
		}
		if len(data) > maxAppearanceBytes {
			return nil, errors.New("appearance library exceeds 8 MiB")
		}
		return data, nil
	})
	snapshot := AppearanceSnapshot{State: written, Revision: appearanceRevision(data), Skins: skinCatalogue.Choices(), Effective: skinCatalogue.Effective(written.Skin)}
	if err == nil {
		// Another window of this application takes the change when it hears this, and the bus's page when it is next drawn.
		a.emit("appearance:changed", map[string]any{"revision": snapshot.Revision, "skin": snapshot.Effective})
	}
	return snapshot, err
}
func (a *App) ImportThemeFile() (*ThemeDocument, error) {
	name, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Import Theme", Filters: []wruntime.FileFilter{{DisplayName: product.Name + " theme (*.dinah-theme.json)", Pattern: "*.json"}}})
	if err != nil || name == "" {
		return nil, err
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxThemeBytes+1))
	if err != nil {
		return nil, err
	}
	var doc ThemeDocument
	if err := strictAppearanceJSON(data, &doc, maxThemeBytes); err != nil {
		return nil, err
	}
	// A theme exported before category colours imports with them filled;
	// the editor shows any rule the filled colours break.
	fillThemeCategories(&doc)
	return &doc, validateTheme(doc)
}
func (a *App) ExportThemeFile(doc ThemeDocument) error {
	if err := validateTheme(doc); err != nil {
		return err
	}
	name, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: "Export Theme", DefaultFilename: "theme.dinah-theme.json", Filters: []wruntime.FileFilter{{DisplayName: product.Name + " theme (*.dinah-theme.json)", Pattern: "*.dinah-theme.json"}}})
	if err != nil || name == "" {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(name, data, 0o600)
}
