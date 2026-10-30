package profile

import (
	"reflect"
	"testing"
)

// TestDinahProfile pins Dinah's values to the ones the program used before
// they moved into the profile, so that a change to any of them is made on
// purpose and seen in review.
func TestDinahProfile(t *testing.T) {
	want := Profile{
		Name:              "Dinah",
		ID:                "dinah",
		Company:           "Paul Parks",
		HomeFolder:        ".dinah",
		WorkspaceFolder:   ".dinah",
		LogFile:           "dinah.log",
		EnvPrefix:         "DINAH_",
		CommandProgram:    "dinah",
		WindowProgram:     "dinah-desktop",
		InstanceLock:      "dinah-desktop-4b1f0c6e",
		WindowClass:       "DinahWindow",
		NotificationClass: "DinahHostNotifications",
		KeychainService:   "dinah",
		ShellCookie:       "dinah_shell",
		BusCookie:         "dinah_bus",
		ShellPort:         47616,
		BusPort:           47615,
		DefaultProvider:   "devin",
		Providers:         []string{"devin", "codex", "claude", "ollama"},
		DefaultTheme:      "mineral",
		Updater:           UpdaterInstalled,

		WorkspaceFileExtension: ".dinah-workspace",
		PackageManifest:        ".dinah/setup-manifest.json",
		PackageTool:            "dinah",
		ReservedPrograms:       []string{"dinah"},
	}
	if got := Current(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Dinah's profile is\n%+v\nwant\n%+v", got, want)
	}
	p := Current()
	for _, c := range []struct{ got, want string }{
		{p.HomeVar(), "DINAH_HOME"},
		{p.HostedVar(), "DINAH_HOSTED"},
		{p.ShellAddr(), "127.0.0.1:47616"},
		{p.BusAddr(), "127.0.0.1:47615"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
	if !p.Offers("claude") || p.Offers("gemini") {
		t.Error("Offers disagrees with Providers")
	}
}

// TestCurrentIsACopy checks that a caller changing what Current returned
// does not change the profile.
func TestCurrentIsACopy(t *testing.T) {
	p := Current()
	p.Name = "Other"
	p.Providers[0] = "other"
	if q := Current(); q.Name != "Dinah" || q.Providers[0] != "devin" {
		t.Fatalf("the profile changed through a copy: %+v", q)
	}
}
