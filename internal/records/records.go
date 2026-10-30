// Package records keeps small JSON records, such as flow runs and applet
// invocations, durably in a folder. Each record is kept in two files that
// take turns: a save writes a complete, checksummed generation over the older
// one and syncs it before returning, so an interrupted save leaves the other
// generation readable, and a damaged or unknown generation is refused rather
// than silently replaced. A record is a JSON object whose "id" field holds the
// record's ID.
//
// The file layout is part of what Dinah keeps on disk: <id>.run0.json
// and <id>.run1.json, with <id>.json read as a record written before
// generations existed, and <id>.hold, an empty file that the process acting
// on a record locks (see Hold).
package records

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"studio/internal/keylock"
)

// Store is the records in one folder.
type Store struct {
	Dir string
}

// generation is one saved state of a record.
type generation struct {
	Version  int             `json:"version"`
	Sequence uint64          `json:"sequence"`
	Payload  json.RawMessage `json:"payload"`
	Checksum string          `json:"checksum"`
}

type state struct {
	latest *generation
	slot   int
	exists bool
}

// locks serialises the reads and writes of each record, so that two saves in
// this process never pick the same generation, while records do not wait on
// each other's disk work. Another process is kept out by Hold.
var locks keylock.Set

// lock locks one record. Folder spellings that differ only in case share a
// lock, since Windows compares paths without case; elsewhere that only makes
// two such records take turns.
func (s Store) lock(id string) (unlock func()) {
	dir, err := filepath.Abs(s.Dir)
	if err != nil {
		dir = filepath.Clean(s.Dir)
	}
	return locks.Lock(strings.ToLower(filepath.Join(dir, id)))
}

const version = 1

// ErrVersion means a record was saved in a newer format than this program
// knows, so it is neither read nor overwritten.
var ErrVersion = errors.New("unsupported saved record format version")

// SlotPath is the file of one of a record's two generations.
func (s Store) SlotPath(id string, slot int) string {
	return filepath.Join(s.Dir, fmt.Sprintf("%s.run%d.json", id, slot))
}

func (s Store) legacyPath(id string) string { return filepath.Join(s.Dir, id+".json") }

func checksum(sequence uint64, payload []byte) string {
	h := sha256.New()
	_, _ = io.WriteString(h, strconv.Itoa(version)+"\x00"+strconv.FormatUint(sequence, 10)+"\x00")
	_, _ = h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// recordID is the "id" field of a record's payload.
func recordID(payload []byte) string {
	var head struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(payload, &head)
	return head.ID
}

func decode(data []byte) (*generation, error) {
	var g generation
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}
	if g.Version != version {
		return nil, ErrVersion
	}
	if g.Sequence == 0 {
		return nil, errors.New("invalid record generation sequence")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, g.Payload); err != nil {
		return nil, err
	}
	g.Payload = compact.Bytes()
	if checksum(g.Sequence, g.Payload) != g.Checksum {
		return nil, errors.New("record generation checksum does not match")
	}
	if recordID(g.Payload) == "" {
		return nil, errors.New("invalid record generation payload")
	}
	return &g, nil
}

// read finds the newest complete generation of a record; the caller holds the record's lock.
func (s Store) read(id string) (state, error) {
	st := state{slot: -1}
	for slot := range 2 {
		data, err := os.ReadFile(s.SlotPath(id, slot))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		st.exists = true
		if err != nil {
			return st, err
		}
		g, err := decode(data)
		if err != nil {
			if errors.Is(err, ErrVersion) {
				return st, err
			}
			continue
		}
		if recordID(g.Payload) != id {
			continue
		}
		if st.latest != nil && g.Sequence == st.latest.Sequence && g.Checksum != st.latest.Checksum {
			return st, errors.New("conflicting record generations")
		}
		if st.latest == nil || g.Sequence > st.latest.Sequence {
			st.latest, st.slot = g, slot
		}
	}
	if st.exists && st.latest == nil {
		return st, errors.New("neither saved record generation is complete")
	}
	return st, nil
}

// Read returns a record's newest complete payload.
func (s Store) Read(id string) ([]byte, error) {
	defer s.lock(id)()
	st, err := s.read(id)
	if err != nil {
		return nil, err
	}
	if st.latest != nil {
		return st.latest.Payload, nil
	}
	return os.ReadFile(s.legacyPath(id))
}

// Write saves a record's payload as its next generation, over the older
// one, and syncs it. It refuses when the record's saved generations are
// damaged or of an unknown version, so nothing is replaced unseen.
func (s Store) Write(id string, payload []byte) error {
	defer s.lock(id)()
	st, err := s.read(id)
	if err != nil {
		return err
	}
	sequence, slot := uint64(1), 0
	if st.latest != nil {
		if st.latest.Sequence == math.MaxUint64 {
			return errors.New("record generation sequence exhausted")
		}
		sequence, slot = st.latest.Sequence+1, 1-st.slot
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, payload); err != nil {
		return err
	}
	payload = compact.Bytes()
	data, err := json.Marshal(generation{Version: version, Sequence: sequence, Payload: payload, Checksum: checksum(sequence, payload)})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.SlotPath(id, slot), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	return errors.Join(writeErr, f.Close())
}

// Remove deletes a record's saved generations, and the file of a record
// written before generations existed. The newest generation goes last, so
// that a removal that stops part way never leaves an older state of the
// record to be read as its current one. Like Write, it refuses a record
// whose generations are damaged or of an unknown version. Its hold file is
// left, since only the caller knows whether anyone may still hold it. A
// record that is not saved is no error.
func (s Store) Remove(id string) error {
	defer s.lock(id)()
	st, err := s.read(id)
	if err != nil {
		return err
	}
	paths := []string{s.legacyPath(id)}
	if st.latest != nil {
		paths = append(paths, s.SlotPath(id, 1-st.slot), s.SlotPath(id, st.slot))
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Exists reports whether anything is saved for a record, readable or not, so
// a damaged record is never mistaken for an absent one.
func (s Store) Exists(id string) bool {
	for _, path := range []string{s.SlotPath(id, 0), s.SlotPath(id, 1), s.legacyPath(id)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			return true
		}
	}
	return false
}

// ID is the record a file of a store belongs to, or "" when the file is not
// a record's.
func ID(fileName string) string {
	for _, suffix := range []string{".run0.json", ".run1.json", ".json"} {
		if id, ok := strings.CutSuffix(fileName, suffix); ok {
			return id
		}
	}
	return ""
}

// Payload returns the payload of a generation file's contents, checked, for
// tests and repair tools that read a slot directly.
func Payload(data []byte) ([]byte, error) {
	g, err := decode(data)
	if err != nil {
		return nil, err
	}
	return g.Payload, nil
}

// Generation returns a generation file's contents for a payload at a
// sequence, for tests that set up a record's slots by hand.
func Generation(sequence uint64, payload []byte) []byte {
	data, _ := json.Marshal(generation{Version: version, Sequence: sequence, Payload: payload, Checksum: checksum(sequence, payload)})
	return data
}
