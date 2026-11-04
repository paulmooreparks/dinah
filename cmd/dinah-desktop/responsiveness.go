package main

import "studio/internal/responsiveness"

// These methods belong only to the authenticated shell page. They are not
// registered as applet actions, extension APIs or bus tools.
func (a *App) SetResponsivenessRecording(enabled bool) responsiveness.State {
	a.responsivenessMu.Lock()
	defer a.responsivenessMu.Unlock()
	r := a.responsiveness.Load()
	if r == nil && !enabled {
		return responsiveness.DisabledState()
	}
	if r == nil {
		r = &responsiveness.Recorder{}
		a.responsiveness.Store(r)
		if bridge, ok := a.events.(interface {
			SetTimingRecorder(*responsiveness.Recorder)
		}); ok {
			bridge.SetTimingRecorder(r)
		}
	}
	return r.Set(enabled)
}
func (a *App) GetResponsivenessTimingSnapshot() responsiveness.Snapshot {
	a.responsivenessMu.Lock()
	defer a.responsivenessMu.Unlock()
	r := a.responsiveness.Load()
	if r == nil {
		return responsiveness.DisabledSnapshot()
	}
	return r.Snapshot()
}
