//go:build windows

package resident

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The documented values the watcher passes. Every claim its correctness rests
// on is quoted in dinah-619's specification, section 4.1, from Microsoft's
// documentation for ReadDirectoryChangesW and FILE_NOTIFY_INFORMATION unless
// another page is named there.
const (
	// changeBufferSize is the change buffer: the largest that also satisfies
	// "ReadDirectoryChangesW fails with ERROR_INVALID_PARAMETER when the
	// buffer length is greater than 64 KB and the application is monitoring a
	// directory over the network."
	changeBufferSize = 65536
	// overlappedSpace is the room the OVERLAPPED takes at the start of the
	// region, ahead of the buffer.
	overlappedSpace = 4096
	// changeFilter is FILE_NOTIFY_CHANGE_FILE_NAME, DIR_NAME, ATTRIBUTES,
	// SIZE, LAST_WRITE and SECURITY. LAST_ACCESS is left out because the
	// watcher's own reads update last-access times wherever the volume keeps
	// them, and CREATION because no read consults a creation time.
	changeFilter = windows.FILE_NOTIFY_CHANGE_FILE_NAME | windows.FILE_NOTIFY_CHANGE_DIR_NAME |
		windows.FILE_NOTIFY_CHANGE_ATTRIBUTES | windows.FILE_NOTIFY_CHANGE_SIZE |
		windows.FILE_NOTIFY_CHANGE_LAST_WRITE | windows.FILE_NOTIFY_CHANGE_SECURITY
)

// The GetFinalPathNameByHandle flags, declared here because
// golang.org/x/sys v0.47.0 does not declare them. The function's page gives
// their values: VOLUME_NAME_DOS (0x0), "Return the path with the drive letter.
// This is the default."; VOLUME_NAME_GUID (0x1), "Return the path with a
// volume GUID path instead of the drive name."; and FILE_NAME_NORMALIZED
// (0x0), "Return the normalized drive name. This is the default."
const (
	volumeNameDOS      = 0x0
	volumeNameGUID     = 0x1
	fileNameNormalized = 0x0
)

// watchable reports whether a volume of the given drive type is watched. A
// network share, a removable drive and a RAM disk are served from disk,
// because change notification on those depends on a redirector or a file
// system this design has no documentation for.
func watchable(driveType uint32) bool {
	return driveType == windows.DRIVE_FIXED
}

// platformNotifier answers the Windows watcher. Whether it serves a given
// workbench is Arm's to decide, since it resolves the root at every Arm.
func platformNotifier(hooks *Hooks) (Notifier, error) {
	return newWinNotifier(hooks), nil
}

// winNotifier watches the root directory of the workbench's volume with
// ReadDirectoryChangesW and keeps the records below the workbench root.
//
// The watch is held on the volume's root directory, and not on the workbench
// root, because Microsoft's file system algorithms specification refuses the
// rename of a folder that "contains open files" ([MS-FSA] 2.1.5.15.12, with
// the algorithm of 2.1.4.2, and its note <186>: "On Windows NTFS, NTFS checks
// for open files beneath the directory being renamed"). A handle held on the
// workbench root would make every folder above it unrenamable while the
// server ran, and the operator ruled that those folders stay renamable,
// movable and deletable (dinah-619/questions/1). No folder has the volume
// root beneath it. The volume root is opened by its volume GUID path:
// "CreateFile processes a volume GUID path with an appended backslash as the
// root directory of the volume" ("Naming a Volume"), and bWatchSubtree
// "monitors the directory tree rooted at the specified directory", which for
// the volume root is every path on the volume.
//
// n.mu guards armed, closing, issued and running, with a cond Next broadcasts
// on return. The rule that keeps Close from hanging is that the check of
// closing and the issue of a call happen under n.mu in one critical section,
// and Close sets closing and cancels under the same lock, so no call can be
// issued after Close has looked for one to cancel.
type winNotifier struct {
	hooks *Hooks
	// closeMu is held by Close for its whole body, so a second Close waits
	// for the first to release everything.
	closeMu sync.Mutex

	mu   sync.Mutex
	cond *sync.Cond
	// armed: Arm acquired the handle, the event and the region, and Close
	// has not yet released them.
	armed bool
	// closing: Close has begun.
	closing bool
	// issued: a call is outstanding and its completion not yet consumed.
	issued bool
	// running: a Next has entered and not yet returned.
	running bool
	// consumed: a completion has been consumed since the last call was
	// issued, so the next Next runs BeforeRearm first.
	consumed bool

	root   string
	handle windows.Handle
	event  windows.Handle
	// region is the VirtualAlloc'd memory holding the OVERLAPPED at offset 0
	// and the change buffer after it. The operating system writes both after
	// the call returns, so they live where the Go collector never looks.
	region     uintptr
	overlapped *windows.Overlapped
	buffer     []byte
	// guidFinal is the workbench root's final path in its volume GUID form,
	// as the last Arm read it, and prefix the elements of the path from the
	// volume root to the workbench root.
	guidFinal string
	prefix    []element
	// moved is set when a record says the workbench root or a folder above
	// it was created, removed or renamed, and cleared by Arm.
	moved atomic.Bool
	// strayIssue records, under n.mu, a call issued after Close had begun,
	// which nobody would cancel. It is never set by correct code, and a test
	// reads it because on a watch of the whole volume any record anywhere on
	// the volume can complete such a call and let Close return anyway.
	strayIssue bool
}

// newWinNotifier answers a notifier that is not yet armed.
func newWinNotifier(hooks *Hooks) *winNotifier {
	n := &winNotifier{hooks: hooks}
	n.cond = sync.NewCond(&n.mu)
	return n
}

// resolved is what resolve reads of the workbench root.
type resolved struct {
	guidFinal  string
	volumeRoot string
	prefix     []element
}

// resolve reads where the workbench root sits on its volume, at every Arm,
// and answers the first error any step meets:
//
//  1. It opens root by path for FILE_READ_ATTRIBUTES alone.
//  2. It reads the root's final path twice, with VOLUME_NAME_GUID and with
//     VOLUME_NAME_DOS, both FILE_NAME_NORMALIZED, and closes the handle. A
//     final path "is the path that is returned when a path is fully resolved",
//     so every symbolic link, junction and subst drive on the served path is
//     already resolved. The page says both flags fail where the volume was
//     not made by the Mount Manager ("only VOLUME_NAME_NT will be
//     available"): a failed GUID query is answered as it stands, and a DOS
//     query failing after the GUID query succeeded is WhyNoDOSPath.
//  3. classify judges the pair's shape.
//  4. GetShortPathName ("Retrieves the short path form of the specified
//     path") gives each element's short form. On a volume that keeps no
//     short names its own sentence, "If the specified path is already in its
//     short form and conversion is not needed, the function simply copies the
//     specified path", makes every short form the long one.
//  5. GetVolumePathName and GetDriveType judge the volume the final path is
//     on, and anything but DRIVE_FIXED is WhyVolumeType.
func resolve(root string) (resolved, error) {
	guidFinal, dosFinal, err := finalPaths(root)
	if err != nil {
		return resolved{}, err
	}
	volumeRoot, long, why, err := classify(guidFinal, dosFinal)
	if err != nil {
		return resolved{}, err
	}
	if why != "" {
		return resolved{}, &Unsupported{Why: why}
	}
	short, err := shortPath(dosFinal)
	if err != nil {
		return resolved{}, err
	}
	_, shortRest, ok := dosDriveOf(short)
	aliases := elementsOf(shortRest)
	if !ok || len(aliases) != len(long) {
		return resolved{}, fmt.Errorf("resident: the short form %q of %q has another shape", short, dosFinal)
	}
	prefix := make([]element, len(long))
	for i := range long {
		prefix[i] = element{long: long[i], alias: aliases[i]}
	}
	driveType, err := driveTypeOf(strings.TrimPrefix(dosFinal, `\\?\`))
	if err != nil {
		return resolved{}, err
	}
	if !watchable(driveType) {
		return resolved{}, &Unsupported{Why: WhyVolumeType}
	}
	return resolved{guidFinal: guidFinal, volumeRoot: volumeRoot, prefix: prefix}, nil
}

// finalPaths opens a path for FILE_READ_ATTRIBUTES alone and answers its
// final path in both forms, closing the handle before it returns.
func finalPaths(path string) (guidFinal, dosFinal string, err error) {
	handle, err := openForQuery(path)
	if err != nil {
		return "", "", err
	}
	defer windows.CloseHandle(handle)
	guidFinal, err = finalPathOf(handle, volumeNameGUID|fileNameNormalized)
	if err != nil {
		return "", "", err
	}
	dosFinal, err = finalPathOf(handle, volumeNameDOS|fileNameNormalized)
	if err != nil {
		return "", "", &Unsupported{Why: WhyNoDOSPath}
	}
	return guidFinal, dosFinal, nil
}

// openForQuery opens a file or a directory for its attributes alone, sharing
// everything, which is the open every query here makes.
func openForQuery(path string) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
}

// finalPathOf answers GetFinalPathNameByHandle for a handle, growing the
// buffer when the call answers the size it needs: "If the function fails
// because lpszFilePath is too small to hold the string plus the terminating
// null character, the return value is the required buffer size, in TCHARs."
func finalPathOf(handle windows.Handle, flags uint32) (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetFinalPathNameByHandle(handle, &buf[0], uint32(len(buf)), flags)
		if err != nil {
			return "", err
		}
		if int(n) < len(buf) {
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n)
	}
}

// shortPath answers GetShortPathName for a path, growing the buffer as
// finalPathOf does.
func shortPath(path string) (string, error) {
	in, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetShortPathName(in, &buf[0], uint32(len(buf)))
		if err != nil {
			return "", err
		}
		if int(n) < len(buf) {
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n)
	}
}

// driveTypeOf answers the drive type of the volume a path lies on:
// GetVolumePathName, which "Retrieves the volume mount point where the
// specified path is mounted", and GetDriveType of that mount point.
func driveTypeOf(path string) (uint32, error) {
	in, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	volume := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(in, &volume[0], uint32(len(volume))); err != nil {
		return 0, err
	}
	return windows.GetDriveType(&volume[0]), nil
}

// Arm resolves the root, opens the root directory of its volume, allocates
// the event and the region, and issues the first call. Its successful return
// is the moment a change is certain to be reported: "When you first call
// ReadDirectoryChangesW, the system allocates a buffer to store change
// information ... Changes that occur between calls to this function are added
// to the buffer and then returned with the next call."
//
// The volume root is opened for FILE_LIST_DIRECTORY, the access
// ReadDirectoryChangesW requires ("This directory must be opened with the
// FILE_LIST_DIRECTORY access right"), with FILE_FLAG_BACKUP_SEMANTICS, which
// is how a directory is opened, and FILE_FLAG_OVERLAPPED, which makes the call
// asynchronous. When the account may not open it so, Arm answers the error
// and the workbench is served from disk.
func (n *winNotifier) Arm(root string) error {
	where, err := resolve(root)
	if err != nil {
		return err
	}
	path, err := windows.UTF16PtrFromString(where.volumeRoot)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(path,
		windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return err
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(handle)
		return err
	}
	region, err := windows.VirtualAlloc(0, changeBufferSize+overlappedSpace, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err != nil {
		windows.CloseHandle(event)
		windows.CloseHandle(handle)
		return err
	}
	release := func() {
		windows.VirtualFree(region, 0, windows.MEM_RELEASE)
		windows.CloseHandle(event)
		windows.CloseHandle(handle)
	}
	if (region+overlappedSpace)%4 != 0 {
		release()
		return errors.New("resident: the change buffer is not DWORD-aligned")
	}
	// The region is outside the Go heap, so the address VirtualAlloc answered
	// is reinterpreted as a pointer rather than converted from a uintptr
	// expression, which is the form go vet reads as a heap pointer's misuse.
	base := *(*unsafe.Pointer)(unsafe.Pointer(&region))
	overlapped := (*windows.Overlapped)(base)
	buffer := unsafe.Slice((*byte)(unsafe.Add(base, overlappedSpace)), changeBufferSize)

	n.mu.Lock()
	defer n.mu.Unlock()
	n.root, n.handle, n.event, n.region = root, handle, event, region
	n.overlapped, n.buffer = overlapped, buffer
	n.guidFinal, n.prefix = where.guidFinal, where.prefix
	n.closing = false
	n.consumed = false
	n.moved.Store(false)
	if err := n.issue(); err != nil {
		release()
		n.handle, n.event, n.region, n.overlapped, n.buffer = 0, 0, 0, nil, nil
		return err
	}
	n.armed = true
	return nil
}

// issue zeroes the OVERLAPPED's members other than hEvent, resets the event,
// and issues one ReadDirectoryChangesW call. n.mu is held.
func (n *winNotifier) issue() error {
	if n.closing {
		n.strayIssue = true
	}
	*n.overlapped = windows.Overlapped{HEvent: n.event}
	if err := windows.ResetEvent(n.event); err != nil {
		return err
	}
	if err := windows.ReadDirectoryChanges(n.handle, &n.buffer[0], changeBufferSize, true, changeFilter, nil, n.overlapped, 0); err != nil {
		return err
	}
	n.issued = true
	return nil
}

// Next issues a call when none is outstanding, waits for its completion,
// decodes it, and keeps the records place judges inside the workbench. A
// completion whose records all fall outside the workbench yields no change,
// and Next then goes back to its own start rather than answering an empty
// batch, so the watcher goroutine is never woken for the rest of the volume.
// An overflow passes through as an overflow, because the buffer that
// overflowed held the whole volume's records and the workbench's may have
// been among them.
func (n *winNotifier) Next() (Batch, error) {
	for {
		batch, err := n.next()
		if err != nil || batch.Overflow {
			return batch, err
		}
		n.mu.Lock()
		prefix := n.prefix
		n.mu.Unlock()
		var kept []Change
		for _, change := range batch.Changes {
			placedChange, where := place(prefix, change.Path, change.Action)
			switch where {
			case inside:
				kept = append(kept, placedChange)
			case moved:
				n.moved.Store(true)
			}
		}
		if len(kept) > 0 {
			return Batch{Changes: kept}, nil
		}
	}
}

// next is one call and its completion, decoded.
func (n *winNotifier) next() (Batch, error) {
	n.mu.Lock()
	n.running = true
	consumed := n.consumed
	n.mu.Unlock()
	if consumed && n.hooks != nil && n.hooks.BeforeRearm != nil {
		n.hooks.BeforeRearm()
	}
	n.mu.Lock()
	if n.closing || !n.armed {
		n.running = false
		n.cond.Broadcast()
		n.mu.Unlock()
		return Batch{}, ErrClosed
	}
	if !n.issued {
		if err := n.issue(); err != nil {
			n.running = false
			n.cond.Broadcast()
			n.mu.Unlock()
			return Batch{}, err
		}
		n.consumed = false
	}
	handle, overlapped, buffer := n.handle, n.overlapped, n.buffer
	n.mu.Unlock()

	var got uint32
	waited := windows.GetOverlappedResult(handle, overlapped, &got, true)

	n.mu.Lock()
	n.issued = false
	n.consumed = true
	closing := n.closing
	n.mu.Unlock()
	// The buffer is decoded before running is cleared, because Close frees
	// the region once no Next is running.
	batch, err := decode(buffer, got, waited, closing)
	n.mu.Lock()
	n.running = false
	n.cond.Broadcast()
	n.mu.Unlock()
	return batch, err
}

// Valid reports whether root still names the directory whose changes the
// notifier reports. It answers false when a record since the last Arm has
// said that the root or a folder above it was created, removed or renamed.
// Otherwise it reads the root's final path in its volume GUID form again and
// answers whether it equals the one Arm recorded, byte for byte; an open or a
// query that fails answers false. The records catch every rename, removal or
// creation of the root or a folder above it on the volume, including one
// undone before the next request, and the comparison catches what no record
// on this volume reports: a symbolic link, junction or mount point on the
// served path that now resolves somewhere else. The open lasts only while
// Valid runs, inside the request that called it.
func (n *winNotifier) Valid() bool {
	if n.moved.Load() {
		return false
	}
	n.mu.Lock()
	armed, root, guidFinal := n.armed && !n.closing, n.root, n.guidFinal
	n.mu.Unlock()
	if !armed {
		return false
	}
	handle, err := openForQuery(root)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	now, err := finalPathOf(handle, volumeNameGUID|fileNameNormalized)
	return err == nil && now == guidFinal
}

// Close sets closing and cancels the outstanding call under n.mu, waits for a
// running Next to return, completes a call nobody consumed, and only then
// releases the handles and the region, because the OVERLAPPED must stay valid
// until the operation completes.
func (n *winNotifier) Close() error {
	n.closeMu.Lock()
	defer n.closeMu.Unlock()
	n.mu.Lock()
	if !n.armed {
		n.mu.Unlock()
		return nil
	}
	n.closing = true
	var cancelErr error
	if n.issued {
		// "If this function cannot find a request to cancel, the return value
		// is 0 (zero), and GetLastError returns ERROR_NOT_FOUND": the call
		// completed before the cancel, and its completion waits to be
		// consumed.
		if err := windows.CancelIoEx(n.handle, n.overlapped); err != nil && !errors.Is(err, windows.ERROR_NOT_FOUND) {
			cancelErr = err
		}
	}
	for n.running {
		n.cond.Wait()
	}
	outstanding := n.issued
	handle, event, region, overlapped := n.handle, n.event, n.region, n.overlapped
	n.mu.Unlock()
	if outstanding {
		var got uint32
		windows.GetOverlappedResult(handle, overlapped, &got, true)
		n.mu.Lock()
		n.issued = false
		n.mu.Unlock()
	}
	released := []error{
		cancelErr,
		windows.CloseHandle(handle),
		windows.CloseHandle(event),
		windows.VirtualFree(region, 0, windows.MEM_RELEASE),
	}
	n.mu.Lock()
	n.armed = false
	n.mu.Unlock()
	return errors.Join(released...)
}

// breakWatch cancels the outstanding call under n.mu without setting closing,
// which a test uses to make the watch fail as a handle failing would.
func (n *winNotifier) breakWatch() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.issued {
		return errors.New("resident: no call is outstanding to cancel")
	}
	return windows.CancelIoEx(n.handle, n.overlapped)
}
