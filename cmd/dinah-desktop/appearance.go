package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"studio/internal/appearance"
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
	files, err := fs.Sub(assets, "frontend/pudl")
	if err != nil {
		panic(err)
	}
	if err := c.ValidateAssets(files); err != nil {
		panic(err)
	}
	return c
}()

const maxThemeBytes = appearance.MaxThemeBytes
const maxAppearanceBytes = appearance.MaxAppearanceBytes

type ThemeFace = appearance.ThemeFace
type ThemeDocument = appearance.ThemeDocument
type ThemeRecord = appearance.ThemeRecord
type ContentAppearance = appearance.ContentAppearance
type AppearanceState = appearance.AppearanceState

func appearanceLibrary() appearance.Library {
	return appearance.Library{Source: themeBuiltinsSource, Default: product.DefaultTheme}
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
var themeCategoryKeys = []string{"category-1", "category-2", "category-3", "category-4", "category-5", "category-6", "category-7", "category-8"}

func builtinThemes() map[string]ThemeDocument { return appearanceLibrary().Builtins() }
func strictAppearanceJSON(data []byte, target any, maximum int) error {
	return appearance.StrictJSON(data, target, maximum)
}
func validateThemeFace(face ThemeFace, role string) error { return appearance.ValidateFace(face, role) }
func validateTheme(doc ThemeDocument) error               { return appearanceLibrary().ValidateTheme(doc) }
func themeContrastErrors(doc ThemeDocument) []string      { return appearance.ContrastErrors(doc) }
func themeCategoryErrors(doc ThemeDocument) []string      { return appearance.CategoryErrors(doc) }
func fillThemeCategories(doc *ThemeDocument) bool         { return appearanceLibrary().FillCategories(doc) }
func validateAppearance(state AppearanceState) error      { return appearanceLibrary().Validate(state) }
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
	return appearanceLibrary().Decode(data)
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

func themePaletteCategoryErrors(mode string, palette map[string]string) []string {
	return appearance.PaletteCategoryErrors(mode, palette)
}
