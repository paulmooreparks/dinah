package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/verb"
)

// collectionAnswer is the machine form of one invocation in the sweeps below:
// the exit code, and the refusal name when the invocation refused. A test
// tallying names rather than asserting a boolean needs both, because "some
// invocation refused" passes against a sweep that reached nothing.
type collectionAnswer struct {
	code    int
	refusal string
	out     string
	errw    string
}

// answerOf runs one invocation with --json and reads the refusal name off the
// payload rather than off the sentence, so a rendering change cannot turn a
// refusal into a pass or the other way round.
func answerOf(t *testing.T, root string, argv ...string) collectionAnswer {
	t.Helper()
	got := runCLI(t, root, append(append([]string{}, argv...), "--json")...)
	answer := collectionAnswer{code: got.code, out: got.out, errw: got.errw}
	if got.code == 0 {
		return answer
	}
	var report struct {
		Refusal string            `json:"refusal"`
		Detail  string            `json:"detail"`
		Context map[string]string `json:"context"`
	}
	if err := json.Unmarshal([]byte(got.out), &report); err != nil {
		t.Fatalf("%v refused with a payload that will not parse: %v\n%s", argv, err, got.errw)
	}
	answer.refusal = report.Refusal
	return answer
}

// refusalContextOf is answerOf's context map, which is where the member count
// rides. It is a value no sentence names, so nothing but the machine form
// reports it.
func refusalContextOf(t *testing.T, root string, argv ...string) (string, map[string]string) {
	t.Helper()
	got := runCLI(t, root, append(append([]string{}, argv...), "--json")...)
	if got.code == 0 {
		t.Fatalf("%v exited 0 and this asked for a refusal:\n%s", argv, got.out)
	}
	var report struct {
		Refusal string            `json:"refusal"`
		Context map[string]string `json:"context"`
	}
	if err := json.Unmarshal([]byte(got.out), &report); err != nil {
		t.Fatalf("%v refused with a payload that will not parse: %v\n%s", argv, err, got.errw)
	}
	return report.Refusal, report.Context
}

// collectionBench builds the fixture the sweeps read: fx-1 carrying two
// comments in a known order, one open question, one acceptance criterion and
// one attachment, and fx-2 carrying nothing at all, which is the card most
// cards resemble and the case the old refusal answered with a filesystem path.
func collectionBench(t *testing.T) (string, string) {
	t.Helper()
	root := newBench(t)
	mustRun(t, root, "add", "a card with things below it")
	mustRun(t, root, "add", "a card with nothing below it")
	mustRun(t, root, "comment", "fx-1", "the first thought")
	mustRun(t, root, "comment", "fx-1", "the second thought")
	mustRun(t, root, "file", "fx-1", "open_question", "does the deadline move?")
	mustRun(t, root, "file", "fx-1", "acceptance_criterion", "the endpoint answers 404")
	source := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(source, []byte("some bytes"), 0o644); err != nil {
		t.Fatalf("write the attachment source: %v", err)
	}
	mustRun(t, root, "attach", "fx-1", source, "--description", "the first file")
	return root, source
}

// TestEveryReferenceTakingCommandAnswersACollectionOrRefusesIt runs all fifteen
// commands that take a reference against one collection reference and pins the
// four that accept it beside the eleven that refuse it.
//
// Both halves are asserted in one sweep on purpose. A guard asserting only the
// refusal passes against code that refuses every collection, which is a shape
// this workstream has produced twice, and a guard asserting only the
// acceptances passes against code that accepts them all, which would let a
// set-wide delete through. The invocation count is asserted too, so a sweep
// that reached nothing reports 0 rather than passing on an empty comparison.
func TestEveryReferenceTakingCommandAnswersACollectionOrRefusesIt(t *testing.T) {
	root, source := collectionBench(t)
	// The editor is pointed at a name no machine carries, so an edit that
	// resolved rather than refusing fails at the launch instead of opening a
	// window this suite would then wait on.
	t.Setenv("DINAH_EDITOR", "dinah-no-such-editor")

	accepting := map[string][]string{
		"path":        {"path", "fx-1/comments"},
		"show":        {"show", "fx-1/comments"},
		"contents":    {"contents", "fx-1/comments"},
		"attachments": {"attachments", "fx-1/comments"},
	}
	refusing := map[string][]string{
		"edit":         {"edit", "fx-1/comments"},
		"attach":       {"attach", "fx-1/comments", source},
		"archive":      {"archive", "fx-1/comments"},
		"restore":      {"restore", "fx-1/comments"},
		"delete":       {"delete", "fx-1/comments", "--yes"},
		"rename":       {"rename", "fx-1/comments", "renamed.txt"},
		"instructions": {"instructions", "fx-1/comments"},
		"cite":         {"cite", "fx-1/comments", "attachment", "1"},
		"resolve":      {"resolve", "fx-1/comments", "a note"},
		"verify":       {"verify", "fx-1/comments", "a note"},
		"fail":         {"fail", "fx-1/comments", "a note"},
		"reopen":       {"reopen", "fx-1/comments", "a reason"},
		"get":          {"get", "fx-1/comments", "body"},
		"set":          {"set", "fx-1/comments", "body", "rewritten"},
	}

	// The roster this sweep covers is held against the one internal/verb
	// derives from its own parameter tables, so an eighteenth command taking a
	// reference reddens here rather than being missed.
	covered := make([]string, 0, len(accepting)+len(refusing))
	for name := range accepting {
		covered = append(covered, name)
	}
	for name := range refusing {
		covered = append(covered, name)
	}
	sort.Strings(covered)
	if got, want := strings.Join(covered, " "), strings.Join(verb.ReferenceTakingCommands(), " "); got != want {
		t.Fatalf("the sweep covers\n  %s\nand the roster is\n  %s", got, want)
	}

	ran, accepted, refused := 0, 0, 0
	for name, argv := range accepting {
		ran++
		answer := answerOf(t, root, argv...)
		if answer.code != 0 {
			t.Errorf("%s refused a collection reference with %s, and it is one of the four that answer one:\n%s", name, answer.refusal, answer.errw)
			continue
		}
		accepted++
	}
	for name, argv := range refusing {
		ran++
		answer := answerOf(t, root, argv...)
		if answer.refusal != contract.IsACollection {
			t.Errorf("%s answered %q with exit %d, and it is one of the eleven that refuse a collection with %s:\n%s%s",
				name, answer.refusal, answer.code, contract.IsACollection, answer.out, answer.errw)
			continue
		}
		if answer.code != 2 {
			t.Errorf("%s refused with exit %d rather than 2", name, answer.code)
			continue
		}
		if answer.refusal == contract.UnknownPath {
			t.Errorf("%s still answers %s, which is the refusal this card replaces", name, contract.UnknownPath)
		}
		refused++
	}
	if ran != 18 {
		t.Fatalf("the sweep ran %d invocations and the roster is eighteen", ran)
	}
	if accepted != 4 || refused != 14 {
		t.Fatalf("the sweep accepted %d and refused %d, and the split is four and fourteen", accepted, refused)
	}
	t.Logf("eighteen invocations ran: %d accepted, %d refused with %s", accepted, refused, contract.IsACollection)
}

// TestShowDrawsACollectionsMembersInCreationOrder pins the members, their
// order, and above all the address printed beside each one.
//
// The address is the arm most likely to be wrong, because appending a position
// to the reference the reader typed is the easier implementation and it prints
// fx-1/checklist/1 for an item every other surface prints as fx-1/questions/1.
// One entity has one printed spelling, so the last clause hands the printed
// address back to show and requires it to resolve.
func TestShowDrawsACollectionsMembersInCreationOrder(t *testing.T) {
	root, _ := collectionBench(t)

	human := mustRun(t, root, "--lang", "en", "show", "fx-1/comments").out
	first := strings.Index(human, "fx-1/comments/1")
	second := strings.Index(human, "fx-1/comments/2")
	if first < 0 || second < 0 {
		t.Fatalf("show prints no address for one of the two comments:\n%s", human)
	}
	if first > second {
		t.Errorf("show prints fx-1/comments/2 before fx-1/comments/1, and creation order is the order:\n%s", human)
	}
	if !strings.Contains(human, "the first thought") || !strings.Contains(human, "the second thought") {
		t.Errorf("show prints an address without the text it names:\n%s", human)
	}

	var listing verb.CollectionListing
	payload := mustRun(t, root, "show", "fx-1/comments", "--json").out
	if err := json.Unmarshal([]byte(payload), &listing); err != nil {
		t.Fatalf("the collection payload will not parse: %v\n%s", err, payload)
	}
	if listing.Ref != "fx-1/comments" {
		t.Errorf("the payload reads back %q rather than the reference the reader typed", listing.Ref)
	}
	if listing.Kind != "comment" {
		t.Errorf("the payload calls the collection's members %q rather than comment", listing.Kind)
	}
	if len(listing.Members) != 2 {
		t.Fatalf("the payload carries %d members and the card carries two:\n%s", len(listing.Members), payload)
	}
	for position, member := range listing.Members {
		want := []string{"fx-1/comments/1", "fx-1/comments/2"}[position]
		if member.Ref != want {
			t.Errorf("member %d is addressed %q rather than %q", position+1, member.Ref, want)
		}
		// The head writes a trailing newline of its own after the text it
		// was handed, so the comparison is against what it was handed.
		alone := strings.TrimRight(mustRun(t, root, "show", want).out, "\n")
		if alone == "" {
			t.Fatalf("show prints nothing for %s, so this comparison would pass on two empty strings", want)
		}
		if member.Text != alone {
			t.Errorf("%s reads differently through its collection than on its own:\n through: %q\n alone:   %q", want, member.Text, alone)
		}
	}

	// The checklist collection is the arm that catches an address composed by
	// appending a position to the typed reference: the question was filed
	// first, so it is member one of the checklist and fx-1/questions/1
	// everywhere it is printed.
	var checklist verb.CollectionListing
	payload = mustRun(t, root, "show", "fx-1/checklist", "--json").out
	if err := json.Unmarshal([]byte(payload), &checklist); err != nil {
		t.Fatalf("the checklist payload will not parse: %v\n%s", err, payload)
	}
	if len(checklist.Members) != 2 {
		t.Fatalf("the checklist carries %d members and the card carries two:\n%s", len(checklist.Members), payload)
	}
	if checklist.Members[0].Ref != "fx-1/questions/1" {
		t.Errorf("the open question is addressed %q rather than fx-1/questions/1", checklist.Members[0].Ref)
	}
	for _, member := range checklist.Members {
		if got := runCLI(t, root, "show", member.Ref); got.code != 0 {
			t.Errorf("show printed the address %q and then refused it: %d %s", member.Ref, got.code, got.errw)
		}
	}
}

// TestAnEmptyCollectionIsAnEmptySetRatherThanAnError covers the commonest card
// rather than an edge, since most cards carry no comment and the comments
// directory of one that never has does not exist.
//
// The refusal's two next-step branches are told apart by the count the machine
// form carries and by the absence of a member key, rather than by quoting a
// rendered sentence.
func TestAnEmptyCollectionIsAnEmptySetRatherThanAnError(t *testing.T) {
	root, _ := collectionBench(t)

	shown := runCLI(t, root, "--lang", "en", "show", "fx-2/comments")
	if shown.code != 0 {
		t.Fatalf("show refused an empty collection: %d %s", shown.code, shown.errw)
	}
	if !strings.Contains(shown.out, "fx-2/comments holds nothing.") {
		t.Errorf("show does not print the show.collection.empty sentence naming the collection:\n%s", shown.out)
	}

	drawn := runCLI(t, root, "--lang", "en", "contents", "fx-2/comments")
	if drawn.code != 0 {
		t.Fatalf("contents refused an empty collection: %d %s", drawn.code, drawn.errw)
	}
	if !strings.Contains(drawn.out, "fx-2/comments contains nothing.") {
		t.Errorf("contents does not print the contents.empty.collection sentence:\n%s", drawn.out)
	}

	name, context := refusalContextOf(t, root, "delete", "fx-2/comments", "--yes")
	if name != contract.IsACollection {
		t.Fatalf("delete answered %q for an empty collection rather than %s", name, contract.IsACollection)
	}
	if context["count"] != "0" {
		t.Errorf("the refusal carries count %q for a collection holding nothing", context["count"])
	}
	if member, carried := context["member"]; carried {
		t.Errorf("the refusal offers the member %q of a collection that holds none", member)
	}
	sentence := runCLI(t, root, "--lang", "en", "delete", "fx-2/comments", "--yes").errw
	if !strings.Contains(sentence, "it holds nothing") {
		t.Errorf("the refusal ends on the next-member splice rather than on the empty one:\n%s", sentence)
	}
}

// TestPathAnswersACollectionWhetherOrNotItsDirectoryExists is the one place
// this card changes path, and the last clause is the arm that can fail:
// dropping the guard on the collection branch must not make a positional
// selector resolve against a directory that is not there.
func TestPathAnswersACollectionWhetherOrNotItsDirectoryExists(t *testing.T) {
	root, _ := collectionBench(t)

	carrying := runCLI(t, root, "path", "fx-1/comments")
	if carrying.code != 0 {
		t.Fatalf("path refused a collection that holds two comments: %d %s", carrying.code, carrying.errw)
	}

	absent := runCLI(t, root, "path", "fx-2/comments")
	if absent.code != 0 {
		t.Fatalf("path refused a collection the containment table declares: %d %s", absent.code, absent.errw)
	}
	answered := strings.TrimSpace(absent.out)
	if !filepath.IsAbs(answered) {
		t.Errorf("path answered %q, which is not an absolute path", answered)
	}
	if filepath.Base(answered) != "comments" {
		t.Errorf("path answered %q, whose last segment is not comments", answered)
	}
	card := strings.TrimSpace(mustRun(t, root, "path", "fx-2").out)
	if got, want := filepath.Dir(answered), filepath.Dir(card); got != want {
		t.Errorf("the collection sits in %q and the card's own file sits in %q", got, want)
	}

	member := answerOf(t, root, "path", "fx-2/comments/1")
	if member.refusal != contract.UnknownPath {
		t.Errorf("fx-2/comments/1 answered %q with exit %d, and a member that is not there is still unknown", member.refusal, member.code)
	}
}

// TestAWalkRootedAtACollectionDrawsTheHoldersRowsForIt compares two walks
// rather than asserting a shape, so a walk that composed its children against
// the collection reference (fx-1/comments/comments/1) or drew the whole
// checklist under a narrowed reference fails on the comparison.
//
// Each subject set has its size asserted, because a parse that found no rows
// otherwise reports success on an empty comparison.
func TestAWalkRootedAtACollectionDrawsTheHoldersRowsForIt(t *testing.T) {
	root, _ := collectionBench(t)

	holder := treeOf(t, root, "fx-1")
	for _, pair := range []struct {
		ref  string
		kind string
		rows int
	}{
		{ref: "fx-1/comments", kind: "comment", rows: 2},
		{ref: "fx-1/attachments", kind: "attachment", rows: 1},
		{ref: "fx-1/checklist", kind: "item", rows: 2},
	} {
		want := collectionRowsOfKind(holder, pair.kind)
		if len(want) != pair.rows {
			t.Fatalf("the walk rooted at fx-1 draws %d %s rows and the fixture carries %d", len(want), pair.kind, pair.rows)
		}
		got := collectionRowsOf(treeOf(t, root, pair.ref).Root.Children)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("the walk rooted at %s draws\n  %s\nand the walk rooted at its holder draws\n  %s",
				pair.ref, strings.Join(got, " | "), strings.Join(want, " | "))
		}
	}

	// The narrowed spelling draws the question and nothing else, which is the
	// arm that fails when a narrowed walk draws the whole checklist.
	narrowed := treeOf(t, root, "fx-1/questions")
	rows := collectionRowsOf(narrowed.Root.Children)
	if len(rows) != 1 {
		t.Fatalf("the walk rooted at fx-1/questions draws %d rows and the card carries one question:\n  %s", len(rows), strings.Join(rows, " | "))
	}
	if !strings.HasPrefix(rows[0], "fx-1/questions/1 ") {
		t.Errorf("the walk rooted at fx-1/questions draws %q rather than the question's own address", rows[0])
	}

	collection := treeOf(t, root, "fx-1/comments")
	if collection.Root.Ref != "fx-1/comments" {
		t.Errorf("the root reads back %q rather than the reference the reader typed", collection.Root.Ref)
	}
	if collection.Root.Kind != verb.KindCollection {
		t.Errorf("the root's kind is %q rather than %q", collection.Root.Kind, verb.KindCollection)
	}
	if collection.Root.ID != "" || collection.Root.Title != "" {
		t.Errorf("the root carries an id %q and a title %q, and a collection has neither", collection.Root.ID, collection.Root.Title)
	}
	if collection.Root.Count != 2 {
		t.Errorf("the root counts %d entities at or below fx-1/comments and there are two", collection.Root.Count)
	}
	header := runCLI(t, root, "--lang", "en", "contents", "fx-1/comments")
	if !strings.Contains(header.out, "fx-1/comments contains 2 entities.") {
		t.Errorf("the header does not name the reference and the count:\n%s", header.out)
	}
}

// treeOf reads one contents tree as the machine form reports it.
func treeOf(t *testing.T, root, ref string) verb.Tree {
	t.Helper()
	payload := mustRun(t, root, "contents", ref, "--depth", "all", "--json").out
	var tree verb.Tree
	if err := json.Unmarshal([]byte(payload), &tree); err != nil {
		t.Fatalf("the tree for %s will not parse: %v\n%s", ref, err, payload)
	}
	return tree
}

// rowsOf is the Reference and Entity cells of a walk's own children, which is
// what the two walks are compared on.
func collectionRowsOf(nodes []verb.TreeNode) []string {
	rows := make([]string, 0, len(nodes))
	for _, node := range nodes {
		rows = append(rows, node.Ref+" "+node.Kind)
	}
	return rows
}

// collectionRowsOfKind is rowsOf narrowed to the children of one kind, which is how the
// holder's own walk is compared against a walk rooted at one of its
// collections.
func collectionRowsOfKind(tree verb.Tree, kind string) []string {
	var kept []verb.TreeNode
	for _, node := range tree.Root.Children {
		if node.Kind == kind {
			kept = append(kept, node)
		}
	}
	return collectionRowsOf(kept)
}

// TestAttachmentsAnswersACollectionFromItsHolder pins the identical-bytes
// arm, which is what proves the listing is answered from the holder rather
// than recomposed, and it fails the moment the header carries the collection
// reference where the holder's belongs.
func TestAttachmentsAnswersACollectionFromItsHolder(t *testing.T) {
	root, source := collectionBench(t)
	mustRun(t, root, "attach", "workbench", source, "--description", "the workbench's own file")
	mustRun(t, root, "attach", "fx-1/comments/1", source, "--description", "the comment's own file")

	for _, pair := range []struct{ long, short string }{
		{long: "fx-1/attachments", short: "fx-1"},
		{long: "fx/attachments", short: "workbench"},
		{long: "fx-1/comments/1/attachments", short: "fx-1/comments/1"},
	} {
		human := mustRun(t, root, "--lang", "en", "attachments", pair.long).out
		want := mustRun(t, root, "--lang", "en", "attachments", pair.short).out
		if human != want {
			t.Errorf("attachments %s prints\n%s\nand attachments %s prints\n%s", pair.long, human, pair.short, want)
		}
		payload := mustRun(t, root, "attachments", pair.long, "--json").out
		wantPayload := mustRun(t, root, "attachments", pair.short, "--json").out
		if payload != wantPayload {
			t.Errorf("attachments %s answers\n%s\nand attachments %s answers\n%s", pair.long, payload, pair.short, wantPayload)
		}
	}

	empty := runCLI(t, root, "--lang", "en", "attachments", "fx-1/comments")
	if empty.code != 0 {
		t.Fatalf("attachments refused a collection that hangs nothing: %d %s", empty.code, empty.errw)
	}
	if !strings.Contains(empty.out, "fx-1/comments carries no attachments.") {
		t.Errorf("attachments does not print the attachments.empty sentence naming the collection:\n%s", empty.out)
	}
	payload := mustRun(t, root, "attachments", "fx-1/comments", "--json").out
	var listing verb.AttachmentListing
	if err := json.Unmarshal([]byte(payload), &listing); err != nil {
		t.Fatalf("the listing will not parse: %v\n%s", err, payload)
	}
	if listing.Ref != "fx-1/comments" || listing.Kind != verb.KindCollection {
		t.Errorf("the listing reports ref %q and kind %q", listing.Ref, listing.Kind)
	}
	if listing.Attachments == nil {
		t.Error("the listing carries a null where an empty array belongs, and the two read differently to a machine caller")
	}
	if !strings.Contains(payload, `"attachments": []`) {
		t.Errorf("the payload does not carry an empty attachments array:\n%s", payload)
	}
}

// TestTheRefusalThisCardReplacesStillAnswers is the other half of the accept
// and refuse split: a guard that only asserts the new refusal passes against
// code raising it for everything.
//
// The tally is reported rather than a boolean, so a run that reached none of
// these invocations reports 0 for both names and fails.
func TestTheRefusalThisCardReplacesStillAnswers(t *testing.T) {
	root, _ := collectionBench(t)

	seen := map[string]int{}
	for _, invocation := range [][]string{
		{"show", "fx/cards/1"},
		{"show", "fx/columns/1"},
		{"delete", "fx/cards", "--yes"},
		{"show", "fx-1/comments/9"},
		{"show", "fx-1/nonsense"},
		{"instructions", "nonsense-9"},
	} {
		answer := answerOf(t, root, invocation...)
		if answer.code == 0 {
			t.Errorf("%v exited 0 and every one of these names nothing:\n%s", invocation, answer.out)
			continue
		}
		seen[answer.refusal]++
		if answer.refusal != contract.UnknownPath {
			t.Errorf("%v answered %q rather than %s", invocation, answer.refusal, contract.UnknownPath)
		}
	}
	// The new name has to occur too, or this guard would pass against code
	// that never raises it.
	seen[answerOf(t, root, "delete", "fx-1/comments", "--yes").refusal]++
	t.Logf("the sweep saw %s %d times and %s %d times",
		contract.UnknownPath, seen[contract.UnknownPath], contract.IsACollection, seen[contract.IsACollection])
	if seen[contract.UnknownPath] != 6 {
		t.Errorf("%s answered %d of the six invocations that still name nothing", contract.UnknownPath, seen[contract.UnknownPath])
	}
	if seen[contract.IsACollection] != 1 {
		t.Errorf("%s answered %d of the one invocation naming a collection", contract.IsACollection, seen[contract.IsACollection])
	}

	// The addressed detail is what tells a reader that a card is named by its
	// own reference, and it is raised above the collection branch, so it must
	// not have turned into the new name.
	_, context := refusalContextOf(t, root, "show", "fx/cards/1")
	if context["addressed"] != "card" {
		t.Errorf("fx/cards/1 refuses with addressed %q rather than card", context["addressed"])
	}
	_, context = refusalContextOf(t, root, "show", "fx/columns/1")
	if context["addressed"] != "column" {
		t.Errorf("fx/columns/1 refuses with addressed %q rather than column", context["addressed"])
	}
}
