package docservice

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// A document shows a picture by a site path only from the web root the
// person chose for it, never from a guess, and never from outside it.
func TestASitePathIsReadFromTheChosenWebRoot(t *testing.T) {
	state := t.TempDir()
	h := start(t, state, true)
	top := t.TempDir()
	site := filepath.Join(top, "wwwroot")
	for _, dir := range []string{filepath.Join(site, "content"), filepath.Join(site, "images", "a"), filepath.Join(top, "elsewhere")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(site, "content", "article.md")
	put(t, file, "# Article\n")
	put(t, filepath.Join(site, "images", "a", "chart.png"), "chart bytes")
	put(t, filepath.Join(site, "images", "a", "notes.txt"), "not a picture")
	put(t, filepath.Join(top, "secret.png"), "secret bytes")
	p := h.open(file)
	get := func(name string) (int, string) {
		t.Helper()
		res, err := p.http.Get(p.base() + "/file?" + url.Values{"doc": {file}, "name": {name}}.Encode())
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(data)
	}
	root := func() string {
		var r string
		p.call(&r, "WebRoot", file)
		return r
	}

	// Nothing is guessed: the folder is called wwwroot and still is no
	// web root until the person chooses it.
	if code, _ := get("/images/a/chart.png"); code != http.StatusNotFound || root() != "" {
		t.Fatalf("a site path was read with no web root chosen: %d %q", code, root())
	}
	// A cancelled dialog changes nothing.
	var chosen string
	if problem := p.call(&chosen, "PickWebRoot", file); problem != "" || chosen != "" || root() != "" {
		t.Fatalf("a cancelled dialog: %q %q", chosen, problem)
	}
	// A folder the document is not in cannot be its web root.
	h.s.SetFolderDialogs([]string{filepath.Join(top, "elsewhere"), site})
	if problem := p.call(&chosen, "PickWebRoot", file); problem == "" || root() != "" {
		t.Fatalf("a folder the document is not in was taken: %q", root())
	}
	if problem := p.call(&chosen, "PickWebRoot", file); problem != "" || chosen != site || root() != site {
		t.Fatalf("choosing the web root: %q %q %s", chosen, root(), problem)
	}
	if code, text := get("/images/a/chart.png"); code != http.StatusOK || text != "chart bytes" {
		t.Fatalf("a picture under the web root: %d %q", code, text)
	}
	if code, _ := get("/images/a/notes.txt"); code != http.StatusNotFound {
		t.Fatalf("a file that is no picture was served: %d", code)
	}
	for _, climbing := range []string{"/../secret.png", "/images/../../secret.png", "/images/a/../../../secret.png", "//secret.png"} {
		if code, text := get(climbing); code == http.StatusOK && text == "secret bytes" {
			t.Fatalf("%s climbed out of the web root", climbing)
		}
	}
	if code, _ := get("/images/a/missing.png"); code != http.StatusNotFound {
		t.Fatalf("a missing picture: %d", code)
	}

	// The choice is kept for the next run, and for another file under the
	// folder, and not for a file outside it.
	h.s.Stop()
	h2 := start(t, state, true)
	other := filepath.Join(site, "index.md")
	put(t, other, "# Index\n")
	outside := filepath.Join(top, "elsewhere", "loose.md")
	put(t, outside, "# Loose\n")
	var kept, none string
	p2 := h2.open(other)
	p2.call(&kept, "WebRoot", other)
	h2.open(outside).call(&none, "WebRoot", outside)
	if kept != site || none != "" {
		t.Fatalf("the next run: %q for a file under the folder, %q for one outside", kept, none)
	}
	if problem := p2.call(nil, "ClearWebRoot", other); problem != "" {
		t.Fatal(problem)
	}
	p2.call(&kept, "WebRoot", other)
	if kept != "" {
		t.Fatalf("the web root was not forgotten: %q", kept)
	}
}
