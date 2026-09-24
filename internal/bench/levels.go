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

// LevelAxes are the three axes a workbench may declare, in the order a reader
// meets them.
//
// Each axis is declared independently of the others. A workbench may declare
// every set, some of them, or none, and every part of this model is keyed by
// axis rather than by workbench, so a workbench declaring severity and no
// priority is an ordinary workbench rather than a degenerate one.
var LevelAxes = []string{"severity", "priority", "tier"}

// LevelsKey is the frontmatter key carrying the workbench's declared level
// sets, one nested block holding one entry per axis.
const LevelsKey = "levels"

// KnownLevelAxis reports whether a name is one of the three axes this model
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
// another axis carries one.
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

// LevelNames is one axis's members in declaration order, which is the order
// every refusal listing a declared set prints them in. It lives here beside
// the set it reads rather than beside either caller, because both the level
// checks and the tier resolver print the same list for the same reason.
func LevelNames(levels []Level) []string {
	names := make([]string, 0, len(levels))
	for _, level := range levels {
		names = append(names, level.Name)
	}
	return names
}

// levelsBlock is what readLevels answers: the declared sets by axis, the raw
// condition each axis carried, and whether any axis was written in the
// mapping form. Open resolves the conditions once the fields block is read
// too, since a condition's gate is a declared field.
type levelsBlock struct {
	axes        map[string][]Level
	conditions  map[string]*rawCondition
	mappingForm bool
}

// The three forms an axis's declaration can be in while its lines are being
// read. The form is settled by the first line beneath an axis line whose
// colon has nothing after it: a dashed line opens the list form and a deeper
// key line opens the mapping form.
const (
	axisFormUnknown = iota
	axisFormList
	axisFormMapping
)

// openAxis is the axis the reader is inside, with what it has settled about
// the lines beneath it so far.
type openAxis struct {
	// name is the axis, and ignored is true where it is one the model does
	// not read, so every nested line goes with it.
	name    string
	ignored bool
	// indent is the axis line's own indent, and memberIndent the indent of
	// the mapping form's members once the first has been met.
	indent       int
	memberIndent int
	form         int
	// member is the mapping-form member the reader is inside, and condition
	// the raw applies_when record while that member is open.
	member    string
	condition *rawCondition
}

// readLevels reads the levels block out of the raw frontmatter lines the way
// readLinks reads a card's links sequence, rather than by introducing a YAML
// parser for one key.
//
// Three declared syntaxes are accepted and they mix freely across axes within
// one block: a flow sequence on the axis's own line, whose entries are bare
// names; a block of dashed entries beneath the axis, where an entry is either
// a bare name or a name mapping to a one-line hint; and the mapping form,
// where the axis carries a values member taking either list spelling and an
// optional applies_when member. The reader tells the last two apart by the
// first non-blank line beneath an axis line with nothing after its colon: a
// dashed line opens the list form and a deeper key line opens the mapping
// form.
//
// Four rules bind. Declaration order within one axis is the rank, and nothing
// sorts the members. Ranks are counted within an axis and never across the
// block, so each axis's ranks run from zero however many members the sibling
// axes carry. A duplicate name within one axis keeps the first occurrence
// for both rank and lookup. A block carrying no parseable child leaves every
// axis undeclared and raises nothing, because the format's reader posture is
// to ignore what it cannot read rather than to fail, and a mapping-form axis
// whose values member is absent declares no set on the same terms.
func readLevels(fm *Frontmatter) levelsBlock {
	block := levelsBlock{axes: map[string][]Level{}, conditions: map[string]*rawCondition{}}
	var open *openAxis
	for _, line := range fm.Raw(LevelsKey) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := levelEntry.FindStringSubmatch(line); m != nil {
			block.readDashed(open, m[1])
			continue
		}
		m := levelAxis.FindStringSubmatch(line)
		if m == nil {
			open = nil
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " 	"))
		name, rest := m[1], unquote(strings.TrimSpace(m[2]))
		if open != nil && indent > open.indent {
			block.readNested(open, indent, name, rest)
			continue
		}
		// An axis outside LevelAxes takes no part in the model, and its own
		// nested lines go with it, which is what keeping it open as ignored
		// rather than forgetting it does.
		open = &openAxis{name: name, indent: indent, ignored: !KnownLevelAxis(name)}
		if open.ignored || rest == "" {
			continue
		}
		// A value on the axis line itself settles the list form, so a line
		// nested beneath it later is read as nothing.
		open.form = axisFormList
		for _, raw := range flowMembers(rest) {
			addLevel(block.axes, open.name, raw, "")
		}
	}
	if len(block.axes) == 0 {
		block.axes = nil
	}
	return block
}

// readDashed takes one dashed line, which is a level entry in the list form
// and beneath a values member, and the dashed spelling of an is member, which
// this reader does not accept.
func (block *levelsBlock) readDashed(open *openAxis, entry string) {
	if open == nil || open.ignored {
		return
	}
	if open.form == axisFormUnknown {
		open.form = axisFormList
	}
	switch {
	case open.form == axisFormList, open.member == levelValuesMember:
		name, hint := splitLevelEntry(entry)
		addLevel(block.axes, open.name, name, hint)
	case open.member == appliesWhenMember && open.condition != nil:
		open.condition.unreadable = true
	}
}

// readNested takes one key line beneath an open axis, which opens or continues
// the mapping form. A key line beneath an axis already in the list form is
// nothing the list form can carry and is skipped.
func (block *levelsBlock) readNested(open *openAxis, indent int, name, value string) {
	if open.ignored {
		return
	}
	if open.form == axisFormUnknown {
		open.form = axisFormMapping
		open.memberIndent = indent
		block.mappingForm = true
	}
	if open.form != axisFormMapping {
		return
	}
	if indent > open.memberIndent {
		if open.member == appliesWhenMember && open.condition != nil {
			open.condition.read(name, value)
		}
		return
	}
	open.member = name
	open.condition = nil
	switch name {
	case levelValuesMember:
		for _, raw := range flowMembers(value) {
			addLevel(block.axes, open.name, raw, "")
		}
	case appliesWhenMember:
		// Anything after the colon is a value that is not one block mapping,
		// which is the unreadable shape; the record is still kept so that a
		// gate naming this axis and the format check both see it.
		open.condition = &rawCondition{unreadable: value != ""}
		block.conditions[open.name] = open.condition
	}
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

// orderLevelAxes puts a levels member's axes in the one order Dinah publishes
// them in: the axes this model reads first, in LevelAxes order, then any
// further axis in sorted order after them. The ordering ranges over the axes
// actually declared, so a member declaring one axis yields one axis and no
// placeholder for the absent one, and an axis the model does not read is
// carried rather than dropped.
//
// Both sides of the interchange call this. Export orders the member it prints
// and the renderer below orders the block it writes, so a workbench declaring
// priority above severity exports one order and gets that same order back,
// and a second export matches the first. Two call sites that happen to agree
// today is what having one function avoids.
func orderLevelAxes(axes []string) []string {
	declared := map[string]bool{}
	for _, axis := range axes {
		declared[axis] = true
	}
	var further []string
	for _, axis := range axes {
		if !KnownLevelAxis(axis) {
			further = append(further, axis)
		}
	}
	sort.Strings(further)
	var ordered []string
	for _, axis := range LevelAxes {
		if declared[axis] {
			ordered = append(ordered, axis)
		}
	}
	return append(ordered, further...)
}

// orderedLevels reorders a levels member's axes by the rule above and re-emits
// the object, which is what makes the first export canonical before a clone is
// written from it.
//
// The member is re-emitted rather than round-tripped through a Go map, because
// a map sorts its keys and would quietly beat the rule. A member this cannot
// read, including the empty string a block with no readable child gives,
// travels on unchanged.
func orderedLevels(raw json.RawMessage) json.RawMessage {
	members, read := jsonMembers(raw)
	if !read {
		return raw
	}
	byAxis := map[string]json.RawMessage{}
	var axes []string
	for _, member := range members {
		byAxis[member.name] = member.value
		axes = append(axes, member.name)
	}
	var ordered []jsonMember
	for _, axis := range orderLevelAxes(axes) {
		ordered = append(ordered, jsonMember{name: axis, value: byAxis[axis]})
	}
	return jsonObject(ordered)
}

// renderLevelsMember renders an interchange definition's levels member as the
// block the reader above parses, and reports whether it could read it.
//
// Two rules bind. The axes are written in the order orderLevelAxes settles, so
// the render is deterministic whatever order the JSON object carried. Each
// axis's value is handed to renderBlock, which takes the flow form for an axis
// of bare names and the dashed form for one carrying a hint, since a flow
// entry has no spelling for a hint.
//
// A member this cannot read, meaning any axis renderBlock refuses, reports
// false, and the caller falls back to the raw JSON line every unrecognized
// member travels as, so nothing is lost.
func renderLevelsMember(raw json.RawMessage) ([]string, bool) {
	members, read := jsonMembers(raw)
	if !read {
		return nil, false
	}
	declared := map[string]json.RawMessage{}
	var axes []string
	for _, member := range members {
		declared[member.name] = member.value
		axes = append(axes, member.name)
	}
	lines := []string{LevelsKey + ":"}
	for _, axis := range orderLevelAxes(axes) {
		rendered, ok := renderBlock(axis, 2, orderAxisMembers(declared[axis]), false)
		if !ok {
			return nil, false
		}
		lines = append(lines, rendered...)
	}
	return lines, true
}

// orderAxisMembers puts a mapping-form axis's members in the one order the
// renderer writes them: values, then applies_when, then any further member
// in the order the object carried it. An axis in list form is an array and
// travels on unchanged, and so does anything jsonMembers cannot read.
func orderAxisMembers(raw json.RawMessage) json.RawMessage {
	if jsonShape(raw) != '{' {
		return raw
	}
	members, read := jsonMembers(raw)
	if !read {
		return raw
	}
	var ordered []jsonMember
	for _, first := range []string{levelValuesMember, appliesWhenMember} {
		for _, member := range members {
			if member.name == first {
				ordered = append(ordered, member)
			}
		}
	}
	for _, member := range members {
		if member.name != levelValuesMember && member.name != appliesWhenMember {
			ordered = append(ordered, member)
		}
	}
	return jsonObject(ordered)
}
