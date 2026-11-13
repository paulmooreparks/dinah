package shortcodes

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

/* A shortcode's drawing, for an applet that lays a document out in pages
   (agreed with yavc.app, 14 November 2026; docs/design/shortcodes.md).

   A container's drawing is a frame around one slot, and its body, which the
   applet draws itself, may be longer than a page. The frame's outer element
   is marked data-sc-frame and the element whose children are the body is
   marked data-sc-body, so the applet can draw the frame once for each page's
   piece of the body. The applet alone writes data-sc-piece on a piece, so
   one a set wrote is taken off here. A drawing with no marked body stays in
   one piece.

   A block the set draws whole may say that its HTML is ordinary flow
   content, which the applet may break between its top-level children,
   between a table's rows and inside a paragraph by line (Flower).

   The same request gives the same bytes: the marks are added by parsing
   the set's HTML and writing it out again, which is a function of that
   HTML alone. An applet relies on this to tell a drawing that changed from
   one that did not. */

// Drawing is one occurrence drawn.
type Drawing struct {
	HTML string `json:"html"`
	// Flow says the HTML is flow content that may be broken across pages.
	Flow bool `json:"flow,omitempty"`
}

// Flower is a set that says which of its drawings are flow content. It is
// asked only for a drawing without a slot, since a container's body is the
// applet's own.
type Flower interface {
	Flows(r Request) bool
}

// The attributes of the agreement.
const (
	frameAttr = "data-sc-frame"
	bodyAttr  = "data-sc-body"
	pieceAttr = "data-sc-piece"
)

// frames are the elements a container's frame may be. A span's drawing, and
// any other drawing whose outer element is in a line, is never a frame.
var frames = map[atom.Atom]bool{atom.Div: true, atom.Section: true, atom.Aside: true, atom.Blockquote: true, atom.Article: true, atom.Details: true, atom.Figure: true}

// marked is a container's drawing with its frame and body marked, or the
// drawing as it was when it is not one frame around one slot.
func marked(source string) string {
	if !strings.Contains(source, "<slot") {
		return source
	}
	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(source), context)
	if err != nil {
		return source
	}
	var root *html.Node
	for _, n := range nodes {
		switch {
		case n.Type == html.TextNode && strings.TrimSpace(n.Data) == "":
		case n.Type == html.ElementNode && root == nil:
			root = n
		default:
			// Text beside the frame, or a second element: not one frame.
			return source
		}
	}
	if root == nil || !frames[root.DataAtom] {
		return source
	}
	var slots []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			removeAttr(n, pieceAttr)
			if n.Data == "slot" && attr(n, "name") == "content" {
				slots = append(slots, n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if len(slots) != 1 {
		return source
	}
	slot := slots[0]
	body := slot.Parent
	if !alone(slot) || !frames[body.DataAtom] {
		// The slot shares its parent with a title or other content, which is
		// no part of the body, so the body gets an element of its own.
		wrapper := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
		body.InsertBefore(wrapper, slot)
		body.RemoveChild(slot)
		wrapper.AppendChild(slot)
		body = wrapper
	}
	setAttr(root, frameAttr)
	setAttr(body, bodyAttr)
	var out bytes.Buffer
	for _, n := range nodes {
		if html.Render(&out, n) != nil {
			return source
		}
	}
	return out.String()
}

// alone reports whether a node is its parent's only content.
func alone(n *html.Node) bool {
	for c := n.Parent.FirstChild; c != nil; c = c.NextSibling {
		if c == n || (c.Type == html.TextNode && strings.TrimSpace(c.Data) == "") {
			continue
		}
		return false
	}
	return true
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func removeAttr(n *html.Node, key string) {
	kept := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Key != key {
			kept = append(kept, a)
		}
	}
	n.Attr = kept
}

func setAttr(n *html.Node, key string) {
	removeAttr(n, key)
	n.Attr = append(n.Attr, html.Attribute{Key: key})
}

// Draw draws one occurrence as Render does, with a container's frame and
// body marked and with whether a block's drawing is flow content.
func Draw(r Request) (Drawing, error) {
	source, set, err := render(r)
	if err != nil {
		return Drawing{}, err
	}
	if r.Kind == Span || r.Kind == Inline {
		return Drawing{HTML: source}, nil
	}
	if strings.Contains(source, "<slot") {
		return Drawing{HTML: marked(source)}, nil
	}
	flower, ok := set.(Flower)
	return Drawing{HTML: source, Flow: ok && flower.Flows(r)}, nil
}
