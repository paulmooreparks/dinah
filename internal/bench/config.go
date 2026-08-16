package bench

import (
	"os"
	"path/filepath"
	"strings"

	"dinah/internal/contract"
)

// ConfigKeys are the settings v0 knows. A key outside this set is refused,
// and a key already in the file that the tool does not know survives a write,
// which is the same reader posture the format asks of every other document.
var ConfigKeys = []string{"lang", "actor", "editor"}

// Config is the user's own settings, held in config.md in the user base.
type Config struct {
	// Path is the file the settings were read from.
	Path string
	fm   *Frontmatter
	body string
}

// LoadConfig reads the user's settings. An absent file is an empty
// configuration rather than an error, because nobody has to write one.
func LoadConfig(home string) *Config {
	path := filepath.Join(UserBase(home), ConfigName)
	text, err := ReadText(path)
	if err != nil {
		return &Config{Path: path, fm: NewFrontmatter()}
	}
	fm, body := ParseAnchor(text)
	return &Config{Path: path, fm: fm, body: body}
}

// Get reads one setting.
func (c *Config) Get(key string) string {
	return c.fm.Value(key)
}

// Set writes one setting, preserving every key the tool does not recognise.
func (c *Config) Set(key, value string) error {
	if !KnownConfigKey(key) {
		return contract.Refuse(contract.UnknownKey, key)
	}
	c.fm.Set(key, value)
	return WriteText(c.Path, c.fm.Render(c.body))
}

// KnownConfigKey reports whether a key is one v0 knows.
func KnownConfigKey(key string) bool {
	for _, known := range ConfigKeys {
		if known == key {
			return true
		}
	}
	return false
}

// Ladder resolves a value from layers tried in order, first hit wins. All
// three of the resolution ladders have this shape, so a reader learns
// flag-then-environment-then-config once and the code says it once.
func Ladder(layers ...string) string {
	for _, layer := range layers {
		if value := strings.TrimSpace(layer); value != "" {
			return value
		}
	}
	return ""
}

// ResolveActor resolves the owner an act is attributed to, by the ladder the
// format's actors section fixes: the flag, then the environment, then the
// user config. An actor resolvable at no layer is refused rather than
// invented, because the format refuses to write an event with no actor.
func ResolveActor(flag string, cfg *Config) (string, error) {
	actor := Ladder(flag, os.Getenv("DINAH_ACTOR"), cfg.Get("actor"))
	if actor == "" {
		return "", contract.Refuse(contract.NoOwner, "")
	}
	return actor, nil
}

// ResolveLang resolves the display language, by the ladder the format's
// display-language section fixes: the flag, then the environment, then the
// user config, then the operating system locale as a hint, then English.
func ResolveLang(flag string, cfg *Config) string {
	tag := Ladder(flag, os.Getenv("DINAH_LANG"), cfg.Get("lang"), osLocale())
	if tag == "" {
		return "en"
	}
	return NormalizeTag(tag)
}

// osLocale reads the operating system's locale as a hint. It describes the
// machine rather than the person reading the screen, which is why it sits
// below the user config rather than above it.
func osLocale() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := os.Getenv(name)
		if value == "" || strings.HasPrefix(value, "C") || strings.HasPrefix(value, "POSIX") {
			continue
		}
		tag, _, _ := strings.Cut(value, ".")
		tag, _, _ = strings.Cut(tag, "@")
		return strings.ReplaceAll(tag, "_", "-")
	}
	return ""
}

// NormalizeTag puts a language tag into the spelling the catalogs use: the
// language subtag in ASCII lowercase, the region subtag in ASCII uppercase,
// and the common miswriting en-UK read as en-GB.
func NormalizeTag(tag string) string {
	language, region, ok := strings.Cut(strings.TrimSpace(tag), "-")
	language = asciiLower(language)
	if !ok {
		return language
	}
	region = asciiUpper(region)
	if language == "en" && region == "UK" {
		region = "GB"
	}
	return language + "-" + region
}

// asciiUpper uppercases using ASCII rules alone, for the same reason
// asciiLower lowercases with them.
func asciiUpper(s string) string {
	out := []byte(s)
	for i := 0; i < len(out); i++ {
		if out[i] >= 'a' && out[i] <= 'z' {
			out[i] -= 'a' - 'A'
		}
	}
	return string(out)
}

// EditorFallback is the editor each platform falls back to once every layer
// of the ladder above it is unset. Windows is settled as notepad by the
// ruling; elsewhere the tool tries the editor an unfamiliar user can leave
// without a manual before the one POSIX guarantees is present.
var EditorFallback = map[string][]string{
	"windows": {"notepad"},
	"default": {"nano", "vi"},
}

// ResolveEditor resolves the editor `edit` opens, by the ladder the operator
// ruled: DINAH_EDITOR, then the user config, then VISUAL, then EDITOR, then
// the platform fallback. The tool-specific variable sits at the top because
// EDITOR is shared global state, and somebody who wants git and dinah opening
// different editors has nowhere else to say so.
//
// lookPath is the search that decides whether a fallback binary is present,
// passed in so a test can drive the ladder without depending on what the
// machine running it happens to have installed.
func ResolveEditor(cfg *Config, goos string, lookPath func(string) bool) (string, error) {
	chosen := Ladder(os.Getenv("DINAH_EDITOR"), cfg.Get("editor"), os.Getenv("VISUAL"), os.Getenv("EDITOR"))
	if chosen != "" {
		return chosen, nil
	}
	candidates, ok := EditorFallback[goos]
	if !ok {
		candidates = EditorFallback["default"]
	}
	for _, candidate := range candidates {
		if lookPath == nil || lookPath(candidate) {
			return candidate, nil
		}
	}
	return "", contract.Refuse(contract.NoEditor, strings.Join(candidates, ", "))
}

// GlobalInstructions reads the user-global instruction layer, which is the
// first layer of the served chain. An absent file is an absent layer rather
// than an error, because most machines will never carry one.
func GlobalInstructions(home string) string {
	text, err := ReadText(filepath.Join(UserBase(home), InstructionsName))
	if err != nil {
		return ""
	}
	return text
}
