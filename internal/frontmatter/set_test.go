package frontmatter

import "testing"

func mustSet(t *testing.T, text, key, value string) string {
	t.Helper()
	out, err := SetAny(text, key, value)
	if err != nil {
		t.Fatalf("SetAny(%s, %s): %v", key, value, err)
	}
	return out
}

// A key already there changes in place with its quote character, a new key
// goes before the closing fence, and every other line stays as it was.
func TestSetKeepsTheRest(t *testing.T) {
	tests := []struct {
		name, text, key, value, want string
	}{
		{"existing key", "---\ntype: design-note\nsite: \"1\"\n# a comment\n---\n\nbody\n",
			"site", "2", "---\ntype: design-note\nsite: \"2\"\n# a comment\n---\n\nbody\n"},
		{"existing key, single quotes", "---\nowner: 'Jan'\n---\nbody\n",
			"owner", "Ann", "---\nowner: 'Ann'\n---\nbody\n"},
		{"new key", "---\ntype: roadmap\n---\nbody\n",
			"title", "T", "---\ntype: roadmap\ntitle: T\n---\nbody\n"},
		{"key set twice", "---\ntitle: A\ntitle: B\n---\n",
			"title", "C", "---\ntitle: A\ntitle: C\n---\n"},
		{"no front matter", "# Title\n",
			"title", "Title", "---\ntitle: Title\n---\n\n# Title\n"},
		{"a title YAML would misread", "---\ntitle: Old\n---\n",
			"title", "Keyboard: Navigation", "---\ntitle: \"Keyboard: Navigation\"\n---\n"},
		{"unknown keys stay", "---\nowner: Jan\ntags:\n  - a\ntype: x\n---\n",
			"type", "roadmap", "---\nowner: Jan\ntags:\n  - a\ntype: roadmap\n---\n"},
		{"an indented key of the same name is not the key", "---\nsite:\n  title: x\n---\n",
			"title", "T", "---\nsite:\n  title: x\ntitle: T\n---\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustSet(t, tc.text, tc.key, tc.value); got != tc.want {
				t.Errorf("got\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// What SetAny writes is what Fields reads back.
func TestSetRoundTripsThroughFields(t *testing.T) {
	values := map[string]string{"type": "roadmap", "title": "A: B", "published": "true", "version": "12", "owner": "~jan"}
	for _, shape := range []string{"", "---\ntype: design-note\n---\n\nbody\n"} {
		text := shape
		for k, v := range values {
			text = mustSet(t, text, k, v)
		}
		fields, present := Fields(text)
		if !present {
			t.Fatalf("no front matter in\n%s", text)
		}
		got := map[string]string{}
		for _, f := range fields {
			got[f.Key] = f.Value
		}
		for k, want := range values {
			if got[k] != want {
				t.Errorf("shape %q: %s = %q, want %q\n%s", shape, k, got[k], want, text)
			}
		}
	}
}

func TestSetKeepsWindowsLineEndings(t *testing.T) {
	text := "---\r\ntype: x\r\n---\r\nbody\r\n"
	got := mustSet(t, text, "title", "T")
	got = mustSet(t, got, "type", "roadmap")
	want := "---\r\ntype: roadmap\r\ntitle: T\r\n---\r\nbody\r\n"
	if got != want {
		t.Errorf("got %q", got)
	}
}
