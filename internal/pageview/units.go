package pageview

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"studio/internal/preview"
)

// Unit is one part of a rendered page, as the preview marks it (see
// preview.Block): its first and last source line, its kind, the HTML the
// preview draws for it, the diagrams that HTML holds, and any marked parts
// inside it.
type Unit struct {
	Start    int
	End      int
	Kind     string
	HTML     string
	Diagrams []preview.Diagram
	Children []Unit
}

// Units reads the parts of a page from the preview's blocks, outermost
// first, each with the parts inside it.
func Units(blocks []preview.Block) []Unit {
	var out []Unit
	for _, b := range blocks {
		units := unitsIn(b.HTML)
		for i := range units {
			attachDiagrams(&units[i], b.Diagrams)
		}
		out = append(out, units...)
	}
	return out
}

func attachDiagrams(u *Unit, diagrams []preview.Diagram) {
	for _, d := range diagrams {
		if strings.Contains(u.HTML, `data-preview-diagram="`+d.ID+`"`) {
			u.Diagrams = append(u.Diagrams, d)
		}
	}
	for i := range u.Children {
		attachDiagrams(&u.Children[i], diagrams)
	}
}

// unitsIn finds the outermost part markers in a run of HTML and the HTML
// each one holds.
func unitsIn(s string) []Unit {
	type open struct {
		unit  *Unit
		inner int
	}
	var (
		stack []open
		roots []Unit
		pos   int
	)
	z := html.NewTokenizer(strings.NewReader(s))
	// Children are gathered on the unit that holds them, in order.
	children := map[*Unit][]*Unit{}
	var order []*Unit
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		raw := len(z.Raw())
		switch tt {
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			if string(name) != "div" {
				break
			}
			var u *Unit
			for hasAttr {
				var k, v []byte
				k, v, hasAttr = z.TagAttr()
				switch string(k) {
				case "class":
					if string(v) == "dinah-src" {
						u = &Unit{}
					}
				case "data-start":
					if u != nil {
						u.Start, _ = strconv.Atoi(string(v))
					}
				case "data-end":
					if u != nil {
						u.End, _ = strconv.Atoi(string(v))
					}
				case "data-kind":
					if u != nil {
						u.Kind = string(v)
					}
				}
			}
			if u != nil {
				for i := len(stack) - 1; i >= 0; i-- {
					if stack[i].unit != nil {
						children[stack[i].unit] = append(children[stack[i].unit], u)
						break
					}
				}
				order = append(order, u)
			}
			stack = append(stack, open{unit: u, inner: pos + raw})
		case html.EndTagToken:
			name, _ := z.TagName()
			if string(name) != "div" || len(stack) == 0 {
				break
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.unit != nil {
				top.unit.HTML = s[top.inner:pos]
			}
		}
		pos += raw
	}
	var build func(u *Unit) Unit
	build = func(u *Unit) Unit {
		out := *u
		for _, c := range children[u] {
			out.Children = append(out.Children, build(c))
		}
		return out
	}
	isChild := map[*Unit]bool{}
	for _, cs := range children {
		for _, c := range cs {
			isChild[c] = true
		}
	}
	for _, u := range order {
		if !isChild[u] {
			roots = append(roots, build(u))
		}
	}
	return roots
}
