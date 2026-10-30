// Package agentpolicy contains host-owned rules for agent tool execution.
package agentpolicy

import (
	"regexp"
	"strings"

	"studio/internal/profile"
)

// The command guard covers direct CLI invocations, including quoted paths.
// It does not parse arbitrary shell programs or provide a security sandbox.
// The programs it covers are the build's (profile.ReservedPrograms).
var reservedCommand = reservedPattern(profile.Current().ReservedPrograms)

// reservedPattern matches a credential or flow-answer command of any of the programs.
func reservedPattern(programs []string) *regexp.Regexp {
	quoted := make([]string, len(programs))
	for i, p := range programs {
		quoted[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile(`(?i)\b(` + strings.Join(quoted, "|") + `)(?:\.exe)?["']?\s+(credential|flow\s+answer)\b`)
}

// ReservedCommand reports CLI operations that only the person may perform.
func ReservedCommand(line string) bool { return reservedCommand.MatchString(line) }
