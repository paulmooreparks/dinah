package bench

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// MemberHolder is what a new comment hangs on: a card, one of its items, or a
// column. Card is nil for a column comment.
type MemberHolder struct {
	// Card is the card the comment belongs to, nil for a column comment.
	Card *Card
	// Item is the identifier of the item the comment hangs on, empty for a
	// comment on the card itself or on a column.
	Item string
	// Column is the identifier of the column the comment hangs on, empty for
	// a comment below a card.
	Column string
}

// HolderOf answers the holder an entity a comment is written on names: the
// card, an item of it, or a column. A kind that mounts no comments answers
// false.
func HolderOf(entity *EntityRef) (MemberHolder, bool) {
	switch entity.Kind {
	case KindCard:
		return MemberHolder{Card: entity.Card}, true
	case KindItem:
		return MemberHolder{Card: entity.Card, Item: entity.ID}, true
	case KindColumn:
		return MemberHolder{Column: entity.ID}, true
	}
	return MemberHolder{}, false
}

// AddComment writes a new comment below its holder and answers it. The caller
// holds the holder's lock, which is the card's for a card or item comment and
// the workbench's for a column comment, and appends the commented line itself
// after CompleteCommented has filled what the layout needs.
//
// In the card-unit layout nothing is written here: the comment is minted, an
// identifier no journal line has named and the next ordinal of its collection,
// and the commented line the caller appends is the comment. Below that format
// the comment's own directory and anchor are written, as the old layout keeps
// them.
func (b *Bench) AddComment(holder MemberHolder, author, ts, body string) (*Comment, error) {
	if !b.CardUnit() {
		comment, err := legacyAddComment(b.legacyHolderDir(holder), author, ts, body)
		if err != nil {
			return nil, err
		}
		comment.Holder = holder.Item + holder.Column
		return comment, nil
	}
	comment := &Comment{Holder: holder.Item + holder.Column, TS: ts, Author: author, Body: NormalizeNewlines(body)}
	if holder.Column != "" {
		id, err := b.MintColumnCommentID()
		if err != nil {
			return nil, err
		}
		ordinal, err := b.NextColumnOrdinal(holder.Column)
		if err != nil {
			return nil, err
		}
		comment.ID, comment.Ordinal = id, ordinal
		comment.Home = filepath.Join(b.columnDirIn(columnHalf(b, holder.Column), holder.Column), CommentsDir, id)
	} else {
		record, err := b.LoadCardRecord(holder.Card)
		if err != nil {
			return nil, err
		}
		id, err := record.MintID()
		if err != nil {
			return nil, err
		}
		comment.ID = id
		comment.Ordinal = record.NextOrdinal(MemberCollection{Kind: KindComment, Holder: holder.Item})
		comment.Home = filepath.Join(holder.Card.Dir, CommentsDir, id)
	}
	comment.RecordedDigest = CommentDigest(comment.Body)
	return comment, nil
}

// CommentPosition answers a live comment's one-based position among its
// holder's live comments, and zero when it is not one of them.
func (b *Bench) CommentPosition(holder MemberHolder, id string) (int, error) {
	if holder.Column != "" {
		comments, err := b.ColumnComments(holder.Column, LiveHalf)
		if err != nil {
			return 0, err
		}
		for n, comment := range comments {
			if comment.ID == id {
				return n + 1, nil
			}
		}
		return 0, nil
	}
	record, err := b.LoadCardRecord(holder.Card)
	if err != nil {
		return 0, err
	}
	return record.Position(MemberCollection{Kind: KindComment, Holder: holder.Item}, LiveHalf, id), nil
}

// CompleteCommented fills the commented line of a comment AddComment wrote
// with what the card-unit layout stores on it: the comment's text and its
// ordinal. Below that format the line carries neither, as it always has.
func (b *Bench) CompleteCommented(ev *Event, comment *Comment) {
	if !b.CardUnit() {
		return
	}
	ev.Text = comment.Body
	ev.Ordinal = comment.Ordinal
}

// AddItem files a new checklist item on a card and answers it. The caller
// holds the card's lock and appends the item_filed line itself after
// CompleteFiled has filled what the layout needs. The column and the owner are
// recorded only when the caller supplies one.
func (b *Bench) AddItem(card *Card, kind, column, owner, ts, text string) (*Item, error) {
	if !b.CardUnit() {
		return legacyAddItem(card.Dir, kind, column, owner, ts, text)
	}
	item, err := b.mintItem(card, ts, text)
	if err != nil {
		return nil, err
	}
	item.Kind, item.Column, item.Owner = kind, column, owner
	return item, nil
}

// AddStandingItem files one instance of a standing entry on a card, on
// AddItem's terms: the entry's kind, the pending state, the declaring column,
// the owner and the evidence scheme where the entry declares them, the
// standing key, and the entry's text, which is a copy taken at minting so one
// card's instance can be edited without touching every other card's.
func (b *Bench) AddStandingItem(card *Card, columnID string, entry StandingItem, ts string) (*Item, error) {
	if !b.CardUnit() {
		return legacyAddStandingItem(card.Dir, columnID, entry, ts)
	}
	item, err := b.mintItem(card, ts, entry.Text)
	if err != nil {
		return nil, err
	}
	item.Kind, item.Column, item.Owner = entry.Kind, columnID, entry.Owner
	item.Standing, item.Evidence = entry.Key, entry.Evidence
	return item, nil
}

// mintItem mints a new item of a card in the card-unit layout: an identifier
// no journal line has named, the checklist's next ordinal, the pending state
// and the text.
func (b *Bench) mintItem(card *Card, ts, text string) (*Item, error) {
	record, err := b.LoadCardRecord(card)
	if err != nil {
		return nil, err
	}
	id, err := record.MintID()
	if err != nil {
		return nil, err
	}
	item := &Item{
		ID:      id,
		State:   ItemPending,
		Ordinal: record.NextOrdinal(MemberCollection{Kind: KindItem}),
		TS:      ts,
		Text:    strings.TrimRight(NormalizeNewlines(text), "\n"),
	}
	return item, nil
}

// CompleteFiled fills the item_filed line of an item AddItem or
// AddStandingItem filed with what the card-unit layout stores on it: the
// ordinal, the text, the owner and the evidence scheme, and the item's column
// and that column's title whenever the item names one, on a hand filing as
// well as a standing one. Below that format the line is left as it always was.
func (b *Bench) CompleteFiled(ev *Event, item *Item) {
	if !b.CardUnit() {
		return
	}
	ev.Ordinal = item.Ordinal
	ev.Text = item.Text
	ev.Owner = item.Owner
	ev.Evidence = item.Evidence
	if item.Column != "" {
		ev.Column = item.Column
		if column := b.Column(item.Column); column != nil {
			ev.ColumnTitle = column.Title
		}
	}
}

// MemberAnchor answers a comment's or an item's anchor as a header and a body.
// Below the card-unit format it is the anchor file the member's directory
// holds. In the card-unit layout it is the anchor the old writers would have
// written for the member as the journal states it, which RenderMemberAnchor
// composes, so a write reads and edits one shape whichever layout it is on.
func (b *Bench) MemberAnchor(entity *EntityRef) (*Frontmatter, string, error) {
	if !b.CardUnit() {
		path, ok := legacyMemberFile(entity)
		if !ok {
			return nil, "", contract.Refuse(contract.UnknownPath, entity.Ref)
		}
		text, err := ReadText(path)
		if err != nil {
			return nil, "", contract.Refuse(contract.UnknownPath, entity.Ref)
		}
		fm, body := ParseAnchor(text)
		return fm, body, nil
	}
	text, err := b.MemberText(entity)
	if err != nil {
		return nil, "", err
	}
	fm, body := ParseAnchor(text)
	return fm, body, nil
}

// MemberText answers the text `dinah show` prints for a comment or an item:
// the anchor file below the card-unit format, and in that layout the anchor
// RenderMemberAnchor composes from the journal.
func (b *Bench) MemberText(entity *EntityRef) (string, error) {
	if !b.CardUnit() {
		path, ok := legacyMemberFile(entity)
		if !ok {
			return "", contract.Refuse(contract.UnknownPath, entity.Ref)
		}
		text, err := ReadText(path)
		if err != nil {
			return "", contract.Refuse(contract.UnknownPath, entity.Ref)
		}
		return text, nil
	}
	comment, item, err := b.memberOf(entity)
	if err != nil {
		return "", err
	}
	if comment != nil {
		return RenderCommentAnchor(comment), nil
	}
	return RenderItemAnchor(item), nil
}

// MemberTextOf answers the text show prints for one of the record's comments
// or items, taken from what the record already read rather than from the
// store again: the anchor file as it was read on the old layout, and the
// anchor RenderCommentAnchor or RenderItemAnchor composes in the card-unit
// layout. It answers false for a member the record does not hold.
func (r *CardRecord) MemberTextOf(kind, id string) (string, bool) {
	switch kind {
	case KindComment:
		comment, ok := r.Comment(id)
		if !ok {
			return "", false
		}
		if !r.journaled {
			return comment.raw, true
		}
		return RenderCommentAnchor(comment), true
	case KindItem:
		item, ok := r.Item(id)
		if !ok {
			return "", false
		}
		if !r.journaled {
			return item.raw, true
		}
		return RenderItemAnchor(item), true
	}
	return "", false
}

// MemberAnchorOf is MemberTextOf parsed into a header and a body.
func (r *CardRecord) MemberAnchorOf(kind, id string) (*Frontmatter, string, bool) {
	text, ok := r.MemberTextOf(kind, id)
	if !ok {
		return nil, "", false
	}
	fm, body := ParseAnchor(text)
	return fm, body, true
}

// memberOf answers the comment or the item an entity reference names, read
// through the card's record or the workbench journal.
func (b *Bench) memberOf(entity *EntityRef) (*Comment, *Item, error) {
	if entity.Card == nil {
		comments, err := b.columnCommentsBothHalves(entity.Holder)
		if err != nil {
			return nil, nil, err
		}
		for _, comment := range comments {
			if comment.ID == entity.ID {
				return comment, nil, nil
			}
		}
		return nil, nil, contract.Refuse(contract.UnknownPath, entity.Ref)
	}
	record, err := b.LoadCardRecord(entity.Card)
	if err != nil {
		return nil, nil, err
	}
	switch entity.Kind {
	case KindComment:
		if comment, ok := record.Comment(entity.ID); ok {
			return comment, nil, nil
		}
	case KindItem:
		if item, ok := record.Item(entity.ID); ok {
			return nil, item, nil
		}
	}
	return nil, nil, contract.Refuse(contract.UnknownPath, entity.Ref)
}

// columnCommentsBothHalves answers every comment of one column, live and
// archived.
func (b *Bench) columnCommentsBothHalves(columnID string) ([]*Comment, error) {
	live, err := b.ColumnComments(columnID, LiveHalf)
	if err != nil {
		return nil, err
	}
	archived, err := b.ColumnComments(columnID, ArchivedHalf)
	if err != nil {
		return nil, err
	}
	return append(live, archived...), nil
}

// WriteMemberAnchor persists a member anchor an act has edited. Below the
// card-unit format it rewrites the anchor file, stamping a comment's digest
// over the body being written. In the card-unit layout it writes nothing,
// because the act's own journal line, completed by CompleteMemberLine, is the
// change.
func (b *Bench) WriteMemberAnchor(entity *EntityRef, fm *Frontmatter, body string) error {
	if b.CardUnit() {
		return nil
	}
	switch entity.Kind {
	case KindComment:
		return WriteCommentAnchor(entity.Dir, fm, body)
	case KindItem:
		return WriteItemAnchor(entity.Dir, fm, body)
	}
	return contract.Refuse(contract.UnknownPath, entity.Ref)
}

// CompleteMemberLine fills the journal line of an act on a comment or an item
// with what the card-unit layout stores on it, given the member's anchor as
// the act left it: the new text on a body or text write, the answer of record
// on a settling line, the observation on a citation, and on an archive, a
// restore or a deletion the member named in its own right. Below that format
// the line is left exactly as the old layout writes it.
func (b *Bench) CompleteMemberLine(ev *Event, entity *EntityRef, fm *Frontmatter, body string) {
	if !b.CardUnit() {
		return
	}
	switch ev.Event {
	case contract.EventCommentUpdated:
		if ev.Field == "body" {
			ev.Text = body
		}
	case contract.EventItemUpdated:
		if ev.Field == "text" {
			ev.Text = strings.TrimRight(body, "\n")
		}
	case contract.EventItemResolved, contract.EventItemVerified, contract.EventItemFailed,
		contract.EventItemWaived, contract.EventItemWithdrawn:
		ev.Resolution = fm.Value(ItemResolutionField)
	case contract.EventItemCited:
		if citations := CitationsOf(fm); len(citations) > 0 {
			ev.Observed = observedOf(citations[len(citations)-1])
		}
	case contract.EventArchived, contract.EventRestored, contract.EventDeleted:
		b.nameMember(ev, entity)
	}
}

// nameMember names the member an archive, restore or deletion line is about,
// in its own right: the comment and, for an item comment, the item beside it,
// or the item; a column comment carries its column and the column's title.
func (b *Bench) nameMember(ev *Event, entity *EntityRef) {
	switch entity.Kind {
	case KindComment:
		ev.Comment = entity.ID
		if entity.Card == nil {
			ev.Column = entity.Holder
			if column := b.Column(entity.Holder); column != nil {
				ev.ColumnTitle = column.Title
			}
			return
		}
		ev.Item = entity.Holder
	case KindItem:
		ev.Item = entity.ID
	}
}

// RunEntityAct performs an archive, a restore or a deletion of any entity. A
// comment or an item in the card-unit layout has no directory to move, so its
// act is the act's own line on the nearest journal, appended under the lock of
// the card that holds it, or of the workbench for a column comment, after the
// act's own Verify; a deleted comment's attachments go with it, as a comment's
// directory took them on the old layout, and so do a deleted item's comments'.
// Every other act, and every act below the card-unit format, runs the
// structural protocol Run carries.
func (b *Bench) RunEntityAct(act *StructuralAct, entity *EntityRef) error {
	member := entity.Kind == KindComment || entity.Kind == KindItem
	if !member || !b.CardUnit() {
		return b.Run(act)
	}
	lockDir := act.LockDir
	if lockDir == "" {
		lockDir = b.Root
	}
	held, err := b.Acquire(lockDir, act.Actor, act.Now)
	if err != nil {
		return err
	}
	defer held.Release()
	if act.Verify != nil {
		if err := act.Verify(); err != nil {
			return err
		}
	}
	locks := ActLocks{bench: held}
	if lockDir != b.Root {
		locks = ActLocks{entity: held}
	}
	homes, err := b.homesRemovedBy(act, entity)
	if err != nil {
		return err
	}
	if err := act.Record(locks); err != nil {
		return err
	}
	for _, home := range homes {
		if Exists(home) {
			if err := durable.RemoveAll(home); err != nil {
				return err
			}
		}
	}
	return nil
}

// homesRemovedBy answers the attachment directories a deletion takes with it
// in the card-unit layout: a deleted comment's own, and each comment's of a
// deleted item. An archive or a restore removes nothing.
func (b *Bench) homesRemovedBy(act *StructuralAct, entity *EntityRef) ([]string, error) {
	if act.Op != OpDelete {
		return nil, nil
	}
	if entity.Kind == KindComment {
		return []string{entity.Dir}, nil
	}
	record, err := b.LoadCardRecord(entity.Card)
	if err != nil {
		return nil, err
	}
	record.holdEverything()
	var homes []string
	for _, comment := range record.Comments {
		if comment.Holder == entity.ID {
			homes = append(homes, comment.Home)
		}
	}
	return homes, nil
}

// Items answers a card's live checklist items in creation order.
func (b *Bench) Items(card *Card) ([]*Item, error) {
	record, err := b.LoadCardRecord(card)
	if err != nil {
		return nil, err
	}
	return record.ItemsIn(LiveHalf, ""), nil
}

// BlockingItems answers the live checklist items of a card that would refuse a
// claim right now. An item whose anchor will not open on the old layout is
// read past, since an unreadable file is a defect dinah check reports rather
// than one a claim or a move discovers.
func (b *Bench) BlockingItems(card *Card) ([]*Item, error) {
	return b.itemsWhere(card, b.ItemBlocksClaim)
}

// GatingItems answers the live checklist items of a card that hold it against
// one column right now. An item holds when its own column field names the
// column and its state does not lift the hold, and that is the whole test:
// every kind an item can carry holds on the same terms, because CORE-GATE-1
// puts the selectivity in which items name a column rather than in the column
// or in the tool.
//
// Which side of that column the items hold is the caller's question rather
// than this one's. canLand asks twice for a move, once against the column the
// card would arrive at and once against the column it would leave, and this
// answers the same way both times. The column is named by identifier, which is
// what an item's column field carries.
func (b *Bench) GatingItems(card *Card, columnID string) ([]*Item, error) {
	if columnID == "" {
		return nil, nil
	}
	return b.itemsWhere(card, func(item *Item) bool {
		return item.Column == columnID && !ItemLiftsColumnHold(item)
	})
}

// itemsWhere answers a card's live items a predicate admits, in identifier
// order, which is the order a refusal naming the first such item has always
// named it in.
func (b *Bench) itemsWhere(card *Card, keep func(*Item) bool) ([]*Item, error) {
	items, err := b.Items(card)
	if err != nil {
		return nil, err
	}
	var kept []*Item
	for _, item := range items {
		if keep(item) {
			kept = append(kept, item)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].ID < kept[j].ID })
	return kept, nil
}

// CountBlockingItems reports how many of a card's checklist items would refuse
// a claim right now, for a reader that wants the number rather than the items.
func (b *Bench) CountBlockingItems(card *Card) (int, error) {
	items, err := b.BlockingItems(card)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

// TallyItems counts, over a card's items, the items that would refuse a claim
// and the items waiting on the operator, so a caller wanting both numbers pays
// for one walk rather than two.
func (b *Bench) TallyItems(items []*Item) ItemTally {
	var tally ItemTally
	for _, item := range items {
		if b.ItemBlocksClaim(item) {
			tally.Blocking++
		}
		if ItemAwaitsOperator(item) {
			tally.AwaitingOperator++
		}
	}
	return tally
}

// RenderCommentAnchor composes the anchor the old writers would have written
// for a comment, with the keys in the order those writers write them: ts,
// author or author_unrecoverable, ordinal and digest, each present only where
// it holds a value, then the body. A redacted comment carries redacted: true
// after the ordinal and no body.
func RenderCommentAnchor(comment *Comment) string {
	fm := NewFrontmatter()
	if comment.TS != "" {
		fm.Set("ts", comment.TS)
	}
	switch {
	case comment.Author != "":
		fm.Set("author", comment.Author)
	case comment.AuthorUnrecoverable:
		fm.Set(CommentAuthorUnrecoverableField, "true")
	}
	if comment.Ordinal > 0 {
		fm.Set(OrdinalField, strconv.Itoa(comment.Ordinal))
	}
	if comment.Redacted != nil {
		fm.Set(RedactedField, "true")
		return fm.Render("")
	}
	if comment.RecordedDigest != "" {
		fm.Set(CommentDigestField, comment.RecordedDigest)
	}
	return fm.Render(comment.Body)
}

// RenderItemAnchor composes the anchor the old writers would have written for
// an item. Those writers put a filing's keys in one order, kind, state,
// column, owner, standing, evidence, ts and ordinal, and appended every key a
// later act added, in the order the acts added it, so the replay tracks that
// order and this follows it; an item carrying no tracked order takes the
// filing order with resolution and citations after it. Each key is present
// only where it holds a value, and the text follows exactly as the anchor
// carried it. A redacted item carries redacted: true after the ordinal and no
// text.
func RenderItemAnchor(item *Item) string {
	values := map[string]string{
		ItemKindField:       item.Kind,
		ItemStateField:      item.State,
		ItemColumnField:     item.Column,
		ItemOwnerField:      item.Owner,
		ItemStandingField:   item.Standing,
		ItemEvidenceField:   item.Evidence,
		"ts":                item.TS,
		ItemResolutionField: item.Resolution,
	}
	if item.Ordinal > 0 {
		values[OrdinalField] = strconv.Itoa(item.Ordinal)
	}
	order := item.keys
	if order == nil {
		order = append(append([]string(nil), filingKeys...), ItemResolutionField, CitationsField)
	}
	fm := NewFrontmatter()
	for _, key := range order {
		switch {
		case key == CitationsField:
			for _, citation := range item.Citations {
				AppendCitation(fm, citation)
			}
		case values[key] != "":
			fm.Set(key, values[key])
		}
		if key == OrdinalField && item.Redacted != nil {
			fm.Set(RedactedField, "true")
		}
	}
	if item.Redacted != nil {
		return fm.Render("")
	}
	body := item.body
	if body == "" {
		body = item.Text
	}
	return fm.Render(body)
}

// RedactedField is the anchor key a composed anchor carries for a member whose
// text dinah redact replaced.
const RedactedField = "redacted"

// ItemEntity answers the entity reference one of the record's items resolves
// to under ref: its directory on the old layout, and none in the card-unit
// layout, where an item is lines of the card's journal.
func (r *CardRecord) ItemEntity(item *Item, ref string) *EntityRef {
	return &EntityRef{Kind: KindItem, Dir: item.dir, ID: item.ID, Ref: ref, Card: r.Card, Archived: item.Archived, Journaled: r.journaled}
}

// CommentEntity answers the entity reference one of the record's comments
// resolves to under ref, its directory being the one its attachments hang
// from.
func (r *CardRecord) CommentEntity(comment *Comment, ref string) *EntityRef {
	return &EntityRef{
		Kind:      KindComment,
		Dir:       comment.Home,
		ID:        comment.ID,
		Ref:       ref,
		Card:      r.Card,
		Holder:    comment.Holder,
		Archived:  commentArchived(r, comment),
		Journaled: r.journaled,
	}
}

// Diverged reports whether a comment's recorded digest disagrees with the body
// standing beside it, which means the body was edited by something other than
// a verb since the digest was recorded. A comment carrying no digest is not
// diverged.
func (c *Comment) Diverged() bool {
	return c.RecordedDigest != "" && c.RecordedDigest != CommentDigest(c.Body)
}
