package release

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Git runs git commands against one repository.
//
// The cut mechanism is mostly a sequence of git questions, and asking them
// through one small type is what lets a test drive the whole thing against a
// fixture repository built in a temporary directory.
type Git struct {
	Dir string
}

// Run executes one git command and returns its trimmed standard output. A
// non-zero exit carries git's own standard error into the error, because the
// operator reading a failed cut needs git's message rather than ours.
func (g Git) Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return strings.TrimRight(out.String(), "\n"), fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimRight(errOut.String(), "\n"))
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// lines splits git output into non-empty lines.
func lines(out string) []string {
	var result []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// MinorStart reports the commit where VERSION was last set to base, which is
// where the current minor begins. A repository whose VERSION never held base
// on any commit that touched it starts at its root commit instead, which is
// the case for a line that has been the only one since the beginning.
//
// The walk continues past a commit that touched VERSION without changing what
// it says, so a whitespace fix inside the current minor does not move the
// minor's start forward over the work that came before it.
func MinorStart(g Git, ref, base string) (string, error) {
	out, err := g.Run("log", "--format=%H", ref, "--", "VERSION")
	if err != nil {
		return "", err
	}
	start := ""
	for _, sha := range lines(out) {
		content, err := g.Run("show", sha+":VERSION")
		if err != nil {
			break
		}
		if strings.TrimSpace(content) != base {
			break
		}
		start = sha
	}
	if start != "" {
		return start, nil
	}
	roots, err := g.Run("rev-list", "--max-parents=0", ref)
	if err != nil {
		return "", err
	}
	all := lines(roots)
	if len(all) == 0 {
		return "", fmt.Errorf("%s has no root commit, so the start of the %s line cannot be resolved", ref, base)
	}
	return all[len(all)-1], nil
}

// Candidate is one commit a cut may carry: a commit on the trunk inside the
// current minor that release.yml gave a dev tag to.
type Candidate struct {
	SHA     string
	Card    string
	Tag     string
	Subject string
}

// Candidates lists, oldest first, every commit after start and up to tip that
// carries a dev tag for this line.
//
// An untagged commit is not eligible, and that is a property of the tag rather
// than a rule invented here. release.yml only tags a push that touched VERSION,
// go.mod, cmd, internal or the release workflow itself, so a docs-only or
// extension-only card produces no dev tag, is already on the trunk, and has
// nothing for a CLI promotion to carry.
func Candidates(g Git, start, tip, base string) ([]Candidate, error) {
	out, err := g.Run("log", "--reverse", "--format=%H%x1f%s", start+".."+tip)
	if err != nil {
		return nil, err
	}
	var result []Candidate
	for _, line := range lines(out) {
		sha, subject, found := strings.Cut(line, "\x1f")
		if !found {
			continue
		}
		tags, err := g.Run("tag", "--points-at", sha)
		if err != nil {
			return nil, err
		}
		devTag := ""
		for _, tag := range lines(tags) {
			if _, ok := Patch(Dev, base, tag); ok {
				devTag = tag
				break
			}
		}
		if devTag == "" {
			continue
		}
		result = append(result, Candidate{
			SHA:     sha,
			Card:    CardOf(subject),
			Tag:     devTag,
			Subject: subject,
		})
	}
	return result, nil
}

// CardOf reads the card a commit belongs to out of its subject line, which
// every flow column on this board writes as "<human-id>: <summary>". A subject
// with no such prefix belongs to no card and cannot be named in a cut.
func CardOf(subject string) string {
	prefix, _, found := strings.Cut(subject, ":")
	if !found {
		return ""
	}
	prefix = strings.TrimSpace(prefix)
	slug, number, found := strings.Cut(prefix, "-")
	if !found || slug == "" || number == "" {
		return ""
	}
	for _, r := range slug {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return ""
		}
	}
	for _, r := range number {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return prefix
}

// CarriedSHAs reads which trunk commits a lineage already carries, from the
// provenance trailer git cherry-pick -x writes into each picked commit. Git
// already records this, so the cut mechanism reads it back rather than keeping
// a second copy of it somewhere that could disagree.
func CarriedSHAs(g Git, ref string) (map[string]bool, error) {
	out, err := g.Run("log", "--format=%B", ref)
	if err != nil {
		return nil, err
	}
	carried := map[string]bool{}
	const marker = "(cherry picked from commit "
	for _, line := range lines(out) {
		at := strings.Index(line, marker)
		if at < 0 {
			continue
		}
		rest := line[at+len(marker):]
		sha, _, found := strings.Cut(rest, ")")
		if !found {
			continue
		}
		sha = strings.TrimSpace(sha)
		if sha != "" {
			carried[sha] = true
		}
	}
	return carried, nil
}

// Link is one incoming dependency: Card cannot ship without Predecessor.
type Link struct {
	Card        string
	Predecessor string
}

// Dependencies is what the dispatcher declared about the cards in a cut. It
// holds the pairs themselves and, separately, the set of cards an answer was
// given for at all.
//
// The second half is the part that matters. A list of pairs alone cannot tell
// a card with no predecessors from a card nobody thought about, and those two
// have to be told apart: the first is a fact and the second is an omission.
type Dependencies struct {
	// Links are the declared dependent-on-predecessor pairs.
	Links []Link
	// answered is every card the input said something about, including the
	// ones it said have no predecessor.
	answered map[string]bool
}

// Answered reports whether the input declared anything at all about a card.
func (d Dependencies) Answered(card string) bool {
	return d.answered[card]
}

// NoPredecessorWord is how the input says a card has no predecessors. It is
// written per card, as "dinah-359>none", and never on its own.
const NoPredecessorWord = "none"

// ParseLinks reads the dependency declaration a cut is dispatched with. Each
// comma-separated entry is a "dependent>predecessor" pair, and a card with no
// predecessors is declared by writing "dependent>none".
//
// The grammar takes one answer per card on purpose, and Select refuses a cut
// naming a card this input said nothing about. An earlier shape accepted the
// single word "none" for a whole cut, which meant one word switched the
// dependency check off for a cut of any size while the refusal on an empty
// input went on looking like protection. A refusal any value satisfies is not
// a refusal, so the placeholder is now something the dispatcher has to write
// against each card by name.
func ParseLinks(spec string) (Dependencies, error) {
	deps := Dependencies{answered: map[string]bool{}}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return deps, fmt.Errorf("the links input is empty; declare every card in this cut, writing each incoming blocks or parked_behind link as dependent>predecessor and each card that has none as dependent>none")
	}
	if spec == NoPredecessorWord {
		return deps, fmt.Errorf("the links input is the bare word %q, which no longer declares anything; write %s once per card, for example dinah-359>none,dinah-297>none, so that a card nobody answered for is refused rather than assumed to be unconstrained", NoPredecessorWord, NoPredecessorWord)
	}
	declaredNone := map[string]bool{}
	hasPredecessor := map[string]bool{}
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		card, predecessor, found := strings.Cut(entry, ">")
		card = strings.TrimSpace(card)
		predecessor = strings.TrimSpace(predecessor)
		if !found || card == "" || predecessor == "" {
			return Dependencies{answered: map[string]bool{}}, fmt.Errorf("%q is not a dependency declaration; write each one as dependent>predecessor, for example dinah-359>dinah-297, or dependent>none for a card with no predecessors", entry)
		}
		deps.answered[card] = true
		if predecessor == NoPredecessorWord {
			declaredNone[card] = true
			continue
		}
		hasPredecessor[card] = true
		deps.Links = append(deps.Links, Link{Card: card, Predecessor: predecessor})
	}
	var contradictory []string
	for card := range declaredNone {
		if hasPredecessor[card] {
			contradictory = append(contradictory, card)
		}
	}
	if len(contradictory) > 0 {
		sort.Strings(contradictory)
		return Dependencies{answered: map[string]bool{}}, fmt.Errorf("the links input declares both a predecessor and none for %s; say one or the other", strings.Join(contradictory, ", "))
	}
	if len(deps.answered) == 0 {
		return Dependencies{answered: map[string]bool{}}, fmt.Errorf("the links input declares no card; declare every card in this cut, writing each one as dependent>predecessor or dependent>none")
	}
	return deps, nil
}

// Selection is what a validated cut carries.
type Selection struct {
	// Picks are the commits to cherry-pick, in the trunk's own order.
	Picks []Candidate
	// AlreadyCarried names the requested cards the beta base already holds,
	// which are skipped rather than picked twice.
	AlreadyCarried []string
	// Unconstrained names the requested cards the dispatcher declared to have
	// no predecessor, in the order the cards input names them. The run prints
	// these, so an assumption the check was told to make is read back rather
	// than supplied invisibly.
	Unconstrained []string
}

// Select resolves the requested cards against the candidates, checks the
// dependency graph, and returns what to cherry-pick.
//
// It refuses in three ways and none of them half-finishes. A requested card
// that resolves to no tagged commit fails the whole run, because the pipeline
// cannot tell a mistyped identifier from a card whose merge has not landed,
// and promoting the smaller set the operator did not ask for is the worse of
// the two answers. A requested card the links input said nothing about fails
// the run, because silence about a card is not a claim that the card is free
// of predecessors. A card whose predecessor is neither in this cut nor already
// in the lineage fails the run too, since holding back the work another card
// was built on ships a tree that has never worked.
//
// The three refusal messages run to several capitalised sentences with
// terminal punctuation, which the Go style standard's rule 5 asks against.
// They are exceptions on purpose. Each one is a terminal message an operator
// reads in a run log, reached through promote.yml's "::error::%s", and none of
// them is ever wrapped into a longer sentence by a caller, so the reason the
// rule exists does not apply. The same holds for the conflict message on
// CherryPick below.
func Select(requested []string, candidates []Candidate, deps Dependencies, carriedSHAs map[string]bool) (*Selection, error) {
	wanted := map[string]bool{}
	var order []string
	for _, id := range requested {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !wanted[id] {
			wanted[id] = true
			order = append(order, id)
		}
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("the cards input names no card, so there is nothing to promote")
	}

	byCard := map[string][]Candidate{}
	for _, c := range candidates {
		if c.Card == "" {
			continue
		}
		byCard[c.Card] = append(byCard[c.Card], c)
	}

	var unresolved []string
	for _, id := range order {
		if len(byCard[id]) == 0 {
			unresolved = append(unresolved, id)
		}
	}
	if len(unresolved) > 0 {
		return nil, fmt.Errorf(
			"these cards match no tagged commit on the trunk since this minor started: %s\n"+
				"A card resolves here once its work has merged to the trunk and the release workflow has tagged that commit. So either the identifier is wrong, in which case correct it, or the merge has not landed yet, in which case wait for it. Fix the cards input and dispatch the cut again. Nothing was assembled, tagged or published.",
			strings.Join(unresolved, ", "))
	}

	var undeclared []string
	for _, id := range order {
		if !deps.Answered(id) {
			undeclared = append(undeclared, id)
		}
	}
	if len(undeclared) > 0 {
		return nil, fmt.Errorf(
			"the links input says nothing about these cards in the cut: %s\n"+
				"Read each one's incoming blocks and parked_behind links off the board and declare them as %s>predecessor, or declare %s>none where the card has no predecessor. A card nobody answered for is refused here rather than assumed to be unconstrained, because an omission and a card with no predecessors are not the same thing. Nothing was assembled, tagged or published.",
			strings.Join(undeclared, ", "), undeclared[0], undeclared[0])
	}

	carriedCards := map[string]bool{}
	for _, c := range candidates {
		if c.Card != "" && carriedSHAs[c.SHA] {
			carriedCards[c.Card] = true
		}
	}

	var violations []string
	for _, link := range deps.Links {
		if !wanted[link.Card] {
			continue
		}
		if wanted[link.Predecessor] || carriedCards[link.Predecessor] {
			continue
		}
		violations = append(violations, fmt.Sprintf("%s depends on %s, which is not in this cut and was not in an earlier one", link.Card, link.Predecessor))
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		return nil, fmt.Errorf(
			"this cut would hold back work another card in it was built on:\n  %s\n"+
				"Add each predecessor to the cards input, or drop the card that depends on it, then dispatch the cut again. Nothing was assembled, tagged or published.",
			strings.Join(violations, "\n  "))
	}

	constrained := map[string]bool{}
	for _, link := range deps.Links {
		constrained[link.Card] = true
	}
	selection := &Selection{}
	for _, id := range order {
		if !constrained[id] {
			selection.Unconstrained = append(selection.Unconstrained, id)
		}
	}
	for _, c := range candidates {
		if c.Card == "" || !wanted[c.Card] {
			continue
		}
		if carriedSHAs[c.SHA] {
			selection.AlreadyCarried = append(selection.AlreadyCarried, c.Card)
			continue
		}
		selection.Picks = append(selection.Picks, c)
	}
	return selection, nil
}

// CherryPick assembles the cut onto base, in the trunk's own order, and stops
// the moment one commit does not apply.
//
// The stop is a stop rather than a recovery. A conflict means two pieces of
// work disagree about the same lines, and choosing between them is the
// operator's call rather than a pipeline's. The sequence is aborted so the
// checkout carries no half-applied state, and since the checkout belongs to
// the job and the job is discarded when it ends, a failed assembly leaves
// nothing behind anywhere.
func CherryPick(g Git, base string, picks []Candidate) error {
	if _, err := g.Run("checkout", "--detach", base); err != nil {
		return err
	}
	for _, pick := range picks {
		if _, err := g.Run("cherry-pick", "-x", pick.SHA); err != nil {
			if _, abortErr := g.Run("cherry-pick", "--abort"); abortErr != nil {
				return fmt.Errorf("%s (%s) does not apply cleanly onto %s, and the cherry-pick could not be aborted afterwards: %w", pick.SHA, pick.Card, base, abortErr)
			}
			return fmt.Errorf(
				"%s (%s) does not apply cleanly onto %s, so the cut was abandoned before any check ran: no tag was pushed and no channel manifest was written.\n"+
					"Resolve it by rebasing that card's work onto the beta base, or drop %s from the cards input, then dispatch the cut again.\n%w",
				pick.SHA, pick.Card, base, pick.Card, err)
		}
	}
	return nil
}
