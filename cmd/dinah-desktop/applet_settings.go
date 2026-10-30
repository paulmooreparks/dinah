package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"

	"studio/internal/applet"
	"studio/internal/sessionhost"
)

//go:embed applet_settings.go settings.go ui_state.go state_file.go state_directory.go providers.go options.go
var settingsAppletSources embed.FS

type AppletSettingsPreferences struct {
	NewWindowMode string `json:"newWindowMode"`
	// WindowModes is each listed kind of window's own choice, which follows
	// Open New Windows when it is "same".
	WindowModes     map[string]string `json:"windowModes"`
	DefaultProvider string            `json:"defaultProvider"`
	ShowThinking    bool              `json:"showThinking"`
}

type AppletSettingsSnapshot struct {
	Preferences AppletSettingsPreferences `json:"preferences"`
	Revision    string                    `json:"revision"`
}

type appletSettingsChanges struct {
	NewWindowMode *string `json:"newWindowMode"`
	// WindowModes changes the kinds given; a null resets a kind to the choice
	// it ships with.
	WindowModes     map[string]*string `json:"windowModes"`
	DefaultProvider *string            `json:"defaultProvider"`
	ShowThinking    *bool              `json:"showThinking"`
}

func settingsAppletSnapshot(settings Settings) AppletSettingsSnapshot {
	mode := settings.NewWindowMode
	if mode == "" {
		mode = "floating"
	}
	preferences := AppletSettingsPreferences{NewWindowMode: mode, WindowModes: effectiveWindowModes(settings), DefaultProvider: defaultProviderOf(settings), ShowThinking: settings.ShowThinking}
	data, _ := json.Marshal(preferences)
	hash := sha256.Sum256(data)
	return AppletSettingsSnapshot{Preferences: preferences, Revision: hex.EncodeToString(hash[:])}
}

// emitSettingsChanged tells the page the preferences as they are now, with a
// generation and version taken under App.mu in the same acquisition that reads
// them (internal/sessionhost/state_events.go). The snapshot is also what the
// settings applet returns, whose output schema is closed, so the numbers go in
// the event beside it and not in the snapshot.
func (a *App) emitSettingsChanged() {
	a.mu.Lock()
	snapshot := settingsAppletSnapshot(a.settings)
	gen, ver := a.appClock().Next()
	a.mu.Unlock()
	a.emit("settings:changed", sessionhost.Stamp(map[string]any{"preferences": snapshot.Preferences, "revision": snapshot.Revision}, gen, ver))
}

func (a *App) settingsAppletBundle() (applet.Bundle, error) {
	properties := func(nullable bool) string {
		return `"newWindowMode":{"type":"string","enum":["floating","maximized"]},"windowModes":` + windowModesSchema(nullable) + `,"defaultProvider":{"type":"string","enum":["devin","codex","claude","ollama"]},"showThinking":{"type":"boolean"}`
	}
	const revision = `{"type":"string","pattern":"^[0-9a-f]{64}$"}`
	output := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["preferences","revision"],"properties":{"preferences":{"type":"object","additionalProperties":false,"required":["newWindowMode","windowModes","defaultProvider","showThinking"],"properties":{` + properties(false) + `}},"revision":` + revision + `}}`)
	actions := []applet.Action{
		{Ref: "dinah.settings/read@1", Title: "Read Preferences", Description: "Read application-wide new-window mode, the window modes of the kinds of window that open their own way (same as the new-window mode, floating or maximized), default agent provider and thinking visibility with their revision. No credentials, paths or permission settings are exposed.", Replay: "pure", Capabilities: []string{"settings.read"}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), OutputSchema: output},
		{Ref: "dinah.settings/update@1", Title: "Update Preferences", Description: "Update the listed application-wide preferences using the revision from Read Preferences. Re-read after a conflict. In windowModes, null resets a kind of window to the mode it ships with. Existing chats retain their provider, and this action does not connect to an agent.", Replay: "once", Capabilities: []string{"settings.write"}, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["revision","changes"],"properties":{"revision":` + revision + `,"changes":{"type":"object","additionalProperties":false,"minProperties":1,"properties":{` + properties(true) + `}}}}`), OutputSchema: output},
	}
	bundle := applet.Bundle{Manifest: applet.Manifest{ID: "dinah.settings", Version: "1.1.0", APIVersion: 1, Title: "Settings", Description: "Read and update a limited set of application-wide preferences with revision protection.", Actions: actions}, Handlers: map[string]applet.Handler{}, Sources: map[string][]byte{}}
	for _, action := range actions {
		ref := action.Ref
		bundle.Handlers[ref] = func(ctx context.Context, input string) (applet.Result, error) {
			return a.executeSettingsApplet(ctx, ref, input)
		}
	}
	entries, err := fs.ReadDir(settingsAppletSources, ".")
	if err != nil {
		return bundle, err
	}
	for _, entry := range entries {
		data, err := settingsAppletSources.ReadFile(entry.Name())
		if err != nil {
			return bundle, err
		}
		bundle.Sources["host/"+entry.Name()] = data
	}
	return bundle, nil
}

func (a *App) executeSettingsApplet(ctx context.Context, ref, input string) (applet.Result, error) {
	if _, ok := applet.InvocationFromContext(ctx); !ok {
		return applet.Result{}, errors.New("a host invocation is required")
	}
	if err := ctx.Err(); err != nil {
		return applet.Result{}, err
	}
	var snapshot AppletSettingsSnapshot
	if ref == "dinah.settings/read@1" {
		a.mu.Lock()
		snapshot = settingsAppletSnapshot(a.settings)
		a.mu.Unlock()
	} else {
		var request struct {
			Revision string                `json:"revision"`
			Changes  appletSettingsChanges `json:"changes"`
		}
		if err := json.Unmarshal([]byte(input), &request); err != nil {
			return applet.Result{}, err
		}
		var err error
		snapshot, err = a.updateAppletSettings(ctx, request.Revision, request.Changes)
		if err != nil {
			return applet.Result{}, err
		}
	}
	data, err := json.Marshal(snapshot)
	return applet.Result{OutputJSON: string(data)}, err
}

func (a *App) updateAppletSettings(ctx context.Context, revision string, changes appletSettingsChanges) (AppletSettingsSnapshot, error) {
	if changes.NewWindowMode == nil && len(changes.WindowModes) == 0 && changes.DefaultProvider == nil && changes.ShowThinking == nil {
		return AppletSettingsSnapshot{}, errors.New("at least one preference must be supplied")
	}
	if changes.NewWindowMode != nil && *changes.NewWindowMode != "floating" && *changes.NewWindowMode != "maximized" {
		return AppletSettingsSnapshot{}, errors.New("unsupported new-window mode")
	}
	if changes.DefaultProvider != nil && !supportedPreferenceProvider(*changes.DefaultProvider) {
		return AppletSettingsSnapshot{}, errors.New("that provider is not supported yet")
	}
	var snapshot AppletSettingsSnapshot
	err := a.updateSettings(func(updated *Settings) error {
		if revision != settingsAppletSnapshot(a.settings).Revision {
			return errors.New("preferences changed since they were read; read Settings again before updating")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if changes.NewWindowMode != nil {
			updated.NewWindowMode = *changes.NewWindowMode
		}
		for _, kind := range windowKindsOf(changes.WindowModes) {
			mode := ""
			if changes.WindowModes[kind] != nil {
				mode = *changes.WindowModes[kind]
			}
			var err error
			if *updated, err = withWindowKindMode(*updated, kind, mode); err != nil {
				return err
			}
		}
		if changes.DefaultProvider != nil {
			updated.DefaultProvider = *changes.DefaultProvider
		}
		if changes.ShowThinking != nil {
			updated.ShowThinking = *changes.ShowThinking
		}
		snapshot = settingsAppletSnapshot(*updated)
		return nil
	})
	if err != nil {
		return AppletSettingsSnapshot{}, err
	}
	a.emitSettingsChanged()
	return snapshot, nil
}
