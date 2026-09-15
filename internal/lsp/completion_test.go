package lsp

import (
	"errors"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
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

// connNotify, serverLog, serverShow and catalogueT are the four methods this
// scan cares about, written as go/types spells a method's full name. Naming
// them by what they are rather than by how a call site spells them is the
// whole point of type-checking the package first.
const (
	connNotify = "(*dinah/internal/lsp.conn).notify"
	serverLog  = "(*dinah/internal/lsp.Server).log"
	serverShow = "(*dinah/internal/lsp.Server).show"
	catalogueT = "(*dinah/internal/msg.Renderer).T"
)

// scanTypes parses and type-checks this package's own non-test sources, and
// answers the file set, the files, the checked package and the recorded types
// and selections. A scan that reads the syntax alone can only compare
// spellings, and a guard that compares spellings guards one spelling.
func scanTypes(t *testing.T) (*token.FileSet, []*ast.File, *types.Package, *types.Info) {
	t.Helper()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package's sources: %v", err)
	}
	fileset := token.NewFileSet()
	files := make([]*ast.File, 0, len(sources))
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileset, source, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", source, err)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		t.Fatalf("the scan found none of its own package's sources to read")
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	config := types.Config{Importer: importer.ForCompiler(fileset, "source", nil)}
	checked, err := config.Check("dinah/internal/lsp", fileset, files, info)
	if err != nil {
		t.Fatalf("type-check dinah/internal/lsp: %v", err)
	}
	return fileset, files, checked, info
}

// isCall reports whether a selector expression denotes a call of the named
// method. The name is resolved against what the receiver's type declares, so
// another type's method of the same name is not it, and this one still is
// whatever the receiver expression is spelled like.
func isCall(info *types.Info, selector *ast.SelectorExpr, method string) bool {
	selection := info.Selections[selector]
	if selection == nil {
		return false
	}
	function, ok := selection.Obj().(*types.Func)
	return ok && function.FullName() == method
}

// constantText answers the string an expression denotes, and whether it
// denotes one at all. Any spelling the compiler folds to a string constant
// answers the same value, so an identifier, a local alias and a concatenation
// of constants are read alike. A variable, a function's answer and anything
// else the compiler cannot fold answer false, and the caller reports those
// rather than passing over them.
func constantText(info *types.Info, expr ast.Expr) (string, bool) {
	value := info.Types[expr].Value
	if value == nil || value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(value), true
}

// declaredText answers the value of a string constant this package declares,
// found by the name it is declared under. The scan resolves the two
// person-facing methods here, once, at their declaration, and compares values
// rather than names at every call site.
func declaredText(t *testing.T, checked *types.Package, name string) string {
	t.Helper()
	declared, ok := checked.Scope().Lookup(name).(*types.Const)
	if !ok {
		t.Fatalf("%s is not a string constant this package declares, so the scan cannot say which notifications a person reads", name)
	}
	if declared.Val().Kind() != constant.String {
		t.Fatalf("%s is a constant of kind %v rather than a string", name, declared.Val().Kind())
	}
	return constant.StringVal(declared.Val())
}

// messageField answers the payload type, its Message field, and where that
// field stands in the struct. The position is what lets a positional
// composite literal be read as well as a keyed one.
func messageField(t *testing.T, checked *types.Package) (types.Type, *types.Var, int) {
	t.Helper()
	declared := checked.Scope().Lookup("logMessageParams")
	if declared == nil {
		t.Fatalf("this package declares no logMessageParams, so the scan cannot tell a message payload from any other value")
	}
	structure, ok := declared.Type().Underlying().(*types.Struct)
	if !ok {
		t.Fatalf("logMessageParams is %v rather than a struct", declared.Type().Underlying())
	}
	for index := range structure.NumFields() {
		if field := structure.Field(index); field.Name() == "Message" {
			return declared.Type(), field, index
		}
	}
	t.Fatalf("logMessageParams carries no Message field, and a message payload is what this scan reads")
	return nil, nil, 0
}

// messageMember answers the expression that becomes the Message member of a
// payload literal. Both spellings of a composite literal are read: a keyed
// element whose key resolves to the field itself, and, where the literal is
// positional, the element standing at the field's own position.
func messageMember(info *types.Info, field *types.Var, index int, literal *ast.CompositeLit) (ast.Expr, bool) {
	keyed := false
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		keyed = true
		if key, ok := pair.Key.(*ast.Ident); ok && info.Uses[key] == field {
			return pair.Value, true
		}
	}
	if keyed || index >= len(literal.Elts) {
		return nil, false
	}
	return literal.Elts[index], true
}

// fromCatalogue reports whether an expression is a call to the catalogue
// renderer's own T method, resolved through the receiver's type. Every other
// expression fails it, because the question the guard asks is where the
// sentence came from rather than which shapes of hard-coded text somebody has
// thought to enumerate.
func fromCatalogue(info *types.Info, expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && isCall(info, selector, catalogueT)
}

// TestNoStringLiteralReachesTheEditorsOwnChannels asserts the second half of
// dinah-515 criterion 27: every argument that becomes the Message member of a
// log line or a shown message is composed by the catalogue renderer, and none
// is a Go string literal written at the call site.
//
// The scan type-checks the package and asks what each notification denotes.
// It sweeps every call of conn.notify, resolves each one's method argument to
// the string constant it stands for, and reads a notification as a person's
// channel when that value is what methodLogMessage or methodShowMessage
// denotes. The method may therefore be spelled as either identifier, as the
// protocol URI itself, as a local alias or as any other constant expression,
// and every one of those is read alike. The second half sweeps every
// composite literal of the payload type wherever it is built, which reaches a
// payload constructed away from any call site, and the third keeps the older
// helper scan. Each half reports the size of the set it swept.
//
// Two shapes the scan cannot resolve, and it names each rather than passing
// over it. A method argument the compiler cannot fold to a constant, which is
// what a variable holding the method gives, is reported with its file and
// line. So is a payload that is not a composite literal at the call site,
// which is what assembling one field by field into a variable gives. Neither
// can reach a person unremarked, but neither is read either, so what this
// guard proves about them is that they exist rather than what they say.
func TestNoStringLiteralReachesTheEditorsOwnChannels(t *testing.T) {
	fileset, files, checked, info := scanTypes(t)
	logMessage := declaredText(t, checked, "methodLogMessage")
	showMessage := declaredText(t, checked, "methodShowMessage")
	payloadType, field, index := messageField(t, checked)

	notifications, channels, payloads, helpers := 0, 0, 0, 0
	for _, file := range files {
		where := func(node ast.Node) string {
			position := fileset.Position(node.Pos())
			return position.Filename + ":" + strconv.Itoa(position.Line)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			// The payload type, wherever it is built. A notification handed a
			// payload assembled away from its call site reaches the channel by
			// a route the call-site scan cannot read, and this half reads it.
			if literal, ok := node.(*ast.CompositeLit); ok && types.Identical(info.Types[literal].Type, payloadType) {
				payloads++
				message, carried := messageMember(info, field, index, literal)
				switch {
				case !carried:
					t.Errorf("%s builds a message payload carrying no Message member, so it would show a person an empty line", where(literal))
				case !fromCatalogue(info, message):
					t.Errorf("%s builds a message payload whose Message member is composed at the call site rather than by the catalogue", where(literal))
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

			// The channel itself: every notification this package sends, read
			// by what its method argument denotes.
			if isCall(info, selector, connNotify) && len(call.Args) == 2 {
				notifications++
				method, resolved := constantText(info, call.Args[0])
				if !resolved {
					t.Errorf("%s sends a notification whose method this scan cannot resolve to a constant, so nothing here can tell whether a person reads it", where(call))
					return true
				}
				if method != logMessage && method != showMessage {
					return true
				}
				channels++
				literal, ok := call.Args[1].(*ast.CompositeLit)
				if !ok || !types.Identical(info.Types[literal].Type, payloadType) {
					t.Errorf("%s sends %s a payload this scan cannot read, so nothing here can tell where its sentence came from", where(call), method)
					return true
				}
				message, carried := messageMember(info, field, index, literal)
				switch {
				case !carried:
					t.Errorf("%s sends %s a payload carrying no Message member", where(call), method)
				case !fromCatalogue(info, message):
					t.Errorf("%s sends %s a sentence composed at the call site rather than drawn from the catalogue", where(call), method)
				}
				return true
			}

			// The helpers, which take a catalogue key rather than a sentence.
			if !isCall(info, selector, serverLog) && !isCall(info, selector, serverShow) {
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
	if notifications != 3 {
		t.Errorf("the scan read %d notifications, and this server sends three", notifications)
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
