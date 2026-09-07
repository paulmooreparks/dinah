package bench

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// relativeTier matches a relative tier expression: a sign and a rung count,
// which is the whole of the syntax. Anything else is an absolute member name
// and is checked against the declared set rather than computed.
var relativeTier = regexp.MustCompile(`^([+-])([0-9]+)$`)

// ResolveTierWrite turns what somebody typed into the absolute member name a
// tier override stores, and reports what that resolution was measured
// against.
//
// Storage is absolute because the tier set is open and ordered by declaration.
// A stored "+1" would name a different rung the day somebody inserts a member
// into the middle of the set, on cards nobody touched, so the convenience
// lives in this function and never on disk. The expression and the value it
// resolved against travel on the journal event the caller writes, which is
// where a later reader learns what a person actually asked for.
//
// An absolute expression never reads the column at all, so a column carrying
// no default, or carrying a stale one, refuses nothing here. A relative
// expression reads it and has three outcomes:
//
//   - the column carries no default, so there is nothing to be relative to,
//     and the write refuses under contract.NoTierDefault;
//   - the column carries a default the workbench's tier set does not declare,
//     so the rung it names is unknown, and the write refuses under
//     contract.UnknownLevel naming that stored value;
//   - the column carries a default that resolves, so the answer is that
//     default's rank moved by the rung count, and a result outside the
//     declared set refuses under contract.TierOutOfRange.
//
// The against value is the column's own default at the moment of resolution,
// and it is empty for an absolute expression, which needed no baseline.
func (b *Bench) ResolveTierWrite(column *Column, expr string) (absolute, against string, refusal *contract.Refusal) {
	levels := b.Levels(TierField)
	if len(levels) == 0 {
		return "", "", contract.RefuseWith(contract.NoLevels, TierField, map[string]string{
			"axis":   TierField,
			"anchor": filepath.Join(b.Root, WorkbenchAnchor),
		})
	}
	match := relativeTier.FindStringSubmatch(strings.TrimSpace(expr))
	if match == nil {
		if b.Level(TierField, expr) == nil {
			return "", "", unknownTierLevel(expr, levels)
		}
		return expr, "", nil
	}
	if column == nil || column.Tier == "" {
		detail := ""
		if column != nil {
			detail = column.Ref()
		}
		return "", "", contract.RefuseWith(contract.NoTierDefault, detail, map[string]string{
			"column": detail,
			"expr":   expr,
		})
	}
	base := b.Level(TierField, column.Tier)
	if base == nil {
		return "", "", unknownTierLevel(column.Tier, levels)
	}
	steps, err := strconv.Atoi(match[2])
	if err != nil {
		return "", "", unknownTierLevel(expr, levels)
	}
	if match[1] == "-" {
		steps = -steps
	}
	wanted := base.Rank + steps
	if wanted < 0 || wanted >= len(levels) {
		return "", "", contract.RefuseWith(contract.TierOutOfRange, expr, map[string]string{
			"attempted": strconv.Itoa(wanted),
			"levels":    strings.Join(LevelNames(levels), ", "),
		})
	}
	return levels[wanted].Name, column.Tier, nil
}

// unknownTierLevel is the refusal a value outside the declared tier set
// carries, whichever side of the write it arrived from: what a person typed,
// or what a column's own frontmatter stores. Both are the same defect from a
// reader's point of view, which is a name the workbench does not declare, so
// both raise the name the workbench already uses for it.
func unknownTierLevel(value string, levels []Level) *contract.Refusal {
	return contract.RefuseWith(contract.UnknownLevel, value, map[string]string{
		"axis":   TierField,
		"levels": strings.Join(LevelNames(levels), ", "),
	})
}

// TierRank is where a declared tier sits in the workbench's own order, and
// false for a name the workbench does not declare, which includes the empty
// string a caller who declared nothing brings.
//
// A caller comparing two tiers reads this rather than comparing the names,
// because the order is the workbench's declaration order and nothing about the
// names themselves carries it.
func (b *Bench) TierRank(name string) (int, bool) {
	if name == "" {
		return 0, false
	}
	level := b.Level(TierField, name)
	if level == nil {
		return 0, false
	}
	return level.Rank, true
}
