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

// setNumber rewrites one card's number in its own anchor, which is the only
// way to build a state add refuses to mint: add scans for the highest number
// in use and counts up.
//
// The card is named by identifier and the root is named explicitly, so the
// same helper reaches a live card and an archived one. A reference would serve
// for neither, because it stops resolving the moment its number collides and
// stops resolving at all once the card is archived.
func setNumber(t *testing.T, root, id string, number int) {
	t.Helper()
	anchor := filepath.Join(root, id, bench.CardAnchor)
	text, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read %s: %v", anchor, err)
	}
	lines := strings.Split(string(text), "\n")
	rewritten := false
	for nth, line := range lines {
		if strings.HasPrefix(line, "number:") {
			lines[nth] = "number: " + strconv.Itoa(number)
			rewritten = true
			break
		}
	}
	if !rewritten {
		t.Fatalf("%s carries no number line to rewrite", anchor)
	}
	if err := os.WriteFile(anchor, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write %s: %v", anchor, err)
	}
}

// ambiguousCardTree builds the workbench dinah-487's CLI criteria are driven
// against and answers the container and the six identifiers in filing order.
//
// The live half answers fx-1 with two cards, fx-2 with one, and fx-3 with
// none. The archive answers fx-1 with one and fx-3 with two. The live
// collision is what a live-half unwrap is driven against, with the archived
// card on the same number standing exactly where the unrepaired fall-through
// would land; without it the fall-through refuses unknown-card and a check
// asserting only that the command stopped would go green on the defect. The
// archive collision is the only shape an archived-half unwrap is reachable in,
// since a live card on the number answers the first attempt.
//
// The order of the steps is load bearing. Every identifier is recorded while
// its number is still unique, the three cards that belong in the mirror are
// archived next, and only then are the numbers rewritten, which is what lets
// an archived card carry a number no live card has.
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
	live := filepath.Join(soleBenchDir(t, container), bench.CardsDir)
	archived := filepath.Join(soleBenchDir(t, container), bench.ArchiveDir, bench.CardsDir)
	for _, c := range []struct {
		root   string
		id     string
		number int
	}{
		{live, ids[0], 1}, {live, ids[1], 1}, {live, ids[2], 2},
		{archived, ids[3], 1}, {archived, ids[4], 3}, {archived, ids[5], 3},
	} {
		setNumber(t, c.root, c.id, c.number)
	}
	return container, ids
}

// cardsCarrying answers the identifiers of the card directories under a root
// whose anchor records the number, read off disk rather than taken from the
// recipe, so the expected set is what the workbench holds rather than what the
// fixture meant to write.
func cardsCarrying(t *testing.T, root string, number int) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		text, err := os.ReadFile(filepath.Join(root, entry.Name(), bench.CardAnchor))
		if err != nil {
			continue
		}
		if strings.Contains(string(text), "\nnumber: "+strconv.Itoa(number)+"\n") {
			found = append(found, entry.Name())
		}
	}
	sort.Strings(found)
	return found
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
// Three references rather than one. splitRef reaches the same scan from the
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
// refusal that names one candidate and hides the other.
func TestTheAmbiguousCardRefusalNamesCandidatesThatResolve(t *testing.T) {
	root, _ := ambiguousCardTree(t)
	live := filepath.Join(soleBenchDir(t, root), bench.CardsDir)
	wanted := cardsCarrying(t, live, 1)
	if len(wanted) != 2 {
		t.Fatalf("the fixture's live half carries number 1 on %v, and two cards should", wanted)
	}

	refused := runCLI(t, root, "show", "fx-1")
	if refused.code == 0 {
		t.Fatalf("two cards answer to fx-1 and show resolved: %s", refused.out)
	}
	var named []string
	for _, field := range strings.Fields(refused.errw) {
		if bench.IsID(field) {
			named = append(named, field)
		}
	}
	sort.Strings(named)
	if strings.Join(named, ",") != strings.Join(wanted, ",") {
		t.Fatalf("the refusal names %v and the live half carries the number on %v: %s", named, wanted, refused.errw)
	}
	for _, id := range named {
		resolved := runCLI(t, root, "path", id)
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
	live := filepath.Join(soleBenchDir(t, root), bench.CardsDir)
	want := strings.Join(cardsCarrying(t, live, 1), "\n")

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
// through the second attempt, which is one of the two this fix modifies, and
// so proves the modification did not close the filter on the case it was
// designed for. The identifier no card file backs reaches the third arm, which
// is unchanged and accepts it because an identifier is never ambiguous.
func TestTheChangesFilterRefusesAnAmbiguousCardNumberOnTheHead(t *testing.T) {
	root, ids := ambiguousCardTree(t)
	for _, ref := range []string{"fx-1", "fx-3"} {
		got := runCLI(t, root, "changes", "--card", ref)
		if got.code == 0 {
			t.Errorf("changes --card %s watched something and two cards answer to it: %s", ref, got.out)
			continue
		}
		if !strings.Contains(got.errw, contract.AmbiguousCard) {
			t.Errorf("changes --card %s refused with %q, wanted %s", ref, got.errw, contract.AmbiguousCard)
		}
		if ref == "fx-1" {
			for _, id := range []string{ids[3]} {
				if strings.Contains(got.out+got.errw, id) {
					t.Errorf("the refusal for fx-1 names the archived card %s, which the caller did not ask for", id)
				}
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
