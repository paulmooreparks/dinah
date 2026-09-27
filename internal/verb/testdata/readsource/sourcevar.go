package verb

import "dinah/internal/bench"

// want: bottom rule: var plantS names the type dinah/internal/bench.Disk

// plantS holds a Disk as a Source, from its initializer.
var plantS bench.Source = bench.Disk{}

func plantReadThroughVariable(p string) ([]byte, error) {
	return plantS.ReadFile(p)
}
