// Package connections is the registry of the services a build connects to
// with an address and a token, such as a wiki or an issue tracker, for
// Settings to show them in one Connections section. A build registers each
// connection when it starts, in the way a database/sql driver registers
// itself, and owns where its settings are kept: this package holds no
// secret and no address, only the registered connections and the calls to
// each one's own store. Dinah registers none (docs/design/source-kinds.md).
package connections

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Settings are a connection's settings as the person enters them.
type Settings struct {
	// Address is the service's base address.
	Address string
	// Token is the person's token for the service, or "" for none.
	Token string
}

// Connection is one registered service.
type Connection struct {
	// Label is the service's name as people see it, such as "Confluence".
	Label string
	// Help is shown under the label: what the connection is for and where
	// the token comes from.
	Help string
	// DefaultAddress is the address offered when none is saved.
	DefaultAddress string
	// Load returns the saved settings; none saved is empty settings, not an
	// error. The token is never shown, only whether one is saved.
	Load func() (Settings, error)
	// Save keeps the settings. An empty token keeps the saved one.
	Save func(Settings) error
	// Remove forgets the saved settings.
	Remove func() error
	// Test reaches the service with the saved settings and returns a line
	// about who it signed in as, or an error. Nil, there is no Test button.
	Test func(context.Context) (string, error)
}

// Registered is a registered connection with its ID.
type Registered struct {
	ID string
	Connection
}

var (
	mu          sync.RWMutex
	connections = map[string]Connection{}
)

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// Register registers a connection. A second registration of an ID, an ID
// that is not a plain lower-case word, or a connection without Load and
// Save panics, as registering a database/sql driver twice does.
func Register(id string, c Connection) {
	mu.Lock()
	defer mu.Unlock()
	if !idPattern.MatchString(id) || c.Load == nil || c.Save == nil || strings.TrimSpace(c.Label) == "" {
		panic("connections: cannot register the connection " + id)
	}
	if _, ok := connections[id]; ok {
		panic("connections: the connection " + id + " is registered twice")
	}
	connections[id] = c
}

// Lookup returns a registered connection.
func Lookup(id string) (Connection, bool) {
	mu.RLock()
	defer mu.RUnlock()
	c, ok := connections[id]
	return c, ok
}

// All returns the registered connections in the order of their labels.
func All() []Registered {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Registered, 0, len(connections))
	for id, c := range connections {
		out = append(out, Registered{ID: id, Connection: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// ErrUnknown is the answer for a connection nobody registered.
var ErrUnknown = errors.New("that connection is not one this build has")
