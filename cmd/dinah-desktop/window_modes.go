package main

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// How a new window opens. Open New Windows is the default for every window. A kind of
// window listed here has a choice of its own, shown in Settings under Open
// New Windows; the person may change it there or reset it to the choice the
// kind ships with. A window the person has placed keeps its placement, which
// the front end applies before either.

// windowModeSame follows Open New Windows.
const windowModeSame = "same"

// windowKindModes are the kinds of window with a choice of their own, and
// the choice each ships with.
var windowKindModes = map[string]string{"editor": "maximized"}

func windowKinds() []string {
	kinds := make([]string, 0, len(windowKindModes))
	for kind := range windowKindModes {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// windowKindsOf is the kinds a change gives, in a stable order.
func windowKindsOf(changes map[string]*string) []string {
	kinds := make([]string, 0, len(changes))
	for kind := range changes {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

func checkWindowKindMode(kind, mode string) error {
	if _, ok := windowKindModes[kind]; !ok {
		return errors.New("that kind of window has no window setting of its own")
	}
	if mode != windowModeSame && mode != "floating" && mode != "maximized" {
		return errors.New("a window setting is same, floating or maximized")
	}
	return nil
}

// effectiveWindowModes is each listed kind's choice: the person's, or the
// one it ships with. An entry for a kind this version does not list is kept
// in the settings and left out here.
func effectiveWindowModes(s Settings) map[string]string {
	out := map[string]string{}
	for kind, shipped := range windowKindModes {
		out[kind] = shipped
		if mode, ok := s.WindowModes[kind]; ok {
			out[kind] = mode
		}
	}
	return out
}

// checkStoredWindowModes refuses settings whose window choices this version
// cannot read, as it does an unknown Open New Windows value.
func checkStoredWindowModes(modes map[string]string) error {
	for kind, mode := range modes {
		if _, ok := windowKindModes[kind]; ok {
			if err := checkWindowKindMode(kind, mode); err != nil {
				return errors.New("settings contain an unsupported window setting for " + kind)
			}
		}
	}
	return nil
}

// withWindowKindMode is s with a kind's choice changed, or removed when mode
// is "", which returns the kind to the choice it ships with.
func withWindowKindMode(s Settings, kind, mode string) (Settings, error) {
	if mode != "" {
		if err := checkWindowKindMode(kind, mode); err != nil {
			return s, err
		}
	} else if _, ok := windowKindModes[kind]; !ok {
		return s, errors.New("that kind of window has no window setting of its own")
	}
	modes := map[string]string{}
	for k, v := range s.WindowModes {
		modes[k] = v
	}
	if mode == "" {
		delete(modes, kind)
	} else {
		modes[kind] = mode
	}
	if len(modes) == 0 {
		modes = nil
	}
	s.WindowModes = modes
	return s, nil
}

// SetWindowKindMode sets how a kind of window opens: same as Open New
// Windows, floating or maximized; "" resets it to the choice it ships with.
func (a *App) SetWindowKindMode(kind, mode string) error {
	if err := a.updateSettings(func(s *Settings) error {
		var err error
		*s, err = withWindowKindMode(*s, kind, mode)
		return err
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}

// windowModesSchema is the JSON schema of the settings applet's windowModes,
// with nullable when a value may be null, which resets a kind.
func windowModesSchema(nullable bool) string {
	value := `{"type":"string","enum":["same","floating","maximized"]}`
	if nullable {
		value = `{"type":["string","null"],"enum":["same","floating","maximized",null]}`
	}
	properties := []string{}
	for _, kind := range windowKinds() {
		name, _ := json.Marshal(kind)
		properties = append(properties, string(name)+":"+value)
	}
	required := ""
	if !nullable {
		names, _ := json.Marshal(windowKinds())
		required = `,"required":` + string(names)
	}
	return `{"type":"object","additionalProperties":false` + required + `,"properties":{` + strings.Join(properties, ",") + `}}`
}
