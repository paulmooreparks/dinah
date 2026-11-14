// Package docservice is the document service of the stand-alone document
// applications (docs/proposals/standalone-document-apps.md, and
// docs/design/standalone-documents.md). A stand-alone application shows one
// document in one window, and each window is a process of its own, so what
// the windows share is here, in one process for the person: which documents
// are open, the files themselves, the unsaved text kept for recovery, and
// the applications' settings.
//
// The service has two faces on one loopback address. A window's page signs
// in with a one-time address and a cookie that reaches its own document and
// nothing else (page.go). A window's process, and any other program of this
// product that wants a document opened or acted on, speaks the control
// interface with the token in the service's own file (control.go,
// client.go).
//
// The service keeps no copy of a document. A read is the file as it is, a
// write is refused when the file is no longer what the writer read, and the
// text the person has not saved is in the recovery record the page keeps
// here (store.go).
package docservice

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"studio/internal/bridge"
)

// Application is one of the stand-alone applications: the applet it mounts
// and the name its window has.
type Application struct {
	// Applet is the name of the applet's package, as the page mounts it.
	Applet string `json:"applet"`
	// Name is the application's name, as its window and taskbar show it.
	Name string `json:"name"`
	// FileApplets selects an applet by the file's lower-case extension.
	// The application keeps its process identity, grants and recovery keys.
	FileApplets map[string]string `json:"fileApplets,omitempty"`
}

func (a Application) fileApplet(path string) string {
	if selected := a.FileApplets[strings.ToLower(filepath.Ext(path))]; selected != "" {
		return selected
	}
	return a.Applet
}

// Options are what the program gives the service.
type Options struct {
	// Dir is the folder the service keeps its files in.
	Dir string
	// Version is the release of the program, which a client compares with
	// its own.
	Version string
	// Files serves the page's files.
	Files http.Handler
	// Page is the page's file among them.
	Page string
	// Cookie is the name of the cookie that signs a page in.
	Cookie string
	// Policy is the page's Content Security Policy. A path in Policies has
	// its own.
	Policy   string
	Policies map[string]string
	// Applications are the applications offered, by the name a window
	// process asks for.
	Applications map[string]Application
	// Start starts a window process for a document, when a page or another
	// program asks for one. The process then asks the service to open the
	// document, giving the ticket back.
	Start func(application, path, ticket string) error
	// Headless is the service with no window processes, as a test runs it.
	// A window that would be started is opened by the service itself and
	// listed (Started), a document stays open until the service ends, and
	// the system's dialogs are answered from what the test said they would
	// choose (SetDialogs).
	Headless bool
	// Appearance gives a page the person's theme and skin.
	Appearance func() (any, error)
	// Product is what a page learns of the product it belongs to.
	Product any
	// Idle is how long the service stays with no document open.
	Idle time.Duration
	// Log records a line.
	Log func(format string, args ...any)
}

// Service is the running document service.
type Service struct {
	opt   Options
	store *store
	token string
	ln    net.Listener
	http  *http.Server
	addr  string

	mu      sync.Mutex
	docs    map[string]*document
	codes   map[string]entryCode
	tickets map[string]*ticket
	started []Started
	dialogs map[string][]string
	opened  time.Time // when the service last had a document or was asked for one

	stop     chan struct{}
	stopOnce sync.Once
}

type entryCode struct {
	doc     *document
	expires time.Time
}

// ticket is a window one page asked for and is waiting on: what the new
// window's applet is to take, the file the asking window may hand over, and
// where the new window's answer goes.
type ticket struct {
	request json.RawMessage
	applet  string
	grant   string
	done    chan callResult
	made    time.Time
}

// callResult is a page's answer to something asked of it.
type callResult struct {
	result json.RawMessage
	err    string
}

// Started is a window the service opened itself, in a headless service: the
// address of its page, or the document that was brought forward instead.
type Started struct {
	ID          string `json:"id"`
	Application string `json:"application"`
	Path        string `json:"path"`
	Entry       string `json:"entry,omitempty"`
	Raised      bool   `json:"raised"`
}

// windowGrace is how long a document stays open with no window process
// asking for its commands. A window process asks continuously, so a longer
// silence means it has ended.
var windowGrace = 6 * time.Second

// startGrace is how long a new document waits for its window process to
// ask the first time, and how long a page waits for a window it asked for.
var startGrace = 90 * time.Second

// New prepares a service and takes its address. It serves nothing until
// Serve is called.
func New(opt Options) (*Service, error) {
	if opt.Dir == "" || opt.Files == nil || opt.Page == "" || opt.Cookie == "" {
		return nil, errors.New("the document service needs its folder, its page and its cookie's name")
	}
	if opt.Idle <= 0 {
		opt.Idle = 20 * time.Second
	}
	if opt.Log == nil {
		opt.Log = func(string, ...any) {}
	}
	if err := os.MkdirAll(opt.Dir, 0o700); err != nil {
		return nil, err
	}
	st, err := newStore(opt.Dir)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Service{opt: opt, store: st, token: rand.Text(), ln: ln, addr: ln.Addr().String(),
		docs: map[string]*document{}, codes: map[string]entryCode{}, tickets: map[string]*ticket{},
		dialogs: map[string][]string{}, opened: time.Now(), stop: make(chan struct{})}
	s.http = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	return s, nil
}

// Addr is the address the service answers on.
func (s *Service) Addr() string { return s.addr }

// Token is the control interface's token. A test reads it; a program reads
// the service's file (client.go).
func (s *Service) Token() string { return s.token }

// Serve announces the service in its file, answers until it has had no
// document for the idle time or Stop is called, and then removes the
// announcement.
func (s *Service) Serve() error {
	if err := writeEndpoint(s.opt.Dir, Endpoint{Addr: s.addr, Token: s.token, PID: os.Getpid(), Version: s.opt.Version}); err != nil {
		s.ln.Close()
		return err
	}
	defer removeEndpoint(s.opt.Dir, s.token)
	done := make(chan error, 1)
	go func() { done <- s.http.Serve(s.ln) }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-s.stop:
			s.http.Close()
			return nil
		case <-tick.C:
			if s.reap() {
				// The announcement goes first, so that a window process
				// starting now starts a service of its own and does not
				// find this one as it ends.
				removeEndpoint(s.opt.Dir, s.token)
				s.http.Close()
				return nil
			}
		}
	}
}

// Stop ends the service.
func (s *Service) Stop() { s.stopOnce.Do(func() { close(s.stop) }) }

// reap closes the documents whose window processes have ended, and reports
// whether the service has been without a document for the idle time. A
// headless service has no window processes, and ends only when stopped.
func (s *Service) reap() bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for code, e := range s.codes {
		if now.After(e.expires) {
			delete(s.codes, code)
		}
	}
	for id, t := range s.tickets {
		if now.Sub(t.made) > 2*startGrace {
			delete(s.tickets, id)
		}
	}
	if s.opt.Headless {
		return false
	}
	for id, d := range s.docs {
		if d.link.gone(now) {
			delete(s.docs, id)
			d.link.close()
			s.opt.Log("the window of %s has ended", d.describe())
		}
	}
	if len(s.docs) > 0 || len(s.tickets) > 0 {
		s.opened = now
		return false
	}
	return now.Sub(s.opened) > s.opt.Idle
}

// document is one window's document: its page's way in, the link to its
// window's process, and the file its applet shows now. That file can change
// under a window: a blank window opens a file in place, and Save As gives a
// document another place.
type document struct {
	id, token   string
	application string
	app         Application
	applet      string
	service     *Service
	bridge      *bridge.Bridge
	link        *link

	mu sync.Mutex
	// start is the file the window was started for, or "" for a blank one.
	start string
	// current is the file the applet shows now, as it last said, or "" for
	// a document with no file.
	current string
	// grants are the files the page may read and write: the one it was
	// started for and the ones the person chose in its dialogs.
	grants map[string]bool
	// key is the recovery record this window last read or wrote.
	key string
	// left is the record of a file the window's text was saved away from,
	// under another name. Nothing in it is unsaved any more.
	left string
	// request is what the window's applet is to take when it starts, and
	// ticket the page that waits for its answer.
	request json.RawMessage
	ticket  string
	// picks are the places chosen in a Save As that is not committed yet.
	picks map[string]pick
	// calls are the actions sent to the page that it has not answered.
	calls map[string]chan callResult
	// ready says the page has said what its applet shows, at least once.
	ready bool
	// touched says the document has changes the person has not saved, as
	// the page last said.
	touched bool
	// flushed is closed when the page has sent what it had to keep, after
	// it was asked to because its window is closing.
	flushed chan struct{}
}

// pick is a place chosen for Save As: the file, and the revision it had
// when it was chosen, or "" when there was no such file.
type pick struct {
	path, revision string
}

func (d *document) describe() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.current != "" {
		return d.current
	}
	return "a new document"
}

func (d *document) granted(path string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return path != "" && d.grants[path]
}

func (d *document) grant(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.grants[path] = true
}

// title is the window's title: the file's name and the application's.
func (d *document) title() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	name := "Untitled"
	if d.current != "" {
		name = filepath.Base(d.current)
	}
	if d.touched {
		// A dot leads the title of a document with unsaved changes.
		name = "• " + name
	}
	return name + " - " + d.app.Name
}

// Document is an open document as another program sees it.
type Document struct {
	ID          string `json:"id"`
	Application string `json:"application"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Changed     bool   `json:"changed"`
	// PID is the process of the document's window, which a program in
	// front may let come forward before it asks something of the window.
	PID int `json:"pid"`
	// Ready says the window's page has started its applet and said what
	// it shows.
	Ready bool `json:"ready"`
}

func (d *document) public() Document {
	pid := d.link.process()
	d.mu.Lock()
	defer d.mu.Unlock()
	name := "Untitled"
	if d.current != "" {
		name = filepath.Base(d.current)
	}
	return Document{ID: d.id, Application: d.application, Name: name, Path: d.current, Changed: d.touched, PID: pid, Ready: d.ready}
}

// Documents lists the open documents in the order of their names.
func (s *Service) Documents() []Document {
	s.mu.Lock()
	docs := make([]*document, 0, len(s.docs))
	for _, d := range s.docs {
		docs = append(docs, d)
	}
	s.mu.Unlock()
	out := make([]Document, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.public())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// OpenRequest asks for a document's window: the application, the file, or
// none for a blank window, and the ticket of the page that asked for the
// window, when one did.
type OpenRequest struct {
	Application string `json:"application"`
	Path        string `json:"path"`
	Ticket      string `json:"ticket,omitempty"`
}

// Opened is the answer: the document and the address that signs its page
// in, or, when the file is already open in that application, that it is and
// that its window was asked to come forward.
type Opened struct {
	ID       string `json:"id"`
	Entry    string `json:"entry,omitempty"`
	Title    string `json:"title,omitempty"`
	Existing bool   `json:"existing"`
	// PID is the process of the window that already has the file, which
	// the asking process may let come forward before it asks it to.
	PID int `json:"pid,omitempty"`
}

// cleanPath is a file's path as the service keeps it.
func cleanPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// Open registers a document for the window process that asks, or answers
// that a window already has the file, which the asking process then brings
// forward (Client.Focus).
func (s *Service) Open(r OpenRequest) (Opened, error) {
	app, ok := s.opt.Applications[r.Application]
	if !ok {
		return Opened{}, fmt.Errorf("there is no application called %q", r.Application)
	}
	path, err := cleanPath(r.Path)
	if err != nil {
		return Opened{}, err
	}
	if path != "" {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return Opened{}, fmt.Errorf("%s is a folder", path)
		}
	}
	var t *ticket
	if r.Ticket != "" {
		s.mu.Lock()
		t = s.tickets[r.Ticket]
		s.mu.Unlock()
		if t == nil {
			return Opened{}, errors.New("the window that asked for this one is no longer waiting")
		}
	}
	if path != "" && t == nil {
		if existing := s.find(r.Application, path); existing != nil {
			return Opened{ID: existing.id, Existing: true, PID: existing.link.process()}, nil
		}
	}
	d := &document{id: rand.Text(), token: rand.Text(), application: r.Application, app: app, service: s,
		link: newLink(), start: path, current: path, grants: map[string]bool{}, picks: map[string]pick{}, calls: map[string]chan callResult{}}
	d.applet = app.fileApplet(path)
	if path != "" {
		d.grants[path] = true
		d.key = fileKey(r.Application, path)
	}
	if t != nil {
		if path == "" && t.applet != "" {
			d.applet = t.applet
		}
		d.request, d.ticket = t.request, r.Ticket
		if t.grant != "" {
			d.grants[t.grant] = true
		}
	}
	d.bridge = bridge.New(&Page{d: d})
	s.mu.Lock()
	s.docs[d.id] = d
	s.opened = time.Now()
	s.mu.Unlock()
	return Opened{ID: d.id, Entry: s.entry(d), Title: d.title()}, nil
}

// openKeys are the recovery keys of the documents that are open.
func (s *Service) openKeys() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := map[string]bool{}
	for _, d := range s.docs {
		d.mu.Lock()
		keys[d.key] = true
		d.mu.Unlock()
	}
	return keys
}

// find is the open document of an application that shows the file at path.
// Two paths are the same file when the operating system says so of the two
// as they are now (os.SameFile), and otherwise when they are the same text,
// which is all that can be said of a file that does not exist yet.
func (s *Service) find(application, path string) *document {
	info, err := os.Stat(path)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.docs {
		if d.application != application {
			continue
		}
		d.mu.Lock()
		other := d.current
		d.mu.Unlock()
		if other == "" {
			continue
		}
		if other == path {
			return d
		}
		if err == nil {
			if there, err := os.Stat(other); err == nil && os.SameFile(info, there) {
				return d
			}
		}
	}
	return nil
}

func (s *Service) doc(id string) *document {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.docs[id]
}

// close ends a document whose window has closed.
func (s *Service) close(d *document) {
	s.mu.Lock()
	delete(s.docs, d.id)
	s.opened = time.Now()
	s.mu.Unlock()
	d.link.close()
}

// entry is an address that signs a document's page in, good once and for
// one minute.
func (s *Service) entry(d *document) string {
	code := rand.Text()
	s.mu.Lock()
	s.codes[code] = entryCode{doc: d, expires: time.Now().Add(time.Minute)}
	s.mu.Unlock()
	return "http://" + s.addr + "/d/" + d.id + "/enter?code=" + code
}

func sameToken(a, b string) bool {
	return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// startWindow gets another window for a page or another program: a window
// process started by the program, or, in a headless service, a document the
// service opens itself and lists.
func (s *Service) startWindow(application, path, ticketID string) error {
	if _, ok := s.opt.Applications[application]; !ok {
		return fmt.Errorf("there is no application called %q", application)
	}
	s.mu.Lock()
	s.opened = time.Now()
	s.mu.Unlock()
	if s.opt.Headless {
		opened, err := s.Open(OpenRequest{Application: application, Path: path, Ticket: ticketID})
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.started = append(s.started, Started{ID: opened.ID, Application: application, Path: path, Entry: opened.Entry, Raised: opened.Existing})
		s.mu.Unlock()
		return nil
	}
	if s.opt.Start == nil {
		return errors.New("this service cannot start a window")
	}
	return s.opt.Start(application, path, ticketID)
}

// Started lists the windows a headless service opened itself, oldest first.
func (s *Service) Started() []Started {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Started{}, s.started...)
}

// SetDialogs says what the system's dialogs will choose next in a headless
// service: each Open takes the next path of open, and each Save As the next
// of save. A dialog with nothing to choose is cancelled.
func (s *Service) SetDialogs(open, save []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialogs["open"] = append(s.dialogs["open"], open...)
	s.dialogs["save"] = append(s.dialogs["save"], save...)
}

// SetFolderDialogs says what the system's folder dialog will choose next in
// a headless service.
func (s *Service) SetFolderDialogs(folders []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialogs["folder"] = append(s.dialogs["folder"], folders...)
}

// choose shows one of the system's dialogs in a document's window and
// returns the path chosen, or "" when the person cancelled.
func (s *Service) choose(ctx context.Context, d *document, kind string, data any) (string, error) {
	if s.opt.Headless {
		s.mu.Lock()
		defer s.mu.Unlock()
		list := s.dialogs[kind]
		if len(list) == 0 {
			return "", nil
		}
		s.dialogs[kind] = list[1:]
		return list[0], nil
	}
	raw, err := d.link.ask(ctx, "pick-"+kind, data)
	if err != nil {
		return "", err
	}
	var chosen string
	if err := json.Unmarshal(raw, &chosen); err != nil {
		return "", errors.New("the window did not say what was chosen")
	}
	return chosen, nil
}
