package lsp

import (
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/bench"
)

// The catalogue keys this package renders a person-facing string from. Every
// string the server shows anybody comes from one of these; none is a Go
// literal.
const (
	keyLabelCard            = "lsp.label.card"
	keyLabelCardHeld        = "lsp.label.card.held"
	keyLabelColumn          = "lsp.label.column"
	keyLabelColumnHoldsIn   = "lsp.label.column.holds-in"
	keyLabelColumnHoldsOut  = "lsp.label.column.holds-out"
	keyLabelColumnHoldsBoth = "lsp.label.column.holds-both"
	keyLabelWorkstream      = "lsp.label.workstream"
	keyLabelItem            = "lsp.label.item"
	keyLabelItemOwned       = "lsp.label.item.owned"
	keyLabelAttachment      = "lsp.label.attachment"
	keyLabelUnresolved      = "lsp.label.unresolved"
	keyHoverWorkbench       = "lsp.hover.workbench"
	keyCompletionColumn     = "lsp.completion.column.detail"
	keyCompletionWorkstream = "lsp.completion.workstream.detail"
	keyCompletionCard       = "lsp.completion.card.detail"
	keyNoWorkbench          = "lsp.no-workbench"
	keyLogWorkbench         = "lsp.log.workbench"
	keyLogNoRefreshSupport  = "lsp.log.no-refresh-support"
	keyLogSlowWalkEntered   = "lsp.log.slow-walk.entered"
	keyLogSlowWalkLeft      = "lsp.log.slow-walk.left"
)

// The members of an annotation's fields map, per target kind. They are
// canonical tokens the machine surface already uses and are never translated,
// so a client colours on a token rather than by parsing a sentence.
const (
	fieldColumn    = "column"
	fieldState     = "state"
	fieldHolder    = "holder"
	fieldHold      = "hold"
	fieldStatus    = "status"
	fieldItemKind  = "item_kind"
	fieldItemState = "item_state"
	fieldOwner     = "owner"
	fieldFilename  = "filename"
)

// The three values the hold member carries. A column that holds neither way
// carries no member at all.
const (
	holdOn   = "on"
	holdOut  = "out"
	holdBoth = "both"
)

// annotation is one reference in one document, with everything the answers
// built over it need: what it says to a person, what it says to a machine,
// and the one file both the link and the definition point at.
type annotation struct {
	// Slot is where the reference stands and what its position declared.
	Slot slot
	// Label is the rendered, translated text an inline annotation shows.
	Label string
	// Tooltip is the rendered markdown a hover answers.
	Tooltip string
	// Target names what the reference resolved to, nil when it resolved to
	// nothing.
	Target *annotationTarget
	// Fields are the canonical tokens describing the target.
	Fields map[string]string
	// File is the one file this reference opens, which both the document
	// link and the definition answer, so no reference opens one file on a
	// click and another on a go-to-definition. It is empty where the
	// reference answers neither.
	File string
	// Inline reports whether an inline annotation is drawn for this
	// reference, which the five kinds of section 4.4 and an unresolved
	// front-matter value get and the four further forms do not. On a
	// reference in prose it is also subject to the annotateProse setting,
	// which Server.annotate applies as it builds the model.
	Inline bool
	// Present reports that there is an annotation here at all. A prose
	// candidate both resolvers refused is a completed read establishing
	// that the candidate names nothing, and it is silence rather than a
	// report: the zero annotation, present nowhere, which drops whatever
	// was retained for it.
	Present bool
}

// memoed is what one resolution left behind: the annotation to answer with,
// and whether there is an annotation at all. A reference that resolved to
// nothing at a declared position is answerable, carrying the unresolved
// label; one whose read could not complete and which nothing was ever
// established for is not.
type memoed struct {
	annotation annotation
	answerable bool
}

// memoKey is what a resolution is memoised under: the text as written and the
// position that read it, because one string at a column position and the same
// string in prose are two different questions.
type memoKey struct {
	kind slotKind
	text string
}

// annotate turns one document's slots into its annotation model.
//
// The prose setting is read here rather than where the annotation is
// composed, and that placement is the whole of what makes the setting take
// effect on a running server. A resolution is memoised, so a decision taken
// inside it survives every later read of the same reference and no later
// write of the setting can reach it. This loop runs afresh on every model
// build, so the value in force when the model was built is the value the
// model carries.
func (s *Server) annotate(slots []slot) []annotation {
	var model []annotation
	for _, at := range slots {
		if at.Text == "" {
			continue
		}
		resolved, ok := s.resolve(at)
		if !ok || !resolved.Present {
			continue
		}
		resolved.Slot = at
		if at.Kind == slotProse && !s.annotateProse {
			resolved.Inline = false
		}
		model = append(model, resolved)
	}
	return model
}

// resolve answers the annotation one slot carries, memoised by content.
//
// The memo is not a cache with an expiry and holds no duration of any kind.
// It is discarded whole by a tick that reports the workbench moved, so a held
// value is served while, and only while, the watch has seen no change, and
// never because it is recent.
//
// A read that could not complete falls back on what was last established for
// the same reference, which is what keeps a chip on screen while a writer is
// composing a directory, and answers nothing at all where there is no such
// value, because a failed read has not established that a reference names
// nothing.
func (s *Server) resolve(at slot) (annotation, bool) {
	key := memoKey{kind: at.Kind, text: at.Text}
	if held, ok := s.memo[key]; ok {
		return held.annotation, held.answerable
	}
	read, established := s.read(at)
	entry := memoed{annotation: read, answerable: established}
	if !established {
		retained, ok := s.retained[key]
		entry = memoed{annotation: retained, answerable: ok}
	} else {
		s.retained[key] = read
	}
	s.memo[key] = entry
	return entry.annotation, entry.answerable
}

// read resolves one reference against the workbench as it stands. The second
// answer reports whether the read completed, which is what separates a
// reference that names nothing from one whose entity was mid-write.
func (s *Server) read(at slot) (annotation, bool) {
	if s.bench == nil {
		return annotation{}, false
	}
	switch at.Kind {
	case slotColumn:
		if column := s.bench.ColumnByRef(at.Text); column != nil {
			return s.columnAnnotation(at, column), true
		}
		return s.unresolvedAnnotation(at), true
	case slotWorkstream:
		workstream, err := s.bench.WorkstreamByRef(at.Text)
		if err != nil {
			return annotation{}, false
		}
		if workstream == nil {
			return s.unresolvedAnnotation(at), true
		}
		return s.workstreamAnnotation(at, workstream), true
	case slotCard:
		resolved, err := s.bench.ResolveCard(at.Text)
		if err != nil {
			if s.incomplete(at.Text) {
				return annotation{}, false
			}
			return s.unresolvedAnnotation(at), true
		}
		return s.cardAnnotation(at, resolved.Card), true
	}
	return s.readProse(at)
}

// readProse resolves a prose candidate, entity resolver first and the
// library's own path resolver after it.
//
// The fall-through is written as a fall-through rather than as a match on the
// two spellings it reaches, so the set of files stays the library's.
// Bench.ResolveReference accepts every reference Bench.ResolvePath accepts
// but two, an attachment's payload and a card's journal, neither of which
// carries an anchor and neither of which therefore names an entity of the
// format. A third file the library ever admits reaches the editor with no
// edit here.
//
// A refusal from both is complete silence: no hint, no link, no hover, no
// definition and no diagnostic. There is no looks-like-a-reference-but-broken
// state in prose, because a number nobody has filed and a card that was
// deleted are both ordinary text to a reader.
func (s *Server) readProse(at slot) (annotation, bool) {
	entity, collection, err := s.bench.ResolveReference(at.Text)
	if err == nil {
		if collection != nil {
			return s.collectionAnnotation(at, collection), true
		}
		return s.entityAnnotation(at, entity)
	}
	if s.incomplete(at.Text) {
		return annotation{}, false
	}
	path, pathErr := s.bench.ResolvePath(at.Text)
	if pathErr != nil {
		return annotation{Slot: at}, true
	}
	return s.fileAnnotation(at, path), true
}

// incomplete reports whether the reference's own head names a card whose
// directory is there and whose anchor is not, which is the detectably
// incomplete thing the format names and the one shape of failed read this
// server can tell apart from a reference that names nothing.
//
// The library answers both with the same refusal, bench.LoadCard turning an
// unreadable anchor into an unknown card, so the refusal's name cannot
// separate them and this asks the filesystem instead. A read that could not
// complete leaves whatever is on screen where it is; a reference that names
// nothing is reported as naming nothing.
func (s *Server) incomplete(ref string) bool {
	head, _, _ := strings.Cut(strings.TrimSpace(ref), "/")
	for _, id := range s.cardIDsFor(head) {
		dir := filepath.Join(s.bench.CardsRoot(), id)
		if bench.Exists(dir) && !bench.Exists(filepath.Join(dir, bench.CardAnchor)) {
			return true
		}
	}
	return false
}

// cardIDsFor answers the card identifiers a reference's head could name,
// which is the identifier itself when it is spelled as one and whatever the
// number registry holds otherwise. It resolves nothing; it only says which
// directories would have to be looked at.
func (s *Server) cardIDsFor(head string) []string {
	if bench.IsID(head) {
		return []string{head}
	}
	cut := strings.LastIndex(head, "-")
	if cut < 0 {
		return nil
	}
	number, err := strconv.Atoi(head[cut+1:])
	if err != nil || number <= 0 {
		return nil
	}
	return s.bench.Numbers.ByNumber[number]
}

// entityAnnotation builds the annotation of a reference the entity resolver
// answered, dispatching on the kind the containment grammar named.
func (s *Server) entityAnnotation(at slot, entity *bench.EntityRef) (annotation, bool) {
	switch entity.Kind {
	case bench.KindCard:
		if entity.Card == nil {
			return annotation{}, false
		}
		return s.cardAnnotation(at, entity.Card), true
	case bench.KindColumn:
		column := s.bench.Column(entity.ID)
		if column == nil {
			return annotation{}, false
		}
		return s.columnAnnotation(at, column), true
	case bench.KindWorkstream:
		workstream := s.bench.Workstream(entity.ID)
		if workstream == nil {
			return annotation{}, false
		}
		return s.workstreamAnnotation(at, workstream), true
	case bench.KindItem:
		item, err := bench.LoadItem(entity.Dir)
		if err != nil {
			return annotation{}, false
		}
		return s.itemAnnotation(at, entity, item), true
	case bench.KindAttachment:
		attachment, err := bench.LoadAttachment(entity.Dir)
		if err != nil {
			return annotation{}, false
		}
		return s.attachmentAnnotation(at, entity, attachment), true
	case bench.KindComment:
		return s.borrowedAnnotation(at, entity), true
	}
	return annotation{Slot: at}, true
}

// cardAnnotation is the annotation of a card reference.
func (s *Server) cardAnnotation(at slot, card *bench.Card) annotation {
	columnTitle := ""
	if column := s.bench.Column(card.Column); column != nil {
		columnTitle = column.Title
	}
	label := s.render(keyLabelCard, "title", card.Title, "column", columnTitle)
	if card.Holder != "" {
		label = s.render(keyLabelCardHeld, "title", card.Title, "column", columnTitle, "holder", card.Holder)
	}
	fields := map[string]string{fieldColumn: card.Column, fieldState: card.State}
	if card.Holder != "" {
		fields[fieldHolder] = card.Holder
	}
	ref := card.Ref(s.bench.Slug)
	return s.compose(at, label, ref, &annotationTarget{Kind: bench.KindCard, ID: card.ID, Ref: ref}, fields,
		filepath.Join(card.Dir, bench.CardAnchor), true)
}

// columnAnnotation is the annotation of a column reference. The hold is
// published through the column's own two predicates rather than by comparing
// its stored string, which is what Column.Hold's comment directs a reader to.
func (s *Server) columnAnnotation(at slot, column *bench.Column) annotation {
	key := keyLabelColumn
	fields := map[string]string{}
	switch {
	case column.HoldsOnEntry() && column.HoldsOnExit():
		key, fields[fieldHold] = keyLabelColumnHoldsBoth, holdBoth
	case column.HoldsOnEntry():
		key, fields[fieldHold] = keyLabelColumnHoldsIn, holdOn
	case column.HoldsOnExit():
		key, fields[fieldHold] = keyLabelColumnHoldsOut, holdOut
	}
	return s.compose(at, s.render(key, "title", column.Title), column.Ref(),
		&annotationTarget{Kind: bench.KindColumn, ID: column.ID, Ref: column.Ref()}, fields,
		s.bench.ColumnAnchorPath(column.ID), true)
}

// workstreamAnnotation is the annotation of a workstream reference. Its file
// is the anchor rather than the directory Bench.ResolvePath answers, because
// an editor is handed a file.
func (s *Server) workstreamAnnotation(at slot, workstream *bench.Workstream) annotation {
	fields := map[string]string{}
	if workstream.Status != "" {
		fields[fieldStatus] = workstream.Status
	}
	return s.compose(at, s.render(keyLabelWorkstream, "title", workstream.Title), workstream.Ref(),
		&annotationTarget{Kind: bench.KindWorkstream, ID: workstream.ID, Ref: workstream.Ref()}, fields,
		filepath.Join(workstream.Dir, bench.WorkstreamAnchor), true)
}

// itemAnnotation is the annotation of a checklist item reference. The kind
// and the state splice untranslated, which is what every other surface a
// person reads already prints.
func (s *Server) itemAnnotation(at slot, entity *bench.EntityRef, item *bench.Item) annotation {
	key, pairs := keyLabelItem, []string{"kind", item.Kind, "state", item.State}
	if item.Owner != "" {
		key = keyLabelItemOwned
		pairs = append(pairs, "owner", item.Owner)
	}
	fields := map[string]string{fieldItemKind: item.Kind, fieldItemState: item.State}
	if item.Owner != "" {
		fields[fieldOwner] = item.Owner
	}
	return s.compose(at, s.render(key, pairs...), entity.Ref,
		s.targetOf(bench.KindItem, entity), fields,
		filepath.Join(entity.Dir, bench.ItemAnchor), true)
}

// attachmentAnnotation is the annotation of an attachment reference. Its file
// is the payload rather than the attachment.md anchor, because the payload is
// the file a person means.
func (s *Server) attachmentAnnotation(at slot, entity *bench.EntityRef, attachment *bench.Attachment) annotation {
	fields := map[string]string{fieldFilename: attachment.Filename}
	return s.compose(at, s.render(keyLabelAttachment, "filename", attachment.Filename), entity.Ref,
		s.targetOf(bench.KindAttachment, entity), fields, attachment.Path, true)
}

// borrowedAnnotation is the answer for a reference carrying no label of its
// own: a comment, which is an entity, and the two files the fall-through
// reaches. Each borrows the label of the entity it is written under, so no
// key is minted for any of them and the annotation wording stays the set
// section 2.4 declares.
//
// None of them draws an inline annotation. A one-line label adds nothing to a
// comment that its own text does not already say, and a file is not an entity
// for an annotation to describe.
func (s *Server) borrowedAnnotation(at slot, entity *bench.EntityRef) annotation {
	label := ""
	if entity.Card != nil {
		label = s.cardAnnotation(at, entity.Card).Label
	}
	anchor, declared := bench.AnchorPathOf(entity)
	if !declared {
		anchor = ""
	}
	return s.compose(at, label, at.Text, s.targetOf(entity.Kind, entity), map[string]string{}, anchor, false)
}

// collectionAnnotation is the answer for a whole collection, which is not an
// entity and has no anchor. It opens its first member's anchor, a position
// being counted in the creation order the member list is already in, and a
// collection holding no member answers hover and nothing else.
func (s *Server) collectionAnnotation(at slot, collection *bench.CollectionRef) annotation {
	label := ""
	if collection.Holder != nil && collection.Holder.Card != nil {
		label = s.cardAnnotation(at, collection.Holder.Card).Label
	}
	file := ""
	if first := collection.FirstMember(); first != "" {
		if member, _, err := s.bench.ResolveReference(first); err == nil && member != nil {
			if anchor, declared := bench.AnchorPathOf(member); declared {
				file = anchor
			}
		}
	}
	return s.compose(at, label, collection.Ref, nil, map[string]string{}, file, false)
}

// fileAnnotation is the answer for a reference the entity resolver refused
// and the path resolver answered, which is a card's journal or an
// attachment's payload. Its label is borrowed from the entity the reference
// is written under: the attachment for a payload, the card for a journal.
func (s *Server) fileAnnotation(at slot, path string) annotation {
	label := ""
	if under := s.entityUnder(at.Text); under != nil {
		label = under.Label
	}
	return s.compose(at, label, at.Text, nil, map[string]string{}, path, false)
}

// entityUnder answers the annotation of the entity a file reference is
// written under, by dropping the reference's last segment and asking the
// entity resolver about what is left. That is the attachment for a payload
// and the card for a journal, and it is composed from the reference rather
// than matched against the two spellings.
func (s *Server) entityUnder(ref string) *annotation {
	trimmed := strings.TrimSpace(ref)
	for {
		cut := strings.LastIndex(trimmed, "/")
		if cut < 0 {
			return nil
		}
		trimmed = trimmed[:cut]
		entity, collection, err := s.bench.ResolveReference(trimmed)
		if err != nil || collection != nil || entity == nil {
			continue
		}
		under, ok := s.entityAnnotation(slot{Kind: slotProse, Text: trimmed}, entity)
		if !ok {
			return nil
		}
		return &under
	}
}

// unresolvedAnnotation is what a declared front-matter position holding a
// value that names nothing carries. It is a report of what the value names
// and not a diagnostic: nothing is published to the Problems panel, and
// dinah-264 owns diagnostics.
func (s *Server) unresolvedAnnotation(at slot) annotation {
	return annotation{
		Slot:    at,
		Label:   s.render(keyLabelUnresolved),
		Tooltip: s.tooltip("", s.render(keyLabelUnresolved)),
		Fields:  map[string]string{},
		Inline:  at.Kind != slotProse,
		Present: true,
	}
}

// compose assembles one annotation out of the parts every kind supplies.
//
// The inline flag it stores is the kind's own, meaning whether this sort of
// reference earns an inline annotation at all. Whether a prose reference
// draws one is a setting, and Server.annotate applies it, because what
// compose produces is memoised and a memoised value must not carry a
// decision a later setting can change.
func (s *Server) compose(at slot, label, ref string, target *annotationTarget, fields map[string]string, file string, inline bool) annotation {
	return annotation{
		Slot:    at,
		Label:   label,
		Tooltip: s.tooltip(ref, label),
		Target:  target,
		Fields:  fields,
		File:    file,
		Inline:  inline,
		Present: true,
	}
}

// tooltip composes the hover body: the canonical reference in a code span,
// the label, and which workbench answered, so a reader with two repositories
// open knows which one did. A hover with no reference to print omits the
// first block, which is the unresolved case.
func (s *Server) tooltip(ref, label string) string {
	var blocks []string
	if ref != "" {
		blocks = append(blocks, "`"+ref+"`")
	}
	if label != "" {
		blocks = append(blocks, label)
	}
	title := ""
	if s.bench != nil {
		title = s.bench.Title
	}
	blocks = append(blocks, s.render(keyHoverWorkbench, "workbench", title))
	return strings.Join(blocks, "\n\n")
}

// targetOf composes the machine half of an annotation for an entity below a
// card, naming the card it belongs to.
func (s *Server) targetOf(kind string, entity *bench.EntityRef) *annotationTarget {
	target := &annotationTarget{Kind: kind, ID: entity.ID, Ref: entity.Ref}
	if entity.Card != nil {
		owner := entity.Card.ID
		target.Card = &owner
	}
	return target
}

// render asks the catalogue for one string. Every person-facing string this
// package produces comes through here.
func (s *Server) render(key string, pairs ...string) string {
	return s.messages.T(key, pairs...)
}
