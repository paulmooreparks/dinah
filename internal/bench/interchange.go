package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"dinah/internal/contract"
)

// knownBenchKeys are the workbench frontmatter keys the interchange form
// carries under a name of its own. Every other key is an unrecognized member
// travelling through, which CORE-JSON-7 requires a tool to preserve.
var knownBenchKeys = map[string]bool{
	"profile": true, "title": true, "states": true,
	"format": true, "slug": true, "operator": true,
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
		object[key] = rawValue(b.FM.Value(key))
	}
	object["profile"] = mustMarshal(b.Profile)
	object["title"] = mustMarshal(b.Title)
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
		element[key] = rawValue(state.FM.Value(key))
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

// rawValue reads a preserved member back. A value that still parses as JSON
// travels as the JSON it was; anything else travels as a string, which is
// what a hand-written frontmatter value is.
func rawValue(stored string) json.RawMessage {
	if json.Valid([]byte(stored)) {
		return json.RawMessage(stored)
	}
	return mustMarshal(stored)
}

// mustMarshal encodes a value that cannot fail to encode: a string, a bool or
// an int. Anything else has no business being an interchange member.
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
// malformed, and so is a state element missing id, title or kind.
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
	major, _, ok := splitProfile(definition.Profile)
	if !ok {
		return nil, contract.Refuse(contract.Malformed, "profile")
	}
	if major != ProfileMajor {
		return nil, contract.Refuse(contract.UnsupportedVer, definition.Profile)
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
func Instantiate(root, slug, operator string, definition *Definition) error {
	if Exists(filepath.Join(root, WorkbenchAnchor)) {
		return contract.Refuse(contract.Exists, root)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	fm := NewFrontmatter()
	fm.Set("format", strconv.Itoa(StorageFormat))
	fm.Set("profile", definition.Profile)
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
	for member, raw := range definition.Object {
		if knownBenchKeys[member] {
			continue
		}
		fm.Set(member, string(raw))
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
	for member, raw := range element {
		if knownStateKeys[member] || member == "id" || member == "capacity" || member == "instructions" {
			continue
		}
		fm.Set(member, string(raw))
	}
	return WriteText(filepath.Join(root, StatesDir, id, StateAnchor), fm.Render(memberString(element, "instructions")))
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
