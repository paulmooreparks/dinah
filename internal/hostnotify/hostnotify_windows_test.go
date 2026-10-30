//go:build windows && amd64

package hostnotify

import (
	"testing"
	"unsafe"
)

// The structures passed to Win32 must have the sizes of their C
// definitions on 64-bit Windows. The test calls nothing and shows nothing.
func TestStructureSizesMatchWin32(t *testing.T) {
	for _, c := range []struct {
		name      string
		got, want uintptr
	}{
		{"NOTIFYICONDATAW", unsafe.Sizeof(notifyIconData{}), 976},
		{"WNDCLASSEXW", unsafe.Sizeof(wndClassEx{}), 80},
		{"MSG", unsafe.Sizeof(winMsg{}), 48},
		{"FLASHWINFO", unsafe.Sizeof(flashWInfo{}), 32},
	} {
		if c.got != c.want {
			t.Errorf("%s is %d bytes, want %d", c.name, c.got, c.want)
		}
	}
}

func TestCopyUTF16ShortensWithoutSplittingAPair(t *testing.T) {
	var buf [4]uint16
	copyUTF16(buf[:], "ab\U0001F600")
	if buf[0] != 'a' || buf[1] != 'b' || buf[2] != 0 {
		t.Fatalf("got %v", buf)
	}
	copyUTF16(buf[:], "abcdef")
	if buf[2] != 'c' || buf[3] != 0 {
		t.Fatalf("got %v", buf)
	}
}
