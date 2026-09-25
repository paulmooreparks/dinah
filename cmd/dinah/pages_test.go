package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/httphead"
)

// The typed line's tests live here rather than beside the rest of the pages'
// tests in internal/httphead, because the parser the typed line runs is the
// terminal's own, in this package, and the head cannot import it. Each builds
// the handler the way serveUntil builds it.

// pagesServer is one handler serving a workbench on a loopback listener.
type pagesServer struct {
	t    *testing.T
	root string
	base string
	port int
}

// startPages serves the workbench under root with the handler serveUntil
// builds, acting as alka by default.
func startPages(t *testing.T, root string) *pagesServer {
	t.Helper()
	anchor := runCLI(t, root, "path", "workbench")
	if anchor.code != 0 {
		t.Fatalf("path workbench: %d %s", anchor.code, anchor.errw)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	home := bench.Home()
	server := &http.Server{Handler: httphead.Handler(httphead.Config{
		Root:         filepath.Dir(strings.TrimSpace(anchor.out)),
		Home:         home,
		DefaultActor: "alka",
		Agent:        bench.Agent{Harness: "fixture"},
		Host:         "127.0.0.1",
		Port:         port,
		Lang:         "en",
		ParseLine:    typedLineParser(bench.LoadConfig(home)),
	})}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	return &pagesServer{t: t, root: root, base: "http://127.0.0.1:" + strconv.Itoa(port), port: port}
}

// pageReply is one response.
type pageReply struct {
	status int
	header http.Header
	body   string
}

// do sends one request without following a redirect.
func (p *pagesServer) do(method, path, body string, header map[string]string) pageReply {
	p.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, p.base+path, reader)
	if err != nil {
		p.t.Fatalf("build %s %s: %v", method, path, err)
	}
	for name, value := range header {
		if name == "Host" {
			req.Host = value
			continue
		}
		req.Header.Set(name, value)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		p.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer response.Body.Close()
	read, _ := io.ReadAll(response.Body)
	return pageReply{status: response.StatusCode, header: response.Header, body: string(read)}
}

// browserNavigation is the Accept header a browser navigating sends.
const browserNavigation = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

// command posts a form to POST /commands the way a browser submits one from
// the board. The header list overrides any header, and "-" leaves one out.
func (p *pagesServer) command(form url.Values, header ...string) pageReply {
	p.t.Helper()
	headers := map[string]string{
		"Content-Type":   "application/x-www-form-urlencoded",
		"Origin":         p.base,
		"Sec-Fetch-Site": "same-origin",
		"Referer":        p.base + "/",
		"Accept":         browserNavigation,
	}
	for i := 0; i+1 < len(header); i += 2 {
		headers[header[i]] = header[i+1]
	}
	for name, value := range headers {
		if value == "-" {
			delete(headers, name)
		}
	}
	return p.do(http.MethodPost, "/commands", form.Encode(), headers)
}

// logEntry is one command log entry as the log page draws it.
type logEntry struct {
	outcome, line, text string
	again               string
}

// entryPattern reads the log page's entries.
var (
	entryPattern   = regexp.MustCompile(`(?s)<li class="cmdlog-entry" data-outcome="([a-z]+)">(.*?)</li>`)
	codePattern    = regexp.MustCompile(`(?s)<code class="cmdlog-cmd">(.*?)</code>`)
	againPattern   = regexp.MustCompile(`(?s)name="entry" value="[0-9]+" /><button class="btn btn-sm" type="submit">(.*?)</button>`)
	tagPattern     = regexp.MustCompile(`<[^>]+>`)
	detailBoundary = `<section class="md-detail"`
)

// log reads the whole command log, newest first.
func (p *pagesServer) log() []logEntry {
	p.t.Helper()
	page := p.do(http.MethodGet, "/commands", "", map[string]string{"Accept": browserNavigation}).body
	start := strings.Index(page, detailBoundary)
	if start < 0 {
		p.t.Fatalf("the log page has no detail pane:\n%s", page)
	}
	end := strings.Index(page, `<section class="cmdlog"`)
	if end < start {
		p.t.Fatalf("the log page has no log panel after its detail pane:\n%s", page)
	}
	var entries []logEntry
	for _, m := range entryPattern.FindAllStringSubmatch(page[start:end], -1) {
		e := logEntry{outcome: m[1], text: htmlText(tagPattern.ReplaceAllString(m[2], " "))}
		if code := codePattern.FindStringSubmatch(m[2]); code != nil {
			e.line = htmlText(code[1])
		}
		if again := againPattern.FindStringSubmatch(m[2]); again != nil {
			e.again = htmlText(again[1])
		}
		entries = append(entries, e)
	}
	return entries
}

// htmlText undoes the escapes html/template writes into text.
func htmlText(s string) string {
	return strings.NewReplacer("&#34;", `"`, "&#39;", "'", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`).Replace(s)
}

// cardMember reads one member of a card as the terminal's --json shows it.
func cardMember(t *testing.T, root, card, member string) string {
	t.Helper()
	got := runCLI(t, root, "show", card, "--json")
	if got.code != 0 {
		t.Fatalf("show %s: %d %s", card, got.code, got.errw)
	}
	var shown struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal([]byte(got.out), &shown); err != nil {
		t.Fatalf("show %s: %v", card, err)
	}
	value, _ := shown.Card[member].(string)
	return value
}

// cardColumn is the slug of the column a card stands in, which is its
// title lowered on the default workbench.
func cardColumn(t *testing.T, root, card string) string {
	t.Helper()
	return strings.ToLower(cardMember(t, root, card, "column_title"))
}

// workbenchFiles reads every file of the workbench, so a refused request can be
// held to changing nothing on disk.
func workbenchFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	filepath.WalkDir(filepath.Join(root, ".dinah"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files[path], _ = os.ReadFile(path)
		}
		return nil
	})
	if len(files) == 0 {
		t.Fatal("the workbench holds no file")
	}
	return files
}

// sameFiles reports whether two snapshots hold the same bytes.
func sameFiles(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for path, data := range a {
		if !bytes.Equal(data, b[path]) {
			return false
		}
	}
	return true
}

// TestTheTypedLine drives the typed line through the terminal's parser.
func TestTheTypedLine(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	p := startPages(t, root)

	moved := p.command(url.Values{"line": {"move fx-1 doing"}})
	if moved.status != http.StatusSeeOther || moved.header.Get("Location") != "/" || cardColumn(t, root, "fx-1") != "doing" {
		t.Errorf("a typed move: %d %q, card in %s", moved.status, moved.header.Get("Location"), cardColumn(t, root, "fx-1"))
	}
	shown := p.command(url.Values{"line": {"show fx-1"}})
	if shown.status != http.StatusSeeOther || shown.header.Get("Location") != "/cards/fx-1" {
		t.Errorf("a typed read: %d %q", shown.status, shown.header.Get("Location"))
	}
	before := workbenchFiles(t, root)
	for _, line := range []string{"file fx-1 decision x", "resolve fx-1/decisions/1 done", "path fx-1", "serve", "ui"} {
		got := p.command(url.Values{"line": {line}})
		if got.status != http.StatusSeeOther {
			t.Errorf("%q: wanted 303, got %d", line, got.status)
		}
	}
	if !sameFiles(before, workbenchFiles(t, root)) {
		t.Error("a command the pages do not serve changed the workbench")
	}
	log := p.log()
	if len(log) != 7 {
		t.Fatalf("the log holds %d entries, wanted seven", len(log))
	}
	if log[6].outcome != "ok" || log[6].line != "dinah move fx-1 doing" || log[6].again != "Run again on the card as it stands now" {
		t.Errorf("the typed move is logged %+v", log[6])
	}
	if log[5].outcome != "read" || log[5].line != "dinah show fx-1" {
		t.Errorf("the typed read is logged %+v", log[5])
	}
	if log[4].outcome != "refused" || log[4].line != "dinah file fx-1 decision x" || log[4].again != "" || !strings.Contains(log[4].text, "dinah.not-served") {
		t.Errorf("a command the pages do not serve is logged %+v", log[4])
	}
	for _, e := range log[:4] {
		if !strings.Contains(e.text, "dinah.not-served") {
			t.Errorf("%q is logged %+v, wanted dinah.not-served", e.line, e)
		}
	}
	for _, row := range []struct {
		name string
		form url.Values
		code int
	}{
		{"line=", url.Values{"line": {""}}, http.StatusSeeOther},
		{"an evicted entry", url.Values{"entry": {"999"}}, http.StatusBadRequest},
		{"entry=abc", url.Values{"entry": {"abc"}}, http.StatusBadRequest},
		{"both members", url.Values{"line": {"show fx-1"}, "entry": {"1"}}, http.StatusBadRequest},
		{"_basis", url.Values{"line": {"show fx-1"}, "_basis": {"*"}}, http.StatusBadRequest},
		{"_type", url.Values{"line": {"show fx-1"}, "_type": {"x"}}, http.StatusBadRequest},
		{"_actor", url.Values{"line": {"show fx-1"}, "_actor": {"bryn"}}, http.StatusBadRequest},
	} {
		if got := p.command(row.form); got.status != row.code {
			t.Errorf("%s: wanted %d, got %d %s", row.name, row.code, got.status, got.body)
		}
	}
	if got := p.command(url.Values{"line": {"claim fx-1 --actor claude"}}, "Dinah-Actor", "someone"); got.status != http.StatusBadRequest || !strings.Contains(got.body, "--actor") {
		t.Errorf("a Dinah-Actor disagreeing with --actor: wanted 400 naming --actor, got %d", got.status)
	}
	if len(p.log()) != 7 {
		t.Errorf("a refused request recorded an entry: %d", len(p.log()))
	}
	claimed := p.command(url.Values{"line": {"claim fx-1"}}, "Dinah-Actor", "someone")
	if claimed.status != http.StatusSeeOther {
		t.Errorf("a Dinah-Actor with no typed --actor: %d", claimed.status)
	}
	if holder := cardMember(t, root, "fx-1", "holder"); holder != "someone" {
		t.Errorf("the claim acted as %q, wanted the header's someone", holder)
	}
}

// TestRunAgainIsGuardedOnlyWhereABasisWasSent holds Run again to the basis
// the original act carried: a form's entry answers stale once another
// process has moved the card, and a typed entry, which carries none, re-runs
// against the card as it stands.
func TestRunAgainIsGuardedOnlyWhereABasisWasSent(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "Guarded")
	runCLI(t, root, "add", "Unchanged")
	runCLI(t, root, "add", "Typed")
	runCLI(t, root, "add", "Occupant")
	p := startPages(t, root)
	revision := func(card string) string { return cardMember(t, root, card, "revision") }
	form := func(card, column string) pageReply {
		values := url.Values{"_method": {"PATCH"}, "_type": {"application/vnd.dinah.move+json"}, "_basis": {revision(card)}, "column": {column}}
		return p.do(http.MethodPost, "/cards/"+card, values.Encode(), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded", "Origin": p.base, "Sec-Fetch-Site": "same-origin",
			"Referer": p.base + "/cards/" + card, "Accept": browserNavigation,
		})
	}
	again := func(seq int) pageReply { return p.command(url.Values{"entry": {strconv.Itoa(seq)}}) }

	form("fx-1", "doing")
	if log := p.log(); len(log) != 1 || log[0].outcome != "ok" || log[0].again != "Run again" {
		t.Fatalf("the form move is logged %+v", log)
	}
	// A card's revision is a digest of what it stores, so moving it back to
	// where it stood would give it the revision the page drew again. The
	// other process moves it on instead.
	runCLI(t, root, "move", "fx-1", "done")
	if got := again(1); got.status != http.StatusSeeOther {
		t.Errorf("Run again answered %d", got.status)
	}
	if log := p.log(); log[0].outcome != "stale" || !strings.Contains(log[0].text, "fx-1") || cardColumn(t, root, "fx-1") != "done" {
		t.Errorf("Run again of a form entry after the card moved: %+v, card in %s", log[0], cardColumn(t, root, "fx-1"))
	}

	// The accepting twin: an entry whose basis still names the card. A move
	// that succeeds changes the card, so its own entry is stale from the
	// moment it lands; the entry here is a move the column's capacity
	// refused, which leaves the card as it was, and the capacity is freed
	// by moving another card rather than this one.
	if got := runCLI(t, root, "set", "doing", "capacity", "1"); got.code != 0 {
		t.Fatalf("set capacity: %d %s", got.code, got.errw)
	}
	runCLI(t, root, "move", "fx-4", "doing")
	form("fx-2", "doing")
	log := p.log()
	if log[0].outcome != "refused" {
		t.Fatalf("the move into a full column is logged %+v", log[0])
	}
	runCLI(t, root, "move", "fx-4", "done")
	again(len(log))
	if log := p.log(); log[0].outcome != "ok" || cardColumn(t, root, "fx-2") != "doing" {
		t.Errorf("Run again of a form entry on an unchanged card: %+v, card in %s", log[0], cardColumn(t, root, "fx-2"))
	}

	// The unguarded row: a typed move carries no basis.
	runCLI(t, root, "move", "fx-3", "doing")
	runCLI(t, root, "set", "doing", "capacity", "")
	p.command(url.Values{"line": {"move fx-3 intake"}})
	typed := len(p.log())
	if log := p.log(); log[0].again != "Run again on the card as it stands now" {
		t.Errorf("a typed entry is labelled %q", log[0].again)
	}
	runCLI(t, root, "move", "fx-3", "done")
	again(typed)
	if log := p.log(); log[0].outcome != "ok" || cardColumn(t, root, "fx-3") != "intake" {
		t.Errorf("Run again of a typed entry after the card moved: %+v, card in %s", log[0], cardColumn(t, root, "fx-3"))
	}
}

// TestCommandsRefusesWhatComesFromElsewhere holds POST /commands to
// dinah-152's admission: each shape a page on another site could send is
// refused as that admission refuses it, with the card unchanged and nothing
// logged, beside the same-origin forms it admits.
func TestCommandsRefusesWhatComesFromElsewhere(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	p := startPages(t, root)
	line := url.Values{"line": {"move fx-1 doing"}}.Encode()
	evil := "evil.example:" + strconv.Itoa(p.port)
	before := workbenchFiles(t, root)
	for _, row := range []struct {
		name    string
		header  []string
		status  int
		refusal string
	}{
		{"a foreign Origin", []string{"Origin", "http://evil.example"}, 403, "dinah.foreign-origin"},
		{"Origin null", []string{"Origin", "null"}, 403, "dinah.foreign-origin"},
		{"Sec-Fetch-Site cross-site", []string{"Sec-Fetch-Site", "cross-site"}, 403, "dinah.foreign-origin"},
		{"Sec-Fetch-Site same-site with no Origin", []string{"Sec-Fetch-Site", "same-site", "Origin", "-"}, 403, "dinah.foreign-origin"},
		{"a form with neither proof", []string{"Sec-Fetch-Site", "-", "Origin", "-"}, 403, "dinah.origin-required"},
		{"a form with Sec-Fetch-Site none", []string{"Sec-Fetch-Site", "none", "Origin", "-"}, 403, "dinah.origin-required"},
		{"a JSON body", []string{"Content-Type", "application/json", `{"line": "move fx-1 doing"}`}, 415, "dinah.unsupported-media-type"},
		{"a text body", []string{"Content-Type", "text/plain", line}, 415, "dinah.unsupported-media-type"},
		{"an empty body with no type", []string{"Content-Type", "-", ""}, 415, "dinah.unsupported-media-type"},
		{"DNS rebinding", []string{"Host", evil, "Origin", "http://" + evil}, 421, "dinah.foreign-host"},
	} {
		header := row.header
		body := line
		if len(header)%2 == 1 {
			body = header[len(header)-1]
			header = header[:len(header)-1]
		}
		headers := map[string]string{
			"Content-Type": "application/x-www-form-urlencoded", "Origin": p.base, "Sec-Fetch-Site": "same-origin",
			"Referer": p.base + "/", "Accept": browserNavigation,
		}
		for i := 0; i+1 < len(header); i += 2 {
			headers[header[i]] = header[i+1]
		}
		for name, value := range headers {
			if value == "-" {
				delete(headers, name)
			}
		}
		got := p.do(http.MethodPost, "/commands", body, headers)
		if got.status != row.status || !strings.Contains(got.body, row.refusal) {
			t.Errorf("%s: wanted %d %s, got %d", row.name, row.status, row.refusal, got.status)
		}
		if row.status == 415 && got.header.Get("Accept-Post") != "application/x-www-form-urlencoded" {
			t.Errorf("%s: Accept-Post is %q", row.name, got.header.Get("Accept-Post"))
		}
		if !sameFiles(before, workbenchFiles(t, root)) || len(p.log()) != 0 {
			t.Fatalf("%s changed the workbench or recorded an entry", row.name)
		}
	}
	if got := p.command(url.Values{"line": {"move fx-1 doing"}}); got.status != http.StatusSeeOther || cardColumn(t, root, "fx-1") != "doing" || len(p.log()) != 1 {
		t.Errorf("the same-origin form: %d, card in %s, %d entries", got.status, cardColumn(t, root, "fx-1"), len(p.log()))
	}
	if got := p.command(url.Values{"line": {"move fx-1 intake"}}, "Origin", "-"); got.status != http.StatusSeeOther || cardColumn(t, root, "fx-1") != "intake" || len(p.log()) != 2 {
		t.Errorf("a form with Sec-Fetch-Site same-origin and no Origin: %d, card in %s, %d entries", got.status, cardColumn(t, root, "fx-1"), len(p.log()))
	}
}

// TestAScriptlessReaderTypesACommand submits the board's command line form
// with a move, as a client with no script engine does.
func TestAScriptlessReaderTypesACommand(t *testing.T) {
	root := newBench(t)
	runCLI(t, root, "add", "A card")
	p := startPages(t, root)
	board := p.do(http.MethodGet, "/", "", map[string]string{"Accept": browserNavigation}).body
	if !strings.Contains(board, `<form class="cmdlog-line" method="post" action="/commands">`) || !strings.Contains(board, `name="line"`) {
		t.Fatal("the board carries no command line form")
	}
	got := p.command(url.Values{"line": {"move fx-1 doing"}})
	if got.status != http.StatusSeeOther || cardColumn(t, root, "fx-1") != "doing" {
		t.Errorf("the typed move: %d, card in %s", got.status, cardColumn(t, root, "fx-1"))
	}
	back := p.do(http.MethodGet, got.header.Get("Location"), "", map[string]string{"Accept": browserNavigation}).body
	if !strings.Contains(back, "dinah move fx-1 doing") {
		t.Error("the page the reader returned to does not show the move in its log")
	}
	t.Log("submitted 1 form")
}
