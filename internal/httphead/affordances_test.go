package httphead

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// affordanceDocument reads GET /affordances.
func affordanceDocument(t *testing.T, f *fixture) []affordanceRow {
	t.Helper()
	got := f.get("/affordances")
	if got.status != http.StatusOK {
		t.Fatalf("GET /affordances: %d %s", got.status, got.body)
	}
	var document struct {
		Affordances []affordanceRow `json:"affordances"`
	}
	if err := json.Unmarshal([]byte(got.body), &document); err != nil {
		t.Fatalf("GET /affordances: %v\n%s", err, got.body)
	}
	if len(document.Affordances) == 0 {
		t.Fatal("GET /affordances listed no rows")
	}
	return document.Affordances
}

// TestTheAffordanceTableAgreesWithTheRoutes is the first half of
// dinah-152/criteria/16: one row per name, the form type only on POST rows,
// and each method and href's accepted types equal to the header the route
// answers with.
func TestTheAffordanceTableAgreesWithTheRoutes(t *testing.T) {
	f := newFixture(t)
	card := f.add("A card", "build")
	rows := affordanceDocument(t, f)

	seen := map[string]bool{}
	union := map[string]map[string]bool{}
	for _, row := range rows {
		if seen[row.Affordance] {
			t.Errorf("the table carries %s twice", row.Affordance)
		}
		seen[row.Affordance] = true
		if row.Method == "" || row.Href == "" || row.Accepts == nil {
			t.Errorf("%s: a row wants a method, an href and an accepts list, got %+v", row.Affordance, row)
		}
		if (row.Method == http.MethodGet) != (row.Form == nil) {
			t.Errorf("%s: a read carries a null form and an act a form, got method %s and form %+v", row.Affordance, row.Method, row.Form)
		}
		if row.Form != nil && row.Form.Method != http.MethodPost {
			t.Errorf("%s: a form posts, and this one says %s", row.Affordance, row.Form.Method)
		}
		for _, accepted := range row.Accepts {
			if accepted == typeForm && row.Method != http.MethodPost {
				t.Errorf("%s: a %s row lists the form type, which only a POST takes", row.Affordance, row.Method)
			}
		}
		key := row.Method + " " + row.Href
		if union[key] == nil {
			union[key] = map[string]bool{}
		}
		for _, accepted := range row.Accepts {
			union[key][accepted] = true
		}
	}

	// The header each method and href answers with, provoked on the fixture:
	// a card's GET carries Accept-Patch, and a 415 on a POST route carries
	// Accept-Post.
	concrete := func(href string) string {
		href = strings.ReplaceAll(href, "{card}", card)
		return strings.ReplaceAll(href, "{path}", "/cards/"+card)
	}
	compared := 0
	for key, types := range union {
		if len(types) == 0 {
			continue
		}
		method, href, _ := strings.Cut(key, " ")
		var header string
		switch method {
		case http.MethodPatch:
			header = f.get(concrete(href)).header.Get("Accept-Patch")
		case http.MethodPost:
			got := f.json(http.MethodPost, concrete(href), "text/plain", "x")
			if got.status != http.StatusUnsupportedMediaType {
				t.Errorf("%s: a text/plain POST answered %d, wanted 415", key, got.status)
			}
			header = got.header.Get("Accept-Post")
		default:
			t.Errorf("%s: a %s row lists body types", key, method)
			continue
		}
		var want []string
		for accepted := range types {
			want = append(want, accepted)
		}
		sort.Strings(want)
		have := strings.Split(header, ", ")
		sort.Strings(have)
		if strings.Join(want, ", ") != strings.Join(have, ", ") {
			t.Errorf("%s: the rows accept %v and the route answers with %q", key, want, header)
		}
		compared++
	}
	if compared == 0 {
		t.Fatal("no method and href listed a body type, so no header was compared")
	}
	t.Logf("compared %d headers over %d rows", compared, len(rows))
}

// collectAffordances gathers every array of strings named affordances at any
// depth of a JSON value.
func collectAffordances(value any, into map[string]bool) {
	switch typed := value.(type) {
	case map[string]any:
		for member, inner := range typed {
			if member == "affordances" {
				if names, ok := inner.([]any); ok {
					for _, name := range names {
						if text, ok := name.(string); ok {
							into[text] = true
						}
					}
				}
			}
			collectAffordances(inner, into)
		}
	case []any:
		for _, inner := range typed {
			collectAffordances(inner, into)
		}
	}
}

// TestEveryPublishedAffordanceHasARow is the second half of
// dinah-152/criteria/16. It drives the head through the positions section 12
// of dinah-152's specification names and collects every affordance name the
// answers publish, so a name the library adds later reaches one of them and
// fails here until the table has a row for it.
func TestEveryPublishedAffordanceHasARow(t *testing.T) {
	f := newFixture(t)
	atBuild := f.add("Ready where a claim takes it up", "build")
	atIntake := f.add("Ready where a pull carries it on", "intake")
	atWaiting := f.add("Ready where nothing takes it up", "waiting")
	active := f.add("Active", "build")
	f.act(&verb.Request{Verb: verb.Claim, Actor: "alka", Card: active})
	blocked := f.add("Blocked", "build")
	f.act(&verb.Request{Verb: verb.Block, Actor: "alka", Card: blocked, Reason: "An obstacle."})
	finished := f.add("Finished", "build")
	f.act(&verb.Request{Verb: verb.Move, Actor: "alka", Card: finished, Column: "finished", Override: true})
	if response := f.library().Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: atBuild, Text: "A note."}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %+v", response)
	}
	if response := f.library().NewWorkstream(&verb.Request{Verb: "workstream", Actor: "alka", Action: "new", Workstream: "Stream", Slug: "stream"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("workstream: %+v", response)
	}

	var answers []reply
	positions := 0
	drive := func(got reply) reply {
		positions++
		answers = append(answers, got)
		return got
	}
	for _, card := range []string{atBuild, atIntake, atWaiting, active, blocked, finished} {
		drive(f.get("/cards/" + card))
		drive(f.get("/cards/" + card + "/instructions"))
	}
	for _, path := range []string{
		"/", "/workbench", "/cards", "/cards?query=state:ready", "/cards/" + atBuild + "/comments",
		"/cards/" + atBuild + "/comments/1", "/cards/" + atBuild + "/journal", "/columns", "/columns/build",
		"/columns/build/instructions", "/workstreams", "/workstreams/stream", "/routes", "/attachments",
		"/next", "/changes", "/tree", "/search?phrase=card", "/views", "/views/mine", "/whoami", "/version",
	} {
		if got := drive(f.get(path)); got.status != http.StatusOK {
			t.Errorf("GET %s answered %d %s", path, got.status, got.body)
		}
	}
	refusedBeforeACard := []struct {
		method, path, mediaType, body string
	}{
		{http.MethodPost, "/cards", typeJSON, `{"title": "T", "column": "nowhere"}`},
		{http.MethodPatch, "/cards/ht-99", typeMove, `{"column": "review"}`},
		{http.MethodPatch, "/cards/ht-99", typeBlock, `{"reason": "R"}`},
		{http.MethodPatch, "/cards/ht-99", typeUnblock, `{}`},
		{http.MethodPost, "/cards/ht-99/claim", typeJSON, `{}`},
		{http.MethodDelete, "/cards/ht-99/claim", "", ""},
		{http.MethodPost, "/claims", typePull, `{"column": "nowhere"}`},
		{http.MethodPost, "/cards/ht-99/comments", typeJSON, `{"text": "T"}`},
		{http.MethodPost, "/columns/nowhere/comments", typeJSON, `{"text": "T"}`},
	}
	for _, act := range refusedBeforeACard {
		got := drive(f.json(act.method, act.path, act.mediaType, act.body, "If-Match", "*"))
		if act.method == http.MethodPost && (act.path == "/cards" || act.path == "/cards/ht-99/comments" || act.path == "/columns/nowhere/comments") {
			got = drive(f.json(act.method, act.path, act.mediaType, act.body))
		}
		if got.object(t)["outcome"] != contract.OutcomeRefused {
			t.Errorf("%s %s was driven to be refused before a card was found and answered %d %s", act.method, act.path, got.status, got.body)
		}
	}
	own := []struct {
		name string
		got  reply
		want int
	}{
		{"a path no route matches", drive(f.get("/nowhere")), http.StatusNotFound},
		{"a type the route does not take", drive(f.json(http.MethodPost, "/cards", "text/plain", "x")), http.StatusUnsupportedMediaType},
		{"a PATCH with no basis", drive(f.json(http.MethodPatch, "/cards/"+atBuild, typeMove, `{"column": "review"}`)), http.StatusPreconditionRequired},
	}
	for _, refusal := range own {
		names := map[string]bool{}
		collectAffordances(refusal.got.object(t), names)
		if refusal.got.status != refusal.want || !names["next_card"] || names["next"] {
			t.Errorf("%s: wanted %d publishing next_card and never next, got %d %v", refusal.name, refusal.want, refusal.got.status, names)
		}
	}

	collected := map[string]bool{}
	for _, got := range answers {
		var value any
		if err := json.Unmarshal([]byte(got.body), &value); err != nil {
			t.Fatalf("an answer is not JSON: %v\n%s", err, got.body)
		}
		collectAffordances(value, collected)
	}
	rows := map[string]bool{}
	for _, row := range affordanceDocument(t, f) {
		rows[row.Affordance] = true
	}
	var names []string
	for name := range collected {
		names = append(names, name)
		if !rows[name] {
			t.Errorf("an answer publishes %s and GET /affordances has no row for it", name)
		}
	}
	sort.Strings(names)
	for _, wanted := range []string{"claim", "pull", "release", "unblock", "next_card"} {
		if !collected[wanted] {
			t.Errorf("no position published %s, so the positions do not cover what the library offers", wanted)
		}
	}
	t.Logf("drove %d positions and collected %d distinct names: %v", positions, len(names), names)
}
