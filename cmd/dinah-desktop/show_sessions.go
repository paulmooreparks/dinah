package main

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"studio/internal/bridge"
	"studio/internal/procjob"
	"studio/internal/shortcodes"
)

// A slide show on another display is a page in a second copy of the window
// program (show_window.go). That page and the editing window's are in
// different browser processes, so the main program carries their messages:
// the editing window posts through the App, and the show's page through a
// bridge of its own, which also gives it the few things a slide is drawn
// from. The messages are Slides' own, JSON text this program does not read
// (docs/design/detached-slide-show.md, and yavc.app's presentation contract,
// version 2).

// showPagePath is the show's page among the page's files.
const showPagePath = "show.html"

// showMessageLimit is the most bytes one message may have. Slides bounds a
// deck's text at 2 MiB of UTF-16 code units, which is at most three times
// that as JSON text.
const showMessageLimit = 8 << 20

// showPendingLimit is the most bytes kept for a show's page that has not
// started reading yet.
const showPendingLimit = 32 << 20

// ShowRequest is what the editing window asks a show for: the application
// and the deck's resource as the applet names them, the draft the deck's
// pictures are served from, the folder its relative paths start at, and
// whether the window is drawn light or dark, which the show's page follows.
type ShowRequest struct {
	Application string `json:"application"`
	Resource    string `json:"resource"`
	Draft       string `json:"draft"`
	Folder      string `json:"folder"`
	Mode        string `json:"mode"`
}

// ShowOpened is the answer: the show's ID, or none when there is no other
// display and the show stays in the main window.
type ShowOpened struct {
	ID string `json:"id"`
}

// ShowAttached is what the show's page learns when it starts reading: which
// deck it shows, the product whose page it is, and the messages posted
// before it was ready, in order.
type ShowAttached struct {
	ID          string      `json:"id"`
	Application string      `json:"application"`
	Resource    string      `json:"resource"`
	Draft       string      `json:"draft"`
	Folder      string      `json:"folder"`
	Mode        string      `json:"mode"`
	Product     ProductInfo `json:"product"`
	Messages    []string    `json:"messages"`
}

// showProcess is the show's process as the main program holds it.
type showProcess interface {
	Kill() error
	Wait() error
}

type showSession struct {
	id, token string
	request   ShowRequest
	bridge    *bridge.Bridge
	process   showProcess

	mu       sync.Mutex
	attached bool
	pending  []string
	held     int
}

// showHost is the App's one show. A second show ends the first: two shows
// cannot both fill the other display.
type showHost struct {
	mu      sync.Mutex
	current *showSession
	// entry gives an address that signs a show's page in (shell_server.go).
	entry func(*showSession) string
}

// startShowProcess starts the show's window on a display. Under test no
// window is started: the address is written beside the main page's own, and
// the test opens it in a browser of its own, as it does the main page.
var startShowProcess = func(entry string, place showDisplay) (showProcess, error) {
	if file := os.Getenv(product.ShellEntryVar()); file != "" {
		if err := os.WriteFile(file+".show", []byte(entry), 0o600); err != nil {
			return nil, err
		}
		return &absentShowProcess{done: make(chan struct{})}, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, showOption)
	cmd.Env = append(os.Environ(), showEntryVar()+"="+entry, showPlaceVar()+"="+formatShowRect(place))
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// The show never outlives the program that started it.
	procjob.Tie(cmd.Process.Pid)
	return execShowProcess{cmd}, nil
}

type execShowProcess struct{ cmd *exec.Cmd }

func (p execShowProcess) Kill() error { return p.cmd.Process.Kill() }
func (p execShowProcess) Wait() error { return p.cmd.Wait() }

// absentShowProcess stands for the window a test does not start.
type absentShowProcess struct {
	once sync.Once
	done chan struct{}
}

func (p *absentShowProcess) Kill() error { p.once.Do(func() { close(p.done) }); return nil }
func (p *absentShowProcess) Wait() error { <-p.done; return nil }

// OpenShow starts a slide show on another display and answers with its ID.
// With no other display it starts nothing and answers with no ID, at once,
// so that Slides can still present in its own window. The window appears a
// little after the answer, and what is posted meanwhile is kept for it.
func (a *App) OpenShow(r ShowRequest) (ShowOpened, error) {
	place, ok := chooseShowDisplay(showDisplays())
	if !ok {
		return ShowOpened{}, nil
	}
	if r.Resource == "" || r.Draft == "" {
		return ShowOpened{}, errors.New("a show needs the deck it presents")
	}
	h := &a.shows
	h.mu.Lock()
	entry := h.entry
	h.mu.Unlock()
	if entry == nil {
		return ShowOpened{}, errors.New("the window is not serving its page yet")
	}
	s := &showSession{id: rand.Text(), token: rand.Text(), request: r}
	s.bridge = bridge.New(&showPage{app: a, s: s})
	process, err := startShowProcess(entry(s), place)
	if err != nil {
		return ShowOpened{}, err
	}
	s.process = process
	h.mu.Lock()
	previous := h.current
	h.current = s
	h.mu.Unlock()
	if previous != nil {
		previous.process.Kill()
	}
	go func() {
		process.Wait()
		h.mu.Lock()
		ended := h.current == s
		if ended {
			h.current = nil
		}
		h.mu.Unlock()
		// The editing window is told whenever a show's window is gone. It
		// already knows of one it closed itself, and learns here of one the
		// person closed.
		a.emit("show:closed", map[string]any{"id": s.id})
	}()
	return ShowOpened{ID: s.id}, nil
}

func (a *App) show(id string) *showSession {
	a.shows.mu.Lock()
	defer a.shows.mu.Unlock()
	if s := a.shows.current; s != nil && id != "" && s.id == id {
		return s
	}
	return nil
}

// showByToken is the show whose page signed in with token.
func (a *App) showByToken(token string) *showSession {
	a.shows.mu.Lock()
	defer a.shows.mu.Unlock()
	if s := a.shows.current; s != nil && token != "" && subtle.ConstantTimeCompare([]byte(s.token), []byte(token)) == 1 {
		return s
	}
	return nil
}

// PostShow passes one of the editing window's messages to the show's page,
// or keeps it until that page reads.
func (a *App) PostShow(id, message string) error {
	s := a.show(id)
	if s == nil {
		return errors.New("that show has ended")
	}
	if len(message) > showMessageLimit {
		return errors.New("the message is too large for a show")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.attached {
		s.bridge.Emit("message", message)
		return nil
	}
	if s.held+len(message) > showPendingLimit {
		return errors.New("the show's window has not started reading yet")
	}
	s.pending = append(s.pending, message)
	s.held += len(message)
	return nil
}

// CloseShow ends a show and its window. Ending one that is gone is no error.
func (a *App) CloseShow(id string) error {
	if s := a.show(id); s != nil {
		return s.process.Kill()
	}
	return nil
}

// closeShows ends the show when the program does.
func (a *App) closeShows() {
	a.shows.mu.Lock()
	s := a.shows.current
	a.shows.mu.Unlock()
	if s != nil {
		s.process.Kill()
	}
}

// showPage is everything the show's page may ask of the program: its
// messages, and what a slide is drawn from, which the Slides canvas reads in
// the main window too. It can write nothing and reaches no chat, workspace
// list or setting.
type showPage struct {
	app *App
	s   *showSession
}

// Attach gives the page its deck and what was posted before it read. The
// page listens for messages first and calls this second, so none is lost
// between the two.
func (p *showPage) Attach() ShowAttached {
	s := p.s
	s.mu.Lock()
	defer s.mu.Unlock()
	out := ShowAttached{ID: s.id, Application: s.request.Application, Resource: s.request.Resource, Draft: s.request.Draft,
		Folder: s.request.Folder, Mode: s.request.Mode, Product: productInfo(), Messages: s.pending}
	if out.Messages == nil {
		out.Messages = []string{}
	}
	s.attached, s.pending, s.held = true, nil, 0
	return out
}

// Post passes one of the show's messages to the editing window.
func (p *showPage) Post(message string) error {
	if len(message) > showMessageLimit {
		return errors.New("the message is too large for a show")
	}
	if p.app.show(p.s.id) == nil {
		return errors.New("that show has ended")
	}
	p.app.emit("show:message", map[string]any{"id": p.s.id, "message": message})
	return nil
}

// ReadAppearance gives the page the person's theme and skin, so that a
// slide is drawn as the window draws it. The page cannot save one.
func (p *showPage) ReadAppearance() (AppearanceSnapshot, error) { return p.app.ReadAppearance() }

func (p *showPage) ShortcodeCatalog() []shortcodes.Descriptor { return p.app.ShortcodeCatalog() }

func (p *showPage) RenderShortcode(r shortcodes.Request) (shortcodes.Drawing, error) {
	return p.app.RenderShortcode(r)
}

func (p *showPage) ReadWorkspaceMountText(path string, maxBytes int) (MountText, error) {
	return p.app.ReadWorkspaceMountText(path, maxBytes)
}

// serveShow answers the show's page: the page's files, its bridge, and the
// pictures of the one draft it presents. Anything else is refused.
func (s *shellServer) serveShow(w http.ResponseWriter, r *http.Request, show *showSession) {
	read := r.Method == http.MethodGet || r.Method == http.MethodHead
	switch {
	case len(r.URL.Path) >= 8 && r.URL.Path[:8] == "/bridge/":
		if !read && !s.sameOrigin(r) {
			http.Error(w, "Calls are accepted only from "+product.Name+"'s own page.", http.StatusForbidden)
			return
		}
		show.bridge.Handler("/bridge").ServeHTTP(w, r)
	case r.URL.Path == "/draft-file" && read && r.URL.Query().Get("draft") == show.request.Draft:
		s.app.docFiles().ServeHTTP(w, r)
	case r.URL.Path == "/draft-file" || r.URL.Path == "/doc-file" || r.URL.Path == "/resource-attachment" || r.URL.Path == "/bus-attachment" || r.URL.Path == "/upload":
		http.Error(w, "A slide show reads only its own deck.", http.StatusForbidden)
	case read:
		s.files.ServeHTTP(w, r)
	default:
		http.Error(w, "A slide show reads only its own deck.", http.StatusForbidden)
	}
}
