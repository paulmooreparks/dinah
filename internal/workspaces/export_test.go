package workspaces

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mkdirs(t *testing.T, root string, folders ...string) {
	t.Helper()
	for _, folder := range append([]string{"."}, folders...) {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(folder)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func raw(fields map[string]any) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for name, value := range fields {
		data, _ := json.Marshal(value)
		out[name] = data
	}
	return out
}

// An export holds the structure of every source, which local folder is the
// main one, the folders inside each local folder and the pins, and none of
// the computer's own IDs. A source and a pin of a kind this build does not
// know are written with the fields it does not know.
func TestBuildExportHoldsTheStructure(t *testing.T) {
	home := t.TempDir()
	main := filepath.Join(home, "OneDrive", "Orchard")
	notes := filepath.Join(home, "OneDrive", "Notes")
	mkdirs(t, main, "Concepts/GAP 01", "Concepts/GAP 02", ".devin/skills", "Research")
	mkdirs(t, notes)
	if err := os.WriteFile(filepath.Join(main, "Concepts", "readme.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := Record{ID: "ws_1", Name: "Orchard", ExecutionPath: main, PrimarySourceID: "a", Sources: []Source{
		{ID: "a", Kind: "local-folder", Name: "Orchard", Path: main},
		{ID: "b", Kind: "local-folder", Name: "Notes", Path: notes},
		{ID: "c", Kind: "web-page", Name: "Concepts", Extra: raw(map[string]any{"baseUrl": "https://wiki.example.com", "pageId": "123", "includeChildren": true})},
	}, Pins: []Pin{
		{Kind: "resource", Title: "GAP 01", Ref: &Ref{WorkspaceID: "ws_1", SourceID: "a", Path: "Concepts/GAP 01"}, Directory: true},
		{Kind: "chat", Title: "A chat", Session: "s1"},
		{Kind: "flow", Title: "Review", Extra: raw(map[string]any{"flowScope": "workspace", "flowFile": "review.flow.md"})},
	}}
	file := filepath.Join(main, "Orchard"+ExportExtension)
	got := BuildExport(record, file, home)
	if got.Format != ExportFormat || got.Name != "Orchard" || len(got.Sources) != 3 {
		t.Fatalf("export: %+v", got)
	}
	local := got.Sources[0]
	if !local.Main || local.Local.FromHome != "OneDrive/Orchard" || local.Local.FromFile != "." || local.Local.Absolute != filepath.ToSlash(main) {
		t.Fatalf("main folder location: %+v %+v", local, local.Local)
	}
	if !reflect.DeepEqual(local.Local.Folders, []string{"Concepts", "Concepts/GAP 01", "Concepts/GAP 02", "Research"}) {
		t.Fatalf("folders without hidden folders and files: %v", local.Local.Folders)
	}
	if got.Sources[1].Main || got.Sources[1].Local.FromFile != "../Notes" {
		t.Fatalf("second folder: %+v", got.Sources[1].Local)
	}
	if len(got.Pins) != 2 || got.Pins[0].Source != "s1" || got.Pins[0].Path != "Concepts/GAP 01" || got.Pins[1].Kind != "flow" {
		t.Fatalf("pins, without the chat: %+v", got.Pins)
	}
	data, _ := json.Marshal(got)
	for _, id := range []string{`"ws_1"`, `"a"`} {
		if strings.Contains(string(data), id) {
			t.Fatalf("export holds the computer's ID %s: %s", id, data)
		}
	}
	// The other kinds' fields are written beside the others, as a build that
	// knows the kinds writes them.
	for _, field := range []string{`"pageId":"123"`, `"includeChildren":true`, `"flowFile":"review.flow.md"`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("export lacks %s: %s", field, data)
		}
	}
	back, err := ReadExport(data)
	if err != nil || !reflect.DeepEqual(back, got) {
		t.Fatalf("read back: %v\n%+v\n%+v", err, back, got)
	}
}

// A file written by a build that knows more kinds, in the same format, is
// read with those kinds kept, and an export of what it made writes them
// again unchanged.
func TestUnknownKindsPassThroughARoundTrip(t *testing.T) {
	text := `{"format":1,"name":"Orchard","later":"ignored","sources":[` +
		`{"key":"s1","kind":"local-folder","name":"Orchard","main":true,"local":{"absolute":"C:/Orchard","folders":["Concepts"]}},` +
		`{"key":"s2","kind":"web-page","name":"Concepts","baseUrl":"https://wiki.example.com","pageId":"123","includeChildren":true},` +
		`{"key":"s3","kind":"tracker-project","name":"ORCH","baseUrl":"https://tracker.example.com","projectKey":"ORCH"}],` +
		`"pins":[{"kind":"flow","title":"Review","flowScope":"workspace","flowFile":"review.flow.md"},` +
		`{"kind":"resource","title":"Page","source":"s2","path":"123/456"}]}`
	in, err := ReadExport([]byte(text))
	if err != nil || len(in.Sources) != 3 || len(in.Pins) != 2 {
		t.Fatalf("read: %+v %v", in, err)
	}
	// The import makes the registry's record from it, as the window does,
	// and the registry is saved and read again.
	record := Record{ID: "ws_x", Name: in.Name}
	ids := map[string]string{}
	for _, s := range in.Sources {
		source := Source{ID: "src_" + s.Key, Kind: s.Kind, Name: s.Name, Extra: copyExtra(s.Extra)}
		if s.Kind == LocalFolder {
			source.Path = filepath.FromSlash(s.Local.Absolute)
			record.ExecutionPath, record.PrimarySourceID = source.Path, source.ID
		}
		ids[s.Key] = source.ID
		record.Sources = append(record.Sources, source)
	}
	for _, p := range in.Pins {
		pin := Pin{Kind: p.Kind, Title: p.Title, Extra: copyExtra(p.Extra)}
		if p.Source != "" {
			pin.Ref = &Ref{WorkspaceID: record.ID, SourceID: ids[p.Source], Path: p.Path}
		}
		record.Pins = append(record.Pins, pin)
	}
	saved, err := json.Marshal([]Record{record})
	if err != nil {
		t.Fatal(err)
	}
	var loaded []Record
	if err := json.Unmarshal(saved, &loaded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded[0].Sources[1].Extra, in.Sources[1].Extra) || PinKey(loaded[0].Pins[0]) != PinKey(record.Pins[0]) {
		t.Fatalf("the registry lost the fields it does not know: %s", saved)
	}
	out := BuildExport(loaded[0], "", "")
	data, _ := json.Marshal(out)
	again, err := ReadExport(data)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 3; i++ {
		if again.Sources[i].Kind != in.Sources[i].Kind || again.Sources[i].Name != in.Sources[i].Name || !SameExtra(again.Sources[i].Extra, in.Sources[i].Extra) {
			t.Fatalf("source %d changed: %+v, was %+v", i, again.Sources[i], in.Sources[i])
		}
	}
	if !SameExtra(again.Pins[0].Extra, in.Pins[0].Extra) || again.Pins[1].Source != "s2" || again.Pins[1].Path != "123/456" {
		t.Fatalf("pins changed: %+v, were %+v", again.Pins, in.Pins)
	}
	if strings.Contains(string(data), "later") {
		t.Fatal("a field of the file itself that this build does not know was kept")
	}
}

func TestReadExportRefusesWhatItCannotTrust(t *testing.T) {
	for name, text := range map[string]string{
		"not json":        `nope`,
		"no format":       `{"name":"x","sources":[]}`,
		"newer":           `{"format":99,"name":"x","sources":[]}`,
		"no name":         `{"format":1,"name":" ","sources":[]}`,
		"no kind":         `{"format":1,"name":"x","sources":[{"key":"s1","kind":"","name":"x"}]}`,
		"odd kind":        `{"format":1,"name":"x","sources":[{"key":"s1","kind":"../x","name":"x"}]}`,
		"main web":        `{"format":1,"name":"x","sources":[{"key":"s1","kind":"ftp","name":"x","main":true}]}`,
		"escape":          `{"format":1,"name":"x","sources":[{"key":"s1","kind":"local-folder","name":"x","local":{"absolute":"C:/x","folders":["../out"]}}]}`,
		"two mains":       `{"format":1,"name":"x","sources":[{"key":"s1","kind":"local-folder","name":"a","main":true,"local":{"absolute":"/a"}},{"key":"s2","kind":"local-folder","name":"b","main":true,"local":{"absolute":"/b"}}]}`,
		"duplicate key":   `{"format":1,"name":"x","sources":[{"key":"s1","kind":"tracker-project","name":"a","projectKey":"A"},{"key":"s1","kind":"tracker-project","name":"b","projectKey":"B"}]}`,
		"local nowhere":   `{"format":1,"name":"x","sources":[{"key":"s1","kind":"local-folder","name":"a"}]}`,
		"refused by kind": `{"format":1,"name":"x","sources":[{"key":"s1","kind":"test-page","name":"a","pageId":"12x"}]}`,
	} {
		if _, err := ReadExport([]byte(text)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	got, err := ReadExport([]byte(`{"format":1,"name":"x","later":"ignored","sources":[],"pins":[{"kind":"resource","source":"s9","path":"a"},{"kind":"chat","title":"c","session":"x"},{"kind":"flow","source":"s9"}]}`))
	if err != nil || len(got.Pins) != 0 {
		t.Fatalf("an unknown field is ignored, and pins to no source and chat pins dropped: %+v %v", got, err)
	}
}

// testPage is a kind registered as a build that has it would register it.
func init() {
	RegisterSourceKind("test-page", SourceKind{
		Read: func(s ExportSource) (ExportSource, error) {
			var id string
			if json.Unmarshal(s.Extra["pageId"], &id) != nil || id == "" || strings.Trim(id, "0123456789") != "" {
				return s, errors.New("the test page is invalid")
			}
			return s, nil
		},
		Describe: func(s ExportSource) (string, string) { return "https://pages.example.com", string(s.Extra["pageId"]) },
		Check:    func(context.Context, ExportSource) string { return "Needs a connection to https://pages.example.com" },
	})
}

// A registered kind's own rules are applied, and registering a kind twice
// or a built-in kind is refused.
func TestRegisteredKinds(t *testing.T) {
	got, err := ReadExport([]byte(`{"format":1,"name":"x","sources":[{"key":"s1","kind":"test-page","name":"a","pageId":"12"}]}`))
	if err != nil || len(got.Sources) != 1 {
		t.Fatalf("a valid page was refused: %v", err)
	}
	kind, ok := LookupSourceKind("test-page")
	if !ok || kind.Check(context.Background(), got.Sources[0]) == "" {
		t.Fatal("the registered kind was not found")
	}
	for _, name := range []string{"test-page", "local-folder", "Bad Kind"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("registering %q was accepted", name)
				}
			}()
			RegisterSourceKind(name, SourceKind{})
		}()
	}
}

// A folder is found from the home folder, then from the export file, then
// in full; when none exists, the home folder's place is proposed.
func TestLocateTriesTheThreeWays(t *testing.T) {
	desktopHome, laptopHome := t.TempDir(), t.TempDir()
	original := filepath.Join(desktopHome, "OneDrive", "Orchard")
	location := Location(original, filepath.Join(original, "Orchard"+ExportExtension), desktopHome)

	// On the laptop the user name differs: the home folder finds it.
	moved := filepath.Join(laptopHome, "OneDrive", "Orchard")
	mkdirs(t, moved)
	if path, found := Locate(location, "", laptopHome); !found || path != moved {
		t.Fatalf("from home: %s %v", path, found)
	}
	// The export file beside its folders, outside the home folder, finds it.
	elsewhere := filepath.Join(t.TempDir(), "Synced", "Orchard")
	mkdirs(t, elsewhere)
	if path, found := Locate(location, filepath.Join(elsewhere, "Orchard"+ExportExtension), t.TempDir()); !found || path != elsewhere {
		t.Fatalf("from the file: %s %v", path, found)
	}
	// An export saved in Downloads does not make Downloads the folder.
	downloads := filepath.Join(t.TempDir(), "Downloads")
	mkdirs(t, downloads)
	if path, found := Locate(location, filepath.Join(downloads, "Orchard"+ExportExtension), t.TempDir()); found || path == downloads {
		t.Fatalf("from Downloads: %s %v", path, found)
	}
	// Not there yet: the home folder's place is proposed.
	if path, found := Locate(location, "", t.TempDir()); found || !strings.HasSuffix(path, filepath.Join("OneDrive", "Orchard")) {
		t.Fatalf("proposal: %s %v", path, found)
	}
	// A folder outside the home folder is found only in full.
	outside := t.TempDir()
	if got := Location(outside, "", desktopHome); got.FromHome != "" || got.FromFile != "" {
		t.Fatalf("outside home: %+v", got)
	}
	if path, found := Locate(Location(outside, "", desktopHome), "", laptopHome); !found || path != outside {
		t.Fatalf("in full: %s %v", path, found)
	}
}

func TestListFoldersStopsAtItsLimits(t *testing.T) {
	root := t.TempDir()
	deep := strings.Repeat("d/", ExportFolderDepth+3)
	mkdirs(t, root, deep)
	got := ListFolders(root)
	if len(got) != ExportFolderDepth || strings.Count(got[len(got)-1], "/")+1 != ExportFolderDepth {
		t.Fatalf("depth: %v", got)
	}
	for _, folder := range got {
		if !CleanFolder(folder) {
			t.Fatalf("listed folder fails its own check: %s", folder)
		}
	}
}

// Load reads the registry file the window saved.
func TestLoadReadsTheRegistry(t *testing.T) {
	home := t.TempDir()
	if records, err := Load(home); !errors.Is(err, ErrNoRegistry) || records != nil {
		t.Fatalf("no registry file: %v %v", records, err)
	}
	text := `{"workspaces":[{"id":"ws_1","name":"Orchard","executionPath":"C:/Orchard","sources":[{"id":"j","kind":"tracker-project","name":"ORCH","projectKey":"ORCH"}]}]}`
	if err := os.WriteFile(filepath.Join(home, RegistryFile), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := Load(home)
	if err != nil || len(records) != 1 || string(records[0].Sources[0].Extra["projectKey"]) != `"ORCH"` {
		t.Fatalf("records: %+v %v", records, err)
	}
}
