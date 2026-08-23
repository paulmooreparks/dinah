package bench

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"dinah/internal/contract"
)

// knownBenchKeys are the workbench frontmatter keys the interchange form
// carries under a name of its own. Every other key is an unrecognized member
// travelling through, which CORE-JSON-7 requires a tool to preserve.
// levels is one of them on both sides now: Instantiate renders it as the
// nested block the level model reads, and Export emits it explicitly beside
// profile and title with its axes in the one published order, rather than
// letting the loop below skip it for being known.
var knownBenchKeys = map[string]bool{
	"profile": true, "title": true, "states": true,
	"format": true, "slug": true, "operator": true,
	LevelsKey: true,
}

// knownStateKeys are the state frontmatter keys the interchange form carries
// under a name of its own.
var knownStateKeys = map[string]bool{
	"title": true, "kind": true, "operator_owned": true, "wip_limit": true,
	"slug": true,
}

// Export writes the interchange form of a bench definition.
//
// An unrecognized member is preserved by riding in the anchor's frontmatter
// under its own key, so a read and a write back carry it unchanged without
// any part of the tool having to understand it.
func (b *Bench) Export() ([]byte, error) {
	object := map[string]json.RawMessage{}
	for _, key := range b.FM.Keys() {
		if knownBenchKeys[key] {
			continue
		}
		object[key] = blockValue(b.FM, key)
	}
	object["profile"] = mustMarshal(b.Profile)
	object["title"] = mustMarshal(b.Title)
	if b.FM.Has(LevelsKey) {
		object[LevelsKey] = orderedLevels(blockValue(b.FM, LevelsKey))
	}
	if b.Standing != "" {
		object["instructions"] = mustMarshal(b.Standing)
	}
	states := make([]map[string]json.RawMessage, 0, len(b.States))
	for _, state := range b.States {
		states = append(states, exportState(state))
	}
	encoded, err := json.Marshal(states)
	if err != nil {
		return nil, err
	}
	object["states"] = encoded
	return json.MarshalIndent(object, "", "  ")
}

// exportState renders one state as an element of the states array.
func exportState(state *State) map[string]json.RawMessage {
	element := map[string]json.RawMessage{}
	for _, key := range state.FM.Keys() {
		if knownStateKeys[key] {
			continue
		}
		element[key] = blockValue(state.FM, key)
	}
	element["id"] = mustMarshal(state.ID)
	element["title"] = mustMarshal(state.Title)
	element["kind"] = mustMarshal(state.Kind)
	if state.Slug != "" {
		element["slug"] = mustMarshal(state.Slug)
	}
	if state.Instructions != "" {
		element["instructions"] = mustMarshal(state.Instructions)
	}
	if state.OperatorOwned {
		element["operator_owned"] = mustMarshal(true)
	}
	if state.Capacity > 0 {
		element["capacity"] = mustMarshal(state.Capacity)
	}
	return element
}

// mustMarshal encodes a value that cannot fail to encode: a string, a bool, an
// int, or a list of strings. Anything else has no business being an
// interchange member.
func mustMarshal(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return encoded
}

// Definition is an interchange object read back, before it becomes a bench on
// disk. It keeps every member it did not recognise.
type Definition struct {
	// Object is the whole interchange object as it was read.
	Object map[string]json.RawMessage
	// Title is the workbench title.
	Title string
	// Profile is the declared conformance target.
	Profile string
	// States are the flow, in the order the array carried.
	States []map[string]json.RawMessage
}

// ReadDefinition parses an interchange object and applies the checks the
// profile puts on one: an object missing profile, title or states is
// malformed, and so is a state element missing id, title or kind. A definition
// declaring a revision outside the window admitProfile applies is refused
// unsupported-version, on the same window Open applies, so the function that
// clones a workbench admits exactly what the function that opens one admits.
func ReadDefinition(data []byte) (*Definition, error) {
	object := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, contract.Refuse(contract.Malformed, "interchange")
	}
	definition := &Definition{Object: object}
	for _, member := range []string{"profile", "title", "states"} {
		if _, ok := object[member]; !ok {
			return nil, contract.Refuse(contract.Malformed, member)
		}
	}
	if err := json.Unmarshal(object["title"], &definition.Title); err != nil {
		return nil, contract.Refuse(contract.Malformed, "title")
	}
	if err := json.Unmarshal(object["profile"], &definition.Profile); err != nil {
		return nil, contract.Refuse(contract.Malformed, "profile")
	}
	if definition.Title == "" {
		return nil, contract.Refuse(contract.Malformed, "title")
	}
	if _, _, err := admitProfile(definition.Profile); err != nil {
		if errors.Is(err, errProfileMalformed) {
			return nil, contract.Refuse(contract.Malformed, "profile")
		}
		return nil, err
	}
	if err := json.Unmarshal(object["states"], &definition.States); err != nil {
		return nil, contract.Refuse(contract.Malformed, "states")
	}
	if len(definition.States) == 0 {
		return nil, contract.Refuse(contract.Malformed, "states")
	}
	for _, element := range definition.States {
		for _, member := range []string{"id", "title", "kind"} {
			if _, ok := element[member]; !ok {
				return nil, contract.Refuse(contract.Malformed, member)
			}
		}
	}
	return definition, nil
}

// memberString reads a member as a string, returning the empty string when it
// is absent or is not one.
func memberString(element map[string]json.RawMessage, member string) string {
	raw, ok := element[member]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

// Instantiate writes a bench to disk from an interchange definition. The
// identifiers of the source are kept, which is what makes intra-definition
// references survive and keeps benches born of one template comparable.
//
// The anchor declares ProfileVersion rather than the source's own profile,
// because CORE-BENCH section 2.3 asks a workbench to name the revision it was
// written against and a workbench this build writes was written against this
// build's revision. A template carrying an older claim still opens, and the
// workbench minted from it no longer inherits a retired spelling.
func Instantiate(root, slug, operator string, definition *Definition) error {
	if Exists(filepath.Join(root, WorkbenchAnchor)) {
		return contract.Refuse(contract.Exists, root)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	fm := NewFrontmatter()
	fm.Set("format", strconv.Itoa(StorageFormat))
	fm.Set("profile", ProfileVersion)
	fm.Set("title", definition.Title)
	if slug != "" {
		fm.Set("slug", slug)
	}
	if operator != "" {
		fm.Set("operator", operator)
	}
	slugs, err := assignStateSlugs(definition.States)
	if err != nil {
		return err
	}
	var ids []string
	for position, element := range definition.States {
		id := memberString(element, "id")
		if !IsID(id) {
			generated, err := NewID()
			if err != nil {
				return err
			}
			id = generated
		}
		if err := writeStateFromMember(root, id, slugs[position], element); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	fm.SetSeq("states", ids)
	if raw, ok := definition.Object[LevelsKey]; ok {
		if lines, readable := renderLevelsMember(raw); readable {
			fm.SetRaw(LevelsKey, lines)
		} else {
			fm.Set(LevelsKey, string(raw))
		}
	}
	for _, member := range sortedMembers(definition.Object) {
		if knownBenchKeys[member] {
			continue
		}
		writeMember(fm, member, definition.Object[member])
	}
	standing := ""
	if raw, ok := definition.Object["instructions"]; ok {
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			standing = text
			fm.Delete("instructions")
		}
	}
	if err := WriteText(filepath.Join(root, IgnoreName), ignoreLocks); err != nil {
		return err
	}
	return WriteText(filepath.Join(root, WorkbenchAnchor), fm.Render(standing))
}

// assignStateSlugs settles the slug every state of a definition is written
// with, in the order the states array carries them.
//
// A slug the author supplied is taken as given and checked, and two authors'
// slugs that collide are malformed rather than resolved by suffixing, because
// each one asked for a value that is not available. The explicit slugs are
// collected first so a derived slug never takes a value an author asked for
// later in the array. A slug the tool derives collides only with another
// derived one, and the second takes the first free suffix, so two states both
// titled Review become review and review-2.
//
// A title that derives to nothing usable is refused rather than left empty or
// filled in from the identifier, since neither is a name anybody would type.
// That refusal names the title, which is the value the author has to change,
// rather than the slug that was never arrived at.
func assignStateSlugs(states []map[string]json.RawMessage) ([]string, error) {
	slugs := make([]string, len(states))
	taken := map[string]bool{}
	for position, element := range states {
		slug := memberString(element, "slug")
		if slug == "" {
			continue
		}
		if !ValidStateSlug(slug) || taken[slug] {
			return nil, contract.Refuse(contract.Malformed, "slug "+slug)
		}
		taken[slug] = true
		slugs[position] = slug
	}
	for position, element := range states {
		if slugs[position] != "" {
			continue
		}
		title := memberString(element, "title")
		derived := SlugifyDashed(title)
		if derived == "" {
			return nil, contract.Refuse(contract.Malformed, "title "+title)
		}
		candidate := derived
		for suffix := 2; taken[candidate]; suffix++ {
			candidate = derived + "-" + strconv.Itoa(suffix)
		}
		taken[candidate] = true
		slugs[position] = candidate
	}
	return slugs, nil
}

// writeStateFromMember writes one state anchor from an interchange element,
// carrying the slug the assignment settled for it.
func writeStateFromMember(root, id, slug string, element map[string]json.RawMessage) error {
	fm := NewFrontmatter()
	fm.Set("title", memberString(element, "title"))
	fm.Set("slug", slug)
	fm.Set("kind", memberString(element, "kind"))
	var operatorOwned bool
	if raw, ok := element["operator_owned"]; ok {
		if err := json.Unmarshal(raw, &operatorOwned); err == nil && operatorOwned {
			fm.Set("operator_owned", "true")
		}
	}
	var capacity int
	if raw, ok := element["capacity"]; ok {
		if err := json.Unmarshal(raw, &capacity); err == nil && capacity > 0 {
			fm.Set("wip_limit", strconv.Itoa(capacity))
		}
	}
	for _, member := range sortedMembers(element) {
		if knownStateKeys[member] || member == "id" || member == "capacity" || member == "instructions" {
			continue
		}
		writeMember(fm, member, element[member])
	}
	return WriteText(filepath.Join(root, StatesDir, id, StateAnchor), fm.Render(memberString(element, "instructions")))
}

// sortedMembers names an interchange object's members in sorted order. Both
// import loops ranged over a Go map before, so two clones of one definition
// differed in frontmatter key order for no reason a reader could predict and
// no byte comparison of a round trip was possible.
func sortedMembers(object map[string]json.RawMessage) []string {
	names := make([]string, 0, len(object))
	for name := range object {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// writeMember writes one unrecognized member into an anchor's frontmatter as
// the block of its own shape, falling back to the one raw JSON line when the
// renderer refuses the value. The fallback loses nothing, because a line that
// parses as JSON reads back as the JSON it carried.
func writeMember(fm *Frontmatter, member string, raw json.RawMessage) {
	if lines, renderable := renderBlock(member, 0, raw); renderable {
		fm.SetRaw(member, lines)
		return
	}
	fm.Set(member, string(raw))
}

// Extract copies a bench's definition into a new directory and leaves the
// work behind, which is the cards, the workstreams, the journals and the
// archive. Identifiers are kept, so a bench instantiated from the result is
// structurally the bench it came from.
//
// The existence check ahead of the write tests Exists rather than
// AnchorRecognized on purpose: this call overwrites whatever file sits at
// the target path, so the question is whether a write here would destroy
// somebody's file, not whether that file happens to be a Dinah workbench.
func (b *Bench) Extract(target string) error {
	if Exists(filepath.Join(target, WorkbenchAnchor)) {
		return contract.Refuse(contract.Exists, target)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	anchor, err := ReadText(filepath.Join(b.Root, WorkbenchAnchor))
	if err != nil {
		return err
	}
	if err := WriteText(filepath.Join(target, WorkbenchAnchor), anchor); err != nil {
		return err
	}
	for _, state := range b.States {
		source := filepath.Join(b.Root, StatesDir, state.ID, StateAnchor)
		text, err := ReadText(source)
		if err != nil {
			return err
		}
		if err := WriteText(filepath.Join(target, StatesDir, state.ID, StateAnchor), text); err != nil {
			return err
		}
	}
	return nil
}
