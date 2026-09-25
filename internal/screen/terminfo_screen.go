package screen

import (
	"io"
	"strings"

	"dinah/internal/contract"
)

// The terminfo colour numbers setaf takes, as terminfo(5) tabulates them
// under "Color Handling": red is 1, yellow 3 and blue 4.
const (
	setafRed    = 1
	setafYellow = 3
	setafBlue   = 4
)

// minimumColours is the colors value a terminal needs before this layer
// colours anything, the eight setaf portably defines.
const minimumColours = 8

// NewTerminfo returns the terminfo layer over a terminal whose description
// is entry, writing through out. A nil entry is a terminal with no
// description, which can neither be coloured nor redrawn in place.
func NewTerminfo(entry *Terminfo, out io.Writer) Screen {
	t := &terminfoScreen{out: out}
	if entry == nil {
		t.reason = contract.WatchNoTerminalDescription
		return t
	}
	t.xon = entry.Flag(boolXonXoff)
	t.clear, t.clearOK = t.plain(entry, strClear)
	t.el, t.elOK = t.plain(entry, strEl)
	t.ed, t.edOK = t.plain(entry, strEd)
	t.civis, _ = t.plain(entry, strCivis)
	t.cnorm, _ = t.plain(entry, strCnorm)
	t.sgr0, t.sgr0OK = t.plain(entry, strSgr0)
	t.cup, t.cupOK = t.parameterised(entry, strCup, 0, 0)
	t.setaf, t.setafOK = t.parameterised(entry, strSetaf, setafRed)
	colours, _ := entry.Numeric(numColors)
	t.colour = colours >= minimumColours && t.setafOK && t.sgr0OK
	for _, needed := range []struct {
		name string
		ok   bool
	}{{"clear", t.clearOK}, {"cup", t.cupOK}, {"el", t.elOK}, {"ed", t.edOK}} {
		if !needed.ok {
			t.reason = contract.WatchMissingCapability
			t.missing = needed.name
			break
		}
	}
	return t
}

// terminfoScreen writes only strings its terminal's description supplied.
// Each capability is checked once, when the screen is opened, so a string
// this package cannot evaluate reads as a capability the terminal lacks.
type terminfoScreen struct {
	out     io.Writer
	xon     bool
	reason  string
	missing string
	colour  bool
	// The raw strings, and whether each is usable. cup and setaf keep the
	// parameterised form and are evaluated at every use.
	clear, el, ed, civis, cnorm, sgr0, cup, setaf string
	clearOK, elOK, edOK, sgr0OK, cupOK, setafOK   bool
}

// plain reads a capability that takes no parameter, with its delays handled.
func (t *terminfoScreen) plain(entry *Terminfo, index int) (string, bool) {
	raw, ok := entry.String(index)
	if !ok {
		return "", false
	}
	return withoutDelays(raw, t.xon)
}

// parameterised reads a capability that takes parameters, checking that it
// evaluates for the sample parameters and that its delays can be handled.
func (t *terminfoScreen) parameterised(entry *Terminfo, index int, sample ...int) (string, bool) {
	raw, ok := entry.String(index)
	if !ok {
		return "", false
	}
	evaluated, err := Evaluate(raw, sample...)
	if err != nil {
		return "", false
	}
	if _, ok := withoutDelays(evaluated, t.xon); !ok {
		return "", false
	}
	return raw, true
}

// expand evaluates a parameterised capability checked when the screen was
// opened. A parameter that makes it fail writes nothing, rather than a
// partial sequence.
func (t *terminfoScreen) expand(raw string, params ...int) string {
	evaluated, err := Evaluate(raw, params...)
	if err != nil {
		return ""
	}
	expanded, _ := withoutDelays(evaluated, t.xon)
	return expanded
}

// withoutDelays removes the delays a string carries, as terminfo(5)
// describes them under "Delays and Padding": $<n>, with an optional decimal
// and the suffixes * and /. With xon set, that section says "actual pad
// characters will not be transmitted", so a delay is dropped. A delay marked
// mandatory with /, or any delay on a terminal without xon, would need pad
// characters timed to the line's speed, which this layer does not send, so
// such a string reads as unusable.
func withoutDelays(s string, xon bool) (string, bool) {
	var b strings.Builder
	for {
		start := strings.Index(s, "$<")
		if start < 0 {
			b.WriteString(s)
			return b.String(), true
		}
		end := strings.IndexByte(s[start:], '>')
		if end < 0 {
			b.WriteString(s)
			return b.String(), true
		}
		delay := s[start+2 : start+end]
		if !isDelay(delay) {
			b.WriteString(s[:start+2])
			s = s[start+2:]
			continue
		}
		if !xon || strings.Contains(delay, "/") {
			return "", false
		}
		b.WriteString(s[:start])
		s = s[start+end+1:]
	}
}

// isDelay reports whether the text between $< and > is a delay: digits, an
// optional decimal part, and then * or / or both.
func isDelay(text string) bool {
	digits := strings.TrimRight(text, "*/")
	if digits == "" {
		return false
	}
	whole, fraction, _ := strings.Cut(digits, ".")
	if whole == "" {
		return false
	}
	for _, part := range []string{whole, fraction} {
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func (t *terminfoScreen) Colour() bool { return t.colour }

func (t *terminfoScreen) Live() (bool, string, string) {
	if t.reason != "" {
		return false, t.reason, t.missing
	}
	return true, "", ""
}

func (t *terminfoScreen) Begin() error {
	if _, err := io.WriteString(t.out, t.civis); err != nil {
		return err
	}
	return t.Clear()
}

func (t *terminfoScreen) Clear() error {
	_, err := io.WriteString(t.out, t.clear)
	return err
}

// Row writes cup for the row, the segments, and el, as one Write, so no
// sequence is ever split between two writes.
func (t *terminfoScreen) Row(row int, segments []Segment) error {
	var b strings.Builder
	b.WriteString(t.expand(t.cup, row-1, 0))
	t.segments(&b, segments)
	b.WriteString(t.el)
	_, err := io.WriteString(t.out, b.String())
	return err
}

// EraseBelow places the cursor at the start of the row after the one named
// and erases from there to the end of the screen.
func (t *terminfoScreen) EraseBelow(row int) error {
	_, err := io.WriteString(t.out, t.expand(t.cup, row, 0)+t.ed)
	return err
}

func (t *terminfoScreen) Line(segments []Segment) error {
	var b strings.Builder
	t.segments(&b, segments)
	b.WriteString("\n")
	_, err := io.WriteString(t.out, b.String())
	return err
}

func (t *terminfoScreen) Reset() error {
	if !t.colour {
		return nil
	}
	_, err := io.WriteString(t.out, t.sgr0)
	return err
}

// Restore writes sgr0, cnorm where the description has it, and cup for the
// start of the row named, then ends that line, all in one Write.
func (t *terminfoScreen) Restore(afterRow int) error {
	if afterRow < 1 {
		afterRow = 1
	}
	_, err := io.WriteString(t.out, t.sgr0+t.cnorm+t.expand(t.cup, afterRow-1, 0)+"\n")
	return err
}

// segments writes a line's segments, each coloured one between setaf and
// sgr0 when colour is available.
func (t *terminfoScreen) segments(b *strings.Builder, segments []Segment) {
	for _, segment := range segments {
		if segment.Colour == None || !t.colour {
			b.WriteString(segment.Text)
			continue
		}
		b.WriteString(t.expand(t.setaf, setafNumber(segment.Colour)))
		b.WriteString(segment.Text)
		b.WriteString(t.sgr0)
	}
}

// setafNumber is the terminfo colour number of a colour.
func setafNumber(colour Colour) int {
	switch colour {
	case Red:
		return setafRed
	case Yellow:
		return setafYellow
	}
	return setafBlue
}
