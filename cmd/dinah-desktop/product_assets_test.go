package main

import (
	"io/fs"
	"regexp"
	"testing"
	"testing/fstest"
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

// Dinah's own build ships the product directory with its README alone, and
// every brand file the page refers to is among the page's files, under the
// product-neutral names a product replaces.
func TestDinahsBuildHasNoProductFiles(t *testing.T) {
	entries, err := fs.ReadDir(productAssets, "product")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "README.md" {
		t.Fatalf("the product directory holds more than its README: %v", entries)
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
