package screen

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The two magic numbers term(5) documents for a compiled entry: the legacy
// storage format, whose numbers are 16-bit, and the extended number format
// ncurses 6.1 introduced, whose numbers are 32-bit and which otherwise has
// the legacy layout.
const (
	magicLegacy         = 0o432
	magicExtendedNumber = 0o1036
)

// The positions, in the Boolean, number and string sections, of the
// capabilities this package reads. term(5) says the sections "are in the same
// order as in the header file term.h", and these are the positions term.h
// gives them. The positions of clear and cup can be read off the compiled
// adm3a entry term(5) prints under EXAMPLES, and the rest are checked against
// the compiled xterm entries committed under testdata.
const (
	boolXonXoff = 20
	numColors   = 13
	strClear    = 5
	strEl       = 6
	strEd       = 7
	strCup      = 10
	strCivis    = 13
	strCnorm    = 16
	strSgr0     = 39
	strSetaf    = 359
)

// ErrNoTerminfo is a terminal type for which no compiled description could
// be found or read.
var ErrNoTerminfo = errors.New("screen: no terminfo description")

// Terminfo is the part of one compiled terminal description this package
// reads: its Boolean, number and string capabilities, each by its position
// in term.h's order.
type Terminfo struct {
	// Names is the terminal names section, the names separated by "|".
	Names    string
	booleans []bool
	numbers  []int
	strings  []string
	present  []bool
}

// Flag reports a Boolean capability by position, false where it is absent.
func (t *Terminfo) Flag(index int) bool {
	return index < len(t.booleans) && t.booleans[index]
}

// Numeric reports a number capability by position, and false where it is
// absent or cancelled.
func (t *Terminfo) Numeric(index int) (int, bool) {
	if index >= len(t.numbers) || t.numbers[index] < 0 {
		return 0, false
	}
	return t.numbers[index], true
}

// String reports a string capability by position, as the compiled entry
// stores it, and false where it is absent or cancelled.
func (t *Terminfo) String(index int) (string, bool) {
	if index >= len(t.strings) || !t.present[index] {
		return "", false
	}
	return t.strings[index], true
}

// ParseTerminfo reads a compiled description in either format term(5)
// documents. Everything after the string table, which is where ncurses puts
// its extended capabilities, is ignored, since this package reads none of
// them.
func ParseTerminfo(data []byte) (*Terminfo, error) {
	r := &reader{data: data}
	magic := r.short()
	numberSize := 2
	switch magic {
	case magicLegacy:
	case magicExtendedNumber:
		numberSize = 4
	default:
		return nil, fmt.Errorf("screen: terminfo magic number %o is neither legacy nor extended-number", magic)
	}
	namesSize := r.short()
	boolCount := r.short()
	numberCount := r.short()
	stringCount := r.short()
	tableSize := r.short()
	if r.err != nil || namesSize < 0 || boolCount < 0 || numberCount < 0 || stringCount < 0 || tableSize < 0 {
		return nil, errors.New("screen: terminfo header is malformed")
	}
	names := r.take(namesSize)
	t := &Terminfo{Names: string(bytes.TrimRight(names, "\x00"))}
	for _, b := range r.take(boolCount) {
		t.booleans = append(t.booleans, b == 1)
	}
	if (namesSize+boolCount)%2 == 1 {
		r.take(1)
	}
	for i := 0; i < numberCount; i++ {
		if numberSize == 4 {
			t.numbers = append(t.numbers, r.long())
			continue
		}
		t.numbers = append(t.numbers, r.short())
	}
	offsets := make([]int, stringCount)
	for i := range offsets {
		offsets[i] = r.short()
	}
	table := r.take(tableSize)
	if r.err != nil {
		return nil, r.err
	}
	t.strings = make([]string, stringCount)
	t.present = make([]bool, stringCount)
	for i, offset := range offsets {
		if offset < 0 {
			continue
		}
		if offset >= len(table) {
			return nil, fmt.Errorf("screen: terminfo string %d points past the string table", i)
		}
		end := bytes.IndexByte(table[offset:], 0)
		if end < 0 {
			return nil, fmt.Errorf("screen: terminfo string %d is not terminated", i)
		}
		t.strings[i] = string(table[offset : offset+end])
		t.present[i] = true
	}
	return t, nil
}

// reader walks a compiled entry, remembering the first short read.
type reader struct {
	data []byte
	at   int
	err  error
}

func (r *reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.at+n > len(r.data) {
		r.err = errors.New("screen: terminfo entry is shorter than its header says")
		return nil
	}
	taken := r.data[r.at : r.at+n]
	r.at += n
	return taken
}

// short reads a little-endian signed 16-bit integer, which term(5) says every
// short integer of the file is.
func (r *reader) short() int {
	b := r.take(2)
	if b == nil {
		return 0
	}
	return int(int16(binary.LittleEndian.Uint16(b)))
}

// long reads a little-endian signed 32-bit integer, the number format of the
// extended number format.
func (r *reader) long() int {
	b := r.take(4)
	if b == nil {
		return 0
	}
	return int(int32(binary.LittleEndian.Uint32(b)))
}

// LoadTerminfo finds and reads the compiled description of a terminal type,
// searching where terminfo(5) says under "Fetching Compiled Descriptions":
// the TERMINFO variable, which names one database or carries a description
// itself after "hex:" or "b64:", then $HOME/.terminfo, then each directory
// TERMINFO_DIRS lists, an empty member meaning the system location, and
// finally the system locations. getenv reads the environment, so a test
// states every variable it depends on.
//
// terminfo(5) leaves the system locations to the build, as "a compiled-in
// list". The list here is the one Debian's ncurses build compiles in,
// /etc/terminfo:/usr/share/terminfo:/lib/terminfo in its debian/rules, whose
// middle member is also where macOS keeps its database. A system whose
// ncurses was built with some other list, and which sets none of the three
// variables, finds nothing here, and the terminal is then treated as having
// no description: no colour and no watch, which is safe.
//
// A database may be a directory tree or a hashed database, and only the
// tree is read. Each tree is tried both ways term(5) documents for the
// intermediate directory: the name's first character, and that character in
// two-digit hexadecimal, which is what a filesystem that ignores case uses.
func LoadTerminfo(name string, getenv func(string) string) (*Terminfo, error) {
	if name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return nil, ErrNoTerminfo
	}
	var databases []string
	if value := getenv("TERMINFO"); value != "" {
		if entry, ok := encodedTerminfo(value, name); ok {
			return entry, nil
		}
		databases = append(databases, value)
	}
	if home := getenv("HOME"); home != "" {
		databases = append(databases, filepath.Join(home, ".terminfo"))
	}
	if dirs, set := lookupDirs(getenv); set {
		for _, dir := range strings.Split(dirs, ":") {
			if dir == "" {
				databases = append(databases, systemTerminfo...)
				continue
			}
			databases = append(databases, dir)
		}
	}
	databases = append(databases, systemTerminfo...)
	for _, database := range databases {
		for _, middle := range []string{name[:1], hex.EncodeToString([]byte{name[0]})} {
			data, err := os.ReadFile(filepath.Join(database, middle, name))
			if err != nil {
				continue
			}
			entry, err := ParseTerminfo(data)
			if err != nil {
				continue
			}
			return entry, nil
		}
	}
	return nil, ErrNoTerminfo
}

// systemTerminfo is the system locations searched last, in the order
// Debian's build lists them.
var systemTerminfo = []string{"/etc/terminfo", "/usr/share/terminfo", "/lib/terminfo"}

// lookupDirs reads TERMINFO_DIRS, reporting whether it is set to anything.
func lookupDirs(getenv func(string) string) (string, bool) {
	dirs := getenv("TERMINFO_DIRS")
	return dirs, dirs != ""
}

// encodedTerminfo reads a description TERMINFO carries itself, which
// terminfo(5) says ncurses uses when it "matches the name sought".
func encodedTerminfo(value, name string) (*Terminfo, bool) {
	var data []byte
	var err error
	switch {
	case strings.HasPrefix(value, "hex:"):
		data, err = hex.DecodeString(strings.TrimPrefix(value, "hex:"))
	case strings.HasPrefix(value, "b64:"):
		data, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "b64:"))
	default:
		return nil, false
	}
	if err != nil {
		return nil, false
	}
	entry, err := ParseTerminfo(data)
	if err != nil {
		return nil, false
	}
	for _, alias := range strings.Split(entry.Names, "|") {
		if alias == name {
			return entry, true
		}
	}
	return nil, false
}
