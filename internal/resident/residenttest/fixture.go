// Package residenttest holds what the tests of dinah-619's resident workbench
// share: the notifier a test drives by hand, and the fixture workbench every
// comparison between the disk and a snapshot is taken over. It is imported by
// tests alone.
package residenttest

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/perfstore"
)

// FixtureSeed is the seed Fixture generates with.
const FixtureSeed uint64 = 619

// LargePayloadBytes is the size Fixture grows one attachment's payload to,
// past bench.AttachmentHeadBytes, so a snapshot holds only its head.
const LargePayloadBytes = bench.AttachmentHeadBytes + 4096

// Built is a fixture workbench and the files it was made to carry.
type Built struct {
	// Root is the workbench directory.
	Root string
	// Store is what perfstore generated.
	Store *perfstore.Store
	// LargePayload is the payload grown past AttachmentHeadBytes.
	LargePayload string
	// CRLFAnchor is an item anchor rewritten with CRLF line endings.
	CRLFAnchor string
	// BOMAnchor is a comment anchor rewritten with a byte-order mark.
	BOMAnchor string
}

// Fixture writes the fixture workbench under a fresh temporary directory: a
// perfstore SmallShape store, which carries cards, items, comments on cards
// and on items, attachments, archived cards and workstreams, with one payload
// grown past bench.AttachmentHeadBytes, one item anchor stored with CRLF line
// endings and one comment anchor stored with a byte-order mark.
func Fixture(t testing.TB) *Built {
	t.Helper()
	store, err := perfstore.Generate(t.TempDir(), FixtureSeed, perfstore.SmallShape())
	if err != nil {
		t.Fatalf("generate the fixture workbench: %v", err)
	}
	built := &Built{Root: store.Root, Store: store}
	var payloads, items, comments []string
	err = filepath.WalkDir(store.Root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		// Sibling ifs rather than a switch, because a switch naming anchor
		// constants is a second copy of the containment grammar, which
		// TestTheContainmentGrammarIsDeclaredOnce refuses.
		if filepath.Base(filepath.Dir(path)) == bench.PayloadDir {
			payloads = append(payloads, path)
		}
		if entry.Name() == bench.ItemAnchor {
			items = append(items, path)
		}
		if entry.Name() == bench.CommentAnchor && strings.Contains(filepath.ToSlash(path), "/"+bench.ChecklistDir+"/") {
			comments = append(comments, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the fixture workbench: %v", err)
	}
	if len(payloads) == 0 || len(items) == 0 || len(comments) == 0 {
		t.Fatalf("the fixture holds %d payloads, %d items and %d item comments, and it needs at least one of each", len(payloads), len(items), len(comments))
	}
	sort.Strings(payloads)
	sort.Strings(items)
	sort.Strings(comments)

	built.LargePayload = payloads[0]
	large := bytes.Repeat([]byte("large payload line, well past the head a snapshot holds\n"), LargePayloadBytes/56+1)
	if err := os.WriteFile(built.LargePayload, large[:LargePayloadBytes], 0o644); err != nil {
		t.Fatalf("grow a payload: %v", err)
	}

	built.CRLFAnchor = items[0]
	rewrite(t, built.CRLFAnchor, func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
	})
	built.BOMAnchor = comments[0]
	rewrite(t, built.BOMAnchor, func(data []byte) []byte {
		return append([]byte(byteOrderMark), data...)
	})
	return built
}

// rewrite replaces a file's bytes with what change makes of them.
func rewrite(t testing.TB, path string, change func([]byte) []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := os.WriteFile(path, change(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// byteOrderMark is the UTF-8 encoding of U+FEFF, spelled as bytes so no
// source file carries the mark itself.
const byteOrderMark = "\xef\xbb\xbf"
