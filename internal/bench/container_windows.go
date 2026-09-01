package bench

import "syscall"

// errCrossDevice is the Win32 error a rename reports when its two paths sit on
// different volumes. Microsoft's System Error Codes list gives 17 the name
// ERROR_NOT_SAME_DEVICE and the sentence "The system cannot move the file to a
// different disk drive", and MoveFileEx documents that a directory move to a
// different volume fails unless MOVEFILE_COPY_ALLOWED is passed, which the
// standard library's os.Rename does not pass.
//
// The number is written out because the syscall package exports no name for
// it on Windows and this module takes no dependency outside the standard
// library. The comment above is the citation that keeps it from being a magic
// number.
var errCrossDevice error = syscall.Errno(17)
