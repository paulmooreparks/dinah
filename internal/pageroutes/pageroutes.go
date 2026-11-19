// Package pageroutes is the registry of the HTTP routes a product built from
// this source adds to the page's own server (docs/design/product-applets.md):
// files an applet of the product serves to the page under a path prefix of
// its own, such as a client's fonts or an embedded app's files. The shell
// server asks the registry for a request its own routes do not take. A route
// added with Register is served behind the same cookie that guards the
// page's files, so it is reached by the page alone. A route added with
// RegisterSelfGuarded is served without that cookie, for files that a
// sandboxed frame loads: such a frame has no origin of its own, so the
// server cannot count on a browser sending the page's cookie with its
// requests, and the handler admits a request by an unguessable token in its
// path instead. Nothing here knows a route; Dinah's own build registers
// none.
package pageroutes

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type route struct {
	handler     http.Handler
	selfGuarded bool
}

var (
	mu     sync.RWMutex
	routes = map[string]route{}
)

var prefixPattern = regexp.MustCompile(`^/[a-z][a-z0-9-]*/$`)

// Register adds a handler for every request whose path starts with prefix,
// which is "/<name>/", lowercase words joined by hyphens, served behind the
// page's cookie. It panics on a malformed or repeated prefix, since both are
// mistakes in the build.
func Register(prefix string, h http.Handler) { add(prefix, h, false) }

// RegisterSelfGuarded adds a handler as Register does, but the shell server
// gives it requests without asking for the page's cookie, and only GET and
// HEAD requests. The handler must refuse every request whose path does not
// contain a token it issued, one long and random enough that it cannot be
// guessed, and must serve nothing a holder of that token is not meant to
// read. It is for files loaded by a sandboxed frame, whose requests a
// browser is free to send without the page's cookie.
func RegisterSelfGuarded(prefix string, h http.Handler) { add(prefix, h, true) }

func add(prefix string, h http.Handler, selfGuarded bool) {
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
	routes[prefix] = route{handler: h, selfGuarded: selfGuarded}
}

func lookup(path string, selfGuarded bool) (http.Handler, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for prefix, r := range routes {
		if r.selfGuarded == selfGuarded && strings.HasPrefix(path, prefix) {
			return r.handler, true
		}
	}
	return nil, false
}

// Lookup returns the handler of a route added with Register for a request
// path, when its prefix starts the path.
func Lookup(path string) (http.Handler, bool) { return lookup(path, false) }

// LookupSelfGuarded returns the handler of a route added with
// RegisterSelfGuarded for a request path, when its prefix starts the path.
func LookupSelfGuarded(path string) (http.Handler, bool) { return lookup(path, true) }

// Prefixes lists the registered prefixes of both kinds, sorted.
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
