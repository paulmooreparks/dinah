package terminals

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Shell is a shell the Terminal can run.
type Shell struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Path is the program; empty for Dinah's own shell.
	Path string   `json:"path"`
	args []string // after the program's own name
}

// BuiltinShell is Dinah's own shell, which runs in the page.
const BuiltinShell = "dinah"

// Shells lists the shells found on this computer, Dinah's own last,
// and the one a new Terminal runs.
type Shells struct {
	Shells  []Shell `json:"shells"`
	Default string  `json:"default"`
}

// findShells looks for each shell where its installer documents it: on the
// PATH, or in the Windows folder.
func findShells(productName string) []Shell {
	var out []Shell
	add := func(id, name, path string, args ...string) {
		if path == "" {
			return
		}
		if _, err := os.Stat(path); err != nil {
			return
		}
		out = append(out, Shell{ID: id, Name: name, Path: path, args: args})
	}
	look := func(name string) string { p, _ := exec.LookPath(name); return p }
	system := os.Getenv("SystemRoot")
	add("pwsh", "PowerShell", look("pwsh.exe"), "-NoLogo")
	if system != "" {
		add("powershell", "Windows PowerShell", filepath.Join(system, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoLogo")
		add("cmd", "Command Prompt", filepath.Join(system, "System32", "cmd.exe"))
	}
	// Git for Windows keeps bash.exe in its bin folder, and the git.exe on
	// the PATH in cmd or in mingw64\bin; its installer's default folder is
	// Git under Program Files.
	var gits []string
	if git := look("git.exe"); git != "" {
		gits = append(gits, filepath.Dir(filepath.Dir(git)), filepath.Dir(filepath.Dir(filepath.Dir(git))))
	}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		gits = append(gits, filepath.Join(pf, "Git"))
	}
	for _, root := range gits {
		if bash := filepath.Join(root, "bin", "bash.exe"); isFile(bash) {
			add("git-bash", "Git Bash", bash, "--login", "-i")
			break
		}
	}
	if system != "" {
		add("wsl", "WSL", filepath.Join(system, "System32", "wsl.exe"))
	}
	out = append(out, Shell{ID: BuiltinShell, Name: productName + " Shell"})
	return out
}

// Profile lets a person register any shell without changing discovery code.
type Profile struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Path string   `json:"path"`
	Args []string `json:"args"`
}

// Available returns discovered shells followed by configured profiles.
func Available(productName string, profiles []Profile) []Shell {
	out := findShells(productName)
	for _, profile := range profiles {
		path, err := exec.LookPath(profile.Path)
		if err != nil {
			continue
		}
		out = append(out, Shell{ID: profile.ID, Name: profile.Name, Path: path, args: append([]string(nil), profile.Args...)})
	}
	return out
}

// defaultShell is the person's choice when it is still on the computer, and
// otherwise the first shell found.
func DefaultShell(shells []Shell, choice string) string {
	for _, s := range shells {
		if s.ID == choice {
			return choice
		}
	}
	return shells[0].ID
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
