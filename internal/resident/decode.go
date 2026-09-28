package resident

import (
	"encoding/binary"
	"errors"
	"syscall"
	"unicode/utf16"
)

// The completion errors decode classifies, with the values Microsoft's
// system error codes give them. They are declared here rather than taken from
// golang.org/x/sys/windows, so decode and its test run on every platform.
const (
	// errorOperationAborted is ERROR_OPERATION_ABORTED: "The I/O operation
	// has been aborted because of either a thread exit or an application
	// request."
	errorOperationAborted syscall.Errno = 995
	// errorNotifyEnumDir is ERROR_NOTIFY_ENUM_DIR: "A notify change request
	// is being completed and the information is not being returned in the
	// caller's buffer. The caller now needs to enumerate the files to find
	// the changes."
	errorNotifyEnumDir syscall.Errno = 1022
)

// recordHeader is the size of FILE_NOTIFY_INFORMATION's three leading
// members: NextEntryOffset, Action and FileNameLength, each a DWORD.
const recordHeader = 12

// decode classifies one ReadDirectoryChangesW completion and reads its
// records. n is the byte count the completion reported, err its error, and
// closing whether Close had begun when it was consumed.
//
//   - err nil and n zero: the buffer overflowed, and the changes were lost.
//   - ERROR_NOTIFY_ENUM_DIR: the same news, told as an error.
//   - ERROR_OPERATION_ABORTED while closing: the notifier is closed.
//   - any other error, an abort without closing included: the watch failed.
//   - err nil and n above zero: the records.
//
// A record whose name runs past n, whose offset is not a multiple of 4, or
// whose action is outside 1 to 5 makes the whole completion an overflow,
// because a buffer that will not decode has lost its records as surely as an
// empty one.
func decode(buf []byte, n uint32, err error, closing bool) (Batch, error) {
	var errno syscall.Errno
	isErrno := errors.As(err, &errno)
	switch {
	case err == nil && n == 0:
		return Batch{Overflow: true}, nil
	case isErrno && errno == errorNotifyEnumDir:
		return Batch{Overflow: true}, nil
	case isErrno && errno == errorOperationAborted && closing:
		return Batch{}, ErrClosed
	case err != nil:
		return Batch{}, err
	}
	if int(n) > len(buf) {
		return Batch{Overflow: true}, nil
	}
	data := buf[:n]
	var changes []Change
	offset := 0
	for {
		if offset%4 != 0 || offset+recordHeader > len(data) {
			return Batch{Overflow: true}, nil
		}
		next := binary.LittleEndian.Uint32(data[offset:])
		action := binary.LittleEndian.Uint32(data[offset+4:])
		length := binary.LittleEndian.Uint32(data[offset+8:])
		start := offset + recordHeader
		if length%2 != 0 || uint64(start)+uint64(length) > uint64(len(data)) {
			return Batch{Overflow: true}, nil
		}
		if action < uint32(Added) || action > uint32(RenamedNew) {
			return Batch{Overflow: true}, nil
		}
		units := make([]uint16, length/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(data[start+2*i:])
		}
		changes = append(changes, Change{Path: string(utf16.Decode(units)), Action: Action(action)})
		if next == 0 {
			break
		}
		if next%4 != 0 {
			return Batch{Overflow: true}, nil
		}
		offset += int(next)
	}
	return Batch{Changes: changes}, nil
}
