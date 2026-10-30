// Package credentials keeps secrets, such as the person's Claude API key, in
// the system keychain.
package credentials

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/99designs/keyring"

	"studio/internal/profile"
)

// ErrNotFound reports that the keychain answered and the key is simply not
// stored in it. A caller wraps it into "you have not configured this yet",
// which is a different thing to tell an operator than "your keychain is
// broken"; without the sentinel both arrive as one opaque backend error.
var ErrNotFound = errors.New("credentials: key not stored in the keychain")

const (
	// KeyClaudeAPIKey is the person's Claude API key. A stored key is the
	// choice to bill Claude to it; with none, Claude uses the person's own
	// Claude Code sign-in (package claudeagent).
	KeyClaudeAPIKey = "claude-api-key"
)

// serviceName is the keychain service the keys are kept under, which the
// profile gives (profile.KeychainService).
var serviceName = profile.Current().KeychainService

// Set stores a credential value in the system keychain.
func Set(key, value string) error {
	kr, err := openKeyring()
	if err != nil {
		return fmt.Errorf("credentials: cannot open keyring: %w", err)
	}
	return kr.Set(keyring.Item{
		Key:   key,
		Data:  []byte(value),
		Label: serviceName + "/" + key,
	})
}

// Remove deletes a credential from the system keychain; a key it does not
// hold is not an error.
func Remove(key string) error {
	kr, err := openKeyring()
	if err != nil {
		return fmt.Errorf("credentials: cannot open keyring: %w", err)
	}
	if err := kr.Remove(key); err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
		return err
	}
	return nil
}

// Get retrieves a credential value from the system keychain. A key the
// keychain does not hold comes back as ErrNotFound; every other failure comes
// back with the backend's own error wrapped.
func Get(key string) (string, error) {
	return get(openKeyring, key)
}

// get is Get with the keyring opener injected, so a test can drive the panic
// and not-found paths without a real keychain.
//
// It recovers from panics in the underlying backend: some Windows session
// contexts crash inside wincred, and an error tells the person far more than
// a stack trace.
func get(open func() (keyring.Keyring, error), key string) (value string, err error) {
	defer func() {
		if r := recover(); r != nil {
			value, err = "", fmt.Errorf("credentials: keyring access panicked: %v", r)
		}
	}()

	kr, err := open()
	if err != nil {
		return "", fmt.Errorf("credentials: cannot open keyring: %w", err)
	}
	item, err := kr.Get(key)
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return "", fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err != nil {
		return "", err
	}
	return string(item.Data), nil
}

// openKeyring opens the platform's own store and no other. Left to choose,
// the library falls through to whatever it finds, such as the pass program
// or an encrypted file store that has no folder configured and fails on
// first use, so a secret could be written where no other copy of this tool
// looks. With the store named, a machine without it fails here.
func openKeyring() (keyring.Keyring, error) {
	kr, err := keyring.Open(keyring.Config{
		ServiceName:     serviceName,
		AllowedBackends: platformBackends(runtime.GOOS),
	})
	if errors.Is(err, keyring.ErrNoAvailImpl) {
		return nil, fmt.Errorf("%w: no %s on this machine", err, StoreName())
	}
	return kr, err
}

// platformBackends are the stores this tool uses on each operating system:
// Windows Credential Manager, the macOS Keychain, and on Linux the Secret
// Service of the desktop session or KWallet. The Keychain needs a build made
// with C interop on a Mac; a build without it has no store on macOS.
func platformBackends(goos string) []keyring.BackendType {
	switch goos {
	case "windows":
		return []keyring.BackendType{keyring.WinCredBackend}
	case "darwin":
		return []keyring.BackendType{keyring.KeychainBackend}
	default:
		return []keyring.BackendType{keyring.SecretServiceBackend, keyring.KWalletBackend}
	}
}

// StoreName is the name of this platform's store, as a person knows it.
func StoreName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows Credential Manager"
	case "darwin":
		return "macOS Keychain"
	default:
		return "desktop keyring (Secret Service or KWallet)"
	}
}
