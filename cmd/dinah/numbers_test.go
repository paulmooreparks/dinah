package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// The tests in this file hold the card-number registry's observable contract
// from the command line: a filing writes the file and nothing else, the
// repair moves the later claimant of a number two lines claim and leaves the
// file's order alone, a workbench predating the registry is still read and
// still refuses to allocate, both number repairs refuse without their
// confirmation, and every reference a command prints follows the registry
// rather than anything a card carries.

// TestFilingACardAppendsOneRegistryLine asserts the two halves of what a
// filing writes: exactly one line lands in the registry, and no number lands
// in the card's own anchor. The number lives in the file and nowhere else,
// which is the whole of what the registry changes, so both halves are
// asserted over the bytes the command wrote rather than over the reading of
// them. A belt-and-braces implementation that wrote the number into the
// frontmatter as well as the registry fails the second half rather than
// passing both. The line a filing writes is the creation ordinal CORE-CARD-10
// says every card carries, and the renumber test below drives the other half
// of the statement: that no other card in the workbench carries the same one.
func TestFilingACardAppendsOneRegistryLine(t *testing.T) {
	root := newBench(t)
	workbench := soleBenchDir(t, root)
	registry := filepath.Join(workbench, bench.CardNumbersName)
	if bench.Exists(registry) {
		t.Fatalf("init wrote %s, and the registry belongs to the first filing rather than to the workbench itself", bench.CardNumbersName)
	}
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s%s", got.code, got.out, got.errw)
	}
	text, err := os.ReadFile(registry)
	if err != nil {
		t.Fatalf("the filing wrote no registry: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(text), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("the filing wrote %d lines, wanted exactly one:\n%s", len(lines), text)
	}
	if !regexp.MustCompile(`^[1-9][0-9]* [0-9a-f]{12}$`).MatchString(lines[0]) {
		t.Fatalf("the line the filing wrote does not read as one number and one identifier: %q", lines[0])
	}
	ids, err := bench.ListIDs(filepath.Join(workbench, bench.CardsDir))
	if err != nil {
		t.Fatalf("listing %s: %v", filepath.Join(workbench, bench.CardsDir), err)
	}
	if len(ids) != 1 {
		t.Fatalf("the workbench holds %d cards, wanted the one just filed", len(ids))
	}
	if fields := strings.Fields(lines[0]); len(fields) != 2 || fields[1] != ids[0] {
		t.Errorf("the line the filing wrote does not name the card it filed: %q against %v", lines[0], ids)
	}
	anchor, err := os.ReadFile(filepath.Join(workbench, bench.CardsDir, ids[0], bench.CardAnchor))
	if err != nil {
		t.Fatalf("reading the anchor the filing wrote: %v", err)
	}
	for _, line := range strings.Split(string(anchor), "\n") {
		if strings.HasPrefix(line, "number:") {
			t.Errorf("the card's anchor carries a number key, and the registry is not the only place the number lives:\n%s", anchor)
		}
	}
}

// duplicateNumberFixture builds the one registry state no writer of this
// format produces: three cards whose filings took 1, 2 and 3, with the
// second line's number hand-edited onto the first line's. Two careless
// clones or a bad hand edit leave this shape behind, and it is the state the
// renumber repair exists for. It answers the workbench and the three card
// identifiers in file order, which is allocation order.
func duplicateNumberFixture(t *testing.T) (string, []string) {
	t.Helper()
	container := newBench(t)
	root := soleBenchDir(t, container)
	for _, title := range []string{"first card", "second card", "third card"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %q: %d %s%s", title, got.code, got.out, got.errw)
		}
	}
	registry := filepath.Join(root, bench.CardNumbersName)
	text, err := os.ReadFile(registry)
	if err != nil {
		t.Fatalf("reading the registry three filings built: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(text), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("three filings wrote %d lines, wanted three:\n%s", len(lines), text)
	}
	var ids []string
	for _, line := range lines {
		ids = append(ids, strings.Fields(line)[1])
	}
	rewriteFile(t, registry, func(text string) string {
		return strings.Replace(text, "2 "+ids[1], "1 "+ids[1], 1)
	})
	return root, ids
}

// TestRenumberRepairsTheLaterClaimant asserts the repair over the state the
// tool refuses to mint itself. Line order is the only record of who claimed
// first, so the earlier line keeps the number, the later line is rewritten
// where it stands with the next number above the high-water mark, and no
// other line moves. The high-water mark over this fixture is 3, the highest
// number the file carries, so the later claimant takes 4, which is neither
// the number it claimed nor any number another line holds: a repair that
// renumbered by some quieter rule, or that appended rather than rewrote in
// place, cannot pass by coincidence. The repair restores the uniqueness
// CORE-CARD-10 states, and the clean check at the end is what makes the
// restored state visible rather than assumed.
func TestRenumberRepairsTheLaterClaimant(t *testing.T) {
	root, ids := duplicateNumberFixture(t)
	registry := filepath.Join(root, bench.CardNumbersName)

	// The duplicate is a defect check reports before the repair, so the
	// repair is held to have cleared something rather than merely to have
	// run, and the no-duplicate half below has a red state to come from.
	saw := runCLI(t, root, "--json", "check")
	if saw.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Fatalf("check over the duplicate exited %d, wanted the findings code:\n%s%s", saw.code, saw.out, saw.errw)
	}
	var before verb.CheckReport
	if err := json.Unmarshal([]byte(saw.out), &before); err != nil {
		t.Fatalf("decode the machine form: %v\n%s", err, saw.out)
	}
	sawDuplicate := false
	for _, finding := range before.Findings {
		if finding.Key == bench.FindingCardNumberDuplicate {
			sawDuplicate = true
		}
	}
	if !sawDuplicate {
		t.Fatalf("check did not see two lines claiming one number: %+v", before.Findings)
	}

	repaired := runCLI(t, root, "--json", "check", "--renumber", "--yes")
	if repaired.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Fatalf("the repair exited %d, wanted the findings code its renumbered finding carries:\n%s%s", repaired.code, repaired.out, repaired.errw)
	}
	var report verb.CheckReport
	if err := json.Unmarshal([]byte(repaired.out), &report); err != nil {
		t.Fatalf("decode the machine form: %v\n%s", err, repaired.out)
	}
	renamed := false
	for _, finding := range report.Findings {
		if finding.Key == bench.FindingCardNumberRenumbered && finding.Detail == ids[1] {
			renamed = true
		}
	}
	if !renamed {
		t.Errorf("the report carries no %s finding naming the later claimant %s: %+v", bench.FindingCardNumberRenumbered, ids[1], report.Findings)
	}

	text, err := os.ReadFile(registry)
	if err != nil {
		t.Fatalf("reading the registry after the repair: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(text), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("the repair left %d lines, wanted the three it stood on:\n%s", len(lines), text)
	}
	if lines[0] != "1 "+ids[0] {
		t.Errorf("the earlier claimant's line did not keep its number:\n%s", text)
	}
	if lines[1] != "4 "+ids[1] {
		t.Errorf("the later claimant was not rewritten in place with the next number above the high-water mark of 3:\n%s", text)
	}
	if lines[2] != "3 "+ids[2] {
		t.Errorf("the line no repair reached did not stay at its index with its number:\n%s", text)
	}

	// The moved card's journal says what happened to it, where history keeps
	// it, and the two cards that kept their numbers carry no such event.
	moved, err := os.ReadFile(filepath.Join(root, bench.CardsDir, ids[1], bench.JournalName))
	if err != nil {
		t.Fatalf("reading the renumbered card's journal: %v", err)
	}
	for _, want := range []string{`"event":"renumbered"`, `"from":"1"`, `"to":"4"`} {
		if !strings.Contains(string(moved), want) {
			t.Errorf("the renumbered card's journal carries no %s:\n%s", want, moved)
		}
	}
	for _, at := range []int{0, 2} {
		still, err := os.ReadFile(filepath.Join(root, bench.CardsDir, ids[at], bench.JournalName))
		if err != nil {
			t.Fatalf("reading a journal the repair did not reach: %v", err)
		}
		if strings.Contains(string(still), "renumbered") {
			t.Errorf("the journal of the card that kept its number %s carries a renumbered event:\n%s", ids[at], still)
		}
	}

	// The surface a person reads says the same thing, so a reader never has
	// to open the journal to learn why a card answers to a number it was not
	// born with: log draws the act with the numbers the event carries, and
	// the card that kept its number draws no such line.
	logged := runCLI(t, root, "log", ids[1])
	if logged.code != 0 {
		t.Fatalf("log on the renumbered card: %d %s", logged.code, logged.errw)
	}
	if flat := flattenWords(logged.out); !strings.Contains(flat, "renumbered") || !strings.Contains(flat, "1 to 4") {
		t.Errorf("the log does not draw the renumbered act with its numbers:\n%s", logged.out)
	}
	unmoved := runCLI(t, root, "log", ids[0])
	if unmoved.code != 0 {
		t.Fatalf("log on the card that kept its number: %d %s", unmoved.code, unmoved.errw)
	}
	if flat := flattenWords(unmoved.out); strings.Contains(flat, "renumbered") {
		t.Errorf("the log of the card that kept its number carries a renumbered line:\n%s", unmoved.out)
	}

	after := runCLI(t, root, "check")
	if after.code != 0 {
		t.Fatalf("a plain check after the repair exited %d, wanted clean:\n%s%s", after.code, after.out, after.errw)
	}
	if !strings.Contains(after.out, msg.For(msg.Base).T("check.clean")) {
		t.Errorf("the check after the repair does not report the workbench clean, so the duplicate survives:\n%s", after.out)
	}
}

// TestAnUnmigratedWorkbenchReadsAndRefusesToAllocate asserts the shape the
// compatibility claim keeps: a workbench that predates the registry is read
// through the number key its cards still carry in frontmatter, every read
// answers the same references it answered before the registry existed, and
// the one act that refuses is the one that would allocate, because the
// registry a filing allocates from is the half of the format that workbench
// has not reached.
func TestAnUnmigratedWorkbenchReadsAndRefusesToAllocate(t *testing.T) {
	_, _, workbench := preNumberRegistryFixture(t)

	shown := runCLI(t, workbench, "show", "fx-1")
	if shown.code != 0 {
		t.Fatalf("show over a workbench below the registry's format: %d %s%s", shown.code, shown.out, shown.errw)
	}
	if !strings.Contains(shown.out, "fx-1") {
		t.Errorf("show resolved the card but did not print its reference:\n%s", shown.out)
	}

	listed := runCLI(t, workbench, "ls")
	if listed.code != 0 {
		t.Fatalf("ls over the same workbench: %d %s%s", listed.code, listed.out, listed.errw)
	}
	if !strings.Contains(listed.out, "fx-1") {
		t.Errorf("ls did not print the reference it printed before the registry existed:\n%s", listed.out)
	}

	refused := runCLI(t, workbench, "add", "another card")
	if refused.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("filing into a workbench below the registry's format exited %d, wanted the refusal code:\n%s%s", refused.code, refused.out, refused.errw)
	}
	if !strings.Contains(refused.errw, contract.NeedsNumberMigration) {
		t.Errorf("the refusal is not %s:\n%s", contract.NeedsNumberMigration, refused.errw)
	}
	if !strings.Contains(refused.errw, "dinah check --migrate-numbers --yes") {
		t.Errorf("the refusal's next step does not name the command that builds the registry:\n%s", refused.errw)
	}
}

// TestTheNumberRepairsRefuseWithoutAConfirmation asserts the gate both
// number repairs read. They change what a card is called, so a reference
// somebody wrote down stops resolving, and unlike the sweep repairs, which
// answer a preview an unconfirmed run reaches, both refuse outright and
// write nothing. Each half runs over a workbench carrying something real to
// write, so the nothing-written assertion cannot pass by having nothing to
// do.
func TestTheNumberRepairsRefuseWithoutAConfirmation(t *testing.T) {
	t.Run("renumber", func(t *testing.T) {
		root, _ := duplicateNumberFixture(t)
		registry := filepath.Join(root, bench.CardNumbersName)
		before, err := os.ReadFile(registry)
		if err != nil {
			t.Fatalf("reading the registry the refusal is held over: %v", err)
		}
		refused := runCLI(t, root, "check", "--renumber")
		if refused.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Fatalf("the unconfirmed repair exited %d, wanted the refusal code:\n%s%s", refused.code, refused.out, refused.errw)
		}
		if !strings.HasPrefix(refused.errw, contract.Unconfirmed+" ") {
			t.Fatalf("the refusal is not led by %s:\n%s", contract.Unconfirmed, refused.errw)
		}
		if !strings.Contains(refused.errw, msg.For(msg.Base).T("refusal.dinah.unconfirmed.check", "detail", "--renumber")) {
			t.Errorf("the refusal does not say why a card-number repair wants confirmation:\n%s", refused.errw)
		}
		if !strings.Contains(refused.errw, msg.For(msg.Base).T("refusal.dinah.unconfirmed.check.next")) {
			t.Errorf("the refusal does not tell the reader to run the command again with --yes:\n%s", refused.errw)
		}
		after, err := os.ReadFile(registry)
		if err != nil {
			t.Fatalf("reading the registry after the refusal: %v", err)
		}
		if string(after) != string(before) {
			t.Errorf("the refused run wrote to the registry:\n%s", after)
		}
	})
	t.Run("migrate-numbers", func(t *testing.T) {
		_, _, workbench := preNumberRegistryFixture(t)
		refused := runCLI(t, workbench, "check", "--migrate-numbers")
		if refused.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Fatalf("the unconfirmed migration exited %d, wanted the refusal code:\n%s%s", refused.code, refused.out, refused.errw)
		}
		if !strings.HasPrefix(refused.errw, contract.Unconfirmed+" ") {
			t.Fatalf("the refusal is not led by %s:\n%s", contract.Unconfirmed, refused.errw)
		}
		if !strings.Contains(refused.errw, msg.For(msg.Base).T("refusal.dinah.unconfirmed.check", "detail", "--migrate-numbers")) {
			t.Errorf("the refusal does not say why a card-number repair wants confirmation:\n%s", refused.errw)
		}
		if !strings.Contains(refused.errw, msg.For(msg.Base).T("refusal.dinah.unconfirmed.check.next")) {
			t.Errorf("the refusal does not tell the reader to run the command again with --yes:\n%s", refused.errw)
		}
		if bench.Exists(filepath.Join(workbench, bench.CardNumbersName)) {
			t.Errorf("the refused run built the registry, and the reader's consent changed nothing")
		}
	})
}

// The two collection patterns the registry criterion fixes, kept verbatim so
// a later hand widening one of them is a change to the criterion rather than
// to a private spelling. The reference pattern anchors on a whole run of
// digits with a non-word character before the fx, so an identifier, a date
// and a per-column count cannot enter the set by accident. The identifier
// pattern collects bare 12-hex identifiers on the same anchoring, which is
// the second half of what the registry fixes: a command printing one where a
// reference belongs is a caller nobody can cite back.
var (
	cardReferencePattern  = regexp.MustCompile(`(^|[^0-9A-Za-z_-])fx-([0-9]+)`)
	bareIdentifierPattern = regexp.MustCompile(`(^|[^0-9A-Za-z_-])([0-9a-f]{12})([^0-9a-f]|$)`)
)

// cardReferences reads every reference match out of one run's combined
// streams as a set of integers, so two answers that differ only in the order
// they printed their cards compare equal, which is the scope the criterion
// fixes: it says which references a command prints, and nothing about the
// order it prints them in.
func cardReferences(t *testing.T, got invocation) map[int]bool {
	t.Helper()
	collected := map[int]bool{}
	for _, match := range cardReferencePattern.FindAllStringSubmatch(got.out+got.errw, -1) {
		number, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatalf("the digits the reference pattern matched do not read as a number: %q", match[2])
		}
		collected[number] = true
	}
	return collected
}

// bareIdentifiers reads every bare 12-hex identifier out of the same
// combined streams. The slice rather than the set keeps a count a reader can
// compare, though no assertion below cares how many there are, only that
// there are none.
func bareIdentifiers(t *testing.T, got invocation) []string {
	t.Helper()
	var collected []string
	for _, match := range bareIdentifierPattern.FindAllStringSubmatch(got.out+got.errw, -1) {
		collected = append(collected, match[2])
	}
	return collected
}

// spellNumbers renders a set of numbers for a failure message, ascending, so
// a red test reads at a glance what a command printed against what it was
// wanted to print.
func spellNumbers(collected map[int]bool) string {
	var numbers []int
	for number := range collected {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	parts := make([]string, len(numbers))
	for at, number := range numbers {
		parts[at] = strconv.Itoa(number)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// TestEveryCardReferenceComesFromTheRegistry holds the criterion that closes
// the design: every card reference a command prints is read out of the
// registry, and no command prints a bare card identifier where a reference
// belongs. The registry is rewritten by hand so every number gains 1000, and
// the seven commands are run over the workbench before and after the
// rewrite. A build that reads a number from anywhere but the file keeps
// printing 1, 2 and 3 after the rewrite, and the disjointness assertion
// catches it before the per-command sets are even read.
//
// The offset is what makes this an attack rather than a reading. Nothing
// inside a card changes across the rewrite, so a command whose references
// follow the file moves and a command whose references follow anything else
// stays still, and the difference between the two is the whole test.
func TestEveryCardReferenceComesFromTheRegistry(t *testing.T) {
	container := newBench(t)
	root := soleBenchDir(t, container)
	for _, title := range []string{"first card", "second card", "third card"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %q: %d %s%s", title, got.code, got.out, got.errw)
		}
	}
	// The criterion names the migration, and on a build whose filings write
	// the registry it re-reads the file and writes nothing, which is the
	// idempotence the migration's own criterion holds. Running it keeps the
	// setup honest on a build whose filings do not, and costs one command.
	if got := runCLI(t, root, "check", "--migrate-numbers", "--yes"); got.code != 0 {
		t.Fatalf("the migration the criterion names: %d %s%s", got.code, got.out, got.errw)
	}

	// The pre-offset commands are the same seven, with show naming the card
	// by the reference it answers to at this point. Their collections union
	// into the pre-offset set the first assertion reads.
	commands := func(show string) []struct {
		name string
		argv []string
	} {
		return []struct {
			name string
			argv []string
		}{
			{"ls", []string{"ls"}},
			{"next", []string{"next"}},
			{"search card", []string{"search", "card"}},
			{"tree", []string{"tree"}},
			{"show", []string{"show", show}},
			{"status", []string{"status"}},
			{"check", []string{"check"}},
		}
	}
	before := map[string]invocation{}
	for _, command := range commands("fx-2") {
		got := runCLI(t, root, command.argv...)
		if got.code == contract.ExitCode(contract.OutcomeRefused) && command.name != "check" {
			t.Fatalf("%s refused before the rewrite: %d %s%s", command.name, got.code, got.out, got.errw)
		}
		before[command.name] = got
	}

	registry := filepath.Join(root, bench.CardNumbersName)
	rewriteFile(t, registry, func(text string) string {
		var rebuilt []string
		for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
			fields := strings.Fields(line)
			number, err := strconv.Atoi(fields[0])
			if err != nil {
				t.Fatalf("the registry line %q does not read as a number and an identifier", line)
			}
			rebuilt = append(rebuilt, strconv.Itoa(number+1000)+" "+fields[1])
		}
		return strings.Join(rebuilt, "\n") + "\n"
	})

	// The offset moves every reference, so the acts that follow name cards
	// by their new references. A claim standing at an intake column is
	// refused, so the first card is carried to the work column first, which
	// is what the criterion's setup does and why.
	carryToDoing(t, root, "fx-1001")
	if got := runCLI(t, root, "claim", "fx-1001"); got.code != 0 {
		t.Fatalf("claim by the new reference: %d %s%s", got.code, got.out, got.errw)
	}
	// The open question is filed carrying a real gate column, and then that
	// column is rewritten on disk to one the workbench does not carry, so
	// check has exactly one defect to report and names the third card doing
	// it. The card itself stays where it was filed.
	if got := runCLI(t, root, "file", "fx-1003", "open_question", "a question the operator answers", "--column", "doing"); got.code != 0 {
		t.Fatalf("filing the open question: %d %s%s", got.code, got.out, got.errw)
	}
	// The third card is the third line of the registry, because line order
	// is allocation order and the rewrite only changed the numbers, so the
	// line's identifier is the card that took number 3 before it took 1003.
	numbered := strings.Split(strings.TrimRight(mustRead(t, registry), "\n"), "\n")
	if len(numbered) != 3 {
		t.Fatalf("the registry holds %d lines, wanted the three filings:\n%s", len(numbered), strings.Join(numbered, "\n"))
	}
	third := strings.Fields(numbered[2])[1]
	items, err := bench.ListIDs(filepath.Join(root, bench.CardsDir, third, bench.ChecklistDir))
	if err != nil {
		t.Fatalf("listing the third card's checklist: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("the third card carries %d checklist items, wanted the one just filed", len(items))
	}
	rewriteFile(t, filepath.Join(root, bench.CardsDir, third, bench.ChecklistDir, items[0], bench.ItemAnchor), func(text string) string {
		var rebuilt []string
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "column:") {
				line = "column: deadbeefdead"
			}
			rebuilt = append(rebuilt, line)
		}
		return strings.Join(rebuilt, "\n")
	})

	after := map[string]invocation{}
	for _, command := range commands("fx-1002") {
		after[command.name] = runCLI(t, root, command.argv...)
	}

	// The first assertion, before anything else: the numbers the commands
	// print after the rewrite and the numbers the same commands printed
	// before it share nothing. A later hand shrinking the offset toward
	// nothing makes the two sets meet here, and the exact sets below stay
	// red rather than going vacuous.
	preOffset := map[int]bool{}
	for _, got := range before {
		for number := range cardReferences(t, got) {
			preOffset[number] = true
		}
	}
	offset := map[int]bool{}
	for _, got := range after {
		for number := range cardReferences(t, got) {
			offset[number] = true
		}
	}
	for number := range offset {
		if preOffset[number] {
			t.Fatalf("the number %d appears both before and after the offset, so some reference is not following the registry:\nbefore %s\nafter %s", number, spellNumbers(preOffset), spellNumbers(offset))
		}
	}

	// The exact sets, per command. The claim sits on the first card, the
	// second card heads the intake queue, and the third card's defect is the
	// one finding check reports. Equality rather than containment, because
	// a command printing one reference too many is half this criterion's
	// complaint.
	wanted := map[string]map[int]bool{
		"ls":          {1001: true, 1002: true, 1003: true},
		"next":        {1002: true},
		"search card": {1001: true, 1002: true, 1003: true},
		"tree":        {1001: true, 1002: true, 1003: true},
		"show":        {1002: true},
		"status":      {1001: true},
		"check":       {1003: true},
	}
	for name, want := range wanted {
		got := cardReferences(t, after[name])
		if len(got) != len(want) {
			t.Errorf("%s collected %s, wanted %s", name, spellNumbers(got), spellNumbers(want))
			continue
		}
		for number := range want {
			if !got[number] {
				t.Errorf("%s collected %s, wanted %s", name, spellNumbers(got), spellNumbers(want))
				break
			}
		}
	}

	// The second half, over the same runs: the six commands other than
	// check print no bare card identifier at all, before the rewrite and
	// after it. Check is excluded because its finding legitimately carries
	// the item's anchor path, the checklist item's own identifier, and the
	// column value the defect is about, and each of those is a 12-hex run
	// the sentence owes the reader.
	for _, phase := range []struct {
		label string
		runs  map[string]invocation
	}{
		{"before the offset", before},
		{"after the offset", after},
	} {
		for _, name := range []string{"ls", "next", "search card", "tree", "show", "status"} {
			if found := bareIdentifiers(t, phase.runs[name]); len(found) > 0 {
				t.Errorf("%s %s printed the bare identifiers %v where a reference belongs", phase.label, name, found)
			}
		}
	}
}

// mustRead reads a file the test cannot proceed without, so the callers read
// as one line rather than as a read and an error check.
func mustRead(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(text)
}
