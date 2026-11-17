package main

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// A build of another product brings its own branding files: the mark the
// page shows (product-mark.svg), the brand stylesheet (brand.css) and the
// theme data with the product's built-in themes (theme-data.js), and any
// other page file it means to replace. They live in the product directory,
// which this repository ships with nothing but its README, and the page is
// served with the product's files over Dinah's: a path found in the product
// directory is served from there, and any other from the frontend
// (docs/design/product-definition.md). The product's identity itself comes
// from its profile (internal/profile, injected.go).
//
//go:embed product
var productAssets embed.FS

// productFrontend is the page's files: the product's over Dinah's.
func productFrontend() (fs.FS, error) {
	base, err := fs.Sub(assets, "frontend")
	if err != nil {
		return nil, err
	}
	over, err := fs.Sub(productAssets, "product")
	if err != nil {
		return nil, err
	}
	// Every page at the root is rendered for the product: the main window's,
	// a document window's and the slide show's.
	plain := overlayFS{over: over, base: base}
	entries, err := fs.ReadDir(plain, ".")
	if err != nil {
		return nil, err
	}
	pages := map[string][]byte{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		page, err := fs.ReadFile(plain, entry.Name())
		if err != nil {
			return nil, err
		}
		rendered, err := renderPage(page, product.Name, product.DefaultTheme)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if !bytes.Equal(rendered, page) {
			pages[entry.Name()] = rendered
		}
	}
	return overlayFS{over: over, base: base, pages: pages}, nil
}

/*
renderPage writes a product's name and default theme into a page before it is
served. The markup is written as Dinah's: its html element has Dinah's name and
default theme, its title has the name, an element marked data-product-name has
the name as its text, and an element with data-product-attrs has the name
inside each attribute listed there. A product's build serves all of these with
its own name, so the page is the product's from its first paint.

product.js makes the same replacements when the engine reports the product,
which is some time after the page is drawn. Until 17 November 2026 only the
html element and the title were rendered here, so a product's About box and
menu bar said Dinah for that time, and the listed attributes kept Dinah's name
for good, because product.js looks in them for the name on the html element,
which was already the product's.

The markup is read as HTML, so an attribute's value may hold any character.
What is not changed is written out as it was read.
*/
func renderPage(page []byte, name, theme string) ([]byte, error) {
	z := html.NewTokenizer(bytes.NewReader(page))
	var out bytes.Buffer
	// own is the name the markup was written with, known from the html element on.
	own := ""
	// marked says the element just opened shows the name as its text.
	marked, inTitle := false, false
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			if err := z.Err(); err != io.EOF {
				return nil, err
			}
			return out.Bytes(), nil
		}
		// Token reuses the buffer Raw returns.
		raw := append([]byte(nil), z.Raw()...)
		token := z.Token()
		wasMarked := marked
		marked = false
		switch kind {
		case html.StartTagToken, html.SelfClosingTagToken:
			changed := false
			if token.Data == "html" {
				for i, attr := range token.Attr {
					switch attr.Key {
					case "data-product-name":
						own = attr.Val
						changed = changed || attr.Val != name
						token.Attr[i].Val = name
					case "data-product-theme":
						changed = changed || attr.Val != theme
						token.Attr[i].Val = theme
					}
				}
			} else {
				listed := ""
				for _, attr := range token.Attr {
					switch attr.Key {
					case "data-product-name":
						marked = kind == html.StartTagToken
					case "data-product-attrs":
						listed = attr.Val
					}
				}
				if own != "" && own != name {
					for _, key := range strings.Fields(listed) {
						for i, attr := range token.Attr {
							if attr.Key == key && strings.Contains(attr.Val, own) {
								token.Attr[i].Val = strings.ReplaceAll(attr.Val, own, name)
								changed = true
							}
						}
					}
				}
			}
			inTitle = token.Data == "title" && kind == html.StartTagToken
			if changed {
				out.WriteString(token.String())
			} else {
				out.Write(raw)
			}
		case html.TextToken:
			switch {
			case wasMarked && token.Data != name:
				out.WriteString(html.EscapeString(name))
			case inTitle && own != "" && own != name && strings.Contains(token.Data, own):
				out.WriteString(html.EscapeString(strings.ReplaceAll(token.Data, own, name)))
			default:
				out.Write(raw)
			}
		case html.EndTagToken:
			// A marked element written empty is given the name.
			if wasMarked {
				out.WriteString(html.EscapeString(name))
			}
			inTitle = false
			out.Write(raw)
		default:
			out.Write(raw)
		}
	}
}

// memFile is a file held in memory: the rendered page.
type memFile struct {
	*bytes.Reader
	name string
}

func (f memFile) Stat() (fs.FileInfo, error) { return memInfo{name: f.name, size: f.Size()}, nil }
func (f memFile) Close() error               { return nil }

type memInfo struct {
	name string
	size int64
}

func (i memInfo) Name() string       { return i.name }
func (i memInfo) Size() int64        { return i.size }
func (i memInfo) Mode() fs.FileMode  { return 0o444 }
func (i memInfo) ModTime() time.Time { return time.Time{} }
func (i memInfo) IsDir() bool        { return false }
func (i memInfo) Sys() any           { return nil }

var _ io.ReadSeeker = memFile{}

// overlayFS serves a file from over when it has one, and from base otherwise.
// The root directory is base's, so that listing it lists the page's files.
type overlayFS struct {
	over, base fs.FS
	// pages are the pages rendered for the product, by name (renderPage). A
	// page that needed no change is not among them and is served as it is.
	pages map[string][]byte
}

func (o overlayFS) Open(name string) (fs.File, error) {
	if page, ok := o.pages[name]; ok {
		return memFile{Reader: bytes.NewReader(page), name: name}, nil
	}
	// The product directory's Go files and README are not page files.
	if name != "." && !strings.HasSuffix(name, ".go") && name != "README.md" {
		if f, err := o.over.Open(name); err == nil {
			if info, statErr := f.Stat(); statErr == nil && !info.IsDir() {
				return f, nil
			}
			f.Close()
		}
	}
	return o.base.Open(name)
}

// productThemeData is the product's theme data, when it brings its own.
func productThemeData() (string, bool) {
	over, err := fs.Sub(productAssets, "product")
	if err != nil {
		return "", false
	}
	data, err := fs.ReadFile(over, "theme-data.js")
	if err != nil || len(data) == 0 {
		return "", false
	}
	return string(data), true
}
