package main

import (
	"errors"
	"sync"

	"studio/internal/bus"
)

// A folder is closed when its app has ended: later writes into it fail rather than recreate it. A goroutine
// that outlives the app (a turn that is slow to end, a late judgement) then fails harmlessly instead of writing
// into a folder that is being removed, as a test's is, or that another run now owns. It is the same rule as the
// log's closedDir, for the other files the app keeps.
var closedFolders = struct {
	sync.Mutex
	dirs map[string]bool
}{dirs: map[string]bool{}}

// errFolderClosed is what a write into a closed folder says.
var errFolderClosed = errors.New("the app has ended, so nothing more is written to its folder")

// currentFolder is configDir without creating the folder.
func currentFolder() string {
	if configOverride != "" {
		return configOverride
	}
	if applicationStateDir != "" {
		return applicationStateDir
	}
	home, err := bus.Home()
	if err != nil {
		return ""
	}
	return home
}

// openFolder lets the current folder be written, as an app starts in it.
func openFolder() {
	closedFolders.Lock()
	delete(closedFolders.dirs, currentFolder())
	closedFolders.Unlock()
}

// closeFolder refuses further writes into the current folder, as the app that owns it ends.
func closeFolder() {
	closedFolders.Lock()
	closedFolders.dirs[currentFolder()] = true
	closedFolders.Unlock()
}

// folderClosed says the current folder belongs to an app that has ended.
func folderClosed() bool {
	closedFolders.Lock()
	defer closedFolders.Unlock()
	return closedFolders.dirs[currentFolder()]
}
