package pages

import (
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Placement is where one window sits: its mode, and its left edge, top edge,
// width and height as fractions of the layer. A maximised or snapped window
// keeps its floating numbers, which are where it returns when restored.
type Placement struct {
	Mode       string
	X, Y, W, H float64
}

// WindowState is the arrangement of windows a URL names, as pudl-windows.js
// 0.5.0 keeps it: the open keys in opening order, the active key, the
// minimised set and each key's placement. Every function below mirrors the
// script function of the same name, so the server draws the windows a URL
// names and writes every window button as a link to the state it produces.
type WindowState struct {
	// Open are the open keys in the order they were opened.
	Open []string
	// Top is the active key, and empty when none is.
	Top string
	// Min are the minimised keys.
	Min map[string]bool
	// Place are the keys' placements.
	Place map[string]Placement
}

// MaxWindows is how many windows a page draws. Keys past it in the URL are
// dropped as though closed, so a hand-built URL cannot make one page render
// hundreds of cards.
const MaxWindows = 12

// windowModes are the four modes a placement names.
var windowModes = []string{"floating", "maximized", "left", "right"}

// windowKey is the grammar of a key, the script's KEY_RE.
var windowKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// placementPattern is the grammar of a placement, the script's parsePlacement.
var placementPattern = regexp.MustCompile(`^([a-z]+):([0-9.]+),([0-9.]+),([0-9.]+),([0-9.]+)$`)

// IsPageParameter reports whether a query parameter's name belongs to the
// window state rather than to the page's own read.
func IsPageParameter(name string) bool {
	return name == "open" || name == "top" || name == "min" || strings.HasPrefix(name, "p.")
}

// keyList is the script's keyList: the comma-separated keys that match the
// grammar, each once.
func keyList(value string) []string {
	seen := map[string]bool{}
	var keys []string
	for _, key := range strings.Split(value, ",") {
		if !windowKey.MatchString(key) || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

// validPlacement is the script's validPlacement.
func validPlacement(p Placement) bool {
	known := false
	for _, mode := range windowModes {
		known = known || p.Mode == mode
	}
	if !known {
		return false
	}
	for _, n := range []float64{p.X, p.Y, p.W, p.H} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1 {
			return false
		}
	}
	return p.W > 0 && p.H > 0
}

// parsePlacement is the script's parsePlacement, answering false where the
// script answers null.
func parsePlacement(value string) (Placement, bool) {
	match := placementPattern.FindStringSubmatch(value)
	if match == nil {
		return Placement{}, false
	}
	numbers := make([]float64, 4)
	for i := range numbers {
		n, err := strconv.ParseFloat(match[i+2], 64)
		if err != nil {
			return Placement{}, false
		}
		numbers[i] = n
	}
	p := Placement{Mode: match[1], X: numbers[0], Y: numbers[1], W: numbers[2], H: numbers[3]}
	return p, validPlacement(p)
}

// cascade is the placement the script gives a window the URL places nowhere,
// by the window's index in the open list.
func cascade(index int) Placement {
	step := float64(index%6) * 0.04
	return Placement{Mode: "floating", X: 0.06 + step, Y: 0.05 + step, W: 0.55, H: 0.75}
}

// ParseWindows is the script's readURL, followed by what its sync does at
// start: a key with no placement gets the cascade, and a state with no top
// takes the last open key that is not minimised. Keys past MaxWindows are
// dropped as though closed.
func ParseWindows(q url.Values) WindowState {
	s := WindowState{Min: map[string]bool{}, Place: map[string]Placement{}}
	s.Open = keyList(q.Get("open"))
	for _, key := range keyList(q.Get("min")) {
		if s.isOpen(key) {
			s.Min[key] = true
		}
	}
	for _, key := range s.Open {
		if p, ok := parsePlacement(q.Get("p." + key)); ok {
			s.Place[key] = p
		}
	}
	if top := q.Get("top"); top != "" && s.isOpen(top) && !s.Min[top] {
		s.Top = top
	}
	for len(s.Open) > MaxWindows {
		s = s.Closed(s.Open[len(s.Open)-1])
	}
	return s.settled()
}

// settled gives every open key without a placement the cascade, and a state
// with no top the last open key that is not minimised.
func (s WindowState) settled() WindowState {
	for i, key := range s.Open {
		if _, placed := s.Place[key]; !placed {
			s.Place[key] = cascade(i)
		}
	}
	if s.Top == "" {
		for i := len(s.Open) - 1; i >= 0; i-- {
			if !s.Min[s.Open[i]] {
				s.Top = s.Open[i]
				break
			}
		}
	}
	return s
}

// Without closes every key drop names, as Closed would. The head drops a key
// naming no live card this way.
func (s WindowState) Without(drop func(key string) bool) WindowState {
	for _, key := range append([]string(nil), s.Open...) {
		if drop(key) {
			s = s.Closed(key)
		}
	}
	return s
}

// isOpen reports whether a key is open.
func (s WindowState) isOpen(key string) bool {
	for _, open := range s.Open {
		if open == key {
			return true
		}
	}
	return false
}

// copied is the script's copy.
func (s WindowState) copied() WindowState {
	c := WindowState{Open: append([]string(nil), s.Open...), Top: s.Top, Min: map[string]bool{}, Place: map[string]Placement{}}
	for key, min := range s.Min {
		c.Min[key] = min
	}
	for key, p := range s.Place {
		c.Place[key] = p
	}
	return c
}

// Stacking is the order the windows are drawn in, bottom to top: the open
// keys with the top one moved last, which is the script's initial zOrder.
func (s WindowState) Stacking() []string {
	var order []string
	for _, key := range s.Open {
		if key != s.Top {
			order = append(order, key)
		}
	}
	if s.Top != "" && s.isOpen(s.Top) {
		order = append(order, s.Top)
	}
	return order
}

// nextTop is the script's nextTop: the topmost visible window other than the
// one given, walking the stacking order from the top.
func (s WindowState) nextTop(except string) string {
	order := s.Stacking()
	for i := len(order) - 1; i >= 0; i-- {
		key := order[i]
		if key != except && s.isOpen(key) && !s.Min[key] {
			return key
		}
	}
	return ""
}

// Raised is the script's raised: the key's minimised mark is cleared and it
// becomes the top window. A key not yet open is opened, appended with a
// cascade placement, as the script's open does.
func (s WindowState) Raised(key string) WindowState {
	c := s.copied()
	if !c.isOpen(key) {
		c.Open = append(c.Open, key)
		c.Place[key] = cascade(len(c.Open) - 1)
	}
	delete(c.Min, key)
	c.Top = key
	return c
}

// Minimized is the script's minimized.
func (s WindowState) Minimized(key string) WindowState {
	c := s.copied()
	c.Min[key] = true
	if c.Top == key {
		c.Top = s.nextTop(key)
	}
	return c
}

// MaximizeToggled is the script's maximizeToggled: the key is raised and its
// mode flips between floating and maximized, keeping its four numbers.
func (s WindowState) MaximizeToggled(key string) WindowState {
	c := s.Raised(key)
	p := c.Place[key]
	if p.Mode == "floating" {
		p.Mode = "maximized"
	} else {
		p.Mode = "floating"
	}
	c.Place[key] = p
	return c
}

// Closed is the script's closed.
func (s WindowState) Closed(key string) WindowState {
	c := s.copied()
	var open []string
	for _, k := range c.Open {
		if k != key {
			open = append(open, k)
		}
	}
	c.Open = open
	delete(c.Min, key)
	delete(c.Place, key)
	if c.Top == key {
		c.Top = s.nextTop(key)
	}
	return c
}

// Tabbed is the script's tabbed: a dock tab minimises the top window and
// raises any other, as a taskbar does.
func (s WindowState) Tabbed(key string) WindowState {
	if s.Top == key && !s.Min[key] {
		return s.Minimized(key)
	}
	return s.Raised(key)
}

// Param is one query parameter the page kept, in the order it arrived.
type Param struct {
	Name, Value string
}

// formatNumber is the script's fmt: rounded to three decimals and written
// without trailing zeros.
func formatNumber(n float64) string {
	return strconv.FormatFloat(math.Round(n*1000)/1000, 'f', -1, 64)
}

// formatPlacement is the script's formatPlacement.
func formatPlacement(p Placement) string {
	return p.Mode + ":" + formatNumber(p.X) + "," + formatNumber(p.Y) + "," + formatNumber(p.W) + "," + formatNumber(p.H)
}

// URLFor is the script's urlFor: the path, then the parameters that are not
// the window state's in the order they arrived, then the window state. The
// window parameters are written unescaped, as the script writes them, because
// their grammar needs no escaping.
func URLFor(path string, rest []Param, s WindowState) string {
	var parts []string
	for _, p := range rest {
		parts = append(parts, url.QueryEscape(p.Name)+"="+url.QueryEscape(p.Value))
	}
	if len(s.Open) > 0 {
		parts = append(parts, "open="+strings.Join(s.Open, ","))
		if s.Top != "" {
			parts = append(parts, "top="+s.Top)
		}
		var mins []string
		for _, key := range s.Open {
			if s.Min[key] {
				mins = append(mins, key)
			}
		}
		if len(mins) > 0 {
			parts = append(parts, "min="+strings.Join(mins, ","))
		}
		for _, key := range s.Open {
			if p, placed := s.Place[key]; placed {
				parts = append(parts, "p."+key+"="+formatPlacement(p))
			}
		}
	}
	if len(parts) == 0 {
		return path
	}
	return path + "?" + strings.Join(parts, "&")
}
