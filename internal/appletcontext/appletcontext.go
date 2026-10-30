// Package appletcontext knows what a chat's applet context looks like and
// removes it from text. A context is the private authorization in each turn's
// prompt (dinah-applet-v1: followed by the host instance and the chat's token):
// it is valid for the chat's whole session and authorizes applet calls as that
// chat, so it must never appear in anything another chat can read or in
// anything kept longer than the session needs it (DESIGN.md).
package appletcontext

import "regexp"

// Prefix begins every context.
const Prefix = "dinah-applet-v1:"

// Redacted replaces a context that is removed.
const Redacted = "[private applet context]"

var pattern = regexp.MustCompile(`dinah-applet-v1:[A-Z2-7]{26,128}\.[A-Z2-7]{26,128}`)

// Redact replaces each context in text with Redacted.
func Redact(text string) string { return pattern.ReplaceAllString(text, Redacted) }

// RedactBytes is Redact for text held as bytes, such as a saved record. A
// context has only letters, digits, a colon, a dash and a dot, so replacing it
// in JSON leaves the document valid.
func RedactBytes(b []byte) []byte { return pattern.ReplaceAll(b, []byte(Redacted)) }

// Contains reports whether text holds a context.
func Contains(text string) bool { return pattern.MatchString(text) }
