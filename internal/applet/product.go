package applet

import "sync"

/* A product built from this source brings applet bundles of its own,
   beside the built-in ones (docs/design/product-applets.md). Its package
   registers a maker for each at start, and the host adds what the makers
   make to the built-in bundles when it assembles the registry, so the
   product's actions reach agents, flows and the page as the built-in ones
   do. A maker that fails stops the assembly with its error, as a built-in
   bundle's would, since a bundle that cannot be made is a mistake in the
   build. */

var (
	productMu     sync.Mutex
	productMakers []func() (Bundle, error)
)

// RegisterBundle registers a maker of a product's bundle. It is called from
// a package's init, before the registry is assembled.
func RegisterBundle(make func() (Bundle, error)) {
	if make == nil {
		panic("applet: nil bundle maker")
	}
	productMu.Lock()
	defer productMu.Unlock()
	productMakers = append(productMakers, make)
}

// ProductBundles makes the registered product bundles, in the order they
// were registered.
func ProductBundles() ([]Bundle, error) {
	productMu.Lock()
	makers := append([]func() (Bundle, error){}, productMakers...)
	productMu.Unlock()
	out := make([]Bundle, 0, len(makers))
	for _, make := range makers {
		b, err := make()
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}
