package terminals

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"studio/internal/pty"
)

type terminalRead struct {
	data string
	err  error
}
type controlledTerminal struct {
	reads        chan terminalRead
	exited       chan struct{}
	closeStarted chan struct{}
	closeGate    chan struct{}
	closed       sync.Once
	waitErr      error
}

func newControlledTerminal() *controlledTerminal {
	return &controlledTerminal{reads: make(chan terminalRead, 4), exited: make(chan struct{}), closeStarted: make(chan struct{})}
}
func (s *controlledTerminal) Read(p []byte) (int, error) {
	r := <-s.reads
	return copy(p, r.data), r.err
}
func (s *controlledTerminal) Write(p []byte) (int, error) { return len(p), nil }
func (s *controlledTerminal) Resize(int, int) error       { return nil }
func (s *controlledTerminal) Pid() int                    { return 123 }
func (s *controlledTerminal) ChildrenOutsideAppJob() bool { return true }
func (s *controlledTerminal) Wait() (int, error)          { <-s.exited; return 7, s.waitErr }
func (s *controlledTerminal) Close() error {
	s.closed.Do(func() { close(s.closeStarted) })
	if s.closeGate != nil {
		<-s.closeGate
	}
	return nil
}
func controlledService(s *controlledTerminal, emit func(string, any)) *Service {
	a := New("test", emit, nil)
	a.spawn = func(pty.Command, int, int) (terminalSession, error) { return s, nil }
	return a
}
func startControlled(t *testing.T, a *Service) Started {
	t.Helper()
	started, err := a.start("synthetic", nil, "", "command", "test", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	return started
}
func testShutdown(t *testing.T, a *Service) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return a.Shutdown(ctx)
}
func incompleteNow(t *testing.T, a *Service) *ShutdownError {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := a.Shutdown(ctx)
	var pending *ShutdownError
	if !errors.As(err, &pending) {
		t.Fatalf("want incomplete accounting, got %v", err)
	}
	return pending
}
func receive(t *testing.T, c <-chan struct{}) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(2 * time.Second):
		t.Fatal("lifecycle signal did not arrive")
	}
}

func TestShutdownRequiresShellAndCleanEOFBeforeExitCallback(t *testing.T) {
	s := newControlledTerminal()
	exitEntered, releaseExit := make(chan struct{}), make(chan struct{})
	defer close(releaseExit)
	a := controlledService(s, func(name string, _ any) {
		if name == "terminal:exit" {
			close(exitEntered)
			<-releaseExit
		}
	})
	started := startControlled(t, a)
	p := incompleteNow(t, a)
	if len(p.Pending) != 1 || p.Pending[0].ID != started.ID || !p.Pending[0].ShellPending || !p.Pending[0].OutputPending {
		t.Fatalf("initial stages: %+v", p)
	}
	receive(t, s.closeStarted)
	// The close API returning cannot establish either completion signal.
	close(s.exited)
	p = incompleteNow(t, a)
	if len(p.Pending) != 1 || !p.Pending[0].OutputPending {
		t.Fatalf("output completion lost: %+v", p)
	}
	s.reads <- terminalRead{err: io.EOF}
	receive(t, exitEntered)
	if err := testShutdown(t, a); err != nil {
		t.Fatal(err)
	}
	if a.Count() != 0 {
		t.Fatal("completed terminal remains registered")
	}
	a.spawn = func(pty.Command, int, int) (terminalSession, error) {
		t.Fatal("shutdown admitted another spawn")
		return nil, errors.New("unreachable")
	}
	if _, err := a.start("synthetic", nil, "", "command", "test", 80, 24); !errors.Is(err, ErrShuttingDown) {
		t.Fatalf("start after shutdown: %v", err)
	}
}

func TestEOFAloneDoesNotDeliverExit(t *testing.T) {
	s := newControlledTerminal()
	events := make(chan struct{}, 1)
	a := controlledService(s, func(name string, _ any) {
		if name == "terminal:exit" {
			events <- struct{}{}
		}
	})
	startControlled(t, a)
	s.reads <- terminalRead{err: io.EOF}
	p := incompleteNow(t, a)
	if !p.Pending[0].ShellPending {
		t.Fatalf("shell completion invented: %+v", p)
	}
	select {
	case <-events:
		t.Fatal("exit delivered before Wait")
	default:
	}
	close(s.exited)
	if err := testShutdown(t, a); err != nil {
		t.Fatal(err)
	}
}

func TestCallbacksCanRequestAllClosuresWithoutJoiningThemselves(t *testing.T) {
	s := newControlledTerminal()
	s.closeGate = make(chan struct{})
	defer close(s.closeGate)
	outputReturned, exitReturned := make(chan struct{}), make(chan struct{})
	var a *Service
	a = controlledService(s, func(name string, _ any) {
		a.CloseAll()
		if name == "terminal:output" {
			close(outputReturned)
		}
		if name == "terminal:exit" {
			close(exitReturned)
		}
	})
	startControlled(t, a)
	s.reads <- terminalRead{data: "ready"}
	receive(t, outputReturned)
	receive(t, s.closeStarted)
	close(s.exited)
	s.reads <- terminalRead{err: io.EOF}
	receive(t, exitReturned)
	if err := testShutdown(t, a); err != nil {
		t.Fatal(err)
	}
}

func TestBlockedOutputCallbackRemainsIncompleteAfterTimeout(t *testing.T) {
	s := newControlledTerminal()
	entered, release := make(chan struct{}), make(chan struct{})
	a := controlledService(s, func(name string, _ any) {
		if name == "terminal:output" {
			close(entered)
			<-release
		}
	})
	startControlled(t, a)
	s.reads <- terminalRead{data: "blocked"}
	receive(t, entered)
	close(s.exited)
	s.reads <- terminalRead{err: io.EOF}
	p := incompleteNow(t, a)
	if !p.Pending[0].OutputPending {
		t.Fatal("blocked output reported complete")
	}
	close(release)
	if err := testShutdown(t, a); err != nil {
		t.Fatal(err)
	}
}

func TestReadErrorCannotBecomeEOFOrDeliverExit(t *testing.T) {
	s := newControlledTerminal()
	events := make(chan struct{}, 1)
	a := controlledService(s, func(name string, _ any) {
		if name == "terminal:exit" {
			events <- struct{}{}
		}
	})
	startControlled(t, a)
	s.reads <- terminalRead{err: errors.New("synthetic read failure")}
	close(s.exited)
	err := testShutdown(t, a)
	var pending *ShutdownError
	if !errors.As(err, &pending) || len(pending.Pending) != 1 || pending.Pending[0].ReadError != "synthetic read failure" || !pending.Pending[0].OutputPending {
		t.Fatalf("read failure: %+v", err)
	}
	select {
	case <-events:
		t.Fatal("read error delivered terminal:exit")
	default:
	}
	if a.Count() != 1 {
		t.Fatal("incomplete terminal was forgotten")
	}
}

func TestWaitErrorCannotConfirmShellTermination(t *testing.T) {
	s := newControlledTerminal()
	s.waitErr = errors.New("synthetic process wait failure")
	events := make(chan struct{}, 1)
	a := controlledService(s, func(name string, _ any) {
		if name == "terminal:exit" {
			events <- struct{}{}
		}
	})
	startControlled(t, a)
	s.reads <- terminalRead{err: io.EOF}
	close(s.exited)
	err := testShutdown(t, a)
	var pending *ShutdownError
	if !errors.As(err, &pending) || len(pending.Pending) != 1 || !pending.Pending[0].ShellPending || pending.Pending[0].WaitError != s.waitErr.Error() {
		t.Fatalf("wait error reported complete: %v", err)
	}
	select {
	case <-events:
		t.Fatal("failed Wait delivered terminal:exit")
	default:
	}
}

func TestShutdownTracksAdmittedSpawnThroughTimeoutAndRegistration(t *testing.T) {
	s := newControlledTerminal()
	a := controlledService(s, func(string, any) {})
	entered, release := make(chan struct{}), make(chan struct{})
	a.spawn = func(pty.Command, int, int) (terminalSession, error) { close(entered); <-release; return s, nil }
	result := make(chan error, 1)
	go func() { _, err := a.start("synthetic", nil, "", "command", "test", 80, 24); result <- err }()
	receive(t, entered)
	p := incompleteNow(t, a)
	if len(p.Pending) != 1 || !p.Pending[0].SpawnPending {
		t.Fatalf("spawn not tracked: %+v", p)
	}
	id := p.Pending[0].ID
	close(release)
	if err := <-result; !errors.Is(err, ErrShuttingDown) {
		t.Fatalf("in-flight start result: %v", err)
	}
	receive(t, s.closeStarted)
	p = incompleteNow(t, a)
	if p.Pending[0].ID != id || p.Pending[0].SpawnPending || p.Pending[0].PID != 123 {
		t.Fatalf("registration identity changed: %+v", p)
	}
	close(s.exited)
	s.reads <- terminalRead{err: io.EOF}
	if err := testShutdown(t, a); err != nil {
		t.Fatal(err)
	}
}

func TestFailedAdmittedSpawnReleasesAccounting(t *testing.T) {
	a := New("test", func(string, any) {}, nil)
	entered, release := make(chan struct{}), make(chan struct{})
	a.spawn = func(pty.Command, int, int) (terminalSession, error) {
		close(entered)
		<-release
		return nil, errors.New("spawn failed")
	}
	result := make(chan error, 1)
	go func() { _, err := a.start("synthetic", nil, "", "command", "test", 80, 24); result <- err }()
	receive(t, entered)
	incompleteNow(t, a)
	close(release)
	if err := <-result; err == nil {
		t.Fatal("spawn failure hidden")
	}
	if err := testShutdown(t, a); err != nil {
		t.Fatal(err)
	}
}
