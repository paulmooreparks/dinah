package resident

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// builder reads directories from disk into held nodes. A build reads every
// file and trusts nothing it held before, so no modification time, size or
// identity decides whether a file is read again, and a reconcile reads the
// paths it names in the same way.
type builder struct {
	root string
	// dirs is the directory map being filled.
	dirs map[string]*dirNode
}

// readTree reads the directory at rel and everything below it into b.dirs,
// and answers the directory's node, or nil when it no longer exists.
// payloads marks a directory whose regular files are attachment payloads.
func (b *builder) readTree(rel string, payloads bool) *dirNode {
	node := b.readDir(rel, payloads)
	if node == nil {
		return nil
	}
	b.dirs[rel] = node
	if node.err != nil {
		return node
	}
	holdsAttachment := false
	if _, ok := node.byName[bench.AttachmentAnchor]; ok {
		holdsAttachment = true
	}
	for _, entry := range node.entries {
		if !entry.IsDir() {
			continue
		}
		b.readTree(joinRel(rel, entry.Name()), holdsAttachment && entry.Name() == bench.PayloadDir)
	}
	return node
}

// dropTree removes the node at rel and every node below it.
func (b *builder) dropTree(rel string) {
	if rel == "." {
		for key := range b.dirs {
			delete(b.dirs, key)
		}
		return
	}
	prefix := rel + string(filepath.Separator)
	for key := range b.dirs {
		if key == rel || (len(key) > len(prefix) && key[:len(prefix)] == prefix) {
			delete(b.dirs, key)
		}
	}
}

// abs answers the absolute path of a relative key.
func (b *builder) abs(rel string) string {
	if rel == "." {
		return b.root
	}
	return filepath.Join(b.root, filepath.FromSlash(rel))
}

// readDir stats and lists one directory through one handle, closed before any
// file is read, and reads every regular file in it, but none of its
// subdirectories. It answers nil when the directory is gone.
func (b *builder) readDir(rel string, payloads bool) *dirNode {
	path := b.abs(rel)
	info, listed, err := listDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &dirNode{err: err}
	}
	if !info.IsDir() {
		return nil
	}
	node := &dirNode{
		info:   &entryInfo{name: info.Name(), size: info.Size(), mode: info.Mode(), modTime: info.ModTime()},
		byName: make(map[string]int, len(listed)),
		folded: make(map[string]bool, len(listed)),
		files:  map[string]*fileNode{},
	}
	for _, entry := range listed {
		held, keep := b.entryOf(path, entry, payloads, node)
		if !keep {
			continue
		}
		node.byName[held.Name()] = len(node.entries)
		node.folded[asciiLower(held.Name())] = true
		node.entries = append(node.entries, held)
	}
	return node
}

// entryOf reads one listed entry. A regular file is stat-ed and read, the
// head alone for a payload, and a directory is stat-ed; the entry carries the
// info the snapshot will answer for it. An entry that has vanished since the
// listing is dropped.
func (b *builder) entryOf(dir string, entry fs.DirEntry, payloads bool, node *dirNode) (*dirEntry, bool) {
	name := entry.Name()
	path := filepath.Join(dir, name)
	held := &dirEntry{name: name, typ: entry.Type()}
	switch {
	case entry.IsDir():
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false
		}
		if err != nil {
			held.infoErr = err
			return held, true
		}
		held.info = &entryInfo{name: name, size: info.Size(), mode: info.Mode(), modTime: info.ModTime()}
		return held, true
	case entry.Type().IsRegular():
		file := readFile(path, payloads)
		if file == nil {
			return nil, false
		}
		node.files[name] = file
		held.info, held.infoErr = file.info, file.err
		return held, true
	default:
		info, err := entry.Info()
		held.info, held.infoErr = info, err
		return held, true
	}
}

// readFile stats and reads one regular file, the head alone for a payload,
// through one handle that shares delete, so the stat and the bytes come from
// the same open and the file is held open once, for the length of the read.
// It answers nil when the file has vanished.
func readFile(path string, payload bool) *fileNode {
	file, err := openShared(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &fileNode{err: err}
	}
	info, data, err := readOpen(file, payload)
	file.Close()
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &fileNode{err: err}
	}
	size := int64(len(data))
	if payload {
		size = info.Size()
	}
	return &fileNode{
		info:    &entryInfo{name: info.Name(), size: size, mode: info.Mode(), modTime: info.ModTime()},
		data:    data,
		partial: payload,
	}
}

// readOpen stats an open file and reads it: the head of a payload, as
// bench.Disk.ReadHead does, or the whole of any other file, as os.ReadFile
// does.
func readOpen(file *os.File, payload bool) (fs.FileInfo, []byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if payload {
		data, err := bench.ReadHeadFrom(file, bench.AttachmentHeadBytes)
		return info, data, err
	}
	data, err := io.ReadAll(file)
	return info, data, err
}

// finish turns a directory map into a snapshot: it counts what is held, opens
// the workbench over it and computes the earliest claim expiry.
func finish(root string, dirs map[string]*dirNode, gen uint64, hooks *Hooks) *Snapshot {
	s := &Snapshot{root: root, prefix: root + string(filepath.Separator), dirs: dirs, files: map[string]*fileNode{}, gen: gen, hooks: hooks}
	for key, dir := range dirs {
		for name, file := range dir.files {
			s.files[joinRel(key, name)] = file
			if file.err == nil {
				s.held++
				s.bytes += int64(len(file.data))
			}
		}
	}
	s.opened, s.openErr = bench.OpenWith(root, s)
	if s.openErr == nil {
		s.lapsing = expiries(s.opened)
		if len(s.lapsing) > 0 {
			s.earliestExpiry = s.lapsing[0].at
		}
	}
	return s
}

// expiries answers every card in the live and archived card collections
// whose state is active and whose claim expiry parses, earliest first.
func expiries(b *bench.Bench) []expiring {
	var found []expiring
	for _, collection := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
		ids, err := b.ListIDs(collection)
		if err != nil {
			continue
		}
		for _, id := range ids {
			card, err := b.LoadCardIn(collection, id)
			if err != nil || card.State != contract.StateActive || card.Expires == "" {
				continue
			}
			at := bench.ParseStamp(card.Expires)
			if at.IsZero() {
				continue
			}
			found = append(found, expiring{at: at, dir: card.Dir})
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].at.Before(found[j].at) })
	return found
}

// buildAll reads the whole tree below root.
func buildAll(root string) map[string]*dirNode {
	b := &builder{root: root, dirs: map[string]*dirNode{}}
	b.readTree(".", false)
	return b.dirs
}
