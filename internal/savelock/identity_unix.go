//go:build !windows

package savelock

import (
	"encoding/binary"

	"golang.org/x/sys/unix"
)

// folderIdentity is the device and inode of a folder, which stat gives after
// following any link to it.
func folderIdentity(folder string) ([]byte, error) {
	var st unix.Stat_t
	if err := unix.Stat(folder, &st); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 24)
	out = append(out, "inode\x00"...)
	out = binary.LittleEndian.AppendUint64(out, uint64(st.Dev))
	return binary.LittleEndian.AppendUint64(out, uint64(st.Ino)), nil
}
