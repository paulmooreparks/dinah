package lsp

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
)

// errNoWorkbench is what a discovery stub answers when it is standing in for
// a directory under which nothing resolves.
var errNoWorkbench = errors.New("no workbench resolves here")

// completionCase is one front-matter position completion is asked at, with
// the anchor it stands in and what the answer has to be a list of.
type completionCase struct {
	// name is how the failure message spells the position.
	name string
	// anchor is the file the position stands in, which decides whether the
	// document is read as a schema at all.
	anchor string
	// text is the anchor's own bytes.
	text string
	// line and character are where the cursor sits.
	line, character int
	// wants says which set of candidates the answer must be drawn from.
	wants slotKind
}

// TestCompletionFiresAtTheEightDeclaredPositionsAndNowhereElse asserts
// dinah-515 criterion 25: the completion position set is exactly the eight
// rows of the section 4.2 table, each answering the kind that table gives it,
// and a key the table declares not to be a reference position answers an
// empty list rather than an error.
func TestCompletionFiresAtTheEightDeclaredPositionsAndNowhereElse(t *testing.T) {
	f := build(t)
	h, _ := f.serve(t)
	h.initialize(nil)

	workbench := filepath.Join(f.bench.Root, bench.WorkbenchAnchor)
	card := f.cardAnchor(f.card)
	column := f.columnAnchor(f.bench.Columns[0].ID)
	item := filepath.Join(f.bench.CardsRoot(), f.card, "checklist")

	cases := []completionCase{
		{
			name: "workbench.md columns[]", anchor: workbench,
			text: "---\ntitle: A workbench\ncolumns:\n  - \n---\n",
			line: 3, character: 4, wants: slotColumn,
		},
		{
			name: "workbench.md groups.<name>[]", anchor: workbench,
			text: "---\ntitle: A workbench\ngroups:\n  left:\n    - \n---\n",
			line: 4, character: 6, wants: slotColumn,
		},
		{
			name: "card.md column", anchor: card,
			text: "---\ntitle: A card\ncolumn: \n---\n",
			line: 2, character: 8, wants: slotColumn,
		},
		{
			name: "card.md workstreams[]", anchor: card,
			text: "---\ntitle: A card\nworkstreams:\n  - \n---\n",
			line: 3, character: 4, wants: slotWorkstream,
		},
		{
			name: "card.md links[].to", anchor: card,
			text: "---\ntitle: A card\nlinks:\n  - kind: relates_to\n    to: \n---\n",
			line: 4, character: 8, wants: slotCard,
		},
		{
			name: "card.md tier_at[].column", anchor: card,
			text: "---\ntitle: A card\ntier_at:\n  - column: \n    tier: frontier\n---\n",
			line: 3, character: 12, wants: slotColumn,
		},
		{
			name: "column.md reject_to", anchor: column,
			text: "---\ntitle: A column\nkind: work\nreject_to: \n---\n",
			line: 3, character: 11, wants: slotColumn,
		},
		{
			name: "item.md column", anchor: filepath.Join(item, "aaaaaaaaaaaa", bench.ItemAnchor),
			text: "---\nkind: open_question\nstate: pending\ncolumn: \n---\n",
			line: 3, character: 8, wants: slotColumn,
		},
	}

	swept := 0
	for _, row := range cases {
		uri := h.openText(row.anchor, row.text)
		list := decode[completionList](t, h.send(methodCompletion, map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": row.line, "character": row.character},
		}))
		if len(list.Items) == 0 {
			t.Errorf("%s answered no candidates", row.name)
			continue
		}
		if !drawnFrom(t, f, list, row.wants) {
			t.Errorf("%s answered candidates of the wrong kind: %+v", row.name, list.Items[0])
		}
		swept++
	}
	if swept != len(cases) {
		t.Errorf("swept %d positions, wanted the eight the section 4.2 table declares", swept)
	}
	if len(cases) != 8 {
		t.Errorf("the fixture names %d positions, and the table declares eight", len(cases))
	}

	quiet := []completionCase{
		{name: "card.md title", anchor: card, text: "---\ntitle: A card\n---\n", line: 1, character: 8},
		{name: "card.md claim_holder", anchor: card, text: "---\ntitle: A card\nclaim_holder: alka\n---\n", line: 2, character: 16},
		{name: "a field_values member", anchor: card, text: "---\ntitle: A card\nfield_values:\n  git.branch: x\n---\n", line: 3, character: 14},
		{name: "a position in prose", anchor: filepath.Join(f.bench.Root, "notes.md"), text: "Some prose about a card.\n", line: 0, character: 10},
	}
	silent := 0
	for _, row := range quiet {
		uri := h.openText(row.anchor, row.text)
		list := decode[completionList](t, h.send(methodCompletion, map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"position":     map[string]any{"line": row.line, "character": row.character},
		}))
		if len(list.Items) != 0 {
			t.Errorf("%s answered %d candidates, wanted an empty list", row.name, len(list.Items))
		}
		silent++
	}
	if silent != len(quiet) {
		t.Errorf("swept %d non-reference positions, wanted %d", silent, len(quiet))
	}
	if len(quiet) != 4 {
		t.Errorf("the quiet half names %d positions, and it is written to carry four", len(quiet))
	}
}

// drawnFrom reports whether a completion list is drawn from the set a
// position's declared kind names, by matching what each candidate inserts
// against the identifiers of that set.
func drawnFrom(t *testing.T, f *fixture, list completionList, kind slotKind) bool {
	t.Helper()
	held := map[string]bool{}
	switch kind {
	case slotColumn:
		for _, column := range f.bench.Columns {
			held[column.ID] = true
		}
	case slotWorkstream:
		workstreams, err := f.bench.Workstreams()
		if err != nil {
			t.Fatalf("workstreams: %v", err)
		}
		for _, workstream := range workstreams {
			held[workstream.ID] = true
		}
	case slotCard:
		for _, id := range mustCards(t, f) {
			held[id] = true
		}
	}
	for _, item := range list.Items {
		if !held[item.InsertText] {
			return false
		}
	}
	return true
}

// TestEveryMessageToAPersonComesFromTheCatalogue asserts dinah-515 criterion
// 27: the five cases section 2.1 enumerates each carry the English of their
// declared key, and no call site in the package hands a Go string literal to
// the log or the message channel.
func TestEveryMessageToAPersonComesFromTheCatalogue(t *testing.T) {
	observed := map[string]bool{}

	// Two of the five stand at initialize: the resolved workbench, and the
	// line a client declaring no inlay-hint refresh support is told.
	f := build(t)
	h, tick := f.serve(t)
	h.initialize(clientDeclaring(false))
	for _, line := range h.await(methodLogMessage, 2) {
		observed[decode[logMessageParams](t, line.Params).Message] = true
	}

	// Two more are the edges of the slow-walk state.
	tick.takes(0)
	tick.step(t)
	tick.takes(30 * time.Second)
	tick.step(t)
	tick.takes(0)
	tick.step(t)
	for _, line := range h.await(methodLogMessage, 4) {
		observed[decode[logMessageParams](t, line.Params).Message] = true
	}

	// The fifth is the show-message a server that resolved no workbench
	// sends, which needs its own process.
	empty := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(empty, "home"))
	quiet := start(t, Options{Wd: empty, Sleep: newTicker().sleep, Discover: func(string) (string, error) { return "", errNoWorkbench }})
	quiet.initialize(nil)
	for _, line := range quiet.await(methodShowMessage, 1) {
		observed[decode[logMessageParams](t, line.Params).Message] = true
	}

	declared := []string{keyLogWorkbench, keyLogNoRefreshSupport, keyLogSlowWalkEntered, keyLogSlowWalkLeft, keyNoWorkbench}
	seen := 0
	for _, key := range declared {
		english := h.server.messages.T(key)
		matched := false
		for message := range observed {
			// The two slow-walk lines splice a duration, so the comparison is
			// against the key's own text with its placeholders removed.
			if message == english || strings.HasPrefix(message, stem(english)) {
				matched = true
			}
		}
		if !matched {
			t.Errorf("no message the server sent carries the English of %s", key)
			continue
		}
		seen++
	}
	if seen != 5 {
		t.Errorf("observed %d of the five messages this server sends a person, wanted five", seen)
	}
}

// stem is a catalogue entry's text up to its first placeholder, which is what
// a message carrying a spliced value still begins with.
func stem(text string) string {
	if cut := strings.Index(text, "{"); cut > 0 {
		return text[:cut]
	}
	return text
}

// messageMember answers the Message member of a window/logMessage or
// window/showMessage payload written as a composite literal, and whether the
// literal carried one at all.
func messageMember(literal *ast.CompositeLit) (ast.Expr, bool) {
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := pair.Key.(*ast.Ident); ok && key.Name == "Message" {
			return pair.Value, true
		}
	}
	return nil, false
}

// fromCatalogue reports whether an expression is a call to the catalogue
// renderer, which is the only composer a sentence bound for a person may have.
// A string literal fails it, and so does every other expression, because the
// question the guard asks is where the sentence came from rather than which
// shapes of hard-coded text somebody has thought to enumerate.
func fromCatalogue(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "T"
}

// payloadLiteral answers the logMessageParams composite literal an expression
// is, and whether it is one.
func payloadLiteral(expr ast.Expr) (*ast.CompositeLit, bool) {
	literal, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	name, ok := literal.Type.(*ast.Ident)
	if !ok || name.Name != "logMessageParams" {
		return nil, false
	}
	return literal, true
}

// TestNoStringLiteralReachesTheEditorsOwnChannels asserts the second half of
// dinah-515 criterion 27: every argument that becomes the Message member of a
// log line or a shown message is composed by the catalogue renderer, and none
// is a Go string literal written at the call site.
//
// The scan reads the channel rather than the helpers. Server.log and
// Server.show are the only route into window/logMessage and
// window/showMessage today, so a scan of those two agrees with the code it
// reads whatever else the package grows: a notification composed anywhere
// else passes it untouched. So the first two halves below sweep every
// notify call carrying one of the two methods and every construction of the
// payload type, and the third keeps the helper scan, each half reporting the
// size of the set it swept.
func TestNoStringLiteralReachesTheEditorsOwnChannels(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package's sources: %v", err)
	}
	fileset := token.NewFileSet()
	channels, payloads, helpers := 0, 0, 0
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileset, source, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", source, err)
		}
		where := func(node ast.Node) string {
			return source + ":" + strconv.Itoa(fileset.Position(node.Pos()).Line)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			// The payload type, wherever it is built. A notification handed a
			// payload assembled into a variable first reaches the channel by a
			// route the call-site scan cannot read, and this half reads it.
			if literal, ok := node.(*ast.CompositeLit); ok {
				if payload, ok := payloadLiteral(literal); ok {
					payloads++
					message, carried := messageMember(payload)
					switch {
					case !carried:
						t.Errorf("%s builds a message payload carrying no Message member, so it would show a person an empty line", where(payload))
					case !fromCatalogue(message):
						t.Errorf("%s builds a message payload whose Message member is composed at the call site rather than by the catalogue", where(payload))
					}
				}
			}

			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			// The channel itself: every notification sent under either of the
			// two methods a person reads.
			if selector.Sel.Name == "notify" && len(call.Args) == 2 {
				method, ok := call.Args[0].(*ast.Ident)
				if ok && (method.Name == "methodLogMessage" || method.Name == "methodShowMessage") {
					channels++
					payload, ok := payloadLiteral(call.Args[1])
					if !ok {
						t.Errorf("%s sends %s a payload this scan cannot read, so nothing here can tell where its sentence came from", where(call), method.Name)
						return true
					}
					message, carried := messageMember(payload)
					switch {
					case !carried:
						t.Errorf("%s sends %s a payload carrying no Message member", where(call), method.Name)
					case !fromCatalogue(message):
						t.Errorf("%s sends %s a sentence composed at the call site rather than drawn from the catalogue", where(call), method.Name)
					}
				}
				return true
			}

			// The helpers, which take a catalogue key rather than a sentence.
			if selector.Sel.Name != "log" && selector.Sel.Name != "show" {
				return true
			}
			helpers++
			if len(call.Args) < 2 {
				t.Errorf("%s calls %s with %d arguments, and it takes a level and a key",
					where(call), selector.Sel.Name, len(call.Args))
				return true
			}
			if _, literal := call.Args[1].(*ast.BasicLit); literal {
				t.Errorf("%s hands %s a string literal where a catalogue key belongs", where(call), selector.Sel.Name)
			}
			return true
		})
	}
	if channels != 2 {
		t.Errorf("the scan read %d notifications bound for a person's own channels, and this server sends them on two", channels)
	}
	if payloads != 2 {
		t.Errorf("the scan read %d constructions of the message payload, and this server builds two", payloads)
	}
	if helpers != 5 {
		t.Errorf("the scan inspected %d helper call sites, and this server sends a person five messages", helpers)
	}
}
