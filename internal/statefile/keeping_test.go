package statefile

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestKeepingPreservesTheFirstFileExactlyOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	backup := path + ".before-change"
	original := []byte("{\n  \"value\": \"original\"\n}\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	kept, err := WriteValidatedKeeping(path, []byte(`{"value":"new"}`), nil, backup)
	if err != nil || !kept {
		t.Fatalf("first keep %v %v", kept, err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if kept, err := WriteValidatedKeeping(path, []byte(`{"value":"later"}`), nil, backup); err != nil || kept {
				t.Errorf("later keep %v %v", kept, err)
			}
		})
	}
	wg.Wait()
	data, err := ReadRegular(backup)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("copy changed: %s %v", data, err)
	}
}

func TestKeepingRefusesToChangeTheFileWhenTheCopyCannotBeMade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	put(t, path, `{"value":"old"}`)
	backup := filepath.Join(filepath.Dir(path), "missing", "copy")
	if _, err := WriteValidatedKeeping(path, []byte(`{"value":"new"}`), nil, backup); err == nil {
		t.Fatal("write survived a failed copy")
	}
	if data, err := ReadRegular(path); err != nil || string(data) != `{"value":"old"}` {
		t.Fatalf("original changed %s %v", data, err)
	}
}

func TestKeepingRecordsAbsenceAndDoesNotBackUpTheNewFormatLater(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	backup := path + ".before-change"
	if kept, err := WriteValidatedKeeping(path, []byte(`{"new":true}`), nil, backup); err != nil || !kept {
		t.Fatalf("absence %v %v", kept, err)
	}
	if _, err := ReadRegular(backup + ".absent"); err != nil {
		t.Fatal(err)
	}
	if kept, err := WriteValidatedKeeping(path, []byte(`{"new":false}`), nil, backup); err != nil || kept {
		t.Fatalf("later %v %v", kept, err)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatal("new format was mistaken for original")
	}
}
