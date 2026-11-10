package docservice

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// store keeps what the service remembers between runs: a recovery record
// for each document, and the applications' settings. Only the service
// writes here, and there is one service, so the store's own lock is all the
// arbitration it needs. Each file is written whole to a temporary name and
// renamed into place, so a record is the old one or the new one.
type store struct {
	dir string
	mu  sync.Mutex
}

// Record is what is remembered of a document: which it is, and its applet's
// recovery snapshot, which this program keeps and does not read. The
// snapshot has the text the person has not saved, and the revision of the
// file that text was based on, so that it is offered again as a draft of
// that revision and never as the file's content
// (yavc.app content-recovery-contract.md).
type Record struct {
	Key         string          `json:"key"`
	Application string          `json:"application"`
	Path        string          `json:"path"`
	Untitled    bool            `json:"untitled"`
	Saved       time.Time       `json:"saved"`
	Snapshot    json.RawMessage `json:"snapshot"`
}

// maxSnapshot bounds a recovery snapshot. The applets bound their own at
// 8 MiB of text, which is at most three times that as JSON.
const maxSnapshot = 32 << 20

// maxConfig bounds one application's settings.
const maxConfig = 1 << 20

func newStore(dir string) (*store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "recovery"), 0o700); err != nil {
		return nil, err
	}
	return &store{dir: dir}, nil
}

// fileKey is the recovery key of a file in an application. It is the path
// as the service keeps it: a file opened later by another path to it is not
// found here, and its record stays under the path it was written with.
func fileKey(application, path string) string { return "file:" + application + ":" + path }

// untitledKey is the recovery key of a document with no file, which is its
// window's own.
func untitledKey(application, id string) string { return "untitled:" + application + ":" + id }

func (st *store) file(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(st.dir, "recovery", hex.EncodeToString(sum[:])+".json")
}

func writeWhole(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}

// get is the record kept under key, or nil.
func (st *store) get(key string) *Record {
	if key == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.read(st.file(key), key)
}

func (st *store) read(file, key string) *Record {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var r Record
	if json.Unmarshal(data, &r) != nil || (key != "" && r.Key != key) || len(r.Snapshot) == 0 {
		return nil
	}
	return &r
}

// put keeps a record under its key.
func (st *store) put(r Record) error {
	if r.Key == "" || !json.Valid(r.Snapshot) {
		return errors.New("the recovery record is not one the service can keep")
	}
	if len(r.Snapshot) > maxSnapshot {
		return errors.New("the recovery record is too large to keep")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return writeWhole(st.file(r.Key), data)
}

func (st *store) remove(key string) {
	if key == "" {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	os.Remove(st.file(key))
}

// orphan is the newest record of an application's document with no file
// that no open window has: the text of a new document whose window ended
// before it was saved anywhere.
func (st *store) orphan(application string, open map[string]bool) *Record {
	st.mu.Lock()
	defer st.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(st.dir, "recovery"))
	if err != nil {
		return nil
	}
	var found []*Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r := st.read(filepath.Join(st.dir, "recovery", e.Name()), "")
		if r != nil && r.Untitled && r.Application == application && !open[r.Key] {
			found = append(found, r)
		}
	}
	if len(found) == 0 {
		return nil
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Saved.After(found[j].Saved) })
	return found[0]
}

func (st *store) configFile() string { return filepath.Join(st.dir, "config.json") }

// config is the settings kept for an application, as the object its applet
// saved.
func (st *store) config(application string) map[string]json.RawMessage {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.allConfig()[application]
}

func (st *store) allConfig() map[string]map[string]json.RawMessage {
	all := map[string]map[string]json.RawMessage{}
	if data, err := os.ReadFile(st.configFile()); err == nil {
		if json.Unmarshal(data, &all) != nil {
			all = map[string]map[string]json.RawMessage{}
		}
	}
	return all
}

// setConfig adds values to an application's settings and returns them all.
func (st *store) setConfig(application string, values map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	all := st.allConfig()
	merged := all[application]
	if merged == nil {
		merged = map[string]json.RawMessage{}
	}
	for k, v := range values {
		merged[k] = v
	}
	if data, _ := json.Marshal(merged); len(data) > maxConfig {
		return nil, errors.New("the settings are too large to keep")
	}
	all[application] = merged
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return nil, err
	}
	return merged, writeWhole(st.configFile(), data)
}
