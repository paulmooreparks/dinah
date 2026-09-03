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
var ConfigKeys = []string{"lang", "actor", "editor", "workbench"}

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
//
// An empty value removes the key from the file rather than storing an empty
// one. The two read alike to every resolver, since a ladder skips a blank
// rung, so a person told to clear a setting gets a file that no longer carries
// the key at all, and `config` reports it as unset because nothing carries it
// any more. The file itself stays, holding whatever else was in it, and so
// does the directory around it. The removal has to be decided before
// filepath.Abs runs rather than inferred from its result, since
// filepath.Abs("") resolves to the current directory instead of to nothing.
//
// The workbench key gets one special case: a value the caller does keep is
// resolved to an absolute path before it is stored. A relative path means one
// thing at write time, when the current directory supplies it, and a different
// thing at read time, when a later invocation stands wherever the person or
// agent happens to be that day; resolving now is what makes the stored value
// mean the same directory both times.
func (c *Config) Set(key, value string) error {
	if !KnownConfigKey(key) {
		return contract.Refuse(contract.UnknownKey, key)
	}
	if strings.TrimSpace(value) == "" {
		c.fm.Delete(key)
		return WriteText(c.Path, c.fm.Render(c.body))
	}
	if key == "workbench" {
		abs, err := filepath.Abs(value)
		if err != nil {
			return err
		}
		value = abs
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
	// SourceSearch names the rung a workbench answer came from when the
	// ancestor walk resolved it, alongside the other rungs a listing or a
	// status line already names, so the two vocabularies never diverge.
	SourceSearch   = "search"
	SourceLocale   = "locale"
	SourceDefault  = "default"
	SourceFallback = "fallback"
	SourceUnset    = "unset"
	SourceUnknown  = "unknown"
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

// ResolveWorkbenchSource resolves which workbench a session would open right
// now, without opening it, sharing DiscoverSource's own resolution rather
// than restating it, so a listing can never report a rung that would not
// actually answer. flag and env are the two override layers a session
// resolves ahead of everything else; they are combined here the way session
// combines them, so the config listing runs the identical ladder a real
// invocation would.
//
// A refusal collapses to an empty value and the unset source. `dinah config`
// has to keep answering for every other setting even when no workbench is
// reachable at all or the stored default has gone stale; only an actual
// workbench-opening command treats that refusal as fatal.
func ResolveWorkbenchSource(start, flag, env, home, nativeHome, configured string) (string, string) {
	override, overrideSource := Resolve(
		Layer{Source: SourceFlag, Value: flag},
		Layer{Source: SourceEnvironment, Value: env},
	)
	root, source, _, _, err := DiscoverSource(
		start,
		override,
		overrideSource,
		home,
		nativeHome,
		configured,
	)
	if err != nil {
		return "", SourceUnset
	}
	return root, source
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

// SlugPattern is the shape a workbench slug has to take: an ASCII letter, then
// runs of ASCII letters and digits separated by single dashes, with no dash
// leading, trailing or doubling.
//
// The pattern alone describes one string ValidSlug refuses, and the exclusion
// is stated here in words because this constant is the only statement of the
// grammar anywhere in the code. A workbench slug may not end in a dash
// followed by a segment of digits alone. A card reference splits at its last
// dash, so `sprint-2` is read as card 2 of the prefix `sprint`, and a slug of
// that shape names a card rather than the workbench that carries it.
const SlugPattern = "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"

// Slugify derives a conforming workbench slug from a name that need not
// conform, which is what a workbench created in a directory called "Dinah
// development" needs. The name goes through SlugifyDashed, so letters
// lowercase by ASCII rules, digits survive, and each run of characters outside
// [a-z0-9] becomes one dash. The result is then repaired against the exclusion
// SlugPattern states: while the final dash-separated segment is digits alone,
// the dash in front of it is removed, so "Sprint 2" yields sprint2 rather than
// a slug the reference grammar would read as card 2.
//
// A name yielding nothing usable returns the empty string, and the caller
// refuses rather than inventing a name of its own.
func Slugify(name string) string {
	slug := SlugifyDashed(name)
	for {
		cut := strings.LastIndex(slug, "-")
		if cut < 0 || !allDigits(slug[cut+1:]) {
			return slug
		}
		slug = slug[:cut] + slug[cut+1:]
	}
}

// ValidSlug reports whether a slug already conforms to the workbench slug
// grammar: the column slug grammar, and a final segment carrying at least one
// letter. The exclusion sits beside the round trip rather than inside the
// pattern because ValidColumnSlug is itself a round trip rather than a match.
func ValidSlug(slug string) bool {
	return ValidColumnSlug(slug) && !allDigits(finalSegment(slug))
}

// finalSegment is what a card reference would read as a card number: whatever
// follows the slug's last dash, or the whole slug when it carries none.
func finalSegment(slug string) string {
	if cut := strings.LastIndex(slug, "-"); cut >= 0 {
		return slug[cut+1:]
	}
	return slug
}

// allDigits reports whether text is one or more ASCII digits and nothing else.
// The empty string is not, since a slug of no final segment is refused by the
// round trip rather than by the exclusion.
func allDigits(text string) bool {
	if text == "" {
		return false
	}
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return true
}

// ColumnSlugPattern is the shape a column slug has to take: an ASCII letter
// opens it, each dash separates two runs of ASCII letters and digits, and no
// dash leads, trails or doubles.
//
// It is the workbench slug's pattern without that grammar's one exclusion. A
// card reference splits at its last dash and takes what follows as the card's
// number, so a workbench slug ending in a dash and digits alone would read as
// a card reference. Nothing rides after a column slug, so a column may end in
// `-2` and the two grammars stay two names rather than becoming one.
const ColumnSlugPattern = "^[a-z][a-z0-9]*(-[a-z0-9]+)*$"

// SlugifyDashed derives a conforming column slug from a name that need not
// conform. The name lowercases by ASCII rules, digits survive, each run of
// characters outside [a-z0-9] becomes one dash, and no dash is left leading or
// trailing. A leading run of digits goes with any dash behind it, since the
// grammar wants a letter first.
//
// Slugify is this function plus one repair, so a workbench slug and a column
// slug derive alike until the exclusion the workbench grammar carries applies.
//
// A name yielding nothing usable returns the empty string, and the caller
// refuses rather than inventing a name of its own, exactly as Slugify's own
// callers do.
func SlugifyDashed(name string) string {
	lowered := asciiLower(name)
	var kept []byte
	for i := 0; i < len(lowered); i++ {
		c := lowered[i]
		letter := c >= 'a' && c <= 'z'
		digit := c >= '0' && c <= '9'
		if letter || digit {
			kept = append(kept, c)
			continue
		}
		if len(kept) > 0 && kept[len(kept)-1] != '-' {
			kept = append(kept, '-')
		}
	}
	for len(kept) > 0 && (kept[0] < 'a' || kept[0] > 'z') {
		kept = kept[1:]
	}
	for len(kept) > 0 && kept[len(kept)-1] == '-' {
		kept = kept[:len(kept)-1]
	}
	return string(kept)
}

// ValidColumnSlug reports whether a slug already conforms to ColumnSlugPattern.
func ValidColumnSlug(slug string) bool {
	return slug != "" && SlugifyDashed(slug) == slug
}
