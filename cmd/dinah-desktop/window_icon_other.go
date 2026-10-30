//go:build !windows

package main

// windowClass applies to Windows only.
const windowClass = ""

// setWindowIcon has nothing to do elsewhere: the toolkit sets the icon.
func setWindowIcon() {}
