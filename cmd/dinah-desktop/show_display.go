package main

import (
	"os"
	"strconv"
	"strings"
)

// showDisplay is one display as a slide show's placement sees it: its
// rectangle on the desktop, in the pixels the operating system places
// windows by, whether it is the primary display, and whether the main window
// is on it (docs/design/detached-slide-show.md).
type showDisplay struct {
	X, Y, Width, Height int
	Primary             bool
	Current             bool
}

// chooseShowDisplay is the display a slide show goes to: one the main
// window is not on. With two displays that is the other one. With more it is
// the largest by area, the primary display winning a tie and the earlier of
// the list winning any tie left. There is none with one display, or when
// which display the main window is on is not known, and the show then stays
// in the main window (Paul, 10 November 2026).
func chooseShowDisplay(all []showDisplay) (showDisplay, bool) {
	known := false
	for _, d := range all {
		known = known || d.Current
	}
	var best showDisplay
	found := false
	for _, d := range all {
		if d.Current || d.Width <= 0 || d.Height <= 0 {
			continue
		}
		area, most := d.Width*d.Height, best.Width*best.Height
		if !found || area > most || area == most && d.Primary && !best.Primary {
			best, found = d, true
		}
	}
	return best, known && found
}

// showDisplaysVar is the environment variable that gives a window under
// test its displays, as "x,y,width,height" rectangles separated by
// semicolons, the first being the one the main window is on. It is read only
// under test (product.ShellEntryVar), where the show's window stays hidden,
// so a check does not depend on the displays of the machine it runs on.
func showDisplaysVar() string { return product.EnvPrefix + "SHOW_DISPLAYS" }

// showPlaceVar, showEntryVar and showHiddenVar are how the main program
// tells the show's process where its window goes, which address signs it
// in, and that it runs under test and shows nothing.
func showPlaceVar() string  { return product.EnvPrefix + "SHOW_PLACE" }
func showEntryVar() string  { return product.EnvPrefix + "SHOW_ENTRY" }
func showHiddenVar() string { return product.EnvPrefix + "SHOW_HIDDEN" }

func parseShowRect(s string) (showDisplay, bool) {
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) != 4 {
		return showDisplay{}, false
	}
	var n [4]int
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return showDisplay{}, false
		}
		n[i] = v
	}
	if n[2] <= 0 || n[3] <= 0 {
		return showDisplay{}, false
	}
	return showDisplay{X: n[0], Y: n[1], Width: n[2], Height: n[3]}, true
}

func formatShowRect(d showDisplay) string {
	return strconv.Itoa(d.X) + "," + strconv.Itoa(d.Y) + "," + strconv.Itoa(d.Width) + "," + strconv.Itoa(d.Height)
}

// showDisplays is the displays a show may be placed on: the test's own under
// test, and the operating system's otherwise.
func showDisplays() []showDisplay {
	if os.Getenv(product.ShellEntryVar()) == "" {
		return systemDisplays()
	}
	var out []showDisplay
	for i, part := range strings.Split(os.Getenv(showDisplaysVar()), ";") {
		if d, ok := parseShowRect(part); ok {
			d.Current, d.Primary = i == 0, i == 0
			out = append(out, d)
		}
	}
	return out
}
