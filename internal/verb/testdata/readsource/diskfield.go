package verb

import "dinah/internal/bench"

// want: bottom rule: type plantHolder names the type dinah/internal/bench.Disk

// plantHolder is declared and never given a value, so the only naming of
// Disk is in a type position.
type plantHolder struct{ d bench.Disk }
