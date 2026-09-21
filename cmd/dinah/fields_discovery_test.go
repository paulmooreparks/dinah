package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/mcp"
	"dinah/internal/verb"
)

// TestAKindsFieldSetIsPublishedRatherThanGuessed asserts that a reader who
// names a field the resolved kind does not record is told that kind's own set,
// and that the set arrives on the machine surface as a published vocabulary
// rather than only in a refusal.
//
// The cross-kind negative is what defeats a raise site listing every field of
// every kind everywhere. It does not defeat a wrong bench.FieldsOf, because
// this check's expectation and the raise site's value both come from that one
// call; what pins the content of FieldsOf is the two-directional sample table
// in cmd/dinah/field_sweep_test.go, and the next reader should not read this
// check as covering it.
func TestAKindsFieldSetIsPublishedRatherThanGuessed(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card carrying a checklist"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "file", "fx-1", "open_question", "does the deadline move?"); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}

	subjects := []struct {
		kind    string
		ref     string
		foreign string
	}{
		{kind: bench.KindCard, ref: "fx-1", foreign: bench.KindItem},
		{kind: bench.KindItem, ref: "fx-1/questions/1", foreign: bench.KindCard},
	}
	for _, subject := range subjects {
		refused := runCLI(t, root, "set", subject.ref, "frobnicate", "x")
		if refused.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Fatalf("%s: naming a field it does not record exited %d", subject.kind, refused.code)
		}
		if name := refusalNameOf(refused.errw); name != contract.UnknownField {
			t.Errorf("%s: the refusal name is %s, wanted %s", subject.kind, name, contract.UnknownField)
		}
		own := bench.FieldsOf(subject.kind)
		if len(own) == 0 {
			t.Fatalf("%s records no field, so this check read nothing", subject.kind)
		}
		for _, name := range own {
			if !strings.Contains(refused.errw, name) {
				t.Errorf("%s: the sentence does not name its own field %s:\n%s", subject.kind, name, refused.errw)
			}
		}
		records := map[string]bool{}
		for _, name := range own {
			records[name] = true
		}
		named := 0
		for _, name := range bench.FieldsOf(subject.foreign) {
			if records[name] {
				continue
			}
			named++
			if strings.Contains(refused.errw, name) {
				t.Errorf("%s: the sentence names %s, which belongs to a %s:\n%s", subject.kind, name, subject.foreign, refused.errw)
			}
		}
		if named == 0 {
			t.Errorf("%s and %s record the same fields, so the cross-kind half of this check read nothing", subject.kind, subject.foreign)
		}
	}

	// The third publication. A tool schema is fixed before a reference is
	// known, so what it can declare is the union over every kind, and the
	// refusal above narrows that to the resolved kind.
	//
	// The shape changed at dinah-498, on the operator's ruling of that day.
	// A workbench declares fields of its own, and no key a workbench mints at
	// run time can appear in a list fixed in the source, so the argument
	// stopped publishing an enum: a strict client reading one would have
	// refused to send a declared key before the call left it. The members this
	// package does know travel under the vendor key instead, beside the name
	// of the source a head resolves the rest from, which is the shape
	// internal/mcp/tools.go already used for an argument whose legal answer is
	// not itself one member.
	published, source, enum := setFieldVocabulary(t, root)
	want := bench.AllFields()
	if len(want) == 0 {
		t.Fatal("the union of every kind's fields is empty, so this check read nothing")
	}
	if len(enum) != 0 {
		t.Errorf("the set_field tool publishes the enum %v, and a workbench-declared key can never appear in one", enum)
	}
	if source != setFieldVocabularySource {
		t.Errorf("the set_field tool publishes the vocabulary source %q, wanted %q", source, setFieldVocabularySource)
	}
	if strings.Join(published, " ") != strings.Join(want, " ") {
		t.Errorf("the set_field tool publishes the members\n  %s\nand the union of every kind's fields is\n  %s",
			strings.Join(published, " "), strings.Join(want, " "))
	}
}

// setFieldVocabularySource is the name the set_field tool publishes for the
// set a head resolves, written here so that the schema and the assertion read
// one string rather than two that happen to agree today.
const setFieldVocabularySource = "fields"

// setFieldVocabulary reads what the set_field tool declares for its field
// argument, off a live tools/list answer rather than off the declaration the
// schema is generated from: the members it publishes, the vocabulary source it
// names, and the enum it does not publish.
//
// The enum is read as well as the members, because this test's whole subject
// is which of the two a client meets, and reading only the half that should be
// there would pass against a schema publishing both.
func setFieldVocabulary(t *testing.T, root string) (members []string, source string, enum []string) {
	t.Helper()
	dir := soleBenchDir(t, root)
	opened, err := bench.Open(dir)
	if err != nil {
		t.Fatalf("open %q: %v", dir, err)
	}
	library := verb.New(opened, os.Getenv("DINAH_HOME"))
	out := &strings.Builder{}
	line := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	if err := mcp.Serve(dir, library, map[string]*verb.Library{}, strings.NewReader(line+"\n"), out, mcp.ProfileAll); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var answer struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties map[string]struct {
						Enum    []string `json:"enum"`
						Members []string `json:"x-dinah-vocabulary-members"`
						Source  string   `json:"x-dinah-vocabulary-source"`
					} `json:"properties"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &answer); err != nil {
		t.Fatalf("decode tools/list: %v (%s)", err, out.String())
	}
	for _, tool := range answer.Result.Tools {
		if tool.Name != "set_field" {
			continue
		}
		field, published := tool.InputSchema.Properties["field"]
		if !published {
			t.Fatal("the set_field tool publishes no field argument")
		}
		return field.Members, field.Source, field.Enum
	}
	t.Fatal("this head serves no set_field tool")
	return nil, "", nil
}

// TestABareWorkstreamSlugIsSentToItsPrefixedSpelling asserts what a reader who
// types the form the retired commands took meets instead of it.
//
// `workstream get autumn status` took a bare slug, and the generic get and set
// that replace it take a reference, so the same reader now types a bare slug at
// a command that reads it as a card. No card carries that name, so the refusal
// is unknown-card and stays unknown-card, because no card was found and that is
// what happened. What changes is where it sends the reader: the card listing
// cannot answer a question about a workstream, so the sentence names the
// prefixed spelling and the guide that gives the grammar.
//
// Three arms hold that together. Both commands are exercised, because a reader
// arrives at either. A name that is neither a card nor a workstream keeps the
// card listing, which is what stops the clause rendering on every unknown card.
// And the spelling the refusal names is run, so the advice is checked against
// the tool rather than only read.
func TestABareWorkstreamSlugIsSentToItsPrefixedSpelling(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "workstream", "new", "Autumn release", "--slug", "autumn"); got.code != 0 {
		t.Fatalf("workstream new: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "a card, so the listing the usual next step names is not empty"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}

	for _, typed := range [][]string{
		{"get", "autumn", "status"},
		{"set", "autumn", "status", "finished"},
	} {
		spelling := strings.Join(typed, " ")
		refused := runCLI(t, root, typed...)
		if refused.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Fatalf("`dinah %s` exited %d, wanted the refused exit code", spelling, refused.code)
		}
		if name := refusalNameOf(refused.errw); name != contract.UnknownCard {
			t.Errorf("`dinah %s` refused %s, wanted %s", spelling, name, contract.UnknownCard)
		}
		if !strings.Contains(refused.errw, "workstream/autumn") {
			t.Errorf("`dinah %s` does not name the prefixed spelling:\n%s", spelling, refused.errw)
		}
		if !strings.Contains(refused.errw, "dinah guide references") {
			t.Errorf("`dinah %s` does not name the guide that spells a reference out:\n%s", spelling, refused.errw)
		}
		if strings.Contains(refused.errw, "dinah ls") {
			t.Errorf("`dinah %s` sends the reader to the card listing, which cannot answer a question about a workstream:\n%s", spelling, refused.errw)
		}
	}

	// A name that is neither a card nor a workstream keeps the next step every
	// other unknown-card raise site carries, so the clause above is switched on
	// by the workstream and not by the command.
	stranger := runCLI(t, root, "get", "frobnicate", "title")
	if name := refusalNameOf(stranger.errw); name != contract.UnknownCard {
		t.Fatalf("a name that is neither refused %s, wanted %s", name, contract.UnknownCard)
	}
	if !strings.Contains(stranger.errw, "dinah list cards") {
		t.Errorf("a name that is neither lost the card listing:\n%s", stranger.errw)
	}
	if strings.Contains(stranger.errw, "workstream/") {
		t.Errorf("a name that is neither is offered a workstream spelling:\n%s", stranger.errw)
	}

	// The spelling the refusal names has to work, or the advice is worse than
	// the advice it replaced.
	answered := runCLI(t, root, "get", "workstream/autumn", "status")
	if answered.code != 0 {
		t.Fatalf("`dinah get workstream/autumn status` exited %d: %s", answered.code, answered.errw)
	}
	if got := strings.TrimSpace(answered.out); got == "" {
		t.Error("the spelling the refusal names answered nothing")
	}
}
