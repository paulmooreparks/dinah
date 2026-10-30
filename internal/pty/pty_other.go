//go:build !windows

package pty

// Session is a program running in a pseudoconsole. Outside Windows Dinah
// has none yet, so Start always fails and there is no program for Close to
// end. An implementation here keeps the Windows one's close behaviour with
// POSIX's documented means: the program leads a session of its own (setsid)
// with the pseudoterminal as its controlling terminal. Close closes the
// pseudoterminal, which hangs it up and sends SIGHUP to the program as the
// terminal's controlling process; the program's end sends SIGHUP to the
// terminal's foreground process group. Close sends SIGKILL to the program
// itself, and to nothing it started, if it has not ended after CloseGrace.
type Session struct{}

// ReleaseStandardHandles does nothing outside Windows.
func ReleaseStandardHandles() error { return nil }

// Start returns ErrUnsupported outside Windows.
func Start(cmd Command, cols, rows int) (*Session, error) { return nil, ErrUnsupported }

func (s *Session) Read(p []byte) (int, error)  { return 0, ErrUnsupported }
func (s *Session) Write(p []byte) (int, error) { return 0, ErrUnsupported }
func (s *Session) Resize(cols, rows int) error { return ErrUnsupported }
func (s *Session) Pid() int                    { return 0 }
func (s *Session) Wait() (int, error)          { return -1, ErrUnsupported }
func (s *Session) Close() error                { return nil }

// ChildrenOutsideAppJob is false outside Windows, where there is no program.
func (s *Session) ChildrenOutsideAppJob() bool { return false }
