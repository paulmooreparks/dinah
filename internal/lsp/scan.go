package lsp

import (
	"strings"
	"unicode/utf16"

	"dinah/internal/bench"
)

// slotKind says what a position in a document declares, which is what decides
// which resolver a value there is put to and whether completion fires.
type slotKind int

const (
	// slotColumn is a front-matter position the format declares to name a
	// column.
	slotColumn slotKind = iota
	// slotWorkstream is a front-matter position the format declares to name
	// a workstream.
	slotWorkstream
	// slotCard is a front-matter position the format declares to name a
	// card.
	slotCard
	// slotProse is a candidate a prose scan found, which declares nothing
	// and is admitted or refused by resolution alone.
	slotProse
)

// slot is one place a reference may stand: the span of the value, what the
// position declares, and the text as written after the value's surrounding
// whitespace, one layer of quotes and any trailing comment are excluded.
//
// A slot with empty Text is a declared position holding nothing. It carries
// no annotation, by section 4.2's rule that an empty value is annotated not
// at all, and it still offers completion, because a value nobody has typed
// yet is exactly where completion earns its place.
type slot struct {
	Kind  slotKind
	Range Range
	Text  string
}

// The front-matter keys the format declares to hold a reference. The table is
// complete, and it is the whole of what the front-matter scan looks at: a key
// absent from it is left alone however its value is spelled, which is why
// claim_holder, block_kind and every member of field_values are untouched.
const (
	keyColumns     = "columns"
	keyGroups      = "groups"
	keyColumn      = "column"
	keyWorkstreams = "workstreams"
	keyLinks       = "links"
	keyLinkTo      = "to"
	keyTierAt      = "tier_at"
	keyRejectTo    = "reject_to"
)

// anchorName reports whether a base filename is an anchor of the format,
// which is the narrower front-matter handling of section 4.1: a document
// under the workbench root whose name is one of these is read as a schema,
// and everything else, an attachment payload included, is prose throughout.
//
// The set is derived from the containment grammar rather than listed. That
// grammar says which kind mounts which collection and which anchor each kind
// carries, and it is declared in one place; a map keyed on the anchor
// constants here would be a second copy of it, and a kind added there would
// silently not be read as a schema here.
//
// The two the grammar does not mount are named outright. The workbench is the
// root the grammar hangs from rather than a member of any collection, and a
// workstream resolves through its own prefix rather than through the
// containment walk.
func anchorName(name string) bool {
	if name == bench.WorkbenchAnchor || name == bench.WorkstreamAnchor {
		return true
	}
	_, declared := bench.KindOfAnchor(name)
	return declared
}

// scanFrontMatter reads the declared reference positions of one anchor, by
// position and never by shape. It answers the slots it found and the line the
// front matter's closing fence sits on, which is where a prose scan of the
// same document starts.
//
// A document whose first line is not the fence has no front matter at all,
// which is bench.ParseAnchor's own reading, and this scan does not invent a
// second one.
func scanFrontMatter(name string, lines []string) (slots []slot, bodyFrom int) {
	if !anchorName(name) || len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, 0
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, 0
	}
	key := ""
	for i := 1; i < end; i++ {
		line := lines[i]
		if name, rest, ok := topLevelKey(line); ok {
			key = name
			slots = append(slots, scalarSlots(name, i, line, len(line)-len(rest))...)
			continue
		}
		slots = append(slots, nestedSlots(key, i, line)...)
	}
	return slots, end + 1
}

// topLevelKey reads a key written at column one, which is the only place a
// top-level key can stand. It answers the key's name and the text after the
// colon, so a caller knows where in the line the value begins.
func topLevelKey(line string) (name, rest string, ok bool) {
	cut := strings.IndexByte(line, ':')
	if cut <= 0 {
		return "", "", false
	}
	name = line[:cut]
	if name != strings.TrimSpace(name) || !plainKey(name) {
		return "", "", false
	}
	return name, line[cut+1:], true
}

// plainKey reports whether a name is spelled the way a front-matter key is,
// which is bench.ParseAnchor's own pattern written out rather than matched
// again: an ASCII letter or underscore, then letters, digits, underscores,
// dots and dashes.
func plainKey(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_':
		case i > 0 && (c >= '0' && c <= '9' || c == '.' || c == '-'):
		default:
			return false
		}
	}
	return true
}

// scalarSlots reads the value on a top-level key's own line. A key the table
// declares to hold a sequence carries its elements here when it is written in
// flow form, one slot per element, with the brackets, the commas and the
// whitespace around each element outside every range.
func scalarSlots(key string, line int, text string, from int) []slot {
	switch key {
	case keyColumn, keyRejectTo:
		// card.md and item.md both spell the column position column, and
		// column.md spells its own reject_to. All three are scalars, and a
		// key of one name in an anchor that does not declare it holds
		// nothing this scan would annotate anyway, because the value would
		// resolve to no column.
		return []slot{valueSlot(slotColumn, line, text, from)}
	case keyColumns:
		return flowSlots(slotColumn, line, text, from)
	case keyWorkstreams:
		return flowSlots(slotWorkstream, line, text, from)
	}
	return nil
}

// nestedSlots reads a line beneath a top-level key: an entry of a block
// sequence, or a key of a mapping inside one.
//
// Four keys carry a nested reference. columns and workstreams carry one
// identifier per dashed entry. links and tier_at carry a mapping per entry,
// whose to and column members are the references. groups is a map of named
// lists, so every dashed entry beneath it names a column, whatever the list
// is called, and a list written in flow form on its own name's line carries
// its elements there.
func nestedSlots(key string, line int, text string) []slot {
	switch key {
	case keyColumns:
		return entrySlots(slotColumn, line, text)
	case keyWorkstreams:
		return entrySlots(slotWorkstream, line, text)
	case keyGroups:
		if found := entrySlots(slotColumn, line, text); found != nil {
			return found
		}
		return nestedFlow(slotColumn, line, text)
	case keyLinks:
		return mappingSlots(slotCard, keyLinkTo, line, text)
	case keyTierAt:
		return mappingSlots(slotColumn, keyColumn, line, text)
	}
	return nil
}

// entrySlots reads one dashed entry of a block sequence, which is a slot when
// the line is one and nothing when it is not.
func entrySlots(kind slotKind, line int, text string) []slot {
	indent := len(text) - len(strings.TrimLeft(text, " \t"))
	rest := text[indent:]
	if !strings.HasPrefix(rest, "-") {
		return nil
	}
	after := rest[1:]
	if after != "" && after[0] != ' ' && after[0] != '\t' {
		return nil
	}
	return []slot{valueSlot(kind, line, text, indent+1)}
}

// mappingSlots reads the named member of a mapping written inside a block
// sequence entry, which is where links[].to and tier_at[].column stand. The
// leading dash of the entry's first member is stepped over, so the member is
// read the same way whether it opens the entry or follows it.
func mappingSlots(kind slotKind, member string, line int, text string) []slot {
	indent := len(text) - len(strings.TrimLeft(text, " \t"))
	rest := text[indent:]
	offset := indent
	if strings.HasPrefix(rest, "- ") {
		rest = rest[2:]
		offset += 2
	}
	name, _, ok := strings.Cut(rest, ":")
	if !ok || strings.TrimSpace(name) != member {
		return nil
	}
	return []slot{valueSlot(kind, line, text, offset+len(name)+1)}
}

// nestedFlow reads a named list written in flow form beneath a map, which is
// the shape groups.<name>: [a, b] takes.
func nestedFlow(kind slotKind, line int, text string) []slot {
	indent := len(text) - len(strings.TrimLeft(text, " \t"))
	name, _, ok := strings.Cut(text[indent:], ":")
	if !ok || !plainKey(strings.TrimSpace(name)) {
		return nil
	}
	return flowSlots(kind, line, text, indent+len(name)+1)
}

// flowSlots reads a sequence written on one line, bracketed and comma
// separated, one slot per element, with the brackets, the commas and the
// whitespace around each element outside every range.
//
// A value that is not bracketed is not a flow sequence. What stands there is
// then the key's own line with a block beneath it, whose value is empty: the
// slot is kept, because an empty value at a declared position is where
// completion earns its place, and it carries no annotation by section 4.2's
// own rule.
func flowSlots(kind slotKind, line int, text string, from int) []slot {
	whole := valueSlot(kind, line, text, from)
	if !strings.HasPrefix(whole.Text, "[") || !strings.HasSuffix(whole.Text, "]") {
		return []slot{whole}
	}
	// The slot's own text was measured back into the line, so the inner span
	// is found by walking the line rather than by re-indexing the text.
	open := strings.Index(text[from:], "[") + from
	shut := strings.LastIndex(text, "]")
	var slots []slot
	element := open + 1
	for cut := element; cut <= shut; cut++ {
		if cut != shut && text[cut] != ',' {
			continue
		}
		slots = append(slots, elementSlot(kind, line, text, element, cut))
		element = cut + 1
	}
	return slots
}

// elementSlot narrows one element of a flow sequence to its own text, with
// the whitespace either side of it outside the range.
func elementSlot(kind slotKind, line int, text string, start, end int) slot {
	for start < end && (text[start] == ' ' || text[start] == '\t') {
		start++
	}
	for end > start && (text[end-1] == ' ' || text[end-1] == '\t') {
		end--
	}
	start, end = unquoted(text, start, end)
	return slot{
		Kind:  kind,
		Range: Range{Start: positionAt(text, line, start), End: positionAt(text, line, end)},
		Text:  text[start:end],
	}
}

// valueSlot composes one slot out of the value that begins at from, with the
// whitespace around it, one layer of matching quotes and any trailing comment
// excluded, so a hint sits against the identifier rather than against the
// note somebody wrote beside it.
func valueSlot(kind slotKind, line int, text string, from int) slot {
	if from > len(text) {
		from = len(text)
	}
	value := text[from:]
	start := from + len(value) - len(strings.TrimLeft(value, " \t"))
	end := len(text)
	if comment := commentAt(text, start); comment >= 0 {
		end = comment
	}
	for end > start && (text[end-1] == ' ' || text[end-1] == '\t') {
		end--
	}
	start, end = unquoted(text, start, end)
	return slot{
		Kind:  kind,
		Range: Range{Start: positionAt(text, line, start), End: positionAt(text, line, end)},
		Text:  text[start:end],
	}
}

// commentAt finds the trailing annotation comment a value carries, which the
// format writes with a space before the hash so a value carrying a hash of
// its own survives. It answers -1 when there is none.
func commentAt(text string, from int) int {
	if i := strings.Index(text[from:], " #"); i >= 0 {
		return from + i
	}
	return -1
}

// unquoted narrows a span past one layer of matching quotes.
func unquoted(text string, start, end int) (int, int) {
	if end-start < 2 {
		return start, end
	}
	first, last := text[start], text[end-1]
	if first != last {
		return start, end
	}
	if first != '\'' && first != '"' {
		return start, end
	}
	return start + 1, end - 1
}

// at composes a protocol position out of a line number and a byte offset,
// counting the character axis in UTF-16 code units, which is what the base
// protocol's default position encoding measures.
func positionAt(text string, line, offset int) Position {
	if offset > len(text) {
		offset = len(text)
	}
	return Position{Line: line, Character: len(utf16.Encode([]rune(text[:offset])))}
}
