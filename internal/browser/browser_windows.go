//go:build windows

package browser

import (
	"runtime"
	"syscall"

	"golang.org/x/sys/windows"
)

// shellExecute hands a verb and a file to ShellExecuteW. Only this package's
// tests replace it.
//
// Microsoft Learn, "ShellExecuteW function (shellapi.h)", documents the call
// and says that "Because ShellExecute can delegate execution to Shell
// extensions (data sources, context menu handlers, verb implementations) that
// are activated using Component Object Model (COM), COM should be initialized
// before ShellExecute is called", which is why COM is initialized on a locked
// thread first. Microsoft Learn, "Registering an Application to a URI Scheme",
// documents the URL case: ShellExecute handed a URI launches the application
// registered for the URI's scheme, which for http is the reader's default
// browser. That page is cited by its title and paraphrased here rather than
// quoted, because it was not reachable when this was written.
var shellExecute = func(verb, file string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// S_FALSE means COM was already initialized on this thread, and it still
	// takes a matching CoUninitialize. RPC_E_CHANGED_MODE means it was
	// initialized in the other mode, which leaves it usable and takes none.
	const sFalse, changedMode = syscall.Errno(1), syscall.Errno(0x80010106)
	switch err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err {
	case nil, sFalse:
		defer windows.CoUninitialize()
	case changedMode:
	default:
		return err
	}
	verbPtr, err := windows.UTF16PtrFromString(verb)
	if err != nil {
		return err
	}
	filePtr, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verbPtr, filePtr, nil, nil, windows.SW_SHOWNORMAL)
}

// open hands the URL to ShellExecute with the verb open.
func open(address string) error {
	return shellExecute("open", address)
}
