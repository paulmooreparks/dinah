package bench

import "golang.org/x/sys/windows"

// errCrossDevice is the Win32 error a rename reports when its two paths sit on
// different volumes. Microsoft's System Error Codes list gives 17 the name
// ERROR_NOT_SAME_DEVICE and the sentence "The system cannot move the file to a
// different disk drive", and MoveFileEx documents that a directory move to a
// different volume fails unless MOVEFILE_COPY_ALLOWED is passed, which
// durable.MoveDir does not pass.
//
// The name comes from golang.org/x/sys/windows, which the module already
// requires directly; the syscall package exports no name for this code on
// Windows.
var errCrossDevice error = windows.ERROR_NOT_SAME_DEVICE
