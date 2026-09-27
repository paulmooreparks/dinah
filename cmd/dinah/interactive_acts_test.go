//go:build tui

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/contract"
)

// actWidth and actHeight are the window every act fixture runs in, wide
// enough that no footer entry is cut.
const (
	actWidth  = 400
	actHeight = 40
)

// actFixture is how TestEveryActIsOfferedWhereItIsReached reaches one row of
// interactiveActs or one entry of the actions menu: the workbench and the
// identity where the library accepts the act, the keys that bring the head to
// where it is reached, the keys that answer its steps, and the effect the act
// has. refuse is the position where the library refuses the act for the
// head's actor, nil for a read, which nothing refuses for want of permission.
type actFixture struct {
	definition string
	actor      string
	arrange    func(t *testing.T, root string)
	reach      string
	steps      string
	effect     func(t *testing.T, root string, run tuiRun)
	refuse     *actRefusal
}

// actRefusal is a position where the library refuses an act for the head's
// actor.
type actRefusal struct {
	definition string
	actor      string
	arrange    func(t *testing.T, root string)
	reach      string
}

// asActor sets the identity a head starts as, the empty actor included.
func asActor(t *testing.T, actor string) {
	t.Helper()
	t.Setenv("DINAH_ACTOR", actor)
	if actor == "" {
		os.Unsetenv("DINAH_ACTOR")
	}
}

// actBench builds the fixture's workbench: the terminal flow, or the
// definition named, with the cards tuiBench files.
func actBench(t *testing.T, definition string) string {
	t.Helper()
	if definition == "" {
		return tuiBench(t)
	}
	root := newBenchFromDefinition(t, definition)
	for _, words := range [][]string{{"add", "Build the parser"}, {"move", "fx-1", "work"}, {"add", "Write the guide"}, {"move", "fx-2", "work"}} {
		step(t, root, words...)
	}
	return root
}

// benchBytes reads every file of the workbench but its locks, by path, which
// is what a refusing fixture holds byte-identical across a press.
func benchBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	dir := soleBenchDir(t, root)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || strings.HasSuffix(entry.Name(), ".lock") {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		files[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// sameBytes compares two reads of a workbench.
func sameBytes(before, after map[string]string) bool {
	if len(before) != len(after) {
		return false
	}
	for path, data := range before {
		if after[path] != data {
			return false
		}
	}
	return true
}

// journaled reports whether a card's journal carries an event.
func journaled(t *testing.T, root, ref, event string) bool {
	t.Helper()
	for _, got := range cardEvents(t, root, ref) {
		if got.Event == event {
			return true
		}
	}
	return false
}

// wantEvent is an effect asserting a card's journal gained an event.
func wantEvent(ref, event string) func(t *testing.T, root string, run tuiRun) {
	return func(t *testing.T, root string, run tuiRun) {
		t.Helper()
		if !journaled(t, root, ref, event) {
			t.Errorf("%s's journal carries no %s event; the head showed %q", ref, event, shownLines(run.model))
		}
	}
}

// wantRead is an effect asserting the head shows exactly the lines the CLI
// prints for the same words on the same workbench at the head's draw width.
func wantRead(words ...string) func(t *testing.T, root string, run tuiRun) {
	return func(t *testing.T, root string, run tuiRun) {
		t.Helper()
		t.Setenv("COLUMNS", "399")
		got := runCLI(t, root, words...)
		want := splitLines(strings.TrimSuffix(got.out+got.errw, "\n"))
		for i := range want {
			want[i] = withoutControls(want[i])
		}
		if shown := shownLines(run.model); strings.Join(shown, "\n") != strings.Join(want, "\n") {
			t.Errorf("the head shows\n%s\nand dinah %s prints\n%s", strings.Join(shown, "\n"), strings.Join(words, " "), strings.Join(want, "\n"))
		}
	}
}

// fixtureFor answers the head's run of keys over a fixture's workbench.
func fixtureRun(t *testing.T, root, actor, keys string) tuiRun {
	t.Helper()
	asActor(t, actor)
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(keys+keyCtrlC), actWidth, actHeight))
	if run.model == nil {
		t.Fatalf("the run of %q never finished: %q", keys, run.errw)
	}
	return run
}

// The item arrangements the item rows and the item entries of the actions
// menu reach, each leaving fx-1 with exactly one item in the state the act
// reads.
var (
	pendingQuestion = func(t *testing.T, root string) {
		step(t, root, "file", "fx-1", "open_question", "Which vendor ships it?")
	}
	operatorQuestion = func(t *testing.T, root string) {
		step(t, root, "file", "fx-1", "open_question", "Which vendor ships it?", "--owner", "operator")
	}
	pendingCriterion = func(t *testing.T, root string) {
		step(t, root, "file", "fx-1", "acceptance_criterion", "The parser reads every line")
	}
	operatorCriterion = func(t *testing.T, root string) {
		step(t, root, "file", "fx-1", "acceptance_criterion", "The parser reads every line", "--owner", "operator")
	}
	resolvedQuestion = func(t *testing.T, root string) {
		pendingQuestion(t, root)
		step(t, root, "resolve", "fx-1/questions/1", "--text", "the usual vendor")
	}
	failedCriterion = func(t *testing.T, root string) {
		pendingCriterion(t, root)
		step(t, root, "fail", "fx-1/criteria/1", "--text", "it dropped a line")
	}
)

// heldBy claims fx-1 for an owner, and blocked blocks it.
func heldBy(owner string) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		step(t, root, "claim", "fx-1", "--actor", owner)
	}
}

// then runs arrangements in order.
func then(arrangements ...func(t *testing.T, root string)) func(t *testing.T, root string) {
	return func(t *testing.T, root string) {
		for _, arrange := range arrangements {
			arrange(t, root)
		}
	}
}

// operatorsColumn is the refusing position of every move out of the
// operator's column: fx-3 in Acceptance, for an agent.
var operatorsColumn = &actRefusal{actor: "brin", reach: "l"}

// noActor is the refusing position of every act the library refuses to a
// request naming no owner.
func noActor(arrange func(t *testing.T, root string), reach string) *actRefusal {
	return &actRefusal{actor: "", arrange: arrange, reach: reach}
}

// actFixtures are the fixtures of TestEveryActIsOfferedWhereItIsReached,
// keyed by a key row's name and by menu:<verb> for every actions-menu entry.
var actFixtures = map[string]actFixture{
	"show": {actor: "alka", steps: "", effect: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the mode", run.model.mode, modeCard)
		wantModel(t, "the card shown", run.model.cardRef, "fx-1")
	}},
	"view": {actor: "alka", steps: "agenda" + keyEnter, effect: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the view", run.model.req.View, "agenda")
	}},
	"query": {actor: "alka", steps: "state:ready" + keyEnter, effect: func(t *testing.T, root string, run tuiRun) {
		wantModel(t, "the filter", run.model.filter, "state:ready")
	}},
	"claim": {actor: "alka", effect: wantEvent("fx-1", contract.EventClaimed),
		refuse: &actRefusal{actor: "brin", arrange: heldBy("cato")}},
	"accept": {actor: "alka", reach: "l", effect: wantEvent("fx-3", contract.EventMoved), refuse: operatorsColumn},
	"advance": {actor: "alka", effect: wantEvent("fx-1", contract.EventMoved),
		refuse: &actRefusal{actor: "brin", arrange: heldBy("cato")}},
	"send-back": {actor: "alka", reach: "l", effect: wantEvent("fx-3", contract.EventMoved), refuse: operatorsColumn},
	"move": {actor: "alka", steps: "1", effect: wantEvent("fx-1", contract.EventMoved),
		refuse: &actRefusal{actor: "brin", arrange: heldBy("cato")}},
	"release": {actor: "alka", arrange: heldBy("alka"), effect: wantEvent("fx-1", contract.EventReleased),
		refuse: &actRefusal{actor: "brin", arrange: heldBy("cato")}},
	"comment": {actor: "alka", steps: "a remark" + keyCtrlD, effect: wantEvent("fx-1", contract.EventCommented),
		refuse: noActor(nil, "")},
	"next":    {actor: "alka", effect: wantRead("next")},
	"status":  {actor: "alka", effect: wantRead("status")},
	"search":  {actor: "alka", steps: "parser" + keyEnter, effect: wantRead("search", "parser")},
	"changes": {actor: "alka", effect: wantRead("changes", "--card", "fx-1")},
	"whoami":  {actor: "alka", effect: wantRead("whoami")},
	"resolve": {actor: "brin", arrange: pendingQuestion, reach: "i", steps: "the usual vendor" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemResolved), refuse: &actRefusal{actor: "brin", arrange: operatorQuestion, reach: "i"}},
	"verify": {actor: "brin", arrange: pendingCriterion, reach: "i", steps: "it read every line" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemVerified), refuse: &actRefusal{actor: "brin", arrange: operatorCriterion, reach: "i"}},
	"fail": {actor: "brin", arrange: pendingCriterion, reach: "i", steps: "it dropped a line" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemFailed), refuse: &actRefusal{actor: "brin", arrange: operatorCriterion, reach: "i"}},
	"waive": {actor: "alka", arrange: pendingCriterion, reach: "i", steps: "not needed here" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemWaived), refuse: &actRefusal{actor: "brin", arrange: pendingCriterion, reach: "i"}},
	"withdraw": {actor: "brin", arrange: pendingQuestion, reach: "i", steps: "it stopped applying" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemWithdrawn), refuse: &actRefusal{actor: "brin", arrange: pendingCriterion, reach: "i"}},
	"reopen": {actor: "brin", arrange: resolvedQuestion, reach: "i", steps: "it was closed wrongly" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemReopened), refuse: &actRefusal{actor: "brin", arrange: failedCriterion, reach: "i"}},
	"cite": {actor: "brin", arrange: pendingCriterion, reach: "i", steps: "cmd/dinah/parser_test.go" + keyEnter + "cmd/dinah/parser_test.go" + keyEnter,
		effect: wantEvent("fx-1", contract.EventItemCited), refuse: noActor(pendingCriterion, "i")},

	"menu:add": {actor: "alka", steps: "A new card" + keyEnter + "1", effect: func(t *testing.T, root string, run tuiRun) {
		if got := runCLI(t, root, "show", "fx-6", "--fields", "card"); got.code != 0 || !strings.Contains(got.out, "A new card") {
			t.Errorf("no card was filed: %s%s", got.out, got.errw)
		}
	}, refuse: noActor(nil, "")},
	"menu:claim": {actor: "alka", effect: wantEvent("fx-1", contract.EventClaimed),
		refuse: &actRefusal{actor: "brin", arrange: heldBy("cato")}},
	"menu:move": {actor: "alka", steps: "1", effect: wantEvent("fx-1", contract.EventMoved), refuse: operatorsColumn},
	"menu:pull": {actor: "alka", arrange: func(t *testing.T, root string) { step(t, root, "add", "Waiting in intake") },
		effect: wantEvent("fx-6", contract.EventClaimed), refuse: &actRefusal{actor: "alka"}},
	"menu:release": {actor: "alka", arrange: heldBy("alka"), effect: wantEvent("fx-1", contract.EventReleased),
		refuse: &actRefusal{actor: "brin", arrange: heldBy("cato")}},
	"menu:block": {actor: "alka", steps: "waiting on a vendor" + keyCtrlD + keyEnter, effect: wantEvent("fx-1", contract.EventBlocked),
		refuse: &actRefusal{actor: "brin", arrange: heldBy("cato")}},
	"menu:unblock": {actor: "alka", arrange: func(t *testing.T, root string) { step(t, root, "block", "fx-1", "waiting") },
		steps: keyCtrlD, effect: wantEvent("fx-1", contract.EventUnblocked),
		refuse: &actRefusal{actor: "brin", arrange: func(t *testing.T, root string) { step(t, root, "block", "fx-1", "waiting") }}},
	"menu:raise": {definition: tieredTUIDefinition, actor: "alka", arrange: heldBy("alka"), steps: "1" + "the work is harder" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventTierOverridden),
		refuse: &actRefusal{definition: tieredTUIDefinition, actor: "brin", arrange: heldBy("cato")}},
	"menu:comment": {actor: "alka", steps: "a remark" + keyCtrlD, effect: wantEvent("fx-1", contract.EventCommented), refuse: noActor(nil, "")},
	"menu:attach":  {actor: "alka", steps: "", effect: wantEvent("fx-1", contract.EventAttached), refuse: noActor(nil, "")},
	"menu:file": {actor: "alka", steps: "2" + "1" + "1" + "Which vendor ships it?" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemFiled), refuse: noActor(nil, "")},
	"menu:cite": {actor: "brin", arrange: pendingCriterion, steps: "1" + "cmd/dinah/parser_test.go" + keyEnter + "cmd/dinah/parser_test.go" + keyEnter,
		effect: wantEvent("fx-1", contract.EventItemCited), refuse: noActor(pendingCriterion, "")},
	"menu:resolve": {actor: "brin", arrange: pendingQuestion, steps: "1" + "the usual vendor" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemResolved), refuse: &actRefusal{actor: "brin", arrange: operatorQuestion}},
	"menu:verify": {actor: "brin", arrange: pendingCriterion, steps: "1" + "it read every line" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemVerified), refuse: &actRefusal{actor: "brin", arrange: operatorCriterion}},
	"menu:fail": {actor: "brin", arrange: pendingCriterion, steps: "1" + "it dropped a line" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemFailed), refuse: &actRefusal{actor: "brin", arrange: operatorCriterion}},
	"menu:waive": {actor: "alka", arrange: pendingCriterion, steps: "1" + "not needed here" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemWaived), refuse: &actRefusal{actor: "brin", arrange: pendingCriterion}},
	"menu:withdraw": {actor: "brin", arrange: pendingQuestion, steps: "1" + "it stopped applying" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemWithdrawn), refuse: &actRefusal{actor: "brin", arrange: pendingCriterion}},
	"menu:reopen": {actor: "brin", arrange: resolvedQuestion, steps: "1" + "it was closed wrongly" + keyCtrlD,
		effect: wantEvent("fx-1", contract.EventItemReopened), refuse: &actRefusal{actor: "brin", arrange: failedCriterion}},
	"menu:grant": {actor: "alka", steps: "1", effect: wantEvent("fx-1", contract.EventRetirementGranted), refuse: &actRefusal{actor: "brin"}},
	"menu:revoke": {actor: "alka", arrange: func(t *testing.T, root string) { step(t, root, "grant", "fx-1", "criterion-retirement") },
		steps: "1", effect: wantEvent("fx-1", contract.EventRetirementRevoked),
		refuse: &actRefusal{actor: "brin", arrange: func(t *testing.T, root string) { step(t, root, "grant", "fx-1", "criterion-retirement") }}},
	"menu:link": {actor: "alka", steps: "relates_to" + keyEnter + "fx-2" + keyEnter, effect: wantEvent("fx-1", contract.EventLinked), refuse: noActor(nil, "")},
	"menu:unlink": {actor: "alka", arrange: func(t *testing.T, root string) { step(t, root, "link", "fx-1", "relates_to", "fx-2") },
		steps: "1", effect: wantEvent("fx-1", contract.EventUnlinked),
		refuse: noActor(func(t *testing.T, root string) { step(t, root, "link", "fx-1", "relates_to", "fx-2") }, "")},
	"menu:join": {actor: "alka", arrange: func(t *testing.T, root string) { step(t, root, "workstream", "new", "Parser", "--slug", "parser") },
		steps: "1", effect: wantEvent("fx-1", contract.EventWorkstreamJoined),
		refuse: noActor(func(t *testing.T, root string) { step(t, root, "workstream", "new", "Parser", "--slug", "parser") }, "")},
	"menu:leave": {actor: "alka", arrange: func(t *testing.T, root string) {
		step(t, root, "workstream", "new", "Parser", "--slug", "parser")
		step(t, root, "join", "fx-1", "parser")
	}, steps: "1", effect: wantEvent("fx-1", contract.EventWorkstreamLeft), refuse: noActor(func(t *testing.T, root string) {
		step(t, root, "workstream", "new", "Parser", "--slug", "parser")
		step(t, root, "join", "fx-1", "parser")
	}, "")},
	"menu:archive": {actor: "alka", steps: "1", effect: func(t *testing.T, root string, run tuiRun) {
		if got := runCLI(t, root, "show", "fx-1", "--archived", "--fields", "card"); got.code != 0 {
			t.Errorf("fx-1 is not in the archive: %s", got.errw)
		}
	}, refuse: noActor(nil, "")},
	"menu:restore": {actor: "alka", arrange: func(t *testing.T, root string) { step(t, root, "archive", "fx-1") }, reach: ":fx-1" + keyEnter,
		effect: func(t *testing.T, root string, run tuiRun) {
			if got := runCLI(t, root, "show", "fx-1", "--fields", "card"); got.code != 0 {
				t.Errorf("fx-1 is not live again: %s", got.errw)
			}
		}, refuse: noActor(func(t *testing.T, root string) { step(t, root, "archive", "fx-1") }, ":fx-1"+keyEnter)},
	"menu:delete": {actor: "alka", steps: "1", effect: func(t *testing.T, root string, run tuiRun) {
		if got := runCLI(t, root, "show", "fx-1", "--fields", "card"); got.code == 0 {
			t.Error("fx-1 is still live")
		}
	}, refuse: noActor(nil, "")},
	"menu:accept-divergence": {actor: "alka", arrange: editedComment, steps: "1",
		effect: wantEvent("fx-1", contract.EventDivergenceAccepted), refuse: noActor(editedComment, "")},
	"menu:rename": {actor: "alka", arrange: attachedFile, steps: "1" + "renamed.txt" + keyEnter,
		effect: wantEvent("fx-1", contract.EventAttachmentRenamed), refuse: noActor(attachedFile, "")},
	"menu:set": {actor: "alka", steps: "1" + "A new title" + keyCtrlD, effect: wantEvent("fx-1", contract.EventCardUpdated), refuse: noActor(nil, "")},
	"menu:edit": {actor: "alka", effect: func(t *testing.T, root string, run tuiRun) {
		if launches := editorLaunches(t, os.Getenv(editorRecordVar)); len(launches) != 1 {
			t.Errorf("the editor was launched %d times: %v", len(launches), launches)
		}
	}, refuse: &actRefusal{actor: "alka", arrange: func(t *testing.T, root string) { step(t, root, "archive", "fx-1") }, reach: ":fx-1" + keyEnter}},
}

// editedComment leaves fx-1 with a comment whose body was edited outside the
// tool.
func editedComment(t *testing.T, root string) {
	t.Helper()
	step(t, root, "comment", "fx-1", "about to be edited")
	path := strings.TrimSpace(runCLI(t, root, "path", "fx-1/comments/1").out)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "about to be edited", "edited by hand", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// attachedFile leaves fx-1 with one attachment.
func attachedFile(t *testing.T, root string) {
	t.Helper()
	step(t, root, "attach", "fx-1", offerAttachment(t))
}

// fixtureNames are the names the act table and the actions menu call for:
// every row's name and menu:<verb> for every entry.
func fixtureNames() []string {
	var names []string
	for _, row := range interactiveActs {
		names = append(names, row.name)
	}
	for name := range actionsMenu {
		names = append(names, "menu:"+name)
	}
	sort.Strings(names)
	return names
}

// TestEveryActIsOfferedWhereItIsReached is dinah-623/criteria/3 and /4. For
// every row of interactiveActs and every entry of the actions menu, a fixture
// where the library accepts the act shows the row's key in its mode's footer
// or the entry's row in the actions menu, and pressing the key or choosing
// the row and answering its steps has the act's effect: the verb's journal
// event, or for a read what the read answers. For every writing row and every
// entry, a fixture where the library refuses the act shows neither, and
// pressing the key or the number the row would have had leaves every file of
// the workbench byte-identical. The fixture table's names equal the table's
// and the menu's.
func TestEveryActIsOfferedWhereItIsReached(t *testing.T) {
	t.Setenv("DINAH_EDITOR", os.Args[0])
	names := fixtureNames()
	var fixtured []string
	for name := range actFixtures {
		fixtured = append(fixtured, name)
	}
	sort.Strings(fixtured)
	if strings.Join(names, " ") != strings.Join(fixtured, " ") {
		t.Fatalf("the fixtures are keyed\n%v\nand the act table and the actions menu call for\n%v", fixtured, names)
	}
	shown, refused := 0, 0
	for _, name := range names {
		fixture := actFixtures[name]
		t.Run(name, func(t *testing.T) {
			t.Setenv(editorRecordVar, filepath.Join(t.TempDir(), "editor.log"))
			shown++
			checkAccepted(t, name, fixture)
			if fixture.refuse == nil {
				return
			}
			refused++
			checkRefused(t, name, fixture.refuse)
		})
	}
	t.Logf("%d fixtures offered and performed their act, %d refused theirs", shown, refused)
	if shown == 0 || refused == 0 {
		t.Error("no fixture ran, so this test proves nothing")
	}
}

// actKeyOf is the help key and description a key row's binding draws in the
// footer, in the head's own language.
func actKeyOf(m *interactiveModel, name string) string {
	row, _ := actNamed(name)
	help := row.binding(m.keys()).Help()
	return help.Key + " " + help.Desc
}

// menuLabel is the row the actions menu draws for a verb.
func menuLabel(m *interactiveModel, verb string) string {
	return withoutControls(m.s.r.T("interactive.actions."+verb, "column", m.pullColumnTitle()))
}

// menuIndex is where the open actions menu lists a verb, -1 where it does not.
func menuIndex(m *interactiveModel, verb string) int {
	if m.menu == nil {
		return -1
	}
	label := menuLabel(m, verb)
	for i, row := range m.menu.rows {
		if row.label == label {
			return i
		}
	}
	return -1
}

// checkAccepted runs a fixture's accepting position twice: once to read what
// the head offers there, and once to perform the act.
func checkAccepted(t *testing.T, name string, fixture actFixture) {
	t.Helper()
	root := actBench(t, fixture.definition)
	if fixture.arrange != nil {
		fixture.arrange(t, root)
	}
	verb, isMenu := strings.CutPrefix(name, "menu:")
	look := fixture.reach
	if isMenu {
		look += "x"
	}
	seen := fixtureRun(t, root, fixture.actor, look)
	act := fixture.reach
	if isMenu {
		index := menuIndex(seen.model, verb)
		if index < 0 {
			t.Fatalf("the actions menu does not list %q: %v", menuLabel(seen.model, verb), seen.model.menu)
		}
		act += "x" + strings.Repeat("j", index) + keyEnter
	} else {
		footer := seen.model.footerText()
		if want := actKeyOf(seen.model, name); !strings.Contains(footer, want) {
			t.Fatalf("the footer does not offer %q:\n%s", want, footer)
		}
		row, _ := actNamed(name)
		act += row.binding(seen.model.keys()).Keys()[0]
		if name == "show" {
			act = fixture.reach + keyEnter
		}
	}
	if name == "menu:attach" {
		act += offerAttachment(t) + keyEnter + keyEnter
	}
	if name == "menu:edit" {
		fixture.effect(t, root, lendRun(t, root, fixture.actor, act))
		return
	}
	run := fixtureRun(t, root, fixture.actor, act+fixture.steps)
	fixture.effect(t, root, run)
}

// checkRefused runs a fixture's refusing position: once to read that the head
// offers nothing there, and once to press the key, or the number the entry
// would have had, and hold the workbench byte-identical.
func checkRefused(t *testing.T, name string, refusal *actRefusal) {
	t.Helper()
	root := actBench(t, refusal.definition)
	if refusal.arrange != nil {
		refusal.arrange(t, root)
	}
	verb, isMenu := strings.CutPrefix(name, "menu:")
	look := refusal.reach
	if isMenu {
		look += "x"
	}
	seen := fixtureRun(t, root, refusal.actor, look)
	press := refusal.reach
	if isMenu {
		if index := menuIndex(seen.model, verb); index >= 0 {
			t.Errorf("the actions menu lists %q where the library refuses it", menuLabel(seen.model, verb))
		}
		press += "x"
		if number := wouldBeNumber(verb); number > 0 && number <= 9 && digitIsSafe(seen.model, number) {
			press += string(rune('0' + number))
		}
	} else {
		footer := seen.model.footerText()
		if want := actKeyOf(seen.model, name); strings.Contains(footer, want) {
			t.Errorf("the footer offers %q where the library refuses it:\n%s", want, footer)
		}
		row, _ := actNamed(name)
		press += row.binding(seen.model.keys()).Keys()[0]
	}
	before := benchBytes(t, root)
	fixtureRun(t, root, refusal.actor, press+keyCtrlG+keyCtrlG)
	if !sameBytes(before, benchBytes(t, root)) {
		t.Errorf("pressing %q where the library refuses %s changed the workbench", press, name)
	}
}

// wouldBeNumber is the number an entry would have had in the actions menu
// were every card verb listed: its place among the commands setting
// actsOnCard, in the command table's order.
func wouldBeNumber(verb string) int {
	number := 0
	for _, c := range commands {
		if !c.actsOnCard {
			continue
		}
		number++
		if c.name == verb {
			return number
		}
	}
	return 0
}

// digitIsSafe reports whether pressing a digit in the open actions menu can
// only start an act that asks a step first, which Ctrl+G then cancels: the
// digit chooses no row, or a row whose entry gathers an argument before it
// runs. A row that acts at once is left unpressed, since pressing it would
// perform another verb than the one refused.
func digitIsSafe(m *interactiveModel, number int) bool {
	if m.menu == nil || number > len(m.menu.rows) {
		return true
	}
	chosen := actionsMenu[m.menu.rows[number-1].value]
	return len(chosen.steps) > 0
}

// lendRun runs keys that lend the terminal, then waits for the cycle after
// the lend before quitting, since keys typed before the lend belong to the
// program the lend ended.
func lendRun(t *testing.T, root, actor, keys string) tuiRun {
	t.Helper()
	asActor(t, actor)
	s, seam := newScript(t, actWidth, actHeight, true)
	cycles := make(chan *tea.Program, 8)
	seam.program = func(program *tea.Program) { cycles <- program }
	run := s.run(root, seam, func() {
		waitForProgram(t, cycles)
		s.write(keys)
		waitForProgram(t, cycles)
		// The cycle after a lend discards the keys typed during it, so a key
		// is written only once its flush has been confirmed.
		s.waitFor("the flush after the lend", isFlushed)
		s.write(keyCtrlC)
	})
	if run.model == nil {
		t.Fatalf("the lend run never finished: %q", run.errw)
	}
	return run
}

// isFlushed accepts the confirmation of a flush.
func isFlushed(msg tea.Msg) bool {
	_, ok := msg.(flushedMsg)
	return ok
}

// waitForProgram waits for the next program a cycle starts.
func waitForProgram(t *testing.T, cycles chan *tea.Program) *tea.Program {
	t.Helper()
	select {
	case program := <-cycles:
		return program
	case <-time.After(tuiWait):
		t.Error("no cycle started")
		return nil
	}
}
