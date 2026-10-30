package sessionhost

import (
	_ "embed"
	"encoding/json"
	"sync"

	"studio/internal/acp"
)

//go:embed details.go
var DetailsSource []byte

// Details owns live update state. Every Locked method requires the injected
// conversation mutex, preserving transactions with admission and turn state.
// No method calls the host. NewDetails must run before session publication.
type Details struct {
	mu             *sync.Mutex
	background     map[string]*BackgroundTask
	backgroundSeq  uint64
	subagents      map[string]*subagent
	subagentSerial int64
	options        []acp.ConfigOption
	substituted    map[string]OptionOverride
	// Replay updates do not enter the live journal. Restoring also blocks
	// admission while provider options are being restored.
	restoringOptions, replaying bool
	// lastUpdate identifies consecutive agent chunks for journal entries.
	lastUpdate string
	// replayed batches page updates; replayHeld waits for a user-message
	// boundary so saved bus deliveries can replace the replayed prompt.
	replayed        []NumberedUpdate
	replayHeld      []json.RawMessage
	replayHolding   bool
	replayHeldStart int
}

func NewDetails(mu *sync.Mutex) *Details { return &Details{mu: mu} }
