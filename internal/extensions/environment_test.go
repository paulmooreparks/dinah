package extensions

import (
	"slices"
	"strings"
	"testing"
)

func TestExtensionEnvironmentUsesAllowListAndSignedDeclarations(t *testing.T) {
	env := []string{"PATH=tools", "SystemRoot=system", "HOME=home", "PRIVATE_PASSWORD=canary-password", "UNEXPECTED_SECRET=canary-secret", "SERVICE_TOKEN=canary-token", "EXPLICIT_SECRET=canary-opt-in", "Path=last-tools", "invalid", "=drive-state", "HOME=bad\x00value"}
	for _, platform := range []string{"windows", "linux", "darwin"} {
		t.Run(platform, func(t *testing.T) {
			got := cleanEnvironmentFor(platform, env, []string{"EXPLICIT_SECRET"})
			text := strings.Join(got, "\n")
			for _, canary := range []string{"canary-password", "canary-secret", "canary-token", "bad\x00value"} {
				if strings.Contains(text, canary) {
					t.Fatal("undeclared or invalid inherited value escaped")
				}
			}
			if !slices.Contains(got, "EXPLICIT_SECRET=canary-opt-in") || !slices.Contains(got, "HOME=home") {
				t.Fatal("declared or standard value missing")
			}
			if platform == "windows" {
				if !slices.Contains(got, "Path=last-tools") || slices.Contains(got, "PATH=tools") {
					t.Fatal("Windows case folding or final duplicate value lost")
				}
			} else if !slices.Contains(got, "PATH=tools") || slices.Contains(got, "Path=last-tools") {
				t.Fatal("Unix environment names were case folded")
			}
			if got := cleanEnvironmentFor(platform, []string{"explicit_secret=hidden"}, []string{"EXPLICIT_SECRET"}); (len(got) == 1) != (platform == "windows") {
				t.Fatal("declared variable name matching differs from policy")
			}
		})
	}
	if got := cleanEnvironment(nil); got == nil || len(got) != 0 {
		t.Fatal("empty input must give an explicit empty child environment")
	}
}
