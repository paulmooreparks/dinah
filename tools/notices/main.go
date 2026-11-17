// Command notices writes the product's third-party notices: every
// open-source component that is built into the programs or served by them,
// with its version and the licence texts its authors require to be kept
// with it.
//
// Nothing in the list is typed by hand. The Go modules are the ones the
// programs link (go list -deps, for each published platform), each with the
// licence files in its own source. The components of the page are found by
// their licence files under the folders the programs embed. So a dependency
// that is added appears the next time this runs, and the check mode fails a
// build whose notices are out of date.
//
//	go run ./tools/notices                 write THIRD-PARTY-NOTICES.md
//	go run ./tools/notices -check          fail if that file is out of date
//
// A product built from this source says what its notices cover in
// cmd/dinah-desktop/product/notices.json (the options type below).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type module struct {
	Path    string
	Version string
	Dir     string
}

type notice struct {
	Name string `json:"file"` // the file's path, as a reader would look for it
	Text string `json:"text"`
}

type component struct {
	Title   string
	Version string
	Kind    string
	Notices []notice
}

// licenceFile reports whether a file's name says it is a notice its
// component asks to be kept: a licence, a notice, a list of authors or
// patents. The names are the ones the components in this repository use.
var licenceFile = regexp.MustCompile("(?i)^(.*[-._])?(licen[sc]es?|copying|notice|patents|unlicense|authors|copyright|ofl)([-._].*)?$")

// provenance is a file named NOTICE. Beside a component vendored into the
// page it is the record of whoever vendored it: where the files came from
// and when.
var provenance = regexp.MustCompile("(?i)^(.*[-._])?notice([-._].*)?$")

// versioned is a folder that ends with its component's version (katex-0.19.1).
var versioned = regexp.MustCompile("^(.*?)-([0-9]+([.][0-9]+)*)$")

// names gives a bundled library the name it is known by, where its folder's
// name is not that.
var names = map[string]string{
	"pudl":       "PUDL, the Pleasantly Usable Design Language",
	"pudl/fonts": "Inter, the typeface bundled with PUDL",
	"yavc":       "yavc.app, the document, diagram and slide applets",
}

// elections records, for a component offered under a choice of licences,
// which one the product takes it under and what that licence asks to be
// said. The Markdown and diagram engines bundled with the page made the same
// choice for the same module.
var elections = map[string]string{
	"github.com/golang/freetype": "It is offered under a choice of the FreeType License and the GNU General Public License, and is used here under the FreeType License. Portions of this software are copyright © 2006 The FreeType Project (www.freetype.org). All rights reserved",
}

// options says what the notices are written for. A product built from this
// source gives its own in productOptions, in the product directory, which
// is the one place a product adds files; without that file these are used.
type options struct {
	// Product is the product's name, for the document's opening.
	Product string `json:"product"`
	// Programs are the programs that are built.
	Programs []string `json:"programs"`
	// Platforms are the platforms the programs are published for, as os/arch.
	Platforms []string `json:"platforms"`
	// Tags are the build tags the programs are built with.
	Tags string `json:"tags"`
	// Roots are the folders whose files the programs embed.
	Roots []string `json:"roots"`
	// Own is what is the product owner's own work and so is left out:
	// module paths, and page components by the name the document gives them.
	Own []string `json:"own"`
	// Out is the file the notices are written to.
	Out string `json:"out"`
	// Page is the file the same notices are written to as data, for the
	// window that shows them (Help, About, Open-Source Notices). The
	// programs serve it with the page.
	Page string `json:"page"`
}

const productOptions = "cmd/dinah-desktop/product/notices.json"

func defaults() options {
	return options{
		Product:   "Dinah",
		Programs:  []string{"./cmd/dinah-desktop", "./cmd/dinah"},
		Platforms: []string{"windows/amd64", "linux/amd64", "darwin/arm64"},
		Tags:      "desktop,production",
		Roots:     []string{"cmd/dinah-desktop/frontend", "internal/web/static"},
		Own:       []string{"github.com/paulmooreparks/dinah-extension-go", "vendor/parks-files", "vendor/parks-terminal"},
		Out:       "THIRD-PARTY-NOTICES.md",
		Page:      "cmd/dinah-desktop/frontend/third-party-notices.json",
	}
}

// load is the options in force: the defaults, with whatever the product's file gives in their place.
func load() (options, error) {
	o := defaults()
	b, err := os.ReadFile(productOptions)
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		return o, err
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return o, fmt.Errorf("%s: %v", productOptions, err)
	}
	return o, nil
}

func main() {
	check := flag.Bool("check", false, "write nothing, and fail if the file is out of date")
	flag.Parse()
	o, err := load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "notices:", err)
		os.Exit(1)
	}
	text, data, err := build(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "notices:", err)
		os.Exit(1)
	}
	for _, file := range [][2]string{{o.Out, text}, {o.Page, data}} {
		if file[0] == "" {
			continue
		}
		if *check {
			if !current(file[0], file[1]) {
				fmt.Fprintf(os.Stderr, "notices: %s is out of date; run go run ./tools/notices\n", file[0])
				os.Exit(1)
			}
			continue
		}
		if err := os.WriteFile(file[0], []byte(file[1]), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "notices:", err)
			os.Exit(1)
		}
	}
}

// current reports whether the file holds this text.
func current(file, text string) bool {
	have, err := os.ReadFile(file)
	return err == nil && bytes.Equal(normal(have), normal([]byte(text)))
}

// normal is a file's text with one kind of line ending, so that a checkout
// that changed the endings is not taken for a change to the notices.
func normal(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

// build is the notices as the document and as the page's data.
func build(o options) (string, string, error) {
	product, roots := o.Product, o.Roots
	ownName := map[string]bool{}
	for _, name := range o.Own {
		ownName[name] = true
	}
	modules, err := linkedModules(o.Programs, o.Platforms, o.Tags, ownName)
	if err != nil {
		return "", "", err
	}
	var goComponents []component
	var missing []string
	for _, m := range modules {
		notices, err := noticesIn(m.Dir, m.Path+"/")
		if err != nil {
			return "", "", err
		}
		if len(notices) == 0 {
			missing = append(missing, m.Path+" "+m.Version)
			continue
		}
		kind := "Go module"
		if note := elections[m.Path]; note != "" {
			kind += ". " + note
		}
		goComponents = append(goComponents, component{Title: m.Path, Version: m.Version, Kind: kind, Notices: notices})
	}
	if len(missing) > 0 {
		return "", "", fmt.Errorf("no licence file was found for: %s", strings.Join(missing, ", "))
	}
	toolchain, err := goToolchain()
	if err != nil {
		return "", "", err
	}
	page, err := pageComponents(roots, ownName)
	if err != nil {
		return "", "", err
	}
	data, err := pageData(append([]component{toolchain}, goComponents...), page)
	if err != nil {
		return "", "", err
	}
	return render(product, toolchain, goComponents, page), data, nil
}

// pageData is the notices as the window that shows them reads them: each
// component with its name, version, licence, where it is used and its texts.
func pageData(built, bundled []component) (string, error) {
	type entry struct {
		Name    string   `json:"name"`
		Version string   `json:"version"`
		Licence string   `json:"licence"`
		Where   string   `json:"where"`
		Note    string   `json:"note,omitempty"`
		Notices []notice `json:"notices"`
	}
	var list []entry
	add := func(where string, components []component) {
		for _, c := range components {
			note := ""
			if _, rest, ok := strings.Cut(c.Kind, ". "); ok {
				note = rest + "."
			}
			list = append(list, entry{Name: c.Title, Version: c.Version, Licence: licencesOf(c), Where: where, Note: note, Notices: c.Notices})
		}
	}
	add("Built into the programs", built)
	add("Bundled with the page", bundled)
	b, err := json.MarshalIndent(struct {
		Components []entry `json:"components"`
	}{list}, "", " ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// linkedModules is every module some program links on some platform, apart
// from the product owner's own.
func linkedModules(programs, platforms []string, tags string, own map[string]bool) ([]module, error) {
	found := map[string]module{}
	for _, platform := range platforms {
		goos, goarch, ok := strings.Cut(platform, "/")
		if !ok {
			return nil, fmt.Errorf("platform %q is not os/arch", platform)
		}
		args := append([]string{"list", "-tags", tags, "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}{{end}}"}, programs...)
		cmd := exec.Command("go", args...)
		// The packages a platform links through C are part of what it ships, and listing them needs no compiler.
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		outBytes, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("go list for %s: %v: %s", platform, err, strings.TrimSpace(stderr.String()))
		}
		for _, line := range strings.Split(strings.TrimSpace(string(outBytes)), "\n") {
			parts := strings.SplitN(strings.TrimRight(line, "\r"), "|", 3)
			if len(parts) < 2 || parts[0] == "" || own[parts[0]] {
				continue
			}
			m := module{Path: parts[0], Version: parts[1]}
			if len(parts) > 2 {
				m.Dir = parts[2]
			}
			found[m.Path] = m
		}
	}
	var out []module
	for _, m := range found {
		if m.Dir == "" {
			// With vendored modules the listing gives no folder, and the module's files are under vendor.
			m.Dir = filepath.Join("vendor", filepath.FromSlash(m.Path))
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Path) < strings.ToLower(out[j].Path) })
	return out, nil
}

// noticesIn is the licence and notice files at the top of a folder, with
// every file of a folder named licenses beside them, which is where a
// module that offers a choice of licences keeps their texts.
func noticesIn(dir, prefix string) ([]notice, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []notice
	read := func(file, name string) error {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		out = append(out, notice{Name: prefix + name, Text: string(normal(b))})
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			if name := strings.ToLower(e.Name()); name != "licenses" && name != "licences" {
				continue
			}
			inner, err := os.ReadDir(filepath.Join(dir, e.Name()))
			if err != nil {
				return nil, err
			}
			for _, f := range inner {
				if f.IsDir() || isSource(f.Name()) {
					continue
				}
				if err := read(filepath.Join(dir, e.Name(), f.Name()), e.Name()+"/"+f.Name()); err != nil {
					return nil, err
				}
			}
			continue
		}
		if !licenceFile.MatchString(e.Name()) || isSource(e.Name()) {
			continue
		}
		if err := read(filepath.Join(dir, e.Name()), e.Name()); err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// isSource is a file that only has a licence-like name: source code, a test or a page.
func isSource(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go", ".js", ".mjs", ".cjs", ".ts", ".css", ".html", ".json", ".svg", ".png", ".ps1", ".sh", ".py":
		return true
	}
	return false
}

func goToolchain() (component, error) {
	rootBytes, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return component{}, fmt.Errorf("go env GOROOT: %v", err)
	}
	root := strings.TrimSpace(string(rootBytes))
	b, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		return component{}, fmt.Errorf("the Go toolchain's licence: %v", err)
	}
	/* The programs carry the Go runtime and standard library of the
	   toolchain that built them. The version given is the one go.mod asks
	   for, so that the document is the same whichever machine writes it. */
	return component{Title: "The Go runtime and standard library", Version: goVersion(), Kind: "Go toolchain",
		Notices: []notice{{Name: "go/LICENSE", Text: string(normal(b))}}}, nil
}

// goVersion is the Go version go.mod asks for: its toolchain line, or else its go line.
func goVersion() string {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	version := ""
	for _, line := range strings.Split(string(normal(b)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "toolchain" {
			return strings.TrimPrefix(fields[1], "go")
		}
		if len(fields) == 2 && fields[0] == "go" {
			version = fields[1]
		}
	}
	return version
}

// pageComponents finds the components the programs embed and serve by
// their licence files: each folder that has one is a component, named by
// its place under the embedded folder. Two folders with one name and the
// same notices, as two copies of one library have, are one component with
// both places.
func pageComponents(roots []string, own map[string]bool) ([]component, error) {
	tracked, err := trackedFiles(roots)
	if err != nil {
		return nil, err
	}
	byDir := map[string][]string{}
	for _, file := range tracked {
		base := path.Base(file)
		if !licenceFile.MatchString(base) || isSource(base) {
			continue
		}
		dir := path.Dir(file)
		// A folder named licenses holds the notices of the component above it.
		if strings.EqualFold(path.Base(dir), "licenses") || strings.EqualFold(path.Base(dir), "licences") {
			dir = path.Dir(dir)
		}
		// A file named for its component (mermaid.LICENSE) is that component's, beside the component's own file.
		if name, ok := strings.CutSuffix(base, ".LICENSE"); ok && name != "" {
			dir = dir + "/" + name
		}
		byDir[dir] = append(byDir[dir], file)
	}

	/* A component is found by its licence file, so a library vendored
	   without one would be left out and nobody would know. Everything
	   directly inside a folder named vendor must therefore belong to a
	   component that was found: a folder by having notices of its own, a
	   single file by a notice named for it beside it (mermaid.LICENSE for
	   mermaid.min.js). */
	var bare []string
	seen := map[string]bool{}
	for _, file := range tracked {
		at := strings.LastIndex(file, "/vendor/")
		if at < 0 {
			continue
		}
		vendor, rest := file[:at+len("/vendor")], file[at+len("/vendor/"):]
		name, _, inFolder := strings.Cut(rest, "/")
		if !inFolder {
			if licenceFile.MatchString(name) && !isSource(name) {
				continue
			}
			name, _, _ = strings.Cut(name, ".")
		}
		unit := vendor + "/" + name
		if seen[unit] {
			continue
		}
		seen[unit] = true
		// A folder is accounted for by notices of its own or of something inside it.
		covered := false
		for dir := range byDir {
			if dir == unit || strings.HasPrefix(dir, unit+"/") {
				covered = true
				break
			}
		}
		if !covered {
			bare = append(bare, unit)
		}
	}
	if len(bare) > 0 {
		sort.Strings(bare)
		return nil, fmt.Errorf("these vendored components have no licence file, so they cannot be listed: %s", strings.Join(bare, ", "))
	}

	type found struct {
		component
		dirs []string
	}
	bySum := map[string]*found{}
	var order []string
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		title, version := nameAndVersion(dir, roots)
		if own[title] {
			continue
		}
		// A library that says its own version in a file named VERSION is given that version.
		if version == "" {
			if b, err := os.ReadFile(filepath.FromSlash(dir + "/VERSION")); err == nil {
				version = strings.TrimSpace(string(b))
			}
		}
		// A library known by a name other than its folder's is listed by that name.
		if name := names[title]; name != "" {
			title = name
		}
		files := byDir[dir]
		sort.Strings(files)
		/* A NOTICE file beside a vendored component is the vendoring
		   project's record of where the files came from. It is the
		   component's own notice only when the component has no other,
		   and then it is kept. */
		var licences []string
		for _, file := range files {
			if !provenance.MatchString(path.Base(file)) {
				licences = append(licences, file)
			}
		}
		if len(licences) > 0 {
			files = licences
		}
		var notices []notice
		sum := sha256.New()
		for _, file := range files {
			b, err := os.ReadFile(filepath.FromSlash(file))
			if err != nil {
				return nil, err
			}
			text := string(normal(b))
			fmt.Fprintf(sum, "%s\x00%s\x00", path.Base(file), text)
			notices = append(notices, notice{Name: underRoot(file, roots), Text: text})
		}
		// Two folders are one component only when they have one name and the same notices.
		key := fmt.Sprintf("%s\x00%x", path.Base(dir), sum.Sum(nil))
		if have := bySum[key]; have != nil {
			have.dirs = append(have.dirs, dir)
			continue
		}
		bySum[key] = &found{component: component{Title: title, Version: version, Kind: "Bundled with the page", Notices: notices}, dirs: []string{dir}}
		order = append(order, key)
	}
	var out []component
	for _, key := range order {
		out = append(out, bySum[key].component)
	}
	return out, nil
}

// underRoot is a file's or folder's place under the embedded folder it is in.
func underRoot(file string, roots []string) string {
	for _, root := range roots {
		if rest, ok := strings.CutPrefix(file, strings.TrimSuffix(root, "/")+"/"); ok {
			return rest
		}
	}
	return file
}

// nameAndVersion reads a component's name and version from its folder: the
// name is the folder's place under the embedded folder, and the version is
// the number the vendored folders end with (vendor/katex-0.19.1).
func nameAndVersion(dir string, roots []string) (string, string) {
	name := underRoot(dir, roots)
	if m := versioned.FindStringSubmatch(name); m != nil {
		return m[1], m[2]
	}
	return name, ""
}

// trackedFiles is the files git tracks under the folders, so that a build's
// leftovers and a developer's node_modules are never taken for components.
func trackedFiles(roots []string) ([]string, error) {
	args := append([]string{"ls-files", "-z", "--"}, roots...)
	outBytes, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %v", err)
	}
	var out []string
	for _, file := range strings.Split(string(outBytes), "\x00") {
		if file == "" || strings.Contains(file, "/node_modules/") || strings.Contains(file, "/e2e/") {
			continue
		}
		out = append(out, file)
	}
	return out, nil
}

// licenceName names the common licences by words that only their texts have.
// A text it does not know is left unnamed and is still printed in full.
func licenceName(text string) string {
	t := strings.Join(strings.Fields(strings.ToLower(text)), " ")
	t = strings.NewReplacer("“", "\"", "”", "\"", "‘", "'", "’", "'").Replace(t)
	has := func(s string) bool { return strings.Contains(t, s) }
	switch {
	case has("apache license") && has("version 2.0"):
		return "Apache-2.0"
	case has("mozilla public license") && has("version 2.0"):
		return "MPL-2.0"
	case has("permission is hereby granted, free of charge") && has("the software is provided \"as is\""):
		return "MIT"
	case has("permission to use, copy, modify, and/or distribute this software for any purpose with or without fee"),
		has("permission to use, copy, modify, and distribute this software for any purpose with or without fee"):
		return "ISC"
	case has("redistribution and use in source and binary forms") && has("neither the name"):
		return "BSD-3-Clause"
	case has("redistribution and use in source and binary forms"):
		return "BSD-2-Clause"
	case has("sil open font license"):
		return "OFL-1.1"
	case has("this is free and unencumbered software released into the public domain"):
		return "Unlicense"
	}
	return ""
}

func licencesOf(c component) string {
	seen := map[string]bool{}
	var names []string
	for _, n := range c.Notices {
		if name := licenceName(n.Text); name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "See its notices"
	}
	return strings.Join(names, ", ")
}

func render(product string, toolchain component, modules, page []component) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Third-Party Notices\n\n")
	fmt.Fprintf(&b, "%s includes the open-source software listed in this document. Each entry gives the component, its version where it has one, and the licence texts that its authors ask to be kept with it. The licence named beside a component is read from those texts and is given for convenience; the texts themselves are what apply.\n\n", product)
	fmt.Fprintf(&b, "This document is written by `go run ./tools/notices` from what the programs link and what they serve, and is not edited by hand.\n\n")

	section := func(title, intro string, list []component) {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n| Component | Version | Licence |\n| --- | --- | --- |\n", title, intro)
		for _, c := range list {
			version := c.Version
			if version == "" {
				version = "as bundled"
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", c.Title, version, licencesOf(c))
		}
		b.WriteString("\n")
	}
	section("Built into the programs", "These are compiled into the programs.", append([]component{toolchain}, modules...))
	section("Bundled with the page", "These are served by the programs as part of the window's page. A component that bundles further components lists them in its own notices.", page)

	fmt.Fprintf(&b, "## The notices\n\n")
	for _, c := range append(append([]component{toolchain}, modules...), page...) {
		heading := c.Title
		if c.Version != "" {
			heading += " " + c.Version
		}
		fmt.Fprintf(&b, "### %s\n\n%s.\n\n", heading, c.Kind)
		for _, n := range c.Notices {
			fmt.Fprintf(&b, "`%s`\n\n%s\n\n", n.Name, fenced(n.Text))
		}
	}
	return b.String()
}

// fenced is a text as a code block whose fence is longer than any run of
// tildes in the text, so that nothing in a notice can end the block.
func fenced(text string) string {
	fence := "~~~~"
	for strings.Contains(text, fence) {
		fence += "~"
	}
	return fence + "text\n" + strings.TrimRight(text, "\n") + "\n" + fence
}
