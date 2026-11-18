package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"studio/internal/bus"
	"studio/internal/sessionhost"
	"studio/internal/statefile"
	"studio/internal/workspaces"
)

// The command line's "workspace export" takes the workspace's chats through
// this program, which keeps the logical chat records that say what a chat
// is (docs/design/workspace-export.md). Started with workspaceExportOption,
// the program writes the export with every chat of the workspace and ends,
// with no window. It reads the window's state and never changes it, so it
// can run while the window does: the settings are read as they are on disk,
// without finishing a save the window has pending, and the registry through
// statefile.ReadShared, as the command line reads it. Only the exporter
// identity may be made, in its own file (workspaces.ExporterIdentity).
const workspaceExportOption = "--export-workspace"

// runWorkspaceExport carries out workspaceExportOption with its arguments,
// --workspace ID and --out FILE, and returns the status to end with.
func runWorkspaceExport(args []string) int {
	if err := workspaceExportCommand(args); err != nil {
		fmt.Fprintln(os.Stderr, product.WindowProgram+":", err)
		return 1
	}
	return 0
}

func workspaceExportCommand(args []string) error {
	flags := flag.NewFlagSet(workspaceExportOption, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	id := flags.String("workspace", "", "the workspace's ID")
	out := flags.String("out", "", "the file to write")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *id == "" || *out == "" {
		return errors.New("usage: " + workspaceExportOption + " --workspace ID --out FILE")
	}
	file, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	if err := initializeStateDirectory(); err != nil {
		return err
	}
	saved, err := readSettingsForExport()
	if err != nil {
		return err
	}
	index := workspaces.Index(saved.WorkspaceRecords, *id)
	if index < 0 {
		return errors.New("that workspace is not registered")
	}
	_, headers := sessionhost.ConversationLinks(configDir())
	chats := readWorkspaceChats(saved, headers, saved.WorkspaceRecords[index])
	chosen := map[string]bool{}
	for _, chat := range chats.chats {
		chosen[chat.chatID] = true
	}
	exporter := ""
	if len(chosen) > 0 {
		if exporter, err = workspaces.ExporterIdentity(configDir()); err != nil {
			return err
		}
	}
	b, err := bus.OpenAt(filepath.Join(configDir(), "bus.db"))
	if err != nil {
		return err
	}
	defer b.Close()
	home, _ := userHome()
	return workspaces.WriteArchiveFile(file, func(w io.Writer) error {
		return chats.writeArchive(context.Background(), w, b, chosen, file, home, exporter)
	})
}

// readSettingsForExport reads the window's settings, its list of chats and
// the workspace registry, writing nothing. A settings file the window is in
// the middle of replacing is read as it is at that moment, the old content
// or the new.
func readSettingsForExport() (Settings, error) {
	var s Settings
	dir := configDir()
	data, err := statefile.ReadRegular(filepath.Join(dir, "settings.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return s, err
	}
	if err == nil {
		if err := json.Unmarshal(data, &s); err != nil {
			return s, fmt.Errorf("read settings: %w", err)
		}
	}
	projection, err := statefile.ReadRegular(filepath.Join(dir, "chat-inventory.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return s, err
	}
	if err == nil {
		if err := mergeListedChatProjection(&s, projection); err != nil {
			return s, err
		}
	}
	records, err := workspaces.Load(dir)
	switch {
	case err == nil:
		s.WorkspaceRecords = records
	case errors.Is(err, workspaces.ErrNoRegistry):
		s.WorkspaceRecords = s.LegacyWorkspaceRecords
	default:
		return s, err
	}
	s.LegacyWorkspaceRecords = nil
	return s, nil
}
