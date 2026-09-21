// Package template serves the workbench templates Dinah ships inside the
// binary, so that `dinah init --from <name>` needs no network access and no
// checkout of anything beyond the binary itself.
//
// A template is an interchange definition, the same JSON dinah export and
// dinah extract write, so the reader that turns it into a workbench is the
// one bench.Instantiate already has. This package's whole job is naming one
// and handing back its bytes.
package template

import (
	"embed"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

//go:embed templates/*.json
var templates embed.FS

// names lists the embedded templates by the word `dinah init --from` accepts
// for each. A name embedded and not listed here is served by nothing, and a
// name listed here and not embedded is offered by nothing; the test in this
// package holds the two sets equal in both directions, the way guide.Topics
// already holds its own set.
var names = []string{
	"pipeline",
}

// Names lists the embedded templates.
func Names() []string {
	return append([]string(nil), names...)
}

// Known reports whether name is an embedded template. Init's readSource
// calls this before it treats source as a directory or a file, so a shipped
// template's name always resolves first, on the precedence
// docs/design/surfaces.md already states for shortnames.
func Known(name string) bool {
	for _, known := range names {
		if known == name {
			return true
		}
	}
	return false
}

// Definition reads one embedded template and parses it. A name Known does
// not answer to is refused rather than served empty, on the same posture
// guide.Text already takes toward an unknown topic.
func Definition(name string) (*bench.Definition, error) {
	if !Known(name) {
		return nil, contract.Refuse(contract.UnknownPath, name)
	}
	data, err := templates.ReadFile("templates/" + name + ".json")
	if err != nil {
		return nil, contract.Refuse(contract.UnknownPath, name)
	}
	return bench.ReadDefinition(data)
}
