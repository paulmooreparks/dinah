package main

import (
	"strings"

	"studio/internal/profile"
)

// product is the profile this build was made with (internal/profile): the
// product's name, its folders, the names the operating system sees, its
// ports and its defaults. The window reads every such value from here, and
// the page learns the ones it shows from State.
var product = profile.Current()

// ProductInfo is the part of the profile the page needs.
type ProductInfo struct {
	// Name is the product's name as people see it.
	Name string `json:"name"`
	// DefaultTheme is the built-in theme the window uses until the person
	// chooses another.
	DefaultTheme string `json:"defaultTheme"`
	// Host is the short name the product's own shell shows in its prompt as
	// the computer's, as in paul@dinah: its home folder's name without the
	// leading dot, which is what the person types in paths such as ~/.dinah.
	Host string `json:"host"`
}

func productInfo() ProductInfo {
	return ProductInfo{Name: product.Name, DefaultTheme: product.DefaultTheme, Host: strings.TrimPrefix(product.HomeFolder, ".")}
}
