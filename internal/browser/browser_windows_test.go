//go:build windows

package browser

import "testing"

// recordShellExecute replaces the ShellExecute seam with a recorder for the
// length of a test.
func recordShellExecute(t *testing.T) *[][2]string {
	t.Helper()
	var calls [][2]string
	previous := shellExecute
	shellExecute = func(verb, file string) error {
		calls = append(calls, [2]string{verb, file})
		return nil
	}
	t.Cleanup(func() { shellExecute = previous })
	return &calls
}

// TestOpenHandsTheURLToTheDocumentedMechanism holds Open to ShellExecute
// with the verb open and the URL as the one file argument.
func TestOpenHandsTheURLToTheDocumentedMechanism(t *testing.T) {
	calls := recordShellExecute(t)
	for _, address := range accepted {
		if err := Open(address); err != nil {
			t.Errorf("Open(%q) = %v", address, err)
		}
	}
	if len(*calls) != len(accepted) {
		t.Fatalf("ShellExecute was called %d times for %d URLs: %v", len(*calls), len(accepted), *calls)
	}
	for i, address := range accepted {
		if got := (*calls)[i]; got != [2]string{"open", address} {
			t.Errorf("ShellExecute got %q, want open %q", got, address)
		}
	}
}

// TestOpenRefusesAnythingButAnHTTPRoot holds Open to refusing every value
// but an http root URL before ShellExecute is reached.
func TestOpenRefusesAnythingButAnHTTPRoot(t *testing.T) {
	calls := recordShellExecute(t)
	for _, address := range refused {
		if err := Open(address); err == nil {
			t.Errorf("Open(%q) was accepted", address)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("ShellExecute was reached: %v", *calls)
	}
	t.Logf("%d values refused", len(refused))
}
