package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/bus"
	"studio/internal/layout"
	"studio/internal/profile"
)

// Restart to Update. `dinah install` puts a new build at the installed
// path while the window runs, and the window, when it was started from that
// path, notices the new file and offers to restart. Restarting holds new
// turns (the person's prompts stay in their boxes, and bus messages wait on
// the bus) and new writes of a document to where it is kept, waits for
// every running turn and every such write to end, closes the window the
// usual way, and starts the new build, which waits for this process to end
// before it takes the single-instance lock. The reopened chats carry on
// from their saved sessions.
//
//	update:state  UpdateState  the update's state changed

// UpdateState is what the tray's update item shows.
type UpdateState struct {
	// Ready is set when a newer build is installed.
	Ready bool `json:"ready"`
	// Pending is set while the window waits for turns to end to restart.
	Pending bool `json:"pending"`
	// Working is how many chats are in a turn.
	Working int `json:"working"`
	// Writing is how many documents are being saved or published.
	Writing int `json:"writing"`
	// Pausing is set when the waiting restart asked the working chats to
	// pause, and Blockers are the chats a waiting restart still waits for,
	// by their titles.
	Pausing  bool     `json:"pausing"`
	Blockers []string `json:"blockers"`
	// Overdue is set when chats asked to pause are still working after
	// pauseGrace.
	Overdue bool `json:"overdue"`
	// StateGen and StateVer order the snapshots of this state the page is
	// given (internal/sessionhost/state_events.go).
	StateGen string `json:"stateGen,omitempty"`
	StateVer uint64 `json:"stateVer,omitempty"`
}

// updateWatch is the installed build the window was started from.
type updateWatch struct {
	path  string // the installed program
	hash  [32]byte
	size  int64
	mod   time.Time
	ready bool
	// pending is set while the window waits to restart, and restarting
	// once the window has agreed to close.
	pending, restarting bool
	// pausing is set when the waiting restart asked the working chats to
	// pause, and telling for the moment in which they are asked, when a
	// steering message is let through although turns are held
	// (restart_pause.go).
	pausing, telling bool
	// pausedAt is when the chats were asked to pause.
	pausedAt time.Time
}

// pauseGrace is how long the chats asked to pause are given before the
// ones still working are shown as what the restart waits for, with the
// choice to restart anyway or to cancel (Paul, 14 November 2026).
const pauseGrace = 2 * time.Minute

// admitWrite is asked before a document is written to where it is kept: a
// file saved, a page published, a source's pictures drawn again. It refuses
// while the window waits to restart, and otherwise counts the write until
// the returned function is called. The refusal and the count are decided
// under one lock, so a write is either refused or waited for.
func (a *App) admitWrite() (func(), error) {
	a.updMu.Lock()
	defer a.updMu.Unlock()
	if a.update != nil && a.update.pending {
		return nil, errors.New(product.Name + " is waiting to restart for its update, so nothing is saved or published until it has restarted. The document keeps its changes; save it after the restart, or cancel the restart")
	}
	a.writing++
	var once sync.Once
	return func() {
		once.Do(func() {
			a.updMu.Lock()
			a.writing--
			a.updMu.Unlock()
		})
	}, nil
}

// updatePoll is how often the installed program is looked at.
const updatePoll = 15 * time.Second

func installedDesktop() (string, error) {
	home, err := bus.Home()
	if err != nil {
		return "", err
	}
	name := product.WindowProgram
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(home, "bin", name), nil
}

func fileHash(path string) ([32]byte, error) {
	var sum [32]byte
	f, err := os.Open(path)
	if err != nil {
		return sum, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return sum, err
	}
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

// watchUpdates records the build the window runs, when it runs the
// installed one, and looks for a newer one until the window closes. A
// window started from anywhere else, such as a test build, offers no
// update, and so does a product whose profile has another updater.
func (a *App) watchUpdates(stop <-chan struct{}) {
	if product.Updater != profile.UpdaterInstalled {
		return
	}
	installed, err := installedDesktop()
	if err != nil {
		return
	}
	self, err := os.Executable()
	if err != nil || !layout.SamePath(self, installed) {
		return
	}
	info, err := os.Stat(installed)
	if err != nil {
		return
	}
	sum, err := fileHash(installed)
	if err != nil {
		return
	}
	a.updMu.Lock()
	a.update = &updateWatch{path: installed, hash: sum, size: info.Size(), mod: info.ModTime()}
	a.updMu.Unlock()
	tick := time.NewTicker(updatePoll)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		a.checkUpdate()
	}
}

// checkUpdate looks at the installed program and says whether it changed.
func (a *App) checkUpdate() {
	a.updMu.Lock()
	w := a.update
	a.updMu.Unlock()
	if w == nil || w.ready {
		return
	}
	info, err := os.Stat(w.path)
	if err != nil || (info.Size() == w.size && info.ModTime().Equal(w.mod)) {
		return
	}
	sum, err := fileHash(w.path)
	if err != nil {
		return
	}
	a.updMu.Lock()
	w.size, w.mod = info.Size(), info.ModTime()
	changed := sum != w.hash
	if changed {
		w.ready = true
	}
	a.updMu.Unlock()
	if changed {
		logf("core", "info", "a newer build of %s is installed", product.Name)
		a.emitUpdate()
	}
}

func (a *App) workingChats() int {
	a.mu.Lock()
	open := make([]*Session, 0, a.runtime.ConversationCountLocked())
	for _, s := range a.runtime.ConversationsLocked() {
		open = append(open, s)
	}
	a.mu.Unlock()
	n := 0
	for _, s := range open {
		// A restart ends background work and subagents with the agent's
		// process, so a chat with some is still working
		// (background_tasks.go, subagents.go).
		state := s.State()
		if state.WorkActivity().Busy() {
			n++
		}
	}
	return n
}

// UpdateState says whether a newer build is installed and whether the
// window is waiting to restart.
func (a *App) UpdateState() UpdateState {
	a.updMu.Lock()
	w := a.update
	st := UpdateState{}
	if w != nil {
		st.Ready, st.Pending, st.Pausing = w.ready, w.pending, w.pending && w.pausing
		st.Overdue = st.Pausing && time.Since(w.pausedAt) >= pauseGrace
	}
	st.Writing = a.writing
	st.StateGen, st.StateVer = a.updateClock().Next()
	a.updMu.Unlock()
	st.Working = a.workingChats()
	st.Blockers = []string{}
	if st.Pending {
		st.Blockers = a.blockers()
	}
	st.Overdue = st.Overdue && len(st.Blockers) > 0
	return st
}

func (a *App) emitUpdate() { a.emit("update:state", a.UpdateState()) }

// holdingTurns refuses a new turn while the window waits to restart.
func (a *App) holdingTurns() error {
	a.updMu.Lock()
	defer a.updMu.Unlock()
	if a.update != nil && a.update.pending && !a.update.telling {
		return errors.New(product.Name + " restarts for its update when the running turns end. Your message is still in the box; send it after the restart")
	}
	return nil
}

// RestartToUpdate restarts into the newer build. With now unset it holds
// new turns and new writes of documents, and waits for the running ones to
// end; with now set it restarts at once, and turns still running end
// unfinished. A write under way when the person chooses that is logged,
// since whether it reached its source is then not known.
func (a *App) RestartToUpdate(now bool) error {
	a.updMu.Lock()
	w := a.update
	if w == nil || !w.ready {
		a.updMu.Unlock()
		return errors.New("no newer build of " + product.Name + " is installed")
	}
	already := w.pending
	w.pending = true
	a.updMu.Unlock()
	a.emitUpdate()
	if now {
		a.updMu.Lock()
		writing := a.writing
		a.updMu.Unlock()
		if writing > 0 {
			logf("core", "warn", "restarting at once with %d document(s) being saved or published; whether each write reached its source is not known, and none is repeated automatically", writing)
		}
		a.closeForUpdate()
		return nil
	}
	if !already {
		go a.restartWhenIdle()
	}
	return nil
}

// CancelRestart stops waiting to restart, and turns may start again.
func (a *App) CancelRestart() {
	a.updMu.Lock()
	// An in-flight pause records who was told before it sends cancellation.
	// Consuming the record here would lose a chat whose steering is settling.
	paused := a.update != nil && a.update.pausing && !a.update.telling
	if a.update != nil {
		a.update.pending, a.update.pausing = false, false
	}
	a.updMu.Unlock()
	a.emitUpdate()
	// The chats that were asked to pause are told to carry on.
	if paused {
		go a.resumePaused(false)
	}
}

func (a *App) restartWhenIdle() {
	last := -1
	for {
		a.updMu.Lock()
		pending := a.update != nil && a.update.pending
		writing := a.writing
		telling := pending && a.update.telling
		overdue := pending && a.update.pausing && time.Since(a.update.pausedAt) >= pauseGrace
		a.updMu.Unlock()
		if !pending {
			return
		}
		// No write starts while pending is set (admitWrite), so a count of
		// none stays none.
		chats := a.workingChats()
		if chats == 0 && writing == 0 && !telling {
			a.closeForUpdate()
			return
		}
		n := chats*1000 + writing
		if overdue {
			n = -2 - n
		}
		if n != last {
			last = n
			a.emitUpdate()
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// closeForUpdate closes the window the usual way, which saves its layout,
// and the new build starts once it has closed (startUpdate). A close the
// page does not finish leaves the window open and the restart off.
func (a *App) closeForUpdate() {
	// The chats still working, and the ones that paused, are told to carry
	// on after the restart (restart_pause.go).
	a.noteInterrupted()
	a.updMu.Lock()
	if a.update != nil {
		a.update.restarting = true
	}
	a.updMu.Unlock()
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}

// restartAbandoned is called when the window stayed open after all.
func (a *App) restartAbandoned() {
	a.updMu.Lock()
	changed := a.update != nil && (a.update.restarting || a.update.pending)
	if a.update != nil {
		a.update.restarting, a.update.pending, a.update.pausing = false, false, false
	}
	a.updMu.Unlock()
	if changed {
		a.emitUpdate()
	}
}

// startUpdate starts the new build as the window shuts down, when the
// window is closing to restart. The new build waits for this process to
// end (--after) before it starts.
func (a *App) startUpdate() {
	a.startUpdateUsing(startDetached)
}

func (a *App) startUpdateUsing(launch func(string, ...string) error) {
	if a.terminalShutdownIncomplete.Load() {
		return
	}
	a.updMu.Lock()
	w := a.update
	restarting := w != nil && w.restarting
	a.updMu.Unlock()
	if !restarting {
		return
	}
	if err := launch(w.path, "--after", strconv.Itoa(os.Getpid())); err != nil {
		logf("core", "error", "starting the new build of %s: %v", product.Name, err)
	}
}

// afterProcessEnds waits for an earlier window to end, as a new build
// started by Restart to Update does before it starts.
func afterProcessEnds(args []string) []string {
	if len(args) >= 3 && args[1] == "--after" {
		if pid, err := strconv.Atoi(args[2]); err == nil {
			// The old window has finished its shutdown when it starts this
			// one, so it usually ends at once. The limit is generous, since
			// starting while it still runs fails on its port, and the log
			// says which happened (logRestartWait).
			began := time.Now()
			if waitForProcess(pid, restartWaitLimit) {
				restartWait = fmt.Sprintf("started by Restart to Update, after the old window ended %s later", time.Since(began).Round(time.Millisecond))
			} else {
				restartWait = fmt.Sprintf("started by Restart to Update while the old window was still running %s later", restartWaitLimit)
			}
		}
		return append([]string{args[0]}, args[3:]...)
	}
	return args
}

// restartWait says how a build started by Restart to Update came to start,
// or is empty for any other start. It is logged once the settings, which
// choose what the log keeps, have been read.
var restartWait string

// logRestartWait writes restartWait to the log.
func logRestartWait() {
	if restartWait != "" {
		logf("core", "info", "%s", restartWait)
	}
}

// restartWaitLimit is how long a new build waits for the window that
// started it to end.
const restartWaitLimit = 5 * time.Minute
