package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

// windowShortcuts makes the window's shortcut in the person's Start menu
// programs under the product's name, and points every other shortcut there
// that starts earlier, the window program where an earlier installation put
// it, at exe instead, so a shortcut the person made keeps working. Only the
// shortcuts at the top of that folder are looked at. The taskbar's pinned
// programs are kept where Windows does not document, so a pin to the earlier
// program is the person's to make again from the Start menu.
func windowShortcuts(exe, earlier string) ([]string, error) {
	if err := startMenuShortcut(product.Name, exe, false); err != nil {
		return nil, err
	}
	notes := []string{"The Start menu has " + product.Name + "."}
	folder, err := windows.KnownFolderPath(windows.FOLDERID_Programs, 0)
	if err != nil {
		return notes, err
	}
	links, _ := filepath.Glob(filepath.Join(folder, "*.lnk"))
	own := strings.ToLower(filepath.Join(folder, product.Name+".lnk"))
	for _, link := range links {
		if strings.ToLower(link) == own {
			continue
		}
		changed, err := retarget(link, earlier, exe)
		if err != nil {
			notes = append(notes, "The shortcut "+filepath.Base(link)+" could not be read or changed: "+err.Error())
			continue
		}
		if changed {
			notes = append(notes, "The shortcut "+strings.TrimSuffix(filepath.Base(link), ".lnk")+" now starts "+exe+".")
		}
	}
	return notes, nil
}

// retarget points the shortcut at link to to when it points to from, through
// the Windows Script Host's shell object, as startMenuShortcut writes one.
func retarget(link, from, to string) (bool, error) {
	// COM initialization, interface calls and cleanup belong to one OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != 1 {
			return false, err
		}
	}
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return false, err
	}
	defer unknown.Release()
	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return false, err
	}
	defer shell.Release()
	made, err := oleutil.CallMethod(shell, "CreateShortcut", link)
	if err != nil {
		return false, err
	}
	shortcut := made.ToIDispatch()
	defer shortcut.Release()
	target, err := oleutil.GetProperty(shortcut, "TargetPath")
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(filepath.Clean(target.ToString()), filepath.Clean(from)) {
		return false, nil
	}
	if _, err := os.Stat(to); err != nil {
		return false, err
	}
	for property, value := range map[string]string{"TargetPath": to, "WorkingDirectory": filepath.Dir(to)} {
		if _, err := oleutil.PutProperty(shortcut, property, value); err != nil {
			return false, err
		}
	}
	_, err = oleutil.CallMethod(shortcut, "Save")
	return err == nil, err
}
