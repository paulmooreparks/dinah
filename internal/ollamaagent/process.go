package ollamaagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"studio/internal/acp"
)

// appletStartWithin bounds starting an applet server: initializing it and
// listing its tools.
const appletStartWithin = 20 * time.Second

// closePhases are how long Close waits for the process to exit after its
// stdin closes, after the graceful signal, and after Kill.
var closePhases = [3]time.Duration{2 * time.Second, 3 * time.Second, 3 * time.Second}

// errStillRunning is Close's error when the process outlasts every phase.
// The process's one Wait goroutine still collects it when it does exit.
var errStillRunning = errors.New("the applet server did not exit")

// appletProcess owns one applet server's process and its MCP session.
//
// The pipes are made with os.Pipe and given to the command as files. The
// os/exec documentation says Wait closes a StdoutPipe and must not be
// called before reads from it finish; with files of our own, Wait touches
// nothing the session reads, so one goroutine can wait from the start.
//
// Pending calls are released by context, not by closing the session:
// ClientSession.Close is documented as waiting for ongoing requests, while
// cancelling a call's context is documented to end that call. Every call
// therefore runs under life as well as its turn's context.
type appletProcess struct {
	cmd    *exec.Cmd
	stdin  *os.File // the parent's write end of the child's stdin
	stdout *os.File // the parent's read end of the child's stdout
	life   context.Context
	stop   context.CancelFunc
	phases [3]time.Duration

	exited  chan struct{}
	waitErr error // set before exited closes

	mu      sync.Mutex
	session *mcp.ClientSession

	closeOnce sync.Once
	closeErr  error
}

// startAppletProcess starts s's command. It does not speak MCP yet.
func startAppletProcess(s server, phases [3]time.Duration) (*appletProcess, error) {
	childIn, parentIn, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	parentOut, childOut, err := os.Pipe()
	if err != nil {
		childIn.Close()
		parentIn.Close()
		return nil, err
	}
	// The process belongs to the cached server, not to a turn, so no
	// context ends it.
	cmd := exec.Command(s.Command, s.Args...)
	cmd.Env = append(cmd.Environ(), s.Env...)
	cmd.Stdin, cmd.Stdout = childIn, childOut
	// Stderr stays nil, the null device: nothing reads it, and an unread
	// pipe would block a child that wrote enough to fill it.
	acp.HideWindow(cmd)
	err = cmd.Start()
	childIn.Close()
	childOut.Close()
	if err != nil {
		parentIn.Close()
		parentOut.Close()
		return nil, err
	}
	life, stop := context.WithCancel(context.Background())
	p := &appletProcess{cmd: cmd, stdin: parentIn, stdout: parentOut, life: life, stop: stop, phases: phases, exited: make(chan struct{})}
	go func() {
		p.waitErr = cmd.Wait()
		close(p.exited)
	}()
	return p, nil
}

// connect initializes the session and lists the tools within the time
// given, or until ctx ends. Connect documents no limit on how long it
// keeps its context, so it gets the process's lifetime and the deadline is
// kept from outside: when it passes, the process is closed, which ends
// Connect, and Connect is joined before the error is returned. On any
// failure the process is closed.
func (p *appletProcess) connect(ctx context.Context, name string, within time.Duration) ([]*mcp.Tool, error) {
	ictx, cancel := context.WithTimeout(ctx, within)
	defer cancel()
	type connected struct {
		session *mcp.ClientSession
		err     error
	}
	done := make(chan connected, 1)
	go func() {
		client := mcp.NewClient(&mcp.Implementation{Name: "dinah-ollama", Version: "1"}, nil)
		cs, err := client.Connect(p.life, &mcp.IOTransport{Reader: p.stdout, Writer: p.stdin, MaxLineLength: -1}, nil)
		done <- connected{cs, err}
	}()
	var r connected
	select {
	case r = <-done:
	case <-ictx.Done():
		p.close()
		r = <-done
		if r.session != nil {
			// A session that connected as the deadline passed is closed
			// without waiting, as close does.
			go r.session.Close()
		}
		return nil, fmt.Errorf("the applet server %s did not start: %w", name, ictx.Err())
	}
	if r.err != nil {
		p.close()
		return nil, fmt.Errorf("the applet server %s did not start: %w", name, r.err)
	}
	p.mu.Lock()
	p.session = r.session
	p.mu.Unlock()
	list, err := r.session.ListTools(ictx, nil)
	if err != nil {
		p.close()
		return nil, fmt.Errorf("the applet server %s did not list its tools: %w", name, err)
	}
	return list.Tools, nil
}

// call runs one tool, ending early if ctx ends or the process is closed.
func (p *appletProcess) call(ctx context.Context, tool string, args map[string]any) (*mcp.CallToolResult, error) {
	p.mu.Lock()
	cs := p.session
	p.mu.Unlock()
	if cs == nil {
		return nil, errors.New("the applet server is not running")
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer context.AfterFunc(p.life, cancel)()
	return cs.CallTool(cctx, &mcp.CallToolParams{Name: tool, Arguments: args})
}

// close ends the server. It is safe to call more than once and from more
// than one goroutine; every call returns the first one's result. Pending
// calls are released first. Then stdin is closed, which is the first
// shutdown phase, and the read end, which also frees us from a grandchild
// that inherited the child's stdout. The session is closed on a goroutine
// of its own: ClientSession.Close is documented as graceful but not as
// bounded, so close never waits for it. Last come the phases: wait, the
// graceful signal and wait, Kill and wait. Close therefore returns within
// the three phases. Returning does not mean the process has exited.
func (p *appletProcess) close() error {
	p.closeOnce.Do(func() {
		p.stop()
		err := p.stdin.Close()
		if e := p.stdout.Close(); e != nil && !errors.Is(e, os.ErrClosed) && err == nil {
			err = e
		}
		p.mu.Lock()
		cs := p.session
		p.mu.Unlock()
		if cs != nil {
			go cs.Close()
		}
		if e, ok := p.exitWithin(p.phases[0]); ok {
			p.closeErr = errors.Join(err, e)
			return
		}
		terminate(p.cmd.Process)
		if e, ok := p.exitWithin(p.phases[1]); ok {
			p.closeErr = errors.Join(err, e)
			return
		}
		if e := p.cmd.Process.Kill(); e != nil && !errors.Is(e, os.ErrProcessDone) && err == nil {
			err = e
		}
		if e, ok := p.exitWithin(p.phases[2]); ok {
			p.closeErr = errors.Join(err, e)
			return
		}
		p.closeErr = errors.Join(err, errStillRunning)
	})
	return p.closeErr
}

func (p *appletProcess) exitWithin(d time.Duration) (error, bool) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-p.exited:
		return p.waitErr, true
	case <-t.C:
		return nil, false
	}
}
