//go:build windows

package keyboard

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/term"

	"dinah/internal/screen"
)

// procReadConsoleInput is ReadConsoleInputW, which golang.org/x/sys/windows
// does not bind. Microsoft documents it on its own page under
// https://learn.microsoft.com/en-us/windows/console/readconsoleinput.
var procReadConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputW")

// rawInputRecord is INPUT_RECORD as the console writes it, holding a
// KEY_EVENT_RECORD in its union: EventType, two bytes of padding the union's
// 32-bit alignment puts after it, then bKeyDown, wRepeatCount,
// wVirtualKeyCode, wVirtualScanCode, uChar and dwControlKeyState, twenty
// bytes in all. A WINDOW_BUFFER_SIZE_RECORD shares the union and is read by
// its EventType alone.
type rawInputRecord struct {
	eventType       uint16
	_               uint16
	keyDown         int32
	repeatCount     uint16
	virtualKeyCode  uint16
	virtualScanCode uint16
	unicodeChar     uint16
	controlKeyState uint32
}

// consoleCalls are the console functions the keyboard entry and exit call,
// behind an interface so a test can record the order they are called in.
type consoleCalls interface {
	GetConsoleMode(handle windows.Handle, mode *uint32) error
	SetConsoleMode(handle windows.Handle, mode uint32) error
	FlushConsoleInputBuffer(handle windows.Handle) error
	Write(out *os.File, text string) error
}

// systemConsole calls the real functions.
type systemConsole struct{}

func (systemConsole) GetConsoleMode(handle windows.Handle, mode *uint32) error {
	return windows.GetConsoleMode(handle, mode)
}

func (systemConsole) SetConsoleMode(handle windows.Handle, mode uint32) error {
	return windows.SetConsoleMode(handle, mode)
}

func (systemConsole) FlushConsoleInputBuffer(handle windows.Handle) error {
	return windows.FlushConsoleInputBuffer(handle)
}

func (systemConsole) Write(out *os.File, text string) error {
	_, err := out.WriteString(text)
	return err
}

// consoleAPI is the console the keyboard calls, which a test replaces.
var consoleAPI consoleCalls = systemConsole{}

// Keyboard is the console's input side while the terminal head runs: the
// modes it found on both handles, which Leave puts back.
type Keyboard struct {
	in, out           *os.File
	savedIn, savedOut uint32
}

// EnterKeyboard takes the console's keyboard for the terminal head. It saves
// the input handle's mode, discards whatever input is waiting, and sets the
// mode to exactly ENABLE_WINDOW_INPUT, which clears ENABLE_PROCESSED_INPUT so
// that Ctrl+C arrives as a key, clears line input, echo, mouse input and
// virtual-terminal input, and reports buffer size changes as records. It then
// saves the output handle's mode, adds ENABLE_VIRTUAL_TERMINAL_PROCESSING and
// DISABLE_NEWLINE_AUTO_RETURN, and only then writes the request for bracketed
// paste, so the console reads that request as a sequence. entry is unused on
// Windows, where no terminfo describes the console.
func EnterKeyboard(in, out *os.File, entry *screen.Terminfo) (*Keyboard, error) {
	k := &Keyboard{in: in, out: out}
	inHandle := windows.Handle(in.Fd())
	outHandle := windows.Handle(out.Fd())
	if err := consoleAPI.GetConsoleMode(inHandle, &k.savedIn); err != nil {
		return nil, err
	}
	if err := consoleAPI.FlushConsoleInputBuffer(inHandle); err != nil {
		return nil, err
	}
	if err := consoleAPI.SetConsoleMode(inHandle, windows.ENABLE_WINDOW_INPUT); err != nil {
		return nil, err
	}
	if err := consoleAPI.GetConsoleMode(outHandle, &k.savedOut); err != nil {
		consoleAPI.SetConsoleMode(inHandle, k.savedIn)
		return nil, err
	}
	vt := k.savedOut | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING | windows.DISABLE_NEWLINE_AUTO_RETURN
	if err := consoleAPI.SetConsoleMode(outHandle, vt); err != nil {
		consoleAPI.SetConsoleMode(inHandle, k.savedIn)
		return nil, err
	}
	if err := consoleAPI.Write(out, screen.BracketedPasteOn); err != nil {
		consoleAPI.SetConsoleMode(outHandle, k.savedOut)
		consoleAPI.SetConsoleMode(inHandle, k.savedIn)
		return nil, err
	}
	return k, nil
}

// Leave gives the keyboard back: it writes the bracketed-paste disable while
// virtual-terminal processing is still on, then restores the output mode and
// the input mode it saved, in that order. Every step runs whatever an earlier
// one answered, and the first error is reported.
func (k *Keyboard) Leave() error {
	var first error
	keep := func(err error) {
		if first == nil {
			first = err
		}
	}
	keep(consoleAPI.Write(k.out, screen.BracketedPasteOff))
	keep(consoleAPI.SetConsoleMode(windows.Handle(k.out.Fd()), k.savedOut))
	keep(consoleAPI.SetConsoleMode(windows.Handle(k.in.Fd()), k.savedIn))
	return first
}

// consoleReader reads the console's input records on a waited handle.
type consoleReader struct {
	in       windows.Handle
	decoder  *screen.ConsoleDecoder
	cancel   windows.Handle
	flush    windows.Handle
	gen      uint64
	sawPaste atomic.Bool
	once     sync.Once
}

// NewReader builds the reader for the console the keyboard was entered on.
func (k *Keyboard) NewReader() (screen.Reader, error) {
	cancel, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, err
	}
	flush, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		windows.CloseHandle(cancel)
		return nil, err
	}
	return &consoleReader{
		in:      windows.Handle(k.in.Fd()),
		decoder: screen.NewConsoleDecoder(),
		cancel:  cancel,
		flush:   flush,
	}, nil
}

// Run waits on the console input handle, the cancel event and the flush
// event together, and reads the records waiting whenever the input handle is
// signalled, which the console's "Low-Level Console Input Functions" page
// says it is while "there are unread records in its input buffer". It
// returns when Stop sets the cancel event, or on an error.
func (r *consoleReader) Run(sink screen.Sink) error {
	defer windows.CloseHandle(r.flush)
	defer windows.CloseHandle(r.cancel)
	handles := []windows.Handle{r.in, r.cancel, r.flush}
	records := make([]rawInputRecord, 64)
	for {
		which, err := windows.WaitForMultipleObjects(handles, false, windows.INFINITE)
		if err != nil {
			return err
		}
		switch which {
		case windows.WAIT_OBJECT_0 + 1:
			return nil
		case windows.WAIT_OBJECT_0 + 2:
			windows.FlushConsoleInputBuffer(r.in)
			r.gen++
			r.decoder.Reset(r.gen)
			sink.Flushed(r.gen)
			continue
		case windows.WAIT_OBJECT_0:
		default:
			return errors.New("keyboard: the console input wait answered an unexpected handle")
		}
		read, err := readConsoleInput(r.in, records)
		if err != nil {
			return err
		}
		events, resized := r.decoder.Feed(inputRecords(records[:read]))
		if r.decoder.SawPaste() {
			r.sawPaste.Store(true)
		}
		for _, event := range events {
			sink.Event(event)
		}
		if resized {
			sink.Resized()
		}
	}
}

// Stop ends Run. It is safe to call more than once.
func (r *consoleReader) Stop() {
	r.once.Do(func() { windows.SetEvent(r.cancel) })
}

// RequestFlush asks Run to discard the input waiting in the console buffer
// and in the decoder, and to confirm through the sink when it has.
func (r *consoleReader) RequestFlush() {
	windows.SetEvent(r.flush)
}

// SawPaste reports whether a paste start marker has been read.
func (r *consoleReader) SawPaste() bool {
	return r.sawPaste.Load()
}

// readConsoleInput reads the records waiting, up to the length of records,
// and answers how many it read.
func readConsoleInput(in windows.Handle, records []rawInputRecord) (int, error) {
	var read uint32
	ok, _, err := procReadConsoleInput.Call(
		uintptr(in),
		uintptr(unsafe.Pointer(&records[0])),
		uintptr(len(records)),
		uintptr(unsafe.Pointer(&read)),
	)
	if ok == 0 {
		return 0, err
	}
	return int(read), nil
}

// inputRecords converts the console's records into the decoder's.
func inputRecords(raw []rawInputRecord) []screen.InputRecord {
	records := make([]screen.InputRecord, 0, len(raw))
	for _, record := range raw {
		records = append(records, screen.InputRecord{
			EventType:       record.eventType,
			KeyDown:         record.keyDown != 0,
			RepeatCount:     record.repeatCount,
			VirtualKeyCode:  record.virtualKeyCode,
			UnicodeChar:     record.unicodeChar,
			ControlKeyState: record.controlKeyState,
		})
	}
	return records
}

// WindowSize answers the width and height of the console window out shows,
// from the srWindow rectangle GetConsoleScreenBufferInfo reports, which the
// CONSOLE_SCREEN_BUFFER_INFO page defines as "the console screen buffer
// coordinates of the upper-left and lower-right corners of the display
// window". dwSize is never read.
func WindowSize(out *os.File) (width, height int, err error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(out.Fd()), &info); err != nil {
		return 0, 0, err
	}
	width = int(info.Window.Right) - int(info.Window.Left) + 1
	height = int(info.Window.Bottom) - int(info.Window.Top) + 1
	return width, height, nil
}

// IsTerminal reports whether f is a console, which is GetConsoleMode
// succeeding, the test the WriteConsole page names.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
