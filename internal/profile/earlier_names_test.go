package profile

import (
	"strings"
	"testing"
)

// renamed is the example product after it gave its command line's earlier
// name to nobody and its tool program's earlier name to the command line:
// the command line was "ex" and is now "example-cli", and the tool program
// was "example-cli" and is now "example-tools".
func renamed() Profile {
	p := example()
	p.CommandProgram = "example-cli"
	p.EarlierCommandPrograms = []string{"ex"}
	p.ToolServers = []ToolServer{{Name: "example-cli", Program: "example-tools", Args: []string{"serve"}, EarlierPrograms: []string{"example-cli"}}}
	p.ProgramCommands = []ProgramCommand{{Name: "serve", Program: "example-tools", Summary: "serve the agent tools"}}
	return p
}

// A product that renamed its programs is accepted when the earlier names
// are sound, and Current copies the earlier names.
func TestEarlierProgramNamesAreAccepted(t *testing.T) {
	p, err := parseInjected(encode(t, renamed()))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.EarlierCommandPrograms) != 1 || p.EarlierCommandPrograms[0] != "ex" || p.ToolServers[0].EarlierPrograms[0] != "example-cli" {
		t.Fatalf("the earlier names were not read: %+v", p)
	}
	if len(Source().EarlierCommandPrograms) != 0 {
		t.Fatal("Dinah's own profile has earlier command program names")
	}
}

// An earlier command program name that is a program the product has now is
// refused, since the window removes that file. A tool server's earlier name
// that is the command program's name is refused unless the command program
// hands the server's first argument on to the server's program.
func TestUnsoundEarlierProgramNamesAreRefused(t *testing.T) {
	for name, change := range map[string]func(*Profile){
		"command program": func(p *Profile) { p.EarlierCommandPrograms = []string{"example-cli"} },
		"window program":  func(p *Profile) { p.EarlierCommandPrograms = []string{"example-desktop"} },
		"tool program":    func(p *Profile) { p.EarlierCommandPrograms = []string{"example-tools"} },
		"not a name":      func(p *Profile) { p.EarlierCommandPrograms = []string{"Ex Program"} },
		"own program":     func(p *Profile) { p.ToolServers[0].EarlierPrograms = []string{"example-tools"} },
	} {
		p := renamed()
		change(&p)
		if _, err := parseInjected(encode(t, p)); err == nil || !strings.Contains(err.Error(), "Earlier") {
			t.Errorf("%s: an unsound earlier name was accepted: %v", name, err)
		}
	}
	p := renamed()
	p.ProgramCommands = nil
	if _, err := parseInjected(encode(t, p)); err == nil || !strings.Contains(err.Error(), "hands its first argument, serve, to example-tools") {
		t.Fatalf("a reused name without the command handed on was accepted: %v", err)
	}
}
