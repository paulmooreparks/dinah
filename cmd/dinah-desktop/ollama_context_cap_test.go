package main

import "testing"

func TestOllamaContextCapIsSavedAndRestored(t *testing.T) {
	a, _ := providerTestApp(t)
	if got := a.State().OllamaContextCap; got != 65536 {
		t.Fatalf("default cap = %d", got)
	}
	if err := a.SetOllamaContextCap(32768); err != nil {
		t.Fatal(err)
	}
	if s, err := loadSettings(); err != nil || s.OllamaContextCap != 32768 || a.State().OllamaContextCap != 32768 {
		t.Fatalf("saved cap = %d, %v", s.OllamaContextCap, err)
	}
	for _, n := range []int{-1, 1023, 2147483648} {
		if err := a.SetOllamaContextCap(n); err == nil || a.State().OllamaContextCap != 32768 {
			t.Fatalf("invalid cap %d changed the setting: %v", n, err)
		}
	}
	if err := a.SetOllamaContextCap(0); err != nil || a.State().OllamaContextCap != 65536 {
		t.Fatalf("restore default: %v", err)
	}
}
