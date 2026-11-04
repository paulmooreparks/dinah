package sessionhost

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"testing"
)

var chatPathNames = regexp.MustCompile(`"agent:(update|updates|turn|steer|sent|arriving)"`)
var chatPathConstants = regexp.MustCompile(`(?i)emit\(\s*(sessionhost\.)?Event(Update|Updates|Turn|Steer|Sent)\b`)
var statePathNames = regexp.MustCompile(`"(agent:(session|queue|options|state|account)|settings:changed|update:state|judgements:changed)"`)
var statePathConstants = regexp.MustCompile(`(?i)emit\(\s*(sessionhost\.)?Event(Session|Queue|Options|State)\b`)

// A case label compares an event name; it cannot emit that event. Mask only
// the parsed string expression, retaining line positions and the case body.
func eventSourceWithoutCaseLabels(t *testing.T, data []byte) []byte {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "event-path.go", data, 0)
	if err != nil {
		t.Fatal(err)
	}
	result := bytes.Clone(data)
	ast.Inspect(file, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expression := range clause.List {
			literal, ok := expression.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			for i := fset.Position(literal.Pos()).Offset; i < fset.Position(literal.End()).Offset; i++ {
				if result[i] != '\n' && result[i] != '\r' {
					result[i] = ' '
				}
			}
		}
		return true
	})
	return result
}

func TestEventPathScanDistinguishesCaseLabelsFromEmissions(t *testing.T) {
	for _, scan := range []struct {
		name, constant   string
		names, constants *regexp.Regexp
	}{
		{"turn", "Turn", chatPathNames, chatPathConstants},
		{"state", "State", statePathNames, statePathConstants},
	} {
		name := scan.name
		for _, fixture := range []struct {
			body string
			want bool
		}{
			{`switch name { case "agent:` + name + `": return Chat }; return Other`, false},
			{`emit("agent:` + name + `", data)`, true},
			{`emit(sessionhost.Event` + scan.constant + `, data)`, true},
			{`emit(Event` + scan.constant + `, data)`, true},
			{`switch name { case "agent:` + name + `": emit("agent:` + name + `", data) }`, true},
			{`switch name { case "agent:` + name + `": emit(sessionhost.Event` + scan.constant + `, data) }`, true},
		} {
			source := []byte("package fixture\nfunc classify(name string) { " + fixture.body + " }")
			scanned := eventSourceWithoutCaseLabels(t, source)
			if got := scan.names.Match(scanned) || scan.constants.Match(scanned); got != fixture.want {
				t.Fatalf("scan=%t want=%t for %s", got, fixture.want, source)
			}
		}
	}
}
