package httphead

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// lastEvent is the last line of a card's journal.
func lastEvent(t *testing.T, f *fixture, ref string) bench.Event {
	t.Helper()
	events := f.journal(ref)
	if len(events) == 0 {
		t.Fatalf("%s has no journal", ref)
	}
	return events[len(events)-1]
}

// TestARequestActsAsTheActorItNames is dinah-152/criteria/4. It sets the
// three variables a run could otherwise inherit itself, and resolves the
// agent from the environment the way runServe does, so the fallback it
// asserts is the one the process would take.
func TestARequestActsAsTheActorItNames(t *testing.T) {
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_HOME", t.TempDir())
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("DINAH_HARNESS", "")
	t.Setenv("DINAH_PROVIDER", "")
	t.Setenv("DINAH_MODEL", "env-model")
	t.Setenv("DINAH_SERVER", "")
	f := newFixture(t, func(cfg *Config) { cfg.Agent = bench.ResolveAgent() })
	card := f.add("A card", "build")

	got := f.json(http.MethodPost, "/cards/"+card+"/claim", typeJSON, "{}", "Dinah-Actor", "bryn", "Dinah-Model", "header-model")
	if got.status != http.StatusCreated {
		t.Fatalf("claim as bryn: %d %s", got.status, got.body)
	}
	event := lastEvent(t, f, card)
	if event.Actor.Name != "bryn" || event.Actor.Model != "header-model" {
		t.Errorf("wanted the claim journaled under bryn and header-model, got %+v", event.Actor)
	}
	got = f.json(http.MethodDelete, "/cards/"+card+"/claim", "", "", "Dinah-Actor", "bryn")
	if got.status != http.StatusOK {
		t.Fatalf("release as bryn: %d %s", got.status, got.body)
	}

	got = f.json(http.MethodPost, "/cards/"+card+"/claim", typeJSON, "{}")
	if got.status != http.StatusCreated {
		t.Fatalf("claim as the default: %d %s", got.status, got.body)
	}
	event = lastEvent(t, f, card)
	if event.Actor.Name != "alka" || event.Actor.Model != "env-model" {
		t.Errorf("wanted the claim journaled under the process default alka and DINAH_MODEL, got %+v", event.Actor)
	}
	f.act(&verb.Request{Verb: verb.Release, Actor: "alka", Card: card})

	got = f.form("/cards/"+card+"/claim", "_actor=bryn", "Dinah-Actor", "carol")
	if got.status != http.StatusBadRequest || got.detail(t) != formActor {
		t.Errorf("_actor and Dinah-Actor disagreeing: wanted 400 with detail %s, got %d %s", formActor, got.status, got.body)
	}

	nobody := newFixture(t, func(cfg *Config) { cfg.DefaultActor = "" })
	other := nobody.add("Another card", "build")
	got = nobody.json(http.MethodPost, "/cards/"+other+"/claim", typeJSON, "{}")
	if got.status != http.StatusForbidden || got.refusal(t) != contract.NoOwner {
		t.Errorf("no actor anywhere: wanted 403 %s, got %d %s", contract.NoOwner, got.status, got.body)
	}
}

// TestACardCarriesItsRevisionAsItsETag is dinah-152/criteria/8.
func TestACardCarriesItsRevisionAsItsETag(t *testing.T) {
	f := newFixture(t)
	card := f.add("A card", "build")
	f.act(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: card})

	read := f.get("/cards/" + card)
	revision := f.card(card).Revision
	if read.header.Get("ETag") != quoted(revision) {
		t.Fatalf("GET: wanted ETag %s, got %q", quoted(revision), read.header.Get("ETag"))
	}

	moved := f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "review"}`, "If-Match", quoted(revision))
	after := f.card(card)
	if moved.status != http.StatusOK || moved.header.Get("ETag") != quoted(after.Revision) || after.Revision == revision {
		t.Fatalf("a move with the current If-Match: wanted 200 and the new ETag %s, got %d %q %s", quoted(after.Revision), moved.status, moved.header.Get("ETag"), moved.body)
	}

	stale := f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "build"}`, "If-Match", quoted(revision))
	if stale.status != http.StatusPreconditionFailed || stale.object(t)["outcome"] != contract.OutcomeStale {
		t.Errorf("a move with an older revision: wanted 412 stale, got %d %s", stale.status, stale.body)
	}
	if stale.header.Get("ETag") != quoted(after.Revision) {
		t.Errorf("the stale answer carries ETag %q, wanted the current %s", stale.header.Get("ETag"), quoted(after.Revision))
	}
	if now := f.card(card); now.Column != after.Column {
		t.Errorf("a stale move moved the card from %s to %s", after.Column, now.Column)
	}

	missing := f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "build"}`)
	if missing.status != http.StatusPreconditionRequired || missing.refusal(t) != contract.BasisRequired {
		t.Errorf("a move with no basis: wanted 428 %s, got %d %s", contract.BasisRequired, missing.status, missing.body)
	}
	star := f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "build"}`, "If-Match", "*")
	if star.status != http.StatusOK {
		t.Errorf("a move with If-Match *: wanted 200, got %d %s", star.status, star.body)
	}
	for _, malformed := range []string{`"a", "b"`, `W/"` + revision + `"`} {
		got := f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "review"}`, "If-Match", malformed)
		if got.status != http.StatusBadRequest || got.detail(t) != "If-Match" {
			t.Errorf("If-Match %s: wanted 400 with detail If-Match, got %d %s", malformed, got.status, got.body)
		}
	}
	for _, path := range []string{"/cards", "/cards/" + card + "/comments"} {
		got := f.json(http.MethodPost, path, typeJSON, `{"title": "T", "text": "T"}`[:0]+bodyFor(path), "If-Match", quoted(revision))
		if got.status != http.StatusBadRequest || got.detail(t) != "If-Match" {
			t.Errorf("If-Match on POST %s: wanted 400 with detail If-Match, got %d %s", path, got.status, got.body)
		}
	}
}

// bodyFor is a valid creation body for the two creations that compare no
// basis.
func bodyFor(path string) string {
	if path == "/cards" {
		return `{"title": "A new card"}`
	}
	return `{"text": "A note."}`
}

// TestTheCardActsAnswerOverHTTP is dinah-152/criteria/9.
func TestTheCardActsAnswerOverHTTP(t *testing.T) {
	f := newFixture(t)
	card := f.add("A card", "build")

	claimed := f.json(http.MethodPost, "/cards/"+card+"/claim", typeJSON, `{"expires": "2h"}`)
	if claimed.status != http.StatusCreated || claimed.header.Get("Location") != "/cards/"+card+"/claim" {
		t.Errorf("claim: wanted 201 with Location /cards/%s/claim, got %d %q %s", card, claimed.status, claimed.header.Get("Location"), claimed.body)
	}
	held := f.json(http.MethodPost, "/cards/"+card+"/claim", typeJSON, "{}", "Dinah-Actor", "bryn")
	if held.status != http.StatusConflict || held.refusal(t) != contract.Held {
		t.Errorf("a second actor's claim: wanted 409 %s, got %d %s", contract.Held, held.status, held.body)
	}
	redirect := f.get("/cards/" + card + "/claim")
	if redirect.status != http.StatusSeeOther || redirect.header.Get("Location") != "/cards/"+card {
		t.Errorf("GET claim: wanted 303 to /cards/%s, got %d %q", card, redirect.status, redirect.header.Get("Location"))
	}
	put := f.send(request{method: http.MethodPut, path: "/cards/" + card + "/claim"})
	if put.status != http.StatusMethodNotAllowed || put.header.Get("Allow") != "GET, HEAD, POST, DELETE" {
		t.Errorf("PUT claim: wanted 405 with Allow GET, HEAD, POST, DELETE, got %d %q", put.status, put.header.Get("Allow"))
	}
	released := f.json(http.MethodDelete, "/cards/"+card+"/claim", "", "")
	if released.status != http.StatusOK || f.card(card).State != contract.StateReady {
		t.Errorf("release by the holder: wanted 200 and a ready card, got %d %s", released.status, released.body)
	}

	blocked := f.json(http.MethodPatch, "/cards/"+card, typeBlock, `{"reason": "An obstacle."}`, "If-Match", "*")
	if blocked.status != http.StatusOK || f.card(card).State != contract.StateBlocked {
		t.Errorf("block: wanted 200 and a blocked card, got %d %s", blocked.status, blocked.body)
	}
	unblocked := f.json(http.MethodPatch, "/cards/"+card, typeUnblock, `{"reason": "Cleared."}`, "If-Match", "*")
	if unblocked.status != http.StatusOK || f.card(card).State != contract.StateReady {
		t.Errorf("unblock: wanted 200 and a ready card, got %d %s", unblocked.status, unblocked.body)
	}

	queued := f.add("A queued card", "intake")
	pulled := f.json(http.MethodPost, "/claims", typePull, `{"column": "build"}`)
	if pulled.status != http.StatusCreated || pulled.header.Get("Location") != "/cards/"+queued+"/claim" {
		t.Errorf("pull: wanted 201 with Location /cards/%s/claim, got %d %q %s", queued, pulled.status, pulled.header.Get("Location"), pulled.body)
	}

	added := f.json(http.MethodPost, "/cards", typeJSON, `{"title": "A new card"}`)
	location := added.header.Get("Location")
	if added.status != http.StatusCreated || !strings.HasPrefix(location, "/cards/ht-") {
		t.Errorf("add: wanted 201 with the new card's Location, got %d %q %s", added.status, location, added.body)
	} else if f.get(location).status != http.StatusOK {
		t.Errorf("the new card's Location %s does not answer", location)
	}

	commented := f.json(http.MethodPost, "/cards/"+card+"/comments", typeJSON, `{"text": "A note."}`)
	if commented.status != http.StatusCreated || commented.header.Get("Location") != "/cards/"+commented.detail(t) {
		t.Errorf("comment: wanted 201 with the Location of the comment the answer names, got %d %q %s", commented.status, commented.header.Get("Location"), commented.body)
	} else if f.get(commented.header.Get("Location")).status != http.StatusOK {
		t.Errorf("the comment's Location %s does not answer", commented.header.Get("Location"))
	}
	column := f.json(http.MethodPost, "/columns/review/comments", typeJSON, `{"text": "A column note."}`)
	if column.status != http.StatusCreated || !strings.HasPrefix(column.header.Get("Location"), "/columns/review/comments/") {
		t.Errorf("a column comment: wanted 201 with a Location under /columns/review/comments/, got %d %q %s", column.status, column.header.Get("Location"), column.body)
	}
}

// specifiedStatus is the table of section 9.1 of dinah-152's specification,
// written out here so the map in status.go is held to the specification
// rather than to itself. dinah.not-served is dinah-338's, whose section 13
// maps it to 501.
var specifiedStatus = map[int][]string{
	400: {"malformed", "no-reason", "dinah.usage", "dinah.malformed-harness", "dinah.malformed-depth", "dinah.malformed-member-name", "dinah.unknown-field", "dinah.unknown-value", "dinah.unknown-axis", "dinah.repeated-axis", "dinah.unknown-depth", "dinah.chain-too-long", "dinah.multiple-words", "dinah.empty-search"},
	403: {"not-operator", "not-holder", "not-requester", "no-owner", "dinah.below-tier", "dinah.foreign-origin", "dinah.origin-required"},
	404: {"unknown-card", "unknown-column", "dinah.unknown-workstream", "dinah.unknown-view", "dinah.unknown-resource"},
	405: {"dinah.method-not-allowed"},
	406: {"dinah.not-acceptable"},
	413: {"dinah.body-too-large"},
	415: {"dinah.unsupported-media-type"},
	421: {"dinah.foreign-host"},
	428: {"dinah.basis-required"},
	501: {"dinah.not-implemented", "dinah.not-served"},
}

// TestEveryOutcomeMapsToItsStatus is dinah-152/criteria/10.
func TestEveryOutcomeMapsToItsStatus(t *testing.T) {
	listed := 0
	for status, names := range specifiedStatus {
		for _, name := range names {
			listed++
			got := statusFor(contract.OutcomeRefused, name)
			if got != status {
				t.Errorf("%s: wanted %d, got %d", name, status, got)
			}
		}
	}
	if listed != len(refusalStatus) {
		t.Errorf("the specification lists %d names and the map carries %d", listed, len(refusalStatus))
	}
	for outcome, status := range map[string]int{contract.OutcomeOK: 200, contract.OutcomeStale: 412, contract.OutcomeUnreachable: 503} {
		if got := statusFor(outcome, ""); got != status {
			t.Errorf("outcome %s: wanted %d, got %d", outcome, status, got)
		}
	}
	for _, unlisted := range []string{contract.Held, contract.UnknownRoute} {
		if got := statusFor(contract.OutcomeRefused, unlisted); got != http.StatusConflict {
			t.Errorf("the unlisted %s: wanted 409, got %d", unlisted, got)
		}
	}
	t.Logf("checked %d listed refusal names", listed)

	f := newFixture(t)
	card := f.add("A card", "build")
	f.act(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: card})
	provoked := []struct {
		name   string
		got    reply
		status int
		want   string
	}{
		{"an unknown card", f.get("/cards/ht-99"), 404, contract.UnknownCard},
		{"a release by somebody else", f.json(http.MethodDelete, "/cards/"+card+"/claim", "", "", "Dinah-Actor", "bryn"), 403, contract.NotHolder},
		{"a claim of a held card", f.json(http.MethodPost, "/cards/"+card+"/claim", typeJSON, "{}", "Dinah-Actor", "bryn"), 409, contract.Held},
		{"a block with no reason", f.json(http.MethodPatch, "/cards/"+card, typeBlock, `{"reason": ""}`, "If-Match", "*"), 400, contract.NoReason},
		{"a move with no basis", f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "review"}`), 428, contract.BasisRequired},
	}
	for _, p := range provoked {
		if p.got.status != p.status || p.got.refusal(t) != p.want {
			t.Errorf("%s: wanted %d %s, got %d %s", p.name, p.status, p.want, p.got.status, p.got.body)
		}
	}
	stale := f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "review"}`, "If-Match", quoted("sha256:"+strings.Repeat("0", 64)))
	if stale.status != http.StatusPreconditionFailed {
		t.Errorf("a stale move: wanted 412, got %d %s", stale.status, stale.body)
	}
}

// TestNegotiationAndHeaders is dinah-152/criteria/11.
func TestNegotiationAndHeaders(t *testing.T) {
	f := newFixture(t)
	card := f.add("A card", "build")
	var seen []reply
	record := func(r reply) reply {
		seen = append(seen, r)
		return r
	}

	asJSON := record(f.json(http.MethodPatch, "/cards/"+card, typeJSON, `{"column": "review"}`, "If-Match", "*"))
	if asJSON.status != http.StatusUnsupportedMediaType || asJSON.header.Get("Accept-Patch") != strings.Join(patchTypes, ", ") {
		t.Errorf("a move sent as application/json: wanted 415 with Accept-Patch %q, got %d %q", strings.Join(patchTypes, ", "), asJSON.status, asJSON.header.Get("Accept-Patch"))
	}
	if html := record(f.get("/cards/"+card, "Accept", "image/png")); html.status != http.StatusNotAcceptable || html.refusal(t) != contract.NotAcceptable {
		t.Errorf("Accept image/png: wanted 406, got %d %s", html.status, html.body)
	}
	if page := record(f.get("/cards/"+card, "Accept", "text/html")); page.status != http.StatusOK || page.header.Get("Content-Type") != htmlType {
		t.Errorf("Accept text/html: wanted 200 %s, which dinah-338 gives every GET row, got %d %q", htmlType, page.status, page.header.Get("Content-Type"))
	}
	vendor := `application/vnd.dinah+json; profile="` + bench.ProfileVersion + `"`
	star := record(f.get("/cards/"+card, "Accept", "*/*"))
	bare := record(f.get("/cards/" + card))
	for name, got := range map[string]reply{"Accept */*": star, "no Accept": bare} {
		if got.header.Get("Content-Type") != vendor {
			t.Errorf("%s: wanted Content-Type %s, got %q", name, vendor, got.header.Get("Content-Type"))
		}
	}
	plain := record(f.get("/cards/"+card, "Accept", "application/json"))
	if plain.header.Get("Content-Type") != typeJSON || plain.body != bare.body {
		t.Errorf("Accept application/json: wanted the same bytes labelled application/json, got %q and %d bytes against %d", plain.header.Get("Content-Type"), len(plain.body), len(bare.body))
	}
	browser := record(f.get("/cards/"+card, "Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"))
	if browser.header.Get("Content-Type") != htmlType {
		t.Errorf("a browser's usual Accept: wanted the page, %s, got %q", htmlType, browser.header.Get("Content-Type"))
	}
	record(f.get("/nowhere"))
	record(f.json(http.MethodPost, "/cards", typeJSON, `{"title": "A card"}`))
	for _, got := range seen {
		for name, want := range map[string]string{"Vary": "Accept", "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff"} {
			if got.header.Get(name) != want {
				t.Errorf("an answer of status %d carries %s %q, wanted %q", got.status, name, got.header.Get(name), want)
			}
		}
	}
	t.Logf("read the three headers on %d answers", len(seen))
}

// TestNegotiateRanksByTheMostSpecificRange holds negotiate to the ranking
// rules of section 11.2 of dinah-152's specification.
func TestNegotiateRanksByTheMostSpecificRange(t *testing.T) {
	cases := []struct {
		accept string
		want   string
		ok     bool
	}{
		{"", typeVendor, true},
		{"*/*", typeVendor, true},
		{"application/json", typeJSON, true},
		{"application/json;q=0, */*", typeVendor, true},
		{"application/vnd.dinah+json;q=0, */*", typeJSON, true},
		{"application/*;q=0.5, application/json", typeJSON, true},
		{"text/html", "", false},
		{"*/*;q=0", "", false},
	}
	for _, c := range cases {
		got, ok := negotiate(c.accept, jsonOffers)
		if got != c.want || ok != c.ok {
			t.Errorf("Accept %q: wanted %q %v, got %q %v", c.accept, c.want, c.ok, got, ok)
		}
	}
}

// TestTheFormTunnelReachesEveryAct is dinah-152/criteria/12.
func TestTheFormTunnelReachesEveryAct(t *testing.T) {
	f := newFixture(t)
	card := f.add("A card", "build")
	f.act(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: card})
	move := func(column, extra string) string {
		return "_method=PATCH&_type=" + url.QueryEscape(typeMove) + "&column=" + column + extra
	}

	revision := f.card(card).Revision
	moved := f.form("/cards/"+card, move("review", "&_basis="+url.QueryEscape(revision)))
	after := f.card(card)
	if moved.status != http.StatusOK || after.Revision == revision {
		t.Fatalf("a tunnelled move: wanted 200 and a moved card, got %d %s", moved.status, moved.body)
	}
	stale := f.form("/cards/"+card, move("build", "&_basis="+url.QueryEscape(revision)))
	if stale.status != http.StatusPreconditionFailed || stale.header.Get("ETag") != quoted(after.Revision) || f.card(card).Column != after.Column {
		t.Errorf("a tunnelled move with a stale _basis: wanted 412 with the current ETag and the card unmoved, got %d %q %s", stale.status, stale.header.Get("ETag"), stale.body)
	}
	if got := f.form("/cards/"+card, move("build", "")); got.status != http.StatusPreconditionRequired || got.refusal(t) != contract.BasisRequired {
		t.Errorf("a tunnelled move with no basis: wanted 428, got %d %s", got.status, got.body)
	}
	if got := f.form("/cards/"+card, move("build", "&_basis=*")); got.status != http.StatusOK {
		t.Errorf("a tunnelled move with _basis=*: wanted 200, got %d %s", got.status, got.body)
	}
	if got := f.form("/cards/"+card, move("review", "&_basis=")); got.status != http.StatusBadRequest || got.detail(t) != formBasis {
		t.Errorf("an empty _basis: wanted 400 with detail _basis, got %d %s", got.status, got.body)
	}
	current := f.card(card).Revision
	if got := f.form("/cards/"+card, move("review", "&_basis="+url.QueryEscape(current)), "If-Match", "*"); got.status != http.StatusBadRequest || got.detail(t) != formBasis {
		t.Errorf("_basis and If-Match disagreeing: wanted 400 with detail _basis, got %d %s", got.status, got.body)
	}
	if got := f.form("/cards/"+card, move("review", "&_basis="+url.QueryEscape(current)), "If-Match", quoted(current)); got.status != http.StatusOK {
		t.Errorf("_basis and If-Match agreeing: wanted 200, got %d %s", got.status, got.body)
	}
	if got := f.form("/cards/"+card, move("build", "&_basis="+url.QueryEscape(f.card(card).Revision)), "If-Match", `W/"x"`); got.status != http.StatusBadRequest || got.detail(t) != "If-Match" {
		t.Errorf("a weak If-Match beside a valid _basis: wanted 400 with detail If-Match, got %d %s", got.status, got.body)
	}
	for _, path := range []string{"/cards", "/cards/" + card + "/comments"} {
		body := "title=A+card&_basis=*"
		if path != "/cards" {
			body = "text=A+note&_basis=*"
		}
		if got := f.form(path, body); got.status != http.StatusBadRequest || got.detail(t) != formBasis {
			t.Errorf("_basis on a form POST to %s: wanted 400 with detail _basis, got %d %s", path, got.status, got.body)
		}
	}
	if got := f.form("/cards/"+card+"/claim", "_method=DELETE"); got.status != http.StatusOK || f.card(card).State != contract.StateReady {
		t.Errorf("_method=DELETE on the claim: wanted 200 and a released card, got %d %s", got.status, got.body)
	}
	for _, method := range []string{"GET", "PUT", "patch"} {
		if got := f.form("/cards/"+card, "_method="+method); got.status != http.StatusBadRequest || got.detail(t) != formMethod {
			t.Errorf("_method=%s: wanted 400 with detail _method, got %d %s", method, got.status, got.body)
		}
	}
	if got := f.form("/cards/"+card, "_method=PATCH&column=review&_basis=*"); got.status != http.StatusBadRequest || got.detail(t) != formType {
		t.Errorf("_method=PATCH without _type: wanted 400 with detail _type, got %d %s", got.status, got.body)
	}
	if got := f.form("/cards/"+card, "column=review"); got.status != http.StatusMethodNotAllowed || got.header.Get("Allow") != "GET, HEAD, PATCH" {
		t.Errorf("a form POST to the card with no _method: wanted 405 with Allow GET, HEAD, PATCH, got %d %q", got.status, got.header.Get("Allow"))
	}
	if got := f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "review", "_method": "PATCH"}`, "If-Match", "*"); got.status != http.StatusBadRequest || got.detail(t) != formMethod {
		t.Errorf("an underscore member in a JSON body: wanted 400 naming it, got %d %s", got.status, got.body)
	}
	for _, body := range []string{"_method=PATCH&_method=PATCH", "_method=PATCH&_method=DELETE"} {
		if got := f.form("/cards/"+card, body); got.status != http.StatusBadRequest || got.detail(t) != formMethod {
			t.Errorf("%s: wanted 400 with detail _method, got %d %s", body, got.status, got.body)
		}
	}
	if got := f.form("/cards/"+card, move("review", "&column=build&_basis=*")); got.status != http.StatusBadRequest || got.detail(t) != "column" {
		t.Errorf("column given twice: wanted 400 with detail column, got %d %s", got.status, got.body)
	}
	before := f.card(card).Column
	if got := f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "review", "column": "build"}`, "If-Match", "*"); got.status != http.StatusBadRequest || got.detail(t) != "column" || f.card(card).Column != before {
		t.Errorf("column given twice in a JSON body: wanted 400 with detail column and the card left in %s, got %d %s and the card in %s", before, got.status, got.body, f.card(card).Column)
	}
	if got := f.json(http.MethodPatch, "/cards/"+card, typeMove, `{"column": "`+before+`", "note": {"a": 1, "a": 2}}`, "If-Match", "*"); got.status != http.StatusBadRequest || got.detail(t) == "a" {
		t.Errorf("a name repeated inside a nested value: wanted the refusal to name the member's type rather than a repetition, got %d %s", got.status, got.body)
	}

	queued := f.add("A queued card", "intake")
	if got := f.form("/claims", "column=build&no-claim=on"); got.status != http.StatusOK || f.card(queued).State != contract.StateReady {
		t.Errorf("a pull with the marker no-claim sent as on: wanted 200 and an unclaimed card, got %d %s", got.status, got.body)
	}
}

// TestPathsAndReferencesMapBothWays is dinah-152/criteria/14.
func TestPathsAndReferencesMapBothWays(t *testing.T) {
	shapes := []struct {
		path string
		kind pathKind
		ref  string
	}{
		{"/workbench", pathWorkbench, "workbench"},
		{"/cards", pathRoster, "cards"},
		{"/columns", pathRoster, "columns"},
		{"/workstreams", pathRoster, "workstreams"},
		{"/routes", pathRoster, "routes"},
		{"/attachments", pathRoster, "attachments"},
		{"/cards/ht-1", pathCard, "ht-1"},
		{"/cards/ht-1/comments/2", pathCard, "ht-1/comments/2"},
		{"/columns/review", pathColumn, "review"},
		{"/columns/review/comments/1", pathColumn, "review/comments/1"},
		{"/workstreams/golden", pathWorkstream, "workstream/golden"},
	}
	for _, shape := range shapes {
		kind, ref, ok := refForPath(shape.path)
		if !ok || kind != shape.kind || ref != shape.ref {
			t.Errorf("refForPath(%s): wanted %d %q, got %d %q %v", shape.path, shape.kind, shape.ref, kind, ref, ok)
		}
		if back := pathForRef(shape.kind, shape.ref); back != shape.path {
			t.Errorf("pathForRef(%d, %q): wanted %s, got %s", shape.kind, shape.ref, shape.path, back)
		}
	}

	f := newFixture(t)
	card := f.add("A card", "build")
	f.act(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: card})
	if response := f.library().Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: card, Text: "A note."}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %+v", response)
	}
	if got := f.get("/columns/" + card); got.status != http.StatusNotFound || got.refusal(t) != contract.UnknownColumn {
		t.Errorf("a card under /columns/: wanted 404 unknown-column, got %d %s", got.status, got.body)
	}
	if got := f.get("/cards/review"); got.status != http.StatusNotFound || got.refusal(t) != contract.UnknownCard {
		t.Errorf("a column under /cards/: wanted 404 unknown-card, got %d %s", got.status, got.body)
	}
	collection := f.get("/cards/" + card + "/comments").object(t)
	item := f.get("/cards/" + card + "/comments/1").object(t)
	listed, shown := showMembers(collection), showMembers(item)
	if listed != "" || shown == "" {
		t.Errorf("a collection answered with the show member %q and an item with %q; wanted list to answer the first and show the second", listed, shown)
	}
}

// showMembers names the member a show answer publishes its subject under,
// and is empty for an answer carrying none of them.
func showMembers(payload map[string]any) string {
	for _, member := range []string{"detail", "item", "record", "text"} {
		if _, ok := payload[member]; ok {
			return member
		}
	}
	return ""
}

// TestTheWindowRouteIsReserved is dinah-152/criteria/15, as dinah-338 left
// it: the route dinah-152 reserved answers with the window renderer that
// replaced its 501, and still answers 404 for a card that does not exist.
func TestTheWindowRouteIsReserved(t *testing.T) {
	f := newFixture(t)
	card := f.add("A card", "build")
	if got := f.get("/cards/" + card + "/window"); got.status != http.StatusOK || !strings.HasPrefix(strings.TrimSpace(got.body), "<article class=\"win") {
		t.Errorf("a card's window: wanted 200 and the window's markup, got %d %s", got.status, got.body)
	}
	if got := f.get("/cards/ht-99/window"); got.status != http.StatusNotFound || !strings.Contains(got.body, contract.UnknownCard) {
		t.Errorf("a missing card's window: wanted 404 unknown-card, got %d %s", got.status, got.body)
	}
	for _, r := range routes {
		if r.pattern == "/cards/{card}/window" {
			if len(r.offered) != 1 || r.offered[0] != typeHTML {
				t.Errorf("the window route offers %v, wanted text/html alone", r.offered)
			}
			return
		}
	}
	t.Error("the route table has no window route")
}

// TestTwoClaimsRaceAndOneWins is dinah-152/criteria/17.
func TestTwoClaimsRaceAndOneWins(t *testing.T) {
	f := newFixture(t)
	card := f.add("A card", "build")
	actors := []string{"alka", "bryn"}
	answers := make([]reply, len(actors))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, actor := range actors {
		wg.Add(1)
		go func(i int, actor string) {
			defer wg.Done()
			<-start
			answers[i] = f.json(http.MethodPost, "/cards/"+card+"/claim", typeJSON, "{}", "Dinah-Actor", actor)
		}(i, actor)
	}
	close(start)
	wg.Wait()
	created, conflicted := 0, 0
	for _, got := range answers {
		switch got.status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			if name := got.refusal(t); name != contract.Held && name != contract.Locked {
				t.Errorf("the loser was refused %s, wanted held or %s", name, contract.Locked)
			}
			conflicted++
		default:
			t.Errorf("a claim answered %d %s", got.status, got.body)
		}
	}
	if created != 1 || conflicted != 1 {
		t.Errorf("wanted one 201 and one 409, got %d and %d", created, conflicted)
	}
	claims := 0
	for _, event := range f.journal(card) {
		if event.Event == "claimed" {
			claims++
		}
	}
	if claims != 1 {
		t.Errorf("the journal carries %d claimed events, wanted one", claims)
	}
	_ = strconv.Itoa
}
