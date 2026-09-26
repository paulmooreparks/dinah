package screen

// The two cursor movements the terminal UI writes to the Windows console in
// place of a line feed and a carriage return: cursor down one line (CUD) and
// cursor to column 1 (CHA), spelled as Microsoft's "Console Virtual Terminal
// Sequences" page lists them under Cursor Positioning. The Windows console
// has no terminfo entry to read them from, so they are written here, in the
// terminal layer, as the page spells them.
const (
	ConsoleCursorDown      = "\x1b[B"
	ConsoleCursorColumnOne = "\x1b[1G"
)
