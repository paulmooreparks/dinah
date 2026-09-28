package verb

import "dinah/internal/bench"

// want: bottom rule: dinah/internal/verb.plantOpenOnDisk names the type dinah/internal/bench.Disk

// plantOpenOnDisk opens a second workbench on the disk and reads a card
// through it.
func plantOpenOnDisk(root string) (*bench.Card, error) {
	b, err := bench.OpenWith(root, bench.Disk{})
	if err != nil {
		return nil, err
	}
	return b.LoadCardIn(b.CardsRoot(), "0123456789ab")
}
