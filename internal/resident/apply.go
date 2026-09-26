package resident

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dinah/internal/bench"
)

// relativeTo answers a clean absolute path's key below root: "." for the
// root itself, the rest of the path with the platform's separators, and
// false for a path outside the root.
func relativeTo(root, path string) (string, bool) {
	if path == root {
		return ".", true
	}
	prefix := root + string(filepath.Separator)
	if !strings.HasPrefix(path, prefix) || len(path) == len(prefix) {
		return "", false
	}
	return path[len(prefix):], true
}

// parentOf answers a relative key's parent key.
func parentOf(rel string) string {
	parent, _ := splitRel(rel)
	return parent
}

// below reports whether rel is at or below dir.
func below(rel, dir string) bool {
	if dir == "." || rel == dir {
		return true
	}
	return strings.HasPrefix(rel, dir+string(filepath.Separator))
}

// passWork is what one pass reconciles, coalesced by resolved path.
type passWork struct {
	// deep are the directories read whole, with their parents re-listed.
	deep map[string]bool
	// shallow are the directories re-listed.
	shallow map[string]bool
	// files are the regular files read again, each with whether a structural
	// action named it, which re-lists its parent too.
	files map[string]bool
}

// reconcile applies one batch of changes and the directories of the Settle
// requests a pass took to the directory map base, and answers the new map and
// the paths it reconciled, slash separated. base is never written: the answer
// is a copy with only the touched nodes replaced.
//
// Every row reads the disk as it stands when the batch is applied, so the
// result depends neither on the order the changes arrived in nor on how many
// changes one write produced.
func reconcile(root string, base map[string]*dirNode, changes []Change, settles []*settleRequest, longForm func(string) (string, error)) (map[string]*dirNode, []string) {
	b := &builder{root: root, dirs: make(map[string]*dirNode, len(base))}
	for key, node := range base {
		b.dirs[key] = node
	}
	work := &passWork{deep: map[string]bool{}, shallow: map[string]bool{}, files: map[string]bool{}}
	for _, change := range changes {
		b.classify(change, work, longForm)
	}
	for _, request := range settles {
		for _, dir := range request.dirs {
			work.deep[dir] = true
		}
		work.shallow["."] = true
	}
	return b.run(work)
}

// classify resolves one change against the held tree and records the work it
// calls for.
func (b *builder) classify(change Change, work *passWork, longForm func(string) (string, error)) {
	rel := filepath.Clean(change.Path)
	if rel == "." || rel == "" || !filepath.IsLocal(rel) {
		work.shallow["."] = true
		return
	}
	resolved, unplaced := b.resolve(rel, longForm)
	for _, dir := range unplaced {
		work.shallow[dir] = true
	}
	for _, path := range resolved {
		b.classifyResolved(path, change.Action, work)
	}
}

// resolve walks the held tree from the root one element at a time, matching
// each element exactly, then ASCII-case-insensitively, taking every held
// entry that matches. An element matching nothing is looked up by its long
// form; when that matches nothing either, the change becomes a shallow
// reconcile of the last directory the walk reached. It answers the held paths
// the change resolved to and the directories it could not go past.
func (b *builder) resolve(rel string, longForm func(string) (string, error)) (resolved, unplaced []string) {
	elements := strings.Split(rel, string(filepath.Separator))
	current := []string{"."}
	for i, element := range elements {
		last := i == len(elements)-1
		var next []string
		for _, dirKey := range current {
			dir, held := b.dirs[dirKey]
			if !held || dir.err != nil {
				unplaced = append(unplaced, dirKey)
				continue
			}
			matches := matchesIn(dir, element)
			if len(matches) == 0 {
				if long, err := longForm(filepath.Join(b.abs(dirKey), element)); err == nil {
					if name := filepath.Base(long); name != element {
						if _, listed := dir.byName[name]; listed {
							matches = []string{name}
						}
					}
				}
			}
			if len(matches) == 0 {
				unplaced = append(unplaced, dirKey)
				continue
			}
			for _, name := range matches {
				key := joinRel(dirKey, name)
				if last {
					resolved = append(resolved, key)
					continue
				}
				entry := dir.entries[dir.byName[name]]
				if !entry.IsDir() {
					// A walk through a file: the directory holding it is
					// what changed shape.
					unplaced = append(unplaced, dirKey)
					continue
				}
				next = append(next, key)
			}
		}
		current = next
	}
	return resolved, unplaced
}

// matchesIn answers the held entries of dir a name matches: itself when held
// exactly, and otherwise every entry equal to it under ASCII case folding.
func matchesIn(dir *dirNode, name string) []string {
	if _, listed := dir.byName[name]; listed {
		return []string{name}
	}
	folded := asciiLower(name)
	if !dir.folded[folded] {
		return nil
	}
	var matches []string
	for _, entry := range dir.entries {
		if asciiLower(entry.Name()) == folded {
			matches = append(matches, entry.Name())
		}
	}
	return matches
}

// classifyResolved records the work for one resolved path, by the first row
// of the reconcile table that fits what the path is on disk now.
func (b *builder) classifyResolved(rel string, action Action, work *passWork) {
	info, err := os.Stat(b.abs(rel))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// Absent: its node and subtree go with the parent's re-list.
		work.shallow[parentOf(rel)] = true
	case err != nil:
		// A stat that fails otherwise is settled by reading the parent
		// again, which records the failure on the node it reads.
		work.shallow[parentOf(rel)] = true
	case info.Mode().IsRegular():
		structural := action == Added || action == Removed || action == RenamedOld || action == RenamedNew
		work.files[rel] = work.files[rel] || structural
	case info.IsDir():
		held, isHeldDir := b.dirs[rel]
		if action == Modified && isHeldDir && held != nil && held.err == nil {
			work.shallow[rel] = true
		} else {
			work.deep[rel] = true
		}
	default:
		work.shallow[parentOf(rel)] = true
	}
}

// run carries out a pass's work in the order that keeps each step's reads
// current: the deep directories, then the files, then the re-lists, deepest
// first, each of which keeps every node an earlier step just read.
func (b *builder) run(work *passWork) (map[string]*dirNode, []string) {
	var paths []string
	deep := outermost(work.deep)
	for _, dir := range deep {
		b.readDeep(dir)
		paths = append(paths, dir)
		if dir != "." {
			work.shallow[parentOf(dir)] = true
		}
	}
	for _, file := range sortedKeys(work.files) {
		if underAny(file, deep) {
			continue
		}
		paths = append(paths, file)
		structural := work.files[file]
		if !b.refreshFile(file) || structural {
			work.shallow[parentOf(file)] = true
		}
	}
	shallow := sortedKeys(work.shallow)
	for i := len(shallow) - 1; i >= 0; i-- {
		dir := shallow[i]
		if underAny(dir, deep) {
			continue
		}
		paths = append(paths, dir)
		paths = append(paths, b.relist(dir)...)
	}
	sort.Strings(paths)
	slashed := make([]string, 0, len(paths))
	for i, path := range paths {
		if i > 0 && paths[i-1] == path {
			continue
		}
		slashed = append(slashed, filepath.ToSlash(path))
	}
	return b.dirs, slashed
}

// readDeep reads a directory's whole subtree again, dropping what it held.
func (b *builder) readDeep(rel string) {
	b.dropTree(rel)
	b.readTree(rel, b.isPayloadDir(rel))
}

// refreshFile reads one regular file again into its parent's node, updating
// the parent's entry for it from the fresh stat. It reports false when the
// parent's listing does not carry the file, or the file is no longer a
// regular file, which leaves the parent to be re-listed.
func (b *builder) refreshFile(rel string) bool {
	parentKey, name := splitRel(rel)
	parent, held := b.dirs[parentKey]
	if !held || parent.err != nil {
		return false
	}
	index, listed := parent.byName[name]
	if !listed || !parent.entries[index].Type().IsRegular() {
		return false
	}
	file := readFile(b.abs(rel), b.isPayloadDir(parentKey))
	if file == nil {
		return false
	}
	updated := parent.copy()
	updated.files[name] = file
	updated.entries[index] = &dirEntry{name: name, typ: parent.entries[index].Type(), info: file.info, infoErr: file.err}
	b.dirs[parentKey] = updated
	return true
}

// relist stats and lists a directory again. A name gone from it is dropped
// with its subtree, a name new to it is read, deep for a directory, and a
// name in both keeps the node it already has in the map, which is the one an
// earlier step of the pass read when the batch named it. It answers the
// paths of the names it read or dropped, so a publish names a file a re-list
// found as well as the directory it re-listed.
func (b *builder) relist(rel string) []string {
	var touched []string
	old, wasHeld := b.dirs[rel]
	now := b.readListing(rel)
	if now == nil {
		b.dropTree(rel)
		if rel != "." {
			// The directory is gone, so its parent's listing changed too.
			parentKey, name := splitRel(rel)
			if parent, ok := b.dirs[parentKey]; ok && parent.err == nil {
				if _, listed := parent.byName[name]; listed {
					touched = append(touched, parentKey)
					touched = append(touched, b.relist(parentKey)...)
				}
			}
		}
		return touched
	}
	if now.err != nil || !wasHeld || old.err != nil {
		b.dropTree(rel)
		b.readTree(rel, b.isPayloadDir(rel))
		return touched
	}
	path := b.abs(rel)
	payloads := b.isPayloadDir(rel)
	holdsAttachment := false
	for _, entry := range now.listed {
		if entry.Name() == bench.AttachmentAnchor {
			holdsAttachment = true
		}
	}
	node := &dirNode{
		info:   now.info,
		byName: make(map[string]int, len(now.listed)),
		folded: make(map[string]bool, len(now.listed)),
		files:  map[string]*fileNode{},
	}
	listedNow := make(map[string]bool, len(now.listed))
	for _, entry := range now.listed {
		name := entry.Name()
		listedNow[name] = true
		kept := keptEntry(b, old, rel, entry, node)
		if kept == nil {
			touched = append(touched, joinRel(rel, name))
			if entry.IsDir() {
				b.dropTree(joinRel(rel, name))
				if b.readTree(joinRel(rel, name), holdsAttachment && name == bench.PayloadDir) == nil {
					continue
				}
			}
			read, keep := b.entryOf(path, entry, payloads, node)
			if !keep {
				continue
			}
			kept = read
		}
		node.byName[name] = len(node.entries)
		node.folded[asciiLower(name)] = true
		node.entries = append(node.entries, kept)
	}
	for _, entry := range old.entries {
		if listedNow[entry.Name()] {
			continue
		}
		touched = append(touched, joinRel(rel, entry.Name()))
		if entry.IsDir() {
			b.dropTree(joinRel(rel, entry.Name()))
		}
	}
	b.dirs[rel] = node
	return touched
}

// keptEntry answers the entry a re-list keeps for a name the old listing
// carried with the same type, filling the new node's file map, or nil when
// the name has to be read.
func keptEntry(b *builder, old *dirNode, rel string, entry fs.DirEntry, node *dirNode) *dirEntry {
	name := entry.Name()
	index, was := old.byName[name]
	if !was || old.entries[index].Type().Type() != entry.Type().Type() {
		return nil
	}
	previous := old.entries[index].(*dirEntry)
	switch {
	case entry.IsDir():
		child, ok := b.dirs[joinRel(rel, name)]
		if !ok || child.err != nil {
			return nil
		}
		return &dirEntry{name: name, typ: previous.typ, info: child.info}
	case entry.Type().IsRegular():
		file, ok := old.files[name]
		if !ok {
			return nil
		}
		node.files[name] = file
		return previous
	}
	return previous
}

// listing is a directory's fresh stat and listing, before any file in it is
// read.
type listing struct {
	info   fs.FileInfo
	listed []fs.DirEntry
	err    error
}

// readListing stats and lists one directory through one handle, answering nil
// when it is gone.
func (b *builder) readListing(rel string) *listing {
	path := b.abs(rel)
	info, listed, err := listDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &listing{err: err}
	}
	if !info.IsDir() {
		return nil
	}
	return &listing{
		info:   &entryInfo{name: info.Name(), size: info.Size(), mode: info.Mode(), modTime: info.ModTime()},
		listed: listed,
	}
}

// isPayloadDir reports whether a directory holds attachment payloads: it is
// named bench.PayloadDir and its parent holds bench.AttachmentAnchor, read
// off the disk as the reconcile reads everything else.
func (b *builder) isPayloadDir(rel string) bool {
	if rel == "." {
		return false
	}
	parentKey, name := splitRel(rel)
	if name != bench.PayloadDir {
		return false
	}
	info, err := os.Stat(filepath.Join(b.abs(parentKey), bench.AttachmentAnchor))
	return err == nil && info.Mode().IsRegular()
}

// copy answers a copy of a node whose maps and entries its holder may change.
func (d *dirNode) copy() *dirNode {
	copied := &dirNode{
		info:    d.info,
		entries: append([]fs.DirEntry(nil), d.entries...),
		byName:  make(map[string]int, len(d.byName)),
		folded:  make(map[string]bool, len(d.folded)),
		files:   make(map[string]*fileNode, len(d.files)),
		err:     d.err,
	}
	for k, v := range d.byName {
		copied.byName[k] = v
	}
	for k, v := range d.folded {
		copied.folded[k] = v
	}
	for k, v := range d.files {
		copied.files[k] = v
	}
	return copied
}

// outermost answers the directories of a set that no other member contains,
// sorted.
func outermost(set map[string]bool) []string {
	keys := sortedKeys(set)
	var kept []string
	for _, key := range keys {
		if underAny(key, kept) {
			continue
		}
		kept = append(kept, key)
	}
	return kept
}

// underAny reports whether rel is at or below any of dirs.
func underAny(rel string, dirs []string) bool {
	for _, dir := range dirs {
		if below(rel, dir) {
			return true
		}
	}
	return false
}

// contains reports whether a sorted list holds a key.
func contains(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

// sortedKeys answers a set's keys in order.
func sortedKeys[V any](set map[string]V) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
