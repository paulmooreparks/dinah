// Package skins reads PUDL's skin catalogue and the person's skin and theme choices, for every host that draws
// PUDL: the window and the bus's web page (docs/plans/pudl-skins.md). The catalogue is the release's own data
// (dist/pudl-skins.json), so no skin is named in Dinah; the choice is kept by the engine in the appearance store,
// once for the whole application.
package skins

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"studio/internal/statefile"
)

// Standard is the skin every release has and the one shown when the person's choice is not in the build.
const Standard = "standard"

// Skin is one entry of the catalogue.
type Skin struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Stylesheet *string `json:"stylesheet"`
	SHA256     string  `json:"sha256,omitempty"`
	Admitted   bool    `json:"admitted"`
}

// Choice is a skin offered to the person: its ID and its label.
type Choice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Catalogue is a release's skins.
type Catalogue struct {
	Version int    `json:"version"`
	Skins   []Skin `json:"skins"`
}

// idPattern is the shape of a skin's ID; a preference that has it but names a skin the build lacks is kept.
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// ValidID says an ID has the shape of a skin's ID.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Parse reads a catalogue.
func Parse(data []byte) (Catalogue, error) {
	var c Catalogue
	if err := json.Unmarshal(data, &c); err != nil {
		return Catalogue{}, err
	}
	if c.Version != 1 {
		return Catalogue{}, errors.New("unsupported skin catalogue version")
	}
	seen := map[string]bool{}
	for _, s := range c.Skins {
		if !ValidID(s.ID) || seen[s.ID] {
			return Catalogue{}, errors.New("the skin catalogue has an invalid or repeated skin ID")
		}
		seen[s.ID] = true
		if strings.TrimSpace(s.Label) == "" {
			return Catalogue{}, errors.New("the skin catalogue has an empty label")
		}
		if s.ID == Standard {
			if !s.Admitted || s.Stylesheet != nil {
				return Catalogue{}, errors.New("the standard skin must be admitted without a stylesheet")
			}
		} else if s.Admitted {
			if s.Stylesheet == nil || !fs.ValidPath(*s.Stylesheet) || strings.Contains(*s.Stylesheet, "/") || strings.Contains(*s.Stylesheet, "\\") || !strings.HasSuffix(*s.Stylesheet, ".css") {
				return Catalogue{}, errors.New("an admitted skin needs a local CSS filename")
			}
			digest, err := hex.DecodeString(s.SHA256)
			if err != nil || len(digest) != sha256.Size {
				return Catalogue{}, errors.New("an admitted skin needs a SHA-256 stylesheet digest")
			}
		}
	}
	if !seen[Standard] {
		return Catalogue{}, errors.New("the skin catalogue lacks " + Standard)
	}
	return c, nil
}

// ValidateAssets checks admission against the actual bundled CSS, independently
// in each host. Runtime registration alone cannot establish release admission.
func (c Catalogue) ValidateAssets(files fs.FS) error {
	for _, s := range c.Admitted() {
		if s.Stylesheet == nil {
			continue
		}
		data, err := fs.ReadFile(files, *s.Stylesheet)
		if err != nil {
			return fmt.Errorf("skin %s stylesheet: %w", s.ID, err)
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), s.SHA256) {
			return fmt.Errorf("skin %s stylesheet does not match its admitted digest", s.ID)
		}
	}
	return nil
}

// Admitted are the skins the release admits, in its order. Standard is always among them.
func (c Catalogue) Admitted() []Skin {
	var out []Skin
	for _, s := range c.Skins {
		if s.Admitted || s.ID == Standard {
			out = append(out, s)
		}
	}
	return out
}

// Choices are the admitted skins as the person is offered them.
func (c Catalogue) Choices() []Choice {
	var out []Choice
	for _, s := range c.Admitted() {
		out = append(out, Choice{ID: s.ID, Label: s.Label})
	}
	return out
}

// Effective is the skin to draw for a preference: the preference when the build admits it, else Standard. The
// preference itself is never changed by this.
func (c Catalogue) Effective(preference string) string {
	for _, s := range c.Admitted() {
		if s.ID == preference {
			return preference
		}
	}
	return Standard
}

// ReadPreference is the skin the person chose, from the appearance store in dir, or Standard when there is none
// or it cannot be read. It reads only the one field and never fails a page.
func ReadPreference(dir string) string {
	if dir == "" {
		return Standard
	}
	data, err := statefile.ReadShared(filepath.Join(dir, "appearance.json"))
	if err != nil || len(data) == 0 {
		return Standard
	}
	var v struct {
		Skin string `json:"skin"`
	}
	if json.Unmarshal(data, &v) != nil || !ValidID(v.Skin) {
		return Standard
	}
	return v.Skin
}

// ReadThemePreference is the person's choice of light, dark or system, from the UI state in dir, or "" when there is
// none or it is not one of those.
func ReadThemePreference(dir string) string {
	if dir == "" {
		return ""
	}
	data, err := statefile.ReadShared(filepath.Join(dir, "ui-state.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ""
	}
	var v struct {
		Values map[string]json.RawMessage `json:"values"`
	}
	if json.Unmarshal(data, &v) != nil {
		return ""
	}
	var pref string
	if json.Unmarshal(v.Values["pudl-theme"], &pref) != nil {
		return ""
	}
	pref = strings.TrimSpace(pref)
	switch pref {
	case "light", "dark", "system":
		return pref
	}
	return ""
}
