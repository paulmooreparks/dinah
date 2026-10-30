package web

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/bus"
	"studio/internal/regional"
)

const addr = "127.0.0.1:47615"

func newServer(t *testing.T) (*Server, *bus.Bus) {
	t.Helper()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	s, err := New(b, addr)
	if err != nil {
		t.Fatal(err)
	}
	return s, b
}

func do(s *Server, method, target string, body url.Values, header map[string]string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, strings.NewReader(body.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.Host = addr
	for k, v := range header {
		if k == "Host" {
			r.Host = v
			continue
		}
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func signIn(t *testing.T, s *Server) string {
	t.Helper()
	u, err := s.SigninURL(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	w := do(s, "GET", strings.TrimPrefix(u, "http://"+addr), nil, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `url=/"`) {
		t.Fatalf("sign-in: %d %s", w.Code, w.Body)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName {
			return c.Name + "=" + c.Value
		}
	}
	t.Fatal("sign-in set no cookie")
	return ""
}

func TestForeignHostIsRefused(t *testing.T) {
	s, _ := newServer(t)
	cookie := signIn(t, s)
	if w := do(s, "GET", "/", nil, map[string]string{"Host": "evil.example:47615", "Cookie": cookie}); w.Code != http.StatusMisdirectedRequest {
		t.Fatalf("rebinding host: %d", w.Code)
	}
}

func TestPagesNeedSignIn(t *testing.T) {
	s, _ := newServer(t)
	if w := do(s, "GET", "/", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: %d", w.Code)
	}
	cookie := signIn(t, s)
	for _, p := range []string{"/", "/rooms", "/releases", "/bus"} {
		if w := do(s, "GET", p, nil, map[string]string{"Cookie": cookie}); w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", p, w.Code, w.Body)
		}
	}
}

func TestFormsMustComeFromThePage(t *testing.T) {
	s, b := newServer(t)
	cookie := signIn(t, s)
	ctx := context.Background()
	dir := t.TempDir()
	if _, err := b.RegisterSession(ctx, "s1", dir, ""); err != nil {
		t.Fatal(err)
	}
	ws := strings.ToLower(filepath.Base(dir))
	form := url.Values{"to": {ws}, "text": {"hello"}, "back": {"/"}}
	if w := do(s, "POST", "/messages", form, map[string]string{"Cookie": cookie, "Origin": "https://evil.example"}); w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin post: %d", w.Code)
	}
	if w := do(s, "POST", "/messages", form, map[string]string{"Cookie": cookie, "Origin": "null", "Sec-Fetch-Site": "cross-site"}); w.Code != http.StatusForbidden {
		t.Fatalf("null-origin cross-site post: %d", w.Code)
	}
	if w := do(s, "POST", "/messages", form, map[string]string{"Cookie": cookie, "Origin": "null", "Sec-Fetch-Site": "same-origin"}); w.Code != http.StatusSeeOther {
		t.Fatalf("null-origin same-origin post: %d %s", w.Code, w.Body)
	}
	b.Deliver(ctx, "s1", "tool")
	if w := do(s, "POST", "/messages", form, map[string]string{"Cookie": cookie, "Origin": "http://" + addr}); w.Code != http.StatusSeeOther {
		t.Fatalf("same-origin post: %d %s", w.Code, w.Body)
	}
	msgs, _, err := b.Deliver(ctx, "s1", "tool")
	if err != nil || len(msgs) != 1 || msgs[0].FromName != bus.Human {
		t.Fatalf("delivery: %v %+v", err, msgs)
	}
	if w := do(s, "GET", "/messages/1", nil, map[string]string{"Cookie": cookie}); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("thread: %d", w.Code)
	}
}

// The bus's page shows a chat by its title and a workspace by its label,
// sends to a chat by its title, and never shows a chat's handle.
func TestThePageShowsTitles(t *testing.T) {
	s, b := newServer(t)
	cookie := signIn(t, s)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "parser")
	b.LabelWorkspace(ctx, dir, "Parser Library")
	chat, _ := b.RegisterSession(ctx, "s1", dir, "Parser Release")
	if _, err := b.Send(ctx, bus.Sender{Session: &chat}, bus.Human, "Tagged.", nil, 0); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"to": {"Parser Release (parser)"}, "text": {"Thanks."}, "back": {"/"}}
	if w := do(s, "POST", "/messages", form, map[string]string{"Cookie": cookie, "Origin": "http://" + addr}); w.Code != http.StatusSeeOther || strings.Contains(w.Header().Get("Location"), "error") {
		t.Fatalf("sending to a title: %d %s", w.Code, w.Header().Get("Location"))
	}
	for _, page := range []string{"/bus", "/inbox"} {
		body := do(s, "GET", page, nil, map[string]string{"Cookie": cookie}).Body.String()
		if !strings.Contains(body, "Parser Release") || !strings.Contains(body, "Parser Library") || strings.Contains(body, "parser-1") {
			t.Fatalf("%s does not show the chat by its title:\n%s", page, body)
		}
	}
}

// The page starts a group chat of chats by their titles, shows it by its
// name with its members' titles, and says in a line who was added.
func TestThePageStartsAGroupOfChats(t *testing.T) {
	s, b := newServer(t)
	cookie := signIn(t, s)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "parser")
	b.RegisterSession(ctx, "s1", dir, "Parser Release")
	b.RegisterSession(ctx, "s2", dir, "Docs Pass")
	post := map[string]string{"Cookie": cookie, "Origin": "http://" + addr}
	w := do(s, "POST", "/rooms", url.Values{"members": {"Parser Release"}, "name": {"Release Review"}}, post)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/rooms/release-review" {
		t.Fatalf("starting a group: %d %s", w.Code, w.Header().Get("Location"))
	}
	if w := do(s, "POST", "/rooms/release-review/members", url.Values{"member": {"Docs Pass"}}, post); w.Code != http.StatusSeeOther || strings.Contains(w.Header().Get("Location"), "error") {
		t.Fatalf("adding a chat: %d %s", w.Code, w.Header().Get("Location"))
	}
	body := do(s, "GET", "/rooms/release-review", nil, map[string]string{"Cookie": cookie}).Body.String()
	for _, want := range []string{"<h1>Release Review</h1>", "Members: Parser Release, Docs Pass, you", "added Docs Pass to the group."} {
		if !strings.Contains(body, want) {
			t.Fatalf("the group's page lacks %q:\n%s", want, body)
		}
	}
	if list := do(s, "GET", "/rooms", nil, map[string]string{"Cookie": cookie}).Body.String(); !strings.Contains(list, "Group Chats") || !strings.Contains(list, "Release Review") {
		t.Fatalf("the list of group chats:\n%s", list)
	}
}

// A bus time is a time element with its datetime and the computer's
// regional format, which the page script writes it again in, and Go's own
// short form as its text for a page without script.
func TestTimesCarryTheRegionalFormat(t *testing.T) {
	at := time.Date(2026, 10, 7, 21, 5, 0, 0, time.Local)
	got := string(when(at.Format(time.RFC3339Nano)))
	want := `<time datetime="` + at.Format(time.RFC3339) + `" data-locale="` + template.HTMLEscapeString(regional.HostLocale()) + `">7 Oct 21:05</time>`
	if got != want {
		t.Fatalf("when = %s, want %s", got, want)
	}
	now := time.Now()
	if got := string(when(now.Format(time.RFC3339Nano))); !strings.HasSuffix(got, ">"+now.Format("15:04")+"</time>") {
		t.Fatalf("a time today should show the time of day: %s", got)
	}
	if got := string(when(`<b>`)); got != "&lt;b&gt;" {
		t.Fatalf("a stamp that is not a time should be escaped: %s", got)
	}
}
