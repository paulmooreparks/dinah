//go:build !windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// On Linux a stand-alone application is made known to the desktop with a
// desktop entry in the person's own applications folder, as the Desktop
// Entry Specification describes: its name, the command that opens a file,
// and the kinds of file it can open. A desktop offers it in Open With for
// those kinds and it is no kind's default.
// https://specifications.freedesktop.org/desktop-entry-spec/latest/
// https://specifications.freedesktop.org/basedir-spec/latest/
//
// On macOS an application is a bundle, which is not built yet, so nothing
// is registered there and the program is started by its name.

// documentMimeTypes are the kinds of file, as a desktop entry names them.
var documentMimeTypes = map[string]string{".md": "text/markdown", ".markdown": "text/markdown", ".txt": "text/plain"}

func documentEntryFile(app documentInstalled) (string, error) {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		data = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(data, "applications", filepath.Base(app.Program)+".desktop"), nil
}

// desktopExec is a program's path as the Exec key takes it: in double
// quotes, with the characters the specification reserves escaped.
func desktopExec(path string) string {
	r := strings.NewReplacer(`\`, `\\\\`, `"`, `\"`, "`", "\\`", `$`, `\$`, `%`, `%%`)
	return `"` + r.Replace(path) + `"`
}

func registerDocumentApplication(app documentInstalled) (string, error) {
	if runtime.GOOS != "linux" {
		return "On this system it is started by its name; nothing was registered.", nil
	}
	file, err := documentEntryFile(app)
	if err != nil {
		return "", err
	}
	var kinds []string
	seen := map[string]bool{}
	for _, ext := range app.Extensions {
		if m := documentMimeTypes[ext]; m != "" && !seen[m] {
			seen[m] = true
			kinds = append(kinds, m)
		}
	}
	if strings.ContainsAny(app.Name, "\n\r") {
		return "", errors.New("the application's name cannot be written in a desktop entry")
	}
	entry := "[Desktop Entry]\nType=Application\nName=" + app.Name + "\nExec=" + desktopExec(app.Program) + " %f\nTerminal=false\nCategories=Office;TextEditor;\nMimeType=" + strings.Join(kinds, ";") + ";\n"
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(file, []byte(entry), 0o644); err != nil {
		return "", err
	}
	return "Its desktop entry is " + file + ".", nil
}

func unregisterDocumentApplication(app documentInstalled) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	file, err := documentEntryFile(app)
	if err != nil {
		return err
	}
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
