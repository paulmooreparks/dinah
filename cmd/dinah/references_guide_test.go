package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/verb"
)

// The references guide is the contract of record for how a person names
// anything in a workbench, and until dinah-457 its command table was a
// hand-written list. It counted ten commands while fifteen pointed their
// reader at it, and nothing in the suite noticed, because a hand-written list
// is only as good as its last editor. The checks below hold the guide's own
// words against the running tool in both directions: a command the library
// points at this guide with no row fails, and a row naming a command the
// library does not point here fails too.
//
// Every check that reads the guide reads guide.Text("references") rather than
// the output of `dinah guide references`, which is wrapped to the reader's
// window and would make a line-sensitive read depend on a terminal width.
// parseReferencesGuideTable, in references_command_resolution_test.go, already
// reads it that way.

// commandsTakingAReference lists, in sorted order, every command the library
// points at the references guide. It reads verb.Guides rather than either
// declaration behind it, because Guides merges the command's own topics with
// every topic its parameters declare, so a command declared in one roster and
// not the other is counted once and no caller has to know there are two.
//
// verb.ReferenceTakingCommands answers the neighbouring question and this file
// deliberately does not call it. That roster returns a command only when the
// two declarations agree, and drops one that carries the topic in a single
// place, which is right for a caller asking what the library promises and
// wrong for a caller asking who sends a reader to this guide: a command
// declaring the topic once still points its reader here, and dropping it would
// let the guide's table lose a row with every check green. The union cannot go
// stale against that roster either, because internal/verb/collection_roster_test.go
// holds both declarations to one another.
func commandsTakingAReference() []string {
	var names []string
	for _, name := range verb.Commands() {
		for _, topic := range verb.Guides(name) {
			if topic == "references" {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

// foldedGuideParagraph returns the references guide's paragraph whose text
// opens with marker, folded to single spaces, and reports whether the guide
// carries such a paragraph at all.
//
// The guide is hard-wrapped and its rewrite licenses a re-wrap of any
// paragraph the prose ledger does not pin, so a caller that matched a marker
// against the start of a source line would report a failure against a guide
// that is perfectly correct.
func foldedGuideParagraph(t *testing.T, marker string) (string, bool) {
	t.Helper()
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	for _, paragraph := range regexp.MustCompile(`\n\s*\n`).Split(text, -1) {
		folded := strings.Join(strings.Fields(paragraph), " ")
		if strings.HasPrefix(folded, marker) {
			return folded, true
		}
	}
	return "", false
}

// foldedGuideParagraphStartingWith is foldedGuideParagraph for the callers
// that require the paragraph to be there. A marker that matches nothing is a
// fatal naming the marker, so a reworded paragraph is repaired rather than
// read as an empty set.
func foldedGuideParagraphStartingWith(t *testing.T, marker string) string {
	t.Helper()
	folded, carried := foldedGuideParagraph(t, marker)
	if !carried {
		t.Fatalf("the references guide carries no paragraph opening %q, so this check read nothing", marker)
	}
	return folded
}

// foldedGuideParagraphMatching returns the whole of the references guide's
// paragraph whose folded text pattern matches, alongside that match. It exists
// for the callers whose subject is the paragraph's own opening figure, which a
// prefix marker would have to spell and would then stop finding the paragraph
// the moment the figure it holds went wrong.
//
// The paragraph and the match are returned separately because the patterns
// here anchor on the opening sentence, so the match covers that sentence and
// not the rest of the paragraph the caller has to read.
func foldedGuideParagraphMatching(t *testing.T, pattern *regexp.Regexp) (string, []string) {
	t.Helper()
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	for _, paragraph := range regexp.MustCompile(`\n\s*\n`).Split(text, -1) {
		folded := strings.Join(strings.Fields(paragraph), " ")
		if match := pattern.FindStringSubmatch(folded); match != nil {
			return folded, match
		}
	}
	t.Fatalf("the references guide carries no paragraph matching %s, so this check read nothing", pattern)
	return "", nil
}

// referencesGuideDetail names the paragraph qualifying individual rows of the
// table, and captures the figure opening it. The figure is the thing under
// test, so the pattern cannot anchor on it; the rest of the sentence names the
// paragraph on its own.
var referencesGuideDetail = regexp.MustCompile(`^([A-Za-z0-9-]+) of those rows carry a detail the table is too coarse to hold\.`)

// referencesGuideQualifiedRows returns the commands the detail paragraph
// qualifies, derived from the guide's own backticks against the roster rather
// than from a second list, and the figure the paragraph opens with.
func referencesGuideQualifiedRows(t *testing.T) (map[string]bool, string) {
	t.Helper()
	roster := commandsTakingAReference()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	folded, match := foldedGuideParagraphMatching(t, referencesGuideDetail)
	return backtickedCommandsIn(folded, roster), match[1]
}

// referencesGuideWorkbenchSpellings returns the references the guide's "This
// workbench" section shows, in the order it draws them. The section's whole
// claim is that the spellings it shows name one thing, so the count and the
// probe both read the shown lines rather than a list written beside them.
func referencesGuideWorkbenchSpellings(t *testing.T) []string {
	t.Helper()
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	shown := regexp.MustCompile(`^ +dinah path (\S+)$`)
	var spellings []string
	inside := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			inside = strings.TrimSpace(line) == "## This workbench"
			continue
		}
		if !inside {
			continue
		}
		if match := shown.FindStringSubmatch(strings.TrimRight(line, "\r")); match != nil {
			spellings = append(spellings, match[1])
		}
	}
	if len(spellings) == 0 {
		t.Fatal("the references guide's \"This workbench\" section shows no spelling, so this check read nothing")
	}
	return spellings
}

// workbenchSpellingRoot is the part of a shown reference that spells this
// workbench, which is everything before the first slash. The section shows one
// of its three spellings with something below it, because Dinah reads a slug
// standing alone as a card, so the count reads the shown lines and the probe
// reads their roots.
func workbenchSpellingRoot(shown string) string {
	root, _, _ := strings.Cut(shown, "/")
	return root
}

// workbenchSpellingProbe rewrites one root the guide shows into the form a
// fixture can run. The guide teaches the slug with its own example workbench,
// so the probe writes the fixture's slug in its place. Like referenceProbeArgs
// this is arguments rather than roster: a root it does not know stops the run
// naming that root, so a fourth spelling is probed deliberately rather than
// silently as a second copy of the third.
func workbenchSpellingProbe(t *testing.T, root, slug string) string {
	t.Helper()
	switch root {
	case "workbench", ".":
		return root
	case "wb":
		return slug
	}
	t.Fatalf("the references guide's \"This workbench\" section spells this workbench %q and this check does not know how to run it", root)
	return ""
}

// backtickedCommandsIn returns the commands of the roster that the folded
// paragraph names in backticks. Both callers derive the set they compare from
// the guide's own words this way rather than from a second hand-written list.
func backtickedCommandsIn(paragraph string, roster []string) map[string]bool {
	takes := map[string]bool{}
	for _, name := range roster {
		takes[name] = true
	}
	named := map[string]bool{}
	for _, token := range regexp.MustCompile("`([a-z-]+)`").FindAllStringSubmatch(paragraph, -1) {
		if takes[token[1]] {
			named[token[1]] = true
		}
	}
	return named
}

// TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference is the check
// this file exists for. It compares the command names in the guide's table
// against the roster the library declares, and fails in both directions.
func TestTheReferencesGuideTableNamesEveryCommandThatTakesAReference(t *testing.T) {
	// parseReferencesGuideTable ends with its own fatal over show, path and
	// edit, so a table stripped of its rows stops there rather than here and
	// the guard below cannot be reached while that fatal stands. It is kept
	// because this test's contract is that an empty subject set stops the run,
	// and the day the parser stops demanding three named rows is the day this
	// line starts carrying that contract on its own.
	declared := parseReferencesGuideTable(t)
	if len(declared) == 0 {
		t.Fatal("the references guide's table draws no row, so this check read nothing")
	}
	roster := commandsTakingAReference()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	takes := map[string]bool{}
	for _, name := range roster {
		takes[name] = true
		if _, drawn := declared[name]; !drawn {
			t.Errorf("%s points at the references guide and the guide's table carries no row for it", name)
		}
	}
	for name := range declared {
		if takes[name] {
			continue
		}
		t.Errorf("the references guide's table carries a row for %s and no command of that name points at the guide", name)
	}
	if len(declared) != len(roster) {
		t.Errorf("the table draws %d rows against a roster of %d: %v", len(declared), len(roster), roster)
	}
}

// referenceProbeArgs returns the arguments a command needs after its
// reference. It is the one hand-written table in this file, and it is
// arguments rather than roster: a command it does not know stops the run
// naming that command, so an eighteenth command entering the roster is probed
// deliberately rather than with the wrong line.
func referenceProbeArgs(t *testing.T, command string) []string {
	t.Helper()
	switch command {
	case "path", "edit", "show", "instructions", "contents", "attachments", "archive", "restore":
		return nil
	case "get":
		return []string{"title"}
	case "set":
		return []string{"title", "a new title"}
	case "delete":
		return []string{"--yes"}
	case "rename":
		return []string{"renamed.txt"}
	case "attach":
		file := filepath.Join(t.TempDir(), "notes.txt")
		if err := os.WriteFile(file, []byte("some bytes"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
		return []string{file}
	case "cite":
		return []string{"url", "https://example.invalid/evidence"}
	case "resolve", "verify", "fail", "reopen":
		return []string{"a note"}
	}
	t.Fatalf("%s takes a reference and this file does not know what arguments follow it, so it cannot be probed", command)
	return nil
}

// commandTookTheReference reports whether a command reached the thing its
// reference named. Every command but edit answers that with its exit code.
// edit launches an editor, and these checks point DINAH_EDITOR at a name no
// machine carries, so edit always exits non-zero and the question for it is
// whether the address was refused before the launch.
func commandTookTheReference(command string, got invocation) bool {
	if command != "edit" {
		return got.code == 0
	}
	return !addressRefused(got.errw)
}

// addressRefused widens references_command_resolution_test.go's
// resolutionRefused with the refusal dinah-455 minted. That file's helper
// names the two refusals a resolver raises when an address does not resolve
// at all; a collection reference resolves and is then refused for what it
// names, which is equally a refusal of the address and not the editor launch
// these checks force.
func addressRefused(errw string) bool {
	if resolutionRefused(errw) {
		return true
	}
	return strings.SplitN(strings.TrimSpace(errw), " ", 2)[0] == contract.IsACollection
}

// workstreamProbeSetup is the invocations to run against the fresh workbench
// before a command is probed against its own workstream, so that a cell the
// declaration grants can answer at all.
//
// restore is the one command that needs any today: an entity that has not been
// archived draws dinah.not-archived, which is a refusal for the state rather than
// for the kind, and reading it as "did not take the reference" is what let this
// check agree with a sentence that omitted restore for as long as it did.
//
// It is written as a declaration with a default of nothing, so a nineteenth
// command needing state is one entry rather than a rewrite. That is the shape
// this card's spec asks a future card to widen to the other five kinds, where the
// same hazard runs deeper: dinah.occupied refuses archive and delete against a
// column the table grants, and restore against this workbench and restore against
// a column answer the identical refusal, so classifying by refusal name cannot
// separate a no cell from a yes cell and only arranging the state can.
func workstreamProbeSetup(command, slug string) [][]string {
	switch command {
	case "restore":
		return [][]string{{"archive", "workstream/" + slug}}
	}
	return nil
}

// TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream holds three answers
// to "which commands take a workstream" against each other: what the binary does,
// what internal/verb declares, and what the guide's sentence names. Every pair is
// compared in both directions, so no single edit can quietly move all three into
// agreement.
//
// Every command of the roster is run against a workstream of its own, so an
// archive or a delete that succeeds cannot change what the next command sees, and
// each one first gets the setup workstreamProbeSetup declares for it.
//
// The two halves are counted separately. A single combined count would hide a
// half that read nothing behind a half that read plenty.
func TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream(t *testing.T) {
	root := newBench(t)
	t.Setenv("DINAH_EDITOR", "dinah-no-such-editor")
	roster := commandsTakingAReference()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	named := backtickedCommandsIn(foldedGuideParagraphStartingWith(t, "Nine commands take a workstream:"), roster)
	reached := map[string]bool{}
	probed, refused := 0, 0
	for at, name := range roster {
		slug := fmt.Sprintf("ws%d", at+1)
		if got := runCLI(t, root, "workstream", "new", "Stream "+slug, "--slug", slug); got.code != 0 {
			t.Fatalf("workstream new %s: %d %s", slug, got.code, got.errw)
		}
		for _, setup := range workstreamProbeSetup(name, slug) {
			if got := runCLI(t, root, setup...); got.code != 0 {
				t.Fatalf("setup for %s (%v): %d %s", name, setup, got.code, got.errw)
			}
		}
		got := runCLI(t, root, append([]string{name, "workstream/" + slug}, referenceProbeArgs(t, name)...)...)
		probed++
		if commandTookTheReference(name, got) {
			reached[name] = true
			continue
		}
		refused++
		t.Logf("refused %s exit %d %s", name, got.code, strings.SplitN(strings.TrimSpace(got.errw), " ", 2)[0])
	}
	if len(reached) == 0 {
		t.Fatal("no command reached a workstream, so the fixture is broken rather than the guide")
	}

	// Three sides, each pair both ways. The binary is the authority the other
	// two answer to, and the guide's sentence and the declaration are held to
	// each other as well so that a sentence and a declaration cannot agree with
	// one another while both disagree with what the tool does.
	for _, name := range roster {
		kinds, ok := verb.ReferenceKindsFor(name)
		declares := false
		if ok {
			for _, kind := range kinds {
				if kind == verb.ReferenceKindWorkstream {
					declares = true
				}
			}
		}
		took := reached[name]
		if took != declares {
			t.Errorf("%s: binary=%t declaration=%t; `dinah %s workstream/<slug>` and internal/verb disagree about the workstream", name, took, declares, name)
		}
		if took != named[name] {
			t.Errorf("%s: binary=%t sentence=%t; `dinah %s workstream/<slug>` and the references guide's workstream sentence disagree", name, took, named[name], name)
		}
		if declares != named[name] {
			t.Errorf("%s: declaration=%t sentence=%t; internal/verb and the references guide's workstream sentence disagree", name, declares, named[name])
		}
	}

	t.Logf("%d commands probed, %d reached the workstream, %d refused", probed, len(reached), refused)
	if probed != len(roster) {
		t.Fatalf("the probe ran %d commands and the roster names %d", probed, len(roster))
	}
	if len(reached) != 9 {
		t.Fatalf("%d commands reached the workstream and nine take one", len(reached))
	}
	if refused != 9 {
		t.Fatalf("%d commands were refused the workstream and nine refuse one", refused)
	}
}

// TestTheReferencesGuideDeclaresEveryCommandTheCollectionColumnIsAheadOf holds
// the guide's collection column, which states the contract dinah-456 settled,
// against what the commands do today. Where the two disagree the guide carries
// a paragraph naming exactly the commands that have not caught up, and where
// nothing disagrees that paragraph has to be gone: naming too few fails,
// naming too many fails, and outliving the disagreement fails.
func TestTheReferencesGuideDeclaresEveryCommandTheCollectionColumnIsAheadOf(t *testing.T) {
	const marker = "Not every command has caught up with the collection column yet."
	const collection = "A collection"
	roster := commandsTakingAReference()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	declared := parseReferencesGuideTable(t)
	root := newBench(t)
	t.Setenv("DINAH_EDITOR", "dinah-no-such-editor")
	disagreeing := map[string]bool{}
	for _, name := range roster {
		// A card of its own per command, each carrying a comment, so every
		// probe meets a collection that exists and holds a member.
		ref := addCard(t, root, "a card for "+name)
		if got := runCLI(t, root, "comment", ref, "a note"); got.code != 0 {
			t.Fatalf("comment %s: %d %s", ref, got.code, got.errw)
		}
		accepts, ok := declared[name][collection]
		if !ok {
			t.Fatalf("the references guide's table declares nothing for %s against %q", name, collection)
		}
		got := runCLI(t, root, append([]string{name, ref + "/comments"}, referenceProbeArgs(t, name)...)...)
		if commandTookTheReference(name, got) != accepts {
			disagreeing[name] = true
		}
	}
	if len(disagreeing) == 0 {
		if _, carried := foldedGuideParagraph(t, marker); carried {
			t.Errorf("every command now agrees with its %q cell and the references guide still carries the paragraph opening %q; dinah-455 has landed, so the paragraph goes", collection, marker)
		}
		return
	}
	named := backtickedCommandsIn(foldedGuideParagraphStartingWith(t, marker), roster)
	for name := range disagreeing {
		if !named[name] {
			t.Errorf("%s disagrees with its %q cell and the references guide's caveat paragraph does not declare it", name, collection)
		}
	}
	for name := range named {
		if !disagreeing[name] {
			t.Errorf("the references guide's caveat paragraph declares %s and %s agrees with its %q cell", name, name, collection)
		}
	}
}

// dinahPathReadableExtensions are the file extensions the name guard reads.
// It reads a declared set rather than every file because a built dinah.exe
// sitting in a developer's checkout embeds the guide's bytes and therefore
// carries the name, and it is git-ignored rather than absent, so a walk over
// every file would fail on a binary that is nobody's defect.
var dinahPathReadableExtensions = []string{
	".go", ".json", ".md", ".mjs", ".mod", ".ndjson", ".ps1", ".py", ".sh", ".sum", ".ts", ".txt", ".yml",
}

// dinahPathSkippedDirectories are the generated and vendored trees the walk
// does not descend into, alongside every directory whose name begins with a
// dot. .gitignore already excludes them.
var dinahPathSkippedDirectories = []string{"node_modules", "dist", "out", "bin", "__pycache__"}

// TestTheNameDinahPathStandsOnlyWhereItIsDeclared holds dinah-456 section
// 13.3. DinahPath is the language a reference is written in, and the name
// stands in the references guide and in design documents and nowhere else: not
// in help text, not in refusal text, and in no field, key or tool name on the
// wire, because a name on the wire is a compatibility commitment.
//
// The guard fails in both directions. A hit in a file the allowlist does not
// carry fails, and zero hits in the references guide fails too, because a
// one-directional guard on a name is satisfied by deleting the name.
func TestTheNameDinahPathStandsOnlyWhereItIsDeclared(t *testing.T) {
	const guidePath = "internal/guide/guides/references.md"
	allowed := map[string]bool{}
	fixture := filepath.Join("testdata", "dinahpath-allowlist.txt")
	body, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read %s: %v", fixture, err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		allowed[entry] = true
		if _, err := os.Stat(filepath.Join(repositoryRoot, filepath.FromSlash(entry))); err != nil {
			t.Errorf("%s names %s and no such file stands in the tree, so the entry is dead", fixture, entry)
		}
	}
	if len(allowed) == 0 {
		t.Fatalf("%s carries no entry, so this check read nothing", fixture)
	}
	readable := map[string]bool{}
	for _, extension := range dinahPathReadableExtensions {
		readable[extension] = true
	}
	skipped := map[string]bool{}
	for _, name := range dinahPathSkippedDirectories {
		skipped[name] = true
	}
	read := 0
	carried := map[string]bool{}
	err = filepath.WalkDir(repositoryRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != repositoryRoot && (strings.HasPrefix(name, ".") || skipped[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		if !readable[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		read++
		relative, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		for at, line := range strings.Split(string(text), "\n") {
			if !strings.Contains(strings.ToLower(line), "dinahpath") {
				continue
			}
			carried[relative] = true
			if !allowed[relative] {
				t.Errorf("%s:%d carries the name DinahPath and %s does not name that file", relative, at+1, fixture)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", repositoryRoot, err)
	}
	if read == 0 {
		t.Fatal("this walk read no file at all, so it proves nothing about where the name stands")
	}
	if !carried[guidePath] {
		t.Errorf("%s carries no occurrence of the name DinahPath, and a guard satisfied by deleting the name is no guard", guidePath)
	}
}

// TestTheReferencesGuideIntroducesDinahPathWithItsCorrection holds dinah-456
// section 13.1: the surface that introduces the name says what the language
// does not admit in the sentence immediately after. The judgement is a human
// read; the mechanical part of it is here.
//
// Every comparison folds the guide's whitespace first, because the file is
// hard-wrapped and a check quoting an unwrapped sentence reports a false
// failure otherwise. internal/guide/guide_test.go:82 folds the same way.
func TestTheReferencesGuideIntroducesDinahPathWithItsCorrection(t *testing.T) {
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	folded := strings.Join(strings.Fields(text), " ")
	if stood := strings.Count(folded, "DinahPath"); stood != 2 {
		t.Errorf("the references guide stands the name DinahPath %d times, and it names the language once and takes it as the next sentence's subject once, which is twice", stood)
	}
	for _, want := range []string{
		"no brackets, no wildcards, no functions, and no axes",
		"rather than the whole of it",
		"dinah guide query",
	} {
		if !strings.Contains(folded, want) {
			t.Errorf("the references guide does not carry %q", want)
		}
	}
	// The four examples follow from the two rules rather than exhausting
	// them, so no closing word may enter that sentence. The read is scoped to
	// the opening paragraph rather than to the guide, because "and nothing
	// else" is ordinary English the guide uses correctly further down, where
	// `instructions` takes a card or a column and nothing else at all.
	opening := foldedGuideParagraphStartingWith(t, "You name a thing to Dinah by writing a reference")
	for _, closing := range []string{"only four", "the four things", "and nothing else"} {
		if strings.Contains(opening, closing) {
			t.Errorf("the references guide's opening paragraph carries %q, and the sentence naming what DinahPath leaves out may not close the list", closing)
		}
	}
}

// TestTheReferencesGuideTableFitsAnEightyColumnWindow measures the width the
// table's headers are written for. A table row is exempt from the guide wrap
// by name at guide_wrap_test.go:275, so the terminal folds a row nothing else
// in the suite measures, and before this check the next person to lengthen a
// heading would have broken the table with the suite green.
func TestTheReferencesGuideTableFitsAnEightyColumnWindow(t *testing.T) {
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	lines := strings.Split(text, "\n")
	headerAt := -1
	for at, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "| Command") {
			headerAt = at
			break
		}
	}
	if headerAt < 0 {
		t.Fatal("the references guide carries no \"| Command\" table header, so this check read nothing")
	}
	// Eighty is the window a guide table is written to fit. The wrap exempts
	// a table row, so nothing else in the suite measures one, and the terminal
	// folds what the wrap leaves alone. `dinah help attach` is held to the
	// same eighty at attachments_command_test.go:287.
	measured := 0
	for _, line := range lines[headerAt:] {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			break
		}
		measured++
		if drawn := displayWidth(line); drawn > 80 {
			t.Errorf("the references guide's table draws a line of %d columns, and the table is written to fit eighty: %q", drawn, line)
		}
	}
	if want := len(commandsTakingAReference()) + 2; measured != want {
		t.Errorf("this check measured %d table lines, and the table draws a header, a separator and one row per command, which is %d", measured, want)
	}
}

// TestTheDetailParagraphQualifiesRowsTheTableActuallyDraws holds the sentence
// under the table to the table. The paragraph says that some of those rows
// carry a detail the table is too coarse to hold, so every command it names in
// backticks has to be a row: a paragraph qualifying a command the table never
// drew is describing a table that is not there.
//
// The figure opening that sentence is held elsewhere, by the qualifiedTableRows
// derivation in the prose figure ledger, which counts the same set this check
// reads. Round one of this card shipped that figure hand-counted, said five and
// detailed nine, and nothing anywhere noticed.
func TestTheDetailParagraphQualifiesRowsTheTableActuallyDraws(t *testing.T) {
	declared := parseReferencesGuideTable(t)
	if len(declared) == 0 {
		t.Fatal("the references guide's table draws no row, so this check read nothing")
	}
	qualified, _ := referencesGuideQualifiedRows(t)
	if len(qualified) == 0 {
		t.Fatal("the references guide's detail paragraph names no command of the roster, so this check read nothing")
	}
	for name := range qualified {
		if _, drawn := declared[name]; !drawn {
			t.Errorf("the references guide's detail paragraph qualifies %s and the guide's table draws no row for it", name)
		}
	}
}

// TestEverySpellingOfThisWorkbenchTheGuideShowsNamesOneThing holds the claim
// the "This workbench" section makes about the spellings it shows, which is
// that they mean the same thing. Every shown spelling is run against one
// fixture and the answers are compared, so a spelling that stops resolving,
// and a spelling added to the block that never resolved, both fail here.
//
// The figure in that section's opening sentence is held by the
// workbenchSpellings derivation in the prose figure ledger, which counts the
// same shown lines. Round one of this card added the third spelling in prose
// and left the sentence above it saying two.
func TestEverySpellingOfThisWorkbenchTheGuideShowsNamesOneThing(t *testing.T) {
	shown := referencesGuideWorkbenchSpellings(t)
	// Every root is probed with the same thing below it, so the answers are
	// comparable however the section chose to show each one. Two shown lines
	// sharing a root would be one spelling written twice, which would make the
	// section's own figure wrong, so that fails here rather than passing as a
	// pair that trivially agrees.
	roots := map[string]string{}
	var order []string
	for _, line := range shown {
		root := workbenchSpellingRoot(line)
		if first, repeated := roots[root]; repeated {
			t.Errorf("the references guide shows `dinah path %s` and `dinah path %s`, which spell this workbench the same way, so the section shows fewer ways than it draws lines", first, line)
			continue
		}
		roots[root] = line
		order = append(order, root)
	}
	root := newBench(t)
	answers := map[string]string{}
	for _, spelling := range order {
		reference := workbenchSpellingProbe(t, spelling, "fx") + "/attachments"
		got := runCLI(t, root, "path", reference)
		if got.code != 0 {
			t.Errorf("the references guide spells this workbench %q and `dinah path %s` exits %d: %s", spelling, reference, got.code, got.errw)
			continue
		}
		answers[spelling] = strings.TrimSpace(got.out)
	}
	if len(answers) == 0 {
		t.Fatal("no spelling the references guide shows resolved, so the fixture is broken rather than the guide")
	}
	first := order[0]
	want, resolved := answers[first]
	if !resolved {
		t.Fatalf("the first spelling the references guide shows, %q, did not resolve, so this check has nothing to compare against", first)
	}
	for _, spelling := range order[1:] {
		if got, ok := answers[spelling]; ok && got != want {
			t.Errorf("the references guide says its spellings of this workbench name the same thing, and %q answers %q where %q answers %q", spelling, got, first, want)
		}
	}
}

// guideDenialOfACapability matches a sentence denying that something exists:
// an absence verb, the word no, and the name being denied. The capture is the
// name, which the caller holds against the tool's own roster.
//
// The pattern is deliberately narrow. It reads a denial rather than every
// mention of a command, because prose says "no" about plenty of things that
// are not capabilities, and a guard that fired on all of them would be
// silenced rather than fixed.
var guideDenialOfACapability = regexp.MustCompile(`\b(?:has|have|had|is|are|was|were|carries|carry|offers|offer|provides|provide|knows|know)\s+no\s+([a-z][a-z-]*)`)

// referencesGuideProseParagraphs returns the references guide's paragraphs
// with its tables and its indented blocks removed, each folded to single
// spaces and stripped of backticks.
//
// The two removals are what make a scan over the whole guide safe. Folding a
// table row runs its cells together, so a `no` cell lands directly beside the
// next row's command name and reads as a denial of it, and an indented block
// holds command lines rather than sentences.
func referencesGuideProseParagraphs(t *testing.T) []string {
	t.Helper()
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	var prose []string
	for _, paragraph := range regexp.MustCompile(`\n\s*\n`).Split(text, -1) {
		var kept []string
		for _, line := range strings.Split(paragraph, "\n") {
			if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(line), "|") {
				continue
			}
			kept = append(kept, line)
		}
		folded := strings.Join(strings.Fields(strings.Join(kept, " ")), " ")
		folded = strings.ReplaceAll(folded, "`", "")
		if folded != "" {
			prose = append(prose, folded)
		}
	}
	return prose
}

// TestTheReferencesGuideDeniesNoCommandTheToolHas holds the guide's prose
// against the tool's command roster in the one direction the table checks
// cannot see. Those checks compare the table's rows with the commands that
// point a reader here, so a row for a new command is added and they go green,
// and a sentence elsewhere in the same guide saying that command does not
// exist stays exactly as it was.
//
// That is not hypothetical. dinah-461 added the `restore` row and the guide
// went on saying "an act that writes to a whole collection cannot be undone
// and Dinah has no restore" thirty lines above it, through a green tree and
// eleven verified criteria, because no check read that sentence.
//
// The guard runs one way. A guide that denies a command the tool has fails
// here; a guide that stays silent about a command passes, which is the table
// checks' subject rather than this one's.
func TestTheReferencesGuideDeniesNoCommandTheToolHas(t *testing.T) {
	roster := map[string]bool{}
	for _, name := range verb.Commands() {
		roster[name] = true
	}
	if len(roster) == 0 {
		t.Fatal("the library declares no command, so this check read nothing")
	}
	prose := referencesGuideProseParagraphs(t)
	if len(prose) == 0 {
		t.Fatal("the references guide carries no prose paragraph, so this check read nothing")
	}
	for _, paragraph := range prose {
		for _, match := range guideDenialOfACapability.FindAllStringSubmatch(paragraph, -1) {
			denied := match[1]
			if !roster[denied] {
				continue
			}
			t.Errorf("the references guide says %q and `dinah %s` is a command this build carries: %s", match[0], denied, paragraph)
		}
	}
}

// TestTheReferencesGuideTableDrawsTheDeclaredReferenceKinds holds the shipped
// guide's "Which command takes what" table against the declaration in
// internal/verb, cell by cell and in both directions. A declared command with no
// row fails, a row naming an undeclared command fails, and a cell whose yes or no
// disagrees with the declaration fails naming the command, the column and both
// answers.
//
// This is what makes the guide derived rather than authoritative. dinah-457 made
// the table the one place the tool declared which kinds a command takes, which
// was a stopgap; the declaration now lives in internal/verb and the table is held
// to it.
//
// The workstream is excluded deliberately rather than silently, because the
// table draws five columns and the declaration carries six. The exclusion is
// asserted to reach exactly one kind and to be that one, so a seventh kind
// arriving fails here rather than becoming a cell nobody compares.
// TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream holds the workstream.
func TestTheReferencesGuideTableDrawsTheDeclaredReferenceKinds(t *testing.T) {
	roster := verb.ReferenceTakingCommands()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}

	var unmapped []verb.ReferenceKind
	var drawn []verb.ReferenceKind
	for _, kind := range verb.ReferenceKindOrder() {
		if _, mapped := referenceGuideHeading(kind); mapped {
			drawn = append(drawn, kind)
			continue
		}
		unmapped = append(unmapped, kind)
	}
	if len(unmapped) != 1 || unmapped[0] != verb.ReferenceKindWorkstream {
		t.Fatalf("the guide's table draws a column for every declared kind but the workstream; these are unmapped instead: %v", unmapped)
	}

	declared := parseReferencesGuideTable(t)
	for command := range declared {
		if _, ok := verb.ReferenceKindsFor(command); !ok {
			t.Errorf("the references guide's table carries a row for %s and internal/verb declares no kinds for it", command)
		}
	}

	rows, cells := 0, 0
	for _, command := range roster {
		accepts, carried := declared[command]
		if !carried {
			t.Errorf("internal/verb declares kinds for %s and the references guide's table carries no row for it", command)
			continue
		}
		rows++
		granted := map[verb.ReferenceKind]bool{}
		kinds, _ := verb.ReferenceKindsFor(command)
		for _, kind := range kinds {
			granted[kind] = true
		}
		for _, kind := range drawn {
			heading, _ := referenceGuideHeading(kind)
			cell, drawnHere := accepts[heading]
			if !drawnHere {
				t.Errorf("the references guide's row for %s draws no %q cell", command, heading)
				continue
			}
			cells++
			if cell != granted[kind] {
				t.Errorf("the references guide's table says %s against %q for %s and internal/verb declares %s", yesNo(cell), heading, command, yesNo(granted[kind]))
			}
		}
	}
	t.Logf("%d rows and %d cells compared", rows, cells)
	if want := len(roster); rows != want {
		t.Fatalf("the comparison read %d rows and the roster names %d commands", rows, want)
	}
	if want := len(roster) * len(drawn); cells != want {
		t.Fatalf("the comparison read %d cells and %d rows against %d drawn columns is %d", cells, rows, len(drawn), want)
	}
}

// yesNo spells a declaration cell the way the guide's table spells it, so a
// disagreement is reported in the words a reader would compare.
func yesNo(accepts bool) string {
	if accepts {
		return "yes"
	}
	return "no"
}
