package docservice

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"studio/internal/savelock"
)

// harness is a service on a folder of its own, with a page of one file.
type harness struct {
	t   *testing.T
	s   *Service
	dir string
}

func start(t *testing.T, dir string, headless bool) *harness {
	t.Helper()
	t.Setenv(savelock.TestDirEnv, t.TempDir())
	files := fstest.MapFS{"document.html": {Data: []byte("<!doctype html><title>page</title>")}, "app.css": {Data: []byte("body{}")}}
	s, err := New(Options{Dir: dir, Version: "test", Files: http.FileServerFS(files), Page: "document.html", Cookie: "doc",
		Policy: "default-src 'self'", Headless: headless, Idle: time.Hour,
		Applications: map[string]Application{"editor": {Applet: "editor", Name: "Test Editor"}}})
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve()
	t.Cleanup(s.Stop)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := Find(dir); err == nil {
			return &harness{t: t, s: s, dir: dir}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the service did not announce itself")
	return nil
}

func (h *harness) client() *Client {
	h.t.Helper()
	c, err := Find(h.dir)
	if err != nil {
		h.t.Fatal(err)
	}
	return c
}

// page is a browser's page for one document: a cookie jar signed in through
// the document's way in.
type page struct {
	t    *testing.T
	h    *harness
	id   string
	http *http.Client
}

func (h *harness) open(path string) *page {
	h.t.Helper()
	opened, err := h.client().Open(context.Background(), OpenRequest{Application: "editor", Path: path})
	if err != nil {
		h.t.Fatal(err)
	}
	if opened.Existing {
		h.t.Fatalf("%s is already open", path)
	}
	return h.enter(opened.ID, opened.Entry)
}

func (h *harness) enter(id, entry string) *page {
	h.t.Helper()
	jar, _ := cookiejar.New(nil)
	p := &page{t: h.t, h: h, id: id, http: &http.Client{Jar: jar}}
	res, err := p.http.Get(entry)
	if err != nil || res.StatusCode != http.StatusOK {
		h.t.Fatalf("the way in: %v %v", res, err)
	}
	res.Body.Close()
	return p
}

func (p *page) base() string { return "http://" + p.h.s.Addr() + "/d/" + p.id }

// call calls one of the page's bridge methods and decodes its result.
func (p *page) call(into any, method string, args ...any) string {
	p.t.Helper()
	raw := make([]json.RawMessage, len(args))
	for i, a := range args {
		raw[i], _ = json.Marshal(a)
	}
	payload, _ := json.Marshal(map[string]any{"method": method, "args": raw})
	req, _ := http.NewRequest(http.MethodPost, p.base()+"/bridge/call", bytes.NewReader(payload))
	req.Header.Set("Origin", "http://"+p.h.s.Addr())
	res, err := p.http.Do(req)
	if err != nil {
		p.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		p.t.Fatalf("%s: %s %s", method, res.Status, data)
	}
	var a struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(data, &a); err != nil {
		p.t.Fatal(err)
	}
	if a.Error == "" && into != nil {
		if err := json.Unmarshal(a.Result, into); err != nil {
			p.t.Fatalf("%s: %v in %s", method, err, a.Result)
		}
	}
	return a.Error
}

func put(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A page is reached with its own document's cookie and with no other.
func TestAPageIsReachedOnlyWithItsOwnCookie(t *testing.T) {
	h := start(t, t.TempDir(), false)
	docs := t.TempDir()
	a, b := filepath.Join(docs, "a.md"), filepath.Join(docs, "b.md")
	put(t, a, "A\n")
	put(t, b, "B\n")
	pa, pb := h.open(a), h.open(b)
	res, err := pa.http.Get(pa.base() + "/app.css")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("the page's own file: %v %v", res, err)
	}
	if res.Header.Get("Content-Security-Policy") == "" {
		t.Error("the page's file has no policy")
	}
	res.Body.Close()
	// The cookie is sent only under its document's path, so A's browser has
	// none to send for B, and one copied there by hand is not B's.
	if res, _ := pa.http.Get(pb.base() + "/app.css"); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("A's page read B's files: %s", res.Status)
	}
	u, _ := url.Parse(pa.base() + "/")
	stolen := pa.http.Jar.Cookies(u)
	req, _ := http.NewRequest(http.MethodGet, pb.base()+"/app.css", nil)
	for _, c := range stolen {
		req.AddCookie(c)
	}
	if res, _ := http.DefaultClient.Do(req); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("A's cookie opened B's page: %s", res.Status)
	}
	if res, _ := http.Get(pa.base() + "/app.css"); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("no cookie at all read a page: %s", res.Status)
	}
	// A call from another origin is refused even with the cookie.
	req, _ = http.NewRequest(http.MethodPost, pa.base()+"/bridge/call", strings.NewReader(`{"method":"Attach","args":[]}`))
	req.Header.Set("Origin", "http://example.com")
	if res, _ := pa.http.Do(req); res.StatusCode != http.StatusForbidden {
		t.Errorf("a call from another origin: %s", res.Status)
	}
	// A way in is good once.
	opened, _ := h.client().Open(context.Background(), OpenRequest{Application: "editor", Path: filepath.Join(docs, "c.md")})
	if res, _ := http.Get(opened.Entry); res.StatusCode != http.StatusOK {
		t.Fatalf("the way in: %s", res.Status)
	}
	if res, _ := http.Get(opened.Entry); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("the way in worked twice: %s", res.Status)
	}
}

// The control interface wants the token and is not for pages.
func TestTheControlInterfaceWantsItsToken(t *testing.T) {
	h := start(t, t.TempDir(), false)
	at := "http://" + h.s.Addr() + "/control/documents"
	if res, _ := http.Get(at); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token: %s", res.Status)
	}
	req, _ := http.NewRequest(http.MethodGet, at, nil)
	req.Header.Set("Authorization", "Bearer "+h.s.Token())
	req.Header.Set("Origin", "http://"+h.s.Addr())
	if res, _ := http.DefaultClient.Do(req); res.StatusCode != http.StatusForbidden {
		t.Errorf("a page with the token: %s", res.Status)
	}
	req, _ = http.NewRequest(http.MethodGet, at, nil)
	req.Header.Set("Authorization", "Bearer "+h.s.Token())
	req.Host = "evil.example:80"
	if res, _ := http.DefaultClient.Do(req); res.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("another host name: %s", res.Status)
	}
	if docs, err := h.client().Documents(context.Background()); err != nil || len(docs) != 0 {
		t.Errorf("the client: %v %v", docs, err)
	}
}

// A file is read as it is and written only while it is what was read. A
// change made by another program refuses the save and changes nothing.
func TestASaveIsRefusedWhenTheFileHasChanged(t *testing.T) {
	h := start(t, t.TempDir(), false)
	file := filepath.Join(t.TempDir(), "notes.md")
	put(t, file, "one\n")
	p := h.open(file)
	var at Attached
	p.call(&at, "Attach")
	if at.Resource != file || at.Name != "notes.md" || at.Applet != "editor" || at.Kept != "" {
		t.Fatalf("attached %+v", at)
	}
	var r ReadResult
	if problem := p.call(&r, "Read", file); problem != "" || r.Text != "one\n" || r.Revision == "" || !r.Writable {
		t.Fatalf("read %+v %s", r, problem)
	}
	var w WriteResult
	if problem := p.call(&w, "Write", file, r.Revision, "two\n"); problem != "" || w.Revision == r.Revision {
		t.Fatalf("write %+v %s", w, problem)
	}
	if read(t, file) != "two\n" {
		t.Fatalf("the file holds %q", read(t, file))
	}
	// Another program saves the file. A save based on what this page read
	// is refused, and the other program's text stays.
	put(t, file, "theirs\n")
	if problem := p.call(nil, "Write", file, w.Revision, "mine\n"); problem != conflictText {
		t.Fatalf("a save over another program's change: %q", problem)
	}
	if read(t, file) != "theirs\n" {
		t.Fatalf("the file holds %q", read(t, file))
	}
	// A page reads and writes its own file and no other.
	other := filepath.Join(filepath.Dir(file), "other.md")
	put(t, other, "other\n")
	if problem := p.call(nil, "Read", other); problem == "" {
		t.Error("the page read a file it was not given")
	}
	if problem := p.call(nil, "Write", other, "x", "taken\n"); problem == "" || read(t, other) != "other\n" {
		t.Errorf("the page wrote a file it was not given: %q", problem)
	}
}

// A file named for a window that does not exist yet reads as empty and is
// made by its first save, which is refused if someone made it meanwhile.
func TestAFileThatDoesNotExistYetIsMadeByItsFirstSave(t *testing.T) {
	h := start(t, t.TempDir(), false)
	file := filepath.Join(t.TempDir(), "new.md")
	p := h.open(file)
	var r ReadResult
	if problem := p.call(&r, "Read", file); problem != "" || r.Text != "" || r.Revision != absentRevision {
		t.Fatalf("read %+v %s", r, problem)
	}
	var w WriteResult
	if problem := p.call(&w, "Write", file, r.Revision, "made\n"); problem != "" || read(t, file) != "made\n" {
		t.Fatalf("write %+v %s", w, problem)
	}
	if problem := p.call(nil, "Write", file, absentRevision, "again\n"); problem != conflictText || read(t, file) != "made\n" {
		t.Fatalf("a second first save: %q", problem)
	}
}

// Opening a file that a window of the application already shows brings that
// window forward and opens none, by whichever path the file is reached.
func TestOpeningAnOpenFileBringsItsWindowForward(t *testing.T) {
	h := start(t, t.TempDir(), false)
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.md")
	put(t, file, "one\n")
	p := h.open(file)
	c := h.client()
	ctx := context.Background()
	again, err := c.Open(ctx, OpenRequest{Application: "editor", Path: filepath.Join(dir, "sub", "..", "notes.md")})
	if err != nil || !again.Existing || again.ID != p.id || again.Entry != "" {
		t.Fatalf("the second open: %+v %v", again, err)
	}
	if err := c.Focus(ctx, p.id); err != nil {
		t.Fatal(err)
	}
	commands, err := c.Next(ctx, p.id, 0)
	if err != nil || len(commands) != 1 || commands[0].Kind != "focus" {
		t.Fatalf("the window was told %+v %v", commands, err)
	}
	if link := filepath.Join(dir, "link.md"); os.Symlink(file, link) == nil {
		if linked, _ := c.Open(ctx, OpenRequest{Application: "editor", Path: link}); !linked.Existing {
			t.Error("a link to the open file opened a second window")
		}
	}
	docs, _ := c.Documents(ctx)
	if len(docs) != 1 || docs[0].Path != file || docs[0].Name != "notes.md" {
		t.Fatalf("the open documents: %+v", docs)
	}
}

// A window process that stops asking has ended, and its document is closed.
// The text the page kept for recovery is offered to the next window that
// opens the file, even from a new service.
func TestAWindowThatEndsByForceLeavesItsUnsavedTextForTheNext(t *testing.T) {
	windowGrace, startGrace = 200*time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() { windowGrace, startGrace = 6*time.Second, 90*time.Second })
	state := t.TempDir()
	h := start(t, state, false)
	file := filepath.Join(t.TempDir(), "notes.md")
	put(t, file, "one\n")
	p := h.open(file)
	ctx := context.Background()
	c := h.client()
	// The window process asks once, as a living one does all the time.
	short, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	c.Next(short, p.id, 0)
	cancel()
	var kept bool
	if problem := p.call(&kept, "PutSnapshot", file, `{"tabs":[{"draft":"unsaved words"}]}`); problem != "" || !kept {
		t.Fatalf("keeping the snapshot: %v %s", kept, problem)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(h.s.Documents()) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("the document of a window that ended is still open")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if read(t, file) != "one\n" {
		t.Fatal("the unsaved text reached the file")
	}
	h.s.Stop()
	time.Sleep(100 * time.Millisecond)
	h2 := start(t, state, false)
	p2 := h2.open(file)
	var at Attached
	p2.call(&at, "Attach")
	if at.Kept != `{"tabs":[{"draft":"unsaved words"}]}` {
		t.Fatalf("the next window was offered %q", at.Kept)
	}
	// Another file's window is offered nothing of it.
	other := filepath.Join(filepath.Dir(file), "other.md")
	put(t, other, "other\n")
	var none Attached
	h2.open(other).call(&none, "Attach")
	if none.Kept != "" {
		t.Fatalf("another file's window was offered %q", none.Kept)
	}
}

// A document with no file is kept for a window that ends by force, and the
// applet is told it is not one the person can get back by opening a file.
// The next blank window shows it. A window the person closed takes its
// record with it.
func TestANewDocumentIsKeptOnlyForAWindowThatEndedByForce(t *testing.T) {
	windowGrace, startGrace = 200*time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() { windowGrace, startGrace = 6*time.Second, 90*time.Second })
	h := start(t, t.TempDir(), false)
	ctx := context.Background()
	c := h.client()
	p := h.open("")
	var at Attached
	p.call(&at, "Attach")
	if at.Resource != "" || at.Kept != "" {
		t.Fatalf("a blank window was given %+v", at)
	}
	var kept bool
	if problem := p.call(&kept, "PutSnapshot", "", `{"tabs":[{"draft":"new words"}]}`); problem != "" || kept {
		t.Fatalf("a document with no file: kept %v %s", kept, problem)
	}
	p.call(nil, "State", "", true)
	if ask, err := c.Closing(ctx, p.id); err != nil || !ask {
		t.Fatalf("closing a changed document with no file: ask %v %v", ask, err)
	}
	// The window ends by force: it never says it closed.
	deadline := time.Now().Add(5 * time.Second)
	for len(h.s.Documents()) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("the document is still open")
		}
		time.Sleep(50 * time.Millisecond)
	}
	p2 := h.open("")
	var again Attached
	p2.call(&again, "Attach")
	if again.Kept != `{"tabs":[{"draft":"new words"}]}` {
		t.Fatalf("the next blank window was offered %q", again.Kept)
	}
	// A third blank window, while the second has that document, gets none.
	var third Attached
	h.open("").call(&third, "Attach")
	if third.Kept != "" {
		t.Fatalf("a second blank window was offered the same document: %q", third.Kept)
	}
	// The person closes the second window and agrees to lose the text.
	p2.call(&kept, "PutSnapshot", "", `{"tabs":[{"draft":"more"}]}`)
	if err := c.Closed(ctx, p2.id); err != nil {
		t.Fatal(err)
	}
	var after Attached
	h.open("").call(&after, "Attach")
	if after.Kept != "" {
		t.Fatalf("a document the person closed came back: %q", after.Kept)
	}
}

// Save As asks the window for a place, keeps the revision of a file that is
// there, and writes only if that file is still what the person agreed to
// replace. The window then shows the new file, and its record follows it.
func TestSaveAsWritesWhatThePersonAgreedToReplace(t *testing.T) {
	h := start(t, t.TempDir(), true)
	dir := t.TempDir()
	target, taken := filepath.Join(dir, "saved.md"), filepath.Join(dir, "taken.md")
	put(t, taken, "there already\n")
	p := h.open("")
	p.call(nil, "Attach")
	// Nothing to choose: the dialog was cancelled.
	var none *SavePick
	if problem := p.call(&none, "PickSave", "Untitled.md"); problem != "" || none != nil {
		t.Fatalf("a cancelled dialog: %+v %s", none, problem)
	}
	h.s.SetDialogs(nil, []string{target, taken, taken})
	var pick SavePick
	if problem := p.call(&pick, "PickSave", "Untitled.md"); problem != "" || pick.Name != "saved.md" || pick.Ticket == "" {
		t.Fatalf("pick %+v %s", pick, problem)
	}
	var w WriteResult
	if problem := p.call(&w, "CommitSave", pick.Ticket, "words\n"); problem != "" || w.Resource != target || read(t, target) != "words\n" {
		t.Fatalf("commit %+v %s", w, problem)
	}
	if problem := p.call(nil, "CommitSave", pick.Ticket, "twice\n"); problem == "" {
		t.Error("a place was committed twice")
	}
	// The page may now use the file, and says it shows it.
	var kept bool
	if problem := p.call(&kept, "PutSnapshot", target, `{"tabs":[]}`); problem != "" || !kept {
		t.Fatalf("the snapshot of the saved file: %v %s", kept, problem)
	}
	if docs := h.s.Documents(); len(docs) != 1 || docs[0].Path != target {
		t.Fatalf("the open documents: %+v", docs)
	}
	// A file that is there is replaced when it is still what was agreed to.
	if problem := p.call(&pick, "PickSave", "x.md"); problem != "" {
		t.Fatal(problem)
	}
	if problem := p.call(&w, "CommitSave", pick.Ticket, "replaced\n"); problem != "" || read(t, taken) != "replaced\n" {
		t.Fatalf("replacing: %s", problem)
	}
	// And not when it changed between the choice and the commit.
	if problem := p.call(&pick, "PickSave", "x.md"); problem != "" {
		t.Fatal(problem)
	}
	put(t, taken, "changed meanwhile\n")
	if problem := p.call(nil, "CommitSave", pick.Ticket, "lost?\n"); problem != conflictText || read(t, taken) != "changed meanwhile\n" {
		t.Fatalf("replacing a file that changed: %q", problem)
	}
}

// A page asks for another window with a request of its applet. The new
// window's applet takes the request, and its answer goes back to the page
// that asked. A file already shown brings its window forward instead.
func TestAPageGetsAnotherWindowForARequest(t *testing.T) {
	h := start(t, t.TempDir(), true)
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first.md"), filepath.Join(dir, "second.md")
	put(t, first, "first\n")
	put(t, second, "second\n")
	p := h.open(first)
	// A file the person has not chosen in this window is not handed on.
	request := `{"operation":"open-file","input":{"path":` + string(mustJSON(second)) + `}}`
	if problem := p.call(nil, "OpenWindow", "editor", request); problem == "" {
		t.Fatal("a window was opened for a file the page was not given")
	}
	h.s.SetDialogs([]string{second}, nil)
	var picked Picked
	if problem := p.call(&picked, "PickOpen"); problem != "" || picked.Path != second || picked.Name != "second.md" {
		t.Fatalf("picked %+v %s", picked, problem)
	}
	answered := make(chan string, 1)
	var result string
	go func() { answered <- p.call(&result, "OpenWindow", "editor", request) }()
	var started []Started
	deadline := time.Now().Add(5 * time.Second)
	for len(started) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no window was started")
		}
		time.Sleep(20 * time.Millisecond)
		started = h.s.Started()
	}
	if started[0].Path != second || started[0].Raised || started[0].Entry == "" {
		t.Fatalf("started %+v", started[0])
	}
	p2 := h.enter(started[0].ID, started[0].Entry)
	var at Attached
	p2.call(&at, "Attach")
	if at.Resource != second || at.Request != request {
		t.Fatalf("the new window was given %+v", at)
	}
	var r ReadResult
	if problem := p2.call(&r, "Read", second); problem != "" || r.Text != "second\n" {
		t.Fatalf("the new window's read: %+v %s", r, problem)
	}
	p2.call(nil, "Accepted", `{"output":{"opened":true}}`, "")
	if problem := <-answered; problem != "" || result != `{"output":{"opened":true}}` {
		t.Fatalf("the asking page was answered %q %s", result, problem)
	}
	// The same request again brings that window forward and starts none.
	if problem := p.call(&result, "OpenWindow", "editor", request); problem != "" || !strings.Contains(result, `"raised":true`) {
		t.Fatalf("the second request: %q %s", result, problem)
	}
	if all := h.s.Started(); len(all) != 2 || !all[1].Raised || all[1].ID != p2.id {
		t.Fatalf("started %+v", all)
	}
	// A new window that fails says so to the page that asked.
	go func() {
		answered <- p.call(&result, "OpenWindow", "editor", `{"operation":"new","input":{"text":"x"}}`)
	}()
	for len(h.s.Started()) < 3 {
		time.Sleep(20 * time.Millisecond)
	}
	third := h.s.Started()[2]
	h.enter(third.ID, third.Entry).call(nil, "Accepted", "", "The applet did not start.")
	if problem := <-answered; problem != "The applet did not start." {
		t.Fatalf("the asking page was told %q", problem)
	}
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

// A document reads a file it refers to from its own folder and below, and
// a picture beside it, and nothing anywhere else.
func TestADocumentReadsOnlyFromItsOwnFolder(t *testing.T) {
	h := start(t, t.TempDir(), false)
	root := t.TempDir()
	dir := filepath.Join(root, "docs")
	os.MkdirAll(filepath.Join(dir, "images"), 0o755)
	file := filepath.Join(dir, "notes.md")
	put(t, file, "one\n")
	put(t, filepath.Join(dir, "plan.canvas"), "{}")
	put(t, filepath.Join(root, "secret.txt"), "secret")
	put(t, filepath.Join(dir, "images", "shot.png"), "png bytes")
	put(t, filepath.Join(dir, "notes.txt"), "not a picture")
	p := h.open(file)
	var folder string
	p.call(&folder, "Directory", file)
	if folder != virtualPath(dir) || !strings.HasPrefix(folder, "/") {
		t.Fatalf("the folder is %q", folder)
	}
	var found FoundText
	if problem := p.call(&found, "ReadText", folder+"/plan.canvas", 0); problem != "" || !found.Found || found.Text != "{}" {
		t.Fatalf("a file beside the document: %+v %s", found, problem)
	}
	if problem := p.call(&found, "ReadText", folder+"/missing.canvas", 0); problem != "" || found.Found {
		t.Fatalf("a missing file: %+v %s", found, problem)
	}
	for _, outside := range []string{folder + "/../secret.txt", virtualPath(filepath.Join(root, "secret.txt"))} {
		found = FoundText{}
		if problem := p.call(&found, "ReadText", outside, 0); found.Found {
			t.Fatalf("a file outside the folder was read through %s: %+v %s", outside, found, problem)
		}
	}
	get := func(name string) (int, string) {
		res, err := p.http.Get(p.base() + "/file?" + url.Values{"doc": {file}, "name": {name}}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(data)
	}
	if code, text := get("shot.png"); code != http.StatusOK || text != "png bytes" {
		t.Fatalf("a picture beside the document: %d %q", code, text)
	}
	if code, _ := get("notes.txt"); code != http.StatusNotFound {
		t.Fatalf("a file that is not a picture: %d", code)
	}
	if code, _ := get("../secret.png"); code != http.StatusNotFound {
		t.Fatalf("a picture outside the folder: %d", code)
	}
}

// An action sent to a document goes to its page, and the page's answer goes
// back to whoever sent it.
func TestAnActionGoesToTheDocumentsPageAndItsAnswerBack(t *testing.T) {
	h := start(t, t.TempDir(), false)
	file := filepath.Join(t.TempDir(), "notes.md")
	put(t, file, "one\n")
	p := h.open(file)
	ctx := context.Background()
	c := h.client()
	// The page listens for its events, as a browser's does.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.base()+"/bridge/events", nil)
	stream, err := p.http.Do(req)
	if err != nil || stream.StatusCode != http.StatusOK {
		t.Fatalf("the page's events: %v %v", stream, err)
	}
	defer stream.Body.Close()
	type outcome struct {
		result json.RawMessage
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := c.Invoke(ctx, p.id, json.RawMessage(`{"operation":"read","input":{}}`), "an agent")
		done <- outcome{result, err}
	}()
	buf := make([]byte, 0, 4096)
	chunk := make([]byte, 1024)
	var call string
	for call == "" {
		n, err := stream.Body.Read(chunk)
		if err != nil {
			t.Fatal(err)
		}
		buf = append(buf, chunk[:n]...)
		for _, line := range strings.Split(string(buf), "\n") {
			if !strings.HasPrefix(line, "data:") || !strings.Contains(line, `"invoke"`) {
				continue
			}
			var event struct {
				Data struct {
					Call, Request, From string
				}
			}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), &event) == nil && event.Data.Call != "" {
				if event.Data.From != "an agent" || !strings.Contains(event.Data.Request, `"read"`) {
					t.Fatalf("the page was sent %+v", event.Data)
				}
				call = event.Data.Call
			}
		}
	}
	p.call(nil, "Answer", call, `{"output":{"text":"one\n"}}`, "")
	got := <-done
	if got.err != nil || string(got.result) != `{"output":{"text":"one\n"}}` {
		t.Fatalf("the sender was answered %s %v", got.result, got.err)
	}
	if _, err := c.Invoke(ctx, "no such document", json.RawMessage(`{}`), "an agent"); err == nil {
		t.Error("an action for a document that is not open was accepted")
	}
}

// The applications' settings are kept for the next window.
func TestSettingsAreKeptForTheNextWindow(t *testing.T) {
	state := t.TempDir()
	h := start(t, state, false)
	file := filepath.Join(t.TempDir(), "notes.md")
	put(t, file, "one\n")
	p := h.open(file)
	var all map[string]json.RawMessage
	if problem := p.call(&all, "ConfigSet", "editor", `{"wrap":true,"view":"text"}`); problem != "" || string(all["view"]) != `"text"` {
		t.Fatalf("set %v %s", all, problem)
	}
	p.call(&all, "ConfigSet", "editor", `{"view":"visual"}`)
	h.s.Stop()
	time.Sleep(100 * time.Millisecond)
	h2 := start(t, state, false)
	var loaded map[string]json.RawMessage
	h2.open(file).call(&loaded, "ConfigLoad", "editor")
	if string(loaded["wrap"]) != "true" || string(loaded["view"]) != `"visual"` {
		t.Fatalf("loaded %v", loaded)
	}
	var none map[string]json.RawMessage
	h2.open("").call(&none, "ConfigLoad", "docs")
	if len(none) != 0 {
		t.Fatalf("another application's settings: %v", none)
	}
}

// Two programs that want the service at once start one between them.
func TestTwoStartsMakeOneService(t *testing.T) {
	dir := t.TempDir()
	files := fstest.MapFS{"document.html": {Data: []byte("x")}}
	starts := 0
	begin := func() error {
		starts++
		s, err := New(Options{Dir: dir, Files: http.FileServerFS(files), Page: "document.html", Cookie: "doc", Idle: time.Hour,
			Applications: map[string]Application{"editor": {Applet: "editor", Name: "Test Editor"}}})
		if err != nil {
			return err
		}
		go s.Serve()
		t.Cleanup(s.Stop)
		return nil
	}
	found := make(chan *Client, 2)
	for range 2 {
		go func() {
			c, err := Ensure(dir, begin)
			if err != nil {
				t.Error(err)
			}
			found <- c
		}()
	}
	a, b := <-found, <-found
	if a == nil || b == nil || a.Endpoint.Addr != b.Endpoint.Addr || starts != 1 {
		t.Fatalf("%d services were started", starts)
	}
}
