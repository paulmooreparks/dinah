package bench_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/resident/residenttest"
)

// derived is what the probe derive function of the Disk test answers: the
// three arguments it was handed, so the test can see what Derive read.
type derived struct {
	path, text, revision string
}

// TestDiskAnswersWhatTheFreeReadersAnswer is part of dinah-619/criteria/13.
// Over the fixture workbench, Disk's six methods answer what the os calls and
// the pre-seam readers answer for every file and directory, together with an
// absent path and a file named as a directory. The expected text and revision
// are computed here from the stored bytes, the way the free readers computed
// them before the seam, so the comparison does not reduce to Disk against
// itself.
func TestDiskAnswersWhatTheFreeReadersAnswer(t *testing.T) {
	built := residenttest.Fixture(t)
	disk := bench.Disk{}
	var files, dirs []string
	err := filepath.WalkDir(built.Root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
		} else {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, path := range files {
		want, wantErr := os.ReadFile(path)
		got, gotErr := disk.ReadFile(path)
		if !sameError(wantErr, gotErr) || string(want) != string(got) {
			t.Errorf("ReadFile(%s) answered %d bytes and %v, and os.ReadFile %d bytes and %v", path, len(got), gotErr, len(want), wantErr)
		}
		head, headErr := disk.ReadHead(path, bench.AttachmentHeadBytes)
		if headErr != nil || string(head) != string(want[:min(len(want), bench.AttachmentHeadBytes)]) {
			t.Errorf("ReadHead(%s) answered %d bytes and %v", path, len(head), headErr)
		}
		wantHead := directHead(t, path, bench.AttachmentHeadBytes)
		if string(head) != string(wantHead) {
			t.Errorf("ReadHead(%s) differs from os.Open with io.ReadFull", path)
		}
		sameStat(t, disk, path)
		stored := string(want)
		wantText := bench.NormalizeNewlines(strings.TrimPrefix(stored, byteOrderMark))
		wantRevision := bench.TextRevision(stored)
		text, revision, textErr := disk.Text(path)
		if textErr != nil || text != wantText || revision != wantRevision {
			t.Errorf("Text(%s) answered %q, %s, %v", path, abbreviate(text), revision, textErr)
		}
		calls := 0
		probe := func(p, text, revision string) (any, error) {
			calls++
			return derived{p, text, revision}, nil
		}
		for range 2 {
			value, deriveErr := disk.Derive(path, bench.DeriveAnchor, probe)
			if deriveErr != nil || value != (derived{path, wantText, wantRevision}) {
				t.Errorf("Derive(%s) answered %v and %v", path, value, deriveErr)
			}
		}
		if calls != 2 {
			t.Errorf("Derive(%s) called derive %d times over two calls, and Disk memoises nothing", path, calls)
		}
		compared++
	}
	for _, dir := range dirs {
		want, wantErr := os.ReadDir(dir)
		got, gotErr := disk.ReadDir(dir)
		if !sameError(wantErr, gotErr) || !reflect.DeepEqual(names(want), names(got)) {
			t.Errorf("ReadDir(%s) answered %v and %v, and os.ReadDir %v and %v", dir, names(got), gotErr, names(want), wantErr)
		}
		sameStat(t, disk, dir)
		compared++
	}
	absent := filepath.Join(built.Root, "no-such-file")
	fileAsDir := filepath.Join(built.CRLFAnchor, "below")
	for _, path := range []string{absent, fileAsDir} {
		_, wantErr := os.ReadFile(path)
		_, gotErr := disk.ReadFile(path)
		if !sameError(wantErr, gotErr) {
			t.Errorf("ReadFile(%s) answered %v, and os.ReadFile %v", path, gotErr, wantErr)
		}
		_, wantDirErr := os.ReadDir(path)
		_, gotDirErr := disk.ReadDir(path)
		if !sameError(wantDirErr, gotDirErr) {
			t.Errorf("ReadDir(%s) answered %v, and os.ReadDir %v", path, gotDirErr, wantDirErr)
		}
		sameStat(t, disk, path)
		if _, _, err := disk.Text(path); !sameError(wantErr, err) {
			t.Errorf("Text(%s) answered %v, and os.ReadFile %v", path, err, wantErr)
		}
		compared++
	}
	// The directory of a file answers the file's read as os.ReadDir does.
	_, wantDirErr := os.ReadDir(built.CRLFAnchor)
	_, gotDirErr := disk.ReadDir(built.CRLFAnchor)
	if !sameError(wantDirErr, gotDirErr) {
		t.Errorf("ReadDir of a file answered %v, and os.ReadDir %v", gotDirErr, wantDirErr)
	}
	large, err := disk.ReadHead(built.LargePayload, bench.AttachmentHeadBytes)
	if err != nil || len(large) != bench.AttachmentHeadBytes {
		t.Errorf("ReadHead of the large payload answered %d bytes and %v, wanted %d", len(large), err, bench.AttachmentHeadBytes)
	}
	t.Logf("compared %d paths: %d files, %d directories and two absent paths", compared, len(files), len(dirs))
	if len(files) < 500 || len(dirs) < 500 || compared != len(files)+len(dirs)+2 {
		t.Fatalf("compared %d paths over %d files and %d directories, and the fixture holds more than five hundred of each", compared, len(files), len(dirs))
	}
}

// directHead is os.Open followed by io.ReadFull, a short read answered as the
// bytes read.
func directHead(t *testing.T, path string, n int) []byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	buf := make([]byte, n)
	read, err := io.ReadFull(file, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	return buf[:read]
}

// sameStat compares Disk.Stat with os.Stat on the members a read consults.
func sameStat(t *testing.T, disk bench.Disk, path string) {
	t.Helper()
	want, wantErr := os.Stat(path)
	got, gotErr := disk.Stat(path)
	if !sameError(wantErr, gotErr) {
		t.Errorf("Stat(%s) answered %v, and os.Stat %v", path, gotErr, wantErr)
		return
	}
	if wantErr != nil {
		return
	}
	if got.Name() != want.Name() || got.Size() != want.Size() || got.IsDir() != want.IsDir() || got.Mode() != want.Mode() {
		t.Errorf("Stat(%s) answered %s %d %v, and os.Stat %s %d %v", path, got.Name(), got.Size(), got.Mode(), want.Name(), want.Size(), want.Mode())
	}
}

// sameError reports whether two errors are both nil or both carry the same
// text, which is what a caller reading the error can tell apart.
func sameError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Error() == b.Error()
}

// names lists directory entries by name and kind.
func names(entries []fs.DirEntry) []string {
	var out []string
	for _, entry := range entries {
		out = append(out, entry.Name()+"|"+entry.Type().String())
	}
	return out
}

// abbreviate shortens text for a failure message.
func abbreviate(text string) string {
	if len(text) > 60 {
		return text[:60] + "..."
	}
	return text
}

// byteOrderMark is the UTF-8 encoding of U+FEFF, spelled as escapes so no
// source file carries the mark itself.
const byteOrderMark = "\xef\xbb\xbf"
