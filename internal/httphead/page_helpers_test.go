package httphead

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// browserAccept is the Accept header a browser sends when it navigates.
const browserAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

// page sends a GET the way a browser navigating sends it.
func (f *fixture) page(path string, header ...string) reply {
	f.t.Helper()
	return f.get(path, append([]string{"Accept", browserAccept}, header...)...)
}

// pageForm posts a form the way a browser submits one from a page of this
// server: the page's Origin, Sec-Fetch-Site same-origin, the page as the
// Referer, and a browser's Accept. The header list overrides any of them,
// and a value of "-" leaves that header out.
func (f *fixture) pageForm(path, body, from string, header ...string) reply {
	f.t.Helper()
	headers := map[string]string{
		"Content-Type":   typeForm,
		"Origin":         f.origin(),
		"Sec-Fetch-Site": "same-origin",
		"Referer":        f.url + from,
		"Accept":         browserAccept,
	}
	for name, value := range pairs(header) {
		headers[name] = value
	}
	for name, value := range headers {
		if value == "-" {
			delete(headers, name)
		}
	}
	return f.send(request{method: "POST", path: path, header: headers, body: body})
}

// stubParser is a ParseLine for the head's own tests, which cannot reach the
// terminal's parser in cmd/dinah. It refuses every line, so a typed line
// records a refused entry and answers 303, which is all these tests read.
func stubParser(words []string) (TypedLine, *contract.Refusal) {
	return TypedLine{}, contract.Refuse(contract.UnknownVerb, strings.Join(words, " "))
}

// node is one element of a page, as the scriptless reader parses it.
type node struct {
	name   string
	attr   map[string]string
	order  []string
	kids   []*node
	text   string
	parent *node
}

// parseHTML reads a page with encoding/xml in non-strict mode, closing HTML's
// void elements and knowing its entities, which is why the templates write a
// void element with a closing slash.
func parseHTML(t *testing.T, body string) *node {
	t.Helper()
	decoder := xml.NewDecoder(strings.NewReader(body))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	root := &node{name: "#document", attr: map[string]string{}}
	current := root
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("the page does not parse: %v\n%s", err, body)
		}
		switch tok := token.(type) {
		case xml.StartElement:
			n := &node{name: strings.ToLower(tok.Name.Local), attr: map[string]string{}, parent: current}
			for _, a := range tok.Attr {
				name := a.Name.Local
				if a.Name.Space != "" {
					name = a.Name.Space + ":" + name
				}
				n.attr[name] = a.Value
				n.order = append(n.order, name)
			}
			current.kids = append(current.kids, n)
			current = n
		case xml.EndElement:
			if current.parent != nil {
				current = current.parent
			}
		case xml.CharData:
			current.text += string(tok)
		}
	}
	return root
}

// all returns every element below n, n included, that match accepts, in
// document order.
func (n *node) all(match func(*node) bool) []*node {
	var found []*node
	var walk func(*node)
	walk = func(at *node) {
		if match(at) {
			found = append(found, at)
		}
		for _, kid := range at.kids {
			walk(kid)
		}
	}
	walk(n)
	return found
}

// named returns every element of a name.
func (n *node) named(name string) []*node {
	return n.all(func(at *node) bool { return at.name == name })
}

// first returns the first element match accepts, or nil.
func (n *node) first(match func(*node) bool) *node {
	if found := n.all(match); len(found) > 0 {
		return found[0]
	}
	return nil
}

// withClass reports whether an element carries a class.
func withClass(class string) func(*node) bool {
	return func(at *node) bool {
		for _, c := range strings.Fields(at.attr["class"]) {
			if c == class {
				return true
			}
		}
		return false
	}
}

// has reports whether an element carries an attribute.
func (n *node) has(name string) bool {
	_, ok := n.attr[name]
	return ok
}

// inside reports whether an element sits below one of a name.
func (n *node) inside(name string) bool {
	for at := n.parent; at != nil; at = at.parent {
		if at.name == name {
			return true
		}
	}
	return false
}

// allText is the text of an element and everything below it.
func (n *node) allText() string {
	var b strings.Builder
	var walk func(*node)
	walk = func(at *node) {
		b.WriteString(at.text)
		for _, kid := range at.kids {
			walk(kid)
		}
	}
	walk(n)
	return b.String()
}
