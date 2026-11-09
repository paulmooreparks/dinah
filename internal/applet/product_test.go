package applet

import (
	"errors"
	"testing"
)

func TestProductBundlesAreMadeInOrderAndAFailureStopsThem(t *testing.T) {
	before, err := ProductBundles()
	if err != nil {
		t.Fatal(err)
	}
	RegisterBundle(func() (Bundle, error) { return Bundle{Manifest: Manifest{ID: "test.first"}}, nil })
	RegisterBundle(func() (Bundle, error) { return Bundle{Manifest: Manifest{ID: "test.second"}}, nil })
	got, err := ProductBundles()
	if err != nil || len(got) != len(before)+2 || got[len(got)-2].Manifest.ID != "test.first" || got[len(got)-1].Manifest.ID != "test.second" {
		t.Fatalf("bundles: %d %v", len(got), err)
	}
	RegisterBundle(func() (Bundle, error) { return Bundle{}, errors.New("no") })
	if _, err := ProductBundles(); err == nil || err.Error() != "no" {
		t.Fatalf("a failing maker did not stop the assembly: %v", err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("a nil maker was accepted")
			}
		}()
		RegisterBundle(nil)
	}()
}
