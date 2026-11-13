package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"studio/internal/profile"
	"studio/internal/programs"
)

/* Commands another program of the product carries out
   (profile.ProgramCommands). The command program starts that program from
   the folder it runs in itself, with the command and its arguments as they
   were given, the same input, output and working folder, and ends with the
   status that program ended with. It adds no output of its own unless the
   program cannot be started. */

// forwarded is the program that carries out a command, when the product
// hands that command to one. A command of the command program's own is
// never handed on: this is asked only for a command it does not have.
func forwarded(name string) (string, bool) {
	for _, c := range profile.Current().ProgramCommands {
		if c.Name == name {
			return c.Program, true
		}
	}
	return "", false
}

// forwardedUsage is the help's lines for the commands handed on.
func forwardedUsage() string {
	var b strings.Builder
	for _, c := range profile.Current().ProgramCommands {
		fmt.Fprintf(&b, "  %s %-14s %s\n", product.CommandProgram, c.Name, c.Summary)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\nCommands that " + profile.Current().Name + "'s other programs carry out:\n" + b.String()
}

// forward runs program, which is installed beside this one, with args, and
// returns the status to end with.
func forward(program string, args []string) int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, product.CommandProgram+":", err)
		return 1
	}
	return forwardFrom(filepath.Dir(self), program, args)
}

func forwardFrom(dir, program string, args []string) int {
	path := filepath.Join(dir, programs.FileName(program))
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		fmt.Fprintf(os.Stderr, "%s: %s carries out %q, and it is not installed in %s\n", product.CommandProgram, programs.FileName(program), args[0], dir)
		return 1
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var ended *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ended):
		// The program has said what went wrong; its status is passed on.
		if code := ended.ExitCode(); code > 0 {
			return code
		}
		return 1
	default:
		fmt.Fprintf(os.Stderr, "%s: %s could not be started: %v\n", product.CommandProgram, programs.FileName(program), err)
		return 1
	}
}
