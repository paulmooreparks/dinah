//go:build windows

package bench

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

// finalPathKernel32 holds GetFinalPathNameByHandleW, which the syscall package does
// not wrap.
var (
	finalPathKernel32         = syscall.NewLazyDLL("kernel32.dll")
	getFinalPathNameByHandleW = finalPathKernel32.NewProc("GetFinalPathNameByHandleW")
)

// Constants from the Win32 documentation cited on FinalPath.
const (
	// fileReadAttributes is FILE_READ_ATTRIBUTES, the access right to read a
	// file's attributes.
	fileReadAttributes = 0x80
	// fileNameNormalized is FILE_NAME_NORMALIZED together with
	// VOLUME_NAME_DOS, both zero, which asks for the normalised drive-letter
	// form of the path.
	fileNameNormalized = 0x0
)

// FinalPath answers the path the filesystem finally reaches for an existing
// file or directory, with every symbolic link, junction and other name
// surrogate along it resolved by the operating system itself.
//
// It does not rest on filepath.EvalSymlinks, which since Go 1.23 no longer
// evaluates mount points on Windows, and a junction is a mount point. It rests
// on two documented Win32 calls instead:
//
//   - CreateFileW, which opens the object a path names and follows reparse
//     points unless FILE_FLAG_OPEN_REPARSE_POINT is given ("If
//     FILE_FLAG_OPEN_REPARSE_POINT is not specified ... the function opens the
//     target file"), and which opens a directory only with
//     FILE_FLAG_BACKUP_SEMANTICS
//     (https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-createfilew).
//   - GetFinalPathNameByHandleW, which "retrieves the final path for the
//     specified file" from the open handle
//     (https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-getfinalpathnamebyhandlew).
//
// The answer carries the \\?\ prefix those calls document, which is removed
// so the path compares with the ones Go builds; a UNC answer, \\?\UNC\server,
// becomes \\server.
func FinalPath(path string) (string, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	share := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE)
	handle, err := syscall.CreateFile(name, fileReadAttributes, share, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(handle)
	buf := make([]uint16, 512)
	for {
		n, _, callErr := getFinalPathNameByHandleW.Call(uintptr(handle), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), fileNameNormalized)
		if n == 0 {
			return "", callErr
		}
		if int(n) < len(buf) {
			return trimVerbatim(syscall.UTF16ToString(buf[:n])), nil
		}
		if int(n) > 1<<16 {
			return "", errors.New("the final path is longer than Windows allows")
		}
		buf = make([]uint16, n+1)
	}
}

// trimVerbatim removes the \\?\ prefix GetFinalPathNameByHandleW writes.
func trimVerbatim(path string) string {
	if rest, ok := strings.CutPrefix(path, `\\?\UNC\`); ok {
		return `\\` + rest
	}
	return strings.TrimPrefix(path, `\\?\`)
}
