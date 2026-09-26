package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// offerDefinition is the flow the offer's parity test stands in. Every
// refusing row the offer's shared checks can reach has a column here that
// raises it: an intake that takes no work up, the working station, an
// operator-owned station, one waiting on somebody outside, one at capacity,
// one holding on entry, one requiring a declared field, one with a loop limit written
// after init, one holding on exit, the done column, and a column after it.
// The tier table lists acme/workhorse below acme/frontier.
const offerDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Offer",
  "levels": { "tier": ["workhorse", "frontier"] },
  "fields": { "card.lane": { "type": "string", "meaning": "the lane a card belongs to" } },
  "tiers": {
    "workhorse": { "meaning": "scoped work", "models": [{ "provider": "acme", "model": "workhorse" }] },
    "frontier": { "meaning": "judgement work", "models": [{ "provider": "acme", "model": "frontier" }] }
  },
  "evidence": { "test": {} },
  "columns": [
    { "id": "o10000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "o10000000002", "title": "Work", "slug": "work", "kind": "work" },
    { "id": "o10000000003", "title": "Review", "slug": "review", "kind": "work", "operator_owned": true },
    { "id": "o10000000004", "title": "Waiting", "slug": "waiting", "kind": "work", "awaiting_outside": true },
    { "id": "o10000000005", "title": "Full", "slug": "full", "kind": "work", "capacity": 1 },
    { "id": "o10000000006", "title": "Gated", "slug": "gated", "kind": "work", "gate_items": true },
    { "id": "o10000000007", "title": "Needs", "slug": "needs", "kind": "work", "require_fields": ["card.lane"] },
    { "id": "o10000000008", "title": "Loop", "slug": "loop", "kind": "work" },
    { "id": "o10000000009", "title": "Exit", "slug": "exit", "kind": "work", "gate_items": "out" },
    { "id": "o10000000010", "title": "Done", "slug": "done", "kind": "done" },
    { "id": "o10000000011", "title": "After", "slug": "after", "kind": "work" }
  ]
}`

// offerCase is one state of section 11.6: how to arrange the card fx-1, and
// the identity the offer and the oracle ask as.
type offerCase struct {
	name    string
	arrange func(t *testing.T, root string)
	asking  func(t *testing.T)
}

// step runs one CLI act of an arrangement, failing the test where it is
// refused.
func step(t *testing.T, root string, argv ...string) {
	t.Helper()
	if got := runCLI(t, root, argv...); got.code != 0 {
		t.Fatalf("%v: %d %s", argv, got.code, got.errw)
	}
}

// asOwner sets the identity the asking side of a case uses.
func asOwner(actor, provider, model, harness string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Setenv("DINAH_ACTOR", actor)
		if actor == "" {
			os.Unsetenv("DINAH_ACTOR")
		}
		t.Setenv("DINAH_PROVIDER", provider)
		t.Setenv("DINAH_MODEL", model)
		t.Setenv("DINAH_HARNESS", harness)
	}
}

// offerCases are the states section 11.6 lists, each with the card fx-1 in
// that state.
var offerCases = []offerCase{
	{name: "a ready card for the operator", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
	}, asking: asOwner("alka", "", "", "")},
	{name: "a ready card at an operator-owned column, for a non-operator", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "review")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a card held by the caller", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		step(t, root, "claim", "fx-1", "--actor", "brin")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a card held by another owner", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		step(t, root, "claim", "fx-1", "--actor", "cato")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a blocked card", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		step(t, root, "block", "fx-1", "waiting on a vendor")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a lapsed claim by another owner", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		step(t, root, "claim", "fx-1", "--actor", "cato", "--expires", "1h")
		lapsed := regexpReplace(anchorText(t, root, "fx-1"), `claim_expires: .*`, "claim_expires: 2000-01-01T00:00:00Z")
		writeAnchor(t, root, "fx-1", lapsed)
	}, asking: asOwner("brin", "", "", "")},
	{name: "a card at a column that takes no work up", arrange: func(t *testing.T, root string) {
	}, asking: asOwner("brin", "", "", "")},
	{name: "a card at a column declaring awaiting_outside", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "waiting")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a card carrying an item that names no declared column", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		step(t, root, "file", "--column", "work", "fx-1", "open_question", "Which vendor?")
		renameItemColumn(t, filepath.Dir(anchorPath(t, root, "fx-1")))
	}, asking: asOwner("brin", "", "", "")},
	{name: "a card above the caller's tier, with a declared model below it", arrange: tieredCard, asking: asOwner("brin", "acme", "workhorse", "")},
	{name: "a card above the caller's tier, with a declared model the table does not list", arrange: tieredCard, asking: asOwner("brin", "acme", "unknown", "")},
	{name: "a card above the caller's tier, with no declared model", arrange: tieredCard, asking: asOwner("brin", "", "", "")},
	{name: "a destination at capacity", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		step(t, root, "move", "fx-2", "full")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a destination holding an item that names it", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		step(t, root, "file", "--column", "gated", "fx-1", "open_question", "Is the gate open?")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a departure holding an exit item", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "exit")
		step(t, root, "file", "--column", "exit", "fx-1", "open_question", "May it leave?")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a destination requiring a field the card lacks", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a departure at its loop limit", arrange: func(t *testing.T, root string) {
		declareLoopLimit(t, root, "Loop", "1")
		step(t, root, "move", "fx-1", "loop")
		step(t, root, "move", "fx-1", "work")
		step(t, root, "move", "fx-1", "loop")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a card at a done column, for a forward move", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "done")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a destination declaring awaiting_outside, for a held card", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		step(t, root, "claim", "fx-1", "--actor", "brin")
	}, asking: asOwner("brin", "", "", "")},
	{name: "a destination being retired", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		retireColumn(t, root, "Loop")
	}, asking: asOwner("brin", "", "", "")},
	{name: "no actor", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
	}, asking: asOwner("", "", "", "")},
	{name: "a malformed harness", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
	}, asking: asOwner("brin", "", "", "a b")},
	{name: "no operator", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		editWorkbenchAnchor(t, filepath.Join(benchDir(t, root), bench.WorkbenchAnchor), "operator: alka\n", "")
	}, asking: asOwner("brin", "", "", "")},
	{name: "items of every kind and state, for an agent", arrange: itemsOfEveryState, asking: asOwner("brin", "", "", "")},
	{name: "items of every kind and state, for the operator", arrange: itemsOfEveryState, asking: asOwner("alka", "", "", "")},
	{name: "items under the criterion-retirement grant, for an agent", arrange: func(t *testing.T, root string) {
		itemsOfEveryState(t, root)
		step(t, root, "grant", "fx-1", verb.CriterionRetirement)
	}, asking: asOwner("brin", "", "", "")},
	{name: "a card with links, workstreams, an attachment and an edited comment, for an agent", arrange: membersOfEveryKind, asking: asOwner("brin", "", "", "")},
	{name: "a card with links, workstreams, an attachment and an edited comment, for the operator", arrange: membersOfEveryKind, asking: asOwner("alka", "", "", "")},
	{name: "a blocked card, for the operator", arrange: func(t *testing.T, root string) {
		step(t, root, "move", "fx-1", "work")
		step(t, root, "block", "fx-1", "waiting on a vendor")
	}, asking: asOwner("alka", "", "", "")},
}

// itemsOfEveryState files on fx-1, standing at the work station, one item in
// every state an item verb reads: an operator-owned question and a holder's
// question, both pending; a decision already resolved; a criterion pending
// with no citation, one pending with a citation, one failed, one waived and
// one withdrawn; and a question demanding evidence of a scheme nothing cited.
func itemsOfEveryState(t *testing.T, root string) {
	t.Helper()
	step(t, root, "move", "fx-1", "work")
	step(t, root, "file", "fx-1", "open_question", "Which vendor ships it?", "--owner", "operator")
	step(t, root, "file", "fx-1", "open_question", "Is the parser in scope?", "--owner", "holder")
	step(t, root, "file", "fx-1", "decision", "Use the stream reader")
	step(t, root, "resolve", "fx-1/decisions/1", "--text", "decided on the stream reader")
	step(t, root, "file", "fx-1", "acceptance_criterion", "The parser reads every line")
	step(t, root, "file", "fx-1", "acceptance_criterion", "The parser rejects a torn line")
	step(t, root, "cite", "fx-1/criteria/2", "test", "cmd/dinah/parser_test.go")
	step(t, root, "file", "fx-1", "acceptance_criterion", "The parser is fast")
	step(t, root, "cite", "fx-1/criteria/3", "test", "cmd/dinah/speed_test.go")
	step(t, root, "fail", "fx-1/criteria/3", "--text", "it took a minute")
	step(t, root, "file", "fx-1", "acceptance_criterion", "The parser logs")
	step(t, root, "waive", "fx-1/criteria/4", "--text", "logging is not needed here")
	step(t, root, "file", "fx-1", "acceptance_criterion", "The parser is pretty")
	step(t, root, "withdraw", "fx-1/criteria/5", "--text", "nobody asked for it")
	step(t, root, "file", "fx-1", "open_question", "Does it meet the benchmark?")
	step(t, root, "set", "fx-1/questions/3", "evidence", "benchmark")
}

// membersOfEveryKind gives fx-1, standing at the work station, a link to
// fx-2, membership of one of two workstreams, an attachment, a comment left as
// written and a comment whose body was edited outside the tool.
func membersOfEveryKind(t *testing.T, root string) {
	t.Helper()
	step(t, root, "move", "fx-1", "work")
	step(t, root, "workstream", "new", "Parser", "--slug", "parser")
	step(t, root, "workstream", "new", "Release", "--slug", "release")
	step(t, root, "join", "fx-1", "parser")
	step(t, root, "link", "fx-1", "relates_to", "fx-2")
	step(t, root, "attach", "fx-1", offerAttachment(t))
	step(t, root, "comment", "fx-1", "left as written")
	step(t, root, "comment", "fx-1", "about to be edited")
	path := runCLI(t, root, "path", "fx-1/comments/2")
	if path.code != 0 {
		t.Fatalf("path of the second comment: %s", path.errw)
	}
	anchor := strings.TrimSpace(path.out)
	data, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), "about to be edited", "edited by hand", 1)
	if edited == string(data) {
		t.Fatalf("the comment's body was not where the fixture looked: %q", data)
	}
	if err := os.WriteFile(anchor, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
}

// tieredCard sets fx-1's own tier to frontier and stands it at the work
// station, where the claim's tier row reads it.
func tieredCard(t *testing.T, root string) {
	t.Helper()
	step(t, root, "set", "fx-1", "tier", "frontier")
	step(t, root, "move", "fx-1", "work")
}

// renameItemColumn rewrites the column every item under a card directory
// names to an identifier the workbench does not declare, which is how an
// item comes to name no declared column.
func renameItemColumn(t *testing.T, cardDir string) {
	t.Helper()
	renamed := 0
	column := regexp.MustCompile(`(?m)^column: .*$`)
	filepath.WalkDir(filepath.Join(cardDir, bench.ChecklistDir), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != bench.ItemAnchor {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := column.ReplaceAllString(string(data), "column: 000000000000")
		if text != string(data) {
			renamed++
		}
		return os.WriteFile(path, []byte(text), 0o644)
	})
	if renamed == 0 {
		t.Fatalf("no item under %s names a column", cardDir)
	}
}

// retireColumn plants the sibling a structural act leaves beside the
// directory of the column with a title while it retires it, which makes the
// move's retiring row reachable.
func retireColumn(t *testing.T, root, title string) {
	t.Helper()
	opened, err := bench.Open(soleBenchDir(t, root))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	dir := ""
	for _, column := range opened.Columns {
		if column.Title == title {
			dir = filepath.Join(soleBenchDir(t, root), bench.ColumnsDir, column.ID)
		}
	}
	if dir == "" {
		t.Fatalf("the fixture flow carries no column titled %s", title)
	}
	line, err := json.Marshal(bench.LockRecord{Actor: "alka", PID: 4321, TS: "2026-09-25T10:00:00Z", Op: bench.OpArchive})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bench.SiblingPath(dir), line, 0o644); err != nil {
		t.Fatalf("plant the sibling: %v", err)
	}
}

// offerOf asks the library what may be offered on fx-1, as the environment
// the test set resolves, with the anchor and the journal read before and
// after so a test can hold the offer to writing nothing.
func offerOf(t *testing.T, root string) (*verb.OfferedActs, string, string) {
	t.Helper()
	opened, err := bench.Open(soleBenchDir(t, root))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	l := verb.New(opened, os.Getenv("DINAH_HOME"))
	offered, err := l.OfferActs(&verb.Request{
		Verb:     verb.Move,
		Card:     "fx-1",
		Column:   offerPullColumn,
		Actor:    os.Getenv("DINAH_ACTOR"),
		Harness:  os.Getenv("DINAH_HARNESS"),
		Provider: os.Getenv("DINAH_PROVIDER"),
		Model:    os.Getenv("DINAH_MODEL"),
	})
	if err != nil {
		t.Fatalf("offer: %v", err)
	}
	return offered, anchorText(t, root, "fx-1"), journalText(t, root, "fx-1")
}

// offerPullColumn is the column the offer is asked a pull into, standing for
// the focused lane's, and the column the oracle's pull names.
const offerPullColumn = "work"

// offeredSet names every act an offer allows, in the oracle's spelling.
func offeredSet(offered *verb.OfferedActs, refOf map[string]string) []string {
	var acts []string
	flags := map[string]bool{
		"claim": offered.Claim, "release": offered.Release, "comment": offered.Comment,
		"add": offered.Add, "pull": offered.Pull, "block": offered.Block, "unblock": offered.Unblock,
		"attach": offered.Attach, "file": offered.File, "archive": offered.Archive,
		"delete": offered.Delete, "edit": offered.Edit, "link": offered.LinkNew,
	}
	for name, on := range flags {
		if on {
			acts = append(acts, name)
		}
	}
	for _, move := range offered.Moves {
		acts = append(acts, "move "+refOf[move.Column])
	}
	for _, tier := range offered.RaiseTiers {
		acts = append(acts, "raise "+tier)
	}
	for _, permission := range offered.Grants {
		acts = append(acts, "grant "+permission)
	}
	for _, permission := range offered.Revokes {
		acts = append(acts, "revoke "+permission)
	}
	for _, link := range offered.Links {
		acts = append(acts, "unlink "+link.Kind+" "+link.To)
	}
	for _, workstream := range offered.Joins {
		acts = append(acts, "join "+workstream)
	}
	for _, workstream := range offered.Leaves {
		acts = append(acts, "leave "+workstream)
	}
	for _, ref := range offered.Divergences {
		acts = append(acts, "accept-divergence "+ref)
	}
	for _, ref := range offered.Renames {
		acts = append(acts, "rename "+ref)
	}
	for _, field := range offered.Fields {
		acts = append(acts, "set "+field)
	}
	for _, item := range offered.Items {
		named := map[string]bool{
			"resolve": item.Resolve, "verify": item.Verify, "fail": item.Fail, "waive": item.Waive,
			"withdraw": item.Withdraw, "reopen": item.Reopen, "cite": item.Cite,
		}
		for name, on := range named {
			if on {
				acts = append(acts, name+" "+item.Ref)
			}
		}
	}
	sort.Strings(acts)
	return acts
}

// oracleAct is one real act the oracle runs on a fresh copy of the
// workbench: the words, and the name the offer's set spells it by.
type oracleAct struct {
	words []string
	name  string
}

// oracleActs are every act a real CLI act is asked for on fx-1 in one state:
// claim, release, comment and a move to every other column, which dinah-603
// asked, and one act per card verb dinah-623 offers, each with arguments that
// pass the rows reading an argument, so only the rows the offer shares can
// refuse it.
//
// Three verbs are asked narrower than the act accepts, because the act takes
// an argument that changes nothing and the offer does not list it: join is
// asked of the workstreams the card is not in and leave of the ones it is,
// and accept-divergence of the comments whose body was edited, since a
// membership already held and a body never edited give the act nothing to
// record.
func oracleActs(t *testing.T, root string, opened *bench.Bench, card *bench.Card) []oracleAct {
	t.Helper()
	acts := []oracleAct{
		{words: []string{"claim", "fx-1"}, name: "claim"},
		{words: []string{"release", "fx-1"}, name: "release"},
		{words: []string{"comment", "fx-1", "a remark"}, name: "comment"},
		{words: []string{"add", "A card filed beside it"}, name: "add"},
		{words: []string{"--json", "pull", offerPullColumn}, name: "pull"},
		{words: []string{"block", "fx-1", "waiting on a vendor"}, name: "block"},
		{words: []string{"unblock", "fx-1"}, name: "unblock"},
		{words: []string{"attach", "fx-1", offerAttachment(t)}, name: "attach"},
		{words: []string{"file", "fx-1", "decision", "a decision to take"}, name: "file"},
		{words: []string{"archive", "fx-1"}, name: "archive"},
		{words: []string{"delete", "fx-1", "--yes"}, name: "delete"},
		{words: []string{"edit", "fx-1"}, name: "edit"},
		{words: []string{"link", "fx-1", "relates_to", "fx-2"}, name: "link"},
		{words: []string{"grant", "fx-1", verb.CriterionRetirement}, name: "grant " + verb.CriterionRetirement},
		{words: []string{"revoke", "fx-1", verb.CriterionRetirement}, name: "revoke " + verb.CriterionRetirement},
	}
	for _, column := range opened.Columns {
		if column.ID != card.Column {
			acts = append(acts, oracleAct{words: []string{"move", "fx-1", column.Ref()}, name: "move " + column.Ref()})
		}
	}
	for _, tier := range bench.LevelNames(opened.Levels(bench.TierField)) {
		acts = append(acts, oracleAct{words: []string{"raise", "fx-1", tier, "the work is harder"}, name: "raise " + tier})
	}
	for _, link := range card.Links {
		acts = append(acts, oracleAct{words: []string{"unlink", "fx-1", link.Kind, link.To}, name: "unlink " + link.Kind + " " + link.To})
	}
	workstreams, err := opened.Workstreams()
	if err != nil {
		t.Fatal(err)
	}
	for _, workstream := range workstreams {
		name := "join"
		if slices.Contains(card.Workstreams, workstream.ID) {
			name = "leave"
		}
		acts = append(acts, oracleAct{words: []string{name, "fx-1", workstream.Slug}, name: name + " " + workstream.Slug})
	}
	acts = append(acts, memberActs(t, root, card)...)
	for _, field := range bench.FieldsOf(bench.KindCard) {
		words := []string{"set", "fx-1", field}
		if field == bench.TitleField {
			words = append(words, "A new title")
		}
		acts = append(acts, oracleAct{words: words, name: "set " + field})
	}
	for _, key := range opened.DeclaredFieldKeysOn(bench.KindCard) {
		acts = append(acts, oracleAct{words: []string{"set", "fx-1", key, "north"}, name: "set " + key})
	}
	return append(acts, itemActs(t, card)...)
}

// memberActs are the oracle's acts on fx-1's comments whose body was edited
// and on its attachments.
func memberActs(t *testing.T, root string, card *bench.Card) []oracleAct {
	t.Helper()
	var acts []oracleAct
	comments, err := bench.Comments(card.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for i, comment := range comments {
		fm, body, err := bench.ReadCommentAnchor(comment.Dir)
		if err != nil || !bench.CommentDiverged(fm, body) {
			continue
		}
		ref := "fx-1/comments/" + strconv.Itoa(i+1)
		acts = append(acts, oracleAct{words: []string{"accept-divergence", ref}, name: "accept-divergence " + ref})
	}
	attachments, err := bench.Attachments(card.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range attachments {
		ref := "fx-1/attachments/" + strconv.Itoa(i+1)
		acts = append(acts, oracleAct{words: []string{"rename", ref, "renamed.txt"}, name: "rename " + ref})
	}
	return acts
}

// itemActs are the oracle's acts on every item of fx-1: each of the seven
// item verbs, with arguments that pass the rows reading one.
func itemActs(t *testing.T, card *bench.Card) []oracleAct {
	t.Helper()
	items, err := bench.Items(card.Dir)
	if err != nil {
		t.Fatal(err)
	}
	position := map[string]int{}
	var acts []oracleAct
	for _, item := range items {
		position[item.Kind]++
		word, _ := bench.WordForItemKind(item.Kind)
		ref := "fx-1/" + word + "/" + strconv.Itoa(position[item.Kind])
		acts = append(acts,
			oracleAct{words: []string{"resolve", ref, "--text", "an answer given"}, name: "resolve " + ref},
			oracleAct{words: []string{"verify", ref, "--text", "the check held"}, name: "verify " + ref},
			oracleAct{words: []string{"fail", ref, "--text", "the check did not hold"}, name: "fail " + ref},
			oracleAct{words: []string{"waive", ref, "--text", "the operator waived it"}, name: "waive " + ref},
			oracleAct{words: []string{"withdraw", ref, "--text", "it stopped applying"}, name: "withdraw " + ref},
			oracleAct{words: []string{"reopen", ref, "it was closed wrongly"}, name: "reopen " + ref},
			oracleAct{words: []string{"cite", ref, "test", "cmd/dinah/offer_test.go"}, name: "cite " + ref},
		)
	}
	return acts
}

// offerAttachment is a file the oracle's attach names, which exists for the
// whole test.
func offerAttachment(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "evidence.txt")
	if err := os.WriteFile(path, []byte("evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestTheOfferMatchesEveryAct is dinah-603/criteria/6, and dinah-623/criteria/17
// and /39. In every state of section 11.6, and in the states dinah-623 adds
// for the card verbs it offers, the acts OfferActs answers for fx-1 are
// exactly the acts a real CLI act accepts on a fresh copy of the workbench:
// claim, release, comment and a move to every other column, and every card
// verb, each item verb on every item of the card, and a filing and a pull
// beside them. The offer leaves the card's anchor and journal byte-identical.
//
// It also counts its coverage. The refusal names the shared check functions
// reference as contract names are read with go/parser, and every one has to
// be reached by some oracle act across the fixtures or stand on the written
// exemption list, and no exempted name may be reached or have stopped being
// referenced. The count reads those bodies alone: a refusal raised in a new
// helper one of them calls, or through a method value, a hand-built response,
// or an error a lower layer returns, is not in the expected set, and is caught
// only where a fixture here builds the state that raises it.
func TestTheOfferMatchesEveryAct(t *testing.T) {
	// The oracle's edit launches an editor, which is this test binary
	// standing in as a recorder, so the edit succeeds without a person.
	t.Setenv("DINAH_EDITOR", os.Args[0])
	t.Setenv(editorRecordVar, filepath.Join(t.TempDir(), "editor.log"))
	reached := map[string]bool{}
	for _, c := range offerCases {
		t.Run(c.name, func(t *testing.T) {
			root := newBenchFromDefinition(t, offerDefinition)
			step(t, root, "add", "The card offered")
			step(t, root, "add", "The card beside it")
			c.arrange(t, root)
			c.asking(t)
			offered, anchor, journal := offerOf(t, root)
			if after := anchorText(t, root, "fx-1"); after != anchor {
				t.Errorf("the offer changed the anchor")
			}
			if after := journalText(t, root, "fx-1"); after != journal {
				t.Errorf("the offer changed the journal")
			}
			opened, err := bench.Open(soleBenchDir(t, root))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			refOf := map[string]string{}
			var acts []oracleAct
			for _, column := range opened.Columns {
				refOf[column.ID] = column.Ref()
			}
			card, err := opened.ResolveCard("fx-1")
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			acts = oracleActs(t, root, opened, card.Card)
			var accepted []string
			for _, act := range acts {
				fresh := copyWorkbench(t, root)
				got := runCLI(t, fresh, act.words...)
				if got.code == 0 && (act.name != "pull" || strings.Contains(got.out, `"card"`)) {
					accepted = append(accepted, act.name)
					continue
				}
				if got.code == 0 {
					continue
				}
				t.Logf("%v refused: %s", act.words, strings.SplitN(got.errw, "\n", 2)[0])
				if fields := strings.Fields(got.errw); len(fields) > 0 {
					reached[fields[0]] = true
				}
			}
			sort.Strings(accepted)
			want := offeredSet(offered, refOf)
			if strings.Join(want, ", ") != strings.Join(accepted, ", ") {
				t.Errorf("offered [%s], and the real acts accepted [%s]", strings.Join(want, ", "), strings.Join(accepted, ", "))
			}
			t.Logf("%d acts tried, %d accepted", len(acts), len(accepted))
		})
	}
	expected := sharedCheckRefusals(t)
	exempt := map[string]string{
		contract.UnknownColumn:   "the offer names only declared columns",
		contract.NotRequester:    "the head never sets a holder",
		contract.UnknownCard:     "the head only acts on a card it just read",
		contract.NotCommentable:  "the head only acts on a card it just read",
		contract.AddNeedsAColumn: "the head files a card only on a workbench whose flow it drew columns from",
		contract.NotAttachable:   "the head attaches only to the card it just read, which mounts attachments",
		contract.NotRenamable:    "the offer asks a rename only of the card's own attachments",
		contract.UnknownPath:     "the offer asks each item act only of an item, a removal only of a card and a divergence only of a comment",
		contract.UnknownValue:    "the offer asks the grant verbs only of the permissions they know",
		contract.Malformed:       "the offer asks the grant verbs only of a permission it names",
	}
	hit, exempted := 0, 0
	for _, name := range expected {
		_, isExempt := exempt[name]
		switch {
		case isExempt && reached[name]:
			t.Errorf("%s is exempted, as %s, and a fixture reached it, so the exemption is stale", name, exempt[name])
		case isExempt:
			exempted++
		case reached[name]:
			hit++
		default:
			t.Errorf("the shared checks can refuse %s, and no fixture reaches it and no exemption names it", name)
		}
	}
	for name := range exempt {
		if !slices.Contains(expected, name) {
			t.Errorf("%s is exempted and no shared check references it any more", name)
		}
	}
	t.Logf("%d refusal names found in the shared checks, %d reached by the fixtures, %d exempted", len(expected), hit, exempted)
	if len(expected) == 0 || len(offerCases) < 20 {
		t.Fatalf("found %d names and ran %d fixtures, so the sweep proves nothing", len(expected), len(offerCases))
	}
}

// sharedCheckFunctions are the functions whose bodies the coverage count
// reads: dinah-603's twelve, and the check functions dinah-623 moved every
// other card verb's rows into.
var sharedCheckFunctions = []string{
	"admit", "malformedHarness", "canComment", "canClaim", "claimableState", "claimableColumn",
	"claimableItems", "claimableTier", "takesNoWorkName", "canRelease", "canRoute", "canLand",
	"admitAdd", "addHasColumns", "canPull", "pullableDeparture", "canBlock", "canUnblock",
	"canRaise", "raiseColumn", "canAttach", "canFile", "admitItem", "canCloseItem",
	"canCloseItemEvidence", "canWaive", "canWithdraw", "canReopen", "admitGrantVerb", "canRevoke",
	"admitLink", "canJoin", "admitRemoval", "canRemove", "canRename", "canAcceptDivergence",
	"canWriteEntity",
}

// sharedCheckRefusals reads the refusal names the shared check functions
// reference as contract.<Name>, keeping only the names contract.Declared and
// contract.Introduced list, in sorted order.
func sharedCheckRefusals(t *testing.T) []string {
	t.Helper()
	values := contractConstants(t)
	legal := map[string]bool{}
	for _, name := range append(append([]string{}, contract.Declared...), contract.Introduced...) {
		legal[name] = true
	}
	dir := filepath.Join("..", "..", "internal", "verb")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	names := map[string]bool{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Body == nil || !slices.Contains(sharedCheckFunctions, function.Name.Name) {
				continue
			}
			found[function.Name.Name] = true
			ast.Inspect(function.Body, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok || !isBareIdentCall(selector.X, "contract") {
					return true
				}
				if value, ok := values[selector.Sel.Name]; ok && legal[value] {
					names[value] = true
				}
				return true
			})
		}
	}
	for _, name := range sharedCheckFunctions {
		if !found[name] {
			t.Errorf("the shared check %s was not found in internal/verb", name)
		}
	}
	var sorted []string
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	return sorted
}

// contractConstants reads internal/contract/contract.go's string constants,
// each a literal or LayerPrefix followed by a literal, into their values.
func contractConstants(t *testing.T) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "..", "internal", "contract", "contract.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	literal := func(expr ast.Expr) (string, bool) {
		lit, ok := expr.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(lit.Value)
		return value, err == nil
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
				continue
			}
			if text, ok := literal(value.Values[0]); ok {
				values[value.Names[0].Name] = text
				continue
			}
			sum, ok := value.Values[0].(*ast.BinaryExpr)
			if !ok || !isBareIdentCall(sum.X, "LayerPrefix") {
				continue
			}
			if text, ok := literal(sum.Y); ok {
				values[value.Names[0].Name] = contract.LayerPrefix + text
			}
		}
	}
	if len(values) < 50 {
		t.Fatalf("read %d constants from contract.go, which is fewer than it declares", len(values))
	}
	return values
}
