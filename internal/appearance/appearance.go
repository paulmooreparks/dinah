// Package appearance validates the appearance library shared by desktop and web.
package appearance

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"studio/internal/skins"
)

const MaxThemeBytes = 256 << 10
const maxThemeBytes = MaxThemeBytes
const MaxAppearanceBytes = 8 << 20
const maxAppearanceBytes = MaxAppearanceBytes

// Library supplies the host's embedded built-ins and profile default.
type Library struct {
	Source  string
	Default string
}

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

var themeHex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var themePercent = regexp.MustCompile(`^(?:100|[0-9]{1,2})(?:\.[0-9]{1,2})?%$`)

func (l Library) Builtins() map[string]ThemeDocument {
	var result map[string]ThemeDocument
	source := strings.TrimSpace(strings.TrimPrefix(l.Source, "window.dinahThemeBuiltins = "))
	if err := json.Unmarshal([]byte(strings.TrimSuffix(source, ";")), &result); err != nil {
		panic(err)
	}
	return result
}
func StrictJSON(data []byte, target any, maximum int) error {
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
func ValidateFace(face ThemeFace, role string) error {
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
func (l Library) ValidateTheme(doc ThemeDocument) error {
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
		if err := ValidateFace(face, role); err != nil {
			return err
		}
	}
	base := l.Builtins()["mineral"]
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
func ContrastErrors(doc ThemeDocument) []string {
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
	return append(failures, CategoryErrors(doc)...)
}
func (l Library) Validate(state AppearanceState) error {
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
	for id := range l.Builtins() {
		ids[id] = true
	}
	active := ids[state.Active]
	for _, record := range state.Themes {
		if len(record.ID) < 1 || len(record.ID) > 128 || ids[record.ID] {
			return errors.New("invalid or duplicate theme identity")
		}
		ids[record.ID] = true
		if err := l.ValidateTheme(record.Document); err != nil {
			return err
		}
		if record.ID == state.Active {
			active = true
			if failures := ContrastErrors(record.Document); len(failures) > 0 {
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
			if err := ValidateFace(*override.Font, kind); err != nil {
				return err
			}
		}
	}
	return nil
}
func (l Library) Decode(data []byte) (AppearanceState, string, error) {
	state := AppearanceState{Version: 1, Active: l.Default, Skin: skins.Standard, Themes: []ThemeRecord{}, Overrides: map[string]ContentAppearance{}}
	if len(data) == 0 {
		return state, "", nil
	}
	state = AppearanceState{}
	if err := StrictJSON(data, &state, maxAppearanceBytes); err != nil {
		return state, "", err
	}
	if state.Skin == "" {
		// A library saved before skins has the default.
		state.Skin = skins.Standard
	}
	notice := ""
	for i := range state.Themes {
		if l.FillCategories(&state.Themes[i].Document) && state.Themes[i].ID == state.Active {
			if failures := CategoryErrors(state.Themes[i].Document); len(failures) > 0 {
				notice = "The theme \"" + state.Themes[i].Document.Name + "\" was made before themes had category colours, and the colours chosen for it do not pass (" + strings.Join(failures, "; ") + "). " + l.Builtins()[l.Default].Name + " is in use until its category colours are set in the Theme Editor."
				state.Active = l.Default
			}
		}
	}
	return state, notice, l.Validate(state)
}

// PUDL 0.91.0 (specification 0.48.0, chapter 3) adds eight category colours,
// which tell things apart without saying anything about their state. A theme
// that changes bg, surface, input-bg or the lighting must set all eight in
// both forms, and they must keep the rules checked here. Every Dinah theme
// changes those, so every palette sets them. appearance.js checks the same
// rules in the window, and the two are kept equal.

var themeCategoryKeys = []string{"category-1", "category-2", "category-3", "category-4", "category-5", "category-6", "category-7", "category-8"}

type themeColor [3]float64

func themeRGB(value string) themeColor {
	data, err := hex.DecodeString(strings.TrimPrefix(value, "#"))
	if err != nil || len(data) != 3 {
		return themeColor{}
	}
	return themeColor{float64(data[0]) / 255, float64(data[1]) / 255, float64(data[2]) / 255}
}

func themeLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func themeLuminance(c themeColor) float64 {
	return 0.2126*themeLinear(c[0]) + 0.7152*themeLinear(c[1]) + 0.0722*themeLinear(c[2])
}

// themeRatio is the WCAG 2.2 contrast ratio of two colours.
func themeRatio(a, b themeColor) float64 {
	x, y := themeLuminance(a), themeLuminance(b)
	return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
}

// themeOKLabLinear converts a colour whose channels are linear light to OKLab.
func themeOKLabLinear(c [3]float64) [3]float64 {
	l := math.Cbrt(0.4122214708*c[0] + 0.5363325363*c[1] + 0.0514459929*c[2])
	m := math.Cbrt(0.2119034982*c[0] + 0.6806995451*c[1] + 0.1073969566*c[2])
	s := math.Cbrt(0.0883024619*c[0] + 0.2817188376*c[1] + 0.6299787005*c[2])
	return [3]float64{
		0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

func themeLinearRGB(c themeColor) [3]float64 {
	return [3]float64{themeLinear(c[0]), themeLinear(c[1]), themeLinear(c[2])}
}

// themeDistanceLinear is the distance between two linear-light colours in
// OKLab, multiplied by 100, as the specification measures it.
func themeDistanceLinear(a, b [3]float64) float64 {
	x, y := themeOKLabLinear(a), themeOKLabLinear(b)
	return 100 * math.Sqrt((x[0]-y[0])*(x[0]-y[0])+(x[1]-y[1])*(x[1]-y[1])+(x[2]-y[2])*(x[2]-y[2]))
}

func themeDistance(a, b themeColor) float64 {
	return themeDistanceLinear(themeLinearRGB(a), themeLinearRGB(b))
}

func themeHue(c themeColor) float64 {
	lab := themeOKLabLinear(themeLinearRGB(c))
	return math.Mod(math.Atan2(lab[2], lab[1])*180/math.Pi+360, 360)
}

func themeHueGap(a, b themeColor) float64 {
	d := math.Mod(math.Abs(themeHue(a)-themeHue(b)), 360)
	return math.Min(d, 360-d)
}

// The matrices of Machado, Oliveira and Fernandes (2009) at full severity,
// applied to linear RGB.
var themeColourVision = [][3][3]float64{
	{{0.152286, 1.052583, -0.204868}, {0.114503, 0.786281, 0.099216}, {-0.003882, -0.048116, 1.051998}},
	{{0.367322, 0.860646, -0.227968}, {0.280085, 0.672501, 0.047413}, {-0.011820, 0.042940, 0.968881}},
}

// themeSimulatedDistance is the smaller of the two distances between a and b
// as a reader with protanopia and one with deuteranopia would see them.
func themeSimulatedDistance(a, b themeColor) float64 {
	least := math.Inf(1)
	for _, m := range themeColourVision {
		simulate := func(c themeColor) [3]float64 {
			l := themeLinearRGB(c)
			var out [3]float64
			for i, row := range m {
				out[i] = math.Min(1, math.Max(0, row[0]*l[0]+row[1]*l[1]+row[2]*l[2]))
			}
			return out
		}
		least = math.Min(least, themeDistanceLinear(simulate(a), simulate(b)))
	}
	return least
}

// themeCategoryBases returns bg, surface and input-bg. PUDL derives input-bg
// by mixing light into surface by lit.
func themeCategoryBases(p map[string]string) [3]themeColor {
	var percent float64
	fmt.Sscanf(p["lit"], "%f%%", &percent)
	surface, light := themeRGB(p["surface"]), themeRGB(p["light"])
	var input themeColor
	for i := range input {
		input[i] = light[i]*percent/100 + surface[i]*(1-percent/100)
	}
	return [3]themeColor{themeRGB(p["bg"]), surface, input}
}

// themeCategorySlotErrors checks one slot against the surfaces, the status
// colours and the accent.
func themeCategorySlotErrors(mode string, p map[string]string, slot int, c themeColor) []string {
	var failures []string
	key := themeCategoryKeys[slot]
	for _, base := range themeCategoryBases(p) {
		if themeRatio(c, base) < 3 {
			failures = append(failures, mode+" "+key+" needs 3:1 contrast")
			break
		}
	}
	for _, status := range []string{"warn", "danger", "positive"} {
		other := themeRGB(p[status])
		if themeDistance(c, other) < 12 || themeHueGap(c, other) < 20 {
			failures = append(failures, mode+" "+key+" is too close to "+status)
		}
	}
	if themeDistance(c, themeRGB(p["accent"])) < 8 {
		failures = append(failures, mode+" "+key+" is too close to accent")
	}
	return failures
}

// themeCategoryNeighbourError checks two neighbouring slots: at least 15 apart
// for normal vision and 8 under simulated protanopia and deuteranopia.
func themeCategoryNeighbourError(mode string, slot int, a, b themeColor) string {
	if themeDistance(a, b) < 15 || themeSimulatedDistance(a, b) < 8 {
		return mode + " " + themeCategoryKeys[slot] + " and " + themeCategoryKeys[slot+1] + " are too alike"
	}
	return ""
}

func PaletteCategoryErrors(mode string, p map[string]string) []string {
	var failures []string
	colors := make([]themeColor, len(themeCategoryKeys))
	for i, key := range themeCategoryKeys {
		colors[i] = themeRGB(p[key])
		failures = append(failures, themeCategorySlotErrors(mode, p, i, colors[i])...)
	}
	for i := 0; i+1 < len(colors); i++ {
		if failure := themeCategoryNeighbourError(mode, i, colors[i], colors[i+1]); failure != "" {
			failures = append(failures, failure)
		}
	}
	return failures
}

// CategoryErrors reports every category rule a valid theme breaks.
func CategoryErrors(doc ThemeDocument) []string {
	var failures []string
	for _, mode := range []string{"light", "dark"} {
		if p, ok := doc.Palettes[mode]; ok {
			failures = append(failures, PaletteCategoryErrors(mode, p)...)
		}
	}
	return failures
}

// FillCategories gives category colours to a theme saved before
// Dinah took PUDL 0.91.0. A palette with none of the eight takes those of the
// built-in palette of the same form whose bg, surface and input-bg are
// nearest its own, so a theme copied from a built-in theme gets that theme's
// colours. A slot that then breaks a rule takes the nearest built-in category
// colour that keeps them. A palette with some but not all eight is left
// alone, so validation reports it. It returns whether it filled a palette;
// a filled palette may still break a rule, which CategoryErrors reports.
func (l Library) FillCategories(doc *ThemeDocument) bool {
	builtins := l.Builtins()
	ids := make([]string, 0, len(builtins))
	for id := range builtins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	filled := false
	for _, mode := range []string{"light", "dark"} {
		p, ok := doc.Palettes[mode]
		if !ok {
			continue
		}
		present := 0
		for _, key := range themeCategoryKeys {
			if _, ok := p[key]; ok {
				present++
			}
		}
		if present != 0 || !themeHex.MatchString(p["bg"]) || !themeHex.MatchString(p["surface"]) || !themeHex.MatchString(p["light"]) || !themePercent.MatchString(p["lit"]) {
			continue
		}
		own := themeCategoryBases(p)
		nearest, best := "", math.Inf(1)
		var pool []string
		seen := map[string]bool{}
		for _, id := range ids {
			other := builtins[id].Palettes[mode]
			theirs, d := themeCategoryBases(other), 0.0
			for i := range own {
				d += themeDistance(own[i], theirs[i])
			}
			if d < best {
				nearest, best = id, d
			}
			for _, key := range themeCategoryKeys {
				if value := strings.ToLower(other[key]); !seen[value] {
					seen[value] = true
					pool = append(pool, value)
				}
			}
		}
		chosen := make([]string, len(themeCategoryKeys))
		for i, key := range themeCategoryKeys {
			chosen[i] = strings.ToLower(builtins[nearest].Palettes[mode][key])
		}
		fits := func(slot int, value string) bool {
			c := themeRGB(value)
			if len(themeCategorySlotErrors(mode, p, slot, c)) > 0 {
				return false
			}
			return slot == 0 || themeCategoryNeighbourError(mode, slot-1, themeRGB(chosen[slot-1]), c) == ""
		}
		for slot := range chosen {
			if fits(slot, chosen[slot]) {
				continue
			}
			original := themeRGB(chosen[slot])
			candidates := append([]string(nil), pool...)
			sort.SliceStable(candidates, func(i, j int) bool {
				return themeDistance(themeRGB(candidates[i]), original) < themeDistance(themeRGB(candidates[j]), original)
			})
			for _, value := range candidates {
				if fits(slot, value) {
					chosen[slot] = value
					break
				}
			}
		}
		for i, key := range themeCategoryKeys {
			p[key] = chosen[i]
		}
		filled = true
	}
	return filled
}
