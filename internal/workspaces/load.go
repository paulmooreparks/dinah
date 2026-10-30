package workspaces

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"studio/internal/statefile"
)

// RegistryFile is the file in the home folder that holds the registry. The
// window is its only writer, through Save. The command line and anything
// else that needs the workspaces only reads it, through Load.
const RegistryFile = "workspaces.json"

// ErrNoRegistry is what Load returns when the registry file does not exist.
// It means the window has not moved the registry out of its settings yet,
// which is different from a registry with no workspaces in it.
var ErrNoRegistry = errors.New("the workspace registry has not been created yet")

type registryFile struct {
	Workspaces []Record `json:"workspaces"`
}

// Load reads the registry from the home folder home. It reads through
// statefile.ReadShared, as the package documentation prescribes for a file
// two programs touch, so a save the window left pending is finished first.
// A missing file gives ErrNoRegistry.
func Load(home string) ([]Record, error) {
	data, err := statefile.ReadShared(filepath.Join(home, RegistryFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoRegistry
	}
	if err != nil {
		return nil, err
	}
	var file registryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("the workspace registry could not be read: %w", err)
	}
	if file.Workspaces == nil {
		return []Record{}, nil
	}
	return file.Workspaces, nil
}

// Save replaces the registry in the home folder home with records. Only the
// window calls it.
func Save(home string, records []Record) error {
	data, err := encodeRegistry(records)
	if err != nil {
		return err
	}
	return statefile.Update(filepath.Join(home, RegistryFile), func([]byte) ([]byte, error) { return data, nil })
}

func encodeRegistry(records []Record) ([]byte, error) {
	if records == nil {
		records = []Record{}
	}
	return json.MarshalIndent(registryFile{Workspaces: records}, "", "  ")
}

// Same reports whether two registries hold the same records in the same
// order, by the form they are saved in.
func Same(a, b []Record) bool {
	x, err := encodeRegistry(a)
	if err != nil {
		return false
	}
	y, err := encodeRegistry(b)
	return err == nil && bytes.Equal(x, y)
}

// Grown returns next followed by the records of old that next no longer
// has. Saving it before the settings that refer to next, and next itself
// after them, keeps every reference in the settings pointing at a record
// that is in the registry at each point (see the window's saveSettings).
func Grown(old, next []Record) []Record {
	have := make(map[string]bool, len(next))
	for _, r := range next {
		have[r.ID] = true
	}
	grown := Copy(next)
	for _, r := range old {
		if !have[r.ID] {
			grown = append(grown, Copy([]Record{r})[0])
		}
	}
	return grown
}
