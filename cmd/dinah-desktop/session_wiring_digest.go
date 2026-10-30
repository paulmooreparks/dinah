package main

import _ "embed"

//go:embed agent_handler.go
var sessionWiringSource0 []byte

//go:embed turn_io.go
var sessionWiringSource1 []byte

//go:embed provider_lifecycle.go
var sessionWiringSource2 []byte

//go:embed session_lifecycle.go
var sessionWiringSource3 []byte

//go:embed closed_sessions.go
var sessionWiringSource4 []byte

//go:embed subagent_host.go
var sessionWiringSource5 []byte

//go:embed subagents.go
var sessionWiringSource6 []byte

//go:embed continuation_host.go
var sessionWiringSource7 []byte

//go:embed continuation.go
var sessionWiringSource8 []byte

//go:embed session_recovery.go
var sessionWiringSource9 []byte

//go:embed transcript_capture.go
var sessionWiringSource10 []byte

//go:embed delivered.go
var sessionWiringSource11 []byte

//go:embed options.go
var sessionWiringSource12 []byte

//go:embed option_substitutions.go
var sessionWiringSource13 []byte

//go:embed steering_recovery.go
var sessionWiringSource14 []byte

// sessionWiringDigestSources includes shell adapters that determine chat behavior.
var sessionWiringDigestSources = map[string][]byte{
	"agent_handler.go":        sessionWiringSource0,
	"turn_io.go":              sessionWiringSource1,
	"provider_lifecycle.go":   sessionWiringSource2,
	"session_lifecycle.go":    sessionWiringSource3,
	"closed_sessions.go":      sessionWiringSource4,
	"subagent_host.go":        sessionWiringSource5,
	"subagents.go":            sessionWiringSource6,
	"continuation_host.go":    sessionWiringSource7,
	"continuation.go":         sessionWiringSource8,
	"session_recovery.go":     sessionWiringSource9,
	"transcript_capture.go":   sessionWiringSource10,
	"delivered.go":            sessionWiringSource11,
	"options.go":              sessionWiringSource12,
	"option_substitutions.go": sessionWiringSource13,
	"steering_recovery.go":    sessionWiringSource14,
}
