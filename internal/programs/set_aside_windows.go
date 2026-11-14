//go:build windows

package programs

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const asideRemovalSupported = true

var mappedImageName = windows.NewLazySystemDLL("psapi.dll").NewProc("GetMappedFileNameW")

// fileIdentity uses the documented 128-bit file ID and volume serial pair.
func fileIdentity(handle windows.Handle) (string, error) {
	var info struct {
		Volume uint64
		ID     [16]byte
	}
	err := windows.GetFileInformationByHandleEx(handle, windows.FileIdInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%016x:%x", info.Volume, info.ID), nil
}

func openAside(path string, access, share uint32) (*os.File, string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, "", err
	}
	h, err := windows.CreateFile(p, access, share, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, "", err
	}
	f := os.NewFile(uintptr(h), path)
	var attrs windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &attrs)
	if err != nil || attrs.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		f.Close()
		if err == nil {
			err = errors.New("the path is not an ordinary executable file")
		}
		return nil, "", err
	}
	id, err := fileIdentity(h)
	if err != nil {
		f.Close()
		return nil, "", err
	}
	return f, id, nil
}

func inspectAside(path string) (os.FileInfo, string, error) {
	f, id, err := openAside(path, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	info, err := f.Stat()
	return info, id, err
}

func runningAsideImages() (map[string][]uint32, []string, error) {
	images := map[string][]uint32{}
	ids := make([]uint32, 1024)
	for {
		var bytes uint32
		if err := windows.EnumProcesses(ids, &bytes); err != nil {
			return nil, nil, err
		}
		if int(bytes) < len(ids)*4 {
			ids = ids[:bytes/4]
			break
		}
		if len(ids) >= 65536 {
			return nil, nil, errors.New("the process list exceeded the inspection limit")
		}
		ids = make([]uint32, len(ids)*2)
	}
	unavailable := 0
	// Repeated mapped libraries share a path across many processes. This
	// cache belongs to one snapshot only; mutation safety is enforced by the
	// deletion handle and the kernel's mapped-image check, never this cache.
	identitiesByPath := map[string]string{}
	for _, pid := range ids {
		if pid == 0 {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
		if err != nil {
			unavailable++
			continue
		}
		identities, complete := processMappedIdentities(h, identitiesByPath)
		windows.CloseHandle(h)
		if !complete {
			unavailable++
		}
		for identity := range identities {
			images[identity] = append(images[identity], pid)
		}
	}
	warnings := []string{}
	if unavailable > 0 {
		warnings = append(warnings, fmt.Sprintf("%d processes could not be inspected or exited during inspection. Windows checks executable use again when removing a copy.", unavailable))
	}
	return images, warnings, nil
}

func processMappedIdentities(process windows.Handle, identitiesByPath map[string]string) (map[string]bool, bool) {
	identities := map[string]bool{}
	modules := make([]windows.Handle, 256)
	for {
		var needed uint32
		bytes := uint32(len(modules)) * uint32(unsafe.Sizeof(modules[0]))
		if err := windows.EnumProcessModulesEx(process, &modules[0], bytes, &needed, windows.LIST_MODULES_ALL); err != nil {
			return identities, false
		}
		if needed <= bytes {
			modules = modules[:needed/uint32(unsafe.Sizeof(modules[0]))]
			break
		}
		if needed > 65536*uint32(unsafe.Sizeof(modules[0])) {
			return identities, false
		}
		modules = make([]windows.Handle, needed/uint32(unsafe.Sizeof(modules[0]))+64)
	}
	complete := true
	for _, module := range modules {
		var name [32768]uint16
		n, _, _ := mappedImageName.Call(uintptr(process), uintptr(module), uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		if n == 0 || n >= uintptr(len(name)) {
			complete = false
			continue
		}
		path := windows.UTF16ToString(name[:n])
		// PSAPI returns a device path. GLOBALROOT is the documented Win32
		// namespace for opening that path without drive-letter assumptions.
		if strings.HasPrefix(path, `\Device\`) {
			path = `\\?\GLOBALROOT` + path
		}
		identity, cached := identitiesByPath[path]
		if !cached {
			_, inspected, err := inspectAside(path)
			if err != nil {
				complete = false
				continue
			}
			identity = inspected
			identitiesByPath[path] = identity
		}
		identities[identity] = true
	}
	return identities, complete
}

func removeAside(file asideFile, check func() error) (string, error) {
	f, identity, err := openAside(file.path, windows.DELETE|windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return "in_use", errors.New("Windows refused exclusive deletion access; the copy may be in use or access is denied")
		}
		return "io_error", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "io_error", err
	}
	if identity != file.identity || info.Size() != file.SizeBytes {
		return "changed_or_missing", errors.New("the selected executable changed; refresh the list")
	}
	if err := check(); err != nil {
		return "changed_or_missing", err
	}
	deleteFile := byte(1)
	err = windows.SetFileInformationByHandle(windows.Handle(f.Fd()), windows.FileDispositionInfo, &deleteFile, 1)
	if err != nil {
		if errors.Is(err, syscall.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return "in_use", errors.New("Windows refused deletion; the executable may be running or access is denied")
		}
		return "io_error", err
	}
	if err := f.Close(); err != nil {
		return "io_error", err
	}
	return "", nil
}
