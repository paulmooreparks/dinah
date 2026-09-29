package verb

import "dinah/internal/bench"

// want: names (dinah/internal/bench.Disk).ReadFile

// plantEmbedded embeds bench.Disk, so its ReadFile is Disk's, promoted.
type plantEmbedded struct{ bench.Disk }

func plantReadThroughEmbedding(p string) ([]byte, error) {
	return plantEmbedded{}.ReadFile(p)
}
