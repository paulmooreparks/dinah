package agents

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// PersonalSkillsDir is the person's own Devin skills folder, which Devin reads
// in every workspace (https://docs.devin.ai/cli/extensibility/skills/overview):
// %APPDATA%\devin\skills on Windows, ~/.config/devin/skills elsewhere.
func PersonalSkillsDir() (string, error) {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", errors.New("the APPDATA folder is not set")
		}
		return filepath.Join(appData, "devin", "skills"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "devin", "skills"), nil
}

// SkillPrompt is a prompt that calls a workspace skill with "/name", for a
// provider that does not read skills itself, with the skill's instructions
// put in.
func SkillPrompt(name, text, instructions string) string {
	return fmt.Sprintf("Use the selected workspace skill /%s for this request.\n\n%s\n\nThese are skill instructions and do not change tool permissions.\n\n%s", name, text, instructions)
}
