//go:build windows

package screen

import (
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// The kernel32 functions golang.org/x/sys/windows does not bind. Each is a
// documented export of kernel32.dll, named on its own page under
// https://learn.microsoft.com/en-us/windows/console/.
var (
	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procSetConsoleTextAttribute    = kernel32.NewProc("SetConsoleTextAttribute")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterW")
	procFillConsoleOutputAttribute = kernel32.NewProc("FillConsoleOutputAttribute")
	procGetConsoleCursorInfo       = kernel32.NewProc("GetConsoleCursorInfo")
	procSetConsoleCursorInfo       = kernel32.NewProc("SetConsoleCursorInfo")
)

// isTerminal reports whether f is a console, which is GetConsoleMode
// succeeding, the test the WriteConsole page names.
func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// openTerminal opens the console layer on the console f is attached to.
func openTerminal(f *os.File, text io.Writer) Screen {
	return NewConsole(OpenConsole(windows.Handle(f.Fd())), text)
}

// OpenConsole binds the Console interface to the console screen buffer a
// handle names.
func OpenConsole(handle windows.Handle) Console {
	return windowsConsole{handle: handle}
}

// windowsConsole calls the real functions on one screen buffer handle.
type windowsConsole struct {
	handle windows.Handle
}

// cursorInfo is CONSOLE_CURSOR_INFO: dwSize, then bVisible as a BOOL.
type cursorInfo struct {
	size    uint32
	visible int32
}

// packCoord is a COORD passed by value. Microsoft's "x64 calling convention"
// page documents that a structure of 32 bits is passed as if it were an
// integer of that size, and COORD is two 16-bit members with X first, so the
// word carries X in its low half and Y in its high half. x/sys/windows passes
// the COORD of SetConsoleCursorPosition the same way.
func packCoord(x, y int) uintptr {
	coord := windows.Coord{X: int16(x), Y: int16(y)}
	return uintptr(*(*uint32)(unsafe.Pointer(&coord)))
}

func (w windowsConsole) ScreenBufferInfo() (BufferInfo, error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(w.handle, &info); err != nil {
		return BufferInfo{}, err
	}
	return BufferInfo{
		CursorX:    int(info.CursorPosition.X),
		CursorY:    int(info.CursorPosition.Y),
		Attributes: info.Attributes,
		Left:       int(info.Window.Left),
		Top:        int(info.Window.Top),
		Right:      int(info.Window.Right),
		Bottom:     int(info.Window.Bottom),
	}, nil
}

func (w windowsConsole) SetCursorPosition(x, y int) error {
	return windows.SetConsoleCursorPosition(w.handle, windows.Coord{X: int16(x), Y: int16(y)})
}

func (w windowsConsole) SetTextAttribute(attributes uint16) error {
	return call(procSetConsoleTextAttribute, uintptr(w.handle), uintptr(attributes))
}

func (w windowsConsole) FillCharacter(character rune, length, x, y int) error {
	var written uint32
	return call(
		procFillConsoleOutputCharacter,
		uintptr(w.handle),
		uintptr(uint16(character)),
		uintptr(uint32(length)),
		packCoord(x, y),
		uintptr(unsafe.Pointer(&written)),
	)
}

func (w windowsConsole) FillAttribute(attributes uint16, length, x, y int) error {
	var written uint32
	return call(
		procFillConsoleOutputAttribute,
		uintptr(w.handle),
		uintptr(attributes),
		uintptr(uint32(length)),
		packCoord(x, y),
		uintptr(unsafe.Pointer(&written)),
	)
}

func (w windowsConsole) CursorInfo() (uint32, bool, error) {
	var info cursorInfo
	if err := call(procGetConsoleCursorInfo, uintptr(w.handle), uintptr(unsafe.Pointer(&info))); err != nil {
		return 0, false, err
	}
	return info.size, info.visible != 0, nil
}

func (w windowsConsole) SetCursorInfo(size uint32, visible bool) error {
	info := cursorInfo{size: size}
	if visible {
		info.visible = 1
	}
	return call(procSetConsoleCursorInfo, uintptr(w.handle), uintptr(unsafe.Pointer(&info)))
}

// call invokes a kernel32 function whose documented failure is a zero
// return, and reports GetLastError's answer as the error.
func call(proc *windows.LazyProc, args ...uintptr) error {
	if err := proc.Find(); err != nil {
		return err
	}
	result, _, err := proc.Call(args...)
	if result == 0 {
		return err
	}
	return nil
}
