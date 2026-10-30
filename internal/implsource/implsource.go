// Package implsource assembles a package's implementation source for the
// digests of the applet actions that use the package. An action's digest
// covers the code it runs, so a change to that code invalidates approvals
// given for the earlier code. A package embeds its own source files, every
// platform's included so the digest is the same on every platform, and passes
// them to Of.
package implsource

import (
	"embed"
	"io/fs"
	"sort"
)

// Of concatenates the embedded files in name order, each under a header with
// its path.
func Of(files embed.FS) []byte {
	names := []string{}
	fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	sort.Strings(names)
	out := []byte{}
	for _, name := range names {
		data, _ := files.ReadFile(name)
		out = append(out, "\n--- "+name+" ---\n"...)
		out = append(out, data...)
	}
	return out
}
