package programs

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"studio/internal/profile"
)

func TestTheProgramFolderIsTheProductsNameInThePersonsProgramFolder(t *testing.T) {
	p := profile.Profile{Name: "Example Product", EnvPrefix: "EXAMPLE_"}
	t.Setenv(FolderVar(p), "")
	dir, err := Folder(p)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "Example Product" {
		t.Fatalf("the folder is %s", dir)
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" && !strings.EqualFold(filepath.Dir(dir), filepath.Join(local, "Programs")) {
			// FOLDERID_UserProgramFiles can be moved, so only note it.
			t.Logf("the person's program folder is %s, not under %s", filepath.Dir(dir), local)
		}
	}
	own := filepath.Join(t.TempDir(), "programs")
	t.Setenv(FolderVar(p), own)
	if dir, err := Folder(p); err != nil || dir != own {
		t.Fatalf("with %s set the folder is %s (%v)", FolderVar(p), dir, err)
	}
	t.Setenv(FolderVar(p), "relative")
	if _, err := Folder(p); err == nil {
		t.Fatal("a relative folder was taken")
	}
	t.Setenv(FolderVar(p), "")
	if _, err := Folder(profile.Profile{Name: `a\b`, EnvPrefix: "EXAMPLE_"}); err == nil {
		t.Fatal("a name that is no folder's name was taken")
	}
}

func TestMoveOutTakesOnlyTheProgramsAndTheirCopies(t *testing.T) {
	from, to := filepath.Join(t.TempDir(), "bin"), filepath.Join(t.TempDir(), "Programs", "Example")
	os.MkdirAll(from, 0o755)
	os.MkdirAll(to, 0o755)
	window, command := FileName("example-desktop"), FileName("example")
	write := func(dir, name, text string) { os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644) }
	write(from, window, "old window")
	write(from, window+".old-1794537927330590600", "older window")
	write(from, window+".before-fix-20261106-153336", "a backup")
	write(from, command+".old-20261113202528808", "older command")
	write(from, "THIRD-PARTY-NOTICES.md", "notices")
	write(from, "seq", "a standard script")
	write(from, "mine.lua", "a person's script")
	write(to, window, "new window")
	write(to, window+".old-1794537927330590600", "a copy already here")
	moved, left, err := MoveOut(from, to, []string{"example-desktop", "example"}, []string{"THIRD-PARTY-NOTICES.md"})
	if err != nil || len(left) > 0 || len(moved) != 5 {
		t.Fatalf("moved %v, left %v, %v", moved, left, err)
	}
	var stay []string
	entries, _ := os.ReadDir(from)
	for _, e := range entries {
		stay = append(stay, e.Name())
	}
	sort.Strings(stay)
	if strings.Join(stay, ",") != "mine.lua,seq" {
		t.Fatalf("the old folder keeps %v", stay)
	}
	if b, _ := os.ReadFile(filepath.Join(to, window)); string(b) != "new window" {
		t.Fatalf("the installed window was replaced: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(to, window+".old-1794537927330590600-2")); string(b) != "older window" {
		t.Fatalf("a copy with a name already taken was not kept under another: %q", b)
	}
	matches, _ := filepath.Glob(filepath.Join(to, window+".old-*-moved"))
	if len(matches) != 1 {
		t.Fatalf("the old window is not a set-aside copy: %v", matches)
	}
	if _, stamp, valid, match := asideDate(filepath.Base(matches[0]), "example-desktop"); !match || !valid || !stamp {
		t.Fatalf("the Updates page would not list %s as a set-aside copy", matches[0])
	}
	// A second run finds nothing more to move.
	if moved, _, _ := MoveOut(from, to, []string{"example-desktop"}, nil); len(moved) != 0 {
		t.Fatalf("a second run moved %v", moved)
	}
	if moved, _, err := MoveOut(filepath.Join(t.TempDir(), "none"), to, []string{"x"}, nil); err != nil || moved != nil {
		t.Fatalf("a folder that is not there: %v %v", moved, err)
	}
}

func TestEditPathPutsTheFolderInPlaceOfTheOldOne(t *testing.T) {
	expand := func(s string) string { return strings.ReplaceAll(s, "%LOCALAPPDATA%", `C:\Users\p\AppData\Local`) }
	newDir := `C:\Users\p\AppData\Local\Programs\Example`
	cases := []struct{ list, old, want string }{
		{`C:\a;C:\Users\p\.example\bin;C:\b`, `C:\Users\p\.example\bin`, `C:\a;` + newDir + `;C:\b`},
		{`C:\a;C:\USERS\P\.example\bin\;C:\b;C:\Users\p\.example\bin`, `C:\Users\p\.example\bin`, `C:\a;` + newDir + `;C:\b`},
		{`C:\a;C:\b`, `C:\Users\p\.example\bin`, `C:\a;C:\b;` + newDir},
		{`C:\a;%LOCALAPPDATA%\Programs\Example;C:\Users\p\.example\bin`, `C:\Users\p\.example\bin`, `C:\a;%LOCALAPPDATA%\Programs\Example`},
		{``, `C:\Users\p\.example\bin`, newDir},
		{`C:\a;;C:\b;`, ``, `C:\a;C:\b;` + newDir},
	}
	for _, c := range cases {
		if got := EditPath(c.list, c.old, newDir, expand); got != c.want {
			t.Errorf("EditPath(%q)\n got %q\nwant %q", c.list, got, c.want)
		}
	}
	if got := EditPath(`C:\a;`+newDir, `C:\old`, newDir, expand); got != `C:\a;`+newDir {
		t.Errorf("a PATH that has the folder was changed: %q", got)
	}
}
