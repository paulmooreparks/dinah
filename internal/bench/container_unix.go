//go:build !windows

package bench

import "syscall"

// errCrossDevice is the errno a rename reports when its two paths sit on
// different filesystems. POSIX names EXDEV for exactly that condition in
// rename(2), and the syscall package exports it under that name on every
// platform this file builds for.
var errCrossDevice error = syscall.EXDEV
