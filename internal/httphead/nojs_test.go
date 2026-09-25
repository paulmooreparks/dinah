package httphead

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"dinah/internal/verb"
)

// scriptless is a client with no script engine. It sends a browser's
// navigation Accept, follows only links and submits only forms, and on a form
// post sends the headers the Fetch standard has a browser send for a
// same-origin submission: the page's Origin, Sec-Fetch-Site: same-origin, and
// the page as the Referer, which a Referrer-Policy of same-origin allows.
type scriptless struct {
	t *testing.T
	f *fixture
	// at is the path and query of the page the client is on.
	at   string
	root *node
	// submitted counts the forms the client posted.
	submitted int
}

// open navigates to a path.
func (s *scriptless) open(path string) {
	s.t.Helper()
	got := s.f.page(path)
	if got.status != http.StatusOK {
		s.t.Fatalf("GET %s answered %d %s", path, got.status, got.body)
	}
	s.at, s.root = path, parseHTML(s.t, got.body)
}

// follow clicks the first link match accepts.
func (s *scriptless) follow(match func(*node) bool) {
	s.t.Helper()
	link := s.root.first(func(n *node) bool { return n.name == "a" && match(n) })
	if link == nil {
		s.t.Fatalf("no link on %s matches", s.at)
	}
	s.open(link.attr["href"])
}

// submit posts the first form match accepts, filling the named controls from
// fill and every other control with what the page drew in it, and follows the
// 303 the post answers with. It returns where the 303 led.
func (s *scriptless) submit(within *node, match func(*node) bool, fill map[string]string) string {
	s.t.Helper()
	if within == nil {
		within = s.root
	}
	form := within.first(func(n *node) bool { return n.name == "form" && match(n) })
	if form == nil {
		s.t.Fatalf("no form on %s matches", s.at)
	}
	values := url.Values{}
	for _, control := range form.all(func(n *node) bool { return n.name == "input" || n.name == "select" || n.name == "textarea" }) {
		name := control.attr["name"]
		if name == "" {
			continue
		}
		value, filled := fill[name]
		if !filled {
			switch control.name {
			case "select":
				if option := control.first(func(n *node) bool { return n.name == "option" }); option != nil {
					value = option.attr["value"]
				}
			case "textarea":
				value = control.allText()
			default:
				value = control.attr["value"]
			}
		}
		values.Set(name, value)
	}
	method := strings.ToUpper(form.attr["method"])
	if method != http.MethodPost {
		s.t.Fatalf("the form posts with %q", method)
	}
	got := s.f.send(request{method: method, path: form.attr["action"], body: values.Encode(), header: map[string]string{
		"Content-Type":   typeForm,
		"Origin":         s.f.origin(),
		"Sec-Fetch-Site": "same-origin",
		"Referer":        s.f.url + s.at,
		"Accept":         browserAccept,
	}})
	s.submitted++
	if got.status != http.StatusSeeOther {
		s.t.Fatalf("POST %s answered %d %s", form.attr["action"], got.status, got.body)
	}
	location := got.header.Get("Location")
	s.open(location)
	return location
}

// hidden reports whether a form carries a hidden member of a name and value.
func hidden(name, value string) func(*node) bool {
	return func(form *node) bool {
		return form.first(func(n *node) bool {
			return n.name == "input" && n.attr["type"] == "hidden" && n.attr["name"] == name && n.attr["value"] == value
		}) != nil
	}
}

// TestAScriptlessReaderClaimsMovesAndComments walks the board to a card's
// page and claims, moves and comments on it with forms alone.
func TestAScriptlessReaderClaimsMovesAndComments(t *testing.T) {
	f := newFixture(t, func(cfg *Config) { cfg.ParseLine = stubParser })
	card := f.add("A card", "build")
	s := &scriptless{t: t, f: f}
	s.open("/")
	s.follow(func(n *node) bool { return n.attr["data-win-open"] == card })
	s.submit(nil, func(n *node) bool {
		return strings.HasSuffix(n.attr["action"], "/claim") && !hidden("_method", "DELETE")(n)
	}, nil)
	// The move form's first option is Waiting, which a held card may not
	// enter, so the reader chooses Review, as a person reading the refusal
	// in the log would.
	s.submit(nil, hidden("_type", typeMove), map[string]string{"column": "review"})
	s.submit(nil, func(n *node) bool { return strings.HasSuffix(n.attr["action"], "/comments") }, map[string]string{"text": "Written without script."})
	got := f.card(card)
	if got.Holder != "alka" || got.ColumnTitle == "Build" {
		t.Errorf("the card is %+v, wanted it claimed by alka and moved out of Build", got)
	}
	detail, _, _, _, err := f.library().Show(&verb.Request{Verb: "show", Card: card, Fields: "comments"})
	if err != nil || detail == nil || len(detail.Comments) != 1 {
		t.Errorf("wanted one comment on the card, got %v", err)
	}
	entries := logEntries(t, f)
	if len(entries) != 3 {
		t.Errorf("the log holds %d entries, wanted the three acts", len(entries))
	}
	t.Logf("submitted %d forms", s.submitted)
}

// TestAScriptlessReaderWorksInsideAWindow claims a card from the window a URL
// names and lands back on that URL, whose window now offers the release.
func TestAScriptlessReaderWorksInsideAWindow(t *testing.T) {
	f := newFixture(t, func(cfg *Config) { cfg.ParseLine = stubParser })
	card := f.add("A card", "build")
	s := &scriptless{t: t, f: f}
	s.open("/?open=" + card)
	win := s.root.first(withClass("win"))
	back := s.submit(win, func(n *node) bool { return strings.HasSuffix(n.attr["action"], "/claim") }, nil)
	if back != "/?open="+card {
		t.Errorf("the claim returned to %q, wanted /?open=%s", back, card)
	}
	win = s.root.first(withClass("win"))
	if win.first(func(n *node) bool { return n.name == "form" && hidden("_method", "DELETE")(n) }) == nil {
		t.Error("the redrawn window offers no release")
	}
	if f.card(card).Holder != "alka" {
		t.Error("the card was not claimed")
	}
	t.Logf("submitted %d forms", s.submitted)
}
