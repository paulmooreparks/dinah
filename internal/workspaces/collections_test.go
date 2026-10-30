package workspaces

import (
	"os"
	"path/filepath"
	"testing"

	"studio/internal/layout"
)

// A registered kind of package comes after skills, and its folder in the main
// folder becomes a collection beside the skills.
func TestARegisteredPackageKindIsACollection(t *testing.T) {
	RegisterPackageKind("test-notes", PackageKind{Label: "Notes"})
	kinds := PackageKinds()
	if kinds[0].Key != Skills || kinds[len(kinds)-1].Key != "test-notes" {
		t.Fatalf("kinds %+v", kinds)
	}
	if k, ok := LookupPackageKind("test-notes"); !ok || k.Label != "Notes" {
		t.Fatalf("lookup %+v %v", k, ok)
	}
	if _, ok := LookupPackageKind("flows"); ok {
		t.Fatal("a kind nobody registered was found")
	}
	folder := t.TempDir()
	os.MkdirAll(filepath.Join(folder, PackageFolder, "skills"), 0o755)
	os.MkdirAll(filepath.Join(folder, PackageFolder, "test-notes"), 0o755)
	record := Record{ID: "ws_1", Name: "One", ExecutionPath: folder, PrimarySourceID: "src", Sources: []Source{{ID: "src", Kind: LocalFolder, Name: "Files", Path: folder}}}
	found := map[string]string{}
	for _, c := range Collections(layout.Layout{State: t.TempDir()}, record) {
		kind, _ := CollectionKind(c.ID)
		found[kind.Key] = c.Name
	}
	if found["skills"] != "Files Skills" || found["test-notes"] != "Files Notes" {
		t.Fatalf("collections %v", found)
	}
	for name, key := range map[string]string{"twice": "test-notes", "skills": Skills, "bad key": "Not A Key"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("registering %s did not panic", name)
				}
			}()
			RegisterPackageKind(key, PackageKind{})
		}()
	}
}

// The person's skills folder is inside an isolated state folder.
func TestPersonalSkillsFolderIsInsideAnIsolatedState(t *testing.T) {
	state := t.TempDir()
	if dir, err := PersonalSkillsFolder(state, true); err != nil || dir != filepath.Join(state, "devin-skills") {
		t.Fatalf("%s %v", dir, err)
	}
}
