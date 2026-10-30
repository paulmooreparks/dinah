// Package terminals owns shell discovery and running terminal processes.
package terminals

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
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

// terminal is a shell running for a Terminal window.
type terminal struct {
	session *pty.Session
}

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
	session, err := pty.Start(pty.Command{Path: path, Args: append([]string{filepath.Base(path)}, args...), Dir: folder,
		Env: terminalEnv(os.Environ())}, cols, rows)
	if err != nil {
		return Started{}, err
	}
	a.warnIfTerminalProgramsEndWithDinah(session)
	id := "term-" + rand.Text()
	a.mu.Lock()
	if a.terminals == nil {
		a.terminals = map[string]*terminal{}
	}
	a.terminals[id] = &terminal{session: session}
	a.mu.Unlock()
	go a.pumpTerminal(id, session)
	return Started{ID: id, Shell: shell, Name: name, Folder: folder}, nil
}

// terminalJobWarning makes the warning below appear once in a run.
var terminalJobWarning sync.Once

// warnIfTerminalProgramsEndWithDinah logs, once, that a Terminal's program
// had to be started inside Dinah's lifetime job, which happens when Dinah
// is in a job that does not allow breakaway. Programs started from such a
// Terminal, windowed ones included, end when Dinah exits.
func (a *Service) warnIfTerminalProgramsEndWithDinah(s *pty.Session) {
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
func (a *Service) pumpTerminal(id string, s *pty.Session) {
	codes := make(chan int, 1)
	go func() {
		code, _ := s.Wait()
		codes <- code
		// The console outlives its shell until it is closed, and its output
		// ends only then. Closing also sends the close event to whatever the
		// shell left running in the console.
		s.Close()
	}()
	buf := make([]byte, 32<<10)
	for {
		n, err := s.Read(buf)
		if n > 0 {
			a.emit("terminal:output", map[string]string{"id": id, "data": base64.StdEncoding.EncodeToString(buf[:n])})
		}
		if err != nil {
			break
		}
	}
	code := <-codes
	a.mu.Lock()
	delete(a.terminals, id)
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
	t, err := a.terminal(id)
	if err != nil {
		return nil
	}
	go t.session.Close()
	return nil
}

// CloseAll closes a snapshot of the running shells. It waits for their Close
// calls, not for their output pumps or exit callbacks to finish. The shells
// close concurrently. The host calls this before its process ends so programs
// attached to a Terminal receive the close event and end; the host exiting
// alone would end the shells but not those programs, which are outside its job.
func (a *Service) CloseAll() {
	a.mu.Lock()
	open := make([]*terminal, 0, len(a.terminals))
	for _, t := range a.terminals {
		open = append(open, t)
	}
	a.mu.Unlock()
	var wg sync.WaitGroup
	for _, t := range open {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.session.Close()
		}()
	}
	wg.Wait()
}
