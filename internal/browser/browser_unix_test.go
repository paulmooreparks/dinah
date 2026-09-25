//go:build !windows && !darwin

package browser

import (
	"reflect"
	"testing"
)

// recordStart replaces the start seam with a recorder for the length of a
// test.
func recordStart(t *testing.T) *[][]string {
	t.Helper()
	var calls [][]string
	previous := startCommand
	startCommand = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	t.Cleanup(func() { startCommand = previous })
	return &calls
}

// TestOpenHandsTheURLToTheDocumentedMechanism holds Open to starting
// xdg-open with the URL as its one argument.
func TestOpenHandsTheURLToTheDocumentedMechanism(t *testing.T) {
	calls := recordStart(t)
	for _, address := range accepted {
		if err := Open(address); err != nil {
			t.Errorf("Open(%q) = %v", address, err)
		}
	}
	if len(*calls) != len(accepted) {
		t.Fatalf("the start seam was called %d times for %d URLs: %v", len(*calls), len(accepted), *calls)
	}
	for i, address := range accepted {
		if got, want := (*calls)[i], []string{"xdg-open", address}; !reflect.DeepEqual(got, want) {
			t.Errorf("started %q, want %q", got, want)
		}
	}
}

// TestOpenRefusesAnythingButAnHTTPRoot holds Open to refusing every value
// but an http root URL before anything is started.
func TestOpenRefusesAnythingButAnHTTPRoot(t *testing.T) {
	calls := recordStart(t)
	for _, address := range refused {
		if err := Open(address); err == nil {
			t.Errorf("Open(%q) was accepted", address)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("the start seam was reached: %v", *calls)
	}
	t.Logf("%d values refused", len(refused))
}
