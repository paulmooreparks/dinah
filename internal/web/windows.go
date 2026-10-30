package web

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"

	"studio/internal/profile"
)

// Dinah's desktop is PUDL's windowed mode: the address holds the open
// windows and their places, the server renders the windows the address
// names, and each window's content comes from /win/{key}. A window is
// another representation of a resource that also has its own page, and the
// key says which resource:
//
//	inbox, rooms, releases, bus          the lists
//	room-<name>                          a room
//	release-<id>                         a release and its verdicts
//	msg-<id>                             a message's thread

// keyPath returns the page a window key stands for, or "" for a key that
// names nothing Dinah knows.
func keyPath(key string) string {
	switch key {
	case "inbox", "rooms", "releases", "bus":
		return "/" + key
	}
	kind, id, ok := strings.Cut(key, "-")
	if !ok || id == "" {
		return ""
	}
	switch kind {
	case "room":
		return "/rooms/" + url.PathEscape(id)
	case "release":
		if digits.MatchString(id) {
			return "/releases/" + id
		}
	case "msg":
		if digits.MatchString(id) {
			return "/messages/" + id
		}
	}
	return ""
}

var (
	digits   = regexp.MustCompile(`^[0-9]+$`)
	keyChars = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

type windowCtx struct{}

// windowed says whether a request wants its answer as a window: a request
// made through /win/{key}, or a form sent from inside a window, which says
// so with the X-PUDL-Window header so that its result comes back as window
// content rather than as a page.
func windowed(r *http.Request) bool {
	if _, ok := r.Context().Value(windowCtx{}).(string); ok {
		return true
	}
	k := r.Header.Get("X-PUDL-Window")
	return k != "" && keyChars.MatchString(k)
}

// window serves a resource as a window, by handing the request to the
// resource's own page marked as wanted in a window.
func (s *Server) window(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	path := keyPath(key)
	if path == "" {
		http.NotFound(w, r)
		return
	}
	r2 := r.Clone(context.WithValue(r.Context(), windowCtx{}, key))
	r2.URL.Path, r2.URL.RawPath, r2.URL.RawQuery = path, "", ""
	w.Header().Set("Cache-Control", "no-store")
	s.mux.ServeHTTP(w, r2)
}

// app is an entry in the desktop's launcher.
type app struct {
	Key, Title, Path string
}

var apps = []app{
	{"inbox", "Inbox", "/inbox"},
	{"rooms", "Rooms", "/rooms"},
	{"releases", "Releases", "/releases"},
	{"bus", "Bus", "/bus"},
}

// defaultWindows open on an address that names no windows.
const defaultWindows = "inbox"

// desktop serves the desktop with the windows its address names rendered
// into it, as PUDL's window contract asks of a server, so the windows are
// there without script.
func (s *Server) desktop(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	open := defaultWindows
	if q.Has("open") {
		open = q.Get("open")
	}
	var wins strings.Builder
	for _, key := range strings.Split(open, ",") {
		if keyPath(key) == "" {
			continue
		}
		rec := httptest.NewRecorder()
		r2 := r.Clone(r.Context())
		r2.SetPathValue("key", key)
		s.window(rec, r2)
		if rec.Code == http.StatusOK {
			wins.WriteString(rec.Body.String())
		}
	}
	p, err := s.newPage(r, "", profile.Current().Name, "")
	if err != nil {
		s.fail(w, err)
		return
	}
	p.Data["Windows"] = template.HTML(wins.String())
	p.Data["Apps"] = apps
	p.Data["Defaults"] = defaultWindows
	s.render(w, "desktop.html", p)
}
