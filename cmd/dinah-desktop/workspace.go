package main

import "path/filepath"

func (a *App) currentWorkspace() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.Workspace
}

func rel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(r)
}
