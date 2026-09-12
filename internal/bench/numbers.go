package bench

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// NumberRegistry is the workbench's card-number registry as read from
// card-numbers.txt: the lines in file order, which is allocation order, plus
// the two indexes a read path asks for.
type NumberRegistry struct {
	Lines    []NumberLine     // file order, malformed lines included
	ByID     map[string]int   // identifier to the first number claiming it
	ByNumber map[int][]string // number to every identifier claiming it, file order
	Highest  int              // the greatest number any well-formed line carries
}

// NumberLine is one line of the registry.
type NumberLine struct {
	Number int    // zero on a malformed line
	ID     string // the identifier, "-" on a tombstone, "" on a malformed line
	Raw    string // the line as stored, which is what a malformed line reports
}

// LoadNumberRegistry reads the registry file, answering an empty registry for a
// workbench that has allocated no numbers. A malformed line is kept in Lines
// and enters neither index, because refusing on read would make a hand-damaged
// workbench unopenable, which is the posture FindingUnknownLevel and
// FindingUnknownColumn already keep.
func LoadNumberRegistry(path string) *NumberRegistry {
	registry := &NumberRegistry{ByID: map[string]int{}, ByNumber: map[int][]string{}}
	text, err := ReadText(path)
	if err != nil {
		return registry
	}
	lines := SplitLines(text)
	// Splitting a file whose final line ends in a newline yields one trailing
	// empty element, which is the newline rather than a line, so it is dropped.
	// Any other empty element is an interior blank line, which the grammar
	// forbids and which is reported as the malformed line it is.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, raw := range lines {
		line := parseNumberLine(raw)
		registry.Lines = append(registry.Lines, line)
		if line.Number == 0 {
			continue
		}
		if line.Number > registry.Highest {
			registry.Highest = line.Number
		}
		if line.ID == "-" {
			continue
		}
		if _, claimed := registry.ByID[line.ID]; !claimed {
			registry.ByID[line.ID] = line.Number
		}
		registry.ByNumber[line.Number] = append(registry.ByNumber[line.Number], line.ID)
	}
	return registry
}

// parseNumberLine reads one line under the grammar the format fixes: a decimal
// number of one or more digits with no leading zero and no sign, one ASCII
// space, and either a 12-hex card identifier or the tombstone character. A
// line that departs from the grammar in any way is malformed, carries no
// number and no identifier, and keeps the bytes it was stored under.
func parseNumberLine(raw string) NumberLine {
	space := strings.IndexByte(raw, ' ')
	if space < 1 {
		return NumberLine{Raw: raw}
	}
	digits, id := raw[:space], raw[space+1:]
	if len(digits) == 0 || digits[0] < '1' || digits[0] > '9' {
		return NumberLine{Raw: raw}
	}
	for i := 1; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return NumberLine{Raw: raw}
		}
	}
	if id != "-" && !IsID(id) {
		return NumberLine{Raw: raw}
	}
	number, err := strconv.Atoi(digits)
	if err != nil {
		return NumberLine{Raw: raw}
	}
	return NumberLine{Number: number, ID: id, Raw: raw}
}

// readNumbers answers the registry this bench holds. On a workbench at
// RegistryFormat or above the file is the whole of it. Below that the file is
// absent in the ordinary case and the by-number index is synthesized from the
// numbers the cards still carry in frontmatter, so resolution by number and
// dinah-487's ambiguity refusal keep answering over a workbench the migration
// has not reached while Lines, ByID and Highest stay empty, which is what
// makes check report every card as carrying no line.
//
// The synthesis walks both halves through the stamping readers, so a card a
// registry line already claims is read from that line rather than from its
// frontmatter and cannot arrive twice. A card or a half that will not read is
// passed over, because a hand-damaged workbench has to stay openable; the
// state is what check reports.
func (b *Bench) readNumbers() *NumberRegistry {
	registry := LoadNumberRegistry(filepath.Join(b.Root, CardNumbersName))
	if b.Format >= RegistryFormat {
		return registry
	}
	for _, root := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
		ids, err := ListIDs(root)
		if err != nil {
			continue
		}
		for _, id := range ids {
			if _, claimed := registry.ByID[id]; claimed {
				continue
			}
			var card *Card
			if b.retiredVocabulary {
				card, err = b.loadRetiredCardIn(root, id)
			} else {
				card, err = b.LoadCardIn(root, id)
			}
			if err != nil {
				continue
			}
			if card.Number == 0 {
				continue
			}
			registry.ByNumber[card.Number] = append(registry.ByNumber[card.Number], id)
		}
	}
	return registry
}

// ReloadNumbers reads the registry file back after a mutation of it, so the
// bench the caller holds and the file on disk cannot disagree about what
// number a card answers to. One file, one parser: every mutation goes through
// a write below and then through here.
func (b *Bench) ReloadNumbers() {
	b.Numbers = b.readNumbers()
}

// AppendNumberLine adds one allocated number to the registry, creating the
// file at the first allocation. The write is an append rather than a rewrite,
// on the same terms AppendEvent writes a journal: a crash can tear at most the
// final line and never the numbers already in the file.
func AppendNumberLine(path string, number int, id string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(strconv.Itoa(number) + " " + id + "\n"); err != nil {
		return err
	}
	return f.Sync()
}

// WriteNumberLines rewrites the registry from whole lines, which is the write a
// tombstone, the renumber repair and the migration all make. It rewrites every
// line it is given including the ones it did not change, because each of
// those callers has composed the whole file and a partial rewrite would leave
// the file disagreeing with the registry it was composed from.
func WriteNumberLines(path string, lines []string) error {
	text := strings.Join(lines, "\n")
	if text != "" {
		text += "\n"
	}
	return WriteText(path, text)
}
