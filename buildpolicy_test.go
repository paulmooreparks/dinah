package buildpolicy

import "testing"

func TestModuleRequiresItsSelectedCompiler(t *testing.T) {
	if _, err := Toolchain(); err != nil {
		t.Fatal(err)
	}
	for _, module := range []string{
		"toolchain go1.26.10\n",
		"go 1.26.0\ntoolchain go1.26.10\n",
		"go invalid\ntoolchain invalid\n",
	} {
		if _, err := toolchain(module); err == nil {
			t.Fatalf("invalid compiler policy accepted: %q", module)
		}
	}
}
