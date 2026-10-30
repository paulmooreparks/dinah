package credentials

// Coverage for the keychain read path. Two behaviours matter to a caller: a
// key the keychain does not hold has to be distinguishable from a keychain
// that is broken, and a backend that panics has to arrive as an ordinary
// error.

import (
	"errors"
	"strings"
	"testing"

	"github.com/99designs/keyring"
)

// stubKeyring is a keyring.Keyring whose Get is whatever the test supplies.
// The other four methods are unused here and fail loudly if that changes.
type stubKeyring struct {
	get func(key string) (keyring.Item, error)
}

func (s *stubKeyring) Get(key string) (keyring.Item, error) { return s.get(key) }

func (s *stubKeyring) GetMetadata(string) (keyring.Metadata, error) {
	panic("stubKeyring.GetMetadata is not part of the read path under test")
}
func (s *stubKeyring) Set(keyring.Item) error {
	panic("stubKeyring.Set is not part of the read path under test")
}
func (s *stubKeyring) Remove(string) error {
	panic("stubKeyring.Remove is not part of the read path under test")
}
func (s *stubKeyring) Keys() ([]string, error) {
	panic("stubKeyring.Keys is not part of the read path under test")
}

func opener(kr keyring.Keyring) func() (keyring.Keyring, error) {
	return func() (keyring.Keyring, error) { return kr, nil }
}

func TestGetReturnsTheStoredValue(t *testing.T) {
	kr := &stubKeyring{get: func(key string) (keyring.Item, error) {
		if key != KeyClaudeAPIKey {
			t.Errorf("backend asked for %q, want %q", key, KeyClaudeAPIKey)
		}
		return keyring.Item{Key: key, Data: []byte("stored-value")}, nil
	}}

	got, err := get(opener(kr), KeyClaudeAPIKey)
	if err != nil {
		t.Fatalf("get() error = %v", err)
	}
	if got != "stored-value" {
		t.Errorf("get() = %q, want %q", got, "stored-value")
	}
}

// A key nobody has configured yet is an ordinary outcome, and it has to carry
// ErrNotFound so the caller can tell the person to configure it instead of
// reporting the keychain as unavailable.
func TestGetReportsAMissingKeyAsErrNotFound(t *testing.T) {
	kr := &stubKeyring{get: func(string) (keyring.Item, error) {
		return keyring.Item{}, keyring.ErrKeyNotFound
	}}

	_, err := get(opener(kr), KeyClaudeAPIKey)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("get() error = %v, want it to wrap ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), KeyClaudeAPIKey) {
		t.Errorf("error %q does not say which key is missing", err)
	}
}

// A backend failure that is not a missing key stays a backend failure, so a
// real keychain fault is not reported as an unconfigured key.
func TestGetPassesThroughOtherBackendErrors(t *testing.T) {
	backendErr := errors.New("wincred: access denied")
	kr := &stubKeyring{get: func(string) (keyring.Item, error) {
		return keyring.Item{}, backendErr
	}}

	_, err := get(opener(kr), KeyClaudeAPIKey)
	if !errors.Is(err, backendErr) {
		t.Fatalf("get() error = %v, want it to wrap the backend error", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("a backend fault was reported as a missing key")
	}
}

func TestGetWrapsAnOpenFailure(t *testing.T) {
	openErr := errors.New("no available backend")
	_, err := get(func() (keyring.Keyring, error) { return nil, openErr }, KeyClaudeAPIKey)
	if !errors.Is(err, openErr) {
		t.Fatalf("get() error = %v, want it to wrap the open error", err)
	}
	if !strings.Contains(err.Error(), "cannot open keyring") {
		t.Errorf("error %q does not say the keyring could not be opened", err)
	}
}

// Some Windows session contexts crash inside wincred; the read guards
// against it, so the person gets a message rather than a stack trace.
func TestGetRecoversFromAPanickingBackend(t *testing.T) {
	cases := []struct {
		name string
		open func() (keyring.Keyring, error)
	}{
		{
			name: "the opener panics",
			open: func() (keyring.Keyring, error) { panic("wincred exploded on open") },
		},
		{
			name: "the backend panics on Get",
			open: opener(&stubKeyring{get: func(string) (keyring.Item, error) {
				panic("wincred exploded on read")
			}}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := get(tc.open, KeyClaudeAPIKey)
			if err == nil {
				t.Fatalf("get() = %q, want an error rather than a panic escaping", got)
			}
			if !strings.Contains(err.Error(), "panicked") {
				t.Errorf("error %q does not say the keyring access panicked", err)
			}
			if got != "" {
				t.Errorf("get() = %q, want an empty value alongside the error", got)
			}
		})
	}
}

// TestPlatformBackends checks that each platform opens its own store and no
// other, so that a secret is never written where another copy of the tool
// does not look.
func TestPlatformBackends(t *testing.T) {
	cases := map[string][]keyring.BackendType{
		"windows": {keyring.WinCredBackend},
		"darwin":  {keyring.KeychainBackend},
		"linux":   {keyring.SecretServiceBackend, keyring.KWalletBackend},
	}
	for goos, want := range cases {
		got := platformBackends(goos)
		if len(got) != len(want) {
			t.Fatalf("%s: got %v, want %v", goos, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: got %v, want %v", goos, got, want)
			}
		}
	}
}
