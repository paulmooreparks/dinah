package httphead

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/answer"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/pages"
	"dinah/internal/verb"
)

// wantedCSP is the whole Content-Security-Policy of dinah-338 section 4.4.
const wantedCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; form-action 'self'; frame-ancestors 'self'; base-uri 'none'; object-src 'none'"

// htmlHeaderFault names the first header of section 4.4 an HTML answer lacks
// or carries wrongly, and is empty when it carries every one. A redirect
// carries every header but Content-Type, because it has no body.
func htmlHeaderFault(got reply, redirect bool) string {
	want := map[string]string{
		"Cache-Control":           "no-store",
		"X-Content-Type-Options":  "nosniff",
		"Vary":                    "Accept",
		"Referrer-Policy":         "same-origin",
		"Content-Security-Policy": wantedCSP,
		"X-Frame-Options":         "SAMEORIGIN",
		"Content-Type":            "text/html; charset=utf-8",
	}
	if redirect {
		delete(want, "Content-Type")
		if got.header.Get("Content-Type") != "" {
			return "a redirect carries Content-Type " + got.header.Get("Content-Type")
		}
	}
	names := make([]string, 0, len(want))
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if values := got.header.Values(name); len(values) != 1 || values[0] != want[name] {
			return fmt.Sprintf("%s is %q, wanted %q", name, values, want[name])
		}
	}
	return ""
}

// frameAncestors is the frame-ancestors directive of a policy, and empty when
// it names none.
func frameAncestors(policy string) string {
	for _, directive := range strings.Split(policy, ";") {
		fields := strings.Fields(directive)
		if len(fields) > 0 && fields[0] == "frame-ancestors" {
			return strings.Join(fields[1:], " ")
		}
	}
	return ""
}

// pageRow is one GET row of the route table, filled in for the fixture.
type pageRow struct {
	pattern, path string
}

// pageFixture is the fixture every sweep of the pages reads: a card at Build
// carrying a comment, a column comment, a workstream, and the rows of every
// GET route of the table, generated from it.
type pageFixture struct {
	*fixture
	card string
	rows []pageRow
}

// newPageFixture builds the fixture and generates the GET rows. The assets
// route is not a row here: it answers the file it names, whatever Accept
// says, and TestTheAssetsRouteAnswersTheFixedTable reads it.
func newPageFixture(t *testing.T, options ...func(*Config)) *pageFixture {
	t.Helper()
	f := newFixture(t, append([]func(*Config){func(cfg *Config) { cfg.ParseLine = stubParser }}, options...)...)
	card := f.add("A card", "build")
	if response := f.library().Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: card, Text: "A note."}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %+v", response)
	}
	if response := f.library().NewWorkstream(&verb.Request{Verb: "workstream", Actor: "alka", Action: "new", Workstream: "Stream", Slug: "stream"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("workstream: %+v", response)
	}
	p := &pageFixture{fixture: f, card: card}
	for _, r := range routes {
		if r.read == nil || r.ignoresAccept {
			continue
		}
		path := strings.TrimSuffix(r.pattern, "{$}")
		for from, to := range map[string]string{"{card}": card, "{column}": "build", "{slug}": "stream", "{view}": "mine", "{rest...}": "comments/1"} {
			path = strings.ReplaceAll(path, from, to)
		}
		if r.pattern == "/search" {
			path += "?phrase=card"
		}
		p.rows = append(p.rows, pageRow{pattern: r.pattern, path: path})
	}
	return p
}

// getRowCount is how many routes answer GET, less the assets route.
func getRowCount() int {
	count := 0
	for _, r := range routes {
		if r.read != nil && !r.ignoresAccept {
			count++
		}
	}
	return count
}

// TestEveryGetRouteAnswersHTML is dinah-338/criteria/3's sweep: every GET row
// answers a browser with text/html and every header of section 4.4, at the
// status its JSON answer has.
func TestEveryGetRouteAnswersHTML(t *testing.T) {
	f := newPageFixture(t)
	driven := 0
	for _, row := range f.rows {
		page := f.page(row.path)
		plain := f.get(row.path, "Accept", "application/json")
		if row.pattern == "/cards/{card}/window" || row.pattern == "/commands" {
			plain = page
		}
		driven++
		redirect := row.pattern == "/cards/{card}/claim"
		if fault := htmlHeaderFault(page, redirect); fault != "" {
			t.Errorf("GET %s: %s", row.path, fault)
		}
		if page.status != plain.status {
			t.Errorf("GET %s: the page answered %d and the JSON %d", row.path, page.status, plain.status)
		}
		if redirect && page.status != http.StatusSeeOther {
			t.Errorf("GET %s: wanted the 303, got %d", row.path, page.status)
		}
	}
	if driven != getRowCount() || driven == 0 {
		t.Fatalf("drove %d GET rows of %d", driven, getRowCount())
	}
	t.Logf("drove %d GET rows", driven)
}

// TestHTMLChangesNoJSONByte holds every JSON answer of the GET rows to
// answer.Encode of the route's payload, asked with application/json, with
// */* and with no Accept at all.
func TestHTMLChangesNoJSONByte(t *testing.T) {
	f := newPageFixture(t)
	checked := 0
	for _, d := range drives(t, f.card, "build", "stream") {
		if d.method != http.MethodGet {
			continue
		}
		path := d.path
		arguments := map[string]any{}
		for name, value := range d.bound {
			arguments[name] = value
		}
		if d.command == "search" {
			path += "?phrase=card"
			arguments["phrase"] = "card"
		}
		if d.command == "query" {
			path += "?query=state:ready"
			arguments["query"] = "state:ready"
		}
		req := answer.Build(d.command, arguments)
		req.Actor = "alka"
		req.Harness, req.Model = "fixture", "fixture-model"
		want, err := answer.RunSealed(d.command, f.library(), req).Encode()
		if err != nil {
			t.Fatalf("encode %s: %v", d.command, err)
		}
		for _, accept := range []string{"application/json", "*/*", ""} {
			var got reply
			if accept == "" {
				got = f.get(path)
			} else {
				got = f.get(path, "Accept", accept)
			}
			checked++
			if d.command == "changes" {
				continue
			}
			if got.body != string(want) {
				t.Errorf("GET %s with Accept %q: the body differs from answer.Encode of %s", path, accept, d.command)
			}
			if got.header.Get("Content-Security-Policy") != "" || got.header.Get("X-Frame-Options") != "" || got.header.Get("Referrer-Policy") != "" {
				t.Errorf("GET %s with Accept %q: a JSON answer carries a page's headers", path, accept)
			}
		}
	}
	if checked == 0 {
		t.Fatal("compared nothing")
	}
	t.Logf("compared %d JSON answers", checked)
}

// TestPageParametersExistOnlyForHTML holds the page parameters to the pages:
// a page takes them, a JSON request is refused for them as for any
// unpublished parameter, and a page refuses one given twice.
func TestPageParametersExistOnlyForHTML(t *testing.T) {
	f := newPageFixture(t)
	if got := f.page("/?open=" + f.card); got.status != http.StatusOK {
		t.Errorf("a page with open: wanted 200, got %d", got.status)
	}
	for _, name := range []string{"open", "top", "min", "p." + f.card} {
		got := f.get("/?"+name+"="+f.card, "Accept", "application/json")
		if got.status != http.StatusBadRequest || got.refusal(t) != contract.Usage || got.detail(t) != name {
			t.Errorf("JSON with %s: wanted 400 naming it, got %d %s", name, got.status, got.body)
		}
	}
	if got := f.page("/?open=a&open=b"); got.status != http.StatusBadRequest || !strings.Contains(got.body, "dinah.usage") {
		t.Errorf("a page with open twice: wanted 400, got %d", got.status)
	}
}

// TestPagesNeverSetNoReferrer holds every answer of the sweep to a referrer
// policy that keeps Origin on a form post.
func TestPagesNeverSetNoReferrer(t *testing.T) {
	f := newPageFixture(t)
	checked := 0
	for _, row := range f.rows {
		for _, got := range []reply{f.page(row.path), f.get(row.path, "Accept", "application/json")} {
			checked++
			if strings.Contains(strings.ToLower(got.header.Get("Referrer-Policy")), "no-referrer") {
				t.Errorf("GET %s carries Referrer-Policy %q", row.path, got.header.Get("Referrer-Policy"))
			}
			if strings.HasPrefix(got.header.Get("Content-Type"), "text/html") && got.header.Get("Referrer-Policy") != "same-origin" {
				t.Errorf("GET %s is HTML and carries Referrer-Policy %q", row.path, got.header.Get("Referrer-Policy"))
			}
		}
	}
	t.Logf("checked %d answers", checked)
}

// TestNoPageCanBeFramedByAnotherSite holds every HTML answer, errors and
// redirects included, to frame-ancestors 'self' and X-Frame-Options, and
// every JSON answer of the same rows to neither.
func TestNoPageCanBeFramedByAnotherSite(t *testing.T) {
	f := newPageFixture(t)
	answers := map[string]reply{}
	for _, row := range f.rows {
		answers["GET "+row.path] = f.page(row.path)
	}
	answers["an unknown path"] = f.page("/nowhere")
	answers["a 405"] = f.send(request{method: http.MethodPut, path: "/cards", header: map[string]string{"Accept": browserAccept}})
	answers["a 403 admission refusal"] = f.pageForm("/cards/"+f.card+"/claim", "", "/", "Origin", "http://evil.example")
	answers["a refused read"] = f.page("/cards/ht-99")
	answers["a 400 from POST /commands"] = f.pageForm("/commands", "line=a&entry=1", "/")
	answers["the 303 of a form claim"] = f.pageForm("/cards/"+f.card+"/claim", "_basis=*", "/")
	answers["the 303 of a typed line"] = f.pageForm("/commands", "line=nothing", "/")
	checked := 0
	for name, got := range answers {
		checked++
		if frameAncestors(got.header.Get("Content-Security-Policy")) != "'self'" || got.header.Get("X-Frame-Options") != "SAMEORIGIN" {
			t.Errorf("%s (%d): Content-Security-Policy %q, X-Frame-Options %q", name, got.status, got.header.Get("Content-Security-Policy"), got.header.Get("X-Frame-Options"))
		}
	}
	for _, want := range []struct {
		name   string
		status int
	}{{"an unknown path", 404}, {"a 405", 405}, {"a 403 admission refusal", 403}, {"a refused read", 404}, {"a 400 from POST /commands", 400}, {"the 303 of a form claim", 303}, {"the 303 of a typed line", 303}} {
		if answers[want.name].status != want.status {
			t.Errorf("%s: wanted %d, got %d", want.name, want.status, answers[want.name].status)
		}
	}
	jsonRows := 0
	for _, row := range f.rows {
		if row.pattern == "/cards/{card}/window" || row.pattern == "/commands" {
			continue
		}
		got := f.get(row.path, "Accept", "application/json")
		jsonRows++
		if got.header.Get("Content-Security-Policy") != "" || got.header.Get("X-Frame-Options") != "" {
			t.Errorf("GET %s as JSON carries a framing header", row.path)
		}
	}
	if len(f.rows) != getRowCount() {
		t.Fatalf("the sweep drove %d GET rows of %d", len(f.rows), getRowCount())
	}
	t.Logf("checked %d HTML answers and %d JSON answers", checked, jsonRows)
}

// scriptFault names the first way a page needs script, and is empty when it
// needs none.
func scriptFault(root *node) string {
	for _, n := range root.all(func(*node) bool { return true }) {
		if n.name == "script" && strings.TrimSpace(n.allText()) != "" {
			return "a script element with content"
		}
		for _, name := range n.order {
			value := n.attr[name]
			if strings.HasPrefix(strings.ToLower(name), "on") {
				return "an attribute " + name
			}
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "javascript:") {
				return "a javascript: value"
			}
			if name == "data-theme-toggle" {
				return "an element carrying data-theme-toggle"
			}
			if name == "href" || name == "src" || name == "action" {
				if strings.HasPrefix(value, "//") || !strings.HasPrefix(value, "/") {
					return name + " " + value + " is not a path beginning with one slash"
				}
			}
		}
		if n.name == "button" && !n.inside("form") {
			return "a button outside a form"
		}
		if n.name == "form" {
			method := strings.ToLower(n.attr["method"])
			if method != "post" && method != "get" {
				return "a form whose method is " + method
			}
			if action := n.attr["action"]; !strings.HasPrefix(action, "/") || strings.HasPrefix(action, "//") {
				return "a form whose action is " + action
			}
		}
	}
	return ""
}

// TestNoPageNeedsScript holds every page of the sweep and every window to
// working without script, with no exception.
func TestNoPageNeedsScript(t *testing.T) {
	f := newPageFixture(t)
	documents, forms := 0, 0
	check := func(name, body string) {
		root := parseHTML(t, body)
		documents++
		forms += len(root.named("form"))
		if fault := scriptFault(root); fault != "" {
			t.Errorf("%s: %s", name, fault)
		}
	}
	for _, row := range f.rows {
		if row.pattern == "/cards/{card}/claim" {
			continue
		}
		got := f.page(row.path)
		check("GET "+row.path, got.body)
	}
	check("a page with a window", f.page("/?open="+f.card).body)
	check("an error page", f.page("/cards/ht-99").body)
	if documents < getRowCount() || forms == 0 {
		t.Fatalf("checked %d documents and %d forms", documents, forms)
	}
	t.Logf("checked %d documents and %d forms", documents, forms)
}

// TestEveryPageNamesItsPane holds every page to the pane PUDL shows on a
// narrow layout, and every detail page to its back link.
func TestEveryPageNamesItsPane(t *testing.T) {
	f := newPageFixture(t)
	backs := map[string]string{
		"/columns/build":                       "/#col-build",
		"/cards/" + f.card + "?open=" + f.card: "/columns/build?open=" + f.card + "&top=" + f.card + "&p." + f.card + "=floating:0.06,0.05,0.55,0.75#card-" + f.card,
		"/cards":                               "/",
	}
	paths := []string{"/?open=" + f.card, "/cards/" + f.card + "?open=" + f.card}
	for _, row := range f.rows {
		if row.pattern != "/cards/{card}/claim" && row.pattern != "/cards/{card}/window" {
			paths = append(paths, row.path)
		}
	}
	for _, path := range paths {
		root := parseHTML(t, f.page(path).body)
		layout := root.first(withClass("md-layout"))
		if layout == nil {
			t.Errorf("%s draws no .md-layout", path)
			continue
		}
		pane := layout.attr["data-md-pane"]
		detail := root.first(withClass("md-detail"))
		back := detail.first(withClass("md-back"))
		isBoard := strings.HasPrefix(path, "/?") || path == "/"
		switch {
		case isBoard && (pane != "list" || back != nil):
			t.Errorf("%s: wanted the list pane and no back link, got %q", path, pane)
		case !isBoard && pane != "detail":
			t.Errorf("%s: wanted the detail pane, got %q", path, pane)
		case !isBoard && (back == nil || len(detail.kids) == 0 || detail.kids[0] != back):
			t.Errorf("%s: the detail pane does not begin with its back link", path)
		}
		if want, named := backs[path]; named && (back == nil || back.attr["href"] != want) {
			got := ""
			if back != nil {
				got = back.attr["href"]
			}
			t.Errorf("%s: the back link is %q, wanted %q", path, got, want)
		}
		ids := map[string]bool{}
		for _, n := range root.all(func(n *node) bool { return n.has("id") }) {
			if ids[n.attr["id"]] {
				t.Errorf("%s: the id %s is written twice", path, n.attr["id"])
			}
			ids[n.attr["id"]] = true
		}
		for _, row := range root.all(withClass("md-row")) {
			if !strings.HasPrefix(row.attr["id"], "col-") {
				t.Errorf("%s: a sidebar row carries the id %q", path, row.attr["id"])
			}
		}
	}
	t.Logf("checked %d pages", len(paths))
}

// TestWindowsAreDrawnFromTheURL holds the server's windows to the URL.
func TestWindowsAreDrawnFromTheURL(t *testing.T) {
	f := newPageFixture(t)
	a := f.card
	b := f.add("Another card", "review")
	query := "open=" + a + "," + b + ",ht-99&top=" + a + "&min=" + b + "&p." + a + "=left:0.1,0.1,0.5,0.5"
	root := parseHTML(t, f.page("/?"+query).body)
	wins := root.all(withClass("win"))
	if len(wins) != 2 {
		t.Fatalf("wanted two windows, since ht-99 names no card, got %d", len(wins))
	}
	top, other := wins[1], wins[0]
	if top.attr["data-win"] != a || top.attr["data-win-mode"] != "left" || !strings.Contains(top.attr["class"], "active") ||
		top.attr["style"] != "--win-x:0.1; --win-y:0.1; --win-w:0.5; --win-h:0.5" || top.has("hidden") {
		t.Errorf("the top window is drawn %v", top.attr)
	}
	if other.attr["data-win"] != b || !other.has("hidden") || strings.Contains(other.attr["class"], "active") {
		t.Errorf("the minimised window is drawn %v", other.attr)
	}
	q, _ := url.ParseQuery(query)
	state := pages.ParseWindows(q).Without(func(key string) bool { return key == "ht-99" })
	for _, want := range []struct {
		action string
		href   string
	}{
		{"minimize", pages.URLFor("/", nil, state.Minimized(a))},
		{"maximize", pages.URLFor("/", nil, state.MaximizeToggled(a))},
		{"close", pages.URLFor("/", nil, state.Closed(a))},
		{"page", "/cards/" + a},
	} {
		button := top.first(func(n *node) bool { return n.attr["data-win-action"] == want.action })
		if button == nil || button.attr["href"] != want.href {
			t.Errorf("the %s button: wanted %q, got %v", want.action, want.href, button)
		}
	}
	tabs := root.all(func(n *node) bool { return n.has("data-win-tab") })
	if len(tabs) != 2 || tabs[0].attr["href"] != pages.URLFor("/", nil, state.Tabbed(a)) || tabs[0].attr["aria-current"] != "true" {
		t.Errorf("the dock draws %d tabs: %v", len(tabs), tabs)
	}
}

// TestTheWindowRoute holds the window route to the markup alone.
func TestTheWindowRoute(t *testing.T) {
	f := newPageFixture(t)
	got := f.page("/cards/" + f.card + "/window")
	if got.status != http.StatusOK || strings.Contains(got.body, "<html") {
		t.Fatalf("wanted 200 and the window alone, got %d", got.status)
	}
	root := parseHTML(t, got.body)
	win := root.first(withClass("win"))
	if win == nil || win.attr["data-win"] != f.card || win.has("style") || strings.Contains(win.attr["class"], "active") {
		t.Errorf("the window is drawn %v", win)
	}
	for _, action := range []string{"minimize", "maximize", "close"} {
		button := win.first(func(n *node) bool { return n.attr["data-win-action"] == action })
		if button == nil || button.attr["href"] != "/cards/"+f.card {
			t.Errorf("the %s button of a fetched window should lead to the card's page, got %v", action, button)
		}
	}
	if got := f.page("/cards/ht-99/window"); got.status != http.StatusNotFound {
		t.Errorf("an unknown card's window: wanted 404, got %d", got.status)
	}
	if got := f.get("/cards/"+f.card+"/window", "Accept", "application/json"); got.status != http.StatusNotAcceptable {
		t.Errorf("a JSON-only Accept: wanted 406, got %d", got.status)
	}
	if got := f.page("/cards/" + f.card + "/window?open=x"); got.status != http.StatusBadRequest {
		t.Errorf("a window with a query: wanted 400, got %d", got.status)
	}
}

// formsOf lists a window's forms as "action members", in order.
func formsOf(win *node) []*node {
	return win.named("form")
}

// TestFormsAreTheAffordances holds a card's forms to its affordances at the
// six positions dinah-152's affordance test builds.
func TestFormsAreTheAffordances(t *testing.T) {
	f := newPageFixture(t)
	atBuild := f.add("Ready where a claim takes it up", "build")
	atIntake := f.add("Ready where a pull carries it on", "intake")
	atWaiting := f.add("Ready where nothing takes it up", "waiting")
	active := f.add("Active", "build")
	f.act(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: active})
	blocked := f.add("Blocked", "build")
	f.act(&verb.Request{Verb: verb.Block, Actor: "alka", Card: blocked, Reason: "An obstacle."})
	finished := f.add("Finished", "build")
	f.act(&verb.Request{Verb: verb.Move, Actor: "alka", Card: finished, Column: "finished", Override: true})
	rows := map[string]answer.AffordanceRow{}
	for _, row := range affordanceDocument(t, f.fixture) {
		rows[row.Affordance] = row
	}
	for _, card := range []string{atBuild, atIntake, atWaiting, active, blocked, finished} {
		show := f.get("/cards/" + card).object(t)
		var names []string
		for _, name := range show["affordances"].([]any) {
			names = append(names, name.(string))
		}
		var served struct {
			Served struct {
				LegalMoves []struct {
					Ref string `json:"ref"`
				} `json:"legal_moves"`
			} `json:"served"`
		}
		json.Unmarshal([]byte(f.get("/cards/"+card+"/instructions").body), &served)
		revision := show["detail"].(map[string]any)["card"].(map[string]any)["revision"].(string)

		win := parseHTML(t, f.page("/cards/"+card+"/window").body)
		var wanted []string
		for _, name := range names {
			row, ok := rows[name]
			if !ok || row.Form == nil {
				continue
			}
			if name == "move" && len(served.Served.LegalMoves) == 0 {
				continue
			}
			wanted = append(wanted, name)
		}
		forms := formsOf(win)
		if len(forms) != len(wanted) {
			t.Errorf("%s: %d forms for the form rows %v", card, len(forms), wanted)
			continue
		}
		signature := func(action, method, kind string) string { return action + "|" + method + "|" + kind }
		byForm := map[string]string{}
		for _, name := range wanted {
			row := rows[name]
			action := strings.ReplaceAll(strings.ReplaceAll(row.Form.Href, "{card}", card), "{path}", "/cards/"+card)
			byForm[signature(action, row.Form.Members["_method"], row.Form.Members["_type"])] = name
		}
		primary := 0
		for _, form := range forms {
			members := map[string]string{}
			for _, input := range form.all(func(n *node) bool { return n.name == "input" && n.attr["type"] == "hidden" }) {
				members[input.attr["name"]] = input.attr["value"]
			}
			name, known := byForm[signature(form.attr["action"], members["_method"], members["_type"])]
			if !known {
				t.Errorf("%s: a form posts to %q with %v, which no form row of its affordances names", card, form.attr["action"], members)
				continue
			}
			delete(byForm, signature(form.attr["action"], members["_method"], members["_type"]))
			_, carriesBasis := members["_basis"]
			if name == "comment" && carriesBasis {
				t.Errorf("%s: the comment form carries _basis", card)
			}
			if name != "comment" && members["_basis"] != revision {
				t.Errorf("%s %s: _basis is %q, wanted the revision %q", card, name, members["_basis"], revision)
			}
			if name == "move" {
				var options []string
				for _, option := range form.named("option") {
					options = append(options, option.attr["value"])
				}
				var moves []string
				for _, move := range served.Served.LegalMoves {
					moves = append(moves, move.Ref)
				}
				sort.Strings(options)
				sort.Strings(moves)
				if strings.Join(options, ",") != strings.Join(moves, ",") {
					t.Errorf("%s: the move options %v are not the legal moves %v", card, options, moves)
				}
			}
			primary += len(form.all(withClass("btn-primary")))
		}
		if len(byForm) > 0 {
			t.Errorf("%s: no form was drawn for %v", card, byForm)
		}
		if primary > 1 {
			t.Errorf("%s: %d primary buttons", card, primary)
		}
		if card == atBuild || card == atIntake || card == active || card == blocked {
			if primary != 1 {
				t.Errorf("%s: wanted exactly one primary button, got %d", card, primary)
			}
		}
	}
	c := &pages.Context{R: msg.For(""), Lang: "en", Path: "/"}
	c.Affordances, _ = answer.EncodeAffordanceTable(affordanceRows())
	planted := []byte(`{"detail": {"card": {"ref": "ht-1", "title": "T", "revision": "r"}, "body": ""}, "affordances": ["claim", "no-such-act"]}`)
	body, err := pages.Window(c, pages.WindowCard{Key: "ht-1", Show: planted})
	if err != nil {
		t.Fatalf("a planted affordance broke the render: %v", err)
	}
	if forms := formsOf(parseHTML(t, string(body))); len(forms) != 1 || !strings.HasSuffix(forms[0].attr["action"], "/claim") {
		t.Errorf("a planted affordance: wanted the claim form alone, got %d forms", len(forms))
	}
}

// TestActsFromAPageRedirectBack holds every act posted as a form to 303, to
// the page it came from, whatever the outcome.
func TestActsFromAPageRedirectBack(t *testing.T) {
	f := newPageFixture(t)
	card := f.card
	from := "/columns/build?open=" + card
	for _, act := range []struct {
		name, path, body string
	}{
		{"claim", "/cards/" + card + "/claim", "_basis=*"},
		{"block", "/cards/" + card, "_method=PATCH&_type=application/vnd.dinah.block%2Bjson&_basis=*&reason=R&kind="},
		{"unblock", "/cards/" + card, "_method=PATCH&_type=application/vnd.dinah.unblock%2Bjson&_basis=*&reason="},
		{"move", "/cards/" + card, "_method=PATCH&_type=application/vnd.dinah.move%2Bjson&_basis=*&column=review"},
		{"release", "/cards/" + card + "/claim", "_method=DELETE&_basis=*"},
		{"comment", "/cards/" + card + "/comments", "text=Hello"},
	} {
		got := f.pageForm(act.path, act.body, from)
		if got.status != http.StatusSeeOther || got.header.Get("Location") != from {
			t.Errorf("%s: wanted 303 to %s, got %d %q %s", act.name, from, got.status, got.header.Get("Location"), got.body)
		}
	}
	added := f.pageForm("/cards", "title=Added&column=&severity=&priority=&route=", "/cards")
	location := added.header.Get("Location")
	if added.status != http.StatusSeeOther || !strings.HasPrefix(location, "/cards?open=") || !strings.Contains(location, "&top=") {
		t.Errorf("add: wanted 303 opening the card as the top window on /cards, got %d %q %s", added.status, location, added.body)
	}
	pulled := f.pageForm("/claims", "_basis=*&column=build", "/")
	top := ""
	if location, err := url.Parse(pulled.header.Get("Location")); err == nil {
		top = location.Query().Get("top")
	}
	if pulled.status != http.StatusSeeOther || top == "" || f.fixture.card(top).Holder != "alka" || f.fixture.card(top).ColumnTitle != "Build" {
		t.Errorf("pull: wanted 303 opening the pulled card as the top window, got %d %q %s", pulled.status, pulled.header.Get("Location"), pulled.body)
	}
	refused := f.pageForm("/cards/"+card+"/claim", "_basis=*", "/", "Referer", "http://evil.example/x")
	if refused.status != http.StatusSeeOther || refused.header.Get("Location") != "/cards/"+card {
		t.Errorf("a foreign Referer: wanted 303 to the card's page, got %d %q", refused.status, refused.header.Get("Location"))
	}
	stale := f.pageForm("/cards/"+card+"/claim", "_basis=sha256:"+strings.Repeat("0", 64), from)
	if stale.status != http.StatusSeeOther || stale.header.Get("Location") != from {
		t.Errorf("a stale act: wanted 303 back, got %d %q", stale.status, stale.header.Get("Location"))
	}
	missing := f.pageForm("/cards/"+card+"/claim", "_basis=*", "", "Referer", "-")
	if missing.status != http.StatusSeeOther || missing.header.Get("Location") != "/cards/"+card {
		t.Errorf("no Referer: wanted 303 to the card's page, got %d %q", missing.status, missing.header.Get("Location"))
	}
}

// TestReturnPathStaysOnThisServer holds every Location the pages write to
// localPath, asserted byte for byte on the escaped string.
func TestReturnPathStaysOnThisServer(t *testing.T) {
	f := newPageFixture(t)
	h := &head{cfg: f.cfg}
	base := "http://127.0.0.1:" + strconv.Itoa(f.port)
	for _, row := range []struct {
		referer, want string
	}{
		{base + "//evil.example/x", "/fallback"},
		{"//evil.example/", "/fallback"},
		{"http://evil.example/cards/x", "/fallback"},
		{base + "/a\x01b", "/fallback"},
		{base + "/a\tb", "/fallback"},
		{base + `/\evil.example/x`, "/%5Cevil.example/x"},
		{base + "/%2Fevil.example/", "/%2Fevil.example/"},
		{base + "/.//evil", "/.//evil"},
		{base + "/／evil", "/%EF%BC%8Fevil"},
		{base + "/", "/"},
		{"http://localhost:" + strconv.Itoa(f.port) + "/cards/ht-1?open=ht-1", "/cards/ht-1?open=ht-1"},
		{base + "/%41b", "/%41b"},
		{base + "/cards#frag", "/cards"},
	} {
		r := &http.Request{Header: http.Header{"Referer": {row.referer}}}
		if got := h.returnPath(r, "/fallback"); got != row.want {
			t.Errorf("Referer %q: returned %q, wanted %q", row.referer, got, row.want)
		}
		if !localPath(h.returnPath(r, "/fallback")) {
			t.Errorf("Referer %q: the Location fails localPath", row.referer)
		}
	}
	for _, s := range []string{"//x", `/\x`, "", "x", "/a\x7f", "/a\nb"} {
		if localPath(s) {
			t.Errorf("localPath(%q) holds", s)
		}
	}
	for _, s := range []string{"/", "/a", "/%2F", "/.//x"} {
		if !localPath(s) {
			t.Errorf("localPath(%q) fails", s)
		}
	}
	for _, referer := range []string{base + "//evil.example/x", base + `/\evil.example/x`} {
		got := f.pageForm("/cards/"+f.card+"/claim", "_basis=*", "", "Referer", referer)
		if location := got.header.Get("Location"); strings.HasPrefix(location, "//") || strings.HasPrefix(location, `/\`) {
			t.Errorf("Referer %q: the handler wrote Location %q", referer, location)
		}
	}
	x := &exchange{r: &http.Request{Header: http.Header{}}, page: &pageState{}}
	h2 := Handler(f.cfg).(*head)
	reads := 0
	for _, command := range routedCommands() {
		if actCommands()[command] {
			continue
		}
		for _, arguments := range []map[string]any{
			{"card": "//evil.example", "ref": "//evil.example", "view": "//evil.example", "query": "a b\tc", "phrase": "x y"},
			{"card": f.card, "ref": "cards", "query": "state:ready"},
		} {
			location, served := h2.readURL(x, command, arguments)
			if !served {
				continue
			}
			reads++
			if !localPath(location) {
				t.Errorf("readURL(%s, %v) = %q, which fails localPath", command, arguments, location)
			}
		}
	}
	if reads == 0 {
		t.Fatal("readURL composed nothing")
	}
	t.Logf("checked %d read URLs", reads)
}

// logEntries reads the command log page's entries as their code lines.
func logEntries(t *testing.T, f *fixture) []*node {
	t.Helper()
	root := parseHTML(t, f.page("/commands").body)
	return root.first(withClass("md-detail")).all(withClass("cmdlog-entry"))
}

// TestTheLogRecordsWhatThePagesDid holds the log to every act a page posted,
// as its derived line, and to nothing a JSON client did.
func TestTheLogRecordsWhatThePagesDid(t *testing.T) {
	f := newPageFixture(t)
	card := f.card
	f.pageForm("/cards/"+card+"/claim", "_basis=*", "/")
	f.pageForm("/cards/"+card+"/claim", "_method=DELETE&_basis=*&_actor=bryn", "/")
	f.pageForm("/cards/"+card+"/claim", "_basis=sha256:"+strings.Repeat("0", 64), "/")
	entries := logEntries(t, f.fixture)
	if len(entries) != 3 {
		t.Fatalf("wanted three entries, got %d", len(entries))
	}
	lines := func(i int) string { return entries[i].first(withClass("cmdlog-cmd")).allText() }
	if got := lines(2); got != "dinah claim "+card {
		t.Errorf("the claim is logged %q", got)
	}
	if got := lines(1); got != "dinah release "+card+" --actor bryn" {
		t.Errorf("the release by bryn is logged %q", got)
	}
	en := msg.For("en")
	refused := contract.NotHolder + " " + en.T("refusal.not-holder", "detail", "alka") + en.T("refusal.not-holder.next", "detail", "alka")
	if why := entries[1].first(withClass("cmdlog-why")); entries[1].attr["data-outcome"] != "refused" || why == nil || why.allText() != refused {
		t.Errorf("a refused release is logged %q, wanted %q", entries[1].allText(), refused)
	}
	if entries[0].attr["data-outcome"] != "stale" || !strings.Contains(entries[0].allText(), card) {
		t.Errorf("a stale claim is logged without its sentence: %s", entries[0].allText())
	}
	f.json(http.MethodPost, "/cards/"+card+"/comments", typeJSON, `{"text": "JSON"}`)
	if got := len(logEntries(t, f.fixture)); got != 3 {
		t.Errorf("an act sent as JSON was logged: %d entries", got)
	}
	log := &commandLog{}
	for i := 0; i < logCapacity+1; i++ {
		log.record(LogEntry{Verb: "claim"})
	}
	if _, held := log.find(1); held || len(log.snapshot()) != logCapacity {
		t.Errorf("the ring holds %d entries and still holds the first: %v", len(log.snapshot()), held)
	}
}

// TestADefectAnswerEscapesItsMessage holds the 500 a page request meets when
// no page can be drawn to text/html that carries the fault's message as text,
// escaped, rather than as markup.
func TestADefectAnswerEscapesItsMessage(t *testing.T) {
	message := `template: <script>alert("x")</script> & more`
	recorder := httptest.NewRecorder()
	x := &exchange{w: recorder, r: &http.Request{Header: http.Header{}}, served: typeHTML}
	(&head{}).defect(x, errors.New(message))
	if recorder.Code != http.StatusInternalServerError || recorder.Body.String() != html.EscapeString(message) {
		t.Errorf("a defect answered %d %q, wanted 500 %q", recorder.Code, recorder.Body.String(), html.EscapeString(message))
	}
}

// TestARefusedEntryNamesItsCard holds a refused entry's next step to the
// card the answer carried, on the one hint naming the card that no page can
// reach, raise's, since the HTTP head has no route for raise. The other four
// are compared with the terminal's own sentence by
// TestARefusalReadsTheSameOnThePagesAsAtTheTerminal in cmd/dinah.
func TestARefusedEntryNamesItsCard(t *testing.T) {
	en := msg.For("en")
	log := &commandLog{}
	log.record(LogEntry{Verb: "raise", Outcome: contract.OutcomeRefused, Refusal: contract.NoReason, Card: "fx-3"})
	drawn := log.pageEntries(en, logCapacity)
	want := contract.NoReason + " " + en.T("refusal.no-reason.raise") + en.T("refusal.no-reason.raise.next", "card", "fx-3")
	if len(drawn) != 1 || drawn[0].Sentence != want {
		t.Errorf("a refused raise is drawn %+v, wanted the sentence %q", drawn, want)
	}
}

// TestEveryPageCarriesACursorChangesAccepts holds a page's cursor to the
// changes route.
func TestEveryPageCarriesACursorChangesAccepts(t *testing.T) {
	f := newPageFixture(t)
	cursor := parseHTML(t, f.page("/").body).first(withClass("md-layout")).attr["data-changes-cursor"]
	if cursor == "" {
		t.Fatal("the board carries no cursor")
	}
	ask := func() bool {
		got := f.get("/changes?since="+url.QueryEscape(cursor), "Accept", "application/json")
		if got.status != http.StatusOK {
			t.Fatalf("changes answered %d %s", got.status, got.body)
		}
		changed, _ := got.object(t)["changed"].(bool)
		return changed
	}
	if ask() {
		t.Error("nothing moved and changes says something changed")
	}
	f.act(&verb.Request{Verb: verb.Move, Actor: "alka", Card: f.card, Column: "review"})
	if !ask() {
		t.Error("a card moved and changes says nothing changed")
	}
}

// TestTheAssetsRouteAnswersTheFixedTable holds the assets route to its table.
func TestTheAssetsRouteAnswersTheFixedTable(t *testing.T) {
	f := newPageFixture(t)
	for path, contentType := range map[string]string{
		"/assets/pudl/pudl.css": "text/css; charset=utf-8", "/assets/pudl/pudl-windows.css": "text/css; charset=utf-8", "/assets/dinah.css": "text/css; charset=utf-8",
		"/assets/pudl/pudl-theme.js": "text/javascript; charset=utf-8", "/assets/pudl/pudl-windows.js": "text/javascript; charset=utf-8", "/assets/dinah.js": "text/javascript; charset=utf-8",
		"/assets/dinah-lantern.svg": "image/svg+xml",
	} {
		got := f.get(path, "Accept", "text/css,*/*;q=0.1")
		if got.status != http.StatusOK || got.header.Get("Content-Type") != contentType || got.header.Get("Cache-Control") != "no-store" || got.header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: %d %q", path, got.status, got.header.Get("Content-Type"))
		}
	}
	// A path carrying .. never reaches the route: the head answers any path
	// that is not in its clean form 404 before routing, in whatever
	// representation the request negotiates.
	if got := f.page("/assets/../go.mod"); got.status != http.StatusNotFound {
		t.Errorf("/assets/../go.mod: wanted 404, got %d", got.status)
	}
	for _, path := range []string{"/assets/pudl/PROVENANCE", "/assets/pudl/LICENSE", "/assets/nothing.css", "/assets/pudl/fonts/InterVariable.woff2"} {
		got := f.page(path)
		if got.status != http.StatusNotFound || !strings.HasPrefix(got.header.Get("Content-Type"), "application/vnd.dinah+json") {
			t.Errorf("%s: wanted 404 in the vendor type, got %d %q", path, got.status, got.header.Get("Content-Type"))
		}
	}
	logo, _ := os.ReadFile(filepath.Join("..", "..", "logo", "dinah-lantern.svg"))
	if got := f.get("/assets/dinah-lantern.svg"); got.body != string(logo) {
		t.Error("the served mark differs from the logo")
	}
}
