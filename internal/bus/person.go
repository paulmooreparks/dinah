package bus

import (
	"database/sql"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The text agents are told calls the person who runs the bus by the name
// they chose in Settings › Chat, and "the person" when they chose none. The
// bus handle, Human, is a different thing: it is the address stored in
// messages, and it does not change with the name.

// PersonFallback is what text for agents calls the person when Settings has
// no name for them.
const PersonFallback = "the person"

// PersonLabel returns name as text for agents writes it: the name, or
// PersonFallback when it is empty.
func PersonLabel(name string) string {
	if name = strings.TrimSpace(name); name != "" {
		return name
	}
	return PersonFallback
}

// SentenceStart returns s with its first letter in upper case, for a
// person's label that begins a sentence ("The person has marked ...").
func SentenceStart(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// PersonName returns the person's name for text for agents, from the bus's
// own database, where the window stores the name the person chose in
// Settings › Chat (SetPersonName). The command line and the bus's helpers
// run in processes of their own, so they read it here; the window passes
// the name it already has instead. No stored name, a cleared one and an
// unreadable database all give PersonFallback.
func (b *Bus) PersonName() string {
	name, _, err := b.StoredPersonName()
	if err != nil {
		return PersonFallback
	}
	return PersonLabel(name)
}

// StoredPersonName returns the name the window stored and whether it stored
// one at all. A stored empty name means the person cleared it, which is
// different from never having chosen one.
func (b *Bus) StoredPersonName() (name string, stored bool, err error) {
	err = b.db.QueryRow(`SELECT name FROM person WHERE id = 1`).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}

// SetPersonName stores the person's name, replacing any stored one. An
// empty name records that the person cleared it.
func (b *Bus) SetPersonName(name string) error {
	_, err := b.db.Exec(`INSERT INTO person (id, name) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET name = excluded.name`, name)
	return err
}

// ImportPersonName stores name only when nothing is stored yet, and says
// whether it did. It is for carrying a name over from where it used to be
// kept, so a name the person has set or cleared since is never replaced.
func (b *Bus) ImportPersonName(name string) (bool, error) {
	res, err := b.db.Exec(`INSERT OR IGNORE INTO person (id, name) VALUES (1, ?)`, name)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
