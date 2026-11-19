package programs

import (
	"path/filepath"
	"strings"
)

// EditPath is the value of a PATH variable, list, with every entry that is
// the folder old taken out and the folder dir in the place of the first of
// them, or at the end when old was not there. An entry that is dir already
// stays where it is, and no second one is added. expand gives an entry's
// value with its variables put in, as %LOCALAPPDATA% in a Windows PATH, so
// that an entry is compared by the folder it is. Entries are compared as
// Windows compares folders, without regard to case. The other entries, and
// their order and spelling, are kept.
func EditPath(list, old, dir string, expand func(string) string) string {
	if expand == nil {
		expand = func(s string) string { return s }
	}
	is := func(entry, folder string) bool {
		e := strings.TrimSpace(expand(entry))
		if e == "" || folder == "" {
			return false
		}
		return strings.EqualFold(filepath.Clean(e), filepath.Clean(folder))
	}
	var out []string
	placed := false
	for _, entry := range strings.Split(list, ";") {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		if is(entry, dir) {
			if placed {
				continue
			}
			placed = true
			out = append(out, entry)
			continue
		}
		if old != "" && is(entry, old) {
			if !placed {
				placed = true
				out = append(out, dir)
			}
			continue
		}
		out = append(out, entry)
	}
	if !placed {
		out = append(out, dir)
	}
	return strings.Join(out, ";")
}
