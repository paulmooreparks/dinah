// Package pageroutes is the registry of the HTTP routes a product built from
// this source adds to the page's own server (docs/design/product-applets.md):
// files an applet of the product serves to the page under a path prefix of
// its own, such as a client's fonts or an embedded app's files. The shell
// server asks the registry for a request its own routes do not take, behind
// the same cookie that guards the page's files, so a route is reached by
// the page alone. Nothing here knows a route; Dinah's own build registers
// none.
package pageroutes

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	mu     sync.RWMutex
	routes = map[string]http.Handler{}
)

var prefixPattern = regexp.MustCompile(`^/[a-z][a-z0-9-]*/$`)

// Register adds a handler for every request whose path starts with prefix,
// which is "/<name>/", lowercase words joined by hyphens. It panics on a
// malformed or repeated prefix, since both are mistakes in the build.
func Register(prefix string, h http.Handler) {
	if !prefixPattern.MatchString(prefix) {
		panic("pageroutes: invalid route prefix " + prefix)
	}
	if h == nil {
		panic("pageroutes: nil handler for " + prefix)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, taken := routes[prefix]; taken {
		panic("pageroutes: route registered twice: " + prefix)
	}
	routes[prefix] = h
}

// Lookup returns the handler for a request path, when a registered prefix
// starts it.
func Lookup(path string) (http.Handler, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for prefix, h := range routes {
		if strings.HasPrefix(path, prefix) {
			return h, true
		}
	}
	return nil, false
}

// Prefixes lists the registered prefixes, sorted.
func Prefixes() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(routes))
	for p := range routes {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
