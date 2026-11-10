package main

import (
	"errors"
	"sync"
	"time"

	"studio/internal/resource"
	"studio/internal/workspaces"
)

// A source that is kept somewhere else, on a site for one, is listed again in
// the windows that show it when its kind finds that it changed there
// (workspaces.SourceChanged, docs/design/source-kinds.md). A kind may say so
// many times while it reads a tree, so the changes of each source are sent as
// one workspace:changed event, of kind source-changed, per workspace the
// source is in, once the source has been quiet for a moment.

type sourceChanges struct {
	mu      sync.Mutex
	pending map[string]bool
	timer   *time.Timer
	stopped bool
	send    func(sourceID string)
}

// hearSourceChanges has the kinds' word that a source changed sent to the
// page. It is called once, when the window starts.
func (a *App) hearSourceChanges() {
	c := &sourceChanges{pending: map[string]bool{}, send: a.sendSourceChanged}
	a.mu.Lock()
	a.sourceChanges = c
	a.mu.Unlock()
	workspaces.OnSourceChanged(c.changed)
}

// stopSourceChanges ends what hearSourceChanges began.
func (a *App) stopSourceChanges() {
	a.mu.Lock()
	c := a.sourceChanges
	a.sourceChanges = nil
	a.mu.Unlock()
	if c == nil {
		return
	}
	workspaces.OnSourceChanged(nil)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
	if c.timer != nil {
		c.timer.Stop()
	}
}

// changed records that a source changed and returns at once; the event is
// sent once no change has come for watchQuiet.
func (c *sourceChanges) changed(sourceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	c.pending[sourceID] = true
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(watchQuiet, func() {
		c.mu.Lock()
		pending := c.pending
		c.pending = map[string]bool{}
		stopped := c.stopped
		c.mu.Unlock()
		if stopped {
			return
		}
		for id := range pending {
			c.send(id)
		}
	})
}

// sendSourceChanged tells the page that a source changed, once for each
// workspace the source is in.
func (a *App) sendSourceChanged(sourceID string) {
	a.mu.Lock()
	var in []string
	for _, record := range a.settings.WorkspaceRecords {
		for _, source := range record.Sources {
			if source.ID == sourceID {
				in = append(in, record.ID)
				break
			}
		}
	}
	a.mu.Unlock()
	for _, workspaceID := range in {
		a.emitWorkspaceMountChanged("source-changed", workspaceID, sourceID)
	}
}

// RefreshWorkspaceResource has a source forget what it has kept about a
// resource and everything under it, so that listing or reading it next asks
// the place the source is kept. A window's Refresh command calls it before it
// lists again. A local folder keeps nothing, and neither does a source whose
// provider is no resource.Refresher, so for those it does nothing.
func (a *App) RefreshWorkspaceResource(ref ResourceRef) error {
	s := a.resourceService()
	source, err := (workspaceVFS{service: s}).resourceSource(ref)
	if err != nil {
		return err
	}
	if workspaces.IsLocalSource(source) {
		return nil
	}
	p, err := s.providerOf(source)
	if err != nil {
		return err
	}
	refresher, ok := p.(resource.Refresher)
	if !ok {
		return nil
	}
	path := ref.Path
	if path == "" {
		path = "."
	}
	if err := refresher.Refresh(s.ctx, path); err != nil && !errors.Is(err, resource.ErrUnsupported) {
		return resourceErrorText(err)
	}
	return nil
}
