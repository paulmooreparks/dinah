package pages

import (
	"net/url"
	"testing"
)

// stateOf parses a query string as the page's window state, the way the head
// does.
func stateOf(t *testing.T, raw string) WindowState {
	t.Helper()
	q, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return ParseWindows(q)
}

// TestWindowStateMirrorsTheScript holds the Go window state to
// pudl-windows.js 0.5.0. Each row is a URL, an operation, and the URL the
// script's own functions (readURL, raised, minimized, maximizeToggled,
// closed, tabbed, urlFor) produce for it, worked through by hand from the
// script: a key missing a placement takes the cascade, as sync gives it at
// start, and urlFor writes every placement the state carries.
func TestWindowStateMirrorsTheScript(t *testing.T) {
	rest := []Param{{Name: "query", Value: "state:ready"}, {Name: "b", Value: "2"}}
	rows := []struct {
		name  string
		query string
		op    func(WindowState) WindowState
		want  string
	}{
		{"a duplicate key is read once", "open=a,a&p.a=floating:0.1,0.1,0.5,0.5", nil,
			"/x?query=state%3Aready&b=2&open=a&top=a&p.a=floating:0.1,0.1,0.5,0.5"},
		{"a key outside the grammar is dropped", "open=a,b%2Fc&p.a=floating:0.1,0.1,0.5,0.5", nil,
			"/x?query=state%3Aready&b=2&open=a&top=a&p.a=floating:0.1,0.1,0.5,0.5"},
		{"min naming a key not open is dropped", "open=a&min=b&p.a=floating:0.1,0.1,0.5,0.5", nil,
			"/x?query=state%3Aready&b=2&open=a&top=a&p.a=floating:0.1,0.1,0.5,0.5"},
		{"top naming a minimised key is dropped, and the last visible key is top", "open=a,b&top=b&min=b&p.a=floating:0.1,0.1,0.5,0.5&p.b=left:0.2,0.2,0.4,0.4", nil,
			"/x?query=state%3Aready&b=2&open=a,b&top=a&min=b&p.a=floating:0.1,0.1,0.5,0.5&p.b=left:0.2,0.2,0.4,0.4"},
		{"a placement with a bad mode cascades", "open=a&p.a=sideways:0.1,0.1,0.5,0.5", nil,
			"/x?query=state%3Aready&b=2&open=a&top=a&p.a=floating:0.06,0.05,0.55,0.75"},
		{"a number above 1 cascades", "open=a&p.a=floating:0.1,0.1,1.5,0.5", nil,
			"/x?query=state%3Aready&b=2&open=a&top=a&p.a=floating:0.06,0.05,0.55,0.75"},
		{"a zero width cascades", "open=a&p.a=floating:0.1,0.1,0,0.5", nil,
			"/x?query=state%3Aready&b=2&open=a&top=a&p.a=floating:0.06,0.05,0.55,0.75"},
		{"numbers round to three decimals without trailing zeros", "open=a&p.a=floating:0.12345,0.1000,0.5556,0.25", nil,
			"/x?query=state%3Aready&b=2&open=a&top=a&p.a=floating:0.123,0.1,0.556,0.25"},
		{"closing the top window makes the next visible one top", "open=a,b,c&top=c&p.a=floating:0.1,0.1,0.5,0.5&p.b=floating:0.2,0.2,0.5,0.5&p.c=floating:0.3,0.3,0.5,0.5",
			func(s WindowState) WindowState { return s.Closed("c") },
			"/x?query=state%3Aready&b=2&open=a,b&top=b&p.a=floating:0.1,0.1,0.5,0.5&p.b=floating:0.2,0.2,0.5,0.5"},
		{"minimising the only window leaves no top", "open=a&p.a=floating:0.1,0.1,0.5,0.5",
			func(s WindowState) WindowState { return s.Minimized("a") },
			"/x?query=state%3Aready&b=2&open=a&min=a&p.a=floating:0.1,0.1,0.5,0.5"},
		{"maximising keeps the four numbers", "open=a,b&top=a&p.a=floating:0.1,0.1,0.5,0.5&p.b=floating:0.2,0.2,0.5,0.5",
			func(s WindowState) WindowState { return s.MaximizeToggled("b") },
			"/x?query=state%3Aready&b=2&open=a,b&top=b&p.a=floating:0.1,0.1,0.5,0.5&p.b=maximized:0.2,0.2,0.5,0.5"},
		{"restoring a maximised window floats it", "open=a&p.a=maximized:0.1,0.1,0.5,0.5",
			func(s WindowState) WindowState { return s.MaximizeToggled("a") },
			"/x?query=state%3Aready&b=2&open=a&top=a&p.a=floating:0.1,0.1,0.5,0.5"},
		{"a tab on the top window minimises it", "open=a,b&top=b&p.a=floating:0.1,0.1,0.5,0.5&p.b=floating:0.2,0.2,0.5,0.5",
			func(s WindowState) WindowState { return s.Tabbed("b") },
			"/x?query=state%3Aready&b=2&open=a,b&top=a&min=b&p.a=floating:0.1,0.1,0.5,0.5&p.b=floating:0.2,0.2,0.5,0.5"},
		{"a tab on a minimised window raises it", "open=a,b&top=a&min=b&p.a=floating:0.1,0.1,0.5,0.5&p.b=floating:0.2,0.2,0.5,0.5",
			func(s WindowState) WindowState { return s.Tabbed("b") },
			"/x?query=state%3Aready&b=2&open=a,b&top=b&p.a=floating:0.1,0.1,0.5,0.5&p.b=floating:0.2,0.2,0.5,0.5"},
		{"raising a key not yet open appends it with a cascade", "open=a&p.a=floating:0.1,0.1,0.5,0.5",
			func(s WindowState) WindowState { return s.Raised("z") },
			"/x?query=state%3Aready&b=2&open=a,z&top=z&p.a=floating:0.1,0.1,0.5,0.5&p.z=floating:0.1,0.09,0.55,0.75"},
	}
	for _, row := range rows {
		s := stateOf(t, row.query)
		if row.op != nil {
			s = row.op(s)
		}
		if got := URLFor("/x", rest, s); got != row.want {
			t.Errorf("%s:\n got  %s\n want %s", row.name, got, row.want)
		}
	}
	if got := URLFor("/x", nil, WindowState{}); got != "/x" {
		t.Errorf("an empty state writes %q, wanted the bare path", got)
	}
	t.Logf("checked %d rows", len(rows))
}

// TestMissingPlacementsCascade holds the cascade to the script's: with i a
// key's index, the step is (i mod 6) times 0.04, so the seventh key wraps to
// the first key's placement.
func TestMissingPlacementsCascade(t *testing.T) {
	s := stateOf(t, "open=k0,k1,k2,k3,k4,k5,k6")
	want := []string{
		"floating:0.06,0.05,0.55,0.75",
		"floating:0.1,0.09,0.55,0.75",
		"floating:0.14,0.13,0.55,0.75",
		"floating:0.18,0.17,0.55,0.75",
		"floating:0.22,0.21,0.55,0.75",
		"floating:0.26,0.25,0.55,0.75",
		"floating:0.06,0.05,0.55,0.75",
	}
	for i, key := range s.Open {
		if got := formatPlacement(s.Place[key]); got != want[i] {
			t.Errorf("%s: %s, want %s", key, got, want[i])
		}
	}
	if len(s.Open) != 7 || s.Top != "k6" {
		t.Errorf("wanted seven open keys with k6 on top, got %v top %q", s.Open, s.Top)
	}
}

// TestNoMoreThanTwelveWindowsAreDrawn holds the cap: keys past the twelfth
// are dropped as though closed.
func TestNoMoreThanTwelveWindowsAreDrawn(t *testing.T) {
	s := stateOf(t, "open=a1,a2,a3,a4,a5,a6,a7,a8,a9,a10,a11,a12,a13,a14&top=a14")
	if len(s.Open) != MaxWindows || s.Open[MaxWindows-1] != "a12" || s.Top != "a12" {
		t.Errorf("wanted the first twelve keys with a12 on top, got %v top %q", s.Open, s.Top)
	}
}
