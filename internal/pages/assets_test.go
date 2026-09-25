package pages

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// publishedIntegrity are the SHA-384 values PUDL's README publishes for
// jsDelivr at v0.5.0, written here as literals so that an edit to a vendored
// file and to PROVENANCE together still fails against a value somebody other
// than this project published.
var publishedIntegrity = map[string]string{
	"pudl.css":         "nIVRM8BP2HkZXTOq81/FHwj0n1LB2i8TlwJH9z9Epbljm4KF9Js/5b9qA6mZIHuL",
	"pudl-theme.js":    "MVBsHKpekAHr+tn0bTWhvmuChc2GE0LuMgNXVtxFYU2ht0dMVHtSreaYuhhEb2my",
	"pudl-windows.css": "a6Hk3ctcWCEH1a1w21ajWl1IJ6RtMngseAQgi5F0x0obYAeTOM3lEwFiONe5o/kn",
	"pudl-windows.js":  "DqM9FWJUDZTv5dfaQoBPgKI5FfAqDQwxEcxwGpoIAu4OSh0CDtCnDXYpmr912Mz9",
}

// TestVendoredPUDLMatchesProvenance holds the embedded pudl directory to the
// five files PROVENANCE lists, each with both of its listed hashes, and
// PROVENANCE to the tag, the commit and the four values PUDL published.
func TestVendoredPUDLMatchesProvenance(t *testing.T) {
	provenance, err := assetFiles.ReadFile("assets/pudl/PROVENANCE")
	if err != nil {
		t.Fatalf("read PROVENANCE: %v", err)
	}
	text := string(provenance)
	for _, line := range []string{"tag v0.5.0", "commit f86222bb66fd5a92c97571d591fc35d7fc7b392d"} {
		if !strings.Contains(text, line+"\n") {
			t.Errorf("PROVENANCE does not carry %q", line)
		}
	}
	for name, value := range publishedIntegrity {
		if !strings.Contains(text, "sha384 "+value+" "+name+"\n") {
			t.Errorf("PROVENANCE does not carry the published sha384 of %s", name)
		}
	}
	listed := map[string]map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && (fields[0] == "sha256" || fields[0] == "sha384") {
			if listed[fields[2]] == nil {
				listed[fields[2]] = map[string]string{}
			}
			listed[fields[2]][fields[0]] = fields[1]
		}
	}
	entries, err := assetFiles.ReadDir("assets/pudl")
	if err != nil {
		t.Fatalf("read the embedded pudl directory: %v", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.Name() != "PROVENANCE" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	if strings.Join(files, " ") != "LICENSE pudl-theme.js pudl-windows.css pudl-windows.js pudl.css" || len(listed) != 5 {
		t.Fatalf("the embedded pudl directory holds %v and PROVENANCE lists %d files, wanted the same five", files, len(listed))
	}
	for _, name := range files {
		data, err := assetFiles.ReadFile("assets/pudl/" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		sum256 := sha256.Sum256(data)
		sum384 := sha512.Sum384(data)
		if got := hex.EncodeToString(sum256[:]); got != listed[name]["sha256"] {
			t.Errorf("%s: sha256 %s, PROVENANCE lists %s", name, got, listed[name]["sha256"])
		}
		if got := base64.StdEncoding.EncodeToString(sum384[:]); got != listed[name]["sha384"] {
			t.Errorf("%s: sha384 %s, PROVENANCE lists %s", name, got, listed[name]["sha384"])
		}
	}
	t.Logf("checked %d vendored files", len(files))
}

// TestNoFontFileIsEmbedded walks the whole embedded tree for a font file.
func TestNoFontFileIsEmbedded(t *testing.T) {
	walked := 0
	err := fs.WalkDir(assetFiles, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		walked++
		switch strings.ToLower(filepath.Ext(path)) {
		case ".woff", ".woff2", ".ttf", ".otf":
			t.Errorf("%s is a font file, and the binary embeds none", path)
		}
		return nil
	})
	if err != nil || walked == 0 {
		t.Fatalf("walked %d files: %v", walked, err)
	}
	t.Logf("walked %d embedded files", walked)
}

// fontToken reads the value dinah.css gives --font on :root.
var fontToken = regexp.MustCompile(`:root\s*\{\s*--font:\s*([^;]+);\s*\}`)

// TestTheFontTokenNamesNoEmbeddedFace holds dinah.css to the ruling of
// 2026-09-25: it sets --font on :root to a stack naming no Inter, and it
// sets no palette token.
func TestTheFontTokenNamesNoEmbeddedFace(t *testing.T) {
	css, err := assetFiles.ReadFile("assets/dinah.css")
	if err != nil {
		t.Fatalf("read dinah.css: %v", err)
	}
	match := fontToken.FindSubmatch(css)
	if match == nil {
		t.Fatal("dinah.css sets no --font on :root")
	}
	if bytes.Contains(match[1], []byte("Inter")) {
		t.Errorf("dinah.css sets --font to %q, which names Inter", match[1])
	}
	declared := regexp.MustCompile(`(?m)^\s*(--[a-z-]+)\s*:`).FindAllSubmatch(css, -1)
	for _, d := range declared {
		t.Errorf("dinah.css declares the token %s, and it sets no token but --font", d[1])
	}
	if bytes.Count(css, []byte("--font:")) != 1 {
		t.Errorf("dinah.css sets --font %d times, wanted once", bytes.Count(css, []byte("--font:")))
	}
}

// TestLanternMatchesLogo holds the embedded mark to the repository's own.
func TestLanternMatchesLogo(t *testing.T) {
	embedded, err := assetFiles.ReadFile("assets/dinah-lantern.svg")
	if err != nil {
		t.Fatalf("read the embedded mark: %v", err)
	}
	logo, err := os.ReadFile(filepath.Join("..", "..", "logo", "dinah-lantern.svg"))
	if err != nil {
		t.Fatalf("read the logo: %v", err)
	}
	if !bytes.Equal(embedded, logo) {
		t.Error("internal/pages/assets/dinah-lantern.svg differs from logo/dinah-lantern.svg")
	}
}

// TestPagesImportNoLibrary holds the package to rendering from payload
// bytes: no non-test file imports a library package.
func TestPagesImportNoLibrary(t *testing.T) {
	forbidden := map[string]bool{
		"dinah/internal/verb": true, "dinah/internal/bench": true, "dinah/internal/answer": true,
		"dinah/internal/mcp": true, "dinah/internal/httphead": true,
	}
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	read := 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		read++
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			if forbidden[path] {
				t.Errorf("%s imports %s", name, path)
			}
		}
	}
	if read == 0 {
		t.Fatal("read no source file")
	}
	t.Logf("read the imports of %d files", read)
}
