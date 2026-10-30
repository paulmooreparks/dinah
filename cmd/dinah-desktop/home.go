package main

import (
	"os/user"
	"regexp"
	"strings"
)

// The person's home folder is mounted at /home/<name>. The name is the one
// kept in the settings, when there is one; without it, the local account's
// name; failing that, the profile's ID ("dinah"). The folder on disk is the same whatever the
// name, so nothing in it moves when the name changes.

var homeNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,63}$`)

// homeNameOf returns name, without a Windows domain before it, when it can
// name a folder in a virtual path, or "".
func homeNameOf(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndex(name, `\`); i >= 0 {
		name = name[i+1:]
	}
	if !homeNameRe.MatchString(name) {
		return ""
	}
	return strings.ToLower(name)
}

// localHomeName is the local account's user name, without its domain.
func localHomeName() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	return homeNameOf(u.Username)
}

// homeName is the name of the person's home folder.
func (a *App) homeName() string {
	a.mu.Lock()
	saved := a.settings.HomeName
	a.mu.Unlock()
	if name := homeNameOf(saved); name != "" {
		return name
	}
	if name := localHomeName(); name != "" {
		return name
	}
	return product.ID
}

// homeMountPath is the home folder's virtual path.
func (a *App) homeMountPath() string { return "/home/" + a.homeName() }

// HomeMountPath returns the home folder's virtual path, for the windows that
// show it.
func (a *App) HomeMountPath() string { return a.homeMountPath() }
