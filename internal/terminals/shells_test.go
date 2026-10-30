package terminals

import (
	"os"
	"testing"
)

func TestProfileArgumentsRemainLiteral(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	shells := Available("Dinah", []Profile{{ID: "custom", Name: "Fixture", Path: executable, Args: []string{"argument with spaces"}}})
	for _, shell := range shells {
		if shell.ID == "custom" {
			if shell.Path != executable || len(shell.args) != 1 || shell.args[0] != "argument with spaces" {
				t.Fatalf("custom shell: %+v", shell)
			}
			return
		}
	}
	t.Fatal("custom shell was not discovered")
}
