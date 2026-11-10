package main

import (
	"path/filepath"
	"testing"
)

// The lock belongs to the state folder, however the folder was said: a
// program an agent starts from the window has the home variable set to the
// person's own folder, and a window started that way must meet the lock of
// the window already open (10 November 2026).
func TestInstanceIDIsTheSameForTheOwnFolderHoweverItIsSaid(t *testing.T) {
	own := t.TempDir()
	old := userHomeDir
	userHomeDir = func() (string, error) { return own, nil }
	t.Cleanup(func() { userHomeDir = old })

	t.Setenv(product.HomeVar(), "")
	plain := instanceID()
	if plain != product.InstanceLock {
		t.Fatalf("with no home variable the lock is %q, want %q", plain, product.InstanceLock)
	}
	t.Setenv(product.HomeVar(), filepath.Join(own, product.HomeFolder))
	if got := instanceID(); got != plain {
		t.Fatalf("the home variable saying the own folder gave the lock %q, want %q", got, plain)
	}
	t.Setenv(product.HomeVar(), filepath.Join(own, "elsewhere"))
	if got := instanceID(); got == plain {
		t.Fatalf("another state folder shares the own folder's lock %q", got)
	}
}

func TestUnknownOption(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		option  string
		unknown bool
	}{
		{[]string{"window.exe"}, "", false},
		{[]string{"window.exe", "--no-such-option", "x"}, "--no-such-option", true},
		{[]string{"window.exe", "-x"}, "-x", true},
		{[]string{"window.exe", "notes.txt"}, "", false},
	} {
		option, unknown := unknownOption(tc.args)
		if option != tc.option || unknown != tc.unknown {
			t.Errorf("unknownOption(%q) = %q, %v; want %q, %v", tc.args, option, unknown, tc.option, tc.unknown)
		}
	}
}
