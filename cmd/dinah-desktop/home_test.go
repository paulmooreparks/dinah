package main

import "testing"

func TestHomeName(t *testing.T) {
	for in, want := range map[string]string{
		"pparks":             "pparks",
		"PParks":             "pparks",
		`EXAMPLE\pparks`:     "pparks",
		"p.parks-2":          "p.parks-2",
		".hidden":            "",
		"two words":          "",
		"a/b":                "",
		"":                   "",
		"pparks@example.com": "",
	} {
		if got := homeNameOf(in); got != want {
			t.Errorf("homeNameOf(%q) = %q, want %q", in, got, want)
		}
	}

	isolateWorkspaceSettings(t)
	a := NewApp()
	a.settings.HomeName = ""
	if local := localHomeName(); local != "" && a.homeMountPath() != "/home/"+local {
		t.Fatalf("without a kept name the home is %q", a.homeMountPath())
	}
	a.settings.HomeName = "PParks"
	if a.homeMountPath() != "/home/pparks" {
		t.Fatalf("with a kept name the home is %q", a.homeMountPath())
	}
	a.settings.HomeName = "not a name"
	if local := localHomeName(); local != "" && a.homeMountPath() != "/home/"+local {
		t.Fatal("an unusable kept name was used")
	}
}
