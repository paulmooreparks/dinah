package bench

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DeclaredField is one field a workbench declares for itself, beside the
// built-in fields of each kind that Field above holds.
//
// The two models never merge. Field is what this build knows a card, a column
// or a workbench records whatever workbench it is reading; a declared field is
// a name the workbench in front of the reader minted, and no part of the tool
// enumerates one. The type is spelled apart from Field for that reason and
// because a second type named Field would not compile beside the first.
type DeclaredField struct {
	// Key is the dotted name a reader types, which DeclaredFieldKey admits.
	Key string
	// Type is one of the five FieldTypes members, which decides what a value
	// written under this key has to look like.
	Type string
	// Meaning is the one line of prose the declaration carries for a reader.
	Meaning string
	// On are the entity kinds this field applies to, drawn from KindCard,
	// KindColumn and KindWorkbench. It is empty on an entry that declared no
	// `on` member at all, which applies to all three, and Declares below is
	// what tells that case from an entry whose `on` named nothing this build
	// recognises.
	On []string
	// EveryKind is true where the entry declared no `on` member, which is
	// what makes the field reach all three kinds.
	EveryKind bool
}

// The frontmatter keys the declaration and the values are carried under. A
// declaration is one nested block on the workbench anchor, and the values are
// one nested block on each entity's own anchor.
const (
	// FieldsKey carries the workbench's declaration block, one entry per
	// declared field.
	FieldsKey = "fields"
	// FieldValuesKey carries an entity's own declared field values. No value
	// is ever written as a bare top-level key: the top level of the workbench
	// definition is the namespace a layer declares itself in, a layer's name
	// carries a full stop and so does every declared field key, and a second
	// tool meeting `git.trunk: main` at the top level could not tell a value
	// to preserve from a layer declaration to evaluate. Nesting removes the
	// question rather than answering it, and the rule is uniform across the
	// three kinds so that one reader and one writer serve all of them.
	FieldValuesKey = "field_values"
	// RequireFieldsKey carries a column's own requirement, a sequence of
	// declared field keys a card must hold a value for before it enters.
	RequireFieldsKey = "require_fields"
)

// The five types a declared field may take. The set is closed, and
// AdmitsFieldValue below is the one place each is checked.
const (
	FieldTypeString  = "string"
	FieldTypeNumber  = "number"
	FieldTypeBoolean = "boolean"
	FieldTypeURL     = "url"
	FieldTypeDate    = "date"
)

// FieldTypes lists the closed set of types a declaration may name, in the
// order the profile states them. A declaration naming anything else is
// reported under FindingFieldDeclarationMalformed and declares nothing.
var FieldTypes = []string{
	FieldTypeString, FieldTypeNumber, FieldTypeBoolean, FieldTypeURL, FieldTypeDate,
}

// FieldDateLayout is the layout a date-typed value is read under. time.Parse
// under it refuses a date the calendar does not carry, which is why the layout
// is the whole of the check.
const FieldDateLayout = "2006-01-02"

// The three members one declaration entry carries.
const (
	fieldTypeMember    = "type"
	fieldMeaningMember = "meaning"
	fieldOnMember      = "on"
)

// DeclaredFieldKeyExpression is the grammar a declared field key matches,
// written out once so the code compiles what the profile publishes rather than
// a second expression derived from it.
//
// A key is two or more segments joined by full stops, and a segment begins
// with a lowercase letter, ends with a lowercase letter or a digit, and
// carries lowercase letters, digits and single interior hyphens between the
// two. The full stop is what keeps a declared key out of the namespace this
// profile and every later revision of it mint names in.
const DeclaredFieldKeyExpression = `^[a-z][a-z0-9]*(-[a-z0-9]+)*(\.[a-z][a-z0-9]*(-[a-z0-9]+)*)+$`

// The two lengths a key is bounded by, so a key nobody can read is refused
// before it is stored rather than after.
const (
	// DeclaredFieldKeyLimit is the longest a whole key may be.
	DeclaredFieldKeyLimit = 128
	// DeclaredFieldSegmentLimit is the longest one segment may be.
	DeclaredFieldSegmentLimit = 64
)

var declaredFieldKey = regexp.MustCompile(DeclaredFieldKeyExpression)

// HarnessNameExpression is the grammar a declared harness name matches, which
// is one segment of the key grammar above. The bound is what lets dinah-497
// form the layer name `harness.<name>` from it, since a layer name has to be a
// legal dotted name.
//
// It is written out rather than cut out of DeclaredFieldKeyExpression at run
// time, on the reasoning that expression is written out for: a grammar
// somebody derived is a grammar that can drift from the one the specification
// publishes. TestTheHarnessGrammarIsOneSegmentOfTheKeyGrammar is what holds
// the two together, by composing the key expression out of this one and
// comparing.
const HarnessNameExpression = `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`

var harnessName = regexp.MustCompile(HarnessNameExpression)

// HarnessName reports whether a value is a well-formed harness name. It is the
// one gate both the environment path and the MCP call path run, so a name a
// harness may declare is a name a layer may later be formed from.
func HarnessName(name string) bool {
	if len(name) == 0 || len(name) > DeclaredFieldSegmentLimit {
		return false
	}
	return harnessName.MatchString(name)
}

// DeclaredFieldKey reports whether a name is a well-formed declared field key.
// It is the one gate both the declaration reader and the write path run, so a
// key a workbench can declare is a key a reader can type.
func DeclaredFieldKey(name string) bool {
	if len(name) == 0 || len(name) > DeclaredFieldKeyLimit {
		return false
	}
	for _, segment := range strings.Split(name, ".") {
		if len(segment) > DeclaredFieldSegmentLimit {
			return false
		}
	}
	return declaredFieldKey.MatchString(name)
}

// DeclarableKinds are the three entity kinds a declaration may reach, in the
// order a reader meets them. No declaration reaches a comment, a checklist
// item, an attachment or a workstream, whatever its `on` member says, so a
// write of a declared key to one of those four is refused by the same rule
// that refuses an undeclared key anywhere.
var DeclarableKinds = []string{KindCard, KindColumn, KindWorkbench}

// declarableKind reports whether a kind is one of the three above.
func declarableKind(kind string) bool {
	for _, known := range DeclarableKinds {
		if known == kind {
			return true
		}
	}
	return false
}

// Declares reports whether this field reaches one entity kind. An entry that
// declared no `on` member reaches all three; one that declared an `on` reaches
// exactly the kinds it named, so a member this build does not recognise
// narrows the field rather than widening it.
func (f DeclaredField) Declares(kind string) bool {
	if !declarableKind(kind) {
		return false
	}
	if f.EveryKind {
		return true
	}
	for _, named := range f.On {
		if named == kind {
			return true
		}
	}
	return false
}

// Kinds reports the entity kinds this field reaches, which is what a refusal
// naming the wrong kind prints. An entry declaring no `on` reports the three
// in the order a reader meets them.
func (f DeclaredField) Kinds() []string {
	if f.EveryKind {
		return append([]string(nil), DeclarableKinds...)
	}
	return append([]string(nil), f.On...)
}

// KnownFieldType reports whether a type is one of the five FieldTypes
// declares. It mirrors KnownHold and KnownItemKind, so a closed vocabulary is
// read the same way wherever one is read.
func KnownFieldType(name string) bool {
	for _, known := range FieldTypes {
		if known == name {
			return true
		}
	}
	return false
}

// AdmitsFieldValue reports whether a value satisfies a declared type. A
// declared field holds a scalar: a value wanting a list or a nested object is
// a document and belongs in the entity's body or in an attachment.
//
// Every rule is one standard-library reader's own answer rather than a second
// grammar written here, which is what keeps the refusal and the documentation
// describing one thing.
func AdmitsFieldValue(declaredType, value string) bool {
	if strings.ContainsAny(value, "\r\n") || strings.TrimSpace(value) == "" {
		return false
	}
	switch declaredType {
	case FieldTypeString:
		return true
	case FieldTypeNumber:
		_, err := strconv.ParseFloat(value, 64)
		return err == nil
	case FieldTypeBoolean:
		return value == "true" || value == "false"
	case FieldTypeURL:
		parsed, err := url.Parse(value)
		if err != nil {
			return false
		}
		return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
	case FieldTypeDate:
		_, err := time.Parse(FieldDateLayout, value)
		return err == nil
	}
	return false
}

// fieldBlockMember matches one `name:` line inside a nested block, keeping the
// indentation that says which level of the block the line belongs to. The
// member name stops at the first colon, so a meaning carrying a colon of its
// own runs to the end of its line unparsed.
var fieldBlockMember = regexp.MustCompile(`^( +)([^\s:][^:]*):(.*)$`)

// fieldBlockEntry matches one dashed entry beneath a member, which is the
// second spelling an `on` list may take, keeping the indentation that says
// which level of the block the line belongs to.
//
// It is tried ahead of the member pattern, so a line opening with a hyphen is
// a dashed entry wherever it stands and is never read as a key. The indent is
// what tells the two apart: an `on` list's entries stand deeper than the entry
// key above them, and a line opening with a hyphen at the entry key's own
// depth is a key the grammar refuses. readDeclaredFields reports that one
// rather than letting it pass into whatever `on` list happens to be open.
var fieldBlockEntry = regexp.MustCompile(`^( *)-\s*(.*)$`)

// readDeclaredFields reads the workbench's fields block, answering the fields
// it declares in declaration order and the keys of the entries it refused.
//
// The reader posture is readLevels's: a line it cannot read is skipped rather
// than raised over, so a hand-damaged block leaves the workbench openable and
// `dinah check` is what puts the damage in front of a reader. Three entry
// defects are refused and reported, and each leaves the entry undeclared: a
// key the grammar refuses, a type absent or outside the five, and a meaning
// absent or blank. A duplicate key keeps its first occurrence, which is the
// rule addLevel already keeps for a repeated level name.
//
// The block's own levels are told apart by indentation rather than by a second
// pattern, because an entry key and a member name are the same shape of line
// and only their depth separates them.
func readDeclaredFields(fm *Frontmatter) ([]DeclaredField, []string) {
	var entries []DeclaredField
	var metOn []bool
	var malformed []string
	seen := map[string]bool{}
	entryIndent := -1
	current := -1
	member := ""
	for _, line := range fm.Raw(FieldsKey) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := fieldBlockEntry.FindStringSubmatch(line); m != nil {
			if current >= 0 && member == fieldOnMember && len(m[1]) > entryIndent {
				if named := unquote(stripComment(m[2])); named != "" {
					entries[current].On = append(entries[current].On, named)
				}
			} else {
				// A dashed line is an entry of an `on` list and nothing else.
				// Met anywhere else, or met at the depth an entry key stands
				// at, it is reported rather than skipped: the pattern above is
				// tried first, so a key spelled with a leading hyphen reads as
				// a dashed entry, and without this it would be swallowed into
				// whichever `on` list was open and named nowhere.
				malformed = append(malformed, strings.TrimSpace(line))
			}
			continue
		}
		m := fieldBlockMember.FindStringSubmatch(line)
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
			entries = append(entries, DeclaredField{Key: name})
			metOn = append(metOn, false)
			current = len(entries) - 1
			continue
		}
		if current < 0 {
			continue
		}
		member = name
		value := unquote(strings.TrimSpace(rest))
		switch member {
		case fieldTypeMember:
			entries[current].Type = value
		case fieldMeaningMember:
			entries[current].Meaning = value
		case fieldOnMember:
			metOn[current] = true
			entries[current].On = append(entries[current].On, flowMembers(value)...)
		}
	}
	declared := make([]DeclaredField, 0, len(entries))
	for at, entry := range entries {
		entry.EveryKind = !metOn[at]
		if !DeclaredFieldKey(entry.Key) || !KnownFieldType(entry.Type) || strings.TrimSpace(entry.Meaning) == "" {
			malformed = append(malformed, entry.Key)
			continue
		}
		declared = append(declared, entry)
	}
	return declared, malformed
}

// flowMembers reads a flow sequence written on a member's own line, which is
// the spelling `on: [card]` takes. A value that is not a flow sequence reads
// as no members at all, and the dashed spelling is read by the entry pattern
// above instead.
func flowMembers(value string) []string {
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil
	}
	var members []string
	for _, raw := range strings.Split(strings.Trim(value, "[]"), ",") {
		if member := unquote(strings.TrimSpace(raw)); member != "" {
			members = append(members, member)
		}
	}
	return members
}

// DeclaredFields are the fields this workbench declares, in declaration order,
// which is the order `show` prints an entity's fields in.
func (b *Bench) DeclaredFields() []DeclaredField {
	return append([]DeclaredField(nil), b.declaredFields...)
}

// DeclaredFieldOf reports one declared field by key, and nil where the
// workbench declares none of that name.
func (b *Bench) DeclaredFieldOf(key string) *DeclaredField {
	for _, field := range b.declaredFields {
		if field.Key == key {
			found := field
			return &found
		}
	}
	return nil
}

// DeclaredFieldsOn are the fields this workbench declares that reach one
// entity kind, in declaration order. It is what a refusal naming an undeclared
// key lists, so the sentence is drawn from the declaration rather than from a
// catalog somebody has to remember to edit.
func (b *Bench) DeclaredFieldsOn(kind string) []DeclaredField {
	var reaching []DeclaredField
	for _, field := range b.declaredFields {
		if field.Declares(kind) {
			reaching = append(reaching, field)
		}
	}
	return reaching
}

// DeclaredFieldKeysOn are the keys DeclaredFieldsOn answers, which is the list
// a refusal prints one to a row.
func (b *Bench) DeclaredFieldKeysOn(kind string) []string {
	reaching := b.DeclaredFieldsOn(kind)
	keys := make([]string, 0, len(reaching))
	for _, field := range reaching {
		keys = append(keys, field.Key)
	}
	return keys
}

// MalformedFieldDeclarations are the keys of the entries the declaration
// reader refused, which `dinah check` reports and nothing else reads.
func (b *Bench) MalformedFieldDeclarations() []string {
	return append([]string(nil), b.malformedFields...)
}

// FieldValue reports what an anchor stores for one declared field key, and the
// empty string where it stores nothing. The read is of the field_values block
// alone: a dotted key at the top level of an anchor belongs to the layer
// namespace and is never a declared field value.
func FieldValue(fm *Frontmatter, key string) string {
	for _, stored := range FieldValues(fm) {
		if stored.Key == key {
			return stored.Value
		}
	}
	return ""
}

// StoredField is one key and value of an anchor's field_values block, in the
// order the block carries them.
type StoredField struct {
	Key   string
	Value string
}

// FieldValues reads an anchor's field_values block in stored order. A key the
// workbench does not declare is read and answered like any other, because
// CORE-LAYER-2 requires content a tool does not understand to survive a read
// and a write of its neighbours.
func FieldValues(fm *Frontmatter) []StoredField {
	lines := fm.Raw(FieldValuesKey)
	if len(lines) < 2 {
		return nil
	}
	var stored []StoredField
	seen := map[string]bool{}
	for _, line := range lines[1:] {
		m := fieldBlockMember.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := unquote(strings.TrimSpace(m[2]))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		stored = append(stored, StoredField{Key: key, Value: unquote(strings.TrimSpace(m[3]))})
	}
	return stored
}

// SetFieldValue writes one declared field value into an anchor's field_values
// block, keeping every other key of the block where it stands and adding a new
// key at the end. An empty value deletes the key, and a block left with no key
// at all is deleted rather than written empty, because an empty block reads
// back as a scalar rather than as a mapping.
func SetFieldValue(fm *Frontmatter, key, value string) {
	stored := FieldValues(fm)
	replaced := false
	for at := range stored {
		if stored[at].Key != key {
			continue
		}
		stored[at].Value = value
		replaced = true
	}
	if !replaced && value != "" {
		stored = append(stored, StoredField{Key: key, Value: value})
	}
	lines := []string{FieldValuesKey + ":"}
	for _, entry := range stored {
		if entry.Value == "" {
			continue
		}
		lines = append(lines, "  "+entry.Key+": "+quote(entry.Value))
	}
	if len(lines) == 1 {
		fm.Delete(FieldValuesKey)
		return
	}
	fm.SetRaw(FieldValuesKey, lines)
}

// RenderFieldsBlock renders a declaration as the frontmatter lines the reader
// above parses, which is what the branch migration writes when it declares the
// key it lifted.
func RenderFieldsBlock(fields []DeclaredField) []string {
	lines := []string{FieldsKey + ":"}
	for _, field := range fields {
		lines = append(lines, "  "+field.Key+":")
		lines = append(lines, "    "+fieldTypeMember+": "+quote(field.Type))
		lines = append(lines, "    "+fieldMeaningMember+": "+quote(field.Meaning))
		if field.EveryKind {
			continue
		}
		lines = append(lines, "    "+fieldOnMember+": ["+strings.Join(field.On, ", ")+"]")
	}
	return lines
}
