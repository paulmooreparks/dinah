package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// ReplacesKey is the column member a new definition declares to say which
// retired columns a column takes over from. It is absent from
// knownColumnKeys, which is what carries it through interchange and onto the
// live column's own frontmatter: writeColumnFromMember's generic pass writes
// every member it does not recognise, and exportColumn's matching pass reads
// every such key back out. reject_to travels the same way and is stored on
// the live column in exactly the same manner, so the precedent covers the
// persistence and not only the round trip.
const ReplacesKey = "replaces"

// SourceDigest is the content hash of the bytes a definition was read from,
// rendered as lowercase hex with no algorithm prefix.
//
// Reshape freezes one of these per run and derives an added column's
// identifier from it, so the digest has to be a function of the source alone.
// Revision, the tool's other content hash, carries a "sha256:" prefix because
// a card's basis is compared rather than composed; this one is composed into
// an identifier, so it carries the hex alone.
func SourceDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// DeriveColumnID mints the identifier of a column a reshape adds from an
// element that declares none of its own: the first twelve hex characters of
// the hash of the run's frozen source digest and the element's position in
// the definition's columns array.
//
// It is deliberately not NewID's random draw, and the departure is scoped to
// this one caller. A run that crashes between creating a column's directory
// and appending its identifier to the workbench's own sequence has, with a
// random identifier, no way to recognise its own half-written attempt: the
// value it drew died with it, and the retry draws another and leaves the
// first directory behind with nothing pointing at it. Both inputs here are
// fixed before any write happens and are unchanged by a crash, so a retry
// against the same source recomputes the same identifier, finds whatever the
// first attempt left, and finishes it in place.
//
// A derived identifier can in principle land on one a live column already
// carries, at the same forty-eight bits of exposure ClaimID already accepts.
// Within a run that is refused rather than resolved, because every identifier
// this verb resolves, derived or declared, goes through one claimed set and a
// second claim on one identifier is dinah.malformed. Across runs it would make
// the element sort as kept and rewrite a column somebody else's definition
// declared, which is a sentence here rather than a check because narrowing it
// would mean minting differently than the rest of the tool does.
//
// The guarantee is exactly as wide as its inputs. A retry against an edited
// source hashes differently and derives a different identifier, so it never
// recognises the earlier attempt's column as its own.
//
// What that costs depends on how far the earlier attempt got, and only the
// narrower of the two cases leaves anything behind. An attempt that died
// inside step one, between creating the directory and appending the identifier
// to the workbench's sequence, leaves a directory no sequence names, and that
// residue is what Bench.OrphanedColumnDirectories and
// check.orphaned-column-directory exist to make visible. An attempt that got
// past that append leaves a live column, which is the ordinary case for a run
// stopped by a refusal in a later step, and a live column the edited
// definition does not name is simply a retirement: the second run carries out
// whatever stands in it, archives it, and drops it from the sequence, leaving
// neither an orphaned directory nor a stranded identifier.
// verb.TestAnEditedRetryAfterALateRefusalArchivesTheAddedColumn holds the
// second case.
func DeriveColumnID(sourceDigest string, position int) string {
	sum := sha256.Sum256([]byte(sourceDigest + ":" + strconv.Itoa(position)))
	return hex.EncodeToString(sum[:])[:12]
}

// AssignColumnSlugs settles the slug each element of a definition is written
// with, which is what Instantiate already does for a workbench being born.
// Reshape reads the same answer for a workbench taking a new shape, so a
// column's slug comes from one rule whichever command wrote it.
func AssignColumnSlugs(columns []map[string]json.RawMessage) ([]string, error) {
	return assignColumnSlugs(columns)
}

// MemberString reads an interchange element's member as a string, answering
// the empty string where the member is absent or is not one.
func MemberString(element map[string]json.RawMessage, member string) string {
	return memberString(element, member)
}

// MemberStrings reads an element's member as a list of strings, answering
// nothing where the member is absent or is not one. It is what reads a
// column element's replaces declaration.
func MemberStrings(element map[string]json.RawMessage, member string) []string {
	raw, ok := element[member]
	if !ok {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}

// WriteColumnFromElement writes one column's anchor from an interchange
// element, creating the column's directory first.
//
// The directory is created explicitly rather than left to WriteText's own
// parent creation, because the two writes are the crash window a retry has to
// be able to finish and a caller reasoning about that window needs them to be
// two steps rather than one. MkdirAll tolerates finding the directory already
// there, which is the whole of the recovery on that side: an existing
// directory at this exact path can only be this same element's own prior
// attempt, since DeriveColumnID ties the identifier to one source and one
// position, so finding it is success rather than a collision to retry past.
// That is why ClaimID's atomic claim-or-retry loop is not used here; it
// exists to separate two different random draws, and nothing here draws.
//
// The anchor write itself is writeColumnFromMember, the same unmodified
// function Instantiate calls, so a column a reshape adds and a column an init
// creates carry the same frontmatter for the same element, replaces and every
// other unrecognised member included. Calling it a second time over a
// half-finished run is a harmless overwrite with identical bytes, since
// WriteText renames a whole temporary file into place and the identifier, the
// slug and the element are all fixed before the write phase begins.
func WriteColumnFromElement(root, id, slug string, element map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Join(root, ColumnsDir, id), 0o755); err != nil {
		return err
	}
	return writeColumnFromMember(root, id, slug, element)
}

// ColumnAnchorText reads a column's anchor as it stands, or the empty string
// where the column carries none yet. Reshape reads it before rewriting a kept
// column so that it records an event only for a column whose text the new
// definition actually changed.
func ColumnAnchorText(root, id string) string {
	text, err := ReadText(filepath.Join(root, ColumnsDir, id, ColumnAnchor))
	if err != nil {
		return ""
	}
	return text
}

// SetColumnSequence writes the workbench's columns sequence and saves the
// anchor. It is the one write that decides which columns the flow carries and
// in which order, so reshape's added-column append and its final reorder both
// go through it rather than each reaching into the frontmatter.
func (b *Bench) SetColumnSequence(ids []string) error {
	b.FM.SetSeq("columns", ids)
	return b.Save()
}

// ColumnSequence is the identifiers the workbench's own columns sequence
// carries, in the order it carries them. It is read rather than derived from
// b.Columns, because a stranded identifier is in the sequence and not in the
// column list, and a caller rewriting the sequence must not drop one silently.
func (b *Bench) ColumnSequence() []string {
	return b.FM.Seq("columns")
}
