package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// storageSource is the store as the old layout states it: every card and
// every column carrying comments, each member read once, and the manifest
// the proof compares the journals against.
type storageSource struct {
	b        *Bench
	cards    []*storageCard
	columns  []*storageColumn
	manifest *manifest
	// problems are the precondition failures found while reading, each an
	// offending file and the rule it breaks.
	problems []storageProblem
	// anchors counts every comment.md and item.md the old layout holds,
	// which is what the run deletes.
	anchors int
}

// storageProblem is one file a precondition refuses the run over.
type storageProblem struct {
	rule string
	path string
}

// storageCard is one card as the old layout states it.
type storageCard struct {
	card     *Card
	archived bool
	record   *CardRecord
	// done says the card's journal already carries card_baseline, which a
	// resumed run's own earlier pass wrote, so the files it read may be
	// gone and the preconditions about them are passed over.
	done bool
	// items are the card's items, live then archived, each half in
	// ordinal order, and comments the card's own comments then each item's
	// in item order, which is the order phase 1 baselines them in.
	items    []*Item
	comments []*Comment
}

// storageColumn is one column carrying comments on the old layout.
type storageColumn struct {
	id       string
	title    string
	archived bool
	dir      string
	comments []*Comment
}

// readOldLayout reads the whole store through the old layout's reader,
// checking the preconditions about its files as it goes, and computes the
// manifest the old layout states.
func (b *Bench) readOldLayout(state *storageState) (*storageSource, error) {
	source := &storageSource{b: b}
	for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
		root := b.cardsRootIn(half)
		ids, err := b.ListIDs(root)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			card, err := b.LoadCardIn(root, id)
			if err != nil {
				continue
			}
			held, err := b.readOldCard(source, card, half == ArchivedHalf)
			if err != nil {
				return nil, err
			}
			source.cards = append(source.cards, held)
		}
	}
	if err := b.readOldColumns(source); err != nil {
		return nil, err
	}
	built, err := source.oldManifest()
	if err != nil {
		return nil, err
	}
	source.manifest = built
	return source, nil
}

// readOldCard reads one card's members through the old layout's reader and
// checks the files it holds.
func (b *Bench) readOldCard(source *storageSource, card *Card, archived bool) (*storageCard, error) {
	events, _, err := b.ReadJournal(card.JournalPath())
	if err != nil {
		return nil, err
	}
	held := &storageCard{card: card, archived: archived}
	for _, ev := range events {
		if ev.Event == contract.EventCardBaseline {
			held.done = true
		}
	}
	record, err := b.recordFromDirectories(card)
	if err != nil {
		return nil, err
	}
	record.holdEverything()
	held.record = record
	for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
		held.items = append(held.items, record.ItemsIn(half, "")...)
	}
	holders := []string{""}
	for _, item := range held.items {
		holders = append(holders, item.ID)
	}
	for _, holder := range holders {
		for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
			held.comments = append(held.comments, record.HeldComments(holder, half)...)
		}
	}
	anchors := legacyAnchorsBelow(b.source(), card.Dir)
	source.anchors += len(anchors)
	if !held.done {
		for _, anchor := range anchors {
			source.checkAnchor(anchor)
		}
		source.checkOrdinals(record, held)
		source.checkMoves(card.Dir)
	}
	return held, nil
}

// readOldColumns reads every live and archived column's comments through the
// old layout's reader.
func (b *Bench) readOldColumns(source *storageSource) error {
	seen := map[string]bool{}
	var columns []*storageColumn
	for _, column := range b.Columns {
		columns = append(columns, &storageColumn{id: column.ID, title: column.Title, dir: b.ColumnDir(column.ID)})
		seen[column.ID] = true
	}
	archivedIDs, err := b.ListIDs(b.ArchivedColumnsRoot())
	if err != nil {
		return err
	}
	for _, id := range archivedIDs {
		if seen[id] {
			continue
		}
		columns = append(columns, &storageColumn{id: id, title: b.columnTitleAnyHalf(id), archived: true, dir: filepath.Join(b.ArchivedColumnsRoot(), id)})
	}
	for _, column := range columns {
		for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
			comments, err := b.columnCommentsFromDirectories(column.id, half)
			if err != nil {
				return err
			}
			for _, comment := range comments {
				// A comment of an archived column reads in the archived half
				// whatever directory it stands in; its own flag says whether
				// it was archived in its own right, which is what the
				// baseline carries.
				comment.Archived = strings.Contains(filepath.ToSlash(comment.Home), "/"+ArchiveDir+"/"+CommentsDir+"/")
			}
			column.comments = append(column.comments, comments...)
		}
		anchors := legacyAnchorsBelowColumn(b.source(), column.dir)
		source.anchors += len(anchors)
		for _, anchor := range anchors {
			source.checkAnchor(anchor)
		}
		source.checkColumnOrdinals(column)
		source.checkColumnMoves(column.dir)
		if len(column.comments) > 0 {
			source.columns = append(source.columns, column)
		}
	}
	return nil
}

// legacyHolders are the directories below a card that hold members on the
// old layout: the card itself and every item directory, in both halves.
func legacyItemDirs(src Source, cardDir string) []string {
	var dirs []string
	for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
		collection := legacyCollection(cardDir, ChecklistDir, half)
		ids, err := listIDs(src, collection)
		if err != nil {
			continue
		}
		for _, id := range ids {
			dirs = append(dirs, filepath.Join(collection, id))
		}
	}
	return dirs
}

// legacyCommentDirs are the comment directories below one holder on the old
// layout, in both halves.
func legacyCommentDirs(src Source, holderDir string) []string {
	var dirs []string
	for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
		collection := legacyCollection(holderDir, CommentsDir, half)
		ids, err := listIDs(src, collection)
		if err != nil {
			continue
		}
		for _, id := range ids {
			dirs = append(dirs, filepath.Join(collection, id))
		}
	}
	return dirs
}

// legacyAnchorsBelow are every comment.md and item.md file standing below a
// card on the old layout.
func legacyAnchorsBelow(src Source, cardDir string) []string {
	var anchors []string
	commentAnchor, itemAnchor := legacyAnchorOf(KindComment), legacyAnchorOf(KindItem)
	holders := []string{cardDir}
	for _, item := range legacyItemDirs(src, cardDir) {
		if exists(src, filepath.Join(item, itemAnchor)) {
			anchors = append(anchors, filepath.Join(item, itemAnchor))
		}
		holders = append(holders, item)
	}
	for _, holder := range holders {
		for _, dir := range legacyCommentDirs(src, holder) {
			if exists(src, filepath.Join(dir, commentAnchor)) {
				anchors = append(anchors, filepath.Join(dir, commentAnchor))
			}
		}
	}
	return anchors
}

// legacyAnchorsBelowColumn are every comment.md file standing below a column
// on the old layout.
func legacyAnchorsBelowColumn(src Source, columnDir string) []string {
	var anchors []string
	commentAnchor := legacyAnchorOf(KindComment)
	for _, dir := range legacyCommentDirs(src, columnDir) {
		if exists(src, filepath.Join(dir, commentAnchor)) {
			anchors = append(anchors, filepath.Join(dir, commentAnchor))
		}
	}
	return anchors
}

// checkAnchor applies the fourth precondition to one member anchor: it
// parses, carries no key outside the set the build writes for its kind, and
// carries no citation member a baseline could not carry.
func (s *storageSource) checkAnchor(path string) {
	text, err := s.b.ReadText(path)
	if err != nil || !anchorParses(text) {
		s.problems = append(s.problems, storageProblem{rule: PreconditionUnparseable, path: path})
		return
	}
	fm, _ := ParseAnchor(text)
	allowed := commentKeys
	if filepath.Base(path) == legacyAnchorOf(KindItem) {
		allowed = itemKeys
	}
	for _, key := range fm.Keys() {
		if !allowed[key] {
			s.problems = append(s.problems, storageProblem{rule: PreconditionUnknownKey, path: path})
			return
		}
	}
	if !fm.Has(CitationsField) {
		return
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(blockValue(fm, CitationsField), &entries); err != nil {
		s.problems = append(s.problems, storageProblem{rule: PreconditionUnparseable, path: path})
		return
	}
	for _, entry := range entries {
		for member, value := range entry {
			if !citationKeys[member] {
				s.problems = append(s.problems, storageProblem{rule: PreconditionUnknownCitation, path: path})
				return
			}
			if member != ItemCitationObserved {
				continue
			}
			var observed map[string]json.RawMessage
			if json.Unmarshal(value, &observed) != nil {
				s.problems = append(s.problems, storageProblem{rule: PreconditionUnknownCitation, path: path})
				return
			}
			for key := range observed {
				if !observationKeys[key] {
					s.problems = append(s.problems, storageProblem{rule: PreconditionUnknownCitation, path: path})
					return
				}
			}
		}
	}
}

// anchorParses reports whether an anchor's text opens with a frontmatter
// block that closes, which is what the old layout's readers take for an
// anchor at all.
func anchorParses(text string) bool {
	lines := SplitLines(text)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			return true
		}
	}
	return false
}

// checkOrdinals applies the fifth and sixth preconditions to one card: every
// member carries an ordinal, no two members of one collection instance
// carry the same one in either half, and no two members of the card carry
// one identifier.
func (s *storageSource) checkOrdinals(record *CardRecord, card *storageCard) {
	seen := map[MemberCollection]map[int]bool{}
	claim := func(collection MemberCollection, ordinal int, path string) {
		if ordinal <= 0 {
			s.problems = append(s.problems, storageProblem{rule: PreconditionOrdinalMissing, path: path})
			return
		}
		if seen[collection] == nil {
			seen[collection] = map[int]bool{}
		}
		if seen[collection][ordinal] {
			s.problems = append(s.problems, storageProblem{rule: PreconditionOrdinalDuplicate, path: path})
			return
		}
		seen[collection][ordinal] = true
	}
	identifiers := map[string]string{}
	name := func(id, path string) {
		if first, taken := identifiers[id]; taken {
			s.problems = append(s.problems, storageProblem{rule: PreconditionIdentifierShared, path: first})
			s.problems = append(s.problems, storageProblem{rule: PreconditionIdentifierShared, path: path})
			return
		}
		identifiers[id] = path
	}
	for _, item := range card.items {
		path := record.FileOf(KindItem, item.ID)
		claim(MemberCollection{Kind: KindItem}, item.Ordinal, path)
		name(item.ID, path)
	}
	for _, comment := range card.comments {
		path := record.FileOf(KindComment, comment.ID)
		claim(MemberCollection{Kind: KindComment, Holder: comment.Holder}, comment.Ordinal, path)
		name(comment.ID, path)
	}
}

// checkColumnOrdinals applies the fifth and sixth preconditions to one
// column's comments.
func (s *storageSource) checkColumnOrdinals(column *storageColumn) {
	seen := map[int]bool{}
	identifiers := map[string]bool{}
	for _, comment := range column.comments {
		path := filepath.Join(comment.Home, legacyAnchorOf(KindComment))
		switch {
		case comment.Ordinal <= 0:
			s.problems = append(s.problems, storageProblem{rule: PreconditionOrdinalMissing, path: path})
		case seen[comment.Ordinal]:
			s.problems = append(s.problems, storageProblem{rule: PreconditionOrdinalDuplicate, path: path})
		}
		seen[comment.Ordinal] = true
		if identifiers[comment.ID] {
			s.problems = append(s.problems, storageProblem{rule: PreconditionIdentifierShared, path: path})
		}
		identifiers[comment.ID] = true
	}
}

// attachmentMove is one attachment directory phase 3 moves to the home the
// card-unit layout gives a comment's attachments.
type attachmentMove struct {
	from, to string
}

// legacyMoves are the attachment directories below a card, or below a
// column, that do not stand where the card-unit layout keeps a comment's
// attachments: comments/<comment>/attachments/ below the holding card or
// column, with the archive mirror of that collection beside it. They are
// read off the directories rather than off the comments, so a rerun after a
// partial removal still finds what is left to move.
func legacyMoves(src Source, holderDir string, cardDir bool) []attachmentMove {
	var moves []attachmentMove
	var commentDirs []string
	for _, dir := range legacyCommentDirs(src, holderDir) {
		if filepath.Base(filepath.Dir(filepath.Dir(dir))) == ArchiveDir {
			commentDirs = append(commentDirs, dir)
		}
	}
	if cardDir {
		for _, item := range legacyItemDirs(src, holderDir) {
			commentDirs = append(commentDirs, legacyCommentDirs(src, item)...)
		}
	}
	for _, dir := range commentDirs {
		home := filepath.Join(holderDir, CommentsDir, filepath.Base(dir))
		for _, collection := range []string{AttachmentsDir, filepath.Join(ArchiveDir, AttachmentsDir)} {
			ids, err := listIDs(src, filepath.Join(dir, collection))
			if err != nil {
				continue
			}
			for _, id := range ids {
				moves = append(moves, attachmentMove{
					from: filepath.Join(dir, collection, id),
					to:   filepath.Join(home, collection, id),
				})
			}
		}
	}
	return moves
}

// checkMoves applies the seventh precondition to a card: no attachment would
// move onto a destination that already holds a different tree.
func (s *storageSource) checkMoves(cardDir string) {
	for _, move := range legacyMoves(s.b.source(), cardDir, true) {
		if exists(s.b.source(), move.to) && !sameTree(s.b.source(), move.from, move.to) {
			s.problems = append(s.problems, storageProblem{rule: PreconditionDestination, path: move.to})
		}
	}
}

// checkColumnMoves applies the seventh precondition to a column.
func (s *storageSource) checkColumnMoves(columnDir string) {
	for _, move := range legacyMoves(s.b.source(), columnDir, false) {
		if exists(s.b.source(), move.to) && !sameTree(s.b.source(), move.from, move.to) {
			s.problems = append(s.problems, storageProblem{rule: PreconditionDestination, path: move.to})
		}
	}
}

// sameTree reports whether two directories hold the same files by relative
// path and SHA-256.
func sameTree(src Source, a, b string) bool {
	left, err := treeDigest(src, a)
	if err != nil {
		return false
	}
	right, err := treeDigest(src, b)
	return err == nil && left == right
}

// treeDigest is a SHA-256 over every relative path and byte below a
// directory.
func treeDigest(src Source, root string) (string, error) {
	return storeDigest(src, root, "")
}

// preconditions refuses the run over every file a precondition names, or
// answers nothing where none does.
func (s *storageSource) preconditions() error {
	if len(s.problems) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var rows []string
	for _, problem := range s.problems {
		row := problem.rule + " " + problem.path
		if seen[row] {
			continue
		}
		seen[row] = true
		rows = append(rows, row)
	}
	first := s.problems[0]
	return contract.RefuseWith(contract.StoragePrecondition, first.path, map[string]string{
		"rule":  first.rule,
		"files": strings.Join(rows, "\n"),
	})
}

// AwaitingStorageFindings are what dinah check reports on a store below the
// card-unit format once the layout is switched on: that the store awaits the
// storage migration, and every file a precondition of that migration would
// refuse it over. No other check runs, since each reads the card-unit layout.
func (b *Bench) AwaitingStorageFindings() ([]Finding, error) {
	findings := []Finding{{Path: filepath.Join(b.Root, WorkbenchAnchor), Key: FindingStoreAwaitingMigration, Detail: b.Root}}
	source, err := b.readOldLayout(nil)
	if err != nil {
		return findings, err
	}
	for _, problem := range source.problems {
		findings = append(findings, Finding{Path: problem.path, Key: FindingStoragePrecondition, Detail: problem.rule})
	}
	return findings, nil
}

// count fills a report's counts from what the old layout holds.
func (s *storageSource) count(report *StorageMigration) {
	for _, card := range s.cards {
		if card.archived {
			report.Cards.Archived++
			report.ItemsArchived += len(card.items)
		} else {
			report.Cards.Live++
			report.ItemsLive += len(card.items)
		}
		for _, comment := range card.comments {
			if comment.Holder == "" {
				report.Comments.Card++
			} else {
				report.Comments.Item++
			}
			if comment.Diverged() {
				report.Divergences = append(report.Divergences, s.commentRef(card, comment))
			}
		}
	}
	report.Items = report.ItemsLive + report.ItemsArchived
	for _, column := range s.columns {
		report.Comments.Column += len(column.comments)
	}
	report.Attachments.Checked = s.manifest.attachments
	report.Manifest.Before = s.manifest.hash()
	report.Manifest.Lines = len(s.manifest.lines)
}

// commentRef composes the reference a report names a comment by: its
// positions where the comment and its item stand live, and identifiers
// otherwise, which the resolver accepts in the same place.
func (s *storageSource) commentRef(card *storageCard, comment *Comment) string {
	ref := card.card.Ref(s.b.Slug)
	if comment.Holder != "" {
		segment := comment.Holder
		if position := card.record.Position(MemberCollection{Kind: KindItem}, LiveHalf, comment.Holder); position > 0 {
			segment = strconv.Itoa(position)
		}
		ref += "/" + ChecklistSegment + "/" + segment
	}
	segment := comment.ID
	if position := card.record.HeldPosition(comment.Holder, LiveHalf, comment.ID); position > 0 {
		segment = strconv.Itoa(position)
	}
	return ref + "/" + CommentsDir + "/" + segment
}

// manifest is one side of the proof: a line per member and per attachment,
// keyed by the line's first fields.
type manifest struct {
	src         Source
	lines       map[string]string
	attachments int
}

// newManifest makes an empty manifest.
func newManifest(src Source) *manifest {
	return &manifest{src: src, lines: map[string]string{}}
}

// add records one line under its key.
func (m *manifest) add(line string) {
	m.lines[manifestKey(line)] = line
	if strings.HasPrefix(line, "attach\t") {
		m.attachments++
	}
}

// sorted answers the lines sorted bytewise.
func (m *manifest) sorted() []string {
	lines := make([]string, 0, len(m.lines))
	for _, line := range m.lines {
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines
}

// hash is the SHA-256 of the sorted lines joined by a newline.
func (m *manifest) hash() string {
	return manifestHash(m.sorted())
}

// stored is the manifest as storage-migration.json records it.
func (m *manifest) stored() *storedManifest {
	lines := m.sorted()
	return &storedManifest{Lines: lines, Count: len(lines), Hash: manifestHash(lines)}
}

// manifestHash is the SHA-256 of sorted lines joined by a newline.
func manifestHash(lines []string) string {
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// manifestKey is a line's key: its first three fields joined by a slash, and
// for an attachment line its first four.
func manifestKey(line string) string {
	fields := strings.Split(line, "\t")
	count := 3
	if len(fields) > 0 && fields[0] == "attach" {
		count = 4
	}
	if len(fields) < count {
		count = len(fields)
	}
	return strings.Join(fields[:count], "/")
}

// manifestField spells one field of a line, with a dash for an empty value.
func manifestField(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// textDigest is a text field as the manifest carries it.
func textDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// flag spells a boolean as the manifest carries it.
func flag(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

// itemLine is an item's manifest line.
func itemLine(cardID string, item *Item, archived bool) string {
	var citations strings.Builder
	for _, cited := range item.Citations {
		citations.WriteString(cited.Scheme + "\t" + cited.Target + "\t" + cited.Before + "\t" + cited.After + "\n")
	}
	return strings.Join([]string{
		"item", cardID, item.ID, strconv.Itoa(item.Ordinal), flag(archived), manifestField(item.TS),
		manifestField(item.Kind), manifestField(item.State), manifestField(item.Column), manifestField(item.Owner),
		manifestField(item.Evidence), manifestField(item.Standing), manifestField(item.Resolution),
		textDigest(citations.String()), textDigest(item.body),
	}, "\t")
}

// commentLine is a comment's manifest line.
func commentLine(owner string, comment *Comment, archived bool) string {
	author := comment.Author
	if comment.AuthorUnrecoverable {
		author = "!unrecoverable"
	}
	return strings.Join([]string{
		"comment", owner, comment.ID, manifestField(comment.Holder), strconv.Itoa(comment.Ordinal), flag(archived),
		manifestField(comment.TS), manifestField(author), manifestField(comment.RecordedDigest), textDigest(comment.Body),
	}, "\t")
}

// attachLines adds the manifest lines of every attachment below one holder
// directory, in both halves, keyed to the card or column that owns them and
// the comment they hang on or none. Where an attachment stands at two of the
// directories given, the first answers.
func (m *manifest) attachLines(owner, holder string, dirs ...string) error {
	seen := map[string]bool{}
	for _, dir := range dirs {
		for _, collection := range []string{AttachmentsDir, filepath.Join(ArchiveDir, AttachmentsDir)} {
			ids, err := listIDs(m.src, filepath.Join(dir, collection))
			if err != nil {
				return err
			}
			for _, id := range ids {
				if seen[id] {
					continue
				}
				seen[id] = true
				member := filepath.Join(dir, collection, id)
				text, err := readText(m.src, filepath.Join(member, AttachmentAnchor))
				if err != nil {
					continue
				}
				attachment := attachmentFromText(member, id, text)
				payload := ""
				if attachment.Path != "" {
					data, err := m.src.ReadFile(attachment.Path)
					if err != nil {
						return err
					}
					payload = textDigest(string(data))
				}
				m.add(strings.Join([]string{
					"attach", owner, manifestField(holder), id, strconv.Itoa(attachment.Ordinal),
					manifestField(attachment.Filename), textDigest(attachment.Description),
					manifestField(attachment.Provenance), manifestField(payload),
				}, "\t"))
			}
		}
	}
	return nil
}

// oldManifest is the manifest the old layout states.
func (s *storageSource) oldManifest() (*manifest, error) {
	built := newManifest(s.b.source())
	for _, card := range s.cards {
		for _, item := range card.items {
			built.add(itemLine(card.card.ID, item, item.Archived))
		}
		for _, comment := range card.comments {
			built.add(commentLine(card.card.ID, comment, commentArchived(card.record, comment)))
			if err := built.attachLines(card.card.ID, comment.ID, comment.Home); err != nil {
				return nil, err
			}
		}
		if err := built.attachLines(card.card.ID, "", card.card.Dir); err != nil {
			return nil, err
		}
	}
	for _, column := range s.allColumns() {
		for _, comment := range column.comments {
			built.add(commentLine(column.id, comment, comment.Archived || column.archived))
			if err := built.attachLines(column.id, comment.ID, comment.Home); err != nil {
				return nil, err
			}
		}
		if err := built.attachLines(column.id, "", column.dir); err != nil {
			return nil, err
		}
	}
	return built, nil
}

// allColumns are every live and archived column, those carrying no comment
// included, which the attachment lines reach too.
func (s *storageSource) allColumns() []*storageColumn {
	carrying := map[string]*storageColumn{}
	for _, column := range s.columns {
		carrying[column.id] = column
	}
	var columns []*storageColumn
	for _, column := range s.b.Columns {
		if held, ok := carrying[column.ID]; ok {
			columns = append(columns, held)
			continue
		}
		columns = append(columns, &storageColumn{id: column.ID, title: column.Title, dir: s.b.ColumnDir(column.ID)})
	}
	ids, _ := s.b.ListIDs(s.b.ArchivedColumnsRoot())
	for _, id := range ids {
		if s.b.Column(id) != nil {
			continue
		}
		if held, ok := carrying[id]; ok {
			columns = append(columns, held)
			continue
		}
		columns = append(columns, &storageColumn{id: id, archived: true, dir: filepath.Join(s.b.ArchivedColumnsRoot(), id)})
	}
	return columns
}

// newManifestFrom is the manifest the journals state: every card journal's
// members replayed by the card-unit rule, the workbench journal's column
// comments, and every attachment wherever its payload now stands, at the
// home the card-unit layout gives a comment's attachments or where the old
// layout kept it.
func (s *storageSource) newManifestFrom(cardEvents func(*storageCard) ([]Event, error), benchEvents []Event) (*manifest, error) {
	built := newManifest(s.b.source())
	for _, card := range s.cards {
		events, err := cardEvents(card)
		if err != nil {
			return nil, err
		}
		replay := replayMembers(events)
		for _, item := range replay.items {
			built.add(itemLine(card.card.ID, item, item.Archived))
		}
		for _, comment := range replay.comments {
			archived := comment.Archived
			if item, ok := replay.items[comment.Holder]; ok && item.Archived {
				archived = true
			}
			built.add(commentLine(card.card.ID, comment, archived))
			dirs := []string{filepath.Join(card.card.Dir, CommentsDir, comment.ID)}
			if old, ok := card.record.Comments[comment.ID]; ok {
				dirs = append(dirs, old.Home)
			}
			if err := built.attachLines(card.card.ID, comment.ID, dirs...); err != nil {
				return nil, err
			}
		}
		if err := built.attachLines(card.card.ID, "", card.card.Dir); err != nil {
			return nil, err
		}
	}
	replay := replayMembers(benchEvents)
	homes := map[string]string{}
	for _, column := range s.columns {
		for _, comment := range column.comments {
			homes[comment.ID] = comment.Home
		}
	}
	for _, column := range s.allColumns() {
		for _, comment := range replay.comments {
			if comment.Holder != column.id {
				continue
			}
			built.add(commentLine(column.id, comment, comment.Archived || column.archived))
			dirs := []string{filepath.Join(column.dir, CommentsDir, comment.ID)}
			if old, ok := homes[comment.ID]; ok {
				dirs = append(dirs, old)
			}
			if err := built.attachLines(column.id, comment.ID, dirs...); err != nil {
				return nil, err
			}
		}
		if err := built.attachLines(column.id, "", column.dir); err != nil {
			return nil, err
		}
	}
	return built, nil
}

// differences are the keys whose lines two manifests do not agree on, with
// both lines, sorted by key.
func differences(before map[string]string, after *manifest) []ManifestDifference {
	var found []ManifestDifference
	for key, line := range before {
		if after.lines[key] != line {
			found = append(found, ManifestDifference{Key: key, Before: line, After: after.lines[key]})
		}
	}
	for key, line := range after.lines {
		if _, ok := before[key]; !ok {
			found = append(found, ManifestDifference{Key: key, After: line})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Key < found[j].Key })
	return found
}

// storedLines answers a stored manifest's lines by key.
func storedLines(stored *storedManifest) map[string]string {
	lines := make(map[string]string, len(stored.Lines))
	for _, line := range stored.Lines {
		lines[manifestKey(line)] = line
	}
	return lines
}

// baselineOf is the item_baseline line that states one item in full.
func (b *Bench) itemBaseline(template Event, item *Item) Event {
	ev := template
	ev.Event = contract.EventItemBaseline
	ev.Item = item.ID
	ev.Kind = item.Kind
	ev.Ordinal = item.Ordinal
	ev.State = item.State
	ev.Text = item.body
	ev.Written = item.TS
	ev.Column = item.Column
	if item.Column != "" {
		ev.ColumnTitle = b.columnTitleAnyHalf(item.Column)
	}
	ev.Owner = item.Owner
	ev.Evidence = item.Evidence
	ev.Standing = item.Standing
	ev.Resolution = item.Resolution
	for _, cited := range item.Citations {
		ev.Citations = append(ev.Citations, CitationRecord{Scheme: cited.Scheme, Target: cited.Target, Observed: observedOf(cited)})
	}
	ev.Archived = item.Archived
	return ev
}

// commentBaseline is the comment_baseline line that states one comment in
// full: on a card journal for a card's or an item's comment, and on the
// workbench journal, naming its column, for a column's.
func (b *Bench) commentBaseline(template Event, comment *Comment, column string) Event {
	ev := template
	ev.Event = contract.EventCommentBaseline
	ev.Comment = comment.ID
	ev.Ordinal = comment.Ordinal
	ev.Text = comment.Body
	ev.Written = comment.TS
	ev.Author = comment.Author
	ev.AuthorUnrecoverable = comment.AuthorUnrecoverable
	ev.Digest = comment.RecordedDigest
	ev.Archived = comment.Archived
	if column != "" {
		ev.Column = column
		ev.ColumnTitle = b.columnTitleAnyHalf(column)
		return ev
	}
	ev.Item = comment.Holder
	return ev
}

// columnTitleAnyHalf is a column's title, live or archived.
func (b *Bench) columnTitleAnyHalf(id string) string {
	if column := b.Column(id); column != nil {
		return column.Title
	}
	fm, _ := loadAnchor(b.source(), filepath.Join(b.ArchivedColumnsRoot(), id, ColumnAnchor))
	return fm.Value(TitleField)
}

// cardBaseline is the card_baseline line carrying a card's card.md.
func cardBaseline(src Source, template Event, card *Card) (Event, error) {
	ev := template
	text, err := readText(src, card.AnchorPath())
	if err != nil {
		return ev, err
	}
	ev.Event = contract.EventCardBaseline
	ev.Text = text
	return ev, nil
}

// baselinesOwed answers the member baselines one card owes, in the order
// phase 1 writes them: every member whose manifest line has no baseline on
// the journal yet, or whose latest baseline no longer states the line the
// stored manifest carries for it.
func (b *Bench) baselinesOwed(template Event, card *storageCard, events []Event, stored map[string]string) []Event {
	var baselines []Event
	for _, ev := range events {
		if ev.Event == contract.EventItemBaseline || ev.Event == contract.EventCommentBaseline {
			baselines = append(baselines, ev)
		}
	}
	stated := replayMembers(baselines)
	var owed []Event
	for _, item := range card.items {
		want := stored[manifestKey(itemLine(card.card.ID, item, item.Archived))]
		if have, ok := stated.items[item.ID]; ok && itemLine(card.card.ID, have, have.Archived) == want {
			continue
		}
		owed = append(owed, b.itemBaseline(template, item))
	}
	for _, comment := range card.comments {
		archived := commentArchived(card.record, comment)
		want := stored[manifestKey(commentLine(card.card.ID, comment, archived))]
		if have, ok := stated.comments[comment.ID]; ok {
			haveArchived := have.Archived
			if item, ok := card.record.Items[have.Holder]; ok && item.Archived {
				haveArchived = true
			}
			if commentLine(card.card.ID, have, haveArchived) == want {
				continue
			}
		}
		owed = append(owed, b.commentBaseline(template, comment, ""))
	}
	return owed
}

// columnBaselinesOwed answers the comment baselines the workbench journal
// owes for every column's comments, on baselinesOwed's terms.
func (s *storageSource) columnBaselinesOwed(template Event, events []Event, stored map[string]string) []Event {
	var baselines []Event
	for _, ev := range events {
		if ev.Event == contract.EventCommentBaseline {
			baselines = append(baselines, ev)
		}
	}
	stated := replayMembers(baselines)
	var owed []Event
	for _, column := range s.columns {
		for _, comment := range column.comments {
			want := stored[manifestKey(commentLine(column.id, comment, comment.Archived || column.archived))]
			if have, ok := stated.comments[comment.ID]; ok && commentLine(column.id, have, have.Archived || column.archived) == want {
				continue
			}
			owed = append(owed, s.b.commentBaseline(template, comment, column.id))
		}
	}
	return owed
}

// rehearseStorage decides everything a run would and writes nothing: it
// composes each journal's baselines in memory, replays them, and compares
// the manifest they state with the old layout's.
func (b *Bench) rehearseStorage(run StorageMigrationRun, source *storageSource, report *StorageMigration) (*StorageMigration, error) {
	report.From = b.Format
	files, err := countFiles(b.source(), b.Root)
	if err != nil {
		return nil, err
	}
	report.Files.Before = files
	report.Files.After = files - source.anchors
	stored := source.manifest.lines
	composed := func(card *storageCard) ([]Event, error) {
		events, _, err := b.ReadJournal(card.card.JournalPath())
		if err != nil {
			return nil, err
		}
		events = append(events, b.baselinesOwed(run.Template, card, events, stored)...)
		if !card.done {
			baseline, err := cardBaseline(b.source(), run.Template, card.card)
			if err != nil {
				return nil, err
			}
			events = append(events, baseline)
		}
		return events, nil
	}
	benchEvents, _, err := b.ReadJournal(b.JournalPath())
	if err != nil {
		return nil, err
	}
	benchEvents = append(benchEvents, source.columnBaselinesOwed(run.Template, benchEvents, stored)...)
	after, err := source.newManifestFrom(composed, benchEvents)
	if err != nil {
		return nil, err
	}
	report.Manifest.After = after.hash()
	report.Outcome = contract.ReadOK
	if found := differences(stored, after); len(found) > 0 {
		report.Outcome = contract.ReadFindings
		report.Phase = StoragePhaseProof
		report.Differences = found
	}
	return report, nil
}

// baselineAndProve runs phase 1 and phase 2, and phase 1 once more where the
// proof finds a member written during the run while the old layout is still
// whole. It answers true where the run stopped, with the report saying why.
func (b *Bench) baselineAndProve(run StorageMigrationRun, held *Lock, state *storageState, source *storageSource, report *StorageMigration) (bool, error) {
	recomputed := false
	for {
		report.Phase = StoragePhaseBaseline
		stored := storedLines(state.Manifest)
		if stopped, err := b.writeBaselines(run, held, source, stored, report); err != nil || stopped {
			return stopped, err
		}
		if err := passPoint("phase-1"); err != nil {
			return false, err
		}
		report.Phase = StoragePhaseProof
		after, err := source.newManifestFrom(func(card *storageCard) ([]Event, error) {
			events, _, err := b.ReadJournal(card.card.JournalPath())
			return events, err
		}, mustEvents(b.source(), b.JournalPath()))
		if err != nil {
			return false, err
		}
		found := differences(stored, after)
		if state.RemovalStarted {
			found, err = b.acceptDifferences(run, state, found, after)
			if err != nil {
				return false, err
			}
		}
		if len(found) == 0 {
			report.Manifest.Before = state.Manifest.Hash
			report.Manifest.After = after.hash()
			report.Manifest.Lines = state.Manifest.Count
			if err := passPoint("phase-2"); err != nil {
				return false, err
			}
			return false, nil
		}
		if state.RemovalStarted || recomputed {
			report.Differences = found
			return true, nil
		}
		// The old layout is still whole, so it is still the authority: a
		// line it now states differently is a member some process wrote
		// during the run, and phase 1 baselines it again.
		recomputed = true
		fresh, err := b.readOldLayout(state)
		if err != nil {
			return false, err
		}
		written := differences(storedLines(state.Manifest), fresh.manifest)
		for _, difference := range written {
			state.WrittenDuringRun = append(state.WrittenDuringRun, difference.Key)
		}
		state.Manifest = fresh.manifest.stored()
		if err := b.writeStorageState(state); err != nil {
			return false, err
		}
		*source = *fresh
	}
}

// mustEvents reads a journal, answering no lines where it will not read, so
// the proof reports what it could not find rather than failing on it.
func mustEvents(src Source, path string) []Event {
	events, _, err := readJournal(src, path)
	if err != nil {
		return nil
	}
	return events
}

// acceptDifferences takes the after line as the record for every key the
// operator named, refusing a key that names no differing line, and answers
// the differences left.
func (b *Bench) acceptDifferences(run StorageMigrationRun, state *storageState, found []ManifestDifference, after *manifest) ([]ManifestDifference, error) {
	differing := map[string]bool{}
	for _, difference := range found {
		differing[difference.Key] = true
	}
	accepted := map[string]bool{}
	for _, key := range state.Accepted {
		accepted[key] = true
	}
	for _, key := range run.Accept {
		if accepted[key] {
			continue
		}
		if !differing[key] {
			return nil, contract.Refuse(contract.NotADifference, key)
		}
		accepted[key] = true
		state.Accepted = append(state.Accepted, key)
	}
	if len(run.Accept) > 0 {
		lines := storedLines(state.Manifest)
		for key := range accepted {
			if line, ok := after.lines[key]; ok {
				lines[key] = line
			} else {
				delete(lines, key)
			}
		}
		rebuilt := newManifest(b.source())
		for _, line := range lines {
			rebuilt.add(line)
		}
		state.Manifest = rebuilt.stored()
		if err := b.writeStorageState(state); err != nil {
			return nil, err
		}
	}
	var left []ManifestDifference
	for _, difference := range found {
		if !accepted[difference.Key] {
			left = append(left, difference)
		}
	}
	return left, nil
}

// writeBaselines runs phase 1: the column comments' baselines on the
// workbench journal, and then every card's member baselines and its card
// baseline under the card's own lock.
func (b *Bench) writeBaselines(run StorageMigrationRun, held *Lock, source *storageSource, stored map[string]string, report *StorageMigration) (bool, error) {
	benchEvents, _, err := b.ReadJournal(b.JournalPath())
	if err != nil {
		return false, err
	}
	if err := AppendEvents(held, b.JournalPath(), planted(source.columnBaselinesOwed(run.Template, benchEvents, stored))); err != nil {
		return false, err
	}
	for _, card := range source.cards {
		lock, err := b.takeLock(card.card.Dir, run.Template.Actor.Name, run.Template.TS)
		if err != nil {
			var refusal *contract.Refusal
			if errors.As(err, &refusal) && refusal.Name == contract.Locked {
				report.Held = &StorageHeldCard{Card: card.card.Ref(b.Slug), Holder: refusal.Detail}
				return true, nil
			}
			return false, err
		}
		stopped, err := b.baselineCard(run, lock, card, stored)
		lock.Release()
		if err != nil || stopped {
			return stopped, err
		}
	}
	return false, nil
}

// baselineCard writes one card's owed member baselines and, where the
// journal does not carry it yet, its card baseline.
func (b *Bench) baselineCard(run StorageMigrationRun, lock *Lock, card *storageCard, stored map[string]string) (bool, error) {
	events, _, err := b.ReadJournal(card.card.JournalPath())
	if err != nil {
		return false, err
	}
	if err := AppendEvents(lock, card.card.JournalPath(), planted(b.baselinesOwed(run.Template, card, events, stored))); err != nil {
		return false, err
	}
	if card.done {
		return false, nil
	}
	if err := passPoint("members:" + card.card.ID); err != nil {
		return false, err
	}
	baseline, err := cardBaseline(b.source(), run.Template, card.card)
	if err != nil {
		return false, err
	}
	if err := AppendEvent(lock, card.card.JournalPath(), baseline); err != nil {
		return false, err
	}
	card.done = true
	return false, nil
}

// removeOldLayout runs phase 3 on every card and column: it moves each
// attachment directory to the home the card-unit layout gives it and then
// deletes what the old layout kept its members in.
func (b *Bench) removeOldLayout(run StorageMigrationRun, state *storageState, source *storageSource, report *StorageMigration) (bool, error) {
	for _, card := range source.cards {
		lock, err := b.takeLock(card.card.Dir, run.Template.Actor.Name, run.Template.TS)
		if err != nil {
			var refusal *contract.Refusal
			if errors.As(err, &refusal) && refusal.Name == contract.Locked {
				report.Held = &StorageHeldCard{Card: card.card.Ref(b.Slug), Holder: refusal.Detail}
				return true, nil
			}
			return false, err
		}
		stopped, err := b.removeBelow(state, card.card.Dir, true, report)
		lock.Release()
		if err != nil || stopped {
			return stopped, err
		}
	}
	for _, column := range source.allColumns() {
		if stopped, err := b.removeBelow(state, column.dir, false, report); err != nil || stopped {
			return stopped, err
		}
	}
	return false, nil
}

// removeBelow moves and deletes below one card or column directory.
func (b *Bench) removeBelow(state *storageState, dir string, card bool, report *StorageMigration) (bool, error) {
	for _, move := range legacyMoves(b.source(), dir, card) {
		if exists(b.source(), move.to) {
			if !sameTree(b.source(), move.from, move.to) {
				return false, contract.RefuseWith(contract.StoragePrecondition, move.to, map[string]string{
					"rule": PreconditionDestination, "files": PreconditionDestination + " " + move.to,
				})
			}
			if stopped, err := storageStep(report, "remove", move.from, func() error { return durable.RemoveAll(move.from) }); err != nil || stopped {
				return stopped, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(move.to), 0o755); err != nil {
			return false, err
		}
		if stopped, err := storageStep(report, "rename", move.from, func() error { return durable.MoveDir(move.from, move.to) }); err != nil || stopped {
			return stopped, err
		}
		state.Moved++
		if err := b.writeStorageState(state); err != nil {
			return false, err
		}
	}
	// Only an anchor the proof covered goes. One standing for a member the
	// stored manifest does not carry was written since phase 1 by a process
	// the run could not keep out, and phase 4 carries it before deleting it.
	carried := storedLines(state.Manifest)
	var anchors []string
	if card {
		anchors = legacyAnchorsBelow(b.source(), dir)
	} else {
		anchors = legacyAnchorsBelowColumn(b.source(), dir)
	}
	owner := filepath.Base(dir)
	for _, anchor := range anchors {
		if _, ok := carried[anchorKey(owner, anchor)]; !ok {
			continue
		}
		if stopped, err := storageStep(report, "remove", anchor, func() error { return removeIfPresent(anchor) }); err != nil || stopped {
			return stopped, err
		}
	}
	return pruneOldLayout(b.source(), dir, report)
}

// anchorKey is the manifest key of the member an old layout's anchor states.
func anchorKey(owner, anchor string) string {
	kind := KindComment
	if filepath.Base(anchor) == legacyAnchorOf(KindItem) {
		kind = KindItem
	}
	return kind + "/" + owner + "/" + filepath.Base(filepath.Dir(anchor))
}

// pruneOldLayout removes every directory the old layout kept members in
// below a card or a column that nothing is left in: the checklist, the
// member archive mirror, and each comment's own directory except where it
// holds that comment's attachments.
func pruneOldLayout(src Source, dir string, report *StorageMigration) (bool, error) {
	roots := []string{
		filepath.Join(dir, ChecklistDir),
		filepath.Join(dir, ArchiveDir, ChecklistDir),
		filepath.Join(dir, ArchiveDir, CommentsDir),
		filepath.Join(dir, CommentsDir),
	}
	for _, root := range roots {
		if stopped, err := pruneEmpty(src, root, report); err != nil || stopped {
			return stopped, err
		}
	}
	return pruneEmpty(src, filepath.Join(dir, ArchiveDir), report)
}

// pruneEmpty removes a directory tree's empty directories from the leaves
// up, the root included where nothing is left in it.
func pruneEmpty(src Source, root string, report *StorageMigration) (bool, error) {
	info, err := src.Stat(root)
	if err != nil || !info.IsDir() {
		return false, nil
	}
	entries, err := src.ReadDir(root)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if stopped, err := pruneEmpty(src, filepath.Join(root, entry.Name()), report); err != nil || stopped {
			return stopped, err
		}
	}
	if left, err := src.ReadDir(root); err != nil || len(left) > 0 {
		return false, err
	}
	return storageStep(report, "remove", root, func() error { return durable.RemoveAll(root) })
}

// storageStep makes one rename or removal of phase 3, answering true with the
// report naming the path and the error where the filesystem refused it.
func storageStep(report *StorageMigration, op, path string, step func() error) (bool, error) {
	var err error
	if storageMigrationStep != nil {
		err = storageMigrationStep(op, path)
	}
	if err == nil {
		err = step()
	}
	if err == nil {
		return false, nil
	}
	report.Refused = &StorageRefusal{Path: path, Error: err.Error()}
	return true, nil
}

// carryStrays runs phase 4: every comment.md or item.md still standing is a
// write an older process made after its card's baselines, so it is appended
// as a baseline, which replaces the member it states, and deleted.
func (b *Bench) carryStrays(run StorageMigrationRun, held *Lock, state *storageState, report *StorageMigration) (bool, error) {
	strays, err := b.StrayMemberFiles()
	if err != nil {
		return false, err
	}
	for _, stray := range strays {
		stopped, err := b.carryStray(run, held, stray, state, report)
		if err != nil || stopped {
			return stopped, err
		}
	}
	return false, nil
}

// StrayMember is a comment.md or item.md standing in a store in the
// card-unit layout, and what holds it.
type StrayMember struct {
	// Path is the stray anchor file.
	Path string
	// Kind is KindComment or KindItem.
	Kind string
	// Card is the card the member belongs to, nil for a column's comment.
	Card *Card
	// Column is the column a column comment belongs to.
	Column string
	// Item is the item a comment hangs on, or the item itself.
	Item string
	// ID is the member's own identifier.
	ID string
	// Archived says the member stood in an archive mirror.
	Archived bool
}

// StrayMemberFiles finds every comment.md and item.md standing below a card
// or a column, which in the card-unit layout nothing should have written.
func (b *Bench) StrayMemberFiles() ([]StrayMember, error) {
	var strays []StrayMember
	for _, half := range []ResolutionHalf{LiveHalf, ArchivedHalf} {
		root := b.cardsRootIn(half)
		ids, err := b.ListIDs(root)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			card, err := b.LoadCardIn(root, id)
			if err != nil {
				continue
			}
			for _, anchor := range legacyAnchorsBelow(b.source(), card.Dir) {
				strays = append(strays, strayOf(anchor, card, ""))
			}
		}
	}
	for _, column := range b.Columns {
		for _, anchor := range legacyAnchorsBelowColumn(b.source(), b.ColumnDir(column.ID)) {
			strays = append(strays, strayOf(anchor, nil, column.ID))
		}
	}
	ids, err := b.ListIDs(b.ArchivedColumnsRoot())
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		for _, anchor := range legacyAnchorsBelowColumn(b.source(), filepath.Join(b.ArchivedColumnsRoot(), id)) {
			strays = append(strays, strayOf(anchor, nil, id))
		}
	}
	return strays, nil
}

// strayOf reads what a stray anchor's path says about the member.
func strayOf(anchor string, card *Card, column string) StrayMember {
	dir := filepath.Dir(anchor)
	stray := StrayMember{Path: anchor, Card: card, Column: column, ID: filepath.Base(dir)}
	stray.Kind = KindComment
	collection := CommentsDir
	if filepath.Base(anchor) == legacyAnchorOf(KindItem) {
		stray.Kind = KindItem
		collection = ChecklistDir
	}
	slashed := filepath.ToSlash(dir)
	stray.Archived = strings.HasSuffix(slashed, "/"+ArchiveDir+"/"+collection+"/"+stray.ID)
	if stray.Kind == KindComment {
		if at := strings.LastIndex(slashed, "/"+ChecklistDir+"/"); at >= 0 {
			stray.Item = strings.SplitN(slashed[at+len(ChecklistDir)+2:], "/", 2)[0]
		}
	}
	return stray
}

// carryStray appends one stray as a baseline and deletes it.
func (b *Bench) carryStray(run StorageMigrationRun, held *Lock, stray StrayMember, state *storageState, report *StorageMigration) (bool, error) {
	text, err := b.ReadText(stray.Path)
	if err != nil {
		return false, err
	}
	dir := filepath.Dir(stray.Path)
	var ev Event
	var key string
	if stray.Kind == KindItem {
		item := itemFromText(dir, text)
		item.Archived = stray.Archived
		ev = b.itemBaseline(run.Template, item)
		key = "item/" + stray.Card.ID + "/" + item.ID
	} else {
		comment := commentFromText(dir, stray.ID, text)
		comment.Holder = stray.Item
		comment.Archived = stray.Archived
		ev = b.commentBaseline(run.Template, comment, stray.Column)
		owner := stray.Column
		if stray.Card != nil {
			owner = stray.Card.ID
		}
		key = "comment/" + owner + "/" + comment.ID
	}
	if stray.Card == nil {
		if err := AppendEvent(held, b.JournalPath(), ev); err != nil {
			return false, err
		}
	} else {
		lock, err := b.takeLock(stray.Card.Dir, run.Template.Actor.Name, run.Template.TS)
		if err != nil {
			var refusal *contract.Refusal
			if errors.As(err, &refusal) && refusal.Name == contract.Locked {
				report.Held = &StorageHeldCard{Card: stray.Card.Ref(b.Slug), Holder: refusal.Detail}
				return true, nil
			}
			return false, err
		}
		err = AppendEvent(lock, stray.Card.JournalPath(), ev)
		lock.Release()
		if err != nil {
			return false, err
		}
	}
	if stopped, err := storageStep(report, "remove", stray.Path, func() error { return removeIfPresent(stray.Path) }); err != nil || stopped {
		return stopped, err
	}
	ownerDir := b.ColumnDir(stray.Column)
	if stray.Card != nil {
		ownerDir = stray.Card.Dir
	} else if b.Column(stray.Column) == nil {
		ownerDir = filepath.Join(b.ArchivedColumnsRoot(), stray.Column)
	}
	if stopped, err := pruneOldLayout(b.source(), ownerDir, report); err != nil || stopped {
		return stopped, err
	}
	state.Strays = append(state.Strays, key)
	return false, b.writeStorageState(state)
}
