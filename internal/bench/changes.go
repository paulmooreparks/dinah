package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
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
// The set of them is exactly the set journalFor can name, with one stated
// exclusion described on WatchedEntities. Nothing here is written and no lock
// is taken, so a walk is safe to run at any moment and against any bench.
type Watched struct {
	// Key names the entity: workbench, workstreams/<id>, or cards/<id>.
	Key string
	// Journal is the entity's own journal, which may not exist yet.
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

// WatchedEntities walks the bench and reports the two halves of the change
// set: the live half, which is the workbench, every live workstream and every
// live card, and the archive half, which is every archived card.
//
// Both slices come back sorted by key, which is the order Digest renders them
// in, so a caller never sorts them again.
//
// Two exclusions are deliberate and neither is an oversight. An archived
// card contributes its journal size alone, because its anchor describes no
// live column a caller would act on and because reading every archived anchor
// on every call is the cost the archive digest term exists to avoid. And the
// archived half of the workstreams collection is out of the walk entirely:
// archiving a workstream drops its key out of the live term, so a caller
// still learns the board moved, and the acts recorded inside an archived
// entity are not acts a caller has anything left to do about.
func (b *Bench) WatchedEntities() (live, archive []Watched) {
	live = append(live, watch(WorkbenchKey, b.JournalPath(), filepath.Join(b.Root, WorkbenchAnchor)))
	for _, id := range ListIDs(b.WorkstreamsRoot()) {
		dir := filepath.Join(b.WorkstreamsRoot(), id)
		live = append(live, watch(WorkstreamsDir+"/"+id, filepath.Join(dir, JournalName), filepath.Join(dir, WorkstreamAnchor)))
	}
	for _, id := range ListIDs(b.CardsRoot()) {
		dir := filepath.Join(b.CardsRoot(), id)
		live = append(live, watch(CardsDir+"/"+id, filepath.Join(dir, JournalName), filepath.Join(dir, CardAnchor)))
	}
	for _, id := range ListIDs(b.ArchivedCardsRoot()) {
		dir := filepath.Join(b.ArchivedCardsRoot(), id)
		archive = append(archive, watch(CardsDir+"/"+id, filepath.Join(dir, JournalName), ""))
	}
	sortWatched(live)
	sortWatched(archive)
	return live, archive
}

// watch reads one entity's two values off the filesystem. An absent journal
// is size zero and an anchor that will not read carries no revision, which is
// the absent-means-empty rule applied to a comparison rather than to a
// listing.
func watch(key, journal, anchor string) Watched {
	entry := Watched{Key: key, Journal: journal, Anchor: anchor}
	if info, err := os.Stat(journal); err == nil {
		entry.Size = info.Size()
	}
	if anchor != "" {
		if revision, err := Revision(anchor); err == nil {
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
