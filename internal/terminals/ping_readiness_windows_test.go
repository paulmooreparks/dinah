//go:build windows

package terminals

import (
	"strings"
	"testing"
)

func pingHasDrawn(output string, writeOffset int) (bool, string) {
	// Parse from the beginning, since the recorded offset can split an OSC.
	before := terminalVisibleText(output[:writeOffset])
	visible := terminalVisibleText(output)[len(before):]
	// The command echo, ping's header and its first reply contain the address
	// in every language. Control-string payloads are not evidence of a reply.
	return strings.Count(visible, "127.0.0.1") >= 3, visible
}

// terminalVisibleText removes the escape sequences from this fixture's UTF-8
// ConPTY transcript. It does not emulate screen repainting. The sequence forms
// and OSC BEL/ST terminators follow https://invisible-island.net/xterm/ctlseqs/ctlseqs.html.
func terminalVisibleText(output string) string {
	var text strings.Builder
	for i := 0; i < len(output); {
		if output[i] != '\x1b' {
			text.WriteByte(output[i])
			i++
			continue
		}
		i++
		if i == len(output) {
			break
		}
		switch output[i] {
		case ']', 'P', 'X', '^', '_':
			osc := output[i] == ']'
			i++
			for i < len(output) {
				if osc && output[i] == '\a' {
					i++
					break
				}
				if output[i] == '\x1b' && i+1 < len(output) && output[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		case '[':
			i++
			for i < len(output) {
				final := output[i] >= 0x40 && output[i] <= 0x7e
				i++
				if final {
					break
				}
			}
		default:
			for i < len(output) && output[i] >= 0x20 && output[i] <= 0x2f {
				i++
			}
			if i < len(output) {
				i++
			}
		}
	}
	return text.String()
}

func TestPingReadinessNeedsVisibleOutput(t *testing.T) {
	// The retained failing transcript is inserted below, with its path normalised.
	const retained = "\u001b[?9001h\u001b[?1004h\u001b[?25l\u001b[2J\u001b[m\u001b[HMicrosoft Windows [Version 10.0.26300.9711]\u001b]0;C:\\WINDOWS\\system32\\cmd.exe\u0007\u001b[?25h\u001b[?25l\r\n(c) Microsoft Corporation. All rights reserved.\u001b[4;1HC:\\terminal-fixture>ping -n 100 127.0.0.1\r\n\u001b]0;C:\\WINDOWS\\system32\\cmd.exe - ping  -n 100 127.0.0.1\u0007\u001b[?25h"
	header := retained + "\r\nPinging 127.0.0.1 with 32 bytes of data:\r\n"
	reply := header + "Reply from 127.0.0.1: bytes=32 time<1ms TTL=128\r\n"
	prefix := "old 127.0.0.1 127.0.0.1 127.0.0.1\r\n"
	partialOSC := "\x1b]9;127.0.0.1 127.0.0.1 "
	for _, tc := range []struct {
		name   string
		output string
		offset int
		ready  bool
	}{
		{"retained failure", retained, 0, false},
		{"header without reply", header, 0, false},
		{"first reply", reply, 0, true},
		{"earlier output excluded", prefix + retained, len(prefix), false},
		{"OSC BEL", retained + "\x1b]9;127.0.0.1 127.0.0.1 127.0.0.1\a", 0, false},
		{"OSC ST", retained + "\x1b]9;127.0.0.1 127.0.0.1 127.0.0.1\x1b\\", 0, false},
		{"other control strings", retained + "\x1bP127.0.0.1\x1b\\\x1b_127.0.0.1\x1b\\", 0, false},
		{"split OSC", partialOSC + "127.0.0.1\a" + header, len(partialOSC), false},
		{"incomplete OSC", retained + "\x1b]9;127.0.0.1 127.0.0.1 127.0.0.1", 0, false},
		{"CSI and character set", "\x1b[2J\x1b(B" + reply, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, visible := pingHasDrawn(tc.output, tc.offset); got != tc.ready {
				t.Fatalf("ready=%v, want %v; visible=%q", got, tc.ready, visible)
			}
		})
	}
}
