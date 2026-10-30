package main

// A name the person gave a chat belongs to that chat, and a continuation made
// with another provider is given its parent's name when it is made
// (sessionhost.Continuations.ContinueSession). A continuation saved before that
// reached every path has none, so the bus knows it by the agent's own title,
// and a mention of it by the name the person knows fails. nameContinuations
// gives each continuation without a name the name of its nearest named
// ancestor, and publishes it to the bus as renaming does.

// nameContinuations runs at start, and is safe to run again: a continuation
// that has a name is left as it is.
func (a *App) nameContinuations() {
	// Each saved chat says its own parent. The map of links from a parent to
	// its child holds one child, which is not enough when a chat was continued
	// more than once.
	_, records := conversationLinks()
	parent := make(map[string]string, len(records))
	for id, head := range records {
		if head.ParentSessionID != "" && !head.Deleted {
			parent[id] = head.ParentSessionID
		}
	}
	a.mu.Lock()
	titles := make(map[string]string, len(a.settings.SessionTitles))
	for id, title := range a.settings.SessionTitles {
		titles[id] = title
	}
	a.mu.Unlock()
	for child := range parent {
		if titles[child] != "" {
			continue
		}
		name := ""
		seen := map[string]bool{child: true}
		for id := parent[child]; id != "" && !seen[id]; id = parent[id] {
			seen[id] = true
			if titles[id] != "" {
				name = titles[id]
				break
			}
		}
		if name == "" {
			continue
		}
		if _, err := a.RenameSession(child, name); err != nil {
			logf("core", "error", "giving the continuation %s the name %q: %v", child, name, err)
		}
	}
}
