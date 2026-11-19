package programs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/profile"
)

// The program folder is where a product's programs are installed: the
// window, the command line, the programs it runs for agents, the stand-alone
// document applications, and the copies of them that an update sets aside.
// It is not the product's home folder. The home folder's bin is /bin in the
// product's virtual file system, which holds the standard scripts its own
// shell runs, so a program installed there would be listed and offered as a
// command of that shell (docs/design/installation.md).
//
// On Windows the folder is the product's name in the person's own program
// folder, which Windows gives as FOLDERID_UserProgramFiles and which is
// %LOCALAPPDATA%\Programs unless it has been moved. It needs no
// administrator, so a computer whose person cannot install for every user
// can still install the product.
// https://learn.microsoft.com/windows/win32/shell/knownfolderid

// FolderVar is the environment variable that sets the program folder in
// place of the usual one, such as DINAH_PROGRAMS. A check that runs an
// installation of its own sets it beside its own home folder.
func FolderVar(p profile.Profile) string { return p.EnvPrefix + "PROGRAMS" }

// Folder is the program folder of the product p.
func Folder(p profile.Profile) (string, error) {
	if set := strings.TrimSpace(os.Getenv(FolderVar(p))); set != "" {
		if !filepath.IsAbs(set) {
			return "", errors.New(FolderVar(p) + " must be a full path")
		}
		return filepath.Clean(set), nil
	}
	if p.Name == "" || strings.ContainsAny(p.Name, `\/:*?"<>|`) {
		return "", errors.New("the product's name cannot be a folder's name")
	}
	root, err := userProgramFolder()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, p.Name), nil
}

// CurrentFolder is the program folder of the product this build is.
func CurrentFolder() (string, error) { return Folder(profile.Current()) }

// FolderOf returns the path of the program called name, such as
// "dinah-desktop", in the product's program folder.
func FolderOf(p profile.Profile, name string) (string, error) {
	dir, err := Folder(p)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName(name)), nil
}
