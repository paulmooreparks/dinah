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
		files:     3,
		mentions:  11,
		functions: 9,
		sites: []resolutionSite{
			{"internal/verb/list.go", "history", 1, "list reading a journal under --archived, which resolves the card in the mirror and then reads the file that card carried in with it; it answers events rather than a card and composes no reference"},
			{"internal/bench/resolve.go", "ResolveCard", 1, "the live accessor, named rather than filtered out because the rule no longer inspects the root argument"},
			{"internal/bench/resolve.go", "ResolveArchivedCard", 2, "the exported archived accessor, whose declaration's own name is the first mention"},
			{"internal/bench/resolve.go", "resolveCardIn", 1, "the declaration of the function this family is about; its number branch reads the registry's by-number index rather than a card's field, which is why family C no longer carries it"},
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
		files:     7,
		mentions:  22,
		functions: 14,
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
			{"internal/bench/resolve.go", "CollectionRootIn", 1, "composes the directory one top-level collection occupies in a half, which reaches no card and resolves no reference"},
			{"internal/verb/list.go", "ListRef", 1, "the one resolution list performs, taking the half from halfFor exactly as show does"},
			{"internal/verb/list.go", "Rosters", 3, "the roster counts, which name the half and compose each collection's directory in it; they count directory entries and reach no card"},
		},
	},
	{
		name: "C", axis: "reading",
		what:      "reading a card's number field: every mention of Number in a selector expression",
		files:     8,
		mentions:  33,
		functions: 12,
		sites: []resolutionSite{
			{"internal/bench/blockjson.go", "jsonNumber", 1, "a false positive recognised by name: the selector is the type json.Number, and telling it apart would mean running go/types over the tree to remove one row"},
			{"internal/bench/card.go", "Ref", 2, "composes the human reference, which is why family E exists"},
			{"internal/bench/card.go", "ByArrival", 2, "sorts on it"},
			{"internal/bench/card.go", "stamp", 2, "the stamping reader every number read now flows through: above the registry's format it takes the number from the by-identifier index, and below it from the frontmatter key the legacy read path still uses"},
			{"internal/bench/check.go", "checkCardNumbers", 3, "the registry's own audit: the malformed-line guards and the duplicate finding, which weighs the by-number index without comparing anything, so a rule keyed on a comparison would walk past a number scan the tree already holds"},
			{"internal/bench/numbermigrate.go", "MigrateNumbers", 5, "the number migration, which reads each line's number to decide which card keeps a colliding one; it composes a file and answers no reference"},
			{"internal/bench/numbermigrate.go", "RenumberCards", 4, "the duplicate repair, which counts the claims each number holds and moves a later claimant above the high-water mark; it rewrites lines and answers no card"},
			{"internal/bench/numbers.go", "LoadNumberRegistry", 6, "the registry loader, which reads each line's number into the two indexes and the high-water mark; the registry is this file's whole subject"},
			{"internal/bench/numbers.go", "readNumbers", 3, "the below-format synthesis, which reads a stamped card's number into the by-number index so resolution keeps answering over a workbench the migration has not reached"},
			{"internal/verb/beyond.go", "tombstoneNumber", 1, "a deletion rewriting the first line claiming the card into the tombstone, which keeps the number allocated; it writes a line and answers no card"},
			{"internal/setup/render.go", "readValue", 1, "a false positive recognised by name: the selector is the type json.Number, read while dinah setup parses a recipe step's JSON value, which holds no card"},
			{"internal/lsp/handlers.go", "cardCandidates", 3, "the language server's card completion, which sorts the candidates newest first and composes a zero-padded sort key from the number; the list it sorts is the live half alone, so no archived card reaches these reads and no reference is resolved by number"},
		},
	},
	{
		name: "D", axis: "reaching",
		what:      "reaching the archived anchors: every mention of ArchivedCardsRoot, cardsRootIn or ArchiveDir",
		files:     19,
		mentions:  47,
		functions: 37,
		sites: []resolutionSite{
			{"internal/bench/commentcheck.go", "commentDirOf", 1, "reading the archived half of one item's comments, because archiving a designated comment stays permitted and the item goes on citing it wherever it now lives; it reaches a comment below a card it was handed and resolves no card"},
			{"internal/bench/commentcheck.go", "checkMissingDesignations", 1, "reading the archived half of one card's checklist, because an archived item settled without an answer says what a live one says; the card came off check's own live walk and no card is resolved here"},
			{"internal/bench/designationmigrate.go", "MigrateDesignations", 1, "walking the archived cards as well as the live ones, which is the whole of why the conversion leaves no positional answer behind for a later restore to put back into a converted store"},
			{"internal/bench/designationmigrate.go", "everyItem", 1, "reading the archived half of one card's checklist, for MigrateDesignations' reason"},
			{"internal/bench/designationmigrate.go", "commentsOfItem", 1, "reading the archived half of one item's comments, because the conversion asks whether an identifier belongs to this item rather than where it sits"},
			{"internal/bench/designationmigrate.go", "commentsElsewhere", 2, "reading the archived half of the card's own comments and of every other item's, which is what lets an unrelated tidy stop making every item on the card unrecoverable"},
			{"cmd/dinah-migrate-notes/main.go", "classify", 1, "the note migration, which walks both halves because an archived card carries items whose notes have to be carried too; it reads each card by identifier from the root it is walking and resolves no reference"},
			{"internal/bench/newlinemigrate.go", "lockDirForFile", 2, "the newline repair's file-to-lock mapping, which composes the archived cards root and the archived workstreams root from the constants so that a file inside an archived card takes that card's own lock; it answers a directory to lock and resolves no reference"},
			{benchPackageOwnFile, packageLevel, 1, "the declaration of the directory name, standing in the file's const block"},
			{benchPackageOwnFile, "ArchivedCardsRoot", 2, "the accessor this family is named for, and its own body's use of the directory constant"},
			{"internal/bench/resolve.go", "CollectionRootIn", 1, "composes a top-level collection's directory under the archive mirror, which reaches no card"},
			{benchPackageOwnFile, "ArchivedColumnsRoot", 1, "the columns half of the mirror, which holds no cards"},
			{benchPackageOwnFile, "HasIdentifier", 1, "existence across both halves, which reads no number"},
			{"internal/bench/changes.go", "WatchedEntities", 2, "fingerprinting, which lists identifiers and reads journals rather than resolving a reference"},
			{"internal/bench/check.go", "checkCardNumbers", 4, "the registry's own audit, which reaches the archived half twice: the stranded probe loads each card a line claims, and the closing walk lists both collections; it reports rather than resolves"},
			{"internal/bench/container.go", packageLevel, 1, "the workbench's member list, used when a whole workbench moves"},
			{"internal/bench/entity.go", "ArchiveTarget", 1, "where an entity directory goes when it is archived; it answers a path for a directory the caller already holds"},
			{"internal/bench/entity.go", "refBelowHead", 2, "rendering a reference for a directory below a head, which reads no number"},
			{"internal/bench/numbermigrate.go", "MigrateNumbers", 1, "the number migration, which walks both halves to read every card's anchor into the registry it composes; it answers no reference"},
			{"internal/bench/numbermigrate.go", "RenumberCards", 1, "the duplicate repair, which reaches into the archived half to find a later claimant's card directory for its event; it rewrites lines rather than resolving"},
			{"internal/bench/numbers.go", "readNumbers", 1, "the below-format synthesis, which lists both halves through the stamping readers so resolution keeps answering over a workbench the migration has not reached"},
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
			{"internal/verb/beyond.go", "attachmentHolderDir", 1, "the directory an attachment hangs from, skipping the archive segment of an archived attachment's own path; it answers a directory the caller already holds and resolves no reference"},
			{"cmd/dinah-migrate-actors/main.go", "journalsUnder", 2, "the actor migration's journal walk, which lists both archived collections to reach the journals inside them and reads no card anchor and no number at all"},
		},
	},
	{
		name: "E", axis: "reading",
		what:      "rendering a card's human reference: every call whose selector is Ref and which carries exactly one argument",
		files:     17,
		mentions:  36,
		functions: 28,
		sites: []resolutionSite{
			{"internal/bench/designationmigrate.go", "ClaimedCards", 1, "naming a live card the designation conversion found claimed, which is what the in-use refusal reports; the walk reads the live cards collection alone, so no archived card reaches this call"},
			{"internal/bench/designationmigrate.go", "plannedDesignations", 1, "naming the card one converted item hangs below, in the conversion's own report; the conversion walks both halves, so an archived card does reach this call, and it composes a reference for a report rather than resolving one"},
			{"internal/bench/designationmigrate.go", "itemRefOf", 1, "composing the reference the conversion's report names one item by; it reaches both halves for plannedDesignations' reason and composes rather than resolves"},
			{"internal/verb/grant.go", "revoke", 1, "naming the card a revoke found carrying no standing authorization, which is what the no-grant refusal reports; the card was resolved live by Library.Do, so no archived card reaches this call"},
			{"internal/verb/checklist.go", "itemCanonicalRef", 1, "composing the canonical reference of one item of one card, which a settling stores as its answer and a forced deletion hands to Reopen; the card is the one the item hangs below and it was resolved live by the verb that is writing it, so no archived card reaches this call"},
			{"internal/verb/read.go", "primePending", 1, "naming the card one Primer.Pending item belongs to; the card came off Library.Cards, the live collection alone, so no archived card reaches this call"},
			{"cmd/dinah-migrate-notes/main.go", "classifyCard", 1, "naming an item in the note migration's own report and in the designation it writes; the run walks both halves, so an archived card does reach this call, and it composes a reference for a report rather than resolving one"},
			{"internal/bench/branchmigrate.go", "MigrateBranches", 5, "naming a card in the branch migration's own report, once for each of the four classes it sorts a card into and once more for the account of what the write pass wrote; the run walks the live half alone, on the rule MigrateNumbers keeps for the archive, so no archived card reaches these calls"},
			{"internal/bench/check.go", "checkTierOverrides", 1, "naming a card in a finding"},
			{"internal/bench/check.go", "checkItemColumns", 1, "naming a card in a finding"},
			{"internal/bench/routecheck.go", "checkCardRoute", 3, "naming a card in a finding, once for a route the workbench does not declare, once for a card standing off its road, and once for a road that carries it around a column reserved to the operator; check walks the live half alone, so no archived card reaches these calls"},
			{"internal/bench/routecheck.go", "checkItemRoutes", 1, "naming a card in a finding about one of its pending items; check walks the live half alone, so no archived card reaches this call"},
			{"internal/bench/commentcheck.go", "checkComments", 1, "composing the reference a comment finding names, so a reader can type what the finding reports"},
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
			{"internal/verb/tree.go", "workstreamContents", 1, "naming a card the walk from a workstream drew as a member; the membership comes off Library.selection, which reads the workbench's own cards over the live half alone, so no archived card reaches this call"},
			{"internal/lsp/annotate.go", "cardAnnotation", 1, "composing the canonical reference the language server prints in a card's hover and carries on the annotation's target; the card came out of a live resolution or out of a document's own reference, and the archive is never read here"},
			{"internal/lsp/handlers.go", "cardCandidates", 1, "composing the label of one card candidate in a completion list, over the live half alone"},
		},
	},
	{
		name: "F", axis: "reading",
		what:      "reading the number out of frontmatter: every basic string literal \"number\"",
		files:     4,
		mentions:  7,
		functions: 5,
		sites: []resolutionSite{
			{"internal/bench/declaredfields.go", "<package-level>", 1, "FieldTypeNumber, the name of one of the five types a declared field may take; it is the word number and never a card's number, and nothing reads a frontmatter key with it"},
			{"internal/bench/card.go", "stamp", 1, "c.FM.Value(\"number\"), the below-format half of the stamping reader, where the legacy read path still takes the number from the card's own frontmatter"},
			{"internal/bench/check.go", "checkCardNumbers", 1, "card.FM.Has(\"number\"), the in-frontmatter finding, which reports a card on a migrated workbench still carrying the key"},
			{"internal/bench/numbermigrate.go", "readMigrant", 2, "fm.Has(\"number\") and fm.Value(\"number\"), the free reader the migration reads pre-registry anchors through, where a stamping reader would answer nothing the run can use"},
			{"internal/bench/numbermigrate.go", "MigrateNumbers", 2, "fm.Has(\"number\") and fm.Delete(\"number\"), the strip that takes the key off every anchor once the registry holds the number"},
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
	// CollectionRootIn joined the set on dinah-523. It composes the directory
	// one of the workbench's own top-level collections occupies in a half
	// rather than resolving a reference in one, so it reads a half without
	// being a resolver, and it is named here so that the roster stays derived
	// rather than pruned to the shape the guard was written for.
	wantedDerived := []string{"CollectionRootIn", "ResolveEntityIn", "ResolvePathIn", "ResolveReferenceIn"}
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
