package workspaces

import (
	"context"
	"regexp"
	"sync"
)

// A build can know kinds of source besides a local folder, and kinds of pin
// besides a chat and a resource, and say how a workspace export treats
// them, by registering them here when it starts, in the way a database/sql
// driver registers itself. A kind nobody registered is still kept: an
// import passes its sources and pins through unchanged, with the fields
// this build does not know (extra.go), and the import plan shows them as
// kept but not checked (docs/design/workspace-export.md).

// SourceKind is what a build knows about one kind of source. Every field
// may be nil, and a nil field does what is done for a kind nobody
// registered.
type SourceKind struct {
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

// LocalFolder is the kind of a source that is a folder on the computer, the
// one kind every build knows.
const LocalFolder = "local-folder"
