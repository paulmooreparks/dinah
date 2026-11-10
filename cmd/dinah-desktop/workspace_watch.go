package main

import (
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// The workspace views refresh when their folders change on disk, however the
// change was made: by an agent, a flow, another program or a sync client. The
// page says which folders it shows (WatchFolders), and only those are watched,
// each on its own, through fsnotify, which uses each system's documented
// notifications (ReadDirectoryChangesW on Windows, inotify on Linux, kqueue on
// macOS). Watching only what is shown keeps clear of inotify's per-user limit
// and kqueue's open file per watched entry, and a folder that is not shown is
// listed afresh when it is opened. A burst of changes is sent as one
// workspace:changed event, of kind disk-changed, per source, once the folder
// has been quiet for a moment.
//
// A document can show a file it refers to, as a deck shows a saved diagram,
// and that file's folder need not be one the page shows. So the folder of a
// file read for a document is watched as well (watchReadFolder), and the
// document reads the file again when the event arrives. The read does not
// say which document asked or when that document closes, so the most
// recently read folders are watched, readFolderLimit of them, and a folder
// leaves when that many others have been read since.

// watchQuiet is how long a watched folder must be quiet before its change is
// sent, so that a program writing many files causes one refresh.
const watchQuiet = 300 * time.Millisecond

// readFolderLimit is how many folders of files that documents refer to are
// watched at once.
const readFolderLimit = 32

type folderWatch struct {
	mu      sync.Mutex
	watcher *fsnotify.Watcher
	// dirs are the watched folders, by their path on disk: the folders the
	// page shows and the folders of files read for documents.
	dirs map[string]ResourceRef
	// shown are the folders the page shows, and reads the folders of files
	// read for documents, the most recently read last.
	shown   map[string]ResourceRef
	reads   []readFolder
	pending map[[2]string]bool
	timer   *time.Timer
	send    func(workspaceID, sourceID string)
}

// WatchFolders replaces the folders watched for changes on disk with these.
// A folder that is not in a local source, or cannot be opened, is left out.
func (a *App) WatchFolders(refs []ResourceRef) error {
	vfs := workspaceVFS{service: a.resourceService()}
	dirs := map[string]ResourceRef{}
	for _, ref := range refs {
		if path, dir, err := vfs.localPath(ref); err == nil && dir {
			dirs[path] = ref
		}
	}
	a.mu.Lock()
	if a.folderWatch == nil {
		a.folderWatch = a.newFolderWatch()
	}
	w := a.folderWatch
	a.mu.Unlock()
	return w.show(dirs)
}

// watchReadFolder watches the folder of a file that was read for a document
// that refers to it, so that the document learns when the file changes. A
// file that is not in a local folder is not watched.
func (a *App) watchReadFolder(file ResourceRef) {
	folder := file
	folder.Path = pathpkg.Dir(file.Path)
	if folder.Path == "." {
		folder.Path = ""
	}
	path, dir, err := workspaceVFS{service: a.resourceService()}.localPath(folder)
	if err != nil || !dir {
		return
	}
	a.mu.Lock()
	if a.folderWatch == nil {
		a.folderWatch = a.newFolderWatch()
	}
	w := a.folderWatch
	a.mu.Unlock()
	_ = w.read(path, folder)
}

func (a *App) newFolderWatch() *folderWatch {
	return &folderWatch{send: func(workspaceID, sourceID string) {
		a.emitWorkspaceMountChanged("disk-changed", workspaceID, sourceID)
	}}
}

type readFolder struct {
	path string
	ref  ResourceRef
}

// show replaces the folders the page shows.
func (w *folderWatch) show(dirs map[string]ResourceRef) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.shown = dirs
	return w.apply()
}

// read adds the folder of a file read for a document, or moves it to the
// end when it is already there, and drops the folder read longest ago when
// there are more than readFolderLimit.
func (w *folderWatch) read(path string, ref ResourceRef) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if n := len(w.reads); n > 0 && w.reads[n-1].path == path {
		return nil
	}
	w.reads = slices.DeleteFunc(w.reads, func(r readFolder) bool { return r.path == path })
	w.reads = append(w.reads, readFolder{path: path, ref: ref})
	if over := len(w.reads) - readFolderLimit; over > 0 {
		w.reads = slices.Clone(w.reads[over:])
	}
	return w.apply()
}

// apply watches the folders shown and the folders read, and stops watching
// every other. It is called with mu held.
func (w *folderWatch) apply() error {
	dirs := map[string]ResourceRef{}
	for _, r := range w.reads {
		dirs[r.path] = r.ref
	}
	for path, ref := range w.shown {
		dirs[path] = ref
	}
	if w.watcher == nil {
		if len(dirs) == 0 {
			return nil
		}
		fw, err := fsnotify.NewWatcher()
		if err != nil {
			return err
		}
		w.watcher, w.dirs, w.pending = fw, map[string]ResourceRef{}, map[[2]string]bool{}
		go w.run(fw)
	}
	for path := range w.dirs {
		if _, keep := dirs[path]; !keep {
			_ = w.watcher.Remove(path)
			delete(w.dirs, path)
		}
	}
	for path, ref := range dirs {
		if _, have := w.dirs[path]; have {
			w.dirs[path] = ref
			continue
		}
		// A folder that cannot be watched, such as one just deleted, is
		// still listed when it is shown; it is simply not refreshed.
		if err := w.watcher.Add(path); err == nil {
			w.dirs[path] = ref
		}
	}
	return nil
}

func (w *folderWatch) run(fw *fsnotify.Watcher) {
	for {
		select {
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			w.changed(ev.Name)
		case _, ok := <-fw.Errors:
			if !ok {
				return
			}
			// An error, such as the system dropping events when too many
			// arrive at once, may hide a change, so every watched folder is
			// listed again.
			w.changedAll()
		}
	}
}

// changed records a change to an entry of a watched folder, or to the folder
// itself, and sends it once the folders are quiet.
func (w *folderWatch) changed(name string) {
	if strings.EqualFold(filepath.Base(name), ".git") || strings.Contains(strings.ToLower(filepath.ToSlash(name)), "/.git/") {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, path := range []string{filepath.Dir(name), name} {
		if ref, ok := w.dirs[path]; ok {
			w.pending[[2]string{ref.WorkspaceID, ref.SourceID}] = true
		}
	}
	w.schedule()
}

func (w *folderWatch) changedAll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, ref := range w.dirs {
		w.pending[[2]string{ref.WorkspaceID, ref.SourceID}] = true
	}
	w.schedule()
}

// schedule sends the pending changes once no change has come for watchQuiet.
// It is called with mu held.
func (w *folderWatch) schedule() {
	if len(w.pending) == 0 {
		return
	}
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(watchQuiet, func() {
		w.mu.Lock()
		pending := w.pending
		w.pending = map[[2]string]bool{}
		w.mu.Unlock()
		for key := range pending {
			w.send(key[0], key[1])
		}
	})
}

func (w *folderWatch) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timer != nil {
		w.timer.Stop()
	}
	if w.watcher != nil {
		_ = w.watcher.Close()
		w.watcher, w.dirs = nil, nil
	}
}
