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

// Keys lists the settings the file carries, in the order it carries them,
// including the ones the tool does not know. A reader listing the settings
// needs those too, since a key the tool never heard of survives every write
// and is still a key somebody set.
func (c *Config) Keys() []string {
	return c.fm.Keys()
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

// The names of the rungs a resolution ladder can answer at. They are machine
// tokens: `config settings` reports one beside each value, and the JSON form
// carries these spellings in every language.
//
// SourceEditorVar names the DINAH_EDITOR rung on its own, rather than folding
// it into SourceFlag: the editor ladder has no flag, and reporting a flag
// where none exists is the defect this card exists to remove. It renders as
// the variable's own name, the same convention SourceVisual already uses for
// VISUAL, so a reader sees which of the two tool-specific variables answered
// rather than a mechanism the tool does not have.
// SourceVisual and SourceEnvironment stay apart on that ladder so that a
// reader whose VISUAL won can see that it was VISUAL and not EDITOR.
const (
	SourceFlag        = "flag"
	SourceEnvironment = "environment"
	SourceVisual      = "visual"
	SourceEditorVar   = "dinah-editor"
	SourceConfig      = "config"
	SourceLocale      = "locale"
	SourceDefault     = "default"
	SourceFallback    = "fallback"
	SourceUnset       = "unset"
	SourceUnknown     = "unknown"
)

// Layer is one rung of a resolution ladder: what the rung carries, and the
// name a listing reports when that rung is the one that answered.
type Layer struct {
	// Source is the rung's name, one of the source constants above.
	Source string
	// Value is what the rung carries, empty when nothing set it.
	Value string
}

// Resolve resolves a value from layers tried in order, first hit wins, and
// names the rung that produced it. Every ladder below is stated once, here,
// so that a listing reporting which rung answered cannot drift from the
// resolution a caller actually gets.
//
// A value carried at no rung comes back empty with an empty source, and the
// ladder above decides what that absence means.
func Resolve(layers ...Layer) (string, string) {
	for _, layer := range layers {
		if value := strings.TrimSpace(layer.Value); value != "" {
			return value, layer.Source
		}
	}
	return "", ""
}

// Ladder resolves a value from layers tried in order, first hit wins. All
// three of the resolution ladders have this shape, so a reader learns
// flag-then-environment-then-config once and the code says it once. It is
// Resolve for the callers that need the value and not the rung.
func Ladder(values ...string) string {
	layers := make([]Layer, 0, len(values))
	for _, value := range values {
		layers = append(layers, Layer{Value: value})
	}
	resolved, _ := Resolve(layers...)
	return resolved
}

// ResolveActor resolves the owner an act is attributed to, by the ladder the
// format's actors section fixes: the flag, then the environment, then the
// user config. An actor resolvable at no layer is refused rather than
// invented, because the format refuses to write an event with no actor.
func ResolveActor(flag string, cfg *Config) (string, error) {
	actor, _ := ResolveActorSource(flag, cfg)
	if actor == "" {
		return "", contract.Refuse(contract.NoOwner, "")
	}
	return actor, nil
}

// ResolveActorSource resolves the owner and names the rung that answered. An
// owner no rung carries comes back empty with the source unset, because a
// listing reports an absence rather than refusing over it; ResolveActor is the
// form that refuses.
func ResolveActorSource(flag string, cfg *Config) (string, string) {
	actor, source := Resolve(
		Layer{Source: SourceFlag, Value: flag},
		Layer{Source: SourceEnvironment, Value: os.Getenv("DINAH_ACTOR")},
		Layer{Source: SourceConfig, Value: cfg.Get("actor")},
	)
	if actor == "" {
		return "", SourceUnset
	}
	return actor, source
}

// ResolveLang resolves the display language, by the ladder the format's
// display-language section fixes: the flag, then the environment, then the
// user config, then the operating system locale as a hint, then English.
func ResolveLang(flag string, cfg *Config) string {
	tag, _ := ResolveLangSource(flag, cfg)
	return tag
}

// ResolveLangSource resolves the display language and names the rung that
// answered. English reached with no rung carrying a tag reports the source
// default, which is what tells a reader that nobody chose the language they
// are looking at.
func ResolveLangSource(flag string, cfg *Config) (string, string) {
	tag, source := Resolve(
		Layer{Source: SourceFlag, Value: flag},
		Layer{Source: SourceEnvironment, Value: os.Getenv("DINAH_LANG")},
		Layer{Source: SourceConfig, Value: cfg.Get("lang")},
		Layer{Source: SourceLocale, Value: osLocale()},
	)
	if tag == "" {
		return "en", SourceDefault
	}
	return NormalizeTag(tag), source
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
	editor, _, err := ResolveEditorSource(cfg, goos, lookPath)
	return editor, err
}

// ResolveEditorSource resolves the editor and names which of the five rungs
// answered, which is the whole point of reporting the ladder at all: an
// editor that came from VISUAL and an editor that came from EDITOR are
// different answers to why `edit` opened what it opened.
//
// A refusal comes back with an empty editor and the source unset, so a
// listing can report the absence without treating it as a failure.
func ResolveEditorSource(cfg *Config, goos string, lookPath func(string) bool) (string, string, error) {
	chosen, source := Resolve(
		Layer{Source: SourceEditorVar, Value: os.Getenv("DINAH_EDITOR")},
		Layer{Source: SourceConfig, Value: cfg.Get("editor")},
		Layer{Source: SourceVisual, Value: os.Getenv("VISUAL")},
		Layer{Source: SourceEnvironment, Value: os.Getenv("EDITOR")},
	)
	if chosen != "" {
		return chosen, source, nil
	}
	candidates, ok := EditorFallback[goos]
	if !ok {
		candidates = EditorFallback["default"]
	}
	for _, candidate := range candidates {
		if lookPath == nil || lookPath(candidate) {
			return candidate, SourceFallback, nil
		}
	}
	return "", SourceUnset, contract.Refuse(contract.NoEditor, strings.Join(candidates, ", "))
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

// SlugPattern is the shape a workbench slug has to take, as the reference
// grammar fixes it: an ASCII letter followed by ASCII letters and digits.
// The slug is the half of a card reference a person types, so it stays inside
// the character set every filesystem and every shell agrees about.
const SlugPattern = "^[a-z][a-z0-9]*$"

// Slugify derives a conforming slug from a name that need not conform, which
// is what a bench created in a directory called "My Project" needs. Letters
// lowercase by ASCII rules, digits survive, everything else is dropped, and a
// leading run of digits goes with it, since the grammar wants a letter first.
//
// A name yielding nothing usable returns the empty string, and the caller
// refuses rather than inventing a name of its own.
func Slugify(name string) string {
	lowered := asciiLower(name)
	var kept []byte
	for i := 0; i < len(lowered); i++ {
		c := lowered[i]
		letter := c >= 'a' && c <= 'z'
		digit := c >= '0' && c <= '9'
		if letter || (digit && len(kept) > 0) {
			kept = append(kept, c)
		}
	}
	return string(kept)
}

// ValidSlug reports whether a slug already conforms to SlugPattern.
func ValidSlug(slug string) bool {
	return slug != "" && Slugify(slug) == slug
}
