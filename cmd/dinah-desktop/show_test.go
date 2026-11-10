package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestChooseShowDisplay(t *testing.T) {
	main := showDisplay{X: 0, Y: 0, Width: 1920, Height: 1080, Primary: true, Current: true}
	small := showDisplay{X: 1920, Y: 0, Width: 1280, Height: 720}
	large := showDisplay{X: -3840, Y: 0, Width: 3840, Height: 2160}
	same := showDisplay{X: 0, Y: 2160, Width: 3840, Height: 2160, Primary: true}
	for _, c := range []struct {
		name string
		all  []showDisplay
		want showDisplay
		ok   bool
	}{
		{"one display has nowhere else", []showDisplay{main}, showDisplay{}, false},
		{"none", nil, showDisplay{}, false},
		{"two displays: the other one", []showDisplay{main, small}, small, true},
		{"three: the largest the window is not on", []showDisplay{main, small, large}, large, true},
		{"a tie goes to the primary display", []showDisplay{{X: 9, Width: 1920, Height: 1080, Current: true}, large, same}, same, true},
		{"a tie with no primary goes to the earlier", []showDisplay{main, small, {X: 5000, Width: 1280, Height: 720}}, small, true},
		{"the window's display is not known", []showDisplay{{Width: 1920, Height: 1080}, small}, showDisplay{}, false},
		{"a display with no area is none", []showDisplay{main, {X: 1920}}, showDisplay{}, false},
	} {
		got, ok := chooseShowDisplay(c.all)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("%s: %+v %v, want %+v %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestShowRectsRoundTrip(t *testing.T) {
	d := showDisplay{X: -3840, Y: 12, Width: 3840, Height: 2160}
	if got, ok := parseShowRect(formatShowRect(d)); !ok || got != d {
		t.Fatalf("%+v %v", got, ok)
	}
	for _, bad := range []string{"", "1,2,3", "1,2,0,4", "a,b,c,d", "1,2,3,4,5"} {
		if _, ok := parseShowRect(bad); ok {
			t.Errorf("%q was read as a rectangle", bad)
		}
	}
}

// showFixture is an App under test with its displays and a served page.
type showFixture struct {
	app    *App
	server *shellServer
	file   string
	mu     sync.Mutex
	events []map[string]any
}

func newShowFixture(t *testing.T, displays string) *showFixture {
	t.Helper()
	f := &showFixture{file: filepath.Join(t.TempDir(), "entry.txt")}
	t.Setenv(product.ShellEntryVar(), f.file)
	t.Setenv(showDisplaysVar(), displays)
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		t.Fatal(err)
	}
	f.app = &App{}
	f.app.sink = func(name string, data any) {
		if strings.HasPrefix(name, "show:") {
			f.mu.Lock()
			f.events = append(f.events, map[string]any{"name": name, "data": data})
			f.mu.Unlock()
		}
	}
	f.server = &shellServer{app: f.app, files: http.FileServerFS(frontend), addr: "127.0.0.1:47616", token: "token", codes: map[string]shellCode{}}
	f.app.shows.entry = f.server.showEntry
	t.Cleanup(f.app.closeShows)
	return f
}

func (f *showFixture) seen(name string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []map[string]any
	for _, e := range f.events {
		if e["name"] == name {
			out = append(out, e["data"].(map[string]any))
		}
	}
	return out
}

func (f *showFixture) waitClosed() []map[string]any {
	deadline := time.Now().Add(5 * time.Second)
	for len(f.seen("show:closed")) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	return f.seen("show:closed")
}

// signIn follows a show's one-time address and returns its cookie.
func (f *showFixture) signIn(t *testing.T) *http.Cookie {
	t.Helper()
	entry, err := os.ReadFile(f.file + ".show")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	f.server.ServeHTTP(w, httptest.NewRequest("GET", string(entry), nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "url=/"+showPagePath) {
		t.Fatalf("the show's way in: %d %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == showCookie {
			return c
		}
	}
	t.Fatal("the show's way in set no show cookie")
	return nil
}

func (f *showFixture) request(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:47616"+path, strings.NewReader(body))
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if method != "GET" {
		r.Header.Set("Origin", "http://127.0.0.1:47616")
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	f.server.ServeHTTP(w, r)
	return w
}

func (f *showFixture) call(t *testing.T, cookie *http.Cookie, method string, args ...any) (json.RawMessage, string) {
	t.Helper()
	if args == nil {
		args = []any{}
	}
	body, _ := json.Marshal(map[string]any{"method": method, "args": args})
	w := f.request("POST", "/bridge/call", string(body), cookie)
	if w.Code != http.StatusOK {
		return nil, strings.TrimSpace(w.Body.String())
	}
	var a struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	return a.Result, a.Error
}

// With one display there is nowhere else for a show, and nothing starts.
func TestAShowNeedsAnotherDisplay(t *testing.T) {
	f := newShowFixture(t, "0,0,1920,1080")
	opened, err := f.app.OpenShow(ShowRequest{Application: "slides", Resource: "r", Draft: "d"})
	if err != nil || opened.ID != "" {
		t.Fatalf("%+v %v", opened, err)
	}
	if _, err := os.Stat(f.file + ".show"); err == nil {
		t.Fatal("a show was started with one display")
	}
}

// The two pages' messages are carried in order, those posted before the
// show's page reads included, and the show's page reaches what a slide is
// drawn from and nothing else of the App.
func TestAShowCarriesMessagesAndReadsOnlyItsDeck(t *testing.T) {
	f := newShowFixture(t, "0,0,1920,1080;1920,0,2560,1440")
	opened, err := f.app.OpenShow(ShowRequest{Application: "slides", Resource: "deck", Draft: "draft-1", Folder: "/workspaces/w/decks"})
	if err != nil || opened.ID == "" {
		t.Fatalf("%+v %v", opened, err)
	}
	state, at := `{"v":1,"type":"state"}`, `{"v":1,"type":"at","slide":2}`
	for _, m := range []string{state, at} {
		if err := f.app.PostShow(opened.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.app.PostShow("another", "{}"); err == nil {
		t.Fatal("a message was posted to a show that is not running")
	}
	if err := f.app.PostShow(opened.ID, strings.Repeat("x", showMessageLimit+1)); err == nil {
		t.Fatal("a message over the limit was posted")
	}
	cookie := f.signIn(t)
	// The way in is good once.
	entry, _ := os.ReadFile(f.file + ".show")
	again := httptest.NewRecorder()
	f.server.ServeHTTP(again, httptest.NewRequest("GET", string(entry), nil))
	if again.Code != http.StatusUnauthorized {
		t.Fatalf("the way in worked twice: %d", again.Code)
	}
	result, failed := f.call(t, cookie, "Attach")
	var attached ShowAttached
	if failed != "" || json.Unmarshal(result, &attached) != nil {
		t.Fatalf("attach: %s %s", result, failed)
	}
	if attached.Resource != "deck" || attached.Folder != "/workspaces/w/decks" || len(attached.Messages) != 2 || attached.Messages[0] != state || attached.Messages[1] != at {
		t.Fatalf("attached %+v", attached)
	}
	// After the page reads, a message is an event of its bridge and is not kept.
	if err := f.app.PostShow(opened.ID, `{"v":1,"type":"end"}`); err != nil {
		t.Fatal(err)
	}
	if result, _ := f.call(t, cookie, "Attach"); !strings.Contains(string(result), `"messages":[]`) {
		t.Fatalf("a message was kept after the page read: %s", result)
	}
	ready := `{"v":1,"type":"ready"}`
	if _, failed := f.call(t, cookie, "Post", ready); failed != "" {
		t.Fatal(failed)
	}
	if got := f.seen("show:message"); len(got) != 1 || got[0]["id"] != opened.ID || got[0]["message"] != ready {
		t.Fatalf("the editing window heard %+v", got)
	}
	if result, failed := f.call(t, cookie, "ShortcodeCatalog"); failed != "" {
		t.Fatalf("the catalogue: %s %s", result, failed)
	}
	// Nothing else of the App answers the show's cookie.
	for _, method := range []string{"State", "GetDraft", "SaveDraft", "Send", "OpenShow", "ReadFile"} {
		if result, failed := f.call(t, cookie, method, "x"); failed == "" {
			t.Errorf("%s answered the show's page: %s", method, result)
		}
	}
	for path, want := range map[string]int{
		"/" + showPagePath:                       http.StatusOK,
		"/bridge/app.js":                         http.StatusOK,
		"/draft-file?draft=another&name=a.png":   http.StatusForbidden,
		"/doc-file?doc=a&name=a.png":             http.StatusForbidden,
		"/bus-attachment?message=1&index=1":      http.StatusForbidden,
		"/resource-attachment?resource=a&name=b": http.StatusForbidden,
	} {
		if w := f.request("GET", path, "", cookie); w.Code != want {
			t.Errorf("GET %s: %d, want %d", path, w.Code, want)
		}
	}
	if w := f.request("POST", "/upload", "", cookie); w.Code != http.StatusForbidden {
		t.Errorf("an upload from the show's page: %d", w.Code)
	}
	// The window's token is not a show's.
	if w := f.request("GET", "/", "", &http.Cookie{Name: showCookie, Value: "token"}); w.Code != http.StatusUnauthorized {
		t.Errorf("the window's token as a show's cookie: %d", w.Code)
	}
	// Ending the show tells the editing window, and its cookie then opens nothing.
	if err := f.app.CloseShow(opened.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.waitClosed(); len(got) != 1 || got[0]["id"] != opened.ID {
		t.Fatalf("closed: %+v", got)
	}
	if w := f.request("GET", "/"+showPagePath, "", cookie); w.Code != http.StatusUnauthorized {
		t.Errorf("an ended show's cookie: %d", w.Code)
	}
	if err := f.app.PostShow(opened.ID, "{}"); err == nil {
		t.Fatal("a message was posted to an ended show")
	}
}

// A second show ends the first.
func TestANewShowEndsTheOneBefore(t *testing.T) {
	f := newShowFixture(t, "0,0,1920,1080;1920,0,2560,1440")
	first, err := f.app.OpenShow(ShowRequest{Application: "slides", Resource: "a", Draft: "a"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.app.OpenShow(ShowRequest{Application: "slides", Resource: "b", Draft: "b"})
	if err != nil || second.ID == "" || second.ID == first.ID {
		t.Fatalf("%+v %v", second, err)
	}
	if got := f.waitClosed(); len(got) != 1 || got[0]["id"] != first.ID {
		t.Fatalf("closed: %+v", got)
	}
	if f.app.PostShow(second.ID, "{}") != nil || f.app.PostShow(first.ID, "{}") == nil {
		t.Fatal("the wrong show is running")
	}
}
