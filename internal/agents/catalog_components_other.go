//go:build !windows

package agents

// The parent-relative open checks every component with Lstat and compares
// opened identities. Windows additionally rejects all reparse-point kinds.
func checkCatalogComponents(string) error { return nil }
