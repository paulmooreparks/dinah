package bench

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// This file is the one place outside the older migrations that knows the
// layout a store below CardUnitFormat keeps its comments and checklist items
// in: a comment is comments/<id>/comment.md below its holder, an item is
// checklist/<id>/item.md below its card, and each half of every collection
// mirrors under archive/. recordFromDirectories reads that layout into a
// CardRecord, so every reader outside this file asks one type one question
// whichever layout the store is in, and the legacy member writers below write
// it. The storage migration reads the old layout through the same reader, so
// what it carries across is what every read of the old layout answered.
//
// dinah-638 deletes the legacy writers, and every call of
// recordFromDirectories outside this file, in the pull request that turns
// CardUnitEnabled on.

// recordFromDirectories reads one card's members from the old layout's
// directories: the card's own comments and its checklist in both halves, and
// every item's comments in both halves, an archived item's included.
//
// A comment's half is the archived one when it stands in an archive mirror or
// hangs on an archived item. Members are ranked in each directory's own
// SortByOrdinal order, so a tie of ordinals keeps the order that function has
// always answered, and every identifier a directory lists keeps its place in
// that order even when its anchor will not read, because a position counts
// the directory unfiltered.
func (b *Bench) recordFromDirectories(card *Card) (*CardRecord, error) {
	src := b.source()
	events, torn, err := readJournal(src, card.JournalPath())
	if err != nil {
		events = nil
	}
	r := &CardRecord{
		Card:     card,
		Events:   events,
		Torn:     torn,
		Comments: map[string]*Comment{},
		Items:    map[string]*Item{},
		rank:     map[string]int{},
		ordinals: map[MemberCollection]int{},
		named:    map[string]bool{},
		placed:   map[placedKey][]string{},
	}
	anchors := map[string]string{}
	r.fileOf = func(kind, id string) string { return anchors[kind+"/"+id] }
	// Every collection directory is listed at most once, so a count taken
	// before a holder's comments are read and the read that follows it make
	// one listing between them.
	listings := map[string]listing{}
	list := func(collection string) ([]string, error) {
		if kept, ok := listings[collection]; ok {
			return kept.ids, kept.err
		}
		ids, err := listIDs(src, collection)
		listings[collection] = listing{ids: ids, err: err}
		return ids, err
	}
	// Comments are read a holder at a time, on the first question about that
	// holder, because a listing that counts an item's comments reads a
	// directory and no anchor, and reading every comment of every card it
	// renders would cost what the old readers never paid. Each comment
	// holder is the card or one of its items, in whichever half the item
	// stands. A comment's own half is the half of the directory it sits in
	// below its holder, and CommentsOf reads an archived item's comments as
	// archived whatever that says.
	r.commentHomes = map[string]string{"": card.Dir}
	r.loaded = map[string]bool{}
	// The checklist is read a half at a time: the live half now, since
	// nearly every question about a card asks about it, and the archived
	// half on the first question that reaches it. Each anchor is read once,
	// and the ordinal it carries sorts the half without opening it again.
	r.itemHalves = map[ResolutionHalf]bool{}
	r.loadItems = func(half ResolutionHalf) error {
		collection := legacyCollection(card.Dir, ChecklistDir, half)
		ids, texts, err := legacyTexts(src, list, collection, ItemAnchor)
		if err != nil {
			return err
		}
		key := placedKey{collection: MemberCollection{Kind: KindItem}, half: half}
		for n, id := range ids {
			r.placed[key] = append(r.placed[key], id)
			r.named[id] = true
			dir := filepath.Join(collection, id)
			anchors[KindItem+"/"+id] = filepath.Join(dir, ItemAnchor)
			r.commentHomes[id] = dir
			text, ok := texts[id]
			if !ok {
				continue
			}
			item := itemFromText(dir, text)
			item.Archived = half == ArchivedHalf
			r.Items[id] = item
			r.rank[id] = n
			r.count(MemberCollection{Kind: KindItem}, item.Ordinal)
		}
		return nil
	}
	if err := r.holdItems(LiveHalf); err != nil {
		return nil, err
	}
	r.load = func(holder string) {
		home, known := r.commentHomes[holder]
		if !known {
			return
		}
		for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
			collection := legacyCollection(home, CommentsDir, half)
			ids, texts, err := legacyTexts(src, list, collection, CommentAnchor)
			if err != nil {
				continue
			}
			members := MemberCollection{Kind: KindComment, Holder: holder}
			key := placedKey{collection: members, half: half}
			for n, id := range ids {
				r.placed[key] = append(r.placed[key], id)
				r.named[id] = true
				dir := filepath.Join(collection, id)
				anchors[KindComment+"/"+id] = filepath.Join(dir, CommentAnchor)
				text, ok := texts[id]
				if !ok {
					continue
				}
				comment := commentFromText(dir, id, text)
				comment.Holder = holder
				comment.Archived = half == ArchivedHalf
				r.Comments[id] = comment
				r.rank[id] = n
				r.count(members, comment.Ordinal)
			}
		}
	}
	r.listed = func(holder string, half ResolutionHalf) (int, error) {
		home, known := r.commentHome(holder)
		if !known {
			return 0, nil
		}
		ids, err := list(legacyCollection(home, CommentsDir, half))
		if err != nil {
			return 0, err
		}
		return len(ids), nil
	}
	return r, nil
}

// legacyTexts lists a collection directory on the old layout and reads each
// member's anchor once, through src. It answers the identifiers in the order
// SortByOrdinal puts them in, with a member whose anchor will not read still
// in its place, and the text of every anchor that did read. An absent
// directory answers as an empty one.
func legacyTexts(src Source, list func(string) ([]string, error), collection, anchor string) ([]string, map[string]string, error) {
	ids, err := list(collection)
	if err != nil {
		return nil, nil, err
	}
	texts := make(map[string]string, len(ids))
	for _, id := range ids {
		if text, err := readText(src, joinMember(collection, id, anchor)); err == nil {
			texts[id] = text
		}
	}
	ordered := sortByOrdinalWith(src, collection, ids, func(id string) int {
		text, ok := texts[id]
		if !ok {
			return 0
		}
		fm, _ := ParseAnchor(text)
		return OrdinalOf(fm)
	})
	return ordered, texts, nil
}

// count raises a collection instance's highest ordinal to one a member
// carries.
func (r *CardRecord) count(collection MemberCollection, ordinal int) {
	if ordinal > r.ordinals[collection] {
		r.ordinals[collection] = ordinal
	}
}

// legacyCollection is one half of a collection directory below a holder on
// the old layout: the collection itself, or its mirror under archive/.
func legacyCollection(holderDir, collection string, half ResolutionHalf) string {
	if half == ArchivedHalf {
		return filepath.Join(holderDir, ArchiveDir, collection)
	}
	return filepath.Join(holderDir, collection)
}

// columnCommentsFromDirectories reads one column's comments on the old layout,
// in one half: a live column's own comments in the live half and its mirror in
// the archived half, and every comment of an archived column in the archived
// half.
func (b *Bench) columnCommentsFromDirectories(columnID string, half ResolutionHalf) ([]*Comment, error) {
	var holders []string
	switch {
	case b.Column(columnID) != nil:
		holders = []string{legacyCollection(b.ColumnDir(columnID), CommentsDir, half)}
	case half == ArchivedHalf:
		dir := b.columnDirIn(ArchivedHalf, columnID)
		holders = []string{legacyCollection(dir, CommentsDir, LiveHalf), legacyCollection(dir, CommentsDir, ArchivedHalf)}
	}
	src := b.source()
	list := func(collection string) ([]string, error) { return listIDs(src, collection) }
	var found []*Comment
	for _, collection := range holders {
		ids, texts, err := legacyTexts(src, list, collection, CommentAnchor)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			dir := filepath.Join(collection, id)
			text, ok := texts[id]
			if !ok {
				continue
			}
			comment := commentFromText(dir, id, text)
			comment.Holder = columnID
			comment.Archived = half == ArchivedHalf
			found = append(found, comment)
		}
	}
	return found, nil
}

// Comments reads a holder directory's own comments on the old layout, in
// creation order. The holder is a card, a checklist item or a column.
//
// The order is the ordinal's rather than the timestamp's, because a timestamp
// is wall-clock and two processes commenting inside one second record the same
// one, which leaves the reader's order to the directory listing. A comment
// carrying no ordinal sorts ahead of every stamped one. For a collection below
// a card, SortByOrdinal recovers the order such comments were written in from
// the card's journal, which is the order check --migrate-ordinals will stamp
// them in. For a collection below a column, journalPathFor answers the empty
// string, so nothing is recovered and the listing order stands.
func Comments(holderDir string) ([]*Comment, error) {
	return legacyComments(Disk{}, holderDir)
}

// legacyComments is Comments's body, reading through src.
func legacyComments(src Source, holderDir string) ([]*Comment, error) {
	collection := filepath.Join(holderDir, CommentsDir)
	ids, err := listIDs(src, collection)
	if err != nil {
		return nil, err
	}
	var found []*Comment
	for _, id := range sortByOrdinal(src, collection, CommentAnchor, ids) {
		comment, err := commentAt(src, joinMember(collection, id))
		if err != nil {
			continue
		}
		found = append(found, comment)
	}
	return found, nil
}

// deriveItem is DeriveItem's derive function.
func deriveItem(path, text, _ string) (any, error) {
	return itemFromText(filepath.Dir(path), text), nil
}

// deriveComment is DeriveComment's derive function.
func deriveComment(path, text, _ string) (any, error) {
	dir := filepath.Dir(path)
	return commentFromText(dir, filepath.Base(dir), text), nil
}

// itemAt reads the item whose directory is dir on the old layout through
// src. Its error is the anchor's read error.
func itemAt(src Source, dir string) (*Item, error) {
	anchor := joinMember(dir, ItemAnchor)
	observeAnchor(anchor)
	value, err := src.Derive(anchor, DeriveItem, deriveItem)
	if err != nil {
		return nil, err
	}
	return value.(*Item).Clone(), nil
}

// commentAt reads the comment whose directory is dir on the old layout
// through src.
func commentAt(src Source, dir string) (*Comment, error) {
	anchor := joinMember(dir, CommentAnchor)
	observeAnchor(anchor)
	value, err := src.Derive(anchor, DeriveComment, deriveComment)
	if err != nil {
		return nil, err
	}
	return value.(*Comment).Clone(), nil
}

// commentFromText builds a comment from the text of its anchor on the old
// layout, whose own directory is the comment's attachments home.
func commentFromText(dir, id, text string) *Comment {
	fm, body := ParseAnchor(text)
	return &Comment{
		ID:                  id,
		Home:                dir,
		TS:                  fm.Value("ts"),
		Ordinal:             OrdinalOf(fm),
		Author:              fm.Value("author"),
		AuthorUnrecoverable: fm.Value(CommentAuthorUnrecoverableField) == "true",
		RecordedDigest:      fm.Value(CommentDigestField),
		Body:                body,
		raw:                 text,
	}
}

// CountComments reports how many comments a directory's own collection holds
// on the old layout, and it opens no comment's anchor to do it.
func CountComments(dir string) (int, error) {
	return countComments(Disk{}, dir)
}

// countComments is CountComments's body, reading through src.
func countComments(src Source, dir string) (int, error) {
	ids, err := listIDs(src, filepath.Join(dir, CommentsDir))
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// CountItems is how many checklist items a card's collection holds on the old
// layout. It reads the collection's directory and opens no item anchor.
func CountItems(cardDir string) (int, error) {
	ids, err := listIDs(Disk{}, filepath.Join(cardDir, ChecklistDir))
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// LoadItem reads one checklist item from its directory on the old layout.
//
// Every field but the three CORE-CLAIM-10 decides on is read for a reader
// rather than for the claim, and an anchor carrying none of them still
// answers the claim exactly as it did: a key a header does not carry reads as
// empty, and an empty column names no column any workbench declares.
func LoadItem(dir string) (*Item, error) {
	return loadItem(Disk{}, dir)
}

// loadItem is LoadItem's body, reading through src.
func loadItem(src Source, dir string) (*Item, error) {
	item, err := itemAt(src, dir)
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, dir)
	}
	return item, nil
}

// itemFromText builds a checklist item from the text of its anchor on the old
// layout.
func itemFromText(dir, text string) *Item {
	fm, body := ParseAnchor(text)
	return &Item{
		ID:         filepath.Base(dir),
		dir:        dir,
		Kind:       fm.Value("kind"),
		State:      fm.Value("state"),
		Ordinal:    OrdinalOf(fm),
		Column:     fm.Value("column"),
		Owner:      fm.Value("owner"),
		Resolution: fm.Value(ItemResolutionField),
		Standing:   fm.Value(ItemStandingField),
		Evidence:   fm.Value(ItemEvidenceField),
		TS:         fm.Value("ts"),
		Citations:  CitationsOf(fm),
		Text:       strings.TrimRight(body, "\n"),
		body:       body,
		keys:       fm.Keys(),
		raw:        text,
	}
}

// LegacyDir is an item's own directory on the old layout, which LoadItem and
// Items record, and empty on an item the card-unit layout answered. The older
// migrations, which read and write that layout alone, are its only readers.
func (i *Item) LegacyDir() string {
	return i.dir
}

// Items reads a card's live checklist items on the old layout, in creation
// order. An item whose anchor will not open is skipped.
func Items(cardDir string) ([]*Item, error) {
	return legacyItems(Disk{}, cardDir)
}

// legacyItems is Items's body, reading through src.
func legacyItems(src Source, cardDir string) ([]*Item, error) {
	collection := filepath.Join(cardDir, ChecklistDir)
	ids, err := listIDs(src, collection)
	if err != nil {
		return nil, err
	}
	var found []*Item
	for _, id := range sortByOrdinal(src, collection, ItemAnchor, ids) {
		item, err := loadItem(src, joinMember(collection, id))
		if err != nil {
			continue
		}
		found = append(found, item)
	}
	return found, nil
}

// CitationsOf reads every entry of an item's citations sequence, in stored
// order, through blockValue, the reader every structured frontmatter value is
// read by. An entry carrying a member this build does not know still answers
// its scheme, target and observation.
func CitationsOf(fm *Frontmatter) []Citation {
	if !fm.Has(CitationsField) {
		return nil
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(blockValue(fm, CitationsField), &entries); err != nil {
		return nil
	}
	var citations []Citation
	for _, entry := range entries {
		var citation Citation
		json.Unmarshal(entry[ItemCitationScheme], &citation.Scheme)
		json.Unmarshal(entry[ItemCitationTarget], &citation.Target)
		var observed map[string]string
		if json.Unmarshal(entry[ItemCitationObserved], &observed) == nil {
			citation.Before, citation.After = observed["before"], observed["after"]
		}
		citations = append(citations, citation)
	}
	return citations
}

// ReadItemAnchor opens an item's anchor on the old layout for a write,
// returning its whole header and its body. A write has to put back every key
// it did not touch, which is what reading the header rather than the entity
// gives it.
func ReadItemAnchor(dir string) (*Frontmatter, string, error) {
	return readItemAnchor(Disk{}, dir)
}

// readItemAnchor is ReadItemAnchor's body, reading through src.
func readItemAnchor(src Source, dir string) (*Frontmatter, string, error) {
	fm, body, err := anchorOf(src, filepath.Join(dir, ItemAnchor))
	if err != nil {
		return nil, "", contract.Refuse(contract.UnknownPath, dir)
	}
	return fm, body, nil
}

// LegacyItemDir is the directory the old layout keeps one checklist item of a
// card in, for a writer that has to produce that layout.
func LegacyItemDir(cardDir, id string) string {
	return filepath.Join(cardDir, ChecklistDir, id)
}

// WriteItemAnchor rewrites an item's anchor on the old layout from a header
// and a body.
func WriteItemAnchor(dir string, fm *Frontmatter, body string) error {
	return WriteText(filepath.Join(dir, ItemAnchor), fm.Render(body))
}

// ReadCommentAnchor opens a comment's anchor on the old layout for a write,
// returning its whole header and its body.
func ReadCommentAnchor(dir string) (*Frontmatter, string, error) {
	return readCommentAnchor(Disk{}, dir)
}

// readCommentAnchor is ReadCommentAnchor's body, reading through src.
func readCommentAnchor(src Source, dir string) (*Frontmatter, string, error) {
	fm, body, err := anchorOf(src, filepath.Join(dir, CommentAnchor))
	if err != nil {
		return nil, "", contract.Refuse(contract.UnknownPath, dir)
	}
	return fm, body, nil
}

// WriteCommentAnchor rewrites a comment's anchor on the old layout from a
// header and a body, stamping the digest over the body being written.
//
// Every writer of a comment's anchor goes through this rather than calling
// WriteText itself, which is what makes "recomputed by every verb that writes
// the anchor" a property of one function instead of a rule each call site has
// to remember.
func WriteCommentAnchor(dir string, fm *Frontmatter, body string) error {
	StampCommentDigest(fm, body)
	return WriteText(filepath.Join(dir, CommentAnchor), fm.Render(body))
}

// MemberPosition is the one-based position of one member directory within its
// collection on the old layout, counted over the collection as the resolver
// counts it: every identifier the directory holds, in the order SortByOrdinal
// puts them, with nothing filtered out.
func MemberPosition(dir, anchor string) (int, error) {
	return memberPosition(Disk{}, dir, anchor)
}

// MemberPosition is the free MemberPosition read through this bench's source.
// An attachment is positioned through it on either layout, since attachments
// keep their directories.
func (b *Bench) MemberPosition(dir, anchor string) (int, error) {
	return memberPosition(b.source(), dir, anchor)
}

// memberPosition is MemberPosition's body, reading through src.
func memberPosition(src Source, dir, anchor string) (int, error) {
	collection := filepath.Dir(dir)
	id := filepath.Base(dir)
	ids, err := listIDs(src, collection)
	if err != nil {
		return 0, err
	}
	for n, member := range sortByOrdinal(src, collection, anchor, ids) {
		if member == id {
			return n + 1, nil
		}
	}
	return 0, nil
}

// legacyHolderDir is the directory a comment's holder occupies on the old
// layout: the card itself, one of its items in whichever half it stands, or a
// column.
func (b *Bench) legacyHolderDir(holder MemberHolder) string {
	switch {
	case holder.Column != "":
		return b.columnDirIn(columnHalf(b, holder.Column), holder.Column)
	case holder.Item != "":
		live := filepath.Join(holder.Card.Dir, ChecklistDir, holder.Item)
		if b.Exists(live) {
			return live
		}
		return filepath.Join(holder.Card.Dir, ArchiveDir, ChecklistDir, holder.Item)
	}
	return holder.Card.Dir
}

// legacyItemDir is an item's directory on the old layout, in whichever half it
// stands.
func legacyItemDir(card *Card, item *Item) string {
	return filepath.Join(legacyCollection(card.Dir, ChecklistDir, halfOf(item.Archived)), item.ID)
}

// halfOf names the half an archived mark stands in.
func halfOf(archived bool) ResolutionHalf {
	if archived {
		return ArchivedHalf
	}
	return LiveHalf
}

// legacyAddComment writes a comment entity under its holder directory on the
// old layout. The caller holds the holder's lock, which is what makes the
// ordinal scan race-free.
func legacyAddComment(src Source, holderDir, author, ts, body string) (*Comment, error) {
	collection := filepath.Join(holderDir, CommentsDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	ordinal, err := nextOrdinal(src, collection, CommentAnchor)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(collection, id)
	fm := NewFrontmatter()
	fm.Set("ts", ts)
	if author != "" {
		fm.Set("author", author)
	}
	fm.Set(OrdinalField, strconv.Itoa(ordinal))
	if err := WriteCommentAnchor(dir, fm, body); err != nil {
		return nil, err
	}
	comment := &Comment{
		ID:             id,
		Home:           dir,
		TS:             ts,
		Ordinal:        ordinal,
		Author:         author,
		Body:           body,
		RecordedDigest: fm.Value(CommentDigestField),
	}
	return comment, nil
}

// legacyAddItem writes a checklist item under a card on the old layout. The
// caller holds the card's lock. The column and the owner are written only when
// the caller supplies one.
func legacyAddItem(src Source, cardDir, kind, column, owner, ts, text string) (*Item, error) {
	collection := filepath.Join(cardDir, ChecklistDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	ordinal, err := nextOrdinal(src, collection, ItemAnchor)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(collection, id)
	fm := NewFrontmatter()
	fm.Set(ItemKindField, kind)
	fm.Set(ItemStateField, ItemPending)
	if column != "" {
		fm.Set(ItemColumnField, column)
	}
	if owner != "" {
		fm.Set(ItemOwnerField, owner)
	}
	fm.Set("ts", ts)
	fm.Set(OrdinalField, strconv.Itoa(ordinal))
	if err := WriteText(filepath.Join(dir, ItemAnchor), fm.Render(text)); err != nil {
		return nil, err
	}
	item := &Item{
		ID:      id,
		Kind:    kind,
		State:   ItemPending,
		Ordinal: ordinal,
		Column:  column,
		Owner:   owner,
		TS:      ts,
		Text:    strings.TrimRight(text, "\n"),
	}
	return item, nil
}

// legacyAddStandingItem writes one instance of a standing entry under a card
// on the old layout, on legacyAddItem's terms: the entry's kind, the pending
// state, the declaring column, the owner and the evidence scheme where the
// entry declares them, the standing key, the stamp and the ordinal, and the
// entry's text as the body.
func legacyAddStandingItem(src Source, cardDir, columnID string, entry StandingItem, ts string) (*Item, error) {
	collection := filepath.Join(cardDir, ChecklistDir)
	id, err := ClaimID(collection, nil)
	if err != nil {
		return nil, err
	}
	ordinal, err := nextOrdinal(src, collection, ItemAnchor)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(collection, id)
	fm := NewFrontmatter()
	fm.Set(ItemKindField, entry.Kind)
	fm.Set(ItemStateField, ItemPending)
	fm.Set(ItemColumnField, columnID)
	if entry.Owner != "" {
		fm.Set(ItemOwnerField, entry.Owner)
	}
	fm.Set(ItemStandingField, entry.Key)
	if entry.Evidence != "" {
		fm.Set(ItemEvidenceField, entry.Evidence)
	}
	fm.Set("ts", ts)
	fm.Set(OrdinalField, strconv.Itoa(ordinal))
	if err := WriteText(filepath.Join(dir, ItemAnchor), fm.Render(entry.Text)); err != nil {
		return nil, err
	}
	item := &Item{
		ID:       id,
		Kind:     entry.Kind,
		State:    ItemPending,
		Ordinal:  ordinal,
		Column:   columnID,
		Owner:    entry.Owner,
		Standing: entry.Key,
		Evidence: entry.Evidence,
		TS:       ts,
		Text:     entry.Text,
	}
	return item, nil
}

// legacyMemberFile answers the anchor file of a comment or an item on the old
// layout, which is what `dinah path` and `dinah edit` open there.
func legacyMemberFile(entity *EntityRef) (string, bool) {
	anchor := legacyAnchorOf(entity.Kind)
	if anchor == "" {
		return "", false
	}
	return filepath.Join(entity.Dir, anchor), true
}

// legacyMemberCollections are the collection directory names the old layout
// keeps members in, for the walks that list every directory below a card.
var legacyMemberCollections = []string{CommentsDir, ChecklistDir}

// legacyAnchorKind reports the member kind an anchor file names on the old
// layout.
func legacyAnchorKind(name string) (string, bool) {
	for _, kind := range []string{KindComment, KindItem} {
		if legacyAnchorOf(kind) == name {
			return kind, true
		}
	}
	return "", false
}

// legacyAnchorOf is the anchor filename the older layout gives a comment or an
// item, and the empty string for any other kind.
//
// It is the old layout's own statement of the two anchors the containment
// grammar no longer declares, since in the card-unit layout neither kind has
// a file, and every reader of that layout asks it rather than naming the two
// anchors again.
func legacyAnchorOf(kind string) string {
	switch kind {
	case KindComment:
		return CommentAnchor
	case KindItem:
		return ItemAnchor
	}
	return ""
}

// checkRetiredNotes reports every checklist item still carrying the note key
// dinah-525 retired, on a workbench whose declared format says the migration
// has run.
//
// A workbench below that format is not reported at all, and it is not reported
// because it is refused: a read cannot open such a store, so nothing reaches
// this walk. What this finding covers is the store the migration ran over and
// did not finish, which is the state a run interrupted between its two writes
// leaves one item in. The card-unit layout carries no note key anywhere, so
// a store in it is not reported either.
func (b *Bench) checkRetiredNotes(card *Card) ([]Finding, error) {
	if b.Format < ResolutionFormat || b.CardUnit() {
		return nil, nil
	}
	collection := filepath.Join(card.Dir, ChecklistDir)
	ids, err := b.ListIDs(collection)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, id := range ids {
		dir := filepath.Join(collection, id)
		fm, _, err := readItemAnchor(b.source(), dir)
		if err != nil {
			continue
		}
		if fm.Value(ItemNoteRetiredField) == "" {
			continue
		}
		findings = append(findings, Finding{
			Path:     filepath.Join(dir, ItemAnchor),
			Key:      FindingItemCarriesRetiredNote,
			Detail:   id,
			Severity: SeverityDefect,
		})
	}
	return findings, nil
}
