//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// How a stand-alone application is made known to Windows, for the person
// only. Everything is under HKEY_CURRENT_USER\Software\Classes, which
// Windows documents as the place for a registration the current user alone
// sees.
//
//   - A ProgID for the application's documents, with the command that opens
//     one, so the application can be a kind of file's handler.
//     https://learn.microsoft.com/windows/win32/shell/fa-progids
//   - The program under Applications, with FriendlyAppName, the name Windows
//     shows for it, and the kinds of file it supports.
//     https://learn.microsoft.com/windows/win32/shell/app-registration
//   - The ProgID under each kind's OpenWithProgids, which lists the
//     application in Open With and leaves the kind's default alone.
//     https://learn.microsoft.com/windows/win32/shell/how-to-include-an-application-on-the-open-with-dialog-box
//   - A shortcut in the person's Start menu programs.
//
// Windows is then told that associations changed, as it asks
// (SHChangeNotify with SHCNE_ASSOCCHANGED).
// https://learn.microsoft.com/windows/win32/shell/fa-file-types

// documentClassesRoot is where the registration is written. A test writes
// under a key of its own.
var documentClassesRoot = `Software\Classes`

var procSHChangeNotify = iconShell32.NewProc("SHChangeNotify")

// documentProgID is the ProgID of an application's documents: the product
// and the application, with no spaces, as the form Windows documents.
func documentProgID(app documentInstalled) string {
	return identityWord(product.Name) + "." + identityWord(app.Name) + ".Document"
}

func setString(root registry.Key, path, name, value string) error {
	k, _, err := registry.CreateKey(root, path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(name, value)
}

// deleteTree removes a key and everything under it. A key that is not there
// is no error.
func deleteTree(root registry.Key, path string) error {
	k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	names, err := k.ReadSubKeyNames(-1)
	k.Close()
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := deleteTree(root, path+`\`+name); err != nil {
			return err
		}
	}
	if err := registry.DeleteKey(root, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

func registerDocumentApplication(app documentInstalled) (string, error) {
	classes, progID := documentClassesRoot, documentProgID(app)
	command := `"` + app.Program + `" "%1"`
	exe := filepath.Base(app.Program)
	values := []struct{ path, name, value string }{
		{classes + `\` + progID, "", app.Name + " Document"},
		{classes + `\` + progID + `\DefaultIcon`, "", app.Program + ",0"},
		{classes + `\` + progID + `\shell\open\command`, "", command},
		{classes + `\Applications\` + exe, "FriendlyAppName", app.Name},
		{classes + `\Applications\` + exe + `\shell\open\command`, "", command},
	}
	for _, ext := range app.Extensions {
		values = append(values,
			struct{ path, name, value string }{classes + `\Applications\` + exe + `\SupportedTypes`, ext, ""},
			struct{ path, name, value string }{classes + `\` + ext + `\OpenWithProgids`, progID, ""})
	}
	for _, v := range values {
		if err := setString(registry.CURRENT_USER, v.path, v.name, v.value); err != nil {
			return "", err
		}
	}
	note := "It is offered in Open With for " + strings.Join(app.Extensions, ", ") + " and is no kind of file's default."
	// Under test only the registry is written, under the test's own key.
	if classes != `Software\Classes` {
		return note, nil
	}
	if err := documentShortcut(app, false); err != nil {
		note += " Its Start menu shortcut could not be made: " + err.Error()
	} else {
		note += " It is in the Start menu."
	}
	associationsChanged()
	return note, nil
}

func unregisterDocumentApplication(app documentInstalled) error {
	classes, progID := documentClassesRoot, documentProgID(app)
	for _, ext := range app.Extensions {
		k, err := registry.OpenKey(registry.CURRENT_USER, classes+`\`+ext+`\OpenWithProgids`, registry.SET_VALUE)
		if err == nil {
			if err := k.DeleteValue(progID); err != nil && !errors.Is(err, registry.ErrNotExist) {
				k.Close()
				return err
			}
			k.Close()
		} else if !errors.Is(err, registry.ErrNotExist) {
			return err
		}
	}
	if err := deleteTree(registry.CURRENT_USER, classes+`\`+progID); err != nil {
		return err
	}
	if err := deleteTree(registry.CURRENT_USER, classes+`\Applications\`+filepath.Base(app.Program)); err != nil {
		return err
	}
	if classes != `Software\Classes` {
		return nil
	}
	err := documentShortcut(app, true)
	associationsChanged()
	return err
}

// associationsChanged tells Windows that file associations changed.
func associationsChanged() {
	const assocChanged, idList = 0x08000000, 0x0000
	procSHChangeNotify.Call(assocChanged, idList, 0, 0)
}

// documentShortcut makes, or removes, the application's shortcut in the
// person's Start menu programs. The folder is the one Windows gives for it
// (FOLDERID_Programs), and the shortcut is written by the Windows Script
// Host's own shell object, which is the documented way for a program to
// write one without the shell's COM interfaces by hand.
// https://learn.microsoft.com/windows/win32/shell/knownfolderid
// https://learn.microsoft.com/previous-versions/windows/internet-explorer/ie-developer/windows-scripting/xk6kst2k(v=vs.84)
func documentShortcut(app documentInstalled, remove bool) error {
	folder, err := windows.KnownFolderPath(windows.FOLDERID_Programs, 0)
	if err != nil {
		return err
	}
	link := filepath.Join(folder, app.Name+".lnk")
	if remove {
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		// S_FALSE means the thread already has COM, which is fine.
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != 1 {
			return err
		}
	}
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return err
	}
	defer unknown.Release()
	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer shell.Release()
	made, err := oleutil.CallMethod(shell, "CreateShortcut", link)
	if err != nil {
		return err
	}
	shortcut := made.ToIDispatch()
	defer shortcut.Release()
	for name, value := range map[string]string{"TargetPath": app.Program, "Description": app.Name, "WorkingDirectory": filepath.Dir(app.Program)} {
		if _, err := oleutil.PutProperty(shortcut, name, value); err != nil {
			return err
		}
	}
	_, err = oleutil.CallMethod(shortcut, "Save")
	return err
}
