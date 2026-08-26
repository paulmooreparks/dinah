// Package msg is the tool's own voice. Every string a person reads reaches
// them through a catalog here, from the first commit, so that adding a
// language is adding a catalog rather than hunting literals through the code.
//
// Machine vocabulary never passes through this package. Refusal names,
// states, column kinds, outcome names, block kinds, link kinds, command
// names, flag names and the interchange member names travel as the profile
// spells them on every surface under every language setting, which is what
// CORE-TEXT-3 requires and what CORE-TEXT-4 leaves room around.
package msg

import (
	"embed"
	"encoding/json"
	"hash/fnv"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Base is the language every other catalog falls back to, per key rather than
// per catalog, so an incomplete translation degrades one string at a time.
const Base = "en"

//go:embed locales/*.json
var locales embed.FS

// Entry is one message: the text a reader sees and the context a translator
// needs in order to render it.
type Entry struct {
	// Text is the message, carrying named placeholders in braces.
	Text string `json:"text"`
	// Context tells a translator what the message is for and what each
	// placeholder will hold.
	Context string `json:"context,omitempty"`
	// Skeleton marks an entry carrying the English text unchanged, which is
	// what a generated catalog ships until somebody translates it.
	Skeleton bool `json:"skeleton,omitempty"`
	// Verbatim marks an entry a translator really did translate and whose
	// answer is the English text, letter for letter, because the language
	// uses the same word. A table heading reading "Name" in German is the
	// ordinary case.
	//
	// The flag exists so that the guard over the catalogs can tell such an
	// entry from English left standing where a translation should be. Every
	// other entry that is not a skeleton is required to differ from its
	// English source, which is what makes a catalog quietly refilled with
	// English fail rather than pass, and a language does not have to be on
	// the Complete roster for that to hold. Nothing outside the guard reads
	// this field: to a reader a verbatim entry is an ordinary translation,
	// and Coverage counts it as one.
	Verbatim bool `json:"verbatim,omitempty"`
	// Source is a fingerprint of the English text this entry was translated
	// from, written when the entry is translated and checked against the
	// current base entry by TestATranslationTracksItsEnglishSource. It is
	// empty on the base catalog itself and on any entry carrying Skeleton,
	// neither of which is a translation of anything.
	Source string `json:"source,omitempty"`
}

// Fingerprint returns a short, stable digest of text, and it is the one place
// this project computes that digest. A translated entry records the
// fingerprint of the English it was translated from, and the guard over the
// catalogs recomputes it from the English of the day to find out whether the
// translation has fallen behind. Two calls on the same text return the same
// value on any machine and under any Go release, because FNV-1a is a pure
// function of the bytes handed to it and hash/fnv fixes the algorithm.
func Fingerprint(text string) string {
	digest := fnv.New64a()
	digest.Write([]byte(text))
	return strconv.FormatUint(digest.Sum64(), 16)
}

// Catalog is one language's messages.
type Catalog struct {
	// Tag is the BCP 47 tag the catalog answers to.
	Tag string `json:"tag"`
	// Entries are the messages, keyed by the key the code names them by.
	Entries map[string]Entry `json:"entries"`
}

// Renderer renders messages in one language, falling back to English for a
// key the language does not carry.
type Renderer struct {
	// Tag is the language the renderer was asked for.
	Tag     string
	primary *Catalog
	base    *Catalog
}

// loaded holds the catalogs read out of the embedded files, read once.
var loaded = readAll()

// readAll reads every embedded catalog. A catalog that will not parse is
// dropped rather than failing the binary's start, because the reader can
// still be served in English.
func readAll() map[string]*Catalog {
	catalogs := map[string]*Catalog{}
	entries, err := locales.ReadDir("locales")
	if err != nil {
		return catalogs
	}
	for _, entry := range entries {
		data, err := locales.ReadFile(path.Join("locales", entry.Name()))
		if err != nil {
			continue
		}
		catalog := &Catalog{}
		if err := json.Unmarshal(data, catalog); err != nil {
			continue
		}
		catalogs[catalog.Tag] = catalog
	}
	return catalogs
}

// Complete lists the catalogs the language ruling declares fully translated:
// the base language plus every locale a human has finished. Skeleton lists
// the ruling's remaining declared locales, each shipped as a generated
// skeleton until somebody translates it. The two lists are the single
// declaration of that roster; a test in this package and a test in
// internal/verb both read them rather than each carrying its own copy of the
// same fact, which is what let German ship translated while two separate
// hardcoded rosters still called it a skeleton.
//
// Hindi and German left this list at dinah-287, and the card's own D-6 records
// why. That rename moved the English text of ninety-five entries, no fluent
// editor of either language was available in the pass that made the move, and
// the two bad answers were both ruled out by name: leaving the retired word
// standing in the one place an English-speaking maintainer would never see it,
// or deleting the keys so the reader silently gets English. The entries carry
// the renamed English with the skeleton flag the package already has for
// saying a string is not translated yet, and the follow-up card that
// retranslates them puts both tags back here.
var Complete = []string{Base}

// Skeleton is documented on Complete, which it is the other half of.
var Skeleton = []string{"cs", "id", "es", "fil", "af"}

// Tags lists every shipped locale tag, sorted, with the base language first
// so a coverage report reads from the complete catalog outward.
func Tags() []string {
	var tags []string
	for tag := range loaded {
		if tag == Base {
			continue
		}
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return append([]string{Base}, tags...)
}

// Keys lists every key the base catalog carries, sorted. It is what a
// coverage report counts against and what the no-literals check enumerates.
func Keys() []string {
	catalog, ok := loaded[Base]
	if !ok {
		return nil
	}
	var keys []string
	for key := range catalog.Entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// BaseEntry returns one entry of the base catalog.
func BaseEntry(key string) (Entry, bool) {
	catalog, ok := loaded[Base]
	if !ok {
		return Entry{}, false
	}
	entry, ok := catalog.Entries[key]
	return entry, ok
}

// Coverage reports how many of the base catalog's keys a language carries
// translated, and how many it carries at all. A generated skeleton carries
// every key and translates none of them.
func Coverage(tag string) (translated, present, total int) {
	base, ok := loaded[Base]
	if !ok {
		return 0, 0, 0
	}
	total = len(base.Entries)
	catalog, ok := loaded[tag]
	if !ok {
		return 0, 0, total
	}
	for key := range base.Entries {
		entry, carried := catalog.Entries[key]
		if !carried {
			continue
		}
		present++
		if !entry.Skeleton {
			translated++
		}
	}
	return translated, present, total
}

// For returns a renderer for a language tag, walking the tag from most
// specific to least: a regional catalog, then its base language, then
// English. A tag no catalog answers to renders in English.
func For(tag string) *Renderer {
	renderer := &Renderer{Tag: tag, base: loaded[Base]}
	if catalog, ok := loaded[tag]; ok {
		renderer.primary = catalog
		return renderer
	}
	if language, _, ok := strings.Cut(tag, "-"); ok {
		if catalog, ok := loaded[language]; ok {
			renderer.primary = catalog
			return renderer
		}
	}
	return renderer
}

// T renders a message. The pairs are alternating placeholder names and
// values, so a translator may reorder a sentence around its parameters
// without the call site knowing.
//
// A key no catalog carries renders as the key itself in braces, which is
// visible in a test and harmless in front of a reader, rather than as an
// empty string that would silently swallow the message.
func (r *Renderer) T(key string, pairs ...string) string {
	text, ok := r.lookup(key)
	if !ok {
		return "{" + key + "}"
	}
	return fill(text, pairs)
}

// Has reports whether any catalog the renderer reads carries a key.
func (r *Renderer) Has(key string) bool {
	_, ok := r.lookup(key)
	return ok
}

// TN renders a message chosen by plural category. The categories are CLDR's,
// with a key per category, and a language carrying only `other` gets it for
// every count.
func (r *Renderer) TN(key string, count int, pairs ...string) string {
	category := "other"
	if count == 1 {
		category = "one"
	}
	if _, ok := r.lookup(key + "." + category); !ok {
		category = "other"
	}
	return r.T(key+"."+category, append(pairs, "count", strconv.Itoa(count))...)
}

// lookup finds a key in the language asked for and then in English, which is
// the per-key fallback the language ruling requires.
func (r *Renderer) lookup(key string) (string, bool) {
	if r.primary != nil {
		if entry, ok := r.primary.Entries[key]; ok && entry.Text != "" {
			return entry.Text, true
		}
	}
	if r.base == nil {
		return "", false
	}
	entry, ok := r.base.Entries[key]
	return entry.Text, ok
}

// fill substitutes named placeholders. A placeholder with no value given is
// left as it stands, so a missing parameter shows up rather than vanishing.
func fill(text string, pairs []string) string {
	for i := 0; i+1 < len(pairs); i += 2 {
		text = strings.ReplaceAll(text, "{"+pairs[i]+"}", pairs[i+1])
	}
	return text
}
