package main

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"regexp"
	"strings"
	"time"
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
	page, err := fs.ReadFile(overlayFS{over: over, base: base}, "index.html")
	if err != nil {
		return nil, err
	}
	return overlayFS{over: over, base: base, page: renderPage(page)}, nil
}

/*
The page's markup is written as Dinah's, with Dinah's name and default

	theme on its html element (product.js reads them first) and in its title.
	A product's build serves the page with its own in their place, so the
	page is the product's from its first paint and nothing is drawn as Dinah
	first. The spans marked data-product-name are filled by product.js from
	the html element, so they need no rewriting here.
*/
var pageProduct = regexp.MustCompile(`(<html[^>]*data-product-name=")[^"]*(")`)
var pageTheme = regexp.MustCompile(`(<html[^>]*data-product-theme=")[^"]*(")`)
var pageTitle = regexp.MustCompile(`(<title>)[^<]*(</title>)`)

func renderPage(page []byte) []byte {
	name := []byte(product.Name)
	page = pageProduct.ReplaceAll(page, append(append([]byte("${1}"), bytes.ReplaceAll(name, []byte("$"), []byte("$$"))...), []byte("${2}")...))
	page = pageTheme.ReplaceAll(page, append(append([]byte("${1}"), []byte(product.DefaultTheme)...), []byte("${2}")...))
	page = pageTitle.ReplaceAll(page, append(append([]byte("${1}"), bytes.ReplaceAll(name, []byte("$"), []byte("$$"))...), []byte("${2}")...))
	return page
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
	// page is the rendered index.html, when the product's name and theme
	// have been written into it (renderPage); nil serves the page as it is.
	page []byte
}

func (o overlayFS) Open(name string) (fs.File, error) {
	if name == "index.html" && o.page != nil {
		return memFile{Reader: bytes.NewReader(o.page), name: name}, nil
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
