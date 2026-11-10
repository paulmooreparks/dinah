package profile

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// example is a whole profile of a made-up product, as another product's
// build would inject it.
func example() Profile {
	return Profile{
		Name: "Example Product", ID: "example", Company: "Example Company",
		HomeFolder: ".example", WorkspaceFolder: ".example", LogFile: "example.log", EnvPrefix: "EXAMPLE_",
		ReservedPrograms:       []string{"example"},
		WorkspaceFileExtension: ".example-workspace", PackageManifest: ".example/setup-manifest.json", PackageTool: "example",
		CommandProgram: "example", WindowProgram: "example-desktop", InstanceLock: "example-desktop-1",
		WindowClass: "ExampleWindow", NotificationClass: "ExampleHostNotifications", KeychainService: "example",
		ShellCookie: "example_shell", BusCookie: "example_bus", ShellPort: 47716, BusPort: 47715,
		AppletServer:    "example-applets",
		DefaultProvider: "devin", Providers: []string{"devin"}, DefaultTheme: "example-theme", Updater: UpdaterInstalled,
	}
}

func encode(t *testing.T, p Profile) string {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(data)
}

// Without an injected profile the build is Dinah's; with one, it is the
// product's, whole, with Dinah's values nowhere in it.
func TestAnInjectedProfileReplacesDinahs(t *testing.T) {
	if injected != "" {
		t.Skip("this test runs on Dinah's own build")
	}
	if Current().Name != "Dinah" || Current().HomeFolder != ".dinah" {
		t.Fatalf("Dinah's build has another profile: %+v", Current())
	}
	p, err := parseInjected(encode(t, example()))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Example Product" || p.HomeFolder != ".example" || p.HomeVar() != "EXAMPLE_HOME" || p.ShellAddr() != "127.0.0.1:47716" || !p.Offers("devin") || p.Offers("codex") {
		t.Fatalf("the injected profile was not taken whole: %+v", p)
	}
	// A copy, so a caller cannot change the profile for anybody else.
	c := Current()
	c.Providers[0] = "changed"
	if Current().Providers[0] == "changed" {
		t.Fatal("Current() shares its slices")
	}
}

// An injected profile that lacks a value, names a provider it does not
// offer, or has a field this version does not know is refused, with every
// problem said at once; a product never runs with a mix of names.
func TestAnIncompleteOrUnknownProfileIsRefused(t *testing.T) {
	short := example()
	short.HomeFolder = ""
	short.KeychainService = " "
	short.DefaultProvider = "codex"
	short.BusPort = short.ShellPort
	short.AppletServer = "Example Applets"
	_, err := parseInjected(encode(t, short))
	if err == nil {
		t.Fatal("an incomplete profile was accepted")
	}
	for _, want := range []string{"HomeFolder", "KeychainService", "DefaultProvider must be one of Providers", "must differ", "AppletServer"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	if _, err := parseInjected("not base64!"); err == nil {
		t.Fatal("text that is not base64 was accepted")
	}
	unknown := base64.StdEncoding.EncodeToString([]byte(`{"Name":"X","Colour":"blue"}`))
	if _, err := parseInjected(unknown); err == nil || !strings.Contains(err.Error(), "Colour") {
		t.Fatalf("an unknown field was accepted: %v", err)
	}
}

// The environment's choice of providers, for development, replaces the
// profile's; the default follows it, and a malformed value changes nothing.
func TestProviderOverride(t *testing.T) {
	p := example()
	got := withProviderOverride(p, "codex, claude")
	if len(got.Providers) != 2 || got.Providers[1] != "claude" || got.DefaultProvider != "codex" {
		t.Fatalf("override: %+v", got)
	}
	kept := withProviderOverride(p, "devin,ollama")
	if kept.DefaultProvider != "devin" || len(kept.Providers) != 2 {
		t.Fatalf("the default should stay when offered: %+v", kept)
	}
	for _, bad := range []string{"", " , ", "Bad Name", "devin;codex"} {
		if same := withProviderOverride(p, bad); len(same.Providers) != 1 || same.Providers[0] != "devin" {
			t.Fatalf("%q changed the profile: %+v", bad, same)
		}
	}
	if example().ProvidersVar() != "EXAMPLE_PROVIDERS" {
		t.Fatal("the variable's name")
	}
}
