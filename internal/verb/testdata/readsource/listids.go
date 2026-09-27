package verb

import "dinah/internal/bench"

// want: names dinah/internal/bench.ListIDs

// plantedListIDs calls the free lister, which reads through Disk.
func (l *Library) plantedListIDs() ([]string, error) {
	return bench.ListIDs(l.Bench.CardsRoot())
}
