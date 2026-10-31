// Package terminals owns shell discovery and running terminal processes.
package terminals

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"studio/internal/pty"
)

// Service owns the running terminals of one host. Callbacks run outside mu.
type Service struct {
	mu          sync.Mutex
	terminals   map[string]*terminal
	productName string
	emit        func(string, any)
	warning     func()
	stopping    bool
	active      map[string]*terminal
	changed     chan struct{}
	spawn       func(pty.Command, int, int) (terminalSession, error)
}

// New creates a terminal service. The required emit callback receives
// terminal:output {id,data} with base64 bytes and terminal:exit {id,code}.
// The optional warning callback supplies host wording.
func New(productName string, emit func(string, any), warning func()) *Service {
	return &Service{productName: productName, emit: emit, warning: warning}
}

// Count reports terminals whose output pump has not yet removed them.
func (a *Service) Count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.terminals)
}

// terminal tracks an admitted start through spawn and completion. Its mutable
// fields are guarded by Service.mu; session is immutable after registration.
type terminal struct {
	id         string
	pid        int
	session    terminalSession
	closing    bool
	shellDone  bool
	outputDone bool
	readErr    error
	waitErr    error
	closeErr   error
}

type terminalSession interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Resize(int, int) error
	Wait() (int, error)
	Close() error
	Pid() int
	ChildrenOutsideAppJob() bool
}

// ErrShuttingDown rejects starts after the host's shutdown admission barrier.
var ErrShuttingDown = errors.New("terminal service is shutting down")

// Started is a shell the page started.
type Started struct {
	ID     string `json:"id"`
	Shell  string `json:"shell"`
	Name   string `json:"name"`
	Folder string `json:"folder"`
}

// StartShell starts the selected shell in the host-resolved folder.
func (a *Service) StartShell(shell string, profiles []Profile, folder string, cols, rows int) (Started, error) {
	var chosen *Shell
	for _, s := range Available(a.productName, profiles) {
		if s.ID == shell && s.Path != "" {
			s := s
			chosen = &s
		}
	}
	if chosen == nil {
		return Started{}, fmt.Errorf("no shell called %q is on this computer", shell)
	}
	return a.start(chosen.Path, chosen.args, folder, chosen.ID, chosen.Name, cols, rows)
}

// StartCommand starts an already-authorized command in the supplied folder.
func (a *Service) StartCommand(program string, args []string, folder string, cols, rows int) (Started, error) {
	executable, err := exec.LookPath(program)
	if err != nil {
		return Started{}, err
	}
	return a.start(executable, args, folder, "command", program, cols, rows)
}

func (a *Service) start(path string, args []string, folder, shell, name string, cols, rows int) (Started, error) {
	a.mu.Lock()
	if a.stopping {
		a.mu.Unlock()
		return Started{}, ErrShuttingDown
	}
	t := &terminal{id: "term-" + rand.Text()}
	if a.active == nil {
		a.active = map[string]*terminal{}
	}
	a.active[t.id] = t
	a.signalLocked()
	a.mu.Unlock()
	spawn := a.spawn
	if spawn == nil {
		spawn = func(c pty.Command, cols, rows int) (terminalSession, error) { return pty.Start(c, cols, rows) }
	}
	session, err := spawn(pty.Command{Path: path, Args: append([]string{filepath.Base(path)}, args...), Dir: folder,
		Env: terminalEnv(os.Environ())}, cols, rows)
	if err != nil {
		a.mu.Lock()
		delete(a.active, t.id)
		a.signalLocked()
		a.mu.Unlock()
		return Started{}, err
	}
	a.mu.Lock()
	if a.terminals == nil {
		a.terminals = map[string]*terminal{}
	}
	t.session, t.pid = session, session.Pid()
	a.terminals[t.id] = t
	stopping := a.stopping
	if stopping {
		a.requestCloseLocked(t)
	}
	a.signalLocked()
	a.mu.Unlock()
	go a.pumpTerminal(t)
	a.warnIfTerminalProgramsEndWithDinah(session)
	if stopping {
		return Started{}, ErrShuttingDown
	}
	return Started{ID: t.id, Shell: shell, Name: name, Folder: folder}, nil
}

// terminalJobWarning makes the warning below appear once in a run.
var terminalJobWarning sync.Once

// warnIfTerminalProgramsEndWithDinah logs, once, that a Terminal's program
// had to be started inside Dinah's lifetime job, which happens when Dinah
// is in a job that does not allow breakaway. Programs started from such a
// Terminal, windowed ones included, end when Dinah exits.
func (a *Service) warnIfTerminalProgramsEndWithDinah(s terminalSession) {
	if s.ChildrenOutsideAppJob() {
		return
	}
	terminalJobWarning.Do(func() {
		if a.warning != nil {
			a.warning()
		}
	})
}

// terminalEnv is the shell's environment: Dinah's own, with TERM saying what
// xterm.js draws, for shells that read it, such as Git Bash's.
func terminalEnv(base []string) []string {
	out := make([]string, 0, len(base)+1)
	for _, kv := range base {
		if !strings.HasPrefix(strings.ToUpper(kv), "TERM=") {
			out = append(out, kv)
		}
	}
	return append(out, "TERM=xterm-256color")
}

// pumpTerminal sends what the shell draws to the page until the shell ends
// and its console is closed, then says it ended.
func (a *Service) pumpTerminal(t *terminal) {
	id, s := t.id, t.session
	codes := make(chan int, 1)
	go func() {
		code, err := s.Wait()
		a.mu.Lock()
		t.shellDone, t.waitErr = true, err
		// The console can outlive its shell. Request closure so connected
		// clients receive the close event while the output pump keeps reading.
		a.requestCloseLocked(t)
		a.signalLocked()
		a.mu.Unlock()
		codes <- code
	}()
	buf := make([]byte, 32<<10)
	for {
		n, err := s.Read(buf)
		if n > 0 {
			a.emit("terminal:output", map[string]string{"id": id, "data": base64.StdEncoding.EncodeToString(buf[:n])})
		}
		if err != nil {
			a.mu.Lock()
			t.outputDone = err == io.EOF
			if err != io.EOF {
				t.readErr = err
			}
			a.signalLocked()
			a.mu.Unlock()
			break
		}
	}
	code := <-codes
	a.mu.Lock()
	if !t.outputDone || t.waitErr != nil {
		a.mu.Unlock()
		return
	}
	delete(a.terminals, id)
	delete(a.active, id)
	a.signalLocked()
	a.mu.Unlock()
	a.emit("terminal:exit", map[string]any{"id": id, "code": code})
}

func (a *Service) terminal(id string) (*terminal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t := a.terminals[id]; t != nil {
		return t, nil
	}
	return nil, errors.New("that shell has ended")
}

// Write sends what the person typed or pasted to a shell.
func (a *Service) Write(id, data string) error {
	t, err := a.terminal(id)
	if err != nil {
		return err
	}
	_, err = t.session.Write([]byte(data))
	return err
}

// Resize gives a shell's console a new size in cells.
func (a *Service) Resize(id string, cols, rows int) error {
	t, err := a.terminal(id)
	if err != nil {
		return err
	}
	return t.session.Resize(cols, rows)
}

// Close ends a shell and the programs still attached to its console;
// a windowed program the shell started goes on running. The shell is given
// pty.CloseGrace to end on its own before it is ended, which happens on a
// goroutine of its own so the page does not wait; the shell's end arrives
// as terminal:exit, as it does when the shell exits by itself. Closing a
// shell that is closing or has ended does nothing more.
func (a *Service) Close(id string) error {
	a.mu.Lock()
	if t := a.terminals[id]; t != nil {
		a.requestCloseLocked(t)
	}
	a.mu.Unlock()
	return nil
}

// CloseAll requests closure of the currently spawned terminals and returns
// without waiting. It is safe from emit callbacks. Host teardown uses Shutdown.
func (a *Service) CloseAll() {
	a.mu.Lock()
	for _, t := range a.terminals {
		a.requestCloseLocked(t)
	}
	a.mu.Unlock()
}
