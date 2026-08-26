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
// below the one that requires a slug. The malformed and duplicate findings are
// stated for every major, since a stored slug that is wrong is a corruption
// rather than a field the migration has not reached yet, though in practice
// they fire in the same window the missing one does. Below that major Open
// carries an absent, malformed or duplicated slug through rather than refusing
// it, which is what leaves a workbench carrying one openable and lets a person
// be told which state to repair; past that major Open refuses the workbench
// before a check can run over it at all.
// Every finding names the state it was raised over and carries the state's own
// anchor as its path, so all three read the same way and a person is told which
// file to open to see the value that is wrong.
func (b *Bench) checkStateSlugs() []Finding {
	return b.checkStateSlugsWithin(SlugMandatoryMajor)
}

// checkStateSlugsWithin is checkStateSlugs with the mandatory major supplied
// rather than read from SlugMandatoryMajor, so a test can drive it to a value
// this build's own constant does not carry. It is the same reason
// admitProfileWithin takes its window as parameters instead of reading
// ProfileFloorMajor/Minor and ProfileMajor/Minor directly.
func (b *Bench) checkStateSlugsWithin(mandatoryMajor int) []Finding {
	major, _, _, ok := resolveProfile(b.Profile, [2]int{ProfileMajor, ProfileMinor})
	var findings []Finding
	seen := map[string]bool{}
	for _, state := range b.States {
		path := b.StateAnchorPath(state.ID)
		if state.Slug == "" {
			if ok && major < mandatoryMajor {
				findings = append(findings, Finding{Path: path, Key: FindingSlugMissing, Detail: state.ID})
			}
			continue
		}
		if !ValidStateSlug(state.Slug) {
			findings = append(findings, Finding{Path: path, Key: FindingSlugMalformed, Detail: state.ID})
			continue
		}
		if seen[state.Slug] {
			findings = append(findings, Finding{Path: path, Key: FindingSlugDuplicate, Detail: state.ID})
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
// cost the operator the account of the states already repaired. It and a title
// no slug can be derived from each carry a finding key of their own, because
// each names what stopped this run rather than a condition the checker could
// find on disk afterwards.
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
			findings = append(findings, Finding{Path: path, Key: FindingSlugUnderivable, Detail: state.ID})
			continue
		}
		candidate := FreeSlug(derived, taken)
		if err := stampSlug(path, candidate); err != nil {
			findings = append(findings, Finding{Path: path, Key: FindingSlugUnwritable, Detail: state.ID})
			continue
		}
		taken[candidate] = true
		state.Slug = candidate
		assigned = append(assigned, SlugAssignment{State: state.ID, Title: state.Title, Slug: candidate})
	}
	return assigned, findings
}

// FreeSlug is the first slug of a derivation's own family that nobody has
// taken: the derived slug itself, or that slug with the lowest counting
// suffix from two upward that is free.
//
// The suffix is what the state slug grammar's trailing dash and digits exist
// to admit. Under the workbench grammar the resolver's own output would be
// illegal, since ValidSlug refuses a final segment of digits alone, so a
// grammar excluding the suffix would leave a collision with no way to resolve
// it at all.
func FreeSlug(derived string, taken map[string]bool) string {
	candidate := derived
	for suffix := 2; taken[candidate]; suffix++ {
		candidate = derived + "-" + strconv.Itoa(suffix)
	}
	return candidate
}

// stampSlug writes a slug onto an anchor that carries none, preserving every
// other key of the header and the entity's own body. It serves a state and a
// workstream alike, since both carry the field under the same name.
//
// The slug goes directly after the title, where the writer that creates an
// anchor puts it, so a migrated anchor and a newly written one read the same
// way rather than differing by which code path wrote them.
func stampSlug(path, slug string) error {
	text, err := ReadText(path)
	if err != nil {
		return err
	}
	fm, body := ParseAnchor(text)
	fm.SetAfter(SlugField, slug, "title")
	return WriteText(path, fm.Render(body))
}
