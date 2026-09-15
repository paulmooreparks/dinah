package bench

import (
	"encoding/json"
	"regexp"
	"strings"
)

// TiersKey is the frontmatter key carrying the workbench's tier table, a
// top-level block and a sibling of the levels and fields blocks.
const TiersKey = "tiers"

// The two members one tier entry carries beneath its own key.
const (
	tierMeaningMember = "meaning"
	tierModelsMember  = "models"
)

// The three members one model entry carries inside its flow mapping. A member
// outside this set makes the entry malformed, which check reports.
const (
	tierProviderMember = "provider"
	tierModelMember    = "model"
	tierServerMember   = "server"
)

// TierModel is one model entry of the table: what the caller has to declare
// for this entry to match.
//
// Every value is compared byte for byte after the unquote and the trim. No
// wildcard is admitted and no case folding is performed, because a pattern
// would promote whatever ships next without anybody deciding to, and a
// provider that distinguishes two models by case would be silently collapsed.
type TierModel struct {
	// Provider is the provider name, required.
	Provider string
	// Model is the model identifier, required.
	Model string
	// Server is the address the model is reached at, optional. An entry
	// declaring none matches a caller whatever server it declared.
	Server string
}

// Render writes one entry the way a refusal and an offer name it:
// provider/model, or provider/model@server where the entry declares one.
func (m TierModel) Render() string {
	rendered := m.Provider + "/" + m.Model
	if m.Server != "" {
		rendered += "@" + m.Server
	}
	return rendered
}

// TierEntry is one rung of the table: the tier it names, the line of prose
// that says what the rung is for, and the models that satisfy it.
type TierEntry struct {
	// Tier is the member of levels.tier this entry declares models for.
	Tier string
	// Meaning is the one line of prose the entry carries for a reader.
	Meaning string
	// Models are the entries that satisfy this rung, in declaration order.
	Models []TierModel
}

// MalformedTierEntry is one entry the reader refused, named so that check can
// put the tier and the line in front of somebody.
type MalformedTierEntry struct {
	// Tier is the key the entry stood under, empty where the malformed line
	// stood under no readable key at all.
	Tier string
	// Line is the line as it stands in the anchor, trimmed.
	Line string
}

// Detail renders one refused entry the way a finding names it: the tier, then
// the line, in that order.
func (e MalformedTierEntry) Detail() string {
	if e.Tier == "" {
		return e.Line
	}
	return e.Tier + " " + e.Line
}

// tierBlockMember matches one `name:` line inside the tiers block, keeping the
// indentation that says which level of the block the line belongs to. It is
// fieldBlockMember's shape, and the two blocks are read by the same rule: an
// entry key and a member name are the same shape of line, and only their depth
// separates them.
var tierBlockMember = regexp.MustCompile(`^( +)([^\s:][^:]*):(.*)$`)

// tierBlockEntry matches one dashed model entry beneath a models member.
var tierBlockEntry = regexp.MustCompile(`^( *)-\s*(.*)$`)

// readTiers reads the workbench's tiers block, answering the entries it
// declares in declaration order and the entries it refused.
//
// The reader posture is readDeclaredFields's: a line it cannot read leaves its
// entry undeclared and is reported rather than raised over, so a hand-damaged
// block leaves the workbench openable and dinah check is what puts the damage
// in front of a reader. A duplicate tier key keeps its first occurrence, which
// is the rule addLevel already keeps for a repeated level name.
func readTiers(fm *Frontmatter) ([]TierEntry, []MalformedTierEntry) {
	var entries []TierEntry
	var malformed []MalformedTierEntry
	seen := map[string]bool{}
	entryIndent := -1
	current := -1
	member := ""
	for _, line := range fm.Raw(TiersKey) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := tierBlockEntry.FindStringSubmatch(line); m != nil {
			tier := ""
			if current >= 0 {
				tier = entries[current].Tier
			}
			if current < 0 || member != tierModelsMember || len(m[1]) <= entryIndent {
				malformed = append(malformed, MalformedTierEntry{Tier: tier, Line: strings.TrimSpace(line)})
				continue
			}
			model, ok := readTierModel(m[2])
			if !ok {
				malformed = append(malformed, MalformedTierEntry{Tier: tier, Line: strings.TrimSpace(line)})
				continue
			}
			entries[current].Models = append(entries[current].Models, model)
			continue
		}
		m := tierBlockMember.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent, name, rest := len(m[1]), unquote(strings.TrimSpace(m[2])), m[3]
		if entryIndent < 0 {
			entryIndent = indent
		}
		if indent <= entryIndent {
			member, current = "", -1
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			entries = append(entries, TierEntry{Tier: name})
			current = len(entries) - 1
			continue
		}
		if current < 0 {
			continue
		}
		member = name
		if member == tierMeaningMember {
			entries[current].Meaning = unquote(strings.TrimSpace(rest))
		}
	}
	declared := make([]TierEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Meaning) == "" || len(entry.Models) == 0 {
			malformed = append(malformed, MalformedTierEntry{Tier: entry.Tier, Line: entry.Tier + ":"})
			continue
		}
		declared = append(declared, entry)
	}
	return declared, malformed
}

// readTierModel reads one dashed model entry, which is one flow mapping on one
// line and nothing else.
//
// The reading is mechanical. The remainder has to open with a brace and close
// with one, the interior is split on commas, each part is split on its first
// colon, and the existing unquote runs over the key and the value. Splitting a
// part on its first colon is what lets a model named qwen3:235b be written
// without quotes, and quoting it works the same way.
//
// A comma is not representable in a value, and quoting does not rescue one.
// The split on commas runs before the unquote, exactly as Frontmatter.Seq
// already splits a flow sequence, so a quoted value carrying a comma is cut in
// half and the entry is malformed. Nothing in a provider name, a model name or
// a server address carries a comma, and the alternative is a quoting-aware
// splitter for a case nobody has.
func readTierModel(text string) (TierModel, bool) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return TierModel{}, false
	}
	var model TierModel
	interior := strings.TrimSuffix(strings.TrimPrefix(trimmed, "{"), "}")
	for _, part := range strings.Split(interior, ",") {
		key, value, mapped := strings.Cut(part, ":")
		if !mapped {
			return TierModel{}, false
		}
		switch unquote(strings.TrimSpace(key)) {
		case tierProviderMember:
			model.Provider = unquote(strings.TrimSpace(value))
		case tierModelMember:
			model.Model = unquote(strings.TrimSpace(value))
		case tierServerMember:
			model.Server = unquote(strings.TrimSpace(value))
		default:
			return TierModel{}, false
		}
	}
	if model.Provider == "" || model.Model == "" {
		return TierModel{}, false
	}
	return model, true
}

// Tiers are the tier table's entries in the order the anchor declares them,
// and nil on a workbench declaring no table.
func (b *Bench) Tiers() []TierEntry {
	return append([]TierEntry(nil), b.tiers...)
}

// DeclaresTierTable reports whether the workbench declares a tiers block at
// all, which is how a workbench asks for the claim gate. A workbench that has
// not written one has not asked, and refuses no claim on tier grounds.
//
// It reads whether the anchor carries the key rather than whether any entry
// survived the read, so a block whose every entry is malformed counts as a
// table somebody asked for and check reports, rather than as a workbench that
// never wrote one.
func (b *Bench) DeclaresTierTable() bool {
	return b.FM.Has(TiersKey)
}

// MalformedTierEntries are the entries the reader refused, which dinah check
// reports and nothing else reads.
func (b *Bench) MalformedTierEntries() []MalformedTierEntry {
	return append([]MalformedTierEntry(nil), b.malformedTiers...)
}

// TierOf resolves the rung a caller's declared provider, model and server sit
// at, and reports false where it resolves to nothing.
//
// A caller declaring no provider or no model resolves to nothing. A caller
// declaring no server is not refused here, because the second pass is written
// for exactly that caller.
//
// The matching runs in two passes, which is what keeps the answer independent
// of the order somebody wrote the table in. The first pass looks for an entry
// whose provider, model and server all equal what the caller declared; the
// second, run only where the first found nothing, looks for an entry whose
// provider and model equal what the caller declared and which declares no
// server at all. So a specific entry always beats a general one, and a
// workbench can list one model at two rungs for two addresses without the two
// fighting over declaration order.
//
// Within one pass the first match in declaration order wins, which is
// addLevel's rule for a repeated name.
func (b *Bench) TierOf(provider, model, server string) (string, bool) {
	if provider == "" || model == "" {
		return "", false
	}
	if server != "" {
		for _, entry := range b.tiers {
			for _, candidate := range entry.Models {
				if candidate.Provider == provider && candidate.Model == model && candidate.Server == server {
					return entry.Tier, true
				}
			}
		}
	}
	for _, entry := range b.tiers {
		for _, candidate := range entry.Models {
			if candidate.Provider == provider && candidate.Model == model && candidate.Server == "" {
				return entry.Tier, true
			}
		}
	}
	return "", false
}

// SatisfyingModels are every entry of the table at or above one rung, in
// levels.tier order from the lowest satisfying rung upward, so the cheapest
// model that would do the work is first. It is what an offer's satisfied_by
// carries and what the three tier refusals render into a sentence.
//
// A rung the workbench does not declare satisfies nothing, which is TierRank's
// own answer rather than a rule decided here.
func (b *Bench) SatisfyingModels(required string) []TierModel {
	floor, known := b.TierRank(required)
	if !known {
		return nil
	}
	var satisfying []TierModel
	for _, level := range b.Levels(TierField) {
		if level.Rank < floor {
			continue
		}
		for _, entry := range b.tiers {
			if entry.Tier != level.Name {
				continue
			}
			satisfying = append(satisfying, entry.Models...)
		}
	}
	return satisfying
}

// RenderTierModels writes a list of entries the way a refusal's context
// carries them, joined by a comma and a space in the order they were given.
func RenderTierModels(models []TierModel) string {
	rendered := make([]string, 0, len(models))
	for _, model := range models {
		rendered = append(rendered, model.Render())
	}
	return strings.Join(rendered, ", ")
}

// tierInterchange is one tier's entry in the interchange form, which is the
// shape the renderer below writes back into the anchor's flow form.
type tierInterchange struct {
	Meaning string              `json:"meaning"`
	Models  []tierModelExchange `json:"models"`
}

// tierModelExchange is one model entry of that form.
type tierModelExchange struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Server   string `json:"server,omitempty"`
}

// renderTiersMember renders an interchange definition's tiers member as the
// block readTiers parses, and reports whether it could read it.
//
// The model entries are written back in the flow form the reader takes, so a
// workbench exported and imported comes back byte-stable. A member this cannot
// read reports false, and the caller falls back to the one raw JSON line every
// unrecognized member travels as, so nothing is lost.
func renderTiersMember(raw json.RawMessage) ([]string, bool) {
	members, read := jsonMembers(raw)
	if !read {
		return nil, false
	}
	lines := []string{TiersKey + ":"}
	for _, member := range members {
		var entry tierInterchange
		if err := json.Unmarshal(member.value, &entry); err != nil {
			return nil, false
		}
		lines = append(lines, "  "+member.name+":")
		lines = append(lines, "    "+tierMeaningMember+": "+quote(entry.Meaning))
		lines = append(lines, "    "+tierModelsMember+":")
		for _, model := range entry.Models {
			flow := "{" + tierProviderMember + ": " + quote(model.Provider) +
				", " + tierModelMember + ": " + quote(model.Model)
			if model.Server != "" {
				flow += ", " + tierServerMember + ": " + quote(model.Server)
			}
			lines = append(lines, "      - "+flow+"}")
		}
	}
	return lines, true
}

// ExportTiers is the tiers member an export writes, and false on a workbench
// declaring no block.
//
// The member is built from the entries the reader declared rather than from
// the anchor's raw lines, because the generic block reader has no spelling for
// a flow mapping inside a dashed entry and would read one as a one-member
// object of nonsense. The keys are written in levels.tier declaration order,
// so a second export matches the first whatever order the anchor carried, and
// a key the tier axis does not declare follows the declared ones in the order
// the anchor carried it rather than being dropped.
func (b *Bench) ExportTiers() (json.RawMessage, bool) {
	if !b.FM.Has(TiersKey) {
		return nil, false
	}
	var members []jsonMember
	written := map[string]bool{}
	for _, level := range b.Levels(TierField) {
		for _, entry := range b.tiers {
			if entry.Tier != level.Name || written[entry.Tier] {
				continue
			}
			written[entry.Tier] = true
			members = append(members, jsonMember{name: entry.Tier, value: mustMarshal(exchangeTier(entry))})
		}
	}
	for _, entry := range b.tiers {
		if written[entry.Tier] {
			continue
		}
		written[entry.Tier] = true
		members = append(members, jsonMember{name: entry.Tier, value: mustMarshal(exchangeTier(entry))})
	}
	return jsonObject(members), true
}

// exchangeTier is one entry in the interchange shape.
func exchangeTier(entry TierEntry) tierInterchange {
	exchanged := tierInterchange{Meaning: entry.Meaning}
	for _, model := range entry.Models {
		exchanged.Models = append(exchanged.Models, tierModelExchange{
			Provider: model.Provider, Model: model.Model, Server: model.Server,
		})
	}
	return exchanged
}
