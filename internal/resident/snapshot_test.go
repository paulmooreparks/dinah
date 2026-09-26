package resident_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/resident"
	"dinah/internal/resident/residenttest"
)

// passThroughs counts the paths a snapshot answered by reading the disk.
type passThroughs struct {
	mu    sync.Mutex
	paths []string
}

func (p *passThroughs) hook(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paths = append(p.paths, path)
}

func (p *passThroughs) take() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	taken := p.paths
	p.paths = nil
	return taken
}

// openSnapshot opens a resident over root with a hand-driven notifier and
// answers its first snapshot.
func openSnapshot(t *testing.T, root string, hooks *resident.Hooks) (*resident.Workbench, *residenttest.Manual, *resident.Snapshot) {
	t.Helper()
	manual := residenttest.NewManual()
	w, err := resident.Open(root, resident.Options{Notifier: manual, Hooks: hooks})
	if err != nil {
		t.Fatalf("open the resident: %v", err)
	}
	t.Cleanup(func() { w.Close() })
	select {
	case <-w.Ready():
	case <-time.After(30 * time.Second):
		t.Fatal("the first build did not finish within 30 seconds")
	}
	pick := w.Current(time.Now())
	if pick.Snapshot == nil {
		t.Fatalf("the first build published no snapshot (lapsing %v)", pick.Lapsing)
	}
	return w, manual, pick.Snapshot
}

// TestASnapshotAnswersWhatTheDiskAnswers is part of dinah-619/criteria/13. A
// snapshot built over the fixture answers every Source method as Disk does
// for every path in the tree, for an absent path, a missing journal, a file
// named as a directory and a path outside the root. A payload's head is
// answered from memory and its whole file from the disk.
func TestASnapshotAnswersWhatTheDiskAnswers(t *testing.T) {
	built := residenttest.Fixture(t)
	counted := &passThroughs{}
	_, _, snapshot := openSnapshot(t, built.Root, &resident.Hooks{PassThrough: counted.hook})
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
	payloads := 0
	compared := 0
	for _, path := range files {
		payload := filepath.Base(filepath.Dir(filepath.Dir(path))) != "" && filepath.Base(filepath.Dir(path)) == bench.PayloadDir
		if payload {
			payloads++
		}
		compareFile(t, snapshot, disk, path, payload)
		compared++
	}
	for _, dir := range dirs {
		want, wantErr := disk.ReadDir(dir)
		got, gotErr := snapshot.ReadDir(dir)
		if !sameError(wantErr, gotErr) || !reflect.DeepEqual(entryNames(want), entryNames(got)) {
			t.Errorf("ReadDir(%s) differs from the disk", dir)
		}
		for i := range got {
			wantInfo, _ := want[i].Info()
			gotInfo, _ := got[i].Info()
			if gotInfo == nil || wantInfo == nil || gotInfo.IsDir() != wantInfo.IsDir() || (!gotInfo.IsDir() && gotInfo.Size() != wantInfo.Size()) {
				t.Errorf("the entry %s of %s carries info that differs from the disk's", got[i].Name(), dir)
			}
		}
		compareStat(t, snapshot, disk, dir)
		compared++
	}
	if got := counted.take(); len(got) != 0 {
		t.Errorf("the snapshot passed %d reads through to the disk, wanted none: %v", len(got), got[:min(len(got), 5)])
	}

	// A payload's whole file passes through, once per payload.
	for _, path := range files {
		if filepath.Base(filepath.Dir(path)) != bench.PayloadDir {
			continue
		}
		want, _ := disk.ReadFile(path)
		got, err := snapshot.ReadFile(path)
		if err != nil || string(got) != string(want) {
			t.Errorf("ReadFile of the payload %s differs from the disk", path)
		}
	}
	if got := counted.take(); len(got) != payloads {
		t.Errorf("reading %d payloads whole passed %d reads through, wanted one each", payloads, len(got))
	}

	// The paths that are not in the tree.
	card := filepath.Dir(built.CRLFAnchor)
	for card != built.Root && filepath.Base(filepath.Dir(card)) != bench.CardsDir {
		card = filepath.Dir(card)
	}
	absent := filepath.Join(built.Root, "no-such-file")
	missingJournal := filepath.Join(built.Root, bench.CardsDir, "000000000000", bench.JournalName)
	for _, path := range []string{absent, missingJournal} {
		for name, read := range map[string]func(bench.Source) error{
			"ReadFile": func(s bench.Source) error { _, err := s.ReadFile(path); return err },
			"ReadHead": func(s bench.Source) error { _, err := s.ReadHead(path, 10); return err },
			"ReadDir":  func(s bench.Source) error { _, err := s.ReadDir(path); return err },
			"Stat":     func(s bench.Source) error { _, err := s.Stat(path); return err },
			"Text":     func(s bench.Source) error { _, _, err := s.Text(path); return err },
		} {
			wantErr, gotErr := read(disk), read(snapshot)
			if !errors.Is(wantErr, fs.ErrNotExist) || !errors.Is(gotErr, fs.ErrNotExist) {
				t.Errorf("%s(%s) answered %v, and the disk %v; both should be not-exist", name, path, gotErr, wantErr)
			}
		}
		compared++
	}
	// A file named as a directory, and a path below a file.
	_, wantDirErr := disk.ReadDir(built.CRLFAnchor)
	_, gotDirErr := snapshot.ReadDir(built.CRLFAnchor)
	if !sameError(wantDirErr, gotDirErr) {
		t.Errorf("ReadDir of a file answered %v, and the disk %v", gotDirErr, wantDirErr)
	}
	compared++
	below := filepath.Join(built.CRLFAnchor, "below")
	_, wantBelow := disk.Stat(below)
	_, gotBelow := snapshot.Stat(below)
	if !sameError(wantBelow, gotBelow) {
		t.Errorf("Stat below a file answered %v, and the disk %v", gotBelow, wantBelow)
	}
	compared++
	// A path outside the root is the disk's.
	outside := filepath.Dir(built.Root)
	wantOutside, _ := disk.ReadDir(outside)
	gotOutside, err := snapshot.ReadDir(outside)
	if err != nil || !reflect.DeepEqual(entryNames(wantOutside), entryNames(gotOutside)) {
		t.Errorf("ReadDir outside the root differs from the disk")
	}
	compared++
	passed := counted.take()
	if len(passed) != 1 || passed[0] != below {
		t.Errorf("the absent paths, the file read as a directory and the path outside the root passed %v through, wanted only the path below a file", passed)
	}

	t.Logf("compared %d paths: %d files (%d payloads), %d directories and five others", compared, len(files), payloads, len(dirs))
	if len(files) < 500 || len(dirs) < 500 || payloads == 0 || compared != len(files)+len(dirs)+5 {
		t.Fatalf("compared %d paths over %d files, %d payloads and %d directories, and the fixture holds more", compared, len(files), payloads, len(dirs))
	}
}

// compareFile compares every Source method on one file.
func compareFile(t *testing.T, snapshot *resident.Snapshot, disk bench.Disk, path string, payload bool) {
	t.Helper()
	wantHead, wantErr := disk.ReadHead(path, bench.AttachmentHeadBytes)
	gotHead, gotErr := snapshot.ReadHead(path, bench.AttachmentHeadBytes)
	if !sameError(wantErr, gotErr) || string(wantHead) != string(gotHead) {
		t.Errorf("ReadHead(%s) differs from the disk", path)
	}
	compareStat(t, snapshot, disk, path)
	if payload {
		return
	}
	want, wantErr := disk.ReadFile(path)
	got, gotErr := snapshot.ReadFile(path)
	if !sameError(wantErr, gotErr) || string(want) != string(got) {
		t.Errorf("ReadFile(%s) differs from the disk", path)
	}
	wantText, wantRevision, wantErr := disk.Text(path)
	gotText, gotRevision, gotErr := snapshot.Text(path)
	if !sameError(wantErr, gotErr) || wantText != gotText || wantRevision != gotRevision {
		t.Errorf("Text(%s) differs from the disk", path)
	}
	probe := func(p, text, revision string) (any, error) { return p + "\x00" + text + "\x00" + revision, nil }
	// A kind of the test's own, which no memo holds, so the snapshot derives
	// from its held text rather than answering a memo another derive filled.
	wantDerived, _ := disk.Derive(path, probeKind, probe)
	gotDerived, _ := snapshot.Derive(path, probeKind, probe)
	if wantDerived != gotDerived {
		t.Errorf("Derive(%s) differs from the disk", path)
	}
	if _, err := snapshot.ReadDir(path); !sameError(err, dirOfFileErr(disk, path)) {
		t.Errorf("ReadDir(%s) of a file answered %v", path, err)
	}
}

// probeKind is a DeriveKind no reader uses.
const probeKind bench.DeriveKind = "probe"

// dirOfFileErr is the error the disk answers for ReadDir of a file.
func dirOfFileErr(disk bench.Disk, path string) error {
	_, err := disk.ReadDir(path)
	return err
}

// compareStat compares Stat on the members a read consults.
func compareStat(t *testing.T, snapshot *resident.Snapshot, disk bench.Disk, path string) {
	t.Helper()
	want, wantErr := disk.Stat(path)
	got, gotErr := snapshot.Stat(path)
	if !sameError(wantErr, gotErr) {
		t.Errorf("Stat(%s) answered %v, and the disk %v", path, gotErr, wantErr)
		return
	}
	if wantErr != nil {
		return
	}
	if got.Name() != want.Name() || got.IsDir() != want.IsDir() || got.Mode() != want.Mode() || (!got.IsDir() && got.Size() != want.Size()) {
		t.Errorf("Stat(%s) answered %s %d %v, and the disk %s %d %v", path, got.Name(), got.Size(), got.Mode(), want.Name(), want.Size(), want.Mode())
	}
}

// TestAMisCasedPathReadsTheDisk is part of dinah-619/criteria/13. A path
// whose last element differs from the held entry by case passes through, and
// answers what the disk answers.
func TestAMisCasedPathReadsTheDisk(t *testing.T) {
	built := residenttest.Fixture(t)
	counted := &passThroughs{}
	_, _, snapshot := openSnapshot(t, built.Root, &resident.Hooks{PassThrough: counted.hook})
	dir, name := filepath.Split(built.CRLFAnchor)
	miscased := filepath.Join(dir, strings.ToUpper(name))
	if miscased == built.CRLFAnchor {
		t.Fatal("the anchor's name has no letter to change the case of")
	}
	want, wantErr := os.ReadFile(miscased)
	got, gotErr := snapshot.ReadFile(miscased)
	if !sameError(wantErr, gotErr) || string(want) != string(got) {
		t.Errorf("ReadFile of a mis-cased path answered %d bytes and %v, and the disk %d bytes and %v", len(got), gotErr, len(want), wantErr)
	}
	if passed := counted.take(); len(passed) != 1 || passed[0] != miscased {
		t.Errorf("the mis-cased path passed %v through, wanted exactly it", passed)
	}
}

// sameError reports whether two errors are both nil or carry the same text.
func sameError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Error() == b.Error()
}

// entryNames lists directory entries by name and kind.
func entryNames(entries []fs.DirEntry) []string {
	var out []string
	for _, entry := range entries {
		out = append(out, entry.Name()+"|"+entry.Type().String())
	}
	return out
}
