// Package guide serves the texts that teach somebody how to use a workbench.
//
// The guides are compiled into the binary and served live, so an upgrade
// updates every reader at once and no guide is ever written into a workbench.
// The same texts are what the mcp head offers as resources, so a person
// reading one and an agent reading one read identical bytes.
package guide

import (
	"embed"
	"path"
	"strings"

	"dinah/internal/contract"
)

//go:embed guides/*.md
var guides embed.FS

// reading is the order Dinah recommends the guides be read in, and every
// surface that offers a list of them offers this one. A reader who has opened
// no guide yet learns which one answers the question they arrived with from
// the order alone, which alphabetical order cannot tell them.
//
// A topic embedded and not named here is served by nothing, and a topic named
// here and not embedded is offered by nothing. The test in this package holds
// the two sets equal in both directions, so a card adding a guide is told to
// place it rather than finding it silently appended.
var reading = []string{
	"first-session",
	"getting-started",
	"verbs",
	"references",
	"query",
	"workbench-layout",
}

// Topics lists the embedded guides by topic, in the order Dinah recommends
// reading them.
func Topics() []string {
	entries, err := guides.ReadDir("guides")
	if err != nil {
		return nil
	}
	embedded := map[string]bool{}
	for _, entry := range entries {
		embedded[strings.TrimSuffix(entry.Name(), ".md")] = true
	}
	var topics []string
	for _, topic := range reading {
		if !embedded[topic] {
			continue
		}
		topics = append(topics, topic)
	}
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
