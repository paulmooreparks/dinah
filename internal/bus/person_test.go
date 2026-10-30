package bus

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestPersonName checks that text for agents calls the person by the name
// the window stored in the bus, and "the person" without one, and that a
// cleared name differs from one never chosen.
func TestPersonName(t *testing.T) {
	dir := t.TempDir()
	b, err := OpenAt(filepath.Join(dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if got := b.PersonName(); got != PersonFallback {
		t.Fatalf("with no name the person is %q, want %q", got, PersonFallback)
	}
	if _, stored, err := b.StoredPersonName(); err != nil || stored {
		t.Fatalf("a name was stored before any was set: %v, %v", stored, err)
	}
	if err := b.SetPersonName("Ada"); err != nil {
		t.Fatal(err)
	}
	if got := b.PersonName(); got != "Ada" {
		t.Fatalf("the person is %q, want Ada", got)
	}
	// Another process opening the same file reads the same name.
	other, err := OpenAt(filepath.Join(dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if got := other.PersonName(); got != "Ada" {
		t.Fatalf("a second handle reads %q, want Ada", got)
	}
	if err := b.SetPersonName("  "); err != nil {
		t.Fatal(err)
	}
	if got := b.PersonName(); got != PersonFallback {
		t.Fatalf("a blank name gives %q, want %q", got, PersonFallback)
	}
	if err := b.SetPersonName(""); err != nil {
		t.Fatal(err)
	}
	if name, stored, err := b.StoredPersonName(); err != nil || !stored || name != "" {
		t.Fatalf("a cleared name reads as %q, stored %v, %v", name, stored, err)
	}
}

// TestImportPersonName checks that carrying a name over from where it used
// to be kept never replaces one the person has set or cleared since.
func TestImportPersonName(t *testing.T) {
	b, err := OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if did, err := b.ImportPersonName("Ada"); err != nil || !did {
		t.Fatalf("importing into an empty bus: %v, %v", did, err)
	}
	if did, err := b.ImportPersonName("Grace"); err != nil || did {
		t.Fatalf("importing over a stored name: %v, %v", did, err)
	}
	if got := b.PersonName(); got != "Ada" {
		t.Fatalf("the person is %q, want Ada", got)
	}
	if err := b.SetPersonName(""); err != nil {
		t.Fatal(err)
	}
	if did, err := b.ImportPersonName("Grace"); err != nil || did {
		t.Fatalf("importing over a cleared name: %v, %v", did, err)
	}
	if got := b.PersonName(); got != PersonFallback {
		t.Fatalf("a cleared name came back as %q", got)
	}
}

// A database that cannot be read gives the fallback instead of failing.
func TestPersonNameUnreadable(t *testing.T) {
	b, err := OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetPersonName("Ada"); err != nil {
		t.Fatal(err)
	}
	b.Close()
	if got := b.PersonName(); got != PersonFallback {
		t.Fatalf("a closed database gives %q, want %q", got, PersonFallback)
	}
}

// TestFormatCallsThePersonByName checks the person's name in what an agent
// is told, and the capital letter where the label begins a sentence.
func TestFormatCallsThePersonByName(t *testing.T) {
	m := Message{ID: 1, FromName: "beta", FromWorkspace: "alpha", ToLabel: "gamma", Kind: "text", Text: "hi", Trusted: true}
	got := FormatForAgent([]Message{m}, "Ada")
	if !strings.Contains(got, "from Ada, or from Dinah") || !strings.Contains(got, "Ada has marked the link") || strings.Contains(got, "Paul") {
		t.Fatalf("the message reads:\n%s", got)
	}
	if got := FormatOne(m, PersonFallback); !strings.Contains(got, "The person has marked the link") || !strings.Contains(got, "a request from the person.") {
		t.Fatalf("the message reads:\n%s", got)
	}
	if got := FormatSteers([]Steer{{Text: "stop"}}, PersonFallback); !strings.HasPrefix(got, "The person steered you") {
		t.Fatalf("the steer reads:\n%s", got)
	}
}
