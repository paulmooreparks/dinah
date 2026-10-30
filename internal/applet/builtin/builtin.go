// Package builtin holds the built-in applets that both the desktop app and
// the command line offer, so that an action has the same implementation, and
// therefore the same digest, in both programs.
//
// Each bundle identifies its implementation by the source files of this
// package that it uses, never by files of either program, which the other
// cannot embed.
package builtin

import (
	"embed"

	"studio/internal/applet"
)

//go:embed *.go
var own embed.FS

// Bundles are the built-in applets every program offers.
func Bundles() ([]applet.Bundle, error) {
	var out []applet.Bundle
	for _, make := range []func() (applet.Bundle, error){
		Notifications,
	} {
		b, err := make()
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// sources are the implementation sources of a bundle: this package's
// builtin.go and the files given, by name.
func sources(files []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, name := range append([]string{"builtin.go"}, files...) {
		data, err := own.ReadFile(name)
		if err != nil {
			return nil, err
		}
		out["builtin/"+name] = data
	}
	return out, nil
}
