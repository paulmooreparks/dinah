package main

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// setNumber rewrites the registry line claiming one card to the number given,
// which is the only way to build a state add refuses to mint: add appends the
// next number above the high-water mark, so no filing of its own can collide.
//
// The card is named by identifier and the registry is one file at the workbench
// root, so the same rewrite reaches a live card and an archived one, and the
// fixture's recipe is what says which is which. A reference would serve for
// neither, because it stops resolving the moment its number collides and stops
// resolving at all once the card is archived. The line is rewritten where it
// stands, because line order is the record of who claimed first and the repair
// reads that record.
func setNumber(t *testing.T, dir, id string, number int) {
	t.Helper()
	registry := filepath.Join(dir, bench.CardNumbersName)
	text, err := os.ReadFile(registry)
	if err != nil {
		t.Fatalf("read %s: %v", registry, err)
	}
	lines := strings.Split(string(text), "\n")
	rewritten := false
	for nth, line := range lines {
		numbered, claimed, found := strings.Cut(line, " ")
		if !found || claimed != id {
			continue
		}
		if _, err := strconv.Atoi(numbered); err != nil {
			continue
		}
		lines[nth] = strconv.Itoa(number) + " " + id
		rewritten = true
		break
	}
	if !rewritten {
		t.Fatalf("%s carries no line claiming %s", registry, id)
	}
	if err := os.WriteFile(registry, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write %s: %v", registry, err)
	}
}

// ambiguousCardTree builds the workbench dinah-487's CLI criteria are driven
// against and answers the container and the six identifiers in filing order.
//
// The registry answers fx-1 with three cards and fx-3 with two, one of the
// three and both of the two standing in the archive mirror. Claimants are
// lines in one file rather than cards in one half, so the collision a merged
// registry carries is driven whole: the refusal names every claimant, live
// and archived alike, which is the candidate list the data test below holds
// to account. The unique numbers, 2 on the link source alone, are what keep
// the unambiguous legs of the other tests running over the same fixture.
//
// The three cards that belong in the mirror are archived after filing and
// before the numbers are rewritten, and the rewrite reaches the lines of live
// and archived cards alike because the registry is one file. The order of the
// two steps no longer decides anything, because archiving moves a directory
// and touches no line; what the order does is record, in the recipe itself,
// that an archived card keeps the line it was filed under.
func ambiguousCardTree(t *testing.T) (string, []string) {
	t.Helper()
	container := newBench(t)
	var ids []string
	for _, title := range []string{
		"the live twin", "the other live twin", "the link source",
		"the archived odd one", "the archived twin", "the other archived twin",
	} {
		ids = append(ids, cardID(t, container, addCard(t, container, title)))
	}
	for _, id := range ids[3:] {
		if got := runCLI(t, container, "archive", id); got.code != 0 {
			t.Fatalf("archive %s: %d %s", id, got.code, got.errw)
		}
	}
	dir := soleBenchDir(t, container)
	for _, c := range []struct {
		id     string
		number int
	}{
		{ids[0], 1}, {ids[1], 1}, {ids[2], 2},
		{ids[3], 1}, {ids[4], 3}, {ids[5], 3},
	} {
		setNumber(t, dir, c.id, c.number)
	}
	return container, ids
}

// cardsClaiming answers the identifiers the registry claims for one number, in
// file order, read off disk rather than taken from the recipe, so the expected
// set is what the workbench holds rather than what the fixture meant to write.
// File order is the order the refusal names its candidates in, which is why
// the helper keeps it rather than sorting.
func cardsClaiming(t *testing.T, dir string, number int) []string {
	t.Helper()
	registry := bench.LoadNumberRegistry(filepath.Join(dir, bench.CardNumbersName))
	return registry.ByNumber[number]
}

// TestAUniqueCardNumberResolvesOnTheHeadAsBefore is dinah-487 AC-1's CLI leg.
//
// One card, one number: show and path answer it and a number no card carries
// goes on refusing unknown-card. Only the two-or-more-matches case moves.
func TestAUniqueCardNumberResolvesOnTheHeadAsBefore(t *testing.T) {
	root := newBench(t)
	addCard(t, root, "the only card")
	for _, argv := range [][]string{{"show", "fx-1"}, {"path", "fx-1"}} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Errorf("%v exited %d and one card carries number 1: %s", argv, got.code, got.errw)
		}
	}
	missing := runCLI(t, root, "show", "fx-404")
	if missing.code == 0 {
		t.Fatalf("a number no card carries resolved: %s", missing.out)
	}
	if !strings.Contains(missing.errw, contract.UnknownCard) {
		t.Errorf("wanted %s, got: %s", contract.UnknownCard, missing.errw)
	}
}

// TestAnAmbiguousCardNumberIsRefusedOnTheHead is dinah-487 AC-2.
//
// Three references rather than one. splitRef reaches the same lookup from the
// prefixed form and from the bare number, and show and path arrive at
// resolveCardIn by two different call chains.
func TestAnAmbiguousCardNumberIsRefusedOnTheHead(t *testing.T) {
	root, _ := ambiguousCardTree(t)
	for _, argv := range [][]string{{"show", "fx-1"}, {"show", "1"}, {"path", "fx-1"}} {
		got := runCLI(t, root, argv...)
		if got.code == 0 {
			t.Errorf("%v resolved and two cards carry number 1: %s", argv, got.out)
			continue
		}
		if !strings.Contains(got.errw, contract.AmbiguousCard) {
			t.Errorf("%v refused with %q, wanted %s", argv, got.errw, contract.AmbiguousCard)
		}
	}
}

// TestTheAmbiguousCardRefusalNamesCandidatesThatResolve is dinah-487 AC-3,
// built on the shape TestTheAmbiguousNameRefusalNamesPositionsThatResolve
// already uses: refuse, then prove every candidate the refusal named resolves
// on its own.
//
// The set equality rather than a substring is what stops a check passing on a
// refusal that names one candidate and hides the others. The claimants of one
// number stand in both halves, so an archived candidate is proven against the
// mirror by the same command that proves a live one against the board, and a
// candidate that resolved in neither half would fail here rather than hide.
func TestTheAmbiguousCardRefusalNamesCandidatesThatResolve(t *testing.T) {
	root, ids := ambiguousCardTree(t)
	wanted := cardsClaiming(t, soleBenchDir(t, root), 1)
	if len(wanted) != 3 {
		t.Fatalf("the registry claims number 1 on %v, and a live pair and one archived card should", wanted)
	}
	archived := map[string]bool{}
	for _, id := range ids[3:] {
		archived[id] = true
	}

	refused := runCLI(t, root, "show", "fx-1")
	if refused.code == 0 {
		t.Fatalf("three cards answer to fx-1 and show resolved: %s", refused.out)
	}
	var named []string
	for _, field := range strings.Fields(refused.errw) {
		if bench.IsID(field) {
			named = append(named, field)
		}
	}
	sort.Strings(named)
	sortedWanted := append([]string(nil), wanted...)
	sort.Strings(sortedWanted)
	if strings.Join(named, ",") != strings.Join(sortedWanted, ",") {
		t.Fatalf("the refusal names %v and the registry claims the number on %v: %s", named, wanted, refused.errw)
	}
	for _, id := range named {
		argv := []string{"path", id}
		if archived[id] {
			argv = []string{"path", "--archived", id}
		}
		resolved := runCLI(t, root, argv...)
		if resolved.code != 0 {
			t.Errorf("the refusal named %s and path refused it: %d %s", id, resolved.code, resolved.errw)
			continue
		}
		if !strings.Contains(filepath.ToSlash(strings.TrimSpace(resolved.out)), "/"+id+"/") {
			t.Errorf("path %s answered %q, which stands outside that card's own directory", id, resolved.out)
		}
	}
}

// TestTheAmbiguousCardRefusalCarriesItsCandidatesAsData is dinah-487 AC-6.
//
// Both machine surfaces are read, because a refusal raised in internal/bench
// travelling intact to both heads is the whole reason that raise site was
// chosen. Neither head needed new code, so this confirms the wiring rather
// than asking for any.
func TestTheAmbiguousCardRefusalCarriesItsCandidatesAsData(t *testing.T) {
	root, _ := ambiguousCardTree(t)
	// The wanted value is the registry's own file order, which is the order the
	// refusal joins its candidates in, so the comparison holds the whole
	// workbench's claim on the number rather than one half of it.
	want := strings.Join(cardsClaiming(t, soleBenchDir(t, root), 1), "\n")

	machine := runCLI(t, root, "--json", "show", "fx-1")
	if machine.code == 0 {
		t.Fatalf("two cards answer to fx-1 and the machine form resolved: %s", machine.out)
	}
	report := decodeReport(t, machine.out)
	if report["refusal"] != contract.AmbiguousCard {
		t.Fatalf("the report refuses %v, wanted %s", report["refusal"], contract.AmbiguousCard)
	}
	context, ok := report["context"].(map[string]any)
	if !ok {
		t.Fatalf("the report carries no context member: %v", report)
	}
	if context["cards"] != want {
		t.Errorf("the CLI report carries the cards %v, wanted %q", context["cards"], want)
	}

	served := mcpToolPayload(t, runCLIWithInput(t, root,
		strings.NewReader(toolCallLine(t, "show", map[string]any{"actor": "alka", "card": "fx-1"})+"\n"), "mcp"))
	if served["refusal"] != contract.AmbiguousCard {
		t.Fatalf("the served answer refuses %v, wanted %s", served["refusal"], contract.AmbiguousCard)
	}
	servedContext, ok := served["context"].(map[string]any)
	if !ok {
		t.Fatalf("the served answer carries no context member: %v", served)
	}
	if servedContext["cards"] != want {
		t.Errorf("the served answer carries the cards %v, wanted %q", servedContext["cards"], want)
	}
}

// TestALinkRefusesAnAmbiguousCardNumber is dinah-487 AC-5's end-to-end leg.
//
// fx-2 is the subject rather than the target, because link resolves its
// subject first and a recipe with no card on number 2 would refuse
// unknown-card there and never reach the target at all.
func TestALinkRefusesAnAmbiguousCardNumber(t *testing.T) {
	root, ids := ambiguousCardTree(t)
	got := runCLI(t, root, "link", "fx-2", "relates-to", "fx-1")
	if got.code == 0 {
		t.Fatalf("two cards answer to fx-1 and the link was recorded: %s", got.out)
	}
	if !strings.Contains(got.errw, contract.AmbiguousCard) {
		t.Errorf("the link refused with %q, wanted %s", got.errw, contract.AmbiguousCard)
	}
	anchor := filepath.Join(soleBenchDir(t, root), bench.CardsDir, ids[2], bench.CardAnchor)
	text, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read %s: %v", anchor, err)
	}
	if strings.Contains(string(text), "relates-to") {
		t.Errorf("the refused link was written to the subject's anchor anyway:\n%s", text)
	}
}

// TestTheChangesFilterRefusesAnAmbiguousCardNumberOnTheHead is dinah-487
// AC-9's and AC-11's CLI legs.
//
// Each ambiguous reference is refused by name, and each of the two legs that
// hold an unchanged arm open still exits 0. The archived identifier exits
// through the second attempt, and the identifier no card file backs reaches
// the third arm, which accepts it because an identifier is never ambiguous.
//
// The refusal for an over-claimed number names every claimant, archived ones
// included, because the registry holds the whole workbench's claim in one file
// and the refusal is honest about all of it. What the caller must never get
// back is an answer, and the exit code is what says the filter answered
// nothing rather than quietly watching one card out of the candidates.
func TestTheChangesFilterRefusesAnAmbiguousCardNumberOnTheHead(t *testing.T) {
	root, ids := ambiguousCardTree(t)
	for _, ref := range []string{"fx-1", "fx-3"} {
		got := runCLI(t, root, "changes", "--card", ref)
		if got.code == 0 {
			t.Errorf("changes --card %s watched something and several cards answer to it: %s", ref, got.out)
			continue
		}
		if !strings.Contains(got.errw, contract.AmbiguousCard) {
			t.Errorf("changes --card %s refused with %q, wanted %s", ref, got.errw, contract.AmbiguousCard)
			continue
		}
		if ref == "fx-1" {
			var named []string
			for _, field := range strings.Fields(got.errw) {
				if bench.IsID(field) {
					named = append(named, field)
				}
			}
			sort.Strings(named)
			wanted := cardsClaiming(t, soleBenchDir(t, root), 1)
			sort.Strings(wanted)
			if strings.Join(named, ",") != strings.Join(wanted, ",") {
				t.Errorf("the refusal for fx-1 names %v and the registry claims the number on %v", named, wanted)
			}
		}
	}
	for _, ref := range []string{ids[3], "0f8d904a8b4c"} {
		if got := runCLI(t, root, "changes", "--card", ref); got.code != 0 {
			t.Errorf("changes --card %s exited %d, and an identifier is never ambiguous: %s", ref, got.code, got.errw)
		}
	}
}

// TestEditRefusesAnAmbiguousCardNumber is dinah-487 AC-12's CLI leg. No editor
// is needed, because the refusal is raised during reference resolution and
// before any editor is launched.
func TestEditRefusesAnAmbiguousCardNumber(t *testing.T) {
	root, _ := ambiguousCardTree(t)
	got := runCLI(t, root, "edit", "fx-1")
	if got.code == 0 {
		t.Fatalf("two cards answer to fx-1 and edit opened one: %s", got.out)
	}
	if !strings.Contains(got.errw, contract.AmbiguousCard) {
		t.Errorf("edit refused with %q, wanted %s", got.errw, contract.AmbiguousCard)
	}
}
