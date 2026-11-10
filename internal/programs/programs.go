// Package programs is what a build knows about its own installed programs:
// the release the running one belongs to, and how an installed program is
// replaced while something may be running it. The installer uses it, and so
// can a product's own packages (docs/design/product-applets.md), which cannot
// read the variables of the program they are linked into.
package programs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	release string
)

// SetRelease records the release the running program belongs to. The program
// calls it once, first thing, with the version its build was given.
func SetRelease(version string) {
	mu.Lock()
	release = version
	mu.Unlock()
}

// Release is the release the running program belongs to, or "" when the
// program has not said.
func Release() string {
	mu.Lock()
	defer mu.Unlock()
	return release
}

// FileName is a program's file name on this platform.
func FileName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// Place copies the program src to dst. Something may be running the program
// at dst, and Windows refuses to overwrite a running program but allows it
// to be renamed, so the old one is moved aside first. Copies set aside
// earlier are removed once nothing runs them.
func Place(src, dst string) error {
	old, _ := filepath.Glob(dst + ".old-*")
	for _, o := range old {
		os.Remove(o) // fails while something still runs it; it goes next time
	}
	// The copy is written beside dst before anything is moved, so src is still
	// readable when it is dst itself (installing from the installed program),
	// and a failed copy leaves the installed program alone. It is renamed into
	// place, so dst is never a half-written program: the window watches it to
	// offer Restart to Update.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		in.Close()
		return err
	}
	_, err = io.Copy(out, in)
	in.Close() // closed before the rename below: Windows will not rename a program that is open
	if err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	aside := ""
	if _, err := os.Stat(dst); err == nil {
		aside = fmt.Sprintf("%s.old-%d", dst, time.Now().UnixNano())
		if err := renameFile(dst, aside); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	if err := renameFile(tmp, dst); err != nil {
		os.Remove(tmp)
		if aside != "" {
			if back := renameFile(aside, dst); back != nil {
				return fmt.Errorf("%w (and the previous program could not be put back: %v)", err, back)
			}
		}
		return err
	}
	return nil
}

// renameFile is os.Rename; a test replaces it to fail a step of Place.
var renameFile = os.Rename
