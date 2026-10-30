package main

import (
	"os"
	"testing"
)

func TestTerminalCustomProfiles(t *testing.T) {
	a, _ := providerTestApp(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddTerminalProfile("Fixture shell", executable, []string{"argument with spaces"}); err != nil {
		t.Fatal(err)
	}
	profiles := a.TerminalProfiles()
	if len(profiles) != 1 {
		t.Fatal(profiles)
	}
	found := false
	for _, shell := range a.availableShells() {
		if shell.ID == profiles[0].ID {
			found = shell.Path == executable
		}
	}
	if !found {
		t.Fatal("custom shell was not discovered with literal arguments")
	}
	if err := a.SetTerminalShell(profiles[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveTerminalProfile(profiles[0].ID); err != nil {
		t.Fatal(err)
	}
	if len(a.TerminalProfiles()) != 0 || a.TerminalShells().Default == profiles[0].ID {
		t.Fatal("removed shell remained selected")
	}
	if err := a.AddTerminalProfile("Missing", "missing-dinah-shell-fixture", nil); err == nil {
		t.Fatal("accepted missing executable")
	}
}
