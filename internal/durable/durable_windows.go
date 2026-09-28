//go:build windows

package durable

import (
	"errors"
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shareAll is every sharing flag CreateFile takes. The CreateFile
// documentation says FILE_SHARE_DELETE "enables subsequent open operations on
// a file or device to request delete access", and without it another process
// cannot rename or delete the file while this handle is open; FILE_SHARE_WRITE
// lets an appender open a journal a reader holds.
const shareAll = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE

// replaceFlags is what a rename over a file passes to MoveFileEx.
// MOVEFILE_WRITE_THROUGH is documented as "the function does not return until
// the file is actually moved on the disk", which is what makes the rename
// itself durable on Windows.
const replaceFlags = windows.MOVEFILE_REPLACE_EXISTING | windows.MOVEFILE_WRITE_THROUGH

// moveFlags is what a directory move passes. The MoveFileEx documentation says
// MOVEFILE_REPLACE_EXISTING "cannot be used if lpNewFileName or
// lpExistingFileName names a directory", so it is left out.
const moveFlags = windows.MOVEFILE_WRITE_THROUGH

// osLockOffset is where the operating-system lock sits: one byte at 2^30,
// past the end of any record. The LockFileEx documentation says "locking a
// region that goes beyond the current end-of-file position is not an error".
const osLockOffset = 0x40000000

// fileDispositionInfo is FILE_DISPOSITION_INFO, whose one member is a BOOLEAN
// (one byte) that asks for the file to be deleted when its last handle closes.
type fileDispositionInfo struct {
	DeleteFile byte
}

// fileStandardInfo is FILE_STANDARD_INFO in its documented layout.
// golang.org/x/sys/windows has the class constant but not the type.
type fileStandardInfo struct {
	AllocationSize int64
	EndOfFile      int64
	NumberOfLinks  uint32
	DeletePending  byte
	Directory      byte
}

// utf16 converts a path for a Win32 call, wrapping the conversion error the
// way the os package wraps its own.
func utf16(op, path string) (*uint16, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: op, Path: path, Err: err}
	}
	return name, nil
}

// createFile opens path with CreateFile and wraps the handle as an *os.File.
func createFile(op, path string, access, share, disposition uint32) (*os.File, error) {
	name, err := utf16(op, path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, access, share, nil, disposition, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: op, Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

// openRead opens a file for reading with every sharing flag.
func openRead(path string) (*os.File, error) {
	return createFile("open", path, windows.GENERIC_READ, shareAll, windows.OPEN_EXISTING)
}

// openRetryable accepts a sharing violation alone. CreateFile also answers
// ERROR_ACCESS_DENIED for a name whose deletion is pending and for a
// directory opened as a file, and a read of either answers at once.
func openRetryable(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// replaceRetryable accepts a sharing violation and a denied access, the codes
// Microsoft's System Error Codes list gives for a share-mode conflict and a
// denied access, unless target carries FILE_ATTRIBUTE_READONLY, whose denial
// is permanent.
func replaceRetryable(target string) func(error) bool {
	return func(err error) bool {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return true
		}
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return false
		}
		return !readOnly(target)
	}
}

// moveRetryable accepts a sharing violation and a denied access.
func moveRetryable(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

// removeAllRetryable adds ERROR_DIR_NOT_EMPTY to moveRetryable's codes: a file
// inside the tree that another process holds with delete sharing is only
// marked for deletion, which the DeleteFile remarks describe as "the file
// deletion does not occur until the last handle to the file is closed", so its
// directory is not empty until then.
func removeAllRetryable(err error) bool {
	return moveRetryable(err) || errors.Is(err, windows.ERROR_DIR_NOT_EMPTY)
}

// createRetryable accepts a denied access, which the DeleteFile remarks say a
// name whose deletion is pending answers: "subsequent calls to CreateFile to
// open the file fail with ERROR_ACCESS_DENIED".
func createRetryable(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

// readOnly reports whether path carries FILE_ATTRIBUTE_READONLY, read with
// GetFileAttributes.
func readOnly(path string) bool {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attributes, err := windows.GetFileAttributes(name)
	if err != nil {
		return false
	}
	return attributes&windows.FILE_ATTRIBUTE_READONLY != 0
}

// replaceOnce makes one MoveFileEx attempt over an existing file.
func replaceOnce(from, to string) error {
	return moveFileEx("replace", from, to, replaceFlags)
}

// moveOnce makes one MoveFileEx attempt onto a path that does not exist.
func moveOnce(from, to string) error {
	return moveFileEx("move", from, to, moveFlags)
}

// moveFileEx reports the step to Observe and calls MoveFileEx once. A test
// fixture skipping its flushes renames without MOVEFILE_WRITE_THROUGH.
func moveFileEx(op, from, to string, flags uint32) error {
	observe(op, to, flags)
	if skipsFlush(to) {
		flags &^= windows.MOVEFILE_WRITE_THROUGH
	}
	source, err := utf16(op, from)
	if err != nil {
		return err
	}
	target, err := utf16(op, to)
	if err != nil {
		return err
	}
	if err := windows.MoveFileEx(source, target, flags); err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}

// removeOnce makes one DeleteFile attempt.
func removeOnce(path string) error {
	name, err := utf16("remove", path)
	if err != nil {
		return err
	}
	if err := windows.DeleteFile(name); err != nil {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	return nil
}

// openJournalOnce opens a journal for reading and writing with every sharing
// flag, creating it when absent. Creation is not reported on Windows, because
// only Linux flushes the directory after it.
func openJournalOnce(path string) (*os.File, bool, error) {
	access := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE)
	f, err := createFile("open", path, access, shareAll, windows.OPEN_ALWAYS)
	return f, false, err
}

// appendAtEnd writes data at the end of the file in one WriteFile call. The
// WriteFile documentation says that giving both Offset and OffsetHigh as
// 0xFFFFFFFF writes to the end of the file and is "functionally equivalent to
// previously calling the CreateFile function to open hFile using
// FILE_APPEND_DATA access".
func appendAtEnd(f *os.File, data []byte) error {
	overlapped := windows.Overlapped{Offset: 0xFFFFFFFF, OffsetHigh: 0xFFFFFFFF}
	var written uint32
	handle := windows.Handle(f.Fd())
	if err := windows.WriteFile(handle, data, &written, &overlapped); err != nil {
		return &os.PathError{Op: "write", Path: f.Name(), Err: err}
	}
	if int(written) != len(data) {
		return &os.PathError{Op: "write", Path: f.Name(), Err: io.ErrShortWrite}
	}
	return nil
}

// createExclusiveOnce makes one CREATE_NEW attempt, asking for the DELETE
// access DeleteHeld needs. The CreateFile documentation names
// ERROR_FILE_EXISTS for CREATE_NEW on an existing file, and the syscall
// package reports that code as fs.ErrExist.
func createExclusiveOnce(path string) (*os.File, error) {
	access := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE)
	return createFile("open", path, access, shareAll, windows.CREATE_NEW)
}

// openLockFileOnce opens an existing lock file for reading and writing, with
// DELETE access so that a reclaimer can release it the way its creator would.
func openLockFileOnce(path string) (*os.File, error) {
	access := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE | windows.DELETE)
	return createFile("open", path, access, shareAll, windows.OPEN_EXISTING)
}

// tryOSLock calls LockFileEx for one exclusive byte at osLockOffset, failing
// immediately rather than waiting. The documentation says locks a process
// holds when it terminates "are unlocked by the operating system". A refusal
// is ERROR_LOCK_VIOLATION; any other error is reported as one, and every
// caller treats an error as a lock it cannot rely on.
func tryOSLock(f *os.File) (bool, error) {
	overlapped := windows.Overlapped{Offset: osLockOffset}
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, &overlapped)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return false, err
}

// unlockOSLock gives back the byte tryOSLock locked. A handle that holds no
// lock answers an error, which is ignored because closing the handle follows.
func unlockOSLock(f *os.File) {
	overlapped := windows.Overlapped{Offset: osLockOffset}
	windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
}

// deleteHeld marks the file the handle refers to for deletion with
// SetFileInformationByHandle and FileDispositionInfo, which needs the DELETE
// access the lock file was opened with. The deletion happens when the last
// handle closes.
func deleteHeld(f *os.File, path string) error {
	info := fileDispositionInfo{DeleteFile: 1}
	buffer := (*byte)(unsafe.Pointer(&info))
	size := uint32(unsafe.Sizeof(info))
	err := windows.SetFileInformationByHandle(windows.Handle(f.Fd()), windows.FileDispositionInfo, buffer, size)
	if err != nil {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	return nil
}

// stillNamed reads FileStandardInfo through the handle and reports whether the
// file's deletion is not pending.
func stillNamed(f *os.File, path string) (bool, error) {
	var info fileStandardInfo
	buffer := (*byte)(unsafe.Pointer(&info))
	size := uint32(unsafe.Sizeof(info))
	err := windows.GetFileInformationByHandleEx(windows.Handle(f.Fd()), windows.FileStandardInfo, buffer, size)
	if err != nil {
		return false, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	return info.DeletePending == 0, nil
}
