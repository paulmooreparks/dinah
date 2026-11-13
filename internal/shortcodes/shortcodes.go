// Package shortcodes is the registry of the shortcodes a build adds to the
// Markdown dialect the content applets edit, beyond the engine's own
// (docs/design/shortcodes.md). A product whose documents are pages on a
// site registers a set: what each shortcode is, for completion and the
// Insert menu, how an occurrence of one is drawn, and, where the set has
// them, the form an occurrence is changed through. The page hands all three
// to the applets through the host's shortcode services. Nothing here knows
// a shortcode; Dinah's own build registers none.
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

// Descriptor describes one shortcode as the applets present it. The title
// is what a menu shows; the name is used when a set gives none.
//
// Prefix is one punctuation character the shortcode may be written with in
// place of its name and colon, as {{@value}} is for a shortcode whose
// prefix is "@". Managed says the product keeps the shortcode with a tool
// of its own: the applets draw it, and neither offer to insert it nor
// change it on the page. Literal says the body of a container is markup
// and not Markdown: the applets read the body whole, blank lines included,
// and edit a body that is one table as a table. ColorAttribute is for a
// literal container whose product keeps a table cell's colour in a data
// attribute beside the cell's style, and is that attribute's name: the
// applets write, change and remove the attribute with the colour.
// ColorClassPrefix is for a literal container whose product also marks a
// coloured cell with a class, and is the beginning of those class names:
// when a cell's colour is changed or removed, the applets remove the
// cell's classes that begin with it, and add none.
//
// CellColor says an inline shortcode is the one a product keeps a table
// cell's colour with in a Markdown table. The applets' Cell Color writes
// it first in each chosen cell, through the set's form and build, so the
// table stays a Markdown table. The catalog keeps it on one inline
// shortcode, the first, of a set that has forms.
type Descriptor struct {
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	Kind        string  `json:"kind"`
	Description string  `json:"description,omitempty"`
	Params      []Param `json:"params,omitempty"`
	Example     string  `json:"example,omitempty"`
	Prefix      string  `json:"prefix,omitempty"`
	Managed     bool    `json:"managed,omitempty"`
	Literal     bool    `json:"literal,omitempty"`
	// ColorAttribute is a data attribute's name, as "data-cell-colour".
	ColorAttribute string `json:"colorAttribute,omitempty"`
	// ColorClassPrefix is the beginning of a class name, as "cell-colour-".
	ColorClassPrefix string `json:"colorClassPrefix,omitempty"`
	// CellColor marks the inline shortcode for a cell's colour.
	CellColor bool `json:"cellColor,omitempty"`
}

// Request is one occurrence of a shortcode to draw: its name, the raw
// argument text after the colon, the whole shortcode as written, and, for
// a container, the Markdown of its body, which the applet draws itself in
// the place of the <slot name="content"></slot> element the HTML holds.
// For a container or a span the source may be the opening alone.
//
// Kind is how the applet read the occurrence, when it says: a container
// opened and closed within one line is read as a span, and its drawing
// goes inside that line.
type Request struct {
	Name    string `json:"name"`
	Args    string `json:"args"`
	Source  string `json:"source"`
	Content string `json:"content,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

// Set is a build's shortcodes: their catalog, and how one is drawn. Render
// returns ErrUnknown for a name the set lacks, so another set may be asked.
type Set interface {
	Catalog() []Descriptor
	Render(r Request) (html string, err error)
}

// Field is one value of a shortcode's form: its name among the form's
// values, the label and help shown with it, the value it has now, and the
// choices when the value is one of a few.
type Field struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Value       string   `json:"value"`
	Values      []string `json:"values,omitempty"`
}

// Form is the form one occurrence of a shortcode is changed through: the
// kind Build is asked for, a title and a line of help, and the fields,
// filled from the occurrence as written.
type Form struct {
	Kind        string  `json:"kind"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Fields      []Field `json:"fields"`
}

// FieldError is a value a form cannot write and the field it is in. The
// field is empty when the fault is not one field's.
type FieldError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func (e *FieldError) Error() string { return e.Message }

// Former is a set whose shortcodes are changed through forms, so that the
// argument grammar stays the set's. Form reads an occurrence as its form,
// and reports false when it has none, in which case the occurrence is
// changed as text. Build writes the shortcode from a form's values; it
// returns a *FieldError for a value the shortcode cannot hold and
// ErrUnknown for a kind the set lacks.
type Former interface {
	Form(source string) (Form, bool)
	Build(kind string, values map[string]string) (source string, err error)
}

// ErrUnknown is a set's answer for a shortcode it does not know.
var ErrUnknown = errors.New("unknown shortcode")

var (
	mu   sync.RWMutex
	sets []Set
)

var namePattern = regexp.MustCompile(`^(@?[a-z][a-z0-9-]*)$`)

// colorAttribute is the name of a data attribute a set may keep a cell's
// colour in.
var colorAttribute = regexp.MustCompile(`^data-[a-z0-9][a-z0-9-]{0,58}$`)

// colorClassPrefix is the beginning of the class names a set may mark a
// coloured cell with, in the form the applets accept.
var colorClassPrefix = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

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
	cellColor := false
	for _, s := range sets {
		_, former := s.(Former)
		for _, d := range s.Catalog() {
			if d.Name == "" || seen[d.Name] {
				continue
			}
			seen[d.Name] = true
			if d.Title == "" {
				d.Title = d.Name
			}
			if d.Params == nil {
				d.Params = []Param{}
			}
			// Only a container has a body to read as markup.
			if d.Kind != Container {
				d.Literal = false
			}
			// Only a literal body has cells whose colour an attribute can
			// keep, and only a data attribute may be named for it.
			if !d.Literal || !colorAttribute.MatchString(d.ColorAttribute) {
				d.ColorAttribute = ""
			}
			if !d.Literal || !colorClassPrefix.MatchString(d.ColorClassPrefix) {
				d.ColorClassPrefix = ""
			}
			// The applets write a cell's colour through the shortcode's form,
			// and expect one shortcode for it.
			if d.CellColor && (d.Kind != Inline || !former || cellColor) {
				d.CellColor = false
			}
			cellColor = cellColor || d.CellColor
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
	html, _, err := render(r)
	return html, err
}

// render is Render, with the set that drew the occurrence.
func render(r Request) (string, Set, error) {
	r.Name = strings.TrimSpace(r.Name)
	if !namePattern.MatchString(r.Name) {
		return "", nil, errors.New("invalid shortcode name")
	}
	if len(r.Source) > 1<<16 || len(r.Content) > 1<<20 {
		return "", nil, errors.New("the shortcode is too large to draw")
	}
	switch r.Kind {
	case "", Inline, Block, Container, Span:
	default:
		return "", nil, errors.New("invalid shortcode kind")
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, s := range sets {
		html, err := s.Render(r)
		if errors.Is(err, ErrUnknown) {
			continue
		}
		return html, s, err
	}
	return "", nil, ErrUnknown
}

// FormOf reads one occurrence, as written, as its form, through the first
// set that has one for it.
func FormOf(source string) (Form, bool) {
	if len(source) > 1<<16 {
		return Form{}, false
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, s := range sets {
		f, ok := s.(Former)
		if !ok {
			continue
		}
		if form, ok := f.Form(source); ok {
			if form.Fields == nil {
				form.Fields = []Field{}
			}
			return form, true
		}
	}
	return Form{}, false
}

// Build writes a shortcode from a form's values through the first set that
// knows the kind.
func Build(kind string, values map[string]string) (string, error) {
	kind = strings.TrimSpace(kind)
	if !namePattern.MatchString(kind) {
		return "", errors.New("invalid shortcode name")
	}
	size := 0
	for name, value := range values {
		size += len(name) + len(value)
	}
	if size > 1<<16 {
		return "", errors.New("the values are too large for a shortcode")
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, s := range sets {
		f, ok := s.(Former)
		if !ok {
			continue
		}
		source, err := f.Build(kind, values)
		if errors.Is(err, ErrUnknown) {
			continue
		}
		return source, err
	}
	return "", ErrUnknown
}
