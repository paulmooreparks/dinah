package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestTheMachineHeadAnswersACollectionAsTheTerminalDoes is the other head's
// half of the accept-and-refuse split, and it is what catches a fix landed in
// cmd/dinah rather than in the library, which would leave this head answering
// dinah.unknown-path for every collection reference.
//
// The subject set is derived at the commit under test as the reference-taking
// roster minus the commands this head deliberately does not serve, so a
// command joining or leaving either list moves the set rather than leaving a
// stale literal here. The counts are asserted for the reason the terminal
// sweep asserts its own: a run that reached no tool reports 0 and 0.
func TestTheMachineHeadAnswersACollectionAsTheTerminalDoes(t *testing.T) {
	library := newLibrary(t)
	if response := library.Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: "fx-1", Text: "the first thought"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %s %s", response.Outcome, response.Refusal)
	}
	if response := library.Comment(&verb.Request{Verb: "comment", Actor: "alka", Card: "fx-1", Text: "the second thought"}); response.Outcome != contract.OutcomeOK {
		t.Fatalf("comment: %s %s", response.Outcome, response.Refusal)
	}
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write the attachment source: %v", err)
	}

	// The served subset is read off this head's own tool list, keyed by the
	// library command each tool runs, so a tool renamed or a command newly
	// exempted moves the set rather than leaving a stale literal here.
	roster := map[string]bool{}
	for _, command := range verb.ReferenceTakingCommands() {
		roster[command] = true
	}
	served := map[string]string{}
	for _, published := range tools {
		if roster[published.command] {
			served[published.command] = published.name
		}
	}
	names := make([]string, 0, len(served))
	for command := range served {
		names = append(names, command)
	}
	sort.Strings(names)
	if len(names) != 15 {
		t.Fatalf("this head serves %d of the seventeen reference-taking commands and it serves fifteen: %s", len(names), strings.Join(names, " "))
	}
	for _, held := range []string{"path", "edit"} {
		if _, exempt := toolExemptions[held]; !exempt {
			t.Errorf("%s is served over this head, and the two the roster loses here are path and edit", held)
		}
		if _, published := served[held]; published {
			t.Errorf("%s is published as a tool, and the two the roster loses here are path and edit", held)
		}
	}

	// Each tool takes the collection reference under whatever argument its own
	// parameter list names, plus whatever else it requires to get as far as
	// resolving that reference.
	arguments := map[string]string{
		"show":         `{"actor":"alka","card":"fx-1/comments"}`,
		"contents":     `{"actor":"alka","ref":"fx-1/comments","depth":"all"}`,
		"attachments":  `{"actor":"alka","ref":"fx-1/comments"}`,
		"instructions": `{"actor":"alka","card":"fx-1/comments"}`,
		"archive":      `{"actor":"alka","ref":"fx-1/comments"}`,
		"delete":       `{"actor":"alka","ref":"fx-1/comments","yes":true}`,
		"rename":       `{"actor":"alka","ref":"fx-1/comments","name":"renamed.txt"}`,
		"attach":       fmt.Sprintf(`{"actor":"alka","ref":"fx-1/comments","file":%q}`, filepath.ToSlash(source)),
		"cite":         `{"actor":"alka","item":"fx-1/comments","scheme":"attachment","target":"1"}`,
		"resolve":      `{"actor":"alka","item":"fx-1/comments","note":"a note"}`,
		"verify":       `{"actor":"alka","item":"fx-1/comments","note":"a note"}`,
		"fail":         `{"actor":"alka","item":"fx-1/comments","note":"a note"}`,
		"reopen":       `{"actor":"alka","item":"fx-1/comments","reason":"a reason"}`,
		"get":          `{"actor":"alka","ref":"fx-1/comments","field":"body"}`,
		"set":          `{"actor":"alka","ref":"fx-1/comments","field":"body","value":"rewritten"}`,
	}
	if len(arguments) != len(names) {
		t.Fatalf("the sweep names %d tools and the served subset holds %d", len(arguments), len(names))
	}

	answered, refused := 0, 0
	for _, command := range names {
		body, named := arguments[command]
		if !named {
			t.Fatalf("the sweep has no invocation for the served command %s", command)
		}
		line := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, served[command], body)
		answer := payload(t, ask(t, library, line))
		if answer["outcome"] == contract.OutcomeRefused {
			if answer["refusal"] != contract.IsACollection {
				t.Errorf("%s refused a collection with %v rather than %s", command, answer["refusal"], contract.IsACollection)
				continue
			}
			refused++
			continue
		}
		switch command {
		case "show":
			listing, carried := answer["collection"].(map[string]any)
			if !carried {
				t.Errorf("the show tool answered a collection reference without a collection member: %v", answer)
				continue
			}
			members, ok := listing["members"].([]any)
			if !ok || len(members) != 2 {
				t.Errorf("the show tool carried %v members and the card carries two", listing["members"])
				continue
			}
			refs := make([]string, 0, len(members))
			for _, member := range members {
				refs = append(refs, fmt.Sprint(member.(map[string]any)["ref"]))
			}
			if strings.Join(refs, " ") != "fx-1/comments/1 fx-1/comments/2" {
				t.Errorf("the show tool carries the members in the order [%s], and creation order is the order", strings.Join(refs, " "))
			}
		case "contents":
			tree, carried := answer["tree"].(map[string]any)
			if !carried {
				encoded, _ := json.Marshal(answer)
				t.Errorf("the contents tool answered no tree: %s", encoded)
				continue
			}
			root, drawn := tree["root"].(map[string]any)
			if !drawn {
				encoded, _ := json.Marshal(answer)
				t.Errorf("the contents tool answered a tree with no root: %s", encoded)
				continue
			}
			if root["kind"] != verb.KindCollection {
				t.Errorf("the contents tool calls the root %v rather than %q", root["kind"], verb.KindCollection)
			}
		case "attachments":
			listing, carried := answer["attachments"].(map[string]any)
			if !carried {
				encoded, _ := json.Marshal(answer)
				t.Errorf("the attachments tool answered no listing: %s", encoded)
				continue
			}
			if listing["kind"] != verb.KindCollection {
				t.Errorf("the attachments tool calls the listing %v rather than %q", listing["kind"], verb.KindCollection)
			}
		default:
			encoded, _ := json.Marshal(answer)
			t.Errorf("%s answered a collection reference rather than refusing it: %s", command, encoded)
			continue
		}
		answered++
	}
	t.Logf("fifteen tools called: %d answered, %d refused with %s", answered, refused, contract.IsACollection)
	if answered != 3 || refused != 12 {
		t.Fatalf("the sweep answered %d and refused %d, and the split is three and twelve", answered, refused)
	}
}
