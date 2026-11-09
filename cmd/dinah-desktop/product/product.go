// Package product is where a build of another product registers what it
// brings to the window program: its kinds of workspace source, its
// connections and anything else the registries of internal take at start
// (docs/design/product-definition.md). The window program imports it for
// its side effects alone. Dinah's own build registers nothing, so this
// package has no code of its own; a product's build adds files beside this
// one that import the product's packages and register in their init.
//
// The directory also holds the product's page files, which the window
// serves over Dinah's (product_assets.go); Go files and the README are not
// served.
package product
