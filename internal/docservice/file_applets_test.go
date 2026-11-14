package docservice

import (
	"path/filepath"
	"testing"
)

func TestFileAppletSelectionKeepsApplicationIdentity(t *testing.T) {
	h := start(t, t.TempDir(), true)
	h.s.opt.Applications["editor"] = Application{Applet: "editor", Name: "Test Editor", FileApplets: map[string]string{".md": "docs", ".markdown": "docs", ".slides.md": "editor"}}
	for _, tc := range []struct{ name, applet string }{{"note.md", "docs"}, {"note.MARKDOWN", "docs"}, {"note.txt", "editor"}, {"note.slides.md", "editor"}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name)
			put(t, path, "# Invented\n")
			p := h.open(path)
			var a Attached
			if problem := p.call(&a, "Attach"); problem != "" {
				t.Fatal(problem)
			}
			if a.Application != "editor" || a.Applet != tc.applet || a.ApplicationName != "Test Editor" || a.Resource != path {
				t.Fatalf("attached %+v", a)
			}
		})
	}
}

func TestBlankFileHandoffRequiresGrantAndPreservesRecovery(t *testing.T) {
	h := start(t, t.TempDir(), true)
	h.s.opt.Applications["editor"] = Application{Applet: "editor", Name: "Test Editor", FileApplets: map[string]string{".md": "docs"}}
	path := filepath.Join(t.TempDir(), "note.md")
	put(t, path, "# Invented\n")
	p := h.open("")
	if problem := p.call(nil, "UseFile", path); problem == "" {
		t.Fatal("ungranted handoff accepted")
	}
	h.s.SetDialogs([]string{path}, nil)
	p.call(nil, "PickOpen")
	if problem := p.call(nil, "State", "", true); problem != "" {
		t.Fatal(problem)
	}
	if problem := p.call(nil, "UseFile", path); problem == "" {
		t.Fatal("changed blank replaced")
	}
	p.call(nil, "State", "", false)
	snapshot := `{"tabs":[{"path":"fixture","draft":"kept"}]}`
	if err := h.s.store.put(Record{Key: fileKey("editor", path), Application: "editor", Path: path, Snapshot: []byte(snapshot)}); err != nil {
		t.Fatal(err)
	}
	var a Attached
	if problem := p.call(&a, "UseFile", path); problem != "" {
		t.Fatal(problem)
	}
	if a.ID != p.id || a.Applet != "docs" || a.Resource != path || a.Kept != snapshot {
		t.Fatalf("handoff %+v", a)
	}
	if problem := p.call(nil, "UseFile", path); problem == "" {
		t.Fatal("used window replaced")
	}
	other := h.open("")
	h.s.SetDialogs([]string{path}, nil)
	other.call(nil, "PickOpen")
	if problem := other.call(nil, "UseFile", path); problem == "" {
		t.Fatal("duplicate file window accepted")
	}
}

func TestDocsNewUsesSameProgramAndRetainsApplet(t *testing.T) {
	h := start(t, t.TempDir(), true)
	h.s.opt.Applications["editor"] = Application{Applet: "editor", Name: "Test Editor", FileApplets: map[string]string{".md": "docs"}}
	// A receiving ticket carries the applet through the ordinary program
	// launch without registering another program with the operating system.
	h.s.tickets["fixture"] = &ticket{request: []byte(`{"operation":"new","input":{}}`), applet: "docs"}
	opened, err := h.s.Open(OpenRequest{Application: "editor", Ticket: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	var a Attached
	h.enter(opened.ID, opened.Entry).call(&a, "Attach")
	if a.Applet != "docs" || a.Application != "editor" {
		t.Fatalf("new %+v", a)
	}
}
