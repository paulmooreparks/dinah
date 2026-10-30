// Package filelock takes the operating system's exclusive lock on an open
// file, which the system releases when the process that holds it ends, so a
// program that crashes leaves no lock behind. The lock file itself is never
// removed by its holder: removing it would let a second holder lock a new
// file while the first still holds the old one.
package filelock

import "os"

// TryLock locks the file exclusively without waiting, and reports whether it
// did. A lock held through another open of the file, in this process or
// another, makes it report false.
func TryLock(f *os.File) (bool, error) { return tryLock(f) }

// Unlock releases a lock TryLock took.
func Unlock(f *os.File) error { return unlock(f) }
