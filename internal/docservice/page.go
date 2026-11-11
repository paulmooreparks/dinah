package docservice

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"studio/internal/resource"
	"studio/internal/resource/localfs"
)

// Page is everything a document's page may ask of the service. Its methods
// are the page's bridge (internal/bridge), reached with the cookie of that
// one document. A page reads and writes the file it was started for and the
// files the person chose in its own dialogs, and no other.
type Page struct{ d *document }

// absentRevision is the revision of a file that does not exist yet, such as
// one named on a command line. The applets require a revision that is not
// empty, and the file provider's own word for "must not exist" is the empty
// one.
const absentRevision = "absent"

// conflictText is what the person reads when a save is refused because the
// file changed.
const conflictText = "The file has changed since it was read, so it was not saved, and your text is kept. Save it under another name with Save As, or open the file again to see what changed."

// Attached is what a page learns when it starts: the application it is,
// the file its window was started for, the recovery snapshot kept for that
// document, and what its applet is to take first when another window asked
// for this one.
type Attached struct {
	ID              string `json:"id"`
	Application     string `json:"application"`
	Applet          string `json:"applet"`
	ApplicationName string `json:"applicationName"`
	Resource        string `json:"resource"`
	Name            string `json:"name"`
	Kept            string `json:"kept"`
	Request         string `json:"request"`
	Product         any    `json:"product"`
	Headless        bool   `json:"headless"`
}

// Attach gives the page its document.
func (p *Page) Attach() Attached {
	d, s := p.d, p.d.service
	d.mu.Lock()
	start, request, key := d.start, d.request, d.key
	d.mu.Unlock()
	out := Attached{ID: d.id, Application: d.application, Applet: d.app.Applet, ApplicationName: d.app.Name,
		Resource: start, Request: string(request), Product: s.opt.Product, Headless: s.opt.Headless}
	var kept *Record
	switch {
	case start != "":
		out.Name = filepath.Base(start)
		kept = s.store.get(fileKey(d.application, start))
	case len(request) == 0:
		// A blank window shows the newest new document whose window ended
		// with its text unsaved. A window that is to take a request takes
		// that and nothing else.
		if key != "" {
			kept = s.store.get(key)
		} else if kept = s.store.orphan(d.application, s.openKeys()); kept != nil {
			d.mu.Lock()
			d.key = kept.Key
			d.mu.Unlock()
		}
	}
	if kept != nil {
		out.Kept = string(kept.Snapshot)
	}
	return out
}

// folderOf is the file provider for the folder of a file, and the file's
// name there. A stand-alone application opens the file the person points
// at, so a name that begins with a dot is an ordinary name.
func folderOf(path string) (localfs.Folder, string) {
	return localfs.Folder{Root: filepath.Dir(path), Dotfiles: true}, filepath.Base(path)
}

// file checks that the page may use a file and returns its cleaned path.
func (p *Page) file(resource string) (string, error) {
	path, err := cleanPath(resource)
	if err != nil || path == "" || !p.d.granted(path) {
		return "", errors.New("this window was not given that file")
	}
	return path, nil
}

// ReadResult is a file as an applet reads it
// (yavc.app content-resource-contract.md, version 1).
type ReadResult struct {
	Resource string `json:"resource"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	Revision string `json:"revision"`
	Writable bool   `json:"writable"`
}

// Read reads a file as it is now. A file the window was started for that
// does not exist yet reads as empty, and its first save creates it.
func (p *Page) Read(ctx context.Context, shown string) (ReadResult, error) {
	path, err := p.file(shown)
	if err != nil {
		return ReadResult{}, err
	}
	folder, name := folderOf(path)
	t, err := folder.ReadText(ctx, name)
	if errors.Is(err, resource.ErrNotFound) {
		if _, serr := os.Stat(path); errors.Is(serr, os.ErrNotExist) {
			return ReadResult{Resource: shown, Name: name, Revision: absentRevision, Writable: true}, nil
		}
	}
	if err != nil {
		return ReadResult{}, fmt.Errorf("%s could not be read: %w", name, err)
	}
	return ReadResult{Resource: shown, Name: name, Text: t.Text, Revision: t.Revision, Writable: true}, nil
}

// WriteResult acknowledges a write: the file, and the revision it has now.
type WriteResult struct {
	Resource string `json:"resource"`
	Name     string `json:"name"`
	Revision string `json:"revision"`
	Writable bool   `json:"writable"`
}

// Write replaces a file's text, provided the file still has the revision
// the page read. The file provider makes the comparison and the replacement
// under the save lock that every program of this product takes
// (internal/savelock), so a change made by any of them meanwhile refuses the
// write and changes nothing.
func (p *Page) Write(ctx context.Context, shown, expected, text string) (WriteResult, error) {
	path, err := p.file(shown)
	if err != nil {
		return WriteResult{}, err
	}
	return write(ctx, shown, path, expected, text)
}

func write(ctx context.Context, shown, path, expected, text string) (WriteResult, error) {
	if expected == "" {
		return WriteResult{}, errors.New("a save says which revision of the file it was based on")
	}
	if expected == absentRevision {
		expected = ""
	}
	folder, name := folderOf(path)
	rev, err := folder.WriteText(ctx, name, expected, text)
	if errors.Is(err, resource.ErrConflict) || errors.Is(err, localfs.ErrExists) {
		return WriteResult{}, errors.New(conflictText)
	}
	if err != nil {
		return WriteResult{}, fmt.Errorf("%s could not be saved: %w", name, err)
	}
	return WriteResult{Resource: shown, Name: name, Revision: rev, Writable: true}, nil
}

// Picked is a file the person chose to open.
type Picked struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// PickOpen shows the system's Open dialog in the page's window and returns
// the file chosen, which the page may then read and write, or nothing when
// the person cancelled.
func (p *Page) PickOpen(ctx context.Context) (*Picked, error) {
	d := p.d
	d.mu.Lock()
	folder := ""
	if d.current != "" {
		folder = filepath.Dir(d.current)
	}
	d.mu.Unlock()
	chosen, err := d.service.choose(ctx, d, "open", map[string]string{"folder": folder})
	if err != nil || chosen == "" {
		return nil, err
	}
	path, err := cleanPath(chosen)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return nil, fmt.Errorf("%s is not a file that can be opened", path)
	}
	d.grant(path)
	return &Picked{Path: path, Name: filepath.Base(path)}, nil
}

// SavePick is a place the person chose in Save As, which the page commits
// with its ticket once the applet has accepted the name.
type SavePick struct {
	Ticket string `json:"ticket"`
	Name   string `json:"name"`
	Path   string `json:"path"`
}

// PickSave shows the system's Save dialog in the page's window. The dialog
// itself asks before a file that exists is chosen. The revision that file
// has now is kept with the choice, and the commit is refused if the file
// has changed by then, so what the person agreed to replace is what is
// replaced.
func (p *Page) PickSave(ctx context.Context, name string) (*SavePick, error) {
	d := p.d
	d.mu.Lock()
	folder := ""
	if d.current != "" {
		folder = filepath.Dir(d.current)
	}
	d.mu.Unlock()
	chosen, err := d.service.choose(ctx, d, "save", map[string]string{"folder": folder, "name": name})
	if err != nil || chosen == "" {
		return nil, err
	}
	path, err := cleanPath(chosen)
	if err != nil {
		return nil, err
	}
	revision := ""
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("%s is a folder", path)
		}
		f, base := folderOf(path)
		t, err := f.ReadText(ctx, base)
		if err != nil {
			return nil, fmt.Errorf("%s cannot be replaced: %w", base, err)
		}
		revision = t.Revision
	}
	ticket := rand.Text()
	d.mu.Lock()
	d.picks = map[string]pick{ticket: {path: path, revision: revision}}
	d.mu.Unlock()
	return &SavePick{Ticket: ticket, Name: filepath.Base(path), Path: path}, nil
}

// CommitSave writes the text to the place chosen with PickSave, which the
// page may then read and write.
func (p *Page) CommitSave(ctx context.Context, ticket, text string) (WriteResult, error) {
	d := p.d
	d.mu.Lock()
	chosen, ok := d.picks[ticket]
	delete(d.picks, ticket)
	d.mu.Unlock()
	if !ok {
		return WriteResult{}, errors.New("choose where to save the document again")
	}
	expected := chosen.revision
	if expected == "" {
		expected = absentRevision
	}
	out, err := write(ctx, chosen.path, chosen.path, expected, text)
	if err != nil {
		return WriteResult{}, err
	}
	d.grant(chosen.path)
	// The text the window showed is now in the file chosen, so the record
	// kept under the file it showed before has nothing unsaved in it, and
	// would be offered as unsaved changes when that file is next opened.
	d.mu.Lock()
	left := d.key
	if strings.HasPrefix(left, "untitled:") || left == fileKey(d.application, chosen.path) {
		left = ""
	}
	d.left = left
	d.mu.Unlock()
	if left != "" {
		d.service.store.remove(left)
	}
	return out, nil
}

// PutSnapshot keeps the applet's recovery snapshot for the document it
// shows now, which is the file resource, or no file when resource is empty.
// It reports whether the snapshot is one the person can get back by opening
// the document again: that is so for a file. A document with no file is
// kept all the same, for a window that ends by force, and the answer is no,
// so the applet still warns before its window is closed.
func (p *Page) PutSnapshot(shown, snapshot string) (bool, error) {
	d, s := p.d, p.d.service
	path := ""
	if shown != "" {
		var err error
		if path, err = p.file(shown); err != nil {
			return false, err
		}
	}
	d.mu.Lock()
	previous := d.key
	key := untitledKey(d.application, d.id)
	if path != "" {
		key = fileKey(d.application, path)
	} else if strings.HasPrefix(previous, "untitled:") {
		key = previous
	}
	d.key = key
	// A snapshot that was already on its way when the text was saved under
	// another name puts the old file's record back, and the next one, for
	// the new file, removes it again.
	left := d.left
	if key != left && previous != left {
		d.left = ""
	}
	d.mu.Unlock()
	if err := s.store.put(Record{Key: key, Application: d.application, Path: path, Untitled: path == "", Saved: time.Now().UTC(), Snapshot: json.RawMessage(snapshot)}); err != nil {
		return false, err
	}
	// The record of what the window showed before goes when the window has
	// moved on from a document with no file, or saved its text away from a
	// file under another name. Any other file's record stays, for the next
	// window that opens the file.
	if previous != "" && previous != key && (strings.HasPrefix(previous, "untitled:") || previous == left) {
		s.store.remove(previous)
	}
	p.setCurrent(path)
	return path != "", nil
}

// State tells the service which file the applet shows now and whether the
// person has changed the document, which decide the window's title, whether
// opening that file elsewhere brings this window forward, and whether
// closing the window asks first.
func (p *Page) State(shown string, changed bool) error {
	path := ""
	if shown != "" {
		var err error
		if path, err = p.file(shown); err != nil {
			return err
		}
	}
	d := p.d
	d.mu.Lock()
	retitle := d.touched != changed || d.current != path
	d.current, d.touched, d.ready = path, changed, true
	d.mu.Unlock()
	if retitle {
		d.link.send("title", map[string]string{"title": d.title()})
	}
	return nil
}

func (p *Page) setCurrent(path string) {
	d := p.d
	d.mu.Lock()
	moved := d.current != path
	d.current = path
	d.mu.Unlock()
	if moved {
		d.link.send("title", map[string]string{"title": d.title()})
	}
}

// windowRequest is what an applet asks another window to take
// (yavc.app document-window-contract.md).
type windowRequest struct {
	Operation string `json:"operation"`
	Input     struct {
		Path string `json:"path"`
	} `json:"input"`
}

// OpenWindow gets another window for a request of this page's applet: a new
// document, a file to open, or text that arrived without a file. It returns
// what the new window's applet answered, once that applet has taken the
// request and kept it for recovery. A file that a window of the application
// already shows brings that window forward and starts none.
func (p *Page) OpenWindow(ctx context.Context, application, request string) (string, error) {
	d, s := p.d, p.d.service
	var r windowRequest
	if err := json.Unmarshal([]byte(request), &r); err != nil {
		return "", errors.New("the request for another window could not be read")
	}
	path := ""
	switch r.Operation {
	case "new", "import-source":
	case "open-file":
		var err error
		if path, err = p.file(r.Input.Path); err != nil {
			return "", err
		}
		if existing := s.find(application, path); existing != nil && existing != d {
			// The asking window is the one in front, so its process lets
			// the other's come forward before that one is asked to.
			if pid := existing.link.process(); pid > 0 && !s.opt.Headless {
				allow, cancel := context.WithTimeout(ctx, 2*time.Second)
				d.link.ask(allow, "allow", map[string]int{"pid": pid})
				cancel()
			}
			existing.link.send("focus", nil)
			if s.opt.Headless {
				s.mu.Lock()
				s.started = append(s.started, Started{ID: existing.id, Application: application, Path: path, Raised: true})
				s.mu.Unlock()
			}
			out, _ := json.Marshal(map[string]any{"output": map[string]any{"path": path, "raised": true}})
			return string(out), nil
		}
	default:
		return "", fmt.Errorf("a window cannot be opened for %q", r.Operation)
	}
	id := rand.Text()
	t := &ticket{request: json.RawMessage(request), grant: path, done: make(chan callResult, 1), made: time.Now()}
	s.mu.Lock()
	s.tickets[id] = t
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.tickets, id)
		s.mu.Unlock()
	}()
	if err := s.startWindow(application, path, id); err != nil {
		return "", err
	}
	wait := time.NewTimer(startGrace)
	defer wait.Stop()
	select {
	case res := <-t.done:
		if res.err != "" {
			return "", errors.New(res.err)
		}
		return string(res.result), nil
	case <-wait.C:
		return "", errors.New("the new window did not start")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Accepted is a new window's answer to the request it was started with:
// what its applet returned, or what went wrong. The page that asked for the
// window is waiting for it.
func (p *Page) Accepted(result, problem string) {
	d, s := p.d, p.d.service
	d.mu.Lock()
	id := d.ticket
	d.ticket, d.request = "", nil
	d.mu.Unlock()
	s.mu.Lock()
	t := s.tickets[id]
	s.mu.Unlock()
	if t == nil {
		return
	}
	if problem == "" && !json.Valid([]byte(result)) {
		result = "null"
	}
	select {
	case t.done <- callResult{result: json.RawMessage(result), err: problem}:
	default:
	}
}

// NewWindow gets a blank window of the page's application.
func (p *Page) NewWindow() error {
	return p.d.service.startWindow(p.d.application, "", "")
}

// CloseWindow asks the page's window to close, as its own close button
// does, so the window's process can ask the person first.
func (p *Page) CloseWindow() {
	p.d.link.send("close", nil)
}

// ConfigLoad gives an application's saved settings.
func (p *Page) ConfigLoad(application string) map[string]json.RawMessage {
	out := p.d.service.store.config(application)
	if out == nil {
		out = map[string]json.RawMessage{}
	}
	return out
}

// ConfigSet adds values to an application's settings and returns them all.
func (p *Page) ConfigSet(application, values string) (map[string]json.RawMessage, error) {
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(values), &parsed); err != nil {
		return nil, errors.New("the settings could not be read")
	}
	return p.d.service.store.setConfig(application, parsed)
}

// FoundText is a file a document refers to: its text, or that there is no
// such file.
type FoundText struct {
	Found bool   `json:"found"`
	Text  string `json:"text"`
}

// virtualPath is a folder or file as an applet writes its path: with
// forward slashes and a leading one, which the applets require of a path
// they resolve a reference against.
func virtualPath(path string) string {
	v := filepath.ToSlash(path)
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	return v
}

// realPath is the file a virtual path stands for.
func realPath(virtual string) string {
	v := virtual
	// A Windows path has a drive after the leading slash.
	if len(v) >= 3 && v[0] == '/' && v[2] == ':' {
		v = v[1:]
	}
	return filepath.Clean(filepath.FromSlash(v))
}

// Directory is the folder of one of the page's files, as a virtual path an
// applet resolves a document's references against.
func (p *Page) Directory(shown string) string {
	path, err := p.file(shown)
	if err != nil {
		return ""
	}
	return virtualPath(filepath.Dir(path))
}

// within says path is in folder or below it.
func within(folder, path string) bool {
	rel, err := filepath.Rel(folder, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// ReadText reads a text file a document refers to, such as a saved diagram
// beside it. The file must be in the folder of one of the page's files or
// below it: a reference that leads anywhere else reads as a file that is
// not there.
func (p *Page) ReadText(ctx context.Context, virtual string, maxBytes int) (FoundText, error) {
	path := realPath(virtual)
	d := p.d
	d.mu.Lock()
	var root string
	for granted := range d.grants {
		if folder := filepath.Dir(granted); within(folder, path) && len(folder) > len(root) {
			root = folder
		}
	}
	d.mu.Unlock()
	if root == "" {
		return FoundText{}, nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return FoundText{}, nil
	}
	t, err := localfs.Folder{Root: root, Dotfiles: true}.ReadText(ctx, filepath.ToSlash(rel))
	if errors.Is(err, resource.ErrNotFound) {
		return FoundText{}, nil
	}
	if err != nil {
		return FoundText{}, err
	}
	if maxBytes > 0 && len(t.Text) > maxBytes {
		return FoundText{}, errors.New("the file is larger than the document may read")
	}
	return FoundText{Found: true, Text: t.Text}, nil
}

// ReadAppearance gives the page the person's theme and skin.
func (p *Page) ReadAppearance() (any, error) {
	if p.d.service.opt.Appearance == nil {
		return nil, errors.New("this service keeps no appearance")
	}
	return p.d.service.opt.Appearance()
}

// Answer is the page's answer to an action sent to it (invoke).
func (p *Page) Answer(call, result, problem string) {
	d := p.d
	d.mu.Lock()
	waiting := d.calls[call]
	delete(d.calls, call)
	d.mu.Unlock()
	if waiting == nil {
		return
	}
	if problem == "" && !json.Valid([]byte(result)) {
		result = "null"
	}
	waiting <- callResult{result: json.RawMessage(result), err: problem}
}

// Flushed says the page has sent everything it had to keep, after it was
// told its window is closing.
func (p *Page) Flushed() {
	d := p.d
	d.mu.Lock()
	done := d.flushed
	d.flushed = nil
	d.mu.Unlock()
	if done != nil {
		close(done)
	}
}

// invoke sends an action to the document's applet and waits for its
// answer. The page asks the person first, in its own window.
func (d *document) invoke(ctx context.Context, request json.RawMessage, from string) (json.RawMessage, error) {
	call := rand.Text()
	waiting := make(chan callResult, 1)
	d.mu.Lock()
	d.calls[call] = waiting
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.calls, call)
		d.mu.Unlock()
	}()
	d.bridge.Emit("invoke", map[string]any{"call": call, "request": string(request), "from": from})
	select {
	case res := <-waiting:
		if res.err != "" {
			return nil, errors.New(res.err)
		}
		return res.result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// closing asks whether the document's window may close. It first has the
// page send what it still has to keep, and then says whether the person
// should be asked: that is so for a changed document with no file, which
// nobody could open again.
func (d *document) closing(ctx context.Context) (ask bool) {
	done := make(chan struct{})
	d.mu.Lock()
	d.flushed = done
	d.mu.Unlock()
	d.bridge.Emit("closing", nil)
	wait := time.NewTimer(3 * time.Second)
	defer wait.Stop()
	select {
	case <-done:
	case <-wait.C:
	case <-ctx.Done():
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.current == "" && d.touched
}

// closed ends the document after its window closed with the person's
// leave. The record of a document with no file goes with it, since the
// person was asked and chose to close.
func (d *document) closed() {
	d.mu.Lock()
	key := d.key
	d.mu.Unlock()
	if strings.HasPrefix(key, "untitled:") {
		d.service.store.remove(key)
	}
	d.service.close(d)
}
