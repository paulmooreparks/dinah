package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// The quick-start guard, and the boundary it stops at.
//
// docs/quick-start.md is the document a reader trusts first and the one the
// tool drifts away from fastest, because its transcripts are composed at run
// time from a message catalog, a table renderer, a locale, and the state of a
// workbench. No scan of Go sources can see text built that way, so the load
// here sits on the bytes the head actually writes: every fenced block whose
// first line opens `$ ` is replayed through runCLI and compared against what
// the head prints now. The source-level checks in guide_guard_test.go are
// backstops for the places replay cannot reach.
//
// What this guard cannot see, written down so a later reader does not
// rediscover the boundary by trusting the guard too far:
//
//   - Whether the prose is clear, whether the order helps a beginner, and
//     whether an example teaches the right thing. Nothing mechanical reaches
//     any of it.
//   - A count written out in prose. The document states how many commands
//     `dinah help` lists and how many tools the MCP head carries, and no
//     check here reads either sentence.
//   - A claim the document makes in prose instead of showing. "Dinah does not
//     touch a name you have already given it" is a real promise and no
//     transcript demonstrates it.
//   - A block skipped on the platform running the suite, which is what
//     `platform=` declares.
//   - The order in which the lines of an exempt block stand, the values
//     interpolated into a catalog sentence, and any output line the discovery
//     rule of quotedEntriesOfBlock does not reach. The quoted-entry check
//     holds the stored sentences an exempt block declares and nothing else
//     about it, so a stale argument, a reordered listing, and a changed table
//     value all pass.
//   - Two shapes that discovery rule cannot see. A catalog sentence standing
//     in the middle of a line that no declared rendering and no refusal name
//     precedes is invisible to it. So is a sentence whose rendering carries
//     only letters, digits, and spaces outside its placeholders and whose
//     interpolated value carries a space, since such a rendering reads its
//     placeholders as runs of non-space characters.
//   - The three embedded guides carry no transcript at all, so the replay
//     never reads them, and the source checks are the whole of their
//     coverage. A transcript added to getting-started.md tomorrow is guarded
//     by nothing until somebody widens the replay's corpus.
//   - The argument grammar of the three checksum commands and of the
//     PowerShell install line. Those four lines drive another tool, so
//     nothing here runs them. The URL check holds the URL the PowerShell line
//     fetches and the artifact check holds the filenames all four name, which
//     is the part of that section a reader types.
//   - Whether the document teaches the best route to a result. A transcript
//     that replays exactly is still a transcript of a command nobody should
//     run.
//
// The fence arithmetic below guards the toggling of the block parser and
// nothing else. A truncation moves the parser's count and leaves the line
// scan's alone, so the two readings disagree and the run fails. It does not
// guard the predicate that decides which lines are marker-shaped, because
// both readings need that predicate and quickStartMarkerRun gives them one
// function for it: a miscount arriving in pairs survives the arithmetic, and
// a fence indented inside a list item is the example, since both of its
// markers go missing together. A miscount of a single line does not survive
// it, because the marker count then turns odd.

// quickStartPath is the document this guard holds, relative to this package.
var quickStartPath = filepath.Join("..", "..", "docs", "quick-start.md")

// quickStartExemptions names every block the replay does not drive, with the
// reason and the catalog entries the block quotes. It ships in the tree beside
// the checks that read it.
var quickStartExemptions = filepath.Join("testdata", "quickstart-exempt.txt")

// narrativeRoot is the home directory the document's session ran under. Every
// absolute path an output line of a transcript block shows lies under it, so a
// regeneration run that wrote a build machine's temporary directory into the
// document is a reported defect rather than a silent one.
const narrativeRoot = "/home/ana"

// replayColumns is the window every replayed block is drawn at, and the width
// the document's blocks are regenerated at.
//
// cmd/dinah/table.go decides whether to draw a table or a stack, and where a
// cell wraps, from the real values before any normalisation reaches them. A
// runner whose temporary directory sits deeper than the document's own root
// therefore draws a stack where the document draws a table, and no path
// normalisation can undo it. Pinning the window takes the path out of both
// decisions. It is a documented contract rather than a measured one: POSIX
// XBD Chapter 8 defines COLUMNS as the user's preferred width, and
// windowWidth in row.go reads it.
const replayColumns = "400"

// wideningColumns is the second width every block is replayed at, purely so
// that a block which stacked or wrapped at replayColumns is reported as such
// rather than reading as content drift.
//
// A table whose columns fit its window renders identically at any wider one,
// since every width is measured from the values in the block and the last
// column takes whatever is left. So a block whose normalised bytes differ
// between the two widths is a block the narrower window could not hold, which
// is what AC-16 asks to arrive as a named failure.
const wideningColumns = "4000"

// updateQuickStart rewrites the document's transcript blocks from the replay
// and then fails the run. Failing after the rewrite is what stops a CI
// invocation passing in update mode and stops an author leaving it switched
// on.
var updateQuickStart = flag.Bool("update-quick-start", false, "rewrite the transcript blocks of docs/quick-start.md from the replay, then fail")

// quickBlock is one fenced block of the document.
type quickBlock struct {
	// fence is the one-based line the opening marker stands on.
	fence int
	// run is how many backticks that marker carries.
	run int
	// info is the marker's info string, trimmed.
	info string
	// kind is the first word of the info string: console, file, or empty.
	kind string
	// directives are the key=value pairs after the kind, in written order.
	directives []quickDirective
	// body is the lines between the two markers.
	body []string
	// bodyAt is the one-based line the first body line stands on.
	bodyAt int
}

// quickDirective is one key=value pair of a fence's info string.
type quickDirective struct {
	key   string
	value string
}

// directive returns the value of the first directive with a key, and whether
// the block carries one at all.
func (b quickBlock) directive(key string) (string, bool) {
	for _, d := range b.directives {
		if d.key == key {
			return d.value, true
		}
	}
	return "", false
}

// commandBlock reports whether a block's first line opens `$ `, which is what
// makes it a transcript rather than a listing.
func (b quickBlock) commandBlock() bool {
	return len(b.body) > 0 && strings.HasPrefix(b.body[0], "$ ")
}

// quickStep is one command of a transcript block with the output and the exit
// code the document shows for it.
type quickStep struct {
	// line is the one-based document line the command stands on.
	line int
	// command is the command line with its leading `$ ` removed.
	command string
	// output is the lines the document shows the command printing.
	output []string
	// exit is the code the document shows, and hasExit says whether the
	// document wrote one. A step showing none expects zero.
	exit    int
	hasExit bool
}

// quickStartMarkerRun reports how many backticks open a line, and zero when
// the line is not marker-shaped. Both readings of the document's fences use
// this one predicate, which is the limit the arithmetic's doc comment states.
func quickStartMarkerRun(line string) int {
	run := 0
	for run < len(line) && line[run] == '`' {
		run++
	}
	if run < 3 {
		return 0
	}
	return run
}

// exitLine matches the line that closes a step with its expected exit code.
var exitLine = regexp.MustCompile(`^\[exit (\d+)\]$`)

// readQuickStart reads the document into its lines, with any carriage returns
// dropped so a checkout under either line-ending convention parses alike.
func readQuickStart(t *testing.T) []string {
	t.Helper()
	source, err := os.ReadFile(quickStartPath)
	if err != nil {
		t.Fatalf("read %s: %v", quickStartPath, err)
	}
	return strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n")
}

// parseQuickStart walks the document's lines into blocks, closing a fence only
// on a marker whose backtick run is the length of the one that opened it. That
// is what lets a four-backtick fence carry a three-backtick example as the
// content it is.
func parseQuickStart(lines []string) []quickBlock {
	var blocks []quickBlock
	for i := 0; i < len(lines); i++ {
		run := quickStartMarkerRun(lines[i])
		if run == 0 {
			continue
		}
		block := quickBlock{fence: i + 1, run: run, info: strings.TrimSpace(lines[i][run:]), bodyAt: i + 2}
		block.kind, block.directives = parseFenceInfo(block.info)
		j := i + 1
		for j < len(lines) && quickStartMarkerRun(lines[j]) != run {
			block.body = append(block.body, lines[j])
			j++
		}
		if j >= len(lines) {
			// An unterminated fence. The integrity rules report it;
			// the block is kept so the arithmetic can see it.
			blocks = append(blocks, block)
			return blocks
		}
		blocks = append(blocks, block)
		i = j
	}
	return blocks
}

// parseFenceInfo splits an info string into its kind and its directives. A
// `skip=` directive takes the rest of the string, since its reason is prose
// and prose carries spaces.
func parseFenceInfo(info string) (string, []quickDirective) {
	if info == "" {
		return "", nil
	}
	kind, rest, _ := strings.Cut(info, " ")
	var directives []quickDirective
	for rest != "" {
		rest = strings.TrimLeft(rest, " ")
		if rest == "" {
			break
		}
		if reason, found := strings.CutPrefix(rest, "skip="); found {
			directives = append(directives, quickDirective{key: "skip", value: reason})
			break
		}
		word, remainder, _ := strings.Cut(rest, " ")
		key, value, ok := strings.Cut(word, "=")
		if ok {
			directives = append(directives, quickDirective{key: key, value: value})
		}
		rest = remainder
	}
	return kind, directives
}

// steps reads a transcript block into the commands it runs and the output and
// exit code the document shows for each. A line opening `$ ` is a command, the
// lines after it are its expected output, and `[exit N]` closes the step.
func (b quickBlock) steps() []quickStep {
	var steps []quickStep
	for i, line := range b.body {
		if command, found := strings.CutPrefix(line, "$ "); found {
			steps = append(steps, quickStep{line: b.bodyAt + i, command: command})
			continue
		}
		if len(steps) == 0 {
			continue
		}
		current := &steps[len(steps)-1]
		if match := exitLine.FindStringSubmatch(line); match != nil {
			current.exit, _ = strconv.Atoi(match[1])
			current.hasExit = true
			continue
		}
		current.output = append(current.output, line)
	}
	return steps
}

// quickExemption is one entry of the exemption file: a block the replay does
// not drive, or a marker-shaped line that is a block's content rather than a
// fence.
type quickExemption struct {
	// at is the line the entry names: a block's opening fence, or the
	// declared inner marker itself.
	at int
	// inner says the entry declares a marker-shaped line inside a block.
	inner bool
	// reason is why, and it is required.
	reason string
	// quotes are the catalog keys the block's output renders, and quotesNone
	// says the block renders none. because carries why it renders none.
	quotes     []string
	quotesNone bool
	because    string
	// declared says whether a quotes= or quotes=none directive was written
	// at all, which an entry for a block must carry.
	declared bool
	// source is the one-based line of the entry, for a finding to name.
	source int
}

// exemptionKeys are the directives an entry may carry. A word carrying one of
// them opens a new value, and every other word continues the value it stands
// in, so a reason may be prose.
var exemptionKeys = []string{"reason", "quotes", "because"}

// readQuickStartExemptions reads the exemption file. A blank line and a line
// opening with a hash are commentary.
func readQuickStartExemptions(t *testing.T) []quickExemption {
	t.Helper()
	source, err := os.ReadFile(quickStartExemptions)
	if err != nil {
		t.Fatalf("read %s: %v", quickStartExemptions, err)
	}
	var entries []quickExemption
	lines := strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n")
	for number, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		entry, err := parseExemption(trimmed)
		if err != nil {
			t.Errorf("%s:%d: %v", quickStartExemptions, number+1, err)
			continue
		}
		entry.source = number + 1
		entries = append(entries, entry)
	}
	return entries
}

// parseExemption reads one entry. The first word is the line the entry names,
// optionally preceded by `inner` for a declared marker-shaped line.
func parseExemption(line string) (quickExemption, error) {
	entry := quickExemption{}
	first, rest, _ := strings.Cut(line, " ")
	if first == "inner" {
		entry.inner = true
		first, rest, _ = strings.Cut(rest, " ")
	}
	at, err := strconv.Atoi(first)
	if err != nil {
		return entry, fmt.Errorf("the entry opens %q, and an entry opens with the line it names", first)
	}
	entry.at = at
	for key, value := range splitDirectives(rest) {
		switch key {
		case "reason":
			entry.reason = value
		case "because":
			entry.because = value
		case "quotes":
			entry.declared = true
			if value == "none" {
				entry.quotesNone = true
				continue
			}
			for _, quoted := range strings.Split(value, ",") {
				if quoted = strings.TrimSpace(quoted); quoted != "" {
					entry.quotes = append(entry.quotes, quoted)
				}
			}
		}
	}
	return entry, nil
}

// splitDirectives reads a run of key=value directives whose values carry
// spaces. A word whose text before its first equals sign is one of the known
// keys opens a value; every other word continues the value it stands in.
func splitDirectives(rest string) map[string]string {
	found := map[string]string{}
	key := ""
	var value []string
	flush := func() {
		if key != "" {
			found[key] = strings.Join(value, " ")
		}
	}
	for _, word := range strings.Fields(rest) {
		name, opened, ok := strings.Cut(word, "=")
		if ok && knownExemptionKey(name) {
			flush()
			key, value = name, nil
			if opened != "" {
				value = append(value, opened)
			}
			continue
		}
		value = append(value, word)
	}
	flush()
	return found
}

// knownExemptionKey reports whether a word names one of the exemption file's
// own directives.
func knownExemptionKey(name string) bool {
	for _, known := range exemptionKeys {
		if known == name {
			return true
		}
	}
	return false
}

// exempt reports whether a block is one the replay does not drive, either
// because it carries `skip=` or because this platform is outside its
// `platform=` list.
func (b quickBlock) exempt() bool {
	if _, skipped := b.directive("skip"); skipped {
		return true
	}
	platforms, pinned := b.directive("platform")
	if !pinned {
		return false
	}
	for _, platform := range strings.Split(platforms, ",") {
		if strings.TrimSpace(platform) == runtime.GOOS {
			return false
		}
	}
	return true
}

// normalisationClass is one value that legitimately differs between two hosts,
// with the token that stands in for it on both sides of every comparison.
//
// The set is closed and it is declared here alone, so a class added later
// stands out in the diff rather than blending into the table it joins. A
// reviewer reads such an addition as a design change.
type normalisationClass struct {
	// name is what a finding calls the class.
	name string
	// token is what stands in for the value on both sides.
	token string
	// pattern locates the value, and group is the submatch carrying it. A
	// group above zero is how a class states the context it needs to
	// recognise a value without swallowing that context.
	pattern *regexp.Regexp
	group   int
}

// normalisationTable is every value the replay normalises, in the order the
// classes are applied. Nothing else is normalised: a count Dinah prints is
// compared as written, so the catalog listing fails the moment a card adds a
// message key, and every sentence of prose the head emits is compared as
// written, which is the whole point of the guard.
var normalisationTable = []normalisationClass{
	{
		// The release value varies per build rather than per host:
		// verb.ToolRelease defaults to a development value and a
		// release build overwrites it through -ldflags -X. The class is
		// deliberately narrow. A token wide enough to swallow the whole
		// line would also swallow a change to the shape of the line, so
		// the value is normalised inside a fixed shape and a line
		// spelled any other way fails instead of passing silently.
		name:    "release",
		token:   "<release>",
		pattern: regexp.MustCompile(`(?m)^(dinah )([0-9][0-9A-Za-z.+-]*)$`),
		group:   2,
	},
	{
		// A card's revision covers bytes that carry timestamps, so it
		// differs on every run. No other route was available, since the
		// digest is the content of the card as the reader read it.
		name:    "revision",
		token:   "<revision>",
		pattern: regexp.MustCompile(`sha256:[0-9a-f]+`),
		group:   0,
	},
	{
		// The instant of the run.
		name:    "timestamp",
		token:   "<ts>",
		pattern: regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`),
		group:   0,
	},
	{
		// The drive, the separator, and the home directory all differ
		// per host. The leading group is the character before the path,
		// which the class needs so that a slash inside a word such as a
		// profile name is not read as the root of one, and which it
		// keeps out of the token rather than swallowing.
		name:    "path",
		token:   "<path>",
		pattern: regexp.MustCompile("(?m)(^|[\\s(>])((?:[A-Za-z]:[\\\\/]|/)[^\\s\"'`,;)\\]]+)"),
		group:   2,
	},
	{
		// Minted per workbench, state, card, comment, and attachment.
		name:    "identifier",
		token:   "<id>",
		pattern: regexp.MustCompile(`\b[0-9a-f]{12}\b`),
		group:   0,
	},
}

// quickSegment is one piece of a stream: either literal text, or one
// occurrence of a normalisation class.
type quickSegment struct {
	// class is the class the segment carries, and empty for literal text.
	class string
	// text is the bytes the segment stands for.
	text string
}

// segmentStream splits a stream into literal text and the class occurrences
// inside it, applying the classes in the order the table declares them. Both
// the normalised form and the restoration read this one segmentation, so what
// the comparison erases and what the update puts back are the same positions.
func segmentStream(text string) []quickSegment {
	segments := []quickSegment{{text: text}}
	for _, class := range normalisationTable {
		var next []quickSegment
		for _, segment := range segments {
			if segment.class != "" {
				next = append(next, segment)
				continue
			}
			next = append(next, splitByClass(segment.text, class)...)
		}
		segments = next
	}
	return segments
}

// splitByClass splits one run of literal text on a class's own pattern.
func splitByClass(text string, class normalisationClass) []quickSegment {
	var segments []quickSegment
	at := 0
	for _, found := range class.pattern.FindAllStringSubmatchIndex(text, -1) {
		start, end := found[2*class.group], found[2*class.group+1]
		if start < 0 {
			continue
		}
		if start > at {
			segments = append(segments, quickSegment{text: text[at:start]})
		}
		segments = append(segments, quickSegment{class: class.name, text: text[start:end]})
		at = end
	}
	if at < len(text) {
		segments = append(segments, quickSegment{text: text[at:]})
	}
	return segments
}

// normalise replaces every class occurrence with its token, which is the form
// both sides of a comparison are read in.
func normalise(text string) string {
	var built strings.Builder
	for _, segment := range segmentStream(text) {
		if segment.class == "" {
			built.WriteString(segment.text)
			continue
		}
		built.WriteString(tokenOf(segment.class))
	}
	return built.String()
}

// tokenOf returns the token a class normalises to.
func tokenOf(name string) string {
	for _, class := range normalisationTable {
		if class.name == name {
			return class.token
		}
	}
	return "<unknown>"
}

// valuesByClass collects a stream's class occurrences in order of appearance,
// which is what the restoration matches positions against.
func valuesByClass(text string) map[string][]string {
	found := map[string][]string{}
	for _, segment := range segmentStream(text) {
		if segment.class == "" {
			continue
		}
		found[segment.class] = append(found[segment.class], segment.text)
	}
	return found
}

// restoreDocumentValues writes the document's own value back into captured
// bytes at every normalised occurrence, matched by class and by order of
// appearance.
//
// Writing the captured bytes into the document as they stand would put a build
// machine's temporary directory on a page a customer reads, and every check
// would stay green afterwards, because a path is exactly what the comparison
// normalises away on both sides. Where the captured output holds more
// occurrences of a class than the document did, the surplus has no value to
// restore, so the caller is refused and leaves the block alone, which sends a
// person to write the new line by hand: only a person knows what the
// narrative's version of a new path should read. Where it holds fewer, the
// surplus document values are dropped, since the output genuinely stopped
// showing them.
//
// The rule is positional, so it is right for a block whose values keep their
// order and wrong for one that reorders two values of the same class, where it
// writes the first document value into the second output slot. Nothing
// standing reports that: the narrative-root rule accepts the result because
// both values lie under the narrative's root, and the replay accepts it
// because the class is what the comparison normalises away on both sides.
func restoreDocumentValues(captured string, document map[string][]string) (string, error) {
	for class, occurrences := range valuesByClass(captured) {
		if len(occurrences) > len(document[class]) {
			return "", fmt.Errorf("the captured output holds %d values of class %s and the document held %d, so the surplus has no value to restore", len(occurrences), class, len(document[class]))
		}
	}
	used := map[string]int{}
	var built strings.Builder
	for _, segment := range segmentStream(captured) {
		if segment.class == "" {
			built.WriteString(segment.text)
			continue
		}
		at := used[segment.class]
		used[segment.class]++
		built.WriteString(document[segment.class][at])
	}
	return built.String(), nil
}

// ruleGlyphRun matches a field drawn entirely from the table separator's own
// glyph, whatever its length. A separator row is not one run of glyphs but
// several, each tracking the width of the column above it, so canonicalising
// the field is what carries that row through a comparison that has dropped
// padding.
var ruleGlyphRun = regexp.MustCompile(`^` + regexp.QuoteMeta(string(ruleGlyph)) + `+$`)

// comparableFields reads one line into the fields a comparison reads: the
// whole line where it carries no run of two or more spaces, and its fields
// otherwise, with a field of the rule glyph canonicalised to one token.
//
// Column widths are computed from the values in a block, and a path is the
// class whose display width differs per host, so a byte comparison of a table
// containing a path can only pass on the machine that captured it. Alignment
// within a block is not lost with the padding: checkColumnsLineUp runs over
// both streams of every runCLI invocation and therefore over every block this
// replay drives, so the transcript check holds the content and the output
// check holds the shape. One narrow gap in that handoff is worth naming.
// foldColumnarBlocks only forms a block from a run of two or more consecutive
// lines sharing an indent and a field count, so a single output line carrying
// a run of two or more spaces has its padding dropped here and picks up no
// alignment check to replace it.
func comparableFields(line string) []string {
	if !strings.Contains(line, "  ") {
		return []string{line}
	}
	var fields []string
	for _, field := range columnarFields(line) {
		if ruleGlyphRun.MatchString(field.text) {
			fields = append(fields, "<rule>")
			continue
		}
		fields = append(fields, field.text)
	}
	return fields
}

// linesAgree reports whether two lines carry the same content once padding is
// out of the comparison.
func linesAgree(wanted, got string) bool {
	a, b := comparableFields(wanted), comparableFields(got)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// firstDisagreement reports the first line at which two streams differ, in the
// shape diffLines already uses, and the empty string when they agree.
func firstDisagreement(wanted, got []string) string {
	for i := range wanted {
		if i >= len(got) {
			return "  line " + strconv.Itoa(i+1) + "\n  wanted: " + wanted[i] + "\n  got:    the output ends here"
		}
		if !linesAgree(wanted[i], got[i]) {
			return "  line " + strconv.Itoa(i+1) + "\n  wanted: " + wanted[i] + "\n  got:    " + got[i]
		}
	}
	if len(got) > len(wanted) {
		return "  line " + strconv.Itoa(len(wanted)+1) + "\n  wanted: the output ends here\n  got:    " + got[len(wanted)]
	}
	return ""
}

// streamLines reads a captured stream into the lines a transcript shows,
// dropping the empty line a trailing newline leaves behind.
func streamLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// quickCapture is one block's replay: what each of its commands printed and
// the code it exited with.
type quickCapture struct {
	// fence is the block's opening line, which is how a capture is matched
	// back to the document.
	fence int
	// steps are the block's commands in order.
	steps []quickCaptured
}

// quickCaptured is one command of a replayed block.
type quickCaptured struct {
	// command is the command line as the document wrote it.
	command string
	// text is the stream or streams the block declares, as captured.
	text string
	// code is the exit code the head returned.
	code int
}

// TestTheQuickStartMatchesTheTool replays every transcript block of
// docs/quick-start.md through the head and compares both what it printed and
// the code it exited with against what the document shows.
//
// The file's own doc comment above says what this guard covers and where it
// stops. Read that before trusting a green run further than it goes.
func TestTheQuickStartMatchesTheTool(t *testing.T) {
	lines := readQuickStart(t)
	blocks := parseQuickStart(lines)
	if len(blocks) == 0 {
		t.Fatalf("%s carries no fenced block, so this test reads nothing", quickStartPath)
	}
	entries := readQuickStartExemptions(t)
	if len(entries) == 0 {
		t.Fatalf("%s names no block, so the exemption rules read nothing", quickStartExemptions)
	}
	inner := map[int]bool{}
	for _, entry := range entries {
		if entry.inner {
			inner[entry.at] = true
		}
	}
	if !fencesAreWhole(t, lines, blocks, inner) {
		t.Fatal("the document does not parse whole, so every count below would be taken over a smaller corpus than the file")
	}
	checkExemptionEntries(t, blocks, entries)
	checkTranscriptPathsAreTheNarrative(t, blocks)

	captured := replayQuickStart(t, blocks, replayColumns)
	if len(captured) == 0 {
		t.Fatal("the replay drove no block, so this test proves nothing")
	}
	checkNothingStacksOrWraps(t, blocks, captured, replayQuickStart(t, blocks, wideningColumns))

	if *updateQuickStart {
		rewriteQuickStart(t, lines, blocks, captured)
		t.Fatalf("update mode rewrote %s and fails on purpose, so no run in this mode can pass; read the diff and commit it", quickStartPath)
	}
	compareQuickStart(t, blocks, captured)
}

// fencesAreWhole runs the four integrity rules that make a truncated read fail
// rather than shrink the corpus quietly, and reports whether they all passed.
//
// A parser that toggles on fence markers can be switched off by the document
// it reads: one unterminated fence swallows the rest of the file, the corpus
// shrinks without emptying, and the run stays green. An empty-corpus assertion
// does not catch that, because a truncated corpus is not an empty one.
//
// Rules one and three are refinements of rule four rather than separate tests,
// since an unterminated fence leaves the parser one block short and an odd
// marker count has no exact half. They are kept for the messages they give.
func fencesAreWhole(t *testing.T, lines []string, blocks []quickBlock, inner map[int]bool) bool {
	t.Helper()
	type marker struct {
		at  int
		run int
	}
	var scanned []marker
	declared := 0
	var fences []marker
	for i, line := range lines {
		run := quickStartMarkerRun(line)
		if run == 0 {
			continue
		}
		scanned = append(scanned, marker{at: i + 1, run: run})
		if inner[i+1] {
			declared++
			continue
		}
		fences = append(fences, marker{at: i + 1, run: run})
	}
	if len(scanned) == 0 {
		t.Errorf("%s carries no fence marker at all, so the arithmetic below reads nothing", quickStartPath)
		return false
	}
	whole := true
	if len(fences)%2 != 0 {
		t.Errorf("%s carries %d fence markers that no entry of %s declares as block content, which is odd and so has no exact half", quickStartPath, len(fences), quickStartExemptions)
		whole = false
	}
	for i := 0; i+1 < len(fences); i += 2 {
		if fences[i].run == fences[i+1].run {
			continue
		}
		t.Errorf("%s:%d: the fence opening here carries %d backticks and the marker closing it at line %d carries %d",
			quickStartPath, fences[i].at, fences[i].run, fences[i+1].at, fences[i+1].run)
		whole = false
	}
	if len(fences)%2 != 0 {
		last := fences[len(fences)-1]
		t.Errorf("%s:%d: the fence opening here has no closing marker", quickStartPath, last.at)
	}
	if 2*len(blocks)+declared != len(scanned) {
		t.Errorf("%s: the parser returned %d blocks and %s declares %d marker-shaped lines as block content, which accounts for %d of the %d marker-shaped lines a plain line scan counted",
			quickStartPath, len(blocks), quickStartExemptions, declared, 2*len(blocks)+declared, len(scanned))
		whole = false
	}
	return whole
}

// checkTranscriptPathsAreTheNarrative requires every absolute path on an
// output line of a transcript block to lie under the narrative's own root.
//
// This is what stops a regeneration run writing a build machine's temporary
// directory into a page a customer reads, which would otherwise leave every
// other check green, because a path is exactly what the comparison normalises
// away on both sides.
//
// The rule reads output lines and not command lines, and the narrowing is the
// whole of what makes it true of this document. A build machine's temporary
// directory arrives in output and never in a command line somebody typed in by
// hand, so a command line carries nothing this rule exists to catch, and the
// document's own command lines carry a /dev/null that a rule reading them
// would have failed on the first time it ran.
func checkTranscriptPathsAreTheNarrative(t *testing.T, blocks []quickBlock) {
	t.Helper()
	read := 0
	for _, block := range blocks {
		if block.kind != "console" {
			continue
		}
		for i, line := range block.body {
			if strings.HasPrefix(line, "$ ") || exitLine.MatchString(strings.TrimSpace(line)) {
				continue
			}
			for _, shown := range valuesByClass(line)["path"] {
				read++
				if strings.HasPrefix(shown, narrativeRoot) {
					continue
				}
				t.Errorf("%s:%d: the output line shows %s, and every absolute path a transcript block shows lies under %s",
					quickStartPath, block.bodyAt+i, shown, narrativeRoot)
			}
		}
	}
	if read == 0 {
		t.Error("no transcript block shows an absolute path, so the narrative-root rule read nothing")
	}
}

// checkExemptionEntries holds the exemption file against the document. Each
// entry is read as a finding rather than as configuration: an entry naming a
// block that is not exempt fails, an entry carrying no reason fails, an exempt
// block with no entry fails, an entry carrying neither quotes= nor quotes=none
// fails, and a quotes=none entry carrying no because= fails.
func checkExemptionEntries(t *testing.T, blocks []quickBlock, entries []quickExemption) {
	t.Helper()
	exempt := map[int]bool{}
	for _, block := range blocks {
		if block.kind == "console" && block.exempt() {
			exempt[block.fence] = true
		}
	}
	named := map[int]bool{}
	for _, entry := range entries {
		if strings.TrimSpace(entry.reason) == "" {
			t.Errorf("%s:%d: the entry for line %d carries no reason, and an entry is a finding rather than a line of configuration",
				quickStartExemptions, entry.source, entry.at)
		}
		if entry.inner {
			continue
		}
		named[entry.at] = true
		if !exempt[entry.at] {
			t.Errorf("%s:%d: the entry names the block at %s:%d, which the replay drives",
				quickStartExemptions, entry.source, quickStartPath, entry.at)
		}
		if !entry.declared {
			t.Errorf("%s:%d: the entry for the block at %s:%d declares neither quotes= nor quotes=none",
				quickStartExemptions, entry.source, quickStartPath, entry.at)
		}
		if entry.quotesNone && strings.TrimSpace(entry.because) == "" {
			t.Errorf("%s:%d: the entry for the block at %s:%d declares quotes=none and does not say because what",
				quickStartExemptions, entry.source, quickStartPath, entry.at)
		}
	}
	var missing []int
	for fence := range exempt {
		if !named[fence] {
			missing = append(missing, fence)
		}
	}
	sort.Ints(missing)
	for _, fence := range missing {
		t.Errorf("%s:%d: the replay does not drive this block and %s does not name it", quickStartPath, fence, quickStartExemptions)
	}
	if len(exempt) == 0 {
		t.Error("no block of the document is exempt, so the exemption rules read nothing")
	}
}

// undrivable reports why the replay cannot drive a command line, and the empty
// string when it can.
//
// Both rules are needed and each catches a line the other admits. The first
// word rule puts the install one-liner out of reach, whose first word is curl.
// The shell-syntax rule puts the POSIX pipe out of reach, whose first word is
// dinah and which only this rule catches.
func undrivable(command string) string {
	for _, syntax := range []string{"|", ">", "<", "&", ";", "`", "$("} {
		if strings.Contains(command, syntax) {
			return "the line carries " + syntax + ", which the in-process head cannot honour"
		}
	}
	first, _, _ := strings.Cut(command, " ")
	if first != "dinah" && first != "mkdir" && first != "cd" {
		return "the first word is " + first + ", which the in-process head cannot run"
	}
	return ""
}

// benchArgument matches the workbench segment of a command-line argument that
// names a workbench by an identifier the document minted.
var benchArgument = regexp.MustCompile(`\.dinah/[0-9a-f]{12}`)

// substituteWorkbench rewrites the one thing the replay changes about a
// command line: a `.dinah/<twelve hex>` segment naming a workbench the
// document minted becomes the workbench directory that actually exists under
// the same parent. No other substitution touches a command line, and a parent
// holding no workbench, or more than one, fails rather than guessing.
func substituteWorkbench(t *testing.T, block quickBlock, cwd string, argv []string) []string {
	t.Helper()
	rewritten := make([]string, len(argv))
	for i, argument := range argv {
		found := benchArgument.FindStringIndex(argument)
		if found == nil {
			rewritten[i] = argument
			continue
		}
		parent := filepath.Join(cwd, filepath.FromSlash(argument[:found[0]]))
		ids := bench.ListIDs(filepath.Join(parent, bench.UserBaseName))
		if len(ids) != 1 {
			t.Fatalf("%s:%d: the argument %s names one workbench under %s and the replay built %d there",
				quickStartPath, block.fence, argument, argument[:found[0]], len(ids))
		}
		rewritten[i] = argument[:found[0]] + bench.UserBaseName + "/" + ids[0] + argument[found[1]:]
	}
	return rewritten
}

// applyBlockEnvironment sets the variables a block's env= directives declare
// for the length of that block, and returns the call that puts the environment
// back.
func applyBlockEnvironment(block quickBlock) func() {
	var restore []func()
	for _, directive := range block.directives {
		if directive.key != "env" {
			continue
		}
		name, value, ok := strings.Cut(directive.value, "=")
		if !ok {
			continue
		}
		previous, had := os.LookupEnv(name)
		os.Setenv(name, value)
		restore = append(restore, func() {
			if had {
				os.Setenv(name, previous)
				return
			}
			os.Unsetenv(name)
		})
	}
	return func() {
		for _, put := range restore {
			put()
		}
	}
}

// replayQuickStart walks the document in order and drives every block the
// replay is not exempt from, at one window width, returning what each block
// printed.
//
// The sandbox is one t.TempDir() tree with the user base pointed inside it and
// every variable the ladders read cleared, in the shape newBench uses. TestMain
// already calls testenv.IsolateTempDir, which puts the whole fixture tree
// outside the developer's home so the ancestor walk cannot reach a real
// workbench, so the replay needs no isolation of its own and writes nothing
// outside the tree t.TempDir() gave it.
func replayQuickStart(t *testing.T, blocks []quickBlock, columns string) map[int]quickCapture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("DINAH_HOME", root)
	t.Setenv("DINAH_ACTOR", "")
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_EDITOR", "")
	t.Setenv("COLUMNS", columns)
	captured := map[int]quickCapture{}
	cwd := root
	for _, block := range blocks {
		switch {
		case block.kind == "file":
			writeNarrativeFile(t, cwd, block)
		case block.kind == "console" && !block.exempt():
			capture, next := replayBlock(t, cwd, block)
			captured[block.fence] = capture
			cwd = next
		}
	}
	return captured
}

// writeNarrativeFile writes the bytes a file block supplies to the path it
// names, before the next command runs.
//
// A listing the document shows carries the narrative's own identifiers, and
// the sandbox minted its own, so the identifiers standing in the file this run
// built are restored into those bytes before they land. Writing the document's
// values would name states no workbench here declares.
func writeNarrativeFile(t *testing.T, cwd string, block quickBlock) {
	t.Helper()
	where, ok := block.directive("path")
	if !ok {
		t.Fatalf("%s:%d: the file block declares no path=", quickStartPath, block.fence)
	}
	target := narrativeFileTarget(t, cwd, where)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("%s:%d: %v", quickStartPath, block.fence, err)
	}
	body := strings.Join(block.body, "\n") + "\n"
	if standing, err := os.ReadFile(target); err == nil {
		restored, err := restoreDocumentValues(body, valuesByClass(string(standing)))
		if err != nil {
			t.Fatalf("%s:%d: %v", quickStartPath, block.fence, err)
		}
		body = restored
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatalf("%s:%d: %v", quickStartPath, block.fence, err)
	}
}

// stateReference matches the one placeholder a file block's path may carry
// below the workbench, which names a state by its slug because the identifier
// naming its directory is minted per run.
var stateReference = regexp.MustCompile(`<state:([a-z0-9-]+)>`)

// narrativeFileTarget resolves a file block's path= against the sandbox.
// `<workbench>` expands to the sole workbench directory under the current
// directory, resolved the way soleBenchDir resolves it.
func narrativeFileTarget(t *testing.T, cwd, where string) string {
	t.Helper()
	rest, rooted := strings.CutPrefix(where, "<workbench>")
	if !rooted {
		return filepath.Join(cwd, filepath.FromSlash(where))
	}
	root := soleBenchDir(t, cwd)
	rest = strings.TrimPrefix(rest, "/")
	if found := stateReference.FindStringSubmatchIndex(rest); found != nil {
		rest = rest[:found[0]] + stateDirectoryOf(t, root, rest[found[2]:found[3]]) + rest[found[1]:]
	}
	return filepath.Join(root, filepath.FromSlash(rest))
}

// stateDirectoryOf returns the identifier naming the directory of the state
// carrying a slug.
func stateDirectoryOf(t *testing.T, root, slug string) string {
	t.Helper()
	opened, err := bench.Open(root)
	if err != nil {
		t.Fatalf("open the workbench at %s: %v", root, err)
	}
	for _, state := range opened.States {
		if state.Slug == slug {
			return state.ID
		}
	}
	t.Fatalf("the workbench at %s declares no state with the slug %s", root, slug)
	return ""
}

// replayBlock drives one transcript block and returns what it printed and the
// directory the block left the replay standing in.
func replayBlock(t *testing.T, cwd string, block quickBlock) (quickCapture, string) {
	t.Helper()
	restore := applyBlockEnvironment(block)
	defer restore()
	stream, _ := block.directive("stream")
	capture := quickCapture{fence: block.fence}
	for _, step := range block.steps() {
		if reason := undrivable(step.command); reason != "" {
			t.Errorf("%s:%d: `%s` is out of the replay's reach because %s, so the block needs skip=",
				quickStartPath, step.line, step.command, reason)
			return capture, cwd
		}
		argv, err := tokenize(step.command)
		if err != nil {
			t.Fatalf("%s:%d: %v", quickStartPath, step.line, err)
		}
		if len(argv) == 0 {
			continue
		}
		switch argv[0] {
		case "mkdir", "cd":
			if len(argv) != 2 {
				t.Fatalf("%s:%d: `%s` takes exactly one argument", quickStartPath, step.line, step.command)
			}
			if argv[0] == "cd" {
				cwd = filepath.Clean(filepath.Join(cwd, filepath.FromSlash(argv[1])))
				capture.steps = append(capture.steps, quickCaptured{command: step.command})
				continue
			}
			if err := os.MkdirAll(filepath.Join(cwd, filepath.FromSlash(argv[1])), 0o755); err != nil {
				t.Fatalf("%s:%d: %v", quickStartPath, step.line, err)
			}
			capture.steps = append(capture.steps, quickCaptured{command: step.command})
		default:
			got := runCLI(t, cwd, substituteWorkbench(t, block, cwd, argv[1:])...)
			capture.steps = append(capture.steps, quickCaptured{command: step.command, text: streamOf(stream, got), code: got.code})
		}
	}
	return capture, cwd
}

// streamOf reads the stream or streams a block declares out of one invocation.
// The default is both, standard output first.
func streamOf(stream string, got invocation) string {
	switch stream {
	case "out":
		return got.out
	case "err":
		return got.errw
	default:
		return got.out + got.errw
	}
}

// checkNothingStacksOrWraps requires every replayed block to render the same
// at two window widths.
//
// The stacking and wrapping decisions read the real values before any
// normalisation reaches them, so a sandbox path long enough turns a table into
// a stack or wraps a cell, which no path normalisation can undo. A pinned
// window is headroom rather than a guarantee, since a path long enough stacks
// a table at any fixed width, and this check is what makes that arrive as a
// named failure instead of reading as content drift.
func checkNothingStacksOrWraps(t *testing.T, blocks []quickBlock, pinned, wider map[int]quickCapture) {
	t.Helper()
	for _, block := range blocks {
		narrow, ok := pinned[block.fence]
		if !ok {
			continue
		}
		widened, ok := wider[block.fence]
		if !ok || len(widened.steps) != len(narrow.steps) {
			t.Errorf("%s:%d: the block replayed differently at the two window widths", quickStartPath, block.fence)
			continue
		}
		for i, step := range narrow.steps {
			if normalise(step.text) == normalise(widened.steps[i].text) {
				continue
			}
			t.Errorf("%s:%d: `%s` renders differently at %s columns and at %s, so the pinned window stacks a table or wraps a cell in this block",
				quickStartPath, block.fence, step.command, replayColumns, wideningColumns)
		}
	}
}

// compareQuickStart holds every replayed block against what the head printed,
// naming the document, the line the block opens at, the command, and the first
// line that differs.
func compareQuickStart(t *testing.T, blocks []quickBlock, captured map[int]quickCapture) {
	t.Helper()
	for _, block := range blocks {
		capture, ok := captured[block.fence]
		if !ok {
			continue
		}
		steps := block.steps()
		if len(steps) != len(capture.steps) {
			t.Errorf("%s:%d: the block shows %d commands and the replay drove %d", quickStartPath, block.fence, len(steps), len(capture.steps))
			continue
		}
		for i, step := range steps {
			got := capture.steps[i]
			if got.code != step.exit {
				t.Errorf("%s:%d: `%s`\n  wanted exit %d, got %d", quickStartPath, block.fence, step.command, step.exit, got.code)
			}
			wanted := streamLines(normalise(strings.Join(step.output, "\n")))
			shown := streamLines(normalise(got.text))
			if difference := firstDisagreement(wanted, shown); difference != "" {
				t.Errorf("%s:%d: `%s`\n%s", quickStartPath, block.fence, step.command, difference)
			}
		}
	}
}

// rewriteQuickStart writes the replay's own output back into the document,
// with the document's readable values restored at every normalised occurrence.
// A block whose captured output gained an occurrence of a class is left alone
// and named, since only a person knows what the narrative's version of a new
// path should read.
func rewriteQuickStart(t *testing.T, lines []string, blocks []quickBlock, captured map[int]quickCapture) {
	t.Helper()
	bodies := map[int][]string{}
	for _, block := range blocks {
		capture, ok := captured[block.fence]
		if !ok {
			continue
		}
		body, err := regeneratedBody(block, capture)
		if err != nil {
			t.Errorf("%s:%d: %v", quickStartPath, block.fence, err)
			continue
		}
		bodies[block.fence] = body
	}
	var rewritten []string
	at := 0
	for _, block := range blocks {
		body, ok := bodies[block.fence]
		if !ok {
			continue
		}
		closer := block.bodyAt + len(block.body) - 1
		rewritten = append(rewritten, lines[at:block.fence]...)
		rewritten = append(rewritten, body...)
		at = closer
	}
	rewritten = append(rewritten, lines[at:]...)
	if err := os.WriteFile(quickStartPath, []byte(strings.Join(rewritten, "\n")), 0o644); err != nil {
		t.Fatalf("write %s: %v", quickStartPath, err)
	}
}

// regeneratedBody composes one block's new body from the replay, with the
// document's own values restored.
func regeneratedBody(block quickBlock, capture quickCapture) ([]string, error) {
	steps := block.steps()
	if len(steps) != len(capture.steps) {
		return nil, fmt.Errorf("the block shows %d commands and the replay drove %d, so there is nothing to write back", len(steps), len(capture.steps))
	}
	var documented []string
	for _, step := range steps {
		documented = append(documented, step.output...)
	}
	var produced []string
	counts := make([]int, len(capture.steps))
	for i, got := range capture.steps {
		shown := streamLines(got.text)
		counts[i] = len(shown)
		produced = append(produced, shown...)
	}
	restored, err := restoreDocumentValues(strings.Join(produced, "\n"), valuesByClass(strings.Join(documented, "\n")))
	if err != nil {
		return nil, err
	}
	back := redrawSeparatorRows(strings.Split(restored, "\n"))
	var body []string
	read := 0
	for i, step := range steps {
		body = append(body, "$ "+step.command)
		body = append(body, back[read:read+counts[i]]...)
		read += counts[i]
		if step.hasExit || capture.steps[i].code != 0 {
			body = append(body, "[exit "+strconv.Itoa(capture.steps[i].code)+"]")
		}
	}
	return body, nil
}

// redrawSeparatorRows redraws every table separator row of a regenerated block
// against the values that will stand around it in the document.
//
// A separator row is not one run of glyphs but several, and each run tracks the
// width of the column above it. Those widths were measured from the values the
// replay captured, and the restoration above puts the document's own shorter
// values back, so a sandbox path forty columns longer than the narrative's
// would otherwise leave a rule running forty columns past the value under it on
// a page a customer reads. The comparison never sees this, since it
// canonicalises a rule field whatever its length, which is exactly why the
// redraw belongs here rather than being left for a failing run to report.
//
// A row is a separator when every field it draws is a run of the rule glyph. Its
// runs are then redrawn to the widest field each column carries among the rows
// that share its indent and its field count, which is the width the head itself
// measures at a window wide enough to hold the table.
func redrawSeparatorRows(lines []string) []string {
	redrawn := append([]string(nil), lines...)
	for i, line := range lines {
		fields := columnarFields(line)
		if len(fields) < 2 || !everyFieldIsARule(fields) {
			continue
		}
		widths := make([]int, len(fields))
		for j := i - 1; j >= 0 && widensTheColumns(lines[j], fields, widths); j-- {
		}
		for j := i + 1; j < len(lines) && widensTheColumns(lines[j], fields, widths); j++ {
		}
		redrawn[i] = separatorRow(fields[0].at, widths)
	}
	return redrawn
}

// everyFieldIsARule reports whether a line draws nothing but rule runs, which
// is what makes it a separator row rather than a row of values.
func everyFieldIsARule(fields []columnarField) bool {
	for _, field := range fields {
		if !ruleGlyphRun.MatchString(field.text) {
			return false
		}
	}
	return true
}

// widensTheColumns folds one neighbouring row into the widths a separator row
// will draw, and reports whether the walk should carry on past it. A row of a
// different shape, or another separator row, ends the block.
func widensTheColumns(line string, separator []columnarField, widths []int) bool {
	fields := columnarFields(line)
	if len(fields) != len(separator) || fields[0].at != separator[0].at || everyFieldIsARule(fields) {
		return false
	}
	for c, field := range fields {
		if drawn := displayWidth(field.text); drawn > widths[c] {
			widths[c] = drawn
		}
	}
	return true
}

// separatorRow draws one separator row: the block's indent, then a rule run per
// column with the table's own gutter between them.
func separatorRow(indent int, widths []int) string {
	var built strings.Builder
	built.WriteString(strings.Repeat(" ", indent))
	for c, width := range widths {
		if c > 0 {
			built.WriteString(strings.Repeat(" ", tableGutter))
		}
		built.WriteString(rule(width))
	}
	return built.String()
}
