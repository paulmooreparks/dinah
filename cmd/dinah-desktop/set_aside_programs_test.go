package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"studio/internal/profile"
	"studio/internal/programs"
)

func TestSetAsideProgramsFollowTheProductProfile(t *testing.T) {
	p := profile.Profile{
		CommandProgram: "command", WindowProgram: "window",
		ToolServers:     []profile.ToolServer{{Program: "tools"}},
		ProgramCommands: []profile.ProgramCommand{{Program: "tools"}},
	}
	if got := setAsideProgramNames(p); !reflect.DeepEqual(got, []string{"command", "window", "tools", "tools"}) {
		t.Fatalf("program names: %v", got)
	}
	if pageMethods["ListSetAsidePrograms"] != pageQuery || pageMethods["RemoveSetAsidePrograms"] != pageWork {
		t.Fatal("set-aside methods have the wrong exit accounting")
	}
}

func TestSetAsidePageUsesOnlyTheConfiguredProductHome(t *testing.T) {
	home := t.TempDir()
	previous := configOverride
	configOverride = home
	t.Cleanup(func() { configOverride = previous })
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	name := programs.FileName(profile.Current().WindowProgram) + ".old-20261114120000000"
	if err := os.WriteFile(filepath.Join(bin, name), []byte("fallback"), 0o700); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	listed, err := a.ListSetAsidePrograms()
	if err != nil || len(listed.Items) != 1 || listed.Items[0].Name != name || !listed.Items[0].Fallback {
		t.Fatalf("page list: %+v %v", listed, err)
	}
	result, err := a.RemoveSetAsidePrograms(programs.RemoveSetAsideInput{IDs: []string{listed.Items[0].ID}})
	if err != nil || len(result.Items) != 1 || result.Items[0].ReasonCode != "fallback" {
		t.Fatalf("page fallback refusal: %+v %v", result, err)
	}
}
