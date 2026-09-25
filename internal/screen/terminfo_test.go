package screen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// testdataTerminfo is the directory tree the committed compiled entries sit
// in. xterm and xterm-256color are ncurses 6.5's own entries, copied byte for
// byte; the dinah-* entries were compiled by the same tic from
// dinah-test.src beside them.
var testdataTerminfo = filepath.Join("testdata", "terminfo")

// readEntry reads one committed compiled entry.
func readEntry(t *testing.T, middle, name string) *Terminfo {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataTerminfo, middle, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	entry, err := ParseTerminfo(data)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return entry
}

// TestTheReaderReadsBothCompiledFormats is dinah-288/criteria/31's reader
// half. xterm is stored in the legacy format, magic 0432 with 16-bit
// numbers, and xterm-256color in the extended number format, magic 01036
// with 32-bit numbers. Each capability the layer reads is compared with what
// infocmp prints for the same entry, and cup and setaf are evaluated to the
// bytes the entry specifies for chosen parameters.
func TestTheReaderReadsBothCompiledFormats(t *testing.T) {
	cases := []struct {
		name    string
		magic   string
		colours int
		setaf   map[int]string
		sgr0    string
		cnorm   string
	}{
		{
			name: "xterm", magic: "\x1a\x01", colours: 8,
			setaf: map[int]string{1: "\x1b[31m", 4: "\x1b[34m"},
			sgr0:  "\x1b(B\x1b[m", cnorm: "\x1b[?12l\x1b[?25h",
		},
		{
			name: "xterm-256color", magic: "\x1e\x02", colours: 256,
			setaf: map[int]string{1: "\x1b[31m", 12: "\x1b[94m", 200: "\x1b[38;5;200m"},
			sgr0:  "\x1b(B\x1b[m", cnorm: "\x1b[?12l\x1b[?25h",
		},
	}
	for _, c := range cases {
		data, err := os.ReadFile(filepath.Join(testdataTerminfo, "78", c.name))
		if err != nil {
			t.Fatalf("read %s: %v", c.name, err)
		}
		if got := string(data[:2]); got != c.magic {
			t.Fatalf("%s starts %q, so the fixture is not the format this case is about", c.name, got)
		}
		entry := readEntry(t, "78", c.name)
		if !strings.HasPrefix(entry.Names, c.name+"|") {
			t.Errorf("%s names itself %q", c.name, entry.Names)
		}
		if colours, ok := entry.Numeric(numColors); !ok || colours != c.colours {
			t.Errorf("%s colors = %d, %v, want %d", c.name, colours, ok, c.colours)
		}
		want := map[int]string{
			strClear: "\x1b[H\x1b[2J",
			strEl:    "\x1b[K",
			strEd:    "\x1b[J",
			strCivis: "\x1b[?25l",
			strCnorm: c.cnorm,
			strSgr0:  c.sgr0,
		}
		for index, text := range want {
			if got, ok := entry.String(index); !ok || got != text {
				t.Errorf("%s string %d = %q, %v, want %q", c.name, index, got, ok, text)
			}
		}
		cup, _ := entry.String(strCup)
		if got, err := Evaluate(cup, 4, 9); err != nil || got != "\x1b[5;10H" {
			t.Errorf("%s cup(4, 9) = %q, %v, want ESC[5;10H", c.name, got, err)
		}
		setaf, _ := entry.String(strSetaf)
		for colour, text := range c.setaf {
			if got, err := Evaluate(setaf, colour); err != nil || got != text {
				t.Errorf("%s setaf(%d) = %q, %v, want %q", c.name, colour, got, err, text)
			}
		}
		if entry.Flag(boolXonXoff) {
			t.Errorf("%s reads as carrying xon, which infocmp does not list", c.name)
		}
	}
	if !readEntry(t, "64", "dinah-xon").Flag(boolXonXoff) {
		t.Error("dinah-xon was compiled with xon and does not read as carrying it")
	}
}

// TestAnUnsupportedOperationReadsAsAMissingCapability is the other half of
// dinah-288/criteria/31: an entry whose setaf uses %Q, which terminfo(5)
// does not define, colours nothing, and an entry with no cup is refused for
// the watch naming cup.
func TestAnUnsupportedOperationReadsAsAMissingCapability(t *testing.T) {
	unsupported := NewTerminfo(readEntry(t, "64", "dinah-unsupported"), &strings.Builder{})
	if unsupported.Colour() {
		t.Error("a setaf using %Q still reads as able to colour")
	}
	if ok, _, _ := unsupported.Live(); !ok {
		t.Error("an entry whose only defect is setaf is refused for the watch")
	}
	noCup := NewTerminfo(readEntry(t, "64", "dinah-no-cup"), &strings.Builder{})
	if ok, reason, extra := noCup.Live(); ok || reason != contract.WatchMissingCapability || extra != "cup" {
		t.Errorf("an entry lacking cup answered %v %q %q", ok, reason, extra)
	}
	none := NewTerminfo(nil, &strings.Builder{})
	if ok, reason, _ := none.Live(); ok || reason != contract.WatchNoTerminalDescription || none.Colour() {
		t.Errorf("no description answered %v %q, colour %v", ok, reason, none.Colour())
	}
}

// TestDelaysAreDroppedOnlyUnderXon asserts the handling of terminfo(5)'s
// delays: under xon a delay is dropped from what is written, and without it
// a string carrying one reads as unusable.
func TestDelaysAreDroppedOnlyUnderXon(t *testing.T) {
	var written strings.Builder
	xon := NewTerminfo(readEntry(t, "64", "dinah-xon"), &written)
	if ok, reason, extra := xon.Live(); !ok {
		t.Fatalf("an xon entry with delays was refused: %s %s", reason, extra)
	}
	if err := xon.Row(3, []Segment{{Text: "abc"}}); err != nil {
		t.Fatalf("row: %v", err)
	}
	if got := written.String(); got != "\x1b[3;1Habc\x1b[K" {
		t.Errorf("an xon row wrote %q", got)
	}
	delay := NewTerminfo(readEntry(t, "64", "dinah-delay"), &strings.Builder{})
	if ok, reason, extra := delay.Live(); ok || reason != contract.WatchMissingCapability || extra != "cup" {
		t.Errorf("a delay without xon answered %v %q %q", ok, reason, extra)
	}
	for _, text := range []string{"5", "5.5", "5*", "5/", "5*/"} {
		if !isDelay(text) {
			t.Errorf("%q is a delay terminfo(5) documents and reads as none", text)
		}
	}
	for _, text := range []string{"", ".5", "x", "5x"} {
		if isDelay(text) {
			t.Errorf("%q reads as a delay", text)
		}
	}
}

// TestTheEvaluatorFollowsTheDocumentedLanguage runs each operation
// terminfo(5) lists under "Parameterized Strings" once.
func TestTheEvaluatorFollowsTheDocumentedLanguage(t *testing.T) {
	cases := []struct {
		capability string
		params     []int
		want       string
	}{
		{"%%", nil, "%"},
		{"%p1%d", []int{42}, "42"},
		{"%p1%3d|", []int{7}, "  7|"},
		{"%p1%:-3d|", []int{7}, "7  |"},
		{"%p1%o %p1%x %p1%X", []int{255}, "377 ff FF"},
		{"%p1%c", []int{65}, "A"},
		{"%p9%d", []int{1, 2, 3, 4, 5, 6, 7, 8, 9}, "9"},
		{"%p1%Pa%ga%ga%+%d", []int{4}, "8"},
		{"%p1%PZ%gZ%d", []int{6}, "6"},
		{"%'A'%d", nil, "65"},
		{"%{12}%{5}%-%d", nil, "7"},
		{"%{12}%{5}%*%d", nil, "60"},
		{"%{12}%{5}%/%d %{12}%{5}%m%d", nil, "2 2"},
		{"%{12}%{0}%/%d", nil, "0"},
		{"%{12}%{10}%&%d %{12}%{10}%|%d %{12}%{10}%^%d", nil, "8 14 6"},
		{"%{1}%{2}%=%d %{1}%{2}%>%d %{1}%{2}%<%d", nil, "0 0 1"},
		{"%{1}%{0}%A%d %{1}%{0}%O%d", nil, "0 1"},
		{"%{0}%!%d %{0}%~%d", nil, "1 -1"},
		{"%i%p1%d;%p2%d", []int{0, 0}, "1;1"},
		{"%?%p1%t yes%e no%;", []int{1}, " yes"},
		{"%?%p1%t yes%e no%;", []int{0}, " no"},
		{"%?%p1%{1}%=%tone%e%p1%{2}%=%ttwo%eelse%;", []int{2}, "two"},
		{"%?%p1%t%?%p2%tboth%eone%;%eneither%;", []int{1, 0}, "one"},
	}
	for _, c := range cases {
		got, err := Evaluate(c.capability, c.params...)
		if err != nil || got != c.want {
			t.Errorf("Evaluate(%q, %v) = %q, %v, want %q", c.capability, c.params, got, err, c.want)
		}
	}
	for _, bad := range []string{"%Q", "%p0", "%d", "%{12", "%s", "%l", "%p1%s"} {
		if got, err := Evaluate(bad, 1); err == nil {
			t.Errorf("Evaluate(%q) = %q with no error", bad, got)
		}
	}
}

// TestTheSearchFollowsTheDocumentedOrder asserts LoadTerminfo's search:
// TERMINFO first, $HOME/.terminfo next, TERMINFO_DIRS after, with both
// spellings of the intermediate directory, and a description carried in
// TERMINFO itself.
func TestTheSearchFollowsTheDocumentedOrder(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(name string) string { return vars[name] }
	}
	if _, err := LoadTerminfo("xterm", env(map[string]string{"TERMINFO": testdataTerminfo})); err != nil {
		t.Errorf("TERMINFO naming the tree did not find xterm under its hexadecimal directory: %v", err)
	}
	lettered := t.TempDir()
	if err := os.MkdirAll(filepath.Join(lettered, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(testdataTerminfo, "78", "xterm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lettered, "x", "xterm"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTerminfo("xterm", env(map[string]string{"TERMINFO_DIRS": t.TempDir() + ":" + lettered})); err != nil {
		t.Errorf("TERMINFO_DIRS did not reach its second member's lettered directory: %v", err)
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".terminfo", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".terminfo", "x", "xterm"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTerminfo("xterm", env(map[string]string{"HOME": home})); err != nil {
		t.Errorf("$HOME/.terminfo was not searched: %v", err)
	}
	if _, err := LoadTerminfo("xterm", env(map[string]string{"TERMINFO": "hex:" + hexOf(data)})); err != nil {
		t.Errorf("a description carried in TERMINFO was not read: %v", err)
	}
	// The name is one no terminfo database carries, since the search goes on
	// to the system's own directories, where vt52 or any real name may be.
	if _, err := LoadTerminfo("dinah-no-such-terminal", env(map[string]string{"TERMINFO": "hex:" + hexOf(data), "TERMINFO_DIRS": t.TempDir()})); err == nil {
		t.Error("a carried description answered for a name it does not carry")
	}
	for _, name := range []string{"", "..", "x/y", `x\y`} {
		if _, err := LoadTerminfo(name, env(map[string]string{"TERMINFO": testdataTerminfo})); err == nil {
			t.Errorf("the terminal name %q was looked up", name)
		}
	}
}

// hexOf encodes bytes the way TERMINFO's hex: form carries them.
func hexOf(data []byte) string {
	const digits = "0123456789abcdef"
	var b strings.Builder
	for _, c := range data {
		b.WriteByte(digits[c>>4])
		b.WriteByte(digits[c&0xf])
	}
	return b.String()
}

// TestATerminfoRowIsOneWrite is the byte-seam half of
// dinah-288/criteria/19: every row, every erase and the restore reach the
// writer as one Write each, so each sequence the entry supplied arrives
// whole within one write, and a coloured segment sits between setaf and
// sgr0.
func TestATerminfoRowIsOneWrite(t *testing.T) {
	writes := &writeRecorder{}
	scr := NewTerminfo(readEntry(t, "78", "xterm-256color"), writes)
	segments := []Segment{{Text: "● ", Colour: Blue}, {Text: "598  claude"}}
	calls := []func() error{
		scr.Begin,
		func() error { return scr.Row(2, segments) },
		func() error { return scr.EraseBelow(2) },
		func() error { return scr.Restore(24) },
	}
	for _, call := range calls {
		before := len(writes.writes)
		if err := call(); err != nil {
			t.Fatalf("call: %v", err)
		}
		if made := len(writes.writes) - before; made > 2 {
			t.Errorf("one call made %d writes", made)
		}
	}
	want := []string{
		"\x1b[?25l",
		"\x1b[H\x1b[2J",
		"\x1b[2;1H\x1b[34m● \x1b(B\x1b[m598  claude\x1b[K",
		"\x1b[3;1H\x1b[J",
		"\x1b(B\x1b[m\x1b[?12l\x1b[?25h\x1b[24;1H\n",
	}
	if strings.Join(writes.writes, "|") != strings.Join(want, "|") {
		t.Errorf("the writes were\n%q\nwant\n%q", writes.writes, want)
	}
}

// writeRecorder keeps every Write as its own string.
type writeRecorder struct {
	writes []string
}

func (w *writeRecorder) Write(p []byte) (int, error) {
	w.writes = append(w.writes, string(p))
	return len(p), nil
}
