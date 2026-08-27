package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/verb"
)

// TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt
// closes the gap dinah-151's Test cycle found. The identity guards in
// internal/verb/tree_test.go (TestEveryReferenceAContentsTreeDrawsResolves,
// TestAWalkRootedAtAReferenceDrawsThatReferenceBack) walk Bench.ResolvePath
// directly, which is the library's own internal resolver. show sits in front
// of that resolver and refuses the workbench root while path and edit both
// accept it, so a guard that never calls show cannot see the refusal, and it
// did not: the gap survived four review cycles and reached Test as a live
// exit-2 refusal instead.
//
// This guard runs show, path and edit themselves, through the same run()
// dispatch main() uses, rather than the resolver underneath them. It does not
// hand-write which command accepts which kind of address either. It reads
// that from internal/guide/guides/references.md's own table, the one place
// the tool already declares which kinds a command takes (dinah-151's own
// spec section 9 calls the guide the documentation of that grammar, and
// TestTheReferencesGuideSaysWhichCommandTakesWhat already holds the guide to
// a fixed literal so the table does not drift silently). So a future
// widening or narrowing of what a command accepts either updates the guide's
// table or reddens here; nothing here carries a second, hand-written copy of
// the exclusion, including show's own workbench exclusion, which is read off
// the table's "no" cell rather than special-cased in this file.
func TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt(t *testing.T) {
	root := newBench(t)
	ref := addCard(t, root, "a card with things below it")
	if got := runCLI(t, root, "comment", ref, "a note"); got.code != 0 {
		t.Fatalf("comment %s: %d %s", ref, got.code, got.errw)
	}
	notes := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(notes, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}
	if got := runCLI(t, root, "attach", ref, notes); got.code != 0 {
		t.Fatalf("attach %s: %d %s", ref, got.code, got.errw)
	}
	if got := runCLI(t, root, "attach", ref+"/comments/1", notes); got.code != 0 {
		t.Fatalf("attach %s/comments/1: %d %s", ref, got.code, got.errw)
	}
	if got := runCLI(t, root, "attach", "workbench", notes); got.code != 0 {
		t.Fatalf("attach workbench: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "attach", "intake", notes); got.code != 0 {
		t.Fatalf("attach intake: %d %s", got.code, got.errw)
	}
	writeChecklistItem(t, root, ref, "AC-1", 1)

	declared := parseReferencesGuideTable(t)

	built := runCLI(t, root, "contents", "workbench", "--depth", "all", "--json")
	if built.code != 0 {
		t.Fatalf("contents: %d %s", built.code, built.errw)
	}
	var tree verb.Tree
	if err := json.Unmarshal([]byte(built.out), &tree); err != nil {
		t.Fatalf("decode contents tree: %v\n%s", err, built.out)
	}

	// The editor is pointed at a name no machine carries, so a successful
	// resolution reaches the launch and fails there instead of opening a
	// window this suite would then wait on. What is asserted for edit is
	// that the failure is not a resolution refusal, mirroring
	// TestPathAndEditReachTheWorkbenchAnchor.
	t.Setenv("DINAH_EDITOR", "dinah-no-such-editor")

	seen := map[string]bool{}
	var walk func(node verb.TreeNode)
	walk = func(node verb.TreeNode) {
		if node.Ref != "" {
			col := column(node.Kind)
			seen[col] = true
			for _, cmd := range []string{"show", "path", "edit"} {
				accepts, ok := declared[cmd][col]
				if !ok {
					t.Fatalf("the references guide's table declares nothing for %s against %q", cmd, col)
				}
				got := runCLI(t, root, cmd, node.Ref)
				refused := resolutionRefused(got.errw)
				switch {
				case accepts && cmd != "edit" && got.code != 0:
					t.Errorf("references.md declares %s accepts %s (%s), but `dinah %s %s` refused: %d %s", cmd, col, node.Ref, cmd, node.Ref, got.code, got.errw)
				case accepts && cmd == "edit" && refused:
					t.Errorf("references.md declares edit accepts %s (%s), but `dinah edit %s` refused resolution: %s", col, node.Ref, node.Ref, got.errw)
				case !accepts && cmd != "edit" && got.code == 0:
					t.Errorf("references.md declares %s does not accept %s (%s), but `dinah %s %s` exited 0", cmd, col, node.Ref, cmd, node.Ref)
				case !accepts && cmd == "edit" && !refused:
					t.Errorf("references.md declares edit does not accept %s (%s), but `dinah edit %s` did not refuse resolution: %d %s", col, node.Ref, node.Ref, got.code, got.errw)
				}
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(tree.Root)

	// The four columns the table declares, named rather than counted: a
	// fixture that happens to miss one proves nothing about that column, the
	// same reasoning TestAWalkRootedAtAReferenceDrawsThatReferenceBack gives
	// for naming kinds rather than counting them.
	for _, want := range []string{"This workbench", "A column", "A card", "Below a card"} {
		if !seen[want] {
			t.Fatalf("the walk drew no node in the %q column, so this guard proves nothing about it", want)
		}
	}
}

// column classifies a contents-tree node's kind against the references
// guide's own table columns. The three named kinds each get their own
// column; every other kind (comment, item, attachment, and any extension
// kind the containment table declares later) hangs off a card, which is the
// fourth column.
func column(kind string) string {
	switch kind {
	case bench.KindWorkbench:
		return "This workbench"
	case bench.KindColumn:
		return "A column"
	case bench.KindCard:
		return "A card"
	default:
		return "Below a card"
	}
}

// resolutionRefused reports whether a command's stderr names one of the two
// refusals a reference resolver raises when the address itself does not
// resolve. Both appear depending on which resolver a command walks: show's
// own head-only branch raises UnknownCard, and ResolvePath raises
// UnknownPath, which is what path, edit and every composed reference go
// through. A failure carrying neither name reached resolution and failed
// downstream of it instead, which for edit is the launch this test forces.
func resolutionRefused(errw string) bool {
	leading := strings.SplitN(strings.TrimSpace(errw), " ", 2)[0]
	return leading == contract.UnknownCard || leading == contract.UnknownPath
}

// parseReferencesGuideTable reads the "Which command takes what" table out
// of the shipped references guide and returns, for each command the table
// names, whether it accepts each of the table's four columns. This is the
// declaration TestEveryAddressTheContentsTreeDrawsResolvesThroughTheCommandsThatDeclareIt
// reads rather than hand-writes: the guide is the one place the tool already
// says which kinds a command takes, and a row this parser cannot find is a
// row the guide no longer carries the way this test expects, which fails
// loudly rather than silently reading zero rows.
func parseReferencesGuideTable(t *testing.T) map[string]map[string]bool {
	t.Helper()
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	lines := strings.Split(text, "\n")
	headerAt := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "| Command") {
			headerAt = i
			break
		}
	}
	if headerAt < 0 {
		t.Fatal("the references guide carries no \"| Command\" table header; this parser and the guide have drifted apart")
	}
	columns := cells(lines[headerAt])[1:]
	declared := map[string]map[string]bool{}
	for _, line := range lines[headerAt+2:] {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		row := cells(line)
		if len(row) != len(columns)+1 {
			t.Fatalf("the references guide's row %q carries %d cells, wanted %d", trimmed, len(row), len(columns)+1)
		}
		accepts := map[string]bool{}
		for i, col := range columns {
			switch row[i+1] {
			case "yes":
				accepts[col] = true
			case "no":
				accepts[col] = false
			default:
				t.Fatalf("the references guide's row for %q names %q for %q, wanted yes or no", row[0], row[i+1], col)
			}
		}
		declared[row[0]] = accepts
	}
	for _, want := range []string{"show", "path", "edit"} {
		if _, ok := declared[want]; !ok {
			t.Fatalf("the references guide's table carries no row for %q", want)
		}
	}
	return declared
}

// cells splits one Markdown table row into its trimmed cells, dropping the
// empty leading and trailing cells the leading and trailing "|" produce.
func cells(line string) []string {
	parts := strings.Split(line, "|")
	var out []string
	for _, part := range parts[1 : len(parts)-1] {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

// writeChecklistItem hand-writes a checklist item onto a card's own anchor,
// mirroring internal/verb/tree_test.go's writeItem: no verb in the declared
// surface creates one, so the fixture writes the frontmatter directly.
func writeChecklistItem(t *testing.T, root, ref, text string, ordinal int) {
	t.Helper()
	id := cardID(t, root, ref)
	cardDir := filepath.Join(benchDir(t, root), bench.CardsDir, id)
	itemID := "c000000000" + strconv.Itoa(10+ordinal)
	fm := bench.NewFrontmatter()
	fm.Set("kind", "acceptance_criterion")
	fm.Set("title", text)
	fm.Set(bench.OrdinalField, strconv.Itoa(ordinal))
	path := filepath.Join(cardDir, bench.ChecklistDir, itemID, bench.ItemAnchor)
	if err := bench.WriteText(path, fm.Render("")); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
