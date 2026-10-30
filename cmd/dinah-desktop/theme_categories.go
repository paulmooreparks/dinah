package main

import (
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
)

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

func themePaletteCategoryErrors(mode string, p map[string]string) []string {
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

// themeCategoryErrors reports every category rule a valid theme breaks.
func themeCategoryErrors(doc ThemeDocument) []string {
	var failures []string
	for _, mode := range []string{"light", "dark"} {
		if p, ok := doc.Palettes[mode]; ok {
			failures = append(failures, themePaletteCategoryErrors(mode, p)...)
		}
	}
	return failures
}

// fillThemeCategories gives category colours to a theme saved before
// Dinah took PUDL 0.91.0. A palette with none of the eight takes those of the
// built-in palette of the same form whose bg, surface and input-bg are
// nearest its own, so a theme copied from a built-in theme gets that theme's
// colours. A slot that then breaks a rule takes the nearest built-in category
// colour that keeps them. A palette with some but not all eight is left
// alone, so validation reports it. It returns whether it filled a palette;
// a filled palette may still break a rule, which themeCategoryErrors reports.
func fillThemeCategories(doc *ThemeDocument) bool {
	builtins := builtinThemes()
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
