package httphead

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/resident"
	"dinah/internal/resident/residenttest"
	"dinah/internal/verb"
)

// waitDeadline bounds every wait for a publish. It bounds the test only.
const waitDeadline = 30 * time.Second

// residentHandle is what a resident fixture keeps beside the head: the
// resident, the notifier the test drives, and the publishes it has seen.
type residentHandle struct {
	w         *resident.Workbench
	manual    *residenttest.Manual
	published chan resident.Published
	passed    *pathLog
}

// pathLog collects the paths a hook reports.
type pathLog struct {
	mu    sync.Mutex
	paths []string
}

func (p *pathLog) add(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paths = append(p.paths, path)
}

func (p *pathLog) take() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	taken := p.paths
	p.paths = nil
	return taken
}

// withResident is an option that populates the workbench, then opens a
// resident over it with a hand-driven notifier, waits for the first snapshot
// and serves it.
func withResident(t *testing.T, handle *residentHandle, populate func(root, home string)) func(*Config) {
	return func(cfg *Config) {
		t.Helper()
		if populate != nil {
			populate(cfg.Root, cfg.Home)
		}
		handle.manual = residenttest.NewManual()
		handle.published = make(chan resident.Published, 256)
		handle.passed = &pathLog{}
		hooks := &resident.Hooks{
			AfterPublish: func(p resident.Published) {
				select {
				case handle.published <- p:
				default:
				}
			},
			PassThrough: handle.passed.add,
		}
		w, err := resident.Open(cfg.Root, resident.Options{Notifier: handle.manual, Hooks: hooks})
		if err != nil {
			t.Fatalf("open the resident: %v", err)
		}
		t.Cleanup(func() { w.Close() })
		select {
		case <-w.Ready():
		case <-time.After(waitDeadline):
			t.Fatal("the resident's first build did not finish")
		}
		handle.w = w
		cfg.Resident = w
	}
}

// await waits for a publish satisfying want.
func (h *residentHandle) await(t *testing.T, what string, want func(resident.Published) bool) resident.Published {
	t.Helper()
	timeout := time.After(waitDeadline)
	for {
		select {
		case p := <-h.published:
			if want(p) {
				return p
			}
		case <-timeout:
			t.Fatalf("no publish %s arrived within %s", what, waitDeadline)
			return resident.Published{}
		}
	}
}

// drain forgets every publish seen so far.
func (h *residentHandle) drain() {
	for {
		select {
		case <-h.published:
		default:
			return
		}
	}
}

// rebuild reports an overflow and waits for the rebuild it causes, which is
// how a test brings the resident up to writes it made after the first build.
func (h *residentHandle) rebuild(t *testing.T) {
	t.Helper()
	h.drain()
	h.manual.Overflow()
	h.await(t, "rebuilding after the overflow", func(p resident.Published) bool { return p.Rebuilt })
}

// fresh opens the workbench from disk the way another process would.
func fresh(t *testing.T, root, home string) *verb.Library {
	t.Helper()
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return verb.New(opened, home)
}

// richRefs are the references of what populateRich wrote.
type richRefs struct {
	card, item, archived string
}

// populateRich writes the workbench the resident tests read: a card at Build
// carrying a decision, a comment on the card and one on the decision, and an
// attachment; a comment on a column; a workstream; and an archived card. So
// every entity that bears a journal carries one, and every collection the
// fixture holds has a member, which is what lets a read that bypasses the
// resident change an answer once the root is gone.
func populateRich(t *testing.T, root, home string) richRefs {
	t.Helper()
	ok := func(what string, response *verb.Response) *verb.Response {
		t.Helper()
		if response.Outcome != contract.OutcomeOK {
			t.Fatalf("%s: %+v", what, response)
		}
		return response
	}
	var refs richRefs
	refs.card = ok("add", fresh(t, root, home).Add(&verb.Request{Verb: "add", Actor: "alka", Title: "A card", Column: "build"})).Card.Ref
	ok("file", fresh(t, root, home).File(&verb.Request{Verb: "file", Actor: "alka", Card: refs.card, Kind: "decision", Text: "A decision."}))
	refs.item = refs.card + "/decisions/1"
	ok("comment on the card", fresh(t, root, home).Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: refs.card, Text: "A note on the card."}))
	ok("comment on the item", fresh(t, root, home).Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: refs.item, Text: "A note on the decision."}))
	ok("comment on a column", fresh(t, root, home).Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: "build", Text: "A note on the column."}))
	attached := filepath.Join(home, "notes.txt")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attached, []byte("an attached note, searchable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok("attach", fresh(t, root, home).Attach(&verb.Request{Verb: "attach", Actor: "alka", Ref: refs.card, File: attached}))
	ok("workstream", fresh(t, root, home).NewWorkstream(&verb.Request{Verb: "workstream", Actor: "alka", Action: "new", Workstream: "Stream", Slug: "stream"}))
	refs.archived = ok("add the archived card", fresh(t, root, home).Add(&verb.Request{Verb: "add", Actor: "alka", Title: "An old card", Column: "build"})).Card.Ref
	ok("archive", fresh(t, root, home).Archive(&verb.Request{Verb: "archive", Actor: "alka", Ref: refs.archived}))
	return refs
}

// getPaths answers every GET row of the route table, filled for the rich
// fixture, with the rows below a card and a column spelled out for each
// collection the fixture holds.
func getPaths(refs richRefs) []string {
	var paths []string
	for _, r := range routes {
		if r.read == nil || r.ignoresAccept {
			continue
		}
		path := strings.TrimSuffix(r.pattern, "{$}")
		for from, to := range map[string]string{"{card}": refs.card, "{column}": "build", "{slug}": "stream", "{view}": "mine"} {
			path = strings.ReplaceAll(path, from, to)
		}
		switch {
		case strings.HasPrefix(r.pattern, "/cards/{card}/{rest...}"):
			for _, rest := range []string{"comments", "comments/1", "decisions", "decisions/1", "decisions/1/comments", "decisions/1/comments/1", "attachments", "attachments/1"} {
				paths = append(paths, strings.ReplaceAll(path, "{rest...}", rest))
			}
		case strings.HasPrefix(r.pattern, "/columns/{column}/{rest...}"):
			for _, rest := range []string{"comments", "comments/1"} {
				paths = append(paths, strings.ReplaceAll(path, "{rest...}", rest))
			}
		case r.pattern == "/search":
			paths = append(paths, path+"?phrase=note", path+"?phrase=searchable")
		default:
			paths = append(paths, path)
		}
	}
	return paths
}

// sendTo serves one request on a handler and records the reply.
func sendTo(h http.Handler, port int, method, path, accept string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.Host = fmt.Sprintf("127.0.0.1:%d", port)
	request.Header.Set("Accept", accept)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	return recorder
}

// sameReply reports how two replies differ: status, every header but Date,
// and body.
func sameReply(a, b *httptest.ResponseRecorder) string {
	if a.Code != b.Code {
		return fmt.Sprintf("status %d against %d", a.Code, b.Code)
	}
	ah, bh := a.Header().Clone(), b.Header().Clone()
	ah.Del("Date")
	bh.Del("Date")
	if !reflect.DeepEqual(ah, bh) {
		return fmt.Sprintf("headers %v against %v", ah, bh)
	}
	if a.Body.String() != b.Body.String() {
		return "bodies differ:\n" + firstDifference(a.Body.String(), b.Body.String())
	}
	return ""
}

// firstDifference shows where two strings first differ.
func firstDifference(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	from := max(0, i-80)
	return fmt.Sprintf("  resident: %q\n  disk:     %q", a[from:min(len(a), i+80)], b[from:min(len(b), i+80)])
}

// TestEveryReadAnswersTheSameFromTheResident is dinah-619/criteria/1. Every
// GET row of the route table, asked for JSON and for HTML, answers with the
// same status, headers and body from a head holding a resident as from one
// reading the disk on the same workbench.
//
// Arming: answering a journal's Stat from a separate stat rather than from
// the held bytes' length changes the changes cursor a page carries.
func TestEveryReadAnswersTheSameFromTheResident(t *testing.T) {
	handle := &residentHandle{}
	var refs richRefs
	f := newFixture(t, func(cfg *Config) { cfg.ParseLine = stubParser }, withResident(t, handle, func(root, home string) { refs = populateRich(t, root, home) }))
	diskConfig := f.cfg
	diskConfig.Resident = nil
	fromDisk, fromResident := Handler(diskConfig), Handler(f.cfg)
	compared := 0
	for _, path := range getPaths(refs) {
		for _, accept := range []string{typeJSON, browserAccept} {
			got := sendTo(fromResident, f.port, http.MethodGet, path, accept)
			want := sendTo(fromDisk, f.port, http.MethodGet, path, accept)
			if diff := sameReply(got, want); diff != "" {
				t.Errorf("GET %s as %s: %s", path, accept, diff)
			}
			compared++
		}
	}
	if passed := handle.passed.take(); len(passed) > 0 {
		t.Errorf("the resident passed %d reads through to the disk: %v", len(passed), passed[:min(len(passed), 5)])
	}
	paths := getPaths(refs)
	t.Logf("compared %d requests over %d GET paths from %d GET rows", compared, len(paths), getRowCount())
	if compared != 2*len(paths) || len(paths) != getRowCount()+9 {
		t.Fatalf("compared %d requests over %d paths, and the route table's %d GET rows and the collections below them make more", compared, len(paths), getRowCount())
	}
}

// TestWarmReadsTouchNoDisk is dinah-619/criteria/2. After one round of every
// GET, the root is renamed away with no change delivered, and every GET
// answers exactly as it did warm, with no read below the old root passing
// through to the disk.
//
// The probe sees a read that bypasses the resident only where the file it
// reads is there before the rename, which is why the fixture gives every
// entity a journal and every collection a member; an absence that holds by
// design is the static guards' to catch.
//
// Arming: reading a search's attachment head with os.Open rather than
// through the workbench changes the GET /search answer once the root is gone.
func TestWarmReadsTouchNoDisk(t *testing.T) {
	handle := &residentHandle{}
	var refs richRefs
	f := newFixture(t, func(cfg *Config) { cfg.ParseLine = stubParser }, withResident(t, handle, func(root, home string) { refs = populateRich(t, root, home) }))
	h := Handler(f.cfg)
	paths := getPaths(refs)
	warm := map[string]*httptest.ResponseRecorder{}
	for _, path := range paths {
		for _, accept := range []string{typeJSON, browserAccept} {
			warm[accept+" "+path] = sendTo(h, f.port, http.MethodGet, path, accept)
		}
	}
	handle.passed.take()
	moved := f.root + "-renamed-away"
	if err := os.Rename(f.root, moved); err != nil {
		t.Fatalf("rename the root away: %v", err)
	}
	t.Cleanup(func() { os.Rename(moved, f.root) })
	compared := 0
	for _, path := range paths {
		for _, accept := range []string{typeJSON, browserAccept} {
			got := sendTo(h, f.port, http.MethodGet, path, accept)
			if diff := sameReply(got, warm[accept+" "+path]); diff != "" {
				t.Errorf("GET %s as %s after the root was renamed away: %s", path, accept, diff)
			}
			compared++
		}
	}
	for _, path := range handle.passed.take() {
		if strings.HasPrefix(path, f.root+string(filepath.Separator)) {
			t.Errorf("a read below the old root passed through to the disk: %s", path)
		}
	}
	t.Logf("compared %d requests warm and with the root gone", compared)
	if compared != len(warm) || compared != 2*len(paths) || len(paths) != getRowCount()+9 {
		t.Fatalf("compared %d requests of %d warm over %d paths", compared, len(warm), len(paths))
	}
}

// TestAnActReadsTheDisk is dinah-619/criteria/3. With the resident held
// unchanged, a PATCH move carrying the ETag the resident served, after
// another library moved the card, answers 412 with the disk's revision; and a
// POST /cards after another library's add mints the next number, one line
// per number in the registry.
//
// Arming: opening an act on the resident lets the stale ETag through.
func TestAnActReadsTheDisk(t *testing.T) {
	handle := &residentHandle{}
	var refs richRefs
	f := newFixture(t, withResident(t, handle, func(root, home string) { refs = populateRich(t, root, home) }))
	handle.manual.Hold()
	served := f.get("/cards/"+refs.card, "Accept", typeJSON)
	etag := served.header.Get("ETag")
	if etag == "" {
		t.Fatalf("the resident served no ETag: %d %s", served.status, served.body)
	}
	f.act(&verb.Request{Verb: verb.Move, Actor: "alka", Card: refs.card, Column: "review"})
	onDisk := f.card(refs.card)
	moved := f.json(http.MethodPatch, "/cards/"+refs.card, typeMove, `{"column": "build"}`, "If-Match", etag)
	if moved.status != http.StatusPreconditionFailed {
		t.Errorf("a move carrying the resident's ETag after another library's move answered %d %s, wanted 412", moved.status, moved.body)
	}
	if got := moved.header.Get("ETag"); got != quoted(onDisk.Revision) {
		t.Errorf("the 412 carried the revision %s, and the disk's is %s", got, quoted(onDisk.Revision))
	}
	if after := f.card(refs.card); after.Column != onDisk.Column {
		t.Errorf("the card stands in %s after the refused move, not where the other library put it", after.Column)
	}

	other := fresh(t, f.root, f.home).Add(&verb.Request{Verb: "add", Actor: "alka", Title: "Added by another process", Column: "build"})
	if other.Card == nil {
		t.Fatalf("the other library's add: %+v", other)
	}
	added := f.json(http.MethodPost, "/cards", typeJSON, `{"title": "Added over HTTP", "column": "build"}`)
	if added.status != http.StatusCreated {
		t.Fatalf("POST /cards answered %d %s", added.status, added.body)
	}
	card, _ := added.object(t)["card"].(map[string]any)
	ref, _ := card["ref"].(string)
	if want := "ht-" + fmt.Sprint(numberOf(other.Card.Ref)+1); ref != want {
		t.Errorf("POST /cards minted %s after the other library's %s, wanted %s", ref, other.Card.Ref, want)
	}
	lines, err := os.ReadFile(filepath.Join(f.root, bench.CardNumbersName))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(lines)), "\n") {
		counts[strings.Fields(line)[0]]++
	}
	for number, n := range counts {
		if n != 1 {
			t.Errorf("the registry carries %d lines for number %s", n, number)
		}
	}
	handle.manual.Release()
}

// numberOf reads the number off a reference.
func numberOf(ref string) int {
	var n int
	fmt.Sscanf(ref[strings.LastIndex(ref, "-")+1:], "%d", &n)
	return n
}

// laneOf answers the title of the board lane a page draws a card in, empty
// when it draws the card in none.
func laneOf(page *node, ref string) string {
	for _, lane := range page.all(withClass("lane")) {
		for _, card := range lane.all(withClass("lane-card")) {
			if card.attr["data-win-open"] == ref {
				return lane.attr["aria-label"]
			}
		}
	}
	return ""
}

// detailText answers the text of a page's detail pane, which leaves out the
// command log, where every act the pages performed is written out again.
func detailText(page *node) string {
	detail := page.first(withClass("md-detail"))
	if detail == nil {
		return ""
	}
	return detail.allText()
}

// laneHolding answers the title of the board lane that draws a card with the
// given title, empty when none does.
func laneHolding(page *node, title string) string {
	for _, lane := range page.all(withClass("lane")) {
		for _, card := range lane.all(withClass("lane-card")) {
			if strings.Contains(card.allText(), title) {
				return lane.attr["aria-label"]
			}
		}
	}
	return ""
}

// sheetColumn answers the column title a page's card sheet carries, which is
// its first chip.
func sheetColumn(page *node) string {
	sheet := page.first(withClass("card-sheet"))
	if sheet == nil {
		return ""
	}
	chip := sheet.first(withClass("chip"))
	if chip == nil {
		return ""
	}
	return strings.TrimSpace(chip.allText())
}

// typedParser is the parser this file's typed lines go through: move, comment
// and add, the three the settle cases type.
func typedParser(words []string) (TypedLine, *contract.Refusal) {
	switch {
	case len(words) == 3 && words[0] == "move":
		return TypedLine{Command: verb.Move, Arguments: map[string]any{"card": words[1], "column": words[2]}}, nil
	case len(words) >= 3 && words[0] == "comment":
		return TypedLine{Command: "comment", Arguments: map[string]any{"card": words[1], "text": strings.Join(words[2:], " ")}}, nil
	case len(words) >= 2 && words[0] == "add":
		return TypedLine{Command: "add", Arguments: map[string]any{"title": strings.Join(words[1:], " "), "column": "build"}}, nil
	}
	return TypedLine{}, contract.Refuse(contract.UnknownVerb, strings.Join(words, " "))
}

// TestAPageSeesItsOwnAct is dinah-619/criteria/4's page half. With every
// notification held back, so nothing but the act's own settle can bring its
// write into the resident, each act answers 303 and the page after it shows
// the act, while another library's move is not drawn until its change is
// delivered and applied. A move and an add are read off the page the redirect
// names; a column comment is read off the comment's own page, because the
// column's comments listing draws no comment's body, and its page is the
// first the settle has to have brought in.
//
// Arming: removing the settle reddens all five; settling only the answer's
// card reddens the two column comments, whose answers carry no card.
func TestAPageSeesItsOwnAct(t *testing.T) {
	cases := []struct {
		name string
		act  func(f *fixture, refs richRefs) reply
		sees func(t *testing.T, f *fixture, refs richRefs, redirected *node) bool
	}{
		{"a form move posted from the card page", func(f *fixture, refs richRefs) reply {
			return f.pageForm("/cards/"+refs.card, "_method=PATCH&_type="+url.QueryEscape(typeMove)+"&column=review&_basis=*", "/cards/"+refs.card)
		}, func(t *testing.T, f *fixture, refs richRefs, page *node) bool {
			return sheetColumn(page) == "Review" && laneOf(parseHTML(t, f.page("/").body), refs.card) == "Review"
		}},
		{"a form comment posted to a column's comments", func(f *fixture, refs richRefs) reply {
			return f.pageForm("/columns/build/comments", "text=A+column+note+from+a+form", "/columns/build/comments")
		}, func(t *testing.T, f *fixture, refs richRefs, page *node) bool {
			return strings.Contains(detailText(parseHTML(t, f.page("/columns/build/comments/2").body)), "A column note from a form")
		}},
		{"a typed move", func(f *fixture, refs richRefs) reply {
			return f.pageForm("/commands", "line="+url.QueryEscape("move "+refs.card+" review"), "/")
		}, func(t *testing.T, f *fixture, refs richRefs, page *node) bool {
			return laneOf(page, refs.card) == "Review"
		}},
		{"a typed comment on a column", func(f *fixture, refs richRefs) reply {
			return f.pageForm("/commands", "line="+url.QueryEscape("comment build A typed column note"), "/columns/build/comments")
		}, func(t *testing.T, f *fixture, refs richRefs, page *node) bool {
			return strings.Contains(detailText(parseHTML(t, f.page("/columns/build/comments/2").body)), "A typed column note")
		}},
		{"a typed add", func(f *fixture, refs richRefs) reply {
			return f.pageForm("/commands", "line="+url.QueryEscape("add A typed card"), "/")
		}, func(t *testing.T, f *fixture, refs richRefs, page *node) bool {
			return laneHolding(page, "A typed card") != ""
		}},
	}
	if len(cases) != 5 {
		t.Fatalf("ran %d cases, wanted the five dinah-619 section 12.4 names", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handle := &residentHandle{}
			var refs richRefs
			var other string
			f := newFixture(t, func(cfg *Config) { cfg.ParseLine = typedParser }, withResident(t, handle, func(root, home string) {
				refs = populateRich(t, root, home)
				other = fresh(t, root, home).Add(&verb.Request{Verb: "add", Actor: "alka", Title: "Another card", Column: "build"}).Card.Ref
			}))
			handle.manual.Hold()
			f.act(&verb.Request{Verb: verb.Move, Actor: "alka", Card: other, Column: "review"})
			answered := c.act(f, refs)
			if answered.status != http.StatusSeeOther {
				t.Fatalf("the act answered %d, wanted 303: %s", answered.status, answered.body)
			}
			location := answered.header.Get("Location")
			page := parseHTML(t, f.page(location).body)
			if !c.sees(t, f, refs, page) {
				t.Errorf("the page the redirect names (%s) does not show the act", location)
			}
			if lane := laneOf(parseHTML(t, f.page("/").body), other); lane != "Build" {
				t.Errorf("another library's move is drawn (in %q) before its change was delivered", lane)
			}
			handle.drain()
			handle.manual.Deliver(resident.Change{Path: filepath.Join(bench.CardsDir, idOf(t, f, other), bench.CardAnchor), Action: resident.Modified})
			handle.manual.Release()
			handle.await(t, "applying the other library's move", func(p resident.Published) bool { return !p.Rebuilt })
			if lane := laneOf(parseHTML(t, f.page("/").body), other); lane != "Review" {
				t.Errorf("another library's move is not drawn (in %q) after its change was applied", lane)
			}
		})
	}
}

// idOf answers a card's identifier as the disk has it.
func idOf(t *testing.T, f *fixture, ref string) string {
	t.Helper()
	return f.card(ref).ID
}

// sidebarCount answers the count the sidebar draws beside a column.
func sidebarCount(page *node, title string) string {
	for _, row := range page.all(withClass("md-item")) {
		count := row.first(withClass("md-count"))
		if count != nil && strings.TrimSpace(strings.TrimSuffix(row.allText(), count.allText())) == title {
			return strings.TrimSpace(count.allText())
		}
	}
	return ""
}

// TestAPageStraddlingAPublishIsConsistent is dinah-619/criteria/5's page
// half. Between the page-state reads and the route's own read, another
// library moves the card and the resident publishes the move. The card page
// still draws the board as it stood, its sidebar counting the card in Build,
// and the card's own sheet in Build too, because one request reads one
// snapshot; the next page draws the move in both.
//
// Arming: having contextLibrary call Current a second time draws the
// instructions, which the page reads after the route's own read, from the
// newer snapshot, beside a board and a sheet from the older.
func TestAPageStraddlingAPublishIsConsistent(t *testing.T) {
	handle := &residentHandle{}
	var refs richRefs
	var armed atomic.Bool
	var once sync.Once
	var fx *fixture
	f := newFixture(t, withResident(t, handle, func(root, home string) { refs = populateRich(t, root, home) }), func(cfg *Config) {
		cfg.BeforeRun = func() {
			if !armed.Load() {
				return
			}
			once.Do(func() {
				other := fresh(t, fx.root, fx.home)
				if response := other.Do(&verb.Request{Verb: verb.Move, Actor: "alka", Card: refs.card, Column: "review"}); response.Outcome != contract.OutcomeOK {
					t.Errorf("the other library's move: %+v", response)
				}
				resolved, err := fresh(t, fx.root, fx.home).Bench.ResolveCard(refs.card)
				if err != nil {
					t.Errorf("resolve the moved card: %v", err)
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), waitDeadline)
				defer cancel()
				if err := handle.w.Settle(ctx, resolved.Card.Dir); err != nil {
					t.Errorf("settle: %v", err)
				}
			})
		}
	})
	fx = f
	before := parseHTML(t, f.page("/cards/"+refs.card).body)
	build, review := sidebarCount(before, "Build"), sidebarCount(before, "Review")
	if build == "" || review == "" || sheetColumn(before) != "Build" {
		t.Fatalf("the page before the move counts Build %q and Review %q and draws the sheet in %q", build, review, sheetColumn(before))
	}
	armed.Store(true)
	page := parseHTML(t, f.page("/cards/"+refs.card).body)
	if got := [3]string{sidebarCount(page, "Build"), sidebarCount(page, "Review"), sheetColumn(page)}; got != [3]string{build, review, "Build"} {
		t.Errorf("the page straddling the publish drew Build %s, Review %s and the sheet in %s; wanted %s, %s and Build, all from the one snapshot", got[0], got[1], got[2], build, review)
	}
	// The instructions the sheet draws are read after the route's own read,
	// through the page's library, so they carry the column layer of
	// whichever snapshot that library holds.
	if text := detailText(page); !strings.Contains(text, "Build text.") || strings.Contains(text, "Review text.") {
		t.Errorf("the page straddling the publish drew the instructions of another snapshot than its board and sheet")
	}
	next := parseHTML(t, f.page("/cards/"+refs.card).body)
	if sidebarCount(next, "Build") == build || sidebarCount(next, "Review") == review || sheetColumn(next) != "Review" {
		t.Errorf("the next page drew Build %s, Review %s and the sheet in %s, which is not the move", sidebarCount(next, "Build"), sidebarCount(next, "Review"), sheetColumn(next))
	}
}

// TestAPollAnswersFromTheResident is dinah-619/criteria/11. A page's cursor
// answers changed false while another library's move is held back, and
// changed true with the move once its change is applied; and a cursor minted
// with no resident on the same bytes equals the resident's.
func TestAPollAnswersFromTheResident(t *testing.T) {
	handle := &residentHandle{}
	var refs richRefs
	f := newFixture(t, withResident(t, handle, func(root, home string) { refs = populateRich(t, root, home) }))
	diskConfig := f.cfg
	diskConfig.Resident = nil
	minted := func(h http.Handler) string {
		got := sendTo(h, f.port, http.MethodGet, "/changes", typeJSON)
		var body struct {
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(got.Body.Bytes(), &body); err != nil || body.Cursor == "" {
			t.Fatalf("mint a cursor: %d %s", got.Code, got.Body.String())
		}
		return body.Cursor
	}
	if a, b := minted(Handler(f.cfg)), minted(Handler(diskConfig)); a != b {
		t.Errorf("a cursor minted on the resident (%s) differs from one minted on the disk (%s) over the same bytes", a, b)
	}
	cursor := parseHTML(t, f.page("/").body).first(withClass("md-layout")).attr["data-changes-cursor"]
	if cursor == "" {
		t.Fatal("the board carries no cursor")
	}
	poll := func() map[string]any {
		got := f.get("/changes?since="+url.QueryEscape(cursor), "Accept", typeJSON)
		if got.status != http.StatusOK {
			t.Fatalf("changes answered %d %s", got.status, got.body)
		}
		return got.object(t)
	}
	handle.manual.Hold()
	f.act(&verb.Request{Verb: verb.Move, Actor: "alka", Card: refs.card, Column: "review"})
	if changed, _ := poll()["changed"].(bool); changed {
		t.Error("the poll answered changed while the move's change was held back")
	}
	id := f.card(refs.card).ID
	handle.drain()
	handle.manual.Deliver(
		resident.Change{Path: filepath.Join(bench.CardsDir, id, bench.CardAnchor), Action: resident.Modified},
		resident.Change{Path: filepath.Join(bench.CardsDir, id, bench.JournalName), Action: resident.Modified},
	)
	handle.manual.Release()
	handle.await(t, "applying the move", func(p resident.Published) bool { return !p.Rebuilt })
	answer := poll()
	if changed, _ := answer["changed"].(bool); !changed {
		t.Error("the poll answered unchanged after the move was applied")
	}
	events, _ := answer["events"].([]any)
	found := false
	for _, e := range events {
		if event, ok := e.(map[string]any); ok && event["event"] == contract.EventMoved {
			found = true
		}
	}
	if !found {
		t.Errorf("the poll after the move carries no moved event: %v", answer)
	}
}

// TestALapsedClaimIsLapsedByTheRead is dinah-619/criteria/10's page half. A
// card whose anchor carries a claim expired before the resident was opened
// is read with GET /: the request reads the disk, the card's journal gains
// the lapse, and the answer shows the card ready. The next GET / reads the
// resident and answers the same.
func TestALapsedClaimIsLapsedByTheRead(t *testing.T) {
	handle := &residentHandle{}
	sources := &sourceLogger{}
	var card string
	f := newFixture(t, func(cfg *Config) { cfg.observeSource = sources.add }, withResident(t, handle, func(root, home string) {
		card = fresh(t, root, home).Add(&verb.Request{Verb: "add", Actor: "alka", Title: "A leased card", Column: "build"}).Card.Ref
		if response := fresh(t, root, home).Do(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: card, Expires: time.Hour}); response.Outcome != contract.OutcomeOK {
			t.Fatalf("claim: %+v", response)
		}
		resolved, err := fresh(t, root, home).Bench.ResolveCard(card)
		if err != nil {
			t.Fatal(err)
		}
		anchor := resolved.Card.AnchorPath()
		text, err := os.ReadFile(anchor)
		if err != nil {
			t.Fatal(err)
		}
		past := bench.Stamp(time.Now().Add(-time.Hour))
		rewritten := strings.Replace(string(text), "claim_expires: "+resolved.Card.Expires, "claim_expires: "+past, 1)
		if rewritten == string(text) {
			t.Fatalf("the anchor carries no claim_expires to move into the past:\n%s", text)
		}
		if err := os.WriteFile(anchor, []byte(rewritten), 0o644); err != nil {
			t.Fatal(err)
		}
	}))
	sources.take()
	first := f.get("/", "Accept", typeJSON)
	if got := sources.take(); len(got) != 1 || got[0] {
		t.Errorf("the first GET / reported the source %v, wanted the disk", got)
	}
	lapsed := false
	for _, event := range f.journal(card) {
		if event.Event == contract.EventExpired {
			lapsed = true
		}
	}
	if !lapsed {
		t.Error("the read did not journal the lapse")
	}
	if state := f.card(card).State; state != contract.StateReady {
		t.Errorf("the card reads %s after the lapse, wanted ready", state)
	}
	second := f.get("/", "Accept", typeJSON)
	if got := sources.take(); len(got) != 1 || !got[0] {
		t.Errorf("the next GET / reported the source %v, wanted the resident", got)
	}
	if first.status != second.status || first.body != second.body {
		t.Errorf("the next GET / answered differently from the resident:\n%s\n%s", first.body, second.body)
	}
}

// TestEveryPageCarriesACursorChangesAcceptsOnTheResident runs
// TestEveryPageCarriesACursorChangesAccepts's body a second time with the
// resident fixture, delivering the move's changes through the notifier.
func TestEveryPageCarriesACursorChangesAcceptsOnTheResident(t *testing.T) {
	handle := &residentHandle{}
	f := newPageFixture(t, withResident(t, handle, nil))
	handle.rebuild(t)
	everyPageCarriesACursor(t, f, func() {
		id := f.fixture.card(f.card).ID
		handle.drain()
		handle.manual.Deliver(
			resident.Change{Path: filepath.Join(bench.CardsDir, id, bench.CardAnchor), Action: resident.Modified},
			resident.Change{Path: filepath.Join(bench.CardsDir, id, bench.JournalName), Action: resident.Modified},
		)
		handle.await(t, "applying the move", func(p resident.Published) bool { return !p.Rebuilt })
	})
}

// commandLogger records the commands TimeRead saw, in order.
type commandLogger struct {
	mu       sync.Mutex
	commands []string
}

func (c *commandLogger) add(command string, _ time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commands = append(c.commands, command)
}

func (c *commandLogger) take() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	taken := c.commands
	c.commands = nil
	return taken
}

// sourceLogger records what observeSource reported.
type sourceLogger struct {
	mu      sync.Mutex
	choices []bool
}

func (s *sourceLogger) add(fromResident bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.choices = append(s.choices, fromResident)
}

func (s *sourceLogger) take() []bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	taken := s.choices
	s.choices = nil
	return taken
}

// TestTimeReadSeesEveryLibraryCall is part of dinah-619/criteria/14. For the
// HTML card page with no window open, TimeRead sees the library acquisition
// and every library call the head makes, in order, on the resident and on the
// disk path. The disk path opens the workbench twice, once for the page's own
// reads and once for the route's, as it always has, and each open is timed;
// the resident path acquires its one library once. The page makes no show of
// its own when no window is open, so the route's show is the only one. The
// 10ms criterion is only as good as this test, because a TimeRead that
// skipped a call would meet the number for free.
//
// Arming: removing the timing of open from open and contextLibrary drops the
// opens from both sequences, and removing it from readFor drops changes,
// status, tree and instructions.
func TestTimeReadSeesEveryLibraryCall(t *testing.T) {
	cases := []struct {
		name     string
		resident bool
		want     []string
	}{
		{"resident", true, []string{"open", "changes", "status", "tree", "show", "instructions"}},
		{"disk", false, []string{"open", "changes", "status", "tree", "open", "show", "instructions"}},
	}
	for _, c := range cases {
		logger := &commandLogger{}
		sources := &sourceLogger{}
		handle := &residentHandle{}
		options := []func(*Config){func(cfg *Config) {
			cfg.TimeRead = logger.add
			cfg.observeSource = sources.add
		}}
		var card string
		populate := func(root, home string) {
			response := fresh(t, root, home).Add(&verb.Request{Verb: "add", Actor: "alka", Title: "A card", Column: "build"})
			if response.Card == nil {
				t.Fatalf("add: %+v", response)
			}
			card = response.Card.Ref
		}
		if c.resident {
			options = append(options, withResident(t, handle, populate))
		}
		f := newFixture(t, options...)
		if !c.resident {
			card = f.add("A card", "build")
		}
		logger.take()
		sources.take()
		answered := f.page("/cards/" + card)
		if answered.status != 200 {
			t.Fatalf("%s: GET the card page answered %d", c.name, answered.status)
		}
		if got := logger.take(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: TimeRead saw %v, wanted %v", c.name, got, c.want)
		}
		if got := sources.take(); len(got) != 1 || got[0] != c.resident {
			t.Errorf("%s: observeSource reported %v, wanted one choice of %v", c.name, got, c.resident)
		}
	}
}
