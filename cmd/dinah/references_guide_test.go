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
// naming that command, so a sixteenth command entering the roster is probed
// deliberately rather than with the wrong line.
func referenceProbeArgs(t *testing.T, command string) []string {
	t.Helper()
	switch command {
	case "path", "edit", "show", "instructions", "contents", "attachments", "archive":
		return nil
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

// TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream holds the guide's
// workstream sentence, which is a hand-written list of six names, against what
// the commands do. Every command of the roster is run against a workstream of
// its own, so an archive or a delete that succeeds cannot change what the next
// command sees.
func TestTheReferencesGuideNamesTheCommandsThatTakeAWorkstream(t *testing.T) {
	root := newBench(t)
	t.Setenv("DINAH_EDITOR", "dinah-no-such-editor")
	roster := commandsTakingAReference()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	named := backtickedCommandsIn(foldedGuideParagraphStartingWith(t, "Six commands take a workstream:"), roster)
	reached := map[string]bool{}
	for at, name := range roster {
		slug := fmt.Sprintf("ws%d", at+1)
		if got := runCLI(t, root, "workstream", "new", "Stream "+slug, "--slug", slug); got.code != 0 {
			t.Fatalf("workstream new %s: %d %s", slug, got.code, got.errw)
		}
		got := runCLI(t, root, append([]string{name, "workstream/" + slug}, referenceProbeArgs(t, name)...)...)
		if commandTookTheReference(name, got) {
			reached[name] = true
		}
	}
	if len(reached) == 0 {
		t.Fatal("no command reached a workstream, so the fixture is broken rather than the guide")
	}
	for name := range reached {
		if !named[name] {
			t.Errorf("`dinah %s workstream/<slug>` works and the references guide's workstream sentence does not name %s", name, name)
		}
	}
	for name := range named {
		if !reached[name] {
			t.Errorf("the references guide's workstream sentence names %s and `dinah %s workstream/<slug>` does not take one", name, name)
		}
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
