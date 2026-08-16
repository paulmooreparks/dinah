//go:build ignore

// Command generate_skeletons writes a skeleton catalog for each language the
// format declares that nobody has translated yet.
//
// It is a repository tool rather than a shipped command, because nobody
// outside this project generates one, and the build tag above keeps it out of
// the binary and out of `go build ./...`. Run it from the repository root:
//
//	go run internal/msg/generate_skeletons.go
//
// A skeleton carries every key of the English catalog with the English text
// and the English context comment, marked as a skeleton so that
// `version --catalogs` reports it as untranslated rather than as done.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// wanted are the tags the format's language section declares beyond the two
// that ship complete.
var wanted = []string{"de", "cs", "id", "es", "fil", "af"}

// entry mirrors the shape internal/msg reads.
type entry struct {
	Text     string `json:"text"`
	Context  string `json:"context,omitempty"`
	Skeleton bool   `json:"skeleton,omitempty"`
}

// catalog mirrors the shape internal/msg reads.
type catalog struct {
	Tag     string           `json:"tag"`
	Entries map[string]entry `json:"entries"`
}

func main() {
	dir := filepath.Join("internal", "msg", "locales")
	data, err := os.ReadFile(filepath.Join(dir, "en.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	base := catalog{}
	if err := json.Unmarshal(data, &base); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, tag := range wanted {
		skeleton := catalog{Tag: tag, Entries: map[string]entry{}}
		for key, value := range base.Entries {
			skeleton.Entries[key] = entry{Text: value.Text, Context: value.Context, Skeleton: true}
		}
		encoded, err := json.MarshalIndent(skeleton, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		path := filepath.Join(dir, tag+".json")
		if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("wrote", path, len(skeleton.Entries), "keys")
	}
}
