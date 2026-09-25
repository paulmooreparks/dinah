// Package browser opens the reader's own browser on the root URL of a server
// this process is running, through the one documented mechanism each
// platform offers for opening a URL in its registered handler.
//
// It opens nothing else. Open refuses any value other than an http URL naming
// a host and the root path, before any platform mechanism is reached, so no
// string a request or a file could carry is ever handed to a program.
package browser

import (
	"errors"
	"net/url"
	"strings"
)

// errNotARoot is what Open answers for a value it will not hand on.
var errNotARoot = errors.New("only an http URL naming a host and the root path is opened")

// Open opens the reader's browser on an http URL whose path is the root. The
// URL reaches the platform mechanism as one argument and through no shell.
func Open(address string) error {
	if !httpRoot(address) {
		return errNotARoot
	}
	return open(address)
}

// httpRoot reports whether a value is exactly "http://" followed by a host and
// "/": parsed with scheme http, a non-empty host, the path "/", no user
// information, no opaque part, no query and no fragment, and carrying no byte
// below 0x21 and none of the double quote, the angle brackets and the
// backslash. The value must also equal "http://" + host + "/" byte for byte,
// which is what refuses a bare "?" or "#", neither of which url.Parse keeps,
// and an escaped path such as /%2F, which it decodes.
func httpRoot(address string) bool {
	for i := 0; i < len(address); i++ {
		if address[i] < 0x21 {
			return false
		}
	}
	if strings.ContainsAny(address, `"<>\`) {
		return false
	}
	u, err := url.Parse(address)
	if err != nil {
		return false
	}
	if u.Scheme != "http" || u.Host == "" || u.Path != "/" || u.User != nil ||
		u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return false
	}
	return address == "http://"+u.Host+"/"
}
