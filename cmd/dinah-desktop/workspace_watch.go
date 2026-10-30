package main

import (
	"path/filepath"
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

// watchQuiet is how long a watched folder must be quiet before its change is
// sent, so that a program writing many files causes one refresh.
const watchQuiet = 300 * time.Millisecond

type folderWatch struct {
	mu      sync.Mutex
	watcher *fsnotify.Watcher
	// dirs are the watched folders, by their path on disk.
	dirs    map[string]ResourceRef
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
		a.folderWatch = &folderWatch{send: func(workspaceID, sourceID string) {
			a.emitWorkspaceMountChanged("disk-changed", workspaceID, sourceID)
		}}
	}
	w := a.folderWatch
	a.mu.Unlock()
	return w.set(dirs)
}

func (w *folderWatch) set(dirs map[string]ResourceRef) error {
	w.mu.Lock()
	defer w.mu.Unlock()
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
