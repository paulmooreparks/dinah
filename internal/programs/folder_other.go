//go:build !windows

package programs

import (
	"os"
	"path/filepath"
)

// userProgramFolder is the person's own folder for programs elsewhere than
// Windows: the XDG data folder's programs, which is ~/.local/share/programs
// when XDG_DATA_HOME is not set.
// https://specifications.freedesktop.org/basedir-spec/latest/
func userProgramFolder() (string, error) {
	if data := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(data) {
		return filepath.Join(data, "programs"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "programs"), nil
}
