// Command dinah-migrate-notes carries one workbench from storage format 5 to
// 6, where a checklist item's answer is a designated comment rather than a
// free-text note.
//
// It is a program of its own rather than a flag on dinah check, on the
// operator's ruling of 2026-09-15 that a migration must be strippable before
// release: a flag would buy a row in the command table, a row in the help
// fixture, a request member, an MCP parameter with its description and a
// catalogue string in eight locales, and all of it would have to be retired
// again. Retiring a directory under cmd is deleting a directory. The refusal a
// reader meets when it opens an unmigrated store is outside that rule and is
// named like any other, because a refusal a user meets without a name is an
// unnamed error string, which this tool does not have.
//
// It reports in plain English written directly rather than through the message
// catalogue, since nobody outside this project runs it.
//
// # What it carries
//
// Every item carrying a note is migrated, whatever state it is in, and the
// note's text becomes a comment of that item. A settled item gains a
// resolution aimed at the new comment. A pending item carrying a note gains
// the comment and no resolution, because reopen never touched the note key, so
// a live store can hold a pending item carrying the note of a settling that
// was undone. Dropping that text would be a silent loss, and aiming a
// resolution at it would assert an answer the item does not have.
//
// # It writes in place, and it claims no atomicity
//
// This is what every other migration in this repository does, and
// MigrateBranches says why: a failure part-way through the write pass is the
// one case the format cannot make atomic, because the format holds no
// transaction across files, and inventing one is not a repair's work. A copy
// aside and a swap would be worse here rather than better, since Windows
// offers no single-step directory swap and a machine dying between the two
// renames leaves the store missing from the path everything looks for it in.
//
// What stands in atomicity's place is what those migrations already rely on. A
// preview classifies exactly as the run does and writes nothing. The run is
// idempotent, which rests on the converted comment's identity rather than on
// the note key: converting one item is two writes, and a run killed between
// them leaves an item holding both its old note and its new comment, which an
// idempotence resting on the note key alone would read as unmigrated and
// convert a second time. And a run that stops part-way names every item it had
// written when it stopped, so the state of the store is readable rather than
// inferred.
//
// The operator's backup is the safety, and this program does not pretend to be
// one. None of the live stores is under version control and several carry
// customer work.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The exit codes this program answers with, on dinah-migrate-actors' own
// numbering and for its reason: 1 and 2 are the Go runtime's, so a program
// numbering its outcomes from 1 would give a crash the same code as its most
// useful answers, and a shell loop over nineteen stores could not tell a store
// that needs the run from a program that fell over on it.
const (
	// exitDone is a run that finished: nothing needed migrating, or --apply
	// converted what it found.
	exitDone = 0
	// exitClassified is a run without --apply that found notes and wrote
	// nothing. It is a code of its own rather than 0 so that a loop can tell
	// a store that still needs the run from one already across.
	exitClassified = 3
	// exitUnattributed is a run that found an item whose settling the journal
	// cannot attribute and wrote nothing. The operator then chooses, per
	// store: fix the journal, or rerun with --unattributed.
	exitUnattributed = 4
	// exitUnusable is a path naming no workbench, a path naming a container
	// holding several, or a store that could not be read.
	exitUnusable = 5
	// exitStopped is a write pass that stopped part-way. The report names
	// every item that had landed, and a rerun finishes the store.
	exitStopped = 6
)

// conversionTag is the fixed word the converted comment's identifier is
// derived from, beside the item's own identifier.
//
// Deriving the identifier rather than minting one is what makes a rerun unable
// to write a second copy. A rerun computes the same identifier, finds the file
// already there, and writes nothing in its place, so two identical comments are
// not merely unlikely, they are unconstructible: there is only one path a
// converted comment can occupy.
//
// The tag is in the derivation rather than left out of it so that the
// identifier of a comment this migration made cannot collide with the
// identifier of one some later migration derives from the same item.
const conversionTag = "dinah-525/note-to-comment"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main's body with its streams and its arguments handed in, so the
// tests drive the whole program rather than its parts.
func run(args []string, out, errw *os.File) int {
	path, apply, unattributed, usage := parseArgs(args)
	if usage != "" {
		fmt.Fprintln(errw, usage)
		fmt.Fprintln(errw, "usage: dinah-migrate-notes <path to one workbench, or to a container holding one> [--apply] [--unattributed]")
		return exitUnusable
	}
	root, refusal := resolveWorkbench(path)
	if refusal != "" {
		fmt.Fprintln(errw, refusal)
		return exitUnusable
	}
	opened, err := bench.OpenAwaitingResolution(root)
	if err != nil {
		fmt.Fprintf(errw, "refused: %s could not be read: %v\n", root, err)
		return exitUnusable
	}
	// The one migration ahead of this one that this one cannot be run
	// without.
	//
	// Stamping the current format says every migration below it has run, and
	// for most of them that claim is cosmetic: a branch heading left in a
	// body is a finding check reports, and a journal carrying string actors
	// is read by a tolerant unmarshaller. The card-number registry is not
	// cosmetic. A store below the format it arrived at keeps its numbers in
	// card frontmatter, and Add refuses to file there precisely because the
	// registry is the thing it would allocate from; stamping such a store to
	// this format would silence that refusal and let the next filing hand out
	// a number a card already answers to.
	//
	// So the run stops and names the repair, rather than carrying a store
	// somewhere it cannot come back from.
	if opened.Format < bench.RegistryFormat {
		fmt.Fprintf(errw, "refused: %s declares format %d, and the card-number registry arrived at format %d.\n",
			root, opened.Format, bench.RegistryFormat)
		fmt.Fprintln(errw, "Run `dinah check --migrate-numbers --yes` against it first, then run this again.")
		return exitUnusable
	}
	planned, err := classify(opened)
	if err != nil {
		fmt.Fprintf(errw, "refused: %s could not be read: %v\n", root, err)
		return exitUnusable
	}
	printClassification(out, planned)
	unrecoverable := unattributable(planned)
	// The refusal is the whole of what "the script refuses rather than
	// inventing an actor" means. It reads every item it would migrate, writes
	// the manifest of those it cannot attribute, and stops without writing
	// anything, so the operator chooses per store rather than finding a
	// choice already made for them.
	if len(unrecoverable) > 0 && !unattributed {
		fmt.Fprintf(out, "%d items the journal cannot attribute:\n", len(unrecoverable))
		for _, item := range unrecoverable {
			fmt.Fprintf(out, "  %s\n", item.ref)
		}
		fmt.Fprintln(out, "Nothing was written. Fix the journal, or run again with --unattributed to record those comments with no author at all.")
		return exitUnattributed
	}
	if !apply {
		if len(planned) == 0 {
			fmt.Fprintln(out, "Nothing needed migrating.")
			return exitDone
		}
		fmt.Fprintf(out, "Nothing was written. Run again with --apply to convert them and stamp format %d.\n", bench.ResolutionFormat)
		return exitClassified
	}
	landed, err := convert(opened, planned)
	// The report names what landed whether the pass finished or stopped, so
	// the state of the store is readable rather than inferred.
	printLanded(out, landed)
	if err != nil {
		fmt.Fprintf(errw, "refused: the conversion stopped: %v\n", err)
		fmt.Fprintln(errw, "The store is part-converted. Run the program again: it finishes from where it stopped.")
		return exitStopped
	}
	// The format stamp is written last, so a run that dies part-way leaves a
	// store whose stamp still says 5 and whose remaining notes the next run
	// finds.
	if err := stampFormat(opened); err != nil {
		fmt.Fprintf(errw, "refused: the format stamp could not be written: %v\n", err)
		return exitStopped
	}
	fmt.Fprintf(out, "Converted %d items. Stamped format %d on the workbench anchor.\n", len(landed), bench.ResolutionFormat)
	return exitDone
}

// parseArgs reads the one positional and the two markers, and answers the
// complaint where the arguments are not those.
func parseArgs(args []string) (path string, apply, unattributed bool, usage string) {
	for _, arg := range args {
		switch {
		case arg == "--apply":
			apply = true
		case arg == "--unattributed":
			unattributed = true
		case strings.HasPrefix(arg, "-"):
			return "", false, false, "refused: " + arg + " is not an argument this program takes."
		case path == "":
			path = arg
		default:
			return "", false, false, "refused: this program takes one path, and it was given two."
		}
	}
	if path == "" {
		return "", false, false, "refused: this program takes the path of one workbench."
	}
	return path, apply, unattributed, ""
}

// resolveWorkbench turns the path the caller named into the one workbench
// directory the run operates on, and answers the refusal where it names none.
// It is dinah-migrate-actors' own resolution, and it accepts the same two
// spellings for the same reason: a workbench directory names itself, and a
// container holding exactly one workbench names that workbench, because there
// is no other one it could mean.
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
	return "", "refused: " + abs + " holds " + strconv.Itoa(len(held)) + " workbenches, and this program takes one:\n  " + strings.Join(held, "\n  ")
}

// conversion is one item the run would convert: where it lives, what its note
// says, who the journal says settled it and when, and whether it is settled at
// all.
type conversion struct {
	// card is the card the item hangs below, held so the write can compose
	// the designation's canonical reference and take the card's own lock.
	card *bench.Card
	// item is the item as it stands on disk before the conversion.
	item *bench.Item
	// ref is the item's canonical reference, which is what the manifest of
	// unattributable items names and what a report row reads.
	ref string
	// note is the text the item's retired note key carries, which becomes
	// the converted comment's body.
	note string
	// author is who the journal records as having settled the item, empty
	// where the journal records no settling at all.
	author string
	// ts is when that settling happened, or the item's own timestamp where
	// the journal records no settling. The spec is explicit that an
	// unattributable comment takes its timestamp from the item's own anchor,
	// because that is a fact the store holds rather than one this program
	// would be choosing.
	ts string
	// settled is whether the item stands at a terminal state now. A pending
	// item carrying a note gains the comment and no resolution, because its
	// settling was undone and a resolution would assert an answer the item
	// does not have.
	settled bool
}

// classify reads every item the run would convert and answers the plan, in a
// fixed order so two runs over one store report the same list.
//
// It writes nothing. The preview and the write pass classify through this one
// function rather than through two, which is what makes "a preview classifies
// exactly as the migration does" a property of the code instead of a promise.
func classify(opened *bench.Bench) ([]conversion, error) {
	var planned []conversion
	for _, root := range []string{opened.CardsRoot(), opened.ArchivedCardsRoot()} {
		ids, err := bench.ListIDs(root)
		if err != nil {
			continue
		}
		sort.Strings(ids)
		for _, id := range ids {
			card, err := opened.LoadCardIn(root, id)
			if err != nil {
				return nil, err
			}
			found, err := classifyCard(opened, card)
			if err != nil {
				return nil, err
			}
			planned = append(planned, found...)
		}
	}
	return planned, nil
}

// classifyCard reads one card's checklist and answers the items carrying a
// note, each with what the card's journal says about its settling.
//
// The journal is read once per card rather than once per item, because a card
// carrying thirty items would otherwise read one file thirty times to answer
// thirty questions the one read already holds.
func classifyCard(opened *bench.Bench, card *bench.Card) ([]conversion, error) {
	items, err := bench.Items(card.Dir)
	if err != nil {
		return nil, nil
	}
	settlings := settlingsIn(card)
	cardRef := card.Ref(opened.Slug)
	kindPosition := map[string]int{}
	var planned []conversion
	for _, item := range items {
		kindPosition[item.Kind]++
		fm, _, err := bench.ReadItemAnchor(item.Dir)
		if err != nil {
			return nil, err
		}
		note := fm.Value(bench.ItemNoteRetiredField)
		if strings.TrimSpace(note) == "" {
			continue
		}
		position, err := bench.MemberPosition(item.Dir, bench.ItemAnchor)
		if err != nil {
			return nil, err
		}
		settling := settlings[item.ID]
		entry := conversion{
			card:    card,
			item:    item,
			ref:     itemRef(cardRef, item.Kind, kindPosition[item.Kind], position),
			note:    note,
			author:  settling.actor,
			ts:      settling.ts,
			settled: item.State != bench.ItemPending && item.State != "",
		}
		if entry.ts == "" {
			entry.ts = fm.Value("ts")
		}
		planned = append(planned, entry)
	}
	return planned, nil
}

// settling is what a card's journal says about one item's last settling.
type settling struct {
	actor string
	ts    string
}

// settlingsIn reads a card's journal and answers, per item, the last settling
// event it records.
//
// The last rather than the first, because an item settled, reopened and
// settled again carries the words of the settling that stands, and an item
// settled and then reopened carries the words of the one settling there was.
// Both cases read the same way: the most recent terminal event for that item is
// the one whose actor wrote the note on disk.
//
// A journal that will not open, or whose final line is torn, yields whatever it
// could read rather than nothing. A torn line is one settling this run cannot
// attribute, and the refusal above names that item; refusing the whole card
// would name every item on it instead of the one the damage reaches.
func settlingsIn(card *bench.Card) map[string]settling {
	found := map[string]settling{}
	events, _, err := bench.ReadJournal(card.JournalPath())
	if err != nil {
		return found
	}
	terminal := map[string]bool{
		contract.EventItemResolved: true,
		contract.EventItemVerified: true,
		contract.EventItemFailed:   true,
	}
	for _, event := range events {
		if event.Item == "" || !terminal[event.Event] {
			continue
		}
		if name := event.Actor.Name; name != "" {
			found[event.Item] = settling{actor: name, ts: event.TS}
		}
	}
	return found
}

// itemRef composes what a person types to reach one checklist item, which is
// the spelling dinah show prints: the card's reference, the item's kind word
// and its position among the items of that kind. An item of a kind the format
// does not declare is named by the checklist collection and its position in
// it, which is what descend resolves without narrowing.
func itemRef(cardRef, kind string, kindPosition, position int) string {
	if word, ok := bench.WordForItemKind(kind); ok {
		return cardRef + "/" + word + "/" + strconv.Itoa(kindPosition)
	}
	return cardRef + "/" + bench.ChecklistDir + "/" + strconv.Itoa(position)
}

// unattributable answers the planned conversions whose settling the journal
// records no actor for.
func unattributable(planned []conversion) []conversion {
	var found []conversion
	for _, entry := range planned {
		if entry.author == "" {
			found = append(found, entry)
		}
	}
	return found
}

// convert performs the write pass, and answers what landed as well as what
// stopped it.
//
// It stops at the first failure rather than carrying on, because a store where
// one write failed is a store the operator has to look at, and converting the
// rest first would bury the one item that needs reading under thirty that do
// not.
func convert(opened *bench.Bench, planned []conversion) ([]string, error) {
	var landed []string
	for _, entry := range planned {
		if err := convertOne(opened, entry); err != nil {
			return landed, fmt.Errorf("%s: %w", entry.ref, err)
		}
		landed = append(landed, entry.ref)
	}
	return landed, nil
}

// convertOne carries one item's note into a comment and finishes the item.
//
// The order is fixed and it is the comment first. The other order would leave
// a resolution aimed at a comment that does not exist, which is a worse half
// state than a note that has outlived its conversion.
//
// Three states an item can be found in, and each is treated by what is on disk:
// no comment and a note, convert both; a comment and a note, write no comment
// and finish the item; no note, already done, leave it. The third case is what
// makes a rerun cheap and the first two are what make it correct.
func convertOne(opened *bench.Bench, entry conversion) error {
	dir := convertedCommentDir(entry.item)
	// The check is for the anchor FILE rather than for the directory. A run
	// killed mid-write can leave the directory there and empty, and a check on
	// the directory would then read the item as converted and skip writing the
	// comment it never wrote.
	if !bench.Exists(filepath.Join(dir, bench.CommentAnchor)) {
		if err := writeConvertedComment(entry, dir); err != nil {
			return err
		}
	}
	return finishItem(opened, entry, dir)
}

// convertedCommentDir is where one item's converted comment lives: a directory
// under the item's own comments collection, named by the identifier derived
// from the item.
func convertedCommentDir(item *bench.Item) string {
	return filepath.Join(item.Dir, bench.CommentsDir, convertedCommentID(item.ID))
}

// convertedCommentID derives the converted comment's identifier from the
// item's own identifier and the fixed tag: the first twelve hex digits of the
// SHA-256 of the two.
func convertedCommentID(itemID string) string {
	sum := sha256.Sum256([]byte(itemID + "\x00" + conversionTag))
	return hex.EncodeToString(sum[:])[:12]
}

// writeConvertedComment writes the comment one item's note becomes.
//
// The ordinal is stamped here and nowhere else, which is to say on the branch
// that actually writes a comment. Stamping it on the branch that finds one
// already there would hand a second comment the same position, which dinah
// check reports as a duplicate.
//
// An unrecoverable author is recorded as absent rather than as a name. An
// author is plain text with nothing reserved, so writing the word unknown
// would make a comment whose author could not be recovered indistinguishable
// from a comment by somebody called unknown. Absence is already
// distinguishable from every name, including from names nobody has thought of,
// which is what a reserved word can never promise, and author_unrecoverable
// says that the absence is a finding rather than an omission.
func writeConvertedComment(entry conversion, dir string) error {
	ordinal, err := bench.NextOrdinalIn(filepath.Join(entry.item.Dir, bench.CommentsDir), bench.CommentAnchor)
	if err != nil {
		return err
	}
	fm := bench.NewFrontmatter()
	fm.Set("ts", entry.ts)
	if entry.author != "" {
		fm.Set("author", entry.author)
	} else {
		fm.Set(bench.CommentAuthorUnrecoverableField, "true")
	}
	fm.Set(bench.OrdinalField, strconv.Itoa(ordinal))
	return bench.WriteCommentAnchor(dir, fm, entry.note)
}

// finishItem drops the retired note key and, on a settled item, records the
// designation.
//
// A converted comment somebody has since edited is left alone. If the derived
// path holds a comment whose body is no longer the note's text, the rerun does
// not overwrite it, which writeConvertedComment's own skip already ensures;
// the item write completes pointing at what is there, because the operator's
// own words outrank the migration's copy of them.
func finishItem(opened *bench.Bench, entry conversion, dir string) error {
	fm, body, err := bench.ReadItemAnchor(entry.item.Dir)
	if err != nil {
		return err
	}
	if fm.Value(bench.ItemNoteRetiredField) == "" && fm.Value(bench.ItemResolutionField) != "" {
		return nil
	}
	fm.Delete(bench.ItemNoteRetiredField)
	if entry.settled {
		ordinal, err := bench.MemberPosition(dir, bench.CommentAnchor)
		if err != nil {
			return err
		}
		fm.Set(bench.ItemResolutionField, entry.ref+"/"+bench.CommentsDir+"/"+strconv.Itoa(ordinal))
	}
	return bench.WriteItemAnchor(entry.item.Dir, fm, body)
}

// stampFormat records the format the store now carries, which is the last
// write of the run.
func stampFormat(opened *bench.Bench) error {
	path := filepath.Join(opened.Root, bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		return err
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set("format", strconv.Itoa(bench.ResolutionFormat))
	return bench.WriteText(path, fm.Render(body))
}

// printClassification prints what the run found, in the order it would convert
// it.
func printClassification(out *os.File, planned []conversion) {
	fmt.Fprintf(out, "Note migration.\n%d items carry a note.\n", len(planned))
	for _, entry := range planned {
		state := "pending, so it gains the comment and no resolution"
		if entry.settled {
			state = "settled, so it gains the comment and a resolution aimed at it"
		}
		author := entry.author
		if author == "" {
			author = "an author the journal cannot recover"
		}
		fmt.Fprintf(out, "  %s  %s  %s\n", entry.ref, state, author)
	}
}

// printLanded names every item the write pass converted, which is what makes
// the state of a store a run stopped part-way readable rather than inferred.
func printLanded(out *os.File, landed []string) {
	if len(landed) == 0 {
		return
	}
	fmt.Fprintf(out, "%d items converted:\n", len(landed))
	for _, ref := range landed {
		fmt.Fprintf(out, "  %s\n", ref)
	}
}
