package resident

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"dinah/internal/bench"
)

// Snapshot is one immutable state of the workbench's files. It implements
// bench.Source.
//
// A snapshot is the tree below the workbench root, held as a map from each
// directory's path relative to the root, with the platform's separators and
// "." for the root itself, to an immutable dirNode. Two snapshots share every dirNode the later one did not
// reconcile, and nothing in a published snapshot is ever written, except the
// memos a fileNode fills lazily under a sync.Once, which are pure functions
// of bytes the node never changes.
type Snapshot struct {
	root string
	// prefix is root with a separator after it, which every path below the
	// root starts with.
	prefix string
	dirs   map[string]*dirNode
	gen    uint64

	// opened and openErr are what bench.OpenWith answered over this
	// snapshot, computed on the applier before the snapshot is published.
	opened  *bench.Bench
	openErr error
	// earliestExpiry is the earliest instant a card's claim lapses, and the
	// zero time when no active card carries an expiry that parses.
	earliestExpiry time.Time
	// lapsing lists every active card with a parsing expiry, by expiry, so
	// Current can name the cards that have lapsed at a given instant.
	lapsing []expiring

	hooks *Hooks
	// files and bytes are what Held reports.
	files int
	bytes int64
}

// expiring is one active card's claim expiry.
type expiring struct {
	at  time.Time
	dir string
}

// dirNode is one held directory.
type dirNode struct {
	info    fs.FileInfo
	entries []fs.DirEntry        // as os.ReadDir answered, in name order
	byName  map[string]int       // entry name to its index in entries
	folded  map[string]bool      // ASCII-lowercased entry names
	files   map[string]*fileNode // regular files among the entries
	// err is what a read of this directory answered when it failed with
	// anything but not-exist. A node carrying one is unheld: every read of
	// it passes through to the disk.
	err error
}

// fileNode is one held regular file.
type fileNode struct {
	info    fs.FileInfo
	data    []byte
	partial bool // an attachment payload, of which only the head is held
	// err is what a read of this file answered when it failed with anything
	// but not-exist, which makes the node unheld.
	err error

	textOnce sync.Once
	text     string
	revision string

	derived [deriveKinds]derivedMemo
}

// derivedMemo is one memoised Derive answer.
type derivedMemo struct {
	once  sync.Once
	value any
	err   error
}

// deriveKinds is how many DeriveKind values bench declares, and kindIndex
// maps each to its slot in fileNode.derived.
const deriveKinds = 7

var kindIndex = map[bench.DeriveKind]int{
	bench.DeriveAnchor:      0,
	bench.DeriveCard:        1,
	bench.DeriveCardRetired: 2,
	bench.DeriveItem:        3,
	bench.DeriveComment:     4,
	bench.DeriveAttachment:  5,
	bench.DeriveJournal:     6,
}

// Bench is the workbench opened over this snapshot, and the error OpenWith
// answered when it would not open.
func (s *Snapshot) Bench() (*bench.Bench, error) {
	return s.opened, s.openErr
}

// Generation is the snapshot's number, one-based and rising.
func (s *Snapshot) Generation() uint64 {
	return s.gen
}

// entryInfo is the fs.FileInfo a snapshot answers for a held file or
// directory. Its size is the held bytes' length, or the stat's for a payload
// of which only the head is held.
type entryInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (i *entryInfo) Name() string       { return i.name }
func (i *entryInfo) Size() int64        { return i.size }
func (i *entryInfo) Mode() fs.FileMode  { return i.mode }
func (i *entryInfo) ModTime() time.Time { return i.modTime }
func (i *entryInfo) IsDir() bool        { return i.mode.IsDir() }
func (i *entryInfo) Sys() any           { return nil }

// dirEntry is the fs.DirEntry a snapshot answers, carrying the info the
// snapshot holds so that Info never reaches the disk.
type dirEntry struct {
	name string
	typ  fs.FileMode
	info fs.FileInfo
	// infoErr is what the stat of this entry answered when it failed.
	infoErr error
}

func (e *dirEntry) Name() string               { return e.name }
func (e *dirEntry) IsDir() bool                { return e.typ.IsDir() }
func (e *dirEntry) Type() fs.FileMode          { return e.typ }
func (e *dirEntry) Info() (fs.FileInfo, error) { return e.info, e.infoErr }
func (e *dirEntry) String() string             { return fs.FormatDirEntry(e) }

// placement is where a path below the root lands in the held tree.
type placement int

const (
	// placedOutside is a path that is not below the root.
	placedOutside placement = iota
	// placedDir is a held directory.
	placedDir
	// placedFile is a held regular file.
	placedFile
	// placedAbsent is a name a held directory's listing does not carry.
	placedAbsent
	// placedUnplaced is a path the held tree cannot answer for: an unheld
	// node, a name matching only under another case, or a walk through a
	// file, which the disk answers.
	placedUnplaced
)

// place answers where a path lands, with the node it lands on.
func (s *Snapshot) place(path string) (placement, *dirNode, *fileNode) {
	rel, ok := s.relative(path)
	if !ok {
		return placedOutside, nil, nil
	}
	if rel == "." {
		if dir, held := s.dirs[rel]; held && dir.err == nil {
			return placedDir, dir, nil
		}
		return placedUnplaced, nil, nil
	}
	// The parent first: most paths a read asks for are files, which the
	// parent's own maps answer without looking the whole path up.
	parentKey, name := splitRel(rel)
	if parent, held := s.dirs[parentKey]; held {
		where, _, file := s.placeIn(parent, name)
		if where != placedDir {
			return where, nil, file
		}
		if dir, held := s.dirs[rel]; held && dir.err == nil {
			return placedDir, dir, nil
		}
		return placedUnplaced, nil, nil
	}
	// The parent is not held: walk from the root to find the element the
	// held tree stops at.
	key := "."
	for _, element := range strings.Split(rel, string(filepath.Separator)) {
		dir, held := s.dirs[key]
		if !held || dir.err != nil {
			return placedUnplaced, nil, nil
		}
		where, _, _ := s.placeIn(dir, element)
		if where != placedDir {
			if where == placedAbsent {
				return placedAbsent, nil, nil
			}
			return placedUnplaced, nil, nil
		}
		key = joinRel(key, element)
	}
	return placedUnplaced, nil, nil
}

// placeIn answers where one name lands in a held directory.
func (s *Snapshot) placeIn(dir *dirNode, name string) (placement, *dirNode, *fileNode) {
	if dir.err != nil {
		return placedUnplaced, nil, nil
	}
	index, listed := dir.byName[name]
	if !listed {
		if dir.folded[asciiLower(name)] {
			return placedUnplaced, nil, nil
		}
		return placedAbsent, nil, nil
	}
	entry := dir.entries[index]
	if entry.IsDir() {
		// A listed directory the snapshot holds no node for is one whose
		// own read failed in a way the build recorded on the parent.
		return placedDir, nil, nil
	}
	if file, held := dir.files[name]; held {
		if file.err != nil {
			return placedUnplaced, nil, nil
		}
		return placedFile, nil, file
	}
	return placedUnplaced, nil, nil
}

// relative answers a path's key below the root, "." for the root itself,
// and false for a path outside it. The key keeps the platform's separators,
// so a lookup allocates nothing.
func (s *Snapshot) relative(path string) (string, bool) {
	if path == s.root {
		return ".", true
	}
	if !strings.HasPrefix(path, s.prefix) {
		return "", false
	}
	rest := path[len(s.prefix):]
	if rest == "" || !cleanRest(rest) {
		return "", false
	}
	return rest, true
}

// cleanRest reports whether the part of a path below the root is clean: no
// empty element, no "." or ".." element and no trailing separator, which is
// what filepath.Join always answers. A path that is not clean is answered by
// the disk, so the snapshot never decides what an unclean spelling names.
func cleanRest(rest string) bool {
	start := 0
	for i := 0; i <= len(rest); i++ {
		if i < len(rest) && !os.IsPathSeparator(rest[i]) {
			continue
		}
		element := rest[start:i]
		if element == "" || element == "." || element == ".." {
			return false
		}
		start = i + 1
	}
	return true
}

// splitRel splits a relative key into its parent's key and its last element.
func splitRel(rel string) (string, string) {
	i := strings.LastIndexByte(rel, filepath.Separator)
	if i < 0 {
		return ".", rel
	}
	return rel[:i], rel[i+1:]
}

// joinRel joins a relative key and one element.
func joinRel(key, element string) string {
	if key == "." {
		return element
	}
	return key + string(filepath.Separator) + element
}

// asciiLower lowercases the ASCII letters of a name and leaves every other
// byte alone, which is the comparison the snapshot uses to decide that a
// name differs from a held one by case only.
func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// notExist is the error a read of an absent path answers.
func notExist(op, path string) error {
	return &fs.PathError{Op: op, Path: path, Err: fs.ErrNotExist}
}

// passThrough answers a read the snapshot cannot, by calling the hook and
// reading the disk.
func (s *Snapshot) passThrough(path string) bench.Disk {
	if s.hooks != nil && s.hooks.PassThrough != nil {
		s.hooks.PassThrough(path)
	}
	return bench.Disk{}
}

// ReadFile is os.ReadFile, answered from the held bytes.
func (s *Snapshot) ReadFile(path string) ([]byte, error) {
	where, _, file := s.place(path)
	switch where {
	case placedOutside:
		return bench.Disk{}.ReadFile(path)
	case placedFile:
		if !file.partial {
			return append([]byte(nil), file.data...), nil
		}
	case placedAbsent:
		return nil, notExist("open", path)
	case placedDir:
		return nil, readFileOfDirError(path)
	}
	return s.passThrough(path).ReadFile(path)
}

// ReadHead is at most n bytes from the start of a file, answered from the
// held bytes when they reach n or the file is no longer than they are.
func (s *Snapshot) ReadHead(path string, n int) ([]byte, error) {
	where, _, file := s.place(path)
	switch where {
	case placedOutside:
		return bench.Disk{}.ReadHead(path, n)
	case placedFile:
		if len(file.data) >= n || file.info.Size() <= int64(len(file.data)) {
			head := file.data
			if len(head) > n {
				head = head[:n]
			}
			return append([]byte(nil), head...), nil
		}
	case placedAbsent:
		return nil, notExist("open", path)
	}
	return s.passThrough(path).ReadHead(path, n)
}

// ReadDir is os.ReadDir, answered from the held listing.
func (s *Snapshot) ReadDir(dir string) ([]fs.DirEntry, error) {
	where, node, _ := s.place(dir)
	switch where {
	case placedOutside:
		return bench.Disk{}.ReadDir(dir)
	case placedDir:
		if node != nil {
			return append([]fs.DirEntry(nil), node.entries...), nil
		}
	case placedFile:
		return nil, readDirOfFileError(dir)
	case placedAbsent:
		return nil, notExist("open", dir)
	}
	return s.passThrough(dir).ReadDir(dir)
}

// Stat is os.Stat, answered from the held info.
func (s *Snapshot) Stat(path string) (fs.FileInfo, error) {
	where, node, file := s.place(path)
	switch where {
	case placedOutside:
		return bench.Disk{}.Stat(path)
	case placedDir:
		if node != nil {
			return node.info, nil
		}
	case placedFile:
		return file.info, nil
	case placedAbsent:
		return nil, notExist("stat", path)
	}
	return s.passThrough(path).Stat(path)
}

// Text is the file's normalised text and the revision of its stored bytes,
// computed once per held file.
func (s *Snapshot) Text(path string) (string, string, error) {
	where, _, file := s.place(path)
	switch where {
	case placedOutside:
		return bench.Disk{}.Text(path)
	case placedFile:
		if !file.partial {
			text, revision := file.textAndRevision()
			return text, revision, nil
		}
	case placedAbsent:
		return "", "", notExist("open", path)
	case placedDir:
		return "", "", readFileOfDirError(path)
	}
	return s.passThrough(path).Text(path)
}

// Derive answers derive applied to the file's text and revision, memoised
// per held file and kind.
func (s *Snapshot) Derive(path string, kind bench.DeriveKind, derive func(path, text, revision string) (any, error)) (any, error) {
	where, _, file := s.place(path)
	slot, known := kindIndex[kind]
	if where == placedFile && !file.partial && known {
		memo := &file.derived[slot]
		memo.once.Do(func() {
			text, revision := file.textAndRevision()
			memo.value, memo.err = derive(path, text, revision)
		})
		return memo.value, memo.err
	}
	text, revision, err := s.Text(path)
	if err != nil {
		return nil, err
	}
	return derive(path, text, revision)
}

// textAndRevision is the node's memoised Text.
func (f *fileNode) textAndRevision() (string, string) {
	f.textOnce.Do(func() {
		f.text, f.revision = bench.TextAndRevisionOf(f.data)
	})
	return f.text, f.revision
}

// Held answers how many files the snapshot holds and how many bytes.
func (s *Snapshot) Held() (int, int64) {
	return s.files, s.bytes
}

// readDirOfFileError replays the error os.ReadDir answers for a regular
// file, taken once per process from a real file in a temporary directory,
// so the snapshot never invents the platform's spelling of it.
func readDirOfFileError(path string) error {
	fileErrors.once.Do(probeFileErrors)
	return withPath(fileErrors.readDir, path)
}

// readFileOfDirError replays the error os.ReadFile answers for a directory.
func readFileOfDirError(path string) error {
	fileErrors.once.Do(probeFileErrors)
	return withPath(fileErrors.readFile, path)
}

// fileErrors holds the two probed errors.
var fileErrors struct {
	once     sync.Once
	readDir  error
	readFile error
}

// probeFileErrors takes the two errors from real reads.
func probeFileErrors() {
	dir, err := os.MkdirTemp("", "dinah-resident-probe-")
	if err != nil {
		fileErrors.readDir = errors.New("resident: readdir of a file")
		fileErrors.readFile = errors.New("resident: read of a directory")
		return
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err == nil {
		_, fileErrors.readDir = os.ReadDir(file)
	}
	_, fileErrors.readFile = os.ReadFile(dir)
	if fileErrors.readDir == nil {
		fileErrors.readDir = errors.New("resident: readdir of a file")
	}
	if fileErrors.readFile == nil {
		fileErrors.readFile = errors.New("resident: read of a directory")
	}
}

// withPath answers a probed error carrying another path.
func withPath(err error, path string) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return &fs.PathError{Op: pathErr.Op, Path: path, Err: pathErr.Err}
	}
	return err
}

// sortedEntries sorts entries by name, which is the order os.ReadDir answers.
func sortedEntries(entries []fs.DirEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
}
