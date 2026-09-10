package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/mcp"
	"dinah/internal/verb"
)

// fieldSweepDefinition declares both level axes and a column carrying a tier
// default, so every guarded field in the sample table below has a legal value
// to be written with.
const fieldSweepDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Sweeping",
  "levels": { "severity": ["trivial", "minor", "major"], "priority": ["later", "soon", "now"], "tier": ["workhorse", "frontier", "apex"] },
  "columns": [
    { "id": "e00000000001", "title": "Intake", "kind": "intake" },
    { "id": "e00000000002", "title": "Doing", "kind": "work", "tier": "workhorse" },
    { "id": "e00000000003", "title": "Finished", "kind": "done" }
  ]
}`

// sweepColumn is the column every column-shaped pair is written against,
// addressed by its identifier so that a slug change does not move it.
const sweepColumn = "e00000000002"

// fieldSample is one field's two legal values, and whatever else a write of it
// has to carry.
//
// There are two values rather than one for a reason worth keeping in view: a
// field whose stored value already equals the sample would pass a write that
// did nothing at all, so the second write is what proves the value moved.
type fieldSample struct {
	// first and second are two values the field's own guard admits.
	first, second string
	// extra rides on every write, which is where a slug's confirmation and a
	// state's note go.
	extra []string
	// secondActor names who makes the second write, empty where the first
	// actor makes both. The operator field is the one that needs it: the
	// first write hands the workbench over, so the second is made by whoever
	// it was handed to.
	secondActor string
}

// fieldSamples is the hand table this sweep drives, keyed by kind and field.
//
// It is written by a person rather than derived, and that is the whole of its
// value. A table generated from bench.FieldsOf and then compared against
// bench.FieldsOf would agree with itself on every input including a wrong one,
// where this one has to be maintained: a field added to a kind fails the run
// naming the missing sample, and an entry naming a field no kind records fails
// it from the other side.
var fieldSamples = map[string]map[string]fieldSample{
	bench.KindWorkbench: {
		"title":        {first: "A workbench", second: "A renamed workbench"},
		"slug":         {first: "wba", second: "wbb", extra: []string{"--yes"}},
		"operator":     {first: "bo", second: "cy", secondActor: "bo"},
		"instructions": {first: "First standing text.", second: "Second standing text.\n\nWith a second paragraph."},
	},
	bench.KindColumn: {
		"title":        {first: "Doing", second: "Doing, renamed"},
		"slug":         {first: "doing-a", second: "doing-b", extra: []string{"--yes"}},
		"kind":         {first: "work", second: "done"},
		"tier":         {first: "workhorse", second: "frontier"},
		"capacity":     {first: "2", second: "3"},
		"instructions": {first: "First station text.", second: "Second station text.\n\nWith a second paragraph."},
	},
	bench.KindCard: {
		"title":    {first: "A card", second: "A renamed card"},
		"body":     {first: "First body.", second: "Second body.\n\nWith a second paragraph."},
		"severity": {first: "minor", second: "major"},
		"priority": {first: "later", second: "now"},
		"tier":     {first: "workhorse", second: "frontier"},
	},
	bench.KindComment: {
		"body": {first: "First thought.", second: "Second thought.\n\nWith a second paragraph."},
	},
	bench.KindItem: {
		"text":   {first: "First question.", second: "Second question.\n\nWith a second paragraph."},
		"state":  {first: "resolved", second: "pending", extra: []string{"--note", "the operator settled it"}},
		"note":   {first: "a first note", second: "a second note"},
		"owner":  {first: "operator", second: "holder"},
		"column": {first: "intake", second: "done"},
	},
	bench.KindAttachment: {
		"filename":    {first: "first.txt", second: "second.txt"},
		"description": {first: "the first description", second: "the second description"},
	},
	bench.KindWorkstream: {
		"title":  {first: "Autumn release", second: "Autumn release, renamed"},
		"slug":   {first: "autumn-a", second: "autumn-b", extra: []string{"--yes"}},
		"status": {first: "paused", second: "finished"},
		"notes":  {first: "First notes.", second: "Second notes.\n\nWith a second paragraph."},
	},
}

// TestEveryFieldOfEveryKindRoundTripsAtBothHeads runs every field of every
// kind through a write and a read, twice, at a terminal and over the protocol,
// and holds the sample table and bench.FieldsOf to each other in both
// directions.
//
// The two-directional pin is what stops this being a check that recomputes its
// own expectation. The sweep's subject set comes from FieldsOf, and the hand
// table is forced to be maintained because a missing sample is a failure
// rather than a skip. Each pair runs against a workbench of its own, so a
// write that moves a reference, which a slug change on three of the kinds
// does, cannot reach the pair that runs after it.
func TestEveryFieldOfEveryKindRoundTripsAtBothHeads(t *testing.T) {
	kinds := bench.EntityKinds()
	if len(kinds) == 0 {
		t.Fatal("the grammar names no kind, so this sweep read nothing")
	}
	pairs := 0
	for _, kind := range kinds {
		samples, sampled := fieldSamples[kind]
		if !sampled {
			t.Errorf("the grammar names the kind %s and the sample table carries no entry for it", kind)
			continue
		}
		records := map[string]bool{}
		for _, field := range bench.FieldsOf(kind) {
			records[field] = true
			sample, named := samples[field]
			if !named {
				t.Errorf("a %s records the field %s and the sample table names no value for it", kind, field)
				continue
			}
			pairs++
			t.Run("terminal: "+kind+"/"+field, func(t *testing.T) {
				roundTripAtTerminal(t, kind, field, sample)
			})
			t.Run("protocol: "+kind+"/"+field, func(t *testing.T) {
				roundTripOverTheProtocol(t, kind, field, sample)
			})
		}
		for field := range samples {
			if !records[field] {
				t.Errorf("the sample table names %s/%s and a %s does not record that field", kind, field, kind)
			}
		}
	}
	if pairs == 0 {
		t.Fatal("the sweep ran no pair, so it proved nothing about either head")
	}
	t.Logf("the sweep ran %d field pairs at each of the two heads", pairs)
}

// roundTripAtTerminal writes each of a field's two values through the command
// and reads it back through the command.
func roundTripAtTerminal(t *testing.T, kind, field string, sample fieldSample) {
	root := newBenchFromDefinition(t, fieldSweepDefinition)
	ref := fileSweepSubject(t, root, kind)
	for at, value := range []string{sample.first, sample.second} {
		argv := append([]string{"set", ref, field, value}, sample.extra...)
		if at == 1 && sample.secondActor != "" {
			argv = append(argv, "--actor", sample.secondActor)
		}
		if written := runCLI(t, root, argv...); written.code != 0 {
			t.Fatalf("write %d of %s/%s: %d %s", at+1, kind, field, written.code, written.errw)
		}
		read := runCLI(t, root, "get", ref, field)
		if read.code != 0 {
			t.Fatalf("read %d of %s/%s: %d %s", at+1, kind, field, read.code, read.errw)
		}
		if got := strings.TrimSuffix(read.out, "\n"); got != value {
			t.Errorf("%s/%s read back %q after write %d, wanted %q", kind, field, got, at+1, value)
		}
	}
}

// roundTripOverTheProtocol runs the same pair through set_field and get_field,
// so what is true at a terminal is shown to be true for an agent rather than
// assumed.
func roundTripOverTheProtocol(t *testing.T, kind, field string, sample fieldSample) {
	root := newBenchFromDefinition(t, fieldSweepDefinition)
	ref := fileSweepSubject(t, root, kind)
	dir := soleBenchDir(t, root)
	for at, value := range []string{sample.first, sample.second} {
		actor := os.Getenv("DINAH_ACTOR")
		if at == 1 && sample.secondActor != "" {
			actor = sample.secondActor
		}
		arguments := map[string]any{"actor": actor, "ref": ref, "field": field, "value": value}
		for i := 0; i < len(sample.extra); i++ {
			switch sample.extra[i] {
			case "--yes":
				arguments["yes"] = true
			case "--note":
				arguments["note"] = sample.extra[i+1]
				i++
			}
		}
		written := sweepCall(t, dir, "set_field", arguments)
		if outcome, _ := written["outcome"].(string); outcome != "ok" {
			t.Fatalf("write %d of %s/%s over the protocol: %v", at+1, kind, field, written)
		}
		read := sweepCall(t, dir, "get_field", map[string]any{
			"actor": actor, "ref": ref, "field": field,
		})
		got, carried := read["value"].(string)
		if !carried {
			t.Fatalf("read %d of %s/%s over the protocol carried no value: %v", at+1, kind, field, read)
		}
		if got != value {
			t.Errorf("%s/%s read back %q over the protocol after write %d, wanted %q", kind, field, got, at+1, value)
		}
	}
}

// fileSweepSubject files one entity of a kind in a fresh workbench and returns
// the reference the sweep addresses it by.
//
// Every reference is one a write cannot move. A column and a workstream are
// addressed by their identifiers rather than their slugs, because three of the
// kinds carry a settable slug and one of them is the workbench, whose slug is
// the prefix every card reference is composed against.
func fileSweepSubject(t *testing.T, root, kind string) string {
	t.Helper()
	switch kind {
	case bench.KindWorkbench:
		return bench.WorkbenchRef
	case bench.KindColumn:
		return sweepColumn
	case bench.KindAttachment:
		// Below the workbench rather than below a card, which is the half of
		// the attachment's own journal rule that lands on the workbench.
		return attachedTo(t, root, bench.WorkbenchRef) + "/attachments/1"
	case bench.KindWorkstream:
		made := runCLI(t, root, "--json", "workstream", "new", "Autumn release")
		if made.code != 0 {
			t.Fatalf("workstream new: %d %s", made.code, made.errw)
		}
		var answer struct {
			Workstream struct {
				ID string `json:"id"`
			} `json:"workstream"`
		}
		if err := json.Unmarshal([]byte(made.out), &answer); err != nil {
			t.Fatalf("decode the created workstream: %v\n%s", err, made.out)
		}
		return "workstream/" + answer.Workstream.ID
	}
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	switch kind {
	case bench.KindCard:
		return "fx-1"
	case bench.KindComment:
		if got := runCLI(t, root, "comment", "fx-1", "First thought."); got.code != 0 {
			t.Fatalf("comment: %d %s", got.code, got.errw)
		}
		return "fx-1/comments/1"
	case bench.KindItem:
		if got := runCLI(t, root, "file", "fx-1", "open_question", "First question."); got.code != 0 {
			t.Fatalf("file: %d %s", got.code, got.errw)
		}
		return "fx-1/questions/1"
	}
	t.Fatalf("the grammar names the kind %s and this sweep does not know how to file one", kind)
	return ""
}

// attachedTo hangs one file below a reference and returns that reference, so
// the caller composes the attachment's own address from it.
func attachedTo(t *testing.T, root, ref string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "first.txt")
	if err := os.WriteFile(source, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write the attachment source: %v", err)
	}
	if got := runCLI(t, root, "attach", ref, source); got.code != 0 {
		t.Fatalf("attach below %s: %d %s", ref, got.code, got.errw)
	}
	return ref
}

// sweepCall runs one tool call through the machine head and returns its
// payload.
func sweepCall(t *testing.T, dir, tool string, arguments map[string]any) map[string]any {
	t.Helper()
	opened, err := bench.Open(dir)
	if err != nil {
		t.Fatalf("open %q: %v", dir, err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	line, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": arguments},
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
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &answer); err != nil {
		t.Fatalf("decode the %s answer: %v (%s)", tool, err, out.String())
	}
	if answer.Error.Message != "" {
		t.Fatalf("the %s call answered an error: %s", tool, answer.Error.Message)
	}
	if len(answer.Result.Content) != 1 {
		t.Fatalf("the %s call carried %d content members", tool, len(answer.Result.Content))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(answer.Result.Content[0].Text), &payload); err != nil {
		t.Fatalf("decode the %s payload: %v", tool, err)
	}
	return payload
}
