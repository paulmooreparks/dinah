package bench

import (
	"errors"
	"syscall"
)

// errSharingViolation is the Win32 error a rename or an open reports when
// another handle's share mode refuses it. Microsoft's System Error Codes list
// gives 32 the name ERROR_SHARING_VIOLATION and the sentence "The process
// cannot access the file because it is being used by another process." The
// syscall package exports no name for it on Windows, which is why the number
// is written out here beside its citation.
var errSharingViolation error = syscall.Errno(32)

// transientRenameRefusal reports whether a rename was refused because a
// handle is open below the directory, which ends when the handle closes:
// ERROR_ACCESS_DENIED or ERROR_SHARING_VIOLATION. A refusal that is truly a
// permission is retried too, and it answers the same way once the budget runs
// out, which costs that act the budget and nothing else.
func transientRenameRefusal(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED) || errors.Is(err, errSharingViolation)
}

// transientRemoveRefusal is transientRenameRefusal for a removal, which also
// meets ERROR_DIR_NOT_EMPTY while a file it deleted is only marked for
// deletion because a handle to it is still open.
func transientRemoveRefusal(err error) bool {
	return transientRenameRefusal(err) || errors.Is(err, syscall.ERROR_DIR_NOT_EMPTY)
}
