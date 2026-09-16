package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/verb"
)

// TestEveryDeclaredParameterIsReadByItsCommand asserts that every parameter
// the verb table declares is one the terminal's own wiring actually reads.
//
// The mcp head has a single dispatch point, so the same claim is checked there
// by driving a value through it. This head has no such point: each command has
// its own run function, and a parameter is wired wherever that function or the
// shared request helper happens to touch it. A parameter can therefore be
// declared, printed in the syntax line, accepted by the flag parser, and then
// read by nothing, and the invocation succeeds while the value goes nowhere.
//
// The declared Field splits the question in two. A parameter the shared helper
// fills is checked once, against that helper's own body, and needs no per
// command evidence; every other parameter is looked for in its command's own
// reachable code. What counts as evidence is either an assignment to the
// declared field or a read of the parameter by name, since a positional slot
// is read out of the word list by index and has no name to find.
//
// This is source inspection rather than execution, and it is one level deep. A
// command that read a parameter inside a function two calls away, or through a
// local variable renamed on the way, would escape it. That is a real limit and
// it is worth stating, because the alternative on offer today is no check at
// all.
func TestEveryDeclaredParameterIsReadByItsCommand(t *testing.T) {
	head := parseHead(t)
	ordered := append([]string(nil), verb.Commands()...)
	sort.Strings(ordered)
	shared := sharedRequestFields(t, head)
	if len(shared) == 0 {
		t.Fatal("the shared request helper yielded no field, so every parameter would fall through to the per-command walk unnoticed")
	}

	for _, command := range ordered {
		for _, param := range verb.Params(command) {
			field, isShared := shared[param.Name]
			if !isShared {
				continue
			}
			if param.Field != field {
				t.Errorf("%s: the shared request helper fills %s from %q and the table declares the field %q",
					command, field, param.Name, param.Field)
			}
		}
	}

	dispatch := runFunctions(t, head)
	checked := 0
	var inert []string
	for _, command := range ordered {
		if _, exempted := commandExemptions[command]; exempted {
			continue
		}
		function, named := dispatch[command]
		if !named {
			t.Errorf("the command table names no run function for %s, so nothing here can be read", command)
			continue
		}
		reachable := reachableFrom(head, function)
		if len(reachable) == 0 {
			t.Errorf("%s: the run function %s was named in the table and not found in the package", command, function)
			continue
		}
		for _, param := range verb.Params(command) {
			// An inert flag is accepted and read nowhere by declaration, so
			// the absence this guard would otherwise report is the answer
			// rather than the defect. The claim is checked in the direction
			// the declaration makes it, which is the half a skip cannot see:
			// a parameter declared inert that IS read is a table that has
			// fallen behind its code, and that is the drift this card has
			// already produced twice. Skipping alone would permit it
			// silently.
			//
			// It is counted as well, so a command whose every parameter were
			// inert could not empty this check without saying so.
			if param.Inert {
				inert = append(inert, command+" --"+param.Name)
				checked++
				// An inert flag carries no value and is given as a flag, so
				// there is no value for a caller to lose. A positional or a
				// valued flag declared inert would be a caller's own text
				// dropped on the floor, and this guard would be the thing
				// saying that was intended.
				if !param.Flag || !param.Marker {
					t.Errorf("%s: the parameter %q is declared inert and is not a marker flag, so a caller's value would be read by nothing and discarded",
						command, param.Name)
				}
				if param.Field != "" {
					t.Errorf("%s: the table declares the parameter %q inert and also names the request field %q, and a parameter that reaches a field is read somewhere",
						command, param.Name, param.Field)
				}
				// The source scan is kept, because a read it CAN see is
				// evidence outright. It is not the proof: see
				// assertInertChangesNothing for why it cannot be.
				if readsParamByName(reachable, param.Name) {
					t.Errorf("%s: the table declares the parameter %q inert, meaning this tool reads it nowhere, and %s reads it by name; either the declaration is stale or the read is",
						command, param.Name, function)
				}
				assertInertChangesNothing(t, command, param.Name)
				continue
			}
			checked++
			if _, isShared := shared[param.Name]; isShared && param.Field != "" && callsByName(reachable, "request") {
				continue
			}
			if readsParamByName(reachable, param.Name) {
				continue
			}
			if param.Field != "" && assignsField(reachable, param.Field) {
				continue
			}
			if param.Field == "" && !param.Flag && callsByName(reachable, "rest") {
				continue
			}
			t.Errorf("%s: the table declares the parameter %q and %s reads nothing under that name, assigns nothing to %q, and is not covered by the shared request helper, so a caller naming it is silently ignored",
				command, param.Name, function, param.Field)
		}
	}
	if checked == 0 {
		t.Fatal("no command declared a parameter, so this check read nothing")
	}
	// The inert set is small and every member of it is a deliberate
	// exemption, so it is pinned by name here rather than by size: a new one
	// has to be argued for in review rather than arriving as a quiet way
	// past this guard, and a reader who trips this is told which parameter
	// to go and look at rather than a number.
	sort.Strings(inert)
	const declaredInert = "lsp --stdio"
	if got := strings.Join(inert, ", "); got != declaredInert {
		t.Errorf("the inert parameters are %q and this tool declares %q", got, declaredInert)
	}
}

// assertInertChangesNothing drives the flag through the command and requires
// the run to be indistinguishable from the run without it.
//
// This is the half that carries the claim, and the source scan above is not.
// A scan decides what a read looks like and then reports every other spelling
// as absent, which is the fail-open direction for an inert parameter: an
// ordinary parameter that this package cannot see being read fails, and an
// inert one that it cannot see being read passes. Code review defeated the
// scan in one attempt by reading the flag through a named constant instead of
// a quoted literal, which is a perfectly ordinary thing to write, and the
// guard reported success on a flag that was genuinely load-bearing. Teaching
// the scan that spelling would leave the next one, so inertness is established
// by behaviour instead, where a read has to change something to matter and the
// spelling it was written in cannot help it hide.
//
// What this does not cover, stated rather than left to be discovered: a read
// whose effect never reaches the exit code or either stream is invisible here.
// That is the boundary of the claim rather than a hole in it, because a flag
// whose presence changes nothing a caller can observe is inert in the only
// sense this table is asserting.
//
// The baseline run is required to succeed, so that the comparison is made
// between two real runs of the command rather than between two refusals that
// would agree whatever the flag did. A command that cannot be run bare needs
// somebody to decide what its baseline is, and failing here is how this asks.
func assertInertChangesNothing(t *testing.T, command, name string) {
	t.Helper()
	root := newBench(t)
	without := runCLI(t, root, command)
	if without.code != 0 {
		t.Errorf("%s: the baseline run exited %d with %q, so the inert comparison for %q would be between two refusals rather than between two runs",
			command, without.code, without.errw, name)
		return
	}
	with := runCLI(t, root, command, "--"+name)
	switch {
	case with.code != without.code:
		t.Errorf("%s: --%s is declared inert and changes the exit code from %d to %d",
			command, name, without.code, with.code)
	case with.out != without.out:
		t.Errorf("%s: --%s is declared inert and changes what the command writes to its output stream",
			command, name)
	case with.errw != without.errw:
		t.Errorf("%s: --%s is declared inert and changes what the command writes to its error stream",
			command, name)
	}
}

// parseHead parses every non-test source file of this package and returns the
// declarations by their own simple names, so a call site can be followed one
// level into the function it names. A method and a function sharing a name are
// both returned under it, which over-reaches rather than under-reaches, and
// the check above is written to tolerate that.
func parseHead(t *testing.T) map[string][]*ast.FuncDecl {
	t.Helper()
	set := token.NewFileSet()
	packages, err := parser.ParseDir(set, ".", func(entry fs.FileInfo) bool {
		return !strings.HasSuffix(entry.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse the package: %v", err)
	}
	declared := map[string][]*ast.FuncDecl{}
	for _, parsed := range packages {
		for _, file := range parsed.Files {
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok {
					continue
				}
				declared[function.Name.Name] = append(declared[function.Name.Name], function)
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("the package parsed to no functions at all, so this check would pass against anything")
	}
	return declared
}

// sharedRequestFields reads the verb.Request literal inside (*session).request
// and returns the parameter each field is filled from, keyed by parameter
// name. It is the one place the terminal wires a parameter for every command
// at once, so it is read once here rather than looked for in each command.
//
// The literal's own type is checked rather than its position. Taking any
// composite literal in the method would let a second literal built from
// parsed.value register a binding to a field verb.Request does not have, which
// would then stand in for wiring nobody wrote.
func sharedRequestFields(t *testing.T, head map[string][]*ast.FuncDecl) map[string]string {
	t.Helper()
	fields := map[string]string{}
	for _, function := range head["request"] {
		if function.Recv == nil || function.Body == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok || !requestLiteralType(literal.Type) {
				return true
			}
			for _, element := range literal.Elts {
				pair, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := pair.Key.(*ast.Ident)
				if !ok {
					continue
				}
				if name, read := paramNameRead(pair.Value); read {
					fields[name] = key.Name
				}
			}
			return true
		})
	}
	return fields
}

// requestLiteralType reports whether a composite literal's type is
// verb.Request, which is the one struct the shared helper builds and the only
// one whose field names this check may read as request fields.
func requestLiteralType(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Request" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "verb"
}

// paramNameRead reports the parameter one expression reads by name, which is
// what `parsed.value("state")` and `parsed.has("override")` both are.
func paramNameRead(expression ast.Expr) (string, bool) {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	if selector.Sel.Name != "value" && selector.Sel.Name != "has" {
		return "", false
	}
	if len(call.Args) != 1 {
		return "", false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	unquoted, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return unquoted, true
}

// runFunctions reads the command table's own entries and returns the run
// function each command names, so the mapping comes from the table rather than
// from a copy of it typed out here.
func runFunctions(t *testing.T, head map[string][]*ast.FuncDecl) map[string]string {
	t.Helper()
	dispatch := map[string]string{}
	for _, function := range head["init"] {
		if function.Body == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			name, runner := "", ""
			for _, element := range literal.Elts {
				pair, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := pair.Key.(*ast.Ident)
				if !ok {
					continue
				}
				switch key.Name {
				case "name":
					if value, ok := pair.Value.(*ast.BasicLit); ok && value.Kind == token.STRING {
						if unquoted, err := strconv.Unquote(value.Value); err == nil {
							name = unquoted
						}
					}
				case "run":
					if value, ok := pair.Value.(*ast.Ident); ok {
						runner = value.Name
					}
				}
			}
			if name != "" && runner != "" {
				dispatch[name] = runner
			}
			return true
		})
	}
	if len(dispatch) == 0 {
		t.Fatal("the command table yielded no run function, so no command below would be walked")
	}
	return dispatch
}

// reachableFrom returns the named function's body together with the bodies of
// every function of this package it calls directly. One level is what carries
// the check past the four commands that dispatch on their own first word and
// past the small helpers that read a flag on a command's behalf.
func reachableFrom(head map[string][]*ast.FuncDecl, name string) []*ast.FuncDecl {
	var bodies []*ast.FuncDecl
	bodies = append(bodies, head[name]...)
	seen := map[string]bool{name: true}
	for _, function := range head[name] {
		if function.Body == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := calleeName(call)
			if callee == "" || seen[callee] {
				return true
			}
			seen[callee] = true
			bodies = append(bodies, head[callee]...)
			return true
		})
	}
	return bodies
}

// calleeName is the simple name a call names, which is the identifier for a
// plain call and the selected name for a method call.
func calleeName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// readsParamByName reports whether any of the bodies reads the named parameter
// through the parsed arguments, which is the evidence a flag leaves behind.
func readsParamByName(bodies []*ast.FuncDecl, name string) bool {
	return anyNode(bodies, func(node ast.Node) bool {
		expression, ok := node.(ast.Expr)
		if !ok {
			return false
		}
		read, ok := paramNameRead(expression)
		return ok && read == name
	})
}

// assignsField reports whether any of the bodies assigns to the named request
// field, which is the evidence a positional slot leaves behind: it is read out
// of the word list by index, so its own name appears nowhere.
//
// Only an assignment counts. A body that merely reads the field would
// otherwise stand in for wiring that was never written, and the reachable set
// is wide enough that some helper reads most of these fields somewhere.
func assignsField(bodies []*ast.FuncDecl, field string) bool {
	return anyNode(bodies, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return false
		}
		for _, target := range assignment.Lhs {
			selector, ok := target.(*ast.SelectorExpr)
			if ok && selector.Sel.Name == field {
				return true
			}
		}
		return false
	})
}

// callsByName reports whether any of the bodies calls a function or method of
// that simple name.
func callsByName(bodies []*ast.FuncDecl, name string) bool {
	return anyNode(bodies, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		return ok && calleeName(call) == name
	})
}

// anyNode reports whether any node of any body satisfies the test.
func anyNode(bodies []*ast.FuncDecl, matches func(ast.Node) bool) bool {
	found := false
	for _, function := range bodies {
		if function.Body == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if found || node == nil {
				return false
			}
			if matches(node) {
				found = true
				return false
			}
			return true
		})
	}
	return found
}
