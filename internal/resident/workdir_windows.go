//go:build windows

package resident

import (
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// finalPath is the GetFinalPathNameByHandle query WorkingDirectory makes: the
// path opened for its attributes alone, queried with flags, and closed. Only
// a test in this package replaces it, to make the workbench's query fail.
var finalPath = func(path string, flags uint32) (string, error) {
	handle, err := openForQuery(path)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	return finalPathOf(handle, flags)
}

// WorkingDirectory answers the directory dinah serve moves its working
// directory to, a volume's root directory, which lies beneath no folder. The
// server finds its workbench by climbing from where it was started, so it
// starts standing in a folder above the workbench, and whether a working
// directory holds that folder is not documented; after the move the
// directory in question is one no rename or deletion of a folder can find
// beneath it.
//
//  1. The workbench's drive, after resolution. It reads the root's final path
//     with VOLUME_NAME_DOS, and when that begins with \\?\ and a drive
//     element the candidate is that drive's root, X:\. The drive letter the
//     root was typed with is never consulted: for a workbench reached as
//     S:\wb through subst S: C:\foo, the root of S: is C:\foo, a folder above
//     the workbench. A final path "is the path that is returned when a path
//     is fully resolved".
//  2. Confirm the candidate: its own final path with VOLUME_NAME_GUID must be
//     a bare volume GUID path, \\?\Volume{...}\ with nothing after the
//     backslash, which "Naming a Volume" makes the root directory of a volume
//     ("CreateFile processes a volume GUID path with an appended backslash as
//     the root directory of the volume"). A candidate naming a subst letter's
//     folder has elements after the volume root and is not confirmed.
//  3. The fallback, when step 1 yields no candidate or step 2 does not
//     confirm it: the drive root of the Windows directory, from
//     GetSystemWindowsDirectory ("Retrieves the path of the shared Windows
//     directory on a multi-user system"), confirmed by step 2 in the same
//     way. When that fails too, WorkingDirectory answers a *NoVolumeRoot.
//
// Each handle is held only for its query.
func WorkingDirectory(root string) (string, error) {
	var tried [2]Refused
	candidate, err := workbenchDrive(root)
	if err == nil {
		err = confirmVolumeRoot(candidate)
		if err == nil {
			return candidate, nil
		}
	}
	tried[0] = Refused{Dir: candidate, Why: err}
	windowsDir, err := windows.GetSystemWindowsDirectory()
	if err != nil {
		tried[1] = Refused{Why: err}
		return "", &NoVolumeRoot{Tried: tried}
	}
	candidate = filepath.VolumeName(windowsDir) + `\`
	if err := confirmVolumeRoot(candidate); err != nil {
		tried[1] = Refused{Dir: candidate, Why: err}
		return "", &NoVolumeRoot{Tried: tried}
	}
	return candidate, nil
}

// workbenchDrive answers the root of the drive the workbench's final path
// names, or "" and why there is none.
func workbenchDrive(root string) (string, error) {
	dosFinal, err := finalPath(root, volumeNameDOS|fileNameNormalized)
	if err != nil {
		return "", err
	}
	drive, _, ok := dosDriveOf(dosFinal)
	if !ok {
		return "", fmt.Errorf("the final path %q names no drive", dosFinal)
	}
	return drive + `\`, nil
}

// confirmVolumeRoot answers nil when a directory's own final path in its
// volume GUID form is a bare volume GUID path.
func confirmVolumeRoot(dir string) error {
	guidFinal, err := finalPath(dir, volumeNameGUID|fileNameNormalized)
	if err != nil {
		return err
	}
	if _, rest, ok := volumeRootOf(guidFinal); !ok || rest != "" {
		return errors.New("its final path " + guidFinal + " is not a bare volume GUID path")
	}
	return nil
}
