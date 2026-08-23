package bench

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// Level is one member of a declared level set.
type Level struct {
	// Name is the level's identity within the workbench.
	Name string
	// Hint is the one-line guidance the declaration may carry, empty when
	// the entry is a bare name.
	Hint string
	// Rank is the entry's zero-based position in the declaration, which is
	// what the format makes the rank.
	Rank int
}

// LevelAxes are the two axes a workbench may declare, in the order a reader
// meets them.
//
// Each axis is declared independently of the other. A workbench may declare
// both sets, one of them, or neither, and every part of this model is keyed by
// axis rather than by workbench, so a workbench declaring severity and no
// priority is an ordinary workbench rather than a degenerate one.
var LevelAxes = []string{"severity", "priority"}

// LevelsKey is the frontmatter key carrying the workbench's declared level
// sets, one nested block holding one entry per axis.
const LevelsKey = "levels"

// KnownLevelAxis reports whether a name is one of the two axes this model
// reads. An axis outside the set is left on disk untouched and takes no part
// in any check.
func KnownLevelAxis(name string) bool {
	for _, known := range LevelAxes {
		if known == name {
			return true
		}
	}
	return false
}

// levelAxis matches an axis line inside the levels block: a key indented
// beneath the block's own key at column one, which is what keeps the nested
// lines attached to levels rather than becoming keys of their own.
var levelAxis = regexp.MustCompile(`^\s+([A-Za-z_][A-Za-z0-9_.-]*):(.*)$`)

// levelEntry matches one dashed entry beneath an axis.
var levelEntry = regexp.MustCompile(`^\s*-\s*(.*)$`)

// Levels returns one axis's declared members in declaration order, and nil
// when the workbench declares no set for that axis. The answer is about the
// named axis alone: an axis carrying no declaration returns nil whether or not
// the other axis carries one.
func (b *Bench) Levels(axis string) []Level {
	if len(b.levels[axis]) == 0 {
		return nil
	}
	return append([]Level(nil), b.levels[axis]...)
}

// Level returns one declared member by name, and nil when the axis declares
// no member of that name.
func (b *Bench) Level(axis, name string) *Level {
	for _, level := range b.levels[axis] {
		if level.Name == name {
			found := level
			return &found
		}
	}
	return nil
}

// readLevels reads the levels block out of the raw frontmatter lines the way
// readLinks reads a card's links sequence, rather than by introducing a YAML
// parser for one key.
//
// Both declared syntaxes are accepted and they mix freely across axes within
// one block: a flow sequence on the axis's own line, whose entries are bare
// names, and a block of dashed entries beneath the axis, where an entry is
// either a bare name or a name mapping to a one-line hint.
//
// Four rules bind. Declaration order within one axis is the rank, and nothing
// sorts the members. Ranks are counted within an axis and never across the
// block, so each axis's ranks run from zero however many members the other
// axis carries. A duplicate name within one axis keeps the first occurrence
// for both rank and lookup. A block carrying no parseable child leaves every
// axis undeclared and raises nothing, because the format's reader posture is
// to ignore what it cannot read rather than to fail.
func readLevels(fm *Frontmatter) map[string][]Level {
	axes := map[string][]Level{}
	axis := ""
	for _, line := range fm.Raw(LevelsKey) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := levelEntry.FindStringSubmatch(line); m != nil {
			if axis == "" {
				continue
			}
			name, hint := splitLevelEntry(m[1])
			addLevel(axes, axis, name, hint)
			continue
		}
		m := levelAxis.FindStringSubmatch(line)
		if m == nil {
			axis = ""
			continue
		}
		// An axis outside LevelAxes takes no part in the model, and its own
		// dashed entries go with it, which is what clearing the current axis
		// rather than remembering it does.
		if !KnownLevelAxis(m[1]) {
			axis = ""
			continue
		}
		axis = m[1]
		inline := unquote(strings.TrimSpace(m[2]))
		if !strings.HasPrefix(inline, "[") || !strings.HasSuffix(inline, "]") {
			continue
		}
		for _, raw := range strings.Split(strings.Trim(inline, "[]"), ",") {
			addLevel(axes, axis, unquote(strings.TrimSpace(raw)), "")
		}
	}
	if len(axes) == 0 {
		return nil
	}
	return axes
}

// splitLevelEntry reads one dashed entry into its name and its hint. The text
// after the first colon is the hint, and an entry carrying no colon is a bare
// name. A trailing annotation comment is dropped from a bare name and left
// alone inside a hint, which is prose a person wrote.
func splitLevelEntry(entry string) (string, string) {
	name, hint, mapped := strings.Cut(entry, ":")
	if !mapped {
		return unquote(stripComment(entry)), ""
	}
	return unquote(strings.TrimSpace(name)), unquote(strings.TrimSpace(hint))
}

// addLevel appends one member to an axis, keeping the first occurrence of a
// duplicated name for both rank and lookup, and skipping an entry that read as
// no name at all.
func addLevel(axes map[string][]Level, axis, name, hint string) {
	if name == "" {
		return
	}
	for _, level := range axes[axis] {
		if level.Name == name {
			return
		}
	}
	axes[axis] = append(axes[axis], Level{Name: name, Hint: hint, Rank: len(axes[axis])})
}

// renderLevelsMember renders an interchange definition's levels member as the
// block the reader above parses, and reports whether it could read it.
//
// Three rules bind. Axes are written in LevelAxes order first and any further
// axis in sorted order after them, so the render is deterministic whatever
// order the JSON object carried and an axis this model does not read is
// preserved rather than dropped. That ordering ranges over the axes the member
// actually declares, so a definition declaring one axis writes one line and no
// placeholder for the absent one. Only the flow form is rendered, since a
// definition has no need of the hint form and CORE-JSON-7 asks for
// preservation rather than for expressiveness.
//
// A member this cannot read, meaning any axis whose value is not an array of
// strings, reports false, and the caller falls back to the raw JSON line every
// unrecognized member travels as, so nothing is lost.
func renderLevelsMember(raw json.RawMessage) ([]string, bool) {
	declared := map[string][]string{}
	if err := json.Unmarshal(raw, &declared); err != nil {
		return nil, false
	}
	var further []string
	for axis := range declared {
		if KnownLevelAxis(axis) {
			continue
		}
		further = append(further, axis)
	}
	sort.Strings(further)
	var ordered []string
	for _, axis := range LevelAxes {
		if _, ok := declared[axis]; ok {
			ordered = append(ordered, axis)
		}
	}
	ordered = append(ordered, further...)
	lines := []string{LevelsKey + ":"}
	for _, axis := range ordered {
		lines = append(lines, "  "+axis+": ["+strings.Join(declared[axis], ", ")+"]")
	}
	return lines, true
}
