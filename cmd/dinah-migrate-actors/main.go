// Command dinah-migrate-actors carries one workbench's journals from storage
// format 4 to 5, where a journal line's actor is an object rather than a
// string.
//
// It is a program of its own rather than a flag on dinah check, on the
// operator's ruling of 2026-09-15 that a migration must be strippable before
// release and that a format which will never be in the wild is served by a
// one-shot program. Format 5 never leaves this machine, because no build has
// shipped, so a flag would buy a row in the command table, a row in the help
// fixture, a request member, an MCP parameter with its description and a
// catalogue string in eight locales, and all of it would have to be retired
// again. Retiring a directory under cmd is deleting a directory.
//
// It is Go rather than a shell script, because it reads a journal line,
// replaces one member and writes the line back, against live data. It gets the
// bench package and the type checker, and a text script doing the same work is
// the version that goes wrong quietly.
//
// It reports in plain English written directly rather than through the message
// catalogue, since nobody outside this project runs it.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dinah/internal/bench"
)

// The exit codes this program answers with. The numbering starts at 3 because
// 1 and 2 are the Go runtime's own: a log.Fatal exits 1 and a panic exits 2, so
// a program numbering its outcomes from 1 would give a crash the same code as
// its two most useful answers, and a shell loop watching sixteen customer
// stores could not tell a store that needs the run from a program that fell
// over on it. Zero stays success, which every shell already assumes.
const (
	// exitDone is a run that finished: nothing needed migrating, or --apply
	// rewrote what it found.
	exitDone = 0
	// exitClassified is a run without --apply that found string actors and
	// wrote nothing. It is a code of its own rather than 0 so that a loop can
	// tell a store that still needs the run from one already across.
	exitClassified = 3
	// exitConflicts is conflicts found and nothing written.
	exitConflicts = 4
	// exitUnusable is a path naming no workbench, a path naming a container
	// holding several, or a store that could not be read.
	exitUnusable = 5
)

// The four conflict tokens. Each is reported as a token rather than a sentence
// so that a shell loop and a person read the same word.
const (
	// tornJournal is a journal whose final line will not parse, which the
	// reader tolerates on a read and which a rewrite must not silently drop.
	tornJournal = "torn-journal"
	// unreadable is a journal that will not open.
	unreadable = "unreadable"
	// actorNotString is a line whose actor is neither a string nor an object
	// carrying name, which is damage rather than an old shape.
	actorNotString = "actor-not-string"
	// spanMismatch is a line whose computed actor span does not hold the bytes
	// the decoder returned for that value.
	spanMismatch = "span-mismatch"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main's body with its streams and its arguments handed in, so the
// tests drive the whole program rather than its parts.
func run(args []string, out, errw *os.File) int {
	path, apply, usage := parseArgs(args)
	if usage != "" {
		fmt.Fprintln(errw, usage)
		fmt.Fprintln(errw, "usage: dinah-migrate-actors <path to one workbench, or to a container holding one> [--apply]")
		return exitUnusable
	}
	root, refusal := resolveWorkbench(path)
	if refusal != "" {
		fmt.Fprintln(errw, refusal)
		return exitUnusable
	}
	journals, err := journalsUnder(root)
	if err != nil {
		fmt.Fprintf(errw, "refused: %s could not be read: %v\n", root, err)
		return exitUnusable
	}
	report, err := classify(journals)
	if err != nil {
		fmt.Fprintf(errw, "refused: %s could not be read: %v\n", root, err)
		return exitUnusable
	}
	printClassification(out, journals, report)
	if len(report.conflicts) > 0 {
		fmt.Fprintf(out, "%d conflicts:\n", len(report.conflicts))
		for _, conflict := range report.conflicts {
			fmt.Fprintf(out, "  %s  %s\n", conflict.journal, conflict.token)
		}
		fmt.Fprintln(out, "Nothing was written. Repair these and run it again.")
		return exitConflicts
	}
	if !apply {
		if report.strings == 0 {
			fmt.Fprintln(out, "Nothing needed migrating.")
			return exitDone
		}
		fmt.Fprintln(out, "Nothing was written. Run again with --apply to rewrite them and stamp format 5.")
		return exitClassified
	}
	rewrote, err := rewrite(journals)
	if err != nil {
		fmt.Fprintf(errw, "refused: the rewrite stopped: %v\n", err)
		return exitUnusable
	}
	// The format stamp is written last, so a run that dies part-way leaves a
	// store whose stamp still says 4 and whose remaining string actors the
	// next run finds.
	if err := stampFormat(root); err != nil {
		fmt.Fprintf(errw, "refused: the format stamp could not be written: %v\n", err)
		return exitUnusable
	}
	fmt.Fprintf(out, "Rewrote %d journals, %d lines re-encoded, %d lines carried through unchanged. Stamped format %d on the workbench anchor.\n",
		rewrote, report.strings, report.objects, bench.ActorObjectFormat)
	return exitDone
}

// parseArgs reads the one positional and the one marker, and answers the
// complaint where the arguments are not those.
func parseArgs(args []string) (path string, apply bool, usage string) {
	for _, arg := range args {
		switch {
		case arg == "--apply":
			apply = true
		case strings.HasPrefix(arg, "-"):
			return "", false, "refused: " + arg + " is not an argument this program takes."
		case path == "":
			path = arg
		default:
			return "", false, "refused: this program takes one path, and it was given two."
		}
	}
	if path == "" {
		return "", false, "refused: this program takes the path of one workbench."
	}
	return path, apply, ""
}

// resolveWorkbench turns the path the caller named into the one workbench
// directory the run operates on, and answers the refusal where it names none.
//
// There are two ways to name one workbench. A workbench directory, which is the
// directory holding a workbench.md, names itself. A container holding exactly
// one workbench names that workbench, because there is no other one it could
// mean. A container holding several is refused, and the refusal lists the
// workbench directories inside it so the operator can see the set and run the
// program against each.
//
// Accepting the single-workbench container is not a softening of the one
// workbench per run rule. Neither problem that rule protects against can arise
// where there is one store: nothing is stopped on behalf of a healthy sibling,
// and no run can die having stamped some of a set.
func resolveWorkbench(path string) (root, refusal string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "refused: " + path + " is not a path this program can resolve."
	}
	if bench.Exists(filepath.Join(abs, bench.WorkbenchAnchor)) {
		return abs, ""
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", "refused: " + abs + " names no workbench."
	}
	var held []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(abs, entry.Name())
		if bench.Exists(filepath.Join(candidate, bench.WorkbenchAnchor)) {
			held = append(held, candidate)
		}
	}
	sort.Strings(held)
	switch len(held) {
	case 0:
		return "", "refused: " + abs + " names no workbench."
	case 1:
		return held[0], ""
	}
	listed := make([]string, 0, len(held)+1)
	listed = append(listed, fmt.Sprintf("refused: that container holds %d workbenches. Name one of these:", len(held)))
	for _, candidate := range held {
		listed = append(listed, "  "+candidate)
	}
	return "", strings.Join(listed, "\n")
}

// journalGroup is one of the five populations a store's journals fall into,
// named so the classification report can count each separately.
type journalGroup struct {
	// label is the population's name as the report prints it.
	label string
	// paths are the journals of that population, in the order the walk found
	// them.
	paths []string
}

// journalsUnder finds every journal in the store, grouped by the population it
// belongs to.
//
// Journals exist in five places and this reaches all five: a card's journal, a
// workstream's journal, the workbench's own journal, an archived card's journal
// and an archived workstream's journal. An archived column carries none, and a
// checklist item carries none either, because an item's events append to the
// enclosing card's journal.
func journalsUnder(root string) ([]journalGroup, error) {
	groups := []journalGroup{
		{label: "card journals"},
		{label: "workstream journals"},
		{label: "workbench journals"},
		{label: "card journals under the archive"},
		{label: "workstream journals under the archive"},
	}
	collections := []struct {
		at  int
		dir string
	}{
		{0, filepath.Join(root, bench.CardsDir)},
		{1, filepath.Join(root, bench.WorkstreamsDir)},
		{3, filepath.Join(root, bench.ArchiveDir, bench.CardsDir)},
		{4, filepath.Join(root, bench.ArchiveDir, bench.WorkstreamsDir)},
	}
	for _, collection := range collections {
		if !bench.Exists(collection.dir) {
			continue
		}
		ids, err := bench.ListIDs(collection.dir)
		if err != nil {
			return nil, err
		}
		sort.Strings(ids)
		for _, id := range ids {
			path := filepath.Join(collection.dir, id, bench.JournalName)
			if bench.Exists(path) {
				groups[collection.at].paths = append(groups[collection.at].paths, path)
			}
		}
	}
	if own := filepath.Join(root, bench.JournalName); bench.Exists(own) {
		groups[2].paths = append(groups[2].paths, own)
	}
	return groups, nil
}

// conflict is one journal the run refuses, named with the condition that
// refused it.
type conflict struct {
	journal string
	token   string
}

// classification is what the reading pass found, counted so that a pass which
// read nothing reports something a reader can tell from a pass that found
// nothing to do.
type classification struct {
	// strings counts the lines whose actor is a string, which a run would
	// re-encode.
	strings int
	// objects counts the lines whose actor is already an object, which a run
	// would leave.
	objects int
	// conflicts are the journals the run refuses, in the order they were met.
	conflicts []conflict
}

// classify reads every journal in the store without writing anything, counting
// the three populations and collecting every conflict rather than the first.
func classify(groups []journalGroup) (classification, error) {
	var report classification
	for _, group := range groups {
		for _, path := range group.paths {
			text, err := os.ReadFile(path)
			if err != nil {
				report.conflicts = append(report.conflicts, conflict{journal: path, token: unreadable})
				continue
			}
			lines, torn := splitJournal(string(text))
			if torn {
				report.conflicts = append(report.conflicts, conflict{journal: path, token: tornJournal})
				continue
			}
			refused := false
			for _, line := range lines {
				span, err := actorSpan(line)
				switch {
				case err != nil:
					report.conflicts = append(report.conflicts, conflict{journal: path, token: err.token})
					refused = true
				case span.object:
					report.objects++
				default:
					report.strings++
				}
				if refused {
					break
				}
			}
		}
	}
	return report, nil
}

// printClassification writes the three populations and the size of the sweep,
// because a pass that read nothing reports success and reads exactly like a
// pass that found nothing to do.
func printClassification(out *os.File, groups []journalGroup, report classification) {
	total := 0
	var parts []string
	for _, group := range groups {
		if len(group.paths) == 0 {
			continue
		}
		total += len(group.paths)
		parts = append(parts, fmt.Sprintf("%d %s", len(group.paths), group.label))
	}
	if len(parts) == 0 {
		fmt.Fprintln(out, "Read 0 journals.")
	} else {
		fmt.Fprintf(out, "Read %d journals: %s.\n", total, strings.Join(parts, ", "))
	}
	fmt.Fprintf(out, "  %d lines carry a string actor.\n", report.strings)
	fmt.Fprintf(out, "  %d lines already carry an object.\n", report.objects)
	fmt.Fprintf(out, "  %d journals could not be read.\n", unreadableCount(report))
}

// unreadableCount is how many journals the pass could not read at all, which is
// reported separately from the other conflicts because it is the count the
// classification promises.
func unreadableCount(report classification) int {
	count := 0
	for _, found := range report.conflicts {
		if found.token == unreadable {
			count++
		}
	}
	return count
}

// splitJournal cuts a journal into its lines and reports whether the final one
// is torn. A blank line is dropped, exactly as the reader drops one.
//
// Torn means what ReadJournal means by it: the last line does not parse. Every
// earlier line that does not parse is not tornness at all, and actorSpan is
// where such a line is refused.
func splitJournal(text string) ([]string, bool) {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return nil, false
	}
	last := lines[len(lines)-1]
	if !json.Valid([]byte(last)) {
		return lines[:len(lines)-1], true
	}
	return lines, false
}

// span is where one line's actor value sits and what shape it carries.
type span struct {
	// start and end bound the value's own bytes inside the line.
	start, end int
	// raw is the value's own literal bytes, as the decoder returned them.
	raw []byte
	// object says the value is already an object, so the line is left alone.
	object bool
}

// spanError is a line the program refuses, carrying the token that refuses it.
type spanError struct {
	token string
}

// Error names the condition, so a spanError satisfies error and can travel as
// one where a caller wants it to.
func (e *spanError) Error() string { return e.token }

// actorSpan finds the byte span of one line's actor value.
//
// The program never decodes a line into bench.Event. AppendEvent marshals that
// struct, so a line round-tripped through it would lose every member the struct
// does not declare and re-encode the rest in struct field order under Go's
// default escaping. The promise that every other member survives byte for byte
// needs a mechanism, and this is it.
//
// A decoder walks the line's top-level object until Token returns the key
// actor. The value is then read with Decode into a json.RawMessage, which holds
// the value's own literal bytes. InputOffset taken after that read is the end of
// the value, and the start of the value is that offset minus the length of the
// raw message.
//
// The subtraction is right whatever the line's spacing. Decode skips leading
// whitespace before the value and the raw message holds the value's own bytes,
// so a line carrying a space before or after the colon lands on the same first
// byte as one carrying none, and it is rewritten rather than refused. Nothing
// here requires the lines to be tightly packed and nothing refuses a line that
// is not.
//
// Taking the offset before the read instead would be wrong, and wrong in a way
// that produces unparseable JSON rather than a visible failure. Go documents
// InputOffset as the end of the most recently returned token, and the colon
// between a key and its value is not a token Token returns, so an offset taken
// after reading the key sits before the colon and the span would run from the
// colon to the end of the value.
func actorSpan(line string) (span, *spanError) {
	decoder := json.NewDecoder(strings.NewReader(line))
	opening, err := decoder.Token()
	if err != nil {
		return span{}, &spanError{token: actorNotString}
	}
	if delim, ok := opening.(json.Delim); !ok || delim != '{' {
		return span{}, &spanError{token: actorNotString}
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return span{}, &spanError{token: actorNotString}
		}
		name, ok := key.(string)
		if !ok {
			return span{}, &spanError{token: actorNotString}
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return span{}, &spanError{token: actorNotString}
		}
		if name != "actor" {
			continue
		}
		end := int(decoder.InputOffset())
		start := end - len(raw)
		if start < 0 || end > len(line) || line[start:end] != string(raw) {
			// A defensive assertion on the standard library's contract and
			// nothing more. The span is derived from the raw message's own
			// length, so the two can differ only if Decode and InputOffset
			// disagree about where the value ended, which is a bug in Go or in
			// this program rather than anything a journal can carry. No line a
			// decoder can read reaches it, whatever whitespace or escaping that
			// line carries, and a line a decoder cannot read has already been
			// counted under torn-journal. It is worth the few lines anyway: the
			// thing it asserts is the one assumption standing between this
			// program and live customer stores.
			return span{}, &spanError{token: spanMismatch}
		}
		switch {
		case len(raw) > 0 && raw[0] == '"':
			return span{start: start, end: end, raw: raw}, nil
		case len(raw) > 0 && raw[0] == '{':
			var carried struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(raw, &carried); err != nil || carried.Name == "" {
				return span{}, &spanError{token: actorNotString}
			}
			return span{start: start, end: end, raw: raw, object: true}, nil
		}
		return span{}, &spanError{token: actorNotString}
	}
	return span{}, &spanError{token: actorNotString}
}

// rewriteLine replaces one line's string actor with the object that carries it
// as the name, and leaves every other byte of the line exactly as it stood:
// key order, spacing, escaping, and any member this build does not declare.
func rewriteLine(line string, at span) string {
	if at.object {
		return line
	}
	return line[:at.start] + `{"name":` + string(at.raw) + "}" + line[at.end:]
}

// rewrite runs the write pass, whole journal by whole journal and in the order
// the lines stood. It runs only after the classification pass returned clean, so
// a partial rewrite of a journal is the one outcome this program is shaped to
// make impossible.
func rewrite(groups []journalGroup) (int, error) {
	rewrote := 0
	for _, group := range groups {
		for _, path := range group.paths {
			text, err := os.ReadFile(path)
			if err != nil {
				return rewrote, err
			}
			lines, _ := splitJournal(string(text))
			rebuilt := make([]string, 0, len(lines))
			for _, line := range lines {
				at, refused := actorSpan(line)
				if refused != nil {
					return rewrote, refused
				}
				rebuilt = append(rebuilt, rewriteLine(line, at))
			}
			body := ""
			if len(rebuilt) > 0 {
				body = strings.Join(rebuilt, "\n") + "\n"
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return rewrote, err
			}
			rewrote++
		}
	}
	return rewrote, nil
}

// stampFormat writes the new storage format onto the workbench anchor, keeping
// every other key and the standing text where they stand.
func stampFormat(root string) error {
	path := filepath.Join(root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		return err
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set("format", fmt.Sprintf("%d", bench.ActorObjectFormat))
	return bench.WriteText(path, fm.Render(body))
}
