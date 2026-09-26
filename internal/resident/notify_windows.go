//go:build windows

package resident

import (
	"errors"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The documented values the watcher passes. Every claim its correctness rests
// on is quoted in dinah-619's specification, section 4.1, from Microsoft's
// documentation for ReadDirectoryChangesW and FILE_NOTIFY_INFORMATION.
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

// fileIDInfo is FILE_ID_INFO, declared here because golang.org/x/sys v0.47.0
// declares the information class FileIdInfo and not the struct: "The file
// identifier and the volume serial number uniquely identify a file on a
// single computer. To determine whether two open handles represent the same
// file, combine the identifier and the volume serial number for each file and
// compare them."
type fileIDInfo struct {
	VolumeSerialNumber uint64
	FileID             [16]byte
}

// watchable reports whether a volume of the given drive type is watched. A
// network share, a removable drive and a RAM disk are served from disk,
// because change notification on those depends on a redirector or a file
// system this design has no documentation for.
func watchable(driveType uint32) bool {
	return driveType == windows.DRIVE_FIXED
}

// platformNotifier answers the Windows watcher for a root on a fixed local
// volume, and ErrUnsupported for any other.
func platformNotifier(root string, hooks *Hooks) (Notifier, error) {
	path, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return nil, ErrUnsupported
	}
	volume := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(path, &volume[0], uint32(len(volume))); err != nil {
		return nil, ErrUnsupported
	}
	if !watchable(windows.GetDriveType(&volume[0])) {
		return nil, ErrUnsupported
	}
	return newWinNotifier(hooks), nil
}

// winNotifier watches one directory tree with ReadDirectoryChangesW.
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
	identity   fileIDInfo
}

// newWinNotifier answers a notifier that is not yet armed.
func newWinNotifier(hooks *Hooks) *winNotifier {
	n := &winNotifier{hooks: hooks}
	n.cond = sync.NewCond(&n.mu)
	return n
}

// Arm opens the root, allocates the event and the region, reads the root's
// identity, and issues the first call. Its successful return is the moment a
// change is certain to be reported: "When you first call
// ReadDirectoryChangesW, the system allocates a buffer to store change
// information ... Changes that occur between calls to this function are
// added to the buffer and then returned with the next call."
func (n *winNotifier) Arm(root string) error {
	path, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(path,
		windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES,
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
	identity, err := identityOf(handle)
	if err != nil {
		release()
		return err
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
	n.overlapped, n.buffer, n.identity = overlapped, buffer, identity
	n.closing = false
	n.consumed = false
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

// Next issues a call when none is outstanding, waits for its completion, and
// decodes it.
func (n *winNotifier) Next() (Batch, error) {
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

// Valid reports whether the root still names the directory being watched:
// the root opened by path now and the watch handle carry the same volume
// serial number and file identifier.
func (n *winNotifier) Valid() bool {
	n.mu.Lock()
	armed, root, identity := n.armed && !n.closing, n.root, n.identity
	n.mu.Unlock()
	if !armed {
		return false
	}
	path, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return false
	}
	handle, err := windows.CreateFile(path, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	now, err := identityOf(handle)
	return err == nil && now == identity
}

// identityOf reads a handle's FILE_ID_INFO.
func identityOf(handle windows.Handle) (fileIDInfo, error) {
	var info fileIDInfo
	err := windows.GetFileInformationByHandleEx(handle, windows.FileIdInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	return info, err
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
