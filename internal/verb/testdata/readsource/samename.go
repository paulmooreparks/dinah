package verb

import "os"

// want: reads os.ReadFile

// plantOther has a method named readSource, as the exempt free function is.
// The exemption is keyed on dinah/internal/verb.readSource's object, so this
// method, (dinah/internal/verb.plantOther).readSource, is judged.
type plantOther struct{ root string }

func (p plantOther) readSource() ([]byte, error) {
	return os.ReadFile(p.root + "/cards/x.md")
}
