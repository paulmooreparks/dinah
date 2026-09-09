package verb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The grounds a view may be exempted on. The set is closed, and a ground
// outside it fails the test rather than passing unread, so an exemption is an
// argument a reviewer weighs rather than a line anybody can add.
const (
	// groundIdentifierResolves is a view whose identifier is itself accepted
	// in the head position of a reference, so a client holding the view can
	// name the entity from what it already has.
	groundIdentifierResolves = "identifier-resolves"
	// groundNotAnEntityView is a report about a run rather than a view of an
	// entity, so there is nothing standing for it to address.
	groundNotAnEntityView = "not-an-entity-view"
)

// viewExemption is one struct the address rule does not reach, with the ground
// it is excused on and the reasoning behind it.
type viewExemption struct {
	typeName string
	ground   string
	reason   string
}

// viewExemptions are the views carrying an identifier and no reference, each
// argued rather than assumed. dinah-454 D-6 is where these were decided.
var viewExemptions = []viewExemption{
	{
		typeName: "ColumnView", ground: groundIdentifierResolves,
		reason: "a column's identifier is accepted in the head position of a reference, as are its slug and its title, " +
			"so a client holding the view can name the column. A comment is the contrasting case, because a comment's " +
			"identifier resolves only in the selector slot of a path that already names the card.",
	},
	{
		typeName: "ReshapeColumn", ground: groundNotAnEntityView,
		reason: "a reshape preview reports what a run would do, and the column it names does not exist yet.",
	},
	{
		typeName: "ReshapeRetirement", ground: groundNotAnEntityView,
		reason: "a retirement entry reports what a run would retire rather than viewing an entity.",
	},
	{
		typeName: "RemintReport", ground: groundNotAnEntityView,
		reason: "a remint report reports what one run did rather than viewing an entity.",
	},
}

// TestEveryViewCarryingAnIdentifierCarriesAReference asserts dinah-454 AC-9:
// no view carrying an entity's identifier can reach a green build without a
// reference beside it or an argued exemption.
//
// The package's own non-test sources are parsed, which is internal/verb/*.go
// with every path ending _test.go skipped. The rule is about the views the
// product ships, and a test fixture declaring a struct with an identifier
// would otherwise redden a guard about production views for a reason that has
// nothing to do with the rule.
//
// This guard is sound only beside dinah-454 AC-3. On its own it is satisfied
// by adding a view to the list above with a plausible ground, and AC-3 is what
// forbids that for the comment, because it asserts against a real CommentView
// that its reference resolves to the comment.
func TestEveryViewCarryingAnIdentifierCarriesAReference(t *testing.T) {
	fset := token.NewFileSet()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package's sources: %v", err)
	}
	carriesRef := map[string]bool{}
	var identified []string
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, source, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", source, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			decl, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			structure, ok := decl.Type.(*ast.StructType)
			if !ok {
				return true
			}
			hasID, hasRef := false, false
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					switch name.Name {
					case "ID":
						hasID = jsonTagOf(field) == "id"
					case "Ref":
						hasRef = true
					}
				}
			}
			if hasID {
				identified = append(identified, decl.Name.Name)
				carriesRef[decl.Name.Name] = hasRef
			}
			return true
		})
	}
	if len(identified) == 0 {
		t.Fatal("the walk found no view carrying an identifier, so it read nothing and this guard proves nothing")
	}

	excused := map[string]viewExemption{}
	for _, exemption := range viewExemptions {
		if exemption.ground != groundIdentifierResolves && exemption.ground != groundNotAnEntityView {
			t.Errorf("%s is excused on the ground %q, which is outside the declared set", exemption.typeName, exemption.ground)
		}
		if _, found := carriesRef[exemption.typeName]; !found {
			t.Errorf("%s is excused, and the walk finds no view of that name carrying an identifier, so the exemption is stale", exemption.typeName)
		}
		excused[exemption.typeName] = exemption
	}
	sort.Strings(identified)
	for _, name := range identified {
		if carriesRef[name] {
			continue
		}
		if _, ok := excused[name]; !ok {
			t.Errorf("verb.%s declares ID with the json tag id, declares no Ref, and is named in no exemption", name)
		}
	}
}

// jsonTagOf is the json name one struct field's tag carries, and the empty
// string where it carries no tag or no json key. The options after the name
// are dropped, since the rule is about which key the field publishes under.
func jsonTagOf(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	unquoted, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return ""
	}
	name, _, _ := strings.Cut(reflect.StructTag(unquoted).Get("json"), ",")
	return name
}
