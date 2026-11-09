// Package shortcodes is the registry of the shortcodes a build adds to the
// Markdown dialect the content applets edit, beyond the engine's own
// (docs/design/shortcodes.md). A product whose documents are pages on a
// site registers a set: what each shortcode is, for completion and the
// Insert menu, and how an occurrence of one is drawn in the preview. The
// page hands both to the applets through the host's shortcodes service.
// Nothing here knows a shortcode; Dinah's own build registers none.
package shortcodes

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Kind is how a shortcode is written: in a line of text (inline), as a
// paragraph of its own (block), opening and closing around Markdown
// (container), or opening and closing within a line (span).
const (
	Inline    = "inline"
	Block     = "block"
	Container = "container"
	Span      = "span"
)

// Param describes one argument of a shortcode, for completion and forms.
type Param struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Descriptor describes one shortcode as the applets present it.
type Descriptor struct {
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Description string  `json:"description,omitempty"`
	Params      []Param `json:"params,omitempty"`
	Example     string  `json:"example,omitempty"`
}

// Request is one occurrence of a shortcode to draw: its name, the raw
// argument text after the colon, the whole shortcode as written, and, for
// a container, the Markdown of its body, which the applet draws itself in
// the place of the <slot name="content"></slot> element the HTML holds.
type Request struct {
	Name    string `json:"name"`
	Args    string `json:"args"`
	Source  string `json:"source"`
	Content string `json:"content,omitempty"`
}

// Set is a build's shortcodes: their catalog, and how one is drawn. Render
// returns ErrUnknown for a name the set lacks, so another set may be asked.
type Set interface {
	Catalog() []Descriptor
	Render(r Request) (html string, err error)
}

// ErrUnknown is a set's answer for a shortcode it does not know.
var ErrUnknown = errors.New("unknown shortcode")

var (
	mu   sync.RWMutex
	sets []Set
)

var namePattern = regexp.MustCompile(`^(@?[a-z][a-z0-9-]*)$`)

// Register adds a set; a build registers its sets at start.
func Register(s Set) {
	if s == nil {
		panic("shortcodes: nil set")
	}
	mu.Lock()
	defer mu.Unlock()
	sets = append(sets, s)
}

// Catalog lists every registered shortcode, by name, the first set's
// description winning for a name two sets share.
func Catalog() []Descriptor {
	mu.RLock()
	defer mu.RUnlock()
	seen := map[string]bool{}
	out := []Descriptor{}
	for _, s := range sets {
		for _, d := range s.Catalog() {
			if d.Name == "" || seen[d.Name] {
				continue
			}
			seen[d.Name] = true
			if d.Params == nil {
				d.Params = []Param{}
			}
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Any reports whether the build registered any shortcodes.
func Any() bool {
	mu.RLock()
	defer mu.RUnlock()
	return len(sets) > 0
}

// Render draws one occurrence through the first set that knows its name.
// The request's name must be a shortcode name; the source is bounded.
func Render(r Request) (string, error) {
	r.Name = strings.TrimSpace(r.Name)
	if !namePattern.MatchString(r.Name) {
		return "", errors.New("invalid shortcode name")
	}
	if len(r.Source) > 1<<16 || len(r.Content) > 1<<20 {
		return "", errors.New("the shortcode is too large to draw")
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, s := range sets {
		html, err := s.Render(r)
		if errors.Is(err, ErrUnknown) {
			continue
		}
		return html, err
	}
	return "", ErrUnknown
}
