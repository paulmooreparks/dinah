package main

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/net/html"
)

// A product's file is served over Dinah's of the same path; a path the
// product lacks is Dinah's; the root is Dinah's, so the page's files list.
func TestProductFilesAreServedOverDinahs(t *testing.T) {
	base := fstest.MapFS{"product-mark.svg": {Data: []byte("dinah")}, "index.html": {Data: []byte("page")}, "pudl/pudl.css": {Data: []byte("css")}}
	over := fstest.MapFS{"product-mark.svg": {Data: []byte("product")}, "brand.css": {Data: []byte("brand")}}
	f := overlayFS{over: over, base: base}
	read := func(name string) string {
		data, err := fs.ReadFile(f, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return string(data)
	}
	if read("product-mark.svg") != "product" || read("brand.css") != "brand" || read("index.html") != "page" || read("pudl/pudl.css") != "css" {
		t.Fatal("the overlay did not serve the product's files over Dinah's")
	}
	entries, err := fs.ReadDir(f, ".")
	if err != nil || len(entries) != 3 {
		t.Fatalf("the root is not Dinah's page: %v %d", err, len(entries))
	}
}

// The served page has the build's product wherever the markup has Dinah's:
// on its html element, in its title, as the text of each element marked
// data-product-name and inside each attribute an element lists in
// data-product-attrs. Nothing else is changed, and a name is written so that
// the page reads it back as it is.
func TestTheServedPageIsTheProducts(t *testing.T) {
	page := []byte(`<!doctype html><html lang="en" data-pudl-persistence="host" data-theme="light" data-product-name="Dinah" data-product-theme="clear-chime"><head><title>Dinah Documents</title><script>var words = "Dinah <b>";</script></head>` +
		`<body><nav aria-label="Dinah" title="Dinah's bar > all" data-product-attrs="aria-label">About <span data-product-name>Dinah</span> and <b data-product-name></b>.</nav>` +
		`<button hidden title="A newer build of Dinah is installed" aria-label="Dinah is newer" data-product-attrs="aria-label title">Update Dinah</button><img alt="Dinah" data-product-attrs="alt"></body></html>`)
	const name = `Parks & "Sons"`
	rendered, err := renderPage(page, name, "night")
	if err != nil {
		t.Fatal(err)
	}
	out := string(rendered)
	for _, want := range []string{
		`data-product-name="Parks &amp; &#34;Sons&#34;" data-product-theme="night"`,
		`<title>Parks &amp; &#34;Sons&#34; Documents</title>`,
		`<span data-product-name>Parks &amp; &#34;Sons&#34;</span>`,
		`<b data-product-name>Parks &amp; &#34;Sons&#34;</b>`,
		`aria-label="Parks &amp; &#34;Sons&#34;"`,
		`title="A newer build of Parks &amp; &#34;Sons&#34; is installed"`,
		`aria-label="Parks &amp; &#34;Sons&#34; is newer"`,
		`alt="Parks &amp; &#34;Sons&#34;"`,
		// What is not marked stays Dinah's: an attribute not listed, a script and ordinary text.
		// A tag with a changed attribute is written again, with the same values.
		`title="Dinah&#39;s bar &gt; all"`,
		`<script>var words = "Dinah <b>";</script>`,
		`>Update Dinah</button>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the rendered page lacks %s:\n%s", want, out)
		}
	}
	// A page rendered with the name it was written with is the page as
	// written, but for the marked element written empty, which is given the name.
	page = []byte(strings.Replace(string(page), `<b data-product-name></b>`, `<b data-product-name>Dinah</b>`, 1))
	same, err := renderPage(page, "Dinah", "clear-chime")
	if err != nil || string(same) != string(page) {
		t.Fatalf("rendering for the markup's own product changed the page: %v\n%s", err, same)
	}
	f, err := productFrontend()
	if err != nil {
		t.Fatal(err)
	}
	served, err := fs.ReadFile(f, "index.html")
	want := `data-product-name="` + html.EscapeString(product.Name) + `" data-product-theme="` + html.EscapeString(product.DefaultTheme) + `"`
	if err != nil || !strings.Contains(string(served), want) {
		t.Fatalf("the served page is not the product's: %v", err)
	}
}

// Every page the window program serves, rendered for another product, has no
// place left where the page's own script would have to put the name in
// later: read as a browser reads it, each marked element's text is the name,
// no listed attribute has Dinah's name, and neither has the title.
func TestEveryServedPageIsRenderedWhole(t *testing.T) {
	base, err := fs.Sub(assets, "frontend")
	if err != nil {
		t.Fatal(err)
	}
	pages, err := fs.Glob(base, "*.html")
	if err != nil || len(pages) == 0 {
		t.Fatalf("no pages: %v", err)
	}
	const name = "Another Product"
	marks := 0
	for _, path := range pages {
		page, err := fs.ReadFile(base, path)
		if err != nil {
			t.Fatal(err)
		}
		rendered, err := renderPage(page, name, "night")
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		doc, err := html.Parse(strings.NewReader(string(rendered)))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		was, err := html.Parse(strings.NewReader(string(page)))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		own := ""
		var text func(n *html.Node) string
		text = func(n *html.Node) string {
			if n.Type == html.TextNode {
				return n.Data
			}
			out := ""
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				out += text(c)
			}
			return out
		}
		attr := func(n *html.Node, key string) (string, bool) {
			for _, a := range n.Attr {
				if a.Key == key {
					return a.Val, true
				}
			}
			return "", false
		}
		var find func(n *html.Node)
		find = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "html" {
				own, _ = attr(n, "data-product-name")
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				find(c)
			}
		}
		find(was)
		if own == "" {
			continue // not a page that shows the product
		}
		var walk func(n *html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode {
				if value, ok := attr(n, "data-product-name"); ok {
					marks++
					if n.Data == "html" {
						if theme, _ := attr(n, "data-product-theme"); value != name || theme != "night" {
							t.Errorf("%s: the html element has %q and %q", path, value, theme)
						}
					} else if text(n) != name {
						t.Errorf("%s: a marked %s shows %q", path, n.Data, text(n))
					}
				}
				if listed, ok := attr(n, "data-product-attrs"); ok {
					for _, key := range strings.Fields(listed) {
						marks++
						if value, _ := attr(n, key); strings.Contains(value, own) || !strings.Contains(value, name) {
							t.Errorf("%s: %s's %s is %q", path, n.Data, key, value)
						}
					}
				}
				if n.Data == "title" && strings.Contains(text(n), own) {
					t.Errorf("%s: the title is %q", path, text(n))
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(doc)
		// The page rendered for its own product is the page as written.
		theme := ""
		var ownTheme func(n *html.Node)
		ownTheme = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "html" {
				theme, _ = attr(n, "data-product-theme")
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				ownTheme(c)
			}
		}
		ownTheme(was)
		if same, err := renderPage(page, own, theme); err != nil || string(same) != string(page) {
			t.Errorf("%s: rendering for the markup's own product changed the page (%v)", path, err)
		}
	}
	if marks < 20 {
		t.Fatalf("only %d places for the product's name were found; the pages' marks have changed", marks)
	}
}

// Dinah's own build ships the product directory with its README and the
// package's documentation file alone, neither of which the page serves, and
// every brand file the page refers to is among the page's files, under the
// product-neutral names a product replaces.
func TestDinahsBuildHasNoProductFiles(t *testing.T) {
	entries, err := fs.ReadDir(productAssets, "product")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "README.md,product.go" {
		t.Fatalf("the product directory holds more than its README and package file: %v", names)
	}
	served, err := productFrontend()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if _, err := fs.Stat(served, name); err == nil {
			t.Fatalf("the page serves the product directory's %s", name)
		}
	}
	page, err := fs.ReadFile(assets, "frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"product-mark.svg", "product-logo.svg", "brand.css", "theme-data.js"} {
		if !regexp.MustCompile(`(?m)["']` + regexp.QuoteMeta(name) + `["']`).Match(page) {
			t.Errorf("the page does not refer to %s", name)
		}
		if _, err := fs.Stat(assets, "frontend/"+name); err != nil {
			t.Errorf("the page lacks %s: %v", name, err)
		}
	}
	if regexp.MustCompile(`dinah-mark`).Match(page) {
		t.Error("the page still refers to the mark by Dinah's name")
	}
}
