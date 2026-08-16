// Package guide serves the texts that teach somebody how to use a bench.
//
// The guides are compiled into the binary and served live, so an upgrade
// updates every reader at once and no guide is ever written into a bench. The
// same texts are what the mcp head offers as resources, so a person reading
// one and an agent reading one read identical bytes.
package guide

import (
	"embed"
	"path"
	"sort"
	"strings"

	"dinah/internal/contract"
)

//go:embed guides/*.md
var guides embed.FS

// Topics lists the embedded guides by topic, sorted.
func Topics() []string {
	entries, err := guides.ReadDir("guides")
	if err != nil {
		return nil
	}
	var topics []string
	for _, entry := range entries {
		topic := strings.TrimSuffix(entry.Name(), ".md")
		topics = append(topics, topic)
	}
	sort.Strings(topics)
	return topics
}

// Text returns one guide. A topic no guide answers to is refused rather than
// served empty, because a caller asking for a guide by name has made a
// mistake worth reporting.
func Text(topic string) (string, error) {
	for _, known := range Topics() {
		if known != topic {
			continue
		}
		data, err := guides.ReadFile(path.Join("guides", topic+".md"))
		if err != nil {
			return "", contract.Refuse(contract.UnknownGuide, topic)
		}
		return string(data), nil
	}
	return "", contract.Refuse(contract.UnknownGuide, topic)
}

// Title returns a guide's own first heading, which is what a listing shows
// beside the topic name. A guide with no heading shows its topic alone.
func Title(topic string) string {
	text, err := Text(topic)
	if err != nil {
		return topic
	}
	for _, line := range strings.Split(text, "\n") {
		if heading, found := strings.CutPrefix(strings.TrimSpace(line), "# "); found {
			return heading
		}
	}
	return topic
}
