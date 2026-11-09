package main

import (
	"embed"
	"io/fs"
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
	return overlayFS{over: over, base: base}, nil
}

// overlayFS serves a file from over when it has one, and from base otherwise.
// The root directory is base's, so that listing it lists the page's files.
type overlayFS struct{ over, base fs.FS }

func (o overlayFS) Open(name string) (fs.File, error) {
	if name != "." {
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
