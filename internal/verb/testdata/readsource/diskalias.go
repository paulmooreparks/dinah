package verb

import "dinah/internal/bench"

// want: names (dinah/internal/bench.Disk).ReadFile

// plantD is bench.Disk under another name, so the call below names no Disk
// and no read where it stands; its ReadFile resolves to Disk's own method.
type plantD = bench.Disk

func plantReadThroughAlias(p string) ([]byte, error) {
	return plantD{}.ReadFile(p)
}
