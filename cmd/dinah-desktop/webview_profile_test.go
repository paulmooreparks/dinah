package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestABrowserProfileIsChosenOnceAndKeptAcrossReleases(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"2.0.0", "2.0.0-show"} {
		os.MkdirAll(filepath.Join(root, name), 0o700)
	}
	// The running release's own folders are adopted, and a kind with none
	// gets a folder that has no release in its name.
	if got := webviewProfile(root, profileMain, "2.0.0"); filepath.Base(got) != "2.0.0" {
		t.Fatalf("the main profile is %s", got)
	}
	if got := webviewProfile(root, profileShow, "2.0.0"); filepath.Base(got) != "2.0.0-show" {
		t.Fatalf("the show's profile is %s", got)
	}
	if got := webviewProfile(root, profileDocuments, "2.0.0"); filepath.Base(got) != "profile-documents" {
		t.Fatalf("the documents' profile is %s", got)
	}
	// A later release uses what was recorded and makes no folder of its own.
	for kind, want := range map[string]string{profileMain: "2.0.0", profileShow: "2.0.0-show", profileDocuments: "profile-documents"} {
		if got := webviewProfile(root, kind, "2.0.1"); filepath.Base(got) != want {
			t.Fatalf("under the next release the %s profile is %s", kind, got)
		}
	}
	if isDir(filepath.Join(root, "2.0.1")) {
		t.Fatal("the next release made a profile of its own")
	}
}

func TestTheOnlyEarlierProfileIsAdoptedAndSeveralAreLeft(t *testing.T) {
	one := t.TempDir()
	os.MkdirAll(filepath.Join(one, "2.0.0"), 0o700)
	os.MkdirAll(filepath.Join(one, "2.0.0-documents"), 0o700)
	if got := webviewProfile(one, profileMain, "2.0.1"); filepath.Base(got) != "2.0.0" {
		t.Fatalf("the only earlier profile was not adopted: %s", got)
	}
	if got := webviewProfile(one, profileDocuments, "2.0.1"); filepath.Base(got) != "2.0.0-documents" {
		t.Fatalf("the only earlier documents' profile was not adopted: %s", got)
	}
	several := t.TempDir()
	os.MkdirAll(filepath.Join(several, "1.9.0"), 0o700)
	os.MkdirAll(filepath.Join(several, "2.0.0"), 0o700)
	if got := webviewProfile(several, profileMain, "2.0.1"); filepath.Base(got) != "profile-main" {
		t.Fatalf("with several earlier profiles the choice is %s", got)
	}
	if !isDir(filepath.Join(several, "1.9.0")) || !isDir(filepath.Join(several, "2.0.0")) {
		t.Fatal("an earlier profile was removed")
	}
	// A recorded folder that is gone is chosen again.
	os.RemoveAll(filepath.Join(one, "2.0.0"))
	if got := webviewProfile(one, profileMain, "2.0.1"); filepath.Base(got) != "profile-main" {
		t.Fatalf("a missing recorded profile gave %s", got)
	}
}
