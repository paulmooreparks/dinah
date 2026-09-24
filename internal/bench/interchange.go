package bench

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// knownBenchKeys are the workbench frontmatter keys the interchange form
// carries under a name of its own. Every other key is an unrecognized member
// travelling through, which CORE-JSON-7 requires a tool to preserve.
// levels is one of them on both sides now: Instantiate renders it as the
// nested block the level model reads, and Export emits it explicitly beside
// profile and title with its axes in the one published order, rather than
// letting the loop below skip it for being known.
//
// The declaration block and the workbench's own declared field values are two
// more. They are named apart because one object carries both, and a member
// named fields holding sometimes a declaration and sometimes values would be a
// member no reader could write code against.
var knownBenchKeys = map[string]bool{
	"profile": true, "title": true, "columns": true,
	"format": true, "slug": true, "operator": true,
	LevelsKey: true, FieldsKey: true, FieldValuesKey: true, TiersKey: true,
}

// knownColumnKeys are the column frontmatter keys the interchange form carries
// under a name of its own. AttachmentsMember is among them so that the member
// carrying a column's attachments is never written into its frontmatter.
var knownColumnKeys = map[string]bool{
	"title": true, "kind": true, "operator_owned": true, "wip_limit": true,
	"slug": true, "awaiting_outside": true, "gate_items": true,
	FieldValuesKey: true, RequireFieldsKey: true, StandingItemsKey: true,
	AttachmentsMember: true,
}

// AttachmentsMember is the column element member carrying the column's live
// attachments with their bytes. The profile does not list it, so a second tool
// preserves it under CORE-JSON-7 on the path reject_to and routes take.
const AttachmentsMember = "attachments"

// DefinitionAttachment is one element of a column's attachments member, with
// its payload decoded. The wire form carries the payload in the standard
// base64 encoding of RFC 4648 section 4, with padding, which is what
// encoding/base64.StdEncoding documents.
type DefinitionAttachment struct {
	// Filename is the payload's name, which ValidAttachmentName accepts.
	Filename string
	// Description is the optional prose describing the attachment.
	Description string
	// Provenance says where the bytes came from, empty where the element
	// carries none.
	Provenance string
	// Payload is the attachment's bytes.
	Payload []byte
}

// wireAttachment is the encoded form of one DefinitionAttachment. The field
// order is the order encoding/json writes a struct's fields in, which is the
// order the member documents.
type wireAttachment struct {
	Filename    string `json:"filename"`
	Description string `json:"description,omitempty"`
	Provenance  string `json:"provenance,omitempty"`
	Payload     string `json:"payload"`
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
	// Both blocks travel as the nested value they already are, read by the
	// one reader every structured frontmatter value is read by, so the
	// declaration order the file carries survives the trip.
	// The tier table travels as the entries the reader declared rather than
	// as the anchor's raw lines, because a model entry is a flow mapping
	// inside a dashed entry and the generic block reader has no spelling for
	// one. ExportTiers is where that is argued and where the key order is
	// settled.
	if tiers, declared := b.ExportTiers(); declared {
		object[TiersKey] = tiers
	}
	if b.FM.Has(FieldsKey) {
		object[FieldsKey] = blockValue(b.FM, FieldsKey)
	}
	if b.FM.Has(FieldValuesKey) {
		object[FieldValuesKey] = blockValue(b.FM, FieldValuesKey)
	}
	if b.Standing != "" {
		object["instructions"] = mustMarshal(b.Standing)
	}
	columns := make([]map[string]json.RawMessage, 0, len(b.Columns))
	for _, column := range b.Columns {
		element, err := b.exportColumn(column)
		if err != nil {
			return nil, err
		}
		columns = append(columns, element)
	}
	encoded, err := json.Marshal(columns)
	if err != nil {
		return nil, err
	}
	object["columns"] = encoded
	return json.MarshalIndent(object, "", "  ")
}

// exportColumn renders one column as an element of the columns array. A
// payload that will not read fails the export rather than being dropped,
// because a template quietly missing a file is the loss the attachments member
// exists to prevent.
func (b *Bench) exportColumn(column *Column) (map[string]json.RawMessage, error) {
	element := map[string]json.RawMessage{}
	for _, key := range column.FM.Keys() {
		if knownColumnKeys[key] {
			continue
		}
		element[key] = blockValue(column.FM, key)
	}
	element["id"] = mustMarshal(column.ID)
	element["title"] = mustMarshal(column.Title)
	element["kind"] = mustMarshal(column.Kind)
	if column.Slug != "" {
		element["slug"] = mustMarshal(column.Slug)
	}
	if column.Instructions != "" {
		element["instructions"] = mustMarshal(column.Instructions)
	}
	if column.OperatorOwned {
		element["operator_owned"] = mustMarshal(true)
	}
	// The member is written only where the flag is set, so a column that does
	// not declare it exports as a column that never heard of it, which is the
	// same shape operator_owned carries above.
	if column.AwaitingOutside {
		element["awaiting_outside"] = mustMarshal(true)
	}
	// CORE-JSON-10 blesses this member, so it travels under a name of its
	// own rather than as an unrecognized member somebody else's tool
	// preserves without understanding. It carries either the boolean true,
	// which holds a card entering the column and is unchanged from the
	// two-value vocabulary, or the string out or both, which is Dinah's own
	// extension of the same member's value set.
	switch column.Hold {
	case HoldOn:
		element["gate_items"] = mustMarshal(true)
	case HoldOut:
		element["gate_items"] = mustMarshal("out")
	case HoldBoth:
		element["gate_items"] = mustMarshal("both")
	}
	if column.Capacity > 0 {
		element["capacity"] = mustMarshal(column.Capacity)
	}
	if column.FM.Has(FieldValuesKey) {
		element[FieldValuesKey] = blockValue(column.FM, FieldValuesKey)
	}
	if len(column.RequireFields) > 0 {
		element[RequireFieldsKey] = mustMarshal(column.RequireFields)
	}
	// CORE-JSON-14 blesses this member. It travels as the nested value the
	// anchor already carries, read by the one reader every structured
	// frontmatter value is read by, so the declaration order the file
	// carries survives the trip and a second export of the import is
	// byte-identical to the first.
	if column.FM.Has(StandingItemsKey) {
		element[StandingItemsKey] = blockValue(column.FM, StandingItemsKey)
	}
	attachments, err := exportAttachments(b.ColumnDir(column.ID))
	if err != nil {
		return nil, err
	}
	if attachments != nil {
		element[AttachmentsMember] = attachments
	}
	return element, nil
}

// exportAttachments encodes the live attachments of one column directory as
// the attachments member, in creation order, and answers nil where the column
// carries none, so a column carrying no attachment exports exactly as it did
// before the member existed.
func exportAttachments(columnDir string) (json.RawMessage, error) {
	attachments, err := Attachments(columnDir)
	if err != nil {
		return nil, err
	}
	if len(attachments) == 0 {
		return nil, nil
	}
	wire := make([]wireAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		if attachment.Path == "" {
			return nil, contract.Refuse(contract.UnknownPath, filepath.Join(attachment.Dir, PayloadDir))
		}
		payload, err := os.ReadFile(attachment.Path)
		if err != nil {
			return nil, err
		}
		element := wireAttachment{
			Filename:    attachment.Filename,
			Description: attachment.Description,
			Provenance:  attachment.Provenance,
			Payload:     base64.StdEncoding.EncodeToString(payload),
		}
		wire = append(wire, element)
	}
	return json.Marshal(wire)
}

// ColumnAttachmentsOf reads a column element's attachments member, answering
// nil where the element carries none. The member is malformed when it is not
// an array, when an element is not an object, when filename is absent or
// ValidAttachmentName refuses it, when payload is absent or does not decode,
// or when description or provenance is present and not a string, and the
// refusal is malformed with the detail attachments in every case.
func ColumnAttachmentsOf(element map[string]json.RawMessage) ([]DefinitionAttachment, error) {
	raw, ok := element[AttachmentsMember]
	if !ok {
		return nil, nil
	}
	malformed := contract.Refuse(contract.Malformed, AttachmentsMember)
	if !jsonKind(raw, '[') {
		return nil, malformed
	}
	var members []json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, malformed
	}
	var attachments []DefinitionAttachment
	for _, member := range members {
		attachment, ok := definitionAttachment(member)
		if !ok {
			return nil, malformed
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

// definitionAttachment decodes one element of an attachments member, and
// reports false where the element breaks any rule ColumnAttachmentsOf states.
func definitionAttachment(raw json.RawMessage) (DefinitionAttachment, bool) {
	var attachment DefinitionAttachment
	if !jsonKind(raw, '{') {
		return attachment, false
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return attachment, false
	}
	filename, ok := stringMember(object, "filename", true)
	if !ok || !ValidAttachmentName(filename) {
		return attachment, false
	}
	encoded, ok := stringMember(object, "payload", true)
	if !ok {
		return attachment, false
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return attachment, false
	}
	description, ok := stringMember(object, "description", false)
	if !ok {
		return attachment, false
	}
	provenance, ok := stringMember(object, "provenance", false)
	if !ok {
		return attachment, false
	}
	attachment = DefinitionAttachment{
		Filename:    filename,
		Description: description,
		Provenance:  provenance,
		Payload:     payload,
	}
	return attachment, true
}

// stringMember reads one member of an object as a JSON string. It reports
// false where the member is present and is not a string, and where it is
// absent and required; an absent optional member reads as the empty string.
func stringMember(object map[string]json.RawMessage, name string, required bool) (string, bool) {
	raw, present := object[name]
	if !present {
		return "", !required
	}
	if !jsonKind(raw, '"') {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// jsonKind reports whether a JSON value's first byte past any leading
// whitespace is the one given, which is how a string, an object and an array
// are told apart from null and from each other before decoding.
func jsonKind(raw json.RawMessage, first byte) bool {
	trimmed := bytes.TrimLeft(raw, jsonSpace)
	return len(trimmed) > 0 && trimmed[0] == first
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
	// Columns are the flow, in the order the array carried.
	Columns []map[string]json.RawMessage
}

// ReadDefinition parses an interchange object and applies the checks the
// profile puts on one: an object missing profile, title or columns is
// malformed, and so is a column element missing id, title or kind. A definition
// declaring a revision outside the window admitProfile applies is refused
// unsupported-version, on the same window Open applies, so the function that
// clones a workbench admits exactly what the function that opens one admits.
func ReadDefinition(data []byte) (*Definition, error) {
	normalized, err := normalizeDefinitionText(data)
	if err != nil {
		return nil, err
	}
	data = normalized
	object := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, contract.Refuse(contract.Malformed, "interchange")
	}
	definition := &Definition{Object: object}
	for _, member := range []string{"profile", "title", "columns"} {
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
	if err := json.Unmarshal(object["columns"], &definition.Columns); err != nil {
		return nil, contract.Refuse(contract.Malformed, "columns")
	}
	if len(definition.Columns) == 0 {
		return nil, contract.Refuse(contract.Malformed, "columns")
	}
	for _, element := range definition.Columns {
		for _, member := range []string{"id", "title", "kind"} {
			if _, ok := element[member]; !ok {
				return nil, contract.Refuse(contract.Malformed, member)
			}
		}
		if _, err := ColumnAttachmentsOf(element); err != nil {
			return nil, err
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
	slugs, err := assignColumnSlugs(definition.Columns)
	if err != nil {
		return err
	}
	var ids []string
	for position, element := range definition.Columns {
		id := memberString(element, "id")
		if !IsID(id) {
			generated, err := NewID()
			if err != nil {
				return err
			}
			id = generated
		}
		if err := writeColumnFromMember(root, id, slugs[position], element); err != nil {
			return err
		}
		if err := writeColumnAttachments(filepath.Join(root, ColumnsDir, id), operator, element); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	fm.SetSeq("columns", ids)
	if raw, ok := definition.Object[LevelsKey]; ok {
		if lines, readable := renderLevelsMember(raw); readable {
			fm.SetRaw(LevelsKey, lines)
		} else {
			fm.Set(LevelsKey, string(raw))
		}
	}
	if raw, ok := definition.Object[TiersKey]; ok {
		if lines, readable := renderTiersMember(raw); readable {
			fm.SetRaw(TiersKey, lines)
		} else {
			fm.Set(TiersKey, string(raw))
		}
	}
	for _, member := range []string{FieldsKey, FieldValuesKey} {
		if raw, ok := definition.Object[member]; ok {
			writeMember(fm, member, raw)
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
	if err := WriteText(filepath.Join(root, AttributesName), unionJournals); err != nil {
		return err
	}
	return WriteText(filepath.Join(root, WorkbenchAnchor), fm.Render(standing))
}

// assignColumnSlugs settles the slug every column of a definition is written
// with, in the order the columns array carries them.
//
// A slug the author supplied is taken as given and checked, and two authors'
// slugs that collide are malformed rather than resolved by suffixing, because
// each one asked for a value that is not available. The explicit slugs are
// collected first so a derived slug never takes a value an author asked for
// later in the array. A slug the tool derives collides only with another
// derived one, and the second takes the first free suffix, so two columns both
// titled Review become review and review-2.
//
// A title that derives to nothing usable is refused rather than left empty or
// filled in from the identifier, since neither is a name anybody would type.
// That refusal names the title, which is the value the author has to change,
// rather than the slug that was never arrived at.
func assignColumnSlugs(columns []map[string]json.RawMessage) ([]string, error) {
	slugs := make([]string, len(columns))
	taken := map[string]bool{}
	for position, element := range columns {
		slug := memberString(element, "slug")
		if slug == "" {
			continue
		}
		if !ValidColumnSlug(slug) || taken[slug] {
			return nil, contract.Refuse(contract.Malformed, "slug "+slug)
		}
		taken[slug] = true
		slugs[position] = slug
	}
	for position, element := range columns {
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

// writeColumnFromMember writes one column anchor from an interchange element,
// carrying the slug the assignment settled for it.
func writeColumnFromMember(root, id, slug string, element map[string]json.RawMessage) error {
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
	var awaitingOutside bool
	if raw, ok := element["awaiting_outside"]; ok {
		if err := json.Unmarshal(raw, &awaitingOutside); err == nil && awaitingOutside {
			fm.Set("awaiting_outside", "true")
		}
	}
	// The member carries a boolean for the entry direction and a string for
	// the two Dinah adds. A shape this build does not recognize is silently
	// not written, on the lenient discipline the members above already
	// follow: one column's one member is not worth refusing a whole import
	// over.
	if raw, ok := element["gate_items"]; ok {
		var gateItems bool
		var direction string
		switch {
		case json.Unmarshal(raw, &gateItems) == nil:
			if gateItems {
				fm.Set("gate_items", "true")
			}
		case json.Unmarshal(raw, &direction) == nil && (direction == HoldOut || direction == HoldBoth):
			fm.Set("gate_items", direction)
		}
	}
	var capacity int
	if raw, ok := element["capacity"]; ok {
		if err := json.Unmarshal(raw, &capacity); err == nil && capacity > 0 {
			fm.Set("wip_limit", strconv.Itoa(capacity))
		}
	}
	if raw, ok := element[FieldValuesKey]; ok {
		writeMember(fm, FieldValuesKey, raw)
	}
	var required []string
	if raw, ok := element[RequireFieldsKey]; ok {
		if err := json.Unmarshal(raw, &required); err == nil {
			fm.SetSeq(RequireFieldsKey, required)
		}
	}
	if raw, ok := element[StandingItemsKey]; ok {
		writeMember(fm, StandingItemsKey, raw)
	}
	for _, member := range sortedMembers(element) {
		if knownColumnKeys[member] || member == "id" || member == "capacity" || member == "instructions" {
			continue
		}
		writeMember(fm, member, element[member])
	}
	return WriteText(filepath.Join(root, ColumnsDir, id, ColumnAnchor), fm.Render(memberString(element, "instructions")))
}

// writeColumnAttachments writes the attachments a column element carries into
// the column's directory, in array order. An element carrying no provenance is
// written with the operator the new workbench is given, which is who put the
// bytes there. No journal line is written, because instantiation writes none
// for the columns either.
func writeColumnAttachments(columnDir, operator string, element map[string]json.RawMessage) error {
	attachments, err := ColumnAttachmentsOf(element)
	if err != nil {
		return err
	}
	for _, attachment := range attachments {
		provenance := attachment.Provenance
		if provenance == "" {
			provenance = operator
		}
		_, err := AddAttachmentBytes(columnDir, attachment.Filename, attachment.Payload, attachment.Description, provenance)
		if err != nil {
			return err
		}
	}
	return nil
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
	for _, column := range b.Columns {
		source := filepath.Join(b.Root, ColumnsDir, column.ID, ColumnAnchor)
		text, err := ReadText(source)
		if err != nil {
			return err
		}
		if err := WriteText(filepath.Join(target, ColumnsDir, column.ID, ColumnAnchor), text); err != nil {
			return err
		}
		if err := extractAttachments(b.ColumnDir(column.ID), filepath.Join(target, ColumnsDir, column.ID)); err != nil {
			return err
		}
	}
	return nil
}

// extractAttachments copies every live attachment directory of one column into
// the same place under the target: the anchor as text, and the one payload file
// byte for byte, never through ReadText, which normalises newlines. Nothing
// under the column's archive or its comments is copied, because those are
// history and conversation rather than definition.
func extractAttachments(sourceColumn, targetColumn string) error {
	collection := filepath.Join(sourceColumn, AttachmentsDir)
	ids, err := ListIDs(collection)
	if err != nil {
		return err
	}
	for _, id := range ids {
		source := filepath.Join(collection, id)
		target := filepath.Join(targetColumn, AttachmentsDir, id)
		anchor, err := ReadText(filepath.Join(source, AttachmentAnchor))
		if err != nil {
			return err
		}
		if err := WriteText(filepath.Join(target, AttachmentAnchor), anchor); err != nil {
			return err
		}
		payload, err := payloadOf(source)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(payload)
		if err != nil {
			return err
		}
		copied := filepath.Join(target, PayloadDir, filepath.Base(payload))
		if err := os.MkdirAll(filepath.Dir(copied), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(copied, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// normalizeDefinitionText answers the interchange document with every string
// VALUE it carries, at any depth, normalised, and refuses a member NAME
// carrying a line ending.
//
// It walks the decoded document rather than its bytes because a JSON string
// spells a carriage return as an escape, which no byte replacement over the
// file would find, and it runs at this one read boundary rather than at each
// renderer because renderScalar guards its output by comparing it against the
// raw message with sameJSON: a renderer that normalised would fail its own
// guard and fall through to a raw JSON line, storing the escape in the anchor.
// Normalising the message first puts both sides of that comparison in the same
// form and the guard passes unchanged.
//
// A value nothing inside changed comes back as its own bytes, whitespace,
// member order and number spelling and all, because CORE-JSON-7 requires a
// member this tool does not understand to travel unchanged and a member the
// renderer cannot read travels as its own raw line. Only a value carrying a
// string the normalisation touched is rendered afresh.
//
// Values and names are treated differently on purpose. A name becomes the
// left-hand side of a frontmatter line, which nothing quotes, so a line ending
// in one does not corrupt a value, it invents a key.
//
// A document that does not parse comes back unchanged with no error, because
// the refusal for that is the caller's own and saying it twice in two voices
// would make one document refuse under two names.
func normalizeDefinitionText(data []byte) ([]byte, error) {
	return normalizeDefinitionValue(json.RawMessage(data))
}

// jsonSpace is the whitespace RFC 8259 admits between the tokens of a
// document, which is what a value's leading bytes are trimmed of before its
// first real byte says which kind of value it is.
const jsonSpace = " \t\r\n"

// lineEndings are the two bytes a member name may not carry, named here so the
// refusal below reads as a rule rather than as a pair of escapes.
const lineEndings = "\r\n"

// normalizeDefinitionValue is normalizeDefinitionText's recursion, over one
// JSON value at a time.
func normalizeDefinitionValue(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimLeft(raw, jsonSpace)
	if len(trimmed) == 0 {
		return raw, nil
	}
	switch trimmed[0] {
	case '"':
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return raw, nil
		}
		normalized := NormalizeNewlines(value)
		if normalized == value {
			return raw, nil
		}
		return json.Marshal(normalized)
	case '{':
		return normalizeDefinitionObject(raw)
	case '[':
		return normalizeDefinitionArray(raw)
	}
	return raw, nil
}

// normalizeDefinitionObject rewrites one object, refusing any member name
// carrying a line ending. The detail names the member with its line ending
// written as an escape, since printing the name raw would split the refusal's
// own line too.
func normalizeDefinitionObject(raw json.RawMessage) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if _, err := dec.Token(); err != nil {
		return raw, nil
	}
	var out bytes.Buffer
	out.WriteByte('{')
	first, changed := true, false
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return raw, nil
		}
		name, named := tok.(string)
		if !named {
			return raw, nil
		}
		if strings.ContainsAny(name, lineEndings) {
			escaped, err := json.Marshal(name)
			if err != nil {
				return nil, contract.Refuse(contract.MalformedMemberName, name)
			}
			return nil, contract.Refuse(contract.MalformedMemberName, string(escaped))
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return raw, nil
		}
		rewritten, err := normalizeDefinitionValue(value)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(rewritten, value) {
			changed = true
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		encoded, err := json.Marshal(name)
		if err != nil {
			return raw, nil
		}
		out.Write(encoded)
		out.WriteByte(':')
		out.Write(rewritten)
	}
	out.WriteByte('}')
	if !changed {
		return raw, nil
	}
	return out.Bytes(), nil
}

// normalizeDefinitionArray rewrites one array, element by element, and answers
// the original bytes where no element changed.
func normalizeDefinitionArray(raw json.RawMessage) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if _, err := dec.Token(); err != nil {
		return raw, nil
	}
	var out bytes.Buffer
	out.WriteByte('[')
	first, changed := true, false
	for dec.More() {
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return raw, nil
		}
		rewritten, err := normalizeDefinitionValue(value)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(rewritten, value) {
			changed = true
		}
		if !first {
			out.WriteByte(',')
		}
		first = false
		out.Write(rewritten)
	}
	out.WriteByte(']')
	if !changed {
		return raw, nil
	}
	return out.Bytes(), nil
}
