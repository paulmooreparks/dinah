package lsp

import (
	"net/url"
	"path/filepath"
	"strings"
)

// fileURI composes the file URI of an absolute path, which is the only shape
// of location this server hands a client.
//
// The path is normalised to forward slashes first, so a Windows path travels
// as the protocol spells one, and a drive letter's colon is percent-encoded
// by url.URL's own escaping rather than by a rule written here.
func fileURI(path string) string {
	if path == "" {
		return ""
	}
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	u := url.URL{Scheme: "file", Path: slashed}
	return u.String()
}

// uriPath reads a file URI back into a filesystem path, answering the empty
// string for a URI of any other scheme. A document the server cannot place on
// disk is one it does not look at, which is section 4.1's first condition.
func uriPath(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" {
		return ""
	}
	path := parsed.Path
	// A Windows path arrives as /C:/..., whose leading separator belongs to
	// the URI rather than to the path.
	if len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path)
}
