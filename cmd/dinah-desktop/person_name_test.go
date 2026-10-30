package main

import (
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/bus"
)

// A delivered message from the person is headed with their name, the one
// they chose or else the account's, never their bus handle; other senders
// keep the bus's names, and the setting refuses what is not a name.
func TestHeadingsUseThePersonsName(t *testing.T) {
	a := NewApp()
	mine := &DeliveredMessage{Kind: "text", From: bus.Human}
	if account := accountName(); account != "" {
		if got := deliveredHeading(mine); got != "Message from "+account {
			t.Fatalf("with no name chosen: %q, want the account's name %q", got, account)
		}
	}
	// The bus file is shared by the tests of this package.
	t.Cleanup(func() { a.SetPersonName("") })
	if err := a.SetPersonName("P. Parks"); err != nil {
		t.Fatal(err)
	}
	if got := deliveredHeading(mine); got != "Message from P. Parks" {
		t.Fatalf("with a name chosen: %q", got)
	}
	if got := deliveredHeading(&DeliveredMessage{Kind: "text", From: "parser-2", Workspace: "Parser"}); got != "Message from parser-2 (workspace Parser)" {
		t.Fatalf("another sender: %q", got)
	}
	if st := a.State(); st.PersonName != "P. Parks" || st.BusHandle != bus.Human {
		t.Fatalf("the page is not told the name and the handle: %+v", st)
	}
	if err := a.SetPersonName(strings.Repeat("x", personNameLimit+1)); err == nil {
		t.Fatal("an overlong name was taken")
	}
	if err := a.SetPersonName("Paul\nParks"); err == nil {
		t.Fatal("a name with a line break was taken")
	}
}

// The name an earlier release kept in settings.json is carried into the bus
// once. A name already in the bus, even a cleared one, is never replaced by
// the old copy, and afterwards settings.json no longer has the name.
func TestPersonNameMovesFromSettingsToTheBus(t *testing.T) {
	for _, tc := range []struct {
		label     string
		legacy    string
		stored    *string
		want      string
		wantInBus string
	}{
		{"carried over", "Ada", nil, "Ada", "Ada"},
		{"blank old name", "  ", nil, "", ""},
		{"no name anywhere", "", nil, "", ""},
		{"the bus wins", "Ada", ptr("Grace"), "Grace", "Grace"},
		{"a cleared name stays cleared", "Ada", ptr(""), "", ""},
	} {
		t.Run(tc.label, func(t *testing.T) {
			isolateWorkspaceSettings(t)
			if tc.stored != nil {
				if err := withPersonBus(func(b *bus.Bus) error { return b.SetPersonName(*tc.stored) }); err != nil {
					t.Fatal(err)
				}
			}
			if err := saveSettings(Settings{LegacyPersonName: tc.legacy}); err != nil {
				t.Fatal(err)
			}
			// Starting twice changes nothing the second time.
			for range 2 {
				a := closeBusWhenDone(t, NewApp())
				if a.startupErr != nil {
					t.Fatal(a.startupErr)
				}
				if got := a.State().PersonName; got != tc.want {
					t.Fatalf("the page is told %q, want %q", got, tc.want)
				}
				if got := bus.PersonLabel(tc.want); a.agentPerson() != got {
					t.Fatalf("agents are told %q, want %q", a.agentPerson(), got)
				}
			}
			data, err := readStateFile(filepath.Join(configDir(), "settings.json"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "personName") {
				t.Fatalf("settings.json still has the name:\n%s", data)
			}
			err = withPersonBus(func(b *bus.Bus) error {
				name, stored, err := b.StoredPersonName()
				if strings.TrimSpace(tc.legacy) == "" && tc.stored == nil {
					if stored {
						t.Errorf("a name was stored out of nothing: %q", name)
					}
					return err
				}
				if !stored || name != tc.wantInBus {
					t.Errorf("the bus has %q (stored %v), want %q", name, stored, tc.wantInBus)
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func ptr(s string) *string { return &s }

// withPersonBus runs f on a handle of its own on the test's bus file, for a
// test that sets up or checks the bus apart from any App.
func withPersonBus(f func(*bus.Bus) error) error {
	b, err := bus.OpenAt(filepath.Join(configDir(), "bus.db"))
	if err != nil {
		return err
	}
	defer b.Close()
	return f(b)
}

// closeBusWhenDone closes the App's bus handle when the test ends. A test
// with a folder of its own would otherwise leave bus.db open, and Windows
// cannot remove the folder.
func closeBusWhenDone(t *testing.T, a *App) *App {
	t.Helper()
	t.Cleanup(func() { a.busHost().CloseHandle() })
	return a
}
