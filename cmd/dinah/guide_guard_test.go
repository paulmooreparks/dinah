package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/msg"
)

// The source-level checks that back the replay up.
//
// A guide's transcript is composed at run time and no scan of Go sources can
// see it, which is why the load sits on the replay in quickstart_test.go.
// These checks cover the places the replay does not reach: the embedded
// guides, which carry no transcript at all, and the blocks of the quick start
// the replay is exempt from. Each one reads text against text, so each one is
// a backstop rather than proof that the tool prints what the document shows.

// repositoryRoot is the tree these checks read, relative to this package.
var repositoryRoot = filepath.Join("..", "..")

// guardedDocument is one text this file's checks read, with the name a finding
// calls it by.
type guardedDocument struct {
	// name is what a finding names, so a reader knows which file to open.
	name string
	// text is the whole document.
	text string
}

// embeddedGuides reads every guide the binary ships. The guides are read apart
// from the quick start because two rules hold of a guide and not of that
// document: a guide drives no transcript, and a guide is served to a reader who
// cannot see the repository it was written in.
func embeddedGuides(t *testing.T) []guardedDocument {
	t.Helper()
	var documents []guardedDocument
	for _, topic := range guide.Topics() {
		text, err := guide.Text(topic)
		if err != nil {
			t.Fatalf("guide %s: %v", topic, err)
		}
		documents = append(documents, guardedDocument{name: "internal/guide/guides/" + topic + ".md", text: text})
	}
	if len(documents) == 0 {
		t.Fatal("the binary embeds no guide, so every check over the guides reads nothing")
	}
	return documents
}

// guardedDocuments reads every guide the binary ships plus the quick start,
// which together are the documents that teach somebody how to use the tool.
func guardedDocuments(t *testing.T) []guardedDocument {
	t.Helper()
	documents := embeddedGuides(t)
	source, err := os.ReadFile(quickStartPath)
	if err != nil {
		t.Fatalf("read %s: %v", quickStartPath, err)
	}
	documents = append(documents, guardedDocument{name: quickStartPath, text: strings.ReplaceAll(string(source), "\r\n", "\n")})
	if len(documents) < 2 {
		t.Fatal("the corpus holds fewer documents than the guides and the quick start, so these checks read almost nothing")
	}
	return documents
}

// TestNoGuideCarriesATranscriptTheReplayDoesNotDrive asserts that no embedded
// guide opens a fenced block with a command line, which is the shape the quick
// start's replay selects on and the shape nothing drives inside a guide.
//
// A guide's fenced blocks show commands to type rather than sessions to
// believe, and the quick start owns the transcripts the replay runs. A guide
// that writes one in the replay's own shape gets neither treatment: no replay
// reaches it, no exemption is demanded of it, and the output it shows stands
// unheld from the day it was written. The check names the shape rather than the
// intent, because the shape is what the replay's selection rule reads.
func TestNoGuideCarriesATranscriptTheReplayDoesNotDrive(t *testing.T) {
	read := 0
	for _, document := range embeddedGuides(t) {
		lines := strings.Split(document.text, "\n")
		for at := 0; at < len(lines); at++ {
			run := quickStartMarkerRun(lines[at])
			if run == 0 {
				continue
			}
			read++
			if at+1 < len(lines) && strings.HasPrefix(lines[at+1], "$ ") {
				t.Errorf("%s:%d: the block opens with a command line, which is the shape the quick start's replay drives and nothing drives here; write the command without its leading dollar sign", document.name, at+2)
			}
			for at++; at < len(lines) && quickStartMarkerRun(lines[at]) != run; at++ {
			}
		}
	}
	if read == 0 {
		t.Error("no embedded guide carries a fenced block, so this check read nothing")
	}
}

// bannedTypography is the character set the profile's style section rules out,
// mapped to the name a finding calls each one by.
//
// The set is stated here as well as in TestHouseStyle, which holds the profile
// itself in internal/profile. A test in one package cannot read a test in
// another, and the alternative was to export the set from a shipped package so
// that two tests could share it, which would put style vocabulary into the
// binary to save a duplicated map.
var bannedTypography = map[string]string{
	"—": "em-dash",
	"–": "en-dash",
	"−": "minus sign",
	"‘": "left single quotation mark",
	"’": "right single quotation mark",
	"“": "left double quotation mark",
	"”": "right double quotation mark",
}

// TestTheDocumentationCarriesNoBannedTypography asserts the profile's
// typographic rules over every guide and the quick start, which TestHouseStyle
// asserts over the profile alone.
//
// The rule reaches these documents for the same reason it reaches the profile.
// A reader meets a curly quotation mark as a character they cannot type back at
// the tool, and an em-dash is house style rather than an accident, so both are
// worth failing a build over rather than catching by eye in review.
func TestTheDocumentationCarriesNoBannedTypography(t *testing.T) {
	read := 0
	for _, document := range guardedDocuments(t) {
		for number, line := range strings.Split(document.text, "\n") {
			read++
			if strings.Contains(line, "\r") {
				t.Errorf("%s:%d: the line carries a carriage return", document.name, number+1)
			}
			for character, name := range bannedTypography {
				if !strings.Contains(line, character) {
					continue
				}
				t.Errorf("%s:%d: the %s stands in the line: %q", document.name, number+1, name, line)
			}
		}
	}
	if read == 0 {
		t.Error("no line of any guide or of the quick start was read, so this check proves nothing")
	}
}

// mintedRefusal matches a refusal name Dinah coins, which carries the layer
// prefix and so can never be an ordinary English word.
var mintedRefusal = regexp.MustCompile(regexp.QuoteMeta(contract.LayerPrefix) + `[a-z][a-z-]*`)

// TestTheGuidesQuoteOnlyDeclaredRefusals asserts that every refusal name a
// guide or the quick start quotes is one the profile declares or one Dinah
// introduces.
//
// The rule reads two shapes and the narrowing between them is deliberate. A
// name carrying the layer prefix is recognised wherever it stands, in prose as
// readily as in a transcript, since no English sentence spells one by
// accident. A name the profile declares is recognised only as the first field
// of an output line inside a fenced block, because `blocked`, `held`,
// `terminal`, and `malformed` are ordinary words that stand all through this
// documentation meaning something else entirely.
//
// The two shapes are counted apart because only one of them is checked. A
// prefixed name is held against the two sets and can fail; a declared word at
// the head of an output line is recognised and nothing more, since a first
// field that is not declared is indistinguishable from an ordinary English
// word. Counting both toward one total would let a corpus carrying declared
// words and no prefixed name pass with nothing validated at all, so the
// corpus assertion reads the checked count alone.
func TestTheGuidesQuoteOnlyDeclaredRefusals(t *testing.T) {
	legal := map[string]bool{}
	for _, name := range contract.Declared {
		legal[name] = true
	}
	declared := map[string]bool{}
	for _, name := range contract.Declared {
		declared[name] = true
	}
	for _, name := range contract.Introduced {
		legal[name] = true
	}
	checked := 0
	recognised := 0
	for _, document := range guardedDocuments(t) {
		inBlock := false
		for number, line := range strings.Split(document.text, "\n") {
			if quickStartMarkerRun(line) > 0 {
				inBlock = !inBlock
				continue
			}
			for _, name := range mintedRefusal.FindAllString(line, -1) {
				checked++
				if legal[name] {
					continue
				}
				t.Errorf("%s:%d: the document quotes the refusal name %s, which neither contract.Declared nor contract.Introduced carries", document.name, number+1, name)
			}
			if !inBlock || strings.HasPrefix(line, "$ ") || exitLine.MatchString(strings.TrimSpace(line)) {
				continue
			}
			first, _, _ := strings.Cut(strings.TrimSpace(line), " ")
			if !declared[first] {
				continue
			}
			recognised++
		}
	}
	if checked == 0 {
		t.Errorf("no refusal name carrying the %s prefix was found in any guide or in the quick start, so this test validated nothing; %d declared word or words stood at the head of an output line, and none of those is checkable",
			contract.LayerPrefix, recognised)
	}
}

// placeholder matches one interpolation slot of a catalog entry's stored text.
var placeholder = regexp.MustCompile(`\{[A-Za-z_][A-Za-z0-9_]*\}`)

// wordyLiteral matches the literal text of a rendering that carries nothing
// but letters, digits, and spaces outside its placeholders.
var wordyLiteral = regexp.MustCompile(`^[A-Za-z0-9 ]*$`)

// separatorLiteral matches the literal text of a rendering whose placeholders
// are joined by one punctuation mark and nothing else. Such an entry is two
// wildcards with a separator between them, so every line carrying that mark
// answers it and a line it finds is evidence of nothing.
//
// The narrowing above cannot reach this case, because a mark is not a letter,
// a digit or a space, so the entry keeps the wide wildcard and matches most of
// the document. tree.hidden.join, whose stored text is `{first}, {second}`, is
// the entry that showed it: the rule found it in four blocks that print no
// tree at all, one of them the installer transcript Dinah writes no line of.
var separatorLiteral = regexp.MustCompile(`^ *[^A-Za-z0-9 ] *$`)

// rendering is one catalog entry compiled for the discovery rule: the form
// that matches at an anchor and ends at a boundary, and the form that matches
// a whole field.
type rendering struct {
	// key is the catalog key.
	key string
	// anchored matches at a position and ends at a word boundary, with the
	// first submatch carrying the rendering itself.
	anchored *regexp.Regexp
	// whole matches a field in its entirety.
	whole *regexp.Regexp
}

// renderingsOfTheCatalog compiles every base-catalog entry into the two forms
// the discovery rule matches with.
//
// A placeholder reads as one or more characters other than a newline, except
// where the rendering's literal text outside its placeholders carries nothing
// but letters, digits, and spaces. Such a rendering is nearly all wildcard, so
// its placeholders read as runs of non-space characters instead.
//
// The narrowing is measured rather than argued. Without it the rule finds
// `log.moved`, whose stored text is `{from} to {to}`, in four blocks of the
// quick start that print no move at all, including the installer's own English
// in the one block that earns `quotes=none`. With it applied to every
// rendering rather than only to the leading placeholder, the rule still finds
// `card.line`, `whoami.line`, and `status.workbench` wherever they stand, which
// are the entries an exempt block showing a card line would most need held and
// which a rule reading every placeholder as a run of non-space characters
// cannot see, since a card title carries spaces.
func renderingsOfTheCatalog(t *testing.T) []rendering {
	t.Helper()
	var compiled []rendering
	for _, key := range msg.Keys() {
		entry, ok := msg.BaseEntry(key)
		if !ok || entry.Text == "" {
			continue
		}
		if identifiesNothing(entry.Text) {
			continue
		}
		pattern := renderingPattern(entry.Text)
		compiled = append(compiled, rendering{
			key:      key,
			anchored: regexp.MustCompile(`^(` + pattern + `)(?:[^A-Za-z0-9_-]|$)`),
			whole:    regexp.MustCompile(`^(?:` + pattern + `)$`),
		})
	}
	if len(compiled) == 0 {
		t.Fatal("the base catalog compiled to no rendering, so the discovery rule reads nothing")
	}
	return compiled
}

// identifiesNothing reports whether an entry's stored text is placeholders
// joined by a single punctuation mark, which recognises any line carrying that
// mark and so cannot be evidence that the entry itself was rendered.
func identifiesNothing(text string) bool {
	literal := placeholder.ReplaceAllString(text, "")
	if literal == text {
		return false
	}
	return separatorLiteral.MatchString(literal)
}

// renderingPattern turns one entry's stored text into the expression that
// recognises it in a line of output.
func renderingPattern(text string) string {
	wildcard := `.+`
	if wordyLiteral.MatchString(placeholder.ReplaceAllString(text, "")) {
		wildcard = `[^ ]+`
	}
	var built strings.Builder
	at := 0
	for _, found := range placeholder.FindAllStringIndex(text, -1) {
		built.WriteString(regexp.QuoteMeta(text[at:found[0]]))
		built.WriteString(wildcard)
		at = found[1]
	}
	built.WriteString(regexp.QuoteMeta(text[at:]))
	return built.String()
}

// quotedEntriesOfBlock reports every catalog entry whose rendering appears in
// an exempt block's output.
//
// A rendering appears in a block when it matches an output line at one of four
// anchors and its match ends at a boundary, where a character is
// boundary-forming when it is neither alphanumeric nor an underscore nor a
// hyphen, and the end of a line is boundary-forming too. Each anchor answers a
// shape the head genuinely prints.
//
//  1. The start of an output line, once its leading whitespace is trimmed.
//     `check` indents a finding and appends the path it names to it, so the
//     sentence starts the line and does not finish it.
//  2. The position just after a leading refusal-name token and the space
//     following it. The head writes a refusal as its name, a space, and the
//     sentence, so the sentence stands in the middle of the line and no
//     line-start anchor reaches it.
//  3. The position just after the end of any rendering already found on that
//     line, and after the single space following it where there is one. A
//     refusal sentence carries an advisory clause appended from a second
//     catalog entry, and that clause is the shape that went stale across the
//     whole document once already. The anchor is applied to a fixpoint, so a
//     line composed of three entries yields all three.
//  4. A whole field of a line split on runs of two or more spaces, which is
//     how a table cell carries a heading or a single-word value.
func quotedEntriesOfBlock(block quickBlock, catalog []rendering) []string {
	found := map[string]bool{}
	for _, line := range block.body {
		if strings.HasPrefix(line, "$ ") || exitLine.MatchString(strings.TrimSpace(line)) {
			continue
		}
		for _, key := range quotedEntriesOfLine(line, catalog) {
			found[key] = true
		}
	}
	var keys []string
	for key := range found {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// quotedEntriesOfLine applies the four anchors to one output line.
func quotedEntriesOfLine(line string, catalog []rendering) []string {
	trimmed := strings.TrimLeft(line, " \t")
	starts := map[int]bool{0: true}
	if after := afterRefusalName(trimmed); after > 0 {
		starts[after] = true
	}
	found := map[string]bool{}
	for spreading := true; spreading; {
		spreading = false
		for start := range starts {
			for _, entry := range catalog {
				match := entry.anchored.FindStringSubmatchIndex(trimmed[start:])
				if match == nil {
					continue
				}
				end := start + match[3]
				if !found[entry.key] {
					found[entry.key] = true
					spreading = true
				}
				if !starts[end] {
					starts[end] = true
					spreading = true
				}
				if end < len(trimmed) && trimmed[end] == ' ' && !starts[end+1] {
					starts[end+1] = true
					spreading = true
				}
			}
		}
	}
	for _, field := range columnarFields(line) {
		for _, entry := range catalog {
			if entry.whole.MatchString(field.text) {
				found[entry.key] = true
			}
		}
	}
	var keys []string
	for key := range found {
		keys = append(keys, key)
	}
	return keys
}

// afterRefusalName reports the position just past a leading refusal name and
// the space after it, and zero where the line opens with something else.
func afterRefusalName(trimmed string) int {
	first, _, ok := strings.Cut(trimmed, " ")
	if !ok || first == "" {
		return 0
	}
	if !strings.HasPrefix(first, contract.LayerPrefix) && !contractDeclares(first) {
		return 0
	}
	return len(first) + 1
}

// contractDeclares reports whether a token is one of the profile's own refusal
// names.
func contractDeclares(name string) bool {
	for _, declared := range contract.Declared {
		if declared == name {
			return true
		}
	}
	return false
}

// TestEveryExemptBlockDeclaresTheCatalogEntriesItQuotes holds the part of an
// exempt block's output that the catalog owns, which is the part a card
// editing internal/msg/locales/en.json can break without noticing.
//
// An exempt block is not replayed, so nothing runs its command and nothing
// holds its output. Two rules govern the declaration and together they make it
// mechanical rather than a matter of judgement. Every key an entry declares
// names an entry of the catalog and its rendering appears in that block, so a
// key whose rendering has stopped appearing fails naming the block, the key,
// and the stored text. And no entry the discovery rule finds in the block is
// left out of the declaration, so a thin declaration cannot leave the block
// quietly unguarded and the first run over an empty declaration prints exactly
// what to write.
//
// The second rule reads no part of the declaration in order to compute its
// answer, so a declaration cannot be its own evidence.
//
// This check cannot run over the whole document, and the reason is a measured
// one. Some messages a reader sees are composed from several catalog entries.
// The usage refusal is one, it matches no single entry, and it is byte for
// byte what the binary prints today. Replay separates that case from a
// genuinely stale line and a catalog scan cannot, so the catalog scan is
// confined to the blocks replay does not read.
func TestEveryExemptBlockDeclaresTheCatalogEntriesItQuotes(t *testing.T) {
	_, blocks, entries := quickStartCorpus(t)
	catalog := renderingsOfTheCatalog(t)
	declared := map[int]quickExemption{}
	for _, entry := range entries {
		if !entry.inner {
			declared[entry.at] = entry
		}
	}
	held := 0
	for _, block := range blocks {
		if block.kind != "console" || !block.exempt() {
			continue
		}
		entry, named := declared[block.fence]
		if !named {
			continue
		}
		appearing := map[string]bool{}
		for _, key := range quotedEntriesOfBlock(block, catalog) {
			appearing[key] = true
		}
		for _, key := range entry.quotes {
			held++
			stored, ok := msg.BaseEntry(key)
			if !ok {
				t.Errorf("%s:%d: the entry declares the catalog key %s, which the catalog does not carry", quickStartExemptions, entry.source, key)
				continue
			}
			if appearing[key] {
				continue
			}
			t.Errorf("%s:%d: the block declares that it quotes %s, whose stored text is %q, and that text no longer renders into the block",
				quickStartPath, block.fence, key, stored.Text)
		}
		for key := range appearing {
			if entryDeclares(entry, key) {
				continue
			}
			t.Errorf("%s:%d: the block renders the catalog entry %s and %s does not declare it", quickStartPath, block.fence, key, quickStartExemptions)
		}
	}
	if held == 0 {
		t.Error("no exempt block declares a catalog key, so this check reads nothing")
	}
}

// entryDeclares reports whether an exemption entry names a key.
func entryDeclares(entry quickExemption, key string) bool {
	for _, declared := range entry.quotes {
		if declared == key {
			return true
		}
	}
	return false
}

// environmentName matches a variable name the documentation writes.
var environmentName = regexp.MustCompile(`DINAH_[A-Z_]+`)

// TestTheGuidesNameOnlyVariablesTheProductReads asserts that every DINAH_ name
// a guide or the quick start writes is one the tool or an install script
// reads.
//
// The set is the names the help block's own environment line carries, plus the
// names the two install scripts read, which an AST walk cannot reach because
// one is shell and the other is PowerShell.
func TestTheGuidesNameOnlyVariablesTheProductReads(t *testing.T) {
	read := map[string]bool{}
	entry, ok := msg.BaseEntry("help.environment")
	if !ok {
		t.Fatal("the catalog carries no help.environment entry, so the set of names the tool reads is empty")
	}
	for _, name := range environmentName.FindAllString(entry.Text, -1) {
		read[name] = true
	}
	for _, script := range []string{"scripts/install.sh", "scripts/install.ps1"} {
		source, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(script)))
		if err != nil {
			t.Fatalf("read %s: %v", script, err)
		}
		for _, name := range environmentName.FindAllString(string(source), -1) {
			read[name] = true
		}
	}
	written := 0
	for _, document := range guardedDocuments(t) {
		for number, line := range strings.Split(document.text, "\n") {
			for _, name := range environmentName.FindAllString(line, -1) {
				written++
				if read[name] {
					continue
				}
				t.Errorf("%s:%d: the document names the variable %s, which neither the help block's environment line nor an install script reads", document.name, number+1, name)
			}
		}
	}
	if written == 0 {
		t.Error("no DINAH_ name was found in any guide or in the quick start, so this test proves nothing")
	}
}

// publishedFile matches a URL naming a file of this repository, in either of
// the two shapes the documentation uses: the raw form an install one-liner
// fetches, and the browsing form a reader follows.
var publishedFile = regexp.MustCompile("https://(?:raw\\.githubusercontent\\.com/paulmooreparks/dinah|github\\.com/paulmooreparks/dinah/blob)/[^/\\s]+/([^\\s)`\"']+)")

// documentsThatPublishURLs is the corpus the published-URL check reads: the
// guides and the quick start, and every message catalog beside them.
//
// The catalogs belong in this one corpus because a URL a reader follows does
// not only stand in a document. The help block points at the quick start, that
// pointer is a catalog entry rather than a line of any guide, and a check
// reading the guarded corpus alone leaves the one URL this product prints as
// the one URL nothing holds. Reading the catalogs as files rather than through
// the renderer names the locale and the line a finding stands on, and it holds
// every translation rather than the base alone. The corpus is widened here
// rather than in guardedDocuments because the other checks in this file read
// the catalog as their source of truth and would be reading it against itself.
func documentsThatPublishURLs(t *testing.T) []guardedDocument {
	t.Helper()
	documents := guardedDocuments(t)
	locales, err := filepath.Glob(filepath.Join(repositoryRoot, "internal", "msg", "locales", "*.json"))
	if err != nil {
		t.Fatalf("read the message catalogs: %v", err)
	}
	if len(locales) == 0 {
		t.Fatal("the tree carries no message catalog, so the text this tool prints is outside this corpus")
	}
	for _, path := range locales {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		documents = append(documents, guardedDocument{
			name: "internal/msg/locales/" + filepath.Base(path),
			text: strings.ReplaceAll(string(source), "\r\n", "\n"),
		})
	}
	return documents
}

// TestTheDocumentationNamesOnlyPublishedFilesThatExist asserts that every URL a
// guide, the quick start, or a message catalog gives for a file of this
// repository names a path the working tree carries. The check reads the path
// and not the network, so it holds the install section's own subject, and the
// help block's pointer to the quick start, without making the suite depend on
// GitHub.
func TestTheDocumentationNamesOnlyPublishedFilesThatExist(t *testing.T) {
	named := 0
	for _, document := range documentsThatPublishURLs(t) {
		for number, line := range strings.Split(document.text, "\n") {
			for _, found := range publishedFile.FindAllStringSubmatch(line, -1) {
				named++
				path := strings.TrimSuffix(found[1], ".")
				if _, err := os.Stat(filepath.Join(repositoryRoot, filepath.FromSlash(path))); err == nil {
					continue
				}
				t.Errorf("%s:%d: the document publishes %s, and the repository carries no such file", document.name, number+1, path)
			}
		}
	}
	if named == 0 {
		t.Error("no published URL was found in any guide, in the quick start, or in a message catalog, so this test proves nothing")
	}
}

// releaseArtifact matches the name of a file a release publishes.
var releaseArtifact = regexp.MustCompile(`dinah-(?:linux|darwin|windows)-(?:amd64|arm64)(?:\.exe)?|SHA256SUMS\.txt`)

// TestTheGuidesNameOnlyArtifactsTheReleaseBuilds asserts that every release
// artifact a guide or the quick start names is one the release workflow builds
// or publishes.
//
// The install section teaches a reader to download an artifact by name and to
// verify it against a checksum file by name, so a renamed artifact would leave
// that section telling them to fetch something the release does not publish.
// The check reads the workflow as text, so it holds the names without running
// a release. The argument grammar of the checksum commands standing beside
// those names belongs to another tool and stays unguarded.
func TestTheGuidesNameOnlyArtifactsTheReleaseBuilds(t *testing.T) {
	workflow, err := os.ReadFile(filepath.Join(repositoryRoot, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("read the release workflow: %v", err)
	}
	published := string(workflow)
	named := 0
	for _, document := range guardedDocuments(t) {
		for number, line := range strings.Split(document.text, "\n") {
			for _, artifact := range releaseArtifact.FindAllString(line, -1) {
				named++
				if workflowPublishes(published, artifact) {
					continue
				}
				t.Errorf("%s:%d: the document names the release artifact %s, which .github/workflows/release.yml neither builds nor publishes", document.name, number+1, artifact)
			}
		}
	}
	if named == 0 {
		t.Error("no release artifact was named in any guide or in the quick start, so this test proves nothing")
	}
}

// layoutGuide is the guide whose directory tree this check holds.
const layoutGuide = "workbench-layout"

// TestTheLayoutGuideDrawsPathsTheToolWrites asserts that every path the
// workbench-layout guide's tree draws exists in a workbench built and
// exercised through the head.
//
// The check runs in one direction. Every path the guide draws exists, and not
// every path the tool writes is drawn: the guide leaves out .gitignore and the
// lock files on purpose, and a check demanding completeness would force them
// into a document that is better without them.
//
// It also puts a constraint on the guide that the guide itself does not say:
// every fenced block in it is read as a directory tree, so an author who
// fences a command there is told that the workbench carries no such path. An
// author meeting that message is not looking at a broken workbench, they are
// looking at a fence this check cannot tell from a tree. The check verifying
// the shape it depends on, rather than depending on it silently, is dinah-144
// work rather than a change to the guide.
func TestTheLayoutGuideDrawsPathsTheToolWrites(t *testing.T) {
	text, err := guide.Text(layoutGuide)
	if err != nil {
		t.Fatalf("guide %s: %v", layoutGuide, err)
	}
	drawn := treePathsOf(text)
	if len(drawn) == 0 {
		t.Fatalf("the %s guide draws no path, so this test reads nothing", layoutGuide)
	}
	root := exercisedWorkbench(t)
	var standing []string
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		standing = append(standing, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatalf("walk the workbench: %v", err)
	}
	for _, path := range drawn {
		pattern := regexp.MustCompile("^" + strings.NewReplacer(
			"<id>", "[0-9a-f]{12}",
			"<file>", "[^/]+",
		).Replace(regexp.QuoteMeta(path)) + "$")
		if matchesOne(pattern, standing) {
			continue
		}
		t.Errorf("the %s guide draws %s, and the workbench the head built carries no such path", layoutGuide, path)
	}
}

// matchesOne reports whether any path of a workbench answers a drawn one.
func matchesOne(pattern *regexp.Regexp, standing []string) bool {
	for _, path := range standing {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

// treePathsOf reads the guide's indented tree into the paths it draws,
// relative to the workbench root. A line's parent is the nearest line above it
// at a smaller indent whose own name ends in a slash.
func treePathsOf(text string) []string {
	var paths []string
	type level struct {
		indent int
		prefix string
	}
	open := []level{}
	inBlock := false
	for _, line := range strings.Split(text, "\n") {
		if quickStartMarkerRun(line) > 0 {
			inBlock = !inBlock
			continue
		}
		if !inBlock || strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		name, _, _ := strings.Cut(strings.TrimSpace(line), "  ")
		for len(open) > 0 && open[len(open)-1].indent >= indent {
			open = open[:len(open)-1]
		}
		prefix := ""
		if len(open) > 0 {
			prefix = open[len(open)-1].prefix
		}
		if strings.HasSuffix(name, "/") {
			// The tree's outermost line is the workbench directory
			// itself, and every path below it is read relative to
			// that, so the root contributes no prefix and is not a
			// path of its own.
			if len(open) == 0 {
				open = append(open, level{indent: indent})
				continue
			}
			open = append(open, level{indent: indent, prefix: prefix + name})
			paths = append(paths, strings.TrimSuffix(prefix+name, "/"))
			continue
		}
		paths = append(paths, prefix+name)
	}
	return paths
}

// exercisedWorkbench builds a workbench through the head and exercises it
// until it carries a state, a card, a comment, an attachment, an archived
// card, and a journal of its own, which is every entity the layout guide's
// tree draws.
func exercisedWorkbench(t *testing.T) string {
	t.Helper()
	container := newBench(t)
	root := soleBenchDir(t, container)
	notes := filepath.Join(container, "notes.txt")
	if err := os.WriteFile(notes, []byte("a file to attach\n"), 0o644); err != nil {
		t.Fatalf("write the file to attach: %v", err)
	}
	steps := [][]string{
		{"add", "A card the guide's tree draws"},
		{"comment", "fx-1", "a comment"},
		{"attach", "fx-1", "notes.txt"},
		{"add", "A card to archive"},
		{"archive", "fx-2"},
		{"workbench", "set", "title", "Layout"},
		// A workstream is created, written to and joined, because the guide's
		// tree draws the collection, one workstream's anchor and its journal,
		// and none of the three exists in a workbench that never held one.
		{"workstream", "new", "Layout work"},
		{"workstream", "set", "layout-work", "status", "finished"},
		{"join", "fx-1", "layout-work"},
	}
	for _, argv := range steps {
		if got := runCLI(t, container, argv...); got.code != 0 {
			t.Fatalf("%v: exit %d, %s", argv, got.code, got.errw)
		}
	}
	if _, err := os.Stat(filepath.Join(root, bench.JournalName)); err != nil {
		t.Fatalf("the workbench's own journal was not written, so the tree's %s is unreachable: %v", bench.JournalName, err)
	}
	return root
}

// workflowPublishes reports whether the release workflow names an artifact,
// bounded so that one name is not read as another it happens to open.
//
// A plain substring test would accept `dinah-windows-amd64` on the strength of
// the workflow's `dinah-windows-amd64.exe`, and a Windows binary published
// without its extension is exactly the rename a reader would meet as a
// download that is not there.
func workflowPublishes(workflow, artifact string) bool {
	bounded := regexp.MustCompile(`(^|[^A-Za-z0-9._-])` + regexp.QuoteMeta(artifact) + `([^A-Za-z0-9._-]|$)`)
	return bounded.MatchString(workflow)
}
