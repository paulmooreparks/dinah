// Command org-chart-seed creates invented local records for the hidden chart check.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func main() {
	if output := os.Getenv("DINAH_CHART_PROVIDER_REFUSAL"); output != "" {
		file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err == nil {
			data, _ := json.Marshal(os.Args[1:])
			_, _ = file.Write(append(data, '\n'))
			_ = file.Close()
		}
		fmt.Fprintln(os.Stderr, "Native provider access is disabled in the chart fixture.")
		os.Exit(1)
	}
	if err := seed(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func seed() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("supply a fresh fixture home")
	}
	home, err := filepath.Abs(os.Args[1])
	if err != nil {
		return err
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		return fmt.Errorf("the fixture home must be fresh")
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(home, "bus.db"))
	if err != nil {
		return err
	}
	defer b.Close()
	titles := map[string]string{}
	recipients := map[string]string{}
	natives := map[string]any{}
	providers := map[string]string{}
	sessions := map[string]bus.Session{}
	for index, name := range []string{"Lena", "Oren", "Tess", "Mara", "Ivo", "Nina", "Cal", "Sasha", "Bo", "Uma"} {
		workspace := filepath.Join(home, "workspaces", "Harbor")
		if index >= 6 {
			workspace = filepath.Join(home, "workspaces", "Orchard")
		}
		if err := os.MkdirAll(workspace, 0700); err != nil {
			return err
		}
		native := "codex:chart-" + name
		recipient := "chart-" + name
		record, err := b.RegisterSession(ctx, recipient, workspace, name)
		if err != nil {
			return err
		}
		sessions[name] = record
		titles[native] = name
		recipients[native] = recipient
		natives[native] = map[string]any{"provider": "codex"}
		providers[native] = "codex"
		if err := sessionhost.WriteTranscript(home, &sessionhost.Transcript{SessionID: native, Provider: "codex", Workspace: workspace, Complete: true}); err != nil {
			return err
		}
	}
	for member, lead := range map[string]string{"Oren": "Lena", "Tess": "Lena", "Ivo": "Mara", "Nina": "Mara", "Cal": "Nina", "Bo": "Sasha"} {
		if _, err := b.SetLead(ctx, sessions[member].ID, sessions[lead].ID); err != nil {
			return err
		}
	}
	lead := sessions["Lena"]
	if _, err := b.CreateGroup(ctx, bus.Sender{Session: &lead}, "Release Review", []string{sessions["Nina"].ID, sessions["Tess"].ID}); err != nil {
		return err
	}
	if _, err := b.CreateGroup(ctx, bus.Sender{}, "Research Exchange", []string{sessions["Cal"].ID, sessions["Bo"].ID}); err != nil {
		return err
	}
	settings := map[string]any{"hostNotificationsOff": true, "workspace": filepath.Join(home, "workspaces", "Harbor"), "sessionTitles": titles, "sessionProviders": providers, "busSessions": recipients, "nativeSessions": natives, "recent": []string{filepath.Join(home, "workspaces", "Harbor"), filepath.Join(home, "workspaces", "Orchard")}}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, "settings.json"), data, 0600)
}
