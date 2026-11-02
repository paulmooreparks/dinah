package extensions

import (
	"runtime"
	"strings"
)

// standardEnvironment is the documented inheritance policy in
// docs/design/extension-environment.md. Other names require a signed declaration.
var standardEnvironment = []string{
	"PATH", "PATHEXT", "SystemRoot", "WINDIR", "COMSPEC", "TEMP", "TMP", "TMPDIR",
	"HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "USER", "USERNAME", "LOGNAME",
	"LANG", "LC_ALL", "LC_CTYPE", "TZ",
	"APPDATA", "LOCALAPPDATA", "ProgramData", "ProgramFiles", "ProgramFiles(x86)",
	"SystemDrive", "NUMBER_OF_PROCESSORS", "PROCESSOR_ARCHITECTURE",
}

func cleanEnvironment(env []string, declared ...string) []string {
	return cleanEnvironmentFor(runtime.GOOS, env, declared)
}

func cleanEnvironmentFor(platform string, env, declared []string) []string {
	keyOf := func(name string) string {
		if platform == "windows" {
			return strings.ToUpper(name)
		}
		return name
	}
	allowed := map[string]bool{}
	for _, names := range [][]string{standardEnvironment, declared} {
		for _, name := range names {
			allowed[keyOf(name)] = true
		}
	}
	out := []string{}
	positions := map[string]int{}
	for _, entry := range env {
		name, _, valid := strings.Cut(entry, "=")
		key := keyOf(name)
		if !valid || name == "" || strings.ContainsRune(entry, 0) || !allowed[key] {
			continue
		}
		if at, exists := positions[key]; exists {
			out[at] = entry
		} else {
			positions[key] = len(out)
			out = append(out, entry)
		}
	}
	return out
}
