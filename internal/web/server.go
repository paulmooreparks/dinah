// Package web serves the bus's page: Paul's inbox, the group chats, the releases
// and who is on the bus, server-rendered in PUDL on the loopback address.
package web

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"studio/internal/bus"
	"studio/internal/profile"
	"studio/internal/regional"
	"studio/internal/skins"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// cookieName is the cookie that signs a browser in to the bus's pages, which
// the profile gives (profile.BusCookie).
var cookieName = profile.Current().BusCookie

//go:embed static/pudl/pudl-skins.json
var skinCatalogueSource []byte

// skinCatalogue is the release's catalogue of admitted skins; the build fails at once if it cannot be read.
var skinCatalogue = func() skins.Catalogue {
	c, err := skins.Parse(skinCatalogueSource)
	if err != nil {
		panic(err)
	}
	return c
}()

// skinHome is where the engine keeps the person's choices.
func skinHome() string {
	h, err := bus.Home()
	if err != nil {
		return ""
	}
	return h
}

// currentSkin is the skin to draw: the person's choice when this build admits it, else Standard.
func currentSkin() string { return skinCatalogue.Effective(skins.ReadPreference(skinHome())) }

// currentThemePref is the person's light, dark or system choice, or "" when they have made none.
func currentThemePref() string { return skins.ReadThemePreference(skinHome()) }

// skinLink is a skin's stylesheet and its catalogue ID.
type skinLink struct{ ID, File string }

// skinLinks are the stylesheets of every admitted skin, which the page links statically so PUDL registers them
// before it first chooses one.
func skinLinks() []skinLink {
	var out []skinLink
	for _, s := range skinCatalogue.Admitted() {
		if s.Stylesheet != nil {
			out = append(out, skinLink{ID: s.ID, File: *s.Stylesheet})
		}
	}
	return out
}

// Server is the page's HTTP server.
type Server struct {
	bus  *bus.Bus
	addr string // host:port the page is served on
	tmpl *template.Template
	mux  *http.ServeMux
}

// New returns a server for the bus that answers on addr.
func New(b *bus.Bus, addr string) (*Server, error) {
	funcs := template.FuncMap{
		"whenTime":  func(t time.Time) template.HTML { return when(t.Format(time.RFC3339Nano)) },
		"when":      when,
		"verdict":   func(v string) string { return bus.Verdicts[v] },
		"short":     func(s string) string { return s[:min(12, len(s))] },
		"pathEsc":   url.PathEscape,
		"fileState": fileState,
		// The person's skin and theme mode, read where the engine keeps them, so a page is drawn in them with no
		// script and no flash (docs/plans/pudl-skins.md).
		"skin":      currentSkin,
		"themePref": currentThemePref,
		"skinLinks": skinLinks,
		"dict": func(kv ...any) map[string]any {
			m := map[string]any{}
			for i := 0; i+1 < len(kv); i += 2 {
				m[kv[i].(string)] = kv[i+1]
			}
			return m
		},
	}
	t, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s := &Server{bus: b, addr: addr, tmpl: t, mux: http.NewServeMux()}
	static, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	s.mux.HandleFunc("GET /signin", s.signin)
	s.mux.HandleFunc("GET /{$}", s.desktop)
	s.mux.HandleFunc("GET /win/{key}", s.window)
	s.mux.HandleFunc("GET /inbox", s.inbox)
	s.mux.HandleFunc("GET /rooms", s.rooms)
	s.mux.HandleFunc("POST /rooms", s.createRoom)
	s.mux.HandleFunc("GET /rooms/{name}", s.room)
	s.mux.HandleFunc("POST /rooms/{name}/members", s.joinRoom)
	s.mux.HandleFunc("GET /releases", s.releases)
	s.mux.HandleFunc("GET /releases/{id}", s.release)
	s.mux.HandleFunc("GET /bus", s.busPage)
	s.mux.HandleFunc("GET /messages/{id}", s.thread)
	s.mux.HandleFunc("POST /messages", s.send)
	s.mux.HandleFunc("GET /events", s.events)
	return s, nil
}

// SigninURL returns the address that signs a browser in with a new code.
func (s *Server) SigninURL(ctx context.Context) (string, error) {
	return SigninURL(ctx, s.bus, s.addr)
}

// SigninURL returns the address that signs a browser in to the page served
// on addr.
func SigninURL(ctx context.Context, b *bus.Bus, addr string) (string, error) {
	code, err := b.NewWebCode(ctx)
	if err != nil {
		return "", err
	}
	return "http://" + addr + "/signin?code=" + code, nil
}

// ServeHTTP checks the request's host, sign-in and origin before routing it.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.hostAllowed(r.Host) {
		http.Error(w, "This page answers only on "+s.addr+".", http.StatusMisdirectedRequest)
		return
	}
	w.Header().Set("X-Frame-Options", "DENY")
	// data: images are allowed because PUDL draws its glyphs as SVG masks
	// written inline in its stylesheet; an image cannot run script.
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; form-action 'self'")
	// same-origin rather than no-referrer: under no-referrer a browser sends
	// "Origin: null" with form posts, which sameOrigin could not check.
	w.Header().Set("Referrer-Policy", "same-origin")
	if strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/signin" {
		s.mux.ServeHTTP(w, r)
		return
	}
	c, err := r.Cookie(cookieName)
	if err != nil || !s.bus.ValidWebToken(r.Context(), c.Value) {
		w.WriteHeader(http.StatusUnauthorized)
		s.render(w, "signedout.html", map[string]any{"Title": "Signed Out", "Product": profile.Current().Name, "Command": profile.Current().CommandProgram})
		return
	}
	if r.Method == http.MethodPost && !s.sameOrigin(r) {
		http.Error(w, "Forms are accepted only from this page.", http.StatusForbidden)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// hostAllowed accepts only the page's own host and port, which defeats DNS
// rebinding.
func (s *Server) hostAllowed(host string) bool {
	_, port, err := net.SplitHostPort(s.addr)
	if err != nil {
		return false
	}
	return host == s.addr || host == "localhost:"+port || host == "127.0.0.1:"+port
}

func (s *Server) sameOrigin(r *http.Request) bool {
	if o := r.Header.Get("Origin"); o != "" && o != "null" {
		u, err := url.Parse(o)
		return err == nil && u.Scheme == "http" && s.hostAllowed(u.Host) && u.Host == r.Host
	}
	return r.Header.Get("Sec-Fetch-Site") == "same-origin"
}

func (s *Server) signin(w http.ResponseWriter, r *http.Request) {
	token, err := s.bus.ExchangeWebCode(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		s.render(w, "signedout.html", map[string]any{"Title": "Signed Out", "Error": err.Error(), "Product": profile.Current().Name, "Command": profile.Current().CommandProgram})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: 365 * 24 * 3600})
	// The page moves on to the desktop itself rather than by a redirect. A
	// sign-in link followed from another site, as the Dinah window's first
	// page is, makes the whole redirect chain cross-site, and a browser
	// withholds a SameSite=Strict cookie from it; a navigation this page
	// starts is same-site, so the new cookie goes with it.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	name := template.HTMLEscapeString(profile.Current().Name)
	fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title>`+
		`<meta http-equiv="refresh" content="0; url=/"></head><body><p><a href="/">Open %s</a></p></body></html>`, name, name)
}

type page struct {
	// Product is the product's name as people see it (internal/profile).
	Product  string
	Title    string
	Section  string
	Counter  int64
	Unread   int
	Error    string
	Data     map[string]any
	Targets  []target
	Selected string
	// Key is the window key of the resource the page shows. Win is set when
	// the page is wanted as a window of the desktop rather than as a page,
	// and Self is the resource's own page.
	Key  string
	Win  bool
	Self string
}

type target struct {
	Value, Label, Group string
}

// newPage starts a page about the resource with the given window key.
func (s *Server) newPage(r *http.Request, key, title, section string) (*page, error) {
	ctx := r.Context()
	n, err := s.bus.ChangeCounter(ctx)
	if err != nil {
		return nil, err
	}
	p := &page{Product: profile.Current().Name, Title: title, Section: section, Counter: n, Data: map[string]any{}, Error: r.URL.Query().Get("error"),
		Key: key, Win: windowed(r), Self: r.URL.Path}
	inbox, err := s.bus.HumanInbox(ctx, 200, false)
	if err != nil {
		return nil, err
	}
	for _, m := range inbox {
		if m.Unread {
			p.Unread++
		}
	}
	ws, err := s.bus.Workspaces(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range ws {
		p.Targets = append(p.Targets, target{w.Name, w.Display(), "Workspaces"})
	}
	ss, err := s.bus.Sessions(ctx)
	if err != nil {
		return nil, err
	}
	// A chat is sent to by its title and its workspace, as agents send to
	// it, and shown with its workspace's label.
	for _, x := range ss {
		p.Targets = append(p.Targets, target{x.Address(), x.Display() + " (" + x.WorkspaceLabel + ")", "Chats"})
	}
	rooms, err := s.bus.Rooms(ctx)
	if err != nil {
		return nil, err
	}
	for _, rm := range rooms {
		p.Targets = append(p.Targets, target{"#" + rm.Name, rm.Title, "Group Chats"})
	}
	return p, nil
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		fmt.Fprintf(w, "<p>The page could not be drawn: %s</p>", template.HTMLEscapeString(err.Error()))
	}
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	if errors.Is(err, bus.ErrNotFound) {
		code = http.StatusNotFound
	}
	http.Error(w, err.Error(), code)
}

func (s *Server) inbox(w http.ResponseWriter, r *http.Request) {
	// Mark the messages read before the page takes the change counter, so
	// the page does not see its own write as news and reload.
	msgs, err := s.bus.HumanInbox(r.Context(), 100, true)
	if err != nil {
		s.fail(w, err)
		return
	}
	p, err := s.newPage(r, "inbox", "Inbox", "inbox")
	if err != nil {
		s.fail(w, err)
		return
	}
	p.Data["Messages"] = msgs
	p.Selected = r.URL.Query().Get("to")
	s.render(w, "inbox.html", p)
}

func (s *Server) rooms(w http.ResponseWriter, r *http.Request) {
	p, err := s.newPage(r, "rooms", "Group Chats", "rooms")
	if err != nil {
		s.fail(w, err)
		return
	}
	rooms, err := s.bus.Rooms(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	p.Data["Rooms"] = rooms
	s.render(w, "rooms.html", p)
}

func (s *Server) room(w http.ResponseWriter, r *http.Request) {
	room, err := s.bus.Room(r.Context(), r.PathValue("name"))
	if err != nil {
		s.fail(w, err)
		return
	}
	p, err := s.newPage(r, "room-"+room.Name, room.Title, "rooms")
	if err != nil {
		s.fail(w, err)
		return
	}
	msgs, err := s.bus.RoomMessages(r.Context(), room.Name, 200)
	if err != nil {
		s.fail(w, err)
		return
	}
	// A line saying who joined or left is written as a sentence.
	person := s.bus.PersonName()
	if person == bus.PersonFallback {
		person = "You"
	}
	lines := map[int64]string{}
	for _, m := range msgs {
		if m.Kind == bus.MemberKind {
			lines[m.ID] = m.MemberLine(person)
		}
	}
	p.Data["Room"] = room
	p.Data["Messages"] = msgs
	p.Data["Lines"] = lines
	p.Selected = "#" + room.Name
	s.render(w, "room.html", p)
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	// Members are separated by commas only, since a chat's title has spaces.
	members := strings.Split(r.FormValue("members"), ",")
	room, err := s.bus.CreateGroup(r.Context(), bus.Sender{}, name, members)
	if err != nil {
		redirectError(w, r, "/rooms", err)
		return
	}
	http.Redirect(w, r, "/rooms/"+url.PathEscape(room.Name), http.StatusSeeOther)
}

func (s *Server) joinRoom(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	back := "/rooms/" + url.PathEscape(name)
	if err := s.bus.AddToGroup(r.Context(), bus.Sender{}, name, []string{r.FormValue("member")}); err != nil {
		redirectError(w, r, back, err)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *Server) releases(w http.ResponseWriter, r *http.Request) {
	p, err := s.newPage(r, "releases", "Releases", "releases")
	if err != nil {
		s.fail(w, err)
		return
	}
	rs, err := s.bus.Releases(r.Context(), 50)
	if err != nil {
		s.fail(w, err)
		return
	}
	p.Data["Releases"] = rs
	s.render(w, "releases.html", p)
}

func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	rel, err := s.bus.ReleaseByID(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	p, err := s.newPage(r, "release-"+strconv.FormatInt(id, 10), rel.FromWorkspaceLabel+" "+rel.Version, "releases")
	if err != nil {
		s.fail(w, err)
		return
	}
	p.Data["Release"] = rel
	s.render(w, "release.html", p)
}

func (s *Server) busPage(w http.ResponseWriter, r *http.Request) {
	p, err := s.newPage(r, "bus", "Bus", "bus")
	if err != nil {
		s.fail(w, err)
		return
	}
	ctx := r.Context()
	ws, err := s.bus.Workspaces(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	ss, err := s.bus.Sessions(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	waiting, err := s.bus.Waiting(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	recent, err := s.bus.Recent(ctx, 100)
	if err != nil {
		s.fail(w, err)
		return
	}
	p.Data["Workspaces"] = ws
	p.Data["Sessions"] = ss
	p.Data["Waiting"] = waiting
	p.Data["Recent"] = recent
	s.render(w, "bus.html", p)
}

func (s *Server) thread(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	msgs, err := s.bus.Thread(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	p, err := s.newPage(r, "msg-"+strconv.FormatInt(id, 10), fmt.Sprintf("Message %d", id), "")
	if err != nil {
		s.fail(w, err)
		return
	}
	p.Data["Messages"] = msgs
	p.Data["Root"] = msgs[0]
	s.render(w, "thread.html", p)
}

func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	back := r.FormValue("back")
	if !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") {
		back = "/inbox"
	}
	replyTo, _ := strconv.ParseInt(r.FormValue("reply_to"), 10, 64)
	if _, err := s.bus.Send(r.Context(), bus.Sender{}, r.FormValue("to"), r.FormValue("text"), nil, replyTo); err != nil {
		redirectError(w, r, back, err)
		return
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func redirectError(w http.ResponseWriter, r *http.Request, back string, err error) {
	sep := "?"
	if strings.Contains(back, "?") {
		sep = "&"
	}
	http.Redirect(w, r, back+sep+"error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
}

// events tells the page when the bus changes, so it can reload.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	last := int64(-1)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		n, err := s.bus.ChangeCounter(r.Context())
		if err == nil && n != last {
			fmt.Fprintf(w, "data: %d\n\n", n)
			fl.Flush()
			last = n
		}
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		}
	}
}

// when writes a bus time as a time element. Its text is the time of day for
// a time today and the day, month and time otherwise, and the page script
// writes it again in the computer's regional format, whose language tag is
// the element's data-locale (dinah.js). Without script the text stays.
func when(stamp string) template.HTML {
	t, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return template.HTML(template.HTMLEscapeString(stamp))
	}
	t = t.Local()
	text := t.Format("2 Jan 15:04")
	if y, m, d := time.Now().Date(); t.Year() == y && t.Month() == m && t.Day() == d {
		text = t.Format("15:04")
	}
	return template.HTML(`<time datetime="` + t.Format(time.RFC3339) + `" data-locale="` + template.HTMLEscapeString(regional.HostLocale()) + `">` + text + `</time>`)
}

// fileState says whether an attachment has changed since it was sent.
func fileState(a bus.Attachment) string {
	sum, err := bus.FileHash(a.Path)
	switch {
	case err != nil:
		return "missing"
	case sum != a.SHA256:
		return "changed"
	default:
		return "unchanged"
	}
}
