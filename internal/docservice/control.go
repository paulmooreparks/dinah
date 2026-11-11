package docservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// pollWait is how long a window process's question waits for a command.
const pollWait = 20 * time.Second

// invokeWait is how long an action waits for the page, which asks the
// person first.
const invokeWait = 10 * time.Minute

// pictureTypes are the kinds of file a page may show from beside its
// document.
var pictureTypes = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".bmp": true, ".avif": true, ".ico": true}

// ServeHTTP answers on the service's own address only, which defeats a page
// elsewhere that reaches this port by another name. Under /control is the
// control interface, which wants the service's token. Under /d/ and a
// document's ID is that document's page, which wants the document's cookie.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.hostAllowed(r.Host) {
		http.Error(w, "The document service answers only on "+s.addr+".", http.StatusMisdirectedRequest)
		return
	}
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Cache-Control", "no-store")
	switch {
	case strings.HasPrefix(r.URL.Path, "/control/"):
		s.serveControl(w, r)
	case strings.HasPrefix(r.URL.Path, "/d/"):
		s.servePage(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Service) hostAllowed(host string) bool {
	_, port, err := net.SplitHostPort(s.addr)
	if err != nil {
		return false
	}
	return host == s.addr || host == "localhost:"+port
}

func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" || o == "null" {
		return r.Header.Get("Sec-Fetch-Site") == "same-origin"
	}
	u, err := url.Parse(o)
	return err == nil && u.Scheme == "http" && u.Host == r.Host
}

// servePage answers one document's page: the way in, the page's files, its
// bridge, and the pictures beside its files.
func (s *Service) servePage(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/d/")
	id, sub, _ := strings.Cut(rest, "/")
	d := s.doc(id)
	if d == nil {
		http.Error(w, "That document's window has closed.", http.StatusNotFound)
		return
	}
	prefix := "/d/" + id
	sub = "/" + sub
	h := w.Header()
	policy := s.opt.Policy
	if own, ok := s.opt.Policies[sub]; ok {
		policy = own
	}
	if policy != "" {
		h.Set("Content-Security-Policy", policy)
	}
	h.Set("X-Frame-Options", "SAMEORIGIN")
	if sub == "/enter" {
		s.enter(w, r, d, prefix)
		return
	}
	c, err := r.Cookie(s.opt.Cookie)
	if err != nil || !sameToken(c.Value, d.token) {
		http.Error(w, "Open the document from its window.", http.StatusUnauthorized)
		return
	}
	read := r.Method == http.MethodGet || r.Method == http.MethodHead
	if !read && !sameOrigin(r) {
		http.Error(w, "Calls are accepted only from the document's own page.", http.StatusForbidden)
		return
	}
	switch {
	case strings.HasPrefix(sub, "/bridge/"):
		d.bridge.Handler(prefix+"/bridge").ServeHTTP(w, r)
	case sub == "/file" && read:
		s.servePicture(w, r, d)
	case read:
		r2 := r.Clone(r.Context())
		r2.URL.Path = sub
		r2.URL.RawPath = ""
		s.opt.Files.ServeHTTP(w, r2)
	default:
		http.Error(w, "A document's page reads its files and calls its bridge.", http.StatusForbidden)
	}
}

// enter exchanges a one-time code for the document's cookie, which is sent
// back only under the document's own path, and moves on to the page with a
// page of its own, as the main window's way in does (a redirect chain from
// the window toolkit's page would be cross-site, and a browser withholds a
// SameSite=Strict cookie from one).
func (s *Service) enter(w http.ResponseWriter, r *http.Request, d *document, prefix string) {
	code := r.URL.Query().Get("code")
	s.mu.Lock()
	entry, ok := s.codes[code]
	delete(s.codes, code)
	s.mu.Unlock()
	if !ok || entry.doc != d || time.Now().After(entry.expires) {
		http.Error(w, "That way in has expired. Open the document from its window.", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.opt.Cookie, Value: d.token, Path: prefix + "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title>`+
		`<meta http-equiv="refresh" content="0; url=%s"></head><body></body></html>`, html.EscapeString(d.title()), html.EscapeString(prefix+"/"+s.opt.Page))
}

// servePicture serves a picture a document shows by a name of its own, from
// beside one of the page's files, as the file provider finds such a file,
// or by a site path, from the document's web root (webroot.go).
func (s *Service) servePicture(w http.ResponseWriter, r *http.Request, d *document) {
	page := &Page{d: d}
	path, err := page.file(r.URL.Query().Get("doc"))
	name := r.URL.Query().Get("name")
	kind := strings.ToLower(filepath.Ext(name))
	if err != nil || !pictureTypes[kind] {
		http.NotFound(w, r)
		return
	}
	var data []byte
	if strings.HasPrefix(name, "/") {
		// A site path, read from the web root the person chose.
		data, err = s.readUnderWebRoot(path, name)
	} else {
		folder, base := folderOf(path)
		data, err = folder.ReadFile(r.Context(), base, name)
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ctype := mime.TypeByExtension(kind)
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	// A picture is shown and never run: an SVG served as a document of its
	// own may run nothing and load nothing.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Write(data)
}

// serveControl answers the control interface, for whoever has the token in
// the service's file: a window process, and another program of this product.
func (s *Service) serveControl(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !sameToken(token, s.token) {
		http.Error(w, "The document service's control interface wants its token.", http.StatusUnauthorized)
		return
	}
	// A browser's page cannot send the token, and is refused all the same
	// if it somehow does: the interface is for programs.
	if r.Header.Get("Origin") != "" {
		http.Error(w, "The control interface is not for pages.", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/control")
	post := r.Method == http.MethodPost
	get := r.Method == http.MethodGet
	switch {
	case path == "/ping" && get:
		reply(w, Endpoint{Addr: s.addr, PID: 0, Version: s.opt.Version})
	case path == "/open" && post:
		var req OpenRequest
		if !body(w, r, &req) {
			return
		}
		opened, err := s.Open(req)
		answer(w, opened, err)
	case path == "/start" && post:
		// Another program asks for a window: the service has it started.
		var req OpenRequest
		if !body(w, r, &req) {
			return
		}
		answer(w, struct{}{}, s.startWindow(req.Application, req.Path, ""))
	case path == "/documents" && get:
		reply(w, s.Documents())
	case path == "/started" && get:
		reply(w, s.Started())
	case path == "/dialogs" && post:
		var req struct {
			Open []string `json:"open"`
			Save []string `json:"save"`
			// Folder is what each folder dialog will choose.
			Folder []string `json:"folder"`
		}
		if !body(w, r, &req) {
			return
		}
		if !s.opt.Headless {
			answer(w, nil, errors.New("only a headless service has its dialogs answered for it"))
			return
		}
		s.SetDialogs(req.Open, req.Save)
		s.SetFolderDialogs(req.Folder)
		reply(w, struct{}{})
	case path == "/stop" && post:
		reply(w, struct{}{})
		s.Stop()
	case strings.HasPrefix(path, "/window/"):
		s.serveWindow(w, r, strings.TrimPrefix(path, "/window/"))
	case strings.HasPrefix(path, "/documents/") && strings.HasSuffix(path, "/invoke") && post:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/documents/"), "/invoke")
		d := s.doc(id)
		if d == nil {
			answer(w, nil, errors.New("that document is not open"))
			return
		}
		var req struct {
			Request json.RawMessage `json:"request"`
			From    string          `json:"from"`
		}
		if !body(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), invokeWait)
		defer cancel()
		d.link.send("focus", nil)
		result, err := d.invoke(ctx, req.Request, req.From)
		answer(w, result, err)
	default:
		http.NotFound(w, r)
	}
}

// serveWindow answers a document's own window process.
func (s *Service) serveWindow(w http.ResponseWriter, r *http.Request, rest string) {
	id, what, _ := strings.Cut(rest, "/")
	d := s.doc(id)
	if d == nil {
		http.Error(w, "That document's window has closed.", http.StatusGone)
		return
	}
	post := r.Method == http.MethodPost
	switch {
	case what == "next" && r.Method == http.MethodGet:
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		pid, _ := strconv.Atoi(r.URL.Query().Get("pid"))
		commands := d.link.after(r.Context(), after, pollWait, pid)
		if commands == nil {
			commands = []Command{}
		}
		reply(w, commands)
	case what == "reply" && post:
		var req struct {
			Seq    int64           `json:"seq"`
			Result json.RawMessage `json:"result"`
		}
		if !body(w, r, &req) {
			return
		}
		d.link.reply(req.Seq, req.Result)
		reply(w, struct{}{})
	case what == "focus" && post:
		d.link.send("focus", nil)
		reply(w, struct{}{})
	case what == "entry" && post:
		// A window that loads its page again needs a new way in.
		reply(w, Opened{ID: d.id, Entry: s.entry(d), Title: d.title()})
	case what == "closing" && post:
		reply(w, map[string]bool{"ask": d.closing(r.Context())})
	case what == "closed" && post:
		d.closed()
		reply(w, struct{}{})
	default:
		http.NotFound(w, r)
	}
}

func body(w http.ResponseWriter, r *http.Request, into any) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if err == nil && len(data) > 0 {
		err = json.Unmarshal(data, into)
	}
	if err != nil {
		http.Error(w, "The request could not be read.", http.StatusBadRequest)
		return false
	}
	return true
}

func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// answer replies with a value, or with an error's text as a refusal the
// caller reads.
func answer(w http.ResponseWriter, v any, err error) {
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, err.Error())
		return
	}
	reply(w, v)
}
