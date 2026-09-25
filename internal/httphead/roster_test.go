package httphead

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"dinah/internal/answer"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// cardReference is how a later-card exemption names the card that carries
// the command.
var cardReference = regexp.MustCompile(`\bdinah-[0-9]+\b`)

// TestEveryCommandIsRoutedOrExempt is dinah-152/criteria/5.
func TestEveryCommandIsRoutedOrExempt(t *testing.T) {
	routed := map[string]bool{}
	for _, name := range routedCommands() {
		routed[name] = true
		if !answer.Runs(name) {
			t.Errorf("%s is routed and package answer has no runner for it", name)
		}
	}
	grounds := map[string]bool{}
	for _, ground := range routeGrounds {
		grounds[ground] = true
	}
	for name, held := range routeExemptions {
		if routed[name] {
			t.Errorf("%s is both routed and exempt", name)
		}
		if !grounds[held.ground] {
			t.Errorf("%s is exempt on %q, which is not one of the closed set %v", name, held.ground, routeGrounds)
		}
		if strings.TrimSpace(held.reason) == "" {
			t.Errorf("%s is exempt with no reason", name)
		}
		if held.ground == GroundLaterCard && !cardReference.MatchString(held.reason) {
			t.Errorf("%s is held for a later card and its reason names no card: %q", name, held.reason)
		}
	}
	commands := verb.Commands()
	for _, name := range commands {
		if _, exempt := routeExemptions[name]; !routed[name] && !exempt {
			t.Errorf("%s is neither routed nor exempt", name)
		}
	}
	known := map[string]bool{}
	for _, name := range commands {
		known[name] = true
	}
	for name := range routed {
		if !known[name] {
			t.Errorf("%s is routed and is not a command", name)
		}
	}
	for name := range routeExemptions {
		if !known[name] {
			t.Errorf("%s is exempt and is not a command", name)
		}
	}
	if len(routed) == 0 || len(routeExemptions) == 0 || len(routed)+len(routeExemptions) != len(commands) {
		t.Errorf("%d routed and %d exempt do not make the %d commands verb.Commands() declares", len(routed), len(routeExemptions), len(commands))
	}
	t.Logf("%d commands routed and %d exempt, of %d", len(routed), len(routeExemptions), len(commands))
}

// drive is one route and command the parameter guard sends a request to.
type drive struct {
	command string
	method  string
	path    string
	// mediaType is the body type of an act, empty for a read.
	mediaType string
	// bound are the parameters the path supplies, with the value each
	// should reach the request as.
	bound map[string]string
	// required are members every request to this act carries, beside the
	// one under test.
	required map[string]any
}

// drives lists every routed read and act with a concrete path on the
// fixture, generated from the route table.
func drives(t *testing.T, card, column, slug string) []drive {
	t.Helper()
	var out []drive
	fill := func(pattern string) string {
		p := strings.ReplaceAll(pattern, "{card}", card)
		p = strings.ReplaceAll(p, "{column}", column)
		p = strings.ReplaceAll(p, "{slug}", slug)
		p = strings.ReplaceAll(p, "{view}", "mine")
		return strings.TrimSuffix(p, "{$}")
	}
	for _, r := range routes {
		for _, command := range r.reads {
			path := fill(r.pattern)
			bound := map[string]string{}
			switch {
			case r.pattern == "/cards" && command == "query":
			case strings.HasSuffix(r.pattern, "{rest...}"):
				rest := "comments/1"
				if command == "list" {
					rest = "comments"
				}
				path = strings.ReplaceAll(path, "{rest...}", rest)
				head := card
				if r.binds == pathColumn {
					head = column
				}
				if command == "list" {
					bound["ref"] = head + "/" + rest
				} else {
					bound["card"] = head + "/" + rest
				}
			case command == "list":
				_, ref, _ := refForPath(path)
				bound["ref"] = ref
			case command == "show" || command == "instructions":
				_, ref, _ := refForPath(strings.TrimSuffix(path, "/instructions"))
				bound["card"] = ref
			case command == "view" && strings.Contains(r.pattern, "{view}"):
				bound["view"] = "mine"
			case command == "view":
				bound = map[string]string{"view": "", "card": "", "explain": ""}
			}
			out = append(out, drive{command: command, method: http.MethodGet, path: path, bound: bound})
		}
		for _, m := range r.methods {
			for _, a := range m.acts {
				path := strings.ReplaceAll(fill(r.pattern), "{rest...}", "comments")
				d := drive{command: a.command, method: m.name, path: path, mediaType: a.selectedBy, required: map[string]any{}, bound: map[string]string{}}
				if d.mediaType == "" && len(m.accepts) > 0 {
					d.mediaType = m.accepts[0]
				}
				if r.binds == pathCard {
					d.bound["card"] = card
				}
				if r.binds == pathColumn {
					d.bound["card"] = column
				}
				for _, param := range verb.Params(a.command) {
					if param.Required && d.bound[param.Name] == "" {
						d.required[param.Name] = sampleOf(param)
					}
				}
				out = append(out, d)
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("the route table generated nothing to drive")
	}
	return out
}

// sampleOf is a value a parameter can carry, a boolean for a marker and a
// string the request builder keeps for anything else.
func sampleOf(param verb.Param) any {
	if param.Marker {
		return true
	}
	if param.Name == "expires" {
		return "2h"
	}
	return "sample-" + param.Name
}

// holds reports whether a request field carries the sample a parameter was
// sent with.
func holds(field reflect.Value, sample any) bool {
	switch value := field.Interface().(type) {
	case string:
		return value == sample
	case bool:
		return value
	case time.Duration:
		return value != 0
	}
	return false
}

// TestEveryParameterReachesItsField is dinah-152/criteria/6. For every
// routed command it sends each declared parameter carrying a Field over the
// route, alone beside whatever the act requires, and holds the request the
// library received to the value sent; a parameter held back must be in
// paramExemptions with a reason. It also sends a member no route publishes,
// on every route, and wants 400 naming it.
func TestEveryParameterReachesItsField(t *testing.T) {
	var got *verb.Request
	f := newFixture(t, func(cfg *Config) {
		cfg.observe = func(_ string, req *verb.Request) { got = req }
	})
	card := f.add("A card", "build")
	if response := f.library().Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: card, Text: "A note."}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %+v", response)
	}
	if response := f.library().Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: "review", Text: "A note."}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("column comment: %+v", response)
	}
	if response := f.library().NewWorkstream(&verb.Request{Verb: "workstream", Actor: "alka", Action: "new", Workstream: "Stream", Slug: "stream"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("workstream: %+v", response)
	}

	for command, held := range paramExemptions {
		declared := map[string]bool{}
		for _, param := range verb.Params(command) {
			declared[param.Name] = param.Field != ""
		}
		for name, reason := range held {
			if !declared[name] {
				t.Errorf("paramExemptions holds back %s %s, which the command does not declare with a Field", command, name)
			}
			if strings.TrimSpace(reason) == "" {
				t.Errorf("paramExemptions holds back %s %s with no reason", command, name)
			}
		}
	}

	driven, reached := 0, map[string]map[string]bool{}
	mark := func(command, name string) {
		if reached[command] == nil {
			reached[command] = map[string]bool{}
		}
		reached[command][name] = true
	}
	for _, d := range drives(t, card, "review", "stream") {
		bare := func(extra map[string]any) reply {
			if d.method == http.MethodGet {
				query := url.Values{}
				for name, value := range extra {
					if flag, ok := value.(bool); ok && flag {
						query.Set(name, "")
						continue
					}
					query.Set(name, value.(string))
				}
				path := d.path
				if len(query) > 0 {
					path += "?" + query.Encode()
				}
				return f.get(path)
			}
			members := map[string]any{}
			for name, value := range d.required {
				members[name] = value
			}
			for name, value := range extra {
				members[name] = value
			}
			header := []string{}
			if comparesBasis[d.command] {
				header = append(header, "If-Match", "*")
			}
			if d.method == http.MethodDelete {
				path := d.path
				for name := range extra {
					path += "?" + name + "=x"
				}
				return f.json(d.method, path, "", "", header...)
			}
			encoded, _ := json.Marshal(members)
			return f.json(d.method, d.path, d.mediaType, string(encoded), header...)
		}
		members := published(d.command, keysOf(d.bound)...)
		names := make([]string, 0, len(members))
		for name := range members {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			param := members[name]
			if d.method != http.MethodGet && d.mediaType == typeForm {
				continue
			}
			sample := sampleOf(param)
			got = nil
			extra := map[string]any{name: sample}
			if d.command == "search" && name != "phrase" {
				extra["phrase"] = "sample-phrase"
			}
			answered := bare(extra)
			driven++
			if got == nil {
				t.Errorf("%s %s with %s=%v never reached the library: %d %s", d.method, d.path, name, sample, answered.status, answered.body)
				continue
			}
			field := reflect.ValueOf(got).Elem().FieldByName(param.Field)
			if !field.IsValid() || !holds(field, sample) {
				t.Errorf("%s %s with %s=%v: the request's %s holds %v", d.method, d.path, name, sample, param.Field, field)
				continue
			}
			mark(d.command, name)
		}
		for name, value := range d.bound {
			if value == "" {
				// The route takes none of the command's own parameter of this
				// name, so naming it is refused rather than read.
				withheld := bare(map[string]any{name: "x"})
				if withheld.status != http.StatusBadRequest || withheld.detail(t) != name {
					t.Errorf("%s %s with %s, which the route withholds: wanted 400 naming it, got %d %s", d.method, d.path, name, withheld.status, withheld.body)
				}
				continue
			}
			param := verb.Param{}
			for _, declared := range verb.Params(d.command) {
				if declared.Name == name {
					param = declared
				}
			}
			got = nil
			extra := map[string]any{}
			if d.command == "search" {
				extra["phrase"] = "sample-phrase"
			}
			answered := bare(extra)
			driven++
			if got == nil {
				t.Errorf("%s %s never reached the library: %d %s", d.method, d.path, answered.status, answered.body)
				continue
			}
			field := reflect.ValueOf(got).Elem().FieldByName(param.Field)
			if !field.IsValid() || field.String() != value {
				t.Errorf("%s %s: the path's %s reached the request's %s as %v, wanted %q", d.method, d.path, name, param.Field, field, value)
				continue
			}
			mark(d.command, name)
		}
		stray := bare(map[string]any{"no-such-member": "x"})
		if stray.status != http.StatusBadRequest || stray.refusal(t) != contract.Usage || stray.detail(t) != "no-such-member" {
			t.Errorf("%s %s with an unpublished member: wanted 400 %s naming it, got %d %s", d.method, d.path, contract.Usage, stray.status, stray.body)
		}
	}
	for _, command := range routedCommands() {
		for _, param := range verb.Params(command) {
			if param.Field == "" {
				continue
			}
			if _, held := paramExemptions[command][param.Name]; held {
				continue
			}
			if !reached[command][param.Name] {
				t.Errorf("%s %s carries a Field, is not held back, and no route carried it to the request", command, param.Name)
			}
		}
	}
	t.Logf("drove %d parameters", driven)
}
