package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestACardIsReadOnce drives dinah-618 criteria/13. A card's anchor is opened
// once per load, and the revision that one read reports is the revision of
// the stored bytes, which is what Revision computes over the same file. The
// three encodings are the three a basis has to survive: the LF the format
// writes, and the CRLF and byte-order mark an editor can leave behind, both of
// which the text is normalised past and the revision is not.
//
// The test writes a package var, so it declares itself non-parallel and puts
// the seam back.
//
// Arming: hashing the normalised text instead of the raw bytes in
// readTextAndRevision reddens the CRLF and byte-order-mark cases on the
// revision and leaves the LF case green, and restoring the second Revision
// read in loadCard reddens all three on the open count.
func TestACardIsReadOnce(t *testing.T) {
	root := newFixture(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	anchor := filepath.Join(root, CardsDir, "c00000000001", CardAnchor)
	t.Cleanup(func() { AnchorReadObserver = nil })
	encodings := []struct {
		name   string
		stored string
	}{
		{"LF", cleanCard},
		{"CRLF", strings.ReplaceAll(cleanCard, "\n", "\r\n")},
		{"byte-order mark", byteOrderMark + cleanCard},
	}
	revisions := map[string]string{}
	checked := 0
	for _, encoding := range encodings {
		write(t, anchor, encoding.stored)
		want, err := Revision(anchor)
		if err != nil {
			t.Fatalf("%s: Revision: %v", encoding.name, err)
		}
		opens := 0
		AnchorReadObserver = func(path string) {
			if path == anchor {
				opens++
			}
		}
		card, err := opened.LoadCardIn(opened.CardsRoot(), "c00000000001")
		AnchorReadObserver = nil
		if err != nil {
			t.Fatalf("%s: LoadCardIn: %v", encoding.name, err)
		}
		checked++
		if opens != 1 {
			t.Errorf("%s: LoadCardIn opened the card anchor %d times, wanted once", encoding.name, opens)
		}
		if card.Revision != want {
			t.Errorf("%s: the card carries revision %s, and Revision over the same file answers %s", encoding.name, card.Revision, want)
		}
		if card.Title != "A card" {
			t.Errorf("%s: the card reads its title as %q, wanted %q, so the text was not normalised", encoding.name, card.Title, "A card")
		}
		revisions[encoding.name] = card.Revision
	}
	if checked != len(encodings) || checked != 3 {
		t.Fatalf("checked %d anchors, wanted 3", checked)
	}
	// The three stored byte sequences differ, so three equal revisions would
	// mean the hash never saw the bytes that differ.
	if revisions["LF"] == revisions["CRLF"] || revisions["LF"] == revisions["byte-order mark"] {
		t.Errorf("the three encodings report revisions %v, and the CRLF and byte-order-mark files should differ from the LF one", revisions)
	}
}

// positionsCase is one collection condition TestPositionsAgreeWithTheResolver
// builds on its own card: how many members to write, what ordinal each
// carries (zero for none), which member to delete after writing, and which
// member to leave without an anchor.
type positionsCase struct {
	name       string
	ordinals   []int
	remove     int
	unreadable int
	// journal names the comments and attachments in reverse, so a journal
	// order that differs from the listing is what fallbackRank recovers.
	journal bool
	members int
}

// TestPositionsAgreeWithTheResolver drives dinah-618 criteria/8. Over a
// stamped collection, an unstamped one ordered by the card's journal, one
// gapped by a removed member, and one holding a member whose anchor will not
// read, Positions answers every position MemberPosition answers, and its
// Items, Comments and Attachments answer what the uncached readers answer,
// element by element. Each condition is built on a card of its own and on all
// three of a card's collections.
//
// Arming: making Positions.Sorted skip a member whose anchor will not read
// shifts every later position by one, which reddens the unreadable case.
func TestPositionsAgreeWithTheResolver(t *testing.T) {
	root := newFixture(t)
	cases := []positionsCase{
		{name: "stamped", ordinals: []int{3, 1, 4, 2}, members: 4},
		{name: "unstamped", ordinals: []int{0, 0, 0, 0}, journal: true, members: 4},
		{name: "gapped", ordinals: []int{1, 2, 3, 4}, remove: 2, members: 3},
		{name: "unreadable", ordinals: []int{2, 3, 1, 4}, unreadable: 3, members: 4},
	}
	mounts := []struct {
		dir, anchor string
	}{
		{ChecklistDir, ItemAnchor},
		{CommentsDir, CommentAnchor},
		{AttachmentsDir, AttachmentAnchor},
	}
	for n, tc := range cases {
		cardDir := filepath.Join(root, CardsDir, fmt.Sprintf("c1000000000%d", n+1))
		write(t, filepath.Join(cardDir, CardAnchor), cleanCard)
		var journal strings.Builder
		journal.WriteString(cleanJournal)
		for _, mount := range mounts {
			collection := filepath.Join(cardDir, mount.dir)
			for i := len(tc.ordinals); i >= 1; i-- {
				id := fmt.Sprintf("a0000000000%d", i)
				plantPositionsMember(t, collection, mount.dir, id, tc.ordinals[i-1], i == tc.unreadable)
				if tc.journal && mount.dir == CommentsDir {
					fmt.Fprintf(&journal, `{"ts":"2026-08-17T09:00:0%dZ","event":"commented","actor":"alka","comment":"%s"}`+"\n", i, id)
				}
				if tc.journal && mount.dir == AttachmentsDir {
					fmt.Fprintf(&journal, `{"ts":"2026-08-17T09:00:0%dZ","event":"attached","actor":"alka","attachment":"%s"}`+"\n", i, id)
				}
			}
			if tc.remove > 0 {
				if err := os.RemoveAll(filepath.Join(collection, fmt.Sprintf("a0000000000%d", tc.remove))); err != nil {
					t.Fatalf("%s: remove: %v", tc.name, err)
				}
			}
		}
		write(t, filepath.Join(cardDir, JournalName), journal.String())

		positions := NewPositions()
		for _, mount := range mounts {
			collection := filepath.Join(cardDir, mount.dir)
			ids, err := ListIDs(collection)
			if err != nil {
				t.Fatalf("%s %s: ListIDs: %v", tc.name, mount.dir, err)
			}
			compared := 0
			for _, id := range ids {
				dir := filepath.Join(collection, id)
				want, err := MemberPosition(dir, mount.anchor)
				if err != nil {
					t.Fatalf("%s %s: MemberPosition: %v", tc.name, mount.dir, err)
				}
				got, err := positions.Of(dir, mount.anchor)
				if err != nil {
					t.Fatalf("%s %s: Positions.Of: %v", tc.name, mount.dir, err)
				}
				if got != want {
					t.Errorf("%s %s: member %s sits at %d by Positions and at %d by MemberPosition", tc.name, mount.dir, id, got, want)
				}
				compared++
			}
			if compared != tc.members {
				t.Errorf("%s %s: compared %d members, wanted %d", tc.name, mount.dir, compared, tc.members)
			}
		}
		if tc.journal {
			// The journal names the comments last to first, so a sort that
			// fell back to the listing would put a00000000001 first and this
			// case would not be exercising fallbackRank's journal order.
			comments, err := positions.Comments(cardDir)
			if err != nil || len(comments) == 0 || comments[0].ID != "a00000000004" {
				t.Fatalf("%s: the journal-ordered comments do not open with a00000000004 (err %v), so the journal was not read", tc.name, err)
			}
		}
		readable := tc.members
		if tc.unreadable > 0 {
			readable--
		}
		comparedLists(t, tc.name+" checklist", readable,
			func() (any, error) { return Items(cardDir) },
			func() (any, error) { return positions.Items(cardDir) })
		comparedLists(t, tc.name+" comments", readable,
			func() (any, error) { return Comments(cardDir) },
			func() (any, error) { return positions.Comments(cardDir) })
		comparedLists(t, tc.name+" attachments", readable,
			func() (any, error) { return Attachments(cardDir) },
			func() (any, error) { return positions.Attachments(cardDir) })
	}
}

// comparedLists asserts that an uncached reader and its Positions counterpart
// answer lists of the same length, equal element by element, and of the
// length the fixture planted as readable.
func comparedLists(t *testing.T, name string, want int, uncached, memoised func() (any, error)) {
	t.Helper()
	a, err := uncached()
	if err != nil {
		t.Fatalf("%s: uncached: %v", name, err)
	}
	b, err := memoised()
	if err != nil {
		t.Fatalf("%s: Positions: %v", name, err)
	}
	left, right := reflect.ValueOf(a), reflect.ValueOf(b)
	if left.Len() != want || right.Len() != want {
		t.Fatalf("%s: the uncached reader answered %d members and Positions %d, wanted %d", name, left.Len(), right.Len(), want)
	}
	for i := 0; i < left.Len(); i++ {
		if !reflect.DeepEqual(left.Index(i).Interface(), right.Index(i).Interface()) {
			t.Errorf("%s: member %d differs:\n uncached  %+v\n Positions %+v", name, i+1, left.Index(i).Elem().Interface(), right.Index(i).Elem().Interface())
		}
	}
}

// plantPositionsMember writes one member of a collection with the ordinal
// given, or none at zero. An unreadable member is a directory with no anchor
// in it, which is what an interrupted claim leaves behind.
func plantPositionsMember(t *testing.T, collection, mount, id string, ordinal int, unreadable bool) {
	t.Helper()
	dir := filepath.Join(collection, id)
	if unreadable {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		return
	}
	stamp := ""
	if ordinal > 0 {
		stamp = fmt.Sprintf("%s: %d\n", OrdinalField, ordinal)
	}
	switch mount {
	case ChecklistDir:
		write(t, filepath.Join(dir, ItemAnchor), "---\nkind: decision\nstate: pending\n"+stamp+"---\nItem "+id+".\n")
	case CommentsDir:
		write(t, filepath.Join(dir, CommentAnchor), "---\nts: 2026-08-17T09:00:00Z\nauthor: alka\n"+stamp+"---\nComment "+id+".\n")
	case AttachmentsDir:
		write(t, filepath.Join(dir, AttachmentAnchor), "---\nfilename: "+id+".txt\n"+stamp+"---\n")
		write(t, filepath.Join(dir, PayloadDir, id+".txt"), "bytes")
	}
}
