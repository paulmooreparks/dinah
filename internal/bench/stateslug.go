package bench

import (
	"path/filepath"
	"strconv"
)

// SlugField is the frontmatter key carrying a state's slug: the short handle a
// person types in place of a title that has to be quoted on a command line.
//
// This is the one statement of the key's spelling. Every reader and writer of
// the field goes through it rather than repeating the literal.
const SlugField = "slug"

// StateAnchorPath is the anchor of one state of a bench.
func (b *Bench) StateAnchorPath(id string) string {
	return filepath.Join(b.Root, StatesDir, id, StateAnchor)
}

// checkStateSlugs applies the slug invariants to the flow: every state carries
// a slug, each one conforms to the grammar, and no two states share one.
//
// The missing finding is reported only while the workbench declares a major
// below the one that requires a slug. Past that point Open refuses the bench
// before a check can run over it, so a finding there would name a state nobody
// could reach. The malformed and duplicate findings are reported at every
// major, because a slug that is present and wrong is a corruption rather than
// a field the migration has not reached yet.
func (b *Bench) checkStateSlugs() []Finding {
	major, _, ok := splitProfile(b.Profile)
	var findings []Finding
	seen := map[string]bool{}
	for _, state := range b.States {
		path := b.StateAnchorPath(state.ID)
		if state.Slug == "" {
			if ok && major < SlugMandatoryMajor {
				findings = append(findings, Finding{Path: path, Key: FindingSlugMissing, Detail: state.ID})
			}
			continue
		}
		if !ValidStateSlug(state.Slug) {
			findings = append(findings, Finding{Path: path, Key: FindingSlugMalformed, Detail: state.Slug})
			continue
		}
		if seen[state.Slug] {
			findings = append(findings, Finding{Path: path, Key: FindingSlugDuplicate, Detail: state.Slug})
			continue
		}
		seen[state.Slug] = true
	}
	return findings
}

// SlugAssignment is one state the slug migration repaired, named together with
// the slug it was given.
//
// The migration reports these rather than only counting them, because the slug
// it derived is what an operator is about to type and the run that wrote it is
// the moment to say so.
type SlugAssignment struct {
	// State is the identifier of the state repaired.
	State string `json:"state"`
	// Title is that state's title, which is what the slug was derived from.
	Title string `json:"title"`
	// Slug is the slug written to the state's anchor.
	Slug string `json:"slug"`
}

// BackfillStateSlugs derives a slug for every state carrying none and writes it
// to the state's anchor, reporting what it assigned and returning a finding for
// every state it could not repair.
//
// The walk takes the states in flow order, so the slug a title derives to is
// the same on every machine and a second run over the same workbench derives
// the same answers again. A slug already on disk is left exactly as it stands,
// including a malformed or duplicated one: somebody chose that value on
// purpose, and the checker names it for a person to resolve rather than having
// the repair overwrite a decision it cannot read.
//
// A state whose anchor this run cannot write is reported and stepped over, the
// way the ordinal migration treats one, so an obstruction on one state does not
// cost the operator the account of the states already repaired.
func (b *Bench) BackfillStateSlugs() ([]SlugAssignment, []Finding) {
	taken := map[string]bool{}
	for _, state := range b.States {
		if state.Slug != "" {
			taken[state.Slug] = true
		}
	}
	assigned := []SlugAssignment{}
	var findings []Finding
	for _, state := range b.States {
		if state.Slug != "" {
			continue
		}
		path := b.StateAnchorPath(state.ID)
		derived := SlugifyDashed(state.Title)
		if derived == "" {
			findings = append(findings, Finding{Path: path, Key: FindingSlugMalformed, Detail: state.ID})
			continue
		}
		candidate := derived
		for suffix := 2; taken[candidate]; suffix++ {
			candidate = derived + "-" + strconv.Itoa(suffix)
		}
		if err := stampStateSlug(path, candidate); err != nil {
			findings = append(findings, Finding{Path: path, Key: FindingSlugMissing, Detail: state.ID})
			continue
		}
		taken[candidate] = true
		state.Slug = candidate
		assigned = append(assigned, SlugAssignment{State: state.ID, Title: state.Title, Slug: candidate})
	}
	return assigned, findings
}

// stampStateSlug writes a slug onto a state anchor that carries none,
// preserving every other key of the header and the state's own instructions.
func stampStateSlug(path, slug string) error {
	text, err := ReadText(path)
	if err != nil {
		return err
	}
	fm, body := ParseAnchor(text)
	fm.Set(SlugField, slug)
	return WriteText(path, fm.Render(body))
}
