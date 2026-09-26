//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package keyboard

import (
	"testing"

	"golang.org/x/sys/unix"
)

// rawInputCleared and rawInputSet are the flags section 6.1's table names:
// the input and local flags rawInput clears, and the output flags it sets.
const (
	rawInputClearedLocal = unix.ICANON | unix.ECHO | unix.ISIG | unix.IEXTEN
	rawInputClearedInput = unix.IXON | unix.ICRNL | unix.INLCR | unix.IGNCR | unix.ISTRIP | unix.BRKINT
	rawInputSetOutput    = unix.OPOST | unix.ONLCR
)

// TestRawInputChangesExactlyTheFlagsItNames is dinah-603/criteria/33. Given a
// termios with every flag of the table set and OPOST and ONLCR clear, and one
// with every flag clear, rawInput clears exactly the named input and local
// flags, sets VMIN to 1 and VTIME to 0, sets OPOST and ONLCR, and leaves every
// other bit of the four flag words as it was.
func TestRawInputChangesExactlyTheFlagsItNames(t *testing.T) {
	var full unix.Termios
	full.Iflag = ^full.Iflag
	full.Oflag = ^full.Oflag &^ rawInputSetOutput
	full.Cflag = ^full.Cflag
	full.Lflag = ^full.Lflag
	full.Cc[unix.VMIN] = 9
	full.Cc[unix.VTIME] = 9
	cases := map[string]unix.Termios{"every flag set": full, "every flag clear": {}}
	for name, before := range cases {
		after := rawInput(before)
		if want := before.Lflag &^ rawInputClearedLocal; after.Lflag != want {
			t.Errorf("%s: c_lflag is %#x, wanted %#x", name, after.Lflag, want)
		}
		if want := before.Iflag &^ rawInputClearedInput; after.Iflag != want {
			t.Errorf("%s: c_iflag is %#x, wanted %#x", name, after.Iflag, want)
		}
		if want := before.Oflag | rawInputSetOutput; after.Oflag != want {
			t.Errorf("%s: c_oflag is %#x, wanted %#x, which keeps OPOST and ONLCR set", name, after.Oflag, want)
		}
		if after.Cflag != before.Cflag {
			t.Errorf("%s: c_cflag changed from %#x to %#x", name, before.Cflag, after.Cflag)
		}
		if after.Cc[unix.VMIN] != 1 || after.Cc[unix.VTIME] != 0 {
			t.Errorf("%s: VMIN %d and VTIME %d, wanted 1 and 0", name, after.Cc[unix.VMIN], after.Cc[unix.VTIME])
		}
	}
}
