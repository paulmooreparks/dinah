package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/layout"
	"studio/internal/profile"
	"studio/internal/programs"
)

// installOption is the last step of an installation, which the window
// program runs on itself once it is in the program folder
// (docs/design/installation.md). The command line's install runs it, and so
// does a product's installer and its updater. It makes the stand-alone
// document applications, the Start menu shortcut, moves the programs an
// earlier installation put in the home folder's bin into the program
// folder, and puts the program folder on the person's PATH. Each part can be
// run again and changes nothing the second time.
const installOption = "--install"

func runInstall() int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p := profile.Current()
	dir, err := programs.Folder(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !layout.SamePath(filepath.Dir(exe), dir) {
		fmt.Fprintf(os.Stderr, "%s finishes an installation only from its program folder, %s, and this copy is in %s.\n", product.Name, dir, filepath.Dir(exe))
		return 2
	}
	failed := false
	oldBin := filepath.Join(configDir(), "bin")
	// A copy started with a home or program folder of its own, as a check
	// does, changes nothing of the person's: the system's lists of
	// applications, their Start menu and their PATH.
	ownFolders := os.Getenv(p.HomeVar()) != "" || os.Getenv(programs.FolderVar(p)) != ""
	if ownFolders {
		fmt.Printf("The document applications, the Start menu and the PATH were left alone, since %s or %s is set.\n", p.HomeVar(), programs.FolderVar(p))
	} else {
		if runDocumentInstall(false) != 0 {
			failed = true
		}
		earlier := filepath.Join(oldBin, programs.FileName(p.WindowProgram))
		if notes, err := windowShortcuts(exe, earlier); err != nil {
			fmt.Fprintf(os.Stderr, "The Start menu shortcut could not be made: %v\n", err)
			failed = true
		} else {
			for _, note := range notes {
				fmt.Println(note)
			}
		}
	}
	// The programs an earlier installation put in the home folder's bin, and
	// the copies updates set aside there, go to the program folder, where
	// the Updates page lists the copies and can remove them.
	moved, left, err := programs.MoveOut(oldBin, dir, installedProgramNames(p), []string{"THIRD-PARTY-NOTICES.md"})
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "The programs in %s could not be moved: %v\n", oldBin, err)
		failed = true
	case len(moved) > 0:
		fmt.Printf("Moved %d files from %s to %s.\n", len(moved), oldBin, dir)
	}
	if len(left) > 0 {
		fmt.Printf("These files in %s could not be moved, perhaps because they are running, and are moved by the next installation: %s\n", oldBin, strings.Join(left, ", "))
	}
	if !ownFolders {
		changed, err := programs.SetUserPath(oldBin, dir)
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "%s could not be put on your PATH: %v\n", dir, err)
			failed = true
		case changed:
			fmt.Printf("%s is on your PATH in place of %s. A Terminal started from now on finds the programs.\n", dir, oldBin)
		}
	}
	// The agent programs' settings are the person's, and an entry there may
	// run a program from the bin by its full path. They are not changed
	// here, since an installer for anyone must not edit them unreviewed;
	// the person is told which still do.
	if !ownFolders {
		if using := settingsUsing(oldBin); len(using) > 0 {
			fmt.Printf("These agent settings still run programs from %s, which are now in %s. Change those paths to %s:\n", oldBin, dir, dir)
			for _, line := range using {
				fmt.Println("  " + line)
			}
		}
	}
	if failed {
		return 1
	}
	return 0
}

// settingsUsing lists the agent programs' settings files that name a file
// in folder by its full path, with how many times: Claude Code's
// settings.json and .claude.json, and Codex's config.toml. A path is looked
// for as it is written in JSON and TOML, with either slash, and in any case.
func settingsUsing(folder string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	codex := os.Getenv("CODEX_HOME")
	if codex == "" {
		codex = filepath.Join(home, ".codex")
	}
	files := []string{filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".claude.json"), filepath.Join(codex, "config.toml")}
	back := strings.ToLower(filepath.Clean(folder)) + `\`
	spellings := []string{back, strings.ReplaceAll(back, `\`, `/`), strings.ReplaceAll(back, `\`, `\\`)}
	var out []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		text, n := strings.ToLower(string(data)), 0
		for _, s := range spellings {
			n += strings.Count(text, s)
		}
		if n > 0 {
			out = append(out, fmt.Sprintf("%s (%d)", file, n))
		}
	}
	return out
}

// installedProgramNames are every program an installation of p puts in the
// program folder: its own, the ones it ran under earlier names, and the
// stand-alone document applications.
func installedProgramNames(p profile.Profile) []string {
	names := setAsideProgramNames(p)
	for id := range documentApplications {
		names = append(names, documentProgramName(id))
	}
	return names
}
