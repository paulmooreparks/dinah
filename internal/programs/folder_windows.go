package programs

import "golang.org/x/sys/windows"

// userProgramFolder is the person's own program folder,
// FOLDERID_UserProgramFiles, created when it is not there yet.
func userProgramFolder() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_UserProgramFiles, windows.KF_FLAG_CREATE)
}
