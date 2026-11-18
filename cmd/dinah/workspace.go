package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/programs"
	"studio/internal/workspaces"
)

// The workspace command reads the window's workspace registry and never
// changes it: it lists the workspaces and writes a workspace's export,
// which Import Workspace… in the window reads on another computer
// (docs/design/workspace-export.md). Importing is done in the window, since
// it asks the person about folders and changes the registry.
//
// An export takes the workspace's chats unless --no-chats is given. What a
// chat is, and which chats a workspace has, is kept by the window program,
// so the export with chats is written by that program, installed beside
// this one and started with no window (its --export-workspace option). The
// export without chats is written here.
func workspaceCommand(args []string, stdout io.Writer) error {
	usage := errors.New("usage: " + product.CommandProgram + " workspace list | export WORKSPACE [--out FILE] [--no-chats]")
	if len(args) == 0 {
		return usage
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return usage
		}
		records, err := loadWorkspaces()
		if err != nil {
			return err
		}
		type row struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			MainFolder string `json:"mainFolder"`
			Sources    int    `json:"sources"`
		}
		out := []row{}
		for _, record := range records {
			out = append(out, row{ID: record.ID, Name: record.Name, MainFolder: workspaces.ExecutionPath(record), Sources: len(record.Sources)})
		}
		return writeIndented(stdout, out)
	case "export":
		flags := flag.NewFlagSet("workspace export", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		to := flags.String("out", "", "the file to write, ending in "+workspaces.ExportExtension)
		noChats := flags.Bool("no-chats", false, "leave the workspace's chats out")
		// The workspace may come before or after --out.
		rest := []string{}
		for list := args[1:]; ; {
			if err := flags.Parse(list); err != nil {
				return usage
			}
			if flags.NArg() == 0 {
				break
			}
			rest = append(rest, flags.Arg(0))
			list = flags.Args()[1:]
		}
		if len(rest) != 1 {
			return usage
		}
		records, err := loadWorkspaces()
		if err != nil {
			return err
		}
		record, err := findWorkspace(records, rest[0])
		if err != nil {
			return err
		}
		file := ""
		if *to != "" {
			if file, err = filepath.Abs(*to); err != nil {
				return err
			}
		}
		// Without --out the archive is written to the standard output, and
		// its local folders can then be found on the other computer only from
		// the home folder or in full.
		target := file
		if file == "" {
			dir, err := os.MkdirTemp("", "workspace-export-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(dir)
			target = filepath.Join(dir, "export"+workspaces.ExportExtension)
		}
		if *noChats {
			home, _ := os.UserHomeDir()
			err = workspaces.WriteArchiveFile(target, func(w io.Writer) error {
				return workspaces.NewArchiveWriter(w).Finish(workspaces.BuildExport(record, file, home))
			})
		} else {
			err = exportWithChats(record.ID, target)
		}
		if err != nil {
			return err
		}
		if file == "" {
			f, err := os.Open(target)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(stdout, f)
			return err
		}
		_, err = fmt.Fprintln(stdout, file)
		return err
	}
	return usage
}

// exportWithChats has the window program, installed beside this one, write
// a workspace's export with its chats to file. It is a variable so that a
// test can stand in for the window program.
var exportWithChats = func(workspaceID, file string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	program := filepath.Join(filepath.Dir(self), programs.FileName(product.WindowProgram))
	if info, err := os.Stat(program); err != nil || info.IsDir() {
		return fmt.Errorf("%s writes an export with chats, and it is not installed in %s; use --no-chats to export without them", programs.FileName(product.WindowProgram), filepath.Dir(self))
	}
	cmd := exec.Command(program, "--export-workspace", "--workspace", workspaceID, "--out", file)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	acp.HideWindow(cmd)
	if err := cmd.Run(); err != nil {
		if text := strings.TrimSpace(stderr.String()); text != "" {
			return errors.New(text)
		}
		return fmt.Errorf("%s could not write the export: %w", product.WindowProgram, err)
	}
	return nil
}

// writeIndented prints a value as indented JSON.
func writeIndented(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// loadWorkspaces reads the registry from the home folder the window uses.
// A registry that does not exist yet has not been moved out of the window's
// settings, which is different from one with no workspaces, so it is said
// to be so and not listed as empty.
func loadWorkspaces() ([]workspaces.Record, error) {
	home, err := bus.Home()
	if err != nil {
		return nil, err
	}
	records, err := workspaces.Load(home)
	if errors.Is(err, workspaces.ErrNoRegistry) {
		return nil, errors.New("the workspace registry has not been moved to its own file yet; start " + product.Name + " once, which moves it, and run this again")
	}
	return records, err
}

// findWorkspace finds a workspace by its ID, its main folder or its name,
// a name matching regardless of case and only when one workspace has it.
func findWorkspace(records []workspaces.Record, which string) (workspaces.Record, error) {
	for _, record := range records {
		if record.ID == which {
			return record, nil
		}
	}
	// A path is told from a name by its separator.
	if strings.ContainsAny(which, `/\`) {
		if abs, err := filepath.Abs(which); err == nil {
			if i := workspaces.IndexAt(records, abs); i >= 0 {
				return records[i], nil
			}
		}
	}
	var found []workspaces.Record
	for _, record := range records {
		if strings.EqualFold(record.Name, which) {
			found = append(found, record)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return workspaces.Record{}, fmt.Errorf("no workspace has the ID, name or main folder %q; %s workspace list shows them", which, product.CommandProgram)
	}
	return workspaces.Record{}, errors.New("more than one workspace has that name; give its ID or main folder instead")
}
