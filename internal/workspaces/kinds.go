package workspaces

import (
	"context"
	"regexp"
	"sort"
	"sync"
)

// A build can know kinds of source besides a local folder, and kinds of pin
// besides a chat and a resource, and say how a workspace export treats
// them, by registering them here when it starts, in the way a database/sql
// driver registers itself. A kind nobody registered is still kept: an
// import passes its sources and pins through unchanged, with the fields
// this build does not know (extra.go), and the import plan shows them as
// kept but not checked (docs/design/workspace-export.md).
//
// A registered kind can also bring what makes it usable: how the person
// adds a source of it (Attach) and the resource provider of a source of it
// (Provider). The desktop lists such kinds in its Add Source dialog, and
// the Files applet, the Editor and the applet actions reach a source of the
// kind through the provider's capabilities (internal/resource), never
// through the kind itself (docs/design/source-kinds.md).

// SourceKind is what a build knows about one kind of source. Every field
// may be nil, and a nil field does what is done for a kind nobody
// registered.
type SourceKind struct {
	// Label is the kind's name as people see it, such as "Wiki Page".
	// Empty, the kind is not offered in the Add Source dialog.
	Label string
	// Attach says how a source of this kind is added, when the person can
	// add one. Nil, the kind can only arrive by import.
	Attach *Attach
	// Provider builds the resource provider of a source of this kind:
	// a value offering the capabilities of internal/resource the source
	// supports. Nil, a source of the kind has no provider, and what asks
	// for one is told resource.ErrUnsupported.
	Provider func(r Resolver, ctx context.Context, s Source) (any, error)
	// DocumentKey is the identity of a document of a source of this kind
	// whose comment threads are kept (internal/threads), when the source
	// gives its documents one of their own, such as a page's ID on its
	// site, which stays when the page is renamed or moved. Nil, a document
	// is known by its kind, workspace, source and path.
	DocumentKey func(s Source, path string) string
	// Read checks a source of this kind as an export file has it, and
	// returns it cleaned, or an error that says what is wrong with it. Nil
	// accepts any source that is not marked as the main folder.
	Read func(ExportSource) (ExportSource, error)
	// Describe returns where a source of this kind is, for the import plan:
	// the site it is on and what identifies it there.
	Describe func(ExportSource) (site, reference string)
	// Check says why a source of this kind cannot be used on this computer
	// yet, such as a missing connection to its site, or returns "" when it
	// can. Nil leaves the source unchecked.
	Check func(context.Context, ExportSource) string
	// Export makes a source's entry in an export, apart from its key, kind
	// and name, which the export sets. Nil copies the fields Extra holds.
	Export func(Source) ExportSource
	// Recreate makes the registry's source from an export's, apart from
	// its ID, kind and name, which the import sets. Nil copies the fields
	// Extra holds.
	Recreate func(ExportSource) Source
	// DefaultName is the name of an imported source of this kind that has
	// none. Nil uses the kind.
	DefaultName func(ExportSource) string
}

// Attach is how the person adds a source of a kind: the form they fill in
// the Add Source dialog, and what makes a source of what they filled in.
type Attach struct {
	// Help is shown above the form: what the source is and what adding it
	// does and does not do.
	Help string
	// Fields are the form's fields, in order.
	Fields []Field
	// Build makes the source from the filled form, keyed by field ID; a
	// check field's value is "true" or "false". It checks the values,
	// reaching the source's site when that is what checking takes, and
	// returns the source with its Kind, Name and Extra set. The caller
	// gives the source its ID.
	Build func(ctx context.Context, values map[string]string) (Source, error)
}

// Field is one field of an Attach form.
type Field struct {
	// ID is the field's key in the values Build is given.
	ID string `json:"id"`
	// Label is the field's label.
	Label string `json:"label"`
	// Kind is "text" or "check".
	Kind string `json:"kind"`
	// Placeholder is a text field's example value, and Help a line shown
	// under the field.
	Placeholder string `json:"placeholder,omitempty"`
	Help        string `json:"help,omitempty"`
	// Required marks a text field that must be filled.
	Required bool `json:"required,omitempty"`
	// Default is the field's value when the dialog opens: a text field's
	// text, or "true" for a check field that starts checked.
	Default string `json:"default,omitempty"`
	// MaxLength bounds a text field; 0 leaves the dialog's own bound.
	MaxLength int `json:"maxLength,omitempty"`
}

// PinKind is what a build knows about one kind of pin.
type PinKind struct {
	// Read says whether a pin of this kind in an export can be kept. Nil
	// keeps any pin of the kind whose source, when it has one, is in the
	// export.
	Read func(ExportPin) bool
}

var (
	kindsMu     sync.RWMutex
	sourceKinds = map[string]SourceKind{}
	pinKinds    = map[string]PinKind{}
)

// kindPattern is what the kind of a source or pin may look like.
var kindPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// ValidKind reports whether a kind of source or pin has a plain form: a
// lower-case letter, then up to 63 lower-case letters, digits and hyphens.
func ValidKind(kind string) bool { return kindPattern.MatchString(kind) }

// RegisterSourceKind registers what this build knows about a kind of
// source. Registering a kind twice, or registering a local folder, which
// is built in, panics, as registering a database/sql driver twice does.
func RegisterSourceKind(kind string, k SourceKind) {
	kindsMu.Lock()
	defer kindsMu.Unlock()
	if !ValidKind(kind) || kind == LocalFolder {
		panic("workspaces: cannot register the source kind " + kind)
	}
	if _, ok := sourceKinds[kind]; ok {
		panic("workspaces: the source kind " + kind + " is registered twice")
	}
	sourceKinds[kind] = k
}

// RegisterPinKind registers what this build knows about a kind of pin.
func RegisterPinKind(kind string, k PinKind) {
	kindsMu.Lock()
	defer kindsMu.Unlock()
	if !ValidKind(kind) || kind == "chat" || kind == "resource" {
		panic("workspaces: cannot register the pin kind " + kind)
	}
	if _, ok := pinKinds[kind]; ok {
		panic("workspaces: the pin kind " + kind + " is registered twice")
	}
	pinKinds[kind] = k
}

// LookupSourceKind returns what this build knows about a kind of source,
// and whether the kind was registered.
func LookupSourceKind(kind string) (SourceKind, bool) {
	kindsMu.RLock()
	defer kindsMu.RUnlock()
	k, ok := sourceKinds[kind]
	return k, ok
}

// LookupPinKind returns what this build knows about a kind of pin, and
// whether the kind was registered.
func LookupPinKind(kind string) (PinKind, bool) {
	kindsMu.RLock()
	defer kindsMu.RUnlock()
	k, ok := pinKinds[kind]
	return k, ok
}

// RegisteredSourceKind is a registered kind with its name.
type RegisteredSourceKind struct {
	Kind string
	SourceKind
}

// AttachableSourceKinds returns the registered kinds the person can add a
// source of, those with a Label and an Attach, in the order of their labels.
func AttachableSourceKinds() []RegisteredSourceKind {
	kindsMu.RLock()
	defer kindsMu.RUnlock()
	out := []RegisteredSourceKind{}
	for kind, k := range sourceKinds {
		if k.Label != "" && k.Attach != nil {
			out = append(out, RegisteredSourceKind{Kind: kind, SourceKind: k})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// HasProvider reports whether a source of the kind has a resource provider
// in this build: a local folder always, another kind when it is registered
// with one.
func HasProvider(kind string) bool {
	if kind == LocalFolder {
		return true
	}
	k, ok := LookupSourceKind(kind)
	return ok && k.Provider != nil
}

// LocalFolder is the kind of a source that is a folder on the computer, the
// one kind every build knows.
const LocalFolder = "local-folder"

// IsLocalSource includes the older desktop spelling while registries migrate.
func IsLocalSource(source Source) bool { return source.Kind == LocalFolder || source.Kind == "local" }

// DocumentKey is the identity of a document whose comment threads are kept:
// the one its source's kind gives it, or its kind, workspace, source and
// path.
func DocumentKey(s Source, path string) string {
	if k, ok := LookupSourceKind(s.Kind); ok && k.DocumentKey != nil {
		if key := k.DocumentKey(s, path); key != "" {
			return key
		}
	}
	return s.Kind + ":" + s.ID + "/" + path
}
