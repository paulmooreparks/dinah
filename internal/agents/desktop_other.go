//go:build !windows

package agents

// DevinDesktop is "" outside Windows, where the installed programs are not
// listed in one documented place.
func DevinDesktop() string { return "" }
