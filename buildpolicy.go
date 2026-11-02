// Package buildpolicy exposes the module's build requirements to installed
// programs without maintaining a second copy of the required compiler version.
package buildpolicy

import (
	_ "embed"
	"fmt"
	"go/version"
	"strings"
)

//go:embed go.mod
var moduleFile string

// Toolchain returns the exact toolchain selected by this build's go.mod.
// It also checks that local builds cannot select an older minimum compiler.
func Toolchain() (string, error) { return toolchain(moduleFile) }

func toolchain(module string) (string, error) {
	var compiler, minimum string
	for line := range strings.SplitSeq(module, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "toolchain":
			compiler = fields[1]
		case "go":
			minimum = "go" + fields[1]
		}
	}
	// Go documents an implicit toolchain equal to the go directive when no
	// explicit toolchain is present: https://go.dev/doc/toolchain.
	if compiler == "" {
		compiler = minimum
	}
	if !version.IsValid(compiler) || compiler != minimum {
		return "", fmt.Errorf("go.mod must select the same exact Go version in go and toolchain directives")
	}
	return compiler, nil
}
