package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/mcp"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// searchWord is the word these fixtures plant. Nothing a bare init writes
// carries it, so what a search answers is what the test put in.
const searchWord = "coelacanth"

// TestSearchPrintsATableAndSaysSoWhenNothingMatches asserts what a person at a
// terminal sees: a table naming what matched and where, and one sentence when
// nothing did.
func TestSearchPrintsATableAndSaysSoWhenNothingMatches(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card about the "+searchWord); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	found := runCLI(t, root, "search", searchWord)
	if found.code != 0 {
		t.Fatalf("search: %d %s", found.code, found.errw)
	}
	for _, wanted := range []string{
		msg.For(msg.Base).T("column.search.kind"),
		msg.For(msg.Base).T("column.search.matched"),
		"fx-1",
		searchWord,
	} {
		if !strings.Contains(found.out, wanted) {
			t.Errorf("the table does not carry %q:\n%s", wanted, found.out)
		}
	}
	empty := runCLI(t, root, "search", "quantum teleportation")
	if empty.code != 0 {
		t.Fatalf("the search that found nothing: %d %s", empty.code, empty.errw)
	}
	if want := msg.For(msg.Base).T("search.empty"); !strings.Contains(empty.out, want) {
		t.Errorf("the empty answer is %q, wanted the sentence %q", empty.out, want)
	}
	if strings.Contains(empty.out, msg.For(msg.Base).T("column.search.kind")) {
		t.Errorf("the empty answer drew a table:\n%s", empty.out)
	}
}

// TestSearchWithNoPhraseIsRefused asserts dinah-268 AC-13 at the terminal: an
// empty phrase is refused by name rather than answered with every card.
func TestSearchWithNoPhraseIsRefused(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for _, argv := range [][]string{{"search", ""}, {"search"}} {
		got := runCLI(t, root, argv...)
		if got.code == 0 {
			t.Errorf("%v was accepted: %s", argv, got.out)
			continue
		}
		if leading := leadingToken(got.errw); leading != contract.EmptySearch {
			t.Errorf("%v: leading token %q, wanted %s", argv, leading, contract.EmptySearch)
		}
		// The name alone is not the answer. A refusal the shape table does not
		// carry still leads with its own name and then says Dinah has no
		// message for it, so the sentence is what proves the refusal was
		// written rather than merely raised.
		for _, key := range []string{"refusal.dinah.empty-search", "refusal.dinah.empty-search.next"} {
			if want := msg.For(msg.Base).T(key); !strings.Contains(got.errw, want) {
				t.Errorf("%v: the refusal does not carry %s (%q): %s", argv, key, want, got.errw)
			}
		}
	}
}

// TestSearchRefusesAnUnquotedPhrase asserts dinah-268 AC-16: several unquoted
// words meet the refusal every open-tail command gives, with the line rebuilt
// quoted and the slot named as the phrase.
func TestSearchRefusesAnUnquotedPhrase(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "search", "two", "words", "--query", "state:ready")
	if got.code == 0 {
		t.Fatalf("the unquoted phrase was accepted: %s", got.out)
	}
	if leading := leadingToken(got.errw); leading != contract.MultipleWords {
		t.Fatalf("leading token %q, wanted %s", leading, contract.MultipleWords)
	}
	if !strings.Contains(got.errw, `dinah search "two words"`) {
		t.Errorf("the refusal does not rebuild the line quoted: %s", got.errw)
	}
	if want := msg.For(msg.Base).T("slot.search"); !strings.Contains(got.errw, want) {
		t.Errorf("the refusal does not name the slot as %q: %s", want, got.errw)
	}
}

// TestSearchAcrossARootGroupsByWorkbench asserts dinah-268 AC-14: a root-scoped
// search answers one member per workbench beneath the root, each carrying that
// workbench's own hits, in the shape dinah ls --root already produces.
//
// The shape is compared against that listing's own answer rather than against
// a fixture written here, so a search that grouped its answer differently
// fails even where its own members are right.
func TestSearchAcrossARootGroupsByWorkbench(t *testing.T) {
	root := newForest(t, "alpha", "customer/beta")
	for _, place := range []string{"alpha", filepath.Join("customer", "beta")} {
		dir := soleBenchDir(t, filepath.Join(root, place))
		if got := runCLI(t, root, "add", "A "+searchWord+" in "+place, "--workbench", dir); got.code != 0 {
			t.Fatalf("add in %s: %d %s", place, got.code, got.errw)
		}
	}
	answer := forestJSON(t, root, "search", searchWord, "--root", root)
	rows := members(t, answer)
	listed := members(t, forestJSON(t, root, "ls", "--root", root))
	if len(rows) != len(listed) {
		t.Fatalf("the search answered for %d workbenches and the listing for %d", len(rows), len(listed))
	}
	for at, row := range rows {
		if row["path"] != listed[at]["path"] {
			t.Errorf("member %d is %v in the search and %v in the listing", at, row["path"], listed[at]["path"])
		}
		results, ok := row["results"].(map[string]any)
		if !ok {
			t.Fatalf("member %d carries no results: %v", at, row)
		}
		hits, ok := results["hits"].([]any)
		if !ok || len(hits) != 1 {
			t.Errorf("member %d carries %v, wanted its own one hit", at, results["hits"])
		}
	}
	if answer["root"] != root {
		t.Errorf("the answer names the root as %v, wanted %s", answer["root"], root)
	}
}

// TestBothHeadsAnswerOneSearchAlike asserts dinah-268 AC-15: the hits the
// search_cards tool publishes are the hits the terminal prints under --json for
// the same workbench and the same phrase, field for field.
func TestBothHeadsAnswerOneSearchAlike(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card about the "+searchWord); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "comment", "fx-1", "The comment mentions "+searchWord+" too."); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	printed := runCLI(t, root, "search", searchWord, "--json")
	if printed.code != 0 {
		t.Fatalf("the terminal: %d %s", printed.code, printed.errw)
	}
	terminal, ok := decode(t, printed.out).(map[string]any)
	if !ok {
		t.Fatalf("the terminal answered something that is not an object: %s", printed.out)
	}
	served := searchOverTheProtocol(t, root, map[string]any{"actor": "alka", "phrase": searchWord})
	results, named := served["results"].(map[string]any)
	if !named {
		t.Fatalf("the tool published no results member: %v", served)
	}
	if !reflect.DeepEqual(terminal["hits"], results["hits"]) {
		t.Errorf("the two heads answer the hits differently:\nterminal: %v\ntool:     %v", terminal["hits"], results["hits"])
	}
	hits, _ := terminal["hits"].([]any)
	if len(hits) != 2 {
		t.Fatalf("the fixture answered %d hits, so the comparison ran against almost nothing", len(hits))
	}
}

// searchOverTheProtocol drives one search_cards call against a server bounded
// by the workbench's own tree and returns the decoded payload.
func searchOverTheProtocol(t *testing.T, root string, arguments map[string]any) map[string]any {
	t.Helper()
	dir := soleBenchDir(t, root)
	opened, err := bench.Open(dir)
	if err != nil {
		t.Fatalf("open %q: %v", dir, err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	line, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": "search_cards", "arguments": arguments},
	})
	if err != nil {
		t.Fatalf("marshal the call: %v", err)
	}
	out := &strings.Builder{}
	if err := mcp.Serve(dir, library, map[string]*verb.Library{}, strings.NewReader(string(line)+"\n"), out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var answer struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &answer); err != nil {
		t.Fatalf("search_cards over the protocol: %v (%s)", err, out.String())
	}
	if len(answer.Error) > 0 {
		t.Fatalf("search_cards answered an error: %s", answer.Error)
	}
	if len(answer.Result.Content) != 1 {
		t.Fatalf("search_cards carried %d content members", len(answer.Result.Content))
	}
	decoded, ok := decode(t, answer.Result.Content[0].Text).(map[string]any)
	if !ok {
		t.Fatalf("search_cards answered something that is not an object: %s", answer.Result.Content[0].Text)
	}
	return decoded
}
