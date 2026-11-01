package appearance

import (
	"strconv"
	"strings"
)

// DiffTints mirrors the desktop's half-percent search against PUDL's diff
// contrast rules. A custom palette may need less than the standard ten percent.
func DiffTints(p map[string]string) (float64, float64) {
	lit, _ := strconv.ParseFloat(strings.TrimSuffix(p["lit"], "%"), 64)
	input := themeRGB(p["surface"])
	light := themeRGB(p["light"])
	for i := range input {
		input[i] = light[i]*lit/100 + input[i]*(1-lit/100)
	}
	amount := func(value string) float64 {
		color := themeRGB(value)
		for percent := 10.0; percent > 0; percent -= .5 {
			var tint themeColor
			for i := range tint {
				tint[i] = color[i]*percent/100 + input[i]*(1-percent/100)
			}
			valid := themeRatio(color, tint) >= 3
			for key, value := range p {
				if strings.HasPrefix(key, "syntax-") && themeRatio(themeRGB(value), tint) < 4.5 {
					valid = false
				}
			}
			if valid {
				return percent
			}
		}
		return 0
	}
	return amount(p["positive"]), amount(p["danger"])
}
