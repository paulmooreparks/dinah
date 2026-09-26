package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strconv"
)

// The entity-key prefixes the change walk names its entities by. A key is the
// collection directory and the identifier, so a card keeps one key across an
// archiving, which moves the directory and not the key.
const (
	// WorkbenchKey is the workbench's own key, which carries no identifier
	// because a workbench is the one entity there is exactly one of.
	WorkbenchKey = "workbench"
)

// Watched is one entity of the change walk: where its history and its anchor
// live, and the two values a call compares against the caller's cursor.
//
// The live and archive halves are exactly the set journalFor can name, with
// one stated exclusion described on WatchedEntities. The column half is the
// one part of the walk that is not, because a column has no journal of its
// own; it carries an anchor and nothing else. Nothing here is written and no
// lock is taken, so a walk is safe to run at any moment and against any bench.
type Watched struct {
	// Key names the entity: workbench, workstreams/<id>, cards/<id>, or
	// columns/<id>.
	Key string
	// Journal is the entity's own journal, which may not exist yet, and
	// which is empty for an entity that carries no journal at all. A
	// reader tests the empty string before it reads, because an empty path
	// is a fact about this struct rather than a file whose absence happens
	// to be reported as one errno on one platform.
	Journal string
	// Anchor is the entity's anchor file, empty for an archived card, whose
	// anchor is deliberately outside every fingerprint.
	Anchor string
	// Size is the journal's byte size, zero for a journal that is absent.
	Size int64
	// Revision is the anchor's content hash, empty when there is no anchor
	// in the fingerprint or the anchor could not be read.
	Revision string
}

// WatchedEntities walks the bench and reports the three halves of the change
// set: the live half, which is the workbench, every live workstream and every
// live card; the archive half, which is every archived card; and the column
// half, which is every live column's anchor.
//
// All three slices come back sorted by key, which is the order Digest renders
// them in, so a caller never sorts them again. A collection that cannot be
// listed is reported rather than contributing nothing, because a change set
// missing half the workbench reads exactly like a workbench where nothing
// changed.
//
// Two exclusions are deliberate and neither is an oversight. An archived
// card contributes its journal size alone, because its anchor describes no
// live state a caller would act on and because reading every archived anchor
// on every call is the cost the archive digest term exists to avoid. And the
// archived half of the workstreams collection is out of the walk entirely:
// archiving a workstream drops its key out of the live term, so a caller
// still learns the board moved, and the acts recorded inside an archived
// entity are not acts a caller has anything left to do about.
//
// The column half is its own term rather than part of the live one. A column
// carries no journal, so an edit to a column anchor delivers no line, and a
// live term that moved with nothing delivered is the unexplained case that
// resyncs every live card. Folding columns into the live half would therefore
// make a hand-edited column instructions file trigger a full card read on
// every caller of Changes. A separate term costs one more sha256 and keeps
// the two questions apart.
func (b *Bench) WatchedEntities() (live, archive, columns []Watched, err error) {
	// A collection directory that does not exist is an ordinary, legitimate
	// shape (a fresh bench carries no workstreams yet, for one), and
	// readCollection reads that as empty rather than as an error, which is
	// what every other caller of ListIDs wants. The workbench's own root
	// directory disappearing out from under an open handle is a different
	// fact: every collection below it reads as equally, uniformly absent,
	// and "the whole board just became empty" is indistinguishable, from
	// that shape alone, from "the workbench went away". dinah-546 AC-4 is
	// this exact case reached through a waiting changes call, so it is
	// checked once, here, ahead of the collection reads the comment above
	// already says should report rather than silently contribute nothing.
	if _, statErr := b.source().Stat(b.Root); statErr != nil {
		return nil, nil, nil, statErr
	}
	live = append(live, watch(b.source(), WorkbenchKey, b.JournalPath(), filepath.Join(b.Root, WorkbenchAnchor)))
	workstreamIDs, err := b.ListIDs(b.WorkstreamsRoot())
	if err != nil {
		return nil, nil, nil, err
	}
	for _, id := range workstreamIDs {
		dir := filepath.Join(b.WorkstreamsRoot(), id)
		live = append(live, watch(b.source(), WorkstreamsDir+"/"+id, filepath.Join(dir, JournalName), filepath.Join(dir, WorkstreamAnchor)))
	}
	cardIDs, err := b.ListIDs(b.CardsRoot())
	if err != nil {
		return nil, nil, nil, err
	}
	for _, id := range cardIDs {
		dir := filepath.Join(b.CardsRoot(), id)
		live = append(live, watch(b.source(), CardsDir+"/"+id, filepath.Join(dir, JournalName), filepath.Join(dir, CardAnchor)))
	}
	archivedIDs, err := b.ListIDs(b.ArchivedCardsRoot())
	if err != nil {
		return nil, nil, nil, err
	}
	for _, id := range archivedIDs {
		dir := filepath.Join(b.ArchivedCardsRoot(), id)
		archive = append(archive, watch(b.source(), CardsDir+"/"+id, filepath.Join(dir, JournalName), ""))
	}
	// The column half reads the flow the bench opened with rather than
	// listing the collection, so a directory carrying no anchor, which
	// dinah check reports as orphaned, contributes nothing here either.
	for _, column := range b.Columns {
		columns = append(columns, watch(b.source(), ColumnsDir+"/"+column.ID, "", b.ColumnAnchorPath(column.ID)))
	}
	sortWatched(live)
	sortWatched(archive)
	sortWatched(columns)
	return live, archive, columns, nil
}

// watch reads one entity's two values off the filesystem. An absent journal
// is size zero and an anchor that will not read carries no revision, which is
// the absent-means-empty rule applied to a comparison rather than to a
// listing. An entity carrying no journal at all is not stat-ed, the empty
// path being tested rather than the error a stat of it happens to give.
func watch(src Source, key, journal, anchor string) Watched {
	entry := Watched{Key: key, Journal: journal, Anchor: anchor}
	if journal != "" {
		if info, err := src.Stat(journal); err == nil {
			entry.Size = info.Size()
		}
	}
	if anchor != "" {
		if revision, err := revision(src, anchor); err == nil {
			entry.Revision = revision
		}
	}
	return entry
}

// sortWatched puts a half of the walk in key order, which is the order the
// digest renders and the order a merged read breaks its timestamp ties by.
func sortWatched(entries []Watched) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
}

// Digest renders one half of the walk as a single opaque term: the sha256 of
// each entity's key, journal size and anchor revision, in key order.
//
// The term rests on file content and on nothing else. It reads no modification
// time, asks no operating system for a notification, and consults no clock, so
// it means the same thing on every platform and on a network share, and a
// caller comparing two terms is comparing bytes that were on disk.
//
// The key travels into the hash beside the two values, so an entity that
// appears or disappears moves the term whatever the sizes around it do.
func Digest(entries []Watched) string {
	sum := sha256.New()
	for _, entry := range entries {
		sum.Write([]byte(entry.Key))
		sum.Write([]byte{0})
		sum.Write([]byte(strconv.FormatInt(entry.Size, 10)))
		sum.Write([]byte{0})
		sum.Write([]byte(entry.Revision))
		sum.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(sum.Sum(nil))
}
