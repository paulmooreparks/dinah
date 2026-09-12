package bench

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// theFreeReaders are the three functions that read a card off its own directory
// and can therefore know nothing about the number the registry holds for it.
// All three answer a card whose Number is zero, and a card like that reaching a
// person prints twelve hex digits where a reference belongs. Every other read
// goes through a bench method that stamps the registry's number onto the card
// first, and the guard below is what keeps a new caller from reaching around
// that method.
//
// The subject is all three readers rather than the exported one, because the
// retired half is reached through loadRetiredCard and a guard reading one name
// would wave the other two through.
var theFreeReaders = map[string]bool{
	"LoadCard":        true,
	"loadRetiredCard": true,
	"loadCard":        true,
}

// readerFinding is one way a scanned file departs from the rules. Rule names
// the half that reported it: "reference" under rule 1, and "2a", "2b" or "2c"
// under rule 2. Detail carries what the message needs, the function or the
// storage under 2a and 2b, the clause under 2c. Text is the source line, so a
// reader of the report never has to open the file to see what was written.
type readerFinding struct {
	Path   string
	Line   int
	Rule   string
	Detail string
	Text   string
}

// position answers the line a node starts on together with the source text of
// that line.
type position func(node ast.Node) (int, string)

// readerExemption is one file the guard admits, together with how many
// references to a free reader it may carry, why it may carry them, and which
// functions in it are permitted to answer a card at all. The reference budget
// counts a declaration of a reader the same as a call of one, because a
// declaration is an identifier node like any other and excluding it would be a
// special case nobody reading the number could predict.
type readerExemption struct {
	path        string
	references  int
	answersCard []string
	reason      string
}

// freeReaderAllowlist holds the three files that may read a card without the
// bench, and no others. A fourth file wanting an entry here is the defect this
// guard exists to report rather than a line to add: a card read outside these
// three files is a card nobody stamped.
//
// The reference counts and the answer lists are asserted in both directions,
// on the kindAllowlist model, so a budget that no longer matches the file it
// names fails the guard rather than sitting quietly on a stale number.
func freeReaderAllowlist(t *testing.T, root string) []readerExemption {
	t.Helper()
	entries := []readerExemption{
		{path: "internal/bench/card.go", references: 7,
			answersCard: []string{"LoadCard", "loadRetiredCard", "loadCard", "LoadCardIn", "loadRetiredCardIn"},
			reason:      "declares the three readers and the two methods that stamp what each reader answers"},
		{path: "internal/bench/check.go", references: 1,
			reason: "probes whether a card directory a registry line names will read at all, which is a question about the file rather than about the card"},
		{path: "internal/bench/numbermigrate.go", references: 1,
			reason: "reads every card's frontmatter number to build the registry, on a workbench that has no registry yet"},
	}
	for i := range entries {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(entries[i].path))); err != nil {
			t.Fatalf("the reader allowlist names %s and no such file exists: %v", entries[i].path, err)
		}
	}
	return entries
}

// readerExemptionFor answers the allowlist entry naming a path, or nil for a
// path no entry names.
func readerExemptionFor(allowed []readerExemption, path string) *readerExemption {
	for i := range allowed {
		if allowed[i].path == path {
			return &allowed[i]
		}
	}
	return nil
}

// namesFunction reports whether an entry permits this function to answer a
// card.
func namesFunction(entry *readerExemption, function *ast.FuncDecl) bool {
	for _, name := range entry.answersCard {
		if name == function.Name.Name {
			return true
		}
	}
	return false
}

// theImportPath is how a file of another package imports this one, and
// thePackageName is this package's own name, read off that path. The
// vocabulary guard in internal/profile reads the string literals of a Go
// file, and this package's name is the short word that guard refuses as a
// bare literal, so everything below that needs the name composes it out of
// the path the guard already admits.
const theImportPath = "dinah/internal/bench"

var thePackageName = path.Base(theImportPath)

// scanForFreeCardReaders parses every .go file under root that is not a test
// file and answers every finding the two rules read. Rule 1 runs over the
// whole tree; rule 2 runs only over the files the allowlist names. prefix is
// prepended to each file's path, so a caller scanning one directory of a
// repository still composes the paths the allowlist carries.
//
// The two node shapes rule 1 reads are an identifier naming a reader in a file
// of this package or in a file dot-importing it, and a selector naming a
// reader whose qualifier that file's own import list binds to this package. The
// second resolves aliases, so a file importing this package under another
// name is read the same as one importing it under its own. A qualifier
// written inside parentheses is unwrapped before it is read, so a selector
// spelled (w).LoadCard is a reference the same as one spelled w.LoadCard.
// Nothing in the scan asks whether a reference is called, because a
// function value handed elsewhere is as good an escape as a call.
func scanForFreeCardReaders(root, prefix string, allowed []readerExemption) ([]readerFinding, error) {
	var found []readerFinding
	fset := token.NewFileSet()
	walk := func(walked string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(walked, ".go") || strings.HasSuffix(walked, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, walked, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", walked, err)
		}
		relative, err := filepath.Rel(root, walked)
		if err != nil {
			return err
		}
		composed := path.Join(prefix, filepath.ToSlash(relative))
		lines, err := readLines(walked)
		if err != nil {
			return err
		}
		at := func(node ast.Node) (int, string) {
			line := fset.Position(node.Pos()).Line
			text := ""
			if line-1 < len(lines) {
				text = strings.TrimSpace(lines[line-1])
			}
			return line, text
		}
		record := func(node ast.Node, rule, detail string) {
			line, text := at(node)
			found = append(found, readerFinding{Path: composed, Line: line, Rule: rule, Detail: detail, Text: text})
		}
		bound := map[string]bool{}
		dotImport := false
		for _, imported := range file.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil || value != theImportPath {
				continue
			}
			if imported.Name == nil {
				bound[thePackageName] = true
				continue
			}
			if imported.Name.Name == "." {
				dotImport = true
				continue
			}
			bound[imported.Name.Name] = true
		}
		ownPackage := file.Name.Name == thePackageName
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.Ident:
				if theFreeReaders[n.Name] && (ownPackage || dotImport) {
					record(n, "reference", "")
				}
			case *ast.SelectorExpr:
				if !theFreeReaders[n.Sel.Name] {
					return true
				}
				qualifier, ok := parenQualifier(n.X)
				if ok && bound[qualifier.Name] {
					record(n, "reference", "")
				}
			}
			return true
		})
		exemption := readerExemptionFor(allowed, composed)
		if exemption == nil {
			return nil
		}
		for _, decl := range file.Decls {
			general, ok := decl.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if value.Type != nil {
					if typeMentionsACardPointer(value.Type) {
						record(value, "2b", "a package-level var holding a *Card")
					}
					continue
				}
				if len(value.Values) == 0 {
					continue
				}
				// A bare reference to a reader is read before the card-pointer
				// test and carries its own storage, because a var holding
				// the reader as a value is an escape of the reader rather
				// than storage for a card. There is no alias map to read
				// here, because the aliases are function-local and this is
				// the package scope.
				if namesAReader(value.Values[0], nil, bound) {
					record(value, "2b", "a package-level var holding a free reader as a value")
					continue
				}
				if valueMentionsACardPointer(value.Values[0], bound) {
					record(value, "2b", "a package-level var holding a *Card")
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			structure, ok := node.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range structure.Fields.List {
				if !typeMentionsACardPointer(field.Type) {
					continue
				}
				detail := "a struct field holding a *Card"
				if len(field.Names) > 0 {
					detail = "the struct field " + field.Names[0].Name + " holding a *Card"
				}
				record(field, "2b", detail)
			}
			return true
		})
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if resultMentionsCard(function) && !namesFunction(exemption, function) {
				record(function, "2a", function.Name.Name)
			}
			if namesFunction(exemption, function) {
				continue
			}
			found = append(found, valueFindings(function, bound, at, composed)...)
		}
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		return nil, err
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].Path != found[j].Path {
			return found[i].Path < found[j].Path
		}
		if found[i].Line != found[j].Line {
			return found[i].Line < found[j].Line
		}
		return found[i].Rule < found[j].Rule
	})
	return found, nil
}

// resultMentionsCard reports whether a function's result list names the card
// type in any composite form, which is the question rule 2a asks. A selector
// for a field of the card answers no card and is not read here.
func resultMentionsCard(function *ast.FuncDecl) bool {
	if function.Type == nil || function.Type.Results == nil {
		return false
	}
	for _, result := range function.Type.Results.List {
		if mentionsTypeNamedCard(result.Type) {
			return true
		}
	}
	return false
}

// typeMentionsACardPointer reports whether a type expression carries *Card,
// which is one half of the question rule 2b asks of a package-level variable,
// the half a declaration names its type in, and the whole of the question it
// asks of every struct field an allowlisted file declares. The other half of
// the package-level question, a declaration that elides its type and holds
// the card in its initializer, is valueMentionsACardPointer's.
func typeMentionsACardPointer(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		star, ok := node.(*ast.StarExpr)
		if !ok {
			return !found
		}
		if mentionsTypeNamedCard(star.X) {
			found = true
		}
		return !found
	})
	return found
}

// mentionsTypeNamedCard reports whether an expression tree names the type Card.
func mentionsTypeNamedCard(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if found {
			return false
		}
		ident, ok := node.(*ast.Ident)
		if ok && ident.Name == "Card" {
			found = true
			return false
		}
		return true
	})
	return found
}

// parenQualifier answers the import name a selector's qualifier spells,
// unwrapping any number of parentheses, so a qualifier written as (w) or
// ((w)) reads the same as one written as w.
func parenQualifier(expr ast.Expr) (*ast.Ident, bool) {
	for {
		switch read := expr.(type) {
		case *ast.ParenExpr:
			expr = read.X
		case *ast.Ident:
			return read, true
		default:
			return nil, false
		}
	}
}

// valueMentionsACardPointer reports whether a package-level initializer hands
// its variable a card pointer, which is the half of rule 2b's question a var
// declaration asks when it elides its type. The shapes read are a call of one
// of the free readers, a call of new naming the card, the address of a
// composite literal naming the card, and a type expression carrying *Card
// anywhere inside the initializer, which reads a slice or a map of card
// pointers. A call of some other function that answers a card is invisible to
// the walk, and that limit is the one rule 2c already carries: an
// enumeration over syntax rather than a proof over types.
func valueMentionsACardPointer(expr ast.Expr, bound map[string]bool) bool {
	call, isCall := expr.(*ast.CallExpr)
	if isCall && isReaderCall(call, nil, bound) {
		return true
	}
	if isCall && isNewCard(call) {
		return true
	}
	address, isAddress := expr.(*ast.UnaryExpr)
	if isAddress && address.Op == token.AND {
		literal, isLiteral := address.X.(*ast.CompositeLit)
		if isLiteral && literal.Type != nil && mentionsTypeNamedCard(literal.Type) {
			return true
		}
	}
	return typeMentionsACardPointer(expr)
}

// isNewCard reports whether a call is the builtin new over the card type,
// which answers a pointer to a card the way the address of an empty literal
// does.
func isNewCard(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "new" || len(call.Args) != 1 {
		return false
	}
	return mentionsTypeNamedCard(call.Args[0])
}

// valueFindings walks one function of an allowlisted file under rule 2c,
// tracking the identifiers a free reader's value is bound to and reporting
// every shape in which that value leaves the function. The walk is one pass in
// source order, so a binding is tracked before the statements below it read,
// and it enters function literals, because a closure that answers a card is
// as much an escape as a return is.
//
// The walk reads an assignment, a var declaration, and a range clause, which
// are every binding form Go has, because a type-switch guard and the receive
// clause of a select are assignments too. Each target of an assignment or a
// var declaration takes the value only from its own pair on the right, so
// the error a free reader answers in its second result pairs with nothing
// and no error return in the file is tainted. A range clause binds both its
// variables, because the walk
// reads syntax and cannot tell which of the two holds the element. A type
// assertion over a tracked identifier carries the taint to whatever it
// binds, because the asserted value is the card itself, so a type-switch
// guard launders nothing. A name bound from a bare reference to a reader
// rather than from its call is tracked as an alias, and a call through that
// alias reads as a call of the reader itself. A bare reference standing in
// a call's argument list is reported alongside a call of the reader, and
// append's argument list is read too, because a function value handed on is
// as good an escape as a call. A channel send is read the same way, for the
// value and for a call inside it, though a call standing in a send value
// cannot compile, because a reader answers two values where a send takes
// one. A return's results are read for the value too, because answering a
// reader out of a function hands it on exactly as a call argument does. The
// three store clauses the card half already reads, through a field, an
// index, or a pointer, on a named result, and on a name the function does
// not introduce, are read for the value with the same ground, because
// handing a reader to outer state answers it as surely as a call argument
// does.
func valueFindings(function *ast.FuncDecl, bound map[string]bool, at position, composed string) []readerFinding {
	var findings []readerFinding
	tracked := map[string]bool{}
	named := map[string]bool{}
	aliases := map[string]bool{}
	introduced := map[string]bool{}
	collectNamed(named, function.Type)
	collectIntroduced(introduced, function)
	record := func(node ast.Node, clause string) {
		line, text := at(node)
		findings = append(findings, readerFinding{Path: composed, Line: line, Rule: "2c", Detail: clause, Text: text})
	}
	ast.Inspect(function, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			collectNamed(named, n.Type)
		case *ast.ValueSpec:
			for i, name := range n.Names {
				if i >= len(n.Values) {
					break
				}
				if name.Name == "_" {
					continue
				}
				if namesAReader(n.Values[i], aliases, bound) {
					aliases[name.Name] = true
				}
				if carriesTheCard(n.Values[i], tracked, aliases, bound) {
					tracked[name.Name] = true
				}
			}
		case *ast.AssignStmt:
			for i, target := range n.Lhs {
				if i >= len(n.Rhs) {
					break
				}
				switch target := target.(type) {
				case *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr:
					if carriesTheCard(n.Rhs[i], tracked, aliases, bound) {
						record(n, "a tracked card was stored through a field, an index, or a pointer")
					}
					if namesAReader(n.Rhs[i], aliases, bound) {
						record(n, "a free reader was stored through a field, an index, or a pointer")
					}
				case *ast.Ident:
					if target.Name == "_" {
						break
					}
					if named[target.Name] && carriesTheCard(n.Rhs[i], tracked, aliases, bound) {
						record(n, "a tracked card was stored on a named result")
					}
					if !introduced[target.Name] && carriesTheCard(n.Rhs[i], tracked, aliases, bound) {
						record(n, "a tracked card was stored on a name the function does not introduce")
					}
					if named[target.Name] && namesAReader(n.Rhs[i], aliases, bound) {
						record(n, "a free reader was stored on a named result")
					}
					if !introduced[target.Name] && namesAReader(n.Rhs[i], aliases, bound) {
						record(n, "a free reader was stored on a name the function does not introduce")
					}
				}
			}
			// Each target takes the value only from its own pair, so the
			// error a free reader answers in its second result pairs with
			// nothing and no error return in the file is tainted.
			for i, target := range n.Lhs {
				if i >= len(n.Rhs) {
					break
				}
				ident, ok := target.(*ast.Ident)
				if !ok || ident.Name == "_" {
					continue
				}
				if namesAReader(n.Rhs[i], aliases, bound) {
					aliases[ident.Name] = true
				}
				if carriesTheCard(n.Rhs[i], tracked, aliases, bound) {
					tracked[ident.Name] = true
				}
			}
		case *ast.RangeStmt:
			if carriesTheCard(n.X, tracked, aliases, bound) {
				for _, target := range []ast.Expr{n.Key, n.Value} {
					if ident, ok := target.(*ast.Ident); ok && ident.Name != "_" {
						tracked[ident.Name] = true
					}
				}
			}
		case *ast.ReturnStmt:
			if mentionsTrackedIdentifier(n, tracked) {
				record(n, "a tracked card was answered in a return")
			}
			if callsAReaderInside(n, aliases, bound) {
				record(n, "a free reader was called inside a return")
			}
			for _, result := range n.Results {
				if namesAReader(result, aliases, bound) {
					record(n, "a free reader was answered in a return")
					break
				}
			}
		case *ast.SendStmt:
			if mentionsTrackedIdentifier(n.Value, tracked) {
				record(n, "a tracked card was sent on a channel")
			}
			if namesAReader(n.Value, aliases, bound) {
				record(n, "a free reader was sent on a channel as a value")
			}
			if callsAReaderInside(n.Value, aliases, bound) {
				record(n, "a free reader was called inside a channel send")
			}
		case *ast.CallExpr:
			// The reference pass runs before the append test, because the
			// builtin's argument list is one more place a reader value is
			// handed on to and the taint-following below it is about cards
			// rather than about references.
			for _, argument := range n.Args {
				if namesAReader(argument, aliases, bound) {
					record(n, "a free reader was handed as a call argument")
					break
				}
			}
			if isAppend(n) {
				return true
			}
			for _, argument := range n.Args {
				if mentionsTrackedIdentifier(argument, tracked) {
					record(n, "a tracked card was passed as a call argument")
					break
				}
			}
			for _, argument := range n.Args {
				if callsAReaderInside(argument, aliases, bound) {
					record(n, "a free reader was called inside a call argument")
					break
				}
			}
		}
		return true
	})
	return findings
}

// collectNamed adds a function type's own named results to the set, because a
// function with named results can answer with a bare return and the return
// clause would otherwise find no expression to read.
func collectNamed(named map[string]bool, functionType *ast.FuncType) {
	if functionType == nil || functionType.Results == nil {
		return
	}
	for _, result := range functionType.Results.List {
		for _, name := range result.Names {
			named[name.Name] = true
		}
	}
}

// collectIntroduced adds every name the function itself binds to the set: the
// receiver, every parameter and named result of the function and of each
// function literal inside it, every name a var declaration or a short
// declaration binds, and every variable a range clause declares. The set is
// what separates a name a function introduces from one it merely assigns, and
// a package-level variable is the second kind, so storing a card on it is an
// escape rather than a local binding.
func collectIntroduced(introduced map[string]bool, function *ast.FuncDecl) {
	if function.Recv != nil {
		for _, field := range function.Recv.List {
			for _, name := range field.Names {
				introduced[name.Name] = true
			}
		}
	}
	ast.Inspect(function, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncType:
			collectFieldNames(introduced, n.Params)
			collectFieldNames(introduced, n.Results)
		case *ast.ValueSpec:
			for _, name := range n.Names {
				introduced[name.Name] = true
			}
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE {
				return true
			}
			for _, target := range n.Lhs {
				if ident, ok := target.(*ast.Ident); ok {
					introduced[ident.Name] = true
				}
			}
		case *ast.RangeStmt:
			if n.Tok != token.DEFINE {
				return true
			}
			for _, target := range []ast.Expr{n.Key, n.Value} {
				if ident, ok := target.(*ast.Ident); ok {
					introduced[ident.Name] = true
				}
			}
		}
		return true
	})
}

// collectFieldNames adds the names a field list declares, which for a function
// type are its parameters or its named results.
func collectFieldNames(introduced map[string]bool, fields *ast.FieldList) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			introduced[name.Name] = true
		}
	}
}

// carriesTheCard reports whether an expression hands a binding the value of a
// card one of the free readers answered, by one of the shapes that carry the
// taint: the reader's own call, direct or through a name the walk bound from
// a reader reference, a tracked identifier read alone or through the alias
// operators, a type assertion whose operand carries the taint, an append
// whose argument carries the taint, and a nested append is read the same
// way through the ellipsis it is spread with, a composite literal carrying
// a tracked identifier anywhere inside it, or a call whose function
// expression carries one, which is a method on the card. The list is closed
// on purpose, because
// tainting on any expression that merely mentions a tracked identifier would
// reach a selector reading one field off the card and forbid the very reads
// the migration's entry exists to permit.
//
// The type-assertion shape reads its operand rather than mentioning it, so a
// conversion inside the assertion does not carry the taint: asserting a
// tracked identifier directly binds the card, while asserting it through a
// conversion leaves the binding untracked, because the conversion is a call
// whose function expression names no tracked identifier. The conversion
// spelling is still caught, at its own line, by the clause that reports a
// tracked identifier standing in a call's argument list.
func carriesTheCard(expr ast.Expr, tracked map[string]bool, aliases map[string]bool, bound map[string]bool) bool {
	switch value := expr.(type) {
	case *ast.CallExpr:
		if isReaderCall(value, aliases, bound) {
			return true
		}
		if isAppend(value) {
			for _, argument := range value.Args {
				if aliasReads(argument, tracked) {
					return true
				}
				// An argument spread from a nested append is read through
				// the ellipsis and then as the call it is, because the
				// nested call's own arguments are one more place a tracked
				// card can hide.
				inner := argument
				if spread, ok := inner.(*ast.Ellipsis); ok {
					inner = spread.Elt
				}
				if carriesTheCard(inner, tracked, aliases, bound) {
					return true
				}
			}
			return false
		}
		return mentionsTrackedIdentifier(value.Fun, tracked)
	case *ast.CompositeLit:
		return mentionsTrackedIdentifier(value, tracked)
	case *ast.TypeAssertExpr:
		return carriesTheCard(value.X, tracked, aliases, bound)
	}
	return aliasReads(expr, tracked)
}

// aliasReads reports whether an expression is a tracked identifier, read
// through any number of parentheses, address-of operators and dereference
// operators. Those operators copy or re-address the card rather than reading
// a field off it, so a binding from them carries the whole card.
func aliasReads(expr ast.Expr, tracked map[string]bool) bool {
	if len(tracked) == 0 {
		return false
	}
	for {
		switch read := expr.(type) {
		case *ast.ParenExpr:
			expr = read.X
		case *ast.StarExpr:
			expr = read.X
		case *ast.UnaryExpr:
			if read.Op != token.AND {
				return false
			}
			expr = read.X
		case *ast.Ident:
			return tracked[read.Name]
		default:
			return false
		}
	}
}

// mentionsTrackedIdentifier reports whether any identifier inside a node is
// one the walk is tracking.
func mentionsTrackedIdentifier(node ast.Node, tracked map[string]bool) bool {
	if node == nil || len(tracked) == 0 {
		return false
	}
	found := false
	ast.Inspect(node, func(child ast.Node) bool {
		if found {
			return false
		}
		ident, ok := child.(*ast.Ident)
		if ok && tracked[ident.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

// callsAReaderInside reports whether a node contains a call of one of the
// free readers, which is the shape a function employs when it hands a caller
// the reader's own answer directly. A return and a call's argument list ask
// the question, and each records its own finding when it answers yes.
func callsAReaderInside(node ast.Node, aliases map[string]bool, bound map[string]bool) bool {
	found := false
	ast.Inspect(node, func(child ast.Node) bool {
		if found {
			return false
		}
		call, ok := child.(*ast.CallExpr)
		if !ok {
			return true
		}
		if isReaderCall(call, aliases, bound) {
			found = true
			return false
		}
		return true
	})
	return found
}

// isReaderCall reports whether a call names one of the free readers, either
// bare in a file of this package, through a qualifier that file's imports
// bind to it, or through a name the walk itself bound from a bare reference
// to a reader. The function expression is unwrapped through parentheses and
// the address-of and dereference operators first, on the model the alias
// walk uses, so a call spelled through those operators is a call of the
// reader, and a qualifier written through parentheses reads the same way a
// bare one does.
func isReaderCall(call *ast.CallExpr, aliases map[string]bool, bound map[string]bool) bool {
	fun := call.Fun
	for {
		switch read := fun.(type) {
		case *ast.ParenExpr:
			fun = read.X
		case *ast.StarExpr:
			fun = read.X
		case *ast.UnaryExpr:
			if read.Op != token.AND {
				return false
			}
			fun = read.X
		case *ast.Ident:
			return theFreeReaders[read.Name] || aliases[read.Name]
		case *ast.SelectorExpr:
			if !theFreeReaders[read.Sel.Name] {
				return false
			}
			qualifier, ok := parenQualifier(read.X)
			return ok && bound[qualifier.Name]
		default:
			return false
		}
	}
}

// namesAReader reports whether an expression is a bare reference to one of the
// free readers rather than a call of one, read through parentheses and the
// address-of and dereference operators on the model the alias walk uses, and
// either as a plain identifier in a file of this package or through a qualifier
// that file's imports bind to it. A name the walk itself bound from a reference
// is read the same way, so a binding taken from an alias binds an alias in
// turn. A composite literal is read through its element values, without
// entering a call, so a reader stored in a literal reads as a reference the
// way a bare one does and a binding from such a literal binds an alias.
// Binding such a reference spends one of rule 1's references and answers a
// function value that calls the reader when called, so the walk tracks the
// bound name as an alias and reads calls made through it as calls of the
// reader itself.
func namesAReader(expr ast.Expr, aliases map[string]bool, bound map[string]bool) bool {
	for {
		switch read := expr.(type) {
		case *ast.ParenExpr:
			expr = read.X
		case *ast.StarExpr:
			expr = read.X
		case *ast.UnaryExpr:
			if read.Op != token.AND {
				return false
			}
			expr = read.X
		case *ast.Ident:
			return theFreeReaders[read.Name] || aliases[read.Name]
		case *ast.SelectorExpr:
			if !theFreeReaders[read.Sel.Name] {
				return false
			}
			qualifier, ok := parenQualifier(read.X)
			return ok && bound[qualifier.Name]
		case *ast.CompositeLit:
			for _, element := range read.Elts {
				value := element
				if pair, isPair := element.(*ast.KeyValueExpr); isPair {
					value = pair.Value
				}
				if namesAReader(value, aliases, bound) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
}

// isAppend reports whether a call is the builtin append, whose taint the walk
// already follows and whose argument list is therefore not read again.
func isAppend(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "append"
}

// readerViolations applies the allowlist to a scan's findings and answers
// every way the scanned tree departs from it: a reference in a file no entry
// names, a reference count above or below the entry's budget, and every
// finding of rule 2. The budget is asserted in both directions so an entry
// that no longer matches its file fails the guard rather than sitting on a
// number nobody re-derived.
func readerViolations(findings []readerFinding, allowed []readerExemption) []string {
	var violations []string
	counted := map[string]int{}
	for _, finding := range findings {
		switch finding.Rule {
		case "reference":
			if readerExemptionFor(allowed, finding.Path) == nil {
				violations = append(violations, fmt.Sprintf(
					"%s:%d refers to the free card reader and no entry of the reader allowlist names this file (rule 1): %s",
					finding.Path, finding.Line, finding.Text))
				continue
			}
			counted[finding.Path]++
		case "2a":
			violations = append(violations, fmt.Sprintf(
				"%s:%d declares %s, which answers a card, and the file's entry does not name it (rule 2a): %s",
				finding.Path, finding.Line, finding.Detail, finding.Text))
		case "2b":
			violations = append(violations, fmt.Sprintf(
				"%s:%d declares %s, which the reader allowlist permits nowhere (rule 2b): %s",
				finding.Path, finding.Line, finding.Detail, finding.Text))
		default:
			violations = append(violations, fmt.Sprintf(
				"%s:%d: %s (rule 2c): %s",
				finding.Path, finding.Line, finding.Detail, finding.Text))
		}
	}
	for _, entry := range allowed {
		if counted[entry.path] > entry.references {
			violations = append(violations, fmt.Sprintf(
				"%s carries %d references to the free card readers and its entry allows %d, so the file has grown a second reader behind its exemption",
				entry.path, counted[entry.path], entry.references))
		}
		if counted[entry.path] < entry.references {
			violations = append(violations, fmt.Sprintf(
				"%s carries %d references to the free card readers and its entry allows %d, so the entry is stale and has to be tightened",
				entry.path, counted[entry.path], entry.references))
		}
	}
	sort.Strings(violations)
	return violations
}

// TestEveryCardNumberReadGoesThroughTheBench asserts that every read of a
// card goes through the bench and stamps the registry's number onto what it
// answers. Rule 1 counts every syntactic reference to the three free readers
// over the product's own source and admits exactly three files, each on a
// stated budget. Rule 2 then reads those three files alone and asserts that a
// card a free reader answered does not leave the function that read it: a
// declaration answering one fails unless the file's entry names it, storage
// holding one fails, and the value half tracks the identifiers the card is
// bound to and reports every escape shape it enumerates.
//
// Rule 2 is an enumeration and not a proof, and the enumeration is what the
// third test below plants and attacks. What covers the shapes no scan can
// list is the observable the card's other criteria drive, that every command
// printing a card reference prints the registry's number.
//
// The scan reads cmd/ and internal/ by name rather than walking the
// repository root, so an untracked worktree inside the tree never reaches it.
func TestEveryCardNumberReadGoesThroughTheBench(t *testing.T) {
	root := repositoryRoot(t)
	allowed := freeReaderAllowlist(t, root)
	var findings []readerFinding
	for _, directory := range []string{"cmd", "internal"} {
		found, err := scanForFreeCardReaders(filepath.Join(root, directory), directory, allowed)
		if err != nil {
			t.Fatalf("scan %s: %v", directory, err)
		}
		findings = append(findings, found...)
	}
	for _, violation := range readerViolations(findings, allowed) {
		t.Errorf("%s", violation)
	}
}

// TestTheRegistryIsNotUnionMerged asserts that the attributes text a new
// workbench is written with names no pattern covering the card-number
// registry. The union merge driver is what lets two clones each append a line
// to one journal without a conflict, and the registry is the one file in a
// workbench where that behaviour would be silent damage: two clones that each
// allocate a number append one line each, the union driver keeps both, and
// the workbench answers a number two cards carry without a conflict anybody
// sees. This is a regression guard, so it passes on the tree as it stands and
// goes red the day somebody adds the registry to the union set.
func TestTheRegistryIsNotUnionMerged(t *testing.T) {
	patterns := unionPatterns(unionJournals)
	if len(patterns) == 0 {
		t.Fatalf("the union text a new workbench's attributes carry holds no pattern at all, so the registry has lost the guard that names it: %q", unionJournals)
	}
	// The matcher has to be able to answer yes, or its no is not evidence.
	// The journal's own pattern is in the set, so the journal is the file
	// that proves the matcher in the positive direction.
	if !coveredBy(patterns, JournalName) {
		t.Fatalf("no pattern in %q covers %s, so the matcher cannot recognise a file the union set does cover and its verdict on the registry proves nothing", unionJournals, JournalName)
	}
	for _, pattern := range patterns {
		if attributeCovers(pattern, CardNumbersName) {
			t.Errorf("the pattern %q in the union text covers %s, and a union merge over the registry is two clones each keeping the number the other allocated", pattern, CardNumbersName)
		}
	}
}

// coveredBy reports whether any pattern in the union text covers a file
// name, which the test reads once in the positive direction over the journal
// before it reads the registry in the negative direction.
func coveredBy(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if attributeCovers(pattern, name) {
			return true
		}
	}
	return false
}

// unionPatterns reads the attribute patterns out of the text a new
// workbench's .gitattributes carries, which is one pattern and one driver per
// line.
func unionPatterns(text string) []string {
	var patterns []string
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		patterns = append(patterns, fields[0])
	}
	return patterns
}

// attributeCovers reports whether one gitattributes pattern covers a file
// name, reading as much of git's pattern grammar as the question needs: a
// pattern with no separator matches the name at any depth, a leading
// separator anchors the pattern where the attributes file lives and so does
// a separator inside it, a single star stops at a separator, a question mark
// stands for one character, and a leading double star may also stand for
// nothing at all.
func attributeCovers(pattern, name string) bool {
	anchored := strings.HasPrefix(pattern, "/")
	if anchored {
		pattern = pattern[1:]
	}
	if strings.HasPrefix(pattern, "**/") {
		return matchAnywhere(pattern[3:], name)
	}
	if !anchored && !strings.Contains(pattern, "/") {
		name = path.Base(name)
	}
	return matchExact(expressionOf(pattern), name)
}

// matchAnywhere reports whether a pattern covers a name at any depth,
// including at the root the attributes file lives in.
func matchAnywhere(pattern, name string) bool {
	return matchExact("(?:.*/)?"+expressionOf(pattern), name)
}

// matchExact reports whether a name matches one anchored expression.
func matchExact(expression, name string) bool {
	matched, err := regexp.MatchString("^"+expression+"$", name)
	return err == nil && matched
}

// expressionOf translates a gitattributes pattern into the body of a regular
// expression over one name.
func expressionOf(pattern string) string {
	var expression strings.Builder
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				expression.WriteString(".*")
				i++
				continue
			}
			expression.WriteString("[^/]*")
		case '?':
			expression.WriteString("[^/]")
		default:
			expression.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	return expression.String()
}

// plantedEscape is one hostile shape planted outside the tree to prove the
// scan reports it. files maps a path relative to the planted root to Go
// source, and the case runner composes each file's package clause out of the
// path's own last directory segment, because a planted source declares the
// package its directory names and this package's own name is the short word
// the vocabulary guard refuses as a bare literal. The planted sources spell
// the receiver type by the product's own word for the same reason: the guard
// strips the long word before it looks for the short one, and a receiver
// spelled the other way would redden the corpus. allowlist is the entry list
// the scan runs with, which every rule 2 case sizes so the planted file
// spends exactly one free reference and rule 1 is satisfied, leaving the
// escape for rule 2 to catch. want lists fragments that must appear among the
// violations, and count is the exact number of violations the shape owes, so
// a clause firing twice or a second rule piling on shows up as a number
// rather than as silence.
type plantedEscape struct {
	name      string
	files     map[string]string
	allowlist []readerExemption
	want      []string
	count     int
}

// oneProbeEntry is the allowlist every rule 2 case runs against: a check.go
// carrying one free reference, which is the whole budget.
func oneProbeEntry() []readerExemption {
	return []readerExemption{{path: "internal/bench/check.go", references: 1}}
}

// TestTheLoadCardGuardGoesRed proves the scan above can fail, by planting
// every shape that defeated an earlier round's guard into a directory of its
// own and requiring the violation that catches it back. A guard nobody has
// watched fail is a guard nobody knows works, and the fixtures never land in
// the tree, so the reproduction cannot trip the production scan.
//
// The corpus is the recorded history of this guard rather than a guess at
// what an evader might write. The first case is the round that lifted the one
// allowed call into an exported method, which passed a guard that counted
// references alone because it spent exactly one. Every later case is a shape
// that got through the rules as the round before it left them: a var
// declaration a walk reading assignments alone never bound, a card handed to
// a helper declared where rule 2 never looks, the same escape through a
// struct, a slice and a map literal, a named result answered by a bare
// return, an alias read through each of the three operators, and a method on
// the card declared outside the allowlisted files. The newest cases are the
// shapes a review planted against the shipped guard and watched escape: an
// index and a star target receiving the reader's answer directly, a
// package-level name receiving it, a named result receiving it without a
// laundering assignment, a range clause binding a card out of a slice, a
// reader call standing inside a call's arguments, a call through a local
// alias of the reader, and package-level storage whose var declaration
// elides its type. The three cases after those are this round's own probes
// of the claims its own record makes: a type-switch guard and a plain type
// assertion binding the card out of a tracked identifier, shapes the walk's
// binding-form sentence had to cover and did not until the assertion shape
// was added, and a tracked card sent on a channel, which is the arm the
// receive residual's defense rests on, planted so that arm cannot be
// removed silently.
func TestTheLoadCardGuardGoesRed(t *testing.T) {
	cases := []plantedEscape{
		{
			name: "an exported method wrapping the one allowed call",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) LoadCardUnstamped(root, id string) (*Card, error) {
	return LoadCard(root, id)
}

func (b *Workbench) checkCards(ids []string) []Finding {
	var findings []Finding
	for _, id := range ids {
		card, err := b.LoadCardUnstamped(b.CardsRoot(), id)
		if err != nil {
			continue
		}
		findings = append(findings, b.checkCard(card)...)
	}
	return findings
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"LoadCardUnstamped, which answers a card", "a free reader was called inside a return"},
			count:     2,
		},
		{
			name: "an unexported function answering a card directly",
			files: map[string]string{"internal/bench/check.go": `func cardAt(root, id string) (*Card, error) {
	c, err := LoadCard(root, id)
	return c, err
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"cardAt, which answers a card", "a tracked card was answered in a return"},
			count:     2,
		},
		{
			name: "a method aliasing the card into a second identifier",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) aliased(root, id string) (*Card, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	d := c
	return d, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"aliased, which answers a card", "a tracked card was answered in a return"},
			count:     2,
		},
		{
			name: "a method storing the card on its receiver",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) stashed(root, id string) error {
	c, err := LoadCard(root, id)
	if err != nil {
		return err
	}
	b.lastCard = c
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was stored through a field, an index, or a pointer"},
			count:     1,
		},
		{
			name: "a method answering the card as any",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) laundered(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	return c, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a method answering a composite literal carrying the card",
			files: map[string]string{
				"internal/bench/check.go": `func (b *Workbench) held(root, id string) (holder, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return holder{}, err
	}
	return holder{card: c}, nil
}
`,
				"internal/bench/types.go": `type holder struct {
	card *Card
}
`,
			},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a method collecting cards with append",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) collected(root string, ids []string) ([]*Card, error) {
	var cards []*Card
	for _, id := range ids {
		c, err := LoadCard(root, id)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"collected, which answers a card", "a tracked card was answered in a return"},
			count:     2,
		},
		{
			name: "a nested append laundering a tracked card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) nested(root, id string) ([]*Card, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	var more []*Card
	cards := append([]*Card{}, append(more, c)...)
	return cards, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"nested, which answers a card", "a tracked card was answered in a return"},
			count:     2,
		},
		{
			name: "a method answering a field of the card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) numbered(root, id string) (int, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return 0, err
	}
	return c.Number, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a method copying the card into a local struct",
			files: map[string]string{
				"internal/bench/check.go": `func (b *Workbench) snapshotted(root, id string) (snapshot, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return snapshot{}, err
	}
	s := snapshot{number: c.Number, dir: c.Dir}
	return s, nil
}
`,
				"internal/bench/types.go": `type snapshot struct {
	number int
	dir    string
}
`,
			},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a closure answering the card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) deferred(root, id string) func() (any, error) {
	return func() (any, error) {
		c, err := LoadCard(root, id)
		if err != nil {
			return nil, err
		}
		return c, nil
	}
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was called inside a return", "a tracked card was answered in a return"},
			count:     2,
		},
		{
			name: "a var declaration answering the card as any",
			files: map[string]string{"internal/bench/check.go": `func variadic(root, id string) (any, error) {
	var c, err = LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	return c, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a method binding with var and answering as any",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) variably(root, id string) (any, error) {
	var c, err = LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	return c, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a probe passing the card to a helper declared elsewhere",
			files: map[string]string{
				"internal/bench/check.go": `func (b *Workbench) probed(root, id string) error {
	c, err := LoadCard(root, id)
	if err != nil {
		return err
	}
	b.stashCard(c)
	return nil
}
`,
				"internal/bench/types.go": `func (b *Workbench) stashCard(c *Card) {
	b.lastCard = c
}

func (b *Workbench) LastCard() *Card {
	return b.lastCard
}
`,
			},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was passed as a call argument"},
			count:     1,
		},
		{
			name: "a probe binding a struct literal carrying the card",
			files: map[string]string{
				"internal/bench/check.go": `func (b *Workbench) carried(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	h := CardHolder{Card: c}
	return h, nil
}
`,
				"internal/bench/types.go": `type CardHolder struct {
	Card *Card
}
`,
				"internal/verb/caller.go": `import w "dinah/internal/bench"

// read is the caller the escape was run against end to end: it asks the
// workbench for the value the probe answered and reads the card's reference
// off it, which is the harm the composite-literal clause exists to prevent.
func read(b *w.Workbench, root, id, slug string) (string, bool) {
	held, err := b.Carried(root, id)
	if err != nil {
		return "", false
	}
	holder := held.(w.CardHolder)
	return holder.Card.Ref(slug), true
}
`,
			},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a slice literal carrying the card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) sliced(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	cs := []*Card{c}
	return cs, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a map literal carrying the card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) mapped(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	var m = map[string]*Card{id: c}
	return m, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a probe assigning the card to its own named result",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) reported(root, id string) (out any, err error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	out = c
	return
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was stored on a named result"},
			count:     1,
		},
		{
			name: "an alias through the address-of operator",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) addressed(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	p := &c
	return p, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "an alias through parentheses",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) parenthesized(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	d := (c)
	return d, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a dereference copy answered by its address",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) dereferenced(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	d := *c
	return &d, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a method on the card declared elsewhere",
			files: map[string]string{
				"internal/bench/check.go": `func (b *Workbench) rebodied(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	d := c.Copied()
	return d, nil
}
`,
				"internal/bench/types.go": `func (c *Card) Copied() *Card {
	card := *c
	return &card
}
`,
			},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "an index target receiving the reader call directly",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) cached(root, id string) error {
	cache := map[string]*Card{}
	cache[id], err := LoadCard(root, id)
	return err
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was stored through a field, an index, or a pointer"},
			count:     1,
		},
		{
			name: "a star target receiving a tracked card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) slotted(root, id string) error {
	c, err := LoadCard(root, id)
	if err != nil {
		return err
	}
	var slot *Card
	*slot = c
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was stored through a field, an index, or a pointer"},
			count:     1,
		},
		{
			name: "a package-level name receiving the reader call directly",
			files: map[string]string{
				"internal/bench/check.go": `func (b *Workbench) probedLast(root, id string) error {
	last, err = LoadCard(root, id)
	return err
}
`,
				"internal/bench/types.go": `var last *Card
`,
			},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was stored on a name the function does not introduce"},
			count:     1,
		},
		{
			name: "a named result receiving the reader call directly",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) reportedDirect(root, id string) (out any, err error) {
	out, err = LoadCard(root, id)
	return
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was stored on a named result"},
			count:     1,
		},
		{
			name: "a reader value stored through a field",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) storedValue(root, id string) error {
	b.anyField = LoadCard
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was stored through a field, an index, or a pointer"},
			count:     1,
		},
		{
			name: "a reader value stored through an index",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) storedIndex(root, id string, out map[string]any) error {
	out[id] = LoadCard
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was stored through a field, an index, or a pointer"},
			count:     1,
		},
		{
			name: "a reader value stored on a named result",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) handedNamed(root, id string) (out func(string, string) (*Card, error), err error) {
	out = LoadCard
	return
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"handedNamed, which answers a card", "a free reader was stored on a named result"},
			count:     2,
		},
		{
			name: "a reader value stored on a name the function does not introduce",
			files: map[string]string{"internal/bench/check.go": `var last any

func (b *Workbench) storedOuter(root, id string) error {
	last = LoadCard
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was stored on a name the function does not introduce"},
			count:     1,
		},
		{
			name: "a range clause laundering a tracked card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) ranged(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	for _, d := range []*Card{c} {
		return d, nil
	}
	return nil, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a reader call nested inside a call argument",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) stashed(root, id string) error {
	stash(LoadCard(root, id))
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was called inside a call argument"},
			count:     1,
		},
		{
			name: "a call through a local alias of the reader",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) aliasedCall(root, id string) error {
	read := LoadCard
	stash(read(root, id))
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was called inside a call argument"},
			count:     1,
		},
		{
			name: "a type-switch guard laundering a tracked card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) switched(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	switch d := c.(type) {
	case *Card:
		return d, nil
	}
	return nil, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a plain type assertion laundering a tracked card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) asserted(root, id string) (any, error) {
	c, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	d := c.(*Card)
	return d, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a tracked card sent on a channel",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) channeled(root, id string, ch chan *Card) error {
	c, err := LoadCard(root, id)
	if err != nil {
		return err
	}
	ch <- c
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was sent on a channel"},
			count:     1,
		},
		{
			name: "a reader handed to a call as a value",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) stashedValue(root, id string) error {
	stash(LoadCard)
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was handed as a call argument"},
			count:     1,
		},
		{
			name: "a package-level var holding the reader as a value",
			files: map[string]string{"internal/bench/check.go": `var read = LoadCard
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a package-level var holding a free reader as a value"},
			count:     1,
		},
		{
			name: "an address-of laundered reader handed to a call",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) stashedAddress(root, id string) error {
	stash(&LoadCard)
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was handed as a call argument"},
			count:     1,
		},
		{
			name: "an address-of laundered reader held by a package-level var",
			files: map[string]string{"internal/bench/check.go": `var read = &LoadCard
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a package-level var holding a free reader as a value"},
			count:     1,
		},
		{
			name: "a package-level var holding a reader inside a composite literal",
			files: map[string]string{"internal/bench/check.go": `var h = holder{read: LoadCard}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a package-level var holding a free reader as a value"},
			count:     1,
		},
		{
			name: "a binding taken from an alias binds an alias in turn",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) rebind(root, id string) error {
	read := LoadCard
	second := read
	stash(second(root, id))
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was called inside a call argument"},
			count:     1,
		},
		{
			name: "an alias handed to a call as a value",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) stashedAlias(root, id string) error {
	read := LoadCard
	stash(read)
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was handed as a call argument"},
			count:     1,
		},
		{
			name: "a reader value handed to append as an argument",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) appended(root, id string) error {
	var cards []any
	cards = append(cards, LoadCard)
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was handed as a call argument"},
			count:     1,
		},
		{
			name: "a composite literal storing a reader, answered as a value",
			files: map[string]string{
				"internal/bench/check.go": `func (b *Workbench) literal(root, id string) (holder, error) {
	h := holder{read: LoadCard}
	return h, nil
}
`,
				"internal/bench/types.go": `type holder struct {
	read any
}
`,
			},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was answered in a return"},
			count:     1,
		},
		{
			name: "a slice literal storing a reader, answered as a value",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) literalSlice(root, id string) ([]any, error) {
	cs := []any{LoadCard}
	return cs, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was answered in a return"},
			count:     1,
		},
		{
			name: "a reader value sent on a channel",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) shipped(root, id string, ch chan func(string, string) (*Card, error)) error {
	ch <- LoadCard
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was sent on a channel as a value"},
			count:     1,
		},
		{
			name: "a reader call standing in a channel send",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) shippedCall(root, id string, ch chan *Card) error {
	ch <- LoadCard(root, id)
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a free reader was called inside a channel send"},
			count:     1,
		},
		{
			name: "a reader value answered in a return",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) handed(root, id string) (func(string, string) (*Card, error), error) {
	return LoadCard, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"handed, which answers a card", "a free reader was answered in a return"},
			count:     2,
		},
		{
			name: "a parenthesized reader call",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) wrappedCall(root, id string) (any, error) {
	c, err := (LoadCard)(root, id)
	if err != nil {
		return nil, err
	}
	return c, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "an operator-laundered reader call",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) relaundered(root, id string) (any, error) {
	c, err := (*&LoadCard)(root, id)
	if err != nil {
		return nil, err
	}
	return c, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a second assignment target laundering a tracked card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) retook(root, id string) (any, error) {
	card, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	var first, second *Card
	first, second = card, card
	return second, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a second var name laundering a tracked card",
			files: map[string]string{"internal/bench/check.go": `func (b *Workbench) redeclared(root, id string) (any, error) {
	card, err := LoadCard(root, id)
	if err != nil {
		return nil, err
	}
	var first, second = card, card
	return second, nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a parenthesized qualifier in a file no entry names",
			files: map[string]string{"internal/verb/escape.go": `import w "dinah/internal/bench"

func unstampedQualified(root, id string) (*w.Card, error) {
	return (w).LoadCard(root, id)
}
`},
			allowlist: []readerExemption{{path: "internal/bench/check.go", references: 0}},
			want:      []string{"internal/verb/escape.go", "refers to the free card reader"},
			count:     1,
		},
		{
			name: "a parenthesized qualifier laundering a tracked card",
			files: map[string]string{"internal/verb/escape.go": `import w "dinah/internal/bench"

func unstampedParenCall(root, id string) (int, error) {
	c, err := (w).LoadCard(root, id)
	if err != nil {
		return 0, err
	}
	return c.Number, nil
}
`},
			allowlist: []readerExemption{{path: "internal/verb/escape.go", references: 1}},
			want:      []string{"a tracked card was answered in a return"},
			count:     1,
		},
		{
			name: "a parenthesized qualifier handing a reader to a call",
			files: map[string]string{"internal/verb/escape.go": `import w "dinah/internal/bench"

func unstampedParenValue(root, id string) error {
	stash((w).LoadCard)
	return nil
}
`},
			allowlist: []readerExemption{{path: "internal/verb/escape.go", references: 1}},
			want:      []string{"a free reader was handed as a call argument"},
			count:     1,
		},
		{
			name: "a reader called from a file the allowlist does not name",
			files: map[string]string{"internal/verb/escape.go": `import w "dinah/internal/bench"

func unstamped(root, id string) (*w.Card, error) {
	return w.LoadCard(root, id)
}
`},
			allowlist: []readerExemption{{path: "internal/bench/check.go", references: 0}},
			want:      []string{"internal/verb/escape.go", "refers to the free card reader"},
			count:     1,
		},
		{
			name: "a second reference behind a budget of one",
			files: map[string]string{"internal/bench/check.go": `func probedA(root, id string) error {
	_, err := LoadCard(root, id)
	return err
}

func probedB(root, id string) error {
	_, err := LoadCard(root, id)
	return err
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"carries 2 references to the free card readers and its entry allows 1"},
			count:     1,
		},
		{
			name: "an entry whose budget is stale",
			files: map[string]string{"internal/bench/check.go": `func quiet(root, id string) error {
	return nil
}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"carries 0 references to the free card readers and its entry allows 1"},
			count:     1,
		},
		{
			name: "storage holding a card",
			files: map[string]string{"internal/bench/check.go": `var lastCard *Card

type escape struct {
	card *Card
}
`},
			allowlist: []readerExemption{{path: "internal/bench/check.go", references: 0}},
			want:      []string{"a package-level var holding a *Card", "the struct field card holding a *Card"},
			count:     2,
		},
		{
			name: "package-level storage with its type elided",
			files: map[string]string{"internal/bench/check.go": `var fromReader, readerErr = LoadCard("cards", "andon-1")
var built = &Card{}
var fresh = new(Card)
var held = []*Card{}
`},
			allowlist: oneProbeEntry(),
			want:      []string{"a package-level var holding a *Card"},
			count:     4,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for planted, source := range c.files {
				if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(path.Dir(planted))), 0o755); err != nil {
					t.Fatalf("plant %s: %v", planted, err)
				}
				clause := "package " + path.Base(path.Dir(planted)) + "\n\n"
				if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(planted)), []byte(clause+source), 0o644); err != nil {
					t.Fatalf("plant %s: %v", planted, err)
				}
			}
			findings, err := scanForFreeCardReaders(dir, "", c.allowlist)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			violations := readerViolations(findings, c.allowlist)
			for _, want := range c.want {
				found := false
				for _, violation := range violations {
					if strings.Contains(violation, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("the guard over the planted %s answered %v and no violation carries %q", c.name, violations, want)
				}
			}
			if len(violations) != c.count {
				t.Errorf("the guard over the planted %s answered %d violations and the shape owes %d: %v", c.name, len(violations), c.count, violations)
			}
		})
	}
}
