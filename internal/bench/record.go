package bench

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// MemberCollection names one collection instance whose ordinals form one
// sequence: the card's own comments, one item's comments, the card's
// checklist, or one column's comments.
type MemberCollection struct {
	// Kind is KindComment or KindItem.
	Kind string
	// Holder is, for comments, empty for the card, an item's identifier, or
	// a column's identifier; for items it is empty.
	Holder string
}

// CardRecord is one card as its two files state it: the anchor's fields and
// body, and every member the journal establishes. On a store below
// CardUnitFormat the members are read from the directories the old layout
// keeps them in instead, by recordFromDirectories, so every reader asks one
// question of one type whichever layout the store is in.
type CardRecord struct {
	// Card is read from card.md, number stamped, exactly as LoadCardIn reads
	// it.
	Card *Card
	// Events are the whole journal in file order.
	Events []Event
	// Torn is ReadJournal's torn-tail report.
	Torn bool
	// Comments are every comment not deleted: the card's own and every
	// item's, both halves.
	Comments map[string]*Comment
	// Items are every item not deleted, both halves.
	Items map[string]*Item

	// journaled is true where the members were replayed from the journal,
	// which is the card-unit layout, and false where they were read from
	// the old layout's directories.
	journaled bool
	// rank orders members that share an ordinal by the file order of the
	// line that established them, or on the old layout by the order the
	// directory reader sorted them in.
	rank map[string]int
	// ordinals are the highest ordinal each collection instance has ever
	// recorded, deleted and archived members included, which is what
	// NextOrdinal counts from so no ordinal is reissued.
	ordinals map[MemberCollection]int
	// named are every member identifier the journal has ever named, in
	// either half and including deleted ones, which MintID draws against.
	named map[string]bool
	// placed are, on the old layout, every member identifier each
	// collection directory lists, in the order a position counts in, with
	// a member whose anchor will not read still in its place. It is nil on
	// the card-unit layout, where the journal is the collection.
	placed map[placedKey][]string
	// unknown are the lines naming a member the replay never established,
	// by one-based line number.
	unknown []int
	// fileOf answers the file that holds one member: its own anchor on the
	// old layout, and the card's journal on the card-unit layout.
	fileOf func(kind, id string) string

	// commentHomes are, on the old layout, the directory each comment
	// holder occupies, keyed by the holder as Comment.Holder spells it. The
	// holder's comments are read by load the first time a question asks
	// about them, and listed counts them without reading an anchor. All
	// four are nil on the card-unit layout, where the journal has already
	// been read whole.
	commentHomes map[string]string
	loaded       map[string]bool
	load         func(holder string)
	listed       func(holder string, half ResolutionHalf) (int, error)
	// itemHalves are, on the old layout, the halves of the checklist
	// loadItems has read. The live half is read when the record is made and
	// the archived half on the first question that reaches it. Both are nil
	// on the card-unit layout.
	itemHalves map[ResolutionHalf]bool
	loadItems  func(half ResolutionHalf) error
}

// holdItems makes sure one half of the checklist has been read. A half the
// directory reader cannot list reads as holding no item, which is what a
// question about a single half answered when every half was read up front
// and the one that failed refused the whole record: the failure is reported
// by the read that makes the record, for the live half, and by dinah check.
func (r *CardRecord) holdItems(half ResolutionHalf) error {
	if r.loadItems == nil || r.itemHalves[half] {
		return nil
	}
	r.itemHalves[half] = true
	return r.loadItems(half)
}

// holdArchivedItems reads the archived half of the checklist on the old
// layout, whose failure a question about one member cannot report.
func (r *CardRecord) holdArchivedItems() {
	_ = r.holdItems(ArchivedHalf)
}

// commentHome answers the directory a comment holder occupies on the old
// layout, reading the archived half of the checklist first where the holder
// is not an item of the live half.
func (r *CardRecord) commentHome(holder string) (string, bool) {
	if home, known := r.commentHomes[holder]; known {
		return home, true
	}
	r.holdArchivedItems()
	home, known := r.commentHomes[holder]
	return home, known
}

// holdComments makes sure one holder's comments have been read.
func (r *CardRecord) holdComments(holder string) {
	if r.load == nil || r.loaded[holder] {
		return
	}
	if _, known := r.commentHome(holder); !known {
		return
	}
	r.loaded[holder] = true
	r.load(holder)
}

// holdAllComments makes sure every holder's comments have been read, the
// archived items' included.
func (r *CardRecord) holdAllComments() {
	r.holdArchivedItems()
	for holder := range r.commentHomes {
		r.holdComments(holder)
	}
}

// holdEverything makes sure every member of the card has been read, for a
// question that walks them all.
func (r *CardRecord) holdEverything() {
	r.holdAllComments()
}

// LoadCardRecord reads one card's members. On a store in the card-unit layout
// it replays the card's journal; below that format it reads the directories
// the old layout keeps members in, through recordFromDirectories. Every member
// read of a card goes through here, so no reader outside this package touches
// either layout.
//
// A journal on which two members carry one identifier is damage, and the card
// is refused dinah.journal-unreadable naming the journal and the identifier,
// since the members' maps would otherwise merge the two silently.
func (b *Bench) LoadCardRecord(card *Card) (*CardRecord, error) {
	if !b.CardUnit() {
		return b.recordFromDirectories(card)
	}
	events, torn, err := ReadJournal(card.JournalPath())
	if err != nil {
		return nil, err
	}
	replay := replayMembers(events)
	if len(replay.collisions) > 0 {
		return nil, refuseCollision(card.JournalPath(), replay.collisions[0])
	}
	for _, comment := range replay.comments {
		comment.Home = filepath.Join(card.Dir, CommentsDir, comment.ID)
	}
	journal := card.JournalPath()
	return &CardRecord{
		Card:      card,
		Events:    events,
		Torn:      torn,
		Comments:  replay.comments,
		Items:     replay.items,
		journaled: true,
		rank:      replay.rank,
		ordinals:  replay.ordinals,
		named:     replay.named,
		unknown:   replay.unknown,
		fileOf:    func(string, string) string { return journal },
	}, nil
}

// MemberIDs answers one collection instance's members in one half, in the
// order a position counts them in, narrowed to one item kind unless kind is
// empty. On the old layout a member whose anchor will not read keeps its place,
// because a position counts the directory unfiltered, and it is left out only
// where a kind narrows the list, since its kind cannot be read.
func (r *CardRecord) MemberIDs(collection MemberCollection, half ResolutionHalf, kind string) []string {
	if collection.Kind == KindComment {
		r.holdComments(collection.Holder)
	} else {
		_ = r.holdItems(half)
	}
	if r.placed != nil {
		var ids []string
		for _, id := range r.placed[placedKey{collection: collection, half: half}] {
			if kind != "" {
				item, ok := r.Items[id]
				if !ok || item.Kind != kind {
					continue
				}
			}
			ids = append(ids, id)
		}
		return ids
	}
	var ids []string
	if collection.Kind == KindItem {
		for _, item := range r.ItemsIn(half, kind) {
			ids = append(ids, item.ID)
		}
		return ids
	}
	for _, comment := range r.CommentsOf(collection.Holder, half) {
		ids = append(ids, comment.ID)
	}
	return ids
}

// HeldComments answers the comments hanging on one holder that stand in one
// half in their own right, in the order a position counts them in: the half a
// comment was archived into itself, whatever half the item it hangs on stands
// in. It is what a view of one holder lists, so an archived item shows the
// comments it carried as they stood.
func (r *CardRecord) HeldComments(holder string, half ResolutionHalf) []*Comment {
	r.holdComments(holder)
	if r.placed != nil {
		var held []*Comment
		for _, id := range r.placed[placedKey{collection: MemberCollection{Kind: KindComment, Holder: holder}, half: half}] {
			if comment, ok := r.Comments[id]; ok {
				held = append(held, comment)
			}
		}
		return held
	}
	var held []*Comment
	for _, comment := range r.Comments {
		if comment.Holder == holder && comment.Archived == (half == ArchivedHalf) {
			held = append(held, comment)
		}
	}
	sort.SliceStable(held, func(i, j int) bool {
		return r.before(held[i].Ordinal, held[i].ID, held[j].Ordinal, held[j].ID)
	})
	return held
}

// HeldCount answers how many comments hang on one holder in one half in their
// own right, counting on the old layout every directory the collection holds,
// as a listing always has, whether or not its anchor reads. A collection
// directory that will not list is reported rather than counted as none, since
// a zero is what a holder carrying nothing answers.
func (r *CardRecord) HeldCount(holder string, half ResolutionHalf) (int, error) {
	if r.listed != nil && !r.loaded[holder] {
		return r.listed(holder, half)
	}
	if r.placed != nil {
		return len(r.placed[placedKey{collection: MemberCollection{Kind: KindComment, Holder: holder}, half: half}]), nil
	}
	return len(r.HeldComments(holder, half)), nil
}

// HeldPosition answers a comment's one-based place among the comments
// HeldComments counts, and zero when it is not one of them. On the old layout
// a comment whose anchor will not read keeps its place, as the directory
// counts it.
func (r *CardRecord) HeldPosition(holder string, half ResolutionHalf, id string) int {
	r.holdComments(holder)
	if r.placed != nil {
		for n, member := range r.placed[placedKey{collection: MemberCollection{Kind: KindComment, Holder: holder}, half: half}] {
			if member == id {
				return n + 1
			}
		}
		return 0
	}
	for n, comment := range r.HeldComments(holder, half) {
		if comment.ID == id {
			return n + 1
		}
	}
	return 0
}

// Position answers a member's one-based place among the members of its
// collection instance in one half, and zero when it is not one of them.
func (r *CardRecord) Position(collection MemberCollection, half ResolutionHalf, id string) int {
	for n, member := range r.MemberIDs(collection, half, "") {
		if member == id {
			return n + 1
		}
	}
	return 0
}

// FileOf answers the file that holds one member, which is the file a finding
// about the member names: its own anchor below the card-unit format, and the
// card's journal in that layout.
func (r *CardRecord) FileOf(kind, id string) string {
	return r.fileOf(kind, id)
}

// ColumnComments answers one column's comments in one half, in ordinal order
// with ties broken by file order. In the card-unit layout they are members of
// the workbench journal, and a comment is in the archived half when it or its
// column is archived; the comments of a column the journal records as deleted
// are gone with it. Below that format they are read from the column's own
// directory.
func (b *Bench) ColumnComments(columnID string, half ResolutionHalf) ([]*Comment, error) {
	if !b.CardUnit() {
		return b.columnCommentsFromDirectories(columnID, half)
	}
	events, _, err := ReadJournal(b.JournalPath())
	if err != nil {
		return nil, err
	}
	replay := replayMembers(events)
	var found []*Comment
	for _, comment := range replay.comments {
		if comment.Holder != columnID || comment.Archived != (half == ArchivedHalf) {
			continue
		}
		comment.Home = filepath.Join(b.columnDirIn(columnHalf(b, columnID), columnID), CommentsDir, comment.ID)
		found = append(found, comment)
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Ordinal != found[j].Ordinal {
			return found[i].Ordinal < found[j].Ordinal
		}
		return replay.rank[found[i].ID] < replay.rank[found[j].ID]
	})
	return found, nil
}

// ColumnCommentCount answers how many live comments one column carries, which
// a listing of the flow prints beside each station. Below the card-unit format
// it counts the column's comments directory without reading an anchor.
func (b *Bench) ColumnCommentCount(columnID string) (int, error) {
	if !b.CardUnit() {
		return CountComments(b.ColumnDir(columnID))
	}
	comments, err := b.ColumnComments(columnID, LiveHalf)
	if err != nil {
		return 0, err
	}
	return len(comments), nil
}

// NextColumnOrdinal answers the ordinal a new comment on one column takes:
// one more than the highest the workbench journal has ever recorded for that
// column's comments.
func (b *Bench) NextColumnOrdinal(columnID string) (int, error) {
	events, _, err := ReadJournal(b.JournalPath())
	if err != nil {
		return 0, err
	}
	replay := replayMembers(events)
	return replay.ordinals[MemberCollection{Kind: KindComment, Holder: columnID}] + 1, nil
}

// MintColumnCommentID draws a new column comment's identifier, redrawn until
// it differs from every comment identifier the workbench journal has named.
func (b *Bench) MintColumnCommentID() (string, error) {
	events, _, err := ReadJournal(b.JournalPath())
	if err != nil {
		return "", err
	}
	named := replayMembers(events).named
	for {
		id, err := mintID()
		if err != nil {
			return "", err
		}
		if !named[id] {
			return id, nil
		}
	}
}

// columnHalf answers the half a column's own directory stands in: the live
// half for a column the workbench declares, and the archived half otherwise.
func columnHalf(b *Bench, columnID string) ResolutionHalf {
	if b.Column(columnID) != nil {
		return LiveHalf
	}
	return ArchivedHalf
}

// placedKey is one collection directory of the old layout: a collection
// instance and the half its directory holds.
type placedKey struct {
	collection MemberCollection
	half       ResolutionHalf
}

// mintID is how a new member's identifier is drawn. A test seeds it to
// return an identifier the journal already names, which is how MintID's
// redraw is reached.
var mintID = NewID

// Comment answers one comment by identifier, in either half.
func (r *CardRecord) Comment(id string) (*Comment, bool) {
	if _, ok := r.Comments[id]; !ok {
		r.holdAllComments()
	}
	comment, ok := r.Comments[id]
	return comment, ok
}

// Item answers one item by identifier, in either half.
func (r *CardRecord) Item(id string) (*Item, bool) {
	if _, ok := r.Items[id]; !ok {
		r.holdArchivedItems()
	}
	item, ok := r.Items[id]
	return item, ok
}

// CommentsOf answers the comments one holder carries in one half, in
// ordinal order with ties broken by file order. holder is empty for the
// card's own comments and an item's identifier otherwise.
func (r *CardRecord) CommentsOf(holder string, half ResolutionHalf) []*Comment {
	r.holdComments(holder)
	var found []*Comment
	for _, comment := range r.Comments {
		if comment.Holder != holder || commentArchived(r, comment) != (half == ArchivedHalf) {
			continue
		}
		found = append(found, comment)
	}
	sort.SliceStable(found, func(i, j int) bool {
		return r.before(found[i].Ordinal, found[i].ID, found[j].Ordinal, found[j].ID)
	})
	return found
}

// ItemsIn answers the card's items in one half, narrowed to one kind unless
// kind is empty, in ordinal order with ties broken by file order.
func (r *CardRecord) ItemsIn(half ResolutionHalf, kind string) []*Item {
	_ = r.holdItems(half)
	var found []*Item
	for _, item := range r.Items {
		if item.Archived != (half == ArchivedHalf) {
			continue
		}
		if kind != "" && item.Kind != kind {
			continue
		}
		found = append(found, item)
	}
	sort.SliceStable(found, func(i, j int) bool {
		return r.before(found[i].Ordinal, found[i].ID, found[j].Ordinal, found[j].ID)
	})
	return found
}

// before orders two members by ordinal and then by rank.
func (r *CardRecord) before(ordinalA int, idA string, ordinalB int, idB string) bool {
	if ordinalA != ordinalB {
		return ordinalA < ordinalB
	}
	return r.rank[idA] < r.rank[idB]
}

// commentArchived answers a comment's half: archived when the comment is,
// or when the item it hangs on is.
func commentArchived(r *CardRecord, comment *Comment) bool {
	if comment.Archived {
		return true
	}
	if comment.Holder == "" {
		return false
	}
	item, ok := r.Items[comment.Holder]
	return ok && item.Archived
}

// NextOrdinal answers the ordinal a new member of one collection instance
// takes: one more than the highest the record has ever seen in it, so an
// ordinal is never reissued after the member carrying it was deleted.
func (r *CardRecord) NextOrdinal(collection MemberCollection) int {
	if collection.Kind == KindComment {
		r.holdComments(collection.Holder)
	} else {
		r.holdArchivedItems()
	}
	return r.ordinals[collection] + 1
}

// MintID draws a new member identifier, redrawn until it differs from every
// comment and item identifier the card has ever named, in either half and
// including deleted ones. The card's lock, which every member write holds,
// serialises the check and the append that follows it.
func (r *CardRecord) MintID() (string, error) {
	r.holdEverything()
	for {
		id, err := mintID()
		if err != nil {
			return "", err
		}
		if !r.named[id] {
			r.named[id] = true
			return id, nil
		}
	}
}

// MemberCollision is one identifier two members of one journal carry, with
// the one-based line numbers of the line that established each.
type MemberCollision struct {
	ID    string
	First int
	Then  int
}

// collide records that the line at index i establishes a member under an
// identifier another member already carries.
func (r *memberReplay) collide(id string, i int) {
	r.collisions = append(r.collisions, MemberCollision{ID: id, First: r.rank[id] + 1, Then: i + 1})
}

// refuseCollision is the refusal a journal whose members collide is read
// with: the journal is the detail, and the identifier and both members' first
// lines travel as values, so the sentence and dinah check can name all three.
func refuseCollision(journal string, collision MemberCollision) error {
	return contract.RefuseWith(contract.JournalUnreadable, journal, map[string]string{
		"member": collision.ID,
		"lines":  strconv.Itoa(collision.First) + ", " + strconv.Itoa(collision.Then),
	})
}

// UnknownMemberLines are the one-based numbers of the journal lines naming a
// member the replay never established.
func (r *CardRecord) UnknownMemberLines() []int {
	return append([]int(nil), r.unknown...)
}

// memberReplay is one walk of a journal's member lines, carrying everything a
// CardRecord or a column's comments are built from.
type memberReplay struct {
	comments   map[string]*Comment
	items      map[string]*Item
	rank       map[string]int
	ordinals   map[MemberCollection]int
	named      map[string]bool
	collisions []MemberCollision
	unknown    []int
	// columns are the columns the workbench journal records archived or
	// deleted, read off lines naming no member.
	archivedColumns map[string]bool
	deletedColumns  map[string]bool
	// deleted are the members a deleted line removed, an item's comments
	// with it, so a redacted line written afterwards is not read as naming a
	// member the replay never established.
	deleted map[string]bool
	// onColumns are the comments that hang on a column rather than on a
	// card or an item.
	onColumns map[string]bool
	// keys track, for each item, the order the old layout's writers would
	// have put its anchor keys in, which RenderItemAnchor composes by.
	keys map[string]*keyOrder
}

// keyOrder is one item's anchor keys in the order the old writers would have
// written them: the keys a filing writes, in the filing's own order, then
// each key a later act added, in the order the acts added them. A key an act
// rewrites keeps its place, and a key an act clears goes.
type keyOrder struct {
	list  []string
	filed map[string]bool
}

// has reports whether the order carries a key.
func (k *keyOrder) has(key string) bool {
	for _, present := range k.list {
		if present == key {
			return true
		}
	}
	return false
}

// add places a key at the end of the order unless it is already there.
func (k *keyOrder) add(key string) {
	if !k.has(key) {
		k.list = append(k.list, key)
	}
}

// drop takes a key out of the order.
func (k *keyOrder) drop(key string) {
	kept := k.list[:0]
	for _, present := range k.list {
		if present != key {
			kept = append(kept, present)
		}
	}
	k.list = kept
}

// filingKeys is the order the old layout's filing writers put an item's keys
// in, every key present only where the item carries a value for it.
var filingKeys = []string{
	ItemKindField, ItemStateField, ItemColumnField, ItemOwnerField,
	ItemStandingField, ItemEvidenceField, "ts", OrdinalField,
}

// itemKeyValues answers which of an item's anchor keys carry a value.
func itemKeyValues(item *Item) map[string]bool {
	return map[string]bool{
		ItemKindField:       item.Kind != "",
		ItemStateField:      item.State != "",
		ItemColumnField:     item.Column != "",
		ItemOwnerField:      item.Owner != "",
		ItemStandingField:   item.Standing != "",
		ItemEvidenceField:   item.Evidence != "",
		"ts":                item.TS != "",
		OrdinalField:        item.Ordinal > 0,
		ItemResolutionField: item.Resolution != "",
		CitationsField:      len(item.Citations) > 0,
	}
}

// track follows one line's effect on an item's key order, whatever side of
// the migration marker it stands on, since the lines before the marker are
// the acts the old writers performed and are what orders a baselined item's
// keys.
func (r *memberReplay) track(ev Event) {
	switch ev.Event {
	case contract.EventItemFiled:
		order := &keyOrder{filed: map[string]bool{}}
		present := map[string]bool{
			ItemKindField: true, ItemStateField: true, "ts": true, OrdinalField: true,
			ItemColumnField: ev.Column != "", ItemOwnerField: ev.Owner != "",
			ItemStandingField: ev.Standing != "", ItemEvidenceField: ev.Evidence != "",
		}
		for _, key := range filingKeys {
			if present[key] {
				order.list = append(order.list, key)
				order.filed[key] = true
			}
		}
		r.keys[ev.Item] = order
	case contract.EventItemUpdated:
		order := r.keys[ev.Note]
		if order == nil {
			return
		}
		switch ev.Field {
		case ItemColumnField, ItemOwnerField, ItemEvidenceField, ItemResolutionField:
		default:
			return
		}
		switch {
		case ev.To == "":
			order.drop(ev.Field)
		case ev.From != "" && !order.has(ev.Field):
			order.filed[ev.Field] = true
		default:
			order.add(ev.Field)
		}
	case contract.EventItemCited:
		if order := r.keys[ev.Item]; order != nil {
			order.add(CitationsField)
		}
	case contract.EventItemResolved, contract.EventItemVerified, contract.EventItemFailed,
		contract.EventItemWaived, contract.EventItemWithdrawn:
		if order := r.keys[ev.Item]; order != nil {
			order.add(ItemResolutionField)
		}
	case contract.EventItemReopened:
		if order := r.keys[ev.Item]; order != nil {
			order.drop(ItemResolutionField)
		}
	}
}

// baselineKeys orders a baselined item's keys: the filing keys it carries a
// value for, in the filing writers' order, except those a later act added,
// then the keys the history shows later acts adding, in that order, then any
// key it carries that the history accounts for neither way.
func (r *memberReplay) baselineKeys(item *Item) *keyOrder {
	present := itemKeyValues(item)
	history := r.keys[item.ID]
	appended := map[string]bool{}
	var later []string
	if history != nil {
		for _, key := range history.list {
			if !history.filed[key] {
				appended[key] = true
				later = append(later, key)
			}
		}
	}
	order := &keyOrder{filed: map[string]bool{}}
	for _, key := range filingKeys {
		if present[key] && !appended[key] {
			order.list = append(order.list, key)
			order.filed[key] = true
		}
	}
	for _, key := range later {
		if present[key] {
			order.list = append(order.list, key)
		}
	}
	for _, key := range []string{ItemResolutionField, CitationsField} {
		if present[key] && !order.has(key) {
			order.list = append(order.list, key)
		}
	}
	return order
}

// ReplayMembers walks a journal's lines in file order and answers every
// comment and item they establish, by the member replay rule of the card-unit
// layout.
//
// The last card_baseline on a card journal, or the last storage_migrated on
// the workbench journal, marks where the storage migration finished. A line
// at or before it changes a member only when it is a baseline, because the
// baselines state every member in full and the history before them carries
// the members' text nowhere. A baseline replaces the whole of the member it
// names wherever it stands, so a member baselined twice takes the later
// line. Every line after the marker applies as the act that wrote it
// describes, and a line naming a member the replay has not established
// contributes nothing.
func ReplayMembers(events []Event) (map[string]*Comment, map[string]*Item) {
	replay := replayMembers(events)
	return replay.comments, replay.items
}

// replayMembers is ReplayMembers answering everything the walk gathered.
func replayMembers(events []Event) *memberReplay {
	r := &memberReplay{
		comments:        map[string]*Comment{},
		items:           map[string]*Item{},
		rank:            map[string]int{},
		ordinals:        map[MemberCollection]int{},
		named:           map[string]bool{},
		archivedColumns: map[string]bool{},
		deletedColumns:  map[string]bool{},
		deleted:         map[string]bool{},
		onColumns:       map[string]bool{},
		keys:            map[string]*keyOrder{},
	}
	marker := migrationMarker(events)
	for i, ev := range events {
		r.name(ev)
		if ev.Event != contract.EventItemBaseline {
			r.track(ev)
		}
		switch ev.Event {
		case contract.EventItemBaseline:
			r.baselineItem(i, ev)
			continue
		case contract.EventCommentBaseline:
			r.baselineComment(i, ev)
			continue
		}
		// A column's own archive, restore or deletion is about the column
		// rather than any member, and no baseline restates it, so it
		// applies wherever it stands: the half a column comment reads in
		// follows it after the migration exactly as before.
		if i <= marker && !namesAColumnAlone(ev) {
			continue
		}
		r.apply(i, ev)
	}
	// A line naming no member names a column on the workbench journal and
	// a card or an item on a card's, so it reaches only the comments that
	// hang on a column.
	for id, comment := range r.comments {
		if !r.onColumns[id] {
			continue
		}
		if r.deletedColumns[comment.Holder] {
			delete(r.comments, id)
			continue
		}
		if r.archivedColumns[comment.Holder] {
			comment.Archived = true
		}
	}
	for id, item := range r.items {
		if order := r.keys[id]; order != nil {
			item.keys = append([]string(nil), order.list...)
		}
	}
	return r
}

// namesAColumnAlone reports an archive, restore or deletion line naming no
// member, which on the workbench journal records a column's own act.
func namesAColumnAlone(ev Event) bool {
	switch ev.Event {
	case contract.EventArchived, contract.EventRestored, contract.EventDeleted:
		return ev.Comment == "" && ev.Item == "" && ev.Note != ""
	}
	return false
}

// migrationMarker answers the index of the last card_baseline, or failing
// that the last storage_migrated, and -1 where the journal carries neither.
func migrationMarker(events []Event) int {
	marker := -1
	for i, ev := range events {
		if ev.Event == contract.EventCardBaseline {
			marker = i
		}
	}
	if marker >= 0 {
		return marker
	}
	for i, ev := range events {
		if ev.Event == contract.EventStorageMigrated {
			marker = i
		}
	}
	return marker
}

// name records every member identifier a line names, whatever the line does
// with it, which is what MintID draws against.
func (r *memberReplay) name(ev Event) {
	if ev.Comment != "" {
		r.named[ev.Comment] = true
	}
	if ev.Item != "" {
		r.named[ev.Item] = true
	}
	switch ev.Event {
	case contract.EventCommentUpdated, contract.EventItemUpdated:
		if ev.Note != "" {
			r.named[ev.Note] = true
		}
	}
}

// count raises a collection instance's highest ordinal to one a line
// records.
func (r *memberReplay) count(collection MemberCollection, ordinal int) {
	if ordinal > r.ordinals[collection] {
		r.ordinals[collection] = ordinal
	}
}

// commentHolder is the holder a comment line names: the item, the column, or
// nobody for a card comment.
func commentHolder(ev Event) string {
	if ev.Item != "" {
		return ev.Item
	}
	return ev.Column
}

// baselineItem replaces an item's whole state with what an item_baseline
// line states.
func (r *memberReplay) baselineItem(i int, ev Event) {
	if _, exists := r.items[ev.Item]; !exists {
		if _, taken := r.comments[ev.Item]; taken {
			r.collide(ev.Item, i)
		}
		r.rank[ev.Item] = i
	}
	item := &Item{
		ID:         ev.Item,
		Kind:       ev.Kind,
		State:      ev.State,
		Ordinal:    ev.Ordinal,
		Column:     ev.Column,
		Owner:      ev.Owner,
		Resolution: ev.Resolution,
		Standing:   ev.Standing,
		Evidence:   ev.Evidence,
		Text:       strings.TrimRight(ev.Text, "\n"),
		body:       ev.Text,
		Archived:   ev.Archived,
		TS:         ev.Written,
	}
	for _, cited := range ev.Citations {
		item.Citations = append(item.Citations, citationOf(cited))
	}
	r.items[ev.Item] = item
	r.keys[ev.Item] = r.baselineKeys(item)
	r.count(MemberCollection{Kind: KindItem}, ev.Ordinal)
}

// baselineComment replaces a comment's whole state with what a
// comment_baseline line states.
func (r *memberReplay) baselineComment(i int, ev Event) {
	if _, exists := r.comments[ev.Comment]; !exists {
		if _, taken := r.items[ev.Comment]; taken {
			r.collide(ev.Comment, i)
		}
		r.rank[ev.Comment] = i
	}
	holder := commentHolder(ev)
	r.onColumns[ev.Comment] = ev.Item == "" && ev.Column != ""
	r.comments[ev.Comment] = &Comment{
		ID:                  ev.Comment,
		Holder:              holder,
		TS:                  ev.Written,
		Ordinal:             ev.Ordinal,
		Author:              ev.Author,
		AuthorUnrecoverable: ev.AuthorUnrecoverable,
		RecordedDigest:      ev.Digest,
		Body:                ev.Text,
		Archived:            ev.Archived,
	}
	r.count(MemberCollection{Kind: KindComment, Holder: holder}, ev.Ordinal)
}

// apply replays one line written after the migration marker.
func (r *memberReplay) apply(i int, ev Event) {
	switch ev.Event {
	case contract.EventCommented:
		r.create(i, ev)
	case contract.EventCommentUpdated:
		comment, ok := r.comments[ev.Note]
		if !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		if ev.Field == "body" {
			comment.Body = ev.Text
			comment.RecordedDigest = CommentDigest(ev.Text)
		}
	case contract.EventDivergenceAccepted:
		comment, ok := r.comments[ev.Comment]
		if !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		comment.RecordedDigest = CommentDigest(comment.Body)
	case contract.EventItemFiled:
		r.file(i, ev)
	case contract.EventItemUpdated:
		r.updateItem(i, ev)
	case contract.EventItemCited:
		item, ok := r.items[ev.Item]
		if !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		before, after, _ := ParseObserved(ev.Observed)
		item.Citations = append(item.Citations, Citation{Scheme: ev.Scheme, Target: ev.Target, Before: before, After: after})
	case contract.EventItemResolved, contract.EventItemVerified, contract.EventItemFailed,
		contract.EventItemWaived, contract.EventItemWithdrawn:
		r.settle(i, ev)
	case contract.EventItemReopened:
		item, ok := r.items[ev.Item]
		if !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		item.State = ItemPending
		item.Resolution = ""
	case contract.EventArchived, contract.EventRestored:
		r.shelve(i, ev, ev.Event == contract.EventArchived)
	case contract.EventDeleted:
		r.remove(i, ev)
	case contract.EventRedacted:
		r.redact(i, ev)
	}
}

// create establishes a comment a commented line wrote.
func (r *memberReplay) create(i int, ev Event) {
	if ev.Comment == "" {
		return
	}
	if _, exists := r.comments[ev.Comment]; exists {
		r.collide(ev.Comment, i)
		return
	}
	if _, taken := r.items[ev.Comment]; taken {
		r.collide(ev.Comment, i)
	}
	holder := commentHolder(ev)
	r.onColumns[ev.Comment] = ev.Item == "" && ev.Column != ""
	r.rank[ev.Comment] = i
	r.comments[ev.Comment] = &Comment{
		ID:             ev.Comment,
		Holder:         holder,
		TS:             ev.TS,
		Ordinal:        ev.Ordinal,
		Author:         ev.Actor.Name,
		RecordedDigest: CommentDigest(ev.Text),
		Body:           ev.Text,
	}
	r.count(MemberCollection{Kind: KindComment, Holder: holder}, ev.Ordinal)
}

// file establishes an item an item_filed line wrote.
func (r *memberReplay) file(i int, ev Event) {
	if ev.Item == "" {
		return
	}
	if _, exists := r.items[ev.Item]; exists {
		r.collide(ev.Item, i)
		return
	}
	if _, taken := r.comments[ev.Item]; taken {
		r.collide(ev.Item, i)
	}
	r.rank[ev.Item] = i
	r.items[ev.Item] = &Item{
		ID:       ev.Item,
		Kind:     ev.Kind,
		State:    ItemPending,
		Ordinal:  ev.Ordinal,
		Column:   ev.Column,
		Owner:    ev.Owner,
		Standing: ev.Standing,
		Evidence: ev.Evidence,
		Text:     strings.TrimRight(ev.Text, "\n"),
		body:     ev.Text,
		TS:       ev.TS,
	}
	r.count(MemberCollection{Kind: KindItem}, ev.Ordinal)
}

// updateItem applies an item_updated line: the text from the line's text,
// every other field from its to.
func (r *memberReplay) updateItem(i int, ev Event) {
	item, ok := r.items[ev.Note]
	if !ok {
		r.unknown = append(r.unknown, i+1)
		return
	}
	switch ev.Field {
	case "text":
		item.Text = strings.TrimRight(ev.Text, "\n")
		item.body = ev.Text
	case ItemColumnField:
		item.Column = ev.To
	case ItemOwnerField:
		item.Owner = ev.To
	case ItemEvidenceField:
		item.Evidence = ev.To
	case ItemResolutionField:
		item.Resolution = ev.To
	}
}

// settle applies one of the five settling lines: the state the verb lands
// at, and the answer of record the line names.
func (r *memberReplay) settle(i int, ev Event) {
	item, ok := r.items[ev.Item]
	if !ok {
		r.unknown = append(r.unknown, i+1)
		return
	}
	switch ev.Event {
	case contract.EventItemResolved:
		item.State = ItemResolved
	case contract.EventItemVerified:
		item.State = ItemVerified
	case contract.EventItemFailed:
		item.State = ItemFailed
	default:
		item.State = ev.To
	}
	item.Resolution = ev.Resolution
}

// shelve flips a member's archived mark, or, for a line naming no member,
// records a column archived or restored on the workbench journal.
func (r *memberReplay) shelve(i int, ev Event, archived bool) {
	switch {
	case ev.Comment != "":
		comment, ok := r.comments[ev.Comment]
		if !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		comment.Archived = archived
	case ev.Item != "":
		item, ok := r.items[ev.Item]
		if !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		item.Archived = archived
	case ev.Note != "":
		r.archivedColumns[ev.Note] = archived
	}
}

// remove takes a deleted member out of every read, an item's comments with
// it, or, for a line naming no member, records a column deleted.
func (r *memberReplay) remove(i int, ev Event) {
	switch {
	case ev.Comment != "":
		if _, ok := r.comments[ev.Comment]; !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		delete(r.comments, ev.Comment)
		r.deleted[ev.Comment] = true
	case ev.Item != "":
		if _, ok := r.items[ev.Item]; !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		delete(r.items, ev.Item)
		r.deleted[ev.Item] = true
		for id, comment := range r.comments {
			if comment.Holder == ev.Item {
				delete(r.comments, id)
				r.deleted[id] = true
			}
		}
	case ev.Note != "":
		r.deletedColumns[ev.Note] = true
	}
}

// redact records that a member's text was replaced by its digest: the text
// reads empty, and the member says who redacted it and when. A member deleted
// before its redaction is out of every read already, so its line changes
// nothing.
func (r *memberReplay) redact(i int, ev Event) {
	switch ev.Kind {
	case KindComment:
		comment, ok := r.comments[ev.Comment]
		if !ok && r.deleted[ev.Comment] {
			return
		}
		if !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		comment.Body = ""
		comment.RecordedDigest = CommentDigest("")
		comment.Redacted = &Redaction{By: ev.Actor.Name, At: ev.TS}
	case KindItem:
		item, ok := r.items[ev.Item]
		if !ok && r.deleted[ev.Item] {
			return
		}
		if !ok {
			r.unknown = append(r.unknown, i+1)
			return
		}
		item.Text, item.body = "", ""
		item.Redacted = &Redaction{By: ev.Actor.Name, At: ev.TS}
	}
}

// citationOf reads one citation as a baseline line carries it.
func citationOf(record CitationRecord) Citation {
	before, after, _ := ParseObserved(record.Observed)
	return Citation{Scheme: record.Scheme, Target: record.Target, Before: before, After: after}
}

// observedOf spells a citation's observation the way a line carries it.
func observedOf(citation Citation) string {
	if citation.Before == "" && citation.After == "" {
		return ""
	}
	return citation.Before + ":" + citation.After
}

// Redaction is who replaced a member's text with its digest, and when.
type Redaction struct {
	// By is the operator who ran dinah redact.
	By string
	// At is the redacted line's timestamp.
	At string
}
