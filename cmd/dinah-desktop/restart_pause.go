package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"studio/internal/bus"
	"studio/internal/statefile"
)

/* Pause and Restart. Restart When Chats Finish waits for every running turn
   to end by itself, and with many agents at work that moment does not come:
   Paul ran one window for days without being able to take an update (14
   November 2026). Pause and Restart asks for the moment. It tells every
   working chat that can be told, through its agent's steering, to stop at
   the next safe point; holds new turns and new writes of documents as the
   other restart does; restarts when the turns have ended; and after the
   restart tells each chat it stopped to carry on, once.

   The chats that were working are written down before anything is asked of
   them, so a restart that comes about another way, Restart Now included,
   still finds them. A chat whose agent cannot be steered is in the list too:
   its turn is waited for, or cut short by Restart Now, and it is told
   afterwards which of the two happened. The message after the restart is a
   host result on the bus, addressed to the chat's own bus session, so it
   opens a closed chat as any message that asks for one does, and its key
   keeps it from being sent twice. */

// pausedChat is one chat that was working when a restart was asked for.
type pausedChat struct {
	Chat  string `json:"chat"`
	Bus   string `json:"bus"`
	Title string `json:"title"`
	// Told is set when the chat's agent was asked to stop, and Ended when
	// its turn had ended before the window closed.
	Told  bool `json:"told"`
	Ended bool `json:"ended"`
}

// pausedRecord is the file the chats to resume are kept in.
type pausedRecord struct {
	// Run tells one restart's messages from another's.
	Run   string       `json:"run"`
	Chats []pausedChat `json:"chats"`
}

const pausedFile = "restart-resume.json"

// pausedMu guards the record's file within the window.
var pausedMu sync.Mutex

func pausedPath() string { return filepath.Join(configDir(), pausedFile) }

// pauseRequest is what a working chat is told.
const pauseRequest = "The person is restarting %s to install an update, and asks every chat to pause. Stop at the next safe point: finish the step you are in the middle of if stopping now would leave something half written, start nothing new, and end your turn with one or two lines that say what you had finished and what you were about to do. Do not commit, push, send messages or start long commands in order to finish first. After the restart you are told to carry on."

func readPaused() pausedRecord {
	var r pausedRecord
	data, err := statefile.ReadRegular(pausedPath())
	if err != nil {
		return pausedRecord{}
	}
	if json.Unmarshal(data, &r) != nil {
		return pausedRecord{}
	}
	return r
}

func writePaused(r pausedRecord) error {
	if len(r.Chats) == 0 {
		err := os.Remove(pausedPath())
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return statefile.Write(pausedPath(), data)
}

// workingSessions are the chats in a turn or with work of their own going
// on, as workingChats counts them.
func (a *App) workingSessions() []*Session {
	a.mu.Lock()
	open := make([]*Session, 0, a.runtime.ConversationCountLocked())
	for _, s := range a.runtime.ConversationsLocked() {
		open = append(open, s)
	}
	a.mu.Unlock()
	var out []*Session
	for _, s := range open {
		state := s.State()
		if state.WorkActivity().Busy() {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// notePaused adds the chats working now to the record of chats to resume,
// keeping what the record already says of a chat.
func (a *App) notePaused(told map[string]bool) (pausedRecord, error) {
	pausedMu.Lock()
	defer pausedMu.Unlock()
	r := readPaused()
	for i := range r.Chats {
		r.Chats[i].Told = r.Chats[i].Told || told[r.Chats[i].Chat]
	}
	if r.Run == "" {
		r.Run = time.Now().UTC().Format("20060102T150405.000000000")
	}
	known := map[string]int{}
	for i, c := range r.Chats {
		known[c.Chat] = i
	}
	for _, s := range a.workingSessions() {
		id := s.ID()
		a.mu.Lock()
		busID := a.settings.BusSessions[id]
		a.mu.Unlock()
		if i, ok := known[id]; ok {
			r.Chats[i].Told = r.Chats[i].Told || told[id]
			continue
		}
		r.Chats = append(r.Chats, pausedChat{Chat: id, Bus: busID, Title: a.chatTitle(s), Told: told[id]})
	}
	return r, writePaused(r)
}

// PauseAndRestart asks every working chat to stop at a safe point and
// restarts into the newer build when their turns have ended. The chats are
// told to carry on after the restart.
func (a *App) PauseAndRestart() error {
	a.updMu.Lock()
	ready := a.update != nil && a.update.ready
	a.updMu.Unlock()
	if !ready {
		return errors.New("no newer build of " + product.Name + " is installed")
	}
	told := map[string]bool{}
	// The working chats are written down before anything else, so that one
	// whose turn ends at once is still told to carry on.
	if _, err := a.notePaused(nil); err != nil {
		logf("core", "warn", "the chats to resume after the restart could not be written down: %v", err)
	}
	// Turns are held from here on, and the request to pause is the one
	// message let through.
	a.updMu.Lock()
	a.update.pausing, a.update.telling, a.update.pausedAt = true, true, time.Now()
	a.updMu.Unlock()
	if err := a.RestartToUpdate(false); err != nil {
		a.updMu.Lock()
		a.update.pausing, a.update.telling = false, false
		a.updMu.Unlock()
		return err
	}
	defer a.emitUpdate()
	defer func() {
		a.updMu.Lock()
		a.update.telling = false
		cancelled := !a.update.pending
		a.updMu.Unlock()
		if cancelled {
			a.resumePaused(false)
		}
	}()
	for _, s := range a.workingSessions() {
		id := s.ID()
		if !a.CanSteer(id, nil) {
			continue
		}
		if err := a.Steer(id, fmt.Sprintf(pauseRequest, product.Name), nil, nil); err != nil {
			logf("core", "warn", "asking %s to pause for the restart: %v", a.chatTitle(s), err)
			continue
		}
		told[id] = true
	}
	r, err := a.notePaused(told)
	if err != nil {
		logf("core", "warn", "the chats to resume after the restart could not be written down: %v", err)
	}
	logf("core", "info", "Pause and Restart: %d chat(s) working, %d asked to pause", len(r.Chats), len(told))
	return nil
}

// noteInterrupted writes down the chats still working when the window
// closes for a restart, and which of the recorded ones have ended.
func (a *App) noteInterrupted() {
	working := map[string]bool{}
	for _, s := range a.workingSessions() {
		working[s.ID()] = true
	}
	r, err := a.notePaused(nil)
	if err == nil {
		pausedMu.Lock()
		for i := range r.Chats {
			r.Chats[i].Ended = !working[r.Chats[i].Chat]
		}
		err = writePaused(r)
		pausedMu.Unlock()
	}
	if err != nil {
		logf("core", "warn", "the chats to resume after the restart could not be written down: %v", err)
	}
}

// blockers are the titles of the chats a waiting restart waits for.
func (a *App) blockers() []string {
	out := []string{}
	for _, s := range a.workingSessions() {
		out = append(out, a.chatTitle(s))
	}
	sort.Strings(out)
	return out
}

// resumeText is what a chat is told when it may carry on.
func resumeText(c pausedChat, restarted bool) string {
	switch {
	case !restarted:
		return "The restart of " + product.Name + " was cancelled. If you paused your work for it, carry on from where you stopped."
	case c.Ended && c.Told:
		return product.Name + " has restarted with its update. You paused your work for the restart; carry on from where you stopped."
	case c.Ended:
		return product.Name + " has restarted with its update. Your last turn ended just before it; if work was left to do, carry on."
	default:
		return product.Name + " has restarted with its update, and your turn was cut short by the restart. Look at what you had done, since the last step may be half finished, and carry on from there."
	}
}

// resumePaused tells each recorded chat to carry on, and forgets the ones
// told. restarted says the window has restarted; unset, a waiting restart
// was cancelled, and only the chats that were asked to pause are told.
func (a *App) resumePaused(restarted bool) {
	pausedMu.Lock()
	defer pausedMu.Unlock()
	r := readPaused()
	if len(r.Chats) == 0 {
		return
	}
	var left []pausedChat
	told := 0
	for _, c := range r.Chats {
		if !restarted && !c.Told {
			continue
		}
		if c.Bus == "" {
			logf("core", "warn", "%s was working before the restart and has no place on the bus, so it was not told to carry on", c.Title)
			continue
		}
		key := "restart-resume:" + r.Run + ":" + c.Bus
		var err error
		// The bus may not be open in the first moments of a start.
		for try := 0; try < 30; try++ {
			err = a.busHost().With(func(b *bus.Bus) error {
				_, err := b.EnqueueResult(context.Background(), key, c.Bus, resumeText(c, restarted))
				return err
			})
			if err == nil {
				break
			}
			time.Sleep(time.Second)
		}
		if err != nil {
			logf("core", "warn", "telling %s to carry on after the restart: %v", c.Title, err)
			left = append(left, c)
			continue
		}
		told++
	}
	r.Chats = left
	if err := writePaused(r); err != nil {
		logf("core", "warn", "the record of chats to resume could not be cleared: %v", err)
	}
	if told > 0 {
		logf("core", "info", "%d chat(s) told to carry on", told)
	}
}
