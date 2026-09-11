package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/addressform"
)

// dinah-487's standing guard over every route by which a card can be resolved
// by number against the archived half of the cards collection.
//
// The sweep that established the three fall-through sites this card removed is
// a paragraph on the card, and a paragraph ages. This is that enumeration
// turned into a test, modelled on address_form_arms_test.go, which is the
// repository's worked precedent for a check that parses the tree and asserts a
// declared set against what the AST holds.
//
// Six families on two axes. Families A, B, and D are the REACHING axis, which
// is every name by which code gets at the archived half. Families C, E, and F
// are the READING axis, which is every name by which code gets at a card's
// number. A route that reaches the archive but reads no number cannot answer a
// number, and a route that reads a number but never reaches the archive cannot
// answer an archived card, so an attacker has to evade one family on each axis
// rather than one family in total.
//
// Each family collects MENTIONS of a name rather than calls of a shape. A rule
// written on the syntax around a name is defeated by hoisting an argument into
// a local or by taking a value from a helper, and both of those are ordinary
// refactors that the tree already contains. A name, wherever it appears, is
// still the name.
//
// Families A through E read identifier nodes, never comments and never string
// literals. Family F reads a string literal, because a frontmatter key is one
// and nothing else. Thirty-three lines in the tree would be false positives
// for a guard written as a substring grep over the nine identifier names
// that families A, B, and D collect: ten in internal/addressform, one in
// internal/guide/guidepin, eighteen comment lines inside internal/bench, and
// four comment lines inside internal/verb. The identifier-node rule excludes
// all of them.
//
// Each row asserts its own mention count. Without that, a second resolution
// added inside a function the table already recognises changes no set and
// fails nothing, which would leave the guard blind to a new fall-through
// written in the one place a reader is least likely to look for it.
//
// WHAT THIS GUARD CANNOT SEE. Six limits, written down rather than left in a
// reviewer's head.
//
//  1. The floor is a route that reaches the archived anchors while naming none
//     of ArchivedCardsRoot, cardsRootIn, ArchiveDir, resolveCardIn,
//     ResolveArchivedCard, or the three half-taking resolvers, AND reads a
//     card's number while naming none of the Number field, a one-argument Ref
//     call, or the literal "number". Both halves have to hold at once, and
//     neither half has to be unusual. The reviewer who built these rules as a
//     working program defeated them twice: once by composing a reach through
//     WatchedEntities with a read through a Card.Ref method value, two
//     constructions this card's spec judges harmless in isolation, and once by
//     reading an archived card's printed reference out of the change listing's
//     own GoneEntity.Ref field, which is exported API carrying no string
//     literal, no method value and no hand-assembled path. Both routes are
//     open. The guard ships with them open because its six rules are correct
//     and closing that hole needs a call graph, which is a larger instrument
//     than this card should carry.
//  2. Family E is defeated on its own by a method value: `render := card.Ref`
//     on one line and `render(slug)` on another is not a call whose callee is
//     a selector named Ref, and a guard parsing without type information
//     cannot tell that method value from a read of a struct field named Ref,
//     which the tree does at ninety-six non-call selector sites.
//  3. Family B collects ArchivedHalf outside internal/bench only. Inside the
//     package the identifier appears on twenty-three non-comment lines, nearly
//     all of them a comparison against a parameter, so collecting it would
//     drown the table in rows carrying no information. In-package code that
//     chooses the archived half names one of family D's anchors or hands the
//     half to one of family B's three resolvers.
//  4. The guard reports source shape and never behaviour. It says a site
//     exists, not that the site handles the refusal correctly. The plants
//     dinah-487 AC-8 calls for are what covers that.
//  5. Recognising a function does not follow that function's callers. A helper
//     wrapping a half-taking resolver is collected at the helper, so the
//     helper's own row has to be argued and its callers are then invisible.
//     This is the mechanism behind limit 1. The per-row counts are what force
//     the argument to be made again when such a helper's body changes.
//  6. Generated code and reflection are outside the guard entirely. It reads
//     the .go files in the tree as they are committed.

// resolutionSite is one recognised place a family's names are mentioned, with
// the count this tree holds and the reason the row is allowed to be there.
type resolutionSite struct {
	file     string
	function string
	mentions int
	why      string
}

// resolutionFamily is one collection rule, its axis, and the set it asserts.
// The three totals are declared rather than computed from the rows, because a
// sweep says how big its set was and a row deleted to make a failure go away
// should stop the run rather than shrink the subject set in silence.
type resolutionFamily struct {
	name      string
	axis      string
	what      string
	files     int
	mentions  int
	functions int
	sites     []resolutionSite
}

// packageLevel is the enclosing site of a mention standing in a declaration
// rather than in a function body, which family D meets twice.
const packageLevel = "<package-level>"

// benchPackageOwnFile names the file inside internal/bench that is called
// after the package itself, composed from addressform.PackageDir rather than
// spelled out. internal/profile's vocabulary guard refuses the short form of
// the product's word inside a Go string literal, and the remedy it names is to
// rewrite the text rather than widen its exceptions, so this is derived here
// the way internal/addressform derives the same name for the same reason.
var benchPackageOwnFile = addressform.PackageDir + "/" + path.Base(addressform.PackageDir) + ".go"

// benchTypeName is the receiver type family B reads its derived name set off.
const benchTypeName = "Bench" // retired spelling, named deliberately

// archivedResolutionFamilies is the whole of the guard's declared knowledge.
var archivedResolutionFamilies = []resolutionFamily{
	{
		name: "A", axis: "reaching",
		what:      "card resolution by number against a chosen root: every mention of resolveCardIn or ResolveArchivedCard",
		files:     2,
		mentions:  10,
		functions: 8,
		sites: []resolutionSite{
			{"internal/bench/resolve.go", "ResolveCard", 1, "the live accessor, named rather than filtered out because the rule no longer inspects the root argument"},
			{"internal/bench/resolve.go", "ResolveArchivedCard", 2, "the exported archived accessor, whose declaration's own name is the first mention"},
			{"internal/bench/resolve.go", "resolveCardIn", 1, "the declaration of the function this family is about"},
			{"internal/bench/resolve.go", "resolveReferenceBody", 1, "the parameterised route, propagating the resolver's error"},
			{"internal/bench/resolve.go", "resolveBelowLanding", 1, "the same parameterised route for a below-a-card reference"},
			{"internal/bench/resolve.go", "ResolveLinkTarget", 1, "reads both halves and unwraps the ambiguity out of each"},
			{"internal/bench/resolve.go", "probe", 2, "the argued fall-through, which answers no card to any caller; the reasoning is on dinah-487 and the row exists so a reader who gives probe a second caller meets it"},
			{"internal/verb/changes.go", "watchedCard", 1, "the changes card filter, which unwraps the ambiguity out of each half"},
		},
	},
	{
		name: "B", axis: "reaching",
		what:      "the half-taking resolvers anywhere, and ArchivedHalf named from outside internal/bench",
		files:     6,
		mentions:  17,
		functions: 11,
		sites: []resolutionSite{
			{"cmd/dinah/commands.go", "runPath", 2, "the --archived flag of dinah path, propagating the resolver's error"},
			{"internal/bench/entity.go", "ResolveEntity", 1, "the one-line live delegate for an entity"},
			{"internal/bench/entity.go", "ResolveEntityIn", 2, "the declaration, and the in-package call that reaches card resolution through resolveReferenceBody"},
			{"internal/bench/resolve.go", "ResolvePath", 1, "the one-line live delegate, which chooses LiveHalf"},
			{"internal/bench/resolve.go", "ResolvePathIn", 1, "the declaration of a collected name"},
			{"internal/bench/resolve.go", "ResolveReference", 1, "the one-line live delegate for a reference"},
			{"internal/bench/resolve.go", "ResolveReferenceIn", 1, "the declaration of a collected name"},
			{"internal/verb/beyond.go", "Restore", 2, "restores an entity from the mirror, propagating the error"},
			{"internal/verb/beyond.go", "halfFor", 1, "the flag-to-half mapping, which is where the archived half is minted outside the package"},
			{"internal/verb/read.go", "Show", 4, "three resolutions, one spelling the half out and two taking it from halfFor; the discard-and-retry re-raises the same refusal on the next line"},
			{"internal/verb/tree.go", "Contents", 1, "under --archived this resolves a card by number against the archived half while naming no half at all, which is why the rule is keyed on the resolver"},
		},
	},
	{
		name: "C", axis: "reading",
		what:      "reading a card's number field: every mention of Number in a selector expression",
		files:     5,
		mentions:  15,
		functions: 9,
		sites: []resolutionSite{
			{benchPackageOwnFile, "NextNumber", 2, "minting takes the highest in use"},
			{"internal/bench/blockjson.go", "jsonNumber", 1, "a false positive recognised by name: the selector is the type json.Number, and telling it apart would mean running go/types over the tree to remove one row"},
			{"internal/bench/card.go", "loadCard", 1, "parses the number in"},
			{"internal/bench/card.go", "Ref", 2, "composes the human reference, which is why family E exists"},
			{"internal/bench/card.go", "Save", 2, "writes the number back to frontmatter"},
			{"internal/bench/card.go", "ByArrival", 2, "sorts on it"},
			{"internal/bench/check.go", "checkCard", 1, "guards a finding against a card carrying no number"},
			{"internal/bench/check.go", "checkCardNumbers", 3, "the duplicate-number finding, which also groups by number without comparing anything, so a rule keyed on a comparison would walk past a number scan the tree already holds"},
			{"internal/bench/resolve.go", "resolveCardIn", 1, "the scan this card repaired, and the only one of these that turns a number into an answer to a reference"},
		},
	},
	{
		name: "D", axis: "reaching",
		what:      "reaching the archived anchors: every mention of ArchivedCardsRoot, cardsRootIn or ArchiveDir",
		files:     11,
		mentions:  29,
		functions: 24,
		sites: []resolutionSite{
			{benchPackageOwnFile, packageLevel, 1, "the declaration of the directory name, standing in the file's const block"},
			{benchPackageOwnFile, "ArchivedCardsRoot", 2, "the accessor this family is named for, and its own body's use of the directory constant"},
			{benchPackageOwnFile, "ArchivedColumnsRoot", 1, "the columns half of the mirror, which holds no cards"},
			{benchPackageOwnFile, "HasIdentifier", 1, "existence across both halves, which reads no number"},
			{benchPackageOwnFile, "NextNumber", 1, "minting, which answers no card; family C carries its number reads"},
			{"internal/bench/changes.go", "WatchedEntities", 2, "fingerprinting, which lists identifiers and reads journals rather than resolving a reference"},
			{"internal/bench/check.go", "checkCardNumbers", 2, "the duplicate-number finding, which reports rather than resolves"},
			{"internal/bench/container.go", packageLevel, 1, "the workbench's member list, used when a whole workbench moves"},
			{"internal/bench/entity.go", "ArchiveTarget", 1, "where an entity directory goes when it is archived; it answers a path for a directory the caller already holds"},
			{"internal/bench/entity.go", "refBelowHead", 2, "rendering a reference for a directory below a head, which reads no number"},
			{"internal/bench/resolve.go", "ResolveArchivedCard", 1, "the archived accessor, which family A also carries; it is a reaching site by one rule and a resolution site by the other"},
			{"internal/bench/resolve.go", "resolveReferenceBody", 1, "the parameterised route family A also carries"},
			{"internal/bench/resolve.go", "resolveBelowLanding", 1, "the same, for a below-a-card reference"},
			{"internal/bench/resolve.go", "descend", 1, "the collection walk, which builds a mounted collection's archived half at each step and resolves nothing by number"},
			{"internal/bench/resolve.go", "cardsRootIn", 2, "the half-to-root mapping, and its own body's use of the accessor"},
			{"internal/bench/resolve.go", "ArchivedColumnByRef", 1, "a column lookup in the mirror, which reads no card"},
			{"internal/bench/resolve.go", "probe", 1, "the argued diagnostic fall-through, which family A also carries"},
			{"internal/bench/resolve.go", "probeBelow", 1, "the diagnostic walk's own descent, reached only from probe"},
			{"internal/bench/vocabulary.go", "migrateCardVocabulary", 1, "the archived cards root assembled from the constants rather than taken from the accessor, which is the whole reason ArchiveDir is collected"},
			{"internal/bench/vocabulary.go", "migrateColumnDirectories", 1, "the same migration for columns"},
			{"internal/bench/workstream.go", "ArchivedWorkstreamsRoot", 1, "the workstreams half of the mirror, which holds no cards"},
			{"internal/verb/changes.go", "anchorOf", 1, "an identifier-keyed live-then-archive pair, which parses no number"},
			{"internal/verb/read.go", "linkRef", 1, "the other identifier-keyed pair, harmless for the same reason"},
			{"internal/verb/search.go", "Search", 1, "searching the archived half, which matches text against loaded cards and answers no reference by number"},
		},
	},
	{
		name: "E", axis: "reading",
		what:      "rendering a card's human reference: every call whose selector is Ref and which carries exactly one argument",
		files:     8,
		mentions:  16,
		functions: 14,
		sites: []resolutionSite{
			{"internal/bench/check.go", "checkTierOverrides", 1, "naming a card in a finding"},
			{"internal/bench/check.go", "checkItemColumns", 1, "naming a card in a finding"},
			{"internal/bench/resolve.go", "resolveReferenceBody", 2, "composing the reference a resolution answers with"},
			{"internal/verb/changes.go", "goneFrom", 1, "filling a departed card's printed reference, which then travels outward on GoneEntity.Ref; limit 1 is about exactly this"},
			{"internal/verb/changes.go", "entityRef", 1, "the shared renderer for an entity's printed reference, which also travels outward"},
			{"internal/verb/library.go", "view", 1, "rendering a card for a caller"},
			{"internal/verb/pull.go", "Pull", 1, "naming the card a pull took up"},
			{"internal/verb/read.go", "detailOf", 1, "rendering a card's detail"},
			{"internal/verb/read.go", "linkRef", 2, "rendering a link's target in either half"},
			{"internal/verb/search.go", "searchCard", 1, "rendering a search hit"},
			{"internal/verb/tree.go", "cardNode", 1, "rendering a card leaf"},
			{"internal/verb/tree.go", "rootOf", 1, "rendering the root of a tree"},
			{"internal/verb/tree.go", "itemRefOf", 1, "rendering a checklist item's holder"},
			{"internal/verb/tree.go", "containedNode", 1, "rendering a contained entity"},
		},
	},
	{
		name: "F", axis: "reading",
		what:      "reading the number out of frontmatter: every basic string literal \"number\"",
		files:     2,
		mentions:  3,
		functions: 3,
		sites: []resolutionSite{
			{"internal/bench/card.go", "loadCard", 1, "fm.Value(\"number\"), which is where the field is read in"},
			{"internal/bench/card.go", "Save", 1, "c.FM.Set(\"number\", ...), which is where it is written back"},
			{"internal/verb/beyond.go", "Add", 1, "fm.Set(\"number\", ...), which is where a new card's number is minted onto its anchor"},
		},
	},
}

// resolutionMention is one mention the walk found.
type resolutionMention struct {
	family   string
	file     string
	line     int
	function string
	name     string
}

// parsedSource is one non-test file of the tree, with the path it is reported
// under.
type parsedSource struct {
	rel  string
	file *ast.File
}

// readTreeSources parses every non-test .go file under the repository root and
// says how many it read, because a scan that read no file collects nothing and
// would otherwise report six empty families as six satisfied ones.
func readTreeSources(t *testing.T, fileset *token.FileSet) []parsedSource {
	t.Helper()
	var sources []parsedSource
	skipped := map[string]bool{".git": true, "node_modules": true, "vendor": true}
	err := filepath.Walk(repositoryRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipped[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fileset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		rel, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		sources = append(sources, parsedSource{rel: filepath.ToSlash(rel), file: parsed})
		return nil
	})
	if err != nil {
		t.Fatalf("reading the tree: %v", err)
	}
	if len(sources) < 50 {
		t.Fatalf("the walk read %d non-test files, and the tree holds far more, so the scan found the wrong directory", len(sources))
	}
	return sources
}

// enclosingName answers the function a position stands in, or packageLevel
// when it stands in a declaration instead.
func enclosingName(file *ast.File, pos token.Pos) string {
	name := ""
	for _, declared := range file.Decls {
		if declared.Pos() > pos || pos > declared.End() {
			continue
		}
		switch typed := declared.(type) {
		case *ast.FuncDecl:
			return typed.Name.Name
		case *ast.GenDecl:
			name = packageLevel
		}
	}
	return name
}

// benchMethodsTakingAHalf answers the name of every exported method of *Bench
// whose first parameter is a ResolutionHalf, read out of internal/bench's own
// declarations. Family B derives its callee set from this rather than from a
// written list, so a fourth such resolver joins the collected set on the run
// after it is declared and is reported before any caller exists.
func benchMethodsTakingAHalf(sources []parsedSource) []string {
	var names []string
	for _, source := range sources {
		if !strings.HasPrefix(source.rel, "internal/bench/") {
			continue
		}
		for _, declared := range source.file.Decls {
			function, ok := declared.(*ast.FuncDecl)
			if !ok || !isBenchMethod(function) || !function.Name.IsExported() {
				continue
			}
			if function.Type.Params == nil || len(function.Type.Params.List) == 0 {
				continue
			}
			named, ok := function.Type.Params.List[0].Type.(*ast.Ident)
			if ok && named.Name == "ResolutionHalf" {
				names = append(names, function.Name.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// isBenchMethod reports whether a declaration is a method on *Bench.
func isBenchMethod(function *ast.FuncDecl) bool {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return false
	}
	pointer, ok := function.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	named, ok := pointer.X.(*ast.Ident)
	return ok && named.Name == benchTypeName
}

// refMethodsTakingAParameter answers every method named Ref declared in
// internal/bench together with how many parameters it takes. Family E
// discriminates on the argument count, which is sound without type information
// only while Card.Ref is the one Ref method that takes a parameter, so the
// guard asserts that from the declarations rather than assuming it.
func refMethodsTakingAParameter(sources []parsedSource) map[string]int {
	counts := map[string]int{}
	for _, source := range sources {
		if !strings.HasPrefix(source.rel, "internal/bench/") {
			continue
		}
		for _, declared := range source.file.Decls {
			function, ok := declared.(*ast.FuncDecl)
			if !ok || function.Name.Name != "Ref" || function.Recv == nil || len(function.Recv.List) == 0 {
				continue
			}
			receiver := function.Recv.List[0].Type
			if pointer, ok := receiver.(*ast.StarExpr); ok {
				receiver = pointer.X
			}
			named, ok := receiver.(*ast.Ident)
			if !ok {
				continue
			}
			taken := 0
			if function.Type.Params != nil {
				for _, field := range function.Type.Params.List {
					taken += len(field.Names)
				}
			}
			counts[named.Name] = taken
		}
	}
	return counts
}

// collectResolutionMentions runs all six collection rules over the tree in one
// walk of each file.
func collectResolutionMentions(sources []parsedSource, fileset *token.FileSet, derived []string) []resolutionMention {
	familyA := map[string]bool{"resolveCardIn": true, "ResolveArchivedCard": true}
	familyD := map[string]bool{"ArchivedCardsRoot": true, "cardsRootIn": true, "ArchiveDir": true}
	familyB := map[string]bool{}
	for _, name := range derived {
		familyB[name] = true
	}
	var found []resolutionMention
	for _, source := range sources {
		source := source
		inBench := strings.HasPrefix(source.rel, "internal/bench/")
		record := func(family string, pos token.Pos, name string) {
			found = append(found, resolutionMention{
				family:   family,
				file:     source.rel,
				line:     fileset.Position(pos).Line,
				function: enclosingName(source.file, pos),
				name:     name,
			})
		}
		ast.Inspect(source.file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.Ident:
				if familyA[typed.Name] {
					record("A", typed.Pos(), typed.Name)
				}
				if familyB[typed.Name] {
					record("B", typed.Pos(), typed.Name)
				}
				if typed.Name == "ArchivedHalf" && !inBench {
					record("B", typed.Pos(), typed.Name)
				}
				if familyD[typed.Name] {
					record("D", typed.Pos(), typed.Name)
				}
			case *ast.SelectorExpr:
				if typed.Sel.Name == "Number" {
					record("C", typed.Sel.Pos(), "Number")
				}
			case *ast.CallExpr:
				if selector, ok := typed.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Ref" && len(typed.Args) == 1 {
					record("E", selector.Sel.Pos(), "Ref")
				}
			case *ast.BasicLit:
				if typed.Kind == token.STRING && typed.Value == `"number"` {
					record("F", typed.Pos(), `"number"`)
				}
			}
			return true
		})
	}
	return found
}

// TestEveryArchivedCardResolutionIsDeclared is dinah-487 AC-10.
func TestEveryArchivedCardResolutionIsDeclared(t *testing.T) {
	fileset := token.NewFileSet()
	sources := readTreeSources(t, fileset)

	derived := benchMethodsTakingAHalf(sources)
	wantedDerived := []string{"ResolveEntityIn", "ResolvePathIn", "ResolveReferenceIn"}
	if strings.Join(derived, ",") != strings.Join(wantedDerived, ",") {
		t.Fatalf("internal/bench declares the half-taking resolvers %v and the guard's family B is written against %v; a new one joins the set here before any caller exists", derived, wantedDerived)
	}

	refMethods := refMethodsTakingAParameter(sources)
	for receiver, taken := range refMethods {
		if (receiver == "Card") != (taken == 1) {
			t.Errorf("%s.Ref takes %d parameters, and family E discriminates on the argument count only while Card.Ref is the one Ref method that takes one", receiver, taken)
		}
	}
	if refMethods["Card"] != 1 {
		t.Fatalf("internal/bench declares no Card.Ref taking one parameter, so family E's rule no longer names what it was written for")
	}

	found := collectResolutionMentions(sources, fileset, derived)
	byFamily := map[string][]resolutionMention{}
	for _, mention := range found {
		byFamily[mention.family] = append(byFamily[mention.family], mention)
	}

	for _, family := range archivedResolutionFamilies {
		family := family
		t.Run(family.name, func(t *testing.T) {
			recognised := map[string]int{}
			for _, site := range family.sites {
				key := site.file + "|" + site.function
				if _, already := recognised[key]; already {
					t.Fatalf("the table declares %s twice, so its own count is the sum of two rows rather than one site's", key)
				}
				recognised[key] = site.mentions
			}
			declaredFiles := map[string]bool{}
			declaredMentions := 0
			for _, site := range family.sites {
				declaredFiles[site.file] = true
				declaredMentions += site.mentions
			}
			if len(declaredFiles) != family.files || declaredMentions != family.mentions || len(family.sites) != family.functions {
				t.Fatalf("family %s declares %d mentions over %d files and %d enclosing sites, and its rows add up to %d over %d and %d",
					family.name, family.mentions, family.files, family.functions, declaredMentions, len(declaredFiles), len(family.sites))
			}

			seen := map[string]int{}
			files := map[string]bool{}
			for _, mention := range byFamily[family.name] {
				key := mention.file + "|" + mention.function
				files[mention.file] = true
				seen[key]++
				if _, known := recognised[key]; !known {
					t.Errorf("family %s (%s, the %s axis) collects %s at %s:%d in %s, and that site is in no declared row: either it is a new route into an archived card's number, which needs the unwrap dinah-487 gave ResolveLinkTarget and watchedCard, or it is harmless and needs a row here saying so",
						family.name, family.what, family.axis, mention.name, mention.file, mention.line, mention.function)
				}
			}
			for _, site := range family.sites {
				key := site.file + "|" + site.function
				got := seen[key]
				if got == site.mentions {
					continue
				}
				if got == 0 {
					t.Errorf("family %s declares %s in %s with %d mentions (%s) and the tree holds none, so the row is stale",
						family.name, site.file, site.function, site.mentions, site.why)
					continue
				}
				t.Errorf("family %s declares %d mentions in %s %s (%s) and the tree holds %d; a mention added inside a recognised function is exactly the place a new fall-through hides, so read the body before moving the count",
					family.name, site.mentions, site.file, site.function, site.why, got)
			}
			if len(byFamily[family.name]) != family.mentions || len(files) != family.files || len(seen) != family.functions {
				t.Errorf("family %s collected %d mentions over %d files and %d enclosing sites, and it declares %d over %d and %d",
					family.name, len(byFamily[family.name]), len(files), len(seen), family.mentions, family.files, family.functions)
			}
		})
	}
}
