//go:build tui

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/msg"
)

// TestEveryBoundDescriptionIsTheCatalogText is the first half of
// dinah-603/criteria/21. It reads every binding the head defines, in every
// mode, through interactiveBindings, and for each of the eight languages holds
// each binding's description to that language's catalog text for the key it
// names, with the arrow and column values filled from the fixed column. It
// counts the bindings it read in each mode and fails at zero.
func TestEveryBoundDescriptionIsTheCatalogText(t *testing.T) {
	tags := msg.Tags()
	if len(tags) != 8 {
		t.Fatalf("the tool carries %d languages, and this test is written for eight", len(tags))
	}
	for _, tag := range tags {
		r := msg.For(tag)
		perMode := map[string]int{}
		for _, b := range interactiveBindings(r) {
			perMode[b.mode]++
			want := r.T(b.key, b.values...)
			if got := b.binding.Help().Desc; got != want {
				t.Errorf("%s: the %s binding for %v is described %q, and the catalog key %s reads %q", tag, b.mode, b.binding.Keys(), got, b.key, want)
			}
		}
		for _, mode := range []string{bindingBrowse, bindingCard, bindingMenu, bindingJump, bindingComment} {
			if perMode[mode] == 0 {
				t.Errorf("%s: read no binding in %s mode", tag, mode)
			}
		}
		t.Logf("%s: %v", tag, perMode)
	}
}

// interactiveProse matches a literal carrying a space and three consecutive
// letters, which is prose rather than a key name or a separator.
var interactiveProse = regexp.MustCompile(`[A-Za-z]{3}`)

// TestTheInteractiveHeadDrawsNoLiteralProse is dinah-603/criteria/22. It
// parses every non-test cmd/dinah/interactive*.go and fails on any string
// literal passed as the description of key.WithHelp, whatever its length,
// and on any other string literal outside an import or a struct tag that
// carries a space and three consecutive letters. It counts the files it read
// and fails at zero.
func TestTheInteractiveHeadDrawsNoLiteralProse(t *testing.T) {
	files, err := filepath.Glob("interactive*.go")
	if err != nil {
		t.Fatal(err)
	}
	read := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		read++
		described := map[token.Pos]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "WithHelp" || !isBareIdentCall(selector.X, "key") {
				return true
			}
			if literal, ok := call.Args[1].(*ast.BasicLit); ok {
				described[literal.Pos()] = true
				t.Errorf("%s:%d: key.WithHelp is given the literal description %s", name, fset.Position(literal.Pos()).Line, literal.Value)
			}
			return true
		})
		skip := map[token.Pos]bool{}
		for _, imported := range file.Imports {
			skip[imported.Path.Pos()] = true
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if field, ok := node.(*ast.Field); ok && field.Tag != nil {
				skip[field.Tag.Pos()] = true
			}
			return true
		})
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING || skip[literal.Pos()] || described[literal.Pos()] {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}
			if strings.Contains(value, " ") && interactiveProse.MatchString(value) {
				t.Errorf("%s:%d: the literal %s is prose that never passed through the catalog", name, fset.Position(literal.Pos()).Line, literal.Value)
			}
			return true
		})
	}
	t.Logf("%d files read", read)
	if read == 0 {
		t.Fatal("no interactive source was read, so this guard proves nothing")
	}
}
